package product

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/leyl1ne/ProductService/internal/service"
	productservice "github.com/leyl1ne/ProductService/internal/service/product"
	"github.com/leyl1ne/ProductService/internal/transport/http/middleware"
	"github.com/leyl1ne/ProductService/internal/transport/http/response"
	"github.com/leyl1ne/ProductService/pkg/logger"
)

type ProductService interface {
	CreateProduct(ctx context.Context, input productservice.CreateProductInput) (*productservice.ProductOutput, error)
	GetProductByID(ctx context.Context, id uuid.UUID) (*productservice.ProductOutput, error)
	ListProducts(ctx context.Context, filter productservice.ProductFilter, page, limit int) (*productservice.ProductListOutput, error)
	UpdateProduct(ctx context.Context, id uuid.UUID, input productservice.UpdateProductInput) (*productservice.ProductOutput, error)
	DeleteProduct(ctx context.Context, id uuid.UUID) error
	GetProductAvailability(ctx context.Context, productID uuid.UUID) (*productservice.ProductAvailabilityOutput, error)
}

type Handler struct {
	log            logger.Logger
	productService ProductService
}

func NewHandler(log logger.Logger, productService ProductService) *Handler {
	return &Handler{
		log:            log,
		productService: productService,
	}
}

// CreateProduct godoc
// @Summary      Create product
// @Description  Creates a new product. SELLER or ADMIN only.
// @Tags         Products
// @Security     bearerAuth
// @Accept       json
// @Produce      json
// @Param        request  body      CreateProductRequest  true  "Product data"
// @Success      201  {object}  ProductResponse
// @Failure      400  {object}  response.ErrorResponse
// @Failure      401  {object}  response.ErrorResponse
// @Failure      403  {object}  response.ErrorResponse
// @Failure      500  {object}  response.ErrorResponse
// @Router       /products [post]
func (h *Handler) CreateProduct() gin.HandlerFunc {
	return func(c *gin.Context) {
		const op = "handler.product.CreateProduct"

		log := h.log.With(
			logger.Field{Key: "op", Value: op},
			logger.Field{Key: "request_id", Value: middleware.GetRequestID(c)},
		)

		var req CreateProductRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.WriteBindError(c, err)
			log.Warn("invalid request body", logger.Err(err))
			return
		}

		input, err := req.ToInput()
		if err != nil {
			response.WriteError(c, http.StatusBadRequest, "invalid category_id")
			log.Warn("invalid category_id", logger.Err(err))
			return
		}

		product, err := h.productService.CreateProduct(c.Request.Context(), input)
		if err != nil {
			switch {
			case errors.Is(err, service.ErrForbidden):
				response.WriteError(c, http.StatusForbidden, "forbidden")
				log.Warn("forbidden", logger.Err(err))
				return
			case errors.Is(err, service.ErrCategoryNotFound):
				response.WriteError(c, http.StatusBadRequest, "category not found")
				log.Warn("category not found", logger.Err(err))
				return
			default:
				response.WriteInternalServerError(c)
				log.Error("internal server error", logger.Err(err))
				return
			}
		}

		c.JSON(http.StatusCreated, toProductResponse(product))
	}
}

// GetProduct godoc
// @Summary      Get product by id
// @Description  Returns a product by ID. Available to all users.
// @Tags         Products
// @Produce      json
// @Param        id  path  string  true  "Product ID (UUID)"
// @Success      200  {object}  ProductResponse
// @Failure      400  {object}  response.ErrorResponse
// @Failure      404  {object}  response.ErrorResponse
// @Failure      500  {object}  response.ErrorResponse
// @Router       /products/{id} [get]
func (h *Handler) GetProduct() gin.HandlerFunc {
	return func(c *gin.Context) {
		const op = "handler.product.GetProduct"

		log := h.log.With(
			logger.Field{Key: "op", Value: op},
			logger.Field{Key: "request_id", Value: middleware.GetRequestID(c)},
		)

		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			response.WriteError(c, http.StatusBadRequest, "invalid product id")
			log.Warn("invalid product id", logger.Err(err))
			return
		}

		product, err := h.productService.GetProductByID(c.Request.Context(), id)
		if err != nil {
			switch {
			case errors.Is(err, service.ErrProductNotFound):
				response.WriteError(c, http.StatusNotFound, "product not found")
				log.Warn("product not found", logger.Err(err))
				return
			default:
				response.WriteInternalServerError(c)
				log.Error("internal server error", logger.Err(err))
				return
			}
		}

		c.JSON(http.StatusOK, toProductResponse(product))
	}
}

// ListProducts godoc
// @Summary      Get products list
// @Description  Returns a paginated list of products with optional filters. Available to all users.
// @Tags         Products
// @Produce      json
// @Param        category_id  query     string  false  "Filter by category UUID"
// @Param        company_id   query     string  false  "Filter by company UUID"
// @Param        min_price    query     number  false  "Minimum price"
// @Param        max_price    query     number  false  "Maximum price"
// @Param        page         query     int     false  "Page number"  default(1)
// @Param        limit        query     int     false  "Items per page"  default(20)
// @Success      200  {object}  ProductListResponse
// @Failure      400  {object}  response.ErrorResponse
// @Failure      500  {object}  response.ErrorResponse
// @Router       /products [get]
func (h *Handler) ListProducts() gin.HandlerFunc {
	return func(c *gin.Context) {
		const op = "handler.product.ListProducts"

		log := h.log.With(
			logger.Field{Key: "op", Value: op},
			logger.Field{Key: "request_id", Value: middleware.GetRequestID(c)},
		)

		var query ListProductsQuery
		if err := c.ShouldBindQuery(&query); err != nil {
			response.WriteBindError(c, err)
			log.Warn("invalid query parameters", logger.Err(err))
			return
		}

		// Defaults
		if query.Page == 0 {
			query.Page = 1
		}
		if query.Limit == 0 {
			query.Limit = 20
		}

		filter, err := query.ToFilter()
		if err != nil {
			response.WriteError(c, http.StatusBadRequest, "invalid filter parameter")
			log.Warn("invalid filter parameter", logger.Err(err))
			return
		}

		result, err := h.productService.ListProducts(c.Request.Context(), filter, query.Page, query.Limit)
		if err != nil {
			response.WriteInternalServerError(c)
			log.Error("internal server error", logger.Err(err))
			return
		}

		c.JSON(http.StatusOK, toProductListResponse(result, query.Page, query.Limit))
	}
}

// UpdateProduct godoc
// @Summary      Update product
// @Description  Partially updates a product. Owner of the product's company or ADMIN only.
// @Tags         Products
// @Security     bearerAuth
// @Accept       json
// @Produce      json
// @Param        id       path      string                true  "Product ID (UUID)"
// @Param        request  body      UpdateProductRequest  true  "Fields to update"
// @Success      200  {object}  ProductResponse
// @Failure      400  {object}  response.ErrorResponse
// @Failure      401  {object}  response.ErrorResponse
// @Failure      403  {object}  response.ErrorResponse
// @Failure      404  {object}  response.ErrorResponse
// @Failure      500  {object}  response.ErrorResponse
// @Router       /products/{id} [patch]
func (h *Handler) UpdateProduct() gin.HandlerFunc {
	return func(c *gin.Context) {
		const op = "handler.product.UpdateProduct"

		log := h.log.With(
			logger.Field{Key: "op", Value: op},
			logger.Field{Key: "request_id", Value: middleware.GetRequestID(c)},
		)

		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			response.WriteError(c, http.StatusBadRequest, "invalid product id")
			log.Warn("invalid product id", logger.Err(err))
			return
		}

		var req UpdateProductRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.WriteBindError(c, err)
			log.Warn("invalid request body", logger.Err(err))
			return
		}

		input, err := req.ToInput()
		if err != nil {
			response.WriteError(c, http.StatusBadRequest, "invalid category_id")
			log.Warn("invalid category_id", logger.Err(err))
			return
		}

		product, err := h.productService.UpdateProduct(c.Request.Context(), id, input)
		if err != nil {
			switch {
			case errors.Is(err, service.ErrForbidden):
				response.WriteError(c, http.StatusForbidden, "forbidden")
				log.Warn("forbidden", logger.Err(err))
				return
			case errors.Is(err, service.ErrProductNotFound):
				response.WriteError(c, http.StatusNotFound, "product not found")
				log.Warn("product not found", logger.Err(err))
				return
			case errors.Is(err, service.ErrCategoryNotFound):
				response.WriteError(c, http.StatusBadRequest, "category not found")
				log.Warn("category not found", logger.Err(err))
				return
			default:
				response.WriteInternalServerError(c)
				log.Error("internal server error", logger.Err(err))
				return
			}
		}

		c.JSON(http.StatusOK, toProductResponse(product))
	}
}

// DeleteProduct godoc
// @Summary      Delete product
// @Description  Deletes a product. Owner of the product's company or ADMIN only.
// @Tags         Products
// @Security     bearerAuth
// @Param        id  path  string  true  "Product ID (UUID)"
// @Success      204
// @Failure      400  {object}  response.ErrorResponse
// @Failure      401  {object}  response.ErrorResponse
// @Failure      403  {object}  response.ErrorResponse
// @Failure      404  {object}  response.ErrorResponse
// @Failure      500  {object}  response.ErrorResponse
// @Router       /products/{id} [delete]
func (h *Handler) DeleteProduct() gin.HandlerFunc {
	return func(c *gin.Context) {
		const op = "handler.product.DeleteProduct"

		log := h.log.With(
			logger.Field{Key: "op", Value: op},
			logger.Field{Key: "request_id", Value: middleware.GetRequestID(c)},
		)

		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			response.WriteError(c, http.StatusBadRequest, "invalid product id")
			log.Warn("invalid product id", logger.Err(err))
			return
		}

		if err := h.productService.DeleteProduct(c.Request.Context(), id); err != nil {
			switch {
			case errors.Is(err, service.ErrForbidden):
				response.WriteError(c, http.StatusForbidden, "forbidden")
				log.Warn("forbidden", logger.Err(err))
				return
			case errors.Is(err, service.ErrProductNotFound):
				response.WriteError(c, http.StatusNotFound, "product not found")
				log.Warn("product not found", logger.Err(err))
				return
			default:
				response.WriteInternalServerError(c)
				log.Error("internal server error", logger.Err(err))
				return
			}
		}

		c.Status(http.StatusNoContent)
	}
}

// GetProductAvailability godoc
// @Summary      Get product availability
// @Description  Returns all batches of the product that have available stock (FIFO by expiration date).
// @Tags         Products
// @Produce      json
// @Param        id  path  string  true  "Product ID (UUID)"
// @Success      200  {object}  ProductAvailabilityResponse
// @Failure      400  {object}  response.ErrorResponse
// @Failure      404  {object}  response.ErrorResponse
// @Failure      500  {object}  response.ErrorResponse
// @Router       /products/{id}/availability [get]
func (h *Handler) GetProductAvailability() gin.HandlerFunc {
	return func(c *gin.Context) {
		const op = "handler.product.GetProductAvailability"

		log := h.log.With(
			logger.Field{Key: "op", Value: op},
			logger.Field{Key: "request_id", Value: middleware.GetRequestID(c)},
		)

		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			response.WriteError(c, http.StatusBadRequest, "invalid product id")
			log.Warn("invalid product id", logger.Err(err))
			return
		}

		availability, err := h.productService.GetProductAvailability(c.Request.Context(), id)
		if err != nil {
			switch {
			case errors.Is(err, service.ErrProductNotFound):
				response.WriteError(c, http.StatusNotFound, "product not found")
				log.Warn("product not found", logger.Err(err))
				return
			default:
				response.WriteInternalServerError(c)
				log.Error("internal server error", logger.Err(err))
				return
			}
		}

		c.JSON(http.StatusOK, toProductAvailabilityResponse(availability))
	}
}

// parsePageLimit is a helper for normalising pagination defaults in handlers
// where the query struct is not directly used (kept for reuse in other packages).
func parsePageLimit(pageStr, limitStr string) (int, int) {
	page, _ := strconv.Atoi(pageStr)
	if page <= 0 {
		page = 1
	}
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return page, limit
}

package category

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/leyl1ne/ProductService/internal/service"
	categoryservice "github.com/leyl1ne/ProductService/internal/service/category"
	"github.com/leyl1ne/ProductService/internal/transport/http/middleware"
	"github.com/leyl1ne/ProductService/internal/transport/http/response"
	"github.com/leyl1ne/ProductService/pkg/logger"
)

type CategoryService interface {
	ListCategories(ctx context.Context) ([]categoryservice.CategoryOutput, error)
	CreateCategory(ctx context.Context, input categoryservice.CreateCategoryInput) (*categoryservice.CategoryOutput, error)
	UpdateCategory(ctx context.Context, id uuid.UUID, input categoryservice.UpdateCategoryInput) (*categoryservice.CategoryOutput, error)
	DeleteCategory(ctx context.Context, id uuid.UUID) error
}

type Handler struct {
	log             logger.Logger
	categoryService CategoryService
}

func NewHandler(log logger.Logger, categoryService CategoryService) *Handler {
	return &Handler{
		log:             log,
		categoryService: categoryService,
	}
}

// ListCategories godoc
// @Summary      Get categories
// @Description  Returns all categories. Available to all users.
// @Tags         Categories
// @Produce      json
// @Success      200  {object}  CategoryListResponse
// @Failure      500  {object}  response.ErrorResponse
// @Router       /categories [get]
func (h *Handler) ListCategories() gin.HandlerFunc {
	return func(c *gin.Context) {
		const op = "handler.category.ListCategories"

		log := h.log.With(
			logger.Field{Key: "op", Value: op},
			logger.Field{Key: "request_id", Value: middleware.GetRequestID(c)},
		)

		list, err := h.categoryService.ListCategories(c.Request.Context())
		if err != nil {
			response.WriteInternalServerError(c)
			log.Error("internal server error", logger.Err(err))
			return
		}

		c.JSON(http.StatusOK, toCategoryListResponse(list))
	}
}

// CreateCategory godoc
// @Summary      Create category
// @Description  Creates a new category. ADMIN only.
// @Tags         Categories
// @Security     bearerAuth
// @Accept       json
// @Produce      json
// @Param        request  body      CreateCategoryRequest  true  "Category data"
// @Success      201  {object}  CategoryResponse
// @Failure      400  {object}  response.ErrorResponse
// @Failure      401  {object}  response.ErrorResponse
// @Failure      403  {object}  response.ErrorResponse
// @Failure      409  {object}  response.ErrorResponse
// @Failure      500  {object}  response.ErrorResponse
// @Router       /categories [post]
func (h *Handler) CreateCategory() gin.HandlerFunc {
	return func(c *gin.Context) {
		const op = "handler.category.CreateCategory"

		log := h.log.With(
			logger.Field{Key: "op", Value: op},
			logger.Field{Key: "request_id", Value: middleware.GetRequestID(c)},
		)

		var req CreateCategoryRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.WriteBindError(c, err)
			log.Warn("invalid request body", logger.Err(err))
			return
		}

		category, err := h.categoryService.CreateCategory(c.Request.Context(), req.ToInput())
		if err != nil {
			switch {
			case errors.Is(err, service.ErrForbidden):
				response.WriteError(c, http.StatusForbidden, "forbidden")
				log.Warn("forbidden", logger.Err(err))
				return
			case errors.Is(err, service.ErrCategoryAlreadyExists):
				response.WriteError(c, http.StatusConflict, "category already exists")
				log.Warn("category already exists", logger.Err(err))
				return
			default:
				response.WriteInternalServerError(c)
				log.Error("internal server error", logger.Err(err))
				return
			}
		}

		c.JSON(http.StatusCreated, toCategoryResponse(category))
	}
}

// UpdateCategory godoc
// @Summary      Update category
// @Description  Updates a category name. ADMIN only.
// @Tags         Categories
// @Security     bearerAuth
// @Accept       json
// @Produce      json
// @Param        id       path      string                true  "Category ID (UUID)"
// @Param        request  body      UpdateCategoryRequest  true  "New name"
// @Success      200  {object}  CategoryResponse
// @Failure      400  {object}  response.ErrorResponse
// @Failure      401  {object}  response.ErrorResponse
// @Failure      403  {object}  response.ErrorResponse
// @Failure      404  {object}  response.ErrorResponse
// @Failure      409  {object}  response.ErrorResponse
// @Failure      500  {object}  response.ErrorResponse
// @Router       /categories/{id} [patch]
func (h *Handler) UpdateCategory() gin.HandlerFunc {
	return func(c *gin.Context) {
		const op = "handler.category.UpdateCategory"

		log := h.log.With(
			logger.Field{Key: "op", Value: op},
			logger.Field{Key: "request_id", Value: middleware.GetRequestID(c)},
		)

		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			response.WriteError(c, http.StatusBadRequest, "invalid category id")
			log.Warn("invalid category id", logger.Err(err))
			return
		}

		var req UpdateCategoryRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.WriteBindError(c, err)
			log.Warn("invalid request body", logger.Err(err))
			return
		}

		category, err := h.categoryService.UpdateCategory(c.Request.Context(), id, req.ToInput())
		if err != nil {
			switch {
			case errors.Is(err, service.ErrForbidden):
				response.WriteError(c, http.StatusForbidden, "forbidden")
				log.Warn("forbidden", logger.Err(err))
				return
			case errors.Is(err, service.ErrCategoryNotFound):
				response.WriteError(c, http.StatusNotFound, "category not found")
				log.Warn("category not found", logger.Err(err))
				return
			case errors.Is(err, service.ErrCategoryAlreadyExists):
				response.WriteError(c, http.StatusConflict, "category already exists")
				log.Warn("category already exists", logger.Err(err))
				return
			default:
				response.WriteInternalServerError(c)
				log.Error("internal server error", logger.Err(err))
				return
			}
		}

		c.JSON(http.StatusOK, toCategoryResponse(category))
	}
}

// DeleteCategory godoc
// @Summary      Delete category
// @Description  Deletes a category. ADMIN only. Returns 409 if category is in use by products.
// @Tags         Categories
// @Security     bearerAuth
// @Param        id  path  string  true  "Category ID (UUID)"
// @Success      204
// @Failure      400  {object}  response.ErrorResponse
// @Failure      401  {object}  response.ErrorResponse
// @Failure      403  {object}  response.ErrorResponse
// @Failure      404  {object}  response.ErrorResponse
// @Failure      409  {object}  response.ErrorResponse
// @Failure      500  {object}  response.ErrorResponse
// @Router       /categories/{id} [delete]
func (h *Handler) DeleteCategory() gin.HandlerFunc {
	return func(c *gin.Context) {
		const op = "handler.category.DeleteCategory"

		log := h.log.With(
			logger.Field{Key: "op", Value: op},
			logger.Field{Key: "request_id", Value: middleware.GetRequestID(c)},
		)

		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			response.WriteError(c, http.StatusBadRequest, "invalid category id")
			log.Warn("invalid category id", logger.Err(err))
			return
		}

		if err := h.categoryService.DeleteCategory(c.Request.Context(), id); err != nil {
			switch {
			case errors.Is(err, service.ErrForbidden):
				response.WriteError(c, http.StatusForbidden, "forbidden")
				log.Warn("forbidden", logger.Err(err))
				return
			case errors.Is(err, service.ErrCategoryNotFound):
				response.WriteError(c, http.StatusNotFound, "category not found")
				log.Warn("category not found", logger.Err(err))
				return
			case errors.Is(err, service.ErrCategoryInUse):
				response.WriteError(c, http.StatusConflict, "category is in use by products")
				log.Warn("category in use", logger.Err(err))
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

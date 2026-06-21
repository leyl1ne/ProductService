package batch

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/leyl1ne/ProductService/internal/service"
	batchservice "github.com/leyl1ne/ProductService/internal/service/batch"
	"github.com/leyl1ne/ProductService/internal/transport/http/middleware"
	"github.com/leyl1ne/ProductService/internal/transport/http/response"
	"github.com/leyl1ne/ProductService/pkg/logger"
)

type BatchService interface {
	CreateBatch(ctx context.Context, productID uuid.UUID, input batchservice.CreateBatchInput) (*batchservice.BatchOutput, error)
	GetBatchByID(ctx context.Context, id uuid.UUID) (*batchservice.BatchOutput, error)
	ListBatchesByProduct(ctx context.Context, productID uuid.UUID) ([]batchservice.BatchOutput, error)
	UpdateBatch(ctx context.Context, id uuid.UUID, input batchservice.UpdateBatchInput) (*batchservice.BatchOutput, error)
	DeleteBatch(ctx context.Context, id uuid.UUID) error
}

type Handler struct {
	log          logger.Logger
	batchService BatchService
}

func NewHandler(log logger.Logger, batchService BatchService) *Handler {
	return &Handler{
		log:          log,
		batchService: batchService,
	}
}

// CreateBatch godoc
// @Summary      Create batch
// @Description  Creates a new batch for a product. SELLER or ADMIN (owner of product's company) only.
// @Tags         Batches
// @Security     bearerAuth
// @Accept       json
// @Produce      json
// @Param        id       path      string              true  "Product ID (UUID)"
// @Param        request  body      CreateBatchRequest  true  "Batch data"
// @Success      201  {object}  BatchResponse
// @Failure      400  {object}  response.ErrorResponse
// @Failure      401  {object}  response.ErrorResponse
// @Failure      403  {object}  response.ErrorResponse
// @Failure      404  {object}  response.ErrorResponse
// @Failure      500  {object}  response.ErrorResponse
// @Router       /products/{id}/batches [post]
func (h *Handler) CreateBatch() gin.HandlerFunc {
	return func(c *gin.Context) {
		const op = "handler.batch.CreateBatch"

		log := h.log.With(
			logger.Field{Key: "op", Value: op},
			logger.Field{Key: "request_id", Value: middleware.GetRequestID(c)},
		)

		productID, err := uuid.Parse(c.Param("id"))
		if err != nil {
			response.WriteError(c, http.StatusBadRequest, "invalid product id")
			log.Warn("invalid product id", logger.Err(err))
			return
		}

		var req CreateBatchRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.WriteBindError(c, err)
			log.Warn("invalid request body", logger.Err(err))
			return
		}

		input, err := req.ToInput()
		if err != nil {
			response.WriteError(c, http.StatusBadRequest, "invalid date format or uuid")
			log.Warn("invalid input", logger.Err(err))
			return
		}

		batch, err := h.batchService.CreateBatch(c.Request.Context(), productID, input)
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
			default:
				response.WriteInternalServerError(c)
				log.Error("internal server error", logger.Err(err))
				return
			}
		}

		c.JSON(http.StatusCreated, toBatchResponse(batch))
	}
}

// ListBatchesByProduct godoc
// @Summary      Get product batches
// @Description  Returns all batches for a product. Owner of product's company or ADMIN only.
// @Tags         Batches
// @Security     bearerAuth
// @Produce      json
// @Param        id  path  string  true  "Product ID (UUID)"
// @Success      200  {object}  BatchListResponse
// @Failure      400  {object}  response.ErrorResponse
// @Failure      401  {object}  response.ErrorResponse
// @Failure      403  {object}  response.ErrorResponse
// @Failure      404  {object}  response.ErrorResponse
// @Failure      500  {object}  response.ErrorResponse
// @Router       /products/{id}/batches [get]
func (h *Handler) ListBatchesByProduct() gin.HandlerFunc {
	return func(c *gin.Context) {
		const op = "handler.batch.ListBatchesByProduct"

		log := h.log.With(
			logger.Field{Key: "op", Value: op},
			logger.Field{Key: "request_id", Value: middleware.GetRequestID(c)},
		)

		productID, err := uuid.Parse(c.Param("id"))
		if err != nil {
			response.WriteError(c, http.StatusBadRequest, "invalid product id")
			log.Warn("invalid product id", logger.Err(err))
			return
		}

		list, err := h.batchService.ListBatchesByProduct(c.Request.Context(), productID)
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
			default:
				response.WriteInternalServerError(c)
				log.Error("internal server error", logger.Err(err))
				return
			}
		}

		c.JSON(http.StatusOK, toBatchListResponse(list))
	}
}

// GetBatch godoc
// @Summary      Get batch by id
// @Description  Returns a batch by ID. Owner of product's company or ADMIN only.
// @Tags         Batches
// @Security     bearerAuth
// @Produce      json
// @Param        id  path  string  true  "Batch ID (UUID)"
// @Success      200  {object}  BatchResponse
// @Failure      400  {object}  response.ErrorResponse
// @Failure      401  {object}  response.ErrorResponse
// @Failure      403  {object}  response.ErrorResponse
// @Failure      404  {object}  response.ErrorResponse
// @Failure      500  {object}  response.ErrorResponse
// @Router       /batches/{id} [get]
func (h *Handler) GetBatch() gin.HandlerFunc {
	return func(c *gin.Context) {
		const op = "handler.batch.GetBatch"

		log := h.log.With(
			logger.Field{Key: "op", Value: op},
			logger.Field{Key: "request_id", Value: middleware.GetRequestID(c)},
		)

		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			response.WriteError(c, http.StatusBadRequest, "invalid batch id")
			log.Warn("invalid batch id", logger.Err(err))
			return
		}

		batch, err := h.batchService.GetBatchByID(c.Request.Context(), id)
		if err != nil {
			switch {
			case errors.Is(err, service.ErrForbidden):
				response.WriteError(c, http.StatusForbidden, "forbidden")
				log.Warn("forbidden", logger.Err(err))
				return
			case errors.Is(err, service.ErrBatchNotFound):
				response.WriteError(c, http.StatusNotFound, "batch not found")
				log.Warn("batch not found", logger.Err(err))
				return
			default:
				response.WriteInternalServerError(c)
				log.Error("internal server error", logger.Err(err))
				return
			}
		}

		c.JSON(http.StatusOK, toBatchResponse(batch))
	}
}

// UpdateBatch godoc
// @Summary      Update batch
// @Description  Partially updates a batch. Owner of product's company or ADMIN only.
// @Tags         Batches
// @Security     bearerAuth
// @Accept       json
// @Produce      json
// @Param        id       path      string              true  "Batch ID (UUID)"
// @Param        request  body      UpdateBatchRequest  true  "Fields to update"
// @Success      200  {object}  BatchResponse
// @Failure      400  {object}  response.ErrorResponse
// @Failure      401  {object}  response.ErrorResponse
// @Failure      403  {object}  response.ErrorResponse
// @Failure      404  {object}  response.ErrorResponse
// @Failure      500  {object}  response.ErrorResponse
// @Router       /batches/{id} [patch]
func (h *Handler) UpdateBatch() gin.HandlerFunc {
	return func(c *gin.Context) {
		const op = "handler.batch.UpdateBatch"

		log := h.log.With(
			logger.Field{Key: "op", Value: op},
			logger.Field{Key: "request_id", Value: middleware.GetRequestID(c)},
		)

		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			response.WriteError(c, http.StatusBadRequest, "invalid batch id")
			log.Warn("invalid batch id", logger.Err(err))
			return
		}

		var req UpdateBatchRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.WriteBindError(c, err)
			log.Warn("invalid request body", logger.Err(err))
			return
		}

		input, err := req.ToInput()
		if err != nil {
			response.WriteError(c, http.StatusBadRequest, "invalid date format")
			log.Warn("invalid date format", logger.Err(err))
			return
		}

		batch, err := h.batchService.UpdateBatch(c.Request.Context(), id, input)
		if err != nil {
			switch {
			case errors.Is(err, service.ErrForbidden):
				response.WriteError(c, http.StatusForbidden, "forbidden")
				log.Warn("forbidden", logger.Err(err))
				return
			case errors.Is(err, service.ErrBatchNotFound):
				response.WriteError(c, http.StatusNotFound, "batch not found")
				log.Warn("batch not found", logger.Err(err))
				return
			default:
				response.WriteInternalServerError(c)
				log.Error("internal server error", logger.Err(err))
				return
			}
		}

		c.JSON(http.StatusOK, toBatchResponse(batch))
	}
}

// DeleteBatch godoc
// @Summary      Delete batch
// @Description  Deletes a batch. Owner of product's company or ADMIN only.
// @Tags         Batches
// @Security     bearerAuth
// @Param        id  path  string  true  "Batch ID (UUID)"
// @Success      204
// @Failure      400  {object}  response.ErrorResponse
// @Failure      401  {object}  response.ErrorResponse
// @Failure      403  {object}  response.ErrorResponse
// @Failure      404  {object}  response.ErrorResponse
// @Failure      500  {object}  response.ErrorResponse
// @Router       /batches/{id} [delete]
func (h *Handler) DeleteBatch() gin.HandlerFunc {
	return func(c *gin.Context) {
		const op = "handler.batch.DeleteBatch"

		log := h.log.With(
			logger.Field{Key: "op", Value: op},
			logger.Field{Key: "request_id", Value: middleware.GetRequestID(c)},
		)

		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			response.WriteError(c, http.StatusBadRequest, "invalid batch id")
			log.Warn("invalid batch id", logger.Err(err))
			return
		}

		if err := h.batchService.DeleteBatch(c.Request.Context(), id); err != nil {
			switch {
			case errors.Is(err, service.ErrForbidden):
				response.WriteError(c, http.StatusForbidden, "forbidden")
				log.Warn("forbidden", logger.Err(err))
				return
			case errors.Is(err, service.ErrBatchNotFound):
				response.WriteError(c, http.StatusNotFound, "batch not found")
				log.Warn("batch not found", logger.Err(err))
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

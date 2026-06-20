package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/leyl1ne/ProductService/internal/model/auth"
	"github.com/leyl1ne/ProductService/internal/transport/http/response"
	"github.com/leyl1ne/ProductService/pkg/logger"
)

const (
	HeaderUserID    = "X-User-ID"
	HeaderUserRole  = "X-User-Role"
	HeaderCompanyID = "X-Company-ID"
	HeaderRequestID = "X-Request-ID"
)

func ExtractHeadersMiddleware(log logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		log := log.With(logger.Field{Key: "component", Value: "middleware/extract-headers"})

		requestID := c.GetHeader(HeaderRequestID)
		SetRequestID(c, requestID)

		userID, err := uuid.Parse(c.GetHeader(HeaderUserID))
		if err != nil {
			log.Error("ivalid user id in header", logger.Err(err))
			response.WriteErrorAbort(c, http.StatusBadRequest, "invalid user id")
			return
		}
		companyUUID, _ := uuid.Parse(c.GetHeader(HeaderCompanyID))
		userRole := auth.Role(c.GetHeader(HeaderUserRole))

		user := auth.User{
			ID:        userID,
			Role:      userRole,
			CompanyID: companyUUID,
		}

		ctx := auth.ContextWithUser(c.Request.Context(), user)
		c.Request = c.Request.WithContext(ctx)

		log.Debug("extracted user context from gateway headers",
			logger.Field{Key: "user_id", Value: user.ID},
			logger.Field{Key: "role", Value: user.Role},
			logger.Field{Key: "request_id", Value: requestID},
		)

		c.Next()
	}
}

package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
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

		userID := c.GetHeader(HeaderUserID)
		if userID == "" {
			// Если нет X-User-ID, значит запрос не прошёл через gateway
			// или gateway не проставил заголовок — это ошибка конфигурации
			log.Warn("missing X-User-ID header, request may bypass gateway")
			response.WriteErrorAbort(c, http.StatusBadRequest, "missing required gateway header")
			return
		}

		user := UserContext{
			UserID:    userID,
			UserRole:  c.GetHeader(HeaderUserRole),
			CompanyID: c.GetHeader(HeaderCompanyID),
		}

		SetUser(c, user)

		log.Debug("extracted user context from gateway headers",
			logger.Field{Key: "user_id", Value: user.UserID},
			logger.Field{Key: "role", Value: user.UserRole},
			logger.Field{Key: "request_id", Value: requestID},
		)

		c.Next()
	}
}

package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/leyl1ne/ProductService/internal/transport/http/response"
	"github.com/leyl1ne/ProductService/pkg/httpserver"
	"github.com/leyl1ne/ProductService/pkg/logger"
)

func ServiceBasicAuthMiddleware(log logger.Logger, basicAuth httpserver.BasicAuth) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !basicAuth.Enabled {
			c.Next()
			return
		}

		user, pass, ok := c.Request.BasicAuth()
		if !ok || user != basicAuth.Username || pass != basicAuth.Password {
			response.WriteErrorAbort(c, http.StatusForbidden, "not forbidden")
			return
		}
		c.Next()
	}
}

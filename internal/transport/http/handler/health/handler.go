package health

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type HealthHandler struct {
	appName    string
	appVersion string
	appEnv     string
}

func NewHealthHandler(appName, appVersion, appEnv string) *HealthHandler {
	return &HealthHandler{
		appName:    appName,
		appVersion: appVersion,
		appEnv:     appEnv,
	}
}

func (h *HealthHandler) Health() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":      "ok",
			"service":     h.appName,
			"version":     h.appVersion,
			"environment": h.appEnv,
		})
	}
}

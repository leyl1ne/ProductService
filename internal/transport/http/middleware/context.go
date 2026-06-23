package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type contextKey string

const (
	RequestIDContextKey contextKey = "request_id"
)

func SetRequestID(c *gin.Context, requestID string) {
	if requestID == "" {
		requestID = uuid.NewString()
	}

	c.Set(RequestIDContextKey, requestID)
}

func GetRequestID(c *gin.Context) string {
	val, exists := c.Get(RequestIDContextKey)
	if !exists {
		return ""
	}

	id, ok := val.(string)
	if !ok {
		return ""
	}
	return id
}

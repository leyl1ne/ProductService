package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type contextKey string

type UserContext struct {
	UserID    string
	UserRole  string
	CompanyID string
}

const (
	UserContextKey      contextKey = "user"
	RequestIDContextKey contextKey = "request_id"
)

func GetUser(c *gin.Context) (UserContext, bool) {
	userContext, exists := c.Get(UserContextKey)
	if !exists {
		return UserContext{}, false
	}
	user, ok := userContext.(UserContext)
	if !ok {
		return UserContext{}, false
	}

	return user, true

}

func SetUser(c *gin.Context, user UserContext) {
	c.Set(UserContextKey, user)
}

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

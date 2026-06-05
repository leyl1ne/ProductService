package response

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

type Error struct {
	Message string `json:"message"`
}

type ErrorResponse struct {
	Error Error `json:"error"`
}

func WriteError(c *gin.Context, status int, msg string) {
	c.JSON(status, ErrorResponse{
		Error: Error{
			Message: msg,
		},
	})
}

func WriteErrorAbort(c *gin.Context, status int, msg string) {
	c.AbortWithStatusJSON(status, ErrorResponse{
		Error: Error{
			Message: msg,
		},
	})
}

func WriteInternalServerError(c *gin.Context) {
	WriteError(c, http.StatusInternalServerError, "internal server error")
}

// func WriteServiceValidationError(c *gin.Context, ve service.ValidationError) {
// 	ValidationError(c, ve.Fields)
// }

func WriteBindError(c *gin.Context, err error) {
	var ve validator.ValidationErrors

	switch {
	case errors.Is(err, io.EOF):
		WriteError(c, http.StatusBadRequest, "request body is empty")
		return
	case errors.As(err, &ve):
		ValidationError(c, formatValidationErrors(ve))
		return
	default:
		WriteError(c, http.StatusBadRequest, "invalid request body")
		return
	}
}

func ValidationError(c *gin.Context, validaitonErr map[string]string) {
	c.JSON(http.StatusUnprocessableEntity, gin.H{
		"error": Error{
			Message: "validation failed",
		},
		"fields": validaitonErr,
	})
}

func formatValidationErrors(ve validator.ValidationErrors) map[string]string {
	errorsMap := make(map[string]string, len(ve))

	for _, fe := range ve {
		field := strings.ToLower(fe.Field())

		switch fe.Tag() {
		case "required":
			errorsMap[field] = "field is required"
		case "min":
			errorsMap[field] = "too short"
		case "max":
			errorsMap[field] = "too long"
		case "email":
			errorsMap[field] = "invalid email format"
		default:
			errorsMap[field] = "invalid value"
		}
	}

	return errorsMap
}

package httpx

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

type ErrorResponse struct {
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

func Error(c *gin.Context, status int, message string, details map[string]string) {
	c.JSON(status, ErrorResponse{
		Message: message,
		Details: details,
	})
}

func BadRequest(c *gin.Context, message string, details map[string]string) {
	Error(c, http.StatusBadRequest, message, details)
}

func Unauthorized(c *gin.Context, message string) {
	Error(c, http.StatusUnauthorized, message, nil)
}

func Conflict(c *gin.Context, message string) {
	Error(c, http.StatusConflict, message, nil)
}

func Internal(c *gin.Context) {
	Error(c, http.StatusInternalServerError, "internal server error", nil)
}

func Unprocessable(c *gin.Context, message string, details map[string]string) {
	Error(c, http.StatusUnprocessableEntity, message, details)
}

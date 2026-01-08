package httpx

import (
	"github.com/gin-gonic/gin"
)

type ErrorResponse struct {
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

func BadRequest(c *gin.Context, message string, details map[string]string) {
	c.JSON(400, ErrorResponse{
		Message: message,
		Details: details,
	})
}

func Unauthorized(c *gin.Context, message string) {
	c.JSON(401, ErrorResponse{
		Message: message,
	})
}

func Conflict(c *gin.Context, message string) {
	c.JSON(409, ErrorResponse{
		Message: message,
	})
}

func Internal(c *gin.Context) {
	c.JSON(500, ErrorResponse{
		Message: "internal server error",
	})
}

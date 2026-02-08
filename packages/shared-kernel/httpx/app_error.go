package httpx

import "github.com/gin-gonic/gin"

func WriteAppError(c *gin.Context, err error) {
	WriteError(c, err)
}

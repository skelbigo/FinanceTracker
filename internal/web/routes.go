package web

import "github.com/gin-gonic/gin"

type Handlers struct {
	R *Renderer
}

func RegisterRoutes(router *gin.Engine, h *Handlers) {
	router.GET("/login", h.GetLogin)
	router.POST("/login", h.PostLogin)

	router.GET("/register", h.GetRegister)
	router.POST("/register", h.PostRegister)

	router.POST("/logout", h.PostLogout)

	router.GET("/app", h.GetDashboard)
}

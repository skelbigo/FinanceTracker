package auth

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(r gin.IRouter) {
	g := r.Group("/auth")
	g.POST("/register", h.register)
}

func (h *Handler) register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid json"})
		return
	}

	resp, err := h.svc.Register(c.Request.Context(), req)
	if err != nil {
		if err == ErrEmailTaken {
			c.JSON(http.StatusConflict, gin.H{"massage": "email already exists"})
			return
		}

		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, resp)
}

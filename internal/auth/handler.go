package auth

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
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
	g.POST("/login", h.login)
	g.POST("/refresh", h.refresh)
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
			c.JSON(http.StatusConflict, gin.H{"message": "email already exists"})
			return
		}

		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, resp)
}

func (h *Handler) login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid json"})
		return
	}

	resp, err := h.svc.Login(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, ErrInvalidCredentials) {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "invalid credentials"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) refresh(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid json"})
		return
	}

	resp, err := h.svc.Refresh(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, ErrInvalidRefreshToken) {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "invalid refresh token"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

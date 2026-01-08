package auth

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/skelbigo/FinanceTracker/internal/httpx"
	"log"
	"net/http"
)

type Handler struct {
	svc *Service
	mw  gin.HandlerFunc
}

func NewHandler(svc *Service, authMV gin.HandlerFunc) *Handler {
	return &Handler{svc: svc, mw: authMV}
}

func (h *Handler) RegisterRoutes(r gin.IRouter) {
	g := r.Group("/auth")
	g.POST("/register", h.register)
	g.POST("/login", h.login)
	g.POST("/refresh", h.refresh)
	g.POST("/logout", h.logout)
	g.GET("/me", h.mw, h.me)
}

func (h *Handler) register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "invalid json", nil)
		return
	}

	resp, err := h.svc.Register(c.Request.Context(), req)
	if err != nil {
		if err == ErrEmailTaken {
			httpx.Conflict(c, "email already exists")
			return
		}

		log.Println("context:", err)
		httpx.Internal(c)
		return
	}

	c.JSON(http.StatusCreated, resp)
}

func (h *Handler) login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "invalid json", nil)
		return
	}

	resp, err := h.svc.Login(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, ErrInvalidCredentials) {
			httpx.Unauthorized(c, "invalid credentials")
			return
		}
		log.Println("context:", err)
		httpx.Internal(c)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) refresh(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "invalid json", nil)
		return
	}

	resp, err := h.svc.Refresh(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, ErrInvalidRefreshToken) {
			httpx.Unauthorized(c, "invalid refresh token")
			return
		}
		log.Println("context:", err)
		httpx.Internal(c)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) logout(c *gin.Context) {
	var req LogoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "invalid json", nil)
		return
	}

	if err := h.svc.Logout(c.Request.Context(), req); err != nil {
		log.Println("context:", err)
		httpx.Internal(c)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *Handler) me(c *gin.Context) {
	v, ok := c.Get(CtxUserIDKey)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "invalid token"})
		return
	}
	userID, ok := v.(string)
	if !ok || userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "invalid token"})
		return
	}

	dto, err := h.svc.Me(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "invalid token"})
			return
		}
		log.Println("context:", err)
		httpx.Internal(c)
		return
	}

	c.JSON(http.StatusOK, gin.H{"user": dto})
}

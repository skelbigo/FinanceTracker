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

func NewHandler(svc *Service, authMW gin.HandlerFunc) *Handler {
	return &Handler{svc: svc, mw: authMW}
}

func (h *Handler) RegisterRoutes(r gin.IRouter) {
	g := r.Group("/auth")
	g.POST("/register", h.register)
	g.POST("/login", h.login)
	g.POST("/refresh", h.refresh)
	g.POST("/logout", h.logout)
	g.GET("/me", h.mw, h.me)
}

func bindJSON[T any](c *gin.Context, dst *T) bool {
	if err := c.ShouldBindJSON(dst); err != nil {
		httpx.BadRequest(c, "invalid json", nil)
		return false
	}
	return true
}

func (h *Handler) register(c *gin.Context) {
	var req RegisterRequest
	if !bindJSON(c, &req) {
		return
	}

	resp, err := h.svc.Register(c.Request.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, ErrEmailTaken):
			httpx.BadRequest(c, "email already exists", nil)
		default:
			log.Printf("auth.register: %v", err)
			httpx.Internal(c)
		}
		return
	}

	c.JSON(http.StatusCreated, resp)
}

func (h *Handler) login(c *gin.Context) {
	var req LoginRequest
	if !bindJSON(c, &req) {
		return
	}

	resp, err := h.svc.Login(c.Request.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows), errors.Is(err, ErrInvalidCredentials):
			httpx.Unauthorized(c, "invalid credentials")
		default:
			log.Printf("auth.login: %v", err)
			httpx.Internal(c)
		}
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) refresh(c *gin.Context) {
	var req RefreshRequest
	if !bindJSON(c, &req) {
		return
	}

	resp, err := h.svc.Refresh(c.Request.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows), errors.Is(err, ErrInvalidRefreshToken):
			httpx.Unauthorized(c, "invalid refresh token")
		default:
			log.Printf("auth.refresh: %v", err)
			httpx.Internal(c)
		}
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) logout(c *gin.Context) {
	var req LogoutRequest
	if !bindJSON(c, &req) {
		return
	}

	if err := h.svc.Logout(c.Request.Context(), req); err != nil {
		log.Printf("auth.logout: %v", err)
		httpx.Internal(c)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *Handler) me(c *gin.Context) {
	v, ok := c.Get(CtxUserIDKey)
	userID, ok := v.(string)
	if !ok || userID == "" {
		httpx.Unauthorized(c, "invalid token")
		return
	}

	dto, err := h.svc.Me(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.Unauthorized(c, "invalid token")
			return
		}
		log.Printf("auth.me: %v", err)
		httpx.Internal(c)
		return
	}

	c.JSON(http.StatusOK, gin.H{"user": dto})
}

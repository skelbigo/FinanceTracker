package workspaces

import (
	"github.com/gin-gonic/gin"
	"github.com/skelbigo/FinanceTracker/internal/auth"
	"github.com/skelbigo/FinanceTracker/internal/httpx"
	"net/http"
	"strings"
)

type Handler struct {
	repo *Repo
}

func NewHandler(repo *Repo) *Handler {
	return &Handler{repo: repo}
}

type createWorkspaceReq struct {
	Name            string `json:"name" binding:"required"`
	DefaultCurrency string `json:"default_currency"`
}

func (h *Handler) CreateWorkspace(c *gin.Context) {
	v, ok := c.Get(auth.CtxUserIDKey)
	creatorID, ok := v.(string)
	if !ok || creatorID == "" {
		httpx.Unauthorized(c, "invalid token")
		return
	}

	var req createWorkspaceReq
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "invalid json", nil)
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.DefaultCurrency = strings.TrimSpace(req.DefaultCurrency)

	w, err := h.repo.CreateWorkspaceWithOwner(c.Request.Context(), creatorID, req.Name, req.DefaultCurrency)
	if err != nil {
		c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, w)
}

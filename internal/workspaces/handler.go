package workspaces

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
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
func (h *Handler) GetWorkspace(c *gin.Context) {
	workspaceID := c.Param("id")

	w, err := h.repo.GetWorkspace(c.Request.Context(), workspaceID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.Error(c, http.StatusNotFound, "workspace not found", nil)
			return
		}
		httpx.Internal(c)
		return
	}

	c.JSON(http.StatusOK, w)
}

func (h *Handler) ListMembers(c *gin.Context) {
	workspaceID := c.Param("id")

	members, err := h.repo.ListMembers(c.Request.Context(), workspaceID)
	if err != nil {
		httpx.Internal(c)
		return
	}

	c.JSON(http.StatusOK, gin.H{"members": members})
}

type addMemberReq struct {
	UserID string `json:"user_id" binding:"required"`
	Role   string `json:"role" binding:"required"`
}

func (h *Handler) AddMember(c *gin.Context) {
	workspaceId := c.Param("id")

	var req addMemberReq
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "invalid json", nil)
		return
	}

	req.UserID = strings.TrimSpace(req.UserID)
	req.Role = strings.TrimSpace(req.Role)

	role := Role(req.Role)
	if role != RoleOwner && role != RoleMember && role != RoleViewer {
		httpx.Unprocessable(c, "invalid role", map[string]string{"role": "role must be one of owner | member | viewer"})
		return
	}

	if err := h.repo.AddMember(c.Request.Context(), workspaceId, req.UserID, role); err != nil {
		httpx.Error(c, http.StatusInternalServerError, "failed to add member", map[string]string{"error": err.Error()})
		return
	}

	c.Status(http.StatusCreated)
}

type updateMemberRoleReq struct {
	Role string `json:"role" binding:"required"`
}

func (h *Handler) UpdateMemberRole(c *gin.Context) {
	workspaceID := c.Param("id")
	userID := c.Param("user_id")

	var req updateMemberRoleReq
	if err := c.ShouldBind(&req); err != nil {
		httpx.BadRequest(c, "invalid json", nil)
		return
	}

	role := Role(strings.TrimSpace(req.Role))
	if role != RoleOwner && role != RoleMember && role != RoleViewer {
		httpx.Unprocessable(c, "invalid role", map[string]string{"role": "role must be one of owner | member | viewer"})
		return
	}

	if err := h.repo.UpdateMemberRole(c.Request.Context(), workspaceID, userID, role); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.Error(c, http.StatusNotFound, "member not found", nil)
			return
		}
		httpx.Internal(c)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *Handler) RemoveMember(c *gin.Context) {
	workspaceID := c.Param("id")
	userID := c.Param("user_id")

	if err := h.repo.RemoveMember(c.Request.Context(), workspaceID, userID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.Error(c, http.StatusNotFound, "member not found", nil)
			return
		}
		httpx.Internal(c)
		return
	}

	c.Status(http.StatusNoContent)
}

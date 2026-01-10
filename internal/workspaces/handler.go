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
	c.JSON(http.StatusCreated, gin.H{
		"workspace": w,
		"role":      RoleOwner,
	})
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
	Email string `json:"email" binding:"required, email"`
	Role  string `json:"role" binding:"required"`
}

func (h *Handler) AddMember(c *gin.Context) {
	workspaceId := c.Param("id")

	var req addMemberReq
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "invalid json", nil)
		return
	}

	role := Role(strings.TrimSpace(req.Role))
	if role != RoleOwner && role != RoleMember && role != RoleViewer {
		httpx.Unprocessable(c, "invalid role", map[string]string{"role": "role must be one of owner | member | viewer"})
		return
	}

	userID, err := h.repo.FindUserIDByEmail(c.Request.Context(), req.Email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.Error(c, http.StatusInternalServerError, "user not found", map[string]string{"error": err.Error()})
			return
		}
		httpx.Internal(c)
		return
	}

	if err := h.repo.AddMemberByUserID(c.Request.Context(), workspaceId, userID, role); err != nil {
		if errors.Is(err, ErrAlreadyMember) {
			httpx.Conflict(c, "user already in conflict")
			return
		}
		httpx.Internal(c)
		return
	}

	c.Status(http.StatusCreated)
}

type updateMemberRoleReq struct {
	Role string `json:"role" binding:"required"`
}

func (h *Handler) UpdateMemberRole(c *gin.Context) {
	v, ok := c.Get(auth.CtxUserIDKey)
	actorID, ok := v.(string)
	if !ok || actorID == "" {
		httpx.Unauthorized(c, "invalid token")
		return
	}

	workspaceID := c.Param("id")
	targetUserID := c.Param("userId")

	var req updateMemberRoleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "invalid json", nil)
		return
	}

	role := Role(strings.TrimSpace(req.Role))
	if role != RoleOwner && role != RoleMember && role != RoleViewer {
		httpx.Unprocessable(c, "invalid role", map[string]string{"role": "role must be one of owner | member | viewer"})
		return
	}

	err := h.repo.UpdateMemberRoleSafe(c.Request.Context(), workspaceID, actorID, targetUserID, role)
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			httpx.Error(c, http.StatusNotFound, "user not found", nil)
		case errors.Is(err, ErrCannotSelfDemote):
			httpx.Conflict(c, "owner cannot change own role on mvp")
		case errors.Is(err, ErrLastOwner):
			httpx.Conflict(c, "cannot demote last owner")
		default:
			httpx.Internal(c)
		}
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *Handler) RemoveMember(c *gin.Context) {
	workspaceID := c.Param("id")
	userID := c.Param("userId")

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

func (h *Handler) ListMyWorkspaces(c *gin.Context) {
	v, ok := c.Get(auth.CtxUserIDKey)
	userID, ok := v.(string)
	if !ok || userID == "" {
		httpx.Unauthorized(c, "invalid token")
		return
	}

	items, err := h.repo.ListMyWorkspaces(c.Request.Context(), userID)
	if err != nil {
		httpx.Internal(c)
		return
	}

	c.JSON(http.StatusOK, gin.H{"workspaces": items})
}

func (h *Handler) GetWorkspace(c *gin.Context) {
	v, ok := c.Get(auth.CtxUserIDKey)
	userID, ok := v.(string)
	if !ok || userID == "" {
		httpx.Unauthorized(c, "invalid token")
		return
	}

	workspaceID := c.Param("id")

	w, role, err := h.repo.GetWorkspaceWithRole(c.Request.Context(), workspaceID, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.Error(c, http.StatusNotFound, "workspace not found", nil)
			return
		}
		httpx.Internal(c)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"workspace": w,
		"role":      role,
	})
}

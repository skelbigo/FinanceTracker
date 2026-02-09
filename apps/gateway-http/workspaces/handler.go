package workspaces

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/skelbigo/FinanceTracker/packages/shared-kernel/httpx"
	"github.com/skelbigo/FinanceTracker/packages/shared-kernel/identity"
	"net/http"
	"strings"
)

type Handler struct {
	svc  *Service
	repo RoleProvider
}

func NewHandler(svc *Service, repo RoleProvider) *Handler {
	return &Handler{
		svc:  svc,
		repo: repo,
	}
}

func (h *Handler) RegisterRoutes(r gin.IRouter) {
	g := r.Group("/workspaces")

	g.POST("", h.CreateWorkspace)
	g.GET("", h.ListMyWorkspaces)

	wsg := g.Group("/:id")

	wsg.GET("", RequireWorkspaceRole(h.repo, RoleViewer), h.GetWorkspace)
	wsg.GET("/members", RequireWorkspaceRole(h.repo, RoleViewer), h.ListMembers)

	wsg.POST("/members", RequireWorkspaceRole(h.repo, RoleOwner), h.AddMember)
	wsg.PATCH("/members/:userId", RequireWorkspaceRole(h.repo, RoleOwner), h.UpdateMemberRole)
	wsg.DELETE("/members/:userId", RequireWorkspaceRole(h.repo, RoleOwner), h.RemoveMember)
}

type createWorkspaceReq struct {
	Name            string `json:"name" binding:"required"`
	DefaultCurrency string `json:"default_currency"`
}

type addMemberReq struct {
	Email string `json:"email" binding:"required,email"`
	Role  string `json:"role" binding:"required"`
}

type updateMemberRoleReq struct {
	Role string `json:"role" binding:"required"`
}

func userIDFromCtx(c *gin.Context) (string, bool) {
	v, exists := c.Get(identity.CtxUserIDKey)
	id, ok := v.(string)
	return id, exists && ok && id != ""
}

func (h *Handler) CreateWorkspace(c *gin.Context) {
	creatorID, ok := userIDFromCtx(c)
	if !ok {
		httpx.Unauthorized(c, "invalid token")
		return
	}

	var req createWorkspaceReq
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "invalid json", nil)
		return
	}

	w, role, err := h.svc.CreateWorkspace(c.Request.Context(), creatorID, req.Name, req.DefaultCurrency)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidWorkspaceName):
			httpx.Unprocessable(c, "invalid workspace name", map[string]string{"name": "1..64 chars"})
		case errors.Is(err, ErrInvalidCurrency):
			httpx.Unprocessable(c, "invalid currency", map[string]string{"default_currency": "3 uppercase letters (e.g., USD)"})
		default:
			httpx.Internal(c)
		}
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"workspace": w,
		"role":      role,
	})
}

func (h *Handler) ListMyWorkspaces(c *gin.Context) {
	userID, ok := userIDFromCtx(c)
	if !ok {
		httpx.Unauthorized(c, "invalid token")
		return
	}

	items, err := h.svc.ListMyWorkspaces(c.Request.Context(), userID)
	if err != nil {
		httpx.Internal(c)
		return
	}

	c.JSON(http.StatusOK, gin.H{"workspaces": items})
}

func (h *Handler) GetWorkspace(c *gin.Context) {
	userID, ok := userIDFromCtx(c)
	if !ok {
		httpx.Unauthorized(c, "invalid token")
		return
	}

	workspaceID := c.Param("id")
	w, role, err := h.svc.GetWorkspace(c.Request.Context(), workspaceID, userID)
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

func (h *Handler) ListMembers(c *gin.Context) {
	workspaceID := c.Param("id")

	members, err := h.svc.ListMembers(c.Request.Context(), workspaceID)
	if err != nil {
		httpx.Internal(c)
		return
	}

	c.JSON(http.StatusOK, gin.H{"members": members})
}

func (h *Handler) AddMember(c *gin.Context) {
	workspaceID := c.Param("id")

	var req addMemberReq
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "invalid json", nil)
		return
	}

	role := Role(strings.TrimSpace(req.Role))
	err := h.svc.AddMemberByEmail(c.Request.Context(), workspaceID, req.Email, role)
	switch {
	case err == nil:
		c.Status(http.StatusCreated)
	case errors.Is(err, ErrUserNotFound):
		httpx.Error(c, http.StatusNotFound, "user not found", nil)
	case errors.Is(err, ErrAlreadyMember):
		httpx.Conflict(c, "user already a member")
	case errors.Is(err, ErrInvalidRole):
		httpx.Unprocessable(c, "invalid role", map[string]string{"role": "owner|member|viewer"})
	default:
		httpx.Internal(c)
	}
}

func (h *Handler) UpdateMemberRole(c *gin.Context) {
	actorID, ok := userIDFromCtx(c)
	if !ok {
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

	err := h.svc.UpdateMemberRole(c.Request.Context(), workspaceID, actorID, targetUserID, role)
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
	actorID, ok := userIDFromCtx(c)
	if !ok {
		httpx.Unauthorized(c, "invalid token")
		return
	}

	workspaceID := c.Param("id")
	targetUserID := c.Param("userId")

	err := h.svc.RemoveMember(c.Request.Context(), workspaceID, actorID, targetUserID)
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			httpx.Error(c, http.StatusNotFound, "member not found", nil)
		case errors.Is(err, ErrLastOwner):
			httpx.Conflict(c, "cannot remove last owner")
		default:
			httpx.Internal(c)
		}
		return
	}

	c.Status(http.StatusNoContent)
}

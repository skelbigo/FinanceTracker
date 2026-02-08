package categories

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/skelbigo/FinanceTracker/apps/gateway-http/workspaces"
	"github.com/skelbigo/FinanceTracker/packages/shared-kernel/httpx"
	"net/http"
	"strings"
)

type Handler struct {
	svc *Service
	ws  workspaces.RoleProvider
}

func NewHandler(svc *Service, ws workspaces.RoleProvider) *Handler {
	return &Handler{svc: svc, ws: ws}
}

func (h *Handler) RegisterRoutes(r gin.IRouter) {
	g := r.Group("/workspaces")

	wsg := g.Group("/:id")
	wsg.POST("/categories", workspaces.RequireWorkspaceRole(h.ws, workspaces.RoleMember), h.create)
	wsg.GET("/categories", workspaces.RequireWorkspaceRole(h.ws, workspaces.RoleViewer), h.list)
	wsg.PUT("/categories/:categoryId", workspaces.RequireWorkspaceRole(h.ws, workspaces.RoleMember), h.update)
	wsg.PATCH("/categories/:categoryId", workspaces.RequireWorkspaceRole(h.ws, workspaces.RoleMember), h.update)
	wsg.DELETE("/categories/:categoryId", workspaces.RequireWorkspaceRole(h.ws, workspaces.RoleMember), h.del)
}

type CreateCategoryReq struct {
	Name string `json:"name" binding:"required"`
	Type string `json:"type" binding:"required"`
}

type UpdateCategoryReq struct {
	Name string `json:"name" binding:"required"`
	Type string `json:"type" binding:"required"`
}

func (h *Handler) create(c *gin.Context) {
	workspaceID, ok := workspaces.GetWorkspaceID(c)
	if !ok {
		httpx.Internal(c)
		return
	}

	var req CreateCategoryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "invalid json", nil)
		return
	}

	t := Type(strings.TrimSpace(strings.ToLower(req.Type)))
	if t != TypeIncome && t != TypeExpense {
		httpx.Unprocessable(c, "invalid category type", map[string]string{"type": "income|expense"})
		return
	}
	cat, err := h.svc.Create(c.Request.Context(), workspaceID, req.Name, t)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidName):
			httpx.Unprocessable(c, "invalid category name", map[string]string{"name": "1..60 chars"})
		case errors.Is(err, ErrInvalidType):
			httpx.Unprocessable(c, "invalid category type", map[string]string{"type": "income|expense"})
		case errors.Is(err, ErrCategoryExists):
			httpx.Conflict(c, "category already exists")
		default:
			httpx.Internal(c)
		}
		return
	}

	c.JSON(http.StatusCreated, gin.H{"category": cat})
}

func (h *Handler) list(c *gin.Context) {
	workspaceID, ok := workspaces.GetWorkspaceID(c)
	if !ok {
		httpx.Internal(c)
		return
	}

	items, err := h.svc.List(c.Request.Context(), workspaceID)
	if err != nil {
		httpx.Internal(c)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) update(c *gin.Context) {
	workspaceID, ok := workspaces.GetWorkspaceID(c)
	if !ok {
		httpx.Internal(c)
		return
	}
	categoryID := strings.TrimSpace(c.Param("categoryId"))

	var req UpdateCategoryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "invalid json", nil)
		return
	}

	t := Type(strings.TrimSpace(strings.ToLower(req.Type)))
	if t != TypeIncome && t != TypeExpense {
		httpx.Unprocessable(c, "invalid category type", map[string]string{"type": "income|expense"})
		return
	}

	cat, err := h.svc.Update(c.Request.Context(), workspaceID, categoryID, req.Name, t)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidName):
			httpx.Unprocessable(c, "invalid category name", map[string]string{"name": "1..60 chars"})
		case errors.Is(err, ErrInvalidType):
			httpx.Unprocessable(c, "invalid category type", map[string]string{"type": "income|expense"})
		case errors.Is(err, ErrCategoryNotFound):
			httpx.NotFound(c, "category not found")
		case errors.Is(err, ErrCategoryExists):
			httpx.Conflict(c, "category already exists")
		default:
			httpx.Internal(c)
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"category": cat})
}

func (h *Handler) del(c *gin.Context) {
	workspaceID, ok := workspaces.GetWorkspaceID(c)
	if !ok {
		httpx.Internal(c)
		return
	}
	categoryID := strings.TrimSpace(c.Param("categoryId"))

	var reassignTo *string
	if v := strings.TrimSpace(c.Query("reassign_to")); v != "" {
		reassignTo = &v
	}

	err := h.svc.Delete(c.Request.Context(), workspaceID, categoryID, reassignTo)
	if err != nil {
		switch {
		case errors.Is(err, ErrCategoryNotFound):
			httpx.NotFound(c, "category not found")
		case errors.Is(err, ErrCategoryInUse):
			httpx.Conflict(c, "category is in use")
		default:
			httpx.Internal(c)
		}
		return
	}

	c.Status(http.StatusNoContent)
}

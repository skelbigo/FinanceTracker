package categories

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/skelbigo/FinanceTracker/internal/httpx"
	"github.com/skelbigo/FinanceTracker/internal/workspaces"
	"net/http"
	"strings"
)

type Handler struct {
	svc *Service
	mw  gin.HandlerFunc
	ws  workspaces.RoleProvider
}

func NewHandler(svc *Service, authMW gin.HandlerFunc, ws workspaces.RoleProvider) *Handler {
	return &Handler{svc: svc, mw: authMW, ws: ws}
}

func (h *Handler) RegisterRoutes(r gin.IRouter) {
	g := r.Group("/workspaces")
	g.Use(h.mw)

	wsg := g.Group("/:id")
	wsg.POST("/categories", workspaces.RequireWorkspaceRole(h.ws, workspaces.RoleMember), h.create)
	wsg.GET("/categories", workspaces.RequireWorkspaceRole(h.ws, workspaces.RoleViewer), h.list)
}

type CreateCategoryReq struct {
	Name string `json:"name" binding:"required"`
	Type string `json:"type" binding:"required"`
}

func (h *Handler) create(c *gin.Context) {
	workspaceID := c.Param("id")

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
	workspaceID := c.Param("id")

	items, err := h.svc.List(c.Request.Context(), workspaceID)
	if err != nil {
		httpx.Internal(c)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

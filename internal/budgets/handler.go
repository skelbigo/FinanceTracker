package budgets

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/skelbigo/FinanceTracker/internal/httpx"
	"github.com/skelbigo/FinanceTracker/internal/workspaces"
)

type Handler struct {
	svc *Service
	ws  workspaces.RoleProvider
	mw  gin.HandlerFunc
}

func NewHandler(svc *Service, ws workspaces.RoleProvider, mw gin.HandlerFunc) *Handler {
	return &Handler{svc: svc, ws: ws, mw: mw}
}

func (h *Handler) RegisterRoutes(r gin.IRouter) {
	g := r.Group("/workspaces")
	g.Use(h.mw)
	wsg := g.Group("/:id")
	wsg.GET("/budgets",
		workspaces.RequireWorkspaceRole(h.ws, workspaces.RoleViewer),
		h.listBudgetsByMonth,
	)
	wsg.PUT("/budgets",
		workspaces.RequireWorkspaceRole(h.ws, workspaces.RoleMember),
		h.upsertBudget,
	)
}

func (h *Handler) upsertBudget(c *gin.Context) {
	workspaceID, ok := parseWorkspaceUUID(c)
	if !ok {
		return
	}

	var req UpsertBudgetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "invalid request body", map[string]string{"body": "must be valid json"})
		return
	}

	res, err := h.svc.UpsertBudget(c.Request.Context(), workspaceID, req)
	if err != nil {
		respondErr(c, err)
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *Handler) listBudgetsByMonth(c *gin.Context) {
	workspaceID, ok := parseWorkspaceUUID(c)
	if !ok {
		return
	}

	year, month, ok := parseYearMonthQuery(c)
	if !ok {
		return
	}

	items, err := h.svc.GetBudgetsForMonth(c.Request.Context(), workspaceID, year, month)
	if err != nil {
		respondErr(c, err)
		return
	}

	c.JSON(http.StatusOK, items)
}

func respondErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrInvalidYear),
		errors.Is(err, ErrInvalidMonth),
		errors.Is(err, ErrInvalidAmount):
		httpx.Error(c, http.StatusBadRequest, "validation error", map[string]string{
			"details": err.Error(),
		})
		return

	case errors.Is(err, ErrCategoryNotFound):
		httpx.Error(c, http.StatusNotFound, "category not found", nil)
		return

	case errors.Is(err, ErrCategoryNotExpense):
		httpx.Error(c, http.StatusUnprocessableEntity, "category is not expense", nil)
		return

	default:
		httpx.Internal(c)
		return
	}
}

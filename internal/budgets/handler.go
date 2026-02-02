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
		h.listBudgets,
	)
	wsg.GET("/budgets/:budgetId",
		workspaces.RequireWorkspaceRole(h.ws, workspaces.RoleViewer),
		h.getBudget,
	)
	wsg.PUT("/budgets",
		workspaces.RequireWorkspaceRole(h.ws, workspaces.RoleMember),
		h.upsertBudget,
	)
	wsg.PUT("/budgets/:budgetId",
		workspaces.RequireWorkspaceRole(h.ws, workspaces.RoleMember),
		h.updateBudget,
	)
	wsg.DELETE("/budgets/:budgetId",
		workspaces.RequireWorkspaceRole(h.ws, workspaces.RoleMember),
		h.deleteBudget,
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

func (h *Handler) listBudgets(c *gin.Context) {
	workspaceID, ok := parseWorkspaceUUID(c)
	if !ok {
		return
	}

	period, ok := parseOptionalPeriodQuery(c)
	if !ok {
		return
	}

	items, err := h.svc.ListBudgets(c.Request.Context(), workspaceID, period)
	if err != nil {
		respondErr(c, err)
		return
	}

	c.JSON(http.StatusOK, items)
}

func (h *Handler) getBudget(c *gin.Context) {
	workspaceID, ok := parseWorkspaceUUID(c)
	if !ok {
		return
	}
	budgetID, ok := parseBudgetUUID(c)
	if !ok {
		return
	}

	item, err := h.svc.GetByID(c.Request.Context(), workspaceID, budgetID)
	if err != nil {
		respondErr(c, err)
		return
	}

	c.JSON(http.StatusOK, item)
}

func (h *Handler) updateBudget(c *gin.Context) {
	workspaceID, ok := parseWorkspaceUUID(c)
	if !ok {
		return
	}
	budgetID, ok := parseBudgetUUID(c)
	if !ok {
		return
	}

	var req UpsertBudgetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "invalid request body", map[string]string{"body": "must be valid json"})
		return
	}

	res, err := h.svc.Update(c.Request.Context(), workspaceID, budgetID, req)
	if err != nil {
		respondErr(c, err)
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *Handler) deleteBudget(c *gin.Context) {
	workspaceID, ok := parseWorkspaceUUID(c)
	if !ok {
		return
	}
	budgetID, ok := parseBudgetUUID(c)
	if !ok {
		return
	}

	if err := h.svc.Delete(c.Request.Context(), workspaceID, budgetID); err != nil {
		respondErr(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func respondErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrInvalidPeriod),
		errors.Is(err, ErrInvalidLimit),
		errors.Is(err, ErrInvalidCurrency):
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

	case errors.Is(err, ErrBudgetExists):
		httpx.Conflict(c, "budget already exists")
		return

	case errors.Is(err, ErrBudgetNotFound):
		httpx.Error(c, http.StatusNotFound, "budget not found", nil)
		return

	default:
		httpx.Internal(c)
		return
	}
}

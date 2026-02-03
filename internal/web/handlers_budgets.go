package web

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/skelbigo/FinanceTracker/internal/budgets"
	"github.com/skelbigo/FinanceTracker/internal/workspaces"
)

type BudgetRowVM struct {
	ID          string
	Category    string
	Period      string
	Currency    string
	Limit       string
	Spent       string
	Remaining   string
	PercentUsed int
	PercentBar  int
	IsOver      bool
	Status      string
}

func budgetRowFromResponse(b budgets.BudgetResponse, catNames map[string]string) BudgetRowVM {
	cat := b.CategoryID.String()
	if name, ok := catNames[cat]; ok {
		cat = name
	}

	bar := b.PercentUsed
	if bar < 0 {
		bar = 0
	}
	if bar > 100 {
		bar = 100
	}

	status := "OK"
	if b.IsOver {
		status = "Overspent"
	}

	return BudgetRowVM{
		ID:          b.ID.String(),
		Category:    cat,
		Period:      string(b.Period),
		Currency:    b.Currency,
		Limit:       formatMinor(b.AmountLimitMinor),
		Spent:       formatMinor(b.SpentMinor),
		Remaining:   formatMinor(b.RemainingMinor),
		PercentUsed: b.PercentUsed,
		PercentBar:  bar,
		IsOver:      b.IsOver,
		Status:      status,
	}
}

func (h *Handlers) GetBudgetsPage(c *gin.Context) {
	if h.Budgets == nil || h.Categories == nil {
		c.String(http.StatusInternalServerError, "budgets/categories service is not configured")
		return
	}

	wsID := c.GetString(workspaces.CtxWorkspaceIDKey)
	if wsID == "" {
		c.String(http.StatusInternalServerError, "workspace not set")
		return
	}

	wsUUID, err := uuid.Parse(wsID)
	if err != nil {
		c.String(http.StatusBadRequest, "invalid workspace id")
		return
	}

	cats, catNames, err := h.loadCategoriesMap(c.Request.Context(), wsID)
	if err != nil {
		c.String(http.StatusInternalServerError, "could not list categories")
		return
	}

	items, err := h.Budgets.ListWithProgress(c.Request.Context(), wsUUID, nil, time.Now())
	if err != nil {
		c.String(http.StatusInternalServerError, "could not list budgets")
		return
	}

	rows := make([]BudgetRowVM, 0, len(items))
	for _, it := range items {
		rows = append(rows, budgetRowFromResponse(it, catNames))
	}

	h.render(c, "app/budgets.html", gin.H{
		"Title":           "Budgets",
		"BodyClass":       "app-dark app-solid",
		"MainClass":       "tx-main",
		"Workspace":       workspaceFromContext(c),
		"Flash":           c.Query("flash"),
		"Categories":      cats,
		"DefaultCurrency": "UAH",
		"Budgets":         rows,
	})
}

func (h *Handlers) PostCreateBudget(c *gin.Context) {
	wsID := c.GetString(workspaces.CtxWorkspaceIDKey)
	if wsID == "" {
		c.String(http.StatusInternalServerError, "workspace not set")
		return
	}

	c.String(http.StatusNotImplemented, "create budget is not implemented yet")
}

func (h *Handlers) PostUpdateBudget(c *gin.Context) {
	wsID := c.GetString(workspaces.CtxWorkspaceIDKey)
	if wsID == "" {
		c.String(http.StatusInternalServerError, "workspace not set")
		return
	}

	c.String(http.StatusNotImplemented, "update budget is not implemented yet")
}

func (h *Handlers) PostDeleteBudget(c *gin.Context) {
	wsID := c.GetString(workspaces.CtxWorkspaceIDKey)
	if wsID == "" {
		c.String(http.StatusInternalServerError, "workspace not set")
		return
	}

	c.String(http.StatusNotImplemented, "delete budget is not implemented yet")
}

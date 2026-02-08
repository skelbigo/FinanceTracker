package web

import (
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/skelbigo/FinanceTracker/apps/gateway-http/workspaces"
	"github.com/skelbigo/FinanceTracker/apps/transaction-service/transactions"
)

type dashSummaryVM struct {
	From     string
	To       string
	Currency string
	Income   string
	Expense  string
	Net      string
}

type dashBudgetVM struct {
	Category    string
	Period      string
	PercentUsed int
	Spent       string
	Limit       string
	Currency    string
	IsOver      bool
}

type dashTxVM struct {
	Occurred string
	Type     string
	Category string
	Amount   string
	Currency string
	Note     string
}

func (h *Handlers) GetDashboard(c *gin.Context) {
	if h.Transactions == nil || h.Analytics == nil || h.Categories == nil {
		c.String(http.StatusInternalServerError, "services are not configured")
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

	currency := "UAH"
	if v, ok := c.Get("workspace"); ok {
		if w, ok2 := v.(workspaces.Workspace); ok2 {
			if w.DefaultCurrency != "" {
				currency = w.DefaultCurrency
			}
		}
	}

	now := time.Now().UTC()
	from := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	fromStr := from.Format("2006-01-02")
	toStr := now.Format("2006-01-02")

	_, catNames, err := h.loadCategoriesMap(c.Request.Context(), wsID)
	if err != nil {
		c.String(http.StatusInternalServerError, "could not list categories")
		return
	}

	sum, err := h.Analytics.Summary(c.Request.Context(), wsUUID, fromStr, toStr, currency)
	if err != nil {
		c.String(http.StatusInternalServerError, "could not load summary")
		return
	}

	summaryVM := dashSummaryVM{
		From:     sum.From,
		To:       sum.To,
		Currency: sum.Currency,
		Income:   formatMinor(sum.IncomeTotal),
		Expense:  formatMinor(sum.ExpenseTotal),
		Net:      formatMinor(sum.Net),
	}

	txRes, err := h.Transactions.List(c.Request.Context(), wsID, transactions.ListFilter{
		Limit:  5,
		Offset: 0,
		Sort:   "occurred_at_desc",
	})
	if err != nil {
		c.String(http.StatusInternalServerError, "could not load recent transactions")
		return
	}

	recent := make([]dashTxVM, 0, len(txRes.Items))
	for _, t := range txRes.Items {
		recent = append(recent, dashTxVM{
			Occurred: t.OccurredAt.Format("2006-01-02"),
			Type:     string(t.Type),
			Category: categoryName(t.CategoryID, catNames),
			Amount:   formatMinor(t.AmountMinor),
			Currency: t.Currency,
			Note:     optionalString(t.Note),
		})
	}

	budgetsTotal := 0
	budgetsOverspent := 0
	var topBudgets []dashBudgetVM
	if h.Budgets != nil {
		items, berr := h.Budgets.ListWithProgress(c.Request.Context(), wsUUID, nil, now)
		if berr == nil {
			budgetsTotal = len(items)
			rows := make([]dashBudgetVM, 0, len(items))
			for _, b := range items {
				if b.IsOver {
					budgetsOverspent++
				}
				pct := b.PercentUsed
				if pct < 0 {
					pct = 0
				}
				if pct > 100 {
					pct = 100
				}
				catID := b.CategoryID.String()
				cat := catID
				if name, ok := catNames[catID]; ok && name != "" {
					cat = name
				}
				rows = append(rows, dashBudgetVM{
					Category:    cat,
					Period:      string(b.Period),
					PercentUsed: pct,
					Spent:       formatMinor(b.SpentMinor),
					Limit:       formatMinor(b.AmountLimitMinor),
					Currency:    b.Currency,
					IsOver:      b.IsOver,
				})
			}

			sort.Slice(rows, func(i, j int) bool {
				return rows[i].PercentUsed > rows[j].PercentUsed
			})
			if len(rows) > 3 {
				rows = rows[:3]
			}
			topBudgets = rows
		}
	}

	kicker := c.Query("flash")
	h.render(c, "app/dashboard.html", gin.H{
		"Title":            "Dashboard",
		"Kicker":           kicker,
		"BodyClass":        "app-dark",
		"MainClass":        "dash-main",
		"Workspace":        workspaceFromContext(c),
		"Summary":          summaryVM,
		"BudgetsTotal":     budgetsTotal,
		"BudgetsOverspent": budgetsOverspent,
		"TopBudgets":       topBudgets,
		"RecentTx":         recent,
	})
}

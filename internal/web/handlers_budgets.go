package web

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/skelbigo/FinanceTracker/internal/budgets"
	"github.com/skelbigo/FinanceTracker/internal/transactions"
	"github.com/skelbigo/FinanceTracker/internal/workspaces"
)

type BudgetRowVM struct {
	ID          string
	CategoryID  string
	Category    string
	Period      string
	Currency    string
	Limit       string
	Spent       string
	Remaining   string
	OverBy      string
	PeriodStart string
	PeriodEnd   string
	PercentUsed int
	PercentBar  int
	IsOver      bool
	Status      string
	CSRF        string
}

func budgetRowFromResponse(b budgets.BudgetResponse, catNames map[string]string, csrf string) BudgetRowVM {
	catID := b.CategoryID.String()
	catName := catID
	if name, ok := catNames[catID]; ok && name != "" {
		catName = name
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

	overBy := ""
	if b.IsOver {
		ob := b.SpentMinor - b.AmountLimitMinor
		if ob < 0 {
			ob = -ob
		}
		overBy = formatMinor(ob)
	}

	periodFrom := ""
	periodTo := ""
	if !b.PeriodStart.IsZero() {
		periodFrom = b.PeriodStart.Format("2006-01-02")
	}
	if !b.PeriodEnd.IsZero() {
		periodTo = b.PeriodEnd.AddDate(0, 0, -1).Format("2006-01-02")
	}

	return BudgetRowVM{
		ID:          b.ID.String(),
		CategoryID:  catID,
		Category:    catName,
		Period:      string(b.Period),
		Currency:    b.Currency,
		Limit:       formatMinor(b.AmountLimitMinor),
		Spent:       formatMinor(b.SpentMinor),
		Remaining:   formatMinor(b.RemainingMinor),
		OverBy:      overBy,
		PeriodStart: periodFrom,
		PeriodEnd:   periodTo,
		PercentUsed: b.PercentUsed,
		PercentBar:  bar,
		IsOver:      b.IsOver,
		Status:      status,
		CSRF:        csrf,
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

	now := time.Now()
	items, err := h.Budgets.ListWithProgress(c.Request.Context(), wsUUID, nil, now)
	if err != nil {
		c.String(http.StatusInternalServerError, "could not list budgets")
		return
	}

	csrf := GenerateCSRF(h.CSRFSecret, h.CSRFTTL)

	rows := make([]BudgetRowVM, 0, len(items))
	for _, it := range items {
		rows = append(rows, budgetRowFromResponse(it, catNames, csrf))
	}

	h.render(c, "app/budgets.html", gin.H{
		"Title":           "Budgets",
		"BodyClass":       "app-dark app-solid",
		"MainClass":       "tx-main",
		"Workspace":       workspaceFromContext(c),
		"Flash":           c.Query("flash"),
		"CSRF":            csrf,
		"Categories":      cats,
		"DefaultCurrency": "UAH",
		"Budgets":         rows,
	})
}

func normalizePeriod(s string) budgets.Period {
	return budgets.Period(strings.ToLower(strings.TrimSpace(s)))
}

func budgetRowData(row BudgetRowVM) gin.H {
	return gin.H{
		"ID":          row.ID,
		"CategoryID":  row.CategoryID,
		"Category":    row.Category,
		"Period":      row.Period,
		"Currency":    row.Currency,
		"Limit":       row.Limit,
		"Spent":       row.Spent,
		"Remaining":   row.Remaining,
		"OverBy":      row.OverBy,
		"PeriodStart": row.PeriodStart,
		"PeriodEnd":   row.PeriodEnd,
		"PercentUsed": row.PercentUsed,
		"PercentBar":  row.PercentBar,
		"IsOver":      row.IsOver,
		"Status":      row.Status,
		"CSRF":        row.CSRF,
	}
}

func (h *Handlers) loadBudgetRowVM(ctx context.Context, wsUUID uuid.UUID, wsID string, budgetID uuid.UUID, csrf string) (BudgetRowVM, error) {
	_, catNames, err := h.loadCategoriesMap(ctx, wsID)
	if err != nil {
		return BudgetRowVM{}, err
	}

	items, err := h.Budgets.ListWithProgress(ctx, wsUUID, nil, time.Now())
	if err != nil {
		return BudgetRowVM{}, err
	}
	for _, it := range items {
		if it.ID == budgetID {
			return budgetRowFromResponse(it, catNames, csrf), nil
		}
	}

	b, err := h.Budgets.GetByID(ctx, wsUUID, budgetID)
	if err != nil {
		return BudgetRowVM{}, err
	}
	start, end := budgets.PeriodBounds(time.Now(), b.Period)
	fallback := budgets.NewBudgetResponse(b, 0, start, end)
	return budgetRowFromResponse(fallback, catNames, csrf), nil
}

func (h *Handlers) PostCreateBudget(c *gin.Context) {
	if h.Budgets == nil || h.Categories == nil {
		c.String(http.StatusInternalServerError, "budgets/categories service is not configured")
		return
	}

	wsID := c.GetString(workspaces.CtxWorkspaceIDKey)
	if wsID == "" {
		c.String(http.StatusInternalServerError, "workspace not set")
		return
	}

	if !h.requireWorkspaceRoleForMutation(c, workspaces.RoleMember, "budget_errors", "/app/budgets") {
		return
	}
	wsUUID, err := uuid.Parse(wsID)
	if err != nil {
		c.String(http.StatusBadRequest, "invalid workspace id")
		return
	}

	errs := make([]string, 0, 4)

	rawCat := strings.TrimSpace(c.PostForm("category_id"))
	var catID uuid.UUID
	if rawCat == "" {
		errs = append(errs, "Category is required")
	} else {
		parsed, perr := uuid.Parse(rawCat)
		if perr != nil {
			errs = append(errs, "Category id is invalid")
		} else {
			catID = parsed
		}
	}

	p := normalizePeriod(c.PostForm("period"))
	if !p.IsValid() {
		errs = append(errs, "Period must be week or month")
	}

	rawLimit := strings.TrimSpace(c.PostForm("limit"))
	limitMinor, lerr := transactions.ParseAmountMinor(rawLimit)
	if lerr != nil || limitMinor <= 0 {
		errs = append(errs, "Limit must be a positive number (e.g. 200.00)")
	}

	rawCur := strings.ToUpper(strings.TrimSpace(c.PostForm("currency")))
	var cur string
	if rawCur == "" {
		errs = append(errs, "Currency is required")
	} else {
		var cerr error
		cur, cerr = transactions.NormalizeCurrencyStrict(rawCur)
		if cerr != nil {
			errs = append(errs, "Currency must be 3 uppercase letters (e.g. UAH)")
		}
	}

	if len(errs) > 0 {
		c.Status(http.StatusUnprocessableEntity)
		h.renderPartial(c, "budget_errors", gin.H{"Errors": errs})
		return
	}

	existing, err := h.Budgets.ListBudgets(c.Request.Context(), wsUUID, nil)
	if err != nil {
		c.String(http.StatusInternalServerError, "could not list budgets")
		return
	}
	for _, b := range existing {
		if b.CategoryID == catID && b.Period == p && strings.EqualFold(b.Currency, cur) {
			c.Status(http.StatusConflict)
			h.renderPartial(c, "budget_errors", gin.H{"Errors": []string{"Budget already exists"}})
			return
		}
	}

	created, err := h.Budgets.UpsertBudget(c.Request.Context(), wsUUID, budgets.UpsertBudgetRequest{
		CategoryID:       catID,
		Period:           p,
		AmountLimitMinor: limitMinor,
		Currency:         cur,
	})
	if err != nil {
		switch {
		case errors.Is(err, budgets.ErrCategoryNotFound):
			c.Status(http.StatusUnprocessableEntity)
			h.renderPartial(c, "budget_errors", gin.H{"Errors": []string{"Category not found"}})
		case errors.Is(err, budgets.ErrCategoryNotExpense):
			c.Status(http.StatusUnprocessableEntity)
			h.renderPartial(c, "budget_errors", gin.H{"Errors": []string{"Category must be expense"}})
		case errors.Is(err, budgets.ErrInvalidPeriod),
			errors.Is(err, budgets.ErrInvalidLimit),
			errors.Is(err, budgets.ErrInvalidCurrency):
			c.Status(http.StatusUnprocessableEntity)
			h.renderPartial(c, "budget_errors", gin.H{"Errors": []string{err.Error()}})
		default:
			c.String(http.StatusInternalServerError, "could not create budget")
		}
		return
	}

	_, catNames, _ := h.loadCategoriesMap(c.Request.Context(), wsID)

	now := time.Now()
	progress, err := h.Budgets.ListWithProgress(c.Request.Context(), wsUUID, nil, now)
	if err != nil {
		c.String(http.StatusInternalServerError, "could not list budgets")
		return
	}

	var row BudgetRowVM
	found := false
	csrf := h.csrfForRows(c)
	for _, it := range progress {
		if it.ID == created.ID {
			row = budgetRowFromResponse(it, catNames, csrf)
			found = true
			break
		}
	}
	if !found {
		start, end := budgets.PeriodBounds(now, created.Period)
		row = budgetRowFromResponse(budgets.NewBudgetResponse(created, 0, start, end), catNames, csrf)
	}

	c.Status(http.StatusOK)
	h.renderPartial(c, "budget_clear_errors", gin.H{})
	_, _ = c.Writer.WriteString("\n<table style=\"display:none\"><tbody>\n<tr id=\"budget-empty\" hx-swap-oob=\"delete\"></tr>\n</tbody></table>\n")
	_, _ = c.Writer.WriteString("\n<table id=\"budget-fragment\" style=\"display:none\"><tbody>\n")
	h.renderPartial(c, "budget_row", budgetRowData(row))
	_, _ = c.Writer.WriteString("\n</tbody></table>\n")
}

func (h *Handlers) PostUpdateBudget(c *gin.Context) {
	if h.Budgets == nil || h.Categories == nil {
		c.String(http.StatusInternalServerError, "budgets/categories service is not configured")
		return
	}

	wsID := c.GetString(workspaces.CtxWorkspaceIDKey)
	if wsID == "" {
		c.String(http.StatusInternalServerError, "workspace not set")
		return
	}

	if !h.requireWorkspaceRoleForMutation(c, workspaces.RoleMember, "budget_errors", "/app/budgets") {
		return
	}
	wsUUID, err := uuid.Parse(wsID)
	if err != nil {
		c.String(http.StatusBadRequest, "invalid workspace id")
		return
	}

	budgetID := strings.TrimSpace(c.Param("id"))
	if budgetID == "" {
		c.String(http.StatusBadRequest, "missing id")
		return
	}
	budgetUUID, err := uuid.Parse(budgetID)
	if err != nil {
		c.String(http.StatusBadRequest, "invalid budget id")
		return
	}

	errs := make([]string, 0, 4)

	rawCat := strings.TrimSpace(c.PostForm("category_id"))
	catUUID, catErr := uuid.Parse(rawCat)
	if rawCat == "" {
		errs = append(errs, "Category is required")
	} else if catErr != nil {
		errs = append(errs, "Category id is invalid")
	}

	rawPeriod := strings.TrimSpace(c.PostForm("period"))
	p := normalizePeriod(rawPeriod)
	if !p.IsValid() {
		errs = append(errs, "Period must be week or month")
	}

	rawLimit := strings.TrimSpace(c.PostForm("limit"))
	limitMinor, lerr := transactions.ParseAmountMinor(rawLimit)
	if lerr != nil || limitMinor <= 0 {
		errs = append(errs, "Limit must be a positive number (e.g. 200.00)")
	}

	rawCur := strings.ToUpper(strings.TrimSpace(c.PostForm("currency")))
	var cur string
	if rawCur == "" {
		errs = append(errs, "Currency is required")
	} else {
		var cerr error
		cur, cerr = transactions.NormalizeCurrencyStrict(rawCur)
		if cerr != nil {
			errs = append(errs, "Currency must be 3 uppercase letters (e.g. UAH)")
		}
	}

	csrf := h.csrfForRows(c)

	if len(errs) > 0 {
		c.Status(http.StatusUnprocessableEntity)
		h.renderPartial(c, "budget_errors", gin.H{"Errors": errs})

		row, rerr := h.loadBudgetRowVM(c.Request.Context(), wsUUID, wsID, budgetUUID, csrf)
		if rerr == nil {
			if catErr == nil {
				_, catNames, _ := h.loadCategoriesMap(c.Request.Context(), wsID)
				row.CategoryID = catUUID.String()
				if name, ok := catNames[catUUID.String()]; ok && name != "" {
					row.Category = name
				}
			}
			if p.IsValid() {
				row.Period = string(p)
			}
			row.Currency = strings.TrimSpace(c.PostForm("currency"))
			row.Limit = strings.TrimSpace(c.PostForm("limit"))
			row.CSRF = csrf
			h.renderPartial(c, "budget_row", budgetRowData(row))
		}
		return
	}

	_, err = h.Budgets.Update(c.Request.Context(), wsUUID, budgetUUID, budgets.UpsertBudgetRequest{
		CategoryID:       catUUID,
		Period:           p,
		AmountLimitMinor: limitMinor,
		Currency:         cur,
	})
	if err != nil {
		msg := "could not update budget"
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, budgets.ErrBudgetExists):
			status = http.StatusConflict
			msg = "Budget already exists"
		case errors.Is(err, budgets.ErrBudgetNotFound):
			status = http.StatusNotFound
			msg = "Budget not found"
		case errors.Is(err, budgets.ErrCategoryNotFound):
			status = http.StatusUnprocessableEntity
			msg = "Category not found"
		case errors.Is(err, budgets.ErrCategoryNotExpense):
			status = http.StatusUnprocessableEntity
			msg = "Category must be expense"
		case errors.Is(err, budgets.ErrInvalidPeriod),
			errors.Is(err, budgets.ErrInvalidLimit),
			errors.Is(err, budgets.ErrInvalidCurrency):
			status = http.StatusUnprocessableEntity
			msg = err.Error()
		}

		c.Status(status)
		if status == http.StatusInternalServerError {
			c.String(status, msg)
			return
		}
		h.renderPartial(c, "budget_errors", gin.H{"Errors": []string{msg}})

		row, rerr := h.loadBudgetRowVM(c.Request.Context(), wsUUID, wsID, budgetUUID, csrf)
		if rerr == nil {
			_, catNames, _ := h.loadCategoriesMap(c.Request.Context(), wsID)
			row.CategoryID = catUUID.String()
			if name, ok := catNames[catUUID.String()]; ok && name != "" {
				row.Category = name
			}
			row.Period = string(p)
			row.Currency = strings.TrimSpace(c.PostForm("currency"))
			row.Limit = strings.TrimSpace(c.PostForm("limit"))
			row.CSRF = csrf
			h.renderPartial(c, "budget_row", budgetRowData(row))
		}
		return
	}

	_, catNames, _ := h.loadCategoriesMap(c.Request.Context(), wsID)
	now := time.Now()
	progress, err := h.Budgets.ListWithProgress(c.Request.Context(), wsUUID, nil, now)
	if err != nil {
		c.String(http.StatusInternalServerError, "could not list budgets")
		return
	}

	var row BudgetRowVM
	found := false
	for _, it := range progress {
		if it.ID == budgetUUID {
			row = budgetRowFromResponse(it, catNames, csrf)
			found = true
			break
		}
	}
	if !found {
		b, berr := h.Budgets.GetByID(c.Request.Context(), wsUUID, budgetUUID)
		if berr != nil {
			c.String(http.StatusInternalServerError, "budget not found")
			return
		}
		start, end := budgets.PeriodBounds(now, b.Period)
		row = budgetRowFromResponse(budgets.NewBudgetResponse(b, 0, start, end), catNames, csrf)
	}

	c.Status(http.StatusOK)
	h.renderPartial(c, "budget_clear_errors", gin.H{})
	h.renderPartial(c, "budget_row", budgetRowData(row))
}

func (h *Handlers) PostDeleteBudget(c *gin.Context) {
	if h.Budgets == nil {
		c.String(http.StatusInternalServerError, "budgets service is not configured")
		return
	}

	wsID := c.GetString(workspaces.CtxWorkspaceIDKey)
	if wsID == "" {
		c.String(http.StatusInternalServerError, "workspace not set")
		return
	}

	if !h.requireWorkspaceRoleForMutation(c, workspaces.RoleMember, "budget_errors", "/app/budgets") {
		return
	}
	wsUUID, err := uuid.Parse(wsID)
	if err != nil {
		c.String(http.StatusBadRequest, "invalid workspace id")
		return
	}

	budgetID := strings.TrimSpace(c.Param("id"))
	if budgetID == "" {
		c.String(http.StatusBadRequest, "missing id")
		return
	}
	budgetUUID, err := uuid.Parse(budgetID)
	if err != nil {
		c.String(http.StatusBadRequest, "invalid budget id")
		return
	}

	err = h.Budgets.Delete(c.Request.Context(), wsUUID, budgetUUID)
	if err != nil {
		switch {
		case errors.Is(err, budgets.ErrBudgetNotFound):
			c.Status(http.StatusNotFound)
			h.renderPartial(c, "budget_errors", gin.H{"Errors": []string{"Budget not found"}})
			return
		default:
			c.String(http.StatusInternalServerError, "could not delete budget")
			return
		}
	}

	c.Status(http.StatusOK)
	h.renderPartial(c, "budget_clear_errors", gin.H{})
	_, _ = c.Writer.WriteString("\n<tr id=\"budget-" + budgetID + "\" hx-swap-oob=\"delete\"></tr>\n")

	items, lerr := h.Budgets.ListBudgets(c.Request.Context(), wsUUID, nil)
	if lerr == nil && len(items) == 0 {
		_, _ = c.Writer.WriteString("\n<tr id=\"budget-empty\" hx-swap-oob=\"beforeend:#budget-tbody\"><td colspan=\"8\">No budgets</td></tr>\n")
	}
}

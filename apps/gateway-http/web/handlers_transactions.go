package web

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/skelbigo/FinanceTracker/apps/auth-service/auth"
	"github.com/skelbigo/FinanceTracker/apps/gateway-http/workspaces"
	"github.com/skelbigo/FinanceTracker/apps/transaction-service/categories"
	"github.com/skelbigo/FinanceTracker/apps/transaction-service/transactions"
)

type txFiltersVM struct {
	From       string
	To         string
	Type       string
	CategoryID string
	Q          string
	Sort       string
	Limit      int
	Offset     int
}

type txRowVM struct {
	ID       string
	Occurred string
	Type     string
	Category string
	Amount   string
	Currency string
	Note     string
	Tags     string
	CSRF     string
}

type txRowEditVM struct {
	ID         string
	Occurred   string
	Type       string
	CategoryID string
	Amount     string
	Currency   string
	Note       string
	Tags       string
}

type txPaginationVM struct {
	ShowPrev   bool
	ShowNext   bool
	PrevOffset int
	NextOffset int
	Info       string
}

type txFormInput struct {
	Type       string
	Amount     string
	Currency   string
	OccurredAt string
	CategoryID string
	Note       string
	Tags       string
}

func readTxForm(c *gin.Context) txFormInput {
	return txFormInput{
		Type:       c.PostForm("type"),
		Amount:     c.PostForm("amount"),
		Currency:   c.PostForm("currency"),
		OccurredAt: c.PostForm("occurred_at"),
		CategoryID: c.PostForm("category_id"),
		Note:       c.PostForm("note"),
		Tags:       c.PostForm("tags"),
	}
}

func parseTxForm(in txFormInput) (typ transactions.Type, minor int64, currency string, occurredAt time.Time, catID *string, note *string, tags []string, errs []string) {
	typ = transactions.NormalizeType(in.Type)
	if !transactions.ValidateType(typ) {
		errs = append(errs, "Type must be income or expense")
	}

	var err error
	minor, err = transactions.ParseAmountMinor(in.Amount)
	if err != nil {
		errs = append(errs, "Amount must be a positive number (e.g. 12.34)")
	}

	currency, err = transactions.NormalizeCurrencyStrict(in.Currency)
	if err != nil {
		errs = append(errs, "Currency must be 3 uppercase letters (e.g. UAH)")
	}

	occurredAt, err = transactions.ParseOccurredAt(in.OccurredAt)
	if err != nil {
		errs = append(errs, "Occurred at must be a valid date")
	}

	catRaw := strings.TrimSpace(in.CategoryID)
	catID, err = transactions.NormalizeOptionalUUID(&catRaw)
	if err != nil {
		errs = append(errs, "Category id is invalid")
	}

	noteRaw := strings.TrimSpace(in.Note)
	note = transactions.NormalizeOptionalNote(&noteRaw)

	tags, err = transactions.ParseTagsCSV(in.Tags)
	if err != nil {
		errs = append(errs, err.Error())
	}

	return
}

func (h *Handlers) loadCategoriesMap(ctx context.Context, wsID string) ([]categories.Category, map[string]string, error) {
	cats, err := h.Categories.List(ctx, wsID)
	if err != nil {
		return nil, nil, err
	}
	names := make(map[string]string, len(cats))
	for _, cat := range cats {
		names[cat.ID] = cat.Name
	}
	return cats, names, nil
}

func (h *Handlers) csrfForRows(c *gin.Context) string {
	csrf := strings.TrimSpace(c.GetHeader("X-CSRF-Token"))
	if csrf == "" {
		csrf = GenerateCSRF(h.CSRFSecret, h.CSRFTTL)
	}
	return csrf
}

func txRowFromModel(t transactions.Transaction, catNames map[string]string, csrf string) txRowVM {
	return txRowVM{
		ID:       t.ID,
		Occurred: t.OccurredAt.Format("2006-01-02"),
		Type:     string(t.Type),
		Category: categoryName(t.CategoryID, catNames),
		Amount:   formatMinor(t.AmountMinor),
		Currency: t.Currency,
		Note:     optionalString(t.Note),
		Tags:     strings.Join(t.Tags, ", "),
		CSRF:     csrf,
	}
}

func (h *Handlers) GetTransactionsPage(c *gin.Context) {
	if h.Categories == nil || h.Transactions == nil {
		c.String(http.StatusInternalServerError, "categories/transactions service is not configured")
		return
	}

	wsID := c.GetString(workspaces.CtxWorkspaceIDKey)
	if wsID == "" {
		c.String(http.StatusInternalServerError, "workspace not set")
		return
	}

	cats, _, err := h.loadCategoriesMap(c.Request.Context(), wsID)
	if err != nil {
		c.String(http.StatusInternalServerError, "could not list categories")
		return
	}

	filters := readTxFiltersFromQuery(c)
	if filters.Sort == "" {
		filters.Sort = "occurred_at_desc"
	}
	if filters.Limit <= 0 {
		filters.Limit = 20
	}

	data := gin.H{
		"Title":           "Transactions",
		"BodyClass":       "app-dark app-solid",
		"MainClass":       "tx-main",
		"Flash":           c.Query("flash"),
		"Workspace":       workspaceFromContext(c),
		"Categories":      cats,
		"Filters":         filters,
		"DefaultCurrency": "UAH",
	}

	h.render(c, "app/transactions.html", data)
}

func (h *Handlers) GetTransactionsTable(c *gin.Context) {
	if h.Categories == nil || h.Transactions == nil {
		c.String(http.StatusInternalServerError, "categories/transactions service is not configured")
		return
	}

	wsID := c.GetString(workspaces.CtxWorkspaceIDKey)
	if wsID == "" {
		c.String(http.StatusInternalServerError, "workspace not set")
		return
	}

	filtersVM := readTxFiltersFromQuery(c)

	f, errList := buildTxListFilter(filtersVM)
	if errList != nil {
		c.Header("HX-Reswap", "none")
		c.Status(http.StatusOK)
		h.renderPartial(c, "tx_form_errors", gin.H{
			"Errors": []string{errList.Error()},
		})
		return
	}

	result, err := h.Transactions.List(c.Request.Context(), wsID, f)
	if err != nil {
		c.String(http.StatusInternalServerError, "could not list transactions")
		return
	}

	_, catNames, err := h.loadCategoriesMap(c.Request.Context(), wsID)
	if err != nil {
		c.String(http.StatusInternalServerError, "could not list categories")
		return
	}

	csrf := h.csrfForRows(c)

	rows := make([]txRowVM, 0, len(result.Items))
	for _, item := range result.Items {
		rows = append(rows, txRowFromModel(item, catNames, csrf))
	}

	p := buildPagination(filtersVM.Offset, result.Limit, len(result.Items), result.HasNext)

	filtersVM.Limit = result.Limit
	filtersVM.Offset = result.Offset

	h.renderPartial(c, "tx_tbody", gin.H{
		"Items":      rows,
		"Pagination": p,
		"Filters":    filtersVM,
	})
}

func (h *Handlers) PostCreateTransaction(c *gin.Context) {
	if h.Categories == nil || h.Transactions == nil {
		c.String(http.StatusInternalServerError, "categories/transactions service is not configured")
		return
	}

	wsID := c.GetString(workspaces.CtxWorkspaceIDKey)
	if wsID == "" {
		c.String(http.StatusInternalServerError, "workspace not set")
		return
	}

	if !h.requireWorkspaceRoleForMutation(c, workspaces.RoleMember, "tx_form_errors", "/app/transactions") {
		return
	}

	userID := c.GetString(auth.CtxUserIDKey)
	if userID == "" {
		c.Redirect(http.StatusSeeOther, "/login?flash=Please+login")
		return
	}

	in := readTxForm(c)
	typ, minor, currency, occurredAt, catID, note, tags, errs := parseTxForm(in)

	if len(errs) > 0 {
		c.Status(http.StatusUnprocessableEntity)
		h.renderPartial(c, "tx_form_errors", gin.H{"Errors": errs})
		return
	}

	out, err := h.Transactions.Create(c.Request.Context(), transactions.Transaction{
		WorkspaceID: wsID,
		UserID:      userID,
		CategoryID:  catID,
		Type:        typ,
		AmountMinor: minor,
		Currency:    currency,
		OccurredAt:  occurredAt,
		Note:        note,
		Tags:        tags,
	})
	if err != nil {
		c.String(http.StatusInternalServerError, "could not create transaction")
		return
	}

	_, catNames, _ := h.loadCategoriesMap(c.Request.Context(), wsID)

	row := txRowFromModel(out, catNames, h.csrfForRows(c))
	h.renderPartial(c, "tx_create_response", gin.H{"Row": row})
}

func (h *Handlers) GetTransactionEdit(c *gin.Context) {
	if h.Categories == nil || h.Transactions == nil {
		c.String(http.StatusInternalServerError, "categories/transactions service is not configured")
		return
	}

	wsID := c.GetString(workspaces.CtxWorkspaceIDKey)
	if wsID == "" {
		c.String(http.StatusInternalServerError, "workspace not set")
		return
	}
	if c.GetHeader("HX-Request") == "" {
		c.Redirect(http.StatusSeeOther, "/app/transactions")
		return
	}

	txID := strings.TrimSpace(c.Param("id"))
	if txID == "" {
		c.String(http.StatusBadRequest, "missing id")
		return
	}

	tx, err := h.Transactions.GetByID(c.Request.Context(), wsID, txID)
	if err != nil {
		c.String(http.StatusNotFound, "not found")
		return
	}

	cats, err := h.Categories.List(c.Request.Context(), wsID)
	if err != nil {
		c.String(http.StatusInternalServerError, "could not list categories")
		return
	}

	catID := ""
	if tx.CategoryID != nil {
		catID = *tx.CategoryID
	}

	row := txRowEditVM{
		ID:         tx.ID,
		Occurred:   tx.OccurredAt.Format("2006-01-02"),
		Type:       string(tx.Type),
		CategoryID: catID,
		Amount:     formatMinor(tx.AmountMinor),
		Currency:   tx.Currency,
		Note:       optionalString(tx.Note),
		Tags:       strings.Join(tx.Tags, ", "),
	}

	h.renderPartial(c, "tx_row_edit_response", gin.H{
		"Row":        row,
		"Categories": cats,
		"CSRF":       h.csrfForRows(c),
	})
}

func (h *Handlers) PostUpdateTransaction(c *gin.Context) {
	if h.Categories == nil || h.Transactions == nil {
		c.String(http.StatusInternalServerError, "categories/transactions service is not configured")
		return
	}

	wsID := c.GetString(workspaces.CtxWorkspaceIDKey)
	if wsID == "" {
		c.String(http.StatusInternalServerError, "workspace not set")
		return
	}

	if !h.requireWorkspaceRoleForMutation(c, workspaces.RoleMember, "tx_form_errors", "/app/transactions") {
		return
	}

	txID := strings.TrimSpace(c.Param("id"))
	if txID == "" {
		c.String(http.StatusBadRequest, "missing id")
		return
	}

	cats, catNames, errCats := h.loadCategoriesMap(c.Request.Context(), wsID)
	if errCats != nil {
		c.String(http.StatusInternalServerError, "could not list categories")
		return
	}

	in := readTxForm(c)
	typ, minor, currency, occurredAt, catIDPtr, note, tags, errs := parseTxForm(in)

	if len(errs) > 0 {
		catID := ""
		if catIDPtr != nil {
			catID = *catIDPtr
		}
		row := txRowEditVM{
			ID:         txID,
			Occurred:   strings.TrimSpace(in.OccurredAt),
			Type:       string(typ),
			CategoryID: catID,
			Amount:     strings.TrimSpace(in.Amount),
			Currency:   strings.TrimSpace(in.Currency),
			Note:       optionalString(note),
			Tags:       strings.TrimSpace(in.Tags),
		}
		c.Status(http.StatusUnprocessableEntity)
		h.renderPartial(c, "tx_update_error", gin.H{
			"Errors":     errs,
			"Row":        row,
			"Categories": cats,
		})
		return
	}

	out, err := h.Transactions.Update(c.Request.Context(), transactions.Transaction{
		WorkspaceID: wsID,
		ID:          txID,
		CategoryID:  catIDPtr,
		Type:        typ,
		AmountMinor: minor,
		Currency:    currency,
		OccurredAt:  occurredAt,
		Note:        note,
		Tags:        tags,
	})
	if err != nil {
		c.String(http.StatusInternalServerError, "could not update transaction")
		return
	}

	row := txRowFromModel(out, catNames, h.csrfForRows(c))
	h.renderPartial(c, "tx_update_response", gin.H{"Row": row})
}

func (h *Handlers) PostDeleteTransaction(c *gin.Context) {
	if h.Transactions == nil {
		c.String(http.StatusInternalServerError, "transactions service is not configured")
		return
	}

	wsID := c.GetString(workspaces.CtxWorkspaceIDKey)
	if wsID == "" {
		c.String(http.StatusInternalServerError, "workspace not set")
		return
	}

	if !h.requireWorkspaceRoleForMutation(c, workspaces.RoleMember, "tx_form_errors", "/app/transactions") {
		return
	}

	txID := strings.TrimSpace(c.Param("id"))
	if txID == "" {
		c.String(http.StatusBadRequest, "missing id")
		return
	}

	deleted, err := h.Transactions.Delete(c.Request.Context(), wsID, txID)
	if err != nil {
		c.String(http.StatusInternalServerError, "could not delete transaction")
		return
	}
	if !deleted {
		c.String(http.StatusNotFound, "not found")
		return
	}

	h.renderPartial(c, "noop", gin.H{})
}

func workspaceFromContext(c *gin.Context) any {
	ws, _ := c.Get("workspace")
	return ws
}

func readTxFiltersFromQuery(c *gin.Context) txFiltersVM {
	limit := parseIntDefault(firstNonEmpty(c.Query("limit"), c.Query("pageSize"), c.Query("page_size")), 20)
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}

	offset := parseIntDefault(c.Query("offset"), 0)
	if offset < 0 {
		offset = 0
	}

	if c.Query("offset") == "" {
		page := parseIntDefault(c.Query("page"), 1)
		if page < 1 {
			page = 1
		}
		offset = (page - 1) * limit
	}

	return txFiltersVM{
		From:       strings.TrimSpace(c.Query("from")),
		To:         strings.TrimSpace(c.Query("to")),
		Type:       strings.TrimSpace(c.Query("type")),
		CategoryID: strings.TrimSpace(c.Query("category_id")),
		Q:          strings.TrimSpace(firstNonEmpty(c.Query("q"), c.Query("search"))),
		Sort:       strings.TrimSpace(c.Query("sort")),
		Limit:      limit,
		Offset:     offset,
	}
}

func buildTxListFilter(vm txFiltersVM) (transactions.ListFilter, error) {
	var f transactions.ListFilter

	if vm.From != "" {
		t, err := transactions.ParseOccurredAt(vm.From)
		if err != nil {
			return transactions.ListFilter{}, fmt.Errorf("invalid from date")
		}
		f.From = &t
	}
	if vm.To != "" {
		t, err := transactions.ParseOccurredAt(vm.To)
		if err != nil {
			return transactions.ListFilter{}, fmt.Errorf("invalid to date")
		}
		if len(vm.To) == 10 {
			t = t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
		}
		f.To = &t
	}

	if err := transactions.ValidateDateRange(f.From, f.To); err != nil {
		return transactions.ListFilter{}, fmt.Errorf("invalid range")
	}

	if vm.Type != "" {
		typ := transactions.NormalizeType(vm.Type)
		if !transactions.ValidateType(typ) {
			return transactions.ListFilter{}, fmt.Errorf("invalid type")
		}
		f.Type = &typ
	}

	if vm.CategoryID != "" {
		cat := vm.CategoryID
		catID, err := transactions.NormalizeOptionalUUID(&cat)
		if err != nil {
			return transactions.ListFilter{}, fmt.Errorf("invalid category_id")
		}
		f.CategoryID = catID
	}

	if vm.Q != "" {
		q := vm.Q
		f.Search = &q
	}

	f.Limit = vm.Limit
	f.Offset = vm.Offset
	sort := vm.Sort
	if sort == "" || !transactions.IsAllowedSort(sort) {
		sort = transactions.SortOccurredAtDesc
	}
	f.Sort = transactions.NormalizeSort(sort)
	return f, nil
}

func categoryName(catID *string, names map[string]string) string {
	if catID == nil {
		return "—"
	}
	if v, ok := names[*catID]; ok && v != "" {
		return v
	}
	return "—"
}

func optionalString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func formatMinor(minor int64) string {
	sign := ""
	if minor < 0 {
		sign = "-"
		minor = -minor
	}
	whole := minor / 100
	frac := minor % 100
	return fmt.Sprintf("%s%d.%02d", sign, whole, frac)
}

func buildPagination(offset, limit, got int, hasNext bool) txPaginationVM {
	if limit <= 0 {
		limit = 20
	}
	showPrev := offset > 0
	prevOffset := offset - limit
	if prevOffset < 0 {
		prevOffset = 0
	}
	nextOffset := offset + limit

	info := "No transactions"
	if got > 0 {
		start := offset + 1
		end := offset + got
		info = fmt.Sprintf("Showing %d–%d", start, end)
	}

	return txPaginationVM{
		ShowPrev:   showPrev,
		ShowNext:   hasNext,
		PrevOffset: prevOffset,
		NextOffset: nextOffset,
		Info:       info,
	}
}

func parseIntDefault(s string, def int) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

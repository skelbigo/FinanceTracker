package web

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/skelbigo/FinanceTracker/internal/auth"
	"github.com/skelbigo/FinanceTracker/internal/transactions"
	"github.com/skelbigo/FinanceTracker/internal/workspaces"
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

	cats, err := h.Categories.List(c.Request.Context(), wsID)
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
		"BodyClass":       "app-dark",
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
		c.Status(http.StatusBadRequest)
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

	cats, err := h.Categories.List(c.Request.Context(), wsID)
	if err != nil {
		c.String(http.StatusInternalServerError, "could not list categories")
		return
	}
	catNames := map[string]string{}
	for _, cat := range cats {
		catNames[cat.ID] = cat.Name
	}

	csrf := strings.TrimSpace(c.GetHeader("X-CSRF-Token"))
	if csrf == "" {
		csrf = GenerateCSRF(h.CSRFSecret, h.CSRFTTL)
	}
	rows := make([]txRowVM, 0, len(result.Items))
	for _, item := range result.Items {
		rows = append(rows, txRowVM{
			ID:       item.ID,
			Occurred: item.OccurredAt.Format("2006-01-02"),
			Type:     string(item.Type),
			Category: categoryName(item.CategoryID, catNames),
			Amount:   formatMinor(item.AmountMinor),
			Currency: item.Currency,
			Note:     optionalString(item.Note),
			Tags:     strings.Join(item.Tags, ", "),
			CSRF:     csrf,
		})
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

	userID := c.GetString(auth.CtxUserIDKey)
	if userID == "" {
		c.Redirect(http.StatusSeeOther, "/login?flash=Please+login")
		return
	}

	var errs []string

	typ := transactions.NormalizeType(c.PostForm("type"))
	if !transactions.ValidateType(typ) {
		errs = append(errs, "Type must be income or expense")
	}

	minor, err := transactions.ParseAmountMinor(c.PostForm("amount"))
	if err != nil {
		errs = append(errs, "Amount must be a positive number (e.g. 12.34)")
	}

	currency, err := transactions.NormalizeCurrencyStrict(c.PostForm("currency"))
	if err != nil {
		errs = append(errs, "Currency must be 3 uppercase letters (e.g. UAH)")
	}

	occurredAt, err := transactions.ParseOccurredAt(c.PostForm("occurred_at"))
	if err != nil {
		errs = append(errs, "Occurred at must be a valid date")
	}

	catRaw := strings.TrimSpace(c.PostForm("category_id"))
	catID, err := transactions.NormalizeOptionalUUID(&catRaw)
	if err != nil {
		errs = append(errs, "Category id is invalid")
	}

	noteRaw := strings.TrimSpace(c.PostForm("note"))
	note := transactions.NormalizeOptionalNote(&noteRaw)

	tags, err := transactions.ParseTagsCSV(c.PostForm("tags"))
	if err != nil {
		errs = append(errs, err.Error())
	}

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

	cats, _ := h.Categories.List(c.Request.Context(), wsID)
	catNames := map[string]string{}
	for _, cat := range cats {
		catNames[cat.ID] = cat.Name
	}

	csrf := strings.TrimSpace(c.GetHeader("X-CSRF-Token"))
	if csrf == "" {
		csrf = GenerateCSRF(h.CSRFSecret, h.CSRFTTL)
	}
	row := txRowVM{
		ID:       out.ID,
		Occurred: out.OccurredAt.Format("2006-01-02"),
		Type:     string(out.Type),
		Category: categoryName(out.CategoryID, catNames),
		Amount:   formatMinor(out.AmountMinor),
		Currency: out.Currency,
		Note:     optionalString(out.Note),
		Tags:     strings.Join(out.Tags, ", "),
		CSRF:     csrf,
	}

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

	h.renderPartial(c, "tx_row_edit", gin.H{
		"Row":        row,
		"Categories": cats,
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

	txID := strings.TrimSpace(c.Param("id"))
	if txID == "" {
		c.String(http.StatusBadRequest, "missing id")
		return
	}

	var errs []string

	typ := transactions.NormalizeType(c.PostForm("type"))
	if !transactions.ValidateType(typ) {
		errs = append(errs, "Type must be income or expense")
	}

	minor, err := transactions.ParseAmountMinor(c.PostForm("amount"))
	if err != nil {
		errs = append(errs, "Amount must be a positive number (e.g. 12.34)")
	}

	currency, err := transactions.NormalizeCurrencyStrict(c.PostForm("currency"))
	if err != nil {
		errs = append(errs, "Currency must be 3 uppercase letters (e.g. UAH)")
	}

	occurredAt, err := transactions.ParseOccurredAt(c.PostForm("occurred_at"))
	if err != nil {
		errs = append(errs, "Occurred at must be a valid date")
	}

	catRaw := strings.TrimSpace(c.PostForm("category_id"))
	catIDPtr, err := transactions.NormalizeOptionalUUID(&catRaw)
	if err != nil {
		errs = append(errs, "Category id is invalid")
	}

	noteRaw := strings.TrimSpace(c.PostForm("note"))
	note := transactions.NormalizeOptionalNote(&noteRaw)

	tags, err := transactions.ParseTagsCSV(c.PostForm("tags"))
	if err != nil {
		errs = append(errs, err.Error())
	}

	cats, errCats := h.Categories.List(c.Request.Context(), wsID)
	if errCats != nil {
		c.String(http.StatusInternalServerError, "could not list categories")
		return
	}

	if len(errs) > 0 {
		catID := ""
		if catIDPtr != nil {
			catID = *catIDPtr
		}
		row := txRowEditVM{
			ID:         txID,
			Occurred:   strings.TrimSpace(c.PostForm("occurred_at")),
			Type:       string(typ),
			CategoryID: catID,
			Amount:     strings.TrimSpace(c.PostForm("amount")),
			Currency:   strings.TrimSpace(c.PostForm("currency")),
			Note:       optionalString(note),
			Tags:       strings.TrimSpace(c.PostForm("tags")),
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

	catNames := map[string]string{}
	for _, cat := range cats {
		catNames[cat.ID] = cat.Name
	}

	csrf := strings.TrimSpace(c.GetHeader("X-CSRF-Token"))
	if csrf == "" {
		csrf = GenerateCSRF(h.CSRFSecret, h.CSRFTTL)
	}
	row := txRowVM{
		ID:       out.ID,
		Occurred: out.OccurredAt.Format("2006-01-02"),
		Type:     string(out.Type),
		Category: categoryName(out.CategoryID, catNames),
		Amount:   formatMinor(out.AmountMinor),
		Currency: out.Currency,
		Note:     optionalString(out.Note),
		Tags:     strings.Join(out.Tags, ", "),
		CSRF:     csrf,
	}

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
	f.Sort = vm.Sort
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
	whole := minor / 100
	frac := minor % 100
	return fmt.Sprintf("%d.%02d", whole, frac)
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

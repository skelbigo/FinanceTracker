package transactions

import (
	"github.com/gin-gonic/gin"
	"github.com/skelbigo/FinanceTracker/internal/auth"
	"github.com/skelbigo/FinanceTracker/internal/httpx"
	"github.com/skelbigo/FinanceTracker/internal/workspaces"
	"log"
	"net/http"
	"strconv"
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
	wsg.POST("/transactions", workspaces.RequireWorkspaceRole(h.ws, workspaces.RoleMember), h.create)
	wsg.GET("/transactions", workspaces.RequireWorkspaceRole(h.ws, workspaces.RoleViewer), h.list)
}

type createTxReq struct {
	Type        string   `json:"type" binding:"required"`
	AmountMinor int64    `json:"amount_minor" binding:"required"`
	Currency    string   `json:"currency" binding:"required"`
	OccurredAt  string   `json:"occurred_at" binding:"required"`
	Note        *string  `json:"note"`
	CategoryID  *string  `json:"category_id"`
	Tags        []string `json:"tags"`
}

func UserIDFromCtx(c *gin.Context) (string, bool) {
	v, ok := c.Get(auth.CtxUserIDKey)
	id, ok2 := v.(string)
	return id, ok && ok2 && id != ""
}

func (h *Handler) create(c *gin.Context) {
	workspaceID, ok := workspaces.GetWorkspaceID(c)
	if !ok {
		httpx.Internal(c)
		return
	}

	userID, ok := UserIDFromCtx(c)
	if !ok {
		httpx.Unauthorized(c, "invalid token")
		return
	}

	var req createTxReq
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "invalid json", nil)
		return
	}

	typ := NormalizeType(req.Type)
	if !ValidateType(typ) {
		httpx.Unprocessable(c, "invalid transactions type", map[string]string{"type": "income|expense"})
		return
	}

	if req.AmountMinor <= 0 {
		httpx.Unprocessable(c, "invalid amount", map[string]string{"amount_minor": "must be > 0"})
		return
	}

	cur, err := NormalizeCurrencyStrict(req.Currency)
	if err != nil {
		httpx.Unprocessable(c, "invalid currency", map[string]string{
			"currency": "ISO 4217 like UAH, USD (uppercase)",
		})
		return
	}

	occ, err := ParseOccurredAt(req.OccurredAt)
	if err != nil {
		httpx.Unprocessable(c, "invalid occurred at", map[string]string{"occurred_at": "YYYY-MM-DD or RFC3339"})
		return
	}

	catID, err := NormalizeOptionalUUID(req.CategoryID)
	if err != nil {
		httpx.Unprocessable(c, "invalid category_id", map[string]string{"category_id": "must be uuid"})
		return
	}

	note := NormalizeOptionalNote(req.Note)

	tags, err := NormalizeTagsSlice(req.Tags)
	if err != nil {
		httpx.Unprocessable(c, "invalid tags", map[string]string{"tags": err.Error()})
		return
	}

	tx := Transaction{
		WorkspaceID: workspaceID,
		UserID:      userID,
		CategoryID:  catID,
		Type:        typ,
		AmountMinor: req.AmountMinor,
		Currency:    cur,
		OccurredAt:  occ,
		Note:        note,
		Tags:        tags,
	}

	out, err := h.svc.Create(c.Request.Context(), tx)
	if err != nil {
		httpx.Internal(c)
		log.Printf("transactions.create: %v", err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"transaction": out})
}

func (h *Handler) list(c *gin.Context) {
	workspaceID, ok := workspaces.GetWorkspaceID(c)
	if !ok {
		httpx.Internal(c)
		return
	}

	var f ListFilter

	if v := strings.TrimSpace(c.Query("from")); v != "" {
		t, err := ParseOccurredAt(v)
		if err != nil {
			httpx.Unprocessable(c, "invalid from", map[string]string{"from": "YYYY-MM-DD or RFC3339"})
			return
		}
		f.From = &t
	}
	if v := strings.TrimSpace(c.Query("to")); v != "" {
		t, err := ParseOccurredAt(v)
		if err != nil {
			httpx.Unprocessable(c, "invalid to", map[string]string{"to": "YYYY-MM-DD or RFC3339"})
			return
		}
		f.To = &t
	}
	if f.From != nil && f.To != nil && f.From.After(*f.To) {
		httpx.Unprocessable(c, "invalid range", map[string]string{"range": "from must be <= to"})
		return
	}
	if v := strings.TrimSpace(c.Query("type")); v != "" {
		typ := NormalizeType(v)
		if !ValidateType(typ) {
			httpx.Unprocessable(c, "invalid transaction type", map[string]string{"type": "income|expense"})
			return
		}
		f.Type = &typ
	}

	if v := strings.TrimSpace(c.Query("category_id")); v != "" {
		vv := v
		catID, err := NormalizeOptionalUUID(&vv)
		if err != nil {
			httpx.Unprocessable(c, "invalid category_id", map[string]string{"category_id": "must be uuid"})
			return
		}
		f.CategoryID = catID
	}

	if v := strings.TrimSpace(c.Query("q")); v != "" {
		f.Search = &v
	} else if v := strings.TrimSpace(c.Query("search")); v != "" {
		f.Search = &v
	}

	if v := strings.TrimSpace(c.Query("limit")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			httpx.Unprocessable(c, "invalid limit", map[string]string{"limit": "must be positive int"})
			return
		}
		f.Limit = n
	} else if v := strings.TrimSpace(c.Query("page_size")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			httpx.Unprocessable(c, "invalid page_size", map[string]string{"page_size": "must be positive int"})
			return
		}
		f.Limit = n
	}

	if v := strings.TrimSpace(c.Query("offset")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			httpx.Unprocessable(c, "invalid offset", map[string]string{"offset": "must be >= 0"})
			return
		}
		f.Offset = n
	}

	if v := strings.TrimSpace(c.Query("sort")); v != "" {
		f.Sort = v
	}

	res, err := h.svc.List(c.Request.Context(), workspaceID, f)
	if err != nil {
		httpx.Internal(c)
		log.Printf("transactions.list: %v", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"items":    res.Items,
		"has_next": res.HasNext,
		"limit":    res.Limit,
		"offset":   res.Offset,
	})
}

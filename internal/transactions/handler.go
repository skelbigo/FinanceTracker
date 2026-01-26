package transactions

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/skelbigo/FinanceTracker/internal/auth"
	"github.com/skelbigo/FinanceTracker/internal/httpx"
	"github.com/skelbigo/FinanceTracker/internal/workspaces"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"
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
	Type        string  `json:"type" binding:"required"`
	AmountMinor int64   `json:"amount_minor" binding:"required"`
	Currency    string  `json:"currency" binding:"required"`
	OccurredAt  string  `json:"occurred_at" binding:"required"`
	Note        *string `json:"note"`
	CategoryID  *string `json:"category_id"`
}

func UserIDFromCtx(c *gin.Context) (string, bool) {
	v, ok := c.Get(auth.CtxUserIDKey)
	id, ok2 := v.(string)
	return id, ok && ok2 && id != ""
}

var currencyRe = regexp.MustCompile(`^[A-Z]{3}$`)

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

	typ := Type(strings.TrimSpace(strings.ToLower(req.Type)))
	if typ != typeIncome && typ != typeExpense {
		httpx.Unprocessable(c, "invalid transactions type", map[string]string{"type": "income|expense"})
		return
	}

	if req.AmountMinor <= 0 {
		httpx.Unprocessable(c, "invalid amount", map[string]string{"amount_minor": "must be > 0"})
		return
	}

	rawCur := strings.TrimSpace(req.Currency)
	cur := strings.ToUpper(rawCur)
	if rawCur != cur || !currencyRe.MatchString(cur) {
		httpx.Unprocessable(c, "invalid currency", map[string]string{
			"currency": "ISO 4217 like UAH, USD (uppercase)",
		})
		return
	}

	occ, err := time.Parse(time.RFC3339, strings.TrimSpace(req.OccurredAt))
	if err != nil {
		httpx.Unprocessable(c, "invalid occurred at", map[string]string{"occurred_at": "RFC3339 timestamp"})
		return
	}

	var catID *string
	if req.CategoryID != nil {
		s := strings.TrimSpace(*req.CategoryID)
		if s != "" {
			if _, err := uuid.Parse(s); err != nil {
				httpx.Unprocessable(c, "invalid category_id", map[string]string{"category_id": "must be uuid"})
				return
			}
			catID = &s
		}
	}

	var note *string
	if req.Note != nil {
		n := strings.TrimSpace(*req.Note)
		if n != "" {
			note = &n
		}
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
		t, err := parseDateOrRFC3339(v)
		if err != nil {
			httpx.Unprocessable(c, "invalid from", map[string]string{"from": "YYYY-MM-DD or RFC3339"})
			return
		}
		f.From = &t
	}
	if v := strings.TrimSpace(c.Query("to")); v != "" {
		t, err := parseDateOrRFC3339(v)
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
		typ := Type(strings.ToLower(v))
		if typ != typeIncome && typ != typeExpense {
			httpx.Unprocessable(c, "invalid transaction type", map[string]string{"type": "income|expense"})
			return
		}
		f.Type = &typ
	}

	if v := strings.TrimSpace(c.Query("category_id")); v != "" {
		if _, err := uuid.Parse(v); err != nil {
			httpx.Unprocessable(c, "invalid category_id", map[string]string{"category_id": "must be uuid"})
			return
		}
		f.CategoryID = &v
	}

	items, err := h.svc.List(c.Request.Context(), workspaceID, f)
	if err != nil {
		httpx.Internal(c)
		log.Printf("transactions.list: %v", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func parseDateOrRFC3339(s string) (time.Time, error) {
	if len(s) == 10 {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			return t, nil
		}
	}
	return time.Parse(time.RFC3339, s)
}

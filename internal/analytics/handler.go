package analytics

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"strconv"

	"github.com/skelbigo/FinanceTracker/internal/httpx"
	"github.com/skelbigo/FinanceTracker/internal/workspaces"
)

type Handler struct {
	svc    *Service
	authMW gin.HandlerFunc
	wsRepo workspaces.RoleProvider
}

func NewHandler(svc *Service, authMW gin.HandlerFunc, wsRepo workspaces.RoleProvider) *Handler {
	return &Handler{svc: svc, authMW: authMW, wsRepo: wsRepo}
}

func (h *Handler) RegisterRoutes(r gin.IRouter) {
	g := r.Group("/workspaces/:id/analytics")
	g.Use(h.authMW)
	g.Use(workspaces.RequireWorkspaceRole(h.wsRepo, workspaces.RoleViewer))

	g.GET("/summary", h.summary)
	g.GET("/by-category", h.byCategory)
	g.GET("/timeseries", h.timeseries)
}

func (h *Handler) summary(c *gin.Context) {
	workspaceID, ok := mustWorkspaceUUID(c)
	if !ok {
		return
	}

	from := c.Query("from")
	to := c.Query("to")
	currency := c.Query("currency")

	resp, err := h.svc.Summary(c.Request.Context(), workspaceID, from, to, currency)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(200, resp)
}

func (h *Handler) byCategory(c *gin.Context) {
	workspaceID, ok := mustWorkspaceUUID(c)
	if !ok {
		return
	}

	from := c.Query("from")
	to := c.Query("to")
	currency := c.Query("currency")
	typ := c.Query("type")

	top := 0
	if topStr := c.Query("top"); topStr != "" {
		v, err := strconv.Atoi(topStr)
		if err != nil {
			writeErr(c, ErrInvalidTop)
			return
		}
		top = v
	}

	resp, err := h.svc.ByCategory(c.Request.Context(), workspaceID, from, to, currency, typ, top)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(200, resp)
}

func (h *Handler) timeseries(c *gin.Context) {
	workspaceID, ok := mustWorkspaceUUID(c)
	if !ok {
		return
	}

	from := c.Query("from")
	to := c.Query("to")
	currency := c.Query("currency")
	bucket := c.Query("bucket")
	typ := c.Query("type")

	resp, err := h.svc.Timeseries(c.Request.Context(), workspaceID, from, to, currency, bucket, typ)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(200, resp)
}

func mustWorkspaceUUID(c *gin.Context) (uuid.UUID, bool) {
	v, ok := c.Get(workspaces.CtxWorkspaceIDKey)
	if !ok {
		httpx.Internal(c)
		return uuid.UUID{}, false
	}
	wsID, _ := v.(string)
	id, err := uuid.Parse(wsID)
	if err != nil {
		httpx.Internal(c)
		return uuid.UUID{}, false
	}
	return id, true
}

func writeErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrInvalidDateRange):
		httpx.BadRequest(c, "invalid date range", map[string]string{"from": "YYYY-MM-DD", "to": "YYYY-MM-DD"})
	case errors.Is(err, ErrInvalidCurrency):
		httpx.BadRequest(c, "invalid currency", map[string]string{"currency": "required, 3-letter code"})
	case errors.Is(err, ErrInvalidType):
		httpx.BadRequest(c, "invalid type", map[string]string{"type": "income|expense"})
	case errors.Is(err, ErrInvalidBucket):
		httpx.BadRequest(c, "invalid bucket", map[string]string{"bucket": "day|week|month"})
	case errors.Is(err, ErrInvalidTop):
		httpx.BadRequest(c, "invalid top", map[string]string{"top": "optional int, 1..100"})
	default:
		httpx.Internal(c)
	}
}

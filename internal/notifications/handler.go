package notifications

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/skelbigo/FinanceTracker/internal/auth"
)

type Handler struct {
	svc    *Service
	authMW gin.HandlerFunc
}

func NewHandler(svc *Service, authMW gin.HandlerFunc) *Handler {
	return &Handler{svc: svc, authMW: authMW}
}

func (h *Handler) RegisterRoutes(r gin.IRouter) {
	g := r.Group("/notifications")
	g.Use(h.authMW)
	g.GET("", h.list)
	g.PATCH("/:id", h.patch)
	g.POST("/:id/read", h.read)
	g.POST("/read-all", h.readAll)

	p := r.Group("/push")
	p.Use(h.authMW)
	p.POST("/subscriptions", h.upsertPushSub)
	p.DELETE("/subscriptions", h.deletePushSub)
}

func userIDFromCtx(c *gin.Context) (string, bool) {
	v, ok := c.Get(auth.CtxUserIDKey)
	if !ok {
		return "", false
	}
	id, ok := v.(string)
	return id, ok
}

type listQuery struct {
	OnlyUnread string `form:"onlyUnread"`
	Limit      string `form:"limit"`
	Cursor     string `form:"cursor"`
}

func (h *Handler) list(c *gin.Context) {
	uid, ok := userIDFromCtx(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var q listQuery
	_ = c.ShouldBindQuery(&q)
	onlyUnread := q.OnlyUnread == "true" || q.OnlyUnread == "1"
	limit := 20
	if q.Limit != "" {
		if v, err := strconv.Atoi(q.Limit); err == nil {
			limit = v
		}
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	cursor := 0
	if q.Cursor != "" {
		if v, err := strconv.Atoi(q.Cursor); err == nil {
			cursor = v
		}
	}
	if cursor < 0 {
		cursor = 0
	}

	res, err := h.svc.ListForUser(c.Request.Context(), uid, onlyUnread, limit, cursor)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	resp := gin.H{
		"notifications": res.Notifications,
		"unreadCount":   res.UnreadCount,
		"unread_count":  res.UnreadCount,
	}
	if res.NextCursor != nil {
		resp["nextCursor"] = *res.NextCursor
		resp["next_cursor"] = *res.NextCursor
	}
	c.JSON(http.StatusOK, resp)
}

type patchBody struct {
	IsRead    *bool `json:"isRead"`
	IsReadAlt *bool `json:"is_read"`
}

func (h *Handler) patch(c *gin.Context) {
	uid, ok := userIDFromCtx(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	id := c.Param("id")
	var b patchBody
	_ = c.ShouldBindJSON(&b)
	val := false
	if b.IsRead != nil {
		val = *b.IsRead
	} else if b.IsReadAlt != nil {
		val = *b.IsReadAlt
	}
	if !val {
		c.JSON(http.StatusBadRequest, gin.H{"error": "isRead must be true"})
		return
	}
	updated, err := h.svc.MarkRead(c.Request.Context(), uid, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if !updated {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) read(c *gin.Context) {
	uid, ok := userIDFromCtx(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	id := c.Param("id")
	updated, err := h.svc.MarkRead(c.Request.Context(), uid, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if !updated {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) readAll(c *gin.Context) {
	uid, ok := userIDFromCtx(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	cnt, err := h.svc.MarkAllRead(c.Request.Context(), uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"updated": cnt})
}

type pushSubBody struct {
	Endpoint string         `json:"endpoint" binding:"required"`
	Keys     map[string]any `json:"keys"`
}

func (h *Handler) upsertPushSub(c *gin.Context) {
	uid, ok := userIDFromCtx(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var b pushSubBody
	if err := c.ShouldBindJSON(&b); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	err := h.svc.SavePushSubscription(c.Request.Context(), uid, b.Endpoint, b.Keys)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) deletePushSub(c *gin.Context) {
	uid, ok := userIDFromCtx(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	endpoint := c.Query("endpoint")
	if endpoint == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "endpoint is required"})
		return
	}
	ok2, err := h.svc.DeletePushSubscription(c.Request.Context(), uid, endpoint)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if !ok2 {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

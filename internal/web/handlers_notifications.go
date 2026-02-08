package web

import (
	"github.com/skelbigo/FinanceTracker/internal/identity"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type notificationVM struct {
	ID        string
	Title     string
	Body      string
	CreatedAt string
	IsRead    bool
	Type      string
	GotoURL   string
	GotoLabel string
}

func (h *Handlers) GetNotificationsPage(c *gin.Context) {
	if h.Notifications == nil {
		c.String(http.StatusInternalServerError, "notifications service is not configured")
		return
	}

	v, ok := c.Get(identity.CtxUserIDKey)
	userID, _ := v.(string)
	if !ok || userID == "" {
		c.Redirect(http.StatusSeeOther, "/login?flash=Please+login")
		return
	}

	onlyUnread := c.Query("onlyUnread") == "true" || c.Query("onlyUnread") == "1"
	limit := 20
	if raw := c.Query("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	cursor := 0
	if raw := c.Query("cursor"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n >= 0 {
			cursor = n
		}
	}

	res, err := h.Notifications.ListForUser(c.Request.Context(), userID, onlyUnread, limit, cursor)
	if err != nil {
		c.String(http.StatusInternalServerError, "could not list notifications")
		return
	}

	items := make([]notificationVM, 0, len(res.Notifications))
	for _, n := range res.Notifications {
		created := n.CreatedAt.Local().Format(time.RFC822)
		gotoLabel := "Open"
		switch n.Type {
		case "new_transaction":
			gotoLabel = "Go to transaction"
		case "overspending":
			gotoLabel = "Go to budget"
		}
		items = append(items, notificationVM{
			ID:        n.ID.String(),
			Title:     n.Title,
			Body:      n.Body,
			CreatedAt: created,
			IsRead:    n.IsRead,
			Type:      string(n.Type),
			GotoURL:   "/app/notifications/" + n.ID.String() + "/goto",
			GotoLabel: gotoLabel,
		})
	}

	nextCursor := 0
	hasNext := false
	if res.NextCursor != nil {
		hasNext = true
		nextCursor = *res.NextCursor
	}

	headerItems, _ := h.Workspaces.ListMyWorkspaces(c.Request.Context(), userID)
	current, _ := c.Cookie(CurrentWorkspaceCookie)

	h.render(c, "app/notifications.html", gin.H{
		"Title":            "Notifications",
		"BodyClass":        "app-dark app-solid",
		"MainClass":        "app-main",
		"Flash":            c.Query("flash"),
		"Notifications":    items,
		"OnlyUnread":       onlyUnread,
		"UnreadCount":      res.UnreadCount,
		"HasNext":          hasNext,
		"NextCursor":       nextCursor,
		"Limit":            limit,
		"Cursor":           cursor,
		"HeaderWorkspaces": headerItems,
		"CurrentID":        current,
		"ReturnTo":         c.Request.URL.Path,
	})
}

func (h *Handlers) PostNotificationRead(c *gin.Context) {
	if h.Notifications == nil {
		c.String(http.StatusInternalServerError, "notifications service is not configured")
		return
	}

	v, ok := c.Get(identity.CtxUserIDKey)
	userID, _ := v.(string)
	if !ok || userID == "" {
		c.Redirect(http.StatusSeeOther, "/login?flash=Please+login")
		return
	}

	nID := c.Param("id")
	_, _ = h.Notifications.MarkRead(c.Request.Context(), userID, nID)

	back := c.Request.Referer()
	if back == "" {
		back = "/app/notifications"
	}
	c.Redirect(http.StatusSeeOther, back)
}

func (h *Handlers) PostNotificationsReadAll(c *gin.Context) {
	if h.Notifications == nil {
		c.String(http.StatusInternalServerError, "notifications service is not configured")
		return
	}

	v, ok := c.Get(identity.CtxUserIDKey)
	userID, _ := v.(string)
	if !ok || userID == "" {
		c.Redirect(http.StatusSeeOther, "/login?flash=Please+login")
		return
	}

	_, _ = h.Notifications.MarkAllRead(c.Request.Context(), userID)
	back := c.Request.Referer()
	if back == "" {
		back = "/app/notifications"
	}
	c.Redirect(http.StatusSeeOther, back)
}

func (h *Handlers) GetNotificationGoto(c *gin.Context) {
	if h.Notifications == nil {
		c.Redirect(http.StatusSeeOther, "/app/notifications")
		return
	}

	v, ok := c.Get(identity.CtxUserIDKey)
	userID, _ := v.(string)
	if !ok || userID == "" {
		c.Redirect(http.StatusSeeOther, "/login?flash=Please+login")
		return
	}

	nID := c.Param("id")
	if nID == "" {
		c.Redirect(http.StatusSeeOther, "/app/notifications")
		return
	}

	n, found, err := h.Notifications.GetForUser(c.Request.Context(), userID, nID)
	if err != nil || !found {
		c.Redirect(http.StatusSeeOther, "/app/notifications")
		return
	}

	if n.WorkspaceID != nil {
		setCurrentWorkspaceCookie(c, h.CookieCfg, n.WorkspaceID.String())
	}

	dest := "/app/notifications"
	switch n.Type {
	case "new_transaction":
		if raw, ok := n.Payload["transactionId"]; ok {
			if txID, ok := raw.(string); ok && txID != "" {
				dest = "/app/transactions"
			} else {
				dest = "/app/transactions"
			}
		} else {
			dest = "/app/transactions"
		}
	case "overspending":
		if raw, ok := n.Payload["budgetId"]; ok {
			if bID, ok := raw.(string); ok && bID != "" {
				dest = "/app/budgets#budget-" + bID
			} else {
				dest = "/app/budgets"
			}
		} else {
			dest = "/app/budgets"
		}
	}

	c.Redirect(http.StatusSeeOther, dest)
}

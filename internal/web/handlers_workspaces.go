package web

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"github.com/skelbigo/FinanceTracker/internal/auth"
)

func (h *Handlers) GetWorkspacesPage(c *gin.Context) {
	if h.Workspaces == nil {
		c.String(http.StatusInternalServerError, "workspaces service is not configured")
		return
	}

	v, ok := c.Get(auth.CtxUserIDKey)
	userID, _ := v.(string)
	if !ok || userID == "" {
		c.Redirect(http.StatusSeeOther, "/login?flash=Please+login")
		return
	}

	items, err := h.Workspaces.ListMyWorkspaces(c.Request.Context(), userID)
	if err != nil {
		c.String(http.StatusInternalServerError, "could not list workspaces")
		return
	}

	current, _ := c.Cookie(CurrentWorkspaceCookie)

	h.render(c, "app/workspaces.html", gin.H{
		"Title":     "Workspaces",
		"BodyClass": "app-dark",
		"Flash":     c.Query("flash"),
		"Items":     items,
		"CurrentID": current,
	})
}

func (h *Handlers) PostCreateWorkspace(c *gin.Context) {
	if h.Workspaces == nil {
		c.String(http.StatusInternalServerError, "workspaces service is not configured")
		return
	}

	v, ok := c.Get(auth.CtxUserIDKey)
	userID, _ := v.(string)
	if !ok || userID == "" {
		c.Redirect(http.StatusSeeOther, "/login?flash=Please+login")
		return
	}

	name := strings.TrimSpace(c.PostForm("name"))
	currency := strings.TrimSpace(c.PostForm("default_currency"))
	if currency == "" {
		currency = "UAH"
	}

	w, _, err := h.Workspaces.CreateWorkspace(c.Request.Context(), userID, name, currency)
	if err != nil {
		flash := "Could not create workspace"
		if errors.Is(err, pgx.ErrNoRows) {
			flash = "Invalid user"
		} else if err.Error() != "" {
			flash = err.Error()
		}
		c.Redirect(http.StatusSeeOther, "/app/workspaces?flash="+urlQueryEscape(flash))
		return
	}

	setCurrentWorkspaceCookie(c, h.CookieCfg, w.ID)
	c.Redirect(http.StatusSeeOther, "/app?flash="+url.QueryEscape("Workspace created"))
}

func urlQueryEscape(s string) string { return url.QueryEscape(s) }

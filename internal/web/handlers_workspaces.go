package web

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"github.com/skelbigo/FinanceTracker/internal/identity"
)

func (h *Handlers) GetWorkspacesPage(c *gin.Context) {
	if h.Workspaces == nil {
		c.String(http.StatusInternalServerError, "workspaces service is not configured")
		return
	}

	v, ok := c.Get(identity.CtxUserIDKey)
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
		"BodyClass": "app-dark app-solid",
		"MainClass": "ws-main",
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

	v, ok := c.Get(identity.CtxUserIDKey)
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
	c.Redirect(http.StatusSeeOther, "/app/workspaces/"+url.PathEscape(w.ID)+"?flash="+url.QueryEscape("Workspace created"))
}

func (h *Handlers) PostSelectWorkspace(c *gin.Context) {
	if h.Workspaces == nil {
		c.String(http.StatusInternalServerError, "workspaces service is not configured")
		return
	}

	v, ok := c.Get(identity.CtxUserIDKey)
	userID, _ := v.(string)
	if !ok || userID == "" {
		c.Redirect(http.StatusSeeOther, "/login?flash=Please+login")
		return
	}

	workspaceID := strings.TrimSpace(c.PostForm("workspace_id"))
	returnTo := safeReturnTo(c.PostForm("return_to"))
	if returnTo == "" {
		returnTo = "/app/workspaces"
	}

	if workspaceID == "" {
		c.Redirect(http.StatusSeeOther, "/app/workspaces?flash="+urlQueryEscape("Select a workspace"))
		return
	}

	_, _, err := h.Workspaces.GetWorkspace(c.Request.Context(), workspaceID, userID)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/app/workspaces?flash="+urlQueryEscape("You are not a member of that workspace"))
		return
	}

	setCurrentWorkspaceCookie(c, h.CookieCfg, workspaceID)
	c.Redirect(http.StatusSeeOther, returnTo)
}

func safeReturnTo(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") {
		return ""
	}
	if !strings.HasPrefix(raw, "/app") {
		return ""
	}
	return raw
}

func urlQueryEscape(s string) string { return url.QueryEscape(s) }

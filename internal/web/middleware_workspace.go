package web

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/skelbigo/FinanceTracker/internal/auth"
	"github.com/skelbigo/FinanceTracker/internal/workspaces"
)

func (h *Handlers) RequireWorkspace() gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.Workspaces == nil {
			c.String(http.StatusInternalServerError, "workspaces service is not configured")
			c.Abort()
			return
		}

		v, ok := c.Get(auth.CtxUserIDKey)
		userID, _ := v.(string)
		if !ok || userID == "" {
			c.Redirect(http.StatusSeeOther, "/login?flash=Please+login")
			c.Abort()
			return
		}

		if wsID, err := c.Cookie(CurrentWorkspaceCookie); err == nil && wsID != "" {
			if _, perr := uuid.Parse(wsID); perr == nil {
				w, role, gerr := h.Workspaces.GetWorkspace(c.Request.Context(), wsID, userID)
				if gerr == nil {
					c.Set(workspaces.CtxWorkspaceIDKey, wsID)
					c.Set(workspaces.CtxWorkspaceRoleKey, role)
					c.Set("workspace", w)
					c.Next()
					return
				}

				if errors.Is(gerr, pgx.ErrNoRows) {
					clearCurrentWorkspaceCookie(c, h.CookieCfg)
				}
			} else {
				clearCurrentWorkspaceCookie(c, h.CookieCfg)
			}
		}

		items, err := h.Workspaces.ListMyWorkspaces(c.Request.Context(), userID)
		if err != nil {
			c.String(http.StatusInternalServerError, "could not list workspaces")
			c.Abort()
			return
		}

		if len(items) == 0 {
			h.render(c, "app/no_workspace.html", gin.H{
				"Title":     "No workspace",
				"BodyClass": "app-dark",
			})
			c.Abort()
			return
		}

		pickedID := items[0].ID
		w, role, gerr := h.Workspaces.GetWorkspace(c.Request.Context(), pickedID, userID)
		if gerr != nil {
			c.String(http.StatusInternalServerError, "could not resolve workspace")
			c.Abort()
			return
		}

		setCurrentWorkspaceCookie(c, h.CookieCfg, pickedID)
		c.Set(workspaces.CtxWorkspaceIDKey, pickedID)
		c.Set(workspaces.CtxWorkspaceRoleKey, role)
		c.Set("workspace", w)

		c.Next()
	}
}

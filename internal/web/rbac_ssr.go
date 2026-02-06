package web

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/skelbigo/FinanceTracker/internal/workspaces"
)

func isHTMX(c *gin.Context) bool {
	return strings.EqualFold(strings.TrimSpace(c.GetHeader("HX-Request")), "true")
}

func workspaceRoleFromContext(c *gin.Context) (workspaces.Role, bool) {
	v, ok := c.Get(workspaces.CtxWorkspaceRoleKey)
	if !ok {
		return "", false
	}
	if r, ok := v.(workspaces.Role); ok {
		return r, true
	}
	if s, ok := v.(string); ok {
		return workspaces.Role(s), true
	}
	return "", false
}

func (h *Handlers) requireWorkspaceRoleForMutation(c *gin.Context, minRole workspaces.Role, htmxErrorPartial string, redirectPath string) bool {
	role, ok := workspaceRoleFromContext(c)
	if ok && workspaces.RoleAtLeast(role, minRole) {
		return true
	}

	msg := "Insufficient permissions"
	if minRole == workspaces.RoleMember {
		msg = "Insufficient permissions (requires member or owner)"
	}
	if minRole == workspaces.RoleOwner {
		msg = "Insufficient permissions (requires owner)"
	}

	if isHTMX(c) {
		c.Status(http.StatusOK)
		if htmxErrorPartial != "" {
			h.renderPartial(c, htmxErrorPartial, gin.H{"Errors": []string{msg}})
			return false
		}
		h.renderPartial(c, "noop", gin.H{})
		return false
	}

	if redirectPath == "" {
		redirectPath = "/app"
	}
	c.Redirect(http.StatusSeeOther, redirectPath+"?flash="+url.QueryEscape(msg))
	return false
}

package web

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/skelbigo/FinanceTracker/internal/workspaces"
)

func (h *Handlers) GetBudgetsPage(c *gin.Context) {
	wsID := c.GetString(workspaces.CtxWorkspaceIDKey)
	if wsID == "" {
		c.String(http.StatusInternalServerError, "workspace not set")
		return
	}

	c.String(http.StatusNotImplemented, "budgets page is not implemented yet")
}

func (h *Handlers) PostCreateBudget(c *gin.Context) {
	wsID := c.GetString(workspaces.CtxWorkspaceIDKey)
	if wsID == "" {
		c.String(http.StatusInternalServerError, "workspace not set")
		return
	}

	c.String(http.StatusNotImplemented, "create budget is not implemented yet")
}

func (h *Handlers) PostUpdateBudget(c *gin.Context) {
	wsID := c.GetString(workspaces.CtxWorkspaceIDKey)
	if wsID == "" {
		c.String(http.StatusInternalServerError, "workspace not set")
		return
	}

	c.String(http.StatusNotImplemented, "update budget is not implemented yet")
}

func (h *Handlers) PostDeleteBudget(c *gin.Context) {
	wsID := c.GetString(workspaces.CtxWorkspaceIDKey)
	if wsID == "" {
		c.String(http.StatusInternalServerError, "workspace not set")
		return
	}

	c.String(http.StatusNotImplemented, "delete budget is not implemented yet")
}

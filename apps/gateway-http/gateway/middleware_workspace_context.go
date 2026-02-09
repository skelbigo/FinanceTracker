package gateway

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/skelbigo/FinanceTracker/apps/gateway-http/workspaces"
	"github.com/skelbigo/FinanceTracker/packages/shared-kernel/httpx"
	"github.com/skelbigo/FinanceTracker/packages/shared-kernel/identity"
)

func WorkspaceContext(repo workspaces.RoleProvider) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := workspaces.GetWorkspaceID(c); ok {
			c.Next()
			return
		}

		workspaceID := extractWorkspaceID(c)
		if workspaceID == "" {
			c.Next()
			return
		}

		if _, err := uuid.Parse(workspaceID); err != nil {
			httpx.BadRequest(c, "invalid workspace id", map[string]string{"id": "must be uuid"})
			c.Abort()
			return
		}

		if repo == nil {
			httpx.Internal(c)
			c.Abort()
			return
		}

		v, exists := c.Get(identity.CtxUserIDKey)
		userID, ok := v.(string)
		if !exists || !ok || userID == "" {
			httpx.Unauthorized(c, "invalid token")
			c.Abort()
			return
		}

		roleStr, err := repo.GetUserRole(c.Request.Context(), workspaceID, userID)
		if err != nil {
			httpx.Internal(c)
			c.Abort()
			return
		}

		if roleStr == "" {
			exists, err := repo.WorkspaceExists(c.Request.Context(), workspaceID)
			if err != nil {
				httpx.Internal(c)
				c.Abort()
				return
			}
			if !exists {
				httpx.Error(c, http.StatusNotFound, "workspace not found", nil)
				c.Abort()
				return
			}

			httpx.Error(c, http.StatusForbidden, "not a workspace member", nil)
			c.Abort()
			return
		}

		actual := workspaces.Role(roleStr)
		switch actual {
		case workspaces.RoleViewer, workspaces.RoleMember, workspaces.RoleOwner:
		default:
			httpx.Internal(c)
			c.Abort()
			return
		}

		c.Set(workspaces.CtxWorkspaceIDKey, workspaceID)
		c.Set(workspaces.CtxWorkspaceRoleKey, actual)
		c.Next()
	}
}

func extractWorkspaceID(c *gin.Context) string {
	workspaceID := c.Param("id")
	if workspaceID == "" {
		workspaceID = c.Param("workspaceId")
	}
	if workspaceID == "" {
		workspaceID = c.GetHeader("X-Workspace-Id")
	}
	if workspaceID == "" {
		workspaceID = c.GetHeader("X-Workspace-ID")
	}
	if workspaceID == "" {
		workspaceID = c.Query("workspace_id")
	}
	return workspaceID
}

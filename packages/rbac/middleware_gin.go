package rbac

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/skelbigo/FinanceTracker/packages/shared-kernel/httpx"
	"github.com/skelbigo/FinanceTracker/packages/shared-kernel/identity"
)

func RequireWorkspaceRole(repo WorkspaceRoleProvider, minRole Role) gin.HandlerFunc {
	return func(c *gin.Context) {
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

		workspaceID, err := ExtractWorkspaceID(c)
		if err != nil {
			if widErr, ok := err.(*WorkspaceIDError); ok {
				httpx.Error(c, widErr.HTTPCode, widErr.Message, widErr.Details)
			} else {
				httpx.Error(c, http.StatusBadRequest, "invalid workspace id", nil)
			}
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

			httpx.Error(c, http.StatusForbidden, "not a workspace member", map[string]string{
				"required": string(minRole),
			})
			c.Abort()
			return
		}

		actual := Role(roleStr)
		if !IsValid(actual) {
			httpx.Internal(c)
			c.Abort()
			return
		}

		if !AtLeast(actual, minRole) {
			httpx.Error(c, http.StatusForbidden, "insufficient role", map[string]string{
				"required": string(minRole),
				"actual":   roleStr,
			})
			c.Abort()
			return
		}

		c.Set(CtxWorkspaceIDKey, workspaceID)
		c.Set(CtxWorkspaceRoleKey, actual)
		c.Next()
	}
}

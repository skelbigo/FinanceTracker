package workspaces

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/skelbigo/FinanceTracker/internal/auth"
	"github.com/skelbigo/FinanceTracker/internal/httpx"
)

const (
	CtxWorkspaceIDKey   = "workspace_id"
	CtxWorkspaceRoleKey = "workspace_role"
)

func AccessRequired(repo *Repo) gin.HandlerFunc {
	return func(c *gin.Context) {
		v, ok := c.Get(auth.CtxUserIDKey)
		userID, ok := v.(string)
		if !ok || userID == "" {
			httpx.Unauthorized(c, "invalid token")
			c.Abort()
			return
		}

		wsStr := c.Param("id")
		if _, err := uuid.Parse(wsStr); err != nil {
			httpx.BadRequest(c, "invalid workspace id", map[string]string{"id": "must be uuid"})
			c.Abort()
			return
		}

		role, err := repo.GetUserRole(c.Request.Context(), wsStr, userID)
		if err != nil {
			httpx.Internal(c)
			c.Abort()
			return
		}
		if role == "" {
			httpx.Error(c, http.StatusForbidden, "not a workspace member", nil)
			c.Abort()
			return
		}

		c.Set(CtxWorkspaceIDKey, wsStr)
		c.Set(CtxWorkspaceRoleKey, role)
		c.Next()
	}
}

func RequireRole(allowed ...Role) gin.HandlerFunc {
	allowedSet := map[Role]struct{}{}
	for _, r := range allowed {
		allowedSet[r] = struct{}{}
	}

	return func(c *gin.Context) {
		v, ok := c.Get(CtxWorkspaceRoleKey)
		roleStr, _ := v.(string)
		if !ok || roleStr == "" {
			httpx.Internal(c)
			c.Abort()
			return
		}

		role := Role(roleStr)
		if _, ok := allowedSet[role]; !ok {
			httpx.Error(c, http.StatusForbidden, "insufficient role", map[string]string{"role": roleStr})
			c.Abort()
			return
		}
		c.Next()
	}
}

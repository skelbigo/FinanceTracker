package workspaces

import (
	"context"
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

type RoleProvider interface {
	GetUserRole(ctx context.Context, workspaceID, userID string) (string, error)
}

func roleRank(r Role) int {
	switch r {
	case RoleViewer:
		return 1
	case RoleMember:
		return 2
	case RoleOwner:
		return 3
	default:
		return 0
	}
}

func AccessRequired(repo RoleProvider) gin.HandlerFunc {
	return func(c *gin.Context) {
		v, exists := c.Get(auth.CtxUserIDKey)
		userID, ok := v.(string)
		if !exists || !ok || userID == "" {
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

func RequireMinRole(min Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		v, exists := c.Get(CtxWorkspaceRoleKey)
		roleStr, _ := v.(string)
		if !exists || roleStr == "" {
			httpx.Internal(c)
			c.Abort()
			return
		}

		actual := Role(roleStr)
		if roleRank(actual) < roleRank(min) {
			httpx.Error(c, http.StatusForbidden, "insufficient role", map[string]string{
				"required": string(min),
				"actual":   roleStr})
			c.Abort()
			return
		}
		c.Next()
	}
}

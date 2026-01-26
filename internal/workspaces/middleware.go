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
	WorkspaceExists(ctx context.Context, workspaceID string) (bool, error)
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

func RoleAtLeast(actual, required Role) bool {
	return roleRank(actual) >= roleRank(required)
}

func GetWorkspaceRole(c *gin.Context) (Role, bool) {
	v, ok := c.Get(CtxWorkspaceRoleKey)
	if !ok {
		return "", false
	}
	r, ok := v.(Role)
	return r, ok
}

func GetWorkspaceID(c *gin.Context) (string, bool) {
	v, ok := c.Get(CtxWorkspaceIDKey)
	if !ok {
		return "", false
	}
	id, ok := v.(string)
	return id, ok
}

func RequireWorkspaceRole(repo RoleProvider, minRole Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		if repo == nil {
			httpx.Internal(c)
			c.Abort()
			return
		}

		v, exists := c.Get(auth.CtxUserIDKey)
		userID, ok := v.(string)
		if !exists || !ok || userID == "" {
			httpx.Unauthorized(c, "invalid token")
			c.Abort()
			return
		}

		workspaceID := c.Param("id")
		if workspaceID == "" {
			workspaceID = c.Param("workspaceId")
		}
		if workspaceID == "" {
			httpx.BadRequest(c, "invalid workspace id", map[string]string{"id": "required"})
			c.Abort()
			return
		}

		if _, err := uuid.Parse(workspaceID); err != nil {
			httpx.BadRequest(c, "invalid workspace id", map[string]string{"id": "must be uuid"})
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
		switch actual {
		case RoleViewer, RoleMember, RoleOwner:
			// valid role from DB
		default:
			httpx.Internal(c)
			c.Abort()
			return
		}

		if !RoleAtLeast(actual, minRole) {
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

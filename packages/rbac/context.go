package rbac

import "github.com/gin-gonic/gin"

const (
	CtxWorkspaceIDKey   = "workspace_id"
	CtxWorkspaceRoleKey = "workspace_role"
)

func GetWorkspaceID(c *gin.Context) (string, bool) {
	v, ok := c.Get(CtxWorkspaceIDKey)
	if !ok {
		return "", false
	}
	id, ok := v.(string)
	return id, ok
}

func GetWorkspaceRole(c *gin.Context) (Role, bool) {
	v, ok := c.Get(CtxWorkspaceRoleKey)
	if !ok {
		return "", false
	}
	r, ok := v.(Role)
	return r, ok
}

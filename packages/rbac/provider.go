package rbac

import "context"

type WorkspaceRoleProvider interface {
	GetUserRole(ctx context.Context, workspaceID, userID string) (string, error)

	WorkspaceExists(ctx context.Context, workspaceID string) (bool, error)
}

package rbac

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type WorkspaceIDError struct {
	Message  string
	Details  map[string]string
	HTTPCode int
}

func (e *WorkspaceIDError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func ExtractWorkspaceID(c *gin.Context) (string, error) {
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

	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return "", &WorkspaceIDError{
			Message:  "invalid workspace id",
			Details:  map[string]string{"id": "required"},
			HTTPCode: http.StatusBadRequest,
		}
	}

	if _, err := uuid.Parse(workspaceID); err != nil {
		return "", &WorkspaceIDError{
			Message:  "invalid workspace id",
			Details:  map[string]string{"id": "must be uuid"},
			HTTPCode: http.StatusBadRequest,
		}
	}

	return workspaceID, nil
}

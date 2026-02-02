package budgets

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/skelbigo/FinanceTracker/internal/httpx"
	"github.com/skelbigo/FinanceTracker/internal/workspaces"
)

func parseWorkspaceUUID(c *gin.Context) (uuid.UUID, bool) {
	if v, ok := c.Get(workspaces.CtxWorkspaceIDKey); ok {
		if s, ok := v.(string); ok && s != "" {
			id, err := uuid.Parse(s)
			if err != nil {
				httpx.BadRequest(c, "invalid workspace id", map[string]string{"id": "must be uuid"})
				return uuid.UUID{}, false
			}
			return id, true
		}
	}

	raw := c.Param("workspaceId")
	if raw == "" {
		raw = c.Param("id")
	}
	if raw == "" {
		httpx.BadRequest(c, "missing workspace id", map[string]string{"id": "required"})
		return uuid.UUID{}, false
	}

	id, err := uuid.Parse(raw)
	if err != nil {
		httpx.BadRequest(c, "invalid workspace id", map[string]string{"id": "must be uuid"})
		return uuid.UUID{}, false
	}

	return id, true
}

func parseOptionalPeriodQuery(c *gin.Context) (*Period, bool) {
	raw := strings.TrimSpace(strings.ToLower(c.Query("period")))
	if raw == "" {
		return nil, true
	}
	p := Period(raw)
	if !p.IsValid() {
		httpx.BadRequest(c, "invalid query params", map[string]string{"period": "must be week or month"})
		return nil, false
	}
	return &p, true
}

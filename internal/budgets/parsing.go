package budgets

import (
	"strconv"

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

func parseYearMonthQuery(c *gin.Context) (int, int, bool) {
	yearStr := c.Query("year")
	monthStr := c.Query("month")

	if yearStr == "" || monthStr == "" {
		httpx.BadRequest(c, "missing query params", map[string]string{
			"year":  "required",
			"month": "required",
		})
		return 0, 0, false
	}

	year, err := strconv.Atoi(yearStr)
	if err != nil {
		httpx.BadRequest(c, "invalid query params", map[string]string{"year": "must be int"})
		return 0, 0, false
	}

	month, err := strconv.Atoi(monthStr)
	if err != nil {
		httpx.BadRequest(c, "invalid query params", map[string]string{"month": "must be int"})
		return 0, 0, false
	}

	return year, month, true
}

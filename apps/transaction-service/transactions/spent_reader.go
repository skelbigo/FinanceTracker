package transactions

import (
	"context"
	"time"

	"github.com/google/uuid"
)

func (r *Repo) GetSpentForCategory(ctx context.Context, workspaceID, categoryID uuid.UUID, currency string, from, to time.Time) (int64, error) {
	const q = `
SELECT COALESCE(SUM(amount_minor), 0)
FROM transactions
WHERE workspace_id = $1
  AND category_id = $2
  AND currency = $3
  AND type = 'expense'
  AND occurred_at >= $4
  AND occurred_at <= $5;
`
	var sum int64
	err := r.pool.QueryRow(ctx, q, workspaceID, categoryID, currency, from, to).Scan(&sum)
	if err != nil {
		return 0, err
	}
	return sum, nil
}

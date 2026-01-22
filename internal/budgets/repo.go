package budgets

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repo struct {
	db *pgxpool.Pool
}

func NewRepo(db *pgxpool.Pool) *Repo {
	return &Repo{db: db}
}

func (r *Repo) Upsert(ctx context.Context, workspaceID uuid.UUID, req UpsertBudgetRequest) (Budget, error) {
	const q = `
INSERT INTO budgets (workspace_id, category_id, year, month, amount)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (workspace_id, category_id, year, month)
DO UPDATE SET amount = EXCLUDED.amount, updated_at = now()
RETURNING id, workspace_id, category_id, year, month, amount, created_at, updated_at;
`

	var b Budget
	err := r.db.QueryRow(ctx, q, workspaceID, req.CategoryID, req.Year, req.Month, req.Amount).
		Scan(&b.ID, &b.WorkspaceID, &b.CategoryID, &b.Year, &b.Month, &b.Amount, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		return Budget{}, err
	}

	return b, nil
}

func (r *Repo) ListWithStats(ctx context.Context, workspaceID uuid.UUID, year int, month int) ([]BudgetResponse, error) {
	start, end := monthRangeUTC(year, month)

	const q = `
SELECT b.id, b.workspace_id, b.category_id, b.year, b.month, b.amount, b.created_at, b.updated_at,
  COALESCE(SUM(t.amount_minor), 0)::bigint AS spent
FROM budgets b
LEFT JOIN transactions t
  ON t.workspace_id = b.workspace_id
 AND t.category_id  = b.category_id
 AND t.occurred_at >= $2
 AND t.occurred_at <  $3
 AND t.type = 'expense'
WHERE b.workspace_id = $1
  AND b.year  = $4
  AND b.month = $5
GROUP BY b.id
ORDER BY b.category_id;
`
	rows, err := r.db.Query(ctx, q, workspaceID, start, end, year, month)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]BudgetResponse, 0)

	for rows.Next() {
		var b Budget
		var spent int64

		if err := rows.Scan(&b.ID, &b.WorkspaceID, &b.CategoryID, &b.Year, &b.Month, &b.Amount, &b.CreatedAt,
			&b.UpdatedAt, &spent); err != nil {
			return nil, err
		}

		out = append(out, NewBudgetResponse(b, spent))
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

func monthRangeUTC(year int, month int) (time.Time, time.Time) {
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	return start, end
}

var _ = pgx.ErrNoRows

package budgets

import (
	"context"

	"github.com/google/uuid"
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
INSERT INTO budgets (workspace_id, category_id, period, amount_limit_minor, currency)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (workspace_id, category_id, period, currency)
DO UPDATE SET amount_limit_minor = EXCLUDED.amount_limit_minor, updated_at = now()
RETURNING id, workspace_id, category_id, period, amount_limit_minor, currency, created_at, updated_at;
`

	var b Budget
	err := r.db.QueryRow(ctx, q, workspaceID, req.CategoryID, req.Period, req.AmountLimitMinor, req.Currency).
		Scan(&b.ID, &b.WorkspaceID, &b.CategoryID, &b.Period, &b.AmountLimitMinor, &b.Currency, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		return Budget{}, err
	}
	return b, nil
}

func (r *Repo) List(ctx context.Context, workspaceID uuid.UUID, period *Period) ([]Budget, error) {
	q := `
SELECT id, workspace_id, category_id, period, amount_limit_minor, currency, created_at, updated_at
FROM budgets
WHERE workspace_id = $1
`
	args := []any{workspaceID}
	if period != nil {
		q += "  AND period = $2\n"
		args = append(args, *period)
	}
	q += "ORDER BY created_at DESC;\n"

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Budget, 0)
	for rows.Next() {
		var b Budget
		if err := rows.Scan(&b.ID, &b.WorkspaceID, &b.CategoryID, &b.Period, &b.AmountLimitMinor, &b.Currency, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

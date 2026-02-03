package budgets

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23505":
				// unique constraint violation
				return Budget{}, ErrBudgetExists
			}
		}
		return Budget{}, err
	}
	return b, nil
}

func (r *Repo) UpdateBudget(ctx context.Context, workspaceID, budgetID uuid.UUID, req UpsertBudgetRequest) (Budget, error) {
	const q = `
UPDATE budgets
SET category_id = $3,
    period = $4,
    amount_limit_minor = $5,
    currency = $6,
    updated_at = now()
WHERE workspace_id = $1 AND id = $2
RETURNING id, workspace_id, category_id, period, amount_limit_minor, currency, created_at, updated_at;
`
	var b Budget
	err := r.db.QueryRow(ctx, q, workspaceID, budgetID, req.CategoryID, req.Period, req.AmountLimitMinor, req.Currency).
		Scan(&b.ID, &b.WorkspaceID, &b.CategoryID, &b.Period, &b.AmountLimitMinor, &b.Currency, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Budget{}, ErrBudgetNotFound
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23505":
				return Budget{}, ErrBudgetExists
			}
		}
		return Budget{}, err
	}
	return b, nil
}

func (r *Repo) DeleteBudget(ctx context.Context, workspaceID, budgetID uuid.UUID) error {
	const q = `
DELETE FROM budgets
WHERE workspace_id = $1 AND id = $2;
`
	ct, err := r.db.Exec(ctx, q, workspaceID, budgetID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrBudgetNotFound
	}
	return nil
}

func (r *Repo) GetBudgetByID(ctx context.Context, workspaceID, budgetID uuid.UUID) (Budget, error) {
	const q = `
SELECT id, workspace_id, category_id, period, amount_limit_minor, currency, created_at, updated_at
FROM budgets
WHERE workspace_id = $1 AND id = $2;
`
	var b Budget
	err := r.db.QueryRow(ctx, q, workspaceID, budgetID).
		Scan(&b.ID, &b.WorkspaceID, &b.CategoryID, &b.Period, &b.AmountLimitMinor, &b.Currency, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Budget{}, ErrBudgetNotFound
		}
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

func (r *Repo) ListByCategory(ctx context.Context, workspaceID, categoryID uuid.UUID, currency string) ([]Budget, error) {
	const q = `
SELECT id, workspace_id, category_id, period, amount_limit_minor, currency, created_at, updated_at
FROM budgets
WHERE workspace_id = $1
  AND category_id = $2
  AND currency = $3
ORDER BY created_at DESC;
`
	rows, err := r.db.Query(ctx, q, workspaceID, categoryID, currency)
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

func (r *Repo) GetSpentForCategory(ctx context.Context, workspaceID, categoryID uuid.UUID, currency string, from, to time.Time) (int64, error) {
	const q = `
SELECT COALESCE(SUM(amount_minor), 0)
FROM transactions
WHERE workspace_id=$1
	AND type='expense'
	AND category_id=$2
	AND currency=$3
	AND occurred_at >= $4
	AND occurred_at <  $5;
`
	var spent int64
	err := r.db.QueryRow(ctx, q, workspaceID, categoryID, currency, from, to).Scan(&spent)
	if err != nil {
		return 0, err
	}
	return spent, nil
}

func (r *Repo) InsertBudgetEvent(ctx context.Context, ev BudgetEvent) error {
	const q = `
INSERT INTO budget_events (workspace_id, budget_id, category_id, period_start, period_end, spent_minor, limit_minor, currency)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (budget_id, period_start, period_end) DO NOTHING;
`
	_, err := r.db.Exec(ctx, q,
		ev.WorkspaceID,
		ev.BudgetID,
		ev.CategoryID,
		ev.PeriodStart,
		ev.PeriodEnd,
		ev.SpentMinor,
		ev.LimitMinor,
		ev.Currency,
	)
	return err
}

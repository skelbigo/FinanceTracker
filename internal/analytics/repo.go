package analytics

import (
	"context"
	"github.com/jackc/pgx/v5"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	Summary(ctx context.Context, workspaceID uuid.UUID, fromInclusive, toExclusive time.Time, currency string) (Summary, error)
	ByCategory(ctx context.Context, workspaceID uuid.UUID, fromInclusive, toExclusive time.Time, currency string, typ TxType, top int) ([]CategoryTotalRow, int64, error)
	Timeseries(ctx context.Context, workspaceID uuid.UUID, fromInclusive, toExclusive time.Time, currency string, bucket Bucket, typ TxType) ([]TimeseriesRow, error)
}

type Repo struct {
	db *pgxpool.Pool
}

func NewRepo(db *pgxpool.Pool) *Repo { return &Repo{db: db} }

func (r *Repo) Summary(ctx context.Context, workspaceID uuid.UUID, fromInclusive, toExclusive time.Time, currency string) (Summary, error) {
	const q = `
SELECT
	COALESCE(SUM(CASE WHEN t.type = 'income'  THEN t.amount_minor ELSE 0 END), 0) AS income_total,
	COALESCE(SUM(CASE WHEN t.type = 'expense' THEN t.amount_minor ELSE 0 END), 0) AS expense_total
FROM transactions t
WHERE t.workspace_id = $1
  AND t.currency     = $2
  AND t.occurred_at >= $3
  AND t.occurred_at <  $4;
`
	var income, expense int64
	if err := r.db.QueryRow(ctx, q, workspaceID, currency, fromInclusive, toExclusive).Scan(&income, &expense); err != nil {
		return Summary{}, err
	}
	return Summary{
		IncomeTotal:  income,
		ExpenseTotal: expense,
		Net:          income - expense,
	}, nil
}

func (r *Repo) ByCategory(ctx context.Context, workspaceID uuid.UUID, fromInclusive, toExclusive time.Time, currency string, typ TxType, top int) ([]CategoryTotalRow, int64, error) {
	const totalQ = `
SELECT COALESCE(SUM(t.amount_minor), 0) AS total
FROM transactions t
WHERE t.workspace_id = $1
  AND t.currency     = $2
  AND t.occurred_at >= $3
  AND t.occurred_at <  $4
  AND t.type         = $5;
`
	var grandTotal int64
	if err := r.db.QueryRow(ctx, totalQ, workspaceID, currency, fromInclusive, toExclusive, string(typ)).Scan(&grandTotal); err != nil {
		return nil, 0, err
	}

	const qNoLimit = `
SELECT
    t.category_id,
    COALESCE(c.name, 'Uncategorized') AS name,
    COALESCE(SUM(t.amount_minor), 0) AS total,
    COUNT(*) AS cnt
FROM transactions t
LEFT JOIN categories c
  ON c.id = t.category_id AND c.workspace_id = t.workspace_id
WHERE t.workspace_id = $1
  AND t.currency     = $2
  AND t.occurred_at >= $3
  AND t.occurred_at <  $4
  AND t.type         = $5
GROUP BY t.category_id, name
ORDER BY total DESC, name ASC;
`

	const qWithLimit = `
SELECT
    t.category_id,
    COALESCE(c.name, 'Uncategorized') AS name,
    COALESCE(SUM(t.amount_minor), 0) AS total,
    COUNT(*) AS cnt
FROM transactions t
LEFT JOIN categories c
  ON c.id = t.category_id AND c.workspace_id = t.workspace_id
WHERE t.workspace_id = $1
  AND t.currency     = $2
  AND t.occurred_at >= $3
  AND t.occurred_at <  $4
  AND t.type         = $5
GROUP BY t.category_id, name
ORDER BY total DESC, name ASC
LIMIT $6;
`

	var (
		rows pgx.Rows
		err  error
	)

	if top > 0 {
		rows, err = r.db.Query(ctx, qWithLimit, workspaceID, currency, fromInclusive, toExclusive, string(typ), top)
	} else {
		rows, err = r.db.Query(ctx, qNoLimit, workspaceID, currency, fromInclusive, toExclusive, string(typ))
	}
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []CategoryTotalRow
	for rows.Next() {
		var row CategoryTotalRow
		var cid *uuid.UUID
		if err := rows.Scan(&cid, &row.Name, &row.Total, &row.Count); err != nil {
			return nil, 0, err
		}
		row.CategoryID = cid
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return out, grandTotal, nil
}

func (r *Repo) Timeseries(ctx context.Context, workspaceID uuid.UUID, fromInclusive, toExclusive time.Time, currency string, bucket Bucket, typ TxType) ([]TimeseriesRow, error) {
	const q = `
SELECT
	date_trunc($5, t.occurred_at)::date AS period_start,
	COALESCE(SUM(t.amount_minor), 0) AS total
FROM transactions t
WHERE t.workspace_id = $1
  AND t.currency     = $2
  AND t.occurred_at >= $3
  AND t.occurred_at <  $4
  AND t.type         = $6
GROUP BY period_start
ORDER BY period_start ASC;
`
	rows, err := r.db.Query(ctx, q, workspaceID, currency, fromInclusive, toExclusive, string(bucket), string(typ))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []TimeseriesRow
	for rows.Next() {
		var row TimeseriesRow
		if err := rows.Scan(&row.PeriodStart, &row.Total); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

package transactions

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

type Repo struct {
	pool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

func (r *Repo) Create(ctx context.Context, t Transaction) (Transaction, error) {
	const q = `
INSERT INTO transactions (workspace_id, user_id, category_id, type, amount_minor, currency, occurred_at, note)
VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8)
RETURNING id::text, workspace_id::text, user_id::text, category_id::text, type, amount_minor, currency, occurred_at, note, created_at, updated_at
`
	var out Transaction
	var typ string

	err := r.pool.QueryRow(ctx, q, t.WorkspaceID, t.UserID, t.CategoryID, string(t.Type), t.AmountMinor, t.Currency,
		t.OccurredAt, t.Note).Scan(&out.ID, &out.WorkspaceID, &out.UserID, &out.CategoryID, &typ, &out.AmountMinor,
		&out.Currency, &out.OccurredAt, &out.Note, &out.CreatedAt, &out.UpdatedAt)

	if err != nil {
		return Transaction{}, err
	}
	out.Type = Type(typ)
	return out, nil
}

type ListFilter struct {
	From       *time.Time
	To         *time.Time
	Type       *Type
	CategoryID *string
}

func (r *Repo) List(ctx context.Context, workspaceID string, f ListFilter) ([]Transaction, error) {
	const q = `
SELECT id::text, workspace_id::text, user_id::text, category_id::text, type, amount_minor, currency, occurred_at, note,
created_at, updated_at
FROM transactions
WHERE workspace_id = $1::uuid
AND ($2::timestamptz IS NULL OR occurred_at >= $2::timestamptz)
AND ($3::timestamptz IS NULL OR occurred_at <= $3::timestamptz)
AND ($4::text IS NULL OR type = $4::text)
AND ($5::uuid IS NULL OR category_id = $5::uuid)
ORDER BY occurred_at DESC
LIMIT 200;
`
	var typeStr *string
	if f.Type != nil {
		s := string(*f.Type)
		typeStr = &s
	}

	rows, err := r.pool.Query(ctx, q, workspaceID, f.From, f.To, typeStr, f.CategoryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Transaction
	for rows.Next() {
		var t Transaction
		var typ string
		if err := rows.Scan(&t.ID, &t.WorkspaceID, &t.UserID, &t.CategoryID, &typ, &t.AmountMinor, &t.Currency,
			&t.OccurredAt, &t.Note, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		t.Type = Type(typ)
		out = append(out, t)
	}
	return out, rows.Err()
}

package transactions

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"strings"
	"time"
)

type Repo struct {
	pool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

func (r *Repo) Create(ctx context.Context, t Transaction) (Transaction, error) {
	if t.Tags == nil {
		t.Tags = []string{}
	}
	const q = `
INSERT INTO transactions (workspace_id, user_id, category_id, type, amount_minor, currency, occurred_at, note, tags)
VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7, $8, $9::text[])
RETURNING id::text, workspace_id::text, user_id::text, category_id::text, type, amount_minor, currency, occurred_at, note, tags, created_at, updated_at
`
	var out Transaction
	var typ string

	err := r.pool.QueryRow(ctx, q, t.WorkspaceID, t.UserID, t.CategoryID, string(t.Type), t.AmountMinor, t.Currency,
		t.OccurredAt, t.Note, t.Tags).Scan(&out.ID, &out.WorkspaceID, &out.UserID, &out.CategoryID, &typ, &out.AmountMinor,
		&out.Currency, &out.OccurredAt, &out.Note, &out.Tags, &out.CreatedAt, &out.UpdatedAt)

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
	Search     *string
	Limit      int
	Offset     int
	Sort       string
}

func orderByFromSort(sort string) string {
	switch strings.TrimSpace(sort) {
	case "occurred_at_asc":
		return "occurred_at ASC"
	case "amount_desc":
		return "amount_minor DESC"
	case "amount_asc":
		return "amount_minor ASC"
	case "occurred_at_desc":
		fallthrough
	default:
		return "occurred_at DESC"
	}
}

func (r *Repo) List(ctx context.Context, workspaceID string, f ListFilter) ([]Transaction, error) {
	orderBy := orderByFromSort(f.Sort)

	args := []any{workspaceID}
	argN := 2

	var sb strings.Builder
	sb.WriteString(`
SELECT id::text, workspace_id::text, user_id::text, category_id::text, type, amount_minor, currency, occurred_at, note,
	tags, created_at, updated_at
FROM transactions
WHERE workspace_id = $1::uuid
`)
	if f.From != nil {
		sb.WriteString(fmt.Sprintf("AND occurred_at >= $%d::timestamptz\n", argN))
		args = append(args, *f.From)
		argN++
	}
	if f.To != nil {
		sb.WriteString(fmt.Sprintf("AND occurred_at <= &%d::timestamptz\n", argN))
		args = append(args, *f.To)
		argN++
	}
	if f.Type != nil {
		sb.WriteString(fmt.Sprintf("AND type = &%d::text\n", argN))
		args = append(args, string(*f.Type))
		argN++
	}
	if f.CategoryID != nil {
		sb.WriteString(fmt.Sprintf("AND category_id = $%d::uuid\n", argN))
		args = append(args, *f.CategoryID)
		argN++
	}
	if f.Search != nil {
		q := strings.TrimSpace(*f.Search)
		if q != "" {
			sb.WriteString(fmt.Sprintf("AND (COALESCE(note, '') ILIKE '%%' || '%d' || '%%' OR $%d = ANY(tags))\n", argN, argN))
			args = append(args, q)
			argN++
		}
	}

	if f.Limit <= 0 {
		f.Limit = 50
	}
	if f.Offset < 0 {
		f.Offset = 0
	}

	sb.WriteString("ORDER BY " + orderBy + "\n")
	sb.WriteString(fmt.Sprintf("LIMIT $%d OFFSET $%d;\n", argN, argN+1))
	args = append(args, f.Limit, f.Offset)

	rows, err := r.pool.Query(ctx, sb.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Transaction
	for rows.Next() {
		var t Transaction
		var typ string
		if err := rows.Scan(&t.ID, &t.WorkspaceID, &t.UserID, &t.CategoryID, &typ, &t.AmountMinor, &t.Currency,
			&t.OccurredAt, &t.Note, &t.Tags, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		t.Type = Type(typ)
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *Repo) GetByID(ctx context.Context, workspaceID, txID string) (Transaction, error) {
	const q = `
SELECT id::text, workspace_id::text, user_id::text, category_id::text, type, amount_minor, currency, occurred_at, note, tags,
	created_at, updated_at
FROM transactions
WHERE workspace_id = $1::uuid AND id = $2::uuid
LIMIT 1;
`
	var out Transaction
	var typ string
	err := r.pool.QueryRow(ctx, q, workspaceID, txID).Scan(&out.ID, &out.WorkspaceID, &out.UserID, &out.CategoryID, &typ,
		&out.AmountMinor, &out.Currency, &out.OccurredAt, &out.Note, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return Transaction{}, err
	}
	out.Type = Type(typ)
	return out, nil
}

func (r *Repo) Update(ctx context.Context, t Transaction) (Transaction, error) {
	if t.Tags == nil {
		t.Tags = []string{}
	}
	const q = `
UPDATE transactions
SET category_id=$3::uuid, type=$4, amount_minor=$5, currency=$6, occurred_at=$7, note=$8, tags=$9::text[], updated_at=now()
WHERE workspace_id=$1::uuid AND id=$2::uuid
RETURNING id::text, workspace_id::text, user_id::text, category_id::text, type, amount_minor, currency, occurred_at, note, tags,
    created_at, updated_at;
`
	var out Transaction
	var typ string
	err := r.pool.QueryRow(ctx, q, t.WorkspaceID, t.ID, t.CategoryID, string(t.Type), t.AmountMinor, t.Currency, t.OccurredAt,
		t.Note, t.Tags).Scan(&out.ID, &out.WorkspaceID, &out.UserID, &out.CategoryID, &typ, &out.AmountMinor, &out.Currency,
		&out.OccurredAt, &out.Note, &out.Tags, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return Transaction{}, err
	}
	out.Type = Type(typ)
	return out, nil
}

func (r *Repo) Delete(ctx context.Context, workspaceID, txID string) (bool, error) {
	const q = `
DELETE FROM transactions
WHERE workspace_id = $1::uuid AND id = $2::uuid
`
	ct, err := r.pool.Exec(ctx, q, workspaceID, txID)
	if err != nil {
		return false, err
	}
	return ct.RowsAffected() > 0, nil
}

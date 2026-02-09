package transactions

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
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

const (
	listQueryOccurredAtAsc = `
SELECT id::text, workspace_id::text, user_id::text, category_id::text, type, amount_minor, currency, occurred_at, note, tags,
       created_at, updated_at
FROM transactions
WHERE workspace_id = $1::uuid
  AND ($2::timestamptz IS NULL OR occurred_at >= $2::timestamptz)
  AND ($3::timestamptz IS NULL OR occurred_at <= $3::timestamptz)
  AND ($4::text IS NULL OR type = $4::text)
  AND ($5::uuid IS NULL OR category_id = $5::uuid)
  AND ($6::text IS NULL OR $6 = '' OR (COALESCE(note, '') ILIKE '%' || $6 || '%' OR $6 = ANY(tags)))
ORDER BY occurred_at ASC, id ASC
LIMIT $7 OFFSET $8;
`

	listQueryOccurredAtDesc = `
SELECT id::text, workspace_id::text, user_id::text, category_id::text, type, amount_minor, currency, occurred_at, note, tags,
       created_at, updated_at
FROM transactions
WHERE workspace_id = $1::uuid
  AND ($2::timestamptz IS NULL OR occurred_at >= $2::timestamptz)
  AND ($3::timestamptz IS NULL OR occurred_at <= $3::timestamptz)
  AND ($4::text IS NULL OR type = $4::text)
  AND ($5::uuid IS NULL OR category_id = $5::uuid)
  AND ($6::text IS NULL OR $6 = '' OR (COALESCE(note, '') ILIKE '%' || $6 || '%' OR $6 = ANY(tags)))
ORDER BY occurred_at DESC, id DESC
LIMIT $7 OFFSET $8;
`

	listQueryAmountAsc = `
SELECT id::text, workspace_id::text, user_id::text, category_id::text, type, amount_minor, currency, occurred_at, note, tags,
       created_at, updated_at
FROM transactions
WHERE workspace_id = $1::uuid
  AND ($2::timestamptz IS NULL OR occurred_at >= $2::timestamptz)
  AND ($3::timestamptz IS NULL OR occurred_at <= $3::timestamptz)
  AND ($4::text IS NULL OR type = $4::text)
  AND ($5::uuid IS NULL OR category_id = $5::uuid)
  AND ($6::text IS NULL OR $6 = '' OR (COALESCE(note, '') ILIKE '%' || $6 || '%' OR $6 = ANY(tags)))
ORDER BY amount_minor ASC, id ASC
LIMIT $7 OFFSET $8;
`

	listQueryAmountDesc = `
SELECT id::text, workspace_id::text, user_id::text, category_id::text, type, amount_minor, currency, occurred_at, note, tags,
       created_at, updated_at
FROM transactions
WHERE workspace_id = $1::uuid
  AND ($2::timestamptz IS NULL OR occurred_at >= $2::timestamptz)
  AND ($3::timestamptz IS NULL OR occurred_at <= $3::timestamptz)
  AND ($4::text IS NULL OR type = $4::text)
  AND ($5::uuid IS NULL OR category_id = $5::uuid)
  AND ($6::text IS NULL OR $6 = '' OR (COALESCE(note, '') ILIKE '%' || $6 || '%' OR $6 = ANY(tags)))
ORDER BY amount_minor DESC, id DESC
LIMIT $7 OFFSET $8;
`
)

func listQueryFromSort(sort string) string {
	switch NormalizeSort(sort) {
	case SortOccurredAtAsc:
		return listQueryOccurredAtAsc
	case SortAmountDesc:
		return listQueryAmountDesc
	case SortAmountAsc:
		return listQueryAmountAsc
	case SortOccurredAtDesc, "":
		fallthrough
	default:
		return listQueryOccurredAtDesc
	}
}

func (r *Repo) List(ctx context.Context, workspaceID string, f ListFilter) ([]Transaction, error) {
	q := listQueryFromSort(f.Sort)

	var fromArg any = nil
	if f.From != nil {
		fromArg = *f.From
	}
	var toArg any = nil
	if f.To != nil {
		toArg = *f.To
	}
	var typeArg any = nil
	if f.Type != nil {
		typeArg = string(*f.Type)
	}
	var categoryArg any = nil
	if f.CategoryID != nil {
		categoryArg = *f.CategoryID
	}
	var searchArg any = nil
	if f.Search != nil {
		qq := strings.TrimSpace(*f.Search)
		if qq != "" {
			searchArg = qq
		}
	}

	if f.Limit <= 0 {
		f.Limit = 50
	}
	if f.Limit > 200 {
		f.Limit = 200
	}
	if f.Offset < 0 {
		f.Offset = 0
	}

	rows, err := r.pool.Query(ctx, q,
		workspaceID,
		fromArg,
		toArg,
		typeArg,
		categoryArg,
		searchArg,
		f.Limit,
		f.Offset,
	)
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
		&out.AmountMinor, &out.Currency, &out.OccurredAt, &out.Note, &out.Tags, &out.CreatedAt, &out.UpdatedAt)
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
		t.Note, t.Tags).Scan(&out.ID, &out.WorkspaceID, &out.UserID, &out.CategoryID, &typ, &out.AmountMinor, &out.Currency, &out.OccurredAt,
		&out.Note, &out.Tags, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return Transaction{}, err
	}
	out.Type = Type(typ)
	return out, nil
}

func (r *Repo) Delete(ctx context.Context, workspaceID, txID string) (bool, error) {
	const q = `
DELETE FROM transactions
WHERE workspace_id = $1::uuid AND id = $2::uuid;
`
	ct, err := r.pool.Exec(ctx, q, workspaceID, txID)
	if err != nil {
		return false, err
	}
	return ct.RowsAffected() > 0, nil
}

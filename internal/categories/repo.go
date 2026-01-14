package categories

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"strings"
)

type Repo struct {
	pool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) *Repo {
	return &Repo{pool: pool}
}

func (r *Repo) CreateCategory(ctx context.Context, workspaceID, name string, t Type) (Category, error) {
	name = strings.TrimSpace(name)

	const q = `
INSERT INTO categories (workspace_id, name, type)
VALUES ($1::uuid, $2, $3)
RETURNING id::text, workspace_id::text, name, type, created_at
`
	var c Category
	err := r.pool.QueryRow(ctx, q, workspaceID, name, string(t)).
		Scan(&c.ID, &c.WorkspaceID, &c.Name, &c.Type, &c.CreatedAt)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23505":
				return Category{}, ErrCategoryExists
			case "23514":
				return Category{}, ErrInvalidType
			}
		}
		return Category{}, err
	}
	return c, nil
}

func (r *Repo) ListCategories(ctx context.Context, workspaceID string) ([]Category, error) {
	const q = `
SELECT id::text, workspace_id::text, name, type, created_at
FROM categories
WHERE workspace_id = $1::uuid
ORDER BY created_at ASC
`
	rows, err := r.pool.Query(ctx, q, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Category
	for rows.Next() {
		var c Category
		var t string
		if err := rows.Scan(&c.ID, &c.WorkspaceID, &c.Name, &t, &c.CreatedAt); err != nil {
			return nil, err
		}
		c.Type = Type(t)
		out = append(out, c)
	}
	return out, rows.Err()
}

package categories

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
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

func (r *Repo) UpdateCategory(ctx context.Context, workspaceID, categoryID, name string, t Type) (Category, error) {
	name = strings.TrimSpace(name)
	const q = `
UPDATE categories
SET name=$3, type=$4
WHERE workspace_id=$1::uuid AND id=$2::uuid
RETURNING id::text, workspace_id::text, name, type, created_at
`
	var c Category
	err := r.pool.QueryRow(ctx, q, workspaceID, categoryID, name, string(t)).
		Scan(&c.ID, &c.WorkspaceID, &c.Name, &c.Type, &c.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Category{}, ErrCategoryNotFound
		}
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

func (r *Repo) DeleteCategory(ctx context.Context, workspaceID, categoryID string) error {
	const q = `
DELETE FROM categories
WHERE workspace_id=$1::uuid AND id=$2::uuid
`
	ct, err := r.pool.Exec(ctx, q, workspaceID, categoryID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23503":
				return ErrCategoryInUse
			}
		}
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrCategoryNotFound
	}
	return nil
}

func (r *Repo) DeleteCategoryWithReassign(ctx context.Context, workspaceID, categoryID string, reassignTo *string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if reassignTo != nil {
		v := strings.TrimSpace(*reassignTo)
		if v == "none" {
			const qNull = `
UPDATE transactions
SET category_id=NULL
WHERE workspace_id=$1::uuid AND category_id=$2::uuid
`
			if _, err := tx.Exec(ctx, qNull, workspaceID, categoryID); err != nil {
				return err
			}
		} else {
			const qExists = `SELECT 1 FROM categories WHERE workspace_id=$1::uuid AND id=$2::uuid`
			var one int
			err := tx.QueryRow(ctx, qExists, workspaceID, v).Scan(&one)
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return ErrCategoryNotFound
				}
				return err
			}

			const qMove = `
UPDATE transactions
SET category_id=$3::uuid
WHERE workspace_id=$1::uuid AND category_id=$2::uuid
`
			if _, err := tx.Exec(ctx, qMove, workspaceID, categoryID, v); err != nil {
				return err
			}
		}
	}

	const qDel = `
DELETE FROM categories
WHERE workspace_id=$1::uuid AND id=$2::uuid
`
	ct, err := tx.Exec(ctx, qDel, workspaceID, categoryID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23503":
				return ErrCategoryInUse
			}
		}
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrCategoryNotFound
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}

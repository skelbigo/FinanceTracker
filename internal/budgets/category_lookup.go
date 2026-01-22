package budgets

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CategoryLookupRepo struct {
	db *pgxpool.Pool
}

func NewCategoryLookup(db *pgxpool.Pool) *CategoryLookupRepo {
	return &CategoryLookupRepo{db: db}
}

func (r *CategoryLookupRepo) ExistsInWorkspace(ctx context.Context, workspaceID, categoryID uuid.UUID) (bool, error) {
	const q = `
SELECT 1
FROM categories
WHERE id = $1 AND workspace_id = $2
LIMIT 1;
`
	var one int
	err := r.db.QueryRow(ctx, q, categoryID, workspaceID).Scan(&one)
	if err == nil {
		return true, nil
	}
	if err == pgx.ErrNoRows {
		return false, nil
	}
	return false, err
}

func (r *CategoryLookupRepo) GetType(ctx context.Context, workspaceID, categoryID uuid.UUID) (string, error) {
	const q = `
SELECT type
FROM categories
WHERE id = $1 AND workspace_id = $2;
`
	var typ string
	err := r.db.QueryRow(ctx, q, categoryID, workspaceID).Scan(&typ)
	if err != nil {
		return "", err
	}
	return typ, nil
}

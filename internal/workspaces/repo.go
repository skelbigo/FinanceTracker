package workspaces

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"strings"
)

type Repo struct {
	pool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) *Repo {
	return &Repo{pool: pool}
}

func (r *Repo) CreateWorkspaceWithOwner(ctx context.Context, createdBy, name, defaultCurrency string) (Workspace, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Workspace{}, errors.New("name is required")
	}
	defaultCurrency = strings.TrimSpace(defaultCurrency)
	if defaultCurrency == "" {
		defaultCurrency = "UAH"
	}

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Workspace{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var w Workspace
	err = tx.QueryRow(ctx, `
INSERT INTO workspaces (name, default_currency, created_by)
VALUES ($1, $2, $3::uuid)
RETURNING id::text, name, default_currency, created_by::text, created_at`, name, defaultCurrency, createdBy).Scan(
		&w.ID, &w.Name, &w.DefaultCurrency, &w.CreatedBy, &w.CreatedAt,
	)
	if err != nil {
		return Workspace{}, err
	}

	fmt.Printf("DEBUG owner insert ws=%q user=%q\n", w.ID, createdBy)
	_, err = tx.Exec(ctx, `
insert into workspaces_members (workspace_id, user_id, role)
values ($1::uuid, $2::uuid, 'owner')
`, w.ID, createdBy)
	if err != nil {
		return Workspace{}, fmt.Errorf("insert owner: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return Workspace{}, fmt.Errorf("commit: %w", err)
	}
	return w, nil
}

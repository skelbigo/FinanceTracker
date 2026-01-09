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

func (r *Repo) GetUserRole(ctx context.Context, workspaceID, userID string) (string, error) {
	const q = `
SELECT role
FROM workspaces_members
WHERE workspace_id = $1::uuid
AND user_id = $2::uuid
`
	var role string
	err := r.pool.QueryRow(ctx, q, workspaceID, userID).Scan(&role)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return role, nil
}

func (r *Repo) GetWorkspace(ctx context.Context, workspaceID string) (Workspace, error) {
	const q = `
SELECT id::text, name, default_currency, created_by::text, created_at
FROM workspaces
WHERE id = $1::uuid
`
	var w Workspace
	err := r.pool.QueryRow(ctx, q, workspaceID).Scan(&w.ID, &w.Name, &w.DefaultCurrency, &w.CreatedBy, &w.CreatedAt)
	return w, err
}

func (r *Repo) ListMembers(ctx context.Context, workspaceID string) ([]Member, error) {
	const q = `
SELECT workspace_id::text, user_id::text, role, created_at
FROM workspaces_members
WHERE workspace_id = $1::uuid
ORDER BY created_at ASC
`
	rows, err := r.pool.Query(ctx, q, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Member
	for rows.Next() {
		var m Member
		var role string
		if err := rows.Scan(&m.WorkspaceID, &m.UserID, &role, &m.CreatedAt); err != nil {
			return nil, err
		}
		m.Role = Role(role)
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *Repo) AddMember(ctx context.Context, workspaceID, userID string, role Role) error {
	const q = `
INSERT INTO workspaces_members (workspace_id, user_id, role)
VALUES ($1::uuid, $2::uuid, $3)
`
	_, err := r.pool.Exec(ctx, q, workspaceID, userID, string(role))
	return err
}

func (r *Repo) UpdateMemberRole(ctx context.Context, workspaceID, userID string, role Role) error {
	const q = `
UPDATE workspaces_members
SET role $3
WHERE workspace_id = $1::uuid
AND user_id = $2::uuid
`
	ct, err := r.pool.Exec(ctx, q, workspaceID, userID, string(role))
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *Repo) RemoveMember(ctx context.Context, workspaceID, userID string) error {
	const q = `
DELETE FROM worspaces_members
WHERE workspase_id = $1::uuid
AND user_id = $2::uuid
`
	ct, err := r.pool.Exec(ctx, q, workspaceID, userID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *Repo) EnsureNotLastOwner(ctx context.Context, workspaceID, targetUserID string, newRole *Role, removing bool) error {
	const qOwners = `
SELECT count(*)
FROM workspaces_members
WHERE workspace_id = $1::uuid AND ROLE == 'owner'
`
	var owners int
	if err := r.pool.QueryRow(ctx, qOwners, workspaceID).Scan(&owners); err != nil {
		return err
	}
	if owners <= 1 {
		const qIsOwner = `
SELECT 1 FROM workspaces_members
WHERE workspace_id = $1::uuid AND ROLE = 'owner'
`
		var one int
		err := r.pool.QueryRow(ctx, qIsOwner, workspaceID, targetUserID).Scan(&one)
		if err == nil {
			if removing {
				return fmt.Errorf("cannot remove last owner")
			}
			if newRole != nil && *newRole == RoleOwner {
				return fmt.Errorf("cannot change role of last owner")
			}
		}
	}
	return nil
}

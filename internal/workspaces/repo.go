package workspaces

import (
	"context"
	"errors"
	"fmt"
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

var (
	ErrUserNotFound     = errors.New("user not found")
	ErrAlreadyMember    = errors.New("already a member")
	ErrLastOwner        = errors.New("cannot remove last owner")
	ErrCannotSelfDemote = errors.New("owner cannot slf demote")
)

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
SET role = $3
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
DELETE FROM workspaces_members
WHERE workspace_id = $1::uuid
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

func (r *Repo) ListMyWorkspaces(ctx context.Context, userID string) ([]WorkspaceListItem, error) {
	const q = `
SELECT w.id::text, w.name, wm.role, w.created_at
FROM workspaces w 
JOIN workspaces_members wm ON wm.workspace_id = w.id
WHERE wm.user_id = $1::uuid
ORDER BY w.created_at DESC
`
	rows, err := r.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []WorkspaceListItem
	for rows.Next() {
		var it WorkspaceListItem
		var role string
		if err := rows.Scan(&it.ID, &it.Name, &role, &it.CreatedAt); err != nil {
			return nil, err
		}
		it.Role = Role(role)
		out = append(out, it)
	}
	return out, rows.Err()
}

func (r *Repo) GetWorkspaceWithRole(ctx context.Context, workspaceID, userID string) (Workspace, Role, error) {
	const q = `
SELECT w.id::text, w.name, w.default_currency, w.created_by::text, w.created_at, wm.role
FROM workspaces w
JOIN workspaces_members wm ON wm.workspace_id = w.id
WHERE w.id = $1::uuid
AND wm.user_id = $2::uuid
`
	var w Workspace
	var role string
	err := r.pool.QueryRow(ctx, q, workspaceID, userID).Scan(&w.ID, &w.Name, &w.DefaultCurrency, &w.CreatedBy, &w.CreatedAt, &role)
	if err != nil {
		return Workspace{}, "", err
	}
	return w, Role(role), nil
}

func (r *Repo) ListMembersInfo(ctx context.Context, workspaceID string) ([]MemberInfo, error) {
	const q = `
SELECT wm.user_id::text, u.email, u.name, wm.role, wm.created_at
FROM workspaces_members wm
JOIN users u on u.id = wm.user_id
WHERE wm.workspace_id = $1::uuid
ORDER BY wm.created_at ASC
`
	rows, err := r.pool.Query(ctx, q, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []MemberInfo
	for rows.Next() {
		var m MemberInfo
		var role string
		if err := rows.Scan(&m.UserID, &m.Email, &m.Name, &role, &m.CreatedAt); err != nil {
			return nil, err
		}
		m.Role = Role(role)
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *Repo) FindUserIDByEmail(ctx context.Context, email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	const q = `SELECT id::text FROM users WHERE email = $1`
	var id string
	err := r.pool.QueryRow(ctx, q, email).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrUserNotFound
		}
		return "", err
	}
	return id, nil
}

func (r *Repo) AddMemberByUserID(ctx context.Context, workspaceID, userID string, role Role) error {
	const q = `
INSERT INTO workspaces_members (workspace_id, user_id, role)
VALUES ($1::uuid, $2::uuid, $3)
`
	_, err := r.pool.Exec(ctx, q, workspaceID, userID, string(role))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrAlreadyMember
		}
		return err
	}
	return nil
}

func (r *Repo) UpdateMemberRoleSafe(ctx context.Context, workspaceID, actorUserID, targetUserID string, newRole Role) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var current string
	err = tx.QueryRow(ctx, `
SELECT role FROM workspaces_members
WHERE workspace_id = $1::uuid AND user_id = $2::uuid
`, workspaceID, targetUserID).Scan(&current)
	if err != nil {
		return err
	}

	if actorUserID == targetUserID && Role(current) == RoleOwner && newRole != RoleOwner {
		return ErrCannotSelfDemote
	}

	if Role(current) == RoleOwner && newRole != RoleOwner {
		var owners int
		if err := tx.QueryRow(ctx, `
SELECT count(*) FROM workspaces_members
WHERE workspace_id = $1::uuid AND role = 'owner'
`, workspaceID).Scan(&owners); err != nil {
			return err
		}
		if owners <= 1 {
			return ErrLastOwner
		}
	}

	ct, err := tx.Exec(ctx, `
UPDATE workspaces_members
SET role = $3
WHERE workspace_id = $1::uuid AND user_id = $2::uuid
`, workspaceID, targetUserID, string(newRole))
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return tx.Commit(ctx)
}

func (r *Repo) RemoveMemberSafe(ctx context.Context, workspaceID, actorUserID, targetUserID string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var current string
	err = tx.QueryRow(ctx, `
SELECT role FROM workspaces_members
WHERE workspace_id = $1::uuid AND user_id = $2::uuid
`, workspaceID, targetUserID).Scan(&current)
	if err != nil {
		return err
	}

	if Role(current) == RoleOwner {
		var owners int
		if err := tx.QueryRow(ctx, `
SELECT count(*) FROM workspaces_members
WHERE workspace_id = $1::uuid AND role = 'owner'
`, workspaceID).Scan(&owners); err != nil {
			return err
		}
		if owners <= 1 {
			return ErrLastOwner
		}
	}

	ct, err := tx.Exec(ctx, `
DELETE FROM workspaces_members
WHERE workspace_id = $1::uuid AND user_id = $2::uuid
`, workspaceID, targetUserID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return tx.Commit(ctx)
}

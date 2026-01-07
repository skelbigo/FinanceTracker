package auth

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrEmailTaken = errors.New("email already taken")

type Repo struct {
	db *pgxpool.Pool
}

func NewRepo(db *pgxpool.Pool) *Repo {
	return &Repo{db: db}
}

func (r *Repo) CreateUser(ctx context.Context, email, passwordHash string, name *string) (User, error) {
	const q = `
INSERT INTO users (email, password_hash, name)
VALUES ($1, $2, $3)
RETURNING id::text, email, password_hash, name, created_at
`
	var u User
	err := r.db.QueryRow(ctx, q, email, passwordHash, name).
		Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return User{}, ErrEmailTaken
		}
		return User{}, err
	}
	return u, nil
}

func (r *Repo) InsertRefreshTokenTime(ctx context.Context, userID, tokenHash string, expiresAt interface{}) error {
	const q = `
INSERT INTO refresh_tokens (user_id, token_hash, expires_at, revoked_at)
VALUES ($1::uuid, $2, $3, NULL)
`
	_, err := r.db.Exec(ctx, q, userID, tokenHash, expiresAt)
	return err
}

func (r *Repo) GetUserByEmail(ctx context.Context, email string) (User, error) {
	const q = `
SELECT id::uuid, email, password_hash, name, created_at
FROM users
WHERE email = $1
`
	var u User
	err := r.db.QueryRow(ctx, q, email).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.CreatedAt)
	if err != nil {
		return User{}, err
	}
	return u, nil
}

func (r *Repo) RevokeExpiredRefreshTokens(ctx context.Context, userID string) error {
	const q = `
UPDATE refresh_tokens
SET revoked_at = NOW()
WHERE user_id = $1::uuid
AND revoked_at IS NULL
AND expires_at <= NOW()
`
	_, err := r.db.Exec(ctx, q, userID)
	return err
}

func (r Repo) GetRefreshTokenByHash(ctx context.Context, tokenHash string) (RefreshTokenRow, error) {
	const q = `
SELECT id::uuid, user_id::text, expires_at, revoked_at
FROM refresh_tokens
WHERE token_hash = $1
`
	var row RefreshTokenRow
	err := r.db.QueryRow(ctx, q, tokenHash).Scan(&row.ID, &row.UserID, &row.ExpiresAt, &row.RevokedAt)
	if err != nil {
		return RefreshTokenRow{}, err
	}
	return row, nil
}

func (r *Repo) RevokeRefreshTokenByID(ctx context.Context, id string) error {
	const q = `
UPDATE refresh_tokens
SET revoked_at = NOW()
WHERE id = $1::uuid
AND revoked_at IS NULL
`
	_, err := r.db.Exec(ctx, q, id)
	return err
}

package auth

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/skelbigo/FinanceTracker/packages/shared-kernel/migrator"
)

type jwtStub struct{}

func (jwtStub) GenerateAccessToken(userID string) (string, error) {
	return "access-" + userID, nil
}

func findMigrationsDir(t *testing.T) string {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	dir := wd
	for i := 0; i < 8; i++ {
		cand := filepath.Join(dir, "migrations")
		if st, err := os.Stat(cand); err == nil && st.IsDir() {
			return cand
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("could not locate migrations dir starting from %s", wd)
	return ""
}

func testDBDSN() string {
	dsn := os.Getenv("FT_TEST_DB_URL")
	if dsn == "" {
		dsn = os.Getenv("TEST_DATABASE_URL")
	}
	return dsn
}

func TestAuthSecurity_RegisterHashesPassword(t *testing.T) {
	dsn := testDBDSN()
	if dsn == "" {
		t.Skip("set FT_TEST_DB_URL (or TEST_DATABASE_URL) to run integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	migrationsDir := findMigrationsDir(t)
	if err := migrator.Run(migrationsDir, dsn, "up", io.Discard); err != nil {
		t.Fatalf("migrate up: %v", err)
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	defer pool.Close()

	repo := NewRepo(pool)
	svc := NewService(repo, jwtStub{}, 30*24*time.Hour, time.Hour, true, 10)

	email := "it_" + uuid.NewString()[:8] + "@example.com"
	password := "Str0ngPassw0rd!"

	resp, err := svc.Register(ctx, RegisterRequest{Email: email, Password: password, Name: "IT"}, TokenMeta{UserAgent: "it", IP: "127.0.0.1"})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	userID := resp.User.ID
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1::uuid;`, userID)
	})

	var storedHash string
	if err := pool.QueryRow(ctx, `SELECT password_hash FROM users WHERE id=$1::uuid;`, userID).Scan(&storedHash); err != nil {
		t.Fatalf("select password_hash: %v", err)
	}
	if storedHash == "" {
		t.Fatalf("expected password_hash to be set")
	}
	if storedHash == password {
		t.Fatalf("expected password_hash to not equal plaintext password")
	}
	if !strings.HasPrefix(storedHash, "$2") {
		t.Fatalf("expected bcrypt hash (prefix $2*), got %q", storedHash)
	}
}

func TestAuthSecurity_RefreshRotationRevokesOldToken(t *testing.T) {
	dsn := testDBDSN()
	if dsn == "" {
		t.Skip("set FT_TEST_DB_URL (or TEST_DATABASE_URL) to run integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	migrationsDir := findMigrationsDir(t)
	if err := migrator.Run(migrationsDir, dsn, "up", io.Discard); err != nil {
		t.Fatalf("migrate up: %v", err)
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	defer pool.Close()

	repo := NewRepo(pool)
	svc := NewService(repo, jwtStub{}, 30*24*time.Hour, time.Hour, true, 10)

	email := "it_" + uuid.NewString()[:8] + "@example.com"
	password := "Str0ngPassw0rd!"

	reg, err := svc.Register(ctx, RegisterRequest{Email: email, Password: password}, TokenMeta{UserAgent: "it", IP: "127.0.0.1"})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	userID := reg.User.ID
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1::uuid;`, userID)
	})

	oldPlain := reg.RefreshToken
	oldHash := HashRefreshToken(oldPlain)

	ref, err := svc.Refresh(ctx, RefreshRequest{RefreshToken: oldPlain}, TokenMeta{UserAgent: "it", IP: "127.0.0.1"})
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	newHash := HashRefreshToken(ref.RefreshToken)

	var revokedAt *time.Time
	var replacedBy *string
	if err := pool.QueryRow(ctx,
		`SELECT revoked_at, replaced_by_token_id::text FROM refresh_tokens WHERE token_hash=$1;`,
		oldHash,
	).Scan(&revokedAt, &replacedBy); err != nil {
		t.Fatalf("select old refresh token row: %v", err)
	}
	if revokedAt == nil {
		t.Fatalf("expected old refresh token to be revoked")
	}
	if replacedBy == nil || *replacedBy == "" {
		t.Fatalf("expected old refresh token to reference replacement token")
	}

	var newRevokedAt *time.Time
	if err := pool.QueryRow(ctx,
		`SELECT revoked_at FROM refresh_tokens WHERE token_hash=$1;`,
		newHash,
	).Scan(&newRevokedAt); err != nil {
		t.Fatalf("select new refresh token row: %v", err)
	}
	if newRevokedAt != nil {
		t.Fatalf("expected new refresh token to be active (not revoked)")
	}
}

func TestAuthSecurity_RefreshReuseRevokesAllSessions(t *testing.T) {
	dsn := testDBDSN()
	if dsn == "" {
		t.Skip("set FT_TEST_DB_URL (or TEST_DATABASE_URL) to run integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	migrationsDir := findMigrationsDir(t)
	if err := migrator.Run(migrationsDir, dsn, "up", io.Discard); err != nil {
		t.Fatalf("migrate up: %v", err)
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	defer pool.Close()

	repo := NewRepo(pool)
	svc := NewService(repo, jwtStub{}, 30*24*time.Hour, time.Hour, true, 10)

	email := "it_" + uuid.NewString()[:8] + "@example.com"
	password := "Str0ngPassw0rd!"

	reg, err := svc.Register(ctx, RegisterRequest{Email: email, Password: password}, TokenMeta{UserAgent: "it", IP: "127.0.0.1"})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	userID := reg.User.ID
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1::uuid;`, userID)
	})

	refresh1 := reg.RefreshToken
	ref2, err := svc.Refresh(ctx, RefreshRequest{RefreshToken: refresh1}, TokenMeta{UserAgent: "it", IP: "127.0.0.1"})
	if err != nil {
		t.Fatalf("Refresh #1: %v", err)
	}
	refresh2 := ref2.RefreshToken

	_, err = svc.Login(ctx, LoginRequest{Email: email, Password: password}, TokenMeta{UserAgent: "it", IP: "127.0.0.1"})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	_, err = svc.Refresh(ctx, RefreshRequest{RefreshToken: refresh1}, TokenMeta{UserAgent: "it", IP: "127.0.0.1"})
	if err == nil {
		t.Fatalf("expected refresh reuse to fail")
	}
	if !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("expected ErrInvalidRefreshToken, got %v", err)
	}

	var active int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM refresh_tokens WHERE user_id=$1::uuid AND revoked_at IS NULL AND expires_at > NOW();`,
		userID,
	).Scan(&active); err != nil {
		t.Fatalf("count active refresh tokens: %v", err)
	}
	if active != 0 {
		t.Fatalf("expected all refresh sessions to be revoked after reuse, active=%d", active)
	}

	var refresh2Revoked *time.Time
	if err := pool.QueryRow(ctx,
		`SELECT revoked_at FROM refresh_tokens WHERE token_hash=$1;`,
		HashRefreshToken(refresh2),
	).Scan(&refresh2Revoked); err != nil {
		t.Fatalf("select refresh2 revoked_at: %v", err)
	}
	if refresh2Revoked == nil {
		t.Fatalf("expected refresh2 to be revoked after reuse")
	}
}

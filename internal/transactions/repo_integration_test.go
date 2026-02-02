package transactions

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/skelbigo/FinanceTracker/internal/migrator"
)

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

func TestRepo_CreateListDelete_List(t *testing.T) {
	dsn := os.Getenv("FT_TEST_DB_URL")
	if dsn == "" {
		dsn = os.Getenv("TEST_DATABASE_URL")
	}
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

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("Ping: %v", err)
	}

	repo := NewRepo(pool)

	rnd := uuid.NewString()[:8]
	email := fmt.Sprintf("it_%s@example.com", rnd)
	var userID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users(email, password_hash, name, default_currency) VALUES ($1, $2, $3, $4) RETURNING id::text;`,
		email, "hash", "IT", "UAH",
	).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1::uuid;`, userID)
	})

	var wsID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO workspaces(name, default_currency, created_by) VALUES ($1, $2, $3::uuid) RETURNING id::text;`,
		"IT "+rnd, "UAH", userID,
	).Scan(&wsID); err != nil {
		t.Fatalf("insert workspace: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM workspaces WHERE id=$1::uuid;`, wsID)
	})

	_, _ = pool.Exec(ctx,
		`INSERT INTO workspaces_members(workspace_id, user_id, role) VALUES ($1::uuid, $2::uuid, 'owner') ON CONFLICT DO NOTHING;`,
		wsID, userID,
	)

	note := "Lunch"
	created, err := repo.Create(ctx, Transaction{
		WorkspaceID: wsID,
		UserID:      userID,
		CategoryID:  nil,
		Type:        TypeExpense,
		AmountMinor: 1234,
		Currency:    "UAH",
		OccurredAt:  time.Now().UTC(),
		Note:        &note,
		Tags:        []string{"food", "lunch"},
	})
	if err != nil {
		t.Fatalf("repo.Create: %v", err)
	}
	if created.ID == "" {
		t.Fatalf("expected created transaction to have ID")
	}

	// List without search.
	items, err := repo.List(ctx, wsID, ListFilter{Limit: 25, Offset: 0, Sort: "occurred_at_desc"})
	if err != nil {
		t.Fatalf("repo.List: %v", err)
	}
	found := false
	for _, it := range items {
		if it.ID == created.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected to find created tx in list")
	}

	q := "food"
	items, err = repo.List(ctx, wsID, ListFilter{Limit: 25, Search: &q})
	if err != nil {
		t.Fatalf("repo.List(search): %v", err)
	}
	found = false
	for _, it := range items {
		if it.ID == created.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected to find created tx in list with search")
	}

	deleted, err := repo.Delete(ctx, wsID, created.ID)
	if err != nil {
		t.Fatalf("repo.Delete: %v", err)
	}
	if !deleted {
		t.Fatalf("expected deleted=true")
	}

	items, err = repo.List(ctx, wsID, ListFilter{Limit: 25})
	if err != nil {
		t.Fatalf("repo.List(after delete): %v", err)
	}
	for _, it := range items {
		if it.ID == created.ID {
			t.Fatalf("did not expect to find deleted tx")
		}
	}
}

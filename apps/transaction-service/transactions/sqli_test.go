package transactions

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestTransactionsList_SQLiSearchDoesNotBypassFilters(t *testing.T) {
	dbURL := testDBURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	admin, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}
	defer admin.Close()

	schema := "test_sqli_" + uuid.NewString()
	_, err = admin.Exec(ctx, fmt.Sprintf(`CREATE SCHEMA "%s"`, schema))
	if err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		ctxDrop, cancelDrop := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelDrop()
		_, _ = admin.Exec(ctxDrop, fmt.Sprintf(`DROP SCHEMA "%s" CASCADE`, schema))
	})

	ddl := fmt.Sprintf(`
CREATE TABLE "%s".transactions (
  id uuid PRIMARY KEY,
  workspace_id uuid NOT NULL,
  user_id uuid NOT NULL,
  category_id uuid NOT NULL,
  type text NOT NULL,
  amount_minor bigint NOT NULL,
  currency text NOT NULL,
  occurred_at timestamptz NOT NULL,
  note text NULL,
  tags text[] NOT NULL DEFAULT '{}',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
`, schema)
	if _, err := admin.Exec(ctx, ddl); err != nil {
		t.Fatalf("create transactions table: %v", err)
	}

	workspaceID := uuid.New().String()
	userID := uuid.New().String()
	catID := uuid.New().String()

	note1 := "coffee"
	note2 := "rent"
	now := time.Now().UTC()

	ins := fmt.Sprintf(`
INSERT INTO "%s".transactions
  (id, workspace_id, user_id, category_id, type, amount_minor, currency, occurred_at, note, tags)
VALUES
  ($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5,$6,$7,$8,$9,$10::text[]);
`, schema)

	if _, err := admin.Exec(ctx, ins, uuid.New().String(), workspaceID, userID, catID, string(TypeExpense), int64(100), "USD", now.Add(-2*time.Hour), note1, []string{"food"}); err != nil {
		t.Fatalf("insert tx1: %v", err)
	}
	if _, err := admin.Exec(ctx, ins, uuid.New().String(), workspaceID, userID, catID, string(TypeExpense), int64(200), "USD", now.Add(-1*time.Hour), note2, []string{"home"}); err != nil {
		t.Fatalf("insert tx2: %v", err)
	}

	repoPool := poolWithSearchPath(t, dbURL, schema)
	defer repoPool.Close()

	repo := NewRepo(repoPool)

	all, err := repo.List(ctx, workspaceID, ListFilter{Limit: 50, Offset: 0})
	if err != nil {
		t.Fatalf("list baseline: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 baseline rows, got %d", len(all))
	}

	qCoffee := "coffee"
	resCoffee, err := repo.List(ctx, workspaceID, ListFilter{Search: &qCoffee, Limit: 50, Offset: 0})
	if err != nil {
		t.Fatalf("list search coffee: %v", err)
	}
	if len(resCoffee) != 1 {
		t.Fatalf("expected 1 row for search 'coffee', got %d", len(resCoffee))
	}

	payload := "' OR 1=1 --"
	res, err := repo.List(ctx, workspaceID, ListFilter{Search: &payload, Limit: 50, Offset: 0})
	if err != nil {
		t.Fatalf("list search payload: %v", err)
	}
	if len(res) != 0 {
		t.Fatalf("expected 0 rows for SQLi payload search, got %d", len(res))
	}
}

func testDBURL(t *testing.T) string {
	t.Helper()
	if v := os.Getenv("TEST_DATABASE_URL"); v != "" {
		return v
	}
	if v := os.Getenv("TEST_DB_URL"); v != "" {
		return v
	}
	if v := os.Getenv("DATABASE_URL"); v != "" {
		return v
	}
	t.Skip("set TEST_DATABASE_URL (or TEST_DB_URL) to run integration SQLi tests")
	return ""
}

func poolWithSearchPath(t *testing.T, dbURL, schema string) *pgxpool.Pool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	cfg, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		t.Fatalf("parse test db url: %v", err)
	}
	if cfg.ConnConfig.RuntimeParams == nil {
		cfg.ConnConfig.RuntimeParams = map[string]string{}
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = fmt.Sprintf(`"%s"`, schema)

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("connect test db (search_path): %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("ping test db: %v", err)
	}
	return pool
}

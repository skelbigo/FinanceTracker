//go:build integration

package analytics

import (
	"context"
	"github.com/skelbigo/FinanceTracker/internal/migrator"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
)

func projectRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Clear(filepath.Join(filepath.Dir(file), "..", ".."))
}

func TestAnalytics_CacheAndInvalidation(t *testing.T) {
	dbURL := os.Getenv("TEST_DB_URL")
	if dbURL == "" {
		os.Getenv("DB_URL")
	}
	if dbURL == "" {
		t.Skip("TEST_DB_URL/DB_URL not set")
	}

	redisHost := os.Getenv("TEST_REDIS_HOST")
	if dbURL == "" {
		os.Getenv("REDIS_HOST")
	}
	if dbURL == "" {
		redisHost = "localhost"
	}
	redisPort := 6379
	if p := os.Getenv("TEST_REDIS_PORT"); p != "" {
		if v, err := strconv.Atoi(p); err != nil {
			redisPort = v
		}
	} else if p := os.Getenv("REDIS_PORT"); p != "" {
		if v, err := strconv.Atoi(p); err != nil {
			redisPort = v
		}
	}

	ctx := context.Background()

	migrationsPath := filepath.Join(projectRoot(), "migrations")
	if err := migrator.Run(migrationsPath, dbURL, "up", io.Discard); err != nil {
		t.Fatalf("migrate up: %v", err)
	}

	pool, err := db.NewPostgresPoolFromURL(ctx, dbURL)
	if err != nil {
		t.Fatalf("connect db: %v", err)
	}
	defer pool.Close()

	_, _ = pool.Exec(ctx, "TRUNCATE TABLE transactions, categories, workspaces_members, workspaces, users CASCADE")

	userID := uuid.New()
	wsID := uuid.New()
	foodID := uuid.New()

	// Seed minimal data.
	_, err = pool.Exec(ctx, `INSERT INTO users (id, email, password_hash, name, default_currency)
		VALUES ($1, $2, $3, $4, $5)`, userID, "test@example.com", "x", "Test", "UAH")
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO workspaces (id, name, default_currency, created_by)
		VALUES ($1, $2, $3, $4)`, wsID, "Test WS", "UAH", userID)
	if err != nil {
		t.Fatalf("insert workspace: %v", err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO workspaces_members (workspace_id, user_id, role)
		VALUES ($1, $2, 'owner')`, wsID, userID)
	if err != nil {
		t.Fatalf("insert membership: %v", err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO categories (id, workspace_id, name, type)
		VALUES ($1, $2, $3, $4)`, foodID, wsID, "Food", "expense")
	if err != nil {
		t.Fatalf("insert category: %v", err)
	}

	_, err = pool.Exec(ctx, `INSERT INTO transactions (workspace_id, user_id, category_id, type, amount_minor, currency, occurred_at)
		VALUES ($1, $2, NULL, 'income', 500, 'UAH', $3),
		       ($1, $2, $4,  'expense', 100, 'UAH', $3),
		       ($1, $2, $4,  'expense',  50, 'UAH', $5)`,
		wsID, userID,
		time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC),
		foodID,
		time.Date(2026, 2, 3, 12, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("insert transactions: %v", err)
	}

	rdb := redisx.NewClient(config.Config{RedisEnabled: true, RedisHost: redisHost, RedisPort: redisPort})
	if rdb == nil {
		t.Skip("redis client not available")
	}
	idx := NewCacheIndex(rdb)
	_ = idx.InvalidateWorkspace(ctx, wsID)

	repo := NewRepo(pool)
	svc := NewService(repo, rdb, idx, 30*time.Minute)

	p1, err := svc.AnalyticsJSON(ctx, wsID, "2026-02-01", "2026-02-03", "UAH", "day", 10)
	if err != nil {
		t.Fatalf("AnalyticsJSON #1: %v", err)
	}
	var r1 AnalyticsResponse
	if err := json.Unmarshal(p1, &r1); err != nil {
		t.Fatalf("unmarshal #1: %v", err)
	}
	if r1.Totals.Income != 500 || r1.Totals.Expense != 150 || r1.Totals.Net != 350 {
		t.Fatalf("unexpected totals #1: %+v", r1.Totals)
	}

	_, err = pool.Exec(ctx, `DELETE FROM transactions WHERE workspace_id = $1`, wsID)
	if err != nil {
		t.Fatalf("delete transactions: %v", err)
	}
	p2, err := svc.AnalyticsJSON(ctx, wsID, "2026-02-01", "2026-02-03", "UAH", "day", 10)
	if err != nil {
		t.Fatalf("AnalyticsJSON #2: %v", err)
	}
	if string(p2) != string(p1) {
		t.Fatalf("expected cached payload to match; got different response")
	}

	txRepo := transactions.NewRepo(pool)
	txCatLookup := transactions.NewCategoryLookup(pool)
	txSvc := transactions.NewService(txRepo, nil, txCatLookup).WithAnalyticsCache(idx)
	catStr := foodID.String()
	_, err = txSvc.Create(ctx, transactions.Transaction{
		WorkspaceID: wsID.String(),
		UserID:      userID.String(),
		CategoryID:  &catStr,
		Type:        transactions.TypeExpense,
		AmountMinor: 250,
		Currency:    "UAH",
		OccurredAt:  time.Date(2026, 2, 2, 12, 0, 0, 0, time.UTC),
		Tags:        []string{},
	})
	if err != nil {
		t.Fatalf("create tx: %v", err)
	}

	p3, err := svc.AnalyticsJSON(ctx, wsID, "2026-02-01", "2026-02-03", "UAH", "day", 10)
	if err != nil {
		t.Fatalf("AnalyticsJSON #3: %v", err)
	}
	if string(p3) == string(p1) {
		t.Fatalf("expected recomputed payload after invalidation; got cached payload")
	}
	var r3 AnalyticsResponse
	if err := json.Unmarshal(p3, &r3); err != nil {
		t.Fatalf("unmarshal #3: %v", err)
	}
	if r3.Totals.Income != 0 || r3.Totals.Expense != 250 || r3.Totals.Net != -250 {
		t.Fatalf("unexpected totals #3: %+v", r3.Totals)
	}
}

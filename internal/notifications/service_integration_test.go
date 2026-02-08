//go:build integration

package notifications_test

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/skelbigo/FinanceTracker/internal/db"
	"github.com/skelbigo/FinanceTracker/internal/migrator"
	"github.com/skelbigo/FinanceTracker/internal/notifications"
)

func getIntegrationDBURL(t *testing.T) string {
	t.Helper()
	if v := os.Getenv("FT_TEST_DB_URL"); v != "" {
		return v
	}
	if v := os.Getenv("TEST_DATABASE_URL"); v != "" {
		return v
	}
	if v := os.Getenv("TEST_DB_URL"); v != "" {
		return v
	}
	if v := os.Getenv("DB_URL"); v != "" {
		return v
	}
	return ""
}

func TestIntegration_NotificationService_CreateInApp_PersistsFields(t *testing.T) {
	dbURL := getIntegrationDBURL(t)
	if dbURL == "" {
		t.Skip("set FT_TEST_DB_URL/TEST_DATABASE_URL (or TEST_DB_URL/DB_URL) to run integration tests")
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

	_, _ = pool.Exec(ctx, "TRUNCATE TABLE notification_delivery, push_subscriptions, notifications, budget_events, budgets, transactions, categories, workspaces_members, workspaces, users CASCADE")

	userID := uuid.New()
	wsID := uuid.New()

	_, err = pool.Exec(ctx, `INSERT INTO users (id, email, password_hash, name, default_currency)
		VALUES ($1, $2, $3, $4, $5)`, userID, "svc@example.com", "x", "Svc", "UAH")
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO workspaces (id, name, default_currency, created_by)
		VALUES ($1, $2, $3, $4)`, wsID, "WS", "UAH", userID)
	if err != nil {
		t.Fatalf("insert workspace: %v", err)
	}

	repo := notifications.NewRepo(pool)
	svc := notifications.NewService(repo, nil, nil, nil, nil, notifications.Options{PublicURL: "http://localhost:8080"})

	ws := wsID.String()
	payload := map[string]any{"transactionId": "tx_123", "amount": int64(123)}
	n, err := svc.CreateInApp(ctx, userID.String(), &ws, notifications.TypeNewTransaction, "Hello", "World", payload)
	if err != nil {
		t.Fatalf("CreateInApp: %v", err)
	}

	var gotUserID, gotWSID, gotType, gotTitle, gotBody, payloadJSON string
	var gotIsRead bool
	var gotCreatedAt time.Time
	err = pool.QueryRow(ctx, `SELECT user_id::text, workspace_id::text, type, title, body, payload::text, is_read, created_at
		FROM notifications WHERE id=$1`, n.ID).Scan(
		&gotUserID,
		&gotWSID,
		&gotType,
		&gotTitle,
		&gotBody,
		&payloadJSON,
		&gotIsRead,
		&gotCreatedAt,
	)
	if err != nil {
		t.Fatalf("query inserted notification: %v", err)
	}

	if gotUserID != userID.String() {
		t.Fatalf("user_id: got %s want %s", gotUserID, userID.String())
	}
	if gotWSID != wsID.String() {
		t.Fatalf("workspace_id: got %s want %s", gotWSID, wsID.String())
	}
	if gotType != string(notifications.TypeNewTransaction) {
		t.Fatalf("type: got %s want %s", gotType, notifications.TypeNewTransaction)
	}
	if gotTitle != "Hello" || gotBody != "World" {
		t.Fatalf("title/body mismatch")
	}
	if gotIsRead {
		t.Fatalf("is_read: got true want false")
	}
	if gotCreatedAt.IsZero() {
		t.Fatalf("created_at: expected non-zero")
	}

	var gotPayload map[string]any
	if err := json.Unmarshal([]byte(payloadJSON), &gotPayload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if gotPayload["transactionId"] != "tx_123" {
		t.Fatalf("payload.transactionId: got %v want %v", gotPayload["transactionId"], "tx_123")
	}
	if v, ok := gotPayload["amount"].(float64); !ok || int64(v) != 123 {
		t.Fatalf("payload.amount: got %v want %v", gotPayload["amount"], 123)
	}
}

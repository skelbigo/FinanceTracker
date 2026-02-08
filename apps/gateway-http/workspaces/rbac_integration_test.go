//go:build integration

package workspaces_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/skelbigo/FinanceTracker/apps/auth-service/auth"
	"github.com/skelbigo/FinanceTracker/apps/gateway-http/workspaces"
	"github.com/skelbigo/FinanceTracker/apps/transaction-service/transactions"
	"github.com/skelbigo/FinanceTracker/packages/db"
	"github.com/skelbigo/FinanceTracker/packages/shared-kernel/migrator"
)

func projectRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func testAuthFromHeader() gin.HandlerFunc {
	return func(c *gin.Context) {
		if uid := c.GetHeader("X-Test-User"); uid != "" {
			c.Set(auth.CtxUserIDKey, uid)
		}
		c.Next()
	}
}

func TestIntegration_RBACWorkspaceAccess(t *testing.T) {
	dbURL := os.Getenv("TEST_DB_URL")
	if dbURL == "" {
		dbURL = os.Getenv("DB_URL")
	}
	if dbURL == "" {
		t.Skip("TEST_DB_URL/DB_URL not set")
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

	ownerID := uuid.New()
	viewerID := uuid.New()
	wsID := uuid.New()

	_, err = pool.Exec(ctx, `INSERT INTO users (id, email, password_hash, name, default_currency)
		VALUES ($1, $2, $3, $4, $5),
		       ($6, $7, $8, $9, $10)`,
		ownerID, "owner@example.com", "x", "Owner", "UAH",
		viewerID, "viewer@example.com", "x", "Viewer", "UAH",
	)
	if err != nil {
		t.Fatalf("insert users: %v", err)
	}

	_, err = pool.Exec(ctx, `INSERT INTO workspaces (id, name, default_currency, created_by, created_at)
		VALUES ($1, $2, $3, $4, $5)`,
		wsID, "Test WS", "UAH", ownerID, time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("insert workspace: %v", err)
	}

	_, err = pool.Exec(ctx, `INSERT INTO workspaces_members (workspace_id, user_id, role, created_at)
		VALUES ($1, $2, 'owner',  $4),
		       ($1, $3, 'viewer', $4)`,
		wsID, ownerID, viewerID, time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("insert memberships: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	authMW := testAuthFromHeader()
	r.Use(authMW)

	wsRepo := workspaces.NewRepo(pool)
	wsSvc := workspaces.NewService(wsRepo, nil)
	wsHandler := workspaces.NewHandler(wsSvc, wsRepo)
	wsHandler.RegisterRoutes(r)

	txRepo := transactions.NewRepo(pool)
	txSvc := transactions.NewService(txRepo, nil, nil)
	txHandler := transactions.NewHandler(txSvc, wsRepo)
	txHandler.RegisterRoutes(r)

	createPayload, _ := json.Marshal(map[string]any{
		"type":         "income",
		"amount_minor": 100,
		"currency":     "UAH",
		"occurred_at":  "2026-02-01",
		"tags":         []string{},
	})

	{
		req := httptest.NewRequest(http.MethodPost, "/workspaces/"+wsID.String()+"/transactions", bytes.NewReader(createPayload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Test-User", viewerID.String())
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Fatalf("viewer create tx: expected %d, got %d", http.StatusForbidden, w.Code)
		}
	}

	{
		req := httptest.NewRequest(http.MethodGet, "/workspaces/"+wsID.String()+"/transactions", nil)
		req.Header.Set("X-Test-User", viewerID.String())
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("viewer list tx: expected %d, got %d", http.StatusOK, w.Code)
		}
	}

	{
		payload, _ := json.Marshal(map[string]any{"role": "member"})
		req := httptest.NewRequest(http.MethodPatch, "/workspaces/"+wsID.String()+"/members/"+viewerID.String(), bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Test-User", ownerID.String())
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusNoContent {
			t.Fatalf("owner update role: expected %d, got %d", http.StatusNoContent, w.Code)
		}
	}

	{
		req := httptest.NewRequest(http.MethodPost, "/workspaces/"+wsID.String()+"/transactions", bytes.NewReader(createPayload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Test-User", viewerID.String())
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("member create tx: expected %d, got %d", http.StatusCreated, w.Code)
		}
	}

	{
		req := httptest.NewRequest(http.MethodDelete, "/workspaces/"+wsID.String()+"/members/"+ownerID.String(), nil)
		req.Header.Set("X-Test-User", ownerID.String())
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusConflict {
			t.Fatalf("remove last owner: expected %d, got %d", http.StatusConflict, w.Code)
		}
	}
}

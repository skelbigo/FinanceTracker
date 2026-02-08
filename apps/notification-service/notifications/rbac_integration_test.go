//go:build integration

package notifications_test

import (
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
	"github.com/skelbigo/FinanceTracker/apps/notification-service/notifications"
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

type listResp struct {
	Notifications []struct {
		ID     string `json:"id"`
		UserID string `json:"user_id"`
	} `json:"notifications"`
	UnreadCount int  `json:"unread_count"`
	NextCursor  *int `json:"next_cursor"`
}

func TestIntegration_NotificationsRBAC_PrivateByUserID(t *testing.T) {
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

	_, _ = pool.Exec(ctx, "TRUNCATE TABLE notification_delivery, push_subscriptions, notifications, transactions, budgets, categories, workspaces_members, workspaces, users CASCADE")

	user1 := uuid.New()
	user2 := uuid.New()

	_, err = pool.Exec(ctx, `INSERT INTO users (id, email, password_hash, name, default_currency)
		VALUES ($1, $2, $3, $4, $5),
		       ($6, $7, $8, $9, $10)`,
		user1, "u1@example.com", "x", "User1", "UAH",
		user2, "u2@example.com", "x", "User2", "UAH",
	)
	if err != nil {
		t.Fatalf("insert users: %v", err)
	}

	n1ID := uuid.New()
	n2ID := uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO notifications (id, user_id, workspace_id, type, title, body, payload, is_read, created_at)
		VALUES ($1, $2, NULL, $3, $4, $5, '{}'::jsonb, false, $6),
		       ($7, $8, NULL, $9, $10, $11, '{}'::jsonb, false, $12)`,
		n1ID, user1, "new_transaction", "N1", "Body1", time.Now().UTC(),
		n2ID, user2, "new_transaction", "N2", "Body2", time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("insert notifications: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	authMW := testAuthFromHeader()
	r.Use(authMW)

	repo := notifications.NewRepo(pool)
	svc := notifications.NewService(repo, nil, nil, nil, notifications.NoopPushSender{}, notifications.Options{})
	h := notifications.NewHandler(svc)
	h.RegisterRoutes(r)

	{
		req := httptest.NewRequest(http.MethodGet, "/notifications", nil)
		req.Header.Set("X-Test-User", user1.String())
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("user1 list: expected %d, got %d", http.StatusOK, w.Code)
		}
		var lr listResp
		if err := json.Unmarshal(w.Body.Bytes(), &lr); err != nil {
			t.Fatalf("decode list: %v", err)
		}
		if len(lr.Notifications) != 1 {
			t.Fatalf("user1 list: expected 1 notification, got %d", len(lr.Notifications))
		}
		if lr.Notifications[0].ID != n1ID.String() {
			t.Fatalf("user1 list: expected id=%s, got %s", n1ID.String(), lr.Notifications[0].ID)
		}
	}

	{
		req := httptest.NewRequest(http.MethodPost, "/notifications/"+n1ID.String()+"/read", nil)
		req.Header.Set("X-Test-User", user2.String())
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("user2 mark read user1 notif: expected %d, got %d", http.StatusNotFound, w.Code)
		}
	}

	{
		req := httptest.NewRequest(http.MethodPost, "/notifications/"+n1ID.String()+"/read", nil)
		req.Header.Set("X-Test-User", user1.String())
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("user1 mark read: expected %d, got %d", http.StatusOK, w.Code)
		}
	}
}

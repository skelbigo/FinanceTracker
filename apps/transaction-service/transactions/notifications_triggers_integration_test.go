//go:build integration

package transactions

import (
	"context"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/skelbigo/FinanceTracker/apps/budget-service/budgets"
	"github.com/skelbigo/FinanceTracker/apps/gateway-http/workspaces"
	"github.com/skelbigo/FinanceTracker/apps/notification-service/notifications"
	"github.com/skelbigo/FinanceTracker/packages/contracts/notificationsv1"
	"github.com/skelbigo/FinanceTracker/packages/shared-kernel/migrator"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

func integrationDSN(t *testing.T) string {
	t.Helper()
	if v := os.Getenv("FT_TEST_DB_URL"); v != "" {
		return v
	}
	if v := os.Getenv("TEST_DATABASE_URL"); v != "" {
		return v
	}
	return ""
}

func setupIntegrationDB(t *testing.T) (context.Context, *pgxpool.Pool) {
	t.Helper()
	dsn := integrationDSN(t)
	if dsn == "" {
		t.Skip("set FT_TEST_DB_URL or TEST_DATABASE_URL to run integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	t.Cleanup(cancel)

	migrationsDir := findMigrationsDir(t)
	if err := migrator.Run(migrationsDir, dsn, "up", io.Discard); err != nil {
		t.Fatalf("migrate up: %v", err)
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("Ping: %v", err)
	}

	_, _ = pool.Exec(ctx, "TRUNCATE TABLE notification_delivery, push_subscriptions, notifications, budget_events, budgets, transactions, categories, workspaces_members, workspaces, users CASCADE")

	return ctx, pool
}

func insertUser(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id uuid.UUID, email, name string) {
	t.Helper()
	_, err := pool.Exec(ctx, `INSERT INTO users (id, email, password_hash, name, default_currency) VALUES ($1, $2, $3, $4, $5)`,
		id, email, "x", name, "UAH",
	)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
}

func insertWorkspaceWithMembers(t *testing.T, ctx context.Context, pool *pgxpool.Pool, wsID uuid.UUID, createdBy uuid.UUID, members map[uuid.UUID]workspaces.Role) {
	t.Helper()
	_, err := pool.Exec(ctx, `INSERT INTO workspaces (id, name, default_currency, created_by) VALUES ($1, $2, $3, $4)`,
		wsID, "WS", "UAH", createdBy,
	)
	if err != nil {
		t.Fatalf("insert workspace: %v", err)
	}
	for uid, role := range members {
		_, err := pool.Exec(ctx, `INSERT INTO workspaces_members (workspace_id, user_id, role) VALUES ($1, $2, $3)`,
			wsID, uid, string(role),
		)
		if err != nil {
			t.Fatalf("insert member: %v", err)
		}
	}
}

func TestIntegration_TriggerNewTransaction_NotifiesMembersExceptAuthor(t *testing.T) {
	ctx, pool := setupIntegrationDB(t)

	u1 := uuid.New()
	u2 := uuid.New()
	u3 := uuid.New()
	insertUser(t, ctx, pool, u1, "u1@example.com", "U1")
	insertUser(t, ctx, pool, u2, "u2@example.com", "U2")
	insertUser(t, ctx, pool, u3, "u3@example.com", "U3")

	wsID := uuid.New()
	insertWorkspaceWithMembers(t, ctx, pool, wsID, u1, map[uuid.UUID]workspaces.Role{
		u1: workspaces.RoleOwner,
		u2: workspaces.RoleMember,
		u3: workspaces.RoleViewer,
	})

	wsRepo := workspaces.NewRepo(pool)
	nRepo := notifications.NewRepo(pool)
	nSvc := notifications.NewService(nRepo, wsRepo, nil, nil, nil, notifications.Options{})

	tRepo := NewRepo(pool)
	tSvc := NewService(tRepo, nil, nil).WithNotifications(nSvc)

	occurred := time.Date(2026, time.February, 4, 12, 0, 0, 0, time.UTC)
	created, err := tSvc.Create(ctx, Transaction{
		WorkspaceID: wsID.String(),
		UserID:      u1.String(),
		CategoryID:  nil,
		Type:        TypeExpense,
		AmountMinor: 100,
		Currency:    "UAH",
		OccurredAt:  occurred,
	})
	if err != nil {
		t.Fatalf("Create tx: %v", err)
	}

	rows, err := pool.Query(ctx, `SELECT user_id::text FROM notifications WHERE workspace_id=$1::uuid AND type=$2 ORDER BY user_id`, wsID, string(notifications.TypeNewTransaction))
	if err != nil {
		t.Fatalf("query notifications: %v", err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got = append(got, uid)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 recipients, got %d (%v)", len(got), got)
	}
	if got[0] != u2.String() || got[1] != u3.String() {
		t.Fatalf("unexpected recipients: %v (want %s,%s)", got, u2.String(), u3.String())
	}

	var createdBy, txID string
	if err := pool.QueryRow(ctx, `SELECT payload->>'createdBy', payload->>'transactionId' FROM notifications WHERE user_id=$1::uuid AND type=$2 LIMIT 1`,
		u2, string(notifications.TypeNewTransaction),
	).Scan(&createdBy, &txID); err != nil {
		t.Fatalf("query payload: %v", err)
	}
	if createdBy != u1.String() {
		t.Fatalf("payload.createdBy: got %s want %s", createdBy, u1.String())
	}
	if txID != created.ID {
		t.Fatalf("payload.transactionId: got %s want %s", txID, created.ID)
	}
}

func TestIntegration_TriggerOverspending_NotifiesOnceAndNoSpam(t *testing.T) {
	ctx, pool := setupIntegrationDB(t)

	u1 := uuid.New()
	u2 := uuid.New()
	u3 := uuid.New()
	insertUser(t, ctx, pool, u1, "u1@example.com", "U1")
	insertUser(t, ctx, pool, u2, "u2@example.com", "U2")
	insertUser(t, ctx, pool, u3, "u3@example.com", "U3")

	wsID := uuid.New()
	insertWorkspaceWithMembers(t, ctx, pool, wsID, u1, map[uuid.UUID]workspaces.Role{
		u1: workspaces.RoleOwner,
		u2: workspaces.RoleMember,
		u3: workspaces.RoleViewer,
	})

	catID := uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO categories (id, workspace_id, name, type) VALUES ($1, $2, $3, $4)`,
		catID, wsID, "Food", "expense",
	)
	if err != nil {
		t.Fatalf("insert category: %v", err)
	}

	budgetID := uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO budgets (id, workspace_id, category_id, period, amount_limit_minor, currency) VALUES ($1, $2, $3, $4, $5, $6)`,
		budgetID, wsID, catID, "week", int64(1000), "UAH",
	)
	if err != nil {
		t.Fatalf("insert budget: %v", err)
	}

	wsRepo := workspaces.NewRepo(pool)
	nRepo := notifications.NewRepo(pool)
	nSvc := notifications.NewService(nRepo, wsRepo, nil, nil, nil, notifications.Options{})

	bRepo := budgets.NewRepo(pool)
	tRepo := NewRepo(pool)
	txCatLookup := NewCategoryLookup(pool)
	bSvc := budgets.NewService(bRepo, tRepo, txCatLookup, false)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	grpcAddr := ln.Addr().String()
	grpcMux := http.NewServeMux()
	notifications.RegisterGRPC(grpcMux, nSvc, log.New(io.Discard, "", 0))
	grpcSrv := &http.Server{Handler: h2c.NewHandler(grpcMux, &http2.Server{})}
	go func() { _ = grpcSrv.Serve(ln) }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = grpcSrv.Shutdown(ctx)
	})

	nClient := notificationsv1.NewClient(grpcAddr)
	bSvc.WithNotifications(wsRepo, nClient)

	tSvc := NewService(tRepo, bSvc, txCatLookup).WithNotifications(nSvc)

	occ1 := time.Date(2026, time.February, 4, 12, 0, 0, 0, time.UTC)
	catStr := catID.String()

	_, err = tSvc.Create(ctx, Transaction{
		WorkspaceID: wsID.String(),
		UserID:      u1.String(),
		CategoryID:  &catStr,
		Type:        TypeExpense,
		AmountMinor: 600,
		Currency:    "UAH",
		OccurredAt:  occ1,
	})
	if err != nil {
		t.Fatalf("Create tx#1: %v", err)
	}
	var cnt int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM notifications WHERE type=$1`, string(notifications.TypeOverspending)).Scan(&cnt); err != nil {
		t.Fatalf("count notif: %v", err)
	}
	if cnt != 0 {
		t.Fatalf("expected 0 overspending notifications, got %d", cnt)
	}

	_, err = tSvc.Create(ctx, Transaction{
		WorkspaceID: wsID.String(),
		UserID:      u1.String(),
		CategoryID:  &catStr,
		Type:        TypeExpense,
		AmountMinor: 500,
		Currency:    "UAH",
		OccurredAt:  occ1.Add(1 * time.Hour),
	})
	if err != nil {
		t.Fatalf("Create tx#2: %v", err)
	}
	var u2Cnt int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM notifications WHERE type=$1 AND user_id=$2`, string(notifications.TypeOverspending), u2).Scan(&u2Cnt); err != nil {
		t.Fatalf("count u2 notif: %v", err)
	}
	if u2Cnt != 1 {
		t.Fatalf("expected 1 overspending notif for u2, got %d", u2Cnt)
	}
	var u1Cnt int
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM notifications WHERE type=$1 AND user_id=$2`, string(notifications.TypeOverspending), u1).Scan(&u1Cnt)
	if u1Cnt != 0 {
		t.Fatalf("expected 0 overspending notif for actor u1, got %d", u1Cnt)
	}
	var u3Cnt int
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM notifications WHERE type=$1 AND user_id=$2`, string(notifications.TypeOverspending), u3).Scan(&u3Cnt)
	if u3Cnt != 0 {
		t.Fatalf("expected 0 overspending notif for viewer u3, got %d", u3Cnt)
	}

	_, err = tSvc.Create(ctx, Transaction{
		WorkspaceID: wsID.String(),
		UserID:      u1.String(),
		CategoryID:  &catStr,
		Type:        TypeExpense,
		AmountMinor: 100,
		Currency:    "UAH",
		OccurredAt:  occ1.Add(2 * time.Hour),
	})
	if err != nil {
		t.Fatalf("Create tx#3: %v", err)
	}
	var u2Cnt2 int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM notifications WHERE type=$1 AND user_id=$2`, string(notifications.TypeOverspending), u2).Scan(&u2Cnt2); err != nil {
		t.Fatalf("count u2 notif #2: %v", err)
	}
	if u2Cnt2 != 1 {
		t.Fatalf("expected still 1 overspending notif for u2, got %d", u2Cnt2)
	}

	var evCnt int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM budget_events WHERE workspace_id=$1`, wsID).Scan(&evCnt); err != nil {
		t.Fatalf("count budget_events: %v", err)
	}
	if evCnt != 1 {
		t.Fatalf("expected 1 budget_event (anti-spam), got %d", evCnt)
	}
}

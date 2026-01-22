package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/skelbigo/FinanceTracker/internal/budgets"
	"github.com/skelbigo/FinanceTracker/internal/categories"
	"github.com/skelbigo/FinanceTracker/internal/transactions"
	"github.com/skelbigo/FinanceTracker/internal/workspaces"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"github.com/skelbigo/FinanceTracker/internal/auth"
	"github.com/skelbigo/FinanceTracker/internal/config"
	"github.com/skelbigo/FinanceTracker/internal/db"
	"github.com/skelbigo/FinanceTracker/internal/migrator"
)

func main() {
	startedAt := time.Now()
	_ = godotenv.Load()

	mode := flag.String("mode", "serve", "run mode: serve|migrate")
	cmd := flag.String("cmd", "up", "migrate command (used only with -mode=migrate): up|down|status|... (depends on migrator)")
	migrationsDir := flag.String("migrations", "migrations", "path to migrations directory")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("invalid environment: %v", err)
	}

	switch *mode {
	case "migrate":
		dbURL := cfg.EffectiveDBURL()

		if err := migrator.Run(*migrationsDir, dbURL, *cmd); err != nil {
			log.Fatalf("migrate cmd=%s dir=%s failed: %v", *cmd, *migrationsDir, err)
		}
		log.Printf("migrations done: cmd=%s dir=%s", *cmd, *migrationsDir)

	case "serve":
		serve(cfg, startedAt)

	default:
		flag.Usage()
		log.Fatalf("unknown -mode=%q (use: serve|migrate)", *mode)
	}
}

func serve(cfg config.Config, startedAt time.Time) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dbURL := cfg.EffectiveDBURL()
	pool := mustDBURL(ctx, dbURL)
	defer pool.Close()

	r := setupRouter(cfg, pool, startedAt)

	addr := fmt.Sprintf(":%d", cfg.AppPort)
	log.Printf("Starting FinanceTracker API mode=%s addr=%s", gin.Mode(), addr)
	if err := r.Run(addr); err != nil {
		log.Fatal(err)
	}
}

func mustDBURL(ctx context.Context, dbURL string) *pgxpool.Pool {
	pool, err := db.NewPostgresPoolFromURL(ctx, dbURL)
	if err != nil {
		log.Fatalf("db connection error (%s): %v", dbURL, err)
	}
	return pool
}

func setupRouter(cfg config.Config, pool *pgxpool.Pool, startedAt time.Time) *gin.Engine {
	r := gin.Default()

	registerHealthRoutes(r, pool, startedAt)

	accessTTL := time.Duration(cfg.JWTAccessTTLMinutes) * time.Minute
	jwtMgr := auth.NewJWTManager(cfg.JWTSecret, accessTTL)

	registerAuthRoutes(r, cfg, pool, jwtMgr)
	registerWorkspacesRoutes(r, pool, jwtMgr)
	registerCategoriesRoutes(r, pool, jwtMgr)
	registerTransactionsRoutes(r, pool, jwtMgr)
	registerBudgetsRoutes(r, pool, jwtMgr)

	return r
}

func registerHealthRoutes(r *gin.Engine, pool *pgxpool.Pool, startedAt time.Time) {
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"uptime": time.Since(startedAt).String(),
		})
	})

	r.GET("/ready", func(c *gin.Context) {
		ctxPing, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		if err := pool.Ping(ctxPing); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready", "db": "down"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready", "db": "up"})
	})
}

func registerAuthRoutes(r *gin.Engine, cfg config.Config, pool *pgxpool.Pool, jwtMgr *auth.JWTManager) {
	refreshTTL := time.Duration(cfg.RefreshTTLDays) * 24 * time.Hour
	authRepo := auth.NewRepo(pool)
	authSvc := auth.NewService(authRepo, jwtMgr, refreshTTL)
	authMW := auth.AuthRequired(jwtMgr)
	authH := auth.NewHandler(authSvc, authMW)
	authH.RegisterRoutes(r)
}

func registerWorkspacesRoutes(r *gin.Engine, pool *pgxpool.Pool, jwtMgr *auth.JWTManager) {
	authMW := auth.AuthRequired(jwtMgr)

	wsRepo := workspaces.NewRepo(pool)
	wsSvc := workspaces.NewService(wsRepo)
	wsH := workspaces.NewHandler(wsSvc, authMW, wsRepo)
	wsH.RegisterRoutes(r)
}

func registerCategoriesRoutes(r *gin.Engine, pool *pgxpool.Pool, jwtMgr *auth.JWTManager) {
	authMW := auth.AuthRequired(jwtMgr)

	wsRepo := workspaces.NewRepo(pool)
	catRepo := categories.NewRepo(pool)
	catSvc := categories.NewService(catRepo)
	catH := categories.NewHandler(catSvc, authMW, wsRepo)

	catH.RegisterRoutes(r)
}

func registerTransactionsRoutes(r *gin.Engine, pool *pgxpool.Pool, jwtMgr *auth.JWTManager) {
	authMW := auth.AuthRequired(jwtMgr)

	wsRepo := workspaces.NewRepo(pool)
	txRepo := transactions.NewRepo(pool)
	txSvc := transactions.NewService(txRepo)
	txH := transactions.NewHandler(txSvc, authMW, wsRepo)
	txH.RegisterRoutes(r)
}

func registerBudgetsRoutes(r *gin.Engine, pool *pgxpool.Pool, jwtMgr *auth.JWTManager) {
	authMW := auth.AuthRequired(jwtMgr)

	wsRepo := workspaces.NewRepo(pool)
	bRepo := budgets.NewRepo(pool)
	catLookup := budgets.NewCategoryLookup(pool)
	bSvc := budgets.NewService(bRepo, catLookup, true)
	bH := budgets.NewHandler(bSvc, wsRepo, authMW)
	bH.RegisterRoutes(r)
}

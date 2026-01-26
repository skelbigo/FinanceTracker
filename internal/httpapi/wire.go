package httpapi

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/skelbigo/FinanceTracker/internal/analytics"
	"github.com/skelbigo/FinanceTracker/internal/auth"
	"github.com/skelbigo/FinanceTracker/internal/budgets"
	"github.com/skelbigo/FinanceTracker/internal/categories"
	"github.com/skelbigo/FinanceTracker/internal/config"
	"github.com/skelbigo/FinanceTracker/internal/transactions"
	"github.com/skelbigo/FinanceTracker/internal/workspaces"
	"time"
)

func BuildRouterDeps(cfg config.Config, pool *pgxpool.Pool, startedAt time.Time) RouterDeps {
	accessTTL := cfg.AccessTTL()
	refreshTTL := cfg.RefreshTTL()

	jwtMgr := auth.NewJWTManager(cfg.JWTSecret, accessTTL)
	authMW := auth.AuthRequired(jwtMgr)

	wsRepo := workspaces.NewRepo(pool)

	// auth
	authRepo := auth.NewRepo(pool)
	authSvc := auth.NewService(authRepo, jwtMgr, refreshTTL)
	authH := auth.NewHandler(authSvc, authMW)

	// workspaces
	wsSvc := workspaces.NewService(wsRepo)
	wsH := workspaces.NewHandler(wsSvc, authMW, wsRepo)

	// categories
	catRepo := categories.NewRepo(pool)
	catSvc := categories.NewService(catRepo)
	catH := categories.NewHandler(catSvc, authMW, wsRepo)

	// transactions
	txRepo := transactions.NewRepo(pool)
	txSvc := transactions.NewService(txRepo)
	txH := transactions.NewHandler(txSvc, authMW, wsRepo)

	// budgets
	bRepo := budgets.NewRepo(pool)
	catLookup := budgets.NewCategoryLookup(pool)
	bSvc := budgets.NewService(bRepo, catLookup, cfg.BudgetsEnforceExpenseCategories)
	bH := budgets.NewHandler(bSvc, wsRepo, authMW)

	// analytics
	aRepo := analytics.NewRepo(pool)
	aSvc := analytics.NewService(aRepo)
	aH := analytics.NewHandler(aSvc, authMW, wsRepo)

	return RouterDeps{
		Readiness: pool,
		StartedAt: startedAt,

		Auth:         authH,
		Workspaces:   wsH,
		Categories:   catH,
		Transactions: txH,
		Budgets:      bH,
		Analytics:    aH,
	}
}

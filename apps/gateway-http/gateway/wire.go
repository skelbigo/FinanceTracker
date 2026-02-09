package gateway

import (
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/skelbigo/FinanceTracker/apps/auth-service/auth"
	"github.com/skelbigo/FinanceTracker/apps/budget-service/budgets"
	"github.com/skelbigo/FinanceTracker/apps/gateway-http/analytics"
	"github.com/skelbigo/FinanceTracker/apps/gateway-http/web"
	"github.com/skelbigo/FinanceTracker/apps/gateway-http/workspaces"
	"github.com/skelbigo/FinanceTracker/apps/notification-service/notifications"
	"github.com/skelbigo/FinanceTracker/apps/transaction-service/categories"
	"github.com/skelbigo/FinanceTracker/apps/transaction-service/transactions"
	"github.com/skelbigo/FinanceTracker/packages/contracts/notificationsv1"
	"github.com/skelbigo/FinanceTracker/packages/shared-kernel/config"
	"github.com/skelbigo/FinanceTracker/packages/shared-kernel/ratelimit"
	"github.com/skelbigo/FinanceTracker/packages/shared-kernel/redisx"
)

func BuildRouterDeps(cfg config.Config, pool *pgxpool.Pool, startedAt time.Time) RouterDeps {
	accessTTL := cfg.AccessTTL()
	refreshTTL := cfg.RefreshTTL()
	resetTTL := 30 * time.Minute
	returnResetToken := cfg.AppEnv != "prod"

	// redis
	rdb := redisx.NewClient(cfg)
	loginLimiter := ratelimit.NewLoginLimiter(cfg, rdb)

	cookieCfg := web.CookieConfig{Domain: cfg.CookieDomain, Secure: cfg.CookieSecure}

	jwtMgr := auth.NewJWTManager(cfg.JWTSecret, accessTTL)
	authMW := auth.AuthRequired(jwtMgr)

	wsRepo := workspaces.NewRepo(pool)

	// auth
	authRepo := auth.NewRepo(pool)
	usersAdapter := NewAuthUsersAdapter(authRepo)
	authSvc := auth.NewService(authRepo, jwtMgr, refreshTTL, resetTTL, returnResetToken, cfg.BCryptCost)
	authH := auth.NewHandler(authSvc, authMW, loginLimiter)

	// workspaces
	wsSvc := workspaces.NewService(wsRepo, usersAdapter)
	wsH := workspaces.NewHandler(wsSvc, wsRepo)

	// categories
	catRepo := categories.NewRepo(pool)
	catSvc := categories.NewService(catRepo)
	catH := categories.NewHandler(catSvc, wsRepo)

	// transactions
	txRepo := transactions.NewRepo(pool)
	txCatLookup := transactions.NewCategoryLookup(pool)

	// budgets
	bRepo := budgets.NewRepo(pool)
	bSvc := budgets.NewService(bRepo, txRepo, txCatLookup, cfg.BudgetsEnforceExpenseCategories)
	bH := budgets.NewHandler(bSvc, wsRepo)

	// notifications
	notifRepo := notifications.NewRepo(pool)
	var emailSender notifications.EmailSender
	if cfg.EmailEnabled {
		emailSender = notifications.NewSMTPSender(notifications.SMTPConfig{
			Host:     cfg.SMTPHost,
			Port:     cfg.SMTPPort,
			User:     cfg.SMTPUser,
			Password: cfg.SMTPPassword,
			From:     cfg.EmailFrom,
			UseTLS:   cfg.SMTPTLS,
		})
	} else {
		emailSender = nil
	}
	pushSender := notifications.NoopPushProvider{}
	notifSvc := notifications.NewService(notifRepo, wsRepo, usersAdapter, emailSender, pushSender, notifications.Options{
		PublicURL:                 cfg.AppPublicURL,
		EmailNotifyOverspending:   cfg.EmailNotifyOverspending,
		EmailNotifyNewTransaction: cfg.EmailNotifyNewTransaction,
	})
	notifH := notifications.NewHandler(notifSvc)

	notifClient := notificationsv1.NewClient(fmt.Sprintf("127.0.0.1:%d", cfg.NotificationsGRPCPort))
	bSvc.WithNotifications(budgetMembersAdapter{wsRepo: wsRepo}, notifClient)

	// analytics cache
	aCacheIndex := analytics.NewCacheIndex(rdb)

	txSvc := transactions.NewService(txRepo, bSvc, txCatLookup).WithAnalyticsCache(aCacheIndex).WithNotifications(notifSvc)
	txH := transactions.NewHandler(txSvc, wsRepo)

	// analytics
	aRepo := analytics.NewRepo(pool)
	aSvc := analytics.NewService(aRepo, rdb, aCacheIndex, cfg.AnalyticsCacheTTL())
	aH := analytics.NewHandler(aSvc, wsRepo)

	return RouterDeps{
		Readiness:     pool,
		StartedAt:     startedAt,
		WorkspaceRBAC: wsRepo,

		JWTM:         jwtMgr,
		AuthSvc:      authSvc,
		LoginLimiter: loginLimiter,
		AccessTTL:    accessTTL,
		RefreshTTL:   refreshTTL,
		CookieCfg:    cookieCfg,

		CSRFSecret: cfg.CSRFSecret,
		CSRFTTL:    cfg.CSRFTTL(),

		WorkspacesSvc:    wsSvc,
		CategoriesSvc:    catSvc,
		BudgetsSvc:       bSvc,
		TransactionsSvc:  txSvc,
		AnalyticsSvc:     aSvc,
		NotificationsSvc: notifSvc,

		Auth:          authH,
		Workspaces:    wsH,
		Categories:    catH,
		Transactions:  txH,
		Budgets:       bH,
		Analytics:     aH,
		Notifications: notifH,
	}
}

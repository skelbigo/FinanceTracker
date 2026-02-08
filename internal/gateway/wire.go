package gateway

import (
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/skelbigo/FinanceTracker/internal/analytics"
	"github.com/skelbigo/FinanceTracker/internal/auth"
	"github.com/skelbigo/FinanceTracker/internal/budgets"
	"github.com/skelbigo/FinanceTracker/internal/categories"
	"github.com/skelbigo/FinanceTracker/internal/config"
	"github.com/skelbigo/FinanceTracker/internal/contracts/notificationsv1"
	"github.com/skelbigo/FinanceTracker/internal/notifications"
	"github.com/skelbigo/FinanceTracker/internal/redisx"
	"github.com/skelbigo/FinanceTracker/internal/transactions"
	"github.com/skelbigo/FinanceTracker/internal/web"
	"github.com/skelbigo/FinanceTracker/internal/workspaces"
)

func BuildRouterDeps(cfg config.Config, pool *pgxpool.Pool, startedAt time.Time) RouterDeps {
	accessTTL := cfg.AccessTTL()
	refreshTTL := cfg.RefreshTTL()
	resetTTL := 30 * time.Minute
	returnResetToken := cfg.AppEnv != "prod"

	cookieCfg := web.CookieConfig{Domain: cfg.CookieDomain, Secure: cfg.CookieSecure}

	jwtMgr := auth.NewJWTManager(cfg.JWTSecret, accessTTL)
	authMW := auth.AuthRequired(jwtMgr)

	wsRepo := workspaces.NewRepo(pool)

	// auth
	authRepo := auth.NewRepo(pool)
	usersAdapter := NewAuthUsersAdapter(authRepo)
	authSvc := auth.NewService(authRepo, jwtMgr, refreshTTL, resetTTL, returnResetToken)
	authH := auth.NewHandler(authSvc, authMW)

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
	notifSvc := notifications.NewService(notifRepo, usersAdapter, emailSender, pushSender, notifications.Options{
		PublicURL:                 cfg.AppPublicURL,
		EmailNotifyOverspending:   cfg.EmailNotifyOverspending,
		EmailNotifyNewTransaction: cfg.EmailNotifyNewTransaction,
	})
	notifH := notifications.NewHandler(notifSvc)

	notifClient := notificationsv1.NewClient(fmt.Sprintf("127.0.0.1:%d", cfg.NotificationsGRPCPort))
	bSvc.WithNotifications(budgetMembersAdapter{wsRepo: wsRepo}, notifClient)

	// redis (analytics cache)
	rdb := redisx.NewClient(cfg)
	aCacheIndex := analytics.NewCacheIndex(rdb)

	txSvc := transactions.NewService(txRepo, bSvc, txCatLookup).
		WithAnalyticsCache(aCacheIndex).
		WithNewTransactionHook(&newTransactionHook{wsRepo: wsRepo, notifs: notifSvc})
	txH := transactions.NewHandler(txSvc, wsRepo)

	// analytics
	aRepo := analytics.NewRepo(pool)
	aSvc := analytics.NewService(aRepo, rdb, aCacheIndex, cfg.AnalyticsCacheTTL())
	aH := analytics.NewHandler(aSvc, wsRepo)

	return RouterDeps{
		Readiness: pool,
		StartedAt: startedAt,

		JWTM:       jwtMgr,
		AuthSvc:    authSvc,
		AccessTTL:  accessTTL,
		RefreshTTL: refreshTTL,
		CookieCfg:  cookieCfg,

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

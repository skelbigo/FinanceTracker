package gateway

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/skelbigo/FinanceTracker/internal/analytics"
	"github.com/skelbigo/FinanceTracker/internal/auth"
	"github.com/skelbigo/FinanceTracker/internal/budgets"
	"github.com/skelbigo/FinanceTracker/internal/categories"
	"github.com/skelbigo/FinanceTracker/internal/notifications"
	"github.com/skelbigo/FinanceTracker/internal/transactions"
	"github.com/skelbigo/FinanceTracker/internal/web"
	"github.com/skelbigo/FinanceTracker/internal/workspaces"
)

var (
	Version   = "dev"
	Commit    = "none"
	BuildTime = "unknown"
)

type RoutesRegistrar interface {
	RegisterRoutes(r gin.IRouter)
}

type ReadinessChecker interface {
	Ping(ctx context.Context) error
}

type RouterDeps struct {
	Readiness ReadinessChecker
	StartedAt time.Time

	JWTM       *auth.JWTManager
	AuthSvc    *auth.Service
	AccessTTL  time.Duration
	RefreshTTL time.Duration
	CookieCfg  web.CookieConfig

	CSRFSecret string
	CSRFTTL    time.Duration

	WorkspacesSvc    *workspaces.Service
	CategoriesSvc    *categories.Service
	BudgetsSvc       *budgets.Service
	TransactionsSvc  *transactions.Service
	AnalyticsSvc     *analytics.Service
	NotificationsSvc *notifications.Service

	Auth          RoutesRegistrar
	Workspaces    RoutesRegistrar
	Categories    RoutesRegistrar
	Transactions  RoutesRegistrar
	Budgets       RoutesRegistrar
	Analytics     RoutesRegistrar
	Notifications RoutesRegistrar
}

func SetupRouter(r *gin.Engine, deps RouterDeps) *gin.Engine {
	if deps.Readiness == nil {
		panic("httpapi: router deps not initialized: Readiness is nil")
	}

	checks := []struct {
		name string
		rr   RoutesRegistrar
	}{
		{"Auth", deps.Auth},
		{"Workspaces", deps.Workspaces},
		{"Categories", deps.Categories},
		{"Transactions", deps.Transactions},
		{"Budgets", deps.Budgets},
		{"Analytics", deps.Analytics},
		{"Notifications", deps.Notifications},
	}

	for _, c := range checks {
		if c.rr == nil {
			panic("httpapi: router deps not initialized: " + c.name + " registrar is nil")
		}
	}

	registerHealthRoutes(r, deps.Readiness, deps.StartedAt)

	r.Static("/static", "./web/static")

	webRenderer := web.NewRenderer("./web/templates")
	webHandlers := &web.Handlers{
		R: webRenderer,

		Auth:          deps.AuthSvc,
		Workspaces:    deps.WorkspacesSvc,
		Categories:    deps.CategoriesSvc,
		Budgets:       deps.BudgetsSvc,
		Transactions:  deps.TransactionsSvc,
		Analytics:     deps.AnalyticsSvc,
		Notifications: deps.NotificationsSvc,

		JWTM:       deps.JWTM,
		CookieCfg:  deps.CookieCfg,
		AccessTTL:  deps.AccessTTL,
		RefreshTTL: deps.RefreshTTL,

		CSRFSecret: deps.CSRFSecret,
		CSRFTTL:    deps.CSRFTTL,
	}
	web.RegisterRoutes(r, webHandlers)

	deps.Auth.RegisterRoutes(r)
	deps.Workspaces.RegisterRoutes(r)
	deps.Categories.RegisterRoutes(r)
	deps.Transactions.RegisterRoutes(r)
	deps.Budgets.RegisterRoutes(r)
	deps.Analytics.RegisterRoutes(r)
	deps.Notifications.RegisterRoutes(r)

	api := r.Group("/api")
	deps.Auth.RegisterRoutes(api)
	deps.Workspaces.RegisterRoutes(api)
	deps.Categories.RegisterRoutes(api)
	deps.Transactions.RegisterRoutes(api)
	deps.Budgets.RegisterRoutes(api)
	deps.Analytics.RegisterRoutes(api)
	deps.Notifications.RegisterRoutes(api)

	return r
}

func registerHealthRoutes(r gin.IRouter, readiness ReadinessChecker, startedAt time.Time) {
	healthPayload := func(status string) gin.H {
		now := time.Now().UTC()
		return gin.H{
			"status":         status,
			"uptime_seconds": time.Since(startedAt).Seconds(),
			"started_at":     startedAt.UTC().Format(time.RFC3339),
			"now":            now.Format(time.RFC3339),
			"version":        Version,
			"commit":         Commit,
			"build_time":     BuildTime,
		}
	}

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, healthPayload("ok"))
	})

	r.GET("/ready", func(c *gin.Context) {
		ctxPing, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		if err := readiness.Ping(ctxPing); err != nil {
			payload := healthPayload("not_ready")
			payload["db"] = "down"
			c.JSON(http.StatusServiceUnavailable, payload)
			return
		}

		payload := healthPayload("ready")
		payload["db"] = "up"
		c.JSON(http.StatusOK, payload)
	})
}

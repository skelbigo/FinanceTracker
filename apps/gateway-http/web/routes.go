package web

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/skelbigo/FinanceTracker/apps/auth-service/auth"
	"github.com/skelbigo/FinanceTracker/apps/budget-service/budgets"
	"github.com/skelbigo/FinanceTracker/apps/gateway-http/analytics"
	"github.com/skelbigo/FinanceTracker/apps/gateway-http/workspaces"
	"github.com/skelbigo/FinanceTracker/apps/notification-service/notifications"
	"github.com/skelbigo/FinanceTracker/apps/transaction-service/categories"
	"github.com/skelbigo/FinanceTracker/apps/transaction-service/transactions"
	"github.com/skelbigo/FinanceTracker/packages/shared-kernel/ratelimit"
)

type Handlers struct {
	R *Renderer

	Auth          *auth.Service
	Workspaces    *workspaces.Service
	Categories    *categories.Service
	Budgets       *budgets.Service
	Transactions  *transactions.Service
	Analytics     *analytics.Service
	Notifications *notifications.Service
	JWTM          *auth.JWTManager
	LoginLimiter  *ratelimit.LoginLimiter

	CookieCfg  CookieConfig
	AccessTTL  time.Duration
	RefreshTTL time.Duration

	CSRFSecret string
	CSRFTTL    time.Duration
}

func RegisterRoutes(router *gin.Engine, h *Handlers) {
	webGroup := router.Group("/")
	webGroup.Use(CSRFMiddleware(h.CSRFSecret))

	webGroup.GET("/login", h.GetLogin)
	webGroup.POST("/login", h.PostLogin)

	webGroup.GET("/reset/request", h.GetResetRequest)
	webGroup.POST("/reset/request", h.PostResetRequest)
	webGroup.GET("/reset/confirm", h.GetResetConfirm)
	webGroup.POST("/reset/confirm", h.PostResetConfirm)

	webGroup.GET("/register", h.GetRegister)
	webGroup.POST("/register", h.PostRegister)

	webGroup.POST("/logout", h.PostLogout)

	app := webGroup.Group("/app")
	app.Use(RequireAuth(h.JWTM, h.Auth, h.CookieCfg, h.AccessTTL, h.RefreshTTL))

	app.GET("/workspaces", h.GetWorkspacesPage)
	app.POST("/workspaces", h.PostCreateWorkspace)

	app.GET("/notifications", h.GetNotificationsPage)
	app.GET("/notifications/goto", h.GetNotificationGoto)
	app.GET("/notifications/:id/goto", h.GetNotificationGoto)
	app.POST("/notifications/:id/read", h.PostNotificationRead)
	app.POST("/notifications/read-all", h.PostNotificationsReadAll)

	app.POST("/workspaces/select", h.PostSelectWorkspace)
	app.GET("/workspaces/:id", h.GetWorkspaceMembersPage)
	app.POST("/workspaces/:id/members", h.PostAddWorkspaceMember)
	app.POST("/workspaces/:id/members/:userId/role", h.PostUpdateWorkspaceMemberRole)
	app.POST("/workspaces/:id/members/:userId/remove", h.PostRemoveWorkspaceMember)

	withWS := app.Group("")
	withWS.Use(h.RequireWorkspace())
	withWS.GET("", h.GetDashboard)
	withWS.GET("/categories", h.GetCategoriesPage)
	withWS.POST("/categories", h.PostCreateCategory)
	withWS.POST("/categories/:id/update", h.PostUpdateCategory)
	withWS.POST("/categories/:id/delete", h.PostDeleteCategory)
	withWS.GET("/budgets", h.GetBudgetsPage)
	withWS.POST("/budgets", h.PostCreateBudget)
	withWS.POST("/budgets/:id/update", h.PostUpdateBudget)
	withWS.POST("/budgets/:id/delete", h.PostDeleteBudget)
	withWS.GET("/transactions", h.GetTransactionsPage)
	withWS.GET("/transactions/table", h.GetTransactionsTable)
	withWS.POST("/transactions", h.PostCreateTransaction)
	withWS.GET("/transactions/:id/edit", h.GetTransactionEdit)
	withWS.POST("/transactions/:id/update", h.PostUpdateTransaction)
	withWS.POST("/transactions/:id/delete", h.PostDeleteTransaction)
	withWS.GET("/analytics", h.GetAnalyticsPage)
	_ = withWS
}

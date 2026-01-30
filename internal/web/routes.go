package web

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/skelbigo/FinanceTracker/internal/auth"
	"github.com/skelbigo/FinanceTracker/internal/categories"
	"github.com/skelbigo/FinanceTracker/internal/transactions"
	"github.com/skelbigo/FinanceTracker/internal/workspaces"
)

type Handlers struct {
	R *Renderer

	Auth         *auth.Service
	Workspaces   *workspaces.Service
	Categories   *categories.Service
	Transactions *transactions.Service
	JWTM         *auth.JWTManager

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
	app.GET("", h.GetDashboard)

	app.GET("/workspaces", h.GetWorkspacesPage)
	app.POST("/workspaces", h.PostCreateWorkspace)

	withWS := app.Group("")
	withWS.Use(h.RequireWorkspace())
	withWS.GET("/transactions", h.GetTransactionsPage)
	withWS.GET("/transactions/table", h.GetTransactionsTable)
	withWS.POST("/transactions", h.PostCreateTransaction)
	withWS.GET("/transactions/:id/edit", h.GetTransactionEdit)
	withWS.POST("/transactions/:id/update", h.PostUpdateTransaction)
	withWS.POST("/transactions/:id/delete", h.PostDeleteTransaction)
	_ = withWS
}

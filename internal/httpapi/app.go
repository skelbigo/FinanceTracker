package httpapi

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/skelbigo/FinanceTracker/internal/config"
	"io"
	"os"
	"time"
)

type App struct {
	deps RouterDeps
}

func NewApp(cfg config.Config, pool *pgxpool.Pool, startedAt time.Time) *App {
	return &App{
		deps: BuildRouterDeps(cfg, pool, startedAt),
	}
}

func (a *App) Router(w io.Writer) *gin.Engine {
	return a.RouterWithWriters(w, w)
}

func (a *App) RouterWithWriters(access io.Writer, errors io.Writer) *gin.Engine {
	if access == nil {
		access = os.Stdout
	}
	if errors == nil {
		errors = os.Stderr
	}

	r := gin.New()

	r.Use(gin.LoggerWithWriter(access))
	r.Use(gin.RecoveryWithWriter(errors))

	return SetupRouter(r, a.deps)
}

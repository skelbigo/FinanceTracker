package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/skelbigo/FinanceTracker/internal/config"
	"github.com/skelbigo/FinanceTracker/internal/db"
	"github.com/skelbigo/FinanceTracker/internal/httpapi"
	"github.com/skelbigo/FinanceTracker/internal/migrator"
)

type cliFlags struct {
	mode          string
	cmd           string
	migrationsDir string
	dotenv        string
}

func parseFlags() cliFlags {
	mode := flag.String("mode", "serve", "run mode: serve|migrate")
	cmd := flag.String("cmd", "up", "migrate command (used only with -mode=migrate): up|down|status|version|force:<n>")
	migrationsDir := flag.String("migrations", "migrations", "path to migrations directory")
	dotenv := flag.String("dotenv", ".env", "path to dotenv file; set empty to disable")
	flag.Parse()

	return cliFlags{
		mode:          *mode,
		cmd:           *cmd,
		migrationsDir: *migrationsDir,
		dotenv:        *dotenv,
	}
}

func loadDotenv(path string) error {
	if path == "" {
		return nil
	}
	return godotenv.Load(path)
}

func main() {
	startedAt := time.Now()
	logger := log.New(os.Stdout, "", log.LstdFlags|log.LUTC)

	f := parseFlags()
	if err := loadDotenv(f.dotenv); err != nil && os.Getenv("APP_ENV") == "dev" {
		logger.Printf(".env not loaded (dev expects it): %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("invalid environment: %v", err)
	}

	switch f.mode {
	case "migrate":
		dbURL := cfg.EffectiveDBURL()
		if err := migrator.Run(f.migrationsDir, dbURL, f.cmd, os.Stdout); err != nil {
			logger.Fatalf("migrate cmd=%s dir=%s failed: %v", f.cmd, f.migrationsDir, err)
		}
		logger.Printf("migrations done: cmd=%s dir=%s", f.cmd, f.migrationsDir)

	case "serve":
		if err := serve(cfg, startedAt, logger); err != nil {
			logger.Fatalf("startup failed: %v", err)
		}

	default:
		flag.Usage()
		logger.Fatalf("unknown -mode=%q (use: serve|migrate)", f.mode)
	}
}

func serve(cfg config.Config, startedAt time.Time, logger *log.Logger) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dbURL := cfg.EffectiveDBURL()
	pool, err := newDBPool(ctx, dbURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	gin.DefaultWriter = logger.Writer()
	gin.DefaultErrorWriter = logger.Writer()

	app := httpapi.NewApp(cfg, pool, startedAt)
	r := app.Router(logger.Writer())

	addr := fmt.Sprintf(":%d", cfg.AppPort)

	srv := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	runCtx, stopSignal := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stopSignal()

	if err := runHTTPServer(runCtx, srv, addr, logger); err != nil {
		return fmt.Errorf("server failed: %w", err)
	}

	logger.Printf("Server stopped")
	return nil
}

func runHTTPServer(ctx context.Context, srv *http.Server, addr string, logger *log.Logger) error {
	errCh := make(chan error, 1)

	go func() {
		logger.Printf("Starting FinanceTracker API mode=%s addr=%s", gin.Mode(), addr)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
	case err := <-errCh:
		if err == nil || errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Printf("graceful shutdown failed: %v", err)
		_ = srv.Close()
	}

	select {
	case err := <-errCh:
		if err == nil || errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-shutdownCtx.Done():
		return nil
	}
}

func newDBPool(ctx context.Context, dbURL string) (*pgxpool.Pool, error) {
	pool, err := db.NewPostgresPoolFromURL(ctx, dbURL)
	if err != nil {
		return nil, fmt.Errorf("db connect (%s): %w", db.MaskPostgresURL(dbURL), err)
	}
	return pool, nil
}

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

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/skelbigo/FinanceTracker/apps/auth-service/auth"
	"github.com/skelbigo/FinanceTracker/apps/gateway-http/workspaces"
	"github.com/skelbigo/FinanceTracker/apps/notification-service/notifications"
	"github.com/skelbigo/FinanceTracker/packages/db"
	"github.com/skelbigo/FinanceTracker/packages/shared-kernel/config"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

type cliFlags struct {
	dotenv string
}

func parseFlags() cliFlags {
	dotenv := flag.String("dotenv", ".env", "path to dotenv file; set empty to disable")
	flag.Parse()
	return cliFlags{dotenv: *dotenv}
}

func loadDotenv(path string) error {
	if path == "" {
		return nil
	}
	return godotenv.Load(path)
}

type authUsersAdapter struct{ repo *auth.Repo }

func (a authUsersAdapter) GetUserEmail(ctx context.Context, userID string) (string, error) {
	u, err := a.repo.GetUserByID(ctx, userID)
	if err != nil {
		return "", err
	}
	return u.Email, nil
}

func main() {
	logger := log.New(os.Stdout, "", log.LstdFlags|log.LUTC)

	f := parseFlags()
	if err := loadDotenv(f.dotenv); err != nil && os.Getenv("APP_ENV") == "dev" {
		logger.Printf(".env not loaded (dev expects it): %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		logger.Fatalf("invalid environment: %v", err)
	}

	if err := serve(cfg, logger); err != nil {
		logger.Fatalf("startup failed: %v", err)
	}
}

func serve(cfg config.Config, logger *log.Logger) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dbURL := cfg.EffectiveDBURL()
	pool, err := newDBPool(ctx, dbURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	wsRepo := workspaces.NewRepo(pool)
	authRepo := auth.NewRepo(pool)
	users := authUsersAdapter{repo: authRepo}

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
	notifSvc := notifications.NewService(notifRepo, wsRepo, users, emailSender, pushSender, notifications.Options{
		PublicURL:                 cfg.AppPublicURL,
		EmailNotifyOverspending:   cfg.EmailNotifyOverspending,
		EmailNotifyNewTransaction: cfg.EmailNotifyNewTransaction,
	})

	grpcAddr := fmt.Sprintf(":%d", cfg.NotificationsGRPCPort)
	grpcMux := http.NewServeMux()
	notifications.RegisterGRPC(grpcMux, notifSvc, logger)

	grpcSrv := &http.Server{
		Addr:              grpcAddr,
		Handler:           h2c.NewHandler(grpcMux, &http2.Server{}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	runCtx, stopSignal := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stopSignal()

	logger.Printf("Starting notification-service gRPC addr=%s", grpcAddr)
	return runHTTPServer(runCtx, grpcSrv, grpcAddr, logger)
}

func runHTTPServer(ctx context.Context, srv *http.Server, addr string, logger *log.Logger) error {
	errCh := make(chan error, 1)
	go func() {
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

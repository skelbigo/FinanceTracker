package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/skelbigo/FinanceTracker/apps/notification-service/notifications"
	"github.com/skelbigo/FinanceTracker/packages/db"
	"github.com/skelbigo/FinanceTracker/packages/shared-kernel/config"
)

type workerFlags struct {
	dotenv string
}

func parseFlags() workerFlags {
	dotenv := flag.String("dotenv", ".env", "path to dotenv file; set empty to disable")
	flag.Parse()
	return workerFlags{dotenv: *dotenv}
}

func loadDotenv(path string) error {
	if path == "" {
		return nil
	}
	return godotenv.Load(path)
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := db.NewPostgresPoolFromURL(ctx, cfg.EffectiveDBURL())
	if err != nil {
		logger.Fatalf("db connect: %v", err)
	}
	defer pool.Close()

	repo := notifications.NewRepo(pool)

	sender := notifications.NewSMTPSender(notifications.SMTPConfig{
		Host:     cfg.SMTPHost,
		Port:     cfg.SMTPPort,
		User:     cfg.SMTPUser,
		Password: cfg.SMTPPassword,
		From:     cfg.EmailFrom,
		UseTLS:   cfg.SMTPTLS,
	})

	interval := durationFromEnv("WORKER_POLL_INTERVAL_SECONDS", 5*time.Second)
	batchSize := intFromEnv("WORKER_BATCH_SIZE", 50)
	maxAttempts := intFromEnv("WORKER_MAX_ATTEMPTS", 5)

	w := notifications.NewEmailWorker(repo, sender, cfg.AppPublicURL, interval, batchSize, maxAttempts, logger)

	runCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	w.Run(runCtx)
}

func intFromEnv(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	var n int
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil || n <= 0 {
		return def
	}
	return n
}

func durationFromEnv(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	var s int
	if _, err := fmt.Sscanf(v, "%d", &s); err != nil || s <= 0 {
		return def
	}
	return time.Duration(s) * time.Second
}

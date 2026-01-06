package config_test

import (
	"strings"
	"testing"

	"github.com/skelbigo/FinanceTracker/internal/config"
)

func unsetConfigEnv(t *testing.T) {
	t.Helper()
	keys := []string{
		"APP_PORT",
		"DB_URL",
		"DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME", "DB_SSLMODE",
		"REDIS_HOST", "REDIS_PORT",
		"JWT_SECRET",
	}
	for _, k := range keys {
		t.Setenv(k, "")
	}
}

func TestLoad_MissingRequired(t *testing.T) {
	unsetConfigEnv(t)

	_, err := config.Load()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	msg := err.Error()
	want := []string{"DB_USER", "DB_PASSWORD", "DB_NAME", "JWT_SECRET"}
	for _, k := range want {
		if !strings.Contains(msg, k) {
			t.Fatalf("expected error to mention %q, got: %s", k, msg)
		}
	}
}

func TestLoad_InvalidDBPort(t *testing.T) {
	unsetConfigEnv(t)

	t.Setenv("APP_PORT", "8080")

	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_PORT", "not-a-number")
	t.Setenv("DB_USER", "postgres")
	t.Setenv("DB_PASSWORD", "postgres")
	t.Setenv("DB_NAME", "financetracker")
	t.Setenv("DB_SSLMODE", "disable")

	t.Setenv("JWT_SECRET", "dev_secret")

	_, err := config.Load()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid DB_PORT") {
		t.Fatalf("expected invalid DB_PORT error, got: %v", err)
	}
}

func TestLoad_OK(t *testing.T) {
	unsetConfigEnv(t)

	t.Setenv("APP_PORT", "8080")

	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_PORT", "5432")
	t.Setenv("DB_USER", "postgres")
	t.Setenv("DB_PASSWORD", "postgres")
	t.Setenv("DB_NAME", "financetracker")
	t.Setenv("DB_SSLMODE", "disable")

	t.Setenv("JWT_SECRET", "dev_secret")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DBPort != 5432 {
		t.Fatalf("expected DBPort=5432, got %d", cfg.DBPort)
	}
}

func TestLoad_DefaultsApplied(t *testing.T) {
	unsetConfigEnv(t)

	t.Setenv("DB_USER", "postgres")
	t.Setenv("DB_PASSWORD", "postgres")
	t.Setenv("DB_NAME", "financetracker")
	t.Setenv("JWT_SECRET", "dev")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.DBHost != "127.0.0.1" {
		t.Fatalf("expected default DBHost, got %q", cfg.DBHost)
	}
	if cfg.DBPort != 5432 {
		t.Fatalf("expected default DBPort=5432, got %d", cfg.DBPort)
	}
	if cfg.DBSSLMode != "prefer" {
		t.Fatalf("expected default DBSSLMode=prefer, got %q", cfg.DBSSLMode)
	}
}

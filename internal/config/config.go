package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	AppPort int

	DBHost     string
	DBPort     int
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string

	DBURL string

	RedisHost string
	RedisPort int

	JWTSecret string
}

func Load() (Config, error) {
	var cfg Config
	var errs []error

	cfg.AppPort = mustInt(getDefault("APP_PORT", "8080"), "APP_PORT", &errs)

	cfg.DBURL = strings.TrimSpace(os.Getenv("DB_URL"))

	cfg.DBHost = getDefault("DB_HOST", "127.0.0.1")
	cfg.DBPort = mustInt(getDefault("DB_PORT", "5432"), "DB_PORT", &errs)
	cfg.DBUser = mustString("DB_USER", &errs)
	cfg.DBPassword = mustString("DB_PASSWORD", &errs)
	cfg.DBName = mustString("DB_NAME", &errs)
	cfg.DBSSLMode = getDefault("DB_SSLMODE", "prefer")
	validateOneOf(
		"DB_SSLMODE",
		cfg.DBSSLMode,
		[]string{"disable", "prefer", "require", "verify-ca", "verify-full"},
		&errs,
	)

	cfg.RedisHost = getDefault("REDIS_HOST", "localhost")
	cfg.RedisPort = mustInt(getDefault("REDIS_PORT", "6379"), "REDIS_PORT", &errs)

	jwt := os.Getenv("JWT_SECRET")
	if strings.TrimSpace(jwt) == "" {
		errs = append(errs, fmt.Errorf("missing required env: JWT_SECRET"))
	}
	cfg.JWTSecret = jwt

	if cfg.DBPort <= 0 || cfg.DBPort > 65535 {
		errs = append(errs, fmt.Errorf("DB_PORT out of range: %d", cfg.DBPort))
	}
	if cfg.AppPort <= 0 || cfg.AppPort > 65535 {
		errs = append(errs, fmt.Errorf("APP_PORT out of range: %d", cfg.AppPort))
	}

	if len(errs) > 0 {
		return Config{}, joinErrors(errs)
	}
	return cfg, nil
}

func mustString(key string, errs *[]error) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		*errs = append(*errs, fmt.Errorf("missing required env: %s", key))
	}
	return v
}

func mustInt(raw string, key string, errs *[]error) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		*errs = append(*errs, fmt.Errorf("missing required env: %s", key))
		return 0
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		*errs = append(*errs, fmt.Errorf("invalid %s=%q (expected int): %w", key, raw, err))
		return 0
	}
	return n
}

func getDefault(key, def string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	return v
}

func validateOneOf(key, value string, allowed []string, errs *[]error) {
	for _, a := range allowed {
		if value == a {
			return
		}
	}
	*errs = append(*errs, fmt.Errorf("%s must be one of %v, got %q", key, allowed, value))
}

func joinErrors(errs []error) error {
	return errors.Join(errs...)
}

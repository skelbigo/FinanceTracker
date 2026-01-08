package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/skelbigo/FinanceTracker/internal/db"
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

	RedisEnabled bool
	RedisHost    string
	RedisPort    int

	JWTSecret           string
	JWTAccessTTLMinutes int
	RefreshTTLDays      int
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

	cfg.RedisEnabled = mustBool(getDefault("REDIS_ENABLED", ""), "REDIS_ENABLED", &errs)

	redisHostRaw := strings.TrimSpace(os.Getenv("REDIS_HOST"))
	redisPortRaw := strings.TrimSpace(os.Getenv("REDIS_PORT"))

	if cfg.RedisEnabled || redisHostRaw != "" || redisPortRaw != "" {
		cfg.RedisEnabled = true
		cfg.RedisHost = getDefault("REDIS_HOST", "localhost")
		cfg.RedisPort = mustInt(getDefault("REDIS_PORT", "6379"), "REDIS_PORT", &errs)

		if cfg.RedisPort <= 0 || cfg.RedisPort > 65535 {
			errs = append(errs, fmt.Errorf("REDIS_PORT out of range: %d", cfg.RedisPort))
		}
	}

	cfg.JWTSecret = strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if cfg.JWTSecret == "" {
		errs = append(errs, fmt.Errorf("missing required env: JWT_SECRET"))
	}

	cfg.JWTAccessTTLMinutes = mustInt(getDefault("JWT_ACCESS_TTL_MINUTES", "15"), "JWT_ACCESS_TTL_MINUTES", &errs)
	cfg.RefreshTTLDays = mustInt(getDefault("REFRESH_TTL_DAYS", "30"), "REFRESH_TTL_DAYS", &errs)

	if cfg.JWTAccessTTLMinutes <= 0 || cfg.JWTAccessTTLMinutes > 24*60 {
		errs = append(errs, fmt.Errorf("JWT_ACCESS_TTL_MINUTES out of range: %d", cfg.JWTAccessTTLMinutes))
	}
	if cfg.RefreshTTLDays <= 0 || cfg.RefreshTTLDays > 365 {
		errs = append(errs, fmt.Errorf("REFRESH_TTL_DAYS out of range: %d", cfg.RefreshTTLDays))
	}
	if cfg.DBPort <= 0 || cfg.DBPort > 65535 {
		errs = append(errs, fmt.Errorf("DB_PORT out of range: %d", cfg.DBPort))
	}
	if cfg.AppPort <= 0 || cfg.AppPort > 65535 {
		errs = append(errs, fmt.Errorf("APP_PORT out of range: %d", cfg.AppPort))
	}

	if len(errs) > 0 {
		return Config{}, errors.Join(errs...)
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

func mustBool(raw string, key string, errs *[]error) bool {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return false
	}
	switch raw {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		*errs = append(*errs, fmt.Errorf("invalid %s=%q (expected bool)", key, raw))
		return false
	}
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

func (c Config) EffectiveDBURL() string {
	if strings.TrimSpace(c.DBURL) != "" {
		return strings.TrimSpace(c.DBURL)
	}

	dbCfg := db.DBConfig{
		Host:     c.DBHost,
		Port:     c.DBPort,
		User:     c.DBUser,
		Password: c.DBPassword,
		Name:     c.DBName,
		SSLMode:  c.DBSSLMode,
	}
	return db.BuildPostgresURL(dbCfg)
}

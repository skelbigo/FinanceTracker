package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultAppPort = "8080"

	defaultDBHost    = "127.0.0.1"
	defaultDBPort    = "5432"
	defaultDBSSLMode = "prefer"

	defaultRedisHost = "localhost"
	defaultRedisPort = "6379"

	defaultJWTAccessTTLMinutes = "15"
	defaultRefreshTTLDays      = "30"

	defaultBudgetsEnforceExpenseCategories = "true"

	maxPort = 65535
)

func (c Config) AccessTTL() time.Duration {
	return time.Duration(c.JWTAccessTTLMinutes) * time.Minute
}

func (c Config) RefreshTTL() time.Duration {
	return time.Duration(c.RefreshTTLDays) * 24 * time.Hour
}

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

	BudgetsEnforceExpenseCategories bool
}

func Load() (Config, error) {
	var cfg Config
	var errs []error

	cfg.AppPort = mustInt(getDefault("APP_PORT", defaultAppPort), "APP_PORT", &errs)

	cfg.DBURL = strings.TrimSpace(os.Getenv("DB_URL"))
	cfg.DBHost = getDefault("DB_HOST", defaultDBHost)
	cfg.DBPort = mustInt(getDefault("DB_PORT", defaultDBPort), "DB_PORT", &errs)
	cfg.DBUser = mustString("DB_USER", &errs)
	cfg.DBPassword = mustString("DB_PASSWORD", &errs)
	cfg.DBName = mustString("DB_NAME", &errs)

	cfg.DBSSLMode = getDefault("DB_SSLMODE", defaultDBSSLMode)
	validateOneOf(
		"DB_SSLMODE",
		cfg.DBSSLMode,
		[]string{"disable", "prefer", "require", "verify-ca", "verify-full"},
		&errs,
	)

	redisHostRaw := strings.TrimSpace(os.Getenv("REDIS_HOST"))
	redisPortRaw := strings.TrimSpace(os.Getenv("REDIS_PORT"))
	redisEnabledRaw := strings.TrimSpace(os.Getenv("REDIS_ENABLED"))

	enabledVal, enabledIsSet, enabledErr := parseBoolOptional(redisEnabledRaw, "REDIS_ENABLED")
	if enabledErr != nil {
		errs = append(errs, enabledErr)
	}

	switch {
	case enabledIsSet && enabledVal:
		cfg.RedisEnabled = true
		cfg.RedisHost = getDefault("REDIS_HOST", defaultRedisHost)
		cfg.RedisPort = mustInt(getDefault("REDIS_PORT", defaultRedisPort), "REDIS_PORT", &errs)

	case enabledIsSet && !enabledVal:
		cfg.RedisEnabled = false
		if redisHostRaw != "" || redisPortRaw != "" {
			errs = append(errs, fmt.Errorf("REDIS_ENABLED=false, but REDIS_HOST/REDIS_PORT provided (remove them or set REDIS_ENABLED=true)"))
		}

	default:
		if redisHostRaw != "" || redisPortRaw != "" {
			cfg.RedisEnabled = true
			cfg.RedisHost = getDefault("REDIS_HOST", defaultRedisHost)
			cfg.RedisPort = mustInt(getDefault("REDIS_PORT", defaultRedisPort), "REDIS_PORT", &errs)
		} else {
			cfg.RedisEnabled = false
		}
	}

	if cfg.RedisEnabled {
		if cfg.RedisPort <= 0 || cfg.RedisPort > maxPort {
			errs = append(errs, fmt.Errorf("REDIS_PORT out of range: %d", cfg.RedisPort))
		}
	}

	cfg.JWTSecret = mustString("JWT_SECRET", &errs)

	cfg.JWTAccessTTLMinutes = mustInt(getDefault("JWT_ACCESS_TTL_MINUTES", defaultJWTAccessTTLMinutes), "JWT_ACCESS_TTL_MINUTES", &errs)
	cfg.RefreshTTLDays = mustInt(getDefault("REFRESH_TTL_DAYS", defaultRefreshTTLDays), "REFRESH_TTL_DAYS", &errs)

	cfg.BudgetsEnforceExpenseCategories = mustBool(
		getDefault("BUDGETS_ENFORCE_EXPENSE_CATEGORIES", defaultBudgetsEnforceExpenseCategories),
		"BUDGETS_ENFORCE_EXPENSE_CATEGORIES",
		&errs,
	)

	if cfg.JWTAccessTTLMinutes <= 0 || cfg.JWTAccessTTLMinutes > 24*60 {
		errs = append(errs, fmt.Errorf("JWT_ACCESS_TTL_MINUTES out of range: %d", cfg.JWTAccessTTLMinutes))
	}
	if cfg.RefreshTTLDays <= 0 || cfg.RefreshTTLDays > 365 {
		errs = append(errs, fmt.Errorf("REFRESH_TTL_DAYS out of range: %d", cfg.RefreshTTLDays))
	}
	if cfg.DBPort <= 0 || cfg.DBPort > maxPort {
		errs = append(errs, fmt.Errorf("DB_PORT out of range: %d", cfg.DBPort))
	}
	if cfg.AppPort <= 0 || cfg.AppPort > maxPort {
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
	raw = strings.TrimSpace(raw)
	if raw == "" {
		*errs = append(*errs, fmt.Errorf("missing required env: %s", key))
		return false
	}
	v, _, err := parseBoolOptional(raw, key)
	if err != nil {
		*errs = append(*errs, err)
		return false
	}
	return v
}

func parseBoolOptional(raw string, key string) (bool, bool, error) {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return false, false, nil
	}
	switch raw {
	case "1", "true", "yes", "y", "on":
		return true, true, nil
	case "0", "false", "no", "n", "off":
		return false, true, nil
	default:
		return false, true, fmt.Errorf("invalid %s=%q (expected bool)", key, raw)
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
	return buildPostgresURL(c.DBHost, c.DBPort, c.DBUser, c.DBPassword, c.DBName, c.DBSSLMode)
}

func buildPostgresURL(host string, port int, user, password, dbname, sslmode string) string {
	u := url.URL{
		Scheme: "postgres",
		Host:   fmt.Sprintf("%s:%d", strings.TrimSpace(host), port),
		Path:   "/" + strings.TrimPrefix(strings.TrimSpace(dbname), "/"),
	}
	if strings.TrimSpace(user) != "" {
		if strings.TrimSpace(password) != "" {
			u.User = url.UserPassword(strings.TrimSpace(user), password)
		} else {
			u.User = url.User(strings.TrimSpace(user))
		}
	}
	q := u.Query()
	if strings.TrimSpace(sslmode) != "" {
		q.Set("sslmode", strings.TrimSpace(sslmode))
	}
	u.RawQuery = q.Encode()
	return u.String()
}

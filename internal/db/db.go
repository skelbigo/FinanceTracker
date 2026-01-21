package db

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PoolConfig struct {
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
	PingTimeout     time.Duration
}

func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MaxConns:        10,
		MinConns:        1,
		MaxConnLifetime: time.Hour,
		PingTimeout:     3 * time.Second,
	}
}

func NewPostgresPool(ctx context.Context, dbc DBConfig) (*pgxpool.Pool, error) {
	return NewPostgresPoolWithConfig(ctx, dbc, DefaultPoolConfig())
}

func NewPostgresPoolWithConfig(ctx context.Context, dbc DBConfig, pc PoolConfig) (*pgxpool.Pool, error) {
	connStr := BuildPostgresURL(dbc)

	cfg, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, err
	}

	cfg.MaxConns = pc.MaxConns
	cfg.MinConns = pc.MinConns
	cfg.MaxConnLifetime = pc.MaxConnLifetime

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}

	pingCtx, cancel := context.WithTimeout(context.Background(), pc.PingTimeout)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, err
	}

	return pool, nil
}

func MaskedURL(c DBConfig) string {
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(c.User, "******"),
		Host:   fmt.Sprintf("%s:%d", c.Host, c.Port),
		Path:   c.Name,
	}
	q := u.Query()
	q.Set("sslmode", c.SSLMode)
	u.RawQuery = q.Encode()
	return u.String()
}

func NewPostgresPoolFromURL(ctx context.Context, dbURL string) (*pgxpool.Pool, error) {
	return NewPostgresPoolFromURLWithConfig(ctx, dbURL, DefaultPoolConfig())
}

func NewPostgresPoolFromURLWithConfig(ctx context.Context, dbURL string, pc PoolConfig) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		return nil, err
	}
	cfg.MaxConns = pc.MaxConns
	cfg.MinConns = pc.MinConns
	cfg.MaxConnLifetime = pc.MaxConnLifetime

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}

	pingCtx, cancel := context.WithTimeout(context.Background(), pc.PingTimeout)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

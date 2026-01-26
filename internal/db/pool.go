package db

import (
	"context"
	"errors"
	"fmt"
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

func (pc PoolConfig) Validate() error {
	if pc.PingTimeout <= 0 {
		return errors.New("PingTimeout must be > 0")
	}
	if pc.MaxConns <= 0 {
		return errors.New("MaxConns must be > 0")
	}
	if pc.MinConns < 0 {
		return errors.New("MinConns must be >= 0")
	}
	if pc.MaxConns < pc.MinConns {
		return fmt.Errorf("MaxConns (%d) must be >= MinConns (%d)", pc.MaxConns, pc.MinConns)
	}
	if pc.MaxConnLifetime < 0 {
		return errors.New("MaxConnLifetime must be >= 0")
	}
	return nil
}

func NewPostgresPoolFromURL(ctx context.Context, dbURL string) (*pgxpool.Pool, error) {
	return NewPostgresPoolFromURLWithConfig(ctx, dbURL, DefaultPoolConfig())
}

func NewPostgresPoolFromURLWithConfig(ctx context.Context, dbURL string, pc PoolConfig) (*pgxpool.Pool, error) {
	if err := pc.Validate(); err != nil {
		return nil, fmt.Errorf("invalid pool config: %w", err)
	}

	cfg, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		return nil, fmt.Errorf("parse pgxpool config: %w", err)
	}

	cfg.MaxConns = pc.MaxConns
	cfg.MinConns = pc.MinConns
	cfg.MaxConnLifetime = pc.MaxConnLifetime

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pgxpool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, pc.PingTimeout)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return pool, nil
}

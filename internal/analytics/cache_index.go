package analytics

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/skelbigo/FinanceTracker/internal/redisx"
)

type CacheIndex interface {
	TrackKey(ctx context.Context, workspaceID uuid.UUID, cacheKey string) error
	InvalidateWorkspace(ctx context.Context, workspaceID uuid.UUID) error
}

type noopCacheIndex struct{}

func (n noopCacheIndex) TrackKey(ctx context.Context, workspaceID uuid.UUID, cacheKey string) error {
	return nil
}
func (n noopCacheIndex) InvalidateWorkspace(ctx context.Context, workspaceID uuid.UUID) error {
	return nil
}

func NewCacheIndex(rdb *redisx.Client) CacheIndex {
	if rdb == nil {
		return noopCacheIndex{}
	}
	return &redisCacheIndex{rdb: rdb}
}

type redisCacheIndex struct {
	rdb *redisx.Client
}

const (
	analyticsKeysPrefix = "analyticsKeys"
	analyticsKeysSetTTL = 24 * time.Hour
)

func (i *redisCacheIndex) setKey(workspaceID uuid.UUID) string {
	return fmt.Sprintf("%s:%s", analyticsKeysPrefix, workspaceID.String())
}

func (i *redisCacheIndex) TrackKey(ctx context.Context, workspaceID uuid.UUID, cacheKey string) error {
	if cacheKey == "" {
		return nil
	}
	k := i.setKey(workspaceID)
	if _, err := i.rdb.Pipeline(ctx,
		[]string{"SADD", k, cacheKey},
		[]string{"EXPIRE", k, fmt.Sprintf("%d", int64(analyticsKeysSetTTL/time.Second))},
	); err != nil {
		return err
	}
	return nil
}

func (i *redisCacheIndex) InvalidateWorkspace(ctx context.Context, workspaceID uuid.UUID) error {
	k := i.setKey(workspaceID)

	keys, err := i.rdb.SMembers(ctx, k)
	if err != nil {
		return err
	}

	if len(keys) > 0 {
		keys = append(keys, k)
		return i.rdb.Del(ctx, keys...)
	}
	return i.rdb.Del(ctx, k)
}

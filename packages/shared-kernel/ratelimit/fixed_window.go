package ratelimit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync"
	"time"

	"github.com/skelbigo/FinanceTracker/packages/shared-kernel/redisx"
)

type FixedWindowLimiter struct {
	prefix string
	max    int64
	window time.Duration

	rdb *redisx.Client
	mem *memFixedWindow
}

type memFixedWindow struct {
	mu sync.Mutex
	m  map[string]*memEntry
}

type memEntry struct {
	count   int64
	resetAt time.Time
}

func NewFixedWindowLimiter(prefix string, max int, window time.Duration, rdb *redisx.Client) *FixedWindowLimiter {
	return &FixedWindowLimiter{
		prefix: strings.TrimSpace(prefix),
		max:    int64(max),
		window: window,
		rdb:    rdb,
		mem:    &memFixedWindow{m: make(map[string]*memEntry)},
	}
}

func (l *FixedWindowLimiter) key(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if l.prefix == "" {
		return raw
	}
	return l.prefix + ":" + raw
}

func (l *FixedWindowLimiter) Allow(ctx context.Context, rawKey string) (allowed bool, retryAfter time.Duration, err error) {
	if l == nil {
		return true, 0, nil
	}
	if l.window <= 0 || l.max <= 0 {
		return true, 0, nil
	}
	key := l.key(rawKey)
	if key == "" {
		return true, 0, nil
	}

	if l.rdb != nil {
		n, rerr := l.rdb.Incr(ctx, key)
		if rerr == nil {
			if n == 1 {
				_ = l.rdb.Expire(ctx, key, l.window)
			}
			if n > l.max {
				ttl, _, terr := l.rdb.TTL(ctx, key)
				if terr == nil && ttl > 0 {
					return false, ttl, nil
				}
				return false, l.window, nil
			}
			return true, 0, nil
		}
	}

	return l.memAllow(rawKey)
}

func (l *FixedWindowLimiter) Reset(ctx context.Context, rawKey string) {
	if l == nil {
		return
	}
	key := l.key(rawKey)
	if key == "" {
		return
	}
	if l.rdb != nil {
		_ = l.rdb.Del(ctx, key)
	}
	l.memReset(rawKey)
}

func (l *FixedWindowLimiter) memAllow(rawKey string) (bool, time.Duration, error) {
	key := l.key(rawKey)
	now := time.Now()

	l.mem.mu.Lock()
	defer l.mem.mu.Unlock()

	e := l.mem.m[key]
	if e == nil || now.After(e.resetAt) {
		e = &memEntry{count: 0, resetAt: now.Add(l.window)}
		l.mem.m[key] = e
	}

	e.count++
	if e.count > l.max {
		retry := time.Until(e.resetAt)
		if retry < time.Second {
			retry = time.Second
		}
		return false, retry, nil
	}
	return true, 0, nil
}

func (l *FixedWindowLimiter) memReset(rawKey string) {
	key := l.key(rawKey)
	l.mem.mu.Lock()
	defer l.mem.mu.Unlock()
	delete(l.mem.m, key)
}

func hashKey(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

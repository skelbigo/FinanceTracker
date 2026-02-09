package ratelimit

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/skelbigo/FinanceTracker/packages/shared-kernel/config"
	"github.com/skelbigo/FinanceTracker/packages/shared-kernel/redisx"
)

type LoginLimiter struct {
	Enabled bool

	perIP    *FixedWindowLimiter
	perEmail *FixedWindowLimiter
}

func NewLoginLimiter(cfg config.Config, rdb *redisx.Client) *LoginLimiter {
	window := time.Duration(cfg.LoginRateLimitWindowSeconds) * time.Second
	return &LoginLimiter{
		Enabled:  cfg.LoginRateLimitEnabled,
		perIP:    NewFixedWindowLimiter("rl:login:ip", cfg.LoginRateLimitMaxAttemptsPerIP, window, rdb),
		perEmail: NewFixedWindowLimiter("rl:login:email", cfg.LoginRateLimitMaxAttemptsPerEmail, window, rdb),
	}
}

func (l *LoginLimiter) Allow(ctx context.Context, ip, email string) (allowed bool, retryAfter time.Duration, err error) {
	if l == nil || !l.Enabled {
		return true, 0, nil
	}
	var retry time.Duration

	ip = strings.TrimSpace(ip)
	if ip != "" {
		ok, ra, e := l.perIP.Allow(ctx, ip)
		if e != nil {
			return false, 0, e
		}
		if !ok {
			retry = maxDur(retry, ra)
		}
	}

	email = normalizeEmail(email)
	if email != "" {
		h := hashKey(email)
		ok, ra, e := l.perEmail.Allow(ctx, h)
		if e != nil {
			return false, 0, e
		}
		if !ok {
			retry = maxDur(retry, ra)
		}
	}

	if retry > 0 {
		return false, retry, nil
	}
	return true, 0, nil
}

func (l *LoginLimiter) Reset(ctx context.Context, ip, email string) {
	if l == nil || !l.Enabled {
		return
	}
	ip = strings.TrimSpace(ip)
	if ip != "" {
		l.perIP.Reset(ctx, ip)
	}
	email = normalizeEmail(email)
	if email != "" {
		l.perEmail.Reset(ctx, hashKey(email))
	}
}

func (l *LoginLimiter) RetryAfterHeader(retryAfter time.Duration) string {
	secs := int64(retryAfter / time.Second)
	if secs < 1 {
		secs = 1
	}
	return fmt.Sprintf("%d", secs)
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func maxDur(a, b time.Duration) time.Duration {
	if b > a {
		return b
	}
	return a
}

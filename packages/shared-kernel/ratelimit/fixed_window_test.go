package ratelimit

import (
	"context"
	"testing"
	"time"
)

func TestFixedWindowLimiter_Memory(t *testing.T) {
	l := NewFixedWindowLimiter("t", 2, 80*time.Millisecond, nil)
	ctx := context.Background()

	ok, _, err := l.Allow(ctx, "k")
	if err != nil || !ok {
		t.Fatalf("expected 1st allow, got ok=%v err=%v", ok, err)
	}
	ok, _, err = l.Allow(ctx, "k")
	if err != nil || !ok {
		t.Fatalf("expected 2nd allow, got ok=%v err=%v", ok, err)
	}
	ok, retry, err := l.Allow(ctx, "k")
	if err != nil || ok {
		t.Fatalf("expected block on 3rd, got ok=%v err=%v", ok, err)
	}
	if retry <= 0 {
		t.Fatalf("expected retryAfter > 0")
	}

	l.Reset(ctx, "k")
	ok, _, err = l.Allow(ctx, "k")
	if err != nil || !ok {
		t.Fatalf("expected allow after reset, got ok=%v err=%v", ok, err)
	}

	_ = ok
	time.Sleep(90 * time.Millisecond)
	ok, _, err = l.Allow(ctx, "k")
	if err != nil || !ok {
		t.Fatalf("expected allow after window, got ok=%v err=%v", ok, err)
	}
}

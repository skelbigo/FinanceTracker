package analytics

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestBuildAnalyticsCacheKey_FormatAndNormalization(t *testing.T) {
	ws := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	from := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	toExclusive := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	key, err := BuildAnalyticsCacheKey(ws, "uah", from, toExclusive, BucketDay)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "analytics:11111111-1111-1111-1111-111111111111:UAH:2026-02-01:2026-02-28:day"
	if key != want {
		t.Fatalf("unexpected key\nwant: %s\n got: %s", want, key)
	}
}

func TestBuildAnalyticsCacheKey_DefaultBucketIsDay(t *testing.T) {
	ws := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	toExclusive := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)

	key, err := BuildAnalyticsCacheKey(ws, "USD", from, toExclusive, Bucket(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key[len(key)-3:] != "day" {
		t.Fatalf("expected default bucket 'day', got key: %s", key)
	}
}

func TestBuildAnalyticsCacheKey_InvalidCurrency(t *testing.T) {
	ws := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	toExclusive := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)

	_, err := BuildAnalyticsCacheKey(ws, "U", from, toExclusive, BucketDay)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

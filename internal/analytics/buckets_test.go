package analytics

import (
	"testing"
	"time"
)

func TestTruncateToBucket(t *testing.T) {
	t.Run("day", func(t *testing.T) {
		in := time.Date(2026, 2, 5, 13, 45, 12, 999, time.UTC)
		got := truncateToBucket(in, BucketDay)
		want := time.Date(2026, 2, 5, 0, 0, 0, 0, time.UTC)
		if !got.Equal(want) {
			t.Fatalf("day: got %v want %v", got, want)
		}
	})

	t.Run("week_monday_start", func(t *testing.T) {
		in := time.Date(2026, 2, 5, 10, 0, 0, 0, time.UTC)
		got := truncateToBucket(in, BucketWeek)
		want := time.Date(2026, 2, 2, 0, 0, 0, 0, time.UTC)
		if !got.Equal(want) {
			t.Fatalf("week: got %v want %v", got, want)
		}
	})

	t.Run("month", func(t *testing.T) {
		in := time.Date(2026, 2, 20, 23, 59, 59, 0, time.UTC)
		got := truncateToBucket(in, BucketMonth)
		want := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
		if !got.Equal(want) {
			t.Fatalf("month: got %v want %v", got, want)
		}
	})
}

func TestAddBucket(t *testing.T) {
	t.Run("day", func(t *testing.T) {
		in := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
		got := addBucket(in, BucketDay)
		want := time.Date(2026, 2, 2, 0, 0, 0, 0, time.UTC)
		if !got.Equal(want) {
			t.Fatalf("day: got %v want %v", got, want)
		}
	})

	t.Run("week", func(t *testing.T) {
		in := time.Date(2026, 2, 2, 0, 0, 0, 0, time.UTC)
		got := addBucket(in, BucketWeek)
		want := time.Date(2026, 2, 9, 0, 0, 0, 0, time.UTC)
		if !got.Equal(want) {
			t.Fatalf("week: got %v want %v", got, want)
		}
	})

	t.Run("month", func(t *testing.T) {
		in := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
		got := addBucket(in, BucketMonth)
		want := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
		if !got.Equal(want) {
			t.Fatalf("month: got %v want %v", got, want)
		}
	})
}

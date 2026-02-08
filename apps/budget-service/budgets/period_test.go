package budgets

import (
	"testing"
	"time"
)

func TestPeriodBounds_Month(t *testing.T) {
	loc := time.FixedZone("T", 2*60*60)
	now := time.Date(2026, time.February, 15, 13, 45, 0, 0, loc)

	start, end := PeriodBounds(now, PeriodMonth)

	wantStart := time.Date(2026, time.February, 1, 0, 0, 0, 0, loc)
	wantEnd := time.Date(2026, time.March, 1, 0, 0, 0, 0, loc)

	if !start.Equal(wantStart) {
		t.Fatalf("start: got %v want %v", start, wantStart)
	}
	if !end.Equal(wantEnd) {
		t.Fatalf("end: got %v want %v", end, wantEnd)
	}
}

func TestPeriodBounds_Week_MondayStart(t *testing.T) {
	loc := time.FixedZone("T", 0)

	now := time.Date(2026, time.February, 4, 18, 0, 0, 0, loc)

	start, end := PeriodBounds(now, PeriodWeek)

	wantStart := time.Date(2026, time.February, 2, 0, 0, 0, 0, loc)
	wantEnd := time.Date(2026, time.February, 9, 0, 0, 0, 0, loc)

	if !start.Equal(wantStart) {
		t.Fatalf("start: got %v want %v", start, wantStart)
	}
	if !end.Equal(wantEnd) {
		t.Fatalf("end: got %v want %v", end, wantEnd)
	}
}

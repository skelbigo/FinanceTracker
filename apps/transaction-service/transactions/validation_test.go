package transactions

import (
	"testing"
	"time"
)

func TestParseAmountMinor(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		want int64
	}{
		{"12.34", 1234},
		{"12", 1200},
		{"12.3", 1230},
		{"0.01", 1},
		{".5", 50},
		{"12,34", 1234},
		{"  7.00 ", 700},
		{"12.", 1200},
	}

	for _, tc := range cases {
		got, err := ParseAmountMinor(tc.in)
		if err != nil {
			t.Fatalf("ParseAmountMinor(%q) err=%v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("ParseAmountMinor(%q) = %d; want %d", tc.in, got, tc.want)
		}
	}

	bad := []string{"", "0", "0.00", "-1", "1.234", "abc", "1..2", "--1", "12.3.4"}
	for _, in := range bad {
		if _, err := ParseAmountMinor(in); err == nil {
			t.Errorf("ParseAmountMinor(%q) expected error", in)
		}
	}
}

func TestTagsParsingAndNormalization(t *testing.T) {
	t.Parallel()

	got, err := ParseTagsCSV(" Food, lunch , , FOOD ,  ")
	if err != nil {
		t.Fatalf("ParseTagsCSV err=%v", err)
	}
	if len(got) != 2 || got[0] != "food" || got[1] != "lunch" {
		t.Fatalf("ParseTagsCSV unexpected result: %#v", got)
	}

	many := "t1,t2,t3,t4,t5,t6,t7,t8,t9,t10,t11"
	got, err = ParseTagsCSV(many)
	if err != nil {
		t.Fatalf("ParseTagsCSV(many) err=%v", err)
	}
	if len(got) != 10 {
		t.Fatalf("expected 10 tags, got %d: %#v", len(got), got)
	}

	got, err = NormalizeTagsSlice([]string{" A ", "b", "", "B", "c"})
	if err != nil {
		t.Fatalf("NormalizeTagsSlice err=%v", err)
	}
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Fatalf("NormalizeTagsSlice unexpected result: %#v", got)
	}
}

func TestCurrencyNormalizationStrict(t *testing.T) {
	t.Parallel()

	if got, err := NormalizeCurrencyStrict("UAH"); err != nil || got != "UAH" {
		t.Fatalf("NormalizeCurrencyStrict(UAH) = %q, %v; want UAH, nil", got, err)
	}
	bad := []string{"uah", "UaH", "US", "USDT", "12A", " EU R ", " UAH"}
	for _, in := range bad {
		if _, err := NormalizeCurrencyStrict(in); err == nil {
			t.Errorf("NormalizeCurrencyStrict(%q) expected error", in)
		}
	}
}

func TestParseOccurredAt(t *testing.T) {
	t.Parallel()

	d, err := ParseOccurredAt("2026-01-02")
	if err != nil {
		t.Fatalf("ParseOccurredAt(date) err=%v", err)
	}
	if d.Format("2006-01-02") != "2026-01-02" {
		t.Fatalf("unexpected date: %s", d.Format(time.RFC3339))
	}

	ts, err := ParseOccurredAt("2026-01-02T03:04:05Z")
	if err != nil {
		t.Fatalf("ParseOccurredAt(rfc3339) err=%v", err)
	}
	if ts.UTC().Format(time.RFC3339) != "2026-01-02T03:04:05Z" {
		t.Fatalf("unexpected ts: %s", ts.UTC().Format(time.RFC3339))
	}
}

func TestNormalizeOptionalUUID(t *testing.T) {
	t.Parallel()

	if got, err := NormalizeOptionalUUID(nil); err != nil || got != nil {
		t.Fatalf("expected nil,nil got=%v err=%v", got, err)
	}

	blank := "   "
	if got, err := NormalizeOptionalUUID(&blank); err != nil || got != nil {
		t.Fatalf("expected nil,nil for blank got=%v err=%v", got, err)
	}

	bad := "not-a-uuid"
	if _, err := NormalizeOptionalUUID(&bad); err == nil {
		t.Fatalf("expected error for invalid uuid")
	}

	good := "550e8400-e29b-41d4-a716-446655440000"
	got, err := NormalizeOptionalUUID(&good)
	if err != nil || got == nil || *got != good {
		t.Fatalf("expected %q,nil got=%v err=%v", good, got, err)
	}
}

func TestValidateType(t *testing.T) {
	t.Parallel()

	if !ValidateType(TypeIncome) {
		t.Fatalf("TypeIncome should be valid")
	}
	if !ValidateType(TypeExpense) {
		t.Fatalf("TypeExpense should be valid")
	}
	if ValidateType(Type("other")) {
		t.Fatalf("unexpected valid type")
	}
}

func TestValidateDateRange(t *testing.T) {
	t.Parallel()

	from := time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 1, 9, 0, 0, 0, 0, time.UTC)
	if err := ValidateDateRange(&from, &to); err != ErrInvalidRange {
		t.Fatalf("expected ErrInvalidRange, got %v", err)
	}

	equal := time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)
	if err := ValidateDateRange(&from, &equal); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if err := ValidateDateRange(nil, &equal); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestOrderByFromSort(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"":                 "occurred_at DESC",
		"occurred_at_desc": "occurred_at DESC",
		"occurred_at_asc":  "occurred_at ASC",
		"amount_desc":      "amount_minor DESC",
		"amount_asc":       "amount_minor ASC",
		"  amount_asc ":    "amount_minor ASC",
		"DROP TABLE":       "occurred_at DESC",
	}
	for in, want := range cases {
		got := orderByFromSort(in)
		if got != want {
			t.Fatalf("orderByFromSort(%q)=%q want %q", in, got, want)
		}
	}
}

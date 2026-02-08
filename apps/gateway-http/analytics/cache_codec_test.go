package analytics

import (
	"math"
	"testing"
)

func TestMarshalUnmarshalAnalyticsResponse_RoundTrip(t *testing.T) {
	catFood := "11111111-1111-1111-1111-111111111111"

	in := AnalyticsResponse{
		Range:  AnalyticsRange{From: "2026-02-01", To: "2026-02-29", GroupBy: "day"},
		Totals: AnalyticsTotals{Income: 12000, Expense: 8300, Net: 3700},
		TopCategories: []AnalyticsTopCategory{
			{CategoryID: &catFood, Name: "Food", Type: "expense", Total: 2400, Share: 0.289},
			{CategoryID: nil, Name: "Uncategorized", Type: "expense", Total: 200, Share: 0.024},
		},
		Cashflow: []AnalyticsCashflowPoint{
			{Bucket: "2026-02-01", Income: 0, Expense: 300, Net: -300},
			{Bucket: "2026-02-02", Income: 500, Expense: 0, Net: 500},
		},
	}

	b, err := MarshalAnalyticsResponse(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	out, err := UnmarshalAnalyticsResponse(b)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if out.Range != in.Range {
		t.Fatalf("range mismatch: %#v != %#v", out.Range, in.Range)
	}
	if out.Totals != in.Totals {
		t.Fatalf("totals mismatch: %#v != %#v", out.Totals, in.Totals)
	}

	if len(out.TopCategories) != len(in.TopCategories) {
		t.Fatalf("topCategories len mismatch: %d != %d", len(out.TopCategories), len(in.TopCategories))
	}
	for i := range in.TopCategories {
		if (out.TopCategories[i].CategoryID == nil) != (in.TopCategories[i].CategoryID == nil) {
			t.Fatalf("categoryId nil mismatch at %d", i)
		}
		if out.TopCategories[i].CategoryID != nil && *out.TopCategories[i].CategoryID != *in.TopCategories[i].CategoryID {
			t.Fatalf("categoryId mismatch at %d", i)
		}
		if out.TopCategories[i].Name != in.TopCategories[i].Name || out.TopCategories[i].Type != in.TopCategories[i].Type || out.TopCategories[i].Total != in.TopCategories[i].Total {
			t.Fatalf("top category mismatch at %d: %#v != %#v", i, out.TopCategories[i], in.TopCategories[i])
		}
		if math.Abs(out.TopCategories[i].Share-in.TopCategories[i].Share) > 1e-9 {
			t.Fatalf("share mismatch at %d: %v != %v", i, out.TopCategories[i].Share, in.TopCategories[i].Share)
		}
	}

	if len(out.Cashflow) != len(in.Cashflow) {
		t.Fatalf("cashflow len mismatch: %d != %d", len(out.Cashflow), len(in.Cashflow))
	}
	for i := range in.Cashflow {
		if out.Cashflow[i] != in.Cashflow[i] {
			t.Fatalf("cashflow mismatch at %d: %#v != %#v", i, out.Cashflow[i], in.Cashflow[i])
		}
	}
}

func TestUnmarshalAnalyticsResponse_Empty(t *testing.T) {
	_, err := UnmarshalAnalyticsResponse(nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

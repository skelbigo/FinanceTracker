package analytics

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

type repoStub struct {
	summaryFn    func(ctx context.Context, workspaceID uuid.UUID, fromInclusive, toExclusive time.Time, currency string) (Summary, error)
	byCategoryFn func(ctx context.Context, workspaceID uuid.UUID, fromInclusive, toExclusive time.Time, currency string, typ TxType, top int) ([]CategoryTotalRow, int64, error)
	timeseriesFn func(ctx context.Context, workspaceID uuid.UUID, fromInclusive, toExclusive time.Time, currency string, bucket Bucket, typ TxType) ([]TimeseriesRow, error)
	cashflowFn   func(ctx context.Context, workspaceID uuid.UUID, fromInclusive, toExclusive time.Time, currency string, bucket Bucket) ([]CashflowRow, error)
}

func (r *repoStub) Summary(ctx context.Context, workspaceID uuid.UUID, fromInclusive, toExclusive time.Time, currency string) (Summary, error) {
	if r.summaryFn == nil {
		return Summary{}, nil
	}
	return r.summaryFn(ctx, workspaceID, fromInclusive, toExclusive, currency)
}

func (r *repoStub) ByCategory(ctx context.Context, workspaceID uuid.UUID, fromInclusive, toExclusive time.Time, currency string, typ TxType, top int) ([]CategoryTotalRow, int64, error) {
	if r.byCategoryFn == nil {
		return nil, 0, nil
	}
	return r.byCategoryFn(ctx, workspaceID, fromInclusive, toExclusive, currency, typ, top)
}

func (r *repoStub) Timeseries(ctx context.Context, workspaceID uuid.UUID, fromInclusive, toExclusive time.Time, currency string, bucket Bucket, typ TxType) ([]TimeseriesRow, error) {
	if r.timeseriesFn == nil {
		return nil, nil
	}
	return r.timeseriesFn(ctx, workspaceID, fromInclusive, toExclusive, currency, bucket, typ)
}

func (r *repoStub) Cashflow(ctx context.Context, workspaceID uuid.UUID, fromInclusive, toExclusive time.Time, currency string, bucket Bucket) ([]CashflowRow, error) {
	if r.cashflowFn == nil {
		return nil, nil
	}
	return r.cashflowFn(ctx, workspaceID, fromInclusive, toExclusive, currency, bucket)
}

func TestTimeseries_FillsMissingBuckets(t *testing.T) {
	ws := uuid.New()
	repo := &repoStub{}
	repo.timeseriesFn = func(ctx context.Context, workspaceID uuid.UUID, fromInclusive, toExclusive time.Time, currency string, bucket Bucket, typ TxType) ([]TimeseriesRow, error) {
		return []TimeseriesRow{
			{PeriodStart: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), Total: 100},
			{PeriodStart: time.Date(2026, 2, 3, 0, 0, 0, 0, time.UTC), Total: 200},
		}, nil
	}

	svc := NewService(repo, nil, nil, 0)
	resp, err := svc.Timeseries(context.Background(), ws, "2026-02-01", "2026-02-03", "UAH", "day", "expense")
	if err != nil {
		t.Fatalf("Timeseries: %v", err)
	}
	if len(resp.Points) != 3 {
		t.Fatalf("expected 3 points, got %d", len(resp.Points))
	}

	if resp.Points[0].Period != "2026-02-01" || resp.Points[0].Total != 100 {
		t.Fatalf("p0 unexpected: %+v", resp.Points[0])
	}
	if resp.Points[1].Period != "2026-02-02" || resp.Points[1].Total != 0 {
		t.Fatalf("p1 expected gap=0: %+v", resp.Points[1])
	}
	if resp.Points[2].Period != "2026-02-03" || resp.Points[2].Total != 200 {
		t.Fatalf("p2 unexpected: %+v", resp.Points[2])
	}
}

func TestAnalytics_FillsMissingCashflowBuckets(t *testing.T) {
	ws := uuid.New()
	repo := &repoStub{}
	repo.summaryFn = func(ctx context.Context, workspaceID uuid.UUID, fromInclusive, toExclusive time.Time, currency string) (Summary, error) {
		return Summary{IncomeTotal: 100, ExpenseTotal: 60, Net: 40}, nil
	}
	repo.byCategoryFn = func(ctx context.Context, workspaceID uuid.UUID, fromInclusive, toExclusive time.Time, currency string, typ TxType, top int) ([]CategoryTotalRow, int64, error) {
		return nil, 0, nil
	}
	repo.cashflowFn = func(ctx context.Context, workspaceID uuid.UUID, fromInclusive, toExclusive time.Time, currency string, bucket Bucket) ([]CashflowRow, error) {
		return []CashflowRow{
			{PeriodStart: time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC), IncomeTotal: 100, ExpenseTotal: 40},
			{PeriodStart: time.Date(2026, 2, 3, 12, 0, 0, 0, time.UTC), IncomeTotal: 0, ExpenseTotal: 20},
		}, nil
	}

	svc := NewService(repo, nil, nil, 0)
	resp, err := svc.Analytics(context.Background(), ws, "2026-02-01", "2026-02-03", "UAH", "day", 10)
	if err != nil {
		t.Fatalf("Analytics: %v", err)
	}
	if len(resp.Cashflow) != 3 {
		t.Fatalf("expected 3 cashflow points, got %d", len(resp.Cashflow))
	}

	if resp.Cashflow[0].Bucket != "2026-02-01" || resp.Cashflow[0].Income != 100 || resp.Cashflow[0].Expense != 40 || resp.Cashflow[0].Net != 60 {
		t.Fatalf("day1 unexpected: %+v", resp.Cashflow[0])
	}
	if resp.Cashflow[1].Bucket != "2026-02-02" || resp.Cashflow[1].Income != 0 || resp.Cashflow[1].Expense != 0 || resp.Cashflow[1].Net != 0 {
		t.Fatalf("day2 expected gap=0: %+v", resp.Cashflow[1])
	}
	if resp.Cashflow[2].Bucket != "2026-02-03" || resp.Cashflow[2].Income != 0 || resp.Cashflow[2].Expense != 20 || resp.Cashflow[2].Net != -20 {
		t.Fatalf("day3 unexpected: %+v", resp.Cashflow[2])
	}
}

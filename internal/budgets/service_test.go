package budgets

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

type fakeRepo struct {
	budgets []Budget
	spent   map[string]int64
	events  []BudgetEvent
}

func keySpent(categoryID uuid.UUID, currency string) string {
	return categoryID.String() + ":" + currency
}

func (r *fakeRepo) Upsert(ctx context.Context, workspaceID uuid.UUID, req UpsertBudgetRequest) (Budget, error) {
	return Budget{}, nil
}

func (r *fakeRepo) InsertBudget(ctx context.Context, workspaceID uuid.UUID, req UpsertBudgetRequest) (Budget, error) {
	b := Budget{
		ID:               uuid.New(),
		WorkspaceID:      workspaceID,
		CategoryID:       req.CategoryID,
		Period:           req.Period,
		AmountLimitMinor: req.AmountLimitMinor,
		Currency:         req.Currency,
		CreatedAt:        time.Now(),
	}
	r.budgets = append(r.budgets, b)
	return b, nil
}

func (r *fakeRepo) UpdateBudget(ctx context.Context, workspaceID, budgetID uuid.UUID, req UpsertBudgetRequest) (Budget, error) {
	return Budget{}, nil
}

func (r *fakeRepo) DeleteBudget(ctx context.Context, workspaceID, budgetID uuid.UUID) error {
	return nil
}

func (r *fakeRepo) GetBudgetByID(ctx context.Context, workspaceID, budgetID uuid.UUID) (Budget, error) {
	for _, b := range r.budgets {
		if b.ID == budgetID {
			return b, nil
		}
	}
	return Budget{}, ErrBudgetNotFound
}

func (r *fakeRepo) List(ctx context.Context, workspaceID uuid.UUID, period *Period) ([]Budget, error) {
	if period == nil {
		return append([]Budget(nil), r.budgets...), nil
	}
	out := make([]Budget, 0)
	for _, b := range r.budgets {
		if b.Period == *period {
			out = append(out, b)
		}
	}
	return out, nil
}

func (r *fakeRepo) ListByCategory(ctx context.Context, workspaceID, categoryID uuid.UUID, currency string) ([]Budget, error) {
	out := make([]Budget, 0)
	for _, b := range r.budgets {
		if b.CategoryID == categoryID && b.Currency == currency {
			out = append(out, b)
		}
	}
	return out, nil
}

func (r *fakeRepo) GetSpentForCategory(ctx context.Context, workspaceID, categoryID uuid.UUID, currency string, from, to time.Time) (int64, error) {
	return r.spent[keySpent(categoryID, currency)], nil
}

func (r *fakeRepo) InsertBudgetEvent(ctx context.Context, ev BudgetEvent) error {
	r.events = append(r.events, ev)
	return nil
}

func TestService_ListWithProgress(t *testing.T) {
	wsID := uuid.New()
	catID := uuid.New()
	bID := uuid.New()

	repo := &fakeRepo{
		budgets: []Budget{{
			ID:               bID,
			WorkspaceID:      wsID,
			CategoryID:       catID,
			Period:           PeriodMonth,
			AmountLimitMinor: 1000,
			Currency:         "UAH",
		}},
		spent: map[string]int64{keySpent(catID, "UAH"): 250},
	}

	svc := NewService(repo, nil, false)

	now := time.Date(2026, time.February, 15, 12, 0, 0, 0, time.UTC)
	items, err := svc.ListWithProgress(context.Background(), wsID, nil, now)
	if err != nil {
		t.Fatalf("ListWithProgress error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items len: got %d want %d", len(items), 1)
	}
	got := items[0]
	if got.SpentMinor != 250 {
		t.Fatalf("spent: got %d want %d", got.SpentMinor, 250)
	}
	if got.RemainingMinor != 750 {
		t.Fatalf("remaining: got %d want %d", got.RemainingMinor, 750)
	}
	if got.PercentUsed != 25 {
		t.Fatalf("percent: got %d want %d", got.PercentUsed, 25)
	}
	if got.IsOver {
		t.Fatalf("is_over: got true want false")
	}
}

func TestService_CheckOverspend_InsertsEvent(t *testing.T) {
	wsID := uuid.New()
	catID := uuid.New()
	bID := uuid.New()

	repo := &fakeRepo{
		budgets: []Budget{{
			ID:               bID,
			WorkspaceID:      wsID,
			CategoryID:       catID,
			Period:           PeriodWeek,
			AmountLimitMinor: 1000,
			Currency:         "UAH",
		}},
		spent: map[string]int64{keySpent(catID, "UAH"): 1200},
	}

	svc := NewService(repo, nil, false)

	now := time.Date(2026, time.February, 4, 18, 0, 0, 0, time.UTC)
	overspent, err := svc.CheckOverspend(context.Background(), wsID, catID, "UAH", now)
	if err != nil {
		t.Fatalf("CheckOverspend error: %v", err)
	}
	if !overspent {
		t.Fatalf("overspent: got false want true")
	}
	if len(repo.events) != 1 {
		t.Fatalf("events len: got %d want %d", len(repo.events), 1)
	}
	ev := repo.events[0]
	if ev.WorkspaceID != wsID || ev.BudgetID != bID || ev.CategoryID != catID {
		t.Fatalf("event ids mismatch")
	}
	if ev.SpentMinor != 1200 || ev.LimitMinor != 1000 || ev.Currency != "UAH" {
		t.Fatalf("event amounts mismatch")
	}
}

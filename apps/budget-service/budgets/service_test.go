package budgets

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/skelbigo/FinanceTracker/apps/gateway-http/workspaces"
	"github.com/skelbigo/FinanceTracker/packages/contracts/notificationsv1"
)

type fakeMembers struct{ members []workspaces.MemberInfo }

func (f fakeMembers) ListMembersInfo(ctx context.Context, workspaceID string) ([]workspaces.MemberInfo, error) {
	return append([]workspaces.MemberInfo(nil), f.members...), nil
}

type fakeNotifClient struct {
	calls []*notificationsv1.CreateNotificationRequest
}

func (f *fakeNotifClient) CreateNotification(ctx context.Context, req *notificationsv1.CreateNotificationRequest) (*notificationsv1.CreateNotificationResponse, error) {
	f.calls = append(f.calls, req)
	return &notificationsv1.CreateNotificationResponse{NotificationIds: []string{"n1"}}, nil
}

type fakeRepo struct {
	budgets   []Budget
	spent     map[string]int64
	events    []BudgetEvent
	eventKeys map[string]struct{}
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

func (r *fakeRepo) InsertBudgetEvent(ctx context.Context, ev BudgetEvent) (bool, error) {
	if r.eventKeys == nil {
		r.eventKeys = make(map[string]struct{})
	}
	key := ev.BudgetID.String() + ":" + ev.PeriodStart.UTC().Format(time.RFC3339Nano) + ":" + ev.PeriodEnd.UTC().Format(time.RFC3339Nano)
	if _, ok := r.eventKeys[key]; ok {
		return false, nil
	}
	r.eventKeys[key] = struct{}{}
	r.events = append(r.events, ev)
	return true, nil
}

func TestService_CheckOverspend_NotOverLimit_NoEvent(t *testing.T) {
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
		spent: map[string]int64{keySpent(catID, "UAH"): 999},
	}

	svc := NewService(repo, repo, nil, false)
	now := time.Date(2026, time.February, 4, 18, 0, 0, 0, time.UTC)
	res, err := svc.CheckOverspendWithEvents(context.Background(), wsID, catID, "UAH", now)
	if err != nil {
		t.Fatalf("CheckOverspendWithEvents error: %v", err)
	}
	if res.Overspent {
		t.Fatalf("overspent: got true want false")
	}
	if len(res.NewEvents) != 0 {
		t.Fatalf("new events: got %d want %d", len(res.NewEvents), 0)
	}
	if len(repo.events) != 0 {
		t.Fatalf("repo events: got %d want %d", len(repo.events), 0)
	}
}

func TestService_CheckOverspend_NoSpam_DoesNotInsertDuplicateEvent(t *testing.T) {
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

	svc := NewService(repo, repo, nil, false)
	now := time.Date(2026, time.February, 4, 18, 0, 0, 0, time.UTC)

	res1, err := svc.CheckOverspendWithEvents(context.Background(), wsID, catID, "UAH", now)
	if err != nil {
		t.Fatalf("CheckOverspendWithEvents #1 error: %v", err)
	}
	if !res1.Overspent || len(res1.NewEvents) != 1 {
		t.Fatalf("#1: overspent=%v newEvents=%d (want true,1)", res1.Overspent, len(res1.NewEvents))
	}

	res2, err := svc.CheckOverspendWithEvents(context.Background(), wsID, catID, "UAH", now)
	if err != nil {
		t.Fatalf("CheckOverspendWithEvents #2 error: %v", err)
	}
	if !res2.Overspent {
		t.Fatalf("#2: overspent: got false want true")
	}
	if len(res2.NewEvents) != 0 {
		t.Fatalf("#2: new events: got %d want %d", len(res2.NewEvents), 0)
	}
	if len(repo.events) != 1 {
		t.Fatalf("repo events: got %d want %d", len(repo.events), 1)
	}
}

func TestService_HandleExpenseTransaction_CallsNotificationClient_OnNewOverspendEvent(t *testing.T) {
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

	actor := uuid.New().String()
	other := uuid.New().String()
	viewer := uuid.New().String()

	mem := fakeMembers{members: []workspaces.MemberInfo{
		{UserID: actor, Role: workspaces.RoleMember},
		{UserID: other, Role: workspaces.RoleOwner},
		{UserID: viewer, Role: workspaces.RoleViewer},
	}}

	client := &fakeNotifClient{}

	svc := NewService(repo, repo, nil, false).WithNotifications(mem, client)

	now := time.Date(2026, time.February, 4, 18, 0, 0, 0, time.UTC)
	txID := uuid.New().String()
	svc.HandleExpenseTransaction(context.Background(), wsID.String(), actor, txID, catID.String(), 100, "UAH", now)

	if len(client.calls) != 1 {
		t.Fatalf("calls: got %d want %d", len(client.calls), 1)
	}
	req := client.calls[0]
	if req.WorkspaceId != wsID.String() {
		t.Fatalf("workspace: got %q want %q", req.WorkspaceId, wsID.String())
	}
	if req.Type != "overspending" {
		t.Fatalf("type: got %q want %q", req.Type, "overspending")
	}
	if len(req.RecipientUserIds) != 1 || req.RecipientUserIds[0] != other {
		t.Fatalf("recipients: got %#v want [%q]", req.RecipientUserIds, other)
	}
}

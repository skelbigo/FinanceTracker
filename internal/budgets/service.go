package budgets

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

type BudgetRepo interface {
	InsertBudget(ctx context.Context, workspaceID uuid.UUID, req UpsertBudgetRequest) (Budget, error)
	Upsert(ctx context.Context, workspaceID uuid.UUID, req UpsertBudgetRequest) (Budget, error)
	UpdateBudget(ctx context.Context, workspaceID, budgetID uuid.UUID, req UpsertBudgetRequest) (Budget, error)
	DeleteBudget(ctx context.Context, workspaceID, budgetID uuid.UUID) error
	GetBudgetByID(ctx context.Context, workspaceID, budgetID uuid.UUID) (Budget, error)
	List(ctx context.Context, workspaceID uuid.UUID, period *Period) ([]Budget, error)
	ListByCategory(ctx context.Context, workspaceID, categoryID uuid.UUID, currency string) ([]Budget, error)
	GetSpentForCategory(ctx context.Context, workspaceID, categoryID uuid.UUID, currency string, from, to time.Time) (int64, error)
	InsertBudgetEvent(ctx context.Context, ev BudgetEvent) (bool, error)
}

type CategoryLookup interface {
	ExistsInWorkspace(ctx context.Context, workspaceID, categoryID uuid.UUID) (bool, error)
	GetType(ctx context.Context, workspaceID, categoryID uuid.UUID) (string, error) // "expense"/"income"
}

type Service struct {
	repo           BudgetRepo
	categories     CategoryLookup
	enforceExpense bool
}

func NewService(repo BudgetRepo, categories CategoryLookup, enforceExpense bool) *Service {
	return &Service{repo: repo, categories: categories, enforceExpense: enforceExpense}
}

var currencyRe = regexp.MustCompile(`^[A-Z]{3}$`)

func normalizeCurrency(s string) string {
	return strings.ToUpper(strings.TrimSpace(s))
}

func (s *Service) UpsertBudget(ctx context.Context, workspaceID uuid.UUID, req UpsertBudgetRequest) (Budget, error) {
	norm, err := s.validateUpsertInput(ctx, workspaceID, req)
	if err != nil {
		return Budget{}, err
	}
	return s.repo.Upsert(ctx, workspaceID, norm)
}

func (s *Service) CreateBudget(ctx context.Context, workspaceID uuid.UUID, req UpsertBudgetRequest) (Budget, error) {
	norm, err := s.validateUpsertInput(ctx, workspaceID, req)
	if err != nil {
		return Budget{}, err
	}
	return s.repo.InsertBudget(ctx, workspaceID, norm)
}

func (s *Service) Update(ctx context.Context, workspaceID, budgetID uuid.UUID, req UpsertBudgetRequest) (Budget, error) {
	norm, err := s.validateUpsertInput(ctx, workspaceID, req)
	if err != nil {
		return Budget{}, err
	}
	return s.repo.UpdateBudget(ctx, workspaceID, budgetID, norm)
}

func (s *Service) Delete(ctx context.Context, workspaceID, budgetID uuid.UUID) error {
	return s.repo.DeleteBudget(ctx, workspaceID, budgetID)
}

func (s *Service) GetByID(ctx context.Context, workspaceID, budgetID uuid.UUID) (Budget, error) {
	return s.repo.GetBudgetByID(ctx, workspaceID, budgetID)
}

func (s *Service) validateUpsertInput(ctx context.Context, workspaceID uuid.UUID, req UpsertBudgetRequest) (UpsertBudgetRequest, error) {
	if !req.Period.IsValid() {
		return UpsertBudgetRequest{}, fmt.Errorf("%w: %s", ErrInvalidPeriod, req.Period)
	}
	if req.AmountLimitMinor <= 0 {
		return UpsertBudgetRequest{}, fmt.Errorf("%w: %d (must be > 0)", ErrInvalidLimit, req.AmountLimitMinor)
	}
	cur := normalizeCurrency(req.Currency)
	if !currencyRe.MatchString(cur) {
		return UpsertBudgetRequest{}, fmt.Errorf("%w: %q", ErrInvalidCurrency, req.Currency)
	}
	req.Currency = cur

	ok, err := s.categories.ExistsInWorkspace(ctx, workspaceID, req.CategoryID)
	if err != nil {
		return UpsertBudgetRequest{}, err
	}
	if !ok {
		return UpsertBudgetRequest{}, ErrCategoryNotFound
	}

	if s.enforceExpense {
		typ, err := s.categories.GetType(ctx, workspaceID, req.CategoryID)
		if err != nil {
			return UpsertBudgetRequest{}, err
		}
		if typ != "expense" {
			return UpsertBudgetRequest{}, ErrCategoryNotExpense
		}
	}

	return req, nil
}

func (s *Service) ListBudgets(ctx context.Context, workspaceID uuid.UUID, period *Period) ([]Budget, error) {
	if period != nil && !period.IsValid() {
		return nil, fmt.Errorf("%w: %s", ErrInvalidPeriod, *period)
	}
	return s.repo.List(ctx, workspaceID, period)
}

func (s *Service) ListWithProgress(ctx context.Context, workspaceID uuid.UUID, period *Period, now time.Time) ([]BudgetResponse, error) {
	items, err := s.ListBudgets(ctx, workspaceID, period)
	if err != nil {
		return nil, err
	}

	out := make([]BudgetResponse, 0, len(items))
	for _, b := range items {
		start, end := PeriodBounds(now, b.Period)
		spent, err := s.repo.GetSpentForCategory(ctx, workspaceID, b.CategoryID, b.Currency, start, end)
		if err != nil {
			return nil, err
		}
		out = append(out, NewBudgetResponse(b, spent, start, end))
	}
	return out, nil
}

type OverspendResult struct {
	Overspent bool
	NewEvents []BudgetEvent
}

func (s *Service) CheckOverspendWithEvents(ctx context.Context, workspaceID, categoryID uuid.UUID, currency string, now time.Time) (OverspendResult, error) {
	cur := normalizeCurrency(currency)
	if !currencyRe.MatchString(cur) {
		return OverspendResult{}, fmt.Errorf("%w: %q", ErrInvalidCurrency, currency)
	}

	budgets, err := s.repo.ListByCategory(ctx, workspaceID, categoryID, cur)
	if err != nil {
		return OverspendResult{}, err
	}
	if len(budgets) == 0 {
		return OverspendResult{Overspent: false, NewEvents: nil}, nil
	}

	res := OverspendResult{Overspent: false, NewEvents: make([]BudgetEvent, 0)}
	for _, b := range budgets {
		start, end := PeriodBounds(now, b.Period)
		spent, err := s.repo.GetSpentForCategory(ctx, workspaceID, b.CategoryID, b.Currency, start, end)
		if err != nil {
			return OverspendResult{}, err
		}
		if spent > b.AmountLimitMinor {
			res.Overspent = true
			ev := BudgetEvent{
				WorkspaceID: workspaceID,
				BudgetID:    b.ID,
				CategoryID:  b.CategoryID,
				PeriodStart: start,
				PeriodEnd:   end,
				SpentMinor:  spent,
				LimitMinor:  b.AmountLimitMinor,
				Currency:    b.Currency,
			}
			inserted, err := s.repo.InsertBudgetEvent(ctx, ev)
			if err != nil {
				return OverspendResult{}, err
			}
			if inserted {
				res.NewEvents = append(res.NewEvents, ev)
			}
		}
	}

	return res, nil
}

func (s *Service) CheckOverspend(ctx context.Context, workspaceID, categoryID uuid.UUID, currency string, now time.Time) (bool, error) {
	res, err := s.CheckOverspendWithEvents(ctx, workspaceID, categoryID, currency, now)
	if err != nil {
		return false, err
	}
	return res.Overspent, nil
}

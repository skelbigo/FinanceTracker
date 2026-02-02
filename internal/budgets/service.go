package budgets

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

type BudgetRepo interface {
	Upsert(ctx context.Context, workspaceID uuid.UUID, req UpsertBudgetRequest) (Budget, error)
	List(ctx context.Context, workspaceID uuid.UUID, period *Period) ([]Budget, error)
}

type CategoryLookup interface {
	ExistsInWorkspace(ctx context.Context, workspaceID, categoryID uuid.UUID) (bool, error)
	GetType(ctx context.Context, workspaceID, categoryID uuid.UUID) (string, error)
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
	if !req.Period.IsValid() {
		return Budget{}, fmt.Errorf("%w: %s", ErrInvalidPeriod, req.Period)
	}
	if req.AmountLimitMinor <= 0 {
		return Budget{}, fmt.Errorf("%w: %d (must be > 0)", ErrInvalidLimit, req.AmountLimitMinor)
	}
	cur := normalizeCurrency(req.Currency)
	if !currencyRe.MatchString(cur) {
		return Budget{}, fmt.Errorf("%w: %q", ErrInvalidCurrency, req.Currency)
	}
	req.Currency = cur

	ok, err := s.categories.ExistsInWorkspace(ctx, workspaceID, req.CategoryID)
	if err != nil {
		return Budget{}, err
	}
	if !ok {
		return Budget{}, ErrCategoryNotFound
	}

	if s.enforceExpense {
		typ, err := s.categories.GetType(ctx, workspaceID, req.CategoryID)
		if err != nil {
			return Budget{}, err
		}
		if typ != "expense" {
			return Budget{}, ErrCategoryNotExpense
		}
	}

	return s.repo.Upsert(ctx, workspaceID, req)
}

func (s *Service) ListBudgets(ctx context.Context, workspaceID uuid.UUID, period *Period) ([]Budget, error) {
	if period != nil && !period.IsValid() {
		return nil, fmt.Errorf("%w: %s", ErrInvalidPeriod, *period)
	}
	return s.repo.List(ctx, workspaceID, period)
}

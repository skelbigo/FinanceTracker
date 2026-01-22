package budgets

import (
	"context"
	"fmt"
	"github.com/google/uuid"
)

type BudgetRepo interface {
	Upsert(ctx context.Context, workspaceID uuid.UUID, req UpsertBudgetRequest) (Budget, error)
	ListWithStats(ctx context.Context, workspaceID uuid.UUID, year int, month int) ([]BudgetResponse, error)
}

type CategoryLookup interface {
	ExistsInWorkspace(ctx context.Context, workspaceID, categoryID uuid.UUID) (bool, error)
	GetType(ctx context.Context, workspaceID, categoryID uuid.UUID) (string, error) // "expense"/"income" або як у тебе
}

type Service struct {
	repo           BudgetRepo
	categories     CategoryLookup
	enforceExpense bool
}

func NewService(repo BudgetRepo, categories CategoryLookup, enforceExpense bool) *Service {
	return &Service{
		repo:           repo,
		categories:     categories,
		enforceExpense: enforceExpense,
	}
}

const (
	minYear = 2025
	maxYear = 2100
)

func validateYearMonthAmount(req UpsertBudgetRequest) error {
	if req.Year < minYear || req.Year > maxYear {
		return fmt.Errorf("%w: %d (allowed %d..%d)", ErrInvalidYear, req.Year, minYear, maxYear)
	}
	if req.Month < 1 || req.Month > 12 {
		return fmt.Errorf("%w: %d (allowed 1..12)", ErrInvalidMonth, req.Month)
	}
	if req.Amount < 0 {
		return fmt.Errorf("%w: %d (must be >= 0)", ErrInvalidAmount, req.Amount)
	}
	return nil
}

func (s *Service) UpsertBudget(ctx context.Context, workspaceID uuid.UUID, req UpsertBudgetRequest) (Budget, error) {
	if err := validateYearMonthAmount(req); err != nil {
		return Budget{}, err
	}

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

func (s *Service) GetBudgetsForMonth(ctx context.Context, workspaceID uuid.UUID, year int, month int) ([]BudgetResponse, error) {
	if year < minYear || year > maxYear {
		return nil, fmt.Errorf("%w: %d (allowed %d..%d)", ErrInvalidYear, year, minYear, maxYear)
	}
	if month < 1 || month > 12 {
		return nil, fmt.Errorf("%w: %d (allowed 1..12)", ErrInvalidMonth, month)
	}

	return s.repo.ListWithStats(ctx, workspaceID, year, month)
}

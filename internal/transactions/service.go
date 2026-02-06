package transactions

import (
	"context"
	"github.com/google/uuid"
	"github.com/skelbigo/FinanceTracker/internal/analytics"
	"log"
	"time"
)

type OverspendChecker interface {
	CheckOverspend(ctx context.Context, workspaceID, categoryID uuid.UUID, currency string, now time.Time) (bool, error)
}

type CategoryLookup interface {
	ExistsInWorkspace(ctx context.Context, workspaceID, categoryID uuid.UUID) (bool, error)
}

type Service struct {
	repo    *Repo
	budgetC OverspendChecker
	cats    CategoryLookup
	inv     analytics.CacheIndex
}

func NewService(repo *Repo, budgetChecker OverspendChecker, cats CategoryLookup, inv analytics.CacheIndex) *Service {
	return &Service{repo: repo, budgetC: budgetChecker, cats: cats, inv: inv}
}

type ListResult struct {
	Items   []Transaction `json:"items"`
	HasNext bool          `json:"has_next"`
	Limit   int           `json:"limit"`
	Offset  int           `json:"offset"`
}

func (s *Service) Create(ctx context.Context, t Transaction) (Transaction, error) {
	if err := s.validateCategory(ctx, t.WorkspaceID, t.CategoryID); err != nil {
		return Transaction{}, err
	}
	out, err := s.repo.Create(ctx, t)
	if err != nil {
		return Transaction{}, err
	}

	s.invalidateAnalyticsBestEffort(ctx, out.WorkspaceID)

	s.checkOverspendBestEffort(ctx, out)
	return out, nil
}

func (s *Service) List(ctx context.Context, workspaceID string, f ListFilter) (ListResult, error) {
	pageSize := f.Limit
	if pageSize <= 0 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}

	offset := f.Offset
	if offset < 0 {
		offset = 0
	}

	f.Limit = pageSize + 1
	f.Offset = offset

	items, err := s.repo.List(ctx, workspaceID, f)
	if err != nil {
		return ListResult{}, err
	}

	hasNext := false
	if len(items) > pageSize {
		hasNext = true
		items = items[:pageSize]
	}

	return ListResult{
		Items:   items,
		HasNext: hasNext,
		Limit:   pageSize,
		Offset:  offset,
	}, nil
}

func (s *Service) GetByID(ctx context.Context, workspaceID, txID string) (Transaction, error) {
	return s.repo.GetByID(ctx, workspaceID, txID)
}

func (s *Service) Update(ctx context.Context, t Transaction) (Transaction, error) {
	if err := s.validateCategory(ctx, t.WorkspaceID, t.CategoryID); err != nil {
		return Transaction{}, err
	}
	out, err := s.repo.Update(ctx, t)
	if err != nil {
		return Transaction{}, err
	}

	s.invalidateAnalyticsBestEffort(ctx, out.WorkspaceID)

	s.checkOverspendBestEffort(ctx, out)
	return out, nil
}

func (s *Service) Delete(ctx context.Context, workspaceID, txID string) (bool, error) {
	ok, err := s.repo.Delete(ctx, workspaceID, txID)
	if err != nil {
		return false, err
	}
	if ok {
		s.invalidateAnalyticsBestEffort(ctx, workspaceID)
	}
	return ok, nil
}

func (s *Service) invalidateAnalyticsBestEffort(ctx context.Context, workspaceID string) {
	if s.inv == nil {
		return
	}
	wsID, err := uuid.Parse(workspaceID)
	if err != nil {
		return
	}
	ctxInv, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := s.inv.InvalidateWorkspace(ctxInv, wsID); err != nil {
		log.Printf("transactions: analytics cache invalidation failed (workspace=%s): %v", wsID, err)
	}
}

func (s *Service) checkOverspendBestEffort(ctx context.Context, tx Transaction) {
	if s.budgetC == nil {
		return
	}
	if tx.Type != TypeExpense {
		return
	}
	if tx.CategoryID == nil {
		return
	}

	wsID, err := uuid.Parse(tx.WorkspaceID)
	if err != nil {
		log.Printf("transactions: overspend check skipped (bad workspace_id=%q): %v", tx.WorkspaceID, err)
		return
	}
	catID, err := uuid.Parse(*tx.CategoryID)
	if err != nil {
		log.Printf("transactions: overspend check skipped (bad category_id=%q): %v", *tx.CategoryID, err)
		return
	}

	if _, err := s.budgetC.CheckOverspend(ctx, wsID, catID, tx.Currency, tx.OccurredAt); err != nil {
		log.Printf("transactions: overspend check failed (workspace=%s category=%s): %v", wsID, catID, err)
	}
}

func (s *Service) validateCategory(ctx context.Context, workspaceID string, categoryID *string) error {
	if categoryID == nil {
		return nil
	}
	if s.cats == nil {
		// allow using the service without a category lookup (tests / minimal wiring)
		return nil
	}

	wsID, err := uuid.Parse(workspaceID)
	if err != nil {
		return ErrCategoryNotFound
	}
	catID, err := uuid.Parse(*categoryID)
	if err != nil {
		return ErrCategoryNotFound
	}

	ok, err := s.cats.ExistsInWorkspace(ctx, wsID, catID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrCategoryNotFound
	}
	return nil
}

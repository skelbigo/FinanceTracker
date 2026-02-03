package transactions

import (
	"context"
	"github.com/google/uuid"
	"log"
	"time"
)

type OverspendChecker interface {
	CheckOverspend(ctx context.Context, workspaceID, categoryID uuid.UUID, currency string, now time.Time) (bool, error)
}

type Service struct {
	repo    *Repo
	budgetC OverspendChecker
}

func NewService(repo *Repo, budgetChecker OverspendChecker) *Service {
	return &Service{repo: repo, budgetC: budgetChecker}
}

type ListResult struct {
	Items   []Transaction `json:"items"`
	HasNext bool          `json:"has_next"`
	Limit   int           `json:"limit"`
	Offset  int           `json:"offset"`
}

func (s *Service) Create(ctx context.Context, t Transaction) (Transaction, error) {
	out, err := s.repo.Create(ctx, t)
	if err != nil {
		return Transaction{}, err
	}

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
	out, err := s.repo.Update(ctx, t)
	if err != nil {
		return Transaction{}, err
	}

	s.checkOverspendBestEffort(ctx, out)
	return out, nil
}

func (s *Service) Delete(ctx context.Context, workspaceID, txID string) (bool, error) {
	return s.repo.Delete(ctx, workspaceID, txID)
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

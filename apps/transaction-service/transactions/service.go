package transactions

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/skelbigo/FinanceTracker/apps/gateway-http/analytics"
	"github.com/skelbigo/FinanceTracker/apps/notification-service/notifications"
)

type BudgetHook interface {
	HandleExpenseTransaction(ctx context.Context, workspaceID, actorUserID, transactionID, categoryID string, amountMinor int64, currency string, occurredAt time.Time)
}

type CategoryLookup interface {
	ExistsInWorkspace(ctx context.Context, workspaceID, categoryID uuid.UUID) (bool, error)
}

type Service struct {
	repo   *Repo
	budget BudgetHook
	cats   CategoryLookup
	inv    analytics.CacheIndex
	notifs *notifications.Service
}

func NewService(repo *Repo, budget BudgetHook, cats CategoryLookup) *Service {
	return &Service{repo: repo, budget: budget, cats: cats}
}

func (s *Service) WithAnalyticsCache(inv analytics.CacheIndex) *Service {
	s.inv = inv
	return s
}

func (s *Service) WithNotifications(notifs *notifications.Service) *Service {
	s.notifs = notifs
	return s
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

	s.notifyNewTransactionBestEffort(ctx, out)

	s.budgetBestEffort(ctx, out)
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

	s.budgetBestEffort(ctx, out)
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
		log.Printf("analytics cache invalidate: invalid workspace id %q: %v", workspaceID, err)
		return
	}
	if err := s.inv.InvalidateWorkspace(ctx, wsID); err != nil {
		log.Printf("analytics cache invalidate: %v", err)
	}
}

func (s *Service) budgetBestEffort(ctx context.Context, tx Transaction) {
	if s.budget == nil {
		return
	}
	if tx.Type != TypeExpense || tx.CategoryID == nil {
		return
	}
	catID := strings.TrimSpace(*tx.CategoryID)
	if catID == "" {
		return
	}

	s.budget.HandleExpenseTransaction(ctx, tx.WorkspaceID, tx.UserID, tx.ID, catID, tx.AmountMinor, tx.Currency, tx.OccurredAt)
}

func (s *Service) notifyNewTransactionBestEffort(ctx context.Context, tx Transaction) {
	if s.notifs == nil {
		return
	}
	catID := ""
	if tx.CategoryID != nil {
		catID = *tx.CategoryID
	}
	s.notifs.NotifyNewTransaction(
		ctx,
		tx.WorkspaceID,
		tx.UserID,
		tx.ID,
		tx.AmountMinor,
		tx.Currency,
		string(tx.Type),
		catID,
		tx.OccurredAt,
	)
}

func (s *Service) validateCategory(ctx context.Context, workspaceID string, catID *string) error {
	if s.cats == nil {
		return nil
	}
	if catID == nil || *catID == "" {
		return nil
	}

	ws, err := uuid.Parse(workspaceID)
	if err != nil {
		return err
	}
	cat, err := uuid.Parse(*catID)
	if err != nil {
		return err
	}

	ok, err := s.cats.ExistsInWorkspace(ctx, ws, cat)
	if err != nil {
		return err
	}
	if !ok {
		return ErrCategoryNotFound
	}
	return nil
}

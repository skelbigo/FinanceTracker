package budgets

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/skelbigo/FinanceTracker/internal/contracts/notificationsv1"
)

type Role string

const (
	RoleOwner  Role = "owner"
	RoleMember Role = "member"
	RoleViewer Role = "viewer"
)

type MemberInfo struct {
	UserID string
	Role   Role
}

type BudgetRepo interface {
	InsertBudget(ctx context.Context, workspaceID uuid.UUID, req UpsertBudgetRequest) (Budget, error)
	Upsert(ctx context.Context, workspaceID uuid.UUID, req UpsertBudgetRequest) (Budget, error)
	UpdateBudget(ctx context.Context, workspaceID, budgetID uuid.UUID, req UpsertBudgetRequest) (Budget, error)
	DeleteBudget(ctx context.Context, workspaceID, budgetID uuid.UUID) error
	GetBudgetByID(ctx context.Context, workspaceID, budgetID uuid.UUID) (Budget, error)
	List(ctx context.Context, workspaceID uuid.UUID, period *Period) ([]Budget, error)
	ListByCategory(ctx context.Context, workspaceID, categoryID uuid.UUID, currency string) ([]Budget, error)
	InsertBudgetEvent(ctx context.Context, ev BudgetEvent) (bool, error)
}

type SpentReader interface {
	GetSpentForCategory(ctx context.Context, workspaceID, categoryID uuid.UUID, currency string, from, to time.Time) (int64, error)
}

type MemberLister interface {
	ListMembers(ctx context.Context, workspaceID string) ([]MemberInfo, error)
}

type NotificationClient interface {
	CreateNotification(ctx context.Context, req *notificationsv1.CreateNotificationRequest) (*notificationsv1.CreateNotificationResponse, error)
}

type CategoryLookup interface {
	ExistsInWorkspace(ctx context.Context, workspaceID, categoryID uuid.UUID) (bool, error)
	GetType(ctx context.Context, workspaceID, categoryID uuid.UUID) (string, error) // "expense"/"income"
}

type Service struct {
	repo           BudgetRepo
	spent          SpentReader
	categories     CategoryLookup
	enforceExpense bool

	members MemberLister
	notifs  NotificationClient
}

func NewService(repo BudgetRepo, spent SpentReader, categories CategoryLookup, enforceExpense bool) *Service {
	return &Service{repo: repo, spent: spent, categories: categories, enforceExpense: enforceExpense}
}

func (s *Service) WithNotifications(members MemberLister, notifs NotificationClient) *Service {
	s.members = members
	s.notifs = notifs
	return s
}

var currencyRe = regexp.MustCompile(`^[A-Z]{3}$`)

func normalizeCurrency(s string) string {
	return strings.ToUpper(strings.TrimSpace(s))
}

func (s *Service) getSpentForCategory(ctx context.Context, workspaceID, categoryID uuid.UUID, currency string, from, to time.Time) (int64, error) {
	if s.spent == nil {
		// Budget module is usable without a Transaction module wired (e.g. unit tests).
		return 0, nil
	}
	return s.spent.GetSpentForCategory(ctx, workspaceID, categoryID, currency, from, to)
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
		spent, err := s.getSpentForCategory(ctx, workspaceID, b.CategoryID, b.Currency, start, end)
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
		spent, err := s.getSpentForCategory(ctx, workspaceID, b.CategoryID, b.Currency, start, end)
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

func (s *Service) HandleExpenseTransaction(ctx context.Context, workspaceID, actorUserID, transactionID, categoryID string, amountMinor int64, currency string, occurredAt time.Time) {
	if s.notifs == nil || s.members == nil {
		return
	}
	wsID, err := uuid.Parse(workspaceID)
	if err != nil {
		return
	}
	catUUID, err := uuid.Parse(categoryID)
	if err != nil {
		return
	}

	res, err := s.CheckOverspendWithEvents(ctx, wsID, catUUID, currency, occurredAt)
	if err != nil {
		log.Printf("budget overspend check: %v", err)
		return
	}
	if !res.Overspent || len(res.NewEvents) == 0 {
		return
	}

	members, err := s.members.ListMembers(ctx, workspaceID)
	if err != nil {
		log.Printf("budget members list: %v", err)
		return
	}

	recipients := make([]string, 0, len(members))
	for _, m := range members {
		if m.Role != RoleOwner && m.Role != RoleMember {
			continue
		}
		if m.UserID == actorUserID {
			continue
		}
		recipients = append(recipients, m.UserID)
	}
	if len(recipients) == 0 {
		return
	}

	for _, ev := range res.NewEvents {
		payload := map[string]any{
			"transactionId": transactionID,
			"createdBy":     actorUserID,
			"categoryId":    categoryID,
			"amountMinor":   amountMinor,
			"currency":      normalizeCurrency(currency),
			"occurredAt":    occurredAt.UTC().Format(time.RFC3339),
			"budgetId":      ev.BudgetID.String(),
			"periodStart":   ev.PeriodStart.UTC().Format(time.RFC3339),
			"periodEnd":     ev.PeriodEnd.UTC().Format(time.RFC3339),
			"spentMinor":    ev.SpentMinor,
			"limitMinor":    ev.LimitMinor,
		}
		payloadJSON, _ := json.Marshal(payload)

		title := "Budget overspending"
		body := fmt.Sprintf("Spent %d %s vs limit %d %s", ev.SpentMinor, ev.Currency, ev.LimitMinor, ev.Currency)

		_, err := s.notifs.CreateNotification(ctx, &notificationsv1.CreateNotificationRequest{
			WorkspaceId:      workspaceID,
			RecipientUserIds: recipients,
			Type:             "overspending",
			Title:            title,
			Body:             body,
			PayloadJson:      string(payloadJSON),
		})
		if err != nil {
			log.Printf("budget -> notification grpc: %v", err)
		}
	}
}

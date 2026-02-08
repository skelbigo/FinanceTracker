package notifications

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/skelbigo/FinanceTracker/apps/gateway-http/workspaces"
)

type MemberLister interface {
	ListMembersInfo(ctx context.Context, workspaceID string) ([]workspaces.MemberInfo, error)
}

type UserLookup interface {
	GetUserEmail(ctx context.Context, userID string) (string, error)
}

type Options struct {
	PublicURL                 string
	EmailNotifyOverspending   bool
	EmailNotifyNewTransaction bool
}

type Service struct {
	repo    *Repo
	members MemberLister
	users   UserLookup
	email   EmailSender
	push    PushProvider
	opts    Options
}

type NotificationService = Service

func NewService(repo *Repo, members MemberLister, users UserLookup, email EmailSender, push PushProvider, opts Options) *Service {
	if opts.PublicURL == "" {
		opts.PublicURL = "http://localhost:8080"
	}
	return &Service{repo: repo, members: members, users: users, email: email, push: push, opts: opts}
}

type ListResult struct {
	Notifications []Notification `json:"notifications"`
	UnreadCount   int            `json:"unread_count"`
	NextCursor    *int           `json:"next_cursor,omitempty"`
}

func (s *Service) CreateInApp(ctx context.Context, userID string, workspaceID *string, typ NotificationType, title, body string, payload map[string]any) (Notification, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return Notification{}, err
	}
	var wsUUID *uuid.UUID
	if workspaceID != nil && *workspaceID != "" {
		tmp, err := uuid.Parse(*workspaceID)
		if err == nil {
			wsUUID = &tmp
		}
	}

	n, err := s.repo.Create(ctx, CreateParams{
		UserID:      uid,
		WorkspaceID: wsUUID,
		Type:        typ,
		Title:       title,
		Body:        body,
		Payload:     payload,
	})
	if err != nil {
		return Notification{}, err
	}

	s.bestEffortDeliver(ctx, n)
	return n, nil
}

func (s *Service) ListForUser(ctx context.Context, userID string, onlyUnread bool, limit int, cursor int) (ListResult, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return ListResult{}, err
	}
	items, hasNext, err := s.repo.ListForUser(ctx, uid, onlyUnread, limit, cursor)
	if err != nil {
		return ListResult{}, err
	}
	unread, err := s.repo.CountUnread(ctx, uid)
	if err != nil {
		return ListResult{}, err
	}

	var next *int
	if hasNext {
		nc := cursor + limit
		next = &nc
	}
	return ListResult{Notifications: items, UnreadCount: unread, NextCursor: next}, nil
}

func (s *Service) MarkRead(ctx context.Context, userID, notificationID string) (bool, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return false, err
	}
	nid, err := uuid.Parse(notificationID)
	if err != nil {
		return false, err
	}
	return s.repo.MarkRead(ctx, uid, nid)
}

func (s *Service) MarkAllRead(ctx context.Context, userID string) (int64, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return 0, err
	}
	return s.repo.MarkAllRead(ctx, uid)
}

func (s *Service) GetForUser(ctx context.Context, userID, notificationID string) (Notification, bool, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return Notification{}, false, err
	}
	nid, err := uuid.Parse(notificationID)
	if err != nil {
		return Notification{}, false, err
	}
	return s.repo.GetForUser(ctx, uid, nid)
}

func (s *Service) SavePushSubscription(ctx context.Context, userID string, endpoint string, keys map[string]any) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return err
	}
	_, err = s.repo.UpsertPushSubscription(ctx, uid, endpoint, keys)
	return err
}

func (s *Service) DeletePushSubscription(ctx context.Context, userID string, endpoint string) (bool, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return false, err
	}
	return s.repo.DeletePushSubscription(ctx, uid, endpoint)
}

func (s *Service) NotifyNewTransaction(
	ctx context.Context,
	workspaceID string,
	actorUserID string,
	txID string,
	amountMinor int64,
	currency string,
	txType string,
	categoryID string,
	occurredAt time.Time,
) {
	members, err := s.members.ListMembersInfo(ctx, workspaceID)
	if err != nil {
		log.Printf("NotifyNewTransaction: list members: %v", err)
		return
	}
	if len(members) <= 1 {
		return
	}
	for _, m := range members {
		if m.UserID == actorUserID && len(members) > 1 {
			continue
		}
		ws := workspaceID
		payload := map[string]any{
			"transactionId": txID,
			"amount":        amountMinor,
			"amountMinor":   amountMinor,
			"currency":      currency,
			"type":          txType,
			"categoryId":    categoryID,
			"date":          occurredAt,
			"createdBy":     actorUserID,
			"recipientRole": string(m.Role),
		}
		title := "New transaction"
		body := fmt.Sprintf("A new transaction was added (%s %d)", currency, amountMinor)
		_, err := s.CreateInApp(ctx, m.UserID, &ws, TypeNewTransaction, title, body, payload)
		if err != nil {
			log.Printf("NotifyNewTransaction: create notif: %v", err)
			continue
		}
	}
}

func (s *Service) bestEffortDeliver(ctx context.Context, n Notification) {
	if s.push != nil {
		err := s.push.Send(n.UserID.String(), map[string]any{"notification_id": n.ID.String(), "type": n.Type})
		if err != nil {
			errTxt := err.Error()
			_ = s.repo.InsertDelivery(ctx, n.ID, ChannelPush, StatusFailed, &errTxt, 1, nil)
			log.Printf("push send: %v", err)
			return
		}
		now := time.Now()
		_ = s.repo.InsertDelivery(ctx, n.ID, ChannelPush, StatusSent, nil, 1, &now)
	}

	switch n.Type {
	case TypeOverspending:
		if s.opts.EmailNotifyOverspending {
			s.bestEffortEmail(ctx, n, "")
		}
	case TypeNewTransaction:
		if s.opts.EmailNotifyNewTransaction {
			if role, ok := n.Payload["recipientRole"].(string); ok && role == string(workspaces.RoleOwner) {
				s.bestEffortEmail(ctx, n, "")
			}
		}
	}
}

func (s *Service) bestEffortEmail(ctx context.Context, n Notification, toEmail string) {
	if s.email == nil || !s.email.Enabled() {
		return
	}
	if toEmail == "" && s.users != nil {
		if email, err := s.users.GetUserEmail(ctx, n.UserID.String()); err == nil {
			toEmail = email
		}
	}
	_ = s.repo.EnsureEmailDelivery(ctx, n.ID, toEmail)
}

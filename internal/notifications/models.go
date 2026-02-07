package notifications

import (
	"time"

	"github.com/google/uuid"
)

type NotificationType string

const (
	TypeOverspending   NotificationType = "overspending"
	TypeNewTransaction NotificationType = "new_transaction"
)

type Notification struct {
	ID          uuid.UUID        `json:"id"`
	UserID      uuid.UUID        `json:"user_id"`
	WorkspaceID *uuid.UUID       `json:"workspace_id,omitempty"`
	Type        NotificationType `json:"type"`
	Title       string           `json:"title"`
	Body        string           `json:"body"`
	Payload     map[string]any   `json:"payload"`
	IsRead      bool             `json:"is_read"`
	CreatedAt   time.Time        `json:"created_at"`
}

type DeliveryChannel string

const (
	ChannelInApp DeliveryChannel = "in_app"
	ChannelEmail DeliveryChannel = "email"
	ChannelPush  DeliveryChannel = "push"
)

type DeliveryStatus string

const (
	StatusPending DeliveryStatus = "pending"
	StatusSent    DeliveryStatus = "sent"
	StatusFailed  DeliveryStatus = "failed"
)

type Delivery struct {
	ID             uuid.UUID       `json:"id"`
	NotificationID uuid.UUID       `json:"notification_id"`
	Channel        DeliveryChannel `json:"channel"`
	Status         DeliveryStatus  `json:"status"`
	Error          *string         `json:"error,omitempty"`
	Attempts       int             `json:"attempts"`
	SentAt         *time.Time      `json:"sent_at,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
}

type PushSubscription struct {
	ID        uuid.UUID      `json:"id"`
	UserID    uuid.UUID      `json:"user_id"`
	Endpoint  string         `json:"endpoint"`
	Keys      map[string]any `json:"keys"`
	CreatedAt time.Time      `json:"created_at"`
}

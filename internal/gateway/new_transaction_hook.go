package gateway

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/skelbigo/FinanceTracker/internal/notifications"
	"github.com/skelbigo/FinanceTracker/internal/transactions"
	"github.com/skelbigo/FinanceTracker/internal/workspaces"
)

type newTransactionHook struct {
	wsRepo *workspaces.Repo
	notifs *notifications.Service
	logger *log.Logger
}

func (h newTransactionHook) HandleNewTransaction(
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
	if h.wsRepo == nil || h.notifs == nil {
		return
	}
	logger := h.logger
	if logger == nil {
		logger = log.Default()
	}

	members, err := h.wsRepo.ListMembersInfo(ctx, workspaceID)
	if err != nil {
		logger.Printf("newTxHook: list members: %v", err)
		return
	}
	if len(members) <= 1 {
		return
	}

	for _, m := range members {
		if m.UserID == actorUserID {
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
		if _, err := h.notifs.CreateInApp(ctx, m.UserID, &ws, notifications.TypeNewTransaction, title, body, payload); err != nil {
			logger.Printf("newTxHook: create notif: %v", err)
		}
	}
}

var _ transactions.NewTransactionHook = (*newTransactionHook)(nil)

package transactions

import "time"

type Type string

const (
	TypeIncome  Type = "income"
	TypeExpense Type = "expense"

	typeIncome  = TypeIncome
	typeExpense = TypeExpense
)

type Transaction struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	UserID      string    `json:"user_id"`
	CategoryID  *string   `json:"category_id"`
	Type        Type      `json:"type"`
	AmountMinor int64     `json:"amount_minor"`
	Currency    string    `json:"currency"`
	OccurredAt  time.Time `json:"occurred_at"`
	Note        *string   `json:"note,omitempty"`
	Tags        []string  `json:"tags,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

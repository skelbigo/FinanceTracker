package budgets

import (
	"time"

	"github.com/google/uuid"
)

type Period string

const (
	PeriodWeek  Period = "week"
	PeriodMonth Period = "month"
)

func (p Period) IsValid() bool {
	switch p {
	case PeriodWeek, PeriodMonth:
		return true
	default:
		return false
	}
}

type Budget struct {
	ID               uuid.UUID `db:"id" json:"id"`
	WorkspaceID      uuid.UUID `db:"workspace_id" json:"workspace_id"`
	CategoryID       uuid.UUID `db:"category_id" json:"category_id"`
	Period           Period    `db:"period" json:"period"`
	AmountLimitMinor int64     `db:"amount_limit_minor" json:"amount_limit_minor"`
	Currency         string    `db:"currency" json:"currency"`
	CreatedAt        time.Time `db:"created_at" json:"created_at"`
	UpdatedAt        time.Time `db:"updated_at" json:"updated_at"`
}

type BudgetResponse struct {
	ID               uuid.UUID `json:"id"`
	CategoryID       uuid.UUID `json:"category_id"`
	Period           Period    `json:"period"`
	AmountLimitMinor int64     `json:"amount_limit_minor"`
	Currency         string    `json:"currency"`
	PeriodStart      time.Time `json:"period_start"`
	PeriodEnd        time.Time `json:"period_end"`
	SpentMinor       int64     `json:"spent_minor"`
	RemainingMinor   int64     `json:"remaining_minor"`
	PercentUsed      int       `json:"percent_used"`
	IsOver           bool      `json:"is_over"`
}

func NewBudgetResponse(b Budget, spent int64, periodStart, periodEnd time.Time) BudgetResponse {
	remaining := b.AmountLimitMinor - spent
	percent := 0
	if b.AmountLimitMinor > 0 {
		percent = int((spent * 100) / b.AmountLimitMinor)
	}
	return BudgetResponse{
		ID:               b.ID,
		CategoryID:       b.CategoryID,
		Period:           b.Period,
		AmountLimitMinor: b.AmountLimitMinor,
		Currency:         b.Currency,
		PeriodStart:      periodStart,
		PeriodEnd:        periodEnd,
		SpentMinor:       spent,
		RemainingMinor:   remaining,
		PercentUsed:      percent,
		IsOver:           spent > b.AmountLimitMinor,
	}
}

package budgets

import "github.com/google/uuid"

type UpsertBudgetRequest struct {
	CategoryID       uuid.UUID `json:"category_id" binding:"required"`
	Period           Period    `json:"period" binding:"required"`
	AmountLimitMinor int64     `json:"amount_limit_minor" binding:"required"`
	Currency         string    `json:"currency" binding:"required"`
}

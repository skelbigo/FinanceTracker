package budgets

import "github.com/google/uuid"

type UpsertBudgetRequest struct {
	CategoryID uuid.UUID `json:"category_id" binding:"required"`
	Year       int       `json:"year" binding:"required"`
	Month      int       `json:"month" binding:"required"`
	Amount     int64     `json:"amount" binding:"required"`
}

type BudgetResponse struct {
	CategoryID uuid.UUID `json:"category_id"`
	Year       int       `json:"year"`
	Month      int       `json:"month"`
	Amount     int64     `json:"amount"`
	Spent      int64     `json:"spent"`
	Remaining  int64     `json:"remaining"`
	IsOver     bool      `json:"is_over"`
}

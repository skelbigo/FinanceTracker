package budgets

import (
	"github.com/google/uuid"
	"time"
)

type Budget struct {
	ID          uuid.UUID `db:"id" json:"id"`
	WorkspaceID uuid.UUID `db:"workspace_id" json:"workspace_id"`
	CategoryID  uuid.UUID `db:"category_id" json:"category_id"`
	Year        int       `db:"year" json:"year"`
	Month       int       `db:"month" json:"month"`
	Amount      int64     `db:"amount" json:"amount"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}

func NewBudgetResponse(b Budget, spent int64) BudgetResponse {
	remaining := b.Amount - spent
	return BudgetResponse{
		CategoryID: b.CategoryID,
		Year:       b.Year,
		Month:      b.Month,
		Amount:     b.Amount,
		Spent:      spent,
		Remaining:  remaining,
		IsOver:     spent > b.Amount,
	}
}

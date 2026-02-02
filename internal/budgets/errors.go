package budgets

import "errors"

var (
	ErrInvalidPeriod      = errors.New("invalid period")
	ErrInvalidLimit       = errors.New("invalid limit")
	ErrInvalidCurrency    = errors.New("invalid currency")
	ErrBudgetExists       = errors.New("budget already exists")
	ErrBudgetNotFound     = errors.New("budget not found")
	ErrCategoryNotFound   = errors.New("category not found in workspace")
	ErrCategoryNotExpense = errors.New("category is not expense")
)

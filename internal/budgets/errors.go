package budgets

import "errors"

var (
	ErrInvalidPeriod      = errors.New("invalid period")
	ErrInvalidLimit       = errors.New("invalid limit")
	ErrInvalidCurrency    = errors.New("invalid currency")
	ErrCategoryNotFound   = errors.New("category not found in workspace")
	ErrCategoryNotExpense = errors.New("category is not expense")
)

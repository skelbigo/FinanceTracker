package budgets

import "errors"

var (
	ErrInvalidYear        = errors.New("invalid year")
	ErrInvalidMonth       = errors.New("invalid month")
	ErrInvalidAmount      = errors.New("invalid amount")
	ErrCategoryNotFound   = errors.New("category not found in workspace")
	ErrCategoryNotExpense = errors.New("category is not expense")
)

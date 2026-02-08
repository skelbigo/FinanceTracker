package transactions

import "errors"

var (
	ErrInvalidAmount    = errors.New("invalid amount")
	ErrInvalidRange     = errors.New("invalid date range")
	ErrCategoryNotFound = errors.New("category not found")
)

package transactions

import "errors"

var (
	ErrInvalidAmount    = errors.New("invalid amount")
	ErrInvalidRange     = errors.New("invalid date range")
	ErrInvalidSort      = errors.New("invalid sort")
	ErrCategoryNotFound = errors.New("category not found")
)

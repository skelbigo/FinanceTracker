package transactions

import "errors"

var (
	ErrInvalidType     = errors.New("invalid type")
	ErrInvalidCurrency = errors.New("invalid currency")
	ErrInvalidAmount   = errors.New("invalid amount")
	ErrInvalidRange    = errors.New("invalid date range")
)

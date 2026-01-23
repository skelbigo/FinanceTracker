package analytics

import "errors"

var (
	ErrInvalidDateRange = errors.New("invalid date range")
	ErrInvalidBucket    = errors.New("invalid bucket")
	ErrInvalidType      = errors.New("invalid type")
	ErrInvalidCurrency  = errors.New("invalid currency")
	ErrInvalidTop       = errors.New("invalid top")
)

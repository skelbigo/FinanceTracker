package transactions

import "strings"

const (
	SortOccurredAtDesc = "occurred_at_desc"
	SortOccurredAtAsc  = "occurred_at_asc"
	SortAmountDesc     = "amount_desc"
	SortAmountAsc      = "amount_asc"
)

func NormalizeSort(sort string) string {
	return strings.ToLower(strings.TrimSpace(sort))
}

func IsAllowedSort(sort string) bool {
	switch NormalizeSort(sort) {
	case SortOccurredAtDesc, SortOccurredAtAsc, SortAmountDesc, SortAmountAsc:
		return true
	default:
		return false
	}
}

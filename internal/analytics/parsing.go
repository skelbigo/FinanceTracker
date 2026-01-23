package analytics

import (
	"strings"
	"time"
)

const dateLayout = "2006-01-02"

func parseDateRange(fromStr, toStr string) (time.Time, time.Time, error) {
	if fromStr == "" || toStr == "" {
		return time.Time{}, time.Time{}, ErrInvalidDateRange
	}

	from, err := time.ParseInLocation(dateLayout, fromStr, time.UTC)
	if err != nil {
		return time.Time{}, time.Time{}, ErrInvalidDateRange
	}
	to, err := time.ParseInLocation(dateLayout, toStr, time.UTC)
	if err != nil {
		return time.Time{}, time.Time{}, ErrInvalidDateRange
	}
	if to.Before(from) {
		return time.Time{}, time.Time{}, ErrInvalidDateRange
	}

	toExclusive := to.AddDate(0, 0, 1)
	return from, toExclusive, nil
}

func parseType(s string) (TxType, error) {
	switch s {
	case string(TypeIncome):
		return TypeIncome, nil
	case string(TypeExpense):
		return TypeExpense, nil
	default:
		return "", ErrInvalidType
	}
}

func parseBucket(s string) (Bucket, error) {
	switch s {
	case string(BucketDay):
		return BucketDay, nil
	case string(BucketWeek):
		return BucketWeek, nil
	case string(BucketMonth):
		return BucketMonth, nil
	default:
		return "", ErrInvalidBucket
	}
}

func parseCurrency(s string) (string, error) {
	s = strings.ToUpper(strings.TrimSpace(s))
	if len(s) != 3 {
		return "", ErrInvalidCurrency
	}
	return s, nil
}

func formatDate(t time.Time) string {
	return t.In(time.UTC).Format(dateLayout)
}

func truncateToBucket(t time.Time, b Bucket) time.Time {
	t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)

	switch b {
	case BucketDay:
		return t
	case BucketWeek:
		wd := int(t.Weekday())
		delta := (wd + 6) % 7
		return t.AddDate(0, 0, -delta)
	case BucketMonth:
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
	default:
		return t
	}
}

func addBucket(t time.Time, b Bucket) time.Time {
	switch b {
	case BucketDay:
		return t.AddDate(0, 0, 1)
	case BucketWeek:
		return t.AddDate(0, 0, 7)
	case BucketMonth:
		return t.AddDate(0, 1, 0)
	default:
		return t
	}
}

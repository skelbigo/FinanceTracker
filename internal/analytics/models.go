package analytics

import (
	"time"

	"github.com/google/uuid"
)

type TxType string

const (
	TypeIncome  TxType = "income"
	TypeExpense TxType = "expense"
)

type Bucket string

const (
	BucketDay   Bucket = "day"
	BucketWeek  Bucket = "week"
	BucketMonth Bucket = "month"
)

type Summary struct {
	IncomeTotal  int64
	ExpenseTotal int64
	Net          int64
}

type CategoryTotalRow struct {
	CategoryID *uuid.UUID
	Name       string
	Total      int64
	Count      int64
}

type TimeseriesRow struct {
	PeriodStart time.Time
	Total       int64
}

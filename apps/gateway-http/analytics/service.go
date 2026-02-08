package analytics

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/skelbigo/FinanceTracker/packages/shared-kernel/redisx"
)

type Service struct {
	repo       Repository
	rdb        *redisx.Client
	cacheIndex CacheIndex
	cacheTTL   time.Duration
}

func NewService(repo Repository, rdb *redisx.Client, cacheIndex CacheIndex, ttl time.Duration) *Service {
	if cacheIndex == nil {
		cacheIndex = NewCacheIndex(nil)
	}
	if ttl <= 0 {
		ttl = 20 * time.Minute
	}
	return &Service{repo: repo, rdb: rdb, cacheIndex: cacheIndex, cacheTTL: ttl}
}

func (s *Service) Summary(ctx context.Context, workspaceID uuid.UUID, fromStr, toStr, currencyStr string) (SummaryResponse, error) {
	from, toExcl, err := parseDateRange(fromStr, toStr)
	if err != nil {
		return SummaryResponse{}, err
	}
	currency, err := parseCurrency(currencyStr)
	if err != nil {
		return SummaryResponse{}, err
	}

	sum, err := s.repo.Summary(ctx, workspaceID, from, toExcl, currency)
	if err != nil {
		return SummaryResponse{}, err
	}

	toIncl := toExcl.AddDate(0, 0, -1)

	return SummaryResponse{
		From:         formatDate(from),
		To:           formatDate(toIncl),
		Currency:     currency,
		IncomeTotal:  sum.IncomeTotal,
		ExpenseTotal: sum.ExpenseTotal,
		Net:          sum.Net,
	}, nil
}

func (s *Service) ByCategory(ctx context.Context, workspaceID uuid.UUID, fromStr, toStr, currencyStr, typeStr string,
	top int) (ByCategoryResponse, error) {
	from, toExcl, err := parseDateRange(fromStr, toStr)
	if err != nil {
		return ByCategoryResponse{}, err
	}
	currency, err := parseCurrency(currencyStr)
	if err != nil {
		return ByCategoryResponse{}, err
	}
	typ, err := parseType(typeStr)
	if err != nil {
		return ByCategoryResponse{}, err
	}
	if top != 0 && (top < 1 || top > 100) {
		return ByCategoryResponse{}, ErrInvalidTop
	}

	rows, grandTotal, err := s.repo.ByCategory(ctx, workspaceID, from, toExcl, currency, typ, top)
	if err != nil {
		return ByCategoryResponse{}, err
	}

	items := make([]ByCategoryItem, 0, len(rows))
	for _, r := range rows {
		share := 0.0
		if grandTotal > 0 {
			share = float64(r.Total) / float64(grandTotal)
		}

		var cid *string
		if r.CategoryID != nil {
			s := r.CategoryID.String()
			cid = &s
		}

		items = append(items, ByCategoryItem{
			CategoryID: cid,
			Name:       r.Name,
			Total:      r.Total,
			Count:      r.Count,
			Share:      share,
		})
	}

	toIncl := toExcl.AddDate(0, 0, -1)

	return ByCategoryResponse{
		From:     formatDate(from),
		To:       formatDate(toIncl),
		Currency: currency,
		Type:     string(typ),
		Total:    grandTotal,
		Items:    items,
	}, nil
}

func (s *Service) Timeseries(ctx context.Context, workspaceID uuid.UUID, fromStr, toStr, currencyStr, bucketStr, typeStr string) (TimeseriesResponse, error) {
	from, toExcl, err := parseDateRange(fromStr, toStr)
	if err != nil {
		return TimeseriesResponse{}, err
	}
	currency, err := parseCurrency(currencyStr)
	if err != nil {
		return TimeseriesResponse{}, err
	}
	bucket, err := parseBucket(bucketStr)
	if err != nil {
		return TimeseriesResponse{}, err
	}
	typ, err := parseType(typeStr)
	if err != nil {
		return TimeseriesResponse{}, err
	}

	rows, err := s.repo.Timeseries(ctx, workspaceID, from, toExcl, currency, bucket, typ)
	if err != nil {
		return TimeseriesResponse{}, err
	}

	m := map[string]int64{}
	for _, r := range rows {
		k := formatDate(truncateToBucket(r.PeriodStart, bucket))
		m[k] = r.Total
	}

	start := truncateToBucket(from, bucket)
	endIncl := truncateToBucket(toExcl.AddDate(0, 0, -1), bucket)

	points := make([]TimeseriesPoint, 0)
	for cur := start; !cur.After(endIncl); cur = addBucket(cur, bucket) {
		k := formatDate(cur)
		points = append(points, TimeseriesPoint{
			Period: k,
			Total:  m[k],
		})
	}

	toIncl := toExcl.AddDate(0, 0, -1)

	return TimeseriesResponse{
		From:     formatDate(from),
		To:       formatDate(toIncl),
		Currency: currency,
		Bucket:   string(bucket),
		Type:     string(typ),
		Points:   points,
	}, nil
}

func (s *Service) AnalyticsJSON(
	ctx context.Context,
	workspaceID uuid.UUID,
	fromStr, toStr, currencyStr, groupByStr string,
	top int,
) ([]byte, error) {
	from, toExcl, err := parseDateRange(fromStr, toStr)
	if err != nil {
		return nil, err
	}
	currency, err := parseCurrency(currencyStr)
	if err != nil {
		return nil, err
	}

	bucket := BucketDay
	if groupByStr != "" {
		b, err := parseBucket(groupByStr)
		if err != nil {
			return nil, err
		}
		bucket = b
	}

	if top == 0 {
		top = 10
	}
	if top < 1 || top > 100 {
		return nil, ErrInvalidTop
	}

	cacheKey, err := BuildAnalyticsCacheKey(workspaceID, currency, from, toExcl, bucket)
	if err != nil {
		return nil, err
	}

	if s.rdb != nil {
		if v, found, err := s.rdb.Get(ctx, cacheKey); err == nil && found {
			return []byte(v), nil
		}
	}

	resp, err := s.computeAnalytics(ctx, workspaceID, from, toExcl, currency, bucket, top)
	if err != nil {
		return nil, err
	}

	payload, err := MarshalAnalyticsResponse(resp)
	if err != nil {
		return nil, err
	}

	if s.rdb != nil {
		if err := s.rdb.SetEX(ctx, cacheKey, payload, s.cacheTTL); err == nil {
			_ = s.cacheIndex.TrackKey(ctx, workspaceID, cacheKey)
		}
	}

	return payload, nil
}

func (s *Service) Analytics(
	ctx context.Context,
	workspaceID uuid.UUID,
	fromStr, toStr, currencyStr, groupByStr string,
	top int,
) (AnalyticsResponse, error) {
	if s.rdb == nil {
		from, toExcl, err := parseDateRange(fromStr, toStr)
		if err != nil {
			return AnalyticsResponse{}, err
		}
		currency, err := parseCurrency(currencyStr)
		if err != nil {
			return AnalyticsResponse{}, err
		}
		bucket := BucketDay
		if groupByStr != "" {
			b, err := parseBucket(groupByStr)
			if err != nil {
				return AnalyticsResponse{}, err
			}
			bucket = b
		}
		if top == 0 {
			top = 10
		}
		if top < 1 || top > 100 {
			return AnalyticsResponse{}, ErrInvalidTop
		}
		return s.computeAnalytics(ctx, workspaceID, from, toExcl, currency, bucket, top)
	}

	payload, err := s.AnalyticsJSON(ctx, workspaceID, fromStr, toStr, currencyStr, groupByStr, top)
	if err != nil {
		return AnalyticsResponse{}, err
	}
	return UnmarshalAnalyticsResponse(payload)
}

func (s *Service) computeAnalytics(
	ctx context.Context,
	workspaceID uuid.UUID,
	from, toExcl time.Time,
	currency string,
	bucket Bucket,
	top int,
) (AnalyticsResponse, error) {
	sum, err := s.repo.Summary(ctx, workspaceID, from, toExcl, currency)
	if err != nil {
		return AnalyticsResponse{}, err
	}

	catRows, expenseTotal, err := s.repo.ByCategory(ctx, workspaceID, from, toExcl, currency, TypeExpense, top)
	if err != nil {
		return AnalyticsResponse{}, err
	}
	topCats := make([]AnalyticsTopCategory, 0, len(catRows))
	for _, r := range catRows {
		share := 0.0
		if expenseTotal > 0 {
			share = float64(r.Total) / float64(expenseTotal)
		}

		var cid *string
		if r.CategoryID != nil {
			s := r.CategoryID.String()
			cid = &s
		}

		topCats = append(topCats, AnalyticsTopCategory{
			CategoryID: cid,
			Name:       r.Name,
			Type:       string(TypeExpense),
			Total:      r.Total,
			Share:      share,
		})
	}

	rows, err := s.repo.Cashflow(ctx, workspaceID, from, toExcl, currency, bucket)
	if err != nil {
		return AnalyticsResponse{}, err
	}

	inc := map[string]int64{}
	exp := map[string]int64{}
	for _, r := range rows {
		k := formatDate(truncateToBucket(r.PeriodStart, bucket))
		inc[k] = r.IncomeTotal
		exp[k] = r.ExpenseTotal
	}

	start := truncateToBucket(from, bucket)
	endIncl := truncateToBucket(toExcl.AddDate(0, 0, -1), bucket)

	cashflow := make([]AnalyticsCashflowPoint, 0)
	for cur := start; !cur.After(endIncl); cur = addBucket(cur, bucket) {
		k := formatDate(cur)
		i := inc[k]
		e := exp[k]
		cashflow = append(cashflow, AnalyticsCashflowPoint{
			Bucket:  k,
			Income:  i,
			Expense: e,
			Net:     i - e,
		})
	}

	toIncl := toExcl.AddDate(0, 0, -1)

	return AnalyticsResponse{
		Range: AnalyticsRange{
			From:    formatDate(from),
			To:      formatDate(toIncl),
			GroupBy: string(bucket),
		},
		Totals: AnalyticsTotals{
			Income:  sum.IncomeTotal,
			Expense: sum.ExpenseTotal,
			Net:     sum.Net,
		},
		TopCategories: topCats,
		Cashflow:      cashflow,
	}, nil
}

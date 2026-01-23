package analytics

import (
	"context"

	"github.com/google/uuid"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service { return &Service{repo: repo} }

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

package analytics

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const analyticsCacheKeyPrefix = "analytics"

func BuildAnalyticsCacheKey(workspaceID uuid.UUID, currency string, fromInclusive, toExclusive time.Time, groupBy Bucket) (string, error) {
	cur, err := parseCurrency(currency)
	if err != nil {
		return "", err
	}

	gb := Bucket(strings.TrimSpace(string(groupBy)))
	if gb == "" {
		gb = BucketDay
	}

	toInclusive := toExclusive.AddDate(0, 0, -1)
	return fmt.Sprintf(
		"%s:%s:%s:%s:%s:%s",
		analyticsCacheKeyPrefix,
		workspaceID.String(),
		cur,
		formatDate(fromInclusive),
		formatDate(toInclusive),
		string(gb),
	), nil
}

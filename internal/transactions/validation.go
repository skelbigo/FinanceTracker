package transactions

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

type FieldErrors map[string]string

func (fe FieldErrors) Add(field, msg string) {
	if fe == nil {
		return
	}
	if _, exists := fe[field]; !exists {
		fe[field] = msg
	}
}

func (fe FieldErrors) Empty() bool { return len(fe) == 0 }

var currencyRe = regexp.MustCompile(`^[A-Z]{3}$`)

func NormalizeType(s string) Type {
	return Type(strings.TrimSpace(strings.ToLower(s)))
}

func ValidateType(t Type) bool {
	return t == TypeIncome || t == TypeExpense
}

func NormalizeCurrencyStrict(s string) (string, error) {
	raw := strings.TrimSpace(s)
	cur := strings.ToUpper(raw)
	if raw != cur || !currencyRe.MatchString(cur) {
		return "", ErrInvalidCurrency
	}
	return cur, nil
}

func ParseOccurredAt(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if len(s) == 10 {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			return t, nil
		}
	}
	return time.Parse(time.RFC3339, s)
}

func NormalizeOptionalUUID(s *string) (*string, error) {
	if s == nil {
		return nil, nil
	}
	v := strings.TrimSpace(*s)
	if v == "" {
		return nil, nil
	}
	if _, err := uuid.Parse(v); err != nil {
		return nil, fmt.Errorf("invalid uuid: %w", err)
	}
	return &v, nil
}

func NormalizeOptionalNote(s *string) *string {
	if s == nil {
		return nil
	}
	v := strings.TrimSpace(*s)
	if v == "" {
		return nil
	}
	return &v
}

func ParseAmountMinor(amount string) (int64, error) {
	s := strings.TrimSpace(amount)
	if s == "" {
		return 0, ErrInvalidAmount
	}

	// Allow comma decimal if dot not present.
	if strings.Contains(s, ",") && !strings.Contains(s, ".") {
		s = strings.ReplaceAll(s, ",", ".")
	}

	neg := false
	if strings.HasPrefix(s, "-") {
		neg = true
		s = strings.TrimPrefix(s, "-")
	}
	if s == "" {
		return 0, ErrInvalidAmount
	}

	parts := strings.Split(s, ".")
	if len(parts) > 2 {
		return 0, ErrInvalidAmount
	}
	whole := parts[0]
	frac := ""
	if len(parts) == 2 {
		frac = parts[1]
	}
	if whole == "" {
		whole = "0"
	}
	if !isDigits(whole) || (frac != "" && !isDigits(frac)) {
		return 0, ErrInvalidAmount
	}
	if len(frac) > 2 {
		return 0, ErrInvalidAmount
	}

	wi, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0, ErrInvalidAmount
	}

	minor := wi * 100
	if len(frac) == 1 {
		fi, _ := strconv.ParseInt(frac, 10, 64)
		minor += fi * 10
	} else if len(frac) == 2 {
		fi, _ := strconv.ParseInt(frac, 10, 64)
		minor += fi
	}

	if neg {
		minor = -minor
	}
	if minor <= 0 {
		return 0, ErrInvalidAmount
	}
	return minor, nil
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func ParseTagsCSV(s string) ([]string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return []string{}, nil
	}

	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, p := range parts {
		v := strings.ToLower(strings.TrimSpace(p))
		if v == "" {
			continue
		}
		if len(v) > 32 {
			return nil, errors.New("tag too long (max 32 chars)")
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
		if len(out) >= 10 {
			break
		}
	}
	return out, nil
}

func NormalizeTagsSlice(tags []string) ([]string, error) {
	if len(tags) == 0 {
		return []string{}, nil
	}
	out := make([]string, 0, len(tags))
	seen := map[string]struct{}{}
	for _, t := range tags {
		v := strings.ToLower(strings.TrimSpace(t))
		if v == "" {
			continue
		}
		if len(v) > 32 {
			return nil, errors.New("tag too long (max 32 chars)")
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
		if len(out) >= 10 {
			break
		}
	}
	return out, nil
}

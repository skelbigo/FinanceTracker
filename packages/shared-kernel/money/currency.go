package money

import (
	"regexp"
	"strings"
)

var currencyRe = regexp.MustCompile(`^[A-Z]{3}$`)

func NormalizeCurrency(s string) string {
	return strings.ToUpper(strings.TrimSpace(s))
}

func IsValidCurrency(s string) bool {
	return currencyRe.MatchString(NormalizeCurrency(s))
}

package stringsx

import (
	"strings"
	"unicode"
)

func StripControl(s string) string {
	if s == "" {
		return ""
	}

	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if unicode.IsControl(r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func FirstNRunes(s string, n int) string {
	if n <= 0 || s == "" {
		return ""
	}
	count := 0
	for i := range s {
		if count == n {
			return s[:i]
		}
		count++
	}
	return s
}

func TrimStrip(s string) string {
	return strings.TrimSpace(StripControl(s))
}

func TrimStripMaxRunes(s string, maxRunes int) string {
	s = TrimStrip(s)
	if s == "" {
		return ""
	}
	if maxRunes > 0 {
		s = FirstNRunes(s, maxRunes)
	}
	return s
}

package db

import (
	"net/url"
	"strings"
)

const maskedSecret = "******"

func MaskPostgresURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	if u, err := url.Parse(raw); err == nil && u.Scheme != "" {
		q := u.Query()
		for _, k := range []string{"password", "pass", "pwd"} {
			if q.Has(k) {
				q.Set(k, maskedSecret)
			}
		}
		encoded := q.Encode()
		encoded = strings.ReplaceAll(encoded, "=%2A%2A%2A%2A%2A%2A", "="+maskedSecret)
		u.RawQuery = encoded

		username := ""
		hasUser := false
		hasPwd := false
		if u.User != nil {
			hasUser = true
			username = u.User.Username()
			_, hasPwd = u.User.Password()
			if username == "" {
				username = maskedSecret
			}
		}

		u.User = nil
		base := u.String()

		if hasUser {
			prefix := u.Scheme + "://"
			if strings.HasPrefix(base, prefix) {
				rest := strings.TrimPrefix(base, prefix)
				if hasPwd {
					return prefix + username + ":" + maskedSecret + "@" + rest
				}
				return prefix + username + "@" + rest
			}
		}

		return base
	}

	for _, k := range []string{"password", "pass", "pwd"} {
		if masked, ok := maskConninfoKey(raw, k, maskedSecret); ok {
			return masked
		}
	}

	if i := strings.Index(raw, "://"); i >= 0 {
		rest := raw[i+3:]
		if at := strings.IndexByte(rest, '@'); at > 0 {
			creds := rest[:at]
			if colon := strings.IndexByte(creds, ':'); colon > 0 {
				return raw[:i+3] + creds[:colon+1] + maskedSecret + rest[at:]
			}
		}
	}

	return raw
}

func isWS(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\r':
		return true
	default:
		return false
	}
}

func maskConninfoKey(s, key, replacement string) (string, bool) {
	n := len(s)
	i := 0

	for i < n {
		for i < n && isWS(s[i]) {
			i++
		}
		if i >= n {
			break
		}

		kstart := i
		for i < n && s[i] != '=' && !isWS(s[i]) {
			i++
		}
		if i >= n || s[i] != '=' {
			for i < n && !isWS(s[i]) {
				i++
			}
			continue
		}

		k := s[kstart:i]
		i++
		vstart := i

		if i < n && (s[i] == '\'' || s[i] == '"') {
			quote := s[i]
			i++
			for i < n {
				if s[i] == '\\' {
					if i+1 < n {
						i += 2
						continue
					}
					i++
					continue
				}
				if s[i] == quote {
					i++
					break
				}
				i++
			}
		} else {
			for i < n && !isWS(s[i]) {
				i++
			}
		}
		vend := i

		if strings.EqualFold(k, key) {
			val := s[vstart:vend]
			if len(val) >= 2 && ((val[0] == '\'' && val[len(val)-1] == '\'') || (val[0] == '"' && val[len(val)-1] == '"')) {
				q := val[:1]
				return s[:vstart] + q + replacement + q + s[vend:], true
			}
			return s[:vstart] + replacement + s[vend:], true
		}
	}

	return s, false
}

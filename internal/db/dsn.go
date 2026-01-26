package db

import (
	"fmt"
	"strings"
)

type DBConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Name     string
	SSLMode  string
}

func BuildPostgresDSN(c DBConfig) string {
	parts := []string{
		fmt.Sprintf("host=%s", escapeConninfoValue(c.Host)),
		fmt.Sprintf("port=%d", c.Port),
		fmt.Sprintf("user=%s", escapeConninfoValue(c.User)),
		fmt.Sprintf("password=%s", escapeConninfoValue(c.Password)),
		fmt.Sprintf("dbname=%s", escapeConninfoValue(c.Name)),
		fmt.Sprintf("sslmode=%s", escapeConninfoValue(c.SSLMode)),
	}
	return strings.Join(parts, " ")
}

func escapeConninfoValue(v string) string {
	if v == "" {
		return v
	}

	needsQuotes := false
	for _, r := range v {
		switch r {
		case ' ', '\t', '\n', '\r', '\v', '\f', '\'', '\\', '=':
			needsQuotes = true
			break
		}
		if needsQuotes {
			break
		}
	}

	if !needsQuotes {
		return v
	}

	replacer := strings.NewReplacer(`\`, `\\`, `'`, `\'`)
	return "'" + replacer.Replace(v) + "'"
}

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

func MaskedDSN(c DBConfig) string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s dbname=%s sslmode=%s",
		c.Host,
		c.Port,
		c.User,
		c.Name,
		c.SSLMode,
	)
}

func BuildPostgresDSN(c DBConfig) string {
	parts := []string{
		fmt.Sprintf("host=%s", c.Host),
		fmt.Sprintf("port=%d", c.Port),
		fmt.Sprintf("user=%s", c.User),
		fmt.Sprintf("password=%s", c.Password),
		fmt.Sprintf("dbname=%s", c.Name),
		fmt.Sprintf("sslmode=%s", c.SSLMode),
	}
	return strings.Join(parts, " ")
}

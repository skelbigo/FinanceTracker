package db

import (
	"strings"
	"testing"
)

func TestBuildPostgresDSN(t *testing.T) {
	cfg := DBConfig{
		Host:     "127.0.0.1",
		Port:     5432,
		User:     "postgres",
		Password: "p@ss:word",
		Name:     "testdb",
		SSLMode:  "disable",
	}

	dsn := BuildPostgresDSN(cfg)

	if !strings.Contains(dsn, "password=p@ss:word") {
		t.Fatalf("password not preserved: %s", dsn)
	}
}

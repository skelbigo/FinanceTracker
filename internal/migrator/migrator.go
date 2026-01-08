package migrator

import (
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func Run(migrationsPath, dbURL, cmd string) error {
	m, err := migrate.New("file://"+migrationsPath, dbURL)
	if err != nil {
		return fmt.Errorf("migrate init error: %w", err)
	}
	defer func() { _, _ = m.Close() }()
	switch cmd {
	case "up":
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			return fmt.Errorf("migrate up error: %w", err)
		}
	case "down":
		if err := m.Down(); err != nil && err != migrate.ErrNoChange {
			return fmt.Errorf("migrate down error: %w", err)
		}
	case "version":
		v, dirty, err := m.Version()
		if err == migrate.ErrNilVersion {
			fmt.Println("no version applied yet")
			return nil
		}
		if err != nil {
			return fmt.Errorf("version error: %w", err)
		}
		fmt.Printf("version=%d dirty=%v\n", v, dirty)
	default:
		return fmt.Errorf("unknown migrate command: %s (use: up|down|version)", cmd)
	}

	return nil
}

package migrator

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func Run(migrationsPath, dbURL, cmd string, out io.Writer) error {
	if out == nil {
		out = io.Discard
	}
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
		return nil

	case "down":
		if err := m.Down(); err != nil && err != migrate.ErrNoChange {
			return fmt.Errorf("migrate down error: %w", err)
		}
		return nil

	case "version", "status":
		v, dirty, err := m.Version()
		if err == migrate.ErrNilVersion {
			fmt.Fprintln(out, "no version applied yet")
			return nil
		}
		if err != nil {
			return fmt.Errorf("version error: %w", err)
		}
		fmt.Fprintf(out, "version=%d dirty=%v\n", v, dirty)
		return nil

	default:
		if strings.HasPrefix(cmd, "force:") {
			vStr := strings.TrimPrefix(cmd, "force:")
			v, err := strconv.Atoi(vStr)
			if err != nil {
				return fmt.Errorf("invalid force version: %q", vStr)
			}
			if err := m.Force(v); err != nil {
				return fmt.Errorf("force error: %w", err)
			}
			return nil
		}
		return fmt.Errorf("unknown migrate command: %s (use: up|down|version|force:<n>)", cmd)
	}
}

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/skelbigo/FinanceTracker/packages/shared-kernel/config"
	"github.com/skelbigo/FinanceTracker/packages/shared-kernel/cryptox"
)

var identRe = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

type flagsT struct {
	table     string
	idCol     string
	cols      string
	encSuffix string
	batch     int
	dryRun    bool
	nullPlain bool
}

func parseFlags() flagsT {
	var f flagsT
	flag.StringVar(&f.table, "table", "integrations", "table name to migrate")
	flag.StringVar(&f.idCol, "id", "id", "primary key column name")
	flag.StringVar(&f.cols, "cols", "access_token,refresh_token,api_key", "comma-separated plaintext columns to encrypt")
	flag.StringVar(&f.encSuffix, "enc-suffix", "_enc", "suffix for encrypted payload columns")
	flag.IntVar(&f.batch, "batch", 200, "batch size")
	flag.BoolVar(&f.dryRun, "dry-run", false, "print what would change without updating")
	flag.BoolVar(&f.nullPlain, "null-plaintext", true, "set plaintext columns to NULL after encrypting")
	flag.Parse()
	return f
}

func validateIdent(kind, v string) error {
	if v == "" || !identRe.MatchString(v) {
		return fmt.Errorf("invalid %s identifier: %q", kind, v)
	}
	return nil
}

func splitCols(s string) ([]string, error) {
	parts := strings.Split(s, ",")
	cols := make([]string, 0, len(parts))
	for _, p := range parts {
		c := strings.TrimSpace(p)
		if c == "" {
			continue
		}
		if err := validateIdent("column", c); err != nil {
			return nil, err
		}
		cols = append(cols, c)
	}
	if len(cols) == 0 {
		return nil, errors.New("no columns provided")
	}
	return cols, nil
}

func main() {
	logger := log.New(os.Stdout, "", log.LstdFlags|log.LUTC)
	f := parseFlags()

	if err := validateIdent("table", f.table); err != nil {
		logger.Fatal(err)
	}
	if err := validateIdent("id column", f.idCol); err != nil {
		logger.Fatal(err)
	}
	cols, err := splitCols(f.cols)
	if err != nil {
		logger.Fatal(err)
	}
	if f.batch <= 0 {
		logger.Fatal("batch must be > 0")
	}
	if f.encSuffix == "" {
		logger.Fatal("enc-suffix must not be empty")
	}

	cfg, err := config.Load()
	if err != nil {
		logger.Fatalf("invalid environment: %v", err)
	}
	if !cfg.EncryptionEnabled {
		logger.Fatal("ENCRYPTION_ENABLED=false; refuse to migrate (enable encryption and provide ENCRYPTION_KEY)")
	}
	cryptoSvc, err := cryptox.NewServiceWithKeyring(cfg.EncryptionEnabled, cfg.EncryptionKeyID, cfg.EncryptionKeys)
	if err != nil {
		logger.Fatalf("crypto init failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.EffectiveDBURL())
	if err != nil {
		logger.Fatalf("db connect failed: %v", err)
	}
	defer pool.Close()

	if ok, err := tableExists(ctx, pool, f.table); err != nil {
		logger.Fatalf("table check failed: %v", err)
	} else if !ok {
		logger.Printf("table %q does not exist; nothing to migrate", f.table)
		return
	}

	encCols := make([]string, 0, len(cols))
	for _, c := range cols {
		enc := c + f.encSuffix
		if err := validateIdent("encrypted column", enc); err != nil {
			logger.Fatal(err)
		}
		encCols = append(encCols, enc)
		if ok, err := columnExists(ctx, pool, f.table, enc); err != nil {
			logger.Fatalf("column check failed: %v", err)
		} else if !ok {
			logger.Fatalf("missing encrypted column %q on table %q (add it first, e.g. via migration)", enc, f.table)
		}
		if ok, err := columnExists(ctx, pool, f.table, c); err != nil {
			logger.Fatalf("column check failed: %v", err)
		} else if !ok {
			logger.Fatalf("missing plaintext column %q on table %q", c, f.table)
		}
	}

	total, err := migrateInBatches(ctx, pool, cryptoSvc, f.table, f.idCol, cols, encCols, f.batch, f.nullPlain, f.dryRun, logger)
	if err != nil {
		logger.Fatalf("migration failed: %v", err)
	}
	if f.dryRun {
		logger.Printf("dry-run complete; would update %d rows", total)
		return
	}
	logger.Printf("migration complete; updated %d rows", total)
}

func tableExists(ctx context.Context, pool *pgxpool.Pool, table string) (bool, error) {
	var reg *string
	err := pool.QueryRow(ctx, "SELECT to_regclass($1)", "public."+table).Scan(&reg)
	if err != nil {
		return false, err
	}
	return reg != nil, nil
}

func columnExists(ctx context.Context, pool *pgxpool.Pool, table, col string) (bool, error) {
	var one int
	err := pool.QueryRow(ctx, `
SELECT 1
FROM information_schema.columns
WHERE table_schema='public' AND table_name=$1 AND column_name=$2
LIMIT 1`, table, col).Scan(&one)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func migrateInBatches(
	ctx context.Context,
	pool *pgxpool.Pool,
	cryptoSvc *cryptox.Service,
	table, idCol string,
	plainCols, encCols []string,
	batch int,
	nullPlain bool,
	dryRun bool,
	logger *log.Logger,
) (int, error) {
	updated := 0

	selectCols := append([]string{idCol}, plainCols...)
	whereParts := make([]string, 0, len(plainCols))
	for _, c := range plainCols {
		whereParts = append(whereParts, fmt.Sprintf("(%s IS NOT NULL AND %s <> '')", c, c))
	}
	selectSQL := fmt.Sprintf(
		"SELECT %s FROM %s WHERE %s LIMIT $1",
		strings.Join(selectCols, ", "),
		table,
		strings.Join(whereParts, " OR "),
	)

	for {
		rows, err := pool.Query(ctx, selectSQL, batch)
		if err != nil {
			return updated, err
		}

		batchIDs := make([]string, 0, batch)
		batchPlain := make([][]string, 0, batch)
		for rows.Next() {
			vals, err := rows.Values()
			if err != nil {
				rows.Close()
				return updated, err
			}
			id := fmt.Sprint(vals[0])
			batchIDs = append(batchIDs, id)
			plain := make([]string, len(plainCols))
			for i := range plainCols {
				if vals[i+1] == nil {
					plain[i] = ""
					continue
				}
				plain[i] = fmt.Sprint(vals[i+1])
			}
			batchPlain = append(batchPlain, plain)
		}
		rows.Close()

		if len(batchIDs) == 0 {
			break
		}

		if dryRun {
			logger.Printf("dry-run: would migrate %d rows", len(batchIDs))
			updated += len(batchIDs)
			continue
		}

		tx, err := pool.Begin(ctx)
		if err != nil {
			return updated, err
		}

		for r := range batchIDs {
			id := batchIDs[r]
			plainVals := batchPlain[r]

			args := make([]any, 0, 1+len(plainCols)*2)
			setParts := make([]string, 0, len(plainCols)*2)
			argN := 1
			for i, pc := range plainCols {
				pv := strings.TrimSpace(plainVals[i])
				if pv == "" {
					continue
				}
				enc, err := cryptoSvc.EncryptForStorage(pv)
				if err != nil {
					_ = tx.Rollback(ctx)
					return updated, fmt.Errorf("encrypt %s for id=%s: %w", pc, id, err)
				}
				setParts = append(setParts, fmt.Sprintf("%s = $%d", encCols[i], argN))
				args = append(args, enc)
				argN++

				if nullPlain {
					setParts = append(setParts, fmt.Sprintf("%s = NULL", pc))
				}
			}
			if len(setParts) == 0 {
				continue
			}

			args = append(args, id)
			updateSQL := fmt.Sprintf(
				"UPDATE %s SET %s WHERE %s = $%d",
				table,
				strings.Join(setParts, ", "),
				idCol,
				argN,
			)
			if _, err := tx.Exec(ctx, updateSQL, args...); err != nil {
				_ = tx.Rollback(ctx)
				return updated, err
			}
			updated++
		}

		if err := tx.Commit(ctx); err != nil {
			return updated, err
		}
	}

	return updated, nil
}

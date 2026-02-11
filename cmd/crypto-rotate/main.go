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
	table  string
	idCol  string
	cols   string
	batch  int
	dryRun bool
}

func parseFlags() flagsT {
	var f flagsT
	flag.StringVar(&f.table, "table", "integrations", "table name to rotate")
	flag.StringVar(&f.idCol, "id", "id", "primary key column name")
	flag.StringVar(&f.cols, "cols", "access_token_enc,refresh_token_enc,api_key_enc", "comma-separated encrypted columns to rotate")
	flag.IntVar(&f.batch, "batch", 200, "batch size")
	flag.BoolVar(&f.dryRun, "dry-run", false, "print what would change without updating")
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

	cfg, err := config.Load()
	if err != nil {
		logger.Fatalf("invalid environment: %v", err)
	}
	if !cfg.EncryptionEnabled {
		logger.Fatal("ENCRYPTION_ENABLED=false; refuse to rotate")
	}
	if len(cfg.EncryptionKeys) < 2 {
		logger.Fatal("rotation requires ENCRYPTION_KEYS with at least 2 keys (old + new)")
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
		logger.Printf("table %q does not exist; nothing to rotate", f.table)
		return
	}
	for _, c := range cols {
		if ok, err := columnExists(ctx, pool, f.table, c); err != nil {
			logger.Fatalf("column check failed: %v", err)
		} else if !ok {
			logger.Fatalf("missing column %q on table %q", c, f.table)
		}
	}

	total, err := rotateInBatches(ctx, pool, cryptoSvc, cfg.EncryptionKeyID, f.table, f.idCol, cols, f.batch, f.dryRun)
	if err != nil {
		logger.Fatalf("rotation failed: %v", err)
	}
	if f.dryRun {
		logger.Printf("dry-run complete; would update %d values", total)
		return
	}
	logger.Printf("rotation complete; updated %d values", total)
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

func rotateInBatches(
	ctx context.Context,
	pool *pgxpool.Pool,
	cryptoSvc *cryptox.Service,
	activeKID string,
	table, idCol string,
	cols []string,
	batch int,
	dryRun bool,
) (int, error) {
	updated := 0

	selectCols := append([]string{idCol}, cols...)
	whereParts := make([]string, 0, len(cols))
	for _, c := range cols {
		whereParts = append(whereParts, fmt.Sprintf("(%s IS NOT NULL AND %s <> '' AND %s NOT LIKE $2)", c, c, c))
	}
	selectSQL := fmt.Sprintf(
		"SELECT %s FROM %s WHERE %s LIMIT $1",
		strings.Join(selectCols, ", "),
		table,
		strings.Join(whereParts, " OR "),
	)
	likePattern := "%\"kid\":\"" + activeKID + "\"%"

	for {
		rows, err := pool.Query(ctx, selectSQL, batch, likePattern)
		if err != nil {
			return updated, err
		}

		batchUpdates := make([]struct {
			id  any
			col string
			val string
		}, 0, batch)

		count := 0
		for rows.Next() {
			count++
			values, err := rows.Values()
			if err != nil {
				return updated, err
			}
			id := values[0]
			for i, col := range cols {
				s, _ := values[i+1].(string)
				if strings.TrimSpace(s) == "" {
					continue
				}
				if strings.Contains(s, `"kid":"`+activeKID+`"`) {
					continue
				}
				pt, err := cryptoSvc.DecryptFromStorage(s)
				if err != nil {
					// If value can't be decrypted, leave it untouched; caller should handle invalid data.
					continue
				}
				enc, err := cryptoSvc.EncryptForStorage(pt)
				if err != nil {
					return updated, err
				}
				if enc != s {
					batchUpdates = append(batchUpdates, struct {
						id  any
						col string
						val string
					}{id: id, col: col, val: enc})
				}
			}
		}
		rows.Close()
		if count == 0 {
			break
		}

		if dryRun {
			updated += len(batchUpdates)
			continue
		}

		b := &pgx.Batch{}
		for _, u := range batchUpdates {
			q := fmt.Sprintf("UPDATE %s SET %s=$1 WHERE %s=$2", table, u.col, idCol)
			b.Queue(q, u.val, u.id)
		}
		br := pool.SendBatch(ctx, b)
		if err := br.Close(); err != nil {
			return updated, err
		}
		updated += len(batchUpdates)
	}
	return updated, nil
}

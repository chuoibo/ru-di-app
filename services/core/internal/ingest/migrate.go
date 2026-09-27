package ingest

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed schema.sql
var schemaSQL string

//go:embed schema_v2.sql
var schemaV2SQL string

// migrations are applied in order, each once, each pinned by its digest. An
// applied file that changes afterwards is refused rather than re-run: the
// database already holds what the old text said.
var migrations = []string{schemaSQL, schemaV2SQL}

// Migrate adds the isolated place-ingest tables after the legacy schema
// migration. Call explicitly from a deployment migration command, never a
// request handler.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(815207)`); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS place_ingest_schema_migrations(version integer PRIMARY KEY, digest text NOT NULL)`); err != nil {
		return err
	}
	for i, sql := range migrations {
		version := i + 1
		digest := fmt.Sprintf("%x", sha256.Sum256([]byte(sql)))
		var existing string
		if err = tx.QueryRow(ctx, `SELECT COALESCE((SELECT digest FROM place_ingest_schema_migrations WHERE version=$1),'')`, version).Scan(&existing); err != nil {
			return err
		}
		if existing != "" {
			if existing != digest {
				return fmt.Errorf("place ingest migration %d checksum mismatch", version)
			}
			continue
		}
		if _, err = tx.Exec(ctx, sql); err != nil {
			return fmt.Errorf("place ingest migration %d: %w", version, err)
		}
		if _, err = tx.Exec(ctx, `INSERT INTO place_ingest_schema_migrations(version,digest) VALUES($1,$2)`, version, digest); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

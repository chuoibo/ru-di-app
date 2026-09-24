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
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(schemaSQL)))
	var existing string
	if err = tx.QueryRow(ctx, `SELECT COALESCE((SELECT digest FROM place_ingest_schema_migrations WHERE version=1),'')`).Scan(&existing); err != nil {
		return err
	}
	if existing != "" {
		if existing != digest {
			return fmt.Errorf("place ingest migration checksum mismatch")
		}
		return tx.Commit(ctx)
	}
	if _, err = tx.Exec(ctx, schemaSQL); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO place_ingest_schema_migrations(version,digest) VALUES(1,$1)`, digest); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

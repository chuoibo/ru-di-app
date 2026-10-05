package accountauth

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed schema.sql
var schemaV1 string

//go:embed schema_v2.sql
var schemaV2 string

// Versions are append-only: an installed version is never edited, a change is
// a new file. Version n is schemas[n-1].
var schemas = []string{schemaV1, schemaV2}

func schemaDigest(sql string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(sql))) }

// Migrate runs only from the operator command, never from HTTP startup.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(5353)`); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS managed_auth_migrations(version integer PRIMARY KEY,digest text NOT NULL)`); err != nil {
		return err
	}
	for i, sql := range schemas {
		version, digest := i+1, schemaDigest(sql)
		var installed string
		if err = tx.QueryRow(ctx, `SELECT coalesce((SELECT digest FROM managed_auth_migrations WHERE version=$1),'')`, version).Scan(&installed); err != nil {
			return err
		}
		if installed != "" {
			if installed != digest {
				return fmt.Errorf("managed auth schema version %d checksum mismatch", version)
			}
			continue
		}
		if _, err = tx.Exec(ctx, sql); err != nil {
			return fmt.Errorf("managed auth schema version %d: %w", version, err)
		}
		if _, err = tx.Exec(ctx, `INSERT INTO managed_auth_migrations VALUES($1,$2)`, version, digest); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// CheckSchema refuses to serve unless every version is installed unchanged.
func CheckSchema(ctx context.Context, pool *pgxpool.Pool) error {
	for i, sql := range schemas {
		var digest string
		if err := pool.QueryRow(ctx, `SELECT digest FROM managed_auth_migrations WHERE version=$1`, i+1).Scan(&digest); err != nil {
			return fmt.Errorf("run core migrate-accounts before enabling managed auth (version %d missing)", i+1)
		}
		if digest != schemaDigest(sql) {
			return fmt.Errorf("managed auth schema version %d checksum mismatch", i+1)
		}
	}
	return nil
}

// SchemaSQL is every version in order, for gates that read what the tables hold.
func SchemaSQL() string { return strings.Join(schemas, "\n") }

// SchemaFiles is each embedded version as its file reads, for the trigger
// gate that matches SQL files by content.
func SchemaFiles() []string { return append([]string(nil), schemas...) }

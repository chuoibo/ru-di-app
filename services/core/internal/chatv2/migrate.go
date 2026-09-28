package chatv2

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed schema.sql
var schemaSQL string

// SchemaSQL is the embedded migration, for the gates that read what its
// triggers write (aigate).
func SchemaSQL() string { return schemaSQL }

// Migrate adds the isolated chat-v2 tables after the legacy schema migration.
// Call explicitly from a deployment migration command, never a request handler.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// repo-guard: allow=long-number reason=synthetic-migration-lock
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(728341922)`); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS chat_v2_schema_migrations(version integer PRIMARY KEY, digest text NOT NULL)`); err != nil {
		return err
	}
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(schemaSQL)))
	var existing string
	if err = tx.QueryRow(ctx, `SELECT COALESCE((SELECT digest FROM chat_v2_schema_migrations WHERE version=1),'')`).Scan(&existing); err != nil {
		return err
	}
	if existing != "" {
		if existing != digest {
			return fmt.Errorf("chat v2 migration checksum mismatch")
		}
		return tx.Commit(ctx)
	}
	if _, err = tx.Exec(ctx, schemaSQL); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO chat_v2_schema_migrations(version,digest) VALUES(1,$1)`, digest); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

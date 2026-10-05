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

//go:embed schema_v2.sql
var schemaV2SQL string

// migrations are applied in order, each once, each pinned by its digest.
var migrations = []string{schemaSQL, schemaV2SQL}

// SchemaSQL is the embedded migrations, for the gates that read what their
// triggers write (aigate).
func SchemaSQL() string { return schemaSQL + "\n" + schemaV2SQL }

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
	for i, sql := range migrations {
		version := i + 1
		digest := fmt.Sprintf("%x", sha256.Sum256([]byte(sql)))
		var existing string
		if err = tx.QueryRow(ctx, `SELECT COALESCE((SELECT digest FROM chat_v2_schema_migrations WHERE version=$1),'')`, version).Scan(&existing); err != nil {
			return err
		}
		if existing != "" {
			if existing != digest {
				return fmt.Errorf("chat v2 migration %d checksum mismatch", version)
			}
			continue
		}
		if _, err = tx.Exec(ctx, sql); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO chat_v2_schema_migrations(version,digest) VALUES($1,$2)`, version, digest); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// CheckSchema refuses a database where Migrate has not applied every version
// this binary embeds, with the same digests.
func CheckSchema(ctx context.Context, pool *pgxpool.Pool) error {
	for i, sql := range migrations {
		digest := fmt.Sprintf("%x", sha256.Sum256([]byte(sql)))
		var existing string
		err := pool.QueryRow(ctx, `SELECT COALESCE((SELECT digest FROM chat_v2_schema_migrations WHERE version=$1),'')`, i+1).Scan(&existing)
		if err != nil {
			return fmt.Errorf("chat v2 schema missing; run core migrate-chat: %w", err)
		}
		if existing != digest {
			return fmt.Errorf("chat v2 migration %d not applied or changed; run core migrate-chat", i+1)
		}
	}
	return nil
}

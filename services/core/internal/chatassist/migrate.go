// Package chatassist implements explicit, invocation-only AI jobs for the legacy
// compatibility client. It cannot serve E2EE conversations or read chat history.
package chatassist

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed schema.sql
var schemaSQL string

// Migrate is run explicitly by the candidate migration command, never a request.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(734129)`); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS chat_ai_schema_migrations(version integer PRIMARY KEY,digest text NOT NULL)`); err != nil {
		return err
	}
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(schemaSQL)))
	var old string
	if err = tx.QueryRow(ctx, `SELECT COALESCE((SELECT digest FROM chat_ai_schema_migrations WHERE version=1),'')`).Scan(&old); err != nil {
		return err
	}
	if old != "" {
		if old != digest {
			return fmt.Errorf("chat AI migration checksum mismatch")
		}
		return tx.Commit(ctx)
	}
	if _, err = tx.Exec(ctx, schemaSQL); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO chat_ai_schema_migrations VALUES(1,$1)`, digest); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

package achievementv1

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed schema.sql
var schemaSQL string

// Migrate runs in a deployment migration command, never a request handler.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := MigrateTx(ctx, tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// MigrateTx is also used by isolated PostgreSQL integration tests.
func MigrateTx(ctx context.Context, tx pgx.Tx) error {
	// repo-guard: allow=long-number reason=synthetic-achievement-migration-lock
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(731482185)`); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS achievement_v1_schema_migrations(version integer PRIMARY KEY,digest text NOT NULL)`); err != nil {
		return err
	}
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(schemaSQL)))
	var existing string
	if err := tx.QueryRow(ctx, `SELECT COALESCE((SELECT digest FROM achievement_v1_schema_migrations WHERE version=1),'')`).Scan(&existing); err != nil {
		return err
	}
	if existing != "" {
		if existing != digest {
			return fmt.Errorf("achievement migration checksum mismatch")
		}
		return nil
	}
	if _, err := tx.Exec(ctx, schemaSQL); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `INSERT INTO achievement_v1_schema_migrations(version,digest) VALUES(1,$1)`, digest)
	return err
}

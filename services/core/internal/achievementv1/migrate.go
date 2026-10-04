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

//go:embed seen.sql
var seenSQL string

// migrations run in order, each once, each pinned by its digest. An applied
// migration is never edited: a change is a new version.
var migrations = []struct {
	version int
	sql     *string
}{{1, &schemaSQL}, {2, &seenSQL}}

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
	for _, m := range migrations {
		digest := fmt.Sprintf("%x", sha256.Sum256([]byte(*m.sql)))
		var existing string
		if err := tx.QueryRow(ctx, `SELECT COALESCE((SELECT digest FROM achievement_v1_schema_migrations WHERE version=$1),'')`, m.version).Scan(&existing); err != nil {
			return err
		}
		if existing != "" {
			if existing != digest {
				return fmt.Errorf("achievement migration %d checksum mismatch", m.version)
			}
			continue
		}
		if _, err := tx.Exec(ctx, *m.sql); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO achievement_v1_schema_migrations(version,digest) VALUES($1,$2)`, m.version, digest); err != nil {
			return err
		}
	}
	return nil
}

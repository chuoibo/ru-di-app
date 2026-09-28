package profilemedia

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed schema.sql
var schema string

// Migrate installs the Go-owned video credit ledger after achievements.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// repo-guard: allow=long-number reason=public-migration-advisory-lock
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(8310092301)`); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS profile_media_schema_migrations(version integer PRIMARY KEY,digest text NOT NULL)`); err != nil {
		return err
	}
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(schema)))
	var existing string
	if err := tx.QueryRow(ctx, `SELECT coalesce((SELECT digest FROM profile_media_schema_migrations WHERE version=1),'')`).Scan(&existing); err != nil {
		return err
	}
	if existing != "" && existing != digest {
		return fmt.Errorf("profile media migration checksum mismatch")
	}
	var installed bool
	if err := tx.QueryRow(ctx, `SELECT to_regclass(format('%I.profile_media_jobs',current_schema())) IS NOT NULL`).Scan(&installed); err != nil {
		return err
	}
	if !installed {
		if _, err := tx.Exec(ctx, schema); err != nil {
			return err
		}
	}
	if existing == "" {
		if _, err := tx.Exec(ctx, `INSERT INTO profile_media_schema_migrations VALUES(1,$1)`, digest); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// Package socialv2 owns the additive Go/SQL social profile extension.
package socialv2

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"mobile/services/core/internal/community"
)

//go:embed schema.sql
var schemaSQL string

// Migrate runs explicitly before social/v2 traffic is enabled. The profile
// wall writes comments and likes through Cộng đồng, so its schema comes first.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	if err := community.Migrate(ctx, pool); err != nil {
		return err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(845623)`); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS social_v2_schema_migrations(version integer PRIMARY KEY,digest text NOT NULL)`); err != nil {
		return err
	}
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(schemaSQL)))
	var installed string
	if err = tx.QueryRow(ctx, `SELECT coalesce((SELECT digest FROM social_v2_schema_migrations WHERE version=1),'')`).Scan(&installed); err != nil {
		return err
	}
	if installed != "" && installed != digest {
		return fmt.Errorf("social v2 migration checksum mismatch")
	}
	if installed == "" {
		if _, err = tx.Exec(ctx, schemaSQL); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO social_v2_schema_migrations(version,digest) VALUES(1,$1)`, digest); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// SchemaFiles returns the embedded SQL, so gates that read every Go trigger
// (internal/aigate) see this package's capture triggers.
func SchemaFiles() []string { return []string{schemaSQL} }

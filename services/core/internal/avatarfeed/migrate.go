// Package avatarfeed tells readers which avatar is current and pushes a hint
// when one changes (ADR-0020 §2.5, M8). It owns no picture: uploads stay in the
// photos routes, and this package only reads uploaded_images and memberships.
package avatarfeed

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed schema.sql
var migration string

// Migrate installs the notify trigger atomically. `core migrate-chat` runs it
// next to the chat feed; serving never runs DDL.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('avatar_feed_migration'))`); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS avatar_feed_schema_migrations(version integer PRIMARY KEY,digest text NOT NULL)`); err != nil {
		return err
	}
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(migration)))
	var old string
	if err = tx.QueryRow(ctx, `SELECT coalesce((SELECT digest FROM avatar_feed_schema_migrations WHERE version=1),'')`).Scan(&old); err != nil {
		return err
	}
	if old != "" && old != digest {
		return fmt.Errorf("avatar feed migration checksum mismatch")
	}
	if old == "" {
		if _, err = tx.Exec(ctx, migration); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO avatar_feed_schema_migrations VALUES(1,$1)`, digest); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// Installed reports whether serve can rely on the trigger.
func Installed(ctx context.Context, pool *pgxpool.Pool) (bool, error) {
	var ok bool
	err := pool.QueryRow(ctx, `SELECT to_regclass('avatar_feed_schema_migrations') IS NOT NULL AND EXISTS (SELECT 1 FROM pg_trigger WHERE tgname='avatar_feed_capture')`).Scan(&ok)
	return ok, err
}

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
var avatarTrigger string

//go:embed membership.sql
var membershipTrigger string

// migrations are applied in order, each once, each pinned by its digest: an
// edited file is a refusal, never a silent re-run. Version 1 shipped first and
// its text must not change.
var migrations = []string{avatarTrigger, membershipTrigger}

// SchemaFiles are the embedded migrations in order, for the gates that read
// what their triggers write (aigate).
func SchemaFiles() []string { return append([]string(nil), migrations...) }

// Migrate installs the notify triggers atomically. `core migrate-chat` runs it
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
	for i, sql := range migrations {
		version := i + 1
		digest := fmt.Sprintf("%x", sha256.Sum256([]byte(sql)))
		var old string
		if err = tx.QueryRow(ctx, `SELECT coalesce((SELECT digest FROM avatar_feed_schema_migrations WHERE version=$1),'')`, version).Scan(&old); err != nil {
			return err
		}
		if old != "" && old != digest {
			return fmt.Errorf("avatar feed migration %d checksum mismatch", version)
		}
		if old != "" {
			continue
		}
		if _, err = tx.Exec(ctx, sql); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO avatar_feed_schema_migrations VALUES($1,$2)`, version, digest); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// Installed reports whether serve can rely on the trigger.
func Installed(ctx context.Context, pool *pgxpool.Pool) (bool, error) {
	var ok bool
	err := pool.QueryRow(ctx, `SELECT to_regclass('avatar_feed_schema_migrations') IS NOT NULL AND EXISTS (SELECT 1 FROM pg_trigger WHERE tgname='avatar_feed_capture') AND EXISTS (SELECT 1 FROM pg_trigger WHERE tgname='avatar_feed_membership')`).Scan(&ok)
	return ok, err
}

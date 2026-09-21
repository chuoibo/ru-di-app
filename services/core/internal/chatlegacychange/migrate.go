// Package chatlegacychange repairs synchronization of the legacy archive during
// migration. It never enables plaintext chat v2 or changes writer ownership.
package chatlegacychange

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed schema.sql
var migration string

// Migrate installs opt-in capture atomically; normal server startup never runs DDL.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// repo-guard: allow=long-number reason=public-migration-advisory-lock
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(8310092101)`); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS chat_legacy_schema_migrations(version integer PRIMARY KEY,digest text NOT NULL)`); err != nil {
		return err
	}
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(migration)))
	var old string
	if err = tx.QueryRow(ctx, `SELECT coalesce((SELECT digest FROM chat_legacy_schema_migrations WHERE version=1),'')`).Scan(&old); err != nil {
		return err
	}
	if old != "" && old != digest {
		return fmt.Errorf("legacy chat migration checksum mismatch")
	}
	var installed bool
	if err = tx.QueryRow(ctx, `SELECT to_regclass(format('%I.chat_legacy_changes',current_schema())) IS NOT NULL`).Scan(&installed); err != nil {
		return err
	}
	if !installed {
		if _, err = tx.Exec(ctx, migration); err != nil {
			return err
		}
	}
	if old == "" {
		if _, err = tx.Exec(ctx, `INSERT INTO chat_legacy_schema_migrations VALUES(1,$1)`, digest); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// Unmigrate removes only this feature's metadata, never messages or votes.
func Unmigrate(ctx context.Context, pool *pgxpool.Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// repo-guard: allow=long-number reason=public-migration-advisory-lock
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(8310092101)`); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
 DROP TRIGGER IF EXISTS chat_legacy_message ON messages;
 DROP TRIGGER IF EXISTS chat_legacy_reaction ON message_reactions;
 DROP TRIGGER IF EXISTS chat_legacy_vote ON votes;
 DROP TRIGGER IF EXISTS chat_legacy_vote_option ON vote_options;
 DROP TRIGGER IF EXISTS chat_legacy_ballot ON vote_ballots;
 DROP FUNCTION IF EXISTS chat_legacy_capture_change();
 DROP TABLE IF EXISTS chat_legacy_change_outbox;
 DROP TABLE IF EXISTS chat_legacy_changes;
 DROP TABLE IF EXISTS chat_legacy_change_heads;
 DROP TABLE IF EXISTS chat_legacy_schema_migrations;`)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

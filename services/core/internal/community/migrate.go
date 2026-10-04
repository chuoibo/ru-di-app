// Package community owns moderated social discovery without reading private chat.
package community

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"mobile/services/core/internal/diary"
)

//go:embed schema.sql
var schema string

//go:embed feed_storage.sql
var feedStorage string

//go:embed post_metrics.sql
var postMetrics string

//go:embed media_review.sql
var mediaReview string

//go:embed notification_source.sql
var notificationSource string

//go:embed notification_actor_erasure.sql
var notificationActorErasure string

var migrations = []string{schema, feedStorage, postMetrics, mediaReview, notificationSource, notificationActorErasure}

// SchemaFiles returns the migrations the binary embeds, in order, so the AI
// trigger gate (internal/aigate) reads the same bytes this package installs.
func SchemaFiles() []string { return append([]string(nil), migrations...) }

// CheckSchema refuses a partially migrated or incompatible deployment before
// the API advertises readiness. It performs no DDL and never repairs checksums.
func CheckSchema(ctx context.Context, pool *pgxpool.Pool) error {
	var exists bool
	if err := pool.QueryRow(ctx, `SELECT to_regclass('community_migrations') IS NOT NULL`).Scan(&exists); err != nil {
		return fmt.Errorf("cannot check community schema")
	}
	if !exists {
		return fmt.Errorf("community schema is missing; run core migrate-community")
	}
	rows, err := pool.Query(ctx, `SELECT version,digest FROM community_migrations ORDER BY version`)
	if err != nil {
		return fmt.Errorf("cannot check community migrations")
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var version int
		var digest string
		if err = rows.Scan(&version, &digest); err != nil {
			return fmt.Errorf("cannot read community migrations")
		}
		if version != count+1 || version > len(migrations) || digest != fmt.Sprintf("%x", sha256.Sum256([]byte(migrations[version-1]))) {
			return fmt.Errorf("community schema version or checksum mismatch")
		}
		count++
	}
	if rows.Err() != nil || count != len(migrations) {
		return fmt.Errorf("community schema is incomplete; run core migrate-community")
	}
	return nil
}

// Migrate is an explicit operator command, never a request side effect.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	if err := diary.Migrate(ctx, pool); err != nil {
		return err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(734138)`); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS community_migrations(version integer PRIMARY KEY,digest text NOT NULL)`); err != nil {
		return err
	}
	for i, source := range migrations {
		version := i + 1
		digest := fmt.Sprintf("%x", sha256.Sum256([]byte(source)))
		var old string
		if err = tx.QueryRow(ctx, `SELECT COALESCE((SELECT digest FROM community_migrations WHERE version=$1),'')`, version).Scan(&old); err != nil {
			return err
		}
		if old != "" && old != digest {
			return fmt.Errorf("community migration checksum mismatch")
		}
		if old == "" {
			if _, err = tx.Exec(ctx, source); err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, `INSERT INTO community_migrations VALUES($1,$2)`, version, digest); err != nil {
				return err
			}
		}
	}
	return tx.Commit(ctx)
}

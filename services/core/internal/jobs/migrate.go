// Package jobs moves due jobs from Postgres to RabbitMQ and back to a worker:
// a payload-free transactional outbox, a relay that marks a row published only
// after the broker confirms it, the queue topology, and a strict consumer.
// Postgres stays the truth (ADR-0031 §2): losing RabbitMQ delays jobs, the
// workers' Postgres poller still claims them, and nothing acknowledged is lost.
package jobs

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed schema.sql
var schemaSQL string

// SchemaVersion is the outbox schema this binary reads and writes. `serve`
// and `work` refuse to start below it, as they do below chatassist's: the
// job table's trigger calls jobs_them, so a database without it fails every
// question at INSERT instead of once, loudly, at startup.
const SchemaVersion = 1

// SchemaSQL is the embedded version 1, for the gates that read it.
func SchemaSQL() string { return schemaSQL }

// SchemaCurrent reports whether the outbox is installed at SchemaVersion or
// later. It runs no DDL.
func SchemaCurrent(ctx context.Context, pool *pgxpool.Pool) (bool, error) {
	var table bool
	if err := pool.QueryRow(ctx, `SELECT to_regclass('job_schema_migrations') IS NOT NULL AND to_regclass('job_outbox') IS NOT NULL`).Scan(&table); err != nil || !table {
		return false, err
	}
	var current bool
	err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM job_schema_migrations WHERE version >= $1)`, SchemaVersion).Scan(&current)
	return current, err
}

// Migrate installs the outbox. Run by `core migrate-chat`, never by a request,
// and before chatassist.Migrate: the job table's trigger calls jobs_them.
// Its own version table, so it never competes for a chatassist version number.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(734130)`); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS job_schema_migrations(version integer PRIMARY KEY,digest text NOT NULL)`); err != nil {
		return err
	}
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(schemaSQL)))
	var old string
	if err = tx.QueryRow(ctx, `SELECT COALESCE((SELECT digest FROM job_schema_migrations WHERE version=1),'')`).Scan(&old); err != nil {
		return err
	}
	if old != "" {
		if old != digest {
			return fmt.Errorf("job outbox migration checksum mismatch")
		}
		return tx.Commit(ctx)
	}
	if _, err = tx.Exec(ctx, schemaSQL); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO job_schema_migrations VALUES(1,$1)`, digest); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

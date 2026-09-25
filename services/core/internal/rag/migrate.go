// Package rag is retrieval for the AI engine, lexical part (slice 8 of the
// approved plan; docs/claude/2026-09-25/thiet-ke-ai/04-rag-va-nap-du-lieu.md):
// a versioned snapshot index of the place catalogue, built from `places`,
// evaluated, promoted and rolled back by `core rag`, and Retrieve, which
// filters hard in SQL (allergy, open at the asked time, budget ceiling, diet,
// destination -- never relaxed), ranks by RRF over a full-text list, a
// trigram list and a closed-vocabulary list, and checks every hit a second
// time against the live `places` row before returning it.
//
// It is the only writer of every rag_* table, and it reads only those, the
// catalogue (`places`) and `destinations` (aigate/rag_gate_test.go). It calls
// no model: embeddings, enrichment and dedupe are slice 16.
package rag

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"fmt"

	"github.com/jackc/pgx/v5"
)

//go:embed schema_tu_vung.sql
var schemaTuVungSQL string

// SchemaVersion is the retrieval schema this binary reads and writes.
const SchemaVersion = 1

// Beginner is a pool or a transaction: something a migration, a build or a
// promotion can open its own transaction (or savepoint) on.
type Beginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// Migrate installs the retrieval schema. Run by `core migrate-rag`, never by
// a request. Its own version table, checksummed like chatassist/migrate.go,
// under its own advisory lock, so it never competes for a chatassist version.
func Migrate(ctx context.Context, db Beginner) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('rag_schema_migration'))`); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS rag_schema_migrations(version integer PRIMARY KEY,digest text NOT NULL)`); err != nil {
		return err
	}
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(schemaTuVungSQL)))
	var old string
	if err = tx.QueryRow(ctx, `SELECT COALESCE((SELECT digest FROM rag_schema_migrations WHERE version=1),'')`).Scan(&old); err != nil {
		return err
	}
	if old != "" {
		if old != digest {
			return fmt.Errorf("retrieval index migration checksum mismatch")
		}
		return tx.Commit(ctx)
	}
	if _, err = tx.Exec(ctx, schemaTuVungSQL); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO rag_schema_migrations VALUES(1,$1)`, digest); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Installed reports whether the retrieval schema is at SchemaVersion or later.
// It runs no DDL and cannot fail a transaction it runs in: to_regclass answers
// NULL for a table that does not exist.
func Installed(ctx context.Context, q Querier) (bool, error) {
	var ok bool
	if err := q.QueryRow(ctx, `SELECT to_regclass('rag_schema_migrations') IS NOT NULL AND to_regclass('rag_tombstones') IS NOT NULL`).Scan(&ok); err != nil || !ok {
		return false, err
	}
	err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM rag_schema_migrations WHERE version>=$1)`, SchemaVersion).Scan(&ok)
	return ok, err
}

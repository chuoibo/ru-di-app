// Package diary owns outing endings and personal memory books (ADR-0039).
package diary

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed schema.sql
var schema string

//go:embed schema_revocation.sql
var revocation string

//go:embed schema_erasure.sql
var erasure string

// SchemaFiles returns the migrations the binary embeds, in order, so the AI
// trigger gate (internal/aigate) reads the same bytes this package installs.
func SchemaFiles() []string { return []string{schema, revocation, erasure} }

// Migrate is an explicit operator action, never a side effect of a request.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(734137)`); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS outing_diary_migrations(version integer PRIMARY KEY,digest text NOT NULL)`); err != nil {
		return err
	}
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(schema)))
	var old string
	if err = tx.QueryRow(ctx, `SELECT COALESCE((SELECT digest FROM outing_diary_migrations WHERE version=1),'')`).Scan(&old); err != nil {
		return err
	}
	if old != "" && old != digest {
		return fmt.Errorf("diary migration checksum mismatch")
	}
	if old == "" {
		if _, err = tx.Exec(ctx, schema); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO outing_diary_migrations VALUES(1,$1)`, digest); err != nil {
			return err
		}
	}
	digest2 := fmt.Sprintf("%x", sha256.Sum256([]byte(revocation)))
	var old2 string
	if err = tx.QueryRow(ctx, `SELECT COALESCE((SELECT digest FROM outing_diary_migrations WHERE version=2),'')`).Scan(&old2); err != nil {
		return err
	}
	if old2 != "" && old2 != digest2 {
		return fmt.Errorf("diary revocation migration checksum mismatch")
	}
	if old2 == "" {
		if _, err = tx.Exec(ctx, revocation); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO outing_diary_migrations VALUES(2,$1)`, digest2); err != nil {
			return err
		}
	}
	digest3 := fmt.Sprintf("%x", sha256.Sum256([]byte(erasure)))
	var old3 string
	if err = tx.QueryRow(ctx, `SELECT COALESCE((SELECT digest FROM outing_diary_migrations WHERE version=3),'')`).Scan(&old3); err != nil {
		return err
	}
	if old3 != "" && old3 != digest3 {
		return fmt.Errorf("diary erasure migration checksum mismatch")
	}
	if old3 == "" {
		if _, err = tx.Exec(ctx, erasure); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO outing_diary_migrations VALUES(3,$1)`, digest3); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

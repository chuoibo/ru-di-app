package nepnho

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

// SchemaSQL is version 1, for the gates that read what the tables can hold.
func SchemaSQL() string { return schemaSQL }

// SchemaVersion is the memory schema this binary reads and writes. `serve`
// and `work` refuse to start below it: the account-deletion trigger lives in
// it, and a host without the trigger would delete accounts and keep memory.
const SchemaVersion = 1

// Migrate installs the schema in its own version table
// (nep_schema_migrations), checksummed like chatassist/migrate.go, under its
// own advisory lock. Run by `core migrate-chat`, never by a request.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('nep_schema_migration'))`); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS nep_schema_migrations(version integer PRIMARY KEY,digest text NOT NULL)`); err != nil {
		return err
	}
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(schemaSQL)))
	var old string
	if err = tx.QueryRow(ctx, `SELECT COALESCE((SELECT digest FROM nep_schema_migrations WHERE version=1),'')`).Scan(&old); err != nil {
		return err
	}
	if old != "" {
		if old != digest {
			return fmt.Errorf("Nếp memory migration checksum mismatch")
		}
		return tx.Commit(ctx)
	}
	// The enqueue trigger calls jobs_them: internal/jobs migrates first.
	var outbox bool
	if err = tx.QueryRow(ctx, `SELECT to_regprocedure('jobs_them(text,uuid,bigint,timestamptz,timestamptz)') IS NOT NULL`).Scan(&outbox); err != nil {
		return err
	}
	if !outbox {
		return fmt.Errorf("Nếp memory migration needs the job outbox (internal/jobs) first")
	}
	if _, err = tx.Exec(ctx, schemaSQL); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO nep_schema_migrations VALUES(1,$1)`, digest); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Reader is a pool or a transaction.
type Reader interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// SchemaCurrent reports whether version 1 is installed with its checksum,
// the account-deletion trigger is in place on people and the enqueue trigger
// on nep_xoa. It runs no DDL.
func SchemaCurrent(ctx context.Context, q Reader) (bool, error) {
	var table bool
	if err := q.QueryRow(ctx, `SELECT to_regclass('nep_schema_migrations') IS NOT NULL`).Scan(&table); err != nil || !table {
		return false, err
	}
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(schemaSQL)))
	var ok bool
	err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM nep_schema_migrations WHERE version=1 AND digest=$1)
	   AND EXISTS (SELECT 1 FROM pg_trigger WHERE tgname='nep_xoa_nguoi' AND tgrelid='people'::regclass AND tgenabled <> 'D')
	   AND EXISTS (SELECT 1 FROM pg_trigger WHERE tgname='nep_xoa_enqueue' AND tgrelid='nep_xoa'::regclass AND tgenabled <> 'D')`, digest).Scan(&ok)
	return ok, err
}

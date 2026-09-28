package nap

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"mobile/services/core/internal/repo"
)

//go:embed schema_nap_1.sql
var schemaNap1SQL string

//go:embed schema_nap_2.sql
var schemaNap2SQL string

// Querier is a pool, a connection or a transaction.
type Querier = repo.Querier

// Beginner opens a transaction (a pool) or a savepoint (a transaction).
type Beginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// CSDL is a pool or a transaction: queries, batches and nested
// transactions.
type CSDL interface {
	Querier
	Beginner
	SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults
}

// migrations are this package's schema versions, in order, in their own
// checksummed table (never a version number of rag's or chatassist's).
var migrations = []struct {
	version int
	sql     string
}{
	{1, schemaNap1SQL},
	{2, schemaNap2SQL},
}

// SchemaVersion is the ingestion schema this binary reads and writes.
const SchemaVersion = 2

// SchemaFiles are the embedded migrations, for the gates that read them.
func SchemaFiles() []string {
	out := make([]string, len(migrations))
	for i, m := range migrations {
		out[i] = m.sql
	}
	return out
}

// ErrThieuPhuThuoc: a schema this one builds on is missing.
var ErrThieuPhuThuoc = errors.New("nap: a schema this migration needs is missing")

// Migrate installs the ingestion schema, run by `core migrate-rag` after
// rag.Migrate and after the outbox's version 2 (`core migrate-chat`): the
// trigger on places calls jobs_them('rag', …). It fails closed, before any
// DDL, when either is missing.
func Migrate(ctx context.Context, db Beginner) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('rag_nap_schema_migration'))`); err != nil {
		return err
	}
	var places, tomb, outbox bool
	if err = tx.QueryRow(ctx, `SELECT to_regclass('places') IS NOT NULL, to_regclass('rag_tombstones') IS NOT NULL,
		to_regclass('job_schema_migrations') IS NOT NULL AND to_regprocedure('jobs_them(text,uuid,bigint,timestamptz,timestamptz)') IS NOT NULL`).Scan(&places, &tomb, &outbox); err != nil {
		return err
	}
	if !places || !tomb {
		return fmt.Errorf("%w: places (Alembic) and the rag schema (core migrate-rag) come first", ErrThieuPhuThuoc)
	}
	if outbox {
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM job_schema_migrations WHERE version>=2)`).Scan(&outbox); err != nil {
			return err
		}
	}
	if !outbox {
		return fmt.Errorf("%w: the job outbox at version 2 (core migrate-chat) comes first", ErrThieuPhuThuoc)
	}
	if _, err = tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS rag_nap_schema_migrations(version integer PRIMARY KEY,digest text NOT NULL)`); err != nil {
		return err
	}
	for _, m := range migrations {
		digest := fmt.Sprintf("%x", sha256.Sum256([]byte(m.sql)))
		var old string
		if err = tx.QueryRow(ctx, `SELECT COALESCE((SELECT digest FROM rag_nap_schema_migrations WHERE version=$1),'')`, m.version).Scan(&old); err != nil {
			return err
		}
		if old != "" {
			if old != digest {
				return fmt.Errorf("vector ingestion migration %d checksum mismatch", m.version)
			}
			continue
		}
		if _, err = tx.Exec(ctx, m.sql); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO rag_nap_schema_migrations VALUES($1,$2)`, m.version, digest); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// Installed reports whether the ingestion schema is at SchemaVersion or
// later, running no DDL.
func Installed(ctx context.Context, q Querier) (bool, error) {
	var ok bool
	if err := q.QueryRow(ctx, `SELECT to_regclass('rag_nap_schema_migrations') IS NOT NULL AND to_regclass('rag_vector_versions') IS NOT NULL`).Scan(&ok); err != nil || !ok {
		return false, err
	}
	err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM rag_nap_schema_migrations WHERE version>=$1)`, SchemaVersion).Scan(&ok)
	return ok, err
}

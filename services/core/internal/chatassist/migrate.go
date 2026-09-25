// Package chatassist implements explicit, invocation-only AI jobs for the legacy
// compatibility client. It cannot serve E2EE conversations or read chat history.
package chatassist

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

// Version 2 adds the group's shared sheet. It is a separate file, not an edit
// to schema.sql, because version 1 has already shipped: changing that file
// would fail every database that installed it with "checksum mismatch" and
// leave no way forward. A new version is the way forward.
//
//go:embed schema_drafts.sql
var draftsSQL string

// Version 3 adds the personal scope and the context the caller hands over.
// A third file, for the reason version 2 was a second one.
//
//go:embed schema_scope.sql
var scopeSQL string

// Version 4 lets the group AI answer inside the thread: the invocation names
// the `@Rủ Đi` message it answers, and the room it runs in carries its lane.
// A fourth file, for the reason version 2 was a second one.
//
// Its deadlock note was corrected before the version reached main (the first
// text claimed publish locked the message before the feed head, the order that
// deadlocked). That changed its checksum; no shared database had installed it,
// so only scratch databases on the working branch need it reinstalled.
//
//go:embed schema_luong.sql
var luongSQL string

// Version 5 puts the jobs on the queue: when a job is due, how many times it
// entered the queue, when its first content left, how many model calls it
// spent -- and the two triggers that write the outbox row in the transaction
// that makes a job due. A fifth file, for the reason version 2 was a second
// one.
//
//go:embed schema_hang_doi.sql
var hangDoiSQL string

// SchemaVersion is the chat AI schema this binary reads and writes. `serve`
// and `work` refuse to start below it: every claim names the columns of
// version 5, so an older schema would turn each job into an error behind a
// healthy /healthz instead of one loud refusal at startup.
const SchemaVersion = 5

// SchemaFiles are the embedded versions in order, for the gates that read
// what the schema does (aigate).
func SchemaFiles() []string { return []string{schemaSQL, draftsSQL, scopeSQL, luongSQL, hangDoiSQL} }

// SchemaCurrent reports whether every chat AI schema version up to
// SchemaVersion is installed. It runs no DDL. Every version, not only the
// highest: Migrate installs them in one transaction, so a gap is a database
// someone edited by hand, and a claim would fail on it.
func SchemaCurrent(ctx context.Context, pool *pgxpool.Pool) (bool, error) {
	var table bool
	if err := pool.QueryRow(ctx, `SELECT to_regclass('chat_ai_schema_migrations') IS NOT NULL`).Scan(&table); err != nil || !table {
		return false, err
	}
	var current bool
	err := pool.QueryRow(ctx, `SELECT count(DISTINCT version) = $1 FROM chat_ai_schema_migrations WHERE version BETWEEN 1 AND $1`, SchemaVersion).Scan(&current)
	return current, err
}

// Migrate is run explicitly by `core migrate-chat`, never by a request.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(734129)`); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS chat_ai_schema_migrations(version integer PRIMARY KEY,digest text NOT NULL)`); err != nil {
		return err
	}
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(schemaSQL)))
	var old string
	if err = tx.QueryRow(ctx, `SELECT COALESCE((SELECT digest FROM chat_ai_schema_migrations WHERE version=1),'')`).Scan(&old); err != nil {
		return err
	}
	if old != "" {
		if old != digest {
			return fmt.Errorf("chat AI migration checksum mismatch")
		}
	} else {
		if _, err = tx.Exec(ctx, schemaSQL); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO chat_ai_schema_migrations VALUES(1,$1)`, digest); err != nil {
			return err
		}
	}
	if err = migrateDrafts(ctx, tx); err != nil {
		return err
	}
	if err = migrateScope(ctx, tx); err != nil {
		return err
	}
	if err = migrateLuong(ctx, tx); err != nil {
		return err
	}
	if err = migrateHangDoi(ctx, tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// migrateHangDoi installs version 5 on top of version 4. Its trigger calls
// jobs_them, so it refuses a database where internal/jobs has not been
// installed: plpgsql would accept the function and fail every INSERT later.
func migrateHangDoi(ctx context.Context, tx pgx.Tx) error {
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(hangDoiSQL)))
	var old string
	if err := tx.QueryRow(ctx, `SELECT COALESCE((SELECT digest FROM chat_ai_schema_migrations WHERE version=5),'')`).Scan(&old); err != nil {
		return err
	}
	if old != "" {
		if old != digest {
			return fmt.Errorf("chat AI queue migration checksum mismatch")
		}
		return nil
	}
	var outbox bool
	if err := tx.QueryRow(ctx, `SELECT to_regprocedure('jobs_them(text,uuid,bigint,timestamptz,timestamptz)') IS NOT NULL`).Scan(&outbox); err != nil {
		return err
	}
	if !outbox {
		return fmt.Errorf("chat AI queue migration needs the job outbox: run jobs.Migrate first (core migrate-chat does)")
	}
	if _, err := tx.Exec(ctx, hangDoiSQL); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `INSERT INTO chat_ai_schema_migrations VALUES(5,$1)`, digest)
	return err
}

// migrateDrafts installs version 2 on top of whatever version 1 left behind,
// so a database that already carries the AI tables gains the shared sheet
// without being rebuilt.
func migrateDrafts(ctx context.Context, tx pgx.Tx) error {
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(draftsSQL)))
	var old string
	if err := tx.QueryRow(ctx, `SELECT COALESCE((SELECT digest FROM chat_ai_schema_migrations WHERE version=2),'')`).Scan(&old); err != nil {
		return err
	}
	if old != "" {
		if old != digest {
			return fmt.Errorf("chat shared draft migration checksum mismatch")
		}
		return nil
	}
	if _, err := tx.Exec(ctx, draftsSQL); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `INSERT INTO chat_ai_schema_migrations VALUES(2,$1)`, digest)
	return err
}

// migrateScope installs version 3 on top of version 2: the personal scope and
// the caller-supplied context column. Same shape as migrateDrafts, and it runs
// inside the same transaction, so a database arrives at one version or none.
func migrateScope(ctx context.Context, tx pgx.Tx) error {
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(scopeSQL)))
	var old string
	if err := tx.QueryRow(ctx, `SELECT COALESCE((SELECT digest FROM chat_ai_schema_migrations WHERE version=3),'')`).Scan(&old); err != nil {
		return err
	}
	if old != "" {
		if old != digest {
			return fmt.Errorf("chat AI scope migration checksum mismatch")
		}
		return nil
	}
	if _, err := tx.Exec(ctx, scopeSQL); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `INSERT INTO chat_ai_schema_migrations VALUES(3,$1)`, digest)
	return err
}

// migrateLuong installs version 4 on top of version 3, in the same
// transaction and with the same pinned checksum as the versions before it.
func migrateLuong(ctx context.Context, tx pgx.Tx) error {
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(luongSQL)))
	var old string
	if err := tx.QueryRow(ctx, `SELECT COALESCE((SELECT digest FROM chat_ai_schema_migrations WHERE version=4),'')`).Scan(&old); err != nil {
		return err
	}
	if old != "" {
		if old != digest {
			return fmt.Errorf("chat AI thread migration checksum mismatch")
		}
		return nil
	}
	if _, err := tx.Exec(ctx, luongSQL); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `INSERT INTO chat_ai_schema_migrations VALUES(4,$1)`, digest)
	return err
}

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

// Migrate is run explicitly by the candidate migration command, never a request.
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
	return tx.Commit(ctx)
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

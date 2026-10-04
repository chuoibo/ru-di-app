package accountauth

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed schema.sql
var schema string

// Migrate runs only from the operator command, never from HTTP startup.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(5353)`); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS managed_auth_migrations(version integer PRIMARY KEY,digest text NOT NULL)`); err != nil {
		return err
	}
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(schema)))
	var installed string
	if err = tx.QueryRow(ctx, `SELECT coalesce((SELECT digest FROM managed_auth_migrations WHERE version=1),'')`).Scan(&installed); err != nil {
		return err
	}
	if installed != "" {
		if installed != digest {
			return fmt.Errorf("managed auth schema checksum mismatch")
		}
		return tx.Commit(ctx)
	}
	if _, err = tx.Exec(ctx, schema); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO managed_auth_migrations VALUES(1,$1)`, digest); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func CheckSchema(ctx context.Context, pool *pgxpool.Pool) error {
	var digest string
	if err := pool.QueryRow(ctx, `SELECT digest FROM managed_auth_migrations WHERE version=1`).Scan(&digest); err != nil {
		return fmt.Errorf("run core migrate-accounts before enabling managed auth")
	}
	if digest != fmt.Sprintf("%x", sha256.Sum256([]byte(schema))) {
		return fmt.Errorf("managed auth schema checksum mismatch")
	}
	return nil
}

// SchemaSQL is included in the repository-wide trigger access audit.
func SchemaSQL() string { return schema }

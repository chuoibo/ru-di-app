package community

import (
	"context"
	"crypto/sha256"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"mobile/services/core/internal/testdb"
)

func TestPostgresCommunityReadinessRejectsMissingPartialAndDriftedSchema(t *testing.T) {
	base := testdb.Pool(t)
	ctx := context.Background()
	schemaName := "community_readiness_" + fmt.Sprintf("%x", sha256.Sum256([]byte(uuid())))[:16]
	identifier := pgx.Identifier{schemaName}.Sanitize()
	if _, err := base.Exec(ctx, "CREATE SCHEMA "+identifier); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = base.Exec(ctx, "DROP SCHEMA "+identifier+" CASCADE") })
	config := base.Config().Copy()
	config.ConnConfig.RuntimeParams["search_path"] = schemaName
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err = CheckSchema(ctx, pool); err == nil {
		t.Fatal("missing schema advertised readiness")
	}
	if _, err = pool.Exec(ctx, `CREATE TABLE community_migrations(version integer PRIMARY KEY,digest text NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	for i, source := range migrations {
		if err = CheckSchema(ctx, pool); err == nil {
			t.Fatalf("partial schema with %d migrations advertised readiness", i)
		}
		if _, err = pool.Exec(ctx, `INSERT INTO community_migrations VALUES($1,$2)`, i+1, fmt.Sprintf("%x", sha256.Sum256([]byte(source)))); err != nil {
			t.Fatal(err)
		}
	}
	if err = CheckSchema(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `UPDATE community_migrations SET digest='incompatible' WHERE version=1`); err != nil {
		t.Fatal(err)
	}
	if err = CheckSchema(ctx, pool); err == nil {
		t.Fatal("checksum drift advertised readiness")
	}
	if err = Migrate(ctx, base); err != nil {
		t.Fatal(err)
	}
	if err = CheckSchema(ctx, base); err != nil {
		t.Fatal("fully migrated schema refused:", err)
	}
}

//go:build postgres

package db

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// testPool connects to the disposable database scripts/go_postgres_tier.sh
// provisions. Without one the test skips — unless the tier asked for the
// database, in which case a missing one is a failure, never a quiet skip.
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("CORE_TEST_DATABASE_URL")
	if url == "" {
		if os.Getenv("CORE_REQUIRE_POSTGRES_TESTS") == "1" {
			t.Fatal("CORE_REQUIRE_POSTGRES_TESTS=1 but CORE_TEST_DATABASE_URL is empty")
		}
		t.Skip("CORE_TEST_DATABASE_URL not set; run scripts/go_postgres_tier.sh")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// The tier's sentinel: it proves the run reached a database Alembic migrated.
func TestPostgresTierReachesDatabase(t *testing.T) {
	pool := testPool(t)
	var head string
	if err := pool.QueryRow(context.Background(), "SELECT version_num FROM alembic_version").Scan(&head); err != nil {
		t.Fatalf("no alembic_version: the schema was not migrated: %v", err)
	}
	if head == "" {
		t.Fatal("alembic_version is empty")
	}
	var isolation string
	if err := pool.QueryRow(context.Background(), "SHOW default_transaction_isolation").Scan(&isolation); err != nil {
		t.Fatal(err)
	}
	if isolation != "read committed" {
		t.Fatalf("default isolation = %q, Python runs under read committed", isolation)
	}
}

func scratch(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	// The database is disposable and owned by this test run; a scratch table
	// keeps the assertions off the application schema.
	if _, err := pool.Exec(context.Background(),
		"CREATE TABLE IF NOT EXISTS core_tier_scratch (id text PRIMARY KEY, note text NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), "TRUNCATE core_tier_scratch"); err != nil {
		t.Fatal(err)
	}
}

func visible(t *testing.T, pool *pgxpool.Pool, id string) bool {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM core_tier_scratch WHERE id = $1", id).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n == 1
}

func TestUnitCommitMakesTheWriteVisible(t *testing.T) {
	pool := testPool(t)
	scratch(t, pool)
	unit := NewUnit(pool)
	tx, err := unit.Tx(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(context.Background(), "INSERT INTO core_tier_scratch VALUES ('committed', 'x')"); err != nil {
		t.Fatal(err)
	}
	if visible(t, pool, "committed") {
		t.Fatal("uncommitted write visible to another connection")
	}
	if err := unit.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !visible(t, pool, "committed") {
		t.Fatal("committed write not visible")
	}
}

func TestUnitRollbackDiscardsTheWrite(t *testing.T) {
	pool := testPool(t)
	scratch(t, pool)
	unit := NewUnit(pool)
	tx, err := unit.Tx(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(context.Background(), "INSERT INTO core_tier_scratch VALUES ('refused', 'x')"); err != nil {
		t.Fatal(err)
	}
	if err := unit.Rollback(context.Background()); err != nil {
		t.Fatal(err)
	}
	if visible(t, pool, "refused") {
		t.Fatal("rolled-back write is visible")
	}
}

func TestUnitReusesOneTransaction(t *testing.T) {
	pool := testPool(t)
	unit := NewUnit(pool)
	defer unit.Rollback(context.Background())
	first, err := unit.Tx(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	second, err := unit.Tx(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var a, b int64
	if err := first.QueryRow(context.Background(), "SELECT pg_backend_pid()").Scan(&a); err != nil {
		t.Fatal(err)
	}
	if err := second.QueryRow(context.Background(), "SELECT pg_backend_pid()").Scan(&b); err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatalf("two Tx calls used two connections (%d, %d)", a, b)
	}
}

// Package testdb connects real-PostgreSQL tests to the disposable database
// scripts/go_postgres_tier.sh provisions.
//
// Without CORE_TEST_DATABASE_URL a test skips — unless the tier set
// CORE_REQUIRE_POSTGRES_TESTS=1, in which case a missing database fails the
// test. A skip is not a pass (CLAUDE.md), and the tier refuses to be green on
// skips.
package testdb

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool returns a pool for the test database, closed when the test ends.
func Pool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := strings.TrimSpace(os.Getenv("CORE_TEST_DATABASE_URL"))
	if url == "" {
		if os.Getenv("CORE_REQUIRE_POSTGRES_TESTS") == "1" {
			t.Fatal("CORE_REQUIRE_POSTGRES_TESTS=1 but CORE_TEST_DATABASE_URL is empty")
		}
		t.Skip("CORE_TEST_DATABASE_URL not set; run scripts/go_postgres_tier.sh")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// Tx returns a transaction rolled back when the test ends, so fixtures a test
// inserts never reach the next test.
func Tx(t *testing.T) pgx.Tx {
	t.Helper()
	pool := Pool(t)
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	return tx
}

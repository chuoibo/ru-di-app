// Package db owns the Go core's PostgreSQL access: one pool per process and
// one lazily started transaction per request, committed before the response
// is written — the shape services/api gets from get_repository plus
// install_commit_before_response (app/api/deps.py, app/api/unit_of_work.py).
//
// The rules copied from Python:
//   - a request that never touches the database opens no transaction and
//     commits nothing (SQLAlchemy autobegins on first use);
//   - a handler that returns normally commits BEFORE the response is sent, so a
//     client never sees a success whose write was lost;
//   - a handler that fails (an ApiProblem or anything else) rolls back, so
//     writes made before a refusal never land;
//   - isolation is the server default, READ COMMITTED, like psycopg's.
//
// Alembic owns the schema. Nothing here issues DDL.
package db

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// EnvDatabaseURL is the variable services/api reads for its engine.
const EnvDatabaseURL = "MOBILE_DATABASE_URL"

// maxConns keeps Go plus SQLAlchemy under PostgreSQL's default 100 connections
// while both processes run against one database (ADR-0029 §2.2).
const maxConns = 15

// EnvMaxConns overrides maxConns for one process. A shared server gives the
// whole app a connection budget (the vnlocal box: 40 for `rudi_owner`), and
// the processes split it -- the API server needs many, the catalogue sync a
// few. It is a separate variable because MOBILE_DATABASE_URL is shared with
// SQLAlchemy, whose driver would refuse pgx's `pool_max_conns` parameter.
const EnvMaxConns = "MOBILE_DB_MAX_CONNS"

// PoolConfig parses a database URL, accepting the SQLAlchemy spelling the
// Python service uses ("postgresql+psycopg://...").
func PoolConfig(raw string) (*pgxpool.Config, error) {
	url := strings.TrimSpace(raw)
	if url == "" {
		return nil, errors.New(EnvDatabaseURL + " is required")
	}
	for _, driver := range []string{"postgresql+psycopg://", "postgresql+psycopg2://"} {
		if strings.HasPrefix(url, driver) {
			url = "postgresql://" + strings.TrimPrefix(url, driver)
		}
	}
	if !strings.HasPrefix(url, "postgresql://") && !strings.HasPrefix(url, "postgres://") {
		return nil, fmt.Errorf("%s must be a postgresql:// URL", EnvDatabaseURL)
	}
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		// pgx echoes the URL in some errors; never let a password reach a log.
		return nil, fmt.Errorf("%s is not a valid PostgreSQL URL", EnvDatabaseURL)
	}
	// Let the server notice a client that died. A process killed in the middle
	// of a batch leaves its backend waiting to write to a socket nobody reads;
	// statement_timeout cannot fire there, and the OS keepalive default is two
	// hours. Seen on the vnlocal box: a replaced sync container's INSERT sat
	// "active" for 52 minutes holding the pull lock, and the new one waited on
	// it. These are ordinary per-session settings (PostgreSQL 12+), ignored on
	// a unix socket; a URL that sets them itself wins.
	for key, value := range map[string]string{
		"tcp_keepalives_idle":     "60",
		"tcp_keepalives_interval": "10",
		"tcp_keepalives_count":    "6",
		"tcp_user_timeout":        "60000",
	} {
		if _, set := config.ConnConfig.RuntimeParams[key]; !set {
			config.ConnConfig.RuntimeParams[key] = value
		}
	}
	config.MaxConns = maxConns
	if n, err := strconv.Atoi(strings.TrimSpace(os.Getenv(EnvMaxConns))); err == nil && n > 0 && n <= 100 {
		config.MaxConns = int32(n)
	}
	return config, nil
}

// ServerSessionDefaults are the per-session limits of the API server's pool
// (audit 2026-10-05, DB-01): no request-serving statement runs for more than
// 30 s, waits on a lock for more than 10 s, or leaves a transaction idle for
// more than 2 minutes (the longest model call a request makes inside its unit
// is 45 s). Batch commands (rag, ingest, migrate) open their pools with Open
// and keep the server's unbounded defaults. A URL that sets one itself wins.
var ServerSessionDefaults = map[string]string{
	"statement_timeout":                   "30000",
	"lock_timeout":                        "10000",
	"idle_in_transaction_session_timeout": "120000",
}

// ServerPoolConfig is PoolConfig plus ServerSessionDefaults.
func ServerPoolConfig(raw string) (*pgxpool.Config, error) {
	config, err := PoolConfig(raw)
	if err != nil {
		return nil, err
	}
	for key, value := range ServerSessionDefaults {
		if _, set := config.ConnConfig.RuntimeParams[key]; !set {
			config.ConnConfig.RuntimeParams[key] = value
		}
	}
	return config, nil
}

// OpenServer is Open for the API server: ServerPoolConfig, connected lazily.
func OpenServer(ctx context.Context, raw string) (*pgxpool.Pool, error) {
	config, err := ServerPoolConfig(raw)
	if err != nil {
		return nil, err
	}
	return pgxpool.NewWithConfig(ctx, config)
}

// Open builds the process-wide pool. It does not connect eagerly: like the
// Python engine, the first query opens the first connection.
func Open(ctx context.Context, raw string) (*pgxpool.Pool, error) {
	config, err := PoolConfig(raw)
	if err != nil {
		return nil, err
	}
	return pgxpool.NewWithConfig(ctx, config)
}

// Unit is the transaction one request owns.
type Unit struct {
	pool *pgxpool.Pool
	mu   sync.Mutex
	tx   pgx.Tx
	done bool
}

// NewUnit returns a unit that has not begun.
func NewUnit(pool *pgxpool.Pool) *Unit {
	return &Unit{pool: pool}
}

// ErrUnitFinished is returned when a finished unit is asked for a transaction.
var ErrUnitFinished = errors.New("db: unit already committed or rolled back")

// Tx returns the request's transaction, beginning it on first use.
func (u *Unit) Tx(ctx context.Context) (pgx.Tx, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.done {
		return nil, ErrUnitFinished
	}
	if u.tx == nil {
		tx, err := u.pool.Begin(ctx)
		if err != nil {
			return nil, err
		}
		u.tx = tx
	}
	return u.tx, nil
}

// Begun reports whether any query opened the transaction.
func (u *Unit) Begun() bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.tx != nil
}

// Commit commits a begun transaction; a unit that never began commits nothing.
func (u *Unit) Commit(ctx context.Context) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.done {
		return nil
	}
	u.done = true
	if u.tx == nil {
		return nil
	}
	return u.tx.Commit(ctx)
}

// Rollback discards a begun transaction. It is safe after Commit and on a unit
// that never began, so callers can defer it unconditionally.
func (u *Unit) Rollback(ctx context.Context) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.done {
		return nil
	}
	u.done = true
	if u.tx == nil {
		return nil
	}
	return u.tx.Rollback(ctx)
}

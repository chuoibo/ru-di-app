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
	config.MaxConns = maxConns
	return config, nil
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

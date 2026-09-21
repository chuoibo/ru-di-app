// Package repo holds the Go core's SQL. Each query mirrors the one
// SqlAlchemyApiRepository issues for the same question, in the same order, so a
// PostgreSQL statement log of the two services can be compared (ADR-0029 §2.6).
package repo

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"mobile/services/core/internal/auth"
)

// Querier is what both a pool and a request transaction offer.
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Sessions implements auth.SessionStore. It is the prod authentication path's
// view of two repository methods and issues no statement of its own: both live
// in account_sessions.go, so the door every request comes through spells
// get_account_session_by_digest and actor_grants exactly as the Python does.
type Sessions struct {
	Q Querier
}

var _ auth.SessionStore = Sessions{}

// SessionByDigest is get_account_session_by_digest, narrowed to the three
// columns authentication reads.
func (s Sessions) SessionByDigest(ctx context.Context, digest []byte) (*auth.SessionRecord, error) {
	record, err := (Repository{Q: s.Q}).GetAccountSessionByDigest(ctx, digest)
	if err != nil || record == nil {
		return nil, err
	}
	return &auth.SessionRecord{PersonID: record.PersonID, ExpiresAt: record.ExpiresAt,
		RevokedAt: record.RevokedAt}, nil
}

// Grants is actor_grants: the person row first, then every membership row.
func (s Sessions) Grants(ctx context.Context, personID string) (auth.Grants, error) {
	grants, err := (Repository{Q: s.Q}).ActorGrants(ctx, personID)
	if err != nil {
		return auth.Grants{}, err
	}
	if !grants.PersonExists {
		return auth.Grants{PersonExists: false}, nil
	}
	return auth.Grants{PersonExists: true, Roles: grants.Roles, Contexts: grants.ContextIDs}, nil
}

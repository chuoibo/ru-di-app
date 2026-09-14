// Package repo holds the Go core's SQL. Each query mirrors the one
// SqlAlchemyApiRepository issues for the same question, in the same order, so a
// PostgreSQL statement log of the two services can be compared (ADR-0029 §2.6).
package repo

import (
	"context"
	"errors"
	"time"

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

// Sessions implements auth.SessionStore.
type Sessions struct {
	Q Querier
}

var _ auth.SessionStore = Sessions{}

// SessionByDigest is get_account_session_by_digest.
func (s Sessions) SessionByDigest(ctx context.Context, digest []byte) (*auth.SessionRecord, error) {
	var record auth.SessionRecord
	var revoked *time.Time
	err := s.Q.QueryRow(ctx,
		`SELECT person_id::text, expires_at, revoked_at
		   FROM account_sessions
		  WHERE token_digest = $1
		  LIMIT 1`, digest).Scan(&record.PersonID, &record.ExpiresAt, &revoked)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	record.RevokedAt = revoked
	return &record, nil
}

// Grants is actor_grants: the person row first, then every membership row.
func (s Sessions) Grants(ctx context.Context, personID string) (auth.Grants, error) {
	var deletedAt *time.Time
	err := s.Q.QueryRow(ctx, `SELECT deleted_at FROM people WHERE id = $1`, personID).Scan(&deletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.Grants{PersonExists: false}, nil
	}
	if err != nil {
		return auth.Grants{}, err
	}
	if deletedAt != nil {
		return auth.Grants{PersonExists: false}, nil
	}

	rows, err := s.Q.Query(ctx,
		`SELECT context_id::text, state FROM memberships WHERE person_id = $1`, personID)
	if err != nil {
		return auth.Grants{}, err
	}
	defer rows.Close()
	states := map[string][]string{}
	for rows.Next() {
		var context, state string
		if err := rows.Scan(&context, &state); err != nil {
			return auth.Grants{}, err
		}
		states[context] = append(states[context], state)
	}
	if err := rows.Err(); err != nil {
		return auth.Grants{}, err
	}
	return auth.GrantsFromMemberships(states), nil
}

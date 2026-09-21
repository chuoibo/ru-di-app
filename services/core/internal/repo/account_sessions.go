package repo

// The W9 slice of SqlAlchemyApiRepository: the rows behind a bearer session
// (services/api/app/api/repository.py). Seven routes reach this file --
// POST/GET/DELETE /sessions, DELETE /sessions/{id}, and the three doors of
// routes/auth.py -- and every one of them is the root of trust, so each method
// below issues the statements the Python method issues, in its order.
//
// Two shapes repeat and mean different things:
//
//   - `session.get(AccountSession, id)` labels its columns
//     `account_sessions_<column>`; `session.scalar(select(AccountSession)...)`
//     does not label them at all. Which one a method used is visible in the
//     statement log, so the two are kept apart rather than folded into one
//     SELECT that would read the same rows.
//   - the raw token never appears here. What the wire calls a session is a
//     SHA-256 digest in `token_digest`, and the only read of it is
//     SessionByDigest; the id path and the digest path have to resolve to the
//     same row, which auth_repo_oracle_postgres_test.go walks both ways.

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"mobile/services/core/internal/auth"
)

// AccountSession is AccountSessionRecord: a stored session without the secret
// that reaches it. token_digest is read by the SELECTs but is not a field of
// the Python record either, so it stays unexported.
type AccountSession struct {
	ID                 string
	PersonID           string
	IssuedFromInviteID *string
	IssuedVia          string
	CreatedAt          time.Time
	ExpiresAt          time.Time
	RevokedAt          *time.Time

	digest []byte
}

// accountSessionColumns is `select(AccountSession)`: every mapped column in
// declaration order, unlabelled.
const accountSessionColumns = `account_sessions.id, account_sessions.person_id, account_sessions.token_digest,
	        account_sessions.issued_from_invite_id, account_sessions.issued_via, account_sessions.created_at,
	        account_sessions.expires_at, account_sessions.revoked_at`

// accountSessionColumnsByID is `session.get(AccountSession, id)`.
const accountSessionColumnsByID = `account_sessions.id AS account_sessions_id,
	        account_sessions.person_id AS account_sessions_person_id,
	        account_sessions.token_digest AS account_sessions_token_digest,
	        account_sessions.issued_from_invite_id AS account_sessions_issued_from_invite_id,
	        account_sessions.issued_via AS account_sessions_issued_via,
	        account_sessions.created_at AS account_sessions_created_at,
	        account_sessions.expires_at AS account_sessions_expires_at,
	        account_sessions.revoked_at AS account_sessions_revoked_at`

var accountSessionInsert = []insertColumn{{"id", "::UUID"}, {"person_id", "::UUID"}, {"token_digest", ""},
	{"issued_from_invite_id", "::UUID"}, {"issued_via", "::VARCHAR"},
	{"created_at", "::TIMESTAMP WITH TIME ZONE"}, {"expires_at", "::TIMESTAMP WITH TIME ZONE"},
	{"revoked_at", "::TIMESTAMP WITH TIME ZONE"}}

func scanAccountSession(row scannable) (*AccountSession, error) {
	var s AccountSession
	err := row.Scan(&s.ID, &s.PersonID, &s.digest, &s.IssuedFromInviteID, &s.IssuedVia,
		&s.CreatedAt, &s.ExpiresAt, &s.RevokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	s.CreatedAt, s.ExpiresAt = s.CreatedAt.UTC(), s.ExpiresAt.UTC()
	s.RevokedAt = utcOptional(s.RevokedAt)
	return &s, nil
}

// AccountSessionInput is create_account_session's keyword arguments.
type AccountSessionInput struct {
	PersonID           string
	TokenDigest        []byte
	IssuedFromInviteID *string
	ExpiresAt          time.Time
	Now                time.Time
	// IssuedVia is the Python keyword's `None` when empty: the method then
	// falls back to what the old schema encoded, `invite` when an invitation
	// is named and `genesis` when none is.
	IssuedVia string
}

// CreateAccountSession is create_account_session: one INSERT of every mapped
// column. created_at is written from the caller's clock rather than left to
// the server default, so nothing here reads the database's now(); revoked_at
// carries no default and is written NULL.
func (r Repository) CreateAccountSession(ctx context.Context, in AccountSessionInput) (AccountSession, error) {
	issuedVia := in.IssuedVia
	if issuedVia == "" {
		issuedVia = "genesis"
		if in.IssuedFromInviteID != nil {
			issuedVia = "invite"
		}
	}
	id, err := newUUID()
	if err != nil {
		return AccountSession{}, err
	}
	created, expires := pythonInstant(in.Now), pythonInstant(in.ExpiresAt)
	if _, err := r.Q.Exec(ctx, renderInsert("account_sessions", accountSessionInsert, 1),
		id, in.PersonID, in.TokenDigest, in.IssuedFromInviteID, issuedVia, created, expires,
		nil); err != nil {
		return AccountSession{}, err
	}
	return AccountSession{ID: id, PersonID: in.PersonID, IssuedFromInviteID: in.IssuedFromInviteID,
		IssuedVia: issuedVia, CreatedAt: created.UTC(), ExpiresAt: expires.UTC(),
		digest: in.TokenDigest}, nil
}

// GetAccountSessionByDigest is get_account_session_by_digest: the one read of
// a bearer in the product. token_digest is unique and LIMIT 1 says so.
func (r Repository) GetAccountSessionByDigest(ctx context.Context, tokenDigest []byte) (*AccountSession, error) {
	return scanAccountSession(r.Q.QueryRow(ctx,
		`SELECT `+accountSessionColumns+`
		   FROM account_sessions
		  WHERE account_sessions.token_digest = $1
		  LIMIT $2::INTEGER`, tokenDigest, 1))
}

// GetAccountSession is get_account_session: the row by primary key, nil when
// there is none. `DELETE /sessions/{id}` reaches a row this way that
// `DELETE /sessions/current` reaches by digest; both must end at one row.
func (r Repository) GetAccountSession(ctx context.Context, sessionID string) (*AccountSession, error) {
	return scanAccountSession(r.Q.QueryRow(ctx,
		`SELECT `+accountSessionColumnsByID+`
		   FROM account_sessions
		  WHERE account_sessions.id = $1::UUID`, sessionID))
}

// ListAccountSessions is list_account_sessions: this person's sessions that a
// bearer could still use, newest first with the id breaking ties.
//
// The deadline is compared with `>`, so a session expiring at exactly `now` is
// already gone -- the same boundary actor_for_session_token draws with `<=`
// from the other side. `now` is the caller's clock and never the database's.
func (r Repository) ListAccountSessions(ctx context.Context, personID string, now time.Time) ([]AccountSession, error) {
	rows, err := r.Q.Query(ctx,
		`SELECT `+accountSessionColumns+`
		   FROM account_sessions
		  WHERE account_sessions.person_id = $1::UUID AND account_sessions.revoked_at IS NULL
		    AND account_sessions.expires_at > $2::TIMESTAMP WITH TIME ZONE
		  ORDER BY account_sessions.created_at DESC, account_sessions.id`,
		personID, pythonInstant(now))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AccountSession{}
	for rows.Next() {
		session, err := scanAccountSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *session)
	}
	return out, rows.Err()
}

// RevokeAccountSession is revoke_account_session: the row by primary key, then
// one UPDATE of revoked_at.
//
// A session already revoked is returned as it stands with no second write. The
// moment it was revoked at must not move: signing out twice is not an event,
// and an audit reading the column wants the first refusal, not the last tap.
func (r Repository) RevokeAccountSession(ctx context.Context, sessionID string, now time.Time) (*AccountSession, error) {
	session, err := r.GetAccountSession(ctx, sessionID)
	if err != nil || session == nil {
		return nil, err
	}
	if session.RevokedAt != nil {
		return session, nil
	}
	revoked := pythonInstant(now)
	if err := r.execUpdate(ctx,
		`UPDATE account_sessions SET revoked_at=$1::TIMESTAMP WITH TIME ZONE
		  WHERE account_sessions.id = $2::UUID`, revoked, session.ID); err != nil {
		return nil, err
	}
	session.RevokedAt = &revoked
	return session, nil
}

// ActorGrants is ActorGrants: what the roster says a session holder may claim.
// PersonExists is apart from an empty Roles on purpose -- a session whose
// person row was deleted is an authentication failure, while a person with no
// membership yet is an ordinary invitee who still has to accept.
type ActorGrants struct {
	PersonExists bool
	Roles        []string
	ContextIDs   []string
}

// ActorGrants is actor_grants: the person row first (an erased account answers
// person_exists=False before any membership is read), then every membership of
// that person, unordered and unlocked.
//
// What the rows mean is auth.GrantsFromMemberships, the one spelling of the
// rule this repository and the prod authentication path share: the five base
// roles for any authenticated person, `former_member` for a membership that
// ended, contexts from the active ones only, and the membership `role` column
// read into the SELECT and then deliberately dropped (`group_admin` is a fact
// about one row and a flat role set cannot say which group it means).
//
// Both frozensets come back sorted. The Python sets carry no order, so sorting
// is the one spelling that can be compared at all.
func (r Repository) ActorGrants(ctx context.Context, personID string) (ActorGrants, error) {
	person, err := scanPerson(r.Q.QueryRow(ctx,
		`SELECT `+personColumns+`
		   FROM people
		  WHERE people.id = $1::UUID`, personID))
	if err != nil {
		return ActorGrants{}, err
	}
	if person == nil || person.DeletedAt != nil {
		return ActorGrants{PersonExists: false, Roles: []string{}, ContextIDs: []string{}}, nil
	}

	rows, err := r.Q.Query(ctx,
		`SELECT memberships.context_id, memberships.role, memberships.state
		   FROM memberships
		  WHERE memberships.person_id = $1::UUID`, personID)
	if err != nil {
		return ActorGrants{}, err
	}
	defer rows.Close()
	states := map[string][]string{}
	for rows.Next() {
		var contextID, role, state string
		if err := rows.Scan(&contextID, &role, &state); err != nil {
			return ActorGrants{}, err
		}
		// `del role` in the Python, for the reason above.
		_ = role
		states[contextID] = append(states[contextID], state)
	}
	if err := rows.Err(); err != nil {
		return ActorGrants{}, err
	}
	granted := auth.GrantsFromMemberships(states)
	return ActorGrants{PersonExists: true, Roles: granted.Roles, ContextIDs: granted.Contexts}, nil
}

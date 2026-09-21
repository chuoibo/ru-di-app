package repo

// The W7 invitations of a trip, and the membership a redeemed link creates.
//
// The server never stores the secret it hands out: an invitation carries only
// a SHA-256 digest of its token, the same shape a guest link has. Revoking and
// rotating both change what a token already in somebody's hands resolves to,
// and rotating is the only way back in for a named person who lost their
// phone -- uq_outing_invites_person allows one named row per person per trip,
// so a second invitation is not available as a second chance.

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// outingInviteColumns is `select(OutingInvite)`: every mapped column in
// declaration order.
const outingInviteColumns = `outing_invites.id, outing_invites.outing_id, outing_invites.source,
	        outing_invites.invited_person_id, outing_invites.invited_by_id, outing_invites.token_digest,
	        outing_invites.accepted_at, outing_invites.accepted_by_id, outing_invites.created_at,
	        outing_invites.expires_at, outing_invites.revoked_at`

// outingInviteColumnsByID is `session.get(OutingInvite, id)`.
const outingInviteColumnsByID = `outing_invites.id AS outing_invites_id, outing_invites.outing_id AS outing_invites_outing_id,
	        outing_invites.source AS outing_invites_source,
	        outing_invites.invited_person_id AS outing_invites_invited_person_id,
	        outing_invites.invited_by_id AS outing_invites_invited_by_id,
	        outing_invites.token_digest AS outing_invites_token_digest,
	        outing_invites.accepted_at AS outing_invites_accepted_at,
	        outing_invites.accepted_by_id AS outing_invites_accepted_by_id,
	        outing_invites.created_at AS outing_invites_created_at,
	        outing_invites.expires_at AS outing_invites_expires_at,
	        outing_invites.revoked_at AS outing_invites_revoked_at`

var outingInviteInsert = []insertColumn{{"id", "::UUID"}, {"outing_id", "::UUID"}, {"source", ""},
	{"invited_person_id", "::UUID"}, {"invited_by_id", "::UUID"}, {"token_digest", ""},
	{"accepted_at", "::TIMESTAMP WITH TIME ZONE"}, {"accepted_by_id", "::UUID"},
	{"created_at", "::TIMESTAMP WITH TIME ZONE"}, {"expires_at", "::TIMESTAMP WITH TIME ZONE"},
	{"revoked_at", "::TIMESTAMP WITH TIME ZONE"}}

var invitedMembershipInsert = []insertColumn{{"id", "::UUID"}, {"context_id", "::UUID"}, {"person_id", "::UUID"},
	{"state", ""}, {"role", ""}, {"origin", ""}, {"invited_by_id", "::UUID"},
	{"joined_at", "::TIMESTAMP WITH TIME ZONE"}, {"left_at", "::TIMESTAMP WITH TIME ZONE"},
	{"created_at", "::TIMESTAMP WITH TIME ZONE"}}

// OutingInvite is OutingInviteRecord. The stored digest is not a field of the
// record -- the Python record does not carry it either -- but the SELECT reads
// it, and rotate compares against it, so it is kept unexported.
type OutingInvite struct {
	ID              string
	OutingID        string
	Source          string
	InvitedPersonID *string
	InvitedByID     string
	AcceptedAt      *time.Time
	AcceptedByID    *string
	CreatedAt       time.Time
	ExpiresAt       time.Time
	RevokedAt       *time.Time

	digest []byte
}

// OutingInviteInput is create_outing_invite's keyword arguments.
type OutingInviteInput struct {
	OutingID        string
	Source          string
	InvitedPersonID *string
	InvitedByID     string
	TokenDigest     []byte
	ExpiresAt       time.Time
	Now             time.Time
}

// CreateOutingInvite is create_outing_invite: one INSERT of every column
// (accepted_at, accepted_by_id and revoked_at written NULL, because neither
// carries a default), and nothing read back. A second named row for the same
// person is refused by uq_outing_invites_person, which reaches the caller as
// the psycopg error it is.
func (r Repository) CreateOutingInvite(ctx context.Context, in OutingInviteInput) (OutingInvite, error) {
	id, err := newUUID()
	if err != nil {
		return OutingInvite{}, err
	}
	created, expires := pythonInstant(in.Now), pythonInstant(in.ExpiresAt)
	if _, err := r.Q.Exec(ctx, renderInsert("outing_invites", outingInviteInsert, 1),
		id, in.OutingID, in.Source, in.InvitedPersonID, in.InvitedByID, in.TokenDigest, nil, nil,
		created, expires, nil); err != nil {
		return OutingInvite{}, err
	}
	return OutingInvite{ID: id, OutingID: in.OutingID, Source: in.Source, InvitedPersonID: in.InvitedPersonID,
		InvitedByID: in.InvitedByID, CreatedAt: created.UTC(), ExpiresAt: expires.UTC(),
		digest: in.TokenDigest}, nil
}

// FindOutingInviteForPerson is find_outing_invite_for_person: the invitation
// naming one person on one trip, LIMIT 1. Revoked and accepted rows count:
// the partial unique index does not exclude them, so this read has to see
// what the index sees.
func (r Repository) FindOutingInviteForPerson(ctx context.Context, outingID, personID string) (*OutingInvite, error) {
	return scanOutingInvite(r.Q.QueryRow(ctx,
		`SELECT `+outingInviteColumns+`
		   FROM outing_invites
		  WHERE outing_invites.outing_id = $1::UUID AND outing_invites.invited_person_id = $2::UUID
		  LIMIT $3::INTEGER`, outingID, personID, 1))
}

// GetOutingInvite is get_outing_invite: the row by primary key, nil when
// there is none.
func (r Repository) GetOutingInvite(ctx context.Context, inviteID string) (*OutingInvite, error) {
	return scanOutingInvite(r.Q.QueryRow(ctx,
		`SELECT `+outingInviteColumnsByID+`
		   FROM outing_invites
		  WHERE outing_invites.id = $1::UUID`, inviteID))
}

// GetOutingInviteByDigest is get_outing_invite_by_digest, the only read of a
// token in the product: the digest is unique, and LIMIT 1 says so.
func (r Repository) GetOutingInviteByDigest(ctx context.Context, tokenDigest []byte) (*OutingInvite, error) {
	return scanOutingInvite(r.Q.QueryRow(ctx,
		`SELECT `+outingInviteColumns+`
		   FROM outing_invites
		  WHERE outing_invites.token_digest = $1
		  LIMIT $2::INTEGER`, tokenDigest, 1))
}

// AcceptOutingInvite is accept_outing_invite.
//
// Statements and refusals, in Python's order:
//  1. the row FOR UPDATE (populate_existing, so always a statement); Conflict
//     OUTING_INVITE_NOT_FOUND when there is none;
//  2. an accepted row is Conflict OUTING_INVITE_ALREADY_ACCEPTED, and a
//     revoked or expired one OUTING_INVITE_NOT_REDEEMABLE, both with the row
//     still locked and nothing written;
//  3. one UPDATE of accepted_at and accepted_by_id, in table order.
func (r Repository) AcceptOutingInvite(ctx context.Context, inviteID, acceptedByID string,
	now time.Time) (OutingInvite, error) {
	invite, err := r.lockOutingInvite(ctx, inviteID)
	if err != nil {
		return OutingInvite{}, err
	}
	if invite == nil {
		return OutingInvite{}, &Conflict{Code: "OUTING_INVITE_NOT_FOUND"}
	}
	if invite.AcceptedAt != nil {
		return OutingInvite{}, &Conflict{Code: "OUTING_INVITE_ALREADY_ACCEPTED"}
	}
	accepted := pythonInstant(now)
	if invite.RevokedAt != nil || !invite.ExpiresAt.After(accepted) {
		return OutingInvite{}, &Conflict{Code: "OUTING_INVITE_NOT_REDEEMABLE"}
	}
	if err := r.execUpdate(ctx,
		`UPDATE outing_invites SET accepted_at=$1::TIMESTAMP WITH TIME ZONE, accepted_by_id=$2::UUID
		  WHERE outing_invites.id = $3::UUID`, accepted, acceptedByID, invite.ID); err != nil {
		return OutingInvite{}, err
	}
	invite.AcceptedAt, invite.AcceptedByID = &accepted, &acceptedByID
	return *invite, nil
}

// RevokeOutingInvite is revoke_outing_invite: the row FOR UPDATE, then one
// UPDATE of revoked_at. An invitation already revoked is returned as it
// stands, with no second write -- revoking twice is not an error and must not
// move the moment it was revoked at.
func (r Repository) RevokeOutingInvite(ctx context.Context, inviteID string, now time.Time) (OutingInvite, error) {
	invite, err := r.lockOutingInvite(ctx, inviteID)
	if err != nil {
		return OutingInvite{}, err
	}
	if invite == nil {
		return OutingInvite{}, &Conflict{Code: "OUTING_INVITE_NOT_FOUND"}
	}
	if invite.AcceptedAt != nil {
		return OutingInvite{}, &Conflict{Code: "OUTING_INVITE_ALREADY_ACCEPTED"}
	}
	if invite.RevokedAt != nil {
		return *invite, nil
	}
	revoked := pythonInstant(now)
	if err := r.execUpdate(ctx,
		`UPDATE outing_invites SET revoked_at=$1::TIMESTAMP WITH TIME ZONE
		  WHERE outing_invites.id = $2::UUID`, revoked, invite.ID); err != nil {
		return OutingInvite{}, err
	}
	invite.RevokedAt = &revoked
	return *invite, nil
}

// RotateOutingInviteDigest is rotate_outing_invite_digest: a new secret on a
// named row, with accepted_at deliberately untouched.
//
// Statements and refusals, in Python's order: the row FOR UPDATE (Conflict
// OUTING_INVITE_NOT_FOUND), a link row is OUTING_INVITE_NOT_NAMED, a revoked
// row OUTING_INVITE_NOT_REDEEMABLE, then one UPDATE of the columns whose
// value changed, in table order -- rotating onto the same digest and the same
// deadline writes nothing.
func (r Repository) RotateOutingInviteDigest(ctx context.Context, inviteID string, tokenDigest []byte,
	expiresAt, now time.Time) (OutingInvite, error) {
	invite, err := r.lockOutingInvite(ctx, inviteID)
	if err != nil {
		return OutingInvite{}, err
	}
	if invite == nil {
		return OutingInvite{}, &Conflict{Code: "OUTING_INVITE_NOT_FOUND"}
	}
	if invite.InvitedPersonID == nil {
		return OutingInvite{}, &Conflict{Code: "OUTING_INVITE_NOT_NAMED"}
	}
	if invite.RevokedAt != nil {
		return OutingInvite{}, &Conflict{Code: "OUTING_INVITE_NOT_REDEEMABLE"}
	}
	expires := pythonInstant(expiresAt)
	set := []string{}
	args := []any{}
	if !sameBytes(invite.digest, tokenDigest) {
		set = append(set, fmt.Sprintf("token_digest=$%d", len(args)+1))
		args = append(args, tokenDigest)
	}
	if !invite.ExpiresAt.Equal(expires) {
		set = append(set, fmt.Sprintf("expires_at=$%d::TIMESTAMP WITH TIME ZONE", len(args)+1))
		args = append(args, expires)
	}
	if len(set) > 0 {
		args = append(args, invite.ID)
		if err := r.execUpdate(ctx, `UPDATE outing_invites SET `+joinComma(set)+
			fmt.Sprintf(` WHERE outing_invites.id = $%d::UUID`, len(args)), args...); err != nil {
			return OutingInvite{}, err
		}
	}
	invite.digest, invite.ExpiresAt = tokenDigest, expires.UTC()
	return *invite, nil
}

// EnsureInvitedMembership is ensure_invited_membership: the person's open
// membership of the context, whatever state it is in, or a new INVITED one.
//
// Statements, in Python's order: the open membership (LIMIT 1), then either
// its person's `_display_names`, or the INSERT of a new row followed by that
// same name read. `origin` is written from the door the request came through
// rather than fixed, because a forwarded link and a member's named choice are
// different claims about who vouched for the newcomer.
func (r Repository) EnsureInvitedMembership(ctx context.Context, contextID, personID, invitedByID, origin string,
	now time.Time) (Membership, error) {
	m, err := scanMembership(r.Q.QueryRow(ctx,
		`SELECT `+membershipColumns+`
		   FROM memberships
		  WHERE memberships.context_id = $1::UUID AND memberships.person_id = $2::UUID
		    AND memberships.left_at IS NULL
		  LIMIT $3::INTEGER`, contextID, personID, 1))
	if err != nil {
		return Membership{}, err
	}
	if m == nil {
		id, err := newUUID()
		if err != nil {
			return Membership{}, err
		}
		created := pythonInstant(now)
		if _, err := r.Q.Exec(ctx, renderInsert("memberships", invitedMembershipInsert, 1),
			id, contextID, personID, "invited", "member", origin, invitedByID, nil, nil, created); err != nil {
			return Membership{}, err
		}
		m = &Membership{ID: id, ContextID: contextID, PersonID: personID, State: "invited", Role: "member",
			Origin: origin, InvitedByID: &invitedByID, CreatedAt: created.UTC()}
	}
	record, err := r.membershipRecord(ctx, m)
	if err != nil {
		return Membership{}, err
	}
	return *record, nil
}

func (r Repository) lockOutingInvite(ctx context.Context, inviteID string) (*OutingInvite, error) {
	rows, err := r.Q.Query(ctx,
		`SELECT `+outingInviteColumns+`
		   FROM outing_invites
		  WHERE outing_invites.id = $1::UUID FOR UPDATE`, inviteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, rows.Err()
	}
	invite, err := scanInviteRow(rows)
	if err != nil {
		return nil, err
	}
	return invite, rows.Err()
}

type scannable interface {
	Scan(dest ...any) error
}

func scanOutingInvite(row scannable) (*OutingInvite, error) {
	invite, err := scanInviteRow(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return invite, err
}

func scanInviteRow(row scannable) (*OutingInvite, error) {
	var i OutingInvite
	if err := row.Scan(&i.ID, &i.OutingID, &i.Source, &i.InvitedPersonID, &i.InvitedByID, &i.digest,
		&i.AcceptedAt, &i.AcceptedByID, &i.CreatedAt, &i.ExpiresAt, &i.RevokedAt); err != nil {
		return nil, err
	}
	i.AcceptedAt, i.RevokedAt = utcOptional(i.AcceptedAt), utcOptional(i.RevokedAt)
	i.CreatedAt, i.ExpiresAt = i.CreatedAt.UTC(), i.ExpiresAt.UTC()
	return &i, nil
}

// sameBytes is Python `==` between a stored `bytes | None` and the new value:
// a NULL digest equals only another NULL, never an empty string of bytes.
func sameBytes(a, b []byte) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

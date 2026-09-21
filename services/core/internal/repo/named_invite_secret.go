package repo

// The named invitation's secret, spent (ADR-0014, ADR-0016). `POST /sessions`
// is the one route in the product that answers without an identity, because it
// is the route where an identity is obtained; this is the write that makes the
// secret it was handed unusable a second time.

import (
	"context"
	"strconv"
	"time"
)

// ConsumeNamedInviteSecret is consume_named_invite_secret: spend the secret by
// removing it, under the row lock.
//
// The digest presented is compared again after the lock rather than trusted
// from the caller's earlier lookup: two requests arriving with the same stolen
// token would otherwise both read a live row and both mint a session. The
// second finds token_digest NULL and loses. Removing the digest, rather than
// setting a flag, is what makes the old secret dead forever -- there is no
// column left for it to match against.
//
// Statements and refusals, in Python's order: the row FOR UPDATE (Conflict
// OUTING_INVITE_NOT_FOUND when there is none), a link row is
// OUTING_INVITE_NOT_NAMED, a spent, wrong, revoked or expired secret is
// OUTING_INVITE_NOT_REDEEMABLE, then one UPDATE. `expires_at <= now` refuses,
// so an invitation expiring at exactly this instant is already gone.
func (r Repository) ConsumeNamedInviteSecret(ctx context.Context, inviteID string, tokenDigest []byte,
	acceptedByID string, now time.Time) (OutingInvite, error) {
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
	if invite.digest == nil || !sameBytes(invite.digest, tokenDigest) {
		return OutingInvite{}, &Conflict{Code: "OUTING_INVITE_NOT_REDEEMABLE"}
	}
	accepted := pythonInstant(now)
	if invite.RevokedAt != nil || !invite.ExpiresAt.After(accepted) {
		return OutingInvite{}, &Conflict{Code: "OUTING_INVITE_NOT_REDEEMABLE"}
	}

	sets := []string{"token_digest=$1"}
	args := []any{nil}
	if invite.AcceptedAt == nil {
		sets = append(sets, "accepted_at=$2::TIMESTAMP WITH TIME ZONE", "accepted_by_id=$3::UUID")
		args = append(args, accepted, acceptedByID)
	}
	args = append(args, invite.ID)
	if err := r.execUpdate(ctx, `UPDATE outing_invites SET `+joinComma(sets)+
		` WHERE outing_invites.id = $`+strconv.Itoa(len(args))+`::UUID`, args...); err != nil {
		return OutingInvite{}, err
	}
	invite.digest = nil
	if invite.AcceptedAt == nil {
		invite.AcceptedAt, invite.AcceptedByID = &accepted, &acceptedByID
	}
	return *invite, nil
}

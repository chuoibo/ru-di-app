package outingsteps

import (
	"sort"
	"strings"
	"time"
)

// requireGroupKind is _require_group_kind: refuse a roster door on a pair
// (ADR-0021 §2.5.5). Called AFTER the permission check, so a stranger still
// gets the same 403 whether the id names a group, a pair or nothing.
func requireGroupKind(s Store, contextID string) error {
	record, err := s.GetContext(contextID)
	if err != nil {
		return err
	}
	if record != nil && isPair(record.Kind) {
		return refusal(409, "not_a_group", "Đây là cuộc trò chuyện riêng, không có danh sách thành viên để đổi.")
	}
	return nil
}

// requireRegisteredPerson is _require_registered_person: refuse before the
// foreign key does, because the foreign key refuses with a 500 and no code.
func requireRegisteredPerson(s Store, personID string) error {
	person, err := s.GetPerson(personID)
	if err != nil {
		return err
	}
	if person == nil {
		return refusal(409, "person_not_registered", "Register this person with PUT /people/{person_id} first")
	}
	return nil
}

// requireParticipantsAreMembers is _require_participants_are_members: every
// name the caller sent must be one the group's active roster contains. Naming
// the strangers back is not a roster leak -- the caller sent these ids, so the
// answer only reflects their own input.
//
// Python sorts the strangers by uuid.bytes. A canonical uuid string sorts the
// same way once its dashes are gone, since every byte is two fixed hex digits.
func requireParticipantsAreMembers(s Store, contextID string, participants []string) error {
	members, err := s.ListMembers(contextID)
	if err != nil {
		return err
	}
	roster := map[string]bool{}
	for _, member := range members {
		if member.State == "active" {
			roster[member.PersonID] = true
		}
	}
	seen := map[string]bool{}
	var strangers []string
	for _, participant := range participants {
		if !roster[participant] && !seen[participant] {
			seen[participant] = true
			strangers = append(strangers, participant)
		}
	}
	if len(strangers) == 0 {
		return nil
	}
	sort.Slice(strangers, func(i, j int) bool {
		return strings.ReplaceAll(strangers[i], "-", "") < strings.ReplaceAll(strangers[j], "-", "")
	})
	return refusal(422, "participant_not_in_context", "Not members of this group: "+strings.Join(strangers, ", "))
}

// CreateOutingInvite is create_outing_invite.
//
// Both kinds carry a secret. A named invitation is what somebody exchanges for
// their first session, so it needs one; before ADR-0014 only links did.
func CreateOutingInvite(s Store, mint Secrets, outingID string, request InviteCreate, actor Actor, now time.Time) (InviteView, error) {
	outing, err := s.GetOuting(outingID)
	if err != nil {
		return InviteView{}, err
	}
	if outing == nil {
		return InviteView{}, refusal(404, "outing_not_found", "Outing does not exist")
	}
	proven, err := groupMember(s, outing.ContextID, actor.ID)
	if err != nil {
		return InviteView{}, err
	}
	if err := requirePermission("invite_to_outing", actor, proven...); err != nil {
		return InviteView{}, err
	}
	if err := requireGroupKind(s, outing.ContextID); err != nil {
		return InviteView{}, err
	}
	raw, digest, err := mint.NewInviteToken()
	if err != nil {
		return InviteView{}, err
	}
	invited := request.PersonID
	if request.Source == "link" {
		invited = nil
	} else {
		if invited == nil {
			return InviteView{}, &Invariant{Reason: "a group or friend invitation with no person survived validation"}
		}
		// The permission above proved the ACTOR belongs to this group. It says
		// nothing about the person they just named, and that id is written
		// straight from the body. Existence first, for both sources: a
		// person_id naming nobody used to surface as a 500 on the foreign key.
		if err := requireRegisteredPerson(s, *invited); err != nil {
			return InviteView{}, err
		}
		// The roster check is exactly as narrow as the claim being made.
		// "friend" deliberately names somebody outside the group -- that is
		// what inviting a friend is -- so gating it would delete the feature.
		if request.Source == "group" {
			if err := requireParticipantsAreMembers(s, outing.ContextID, []string{*invited}); err != nil {
				return InviteView{}, err
			}
		}
		// This friendly pre-check races with a concurrent insert; the partial
		// unique index is the real duplicate guarantee behind it.
		existing, err := s.FindOutingInviteForPerson(outingID, *invited)
		if err != nil {
			return InviteView{}, err
		}
		if existing != nil {
			return InviteView{}, refusal(409, "invite_already_exists", "Person is already invited to this outing")
		}
	}
	record, err := s.CreateOutingInvite(InviteDraft{
		OutingID:        outingID,
		Source:          request.Source,
		InvitedPersonID: invited,
		InvitedByID:     actor.ID,
		TokenDigest:     digest,
		ExpiresAt:       now.Add(InviteTTL),
		Now:             now,
	})
	if err != nil {
		return InviteView{}, err
	}
	// The raw token is returned exactly once and never persisted; only its
	// digest crossed Store.
	return wireInvite(record, &raw), nil
}

// AcceptOutingInvite is accept_outing_invite: redeem a bearer link into a
// request capped at INVITED.
//
// A forwardable link identifies its holder as the person requesting entry, not
// as an approver. A different person who is already ACTIVE in the group must
// approve the request before group data becomes visible. There is no
// permission check on this route at all, by design: the token is the claim.
//
// tokenDigest is token_digest(token); the hashing happens outside, and the raw
// token never reaches this layer.
func AcceptOutingInvite(s Store, tokenDigest []byte, actor Actor, now time.Time) (InviteAcceptView, error) {
	notValid := func() (InviteAcceptView, error) {
		return InviteAcceptView{}, refusal(404, "invite_not_found", "Invite link is not valid")
	}
	alreadyUsed := func() (InviteAcceptView, error) {
		return InviteAcceptView{}, refusal(409, "invite_already_accepted", "Invite link was already used")
	}
	invite, err := s.GetOutingInviteByDigest(tokenDigest)
	if err != nil {
		return InviteAcceptView{}, err
	}
	if invite == nil {
		return notValid()
	}
	if invite.InvitedPersonID != nil {
		// A named invitation is the other door's credential. Accepting it here
		// would spend the row somebody's session was going to come from, on
		// behalf of whoever happened to be holding the token. Answered as "not
		// valid" rather than "wrong door" for the reason every other refusal
		// on this route is: the token must not report what it found.
		return notValid()
	}
	if invite.AcceptedAt != nil {
		return alreadyUsed()
	}
	if invite.RevokedAt != nil || !invite.ExpiresAt.After(now) {
		return notValid()
	}
	outing, err := s.GetOuting(invite.OutingID)
	if err != nil {
		return InviteAcceptView{}, err
	}
	if outing == nil {
		// Preserve the capability boundary even if referential integrity is
		// broken: the token must not reveal whether an outing existed.
		return notValid()
	}
	if _, err := s.AcceptOutingInvite(invite.ID, actor.ID, now); err != nil {
		if conflict, ok := err.(*Conflict); ok {
			switch conflict.Code {
			case "OUTING_INVITE_ALREADY_ACCEPTED":
				return alreadyUsed()
			case "OUTING_INVITE_NOT_FOUND", "OUTING_INVITE_NOT_REDEEMABLE":
				return notValid()
			}
		}
		return InviteAcceptView{}, err
	}
	membership, err := s.EnsureInvitedMembership(MembershipDraft{
		ContextID:   outing.ContextID,
		PersonID:    actor.ID,
		InvitedByID: invite.InvitedByID,
		Origin:      "link",
		Now:         now,
	})
	if err != nil {
		return InviteAcceptView{}, err
	}
	return InviteAcceptView{
		InviteID:        invite.ID,
		OutingID:        invite.OutingID,
		ContextID:       outing.ContextID,
		MembershipID:    membership.ID,
		MembershipState: membership.State,
	}, nil
}

// inviteOf is the pair of reads every invite door on an outing starts with,
// and the one 404 that covers no outing, no invite and an invite belonging to
// another outing. Both reads always happen, in this order.
func inviteOf(s Store, outingID, inviteID string) (*Outing, *Invite, error) {
	outing, err := s.GetOuting(outingID)
	if err != nil {
		return nil, nil, err
	}
	invite, err := s.GetOutingInvite(inviteID)
	if err != nil {
		return nil, nil, err
	}
	if outing == nil || invite == nil || invite.OutingID != outingID {
		return nil, nil, refusal(404, "invite_not_found", "Invite link is not valid")
	}
	return outing, invite, nil
}

// RotateOutingInviteSecret is rotate_outing_invite_secret: put a fresh secret
// on a named invitation, for somebody signing in again.
//
// A second named row for the same person and outing cannot exist -- the partial
// unique index refuses it -- so re-inviting is not the way back in for a member
// who lost their phone. Rotating is: the row stays, the old secret is
// overwritten and can never be presented again, and the person the invitation
// names is unchanged, which is what keeps this from becoming a way to hand
// somebody else's account to a third party.
func RotateOutingInviteSecret(s Store, mint Secrets, outingID, inviteID string, actor Actor, now time.Time) (InviteView, error) {
	outing, invite, err := inviteOf(s, outingID, inviteID)
	if err != nil {
		return InviteView{}, err
	}
	proven, err := groupMember(s, outing.ContextID, actor.ID)
	if err != nil {
		return InviteView{}, err
	}
	if err := requirePermission("invite_to_outing", actor, proven...); err != nil {
		return InviteView{}, err
	}
	if invite.InvitedPersonID == nil {
		return InviteView{}, refusal(409, "invite_not_named", "Only a named invitation can be rotated")
	}
	raw, digest, err := mint.NewInviteToken()
	if err != nil {
		return InviteView{}, err
	}
	record, err := s.RotateOutingInviteDigest(invite.ID, digest, now.Add(InviteTTL), now)
	if err != nil {
		if _, ok := err.(*Conflict); ok {
			// Every conflict, whatever it was, is the same 404 here.
			return InviteView{}, refusal(404, "invite_not_found", "Invite link is not valid")
		}
		return InviteView{}, err
	}
	return wireInvite(record, &raw), nil
}

// RevokeOutingInvite is revoke_outing_invite.
func RevokeOutingInvite(s Store, outingID, inviteID string, actor Actor, now time.Time) (InviteView, error) {
	outing, invite, err := inviteOf(s, outingID, inviteID)
	if err != nil {
		return InviteView{}, err
	}
	proven, err := groupMember(s, outing.ContextID, actor.ID)
	if err != nil {
		return InviteView{}, err
	}
	if err := requirePermission("revoke_outing_invite", actor, proven...); err != nil {
		return InviteView{}, err
	}
	if invite.AcceptedAt != nil {
		return InviteView{}, refusal(409, "invite_already_accepted", "Invite link was already used")
	}
	revoked, err := s.RevokeOutingInvite(invite.ID, now)
	if err != nil {
		if conflict, ok := err.(*Conflict); ok {
			switch conflict.Code {
			case "OUTING_INVITE_ALREADY_ACCEPTED":
				return InviteView{}, refusal(409, "invite_already_accepted", "Invite link was already used")
			case "OUTING_INVITE_NOT_FOUND":
				return InviteView{}, refusal(404, "invite_not_found", "Invite link is not valid")
			}
		}
		return InviteView{}, err
	}
	return wireInvite(revoked, nil), nil
}

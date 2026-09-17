package authsteps

import "time"

// inviteNotFound is the one answer every refusal of the invitation door gives:
// a secret must not reveal whether the thing behind it exists, was already
// spent, or expired an hour ago.
func inviteNotFound() *Refusal {
	return refusal(404, "invite_not_found", "Invite link is not valid")
}

// BootstrapSessionFromInvite is bootstrap_session_from_invite (POST /sessions):
// a named invitation exchanged for a session, with no actor on the way in
// because this is where an identity is obtained.
//
// What makes it safe is that the caller says nothing about who they are. The
// session is issued to the invitation's `invited_person_id`, which an existing
// member wrote when they named it, so holding the secret is a claim about a
// person somebody already vouched for rather than a claim the holder makes
// about themselves. A link's secret names nobody and is refused here for the
// mirror of that reason.
//
// The membership it leaves behind is INVITED, exactly as the link door leaves
// it: group data stays behind the existing approval.
func BootstrapSessionFromInvite(s Store, mint Secrets, token string, now time.Time) (SessionView, error) {
	tokenDigest := mint.TokenDigest(token)
	invite, err := s.GetOutingInviteByDigest(tokenDigest)
	if err != nil {
		return SessionView{}, err
	}
	// Refusing a link here is a fail-fast rather than the only guard:
	// ConsumeNamedInviteSecret refuses the same row. It is kept because
	// reaching the repository with no person in hand is a worse way to find
	// that out.
	if invite == nil || invite.InvitedPersonID == nil {
		return SessionView{}, inviteNotFound()
	}
	if invite.RevokedAt != nil || !invite.ExpiresAt.After(now) {
		return SessionView{}, inviteNotFound()
	}
	outing, err := s.GetOuting(invite.OutingID)
	if err != nil {
		return SessionView{}, err
	}
	if outing == nil {
		return SessionView{}, inviteNotFound()
	}

	personID := *invite.InvitedPersonID
	if _, err := s.ConsumeNamedInviteSecret(invite.ID, tokenDigest, personID, now); err != nil {
		if isConflict(err) {
			return SessionView{}, inviteNotFound()
		}
		return SessionView{}, err
	}
	// Named, because a member chose this person by name. The row this writes
	// is what accept_context_membership later reads to decide whether the
	// invitee may consent for themselves.
	membership, err := s.EnsureInvitedMembership(outing.ContextID, personID, invite.InvitedByID, "named", now)
	if err != nil {
		return SessionView{}, err
	}
	raw, err := mint.NewSessionToken()
	if err != nil {
		return SessionView{}, err
	}
	sessionDigest := mint.TokenDigest(raw)
	expiresAt, err := after(now, AccountSessionTTL)
	if err != nil {
		return SessionView{}, err
	}
	record, err := s.CreateAccountSession(personID, sessionDigest, &invite.ID, expiresAt, now, "invite")
	if err != nil {
		return SessionView{}, err
	}
	return sessionResponse(
		s, raw, record, personID,
		// Already in hand: the outing was loaded above to check the expiry, so
		// naming the group costs no second query.
		&outing.ContextID, &membership.State, &membership.ID, false,
	)
}

// ListAccountSessions is list_account_sessions (GET /sessions): every session
// of the caller's that a bearer could still use.
//
// `current` is computed from the token this request arrived on rather than
// stored, so the screen can refuse to offer «đăng xuất phiên này» as if it
// were somebody else's. A nil digest is `dev` mode, where there is no bearer
// and every row answers false -- which is true: no session is being used.
func ListAccountSessions(s Store, mint Secrets, actor Actor, currentToken *string, now time.Time) (SessionListView, error) {
	if err := requirePermission("manage_own_sessions", actor, fact{"is_self", true}); err != nil {
		return SessionListView{}, err
	}
	var here []byte
	if currentToken != nil {
		here = mint.TokenDigest(*currentToken)
	}
	rows, err := s.ListAccountSessions(actor.ID, now)
	if err != nil {
		return SessionListView{}, err
	}
	currentID := ""
	if here != nil {
		record, err := s.GetAccountSessionByDigest(here)
		if err != nil {
			return SessionListView{}, err
		}
		if record != nil {
			currentID = record.ID
		}
	}
	view := SessionListView{Sessions: make([]SessionSummaryView, 0, len(rows))}
	for _, row := range rows {
		view.Sessions = append(view.Sessions, SessionSummaryView{
			ID:        row.ID,
			IssuedVia: row.IssuedVia,
			CreatedAt: row.CreatedAt,
			ExpiresAt: row.ExpiresAt,
			// Python compares the row's id with None when no token arrived, so
			// a row can never be current then. An id is a uuid and is never
			// the empty string, so this comparison says the same thing.
			Current: row.ID == currentID,
		})
	}
	return view, nil
}

// RevokeSessionToken is revoke_session_token (DELETE /sessions/current): sign
// out. It answers the same way whether or not the token was live, so a caller
// holding a token learns nothing from this route and a caller holding a guess
// learns nothing either.
func RevokeSessionToken(s Store, mint Secrets, token string, now time.Time) error {
	record, err := s.GetAccountSessionByDigest(mint.TokenDigest(token))
	if err != nil {
		return err
	}
	if record == nil {
		return nil
	}
	_, err = s.RevokeAccountSession(record.ID, now)
	return err
}

// RevokeAccountSession is revoke_account_session (DELETE /sessions/{id}): sign
// one other device out. Somebody else's session answers 404, not 403: a 403
// would confirm that the id names a real session.
func RevokeAccountSession(s Store, sessionID string, actor Actor, now time.Time) error {
	if err := requirePermission("manage_own_sessions", actor, fact{"is_self", true}); err != nil {
		return err
	}
	record, err := s.GetAccountSession(sessionID)
	if err != nil {
		return err
	}
	if record == nil || record.PersonID != actor.ID {
		return refusal(404, "session_not_found", "Phiên này không còn.")
	}
	_, err = s.RevokeAccountSession(sessionID, now)
	return err
}

package authsteps

import (
	"time"

	"mobile/services/core/internal/domain/peoplesteps"
)

// ProfileSummary is ProfileSummary: what a session holder is called. Nothing
// else is derived from it.
type ProfileSummary struct {
	DisplayName string
}

// SessionView is SessionResponse: the raw token, handed over once, and enough
// about the person for a client to know where it may go.
//
// ContextID, MembershipState and MembershipID are nil for the doors that are
// not an invitation. The invite door fills them because it knows them for
// free; the OTP and Google doors (ADR-0016) say None rather than inventing a
// group.
type SessionView struct {
	Token           string
	PersonID        string
	ExpiresAt       time.Time
	IssuedVia       string
	IsNewPerson     bool
	Profile         ProfileSummary
	Contexts        []ContextSummary
	ContextID       *string
	MembershipState *string
	MembershipID    *string
}

// OtpRequestView is OtpRequestResponse. The code went to the telephone and
// never into this body.
type OtpRequestView struct {
	ChallengeID        string
	ExpiresInSeconds   int64
	ResendAfterSeconds int64
}

// SessionSummaryView is SessionSummary: one live session of the caller's own
// (ADR-0023 §2.5). No device label, no address, no user agent: the table
// stores none of those.
type SessionSummaryView struct {
	ID        string
	IssuedVia string
	CreatedAt time.Time
	ExpiresAt time.Time
	Current   bool
}

// SessionListView is SessionListResponse.
type SessionListView struct {
	Sessions []SessionSummaryView
}

// sessionResponse is _session_response: one shape for every door.
//
// The person row is read back for the name alone, and a row that is not there
// answers with the same placeholder a brand-new account gets rather than
// refusing -- the session has already been minted by this point. `contexts` is
// the list GET /people/me/contexts returns, computed here so a client told who
// it is is told where it may go in the same answer.
func sessionResponse(
	s Store,
	rawToken string,
	record AccountSession,
	personID string,
	contextID, membershipState, membershipID *string,
	isNewPerson bool,
) (SessionView, error) {
	person, err := s.GetPerson(personID)
	if err != nil {
		return SessionView{}, err
	}
	displayName := NewPersonName
	if person != nil {
		displayName = person.DisplayName
	}
	contexts, err := peoplesteps.ContextSummaries(s, personID)
	if err != nil {
		return SessionView{}, err
	}
	return SessionView{
		Token:           rawToken,
		PersonID:        personID,
		ExpiresAt:       record.ExpiresAt,
		IssuedVia:       record.IssuedVia,
		IsNewPerson:     isNewPerson,
		Profile:         ProfileSummary{DisplayName: displayName},
		Contexts:        contexts,
		ContextID:       contextID,
		MembershipState: membershipState,
		MembershipID:    membershipID,
	}, nil
}

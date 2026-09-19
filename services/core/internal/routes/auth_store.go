package routes

import (
	"context"
	"errors"
	"time"

	"mobile/services/core/internal/domain/authsteps"
	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/repo"
)

// authStore is authsteps.Store over one request's repository. The transaction
// begins on the first query, so a door that refuses before it reads (a nil
// Google verifier, a missing phone) does not open one.
type authStore struct {
	ctx    context.Context
	call   *endpoint.Call
	people peopleStore
}

func newAuthStore(ctx context.Context, call *endpoint.Call) *authStore {
	return &authStore{ctx: ctx, call: call}
}

func (a *authStore) ready() error {
	if a.people.store.Q != nil {
		return nil
	}
	people, err := newPeopleStore(a.ctx, a.call)
	if err != nil {
		return err
	}
	a.people = people
	return nil
}

func authConflict(err error) error {
	var conflict *repo.Conflict
	if errors.As(err, &conflict) {
		return &authsteps.Conflict{Code: conflict.Code}
	}
	return err
}

func (a *authStore) GetPerson(personID string) (*authsteps.Person, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.people.GetPerson(personID)
}

func (a *authStore) ListPersonContextSummaries(personID string) ([]authsteps.SummaryRecord, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.people.ListPersonContextSummaries(personID)
}

func (a *authStore) GetFriendEdge(x, y string) (*authsteps.FriendEdge, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.people.GetFriendEdge(x, y)
}

func (a *authStore) CreatePerson(personID, displayName string) (authsteps.Person, error) {
	if err := a.ready(); err != nil {
		return authsteps.Person{}, err
	}
	record, err := a.people.store.CreatePerson(a.ctx, personID, displayName)
	if err != nil {
		return authsteps.Person{}, authConflict(err)
	}
	return *a.people.person(&record), nil
}

func (a *authStore) CreatePersonWithIdentity(personID, displayName, provider, subject string, now time.Time) (authsteps.AccountIdentity, error) {
	if err := a.ready(); err != nil {
		return authsteps.AccountIdentity{}, err
	}
	record, err := a.people.store.CreatePersonWithIdentity(a.ctx, personID, displayName, provider, subject, now)
	if err != nil {
		return authsteps.AccountIdentity{}, authConflict(err)
	}
	return identityOf(record), nil
}

func (a *authStore) GetAccountIdentity(provider, subject string) (*authsteps.AccountIdentity, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	record, err := a.people.store.GetAccountIdentity(a.ctx, provider, subject)
	if err != nil || record == nil {
		return nil, err
	}
	out := identityOf(*record)
	return &out, nil
}

func (a *authStore) UpsertAccountIdentity(personID, provider, subject string, now time.Time) (authsteps.AccountIdentity, error) {
	if err := a.ready(); err != nil {
		return authsteps.AccountIdentity{}, err
	}
	record, err := a.people.store.UpsertAccountIdentity(a.ctx, personID, provider, subject, now)
	if err != nil {
		return authsteps.AccountIdentity{}, authConflict(err)
	}
	return identityOf(record), nil
}

func (a *authStore) CreateOtpChallenge(challengeID string, phoneDigest, codeDigest []byte, expiresAt, now time.Time) (authsteps.OtpChallenge, error) {
	if err := a.ready(); err != nil {
		return authsteps.OtpChallenge{}, err
	}
	record, err := a.people.store.CreateOtpChallenge(a.ctx, repo.OtpChallengeInput{
		ChallengeID: challengeID, PhoneDigest: phoneDigest, CodeDigest: codeDigest,
		ExpiresAt: expiresAt, Now: now,
	})
	if err != nil {
		return authsteps.OtpChallenge{}, authConflict(err)
	}
	return otpOf(record), nil
}

func (a *authStore) RecentOtpChallenges(phoneDigest []byte, since time.Time) ([]authsteps.OtpChallenge, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	rows, err := a.people.store.RecentOtpChallenges(a.ctx, phoneDigest, since)
	if err != nil {
		return nil, err
	}
	out := make([]authsteps.OtpChallenge, len(rows))
	for i, row := range rows {
		out[i] = otpOf(row)
	}
	return out, nil
}

func (a *authStore) GetOtpChallenge(challengeID string) (*authsteps.OtpChallenge, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	record, err := a.people.store.GetOtpChallenge(a.ctx, challengeID)
	if err != nil || record == nil {
		return nil, err
	}
	out := otpOf(*record)
	return &out, nil
}

func (a *authStore) RecordOtpAttempt(challengeID string, attempts int64, consumed bool, now time.Time) (*authsteps.OtpChallenge, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	record, err := a.people.store.RecordOtpAttempt(a.ctx, challengeID, attempts, consumed, now)
	if err != nil {
		return nil, authConflict(err)
	}
	if record == nil {
		return nil, nil
	}
	out := otpOf(*record)
	return &out, nil
}

func (a *authStore) GetOutingInviteByDigest(tokenDigest []byte) (*authsteps.Invite, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	record, err := a.people.store.GetOutingInviteByDigest(a.ctx, tokenDigest)
	if err != nil || record == nil {
		return nil, err
	}
	return inviteOf(record), nil
}

func (a *authStore) GetOuting(outingID string) (*authsteps.Outing, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	record, err := a.people.store.GetOuting(a.ctx, outingID)
	if err != nil || record == nil {
		return nil, err
	}
	return &authsteps.Outing{ID: record.ID, ContextID: record.ContextID}, nil
}

func (a *authStore) ConsumeNamedInviteSecret(inviteID string, tokenDigest []byte, acceptedByID string, now time.Time) (authsteps.Invite, error) {
	if err := a.ready(); err != nil {
		return authsteps.Invite{}, err
	}
	record, err := a.people.store.ConsumeNamedInviteSecret(a.ctx, inviteID, tokenDigest, acceptedByID, now)
	if err != nil {
		return authsteps.Invite{}, authConflict(err)
	}
	return *inviteOf(&record), nil
}

func (a *authStore) EnsureInvitedMembership(contextID, personID, invitedByID, origin string, now time.Time) (authsteps.Membership, error) {
	if err := a.ready(); err != nil {
		return authsteps.Membership{}, err
	}
	record, err := a.people.store.EnsureInvitedMembership(a.ctx, contextID, personID, invitedByID, origin, now)
	if err != nil {
		return authsteps.Membership{}, authConflict(err)
	}
	return authsteps.Membership{ID: record.ID, State: record.State}, nil
}

func (a *authStore) CreateAccountSession(personID string, tokenDigest []byte, issuedFromInviteID *string, expiresAt, now time.Time, issuedVia string) (authsteps.AccountSession, error) {
	if err := a.ready(); err != nil {
		return authsteps.AccountSession{}, err
	}
	record, err := a.people.store.CreateAccountSession(a.ctx, repo.AccountSessionInput{
		PersonID: personID, TokenDigest: tokenDigest, IssuedFromInviteID: issuedFromInviteID,
		ExpiresAt: expiresAt, Now: now, IssuedVia: issuedVia,
	})
	if err != nil {
		return authsteps.AccountSession{}, authConflict(err)
	}
	return sessionOf(record), nil
}

func (a *authStore) GetAccountSessionByDigest(tokenDigest []byte) (*authsteps.AccountSession, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	record, err := a.people.store.GetAccountSessionByDigest(a.ctx, tokenDigest)
	if err != nil || record == nil {
		return nil, err
	}
	out := sessionOf(*record)
	return &out, nil
}

func (a *authStore) GetAccountSession(sessionID string) (*authsteps.AccountSession, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	record, err := a.people.store.GetAccountSession(a.ctx, sessionID)
	if err != nil || record == nil {
		return nil, err
	}
	out := sessionOf(*record)
	return &out, nil
}

func (a *authStore) RevokeAccountSession(sessionID string, now time.Time) (*authsteps.AccountSession, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	record, err := a.people.store.RevokeAccountSession(a.ctx, sessionID, now)
	if err != nil {
		return nil, authConflict(err)
	}
	if record == nil {
		return nil, nil
	}
	out := sessionOf(*record)
	return &out, nil
}

func (a *authStore) ListAccountSessions(personID string, now time.Time) ([]authsteps.AccountSession, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	rows, err := a.people.store.ListAccountSessions(a.ctx, personID, now)
	if err != nil {
		return nil, err
	}
	out := make([]authsteps.AccountSession, len(rows))
	for i, row := range rows {
		out[i] = sessionOf(row)
	}
	return out, nil
}

func identityOf(record repo.AccountIdentity) authsteps.AccountIdentity {
	return authsteps.AccountIdentity{
		ID: record.ID, PersonID: record.PersonID, Provider: record.Provider, Subject: record.Subject,
		CreatedAt: record.CreatedAt, LastLoginAt: record.LastLoginAt,
	}
}

func otpOf(record repo.OtpChallenge) authsteps.OtpChallenge {
	return authsteps.OtpChallenge{
		ID: record.ID, PhoneDigest: record.PhoneDigest, CodeDigest: record.CodeDigest,
		CreatedAt: record.CreatedAt, ExpiresAt: record.ExpiresAt, Attempts: record.Attempts,
		ConsumedAt: record.ConsumedAt,
	}
}

func inviteOf(record *repo.OutingInvite) *authsteps.Invite {
	if record == nil {
		return nil
	}
	return &authsteps.Invite{
		ID: record.ID, OutingID: record.OutingID, Source: record.Source,
		InvitedPersonID: record.InvitedPersonID, InvitedByID: record.InvitedByID,
		AcceptedAt: record.AcceptedAt, AcceptedByID: record.AcceptedByID,
		CreatedAt: record.CreatedAt, ExpiresAt: record.ExpiresAt, RevokedAt: record.RevokedAt,
	}
}

func sessionOf(record repo.AccountSession) authsteps.AccountSession {
	return authsteps.AccountSession{
		ID: record.ID, PersonID: record.PersonID, IssuedFromInviteID: record.IssuedFromInviteID,
		IssuedVia: record.IssuedVia, CreatedAt: record.CreatedAt, ExpiresAt: record.ExpiresAt,
		RevokedAt: record.RevokedAt,
	}
}

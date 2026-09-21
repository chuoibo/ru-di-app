package auth

import (
	"context"
	"crypto/sha256"
	"net/http"
	"time"
	"unicode/utf8"
)

// SessionRecord is the part of an account_sessions row authentication reads.
type SessionRecord struct {
	PersonID  string
	ExpiresAt time.Time
	RevokedAt *time.Time
}

// Grants is repository.actor_grants: what the roster lets a person claim.
type Grants struct {
	PersonExists bool
	Roles        []string
	Contexts     []string
}

// SessionStore is the database half of prod authentication.
type SessionStore interface {
	SessionByDigest(ctx context.Context, digest []byte) (*SessionRecord, error)
	Grants(ctx context.Context, personID string) (Grants, error)
}

var invalidSession = &Problem{401, "authentication_required", "Session is not valid"}

// TokenDigest is service.token_digest: sha256 over token.encode("utf-8").
//
// The token Python hashes is the header value Starlette decoded as latin-1,
// so every byte at or above 0x80 becomes a two-byte UTF-8 sequence before the
// hash. Hashing the raw header bytes would look identical for every real token
// (they are base64url) and differ for any other input.
func TokenDigest(token string) []byte {
	encoded := make([]byte, 0, len(token))
	for i := 0; i < len(token); i++ {
		encoded = utf8.AppendRune(encoded, rune(token[i]))
	}
	sum := sha256.Sum256(encoded)
	return sum[:]
}

// ProdActor is get_actor in prod mode followed by
// ApiService.actor_for_session_token. Every refusal after the header check is
// the same 401 with the same sentence: a token that never existed, one that
// expired, one revoked and one whose person was erased must be
// indistinguishable to whoever is testing tokens.
func ProdActor(ctx context.Context, h http.Header, store SessionStore, now time.Time) (*Actor, *Problem, error) {
	token, problem := BearerToken(h)
	if problem != nil {
		return nil, problem, nil
	}
	record, err := store.SessionByDigest(ctx, TokenDigest(token))
	if err != nil {
		return nil, nil, err
	}
	if record == nil || record.RevokedAt != nil || !record.ExpiresAt.After(now) {
		return nil, invalidSession, nil
	}
	grants, err := store.Grants(ctx, record.PersonID)
	if err != nil {
		return nil, nil, err
	}
	if !grants.PersonExists {
		return nil, invalidSession, nil
	}
	return &Actor{ID: record.PersonID, Roles: grants.Roles, Contexts: grants.Contexts}, nil, nil
}

// BaseGrantedRoles are granted to every authenticated person with a live row;
// see the actor_grants docstring for why these four plus member.
var BaseGrantedRoles = []string{"advancer", "creditor", "member", "recipient", "sender"}

// GrantsFromMemberships applies actor_grants' rules to a person's membership
// rows: contexts come from active memberships only, and any left membership
// adds former_member. The membership role column is deliberately ignored.
func GrantsFromMemberships(states map[string][]string) Grants {
	roles := map[string]bool{}
	for _, role := range BaseGrantedRoles {
		roles[role] = true
	}
	contexts := map[string]bool{}
	for context, perContext := range states {
		for _, state := range perContext {
			switch state {
			case "active":
				contexts[context] = true
			case "left":
				roles["former_member"] = true
			}
		}
	}
	return Grants{PersonExists: true, Roles: sortedKeys(roles), Contexts: sortedKeys(contexts)}
}

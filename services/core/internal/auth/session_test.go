package auth

import (
	"context"
	"crypto/sha256"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

type fakeStore struct {
	sessions map[string]*SessionRecord // hex digest -> record
	grants   map[string]Grants
	err      error
	asked    [][]byte
}

func (f *fakeStore) SessionByDigest(_ context.Context, digest []byte) (*SessionRecord, error) {
	f.asked = append(f.asked, digest)
	if f.err != nil {
		return nil, f.err
	}
	return f.sessions[string(digest)], nil
}

func (f *fakeStore) Grants(_ context.Context, personID string) (Grants, error) {
	return f.grants[personID], nil
}

const person = "abcdefab-cdef-4bcd-8fab-cdefabcdefab"

func bearer(token string) http.Header {
	return http.Header{"Authorization": {"Bearer " + token}}
}

func TestProdActorAcceptsALiveSession(t *testing.T) {
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
	store := &fakeStore{
		sessions: map[string]*SessionRecord{string(TokenDigest("tok")): {PersonID: person, ExpiresAt: now.Add(time.Second)}},
		grants:   map[string]Grants{person: {PersonExists: true, Roles: []string{"member"}, Contexts: []string{"c"}}},
	}
	actor, problem, err := ProdActor(context.Background(), bearer("tok"), store, now)
	if err != nil || problem != nil {
		t.Fatalf("problem=%+v err=%v", problem, err)
	}
	if actor.ID != person || strings.Join(actor.Roles, ",") != "member" || strings.Join(actor.Contexts, ",") != "c" {
		t.Fatalf("actor = %+v", actor)
	}
}

func TestEveryInvalidSessionIsTheSameRefusal(t *testing.T) {
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
	revoked := now.Add(-time.Hour)
	cases := map[string]*fakeStore{
		"unknown token": {sessions: map[string]*SessionRecord{}},
		"expired exactly now": {sessions: map[string]*SessionRecord{
			string(TokenDigest("tok")): {PersonID: person, ExpiresAt: now}}},
		"revoked": {sessions: map[string]*SessionRecord{
			string(TokenDigest("tok")): {PersonID: person, ExpiresAt: now.Add(time.Hour), RevokedAt: &revoked}}},
		"person erased": {
			sessions: map[string]*SessionRecord{string(TokenDigest("tok")): {PersonID: person, ExpiresAt: now.Add(time.Hour)}},
			grants:   map[string]Grants{person: {PersonExists: false}},
		},
	}
	for name, store := range cases {
		t.Run(name, func(t *testing.T) {
			actor, problem, err := ProdActor(context.Background(), bearer("tok"), store, now)
			if err != nil || actor != nil || problem == nil || *problem != *invalidSession {
				t.Fatalf("actor=%+v problem=%+v err=%v", actor, problem, err)
			}
		})
	}
}

func TestMissingBearerNeverReachesTheStore(t *testing.T) {
	store := &fakeStore{}
	_, problem, _ := ProdActor(context.Background(), http.Header{"X-Actor-Id": {person}}, store, time.Now())
	if problem == nil || problem.Detail != "Missing bearer session" {
		t.Fatalf("problem = %+v", problem)
	}
	if len(store.asked) != 0 {
		t.Fatal("store was queried without a bearer token")
	}
}

func TestStoreErrorsAreNotAuthenticationFailures(t *testing.T) {
	store := &fakeStore{err: errors.New("connection refused")}
	_, problem, err := ProdActor(context.Background(), bearer("tok"), store, time.Now())
	if err == nil || problem != nil {
		t.Fatalf("a database failure must surface as an error, got problem=%+v err=%v", problem, err)
	}
}

func TestTokenDigestReencodesLatin1AsUTF8(t *testing.T) {
	ascii := sha256.Sum256([]byte("tok"))
	if string(TokenDigest("tok")) != string(ascii[:]) {
		t.Fatal("ASCII token must hash its own bytes")
	}
	// Header byte 0xa0 is U+00A0 to Starlette, which Python encodes as C2 A0.
	python := sha256.Sum256([]byte{'t', 0xc2, 0xa0})
	if string(TokenDigest("t\xa0")) != string(python[:]) {
		t.Fatal("latin-1 byte was hashed raw instead of as UTF-8")
	}
}

func TestGrantsFromMemberships(t *testing.T) {
	g := GrantsFromMemberships(map[string][]string{
		"ctx-active":  {"active"},
		"ctx-invited": {"invited"},
		"ctx-left":    {"left"},
	})
	if strings.Join(g.Contexts, ",") != "ctx-active" {
		t.Fatalf("contexts = %v", g.Contexts)
	}
	if strings.Join(g.Roles, ",") != "advancer,creditor,former_member,member,recipient,sender" {
		t.Fatalf("roles = %v", g.Roles)
	}
	none := GrantsFromMemberships(nil)
	if strings.Join(none.Roles, ",") != "advancer,creditor,member,recipient,sender" || len(none.Contexts) != 0 {
		t.Fatalf("invitee grants = %+v", none)
	}
}

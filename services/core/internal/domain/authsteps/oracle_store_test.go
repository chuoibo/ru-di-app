package authsteps

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"mobile/services/core/internal/domain/pairpaper"
	"mobile/services/core/internal/identity"
	"mobile/services/core/internal/oracletest"
)

// This file is the Python side of scripts/render_domain_w9_goldens.py, in Go:
// the recording stub repository and the four recording seams, answering from
// the case's world and appending every call in the same order and the same
// shape the script records.

// standIn is the script's `stand_in`: `size` bytes whose hexadecimal spelling
// is letters only, so a golden can carry a digest without tripping the
// repository guard's run-of-nine-digits rule.
func standIn(seed string, size int) []byte {
	sum := sha256.Sum256([]byte(seed))
	spelled := make([]byte, 0, 2*size)
	for _, b := range sum[:2*size] {
		spelled = append(spelled, "abcdef"[int(b)%6])
	}
	raw, err := hex.DecodeString(string(spelled))
	if err != nil {
		panic(err)
	}
	return raw
}

func phoneDigestOf(canonical string) []byte { return standIn("phone|"+canonical, 4) }
func tokenDigestOf(token string) []byte     { return standIn("token|"+token, 4) }

func codeDigestOf(challengeID, code string) []byte {
	return standIn(fmt.Sprintf("code|%s|%s", challengeID, code), 4)
}

func iso(moment time.Time) any { return pairpaper.ISOFormat(moment) }

func optionalISO(moment *time.Time) any {
	if moment == nil {
		return nil
	}
	return pairpaper.ISOFormat(*moment)
}

func uuidBytes(canonical string) ([16]byte, error) {
	var raw [16]byte
	decoded, err := hex.DecodeString(strings.ReplaceAll(canonical, "-", ""))
	if err != nil || len(decoded) != 16 {
		return raw, fmt.Errorf("%q is not a uuid", canonical)
	}
	copy(raw[:], decoded)
	return raw, nil
}

// --- the world --------------------------------------------------------------

type stub struct {
	h     *harness
	calls []any

	key       string
	people    map[string][2]any // alias -> {display name, deleted at}
	summaries []SummaryRecord
	edges     map[string]*FriendEdge
	invites   map[string]*Invite // digest hex -> row
	outings   map[string]string  // outing alias -> context alias
	member    [2]string
	recent    []time.Time
	challenge *OtpChallenge
	ids       map[string]string // "provider|subject" -> person alias
	sessions  []AccountSession
	byDigest  map[string]*AccountSession
	session   *AccountSession
	conflicts map[string][]any

	sms       any
	google    []any
	debugCode *string

	queues map[string][]any

	createdChallenge *string
	createdSession   [2]any
}

func (s *stub) rec(name string, args ...any) {
	s.calls = append(s.calls, append([]any{name}, args...))
}

func (s *stub) take(key string) (any, error) {
	if len(s.queues[key]) == 0 {
		return nil, fmt.Errorf("unscripted %s", key)
	}
	value := s.queues[key][0]
	s.queues[key] = s.queues[key][1:]
	return value, nil
}

func (s *stub) maybeConflict(name string) error {
	codes := s.conflicts[name]
	if len(codes) == 0 {
		return nil
	}
	code := codes[0]
	s.conflicts[name] = codes[1:]
	if code == nil {
		return nil
	}
	text, ok := code.(string)
	if !ok {
		return fmt.Errorf("conflict code %v", code)
	}
	return &Conflict{Code: text}
}

// --- Store ------------------------------------------------------------------

func (s *stub) GetPerson(personID string) (*Person, error) {
	s.rec("get_person", s.h.name(personID))
	row, found := s.people[s.h.alias(personID)]
	if !found {
		return nil, nil
	}
	name, _ := row[0].(string)
	var deleted *time.Time
	if moment, ok := row[1].(time.Time); ok {
		deleted = &moment
	}
	return &Person{
		ID:          personID,
		DisplayName: name,
		CreatedAt:   s.h.base.Add(-30 * 24 * time.Hour),
		DeletedAt:   deleted,
	}, nil
}

func (s *stub) CreatePerson(personID, displayName string) (Person, error) {
	s.rec("create_person", s.h.name(personID), displayName)
	if err := s.maybeConflict("create_person"); err != nil {
		return Person{}, err
	}
	return Person{ID: personID, DisplayName: displayName}, nil
}

func (s *stub) ListPersonContextSummaries(personID string) ([]SummaryRecord, error) {
	s.rec("list_person_context_summaries", s.h.name(personID))
	return s.summaries, nil
}

func (s *stub) GetFriendEdge(a, b string) (*FriendEdge, error) {
	s.rec("get_friend_edge", s.h.name(a), s.h.name(b))
	return s.edges[s.h.alias(a)+"|"+s.h.alias(b)], nil
}

func (s *stub) GetAccountIdentity(provider, subject string) (*AccountIdentity, error) {
	s.rec("get_account_identity", provider, subject)
	alias, found := s.ids[provider+"|"+subject]
	if !found {
		return nil, nil
	}
	return &AccountIdentity{
		ID:       s.h.ids["AI1"],
		PersonID: s.h.ids[alias],
		Provider: provider,
		Subject:  subject,
	}, nil
}

func (s *stub) UpsertAccountIdentity(personID, provider, subject string, now time.Time) (AccountIdentity, error) {
	s.rec("upsert_account_identity", s.h.name(personID), provider, subject, iso(now))
	if err := s.maybeConflict("upsert_account_identity"); err != nil {
		return AccountIdentity{}, err
	}
	return AccountIdentity{ID: s.h.ids["AI1"], PersonID: personID, Provider: provider, Subject: subject}, nil
}

func (s *stub) CreatePersonWithIdentity(personID, displayName, provider, subject string, now time.Time) (AccountIdentity, error) {
	s.rec("create_person_with_identity", s.h.name(personID), displayName, provider, subject, iso(now))
	if err := s.maybeConflict("create_person_with_identity"); err != nil {
		return AccountIdentity{}, err
	}
	return AccountIdentity{ID: s.h.ids["AI1"], PersonID: personID, Provider: provider, Subject: subject}, nil
}

func (s *stub) CreateOtpChallenge(challengeID string, phoneDigest, codeDigest []byte, expiresAt, now time.Time) (OtpChallenge, error) {
	s.rec("create_otp_challenge", s.h.name(challengeID), hexOf(phoneDigest), hexOf(codeDigest), iso(expiresAt), iso(now))
	if err := s.maybeConflict("create_otp_challenge"); err != nil {
		return OtpChallenge{}, err
	}
	stored := challengeID
	if s.createdChallenge != nil {
		stored = s.h.ids[*s.createdChallenge]
	}
	return OtpChallenge{
		ID:          stored,
		PhoneDigest: phoneDigest,
		CodeDigest:  codeDigest,
		CreatedAt:   now,
		ExpiresAt:   expiresAt,
	}, nil
}

func (s *stub) RecentOtpChallenges(phoneDigest []byte, since time.Time) ([]OtpChallenge, error) {
	s.rec("recent_otp_challenges", hexOf(phoneDigest), iso(since))
	rows := make([]OtpChallenge, len(s.recent))
	for i, created := range s.recent {
		rows[i] = OtpChallenge{
			ID:          s.h.ids["CH1"],
			PhoneDigest: phoneDigest,
			CodeDigest:  phoneDigest,
			CreatedAt:   created,
			ExpiresAt:   created.Add(5 * time.Minute),
		}
	}
	return rows, nil
}

func (s *stub) GetOtpChallenge(challengeID string) (*OtpChallenge, error) {
	s.rec("get_otp_challenge", s.h.name(challengeID))
	return s.challenge, nil
}

func (s *stub) RecordOtpAttempt(challengeID string, attempts int64, consumed bool, now time.Time) (*OtpChallenge, error) {
	s.rec("record_otp_attempt", s.h.name(challengeID), attempts, consumed, iso(now))
	if err := s.maybeConflict("record_otp_attempt"); err != nil {
		return nil, err
	}
	return nil, nil
}

func (s *stub) GetOutingInviteByDigest(tokenDigest []byte) (*Invite, error) {
	s.rec("get_outing_invite_by_digest", hexOf(tokenDigest))
	return s.invites[hexOf(tokenDigest)], nil
}

func (s *stub) GetOuting(outingID string) (*Outing, error) {
	s.rec("get_outing", s.h.name(outingID))
	context, found := s.outings[s.h.alias(outingID)]
	if !found {
		return nil, nil
	}
	return &Outing{ID: outingID, ContextID: s.h.ids[context]}, nil
}

func (s *stub) ConsumeNamedInviteSecret(inviteID string, tokenDigest []byte, acceptedByID string, now time.Time) (Invite, error) {
	s.rec("consume_named_invite_secret", s.h.name(inviteID), hexOf(tokenDigest), s.h.name(acceptedByID), iso(now))
	if err := s.maybeConflict("consume_named_invite_secret"); err != nil {
		return Invite{}, err
	}
	return Invite{}, nil
}

func (s *stub) EnsureInvitedMembership(contextID, personID, invitedByID, origin string, now time.Time) (Membership, error) {
	s.rec("ensure_invited_membership", s.h.name(contextID), s.h.name(personID), s.h.name(invitedByID), origin, iso(now))
	if err := s.maybeConflict("ensure_invited_membership"); err != nil {
		return Membership{}, err
	}
	return Membership{ID: s.h.ids[s.member[0]], State: s.member[1]}, nil
}

func (s *stub) CreateAccountSession(personID string, tokenDigest []byte, issuedFromInviteID *string, expiresAt, now time.Time, issuedVia string) (AccountSession, error) {
	s.rec("create_account_session", s.h.name(personID), hexOf(tokenDigest),
		s.h.optionalName(issuedFromInviteID), iso(expiresAt), iso(now), issuedVia)
	if err := s.maybeConflict("create_account_session"); err != nil {
		return AccountSession{}, err
	}
	alias, _ := s.createdSession[0].(string)
	via := issuedVia
	if stored, ok := s.createdSession[1].(string); ok {
		via = stored
	}
	return AccountSession{
		ID:                 s.h.ids[alias],
		PersonID:           personID,
		IssuedFromInviteID: issuedFromInviteID,
		IssuedVia:          via,
		CreatedAt:          now,
		ExpiresAt:          expiresAt,
	}, nil
}

func (s *stub) GetAccountSessionByDigest(tokenDigest []byte) (*AccountSession, error) {
	s.rec("get_account_session_by_digest", hexOf(tokenDigest))
	return s.byDigest[hexOf(tokenDigest)], nil
}

func (s *stub) GetAccountSession(sessionID string) (*AccountSession, error) {
	s.rec("get_account_session", s.h.name(sessionID))
	return s.session, nil
}

func (s *stub) RevokeAccountSession(sessionID string, now time.Time) (*AccountSession, error) {
	s.rec("revoke_account_session", s.h.name(sessionID), iso(now))
	if err := s.maybeConflict("revoke_account_session"); err != nil {
		return nil, err
	}
	return nil, nil
}

func (s *stub) ListAccountSessions(personID string, now time.Time) ([]AccountSession, error) {
	s.rec("list_account_sessions", s.h.name(personID), iso(now))
	return s.sessions, nil
}

// --- the four seams ---------------------------------------------------------

type identitySeam struct{ s *stub }

func (i identitySeam) CanonicalMobile(raw string) (string, bool) {
	i.s.rec("identity.canonical_mobile", raw)
	return identity.CanonicalMobile(raw)
}

// ReadKey runs the real read_key over the case's environment, so the length
// rule and the 503 are the module's own. PersonIdKeyMissing becomes the
// package's own KeyMissing, which is what the route's adapter does; anything
// else (the UnicodeEncodeError a key of lone surrogates raises) passes through
// and ends the request as Python's does.
func (i identitySeam) ReadKey() ([]byte, error) {
	i.s.rec("identity.read_key")
	key, err := identity.ReadKey(i.s.key)
	var missing *identity.PersonIDKeyMissing
	if errors.As(err, &missing) {
		return nil, &KeyMissing{Message: missing.Message}
	}
	return key, err
}

func (i identitySeam) PhoneDigest(canonical string, key []byte) ([]byte, error) {
	i.s.rec("identity.phone_digest", canonical)
	if _, err := identity.DerivePhoneDigest(canonical, key); err != nil {
		return nil, err
	}
	return phoneDigestOf(canonical), nil
}

func (i identitySeam) CodeDigest(challengeID, code string, key []byte) ([]byte, error) {
	i.s.rec("identity.code_digest", i.s.h.name(challengeID), code)
	raw, err := uuidBytes(challengeID)
	if err != nil {
		return nil, err
	}
	if _, err := identity.DeriveCodeDigest(raw, code, key); err != nil {
		return nil, err
	}
	return codeDigestOf(challengeID, code), nil
}

func (i identitySeam) PersonID(canonical string, key []byte) (string, error) {
	i.s.rec("identity.person_id", canonical)
	if _, err := identity.DerivePersonID(canonical, key); err != nil {
		return "", err
	}
	alias, err := i.s.take("derived")
	if err != nil {
		return "", err
	}
	return i.s.h.id(alias)
}

type secretsSeam struct{ s *stub }

func (m secretsSeam) NewUUID() (string, error) {
	m.s.rec("secrets.uuid4")
	alias, err := m.s.take("uuids")
	if err != nil {
		return "", err
	}
	return m.s.h.id(alias)
}

func (m secretsSeam) NewSessionToken() (string, error) {
	m.s.rec("secrets.token_urlsafe", int64(32))
	name, err := m.s.take("tokens")
	if err != nil {
		return "", err
	}
	text, ok := name.(string)
	if !ok {
		return "", fmt.Errorf("token name %v", name)
	}
	raw, found := m.s.h.tokens[text]
	if !found {
		return "", fmt.Errorf("no token %q", text)
	}
	return raw, nil
}

func (m secretsSeam) TokenDigest(raw string) []byte {
	m.s.rec("secrets.token_digest", raw)
	return tokenDigestOf(raw)
}

func (m secretsSeam) RandomBelow(bound int64) (int64, error) {
	m.s.rec("secrets.randbelow", bound)
	drawn, err := m.s.take("draws")
	if err != nil {
		return 0, err
	}
	return oracletest.Int64(drawn)
}

type smsSeam struct{ s *stub }

func (g smsSeam) SendOTP(canonicalPhone, code, challengeID string) error {
	g.s.rec("sms.send_otp", canonicalPhone, code, g.s.h.name(challengeID))
	if g.s.sms == nil {
		return nil
	}
	text, ok := g.s.sms.(string)
	if !ok {
		return fmt.Errorf("sms failure %v", g.s.sms)
	}
	return &DeliveryError{Message: text}
}

type googleSeam struct{ s *stub }

func (g googleSeam) Verify(idToken string) (GoogleClaims, error) {
	g.s.rec("google.verify", idToken)
	answer := g.s.google
	kind, _ := answer[0].(string)
	if kind == "bad" {
		message, _ := answer[1].(string)
		return GoogleClaims{}, &GoogleTokenInvalid{Message: message}
	}
	subject, _ := answer[1].(string)
	var name *string
	if text, ok := answer[2].(string); ok {
		name = &text
	}
	return GoogleClaims{Subject: subject, DisplayName: name}, nil
}

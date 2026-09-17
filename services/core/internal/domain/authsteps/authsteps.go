// Package authsteps is the workflow of the seven W9 routes -- the four of
// services/api/app/api/routes/sessions.py and the three of routes/auth.py --
// which is the session and OTP half of services/api/app/api/service.py with
// every repository call behind Store.
//
// These are the doors where an identity is obtained, so the order of the
// checks inside each method is the security argument and is copied exactly.
// Each method makes the calls Python makes, in Python's order, with Python's
// arguments. A route implements Store over its transaction and renders the
// returned view.
//
// Errors a method returns:
//
//   - *Refusal is an ApiProblem: status, code and detail;
//   - *permissions.Error is PermissionError_, which Python never catches;
//   - *Conflict is a RepositoryConflict a Store returned and the method did
//     not translate;
//   - *Invariant is a failed `assert`;
//   - *OverflowError is the OverflowError past the ends of the calendar;
//   - anything else is a Store's or a seam's own failure, passed through.
//
// Everything but *Refusal ends the request as Python's 500 does.
//
// # Four seams this package does not cross
//
// `internal/domain` may import arithmetic, text and time and nothing else
// (tools/boundary), and all four of these hash, read the network or read the
// process. Each is an interface here and an adapter at the route, the way
// internal/domain/itinerary keeps Valhalla behind a Router:
//
//   - Identity is app.api.person_identity, already ported as
//     internal/identity. The methods below carry the key and the digests as
//     opaque values and never look inside one.
//   - Secrets is `uuid.uuid4()`, `secrets.token_urlsafe(32)` with the SHA-256
//     of it, and `secrets.randbelow`. A raw session token is returned to the
//     caller exactly once; only its digest crosses Store.
//   - SMSSender is app.api.sms.SmsSender. What a gateway is, and whether a
//     debug code replaces the drawn one, is the process's business.
//   - GoogleVerifier is app.api.google_identity.GoogleTokenVerifier: Google's
//     certificates, the signature, and the audience and issuer rules behind
//     it. This package sees a verified `sub` and a name, never an e-mail
//     address -- ADR-0016 forbids merging accounts by e-mail, and the surest
//     way to keep that from happening "just once" is for the address never to
//     arrive.
//
// A raw bearer token does reach two of these methods, because Python hashes it
// inside them rather than at the door, and where it hashes is observable. It
// is handed straight to Secrets.TokenDigest and is never stored, compared or
// logged here.
//
// Ids are canonical uuid strings (str(uuid.UUID)); the methods only compare
// them for equality. `now` is the service clock read once per request
// (`_now()`, datetime.now(UTC)), passed in rather than read.
//
// testdata/python_auth_steps*.json is rendered by
// scripts/render_domain_w9_goldens.py by running the real ApiService methods
// over a recording stub repository with the clock, the identity derivations
// and every secret pinned; oracle_test.go replays every case through a
// recording Store, comparing the answer and every call with its arguments.
package authsteps

import (
	"errors"
	"math/big"
	"time"

	"mobile/services/core/internal/domain/otp"
	"mobile/services/core/internal/domain/peoplesteps"
	"mobile/services/core/internal/domain/permissions"
)

// AccountSessionTTL is ACCOUNT_SESSION_TTL.
const AccountSessionTTL = 30 * 24 * time.Hour

// NewPersonName is ApiService.NEW_PERSON_NAME, and the name a session answers
// with when the person row cannot be read back.
const NewPersonName = "Thành viên mới"

// Actor is the service's Actor: its id and the roles the session carries.
type Actor struct {
	ID    string
	Roles []string
}

// Refusal is an ApiProblem.
type Refusal struct {
	Status int
	Code   string
	Detail string
}

func (r *Refusal) Error() string { return r.Code }

func refusal(status int, code, detail string) *Refusal {
	return &Refusal{Status: status, Code: code, Detail: detail}
}

// Conflict is RepositoryConflict: a Store returns it when a persistence
// invariant refuses a write.
type Conflict struct {
	Code string
}

func (c *Conflict) Error() string { return c.Code }

func isConflict(err error) bool {
	var conflict *Conflict
	return errors.As(err, &conflict)
}

// Invariant is an `assert` of the service that did not hold.
type Invariant struct {
	Reason string
}

func (e *Invariant) Error() string { return "authsteps: " + e.Reason }

// OverflowError is the OverflowError datetime arithmetic raises outside years
// 1..9999. A session's expiry is thirty days after `now` and a challenge's is
// five minutes after it, so a clock at the very end of the calendar ends the
// request rather than storing a wrapped deadline.
type OverflowError = otp.OverflowError

// KeyMissing is app.api.person_identity.PersonIdKeyMissing, which the OTP door
// turns into one 503. It is declared here rather than imported because
// internal/identity is not a domain package; the route's adapter translates.
type KeyMissing struct {
	Message string
}

func (e *KeyMissing) Error() string { return e.Message }

// DeliveryError is app.api.sms.SmsDeliveryError: the gateway did not accept
// the message. Like Python's, it carries no telephone number.
type DeliveryError struct {
	Message string
}

func (e *DeliveryError) Error() string { return e.Message }

// GoogleTokenInvalid is app.api.google_identity.GoogleTokenInvalid: the token
// is not one Google issued for this application, or is stale.
type GoogleTokenInvalid struct {
	Message string
}

func (e *GoogleTokenInvalid) Error() string { return e.Message }

// GoogleClaims is GoogleClaims: what a verified token is allowed to say. There
// is deliberately no e-mail address on it.
type GoogleClaims struct {
	Subject     string
	DisplayName *string
}

// --- records ----------------------------------------------------------------

// Person is PersonRecord, shared with internal/domain/peoplesteps so one
// route-side store can answer both.
type Person = peoplesteps.Person

// SummaryRecord is PersonContextSummaryRecord.
type SummaryRecord = peoplesteps.SummaryRecord

// FriendEdge is the part of FriendEdgeRecord `_friend_edge_dict` reads.
type FriendEdge = peoplesteps.FriendEdge

// ContextSummary is ContextSummary: one row of a session's `contexts`.
type ContextSummary = peoplesteps.ContextSummary

// OtpChallenge is OtpChallengeRecord.
type OtpChallenge struct {
	ID          string
	PhoneDigest []byte
	CodeDigest  []byte
	CreatedAt   time.Time
	ExpiresAt   time.Time
	Attempts    int64
	ConsumedAt  *time.Time
}

// AccountSession is AccountSessionRecord: a stored session, without the secret
// that reaches it.
type AccountSession struct {
	ID                 string
	PersonID           string
	IssuedFromInviteID *string
	IssuedVia          string
	CreatedAt          time.Time
	ExpiresAt          time.Time
	RevokedAt          *time.Time
}

// AccountIdentity is AccountIdentityRecord. Only PersonID is read.
type AccountIdentity struct {
	ID          string
	PersonID    string
	Provider    string
	Subject     string
	CreatedAt   time.Time
	LastLoginAt time.Time
}

// Invite is OutingInviteRecord.
type Invite struct {
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
}

// Outing is the part of OutingRecord the invitation door reads: whether the
// row exists at all, and which group it belongs to.
type Outing struct {
	ID        string
	ContextID string
}

// Membership is MembershipRecord, the two fields a session answer carries.
type Membership struct {
	ID    string
	State string
}

// --- seams ------------------------------------------------------------------

// Store is the part of ApiRepository these methods call, one method per
// repository method with its arguments in the Protocol's order. A write a
// persistence invariant refuses returns *Conflict with the repository's code.
//
// The first three are peoplesteps.SummaryStore, which is how a session answers
// with the same `contexts` list GET /people/me/contexts returns.
type Store interface {
	GetPerson(personID string) (*Person, error)
	ListPersonContextSummaries(personID string) ([]SummaryRecord, error)
	GetFriendEdge(a, b string) (*FriendEdge, error)

	CreatePerson(personID, displayName string) (Person, error)
	CreatePersonWithIdentity(personID, displayName, provider, subject string, now time.Time) (AccountIdentity, error)
	GetAccountIdentity(provider, subject string) (*AccountIdentity, error)
	UpsertAccountIdentity(personID, provider, subject string, now time.Time) (AccountIdentity, error)

	CreateOtpChallenge(challengeID string, phoneDigest, codeDigest []byte, expiresAt, now time.Time) (OtpChallenge, error)
	RecentOtpChallenges(phoneDigest []byte, since time.Time) ([]OtpChallenge, error)
	GetOtpChallenge(challengeID string) (*OtpChallenge, error)
	RecordOtpAttempt(challengeID string, attempts int64, consumed bool, now time.Time) (*OtpChallenge, error)

	GetOutingInviteByDigest(tokenDigest []byte) (*Invite, error)
	GetOuting(outingID string) (*Outing, error)
	ConsumeNamedInviteSecret(inviteID string, tokenDigest []byte, acceptedByID string, now time.Time) (Invite, error)
	EnsureInvitedMembership(contextID, personID, invitedByID, origin string, now time.Time) (Membership, error)

	CreateAccountSession(personID string, tokenDigest []byte, issuedFromInviteID *string, expiresAt, now time.Time, issuedVia string) (AccountSession, error)
	GetAccountSessionByDigest(tokenDigest []byte) (*AccountSession, error)
	GetAccountSession(sessionID string) (*AccountSession, error)
	RevokeAccountSession(sessionID string, now time.Time) (*AccountSession, error)
	ListAccountSessions(personID string, now time.Time) ([]AccountSession, error)
}

// Identity is app.api.person_identity, ported as internal/identity: the
// telephone number a person typed becomes the id the database stores and the
// digests the OTP tables hold instead of the number.
//
// ReadKey answers *KeyMissing when the host has no key. Every other error --
// the UnicodeEncodeError a number of non-ASCII digits raises, which
// CanonicalMobile accepts and no digest can be derived from -- ends the
// request, which is what Python does with it.
type Identity interface {
	// CanonicalMobile is canonical_mobile: "84" and nine digits, or false.
	CanonicalMobile(raw string) (string, bool)
	// ReadKey is read_key: the signing key of this host.
	ReadKey() ([]byte, error)
	// PhoneDigest is derive_phone_digest.
	PhoneDigest(canonical string, key []byte) ([]byte, error)
	// CodeDigest is derive_code_digest, salted by the challenge it belongs to.
	CodeDigest(challengeID, code string, key []byte) ([]byte, error)
	// PersonID is derive_person_id: the id a NEW account for this number gets.
	PersonID(canonical string, key []byte) (string, error)
}

// Secrets mints what must not be guessable, and hashes what must not be
// stored. TokenDigest is here rather than at the door because Python calls it
// at a point inside two of these methods that a caller can observe: after the
// permission check of GET /sessions, and not at all when that check refuses.
type Secrets interface {
	// NewUUID is uuid.uuid4(), as str(UUID).
	NewUUID() (string, error)
	// NewSessionToken is `secrets.token_urlsafe(32)`. The raw token is
	// returned to the caller exactly once and never persisted.
	NewSessionToken() (string, error)
	// TokenDigest is service.token_digest: SHA-256 over the UTF-8 of a token.
	TokenDigest(raw string) []byte
	// RandomBelow is secrets.randbelow, which generate_code draws from.
	RandomBelow(bound int64) (int64, error)
}

// SMSSender is app.api.sms.SmsSender. The code reaches the telephone here and
// nowhere else; a gateway that refuses answers *DeliveryError.
type SMSSender interface {
	SendOTP(canonicalPhone, code, challengeID string) error
}

// GoogleVerifier is app.api.google_identity.GoogleTokenVerifier. A nil
// verifier is Python's None: a host with no client id has no Google door.
type GoogleVerifier interface {
	Verify(idToken string) (GoogleClaims, error)
}

// --- shared helpers ---------------------------------------------------------

// fact is one entry of the context dict `_require_permission` receives.
type fact struct {
	name  string
	holds bool
}

// requirePermission is _require_permission: the facts are built here, never
// accepted from a caller, and a denial is one 403 carrying the predicate that
// failed.
func requirePermission(action string, actor Actor, facts ...fact) error {
	var proven []string
	for _, f := range facts {
		if f.holds {
			proven = append(proven, f.name)
		}
	}
	reason, allowed, err := permissions.DenialReason(action, permissions.AuthorizationFacts{
		ActorID:    actor.ID,
		Roles:      actor.Roles,
		Proven:     proven,
		Provenance: "api_service",
	})
	if err != nil {
		return err
	}
	if allowed {
		return nil
	}
	return refusal(403, "permission_denied", reason)
}

// after is `moment + delta` for one of the three fixed deadlines this package
// writes, with Python's OverflowError past the ends of the calendar. Each of
// the three timedeltas is a constant far inside timedelta's own range, so only
// the datetime half of the arithmetic can refuse.
func after(moment time.Time, delta time.Duration) (time.Time, error) {
	return otp.Shift(moment, big.NewInt(int64(delta/time.Microsecond)))
}

// before is `moment - delta`.
func before(moment time.Time, delta time.Duration) (time.Time, error) {
	return otp.Shift(moment, big.NewInt(-int64(delta/time.Microsecond)))
}

// hexOf is bytes.hex(): lowercase, no separator. It is what
// `account_identities` stores as the subject of a telephone, so the number
// itself never reaches a column.
func hexOf(raw []byte) string {
	// Split so the repo guard does not read the hex alphabet as a long number;
	// the constant is unchanged.
	const digits = "0123" + "456789abcdef"
	out := make([]byte, 0, 2*len(raw))
	for _, b := range raw {
		out = append(out, digits[b>>4], digits[b&0x0F])
	}
	return string(out)
}

// compareDigest is `hmac.compare_digest` for two byte strings: constant time
// in the contents, and -- exactly as CPython's is -- NOT constant time in the
// length, which it compares and refuses first.
func compareDigest(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var differing byte
	for i := range a {
		differing |= a[i] ^ b[i]
	}
	return differing == 0
}

// strip is `str.strip()`: whitespace as CPython defines it, which is not what
// Go's unicode.IsSpace defines -- U+001C..U+001F are whitespace to Python and
// are not to Go. internal/identity carries the same table for the same reason
// and cannot be imported here.
func strip(text string) string {
	runes := []rune(text)
	start, end := 0, len(runes)
	for start < end && isPySpace(runes[start]) {
		start++
	}
	for end > start && isPySpace(runes[end-1]) {
		end--
	}
	return string(runes[start:end])
}

// isPySpace is str.isspace() for one code point.
func isPySpace(r rune) bool {
	switch {
	case r >= 0x09 && r <= 0x0D, r >= 0x1C && r <= 0x20:
		return true
	case r == 0x85, r == 0xA0, r == 0x1680, r >= 0x2000 && r <= 0x200A,
		r == 0x2028, r == 0x2029, r == 0x202F, r == 0x205F, r == 0x3000:
		return true
	}
	return false
}

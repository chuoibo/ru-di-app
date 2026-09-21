//go:build postgres

package repo

// The world of the W9 oracle (auth_repo_oracle_postgres_test.go): one group,
// one pair conversation, the people a session can belong to, the sessions
// themselves, the invitations `POST /sessions` redeems, and the OTP challenges
// the code door reads. Written as literal SQL through seed_postgres_test.go's
// world, so the same statements seed both sides.
//
//	an       a member of the group, and the holder of most of the sessions:
//	         two live ones, two tied on created_at, one revoked, one whose
//	         deadline is exactly the case's clock, and one minted by an invite
//	binh     a member of the group and of a pair with chi, so the answer a
//	         session door builds reaches the pair branch of _context_summaries
//	chi      invited to the group and named by two invitations
//	roi      a membership that ended: the one row that grants former_member
//	xoa      an erased account that still holds a live session
//	moi      nobody's member, nobody's friend: what a new number looks like
//
// Nothing secret is written down.
//
//   - An invitation's and a session's token_digest is `sha256(convert_to(word))`
//     computed inside PostgreSQL from a short sample word, and the cases hand
//     both sides that same word (storedDigest, from the W7 world).
//   - A telephone number appears in no file. authPhone builds one at run time
//     out of a prefix and a small counter, and what the fixture stores is the
//     HMAC of it under the run's own key -- a key that is itself generated at
//     run time and handed to the Python container as an environment variable.
//   - A code is six digits built the same way, and the row keeps only the HMAC
//     of it salted by the challenge's own id.

import (
	"encoding/hex"
	"fmt"

	"mobile/services/core/internal/identity"
)

const (
	kindAccountSession = 0x97
	kindOtpChallenge   = 0x98
	kindAuthIdentity   = 0x99
)

// authNow is the instant every W9 case reads its clock at.
const authNow = "2030-10-01T05:00:00.123456Z"

// authCreated is when the fixture rows were written.
const authCreated = "2030-09-01T00:00:00.123456Z"

// storedBytes writes a digest computed in Go (an HMAC, which PostgreSQL has no
// built-in for) as a bytea literal. Used only for values derived from the run's
// own key, so nothing constant reaches the statement.
func storedBytes(value []byte) raw {
	return raw("'\\x" + hex.EncodeToString(value) + "'::bytea")
}

// authPhone is a telephone number built at run time: a Vietnamese mobile
// prefix and a counter, never a literal in this file.
func authPhone(counter int) string {
	return "0" + "9" + fmt.Sprintf("%08d", counter)
}

// authCode is a six-digit code built the same way.
func authCode(counter int) string { return fmt.Sprintf("%06d", counter) }

type authWorld struct {
	world
	key []byte

	an, binh, chi, roi, xoa, moi, missingPerson string
	group, pairCtx, khac, missingContext        string
	outing, otherOuting                         string

	invChi, invMoi, invLink, invRevoked  string
	invEdge, invSpent, invAcceptedLive   string
	missingInvite                        string
	tokChi, tokMoi, tokLink, tokRevoked  string
	tokEdge, tokSpent, tokAccepted       string
	tokNobodyIssued                      string
	sesAn, sesAnOld, sesTieA, sesTieB    string
	sesRevoked, sesEdge, sesBinh, sesXoa string
	sesInvite, missingSession            string
	bearerAn, bearerAnOld, bearerTieA    string
	bearerTieB, bearerRevoked            string
	bearerEdge, bearerBinh, bearerXoa    string
	bearerInvite, bearerNobodyIssued     string

	// The OTP side. phoneA belongs to `an` through an account_identities row;
	// phoneB belongs to nobody yet; phoneC belonged to an account that was
	// erased, which is the number that must come back as a NEW person.
	phoneA, phoneB, phoneC, phoneD     string
	phoneE                             string
	digestA, digestB, digestC, digestD []byte
	digestE                            []byte
	// The ids derive_person_id makes of phoneB, phoneC and phoneD. Only C and D
	// are seeded: phoneB's is what a brand-new account gets, phoneC's account
	// was erased so that number must come back as a NEW person under a fresh
	// uuid4, and phoneD's is alive and unbound, so it is that person signing in.
	personB, personC, personD          string
	chLive, chSecond, chEdge, chBurned string
	chConsumed, chExpired, chJustNow   string
	chLiveB, chLiveC, chLiveD          string
	missingCh                          string
	codeLive, codeBurned, codeWrong    string
	idPhoneAn, idGoogleBinh            string
	googleBinh                         string
}

func (w *authWorld) member(n int, contextID, personID, state string, extra ...any) {
	pairs := []any{"id", fid(kindMembership, n), "context_id", contextID, "person_id", personID,
		"state", state, "created_at", authCreated}
	switch state {
	case "active":
		pairs = append(pairs, "joined_at", "2030-09-02T00:00:00Z")
	case "left":
		pairs = append(pairs, "joined_at", "2030-09-02T00:00:00Z", "left_at", "2030-09-03T00:00:00Z")
	}
	w.insert("memberships", append(pairs, extra...)...)
}

func (w *authWorld) invite(n int, outingID, source string, person any, token, expires string,
	extra ...any) string {
	id := fid(kindOutingInvite, n)
	pairs := []any{"id", id, "outing_id", outingID, "source", source, "invited_person_id", person,
		"invited_by_id", w.an, "token_digest", storedDigest(token), "created_at", authCreated,
		"expires_at", expires}
	w.insert("outing_invites", append(pairs, extra...)...)
	return id
}

// session seeds one account_sessions row. The token is the sample word the
// cases present; the row keeps only what sha256 makes of it.
func (w *authWorld) session(n int, personID, token, issuedVia, createdAt, expiresAt string,
	extra ...any) string {
	id := fid(kindAccountSession, n)
	pairs := []any{"id", id, "person_id", personID, "token_digest", storedDigest(token),
		"issued_via", issuedVia, "created_at", createdAt, "expires_at", expiresAt}
	w.insert("account_sessions", append(pairs, extra...)...)
	return id
}

func (w *authWorld) challenge(n int, phoneDigest []byte, code, createdAt, expiresAt string,
	extra ...any) string {
	id := fid(kindOtpChallenge, n)
	codeDigest, err := identity.DeriveCodeDigest(uuidArray16(id), code, w.key)
	if err != nil {
		panic(err)
	}
	pairs := []any{"id", id, "phone_digest", storedBytes(phoneDigest),
		"code_digest", storedBytes(codeDigest), "created_at", createdAt, "expires_at", expiresAt,
		"attempts", 0}
	w.insert("otp_challenges", append(pairs, extra...)...)
	return id
}

func (w *authWorld) loginProof(n int, personID, provider, subject, lastLogin string) string {
	id := fid(kindAuthIdentity, n)
	w.insert("account_identities", "id", id, "person_id", personID, "provider", provider,
		"subject", subject, "created_at", authCreated, "last_login_at", lastLogin)
	return id
}

// uuidArray16 is a canonical UUID string as the sixteen bytes Python's
// `UUID.bytes` gives derive_code_digest.
func uuidArray16(id string) [16]byte {
	decoded, err := hex.DecodeString(id[0:8] + id[9:13] + id[14:18] + id[19:23] + id[24:36])
	if err != nil {
		panic(err)
	}
	var out [16]byte
	copy(out[:], decoded)
	return out
}

// derivedPerson is derive_person_id: what id a NEW account for this number
// would be given.
func (w *authWorld) derivedPerson(phone string) string {
	canonical, ok := identity.CanonicalMobile(phone)
	if !ok {
		panic("authPhone built a number canonical_mobile refuses: " + phone)
	}
	id, err := identity.DerivePersonID(canonical, w.key)
	if err != nil {
		panic(err)
	}
	return id
}

func (w *authWorld) phoneDigest(phone string) []byte {
	canonical, ok := identity.CanonicalMobile(phone)
	if !ok {
		panic("authPhone built a number canonical_mobile refuses: " + phone)
	}
	digest, err := identity.DerivePhoneDigest(canonical, w.key)
	if err != nil {
		panic(err)
	}
	return digest
}

func newAuthWorld(key []byte) *authWorld {
	w := &authWorld{key: key}
	w.an = w.person(0x90, "An (dữ liệu mẫu)")
	w.binh = w.person(0x91, "Bình (dữ liệu mẫu)")
	w.chi = w.person(0x92, "Chi (dữ liệu mẫu)")
	w.roi = w.person(0x93, "Người đã rời (dữ liệu mẫu)")
	w.xoa = w.person(0x94, "Tài khoản đã xoá (dữ liệu mẫu)", "deleted_at", "2030-09-20T00:00:00Z")
	w.moi = w.person(0x95, "Người mới (dữ liệu mẫu)")
	w.missingPerson = fid(kindPerson, 0x9f)

	w.group = fid(kindContext, 0x90)
	w.insert("contexts", "id", w.group, "display_name", "Nhóm đi chơi (dữ liệu mẫu)",
		"created_by_id", w.an, "created_at", authCreated)
	w.pairCtx = fid(kindContext, 0x91)
	w.insert("contexts", "id", w.pairCtx, "display_name", "", "created_by_id", w.binh,
		"created_at", authCreated, "kind", "pair", "pair_key", w.binh+":"+w.chi)
	w.khac = fid(kindContext, 0x92)
	w.insert("contexts", "id", w.khac, "display_name", "Nhóm khác (dữ liệu mẫu)",
		"created_by_id", w.binh, "created_at", authCreated)
	w.missingContext = fid(kindContext, 0x9f)

	w.member(0x90, w.group, w.an, "active", "role", "admin")
	w.member(0x91, w.group, w.binh, "active")
	w.member(0x92, w.group, w.chi, "invited", "invited_by_id", w.an, "origin", "named")
	w.member(0x93, w.group, w.roi, "left")
	w.member(0x94, w.group, w.xoa, "active")
	w.member(0x95, w.pairCtx, w.binh, "active")
	w.member(0x96, w.pairCtx, w.chi, "active")
	w.member(0x97, w.khac, w.binh, "active")
	// `an` left this one: the single row that turns former_member on.
	w.member(0x98, w.khac, w.an, "left")

	w.outing = fid(kindOuting, 0x90)
	w.insert("outings", "id", w.outing, "context_id", w.group, "created_by_id", w.an,
		"title", "Chuyến đi (dữ liệu mẫu)", "starts_on", "2030-10-05", "ends_on", "2030-10-07",
		"headcount", 4, "budget_per_person_vnd", int64(500000), "created_at", authCreated)
	w.otherOuting = fid(kindOuting, 0x91)
	w.insert("outings", "id", w.otherOuting, "context_id", w.khac, "created_by_id", w.binh,
		"title", "Chuyến khác (dữ liệu mẫu)", "starts_on", "2030-10-05", "ends_on", "2030-10-06",
		"headcount", 2, "budget_per_person_vnd", int64(300000), "created_at", authCreated)

	w.tokChi, w.tokMoi, w.tokLink = "moi-chi-mau", "moi-nguoi-moi-mau", "moi-lien-ket-mau"
	w.tokRevoked, w.tokEdge = "moi-da-thu-hoi-mau", "moi-dung-han-mau"
	w.tokSpent, w.tokNobodyIssued = "moi-da-dung-mau", "khong-ai-phat-mau"
	w.invChi = w.invite(0x90, w.outing, "group", w.chi, w.tokChi, "2030-12-01T00:00:00Z")
	w.invMoi = w.invite(0x91, w.otherOuting, "friend", w.moi, w.tokMoi, "2030-12-01T00:00:00Z")
	w.invLink = w.invite(0x92, w.outing, "link", nil, w.tokLink, "2030-12-01T00:00:00Z")
	w.invRevoked = w.invite(0x93, w.outing, "group", w.roi, w.tokRevoked, "2030-12-01T00:00:00Z",
		"revoked_at", "2030-09-26T00:00:00Z")
	// The deadline is the case's clock to the microsecond: `expires_at <= now`
	// refuses, so this invitation is already gone.
	w.invEdge = w.invite(0x94, w.outing, "group", w.binh, w.tokEdge, authNow)
	// A named row whose secret was already spent: token_digest NULL, accepted.
	w.invSpent = w.invite(0x95, w.outing, "group", w.xoa, w.tokSpent, "2030-12-01T00:00:00Z",
		"token_digest", nil, "accepted_at", "2030-09-27T00:00:00Z", "accepted_by_id", w.xoa)
	// A named row somebody already accepted whose secret is still live: the
	// consumption that removes the digest must leave accepted_at where it is.
	w.tokAccepted = "moi-da-nhan-mau"
	w.invAcceptedLive = w.invite(0x96, w.otherOuting, "group", w.roi, w.tokAccepted,
		"2030-12-01T00:00:00Z", "accepted_at", "2030-09-28T00:00:00Z", "accepted_by_id", w.roi)
	w.missingInvite = fid(kindOutingInvite, 0x9f)

	w.bearerAn, w.bearerAnOld = "phien-an-mau", "phien-an-cu-mau"
	w.bearerTieA, w.bearerTieB = "phien-hoa-a-mau", "phien-hoa-b-mau"
	w.bearerRevoked, w.bearerEdge = "phien-da-thu-hoi-mau", "phien-dung-han-mau"
	w.bearerBinh, w.bearerXoa = "phien-binh-mau", "phien-da-xoa-mau"
	w.bearerInvite, w.bearerNobodyIssued = "phien-tu-loi-moi-mau", "khong-ai-cap-mau"
	w.sesAn = w.session(0x90, w.an, w.bearerAn, "otp", "2030-09-10T00:00:00Z", "2030-12-01T00:00:00Z")
	w.sesAnOld = w.session(0x91, w.an, w.bearerAnOld, "google", "2030-09-05T00:00:00Z",
		"2030-12-01T00:00:00Z")
	// Two rows tied on created_at, so the order falls to the id. The ids are
	// seeded out of order on purpose: 0x93 before 0x92.
	w.sesTieB = w.session(0x93, w.an, w.bearerTieB, "otp", "2030-09-12T00:00:00.5Z",
		"2030-12-01T00:00:00Z")
	w.sesTieA = w.session(0x92, w.an, w.bearerTieA, "otp", "2030-09-12T00:00:00.5Z",
		"2030-12-01T00:00:00Z")
	w.sesRevoked = w.session(0x94, w.an, w.bearerRevoked, "otp", "2030-09-11T00:00:00Z",
		"2030-12-01T00:00:00Z", "revoked_at", "2030-09-15T00:00:00Z")
	// The deadline is the clock to the microsecond: `expires_at > now` drops it
	// from the list and `expires_at <= now` refuses the bearer.
	w.sesEdge = w.session(0x95, w.an, w.bearerEdge, "otp", "2030-09-13T00:00:00Z", authNow)
	w.sesBinh = w.session(0x96, w.binh, w.bearerBinh, "google", "2030-09-14T00:00:00Z",
		"2030-12-01T00:00:00Z")
	w.sesXoa = w.session(0x97, w.xoa, w.bearerXoa, "otp", "2030-09-14T00:00:00Z",
		"2030-12-01T00:00:00Z")
	w.sesInvite = w.session(0x98, w.an, w.bearerInvite, "invite", "2030-09-16T00:00:00Z",
		"2030-12-01T00:00:00Z", "issued_from_invite_id", w.invSpent)
	w.missingSession = fid(kindAccountSession, 0x9f)

	w.phoneA, w.phoneB = authPhone(1234501), authPhone(1234502)
	w.phoneC, w.phoneD = authPhone(1234503), authPhone(1234504)
	w.phoneE = authPhone(1234505)
	w.digestA = w.phoneDigest(w.phoneA)
	w.digestB = w.phoneDigest(w.phoneB)
	w.digestC = w.phoneDigest(w.phoneC)
	w.digestD = w.phoneDigest(w.phoneD)
	w.digestE = w.phoneDigest(w.phoneE)
	w.personB = w.derivedPerson(w.phoneB)
	w.personC = w.derivedPerson(w.phoneC)
	w.personD = w.derivedPerson(w.phoneD)
	w.insert("people", "id", w.personC, "display_name", "Số cũ đã xoá (dữ liệu mẫu)",
		"created_at", authCreated, "deleted_at", "2030-09-22T00:00:00Z")
	w.insert("people", "id", w.personD, "display_name", "Số chưa buộc (dữ liệu mẫu)",
		"created_at", authCreated)
	w.codeLive, w.codeBurned, w.codeWrong = authCode(424201), authCode(424202), authCode(424203)

	// Two live challenges for phoneA, the newer one first in time. The window
	// is fifteen minutes and the cooldown one minute, both from the service's
	// clock, so these sit either side of both edges.
	w.chLive = w.challenge(0x90, w.digestA, w.codeLive, "2030-10-01T04:57:00Z",
		"2030-10-01T05:02:00Z")
	w.chSecond = w.challenge(0x91, w.digestA, w.codeLive, "2030-10-01T04:52:00Z",
		"2030-10-01T04:57:00Z")
	// Created exactly fifteen minutes before the clock: `created_at > since` is
	// strict, so this one is outside the window by a hair.
	w.chEdge = w.challenge(0x92, w.digestA, w.codeLive, "2030-10-01T04:45:00.123456Z",
		"2030-10-01T04:50:00Z")
	w.chBurned = w.challenge(0x93, w.digestB, w.codeBurned, "2030-10-01T04:58:00Z",
		"2030-10-01T05:03:00Z", "attempts", 5)
	w.chConsumed = w.challenge(0x94, w.digestB, w.codeLive, "2030-10-01T04:59:00Z",
		"2030-10-01T05:04:00Z", "attempts", 1, "consumed_at", "2030-10-01T04:59:30Z")
	// Expires at the clock to the microsecond: `expires_at <= now` is expired.
	w.chExpired = w.challenge(0x95, w.digestC, w.codeLive, "2030-10-01T04:56:00Z", authNow)
	// Thirty seconds old: inside the one-minute cooldown, so a second code for
	// phoneA is refused before the window ceiling is even counted.
	w.chJustNow = w.challenge(0x96, w.digestA, w.codeLive, "2030-10-01T04:59:30Z",
		"2030-10-01T05:04:30Z")
	// One live challenge per number that a verify case spends. Each was created
	// outside the fifteen-minute window, so it does not stand in the way of a
	// request case for the same number.
	w.chLiveB = w.challenge(0x97, w.digestB, w.codeLive, "2030-10-01T04:41:30Z",
		"2030-10-01T05:11:00Z")
	w.chLiveC = w.challenge(0x98, w.digestC, w.codeLive, "2030-10-01T04:41:00Z",
		"2030-10-01T05:10:00Z")
	w.chLiveD = w.challenge(0x99, w.digestD, w.codeLive, "2030-10-01T04:40:00Z",
		"2030-10-01T05:10:00Z")
	// Five codes for one number, none of them newer than the cooldown: the
	// window ceiling, which is the other half of plan_request.
	for i, minute := range []string{"04:50", "04:51", "04:52", "04:53", "04:54"} {
		w.challenge(0x9a+i, w.digestE, w.codeLive, "2030-10-01T"+minute+":00Z",
			"2030-10-01T05:20:00Z")
	}
	w.missingCh = fid(kindOtpChallenge, 0x9f)

	w.googleBinh = "google-sub-binh-mau"
	w.idPhoneAn = w.loginProof(0x90, w.an, "phone", hex.EncodeToString(w.digestA),
		"2030-09-20T00:00:00Z")
	w.idGoogleBinh = w.loginProof(0x91, w.binh, "google", w.googleBinh, "2030-09-21T00:00:00Z")
	return w
}

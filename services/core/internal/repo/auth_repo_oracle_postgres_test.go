//go:build postgres

package repo

// Differential test of the W9 repository methods (the root of trust: bearer
// sessions, the roster a session may claim, the named invitation's secret, and
// the OTP challenge lifecycle) against the real SqlAlchemyApiRepository, driven
// through scripts/render_w9_repo_oracle.py. Same design as
// outings_repo_oracle_postgres_test.go, whose case format, value tags,
// statement normalisation, conflict tagging, row lock probe and comparison it
// reuses.
//
// What this wave has to pin that the earlier ones did not:
//
//   - Two paths to one row. A session is named on the wire by a bearer, and
//     what is stored is a digest of it; `DELETE /sessions/current` reaches the
//     row through that digest and `DELETE /sessions/{id}` through its primary
//     key. The corpus walks both at the same row and compares the table dump
//     after each, so a port that resolved them differently could not read green.
//   - Two deadlines, drawn from opposite sides. actor_for_session_token refuses
//     at `expires_at <= now` and list_account_sessions keeps at
//     `expires_at > now`; the fixture puts one session's deadline on the case's
//     clock to the microsecond, so a `<` written as `<=` moves a row.
//   - Secrets that must not be written down. No telephone number, no code and
//     no bearer appears in any file of this wave: an invitation's and a
//     session's digest is sha256 of a sample word computed inside PostgreSQL,
//     and a phone or code digest is an HMAC under a key the test generates at
//     run time and hands to the Python container.
//
// A known property of this oracle, so nobody leans on it wrongly: it records
// the SQL TEXT, not the bound arguments. A wrong value is caught only when it
// changes a result or a table dump. Every argument of this wave that can
// change neither -- the `LIMIT 1` of the two digest reads, which sits on a
// unique column -- is pinned instead by TestAuthRepoBindsWhatPythonBinds
// below, whose expectations are read off the Python source rather than off the
// oracle.
//
// Without CORE_PYTHON_IMAGE every test here skips; scripts/go_postgres_tier.sh
// sets it and refuses skips.

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	osexec "os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"mobile/services/core/internal/testdb"
)

var (
	authPeopleDump     = peopleDump("people")
	authSessionsDump   = peopleDump("account_sessions")
	authChallengesDump = peopleDump("otp_challenges")
	authIdentitiesDump = peopleDump("account_identities")
	authInvitesDump    = peopleDump("outing_invites")
	authMembersDump    = peopleDump("memberships")
)

// probeAuthRowLocks lists the row locks held on every table a W9 method locks
// FOR UPDATE, updates, inserts or references through a foreign key it writes.
var probeAuthRowLocks = func() string {
	var parts []string
	for _, table := range []string{"people", "contexts", "memberships", "outings", "outing_invites",
		"account_sessions", "account_identities", "otp_challenges"} {
		parts = append(parts, `SELECT '`+table+`' AS rel, t.id::text AS id, array_to_string(r.modes, ',') AS modes
			FROM public.pgrowlocks('`+table+`') r LEFT JOIN `+table+` t ON t.ctid = r.locked_row`)
	}
	return `SELECT l.rel || ' ' || coalesce(l.id, '(superseded row)') || ' ' || l.modes FROM (` +
		strings.Join(parts, " UNION ALL ") + `) l ORDER BY 1`
}()

// ---------------------------------------------------------------------------
// Tags and the Go side
// ---------------------------------------------------------------------------

// tBytes is how the driver tags a `bytes`: its hexadecimal. Only
// OtpChallengeRecord carries any.
func tBytes(value []byte) any { return tv("bytes", hex.EncodeToString(value)) }

func tAccountSession(s AccountSession) any {
	return tRecord("AccountSessionRecord", "id", tUUID(s.ID), "person_id", tUUID(s.PersonID),
		"issued_from_invite_id", optional(s.IssuedFromInviteID, tUUID), "issued_via", tStr(s.IssuedVia),
		"created_at", tInstant(s.CreatedAt), "expires_at", tInstant(s.ExpiresAt),
		"revoked_at", optional(s.RevokedAt, tInstant))
}

func nilAccountSession(s *AccountSession) any {
	if s == nil {
		return nil
	}
	return tAccountSession(*s)
}

func tOtpChallenge(c OtpChallenge) any {
	return tRecord("OtpChallengeRecord", "id", tUUID(c.ID), "phone_digest", tBytes(c.PhoneDigest),
		"code_digest", tBytes(c.CodeDigest), "created_at", tInstant(c.CreatedAt),
		"expires_at", tInstant(c.ExpiresAt), "attempts", tInt(c.Attempts),
		"consumed_at", optional(c.ConsumedAt, tInstant))
}

func nilOtpChallenge(c *OtpChallenge) any {
	if c == nil {
		return nil
	}
	return tOtpChallenge(*c)
}

func tAccountIdentity(a AccountIdentity) any {
	return tRecord("AccountIdentityRecord", "id", tUUID(a.ID), "person_id", tUUID(a.PersonID),
		"provider", tStr(a.Provider), "subject", tStr(a.Subject), "created_at", tInstant(a.CreatedAt),
		"last_login_at", tInstant(a.LastLoginAt))
}

func nilAccountIdentity(a *AccountIdentity) any {
	if a == nil {
		return nil
	}
	return tAccountIdentity(*a)
}

// tActorGrants tags the two frozensets the way render_social_repo_oracle.py
// does: members sorted by their tagged JSON text, which for one tag key is the
// value's own order. ActorGrants already returns both sorted.
func tActorGrants(g ActorGrants) any {
	contexts := []any{}
	for _, id := range g.ContextIDs {
		contexts = append(contexts, tUUID(id))
	}
	return tRecord("ActorGrants", "person_exists", tBool(g.PersonExists), "roles", tSet(g.Roles),
		"context_ids", tv("set", contexts))
}

// authHexDigest reads a hexadecimal digest a case carries.
func authHexDigest(text string) []byte {
	raw, err := hex.DecodeString(text)
	if err != nil {
		panic(err)
	}
	return raw
}

func authRepoGoCall(repo Repository, method string, a map[string]any) (any, error) {
	s := func(key string) string { return argString(a, key) }
	now := func() time.Time { return argInstant(s("now")) }
	if strings.HasPrefix(method, "route.") {
		return authRouteGo(repo, method, a)
	}
	switch method {
	case "create_account_session":
		session, err := repo.CreateAccountSession(bg, AccountSessionInput{PersonID: s("person_id"),
			TokenDigest: digestOf(s("token")), IssuedFromInviteID: argText(a["issued_from_invite_id"]),
			ExpiresAt: argInstant(s("expires_at")), Now: now(), IssuedVia: s("issued_via")})
		if err != nil {
			return nil, err
		}
		return tAccountSession(session), nil
	case "get_account_session_by_digest":
		session, err := repo.GetAccountSessionByDigest(bg, digestOf(s("token")))
		return nilAccountSession(session), err
	case "get_account_session":
		session, err := repo.GetAccountSession(bg, s("session_id"))
		return nilAccountSession(session), err
	case "list_account_sessions":
		rows, err := repo.ListAccountSessions(bg, s("person_id"), now())
		items := []any{}
		for _, row := range rows {
			items = append(items, tAccountSession(row))
		}
		return tSeq(items), err
	case "revoke_account_session":
		session, err := repo.RevokeAccountSession(bg, s("session_id"), now())
		return nilAccountSession(session), err
	case "actor_grants":
		grants, err := repo.ActorGrants(bg, s("person_id"))
		if err != nil {
			return nil, err
		}
		return tActorGrants(grants), nil
	case "consume_named_invite_secret":
		invite, err := repo.ConsumeNamedInviteSecret(bg, s("invite_id"), digestOf(s("token")),
			s("accepted_by_id"), now())
		if err != nil {
			return nil, err
		}
		return tOutingInvite(invite), nil
	case "create_otp_challenge":
		challenge, err := repo.CreateOtpChallenge(bg, OtpChallengeInput{ChallengeID: s("challenge_id"),
			PhoneDigest: authHexDigest(s("phone_digest")), CodeDigest: authHexDigest(s("code_digest")),
			ExpiresAt: argInstant(s("expires_at")), Now: now()})
		if err != nil {
			return nil, err
		}
		return tOtpChallenge(challenge), nil
	case "recent_otp_challenges":
		rows, err := repo.RecentOtpChallenges(bg, authHexDigest(s("phone_digest")), argInstant(s("since")))
		items := []any{}
		for _, row := range rows {
			items = append(items, tOtpChallenge(row))
		}
		return tSeq(items), err
	case "get_otp_challenge":
		challenge, err := repo.GetOtpChallenge(bg, s("challenge_id"))
		return nilOtpChallenge(challenge), err
	case "record_otp_attempt":
		consumed, _ := a["consumed"].(bool)
		challenge, err := repo.RecordOtpAttempt(bg, s("challenge_id"), argNumber(a["attempts"]), consumed, now())
		return nilOtpChallenge(challenge), err
	case "get_account_identity":
		found, err := repo.GetAccountIdentity(bg, s("provider"), s("subject"))
		return nilAccountIdentity(found), err
	case "upsert_account_identity":
		found, err := repo.UpsertAccountIdentity(bg, s("person_id"), s("provider"), s("subject"), now())
		if err != nil {
			return nil, err
		}
		return tAccountIdentity(found), nil
	case "create_person_with_identity":
		found, err := repo.CreatePersonWithIdentity(bg, s("person_id"), s("display_name"), s("provider"),
			s("subject"), now())
		if err != nil {
			return nil, err
		}
		return tAccountIdentity(found), nil
	case "create_person":
		person, err := repo.CreatePerson(bg, s("person_id"), s("display_name"))
		if err != nil {
			return nil, err
		}
		return tPerson(&person), nil
	}
	return outingRepoGoCall(repo, method, a)
}

func runAuthGoCase(t *testing.T, pool *pgxpool.Pool, c oracleCase) []any {
	t.Helper()
	tx, err := pool.Begin(bg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(bg) }()
	for _, sql := range c.Setup {
		if _, err := tx.Exec(bg, sql); err != nil {
			t.Fatalf("setup: %v\n%s", err, sql)
		}
	}
	steps := []any{}
	for _, step := range c.Steps {
		for _, sql := range step.Before {
			if _, err := tx.Exec(bg, sql); err != nil {
				t.Fatalf("before: %v\n%s", err, sql)
			}
		}
		rec := &recorder{Querier: tx}
		value, err := authRepoGoCall(Repository{Q: rec}, step.Call, step.Args)
		statements := []any{}
		for _, sql := range rec.log {
			statements = append(statements, normalizeSQL(sql))
		}
		out := map[string]any{"result": nil, "error": nil, "warnings": []any{}, "statements": statements,
			"probes": nil}
		if err != nil {
			out["error"] = pairRepoGoError(err)
			steps = append(steps, generic(t, out))
			break
		}
		out["result"] = value
		probes := []any{}
		for _, sql := range step.Probes {
			rows, err := tx.Query(bg, sql)
			if err != nil {
				t.Fatalf("probe: %v\n%s", err, sql)
			}
			texts, err := pgx.CollectRows(rows, pgx.RowTo[string])
			if err != nil {
				t.Fatal(err)
			}
			probes = append(probes, append([]string{}, texts...))
		}
		out["probes"] = probes
		steps = append(steps, generic(t, out))
	}
	return steps
}

// normalizeAuthSteps is normalizeOutingSteps for this wave's probes.
func normalizeAuthSteps(steps []any, c oracleCase) []any {
	spec, _ := json.Marshal(c)
	known := map[string]bool{}
	for _, id := range uuidText.FindAllString(string(spec), -1) {
		known[id] = true
	}
	out := make([]any, len(steps))
	for i, step := range steps {
		out[i] = step
		probes, _ := step.(map[string]any)["probes"].([]any)
		for j, probe := range c.Steps[i].Probes {
			if j >= len(probes) {
				continue
			}
			rows, ok := probes[j].([]any)
			if !ok {
				continue
			}
			switch probe {
			case probeAuthRowLocks:
				masked := make([]string, len(rows))
				for k, row := range rows {
					masked[k] = uuidText.ReplaceAllStringFunc(row.(string), func(id string) string {
						if known[id] {
							return id
						}
						return "<generated>"
					})
				}
				sort.Strings(masked)
				sorted := make([]any, len(masked))
				for k, row := range masked {
					sorted[k] = row
				}
				probes[j] = sorted
			case probeNow:
				if len(rows) == 1 {
					if now, ok := rows[0].(string); ok && now != "" {
						out[i] = replaceText(out[i], now, transactionNow)
					}
				}
			}
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Cases
// ---------------------------------------------------------------------------

func authRepoOracleCases(w *authWorld) ([]socialCase, oracleSpec) {
	var cases []socialCase
	probes := []string{probeLocks, probeWrites, probeAuthRowLocks, authPeopleDump, authSessionsDump,
		authChallengesDump, authIdentitiesDump, authInvitesDump, authMembersDump, probeNow}
	args := func(pairs ...any) map[string]any {
		out := map[string]any{"now": authNow}
		for i := 0; i < len(pairs); i += 2 {
			out[pairs[i].(string)] = pairs[i+1]
		}
		return out
	}
	step := func(call string, pairs ...any) oracleCall {
		return oracleCall{Call: call, Args: args(pairs...), Before: append([]string{}, writesBaseline...),
			Probes: append([]string{}, probes...)}
	}
	add := func(name, wantEnd string, steps ...oracleCall) {
		cases = append(cases, socialCase{oracleCase{Name: name, Setup: append([]string{}, w.sql...),
			Steps: steps}, wantEnd})
	}
	// A clock one microsecond either side of the deadline the fixture sits on.
	const justBefore = "2030-10-01T05:00:00.123455Z"
	const justAfter = "2030-10-01T05:00:00.123457Z"
	hexA, hexB := hex.EncodeToString(w.digestA), hex.EncodeToString(w.digestB)
	hexC := hex.EncodeToString(w.digestC)

	// --- create_account_session --------------------------------------------
	add("create_account_session: every door, and the provenance each implies", "",
		step("create_account_session", "person_id", w.an, "token", "cap-moi-mot-mau",
			"issued_from_invite_id", nil, "expires_at", "2030-11-01T00:00:00Z", "issued_via", "otp"),
		step("create_account_session", "person_id", w.binh, "token", "cap-moi-hai-mau",
			"issued_from_invite_id", nil, "expires_at", "2030-11-01T00:00:00.5Z", "issued_via", "google"),
		step("create_account_session", "person_id", w.an, "token", "cap-moi-ba-mau",
			"issued_from_invite_id", w.invChi, "expires_at", "2030-11-01T00:00:00Z"),
		step("create_account_session", "person_id", w.an, "token", "cap-moi-bon-mau",
			"issued_from_invite_id", nil, "expires_at", "2030-11-01T00:00:00Z"))
	add("create_account_session: an erased account still gets a row", "",
		step("create_account_session", "person_id", w.xoa, "token", "cap-cho-nguoi-da-xoa-mau",
			"issued_from_invite_id", nil, "expires_at", "2030-11-01T00:00:00Z", "issued_via", "otp"))
	add("create_account_session: a person with no row", "IntegrityError",
		step("create_account_session", "person_id", w.missingPerson, "token", "cap-ma-mau",
			"issued_from_invite_id", nil, "expires_at", "2030-11-01T00:00:00Z", "issued_via", "otp"))
	add("create_account_session: the digest is already somebody's", "IntegrityError",
		step("create_account_session", "person_id", w.binh, "token", w.bearerAn,
			"issued_from_invite_id", nil, "expires_at", "2030-11-01T00:00:00Z", "issued_via", "otp"))
	add("create_account_session: a deadline the check refuses", "IntegrityError",
		step("create_account_session", "person_id", w.an, "token", "cap-han-sai-mau",
			"issued_from_invite_id", nil, "expires_at", "2030-09-01T00:00:00Z", "issued_via", "otp"))

	// --- reading a session --------------------------------------------------
	var byDigest []oracleCall
	for _, token := range []string{w.bearerAn, w.bearerRevoked, w.bearerEdge, w.bearerXoa,
		w.bearerInvite, w.bearerNobodyIssued} {
		byDigest = append(byDigest, step("get_account_session_by_digest", "token", token))
	}
	add("get_account_session_by_digest: live, revoked, on the deadline, erased, unknown", "", byDigest...)

	var byID []oracleCall
	for _, id := range []string{w.sesAn, w.sesRevoked, w.sesEdge, w.sesInvite, w.missingSession} {
		byID = append(byID, step("get_account_session", "session_id", id))
	}
	add("get_account_session: by primary key, and one that is not there", "", byID...)
	add("get_account_session and get_account_session_by_digest name the same row", "",
		step("get_account_session_by_digest", "token", w.bearerAn),
		step("get_account_session", "session_id", w.sesAn))

	// --- list_account_sessions ---------------------------------------------
	add("list_account_sessions: newest first, the id breaks the tie, revoked and expired are gone", "",
		step("list_account_sessions", "person_id", w.an),
		step("list_account_sessions", "person_id", w.binh),
		step("list_account_sessions", "person_id", w.moi),
		step("list_account_sessions", "person_id", w.missingPerson))
	add("list_account_sessions: one microsecond either side of a deadline", "",
		func() oracleCall {
			s := step("list_account_sessions", "person_id", w.an)
			s.Args["now"] = justBefore
			return s
		}(),
		func() oracleCall {
			s := step("list_account_sessions", "person_id", w.an)
			s.Args["now"] = justAfter
			return s
		}())

	// --- revoke_account_session ---------------------------------------------
	add("revoke_account_session: once writes, twice does not move the moment", "",
		step("revoke_account_session", "session_id", w.sesAn),
		step("revoke_account_session", "session_id", w.sesAn))
	add("revoke_account_session: a session revoked before keeps its moment", "",
		step("revoke_account_session", "session_id", w.sesRevoked))
	add("revoke_account_session: a session that is not there", "",
		step("revoke_account_session", "session_id", w.missingSession))
	add("revoke_account_session: an expired session is still revocable", "",
		step("revoke_account_session", "session_id", w.sesEdge))

	// --- actor_grants -------------------------------------------------------
	var grants []oracleCall
	for _, id := range []string{w.an, w.binh, w.chi, w.roi, w.xoa, w.moi, w.missingPerson} {
		grants = append(grants, step("actor_grants", "person_id", id))
	}
	add("actor_grants: every membership state, an erased account and a missing one", "", grants...)

	// --- consume_named_invite_secret ----------------------------------------
	add("consume_named_invite_secret: spent once, then the same word again",
		"OUTING_INVITE_NOT_REDEEMABLE",
		step("consume_named_invite_secret", "invite_id", w.invChi, "token", w.tokChi,
			"accepted_by_id", w.chi),
		step("consume_named_invite_secret", "invite_id", w.invChi, "token", w.tokChi,
			"accepted_by_id", w.chi))
	add("consume_named_invite_secret: an invitation already accepted keeps its moment", "",
		step("consume_named_invite_secret", "invite_id", w.invAcceptedLive, "token", w.tokAccepted,
			"accepted_by_id", w.roi))
	add("consume_named_invite_secret: a link names nobody", "OUTING_INVITE_NOT_NAMED",
		step("consume_named_invite_secret", "invite_id", w.invLink, "token", w.tokLink,
			"accepted_by_id", w.chi))
	add("consume_named_invite_secret: a word that is not this row's", "OUTING_INVITE_NOT_REDEEMABLE",
		step("consume_named_invite_secret", "invite_id", w.invMoi, "token", w.tokChi,
			"accepted_by_id", w.moi))
	add("consume_named_invite_secret: a row somebody revoked", "OUTING_INVITE_NOT_REDEEMABLE",
		step("consume_named_invite_secret", "invite_id", w.invRevoked, "token", w.tokRevoked,
			"accepted_by_id", w.roi))
	add("consume_named_invite_secret: a deadline that is exactly now", "OUTING_INVITE_NOT_REDEEMABLE",
		step("consume_named_invite_secret", "invite_id", w.invEdge, "token", w.tokEdge,
			"accepted_by_id", w.binh))
	add("consume_named_invite_secret: a row whose secret was already removed", "OUTING_INVITE_NOT_REDEEMABLE",
		step("consume_named_invite_secret", "invite_id", w.invSpent, "token", w.tokSpent,
			"accepted_by_id", w.xoa))
	add("consume_named_invite_secret: an invitation that is not there", "OUTING_INVITE_NOT_FOUND",
		step("consume_named_invite_secret", "invite_id", w.missingInvite, "token", w.tokChi,
			"accepted_by_id", w.chi))
	add("consume_named_invite_secret: one microsecond before the deadline", "",
		func() oracleCall {
			s := step("consume_named_invite_secret", "invite_id", w.invEdge, "token", w.tokEdge,
				"accepted_by_id", w.binh)
			s.Args["now"] = justBefore
			return s
		}())

	// --- the OTP challenge --------------------------------------------------
	fresh := fid(kindOtpChallenge, 0xa0)
	add("create_otp_challenge: a new row, then the same id twice", "IntegrityError",
		step("create_otp_challenge", "challenge_id", fresh, "phone_digest", hexA,
			"code_digest", hexB, "expires_at", "2030-10-01T05:05:00Z"),
		step("create_otp_challenge", "challenge_id", fresh, "phone_digest", hexA,
			"code_digest", hexB, "expires_at", "2030-10-01T05:05:00Z"))
	add("create_otp_challenge: a deadline the check refuses", "IntegrityError",
		step("create_otp_challenge", "challenge_id", fid(kindOtpChallenge, 0xa1), "phone_digest", hexA,
			"code_digest", hexB, "expires_at", "2030-10-01T04:00:00Z"))

	add("recent_otp_challenges: the window edge is strict, newest first", "",
		step("recent_otp_challenges", "phone_digest", hexA, "since", "2030-10-01T04:45:00.123456Z"),
		step("recent_otp_challenges", "phone_digest", hexA, "since", "2030-10-01T04:45:00.123455Z"),
		step("recent_otp_challenges", "phone_digest", hexB, "since", "2030-10-01T04:45:00.123456Z"),
		step("recent_otp_challenges", "phone_digest", hexC, "since", "2030-10-01T04:45:00.123456Z"),
		step("recent_otp_challenges", "phone_digest", hex.EncodeToString(w.digestD),
			"since", "2030-10-01T04:45:00.123456Z"))

	var challenges []oracleCall
	for _, id := range []string{w.chLive, w.chBurned, w.chConsumed, w.chExpired, w.missingCh} {
		challenges = append(challenges, step("get_otp_challenge", "challenge_id", id))
	}
	add("get_otp_challenge: every state, and one that is not there", "", challenges...)

	add("record_otp_attempt: a wrong guess, the same count again, then the code is spent", "",
		step("record_otp_attempt", "challenge_id", w.chLive, "attempts", 1, "consumed", false),
		step("record_otp_attempt", "challenge_id", w.chLive, "attempts", 1, "consumed", false),
		step("record_otp_attempt", "challenge_id", w.chLive, "attempts", 2, "consumed", true))
	add("record_otp_attempt: a challenge already spent keeps the moment it was spent at", "",
		step("record_otp_attempt", "challenge_id", w.chConsumed, "attempts", 2, "consumed", true))
	add("record_otp_attempt: a challenge that is not there", "",
		step("record_otp_attempt", "challenge_id", w.missingCh, "attempts", 1, "consumed", false))
	add("record_otp_attempt: a count the check refuses", "IntegrityError",
		step("record_otp_attempt", "challenge_id", w.chLive, "attempts", -1, "consumed", false))

	// --- account identities --------------------------------------------------
	add("get_account_identity: bound, and two proofs nobody ever presented", "",
		step("get_account_identity", "provider", "phone", "subject", hexA),
		step("get_account_identity", "provider", "google", "subject", w.googleBinh),
		step("get_account_identity", "provider", "phone", "subject", hexB),
		step("get_account_identity", "provider", "google", "subject", hexA))
	add("upsert_account_identity: a re-login moves the timestamp and nothing else", "",
		step("upsert_account_identity", "person_id", w.an, "provider", "phone", "subject", hexA),
		step("upsert_account_identity", "person_id", w.an, "provider", "phone", "subject", hexA),
		step("upsert_account_identity", "person_id", w.binh, "provider", "phone", "subject", hexA))
	add("upsert_account_identity: a proof nobody presented before", "",
		step("upsert_account_identity", "person_id", w.moi, "provider", "phone", "subject", hexB))
	add("upsert_account_identity: a person with no row", "IntegrityError",
		step("upsert_account_identity", "person_id", w.missingPerson, "provider", "google",
			"subject", "google-sub-ma-mau"))

	add("create_person_with_identity: a new person and the proof that made them", "",
		step("create_person_with_identity", "person_id", fid(kindPerson, 0xa1),
			"display_name", "Người Google (dữ liệu mẫu)", "provider", "google",
			"subject", "google-sub-moi-mau"))
	add("create_person_with_identity: the proof is already bound, so neither row survives",
		"IDENTITY_ALREADY_BOUND",
		step("create_person_with_identity", "person_id", fid(kindPerson, 0xa2),
			"display_name", "Người thua (dữ liệu mẫu)", "provider", "google", "subject", w.googleBinh))
	add("create_person_with_identity: the person id is already taken", "IDENTITY_ALREADY_BOUND",
		step("create_person_with_identity", "person_id", w.an, "display_name", "Trùng (dữ liệu mẫu)",
			"provider", "google", "subject", "google-sub-khac-mau"))

	// --- routes --------------------------------------------------------------
	route := func(name string, pairs ...any) oracleCall {
		return step(name, pairs...)
	}
	add("route.create_session: a named invitation becomes a session", "",
		route("route.create_session", "token", w.tokChi, "session_token", "phien-moi-chi-mau"))
	add("route.create_session: the same word a second time", "404:invite_not_found",
		route("route.create_session", "token", w.tokChi, "session_token", "phien-moi-chi-mau"),
		route("route.create_session", "token", w.tokChi, "session_token", "phien-moi-chi-hai-mau"))
	add("route.create_session: a person with no membership yet", "",
		route("route.create_session", "token", w.tokMoi, "session_token", "phien-moi-nguoi-moi-mau"))
	add("route.create_session: a link names nobody", "404:invite_not_found",
		route("route.create_session", "token", w.tokLink, "session_token", "khong-cap-mau"))
	add("route.create_session: a revoked invitation", "404:invite_not_found",
		route("route.create_session", "token", w.tokRevoked, "session_token", "khong-cap-mau"))
	add("route.create_session: a deadline that is exactly now", "404:invite_not_found",
		route("route.create_session", "token", w.tokEdge, "session_token", "khong-cap-mau"))
	add("route.create_session: a word nobody ever issued", "404:invite_not_found",
		route("route.create_session", "token", w.tokNobodyIssued, "session_token", "khong-cap-mau"))

	add("route.actor_for_session_token: a live bearer", "",
		route("route.actor_for_session_token", "bearer", w.bearerAn))
	add("route.actor_for_session_token: a bearer nobody was ever given", "401:authentication_required",
		route("route.actor_for_session_token", "bearer", w.bearerNobodyIssued))
	add("route.actor_for_session_token: a bearer somebody revoked", "401:authentication_required",
		route("route.actor_for_session_token", "bearer", w.bearerRevoked))
	add("route.actor_for_session_token: a deadline that is exactly now", "401:authentication_required",
		route("route.actor_for_session_token", "bearer", w.bearerEdge))
	add("route.actor_for_session_token: one microsecond before that deadline", "",
		func() oracleCall {
			s := route("route.actor_for_session_token", "bearer", w.bearerEdge)
			s.Args["now"] = justBefore
			return s
		}())
	add("route.actor_for_session_token: the person behind the session was erased",
		"401:authentication_required",
		route("route.actor_for_session_token", "bearer", w.bearerXoa))

	add("route.list_sessions: this session is named among the others", "",
		route("route.list_sessions", "bearer", w.bearerAn, "actor_id", w.an))
	add("route.list_sessions: a holder whose only group is a pair", "",
		route("route.list_sessions", "bearer", w.bearerBinh, "actor_id", w.binh))
	add("route.list_sessions: a dead bearer never reaches the list", "401:authentication_required",
		route("route.list_sessions", "bearer", w.bearerRevoked, "actor_id", w.an))

	add("route.revoke_current_session: the digest path and the id path reach one row", "",
		route("route.revoke_current_session", "bearer", w.bearerAn),
		route("route.revoke_current_session", "bearer", w.bearerAn))
	add("route.revoke_current_session: a bearer nobody was ever given is still 204", "",
		route("route.revoke_current_session", "bearer", w.bearerNobodyIssued))
	add("route.revoke_current_session: an expired bearer is still signed out", "",
		route("route.revoke_current_session", "bearer", w.bearerEdge))

	add("route.revoke_session: another device of the same account", "",
		route("route.revoke_session", "bearer", w.bearerAn, "actor_id", w.an,
			"session_id", w.sesAnOld))
	add("route.revoke_session: this very session, by its id", "",
		route("route.revoke_session", "bearer", w.bearerAn, "actor_id", w.an, "session_id", w.sesAn))
	add("route.revoke_session: somebody else's session is 404, not 403", "404:session_not_found",
		route("route.revoke_session", "bearer", w.bearerAn, "actor_id", w.an, "session_id", w.sesBinh))
	add("route.revoke_session: a session that is not there", "404:session_not_found",
		route("route.revoke_session", "bearer", w.bearerAn, "actor_id", w.an,
			"session_id", w.missingSession))

	add("route.request_otp: a number with nothing recent", "",
		route("route.request_otp", "phone", w.phoneD, "code", w.codeLive,
			"ids", []any{fid(kindOtpChallenge, 0xa2)}))
	add("route.request_otp: the cooldown since the newest code", "429:otp_resend_too_soon",
		route("route.request_otp", "phone", w.phoneA, "code", w.codeLive,
			"ids", []any{fid(kindOtpChallenge, 0xa3)}))
	add("route.request_otp: five codes in one window is the other ceiling",
		"429:otp_too_many_requests",
		route("route.request_otp", "phone", w.phoneE, "code", w.codeLive,
			"ids", []any{fid(kindOtpChallenge, 0xa6)}))
	add("route.request_otp: the gateway refuses, so the challenge is burned where it stands",
		"503:sms_unavailable",
		route("route.request_otp", "phone", w.phoneD, "code", w.codeLive, "sms_fails", true,
			"ids", []any{fid(kindOtpChallenge, 0xa4)}))
	add("route.request_otp: a number that is not a mobile", "422:phone_not_mobile",
		route("route.request_otp", "phone", "khong-phai-so", "code", w.codeLive,
			"ids", []any{fid(kindOtpChallenge, 0xa5)}))

	add("route.verify_otp: the right code for a number already bound", "",
		route("route.verify_otp", "challenge_id", w.chLive, "phone", w.phoneA, "code", w.codeLive,
			"session_token", "phien-otp-an-mau", "ids", []any{}))
	add("route.verify_otp: a wrong code leaves the count one higher", "422:otp_code_invalid",
		route("route.verify_otp", "challenge_id", w.chLive, "phone", w.phoneA, "code", w.codeWrong,
			"session_token", "khong-cap-mau", "ids", []any{}))
	add("route.verify_otp: the guess that burns the challenge", "429:otp_too_many_attempts",
		func() oracleCall {
			s := route("route.verify_otp", "challenge_id", w.chLive, "phone", w.phoneA,
				"code", w.codeWrong, "session_token", "khong-cap-mau", "ids", []any{})
			s.Before = append([]string{`UPDATE otp_challenges SET attempts = 4 WHERE id = '` +
				w.chLive + `'`}, s.Before...)
			return s
		}())
	add("route.verify_otp: a challenge issued to another number is no challenge",
		"404:otp_challenge_not_found",
		route("route.verify_otp", "challenge_id", w.chLive, "phone", w.phoneB, "code", w.codeLive,
			"session_token", "khong-cap-mau", "ids", []any{}))
	add("route.verify_otp: a challenge already spent", "404:otp_challenge_not_found",
		route("route.verify_otp", "challenge_id", w.chConsumed, "phone", w.phoneB, "code", w.codeLive,
			"session_token", "khong-cap-mau", "ids", []any{}))
	add("route.verify_otp: a deadline that is exactly now", "404:otp_challenge_not_found",
		route("route.verify_otp", "challenge_id", w.chExpired, "phone", w.phoneC, "code", w.codeLive,
			"session_token", "khong-cap-mau", "ids", []any{}))
	add("route.verify_otp: a challenge burned before this guess", "404:otp_challenge_not_found",
		route("route.verify_otp", "challenge_id", w.chBurned, "phone", w.phoneB, "code", w.codeBurned,
			"session_token", "khong-cap-mau", "ids", []any{}))
	add("route.verify_otp: a number nobody registered becomes a new person", "",
		// `expected_person_id` is read by neither side. It names the id
		// derive_person_id must mint here, so the comparison pins that exact
		// value instead of binding it as «some id both sides generated» -- and
		// so the corpus states that a derived id is a version 8 UUID, not a
		// uuid4.
		route("route.verify_otp", "challenge_id", w.chLiveB, "phone", w.phoneB,
			"code", w.codeLive, "session_token", "phien-otp-moi-mau", "ids", []any{},
			"expected_person_id", w.personB))
	add("route.verify_otp: a number whose account was erased comes back as somebody else", "",
		route("route.verify_otp", "challenge_id", w.chLiveC, "phone", w.phoneC, "code", w.codeLive,
			"session_token", "phien-otp-so-cu-mau", "ids", []any{fid(kindPerson, 0xa3)}))
	add("route.verify_otp: a number whose person exists but was never bound", "",
		route("route.verify_otp", "challenge_id", w.chLiveD, "phone", w.phoneD, "code", w.codeLive,
			"session_token", "phien-otp-chua-buoc-mau", "ids", []any{}))

	add("route.login_google: a sub seen before", "",
		route("route.login_google", "id_token", "id-token-mau", "subject", w.googleBinh, "display_name", "Bình (dữ liệu mẫu)",
			"session_token", "phien-google-binh-mau", "ids", []any{}))
	add("route.login_google: a first sub is a NEW person", "",
		route("route.login_google", "id_token", "id-token-mau", "subject", "google-sub-lan-dau-mau", "display_name",
			"Tên Google (dữ liệu mẫu)", "session_token", "phien-google-moi-mau",
			"ids", []any{fid(kindPerson, 0xa4)}))
	add("route.login_google: a first sub with no name at all", "",
		route("route.login_google", "id_token", "id-token-mau", "subject", "google-sub-khong-ten-mau", "display_name", "",
			"session_token", "phien-google-khong-ten-mau", "ids", []any{fid(kindPerson, 0xa5)}))
	add("route.login_google: a host with no client ids refuses before it reads the token",
		"503:google_not_configured",
		route("route.login_google", "id_token", "id-token-mau", "google_configured", false, "subject", w.googleBinh,
			"session_token", "khong-cap-mau", "ids", []any{}))
	add("route.login_google: a blank id_token", "422:id_token_required",
		route("route.login_google", "id_token", "   ", "subject", w.googleBinh,
			"session_token", "khong-cap-mau", "ids", []any{}))
	add("route.login_google: a token the verifier does not vouch for", "401:google_token_invalid",
		route("route.login_google", "id_token", "id-token-mau", "google_invalid", true, "subject", w.googleBinh,
			"session_token", "khong-cap-mau", "ids", []any{}))

	spec := oracleSpec{Clock: []string{}, Cases: make([]oracleCase, 0, len(cases))}
	for _, c := range cases {
		spec.Cases = append(spec.Cases, c.oracleCase)
	}
	return cases, spec
}

// authMethods must each answer normally in at least one case.
var authMethods = []string{
	"create_account_session", "get_account_session_by_digest", "get_account_session",
	"list_account_sessions", "revoke_account_session", "actor_grants",
	"consume_named_invite_secret", "create_otp_challenge", "recent_otp_challenges",
	"get_otp_challenge", "record_otp_attempt", "get_account_identity", "upsert_account_identity",
	"create_person_with_identity",
	"route.create_session", "route.actor_for_session_token", "route.list_sessions",
	"route.revoke_current_session", "route.revoke_session", "route.request_otp",
	"route.verify_otp", "route.login_google",
}

// authBranches are the write branches the corpus must reach in Python, each
// named by the call and a statement only that branch issues.
var authBranches = []struct{ call, prefix, contains, absent string }{
	{"create_account_session", "INSERT INTO account_sessions", "", ""},
	{"revoke_account_session", "UPDATE account_sessions SET revoked_at", "", ""},
	{"consume_named_invite_secret", "SELECT outing_invites.id", "FOR UPDATE", ""},
	{"consume_named_invite_secret", "UPDATE outing_invites SET token_digest=?, accepted_at", "", ""},
	{"consume_named_invite_secret", "UPDATE outing_invites SET token_digest=? WHERE", "", ""},
	{"create_otp_challenge", "INSERT INTO otp_challenges", "", ""},
	{"record_otp_attempt", "UPDATE otp_challenges SET attempts=?::INTEGER WHERE", "", ""},
	{"record_otp_attempt", "UPDATE otp_challenges SET attempts=?::INTEGER, consumed_at", "", ""},
	{"upsert_account_identity", "INSERT INTO account_identities", "", ""},
	{"upsert_account_identity", "UPDATE account_identities SET last_login_at", "", ""},
	{"create_person_with_identity", "SAVEPOINT", "", "ROLLBACK"},
	{"create_person_with_identity", "ROLLBACK TO SAVEPOINT", "", ""},
	{"create_person_with_identity", "RELEASE SAVEPOINT", "", ""},
	{"route.create_session", "INSERT INTO memberships", "", ""},
	{"route.verify_otp", "INSERT INTO people", "", ""},
	{"route.verify_otp", "UPDATE otp_challenges SET attempts=?::INTEGER, consumed_at", "", ""},
	{"route.request_otp", "INSERT INTO otp_challenges", "", ""},
	{"route.login_google", "INSERT INTO account_identities", "", ""},
	{"route.login_google", "UPDATE account_identities SET last_login_at", "", ""},
	{"route.revoke_session", "UPDATE account_sessions SET revoked_at", "", ""},
	{"route.revoke_current_session", "UPDATE account_sessions SET revoked_at", "", ""},
	{"route.list_sessions", "SELECT account_sessions.id, account_sessions.person_id", "ORDER BY", ""},
}

// newAuthKey is MOBILE_PERSON_ID_KEY for one run: long enough for read_key,
// random, and never written to a file.
func newAuthKey(t *testing.T) string {
	t.Helper()
	var raw [24]byte
	if _, err := rand.Read(raw[:]); err != nil {
		t.Fatal(err)
	}
	return "khoa-thu-nghiem-w9-" + hex.EncodeToString(raw[:])
}

func TestAuthRepositoryOracle(t *testing.T) {
	image := os.Getenv("CORE_PYTHON_IMAGE")
	if image == "" {
		t.Skip("CORE_PYTHON_IMAGE not set; see the comment at the top of auth_repo_oracle_postgres_test.go")
	}
	scripts, err := filepath.Abs("../../../../scripts")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(scripts, "render_w9_repo_oracle.py")); err != nil {
		t.Fatal(err)
	}
	if _, err := testdb.Pool(t).Exec(bg, `CREATE EXTENSION IF NOT EXISTS pgrowlocks WITH SCHEMA public`); err != nil {
		t.Fatal(err)
	}
	pool, url := migratedOracleSchema(t, image, "auth_repo_oracle_")

	key := newAuthKey(t)
	authKey = []byte(key)
	t.Cleanup(func() { authKey = nil })
	cases, built := authRepoOracleCases(newAuthWorld(authKey))
	payload, err := json.Marshal(built)
	if err != nil {
		t.Fatal(err)
	}
	var spec oracleSpec
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	if err := decoder.Decode(&spec); err != nil {
		t.Fatal(err)
	}
	driver := osexec.Command("docker", "run", "--rm", "-i", "--network", "host",
		"-e", "ORACLE_DATABASE_URL="+url, "-e", "MOBILE_PERSON_ID_KEY="+key,
		"-v", scripts+":/oracle:ro", "--entrypoint", "python", image,
		"/oracle/render_w9_repo_oracle.py")
	driver.Stdin = bytes.NewReader(payload)
	var stdout, stderr bytes.Buffer
	driver.Stdout, driver.Stderr = &stdout, &stderr
	if err := driver.Run(); err != nil {
		t.Fatalf("render_w9_repo_oracle.py: %v\n%s", err, tail(stderr.Bytes()))
	}
	var python pythonRun
	if err := json.Unmarshal(stdout.Bytes(), &python); err != nil {
		t.Fatalf("python output: %v", err)
	}
	if len(python.Cases) != len(spec.Cases) {
		t.Fatalf("python answered %d cases of %d", len(python.Cases), len(spec.Cases))
	}

	tally := &repoTally{}
	returned := map[string]int{}
	reached := make([]int, len(authBranches))
	routeSteps, dumpRows := 0, 0
	for i, c := range spec.Cases {
		tally.cases++
		if python.Cases[i].Name != c.Name {
			t.Fatalf("case %d is %q in python", i, python.Cases[i].Name)
		}
		pySteps := []any{}
		for j, s := range python.Cases[i].Steps {
			call := c.Steps[j].Call
			if strings.HasPrefix(call, "route.") {
				routeSteps++
			}
			statements := []any{}
			for _, entry := range s.Statements {
				for k := 0; k < int(entry[1].(float64)); k++ {
					statement := normalizeSQL(entry[0].(string))
					statements = append(statements, statement)
					for b, branch := range authBranches {
						if call == branch.call && strings.HasPrefix(statement, branch.prefix) &&
							strings.Contains(statement, branch.contains) &&
							(branch.absent == "" || !strings.Contains(statement, branch.absent)) {
							reached[b]++
						}
					}
				}
			}
			if rows, ok := s.Probes.([]any); ok {
				for k, probe := range c.Steps[j].Probes {
					if k >= len(rows) || !strings.HasPrefix(probe, peopleDumpPrefix) {
						continue
					}
					if list, ok := rows[k].([]any); ok {
						dumpRows += len(list)
					}
				}
			}
			warnings := s.Warnings
			if warnings == nil {
				warnings = []any{}
			}
			if s.Error == nil {
				returned[call]++
			}
			pySteps = append(pySteps, generic(t, map[string]any{"result": s.Result, "error": s.Error,
				"warnings": warnings, "statements": statements, "probes": s.Probes}))
		}

		pyCase := python.Cases[i]
		end := ""
		if len(pyCase.Steps) > 0 {
			if e, ok := pyCase.Steps[len(pyCase.Steps)-1].Error.(map[string]any); ok {
				end, _ = e["type"].(string)
				if code, ok := e["code"].(string); ok {
					end = code
				}
			}
		}
		if len(pyCase.Steps) != len(c.Steps) && end == "" {
			t.Errorf("case %q: python ran %d of %d steps", c.Name, len(pyCase.Steps), len(c.Steps))
		}
		if end != cases[i].wantEnd {
			var last any
			if len(pyCase.Steps) > 0 {
				last = pyCase.Steps[len(pyCase.Steps)-1].Error
			}
			t.Errorf("case %q: python ended in %q, the case is written for %q (%v)", c.Name, end,
				cases[i].wantEnd, last)
		}

		t.Run(c.Name, func(t *testing.T) {
			golang := normalizeAuthSteps(runAuthGoCase(t, pool, c), c)
			py := normalizeAuthSteps(pySteps, c)
			before := tally.mismatches
			compareCase(t, tally, c, py, golang)
			if tally.mismatches > before {
				logPeopleDifferences(t, c, py, golang)
			}
		})
	}
	for _, method := range authMethods {
		if returned[method] == 0 {
			t.Errorf("no case reaches a normal return of %s", method)
		}
	}
	for b, branch := range authBranches {
		if reached[b] == 0 {
			t.Errorf("no case makes python issue, from %s: %s...%s (without %q)", branch.call, branch.prefix,
				branch.contains, branch.absent)
		}
	}
	t.Logf("auth repo oracle: %d cases, %d steps (%d route steps; %d results, %d refusals), %d statements, "+
		"%d probe rows of which %d whole-table rows, %d generated ids bound, %d branches reached, %d mismatches",
		tally.cases, tally.steps, routeSteps, tally.results, tally.errors, tally.statements, tally.probeRows,
		dumpRows, tally.generated, len(authBranches), tally.mismatches)
}

// ---------------------------------------------------------------------------
// What the oracle cannot see: the bound arguments
// ---------------------------------------------------------------------------

// argRecorder is recorder plus the arguments, which the oracle does not compare
// because the Python driver's statement log does not carry them.
type argRecorder struct {
	Querier
	calls []struct {
		sql  string
		args []any
	}
}

func (r *argRecorder) note(sql string, args []any) {
	r.calls = append(r.calls, struct {
		sql  string
		args []any
	}{sql, args})
}

func (r *argRecorder) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	r.note(sql, args)
	return r.Querier.Exec(ctx, sql, args...)
}

func (r *argRecorder) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	r.note(sql, args)
	return r.Querier.Query(ctx, sql, args...)
}

func (r *argRecorder) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	r.note(sql, args)
	return r.Querier.QueryRow(ctx, sql, args...)
}

// TestAuthRepoBindsWhatPythonBinds pins the argument values the differential
// oracle structurally cannot see.
//
// The oracle compares statement TEXT and results. A bound value that changes
// neither is invisible to it, and this wave has two: the `LIMIT 1` of the two
// reads that sit on a unique column (`token_digest`), where binding 2 would
// read the same one row. The expectations below are read off the Python source
// (`.limit(1)` in get_account_session_by_digest, `.limit(1)` in
// get_outing_invite_by_digest), not off the oracle, and they are stated here so
// nobody mistakes the oracle's green for coverage it does not have.
func TestAuthRepoBindsWhatPythonBinds(t *testing.T) {
	image := os.Getenv("CORE_PYTHON_IMAGE")
	if image == "" {
		t.Skip("CORE_PYTHON_IMAGE not set; see the comment at the top of auth_repo_oracle_postgres_test.go")
	}
	pool, _ := migratedOracleSchema(t, image, "auth_repo_binds_")
	authKey = []byte(newAuthKey(t))
	t.Cleanup(func() { authKey = nil })
	w := newAuthWorld(authKey)
	tx, err := pool.Begin(bg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(bg) }()
	for _, sql := range w.sql {
		if _, err := tx.Exec(bg, sql); err != nil {
			t.Fatalf("seed: %v\n%s", err, sql)
		}
	}

	at := argInstant(authNow)
	for _, probe := range []struct {
		name string
		run  func(Repository) error
		want []any
	}{
		{"get_account_session_by_digest binds the digest and LIMIT 1",
			func(r Repository) error {
				_, err := r.GetAccountSessionByDigest(bg, digestOf(w.bearerAn))
				return err
			},
			[]any{digestOf(w.bearerAn), 1}},
		{"get_outing_invite_by_digest binds the digest and LIMIT 1",
			func(r Repository) error {
				_, err := r.GetOutingInviteByDigest(bg, digestOf(w.tokChi))
				return err
			},
			[]any{digestOf(w.tokChi), 1}},
		{"list_account_sessions binds the person and the caller's clock",
			func(r Repository) error {
				_, err := r.ListAccountSessions(bg, w.an, at)
				return err
			},
			[]any{w.an, pythonInstant(at)}},
		{"recent_otp_challenges binds the digest and the window start",
			func(r Repository) error {
				_, err := r.RecentOtpChallenges(bg, w.digestA, at.Add(-otpWindowSeconds*time.Second))
				return err
			},
			[]any{w.digestA, pythonInstant(at.Add(-otpWindowSeconds * time.Second))}},
	} {
		t.Run(probe.name, func(t *testing.T) {
			rec := &argRecorder{Querier: tx}
			if err := probe.run(Repository{Q: rec}); err != nil {
				t.Fatal(err)
			}
			if len(rec.calls) != 1 {
				t.Fatalf("issued %d statements, expected one", len(rec.calls))
			}
			if got := fmt.Sprintf("%v", rec.calls[0].args); got != fmt.Sprintf("%v", probe.want) {
				t.Fatalf("bound %s, expected %v", got, probe.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Two requests at once (Go only)
// ---------------------------------------------------------------------------

// TestNamedInviteSecretSerialisesRequests is the race the bootstrap door is
// built on: two requests arriving with the same stolen secret must not both
// mint a session. The Python side of the same guarantee is the FOR UPDATE
// statement text the oracle already pins.
func TestNamedInviteSecretSerialisesRequests(t *testing.T) {
	image := os.Getenv("CORE_PYTHON_IMAGE")
	if image == "" {
		t.Skip("CORE_PYTHON_IMAGE not set; see the comment at the top of auth_repo_oracle_postgres_test.go")
	}
	pool, _ := migratedOracleSchema(t, image, "auth_locks_")
	authKey = []byte(newAuthKey(t))
	t.Cleanup(func() { authKey = nil })
	w := newAuthWorld(authKey)
	for _, sql := range w.sql {
		if _, err := pool.Exec(bg, sql); err != nil {
			t.Fatalf("seed: %v\n%s", err, sql)
		}
	}
	now := argInstant(authNow)
	begin := func(t *testing.T) pgx.Tx {
		t.Helper()
		tx, err := pool.Begin(bg)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
		return tx
	}

	first := begin(t)
	if _, err := (Repository{Q: first}).ConsumeNamedInviteSecret(bg, w.invChi, digestOf(w.tokChi),
		w.chi, now); err != nil {
		t.Fatal(err)
	}
	second := begin(t)
	done := make(chan error, 1)
	go func() {
		_, err := (Repository{Q: second}).ConsumeNamedInviteSecret(bg, w.invChi, digestOf(w.tokChi),
			w.chi, now)
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatalf("the second redemption did not wait for the first: %v", err)
	case <-time.After(300 * time.Millisecond):
	}
	if err := first.Commit(bg); err != nil {
		t.Fatal(err)
	}
	var conflict *Conflict
	if err := <-done; !errors.As(err, &conflict) || conflict.Code != "OUTING_INVITE_NOT_REDEEMABLE" {
		t.Fatalf("the second redemption, after the first committed: %v", err)
	}
	var digest []byte
	if err := pool.QueryRow(bg, `SELECT token_digest FROM outing_invites WHERE id = $1::UUID`,
		w.invChi).Scan(&digest); err != nil {
		t.Fatal(err)
	}
	if digest != nil {
		t.Fatal("the secret survived its own redemption")
	}
}

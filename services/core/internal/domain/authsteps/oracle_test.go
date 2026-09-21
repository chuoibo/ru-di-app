package authsteps

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"mobile/services/core/internal/domain/otp"
	"mobile/services/core/internal/domain/peoplesteps"
	"mobile/services/core/internal/domain/permissions"
	"mobile/services/core/internal/identity"
	"mobile/services/core/internal/oracletest"
)

// testdata/python_auth_steps*.json is rendered by
// scripts/render_domain_w9_goldens.py by running the real session and OTP
// methods of app.api.service over a recording stub repository, with the clock,
// the identity derivations and every secret pinned, in the parity API image.
// Every case is replayed here through a recording Store and the same four
// recording seams: the answer (or the ApiProblem, or the exception that is a
// 500) and every crossed seam with its arguments must match.

// methods is every ported service method, in the order of the script's CALLERS.
var methods = []string{
	"bootstrap_session_from_invite", "list_account_sessions", "revoke_session_token",
	"revoke_account_session", "request_otp", "verify_otp", "login_with_google",
}

// problemCodes is every ApiProblem these seven doors can answer.
var problemCodes = []string{
	"invite_not_found", "permission_denied", "session_not_found",
	"phone_required", "phone_not_mobile", "identity_key_missing",
	"otp_resend_too_soon", "otp_too_many_requests", "sms_unavailable",
	"code_required", "otp_challenge_not_found", "otp_code_invalid",
	"otp_too_many_attempts", "google_not_configured", "id_token_required",
	"google_token_invalid",
}

// raisedTypes is every exception class the committed corpus must show ending a
// request as a 500.
var raisedTypes = []string{"RepositoryConflict", "OverflowError", "UnicodeEncodeError"}

// --- the harness ------------------------------------------------------------

type harness struct {
	ids      map[string]string
	names    map[string]string
	tokens   map[string]string
	phones   map[string]string
	defaults map[string]any
	base     time.Time
}

func newHarness(t testing.TB, constants map[string]any) *harness {
	t.Helper()
	h := &harness{
		ids: map[string]string{}, names: map[string]string{},
		tokens: map[string]string{}, phones: map[string]string{},
		base: time.Date(2030, 9, 18, 5, 0, 0, 0, time.UTC),
	}
	for _, spec := range []struct {
		key  string
		into map[string]string
		back bool
	}{
		{"aliases", h.ids, true},
		{"tokens", h.tokens, false},
		{"phones", h.phones, false},
	} {
		rows, err := oracletest.List(constants[spec.key])
		if err != nil {
			t.Fatal(err)
		}
		for _, raw := range rows {
			pair, err := oracletest.Strings(raw)
			if err != nil || len(pair) != 2 {
				t.Fatalf("%s %v: %v", spec.key, raw, err)
			}
			spec.into[pair[0]] = pair[1]
			if spec.back {
				h.names[pair[1]] = pair[0]
			}
		}
	}
	defaults, err := oracletest.Row(constants["world_defaults"])
	if err != nil {
		t.Fatal(err)
	}
	h.defaults = defaults
	return h
}

// id is an alias as the service sees it: the canonical uuid string.
func (h *harness) id(value any) (string, error) {
	name, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("%v (%T) is not an alias", value, value)
	}
	if id, found := h.ids[name]; found {
		return id, nil
	}
	return name, nil
}

func (h *harness) optionalID(value any) (*string, error) {
	if value == nil {
		return nil, nil
	}
	id, err := h.id(value)
	return &id, err
}

func (h *harness) name(id string) any {
	if name, found := h.names[id]; found {
		return name
	}
	return id
}

func (h *harness) optionalName(id *string) any {
	if id == nil {
		return nil
	}
	return h.name(*id)
}

// alias is the short name of an id, or the id when nothing names it. The
// world's tables are keyed by the short name.
func (h *harness) alias(id string) string {
	if name, found := h.names[id]; found {
		return name
	}
	return id
}

func (h *harness) world(value any) (map[string]any, error) {
	overrides, err := oracletest.Row(value)
	if err != nil {
		return nil, err
	}
	merged := make(map[string]any, len(h.defaults)+len(overrides))
	for key, item := range h.defaults {
		merged[key] = item
	}
	for key, item := range overrides {
		merged[key] = item
	}
	return merged, nil
}

// digestFor is the script's `digest_for`: a stored digest as a case spells it.
func (h *harness) digestFor(spec any) ([]byte, error) {
	if text, ok := spec.(string); ok && strings.HasPrefix(text, "@") {
		phone, found := h.phones[text[1:]]
		if !found {
			return nil, fmt.Errorf("no telephone %q", text)
		}
		canonical, ok := identity.CanonicalMobile(phone)
		if !ok {
			return nil, fmt.Errorf("%q is not a mobile", text)
		}
		return phoneDigestOf(canonical), nil
	}
	if pair, ok := spec.([]any); ok && len(pair) == 2 {
		id, err := h.id(pair[0])
		if err != nil {
			return nil, err
		}
		code, err := oracletest.Str(pair[1])
		if err != nil {
			return nil, err
		}
		return codeDigestOf(id, code), nil
	}
	text, err := oracletest.Str(spec)
	if err != nil {
		return nil, err
	}
	return standIn("other|"+text, 4), nil
}

// subjectOf is the script's `subject_of`.
func (h *harness) subjectOf(spec string) (string, error) {
	provider, rest, found := strings.Cut(spec, "|")
	if !found || provider != "phone" {
		return spec, nil
	}
	digest, err := h.digestFor(rest)
	if err != nil {
		return "", err
	}
	return "phone|" + hexOf(digest), nil
}

// --- decoding a world -------------------------------------------------------

func optionalInstant(value any) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}
	moment, err := oracletest.Instant(value)
	return &moment, err
}

func fields(value any, n int) ([]any, error) {
	items, err := oracletest.List(value)
	if err != nil || len(items) != n {
		return nil, fmt.Errorf("%v is not a record of %d fields", value, n)
	}
	return items, nil
}

func (h *harness) inviteOf(value any) (*Invite, error) {
	f, err := fields(value, 10)
	if err != nil {
		return nil, err
	}
	row := &Invite{}
	if row.ID, err = h.id(f[0]); err != nil {
		return nil, err
	}
	if row.OutingID, err = h.id(f[1]); err != nil {
		return nil, err
	}
	if row.Source, err = oracletest.Str(f[2]); err != nil {
		return nil, err
	}
	if row.InvitedPersonID, err = h.optionalID(f[3]); err != nil {
		return nil, err
	}
	if row.InvitedByID, err = h.id(f[4]); err != nil {
		return nil, err
	}
	if row.AcceptedAt, err = optionalInstant(f[5]); err != nil {
		return nil, err
	}
	if row.AcceptedByID, err = h.optionalID(f[6]); err != nil {
		return nil, err
	}
	if row.CreatedAt, err = oracletest.Instant(f[7]); err != nil {
		return nil, err
	}
	if row.ExpiresAt, err = oracletest.Instant(f[8]); err != nil {
		return nil, err
	}
	if row.RevokedAt, err = optionalInstant(f[9]); err != nil {
		return nil, err
	}
	return row, nil
}

func (h *harness) challengeOf(value any) (*OtpChallenge, error) {
	f, err := fields(value, 7)
	if err != nil {
		return nil, err
	}
	row := &OtpChallenge{}
	if row.ID, err = h.id(f[0]); err != nil {
		return nil, err
	}
	if row.PhoneDigest, err = h.digestFor(f[1]); err != nil {
		return nil, err
	}
	if row.CodeDigest, err = h.digestFor(f[2]); err != nil {
		return nil, err
	}
	if row.CreatedAt, err = oracletest.Instant(f[3]); err != nil {
		return nil, err
	}
	if row.ExpiresAt, err = oracletest.Instant(f[4]); err != nil {
		return nil, err
	}
	if row.Attempts, err = oracletest.Int64(f[5]); err != nil {
		return nil, err
	}
	if row.ConsumedAt, err = optionalInstant(f[6]); err != nil {
		return nil, err
	}
	return row, nil
}

func (h *harness) sessionOf(value any) (*AccountSession, error) {
	f, err := fields(value, 7)
	if err != nil {
		return nil, err
	}
	row := &AccountSession{}
	if row.ID, err = h.id(f[0]); err != nil {
		return nil, err
	}
	if row.PersonID, err = h.id(f[1]); err != nil {
		return nil, err
	}
	if row.IssuedFromInviteID, err = h.optionalID(f[2]); err != nil {
		return nil, err
	}
	if row.IssuedVia, err = oracletest.Str(f[3]); err != nil {
		return nil, err
	}
	if row.CreatedAt, err = oracletest.Instant(f[4]); err != nil {
		return nil, err
	}
	if row.ExpiresAt, err = oracletest.Instant(f[5]); err != nil {
		return nil, err
	}
	if row.RevokedAt, err = optionalInstant(f[6]); err != nil {
		return nil, err
	}
	return row, nil
}

func (h *harness) summaryOf(value any) (SummaryRecord, error) {
	f, err := fields(value, 13)
	if err != nil {
		return SummaryRecord{}, err
	}
	row := SummaryRecord{}
	if row.ID, err = h.id(f[0]); err != nil {
		return row, err
	}
	if row.DisplayName, err = oracletest.Str(f[1]); err != nil {
		return row, err
	}
	if row.MemberCount, err = oracletest.Int64(f[2]); err != nil {
		return row, err
	}
	if row.MyRole, err = oracletest.Str(f[3]); err != nil {
		return row, err
	}
	if row.MyState, err = oracletest.Str(f[4]); err != nil {
		return row, err
	}
	if row.MembershipID, err = h.id(f[5]); err != nil {
		return row, err
	}
	if row.JoinedAt, err = optionalInstant(f[6]); err != nil {
		return row, err
	}
	if f[7] != nil {
		last, err := fields(f[7], 6)
		if err != nil {
			return row, err
		}
		message := &peoplesteps.LastMessage{}
		if message.ID, err = h.id(last[0]); err != nil {
			return row, err
		}
		if message.Kind, err = oracletest.Str(last[1]); err != nil {
			return row, err
		}
		if message.Preview, err = oracletest.Str(last[2]); err != nil {
			return row, err
		}
		if message.AuthorID, err = h.optionalID(last[3]); err != nil {
			return row, err
		}
		if message.AuthorDisplayName, err = oracletest.OptionalString(last[4]); err != nil {
			return row, err
		}
		if message.CreatedAt, err = oracletest.Instant(last[5]); err != nil {
			return row, err
		}
		row.LastMessage = message
	}
	if row.UnreadCount, err = oracletest.Int64(f[8]); err != nil {
		return row, err
	}
	if row.Theme, err = oracletest.Str(f[9]); err != nil {
		return row, err
	}
	if row.Kind, err = oracletest.Str(f[10]); err != nil {
		return row, err
	}
	if row.CounterpartID, err = h.optionalID(f[11]); err != nil {
		return row, err
	}
	if row.CounterpartDisplayName, err = oracletest.OptionalString(f[12]); err != nil {
		return row, err
	}
	return row, nil
}

func (h *harness) newStub(value any, now time.Time) (*stub, error) {
	world, err := h.world(value)
	if err != nil {
		return nil, err
	}
	s := &stub{
		h: h, calls: []any{},
		people:    map[string][2]any{},
		edges:     map[string]*FriendEdge{},
		invites:   map[string]*Invite{},
		outings:   map[string]string{},
		ids:       map[string]string{},
		byDigest:  map[string]*AccountSession{},
		conflicts: map[string][]any{},
		queues:    map[string][]any{},
	}
	if s.key, err = oracletest.Str(world["key"]); err != nil {
		return nil, err
	}
	people, err := oracletest.Row(world["people"])
	if err != nil {
		return nil, err
	}
	for alias, raw := range people {
		pair, err := fields(raw, 2)
		if err != nil {
			return nil, err
		}
		name, err := oracletest.Str(pair[0])
		if err != nil {
			return nil, err
		}
		deleted, err := optionalInstant(pair[1])
		if err != nil {
			return nil, err
		}
		row := [2]any{name, nil}
		if deleted != nil {
			row[1] = *deleted
		}
		s.people[alias] = row
	}
	summaries, err := oracletest.List(world["summaries"])
	if err != nil {
		return nil, err
	}
	for _, raw := range summaries {
		row, err := h.summaryOf(raw)
		if err != nil {
			return nil, err
		}
		s.summaries = append(s.summaries, row)
	}
	edges, err := oracletest.Row(world["edges"])
	if err != nil {
		return nil, err
	}
	for key, raw := range edges {
		pair, err := fields(raw, 2)
		if err != nil {
			return nil, err
		}
		state, err := oracletest.Str(pair[0])
		if err != nil {
			return nil, err
		}
		decided, err := h.optionalID(pair[1])
		if err != nil {
			return nil, err
		}
		s.edges[key] = &FriendEdge{State: state, DecidedByID: decided}
	}
	invites, err := oracletest.Row(world["invites"])
	if err != nil {
		return nil, err
	}
	for name, raw := range invites {
		row, err := h.inviteOf(raw)
		if err != nil {
			return nil, err
		}
		s.invites[hexOf(tokenDigestOf(h.tokens[name]))] = row
	}
	outings, err := oracletest.Row(world["outings"])
	if err != nil {
		return nil, err
	}
	for outing, raw := range outings {
		context, err := oracletest.Str(raw)
		if err != nil {
			return nil, err
		}
		s.outings[outing] = context
	}
	member, err := oracletest.Strings(world["membership"])
	if err != nil || len(member) != 2 {
		return nil, fmt.Errorf("membership %v: %v", world["membership"], err)
	}
	s.member = [2]string{member[0], member[1]}
	recent, err := oracletest.List(world["recent"])
	if err != nil {
		return nil, err
	}
	for _, raw := range recent {
		moment, err := oracletest.Instant(raw)
		if err != nil {
			return nil, err
		}
		s.recent = append(s.recent, moment)
	}
	if world["challenge"] != nil {
		if s.challenge, err = h.challengeOf(world["challenge"]); err != nil {
			return nil, err
		}
	}
	ids, err := oracletest.Row(world["identities"])
	if err != nil {
		return nil, err
	}
	for spec, raw := range ids {
		key, err := h.subjectOf(spec)
		if err != nil {
			return nil, err
		}
		alias, err := oracletest.Str(raw)
		if err != nil {
			return nil, err
		}
		s.ids[key] = alias
	}
	sessions, err := oracletest.List(world["sessions"])
	if err != nil {
		return nil, err
	}
	for _, raw := range sessions {
		row, err := h.sessionOf(raw)
		if err != nil {
			return nil, err
		}
		s.sessions = append(s.sessions, *row)
	}
	byDigest, err := oracletest.Row(world["by_digest"])
	if err != nil {
		return nil, err
	}
	for name, raw := range byDigest {
		row, err := h.sessionOf(raw)
		if err != nil {
			return nil, err
		}
		s.byDigest[hexOf(tokenDigestOf(h.tokens[name]))] = row
	}
	if world["session"] != nil {
		if s.session, err = h.sessionOf(world["session"]); err != nil {
			return nil, err
		}
	}
	conflicts, err := oracletest.Row(world["conflicts"])
	if err != nil {
		return nil, err
	}
	for method, raw := range conflicts {
		codes, err := oracletest.List(raw)
		if err != nil {
			return nil, err
		}
		s.conflicts[method] = codes
	}
	s.sms = world["sms"]
	if world["google"] != nil {
		if s.google, err = oracletest.List(world["google"]); err != nil {
			return nil, err
		}
	}
	for _, key := range []string{"uuids", "derived", "tokens", "draws"} {
		queue, err := oracletest.List(world[key])
		if err != nil {
			return nil, err
		}
		s.queues[key] = queue
	}
	if world["created_challenge"] != nil {
		alias, err := oracletest.Str(world["created_challenge"])
		if err != nil {
			return nil, err
		}
		s.createdChallenge = &alias
	}
	created, err := fields(world["created_session"], 2)
	if err != nil {
		return nil, err
	}
	s.createdSession = [2]any{created[0], created[1]}
	if world["debug_code"] != nil {
		code, err := oracletest.Str(world["debug_code"])
		if err != nil {
			return nil, err
		}
		s.debugCode = &code
	}
	return s, nil
}

// --- rendering the answer ---------------------------------------------------

func (h *harness) sessionWire(view SessionView) any {
	contexts := make([]any, 0, len(view.Contexts))
	for _, summary := range view.Contexts {
		var last any
		if summary.LastMessage != nil {
			last = map[string]any{
				"id":                  h.name(summary.LastMessage.ID),
				"kind":                summary.LastMessage.Kind,
				"preview":             summary.LastMessage.Preview,
				"author_id":           h.optionalName(summary.LastMessage.AuthorID),
				"author_display_name": optionalText(summary.LastMessage.AuthorDisplayName),
				"created_at":          iso(summary.LastMessage.CreatedAt),
			}
		}
		var counterpart any
		if summary.Counterpart != nil {
			counterpart = map[string]any{
				"id":           h.name(summary.Counterpart.ID),
				"display_name": summary.Counterpart.DisplayName,
			}
		}
		contexts = append(contexts, map[string]any{
			"id":            h.name(summary.ID),
			"display_name":  summary.DisplayName,
			"member_count":  summary.MemberCount,
			"my_role":       summary.MyRole,
			"my_state":      summary.MyState,
			"membership_id": h.name(summary.MembershipID),
			"joined_at":     optionalISO(summary.JoinedAt),
			"last_message":  last,
			"unread_count":  summary.UnreadCount,
			"theme":         summary.Theme,
			"kind":          summary.Kind,
			"counterpart":   counterpart,
			"unavailable":   summary.Unavailable,
		})
	}
	return map[string]any{
		"token":            view.Token,
		"person_id":        h.name(view.PersonID),
		"expires_at":       iso(view.ExpiresAt),
		"issued_via":       view.IssuedVia,
		"is_new_person":    view.IsNewPerson,
		"profile":          map[string]any{"display_name": view.Profile.DisplayName},
		"contexts":         contexts,
		"context_id":       h.optionalName(view.ContextID),
		"membership_state": optionalText(view.MembershipState),
		"membership_id":    h.optionalName(view.MembershipID),
	}
}

func optionalText(text *string) any {
	if text == nil {
		return nil
	}
	return *text
}

func (h *harness) sessionListWire(view SessionListView) any {
	rows := make([]any, 0, len(view.Sessions))
	for _, row := range view.Sessions {
		rows = append(rows, map[string]any{
			"id":         h.name(row.ID),
			"issued_via": row.IssuedVia,
			"created_at": iso(row.CreatedAt),
			"expires_at": iso(row.ExpiresAt),
			"current":    row.Current,
		})
	}
	return map[string]any{"sessions": rows}
}

func (h *harness) otpRequestWire(view OtpRequestView) any {
	return map[string]any{
		"challenge_id":         h.name(view.ChallengeID),
		"expires_in_seconds":   view.ExpiresInSeconds,
		"resend_after_seconds": view.ResendAfterSeconds,
	}
}

// classify turns a Go error into the `problem` or the `raised` the script
// records, or reports that the port answered with something Python never does.
func classify(err error) (problem, raised map[string]any, unexpected error) {
	var refused *Refusal
	if errors.As(err, &refused) {
		return map[string]any{
			"status": int64(refused.Status),
			"code":   refused.Code,
			"detail": refused.Detail,
		}, nil, nil
	}
	var conflict *Conflict
	if errors.As(err, &conflict) {
		return nil, raise("RepositoryConflict", conflict.Code, conflict.Code), nil
	}
	var denied *permissions.Error
	if errors.As(err, &denied) {
		return nil, raise("PermissionError_", denied.Code, denied.Code), nil
	}
	var overflow *otp.OverflowError
	if errors.As(err, &overflow) {
		return nil, raise("OverflowError", nil, overflow.Message), nil
	}
	var badText *identity.UnicodeEncodeError
	if errors.As(err, &badText) {
		return nil, raise("UnicodeEncodeError", nil, badText.Error()), nil
	}
	var missing *KeyMissing
	if errors.As(err, &missing) {
		return nil, raise("PersonIdKeyMissing", nil, missing.Message), nil
	}
	var invariant *Invariant
	if errors.As(err, &invariant) {
		return nil, raise("AssertionError", nil, ""), nil
	}
	return nil, nil, err
}

func raise(kind string, code any, message string) map[string]any {
	return map[string]any{"type": kind, "code": code, "message": message}
}

// --- replay -----------------------------------------------------------------

// phoneArg is the script's `phone_arg`: "@NAME" is that telephone, anything
// else is itself, including the values that are not strings at all.
func (h *harness) phoneArg(spec any) any {
	if text, ok := spec.(string); ok && strings.HasPrefix(text, "@") {
		if phone, found := h.phones[text[1:]]; found {
			return phone
		}
	}
	return spec
}

func (h *harness) actorOf(value any) (Actor, error) {
	pair, err := oracletest.List(value)
	if err != nil || len(pair) != 2 {
		return Actor{}, fmt.Errorf("actor %v", value)
	}
	id, err := h.id(pair[0])
	if err != nil {
		return Actor{}, err
	}
	roles, err := oracletest.Strings(pair[1])
	if err != nil {
		return Actor{}, err
	}
	return Actor{ID: id, Roles: roles}, nil
}

func (h *harness) replay(c oracletest.Case, args map[string]any) (any, error) {
	now, err := oracletest.Instant(args["now"])
	if err != nil {
		return nil, oracletest.Decode(err)
	}
	actor, err := h.actorOf(args["actor"])
	if err != nil {
		return nil, oracletest.Decode(err)
	}
	req, err := oracletest.Row(args["req"])
	if err != nil {
		return nil, oracletest.Decode(err)
	}
	s, err := h.newStub(args["world"], now)
	if err != nil {
		return nil, oracletest.Decode(err)
	}
	mint := secretsSeam{s}
	who := identitySeam{s}

	var response any
	var failed error
	switch c.Fn {
	case "bootstrap_session_from_invite":
		name, err := oracletest.Str(req["token"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		view, callErr := BootstrapSessionFromInvite(s, mint, h.tokens[name], now)
		failed = callErr
		if callErr == nil {
			response = h.sessionWire(view)
		}

	case "list_account_sessions":
		var token *string
		if req["token"] != nil {
			name, err := oracletest.Str(req["token"])
			if err != nil {
				return nil, oracletest.Decode(err)
			}
			raw := h.tokens[name]
			token = &raw
		}
		view, callErr := ListAccountSessions(s, mint, actor, token, now)
		failed = callErr
		if callErr == nil {
			response = h.sessionListWire(view)
		}

	case "revoke_session_token":
		name, err := oracletest.Str(req["token"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		failed = RevokeSessionToken(s, mint, h.tokens[name], now)

	case "revoke_account_session":
		id, err := h.id(req["session_id"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		failed = RevokeAccountSession(s, id, actor, now)

	case "request_otp":
		view, callErr := RequestOTP(s, who, mint, smsSeam{s}, h.phoneArg(req["phone"]), s.debugCode, now)
		failed = callErr
		if callErr == nil {
			response = h.otpRequestWire(view)
		}

	case "verify_otp":
		id, err := h.id(req["challenge_id"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		view, callErr := VerifyOTP(s, who, mint, id, h.phoneArg(req["phone"]), req["code"], now)
		failed = callErr
		if callErr == nil {
			response = h.sessionWire(view)
		}

	case "login_with_google":
		var verifier GoogleVerifier
		if s.google != nil {
			verifier = googleSeam{s}
		}
		view, callErr := LoginWithGoogle(s, mint, verifier, req["id_token"], now)
		failed = callErr
		if callErr == nil {
			response = h.sessionWire(view)
		}

	default:
		return nil, oracletest.Decode(fmt.Errorf("no replay for %s", c.Fn))
	}

	problem, raised, unexpected := classify(failed)
	if unexpected != nil {
		return nil, unexpected
	}
	out := map[string]any{
		"calls":    s.calls,
		"problem":  nil,
		"raised":   nil,
		"response": response,
	}
	if problem != nil {
		out["problem"] = problem
	}
	if raised != nil {
		out["raised"] = raised
	}
	return out, nil
}

// --- the gate ---------------------------------------------------------------

func asRefusal(error) (class, code string, ok bool) { return "", "", false }

func checkSteps(t *testing.T, h *harness, files []oracletest.File, committed bool) {
	t.Helper()
	report := oracletest.Agree(t, files, "auth_steps", h.replay, asRefusal)
	if !committed {
		return
	}
	for _, name := range methods {
		if report.ByFn[name] == nil {
			t.Errorf("no case calls %s", name)
		}
	}
	for _, code := range problemCodes {
		if report.Codes[code] == 0 {
			t.Errorf("no committed case answers %q", code)
		}
	}
	seen := map[string]bool{}
	for _, file := range files {
		for _, c := range file.Cases {
			body, ok := c.Result["ok"].(map[string]any)
			if !ok {
				continue
			}
			if row, ok := body["raised"].(map[string]any); ok {
				kind, err := oracletest.Text(row["type"])
				if err == nil {
					seen[kind] = true
				}
			}
		}
	}
	for _, kind := range raisedTypes {
		if !seen[kind] {
			t.Errorf("no committed case ends with %s", kind)
		}
	}
}

func TestAuthStepsMatchPython(t *testing.T) {
	files := oracletest.Load(t, "testdata/python_auth_steps*.json")
	constants := oracletest.Constants(t, files, "auth_steps")
	h := newHarness(t, constants)
	ttl, err := oracletest.Int64(constants["session_ttl_seconds"])
	if err != nil || ttl != int64(AccountSessionTTL.Seconds()) {
		t.Errorf("session ttl: Python %v, Go %v (%v)", constants["session_ttl_seconds"], AccountSessionTTL, err)
	}
	name, err := oracletest.Str(constants["new_person_name"])
	if err != nil || name != NewPersonName {
		t.Errorf("NEW_PERSON_NAME: Python %v, Go %q (%v)", constants["new_person_name"], NewPersonName, err)
	}
	names, err := oracletest.Strings(constants["methods"])
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != len(methods) {
		t.Errorf("Python ports %d methods, this test names %d", len(names), len(methods))
	}
	checkSteps(t, h, files, true)
}

package pairsteps

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"strings"
	"testing"
	"time"

	"mobile/services/core/internal/domain/pairnotebook"
	"mobile/services/core/internal/domain/pairpaper"
	"mobile/services/core/internal/domain/permissions"
	"mobile/services/core/internal/oracletest"
	"mobile/services/core/internal/pyjson"
)

// testdata/python_pair_steps*.json is rendered by
// scripts/render_domain_w8_goldens.py by running the real pair methods of
// app.api.service over a recording stub repository, with the clock pinned to
// each case's `now`, in the parity API image. Every case is replayed here
// through a recording Store: the answer (or the ApiProblem, or the exception
// that is a 500) and every repository call with its arguments must match.

// methods is every ported service method, in the order of the script's
// CALLERS.
var methods = []string{
	"pair_notebook", "propose_pair_consent", "grant_pair_consent", "revoke_pair_consent",
	"put_pair_constraint", "delete_pair_constraint", "preview_close_pair_notebook", "close_pair_notebook",
	"list_pair_papers", "draft_pair_paper", "pair_paper", "edit_pair_draft", "send_pair_paper",
	"mark_pair_paper_viewed", "respond_pair_paper", "withdraw_pair_paper", "skip_pair_week",
	"record_pair_outing_done", "keep_pair_paper_line",
}

// problemCodes is every ApiProblem the pair methods can answer through the
// service; paper_needs_recorder, paper_event_unknown and
// paper_draft_needs_date cannot be reached from a route.
var problemCodes = []string{
	"notebook_not_found", "permission_denied", "consent_missing", "cycle_not_active",
	"consent_proposal_not_found", "consent_proposal_expired", "couple_slot_taken", "consent_purpose_unknown",
	"constraint_kind_unknown", "notebook_revision_stale", "paper_not_found", "paper_wrong_state",
	"paper_version_stale", "paper_self_response", "paper_not_withdrawable", "paper_expired",
	"paper_frozen", "paper_outing_exists",
}

// raisedTypes is every exception class the corpus must show ending a request.
var raisedTypes = []string{"AssertionError", "PermissionError_", "RepositoryConflict"}

type harness struct {
	ids   map[string]string
	names map[string]string
}

func newHarness(t testing.TB, constants map[string]any) *harness {
	t.Helper()
	rows, err := oracletest.List(constants["aliases"])
	if err != nil {
		t.Fatal(err)
	}
	h := &harness{ids: map[string]string{}, names: map[string]string{}}
	for _, raw := range rows {
		pair, err := oracletest.Strings(raw)
		if err != nil || len(pair) != 2 {
			t.Fatalf("alias %v", raw)
		}
		h.ids[pair[0]] = pair[1]
		h.names[pair[1]] = pair[0]
	}
	return h
}

func (h *harness) id(value any) (string, error) {
	name, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("%v (%T) is not an alias", value, value)
	}
	id, ok := h.ids[name]
	if !ok {
		return "", fmt.Errorf("no alias %q", name)
	}
	return id, nil
}

func (h *harness) optionalID(value any) (*string, error) {
	if value == nil {
		return nil, nil
	}
	id, err := h.id(value)
	return &id, err
}

func (h *harness) name(id string) any {
	if name, ok := h.names[id]; ok {
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

func (h *harness) nameList(ids []string) []any {
	out := make([]any, len(ids))
	for i, id := range ids {
		out[i] = h.name(id)
	}
	return out
}

func optionalText(text *string) any {
	if text == nil {
		return nil
	}
	return *text
}

func iso(t time.Time) any { return pairpaper.ISOFormat(t) }

func optionalISO(t *time.Time) any {
	if t == nil {
		return nil
	}
	return pairpaper.ISOFormat(*t)
}

func optionalInstant(value any) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}
	t, err := oracletest.Instant(value)
	return &t, err
}

func dateOf(value any) (pairpaper.Date, error) {
	year, month, day, err := oracletest.CivilDate(value)
	return pairpaper.Date{Year: year, Month: month, Day: day}, err
}

func intOf(value any) (int, error) {
	n, err := oracletest.Int64(value)
	return int(n), err
}

// saturated reads a Python int a request carries, clamped to int64.
func saturated(value any) (int64, error) {
	n, err := oracletest.Integer(value)
	if err != nil || n == nil {
		return 0, fmt.Errorf("%v is not an int: %v", value, err)
	}
	switch {
	case n.IsInt64():
		return n.Int64(), nil
	case n.Sign() > 0:
		return math.MaxInt64, nil
	}
	return math.MinInt64, nil
}

// fields reads a positional record of exactly n values.
func fields(value any, n int) ([]any, error) {
	items, err := oracletest.List(value)
	if err != nil || len(items) != n {
		return nil, fmt.Errorf("%v is not a record of %d fields", value, n)
	}
	return items, nil
}

// stored turns the JSON text of a stored value into the model Version.Content
// documents, through the json.loads port.
func stored(value any) (any, error) {
	text, err := oracletest.Str(value)
	if err != nil {
		return nil, err
	}
	loaded, err := pyjson.Loads([]byte(text))
	if err != nil {
		return nil, err
	}
	return fromPyJSON(loaded)
}

func fromPyJSON(value pyjson.Value) (any, error) {
	switch v := value.(type) {
	case nil, pyjson.Null:
		return nil, nil
	case pyjson.Bool:
		return bool(v), nil
	case pyjson.Int:
		return new(big.Int).Set(v.Big()), nil
	case pyjson.Float:
		return float64(v), nil
	case pyjson.String:
		return string(v), nil
	case pyjson.List:
		out := make([]any, len(v))
		for i, item := range v {
			converted, err := fromPyJSON(item)
			if err != nil {
				return nil, err
			}
			out[i] = converted
		}
		return out, nil
	case *pyjson.OrderedMap:
		out := &Object{}
		for key, item := range v.All() {
			converted, err := fromPyJSON(item)
			if err != nil {
				return nil, err
			}
			out.Keys = append(out.Keys, key)
			out.Values = append(out.Values, converted)
		}
		return out, nil
	}
	return nil, fmt.Errorf("unexpected JSON %T", value)
}

func (h *harness) proposalOf(value any) (*Proposal, error) {
	if value == nil {
		return nil, nil
	}
	f, err := fields(value, 6)
	if err != nil {
		return nil, err
	}
	var p Proposal
	if p.ID, err = h.id(f[0]); err != nil {
		return nil, err
	}
	if p.CycleID, err = h.id(f[1]); err != nil {
		return nil, err
	}
	if p.Purpose, err = oracletest.Str(f[2]); err != nil {
		return nil, err
	}
	if p.ProposedByID, err = h.id(f[3]); err != nil {
		return nil, err
	}
	if p.CompletedAt, err = optionalInstant(f[4]); err != nil {
		return nil, err
	}
	if p.ExpiresAt, err = oracletest.Instant(f[5]); err != nil {
		return nil, err
	}
	return &p, nil
}

// notebookOf reads [id, cycle, state, participants, consents, proposals,
// constraints].
func (h *harness) notebookOf(value any) (*Notebook, error) {
	if value == nil {
		return nil, nil
	}
	f, err := fields(value, 7)
	if err != nil {
		return nil, err
	}
	n := &Notebook{Participants: []string{}}
	if n.ID, err = h.id(f[0]); err != nil {
		return nil, err
	}
	if n.CycleID, err = h.optionalID(f[1]); err != nil {
		return nil, err
	}
	if n.CycleState, err = oracletest.OptionalString(f[2]); err != nil {
		return nil, err
	}
	people, err := oracletest.List(f[3])
	if err != nil {
		return nil, err
	}
	for _, person := range people {
		id, err := h.id(person)
		if err != nil {
			return nil, err
		}
		n.Participants = append(n.Participants, id)
	}
	consents, err := oracletest.List(f[4])
	if err != nil {
		return nil, err
	}
	for _, raw := range consents {
		c, err := fields(raw, 5)
		if err != nil {
			return nil, err
		}
		var row Consent
		if row.PersonID, err = h.id(c[0]); err != nil {
			return nil, err
		}
		if row.Purpose, err = oracletest.Str(c[1]); err != nil {
			return nil, err
		}
		if row.GrantedAt, err = optionalInstant(c[2]); err != nil {
			return nil, err
		}
		if row.RevokedAt, err = optionalInstant(c[3]); err != nil {
			return nil, err
		}
		if row.ProposalExpiresAt, err = oracletest.Instant(c[4]); err != nil {
			return nil, err
		}
		n.Consents = append(n.Consents, row)
	}
	proposals, err := oracletest.List(f[5])
	if err != nil {
		return nil, err
	}
	for _, raw := range proposals {
		p, err := h.proposalOf(raw)
		if err != nil {
			return nil, err
		}
		n.Proposals = append(n.Proposals, *p)
	}
	constraints, err := oracletest.List(f[6])
	if err != nil {
		return nil, err
	}
	for _, raw := range constraints {
		c, err := fields(raw, 4)
		if err != nil {
			return nil, err
		}
		var row Constraint
		if row.OwnerID, err = h.id(c[0]); err != nil {
			return nil, err
		}
		if row.Kind, err = oracletest.Str(c[1]); err != nil {
			return nil, err
		}
		if row.Content, err = oracletest.Str(c[2]); err != nil {
			return nil, err
		}
		if row.Version, err = intOf(c[3]); err != nil {
			return nil, err
		}
		n.Constraints = append(n.Constraints, row)
	}
	return n, nil
}

// paperOf reads [id, context, owner, state, version, tuan, expires, outing,
// versions, views, responses, keeps].
func (h *harness) paperOf(value any) (*Paper, error) {
	if value == nil {
		return nil, nil
	}
	f, err := fields(value, 12)
	if err != nil {
		return nil, err
	}
	p := &Paper{}
	if p.ID, err = h.id(f[0]); err != nil {
		return nil, err
	}
	if p.ContextID, err = h.id(f[1]); err != nil {
		return nil, err
	}
	if p.DraftOwnerID, err = h.id(f[2]); err != nil {
		return nil, err
	}
	if p.State, err = oracletest.Str(f[3]); err != nil {
		return nil, err
	}
	if p.CurrentVersion, err = intOf(f[4]); err != nil {
		return nil, err
	}
	if p.Tuan, err = dateOf(f[5]); err != nil {
		return nil, err
	}
	if p.ExpiresAt, err = oracletest.Instant(f[6]); err != nil {
		return nil, err
	}
	if p.OutingID, err = h.optionalID(f[7]); err != nil {
		return nil, err
	}
	versions, err := oracletest.List(f[8])
	if err != nil {
		return nil, err
	}
	for _, raw := range versions {
		v, err := fields(raw, 6)
		if err != nil {
			return nil, err
		}
		var row Version
		if row.Version, err = intOf(v[0]); err != nil {
			return nil, err
		}
		if row.Content, err = stored(v[1]); err != nil {
			return nil, err
		}
		if row.LyDo, err = oracletest.OptionalString(v[2]); err != nil {
			return nil, err
		}
		if row.AuthorType, err = oracletest.Str(v[3]); err != nil {
			return nil, err
		}
		if row.SentAt, err = optionalInstant(v[4]); err != nil {
			return nil, err
		}
		if row.SentBy, err = h.optionalID(v[5]); err != nil {
			return nil, err
		}
		p.Versions = append(p.Versions, row)
	}
	views, err := oracletest.List(f[9])
	if err != nil {
		return nil, err
	}
	for _, raw := range views {
		v, err := fields(raw, 3)
		if err != nil {
			return nil, err
		}
		var row View
		if row.Version, err = intOf(v[0]); err != nil {
			return nil, err
		}
		if row.PersonID, err = h.id(v[1]); err != nil {
			return nil, err
		}
		if row.SeenAt, err = oracletest.Instant(v[2]); err != nil {
			return nil, err
		}
		p.Views = append(p.Views, row)
	}
	responses, err := oracletest.List(f[10])
	if err != nil {
		return nil, err
	}
	for _, raw := range responses {
		v, err := fields(raw, 3)
		if err != nil {
			return nil, err
		}
		var row Response
		if row.Version, err = intOf(v[0]); err != nil {
			return nil, err
		}
		if row.PersonID, err = h.id(v[1]); err != nil {
			return nil, err
		}
		if row.Kind, err = oracletest.Str(v[2]); err != nil {
			return nil, err
		}
		p.Responses = append(p.Responses, row)
	}
	keeps, err := oracletest.List(f[11])
	if err != nil {
		return nil, err
	}
	for _, raw := range keeps {
		v, err := fields(raw, 3)
		if err != nil {
			return nil, err
		}
		var row Keep
		if row.ID, err = h.id(v[0]); err != nil {
			return nil, err
		}
		if row.Line, err = oracletest.Str(v[1]); err != nil {
			return nil, err
		}
		if row.CreatedAt, err = oracletest.Instant(v[2]); err != nil {
			return nil, err
		}
		p.Keeps = append(p.Keeps, row)
	}
	return p, nil
}

// errUnscripted is a Store call the case gave no answer for: Go made a call
// Python did not, which the calls comparison reports.
var errUnscripted = errors.New("unscripted repository call")

// fakeStore answers from the case's world, as the script's Stub does, and
// records every call in the script's encoding.
type fakeStore struct {
	h                 *harness
	calls             []any
	context           *Context
	member            bool
	roster            []Member
	locks, notebooks  []*Notebook
	proposal          *Proposal
	papers            []Paper
	reads, locked     []*Paper
	outings           []*string
	conflicts         map[string][]any
	constraintVersion int
}

func (h *harness) newStore(world map[string]any) (*fakeStore, error) {
	s := &fakeStore{
		h:                 h,
		calls:             []any{},
		context:           &Context{Kind: "pair"},
		member:            true,
		roster:            []Member{{PersonID: h.ids["TOI"], State: "active"}, {PersonID: h.ids["KIA"], State: "active"}},
		conflicts:         map[string][]any{},
		constraintVersion: 1,
	}
	if raw, ok := world["context"]; ok {
		s.context = nil
		if raw != nil {
			kind, err := oracletest.Str(raw)
			if err != nil {
				return nil, err
			}
			s.context = &Context{Kind: kind}
		}
	}
	if raw, ok := world["member"]; ok {
		member, err := oracletest.Bool(raw)
		if err != nil {
			return nil, err
		}
		s.member = member
	}
	if raw, ok := world["roster"]; ok {
		rows, err := oracletest.List(raw)
		if err != nil {
			return nil, err
		}
		s.roster = []Member{}
		for _, row := range rows {
			pair, err := fields(row, 2)
			if err != nil {
				return nil, err
			}
			person, err := h.id(pair[0])
			if err != nil {
				return nil, err
			}
			state, err := oracletest.Str(pair[1])
			if err != nil {
				return nil, err
			}
			s.roster = append(s.roster, Member{PersonID: person, State: state})
		}
	}
	notebooks := func(key string) ([]*Notebook, error) {
		items, err := oracletest.List(orEmpty(world[key]))
		if err != nil {
			return nil, err
		}
		out := []*Notebook{}
		for _, item := range items {
			n, err := h.notebookOf(item)
			if err != nil {
				return nil, err
			}
			out = append(out, n)
		}
		return out, nil
	}
	papers := func(raw any) ([]*Paper, error) {
		items, err := oracletest.List(orEmpty(raw))
		if err != nil {
			return nil, err
		}
		out := []*Paper{}
		for _, item := range items {
			p, err := h.paperOf(item)
			if err != nil {
				return nil, err
			}
			out = append(out, p)
		}
		return out, nil
	}
	var err error
	if s.locks, err = notebooks("locks"); err != nil {
		return nil, err
	}
	if s.notebooks, err = notebooks("notebooks"); err != nil {
		return nil, err
	}
	if s.proposal, err = h.proposalOf(world["proposal"]); err != nil {
		return nil, err
	}
	listed, err := papers(world["papers"])
	if err != nil {
		return nil, err
	}
	for _, p := range listed {
		s.papers = append(s.papers, *p)
	}
	if s.reads, err = papers(world["reads"]); err != nil {
		return nil, err
	}
	if raw, ok := world["locked"]; ok {
		if s.locked, err = papers(raw); err != nil {
			return nil, err
		}
	} else if s.locked, err = papers(world["reads"]); err != nil {
		return nil, err
	} else if len(s.locked) > 1 {
		s.locked = s.locked[:1]
	}
	outings, err := oracletest.List(orEmpty(world["outings"]))
	if err != nil {
		return nil, err
	}
	for _, raw := range outings {
		id, err := h.optionalID(raw)
		if err != nil {
			return nil, err
		}
		s.outings = append(s.outings, id)
	}
	if raw, ok := world["conflicts"].(map[string]any); ok {
		for name, codes := range raw {
			if s.conflicts[name], err = oracletest.List(codes); err != nil {
				return nil, err
			}
		}
	}
	if raw, ok := world["constraint_version"]; ok {
		if s.constraintVersion, err = intOf(raw); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func orEmpty(value any) any {
	if value == nil {
		return []any{}
	}
	return value
}

func (s *fakeStore) rec(name string, args ...any) {
	s.calls = append(s.calls, append([]any{name}, args...))
}

// conflict answers a write with the next scripted conflict code, if any.
func (s *fakeStore) conflict(name string) error {
	codes := s.conflicts[name]
	if len(codes) == 0 {
		return nil
	}
	s.conflicts[name] = codes[1:]
	if code, ok := codes[0].(string); ok {
		return &Conflict{Code: code}
	}
	return nil
}

func popNotebook(queue *[]*Notebook) (*Notebook, error) {
	if len(*queue) == 0 {
		return nil, errUnscripted
	}
	head := (*queue)[0]
	*queue = (*queue)[1:]
	return head, nil
}

func popPaper(queue *[]*Paper) (*Paper, error) {
	if len(*queue) == 0 {
		return nil, errUnscripted
	}
	head := (*queue)[0]
	*queue = (*queue)[1:]
	return head, nil
}

func (h *harness) content(c pairpaper.Content) any {
	chang := make([]any, len(c.Chang))
	for i, stop := range c.Chang {
		chang[i] = map[string]any{"gio": stop.Gio, "viec": stop.Viec, "place_id": optionalText(stop.PlaceID), "can_kiem": stop.CanKiem}
	}
	return map[string]any{"ngay": c.Ngay, "chang": chang}
}

func nguon(n pairpaper.Nguon) any {
	return map[string]any{"scope": n.Scope, "dung": oracletest.AnyStrings(n.Dung), "luc": n.Luc}
}

func (s *fakeStore) GetContext(contextID string) (*Context, error) {
	s.rec("get_context", s.h.name(contextID))
	return s.context, nil
}

func (s *fakeStore) IsMember(contextID, personID string) (bool, error) {
	s.rec("is_member", s.h.name(contextID), s.h.name(personID))
	return s.member, nil
}

func (s *fakeStore) ListMembers(contextID string) ([]Member, error) {
	s.rec("list_members", s.h.name(contextID))
	return append([]Member(nil), s.roster...), nil
}

func (s *fakeStore) GetPairNotebook(contextID string) (*Notebook, error) {
	s.rec("get_pair_notebook", s.h.name(contextID))
	return popNotebook(&s.notebooks)
}

func (s *fakeStore) CreatePairNotebook(contextID string, now time.Time) error {
	s.rec("create_pair_notebook", s.h.name(contextID), iso(now))
	return nil
}

func (s *fakeStore) LockPairNotebook(contextID string) (*Notebook, error) {
	s.rec("lock_pair_notebook", s.h.name(contextID))
	return popNotebook(&s.locks)
}

func (s *fakeStore) OpenPairCycle(notebookID string, participants []string, termsVersion int, now time.Time) (string, error) {
	s.rec("open_pair_cycle", s.h.name(notebookID), s.h.nameList(participants), int64(termsVersion), iso(now))
	return s.h.ids["CYN"], nil
}

func (s *fakeStore) ActivatePairCycle(cycleID string, now time.Time) error {
	s.rec("activate_pair_cycle", s.h.name(cycleID), iso(now))
	return nil
}

func (s *fakeStore) ClosePairCycle(cycleID string, now time.Time) error {
	s.rec("close_pair_cycle", s.h.name(cycleID), iso(now))
	return nil
}

func (s *fakeStore) CreateConsentProposal(d ProposalDraft) (Proposal, error) {
	s.rec("create_consent_proposal", s.h.name(d.CycleID), d.Purpose, s.h.name(d.ProposedByID), int64(d.TermsVersion), iso(d.ExpiresAt), iso(d.Now))
	return Proposal{ID: s.h.ids["PRN"], CycleID: d.CycleID, Purpose: d.Purpose, ProposedByID: d.ProposedByID, ExpiresAt: d.ExpiresAt}, nil
}

func (s *fakeStore) GetConsentProposal(proposalID string) (*Proposal, error) {
	s.rec("get_consent_proposal", s.h.name(proposalID))
	return s.proposal, nil
}

func (s *fakeStore) GrantConsent(proposalID, personID string, now time.Time) error {
	s.rec("grant_consent", s.h.name(proposalID), s.h.name(personID), iso(now))
	return nil
}

func (s *fakeStore) CompleteConsentProposal(proposalID string, now time.Time) error {
	s.rec("complete_consent_proposal", s.h.name(proposalID), iso(now))
	return nil
}

func (s *fakeStore) RevokeConsents(cycleID, purpose, personID string, now time.Time) error {
	s.rec("revoke_consents", s.h.name(cycleID), purpose, s.h.name(personID), iso(now))
	return nil
}

func (s *fakeStore) SetCoupleMember(personID, cycleID string, now time.Time) error {
	s.rec("set_couple_member", s.h.name(personID), s.h.name(cycleID), iso(now))
	return s.conflict("set_couple_member")
}

func (s *fakeStore) ClearCoupleMember(personID string) error {
	s.rec("clear_couple_member", s.h.name(personID))
	return nil
}

func (s *fakeStore) SetPairConstraint(d ConstraintDraft) (Constraint, error) {
	s.rec("set_pair_constraint", s.h.name(d.CycleID), s.h.name(d.OwnerID), d.Kind, d.Content, iso(d.Now))
	return Constraint{OwnerID: d.OwnerID, Kind: d.Kind, Content: d.Content, Version: s.constraintVersion}, nil
}

func (s *fakeStore) DeletePairConstraint(cycleID, ownerID, kind string) error {
	s.rec("delete_pair_constraint", s.h.name(cycleID), s.h.name(ownerID), kind)
	return nil
}

func (s *fakeStore) CreatePairPaper(d PaperDraft) (Paper, error) {
	s.rec("create_pair_paper", s.h.name(d.ContextID), s.h.optionalName(d.CycleID), s.h.name(d.DraftOwnerID), d.Tuan.ISOFormat(),
		iso(d.ExpiresAt), s.h.content(d.Content), optionalText(d.LyDo), nguon(d.Nguon), d.AuthorType, iso(d.Now))
	return Paper{
		ID: s.h.ids["PPN"], ContextID: d.ContextID, DraftOwnerID: d.DraftOwnerID, State: "nhap", CurrentVersion: 1,
		Tuan: d.Tuan, ExpiresAt: d.ExpiresAt,
	}, nil
}

func (s *fakeStore) GetPairPaper(paperID string) (*Paper, error) {
	s.rec("get_pair_paper", s.h.name(paperID))
	return popPaper(&s.reads)
}

func (s *fakeStore) LockPairPaper(paperID string) (*Paper, error) {
	s.rec("lock_pair_paper", s.h.name(paperID))
	return popPaper(&s.locked)
}

func (s *fakeStore) ListPairPapers(contextID string) ([]Paper, error) {
	s.rec("list_pair_papers", s.h.name(contextID))
	return append([]Paper(nil), s.papers...), nil
}

func (s *fakeStore) UpdatePairDraft(paperID string, content pairpaper.Content, lyDo *string) error {
	s.rec("update_pair_draft", s.h.name(paperID), s.h.content(content), optionalText(lyDo))
	return nil
}

func (s *fakeStore) AddPaperVersion(d VersionDraft) error {
	s.rec("add_paper_version", s.h.name(d.PaperID), int64(d.Version), s.h.content(d.Content), optionalText(d.LyDo), nguon(d.Nguon),
		d.AuthorType, iso(d.SentAt), s.h.name(d.SentBy), iso(d.Now))
	return nil
}

func (s *fakeStore) MarkVersionSent(paperID string, version int, sentBy *string, now time.Time) error {
	s.rec("mark_version_sent", s.h.name(paperID), int64(version), s.h.optionalName(sentBy), iso(now))
	return nil
}

func (s *fakeStore) SetPaperState(paperID, state string, now time.Time, currentVersion *int, recordedByID *string) error {
	var version any
	if currentVersion != nil {
		version = int64(*currentVersion)
	}
	s.rec("set_paper_state", s.h.name(paperID), state, iso(now), version, s.h.optionalName(recordedByID))
	return nil
}

func (s *fakeStore) MarkPaperViewed(paperID string, version int, personID string, now time.Time) error {
	s.rec("mark_paper_viewed", s.h.name(paperID), int64(version), s.h.name(personID), iso(now))
	return nil
}

func (s *fakeStore) AddPaperResponse(paperID string, version int, personID, kind string, now time.Time) error {
	s.rec("add_paper_response", s.h.name(paperID), int64(version), s.h.name(personID), kind, iso(now))
	return s.conflict("add_paper_response")
}

func (s *fakeStore) LinkPaperOuting(paperID string, version int, outingID string, now time.Time) error {
	s.rec("link_paper_outing", s.h.name(paperID), int64(version), s.h.name(outingID), iso(now))
	return s.conflict("link_paper_outing")
}

func (s *fakeStore) GetPaperOuting(paperID string) (*string, error) {
	s.rec("get_paper_outing", s.h.name(paperID))
	if len(s.outings) == 0 {
		return nil, errUnscripted
	}
	head := s.outings[0]
	s.outings = s.outings[1:]
	return head, nil
}

func (s *fakeStore) AddPaperKeep(paperID, personID, line string, now time.Time) (Keep, error) {
	s.rec("add_paper_keep", s.h.name(paperID), s.h.name(personID), line, iso(now))
	return Keep{ID: s.h.ids["KPN"], Line: line, CreatedAt: now}, nil
}

func (s *fakeStore) CloseOpenPairPapers(contextID string, now time.Time) error {
	s.rec("close_open_pair_papers", s.h.name(contextID), iso(now))
	return nil
}

func (s *fakeStore) CreateOuting(d OutingDraft) (string, error) {
	s.rec("create_outing", s.h.name(d.ContextID), s.h.name(d.CreatedByID), d.Title, d.StartsOn.ISOFormat(), d.EndsOn.ISOFormat(),
		int64(d.Headcount), d.BudgetPerPersonVND, iso(d.Now))
	return s.h.ids["OUN"], nil
}

// --- the answers, in model_dump()'s shape -----------------------------------

func (h *harness) proposalView(v ProposalView) any {
	return map[string]any{
		"id": h.name(v.ID), "purpose": v.Purpose, "expires_at": iso(v.ExpiresAt),
		"proposed_by_id": h.name(v.ProposedByID), "my_granted": v.MyGranted,
	}
}

func (h *harness) constraintView(c Constraint) any {
	return map[string]any{"owner_id": h.name(c.OwnerID), "kind": c.Kind, "content": c.Content, "version": int64(c.Version)}
}

func (h *harness) notebookView(v NotebookView) any {
	mine := []any{}
	for _, state := range v.MyConsents {
		mine = append(mine, map[string]any{"purpose": state.Purpose, "granted": state.Granted})
	}
	theirs := map[string]any{}
	for _, state := range v.TheirConsentsGranted {
		theirs[state.Purpose] = state.Granted
	}
	pending := []any{}
	for _, proposal := range v.PendingProposals {
		pending = append(pending, h.proposalView(proposal))
	}
	constraints := []any{}
	for _, constraint := range v.Constraints {
		constraints = append(constraints, h.constraintView(constraint))
	}
	return map[string]any{
		"context_id": h.name(v.ContextID), "cycle_state": optionalText(v.CycleState), "participants": h.nameList(v.Participants),
		"my_consents": mine, "their_consents_granted": theirs, "pending_proposals": pending, "constraints": constraints,
		"nep_gui_ho": v.NepGuiHo, "open_paper_id": h.optionalName(v.OpenPaperID),
	}
}

func previewView(p pairnotebook.ClosePreview) any {
	return map[string]any{
		"revision": p.Revision, "so_nhap_bo": int64(p.SoNhapBo), "so_to_huy": int64(p.SoToHuy),
		"so_to_khoa": int64(p.SoToKhoa), "so_de_nghi_huy": int64(p.SoDeNghiHuy),
	}
}

func (h *harness) summaries(rows []PaperSummary) any {
	papers := []any{}
	for _, row := range rows {
		var ngay, first any
		if row.Ngay != nil {
			ngay = row.Ngay.ISOFormat()
		}
		if row.ChangDau != nil {
			first = map[string]any{"gio": row.ChangDau.Gio, "viec": row.ChangDau.Viec}
		}
		papers = append(papers, map[string]any{
			"id": h.name(row.ID), "state": row.State, "version": int64(row.Version), "tuan": row.Tuan.ISOFormat(),
			"ngay": ngay, "expires_at": iso(row.ExpiresAt), "chang_dau": first, "dong_giu_dau": optionalText(row.DongGiuDau),
		})
	}
	return map[string]any{"papers": papers}
}

func (h *harness) command(c Command) any {
	return map[string]any{"id": h.name(c.ID), "state": c.State, "version": int64(c.Version), "outing_id": h.optionalName(c.OutingID)}
}

func (h *harness) keepView(k Keep) any {
	return map[string]any{"id": h.name(k.ID), "line": k.Line, "created_at": iso(k.CreatedAt)}
}

func (h *harness) paperView(v PaperView) any {
	versions := []any{}
	for _, version := range v.Versions {
		chang := []any{}
		for _, stop := range version.Content.Chang {
			chang = append(chang, map[string]any{"gio": stop.Gio, "viec": stop.Viec, "place_id": h.optionalName(stop.PlaceID), "can_kiem": stop.CanKiem})
		}
		versions = append(versions, map[string]any{
			"version":                int64(version.Version),
			"content":                map[string]any{"ngay": version.Content.Ngay.ISOFormat(), "chang": chang},
			"ly_do":                  optionalText(version.LyDo),
			"author_type":            version.AuthorType,
			"sent_at":                optionalISO(version.SentAt),
			"sent_by":                h.optionalName(version.SentBy),
			"my_response":            optionalText(version.MyResponse),
			"their_agreed":           version.TheirAgreed,
			"viewed_by_recipient_at": optionalISO(version.ViewedByRecipientAt),
		})
	}
	keeps := []any{}
	for _, keep := range v.Keeps {
		keeps = append(keeps, h.keepView(keep))
	}
	return map[string]any{
		"id": h.name(v.ID), "state": v.State, "version": int64(v.Version), "author_type": v.AuthorType,
		"sent_by": h.optionalName(v.SentBy), "tuan": v.Tuan.ISOFormat(), "expires_at": iso(v.ExpiresAt),
		"outing_id": h.optionalName(v.OutingID), "co_the_ghi_da_di": v.CoTheGhiDaDi, "versions": versions, "keeps": keeps,
	}
}

// outcome is run_step's dict: the calls, and exactly one of problem, raised
// or response.
func outcome(s *fakeStore, response any, err error) any {
	out := map[string]any{"calls": s.calls, "problem": nil, "raised": nil, "response": nil}
	var (
		refused  *Refusal
		denied   *permissions.Error
		conflict *Conflict
		broken   *Invariant
		paper    *pairpaper.PaperError
	)
	switch {
	case err == nil:
		out["response"] = response
	case errors.As(err, &refused):
		out["problem"] = map[string]any{"status": int64(refused.Status), "code": refused.Code, "detail": refused.Detail}
	case errors.As(err, &denied):
		out["raised"] = map[string]any{"type": "PermissionError_", "code": denied.Code}
	case errors.As(err, &conflict):
		out["raised"] = map[string]any{"type": "RepositoryConflict", "code": conflict.Code}
	case errors.As(err, &broken):
		out["raised"] = map[string]any{"type": "AssertionError", "code": nil}
	case errors.As(err, &paper):
		out["raised"] = map[string]any{"type": "PaperError", "code": paper.Code}
	default:
		out["raised"] = map[string]any{"type": "go: " + err.Error(), "code": nil}
	}
	return out
}

func contentInput(value any) (ContentInput, error) {
	m, err := oracletest.Row(value, "ngay", "chang")
	if err != nil {
		return ContentInput{}, err
	}
	var in ContentInput
	if in.Ngay, err = dateOf(m["ngay"]); err != nil {
		return in, err
	}
	stops, err := oracletest.List(m["chang"])
	if err != nil {
		return in, err
	}
	for _, raw := range stops {
		f, err := fields(raw, 4)
		if err != nil {
			return in, err
		}
		var stop StopInput
		if stop.Gio, err = oracletest.Str(f[0]); err != nil {
			return in, err
		}
		if stop.Viec, err = oracletest.Str(f[1]); err != nil {
			return in, err
		}
		if stop.PlaceID, err = oracletest.OptionalString(f[2]); err != nil {
			return in, err
		}
		if stop.CanKiem, err = oracletest.Bool(f[3]); err != nil {
			return in, err
		}
		in.Chang = append(in.Chang, stop)
	}
	return in, nil
}

func (h *harness) replay(c oracletest.Case, args map[string]any) (any, error) {
	decode := func(err error) (any, error) { return nil, oracletest.Decode(fmt.Errorf("%s: %w", c.Name, err)) }
	now, err := oracletest.Instant(args["now"])
	if err != nil {
		return decode(err)
	}
	who, err := fields(args["actor"], 2)
	if err != nil {
		return decode(err)
	}
	actor := Actor{}
	if actor.ID, err = h.id(who[0]); err != nil {
		return decode(err)
	}
	if actor.Roles, err = oracletest.Strings(who[1]); err != nil {
		return decode(err)
	}
	req, ok := args["req"].(map[string]any)
	if !ok {
		return decode(fmt.Errorf("req %v", args["req"]))
	}
	world, ok := args["world"].(map[string]any)
	if !ok {
		return decode(fmt.Errorf("world %v", args["world"]))
	}
	s, err := h.newStore(world)
	if err != nil {
		return decode(err)
	}
	text := func(key string) (string, error) { return oracletest.Str(req[key]) }
	var contextID, paperID string
	if _, ok := req["context_id"]; ok {
		if contextID, err = h.id(req["context_id"]); err != nil {
			return decode(err)
		}
	}
	if _, ok := req["paper_id"]; ok {
		if paperID, err = h.id(req["paper_id"]); err != nil {
			return decode(err)
		}
	}
	var version int64
	if _, ok := req["version"]; ok {
		if version, err = saturated(req["version"]); err != nil {
			return decode(err)
		}
	}
	switch c.Fn {
	case "pair_notebook":
		v, err := ReadNotebook(s, actor, contextID, now)
		return outcome(s, h.notebookView(v), err), nil
	case "propose_pair_consent":
		purpose, err := text("purpose")
		if err != nil {
			return decode(err)
		}
		v, err := ProposeConsent(s, actor, contextID, purpose, now)
		return outcome(s, h.proposalView(v), err), nil
	case "grant_pair_consent":
		proposalID, err := h.id(req["proposal_id"])
		if err != nil {
			return decode(err)
		}
		v, err := GrantConsent(s, actor, contextID, proposalID, now)
		return outcome(s, h.proposalView(v), err), nil
	case "revoke_pair_consent":
		purpose, err := text("purpose")
		if err != nil {
			return decode(err)
		}
		return outcome(s, nil, RevokeConsent(s, actor, contextID, purpose, now)), nil
	case "put_pair_constraint":
		kind, err := text("kind")
		if err != nil {
			return decode(err)
		}
		content, err := text("content")
		if err != nil {
			return decode(err)
		}
		v, err := PutConstraint(s, actor, contextID, kind, content, now)
		return outcome(s, h.constraintView(v), err), nil
	case "delete_pair_constraint":
		kind, err := text("kind")
		if err != nil {
			return decode(err)
		}
		return outcome(s, nil, DeleteConstraint(s, actor, contextID, kind, now)), nil
	case "preview_close_pair_notebook":
		v, err := PreviewClose(s, actor, contextID, now, sha256.Sum256)
		return outcome(s, previewView(v), err), nil
	case "close_pair_notebook":
		revision, err := text("revision")
		if err != nil {
			return decode(err)
		}
		return outcome(s, nil, CloseNotebook(s, actor, contextID, revision, now, sha256.Sum256)), nil
	case "list_pair_papers":
		v, err := ListPapers(s, actor, contextID, now)
		return outcome(s, h.summaries(v), err), nil
	case "draft_pair_paper":
		v, err := DraftPaper(s, actor, contextID, now)
		return outcome(s, h.command(v), err), nil
	case "pair_paper":
		v, err := ReadPaper(s, actor, paperID, now)
		return outcome(s, h.paperView(v), err), nil
	case "edit_pair_draft":
		content, err := contentInput(req["content"])
		if err != nil {
			return decode(err)
		}
		lyDo, err := oracletest.OptionalString(req["ly_do"])
		if err != nil {
			return decode(err)
		}
		v, err := EditDraft(s, actor, paperID, content, lyDo, now)
		return outcome(s, h.command(v), err), nil
	case "send_pair_paper":
		v, err := SendPaper(s, actor, paperID, version, now)
		return outcome(s, h.command(v), err), nil
	case "mark_pair_paper_viewed":
		return outcome(s, nil, MarkViewed(s, actor, paperID, version, now)), nil
	case "respond_pair_paper":
		raw, ok := req["reply"].(map[string]any)
		if !ok {
			return decode(fmt.Errorf("reply %v", req["reply"]))
		}
		reply := Reply{}
		if reply.Kind, err = oracletest.Str(raw["kind"]); err != nil {
			return decode(err)
		}
		if _, ok := raw["content"]; ok {
			if reply.Content, err = contentInput(raw["content"]); err != nil {
				return decode(err)
			}
			if reply.LyDo, err = oracletest.OptionalString(raw["ly_do"]); err != nil {
				return decode(err)
			}
		}
		v, err := RespondPaper(s, actor, paperID, version, reply, now)
		return outcome(s, h.command(v), err), nil
	case "withdraw_pair_paper":
		v, err := WithdrawPaper(s, actor, paperID, version, now)
		return outcome(s, h.command(v), err), nil
	case "skip_pair_week":
		v, err := SkipWeek(s, actor, paperID, now)
		return outcome(s, h.command(v), err), nil
	case "record_pair_outing_done":
		v, err := RecordDone(s, actor, paperID, now)
		return outcome(s, h.command(v), err), nil
	case "keep_pair_paper_line":
		line, err := text("line")
		if err != nil {
			return decode(err)
		}
		v, err := KeepLine(s, actor, paperID, line, now)
		return outcome(s, h.keepView(v), err), nil
	}
	return decode(fmt.Errorf("unknown method %q", c.Fn))
}

// noRefusal: a step answers every outcome inside its result, never as a
// top-level raise.
func noRefusal(error) (string, string, bool) { return "", "", false }

func checkConstants(t *testing.T, k map[string]any) {
	t.Helper()
	got, err := oracletest.Strings(k["methods"])
	if err != nil || !reflect.DeepEqual(got, methods) {
		t.Errorf("methods: Python %v, Go %v", got, methods)
	}
	rows, err := oracletest.List(k["permission_refusals"])
	if err != nil {
		t.Fatal(err)
	}
	refusals := map[string]Refusal{}
	for _, raw := range rows {
		entry, err := fields(raw, 2)
		if err != nil {
			t.Fatal(err)
		}
		answer, err := fields(entry[1], 3)
		if err != nil {
			t.Fatal(err)
		}
		name, _ := entry[0].(string)
		status, _ := answer[0].(int64)
		code, _ := answer[1].(string)
		detail, _ := answer[2].(string)
		refusals[name] = Refusal{Status: int(status), Code: code, Detail: detail}
	}
	if !reflect.DeepEqual(refusals, PermissionRefusals()) {
		t.Errorf("_TU_CHOI_TO_GIAY: Python %v, Go %v", refusals, PermissionRefusals())
	}
	rows, err = oracletest.List(k["paper_error_details"])
	if err != nil {
		t.Fatal(err)
	}
	details := map[string]string{}
	for _, raw := range rows {
		pair, err := oracletest.Strings(raw)
		if err != nil || len(pair) != 2 {
			t.Fatalf("detail %v", raw)
		}
		details[pair[0]] = pair[1]
	}
	if !reflect.DeepEqual(details, PaperErrorDetails()) {
		t.Errorf("_LOI_TO_GIAY: Python %v, Go %v", details, PaperErrorDetails())
	}
	if k["dieu_khoan_hien_tai"] != int64(DieuKhoanHienTai) {
		t.Errorf("DIEU_KHOAN_HIEN_TAI: Python %v, Go %d", k["dieu_khoan_hien_tai"], DieuKhoanHienTai)
	}
	frame := map[string]any{"gio": khungGio, "viec": khungViec, "di_tiep": nil}
	if !reflect.DeepEqual(k["khung_mac_dinh"], frame) {
		t.Errorf("_KHUNG_MAC_DINH: Python %v, Go %v", k["khung_mac_dinh"], frame)
	}
}

// checkSteps replays module's cases in files and holds the corpus to its
// spread: at least least cases, every method, every problem code and every
// class of 500.
func checkSteps(t *testing.T, h *harness, files []oracletest.File, least int, everyCode bool) {
	t.Helper()
	report := oracletest.Agree(t, files, "pair_steps", h.replay, noRefusal)
	total := 0
	for _, tally := range report.ByFn {
		total += tally.Cases
	}
	if total < least {
		t.Errorf("%d cases, want at least %d", total, least)
	}
	for _, fn := range methods {
		if report.ByFn[fn] == nil {
			t.Errorf("no Python case of %s", fn)
		}
	}
	raised := map[string]int{}
	for _, file := range files {
		for _, c := range file.Cases {
			value, _, err := c.Outcome()
			if err != nil {
				t.Fatal(err)
			}
			if row, ok := value.(map[string]any); ok {
				if exception, ok := row["raised"].(map[string]any); ok {
					raised[fmt.Sprint(exception["type"])]++
				}
			}
		}
	}
	if everyCode {
		for _, code := range problemCodes {
			if report.Codes[code] == 0 {
				t.Errorf("no Python case answered %s", code)
			}
		}
		for _, kind := range raisedTypes {
			if raised[kind] == 0 {
				t.Errorf("no Python case raised %s", kind)
			}
		}
	}
	t.Logf("problems %v; raised %v", report.Codes, raised)
}

func TestPairStepsMatchPython(t *testing.T) {
	files := oracletest.Load(t, "testdata/python_pair_steps*.json")
	constants := oracletest.Constants(t, files, "pair_steps")
	checkConstants(t, constants)
	checkSteps(t, newHarness(t, constants), files, 450, true)
}

// A request version outside int64 saturates, and no stored version is that
// large, so it is stale rather than wrapped onto a real one.
func TestVersionSaturation(t *testing.T) {
	huge, _ := new(big.Int).SetString(strings.Repeat("9", 30), 10)
	for _, value := range []any{oracletest.BigInt(huge.String()), oracletest.BigInt("-" + huge.String())} {
		got, err := saturated(value)
		if err != nil || (got != math.MaxInt64 && got != math.MinInt64) {
			t.Fatalf("saturated(%v) = %d, %v", value, got, err)
		}
	}
}

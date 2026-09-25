package peoplesteps

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"mobile/services/core/internal/domain/friendship"
	"mobile/services/core/internal/domain/interests"
	"mobile/services/core/internal/domain/pairpaper"
	"mobile/services/core/internal/domain/permissions"
	"mobile/services/core/internal/oracletest"
)

// testdata/python_people_steps*.json is rendered by
// scripts/render_domain_w10_goldens.py by running the real people methods of
// app.api.service over a recording stub repository and photo storage, with the
// clock pinned to each case's `now`, in the parity API image. Every case is
// replayed here through a recording Store: the answer (or the ApiProblem, or
// the exception that is a 500), every call with its arguments, and every log
// line must match.

// methods is every ported service method, in the order of the script's
// CALLERS.
var methods = []string{
	"list_my_contexts", "get_my_profile", "update_my_profile", "list_saved_places", "save_place",
	"unsave_place", "list_blocked_people", "delete_own_account", "block_person", "unblock_person",
	"open_direct_message", "get_person_profile", "register_person",
}

// problemCodes is every ApiProblem code the committed corpus must show.
var problemCodes = []string{
	"permission_denied", "person_not_found", "person_not_visible", "place_not_found", "confirm_required",
	"not_blocked", "only_blocker_may_unblock", "self_direct_message", "pair_exists", "context_not_found",
	"person_exists",
}

// raisedTypes is every exception class the committed corpus must show ending a
// request. AssertionError (get_person_profile's assert) and FriendshipError
// (pair_key of non-uuid ids) cannot be reached.
var raisedTypes = []string{"PermissionError_", "RepositoryConflict", "ValueError", "InterestError"}

type harness struct {
	ids      map[string]string
	names    map[string]string
	defaults map[string]any
	earlier  time.Time
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
	if h.defaults, err = oracletest.Row(constants["world_defaults"]); err != nil {
		t.Fatal(err)
	}
	if h.earlier, err = oracletest.Instant(constants["earlier"]); err != nil {
		t.Fatal(err)
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

// fields reads a positional record of exactly n values.
func fields(value any, n int) ([]any, error) {
	items, err := oracletest.List(value)
	if err != nil || len(items) != n {
		return nil, fmt.Errorf("%v is not a record of %d fields", value, n)
	}
	return items, nil
}

// personOf reads [id, display_name, created_at, bio, city, budget_band,
// wall_comment_policy, discoverable_by_phone, deleted_at].
func (h *harness) personOf(value any) (*Person, error) {
	if value == nil {
		return nil, nil
	}
	f, err := fields(value, 9)
	if err != nil {
		return nil, err
	}
	p := &Person{}
	if p.ID, err = h.id(f[0]); err != nil {
		return nil, err
	}
	if p.DisplayName, err = oracletest.Str(f[1]); err != nil {
		return nil, err
	}
	if p.CreatedAt, err = oracletest.Instant(f[2]); err != nil {
		return nil, err
	}
	if p.Bio, err = oracletest.OptionalString(f[3]); err != nil {
		return nil, err
	}
	if p.City, err = oracletest.OptionalString(f[4]); err != nil {
		return nil, err
	}
	if p.BudgetBand, err = oracletest.OptionalString(f[5]); err != nil {
		return nil, err
	}
	if p.WallCommentPolicy, err = oracletest.Str(f[6]); err != nil {
		return nil, err
	}
	if p.DiscoverableByPhone, err = oracletest.Bool(f[7]); err != nil {
		return nil, err
	}
	if p.DeletedAt, err = optionalInstant(f[8]); err != nil {
		return nil, err
	}
	return p, nil
}

// summaryOf reads the SUMMARY_KEYS record; last_message is nil or [id, kind,
// preview, author_id, author_display_name, created_at].
func (h *harness) summaryOf(value any) (SummaryRecord, error) {
	var r SummaryRecord
	f, err := fields(value, 13)
	if err != nil {
		return r, err
	}
	if r.ID, err = h.id(f[0]); err != nil {
		return r, err
	}
	if r.DisplayName, err = oracletest.Str(f[1]); err != nil {
		return r, err
	}
	if r.MemberCount, err = oracletest.Int64(f[2]); err != nil {
		return r, err
	}
	if r.MyRole, err = oracletest.Str(f[3]); err != nil {
		return r, err
	}
	if r.MyState, err = oracletest.Str(f[4]); err != nil {
		return r, err
	}
	if r.MembershipID, err = h.id(f[5]); err != nil {
		return r, err
	}
	if r.JoinedAt, err = optionalInstant(f[6]); err != nil {
		return r, err
	}
	if f[7] != nil {
		m, err := fields(f[7], 6)
		if err != nil {
			return r, err
		}
		last := &LastMessage{}
		if last.ID, err = h.id(m[0]); err != nil {
			return r, err
		}
		if last.Kind, err = oracletest.Str(m[1]); err != nil {
			return r, err
		}
		if last.Preview, err = oracletest.Str(m[2]); err != nil {
			return r, err
		}
		if last.AuthorID, err = h.optionalID(m[3]); err != nil {
			return r, err
		}
		if last.AuthorDisplayName, err = oracletest.OptionalString(m[4]); err != nil {
			return r, err
		}
		if last.CreatedAt, err = oracletest.Instant(m[5]); err != nil {
			return r, err
		}
		r.LastMessage = last
	}
	if r.UnreadCount, err = oracletest.Int64(f[8]); err != nil {
		return r, err
	}
	if r.Theme, err = oracletest.Str(f[9]); err != nil {
		return r, err
	}
	if r.Kind, err = oracletest.Str(f[10]); err != nil {
		return r, err
	}
	if r.CounterpartID, err = h.optionalID(f[11]); err != nil {
		return r, err
	}
	if r.CounterpartDisplayName, err = oracletest.OptionalString(f[12]); err != nil {
		return r, err
	}
	return r, nil
}

// errUnscripted is a call the case gave no answer for: Go made a call Python
// did not, which the calls comparison reports.
var errUnscripted = errors.New("unscripted repository call")

type edgeRow struct {
	x, y string
	edge FriendEdge
}

type placeRow struct {
	id    string
	place Place
}

// fakeStore answers from the case's world, as the script's Stub does, and
// records every call in the script's encoding.
type fakeStore struct {
	h            *harness
	now          time.Time
	calls        []any
	people       map[string]*Person
	friends      [][2]string
	couples      [][2]string
	groupmates   [][2]string
	edges        []edgeRow
	summaries    []SummaryRecord
	pairContexts []*string
	counts       ProfileCounts
	providers    []string
	interests    []string
	saved        []SavedPlace
	places       []placeRow
	saveCreated  bool
	unsave       bool
	blocked      []BlockedEdge
	storageKeys  []string
	photo        []any
	conflicts    map[string][]any
	updated      *Person
	hasUpdated   bool
	renamed      *Person
	hasRenamed   bool
}

func (h *harness) pairs(raw any) ([][2]string, error) {
	rows, err := oracletest.List(raw)
	if err != nil {
		return nil, err
	}
	out := [][2]string{}
	for _, row := range rows {
		f, err := fields(row, 2)
		if err != nil {
			return nil, err
		}
		a, err := h.id(f[0])
		if err != nil {
			return nil, err
		}
		b, err := h.id(f[1])
		if err != nil {
			return nil, err
		}
		out = append(out, [2]string{a, b})
	}
	return out, nil
}

func (h *harness) newStore(raw map[string]any, now time.Time) (*fakeStore, error) {
	world := map[string]any{}
	for key, value := range h.defaults {
		world[key] = value
	}
	for key, value := range raw {
		world[key] = value
	}
	s := &fakeStore{h: h, now: now, calls: []any{}, people: map[string]*Person{}, conflicts: map[string][]any{}}
	people, err := oracletest.Row(world["people"])
	if err != nil {
		return nil, err
	}
	for alias, row := range people {
		id, err := h.id(alias)
		if err != nil {
			return nil, err
		}
		if s.people[id], err = h.personOf(row); err != nil {
			return nil, err
		}
	}
	if s.friends, err = h.pairs(world["friends"]); err != nil {
		return nil, err
	}
	if s.groupmates, err = h.pairs(world["groupmates"]); err != nil {
		return nil, err
	}
	if s.couples, err = h.pairs(world["couples"]); err != nil {
		return nil, err
	}
	edges, err := oracletest.List(world["edges"])
	if err != nil {
		return nil, err
	}
	for _, raw := range edges {
		f, err := oracletest.List(raw)
		if err != nil || (len(f) != 4 && len(f) != 6) {
			return nil, fmt.Errorf("edge %v", raw)
		}
		var row edgeRow
		if row.x, err = h.id(f[0]); err != nil {
			return nil, err
		}
		if row.y, err = h.id(f[1]); err != nil {
			return nil, err
		}
		row.edge.RequesterID, row.edge.AddresseeID = row.x, row.y
		if len(f) == 6 {
			if row.edge.RequesterID, err = h.id(f[4]); err != nil {
				return nil, err
			}
			if row.edge.AddresseeID, err = h.id(f[5]); err != nil {
				return nil, err
			}
		}
		if row.edge.State, err = oracletest.Str(f[2]); err != nil {
			return nil, err
		}
		if row.edge.DecidedByID, err = h.optionalID(f[3]); err != nil {
			return nil, err
		}
		s.edges = append(s.edges, row)
	}
	summaries, err := oracletest.List(world["summaries"])
	if err != nil {
		return nil, err
	}
	for _, raw := range summaries {
		record, err := h.summaryOf(raw)
		if err != nil {
			return nil, err
		}
		s.summaries = append(s.summaries, record)
	}
	contexts, err := oracletest.List(world["pair_contexts"])
	if err != nil {
		return nil, err
	}
	for _, raw := range contexts {
		id, err := h.optionalID(raw)
		if err != nil {
			return nil, err
		}
		s.pairContexts = append(s.pairContexts, id)
	}
	counts, err := fields(world["counts"], 5)
	if err != nil {
		return nil, err
	}
	for i, target := range []*int64{&s.counts.Friends, &s.counts.Contexts, &s.counts.Outings, &s.counts.PlacesCheckedIn, &s.counts.Memories} {
		if *target, err = oracletest.Int64(counts[i]); err != nil {
			return nil, err
		}
	}
	if s.providers, err = oracletest.Strings(world["providers"]); err != nil {
		return nil, err
	}
	if s.interests, err = oracletest.Strings(world["interests"]); err != nil {
		return nil, err
	}
	saved, err := oracletest.List(world["saved"])
	if err != nil {
		return nil, err
	}
	for _, raw := range saved {
		f, err := fields(raw, 3)
		if err != nil {
			return nil, err
		}
		var row SavedPlace
		if row.PlaceID, err = oracletest.Str(f[1]); err != nil {
			return nil, err
		}
		if row.CreatedAt, err = oracletest.Instant(f[2]); err != nil {
			return nil, err
		}
		s.saved = append(s.saved, row)
	}
	places, err := oracletest.List(world["places"])
	if err != nil {
		return nil, err
	}
	for _, raw := range places {
		f, err := oracletest.Strings(raw)
		if err != nil || len(f) != 3 {
			return nil, fmt.Errorf("place %v", raw)
		}
		s.places = append(s.places, placeRow{id: f[0], place: Place{Name: f[1], Category: f[2]}})
	}
	if s.saveCreated, err = oracletest.Bool(world["save_created"]); err != nil {
		return nil, err
	}
	if s.unsave, err = oracletest.Bool(world["unsave"]); err != nil {
		return nil, err
	}
	blocked, err := oracletest.List(world["blocked"])
	if err != nil {
		return nil, err
	}
	for _, raw := range blocked {
		f, err := fields(raw, 5)
		if err != nil {
			return nil, err
		}
		var row BlockedEdge
		if row.OtherPersonID, err = h.id(f[1]); err != nil {
			return nil, err
		}
		if row.OtherDisplayName, err = oracletest.Str(f[2]); err != nil {
			return nil, err
		}
		if row.CreatedAt, err = oracletest.Instant(f[3]); err != nil {
			return nil, err
		}
		if row.DecidedAt, err = optionalInstant(f[4]); err != nil {
			return nil, err
		}
		s.blocked = append(s.blocked, row)
	}
	if s.storageKeys, err = oracletest.Strings(world["storage_keys"]); err != nil {
		return nil, err
	}
	if s.photo, err = oracletest.List(world["photo"]); err != nil {
		return nil, err
	}
	conflicts, err := oracletest.Row(world["conflicts"])
	if err != nil {
		return nil, err
	}
	for name, codes := range conflicts {
		if s.conflicts[name], err = oracletest.List(codes); err != nil {
			return nil, err
		}
	}
	if raw, ok := world["updated"]; ok {
		s.hasUpdated = true
		if s.updated, err = h.personOf(raw); err != nil {
			return nil, err
		}
	}
	if raw, ok := world["renamed"]; ok {
		s.hasRenamed = true
		if s.renamed, err = h.personOf(raw); err != nil {
			return nil, err
		}
	}
	return s, nil
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

func (s *fakeStore) person(id string) *Person {
	if p := s.people[id]; p != nil {
		copied := *p
		return &copied
	}
	return nil
}

func samePair(pairs [][2]string, a, b string) bool {
	for _, p := range pairs {
		if (p[0] == a && p[1] == b) || (p[0] == b && p[1] == a) {
			return true
		}
	}
	return false
}

func (s *fakeStore) GetPerson(personID string) (*Person, error) {
	s.rec("get_person", s.h.name(personID))
	return s.person(personID), nil
}

func (s *fakeStore) CreatePerson(personID, displayName string) (Person, error) {
	s.rec("create_person", s.h.name(personID), displayName)
	if err := s.conflict("create_person"); err != nil {
		return Person{}, err
	}
	return Person{ID: personID, DisplayName: displayName, CreatedAt: s.now, WallCommentPolicy: "readers", DiscoverableByPhone: true}, nil
}

func (s *fakeStore) RenamePerson(personID, displayName string) (*Person, error) {
	s.rec("rename_person", s.h.name(personID), displayName)
	if s.hasRenamed {
		return s.renamed, nil
	}
	current := s.person(personID)
	if current != nil {
		current.DisplayName = displayName
	}
	return current, nil
}

func (s *fakeStore) GetPairContext(pairKey string) (*Context, error) {
	s.rec("get_pair_context", pairKey)
	if len(s.pairContexts) == 0 {
		return nil, errUnscripted
	}
	head := s.pairContexts[0]
	s.pairContexts = s.pairContexts[1:]
	if head == nil {
		return nil, nil
	}
	return &Context{ID: *head}, nil
}

func (s *fakeStore) CreatePairContext(pairKey string, memberIDs [2]string, createdByID string, now time.Time) (Context, error) {
	s.rec("create_pair_context", pairKey, []any{s.h.name(memberIDs[0]), s.h.name(memberIDs[1])}, s.h.name(createdByID), iso(now))
	if err := s.conflict("create_pair_context"); err != nil {
		return Context{}, err
	}
	return Context{ID: s.h.ids["PXN"]}, nil
}

func (s *fakeStore) ListPersonContextSummaries(personID string) ([]SummaryRecord, error) {
	s.rec("list_person_context_summaries", s.h.name(personID))
	return append([]SummaryRecord{}, s.summaries...), nil
}

func (s *fakeStore) UpdatePersonProfile(personID string, changes []Change) (*Person, error) {
	encoded := make([]any, len(changes))
	for i, change := range changes {
		encoded[i] = []any{change.Field, change.Value}
	}
	s.rec("update_person_profile", s.h.name(personID), encoded)
	if err := s.conflict("update_person_profile"); err != nil {
		return nil, err
	}
	if s.hasUpdated {
		return s.updated, nil
	}
	current := s.person(personID)
	if current == nil {
		return nil, nil
	}
	text := func(value any) *string {
		if value == nil {
			return nil
		}
		v := value.(string)
		return &v
	}
	for _, change := range changes {
		switch change.Field {
		case "display_name":
			current.DisplayName = change.Value.(string)
		case "bio":
			current.Bio = text(change.Value)
		case "city":
			current.City = text(change.Value)
		case "budget_band":
			current.BudgetBand = text(change.Value)
		case "wall_comment_policy":
			current.WallCommentPolicy = change.Value.(string)
		case "discoverable_by_phone":
			current.DiscoverableByPhone = change.Value.(bool)
		default:
			return nil, fmt.Errorf("no field %q", change.Field)
		}
	}
	return current, nil
}

func (s *fakeStore) ProfileCounts(personID string) (ProfileCounts, error) {
	s.rec("profile_counts", s.h.name(personID))
	return s.counts, nil
}

func (s *fakeStore) ListLoginProviders(personID string) ([]string, error) {
	s.rec("list_login_providers", s.h.name(personID))
	return append([]string{}, s.providers...), nil
}

func (s *fakeStore) ListPersonInterests(personID string) ([]string, error) {
	s.rec("list_person_interests", s.h.name(personID))
	return append([]string{}, s.interests...), nil
}

func (s *fakeStore) AreFriends(a, b string) (bool, error) {
	s.rec("are_friends", s.h.name(a), s.h.name(b))
	return samePair(s.friends, a, b), nil
}

func (s *fakeStore) SameCouple(a, b string) (bool, error) {
	s.rec("same_couple", s.h.name(a), s.h.name(b))
	return samePair(s.couples, a, b), nil
}

func (s *fakeStore) ShareActiveContext(a, b string) (bool, error) {
	s.rec("share_active_context", s.h.name(a), s.h.name(b))
	return samePair(s.groupmates, a, b), nil
}

func (s *fakeStore) GetPlace(placeID string) (*Place, error) {
	s.rec("get_place", placeID)
	for _, row := range s.places {
		if row.id == placeID {
			place := row.place
			return &place, nil
		}
	}
	return nil, nil
}

func (s *fakeStore) ListSavedPlaces(personID string) ([]SavedPlace, error) {
	s.rec("list_saved_places", s.h.name(personID))
	return append([]SavedPlace{}, s.saved...), nil
}

func (s *fakeStore) SavePlace(personID, placeID string, now time.Time) (SavedPlace, bool, error) {
	s.rec("save_place", s.h.name(personID), placeID, iso(now))
	if err := s.conflict("save_place"); err != nil {
		return SavedPlace{}, false, err
	}
	record := SavedPlace{PlaceID: placeID, CreatedAt: s.h.earlier}
	if s.saveCreated {
		record.CreatedAt = now
	}
	return record, s.saveCreated, nil
}

func (s *fakeStore) UnsavePlace(personID, placeID string) (bool, error) {
	s.rec("unsave_place", s.h.name(personID), placeID)
	return s.unsave, nil
}

func (s *fakeStore) OpenBlockEdge(blockerID, addresseeID string, now time.Time) error {
	s.rec("open_block_edge", s.h.name(blockerID), s.h.name(addresseeID), iso(now))
	return s.conflict("open_block_edge")
}

func (s *fakeStore) LiftBlockEdge(blockerID, addresseeID string, now time.Time) error {
	s.rec("lift_block_edge", s.h.name(blockerID), s.h.name(addresseeID), iso(now))
	return s.conflict("lift_block_edge")
}

func (s *fakeStore) ListBlocked(personID string) ([]BlockedEdge, error) {
	s.rec("list_blocked", s.h.name(personID))
	return append([]BlockedEdge{}, s.blocked...), nil
}

func (s *fakeStore) ErasePerson(personID string, now time.Time) (ErasureReport, error) {
	s.rec("erase_person", s.h.name(personID), iso(now))
	if err := s.conflict("erase_person"); err != nil {
		return ErasureReport{}, err
	}
	return ErasureReport{Counts: map[string]int64{}, StorageKeys: append([]string{}, s.storageKeys...)}, nil
}

func (s *fakeStore) GetFriendEdge(a, b string) (*FriendEdge, error) {
	s.rec("get_friend_edge", s.h.name(a), s.h.name(b))
	for _, row := range s.edges {
		if (row.x == a && row.y == b) || (row.x == b && row.y == a) {
			edge := row.edge
			return &edge, nil
		}
	}
	return nil, nil
}

// fakePhotos is the script's Photos: it records into the same call list and
// answers from the world's photo outcomes, true once they run out.
type fakePhotos struct{ s *fakeStore }

func (p fakePhotos) Delete(storageKey string) (bool, error) {
	p.s.rec("photo_storage.delete", storageKey)
	if len(p.s.photo) == 0 {
		return true, nil
	}
	head := p.s.photo[0]
	p.s.photo = p.s.photo[1:]
	switch v := head.(type) {
	case bool:
		return v, nil
	case string:
		return false, errors.New(v)
	}
	return false, errUnscripted
}

func (h *harness) personMap(p Person) any {
	return map[string]any{
		"id": h.name(p.ID), "display_name": p.DisplayName, "created_at": iso(p.CreatedAt),
		"bio": optionalText(p.Bio), "city": optionalText(p.City), "budget_band": optionalText(p.BudgetBand),
		"wall_comment_policy": p.WallCommentPolicy, "discoverable_by_phone": p.DiscoverableByPhone,
		"deleted_at": optionalISO(p.DeletedAt),
	}
}

func (h *harness) summaryMap(v ContextSummary) any {
	var last, counterpart any
	if v.LastMessage != nil {
		m := v.LastMessage
		last = map[string]any{
			"id": h.name(m.ID), "kind": m.Kind, "preview": m.Preview, "author_id": h.optionalName(m.AuthorID),
			"author_display_name": optionalText(m.AuthorDisplayName), "created_at": iso(m.CreatedAt),
		}
	}
	if v.Counterpart != nil {
		counterpart = map[string]any{"id": h.name(v.Counterpart.ID), "display_name": v.Counterpart.DisplayName}
	}
	return map[string]any{
		"id": h.name(v.ID), "display_name": v.DisplayName, "member_count": v.MemberCount, "my_role": v.MyRole,
		"my_state": v.MyState, "membership_id": h.name(v.MembershipID), "joined_at": optionalISO(v.JoinedAt),
		"last_message": last, "unread_count": v.UnreadCount, "theme": v.Theme, "kind": v.Kind,
		"counterpart": counterpart, "unavailable": v.Unavailable,
	}
}

func (h *harness) profileMap(v Profile) any {
	return map[string]any{
		"id": h.name(v.ID), "display_name": v.DisplayName, "bio": optionalText(v.Bio), "city": optionalText(v.City),
		"created_at": iso(v.CreatedAt),
		"counts": map[string]any{
			"friends": v.Counts.Friends, "contexts": v.Counts.Contexts, "outings": v.Counts.Outings,
			"places_checked_in": v.Counts.PlacesCheckedIn, "memories": v.Counts.Memories,
		},
		"login_methods": oracletest.AnyStrings(v.LoginMethods), "interests": oracletest.AnyStrings(v.Interests),
		"budget_band": optionalText(v.BudgetBand), "wall_comment_policy": v.WallCommentPolicy,
		"discoverable_by_phone": v.DiscoverableByPhone,
	}
}

func (h *harness) publicMap(v PublicPerson) any {
	return map[string]any{
		"id": h.name(v.ID), "display_name": v.DisplayName, "bio": optionalText(v.Bio), "city": optionalText(v.City),
		"created_at": iso(v.CreatedAt), "relation": v.Relation,
	}
}

func savedMap(v SavedPlaceSummary) any {
	return map[string]any{"place_id": v.PlaceID, "name": v.Name, "category": v.Category, "saved_at": iso(v.SavedAt)}
}

func (h *harness) blockMap(v BlockState) any {
	return map[string]any{"person_id": h.name(v.PersonID), "state": v.State}
}

// outcome is run_step's dict: the calls, the logs, and exactly one of
// problem, raised or response.
func outcome(s *fakeStore, response func() any, err error, logs []LogLine) any {
	encodedLogs := []any{}
	for _, line := range logs {
		encodedLogs = append(encodedLogs, []any{line.Level, line.Message})
	}
	out := map[string]any{"calls": s.calls, "problem": nil, "raised": nil, "response": nil, "logs": encodedLogs}
	var (
		refused   *Refusal
		denied    *permissions.Error
		conflict  *Conflict
		broken    *Invariant
		badState  *friendship.ValueError
		friendErr *friendship.FriendshipError
		interest  *interests.InterestError
	)
	switch {
	case err == nil:
		out["response"] = response()
	case errors.As(err, &refused):
		out["problem"] = map[string]any{"status": int64(refused.Status), "code": refused.Code, "detail": refused.Detail}
	case errors.As(err, &denied):
		out["raised"] = map[string]any{"type": "PermissionError_", "code": denied.Code, "message": denied.Code}
	case errors.As(err, &conflict):
		out["raised"] = map[string]any{"type": "RepositoryConflict", "code": conflict.Code, "message": conflict.Code}
	case errors.As(err, &broken):
		out["raised"] = map[string]any{"type": "AssertionError", "code": nil, "message": ""}
	case errors.As(err, &badState):
		out["raised"] = map[string]any{"type": "ValueError", "code": nil, "message": badState.Message}
	case errors.As(err, &friendErr):
		out["raised"] = map[string]any{"type": "FriendshipError", "code": friendErr.Code, "message": friendErr.Code}
	case errors.As(err, &interest):
		// InterestError carries its code only as str(exc), not as .code.
		out["raised"] = map[string]any{"type": "InterestError", "code": nil, "message": interest.Code}
	default:
		out["raised"] = map[string]any{"type": "go: " + err.Error(), "code": nil, "message": nil}
	}
	return out
}

func (h *harness) profileUpdate(raw any) (ProfileUpdate, error) {
	var request ProfileUpdate
	rows, err := oracletest.List(raw)
	if err != nil {
		return request, err
	}
	for _, row := range rows {
		f, err := fields(row, 2)
		if err != nil {
			return request, err
		}
		field, err := oracletest.Str(f[0])
		if err != nil {
			return request, err
		}
		if f[1] == nil {
			continue
		}
		if field == "discoverable_by_phone" {
			value, err := oracletest.Bool(f[1])
			if err != nil {
				return request, err
			}
			request.DiscoverableByPhone = &value
			continue
		}
		value, err := oracletest.Str(f[1])
		if err != nil {
			return request, err
		}
		switch field {
		case "display_name":
			request.DisplayName = &value
		case "bio":
			request.Bio = &value
		case "city":
			request.City = &value
		case "wall_comment_policy":
			request.WallCommentPolicy = &value
		default:
			return request, fmt.Errorf("no field %q", field)
		}
	}
	return request, nil
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
	s, err := h.newStore(world, now)
	if err != nil {
		return decode(err)
	}
	var personID string
	if _, ok := req["person_id"]; ok {
		if personID, err = h.id(req["person_id"]); err != nil {
			return decode(err)
		}
	}
	text := func(key string) (string, error) { return oracletest.Str(req[key]) }
	switch c.Fn {
	case "list_my_contexts":
		v, err := ListMyContexts(s, actor)
		return outcome(s, func() any {
			rows := make([]any, len(v))
			for i, row := range v {
				rows[i] = h.summaryMap(row)
			}
			return map[string]any{"contexts": rows}
		}, err, nil), nil
	case "get_my_profile":
		v, err := GetMyProfile(s, actor)
		return outcome(s, func() any { return h.profileMap(v) }, err, nil), nil
	case "update_my_profile":
		request, err := h.profileUpdate(req["patch"])
		if err != nil {
			return decode(err)
		}
		v, err := UpdateMyProfile(s, actor, request)
		return outcome(s, func() any { return h.profileMap(v) }, err, nil), nil
	case "list_saved_places":
		v, err := ListSavedPlaces(s, actor)
		return outcome(s, func() any {
			rows := make([]any, len(v))
			for i, row := range v {
				rows[i] = savedMap(row)
			}
			return map[string]any{"saved": rows}
		}, err, nil), nil
	case "save_place":
		placeID, err := text("place_id")
		if err != nil {
			return decode(err)
		}
		v, created, err := SavePlace(s, actor, placeID, now)
		return outcome(s, func() any { return map[string]any{"body": savedMap(v), "created": created} }, err, nil), nil
	case "unsave_place":
		placeID, err := text("place_id")
		if err != nil {
			return decode(err)
		}
		return outcome(s, func() any { return nil }, UnsavePlace(s, actor, placeID), nil), nil
	case "list_blocked_people":
		v, err := ListBlockedPeople(s, actor)
		return outcome(s, func() any {
			rows := make([]any, len(v))
			for i, row := range v {
				rows[i] = map[string]any{"person_id": h.name(row.PersonID), "display_name": row.DisplayName, "blocked_at": iso(row.BlockedAt)}
			}
			return map[string]any{"blocked": rows}
		}, err, nil), nil
	case "delete_own_account":
		confirm, err := oracletest.Bool(req["confirm"])
		if err != nil {
			return decode(err)
		}
		erased, err := DeleteOwnAccount(s, fakePhotos{s}, actor, confirm, now)
		var logs []LogLine
		if err == nil {
			logs = erased.Logs()
		}
		return outcome(s, func() any { return nil }, err, logs), nil
	case "block_person":
		v, err := BlockPerson(s, actor, personID, now)
		return outcome(s, func() any { return h.blockMap(v) }, err, nil), nil
	case "unblock_person":
		v, err := UnblockPerson(s, actor, personID, now)
		return outcome(s, func() any { return h.blockMap(v) }, err, nil), nil
	case "open_direct_message":
		v, created, err := OpenDirectMessage(s, actor, personID, now)
		return outcome(s, func() any { return map[string]any{"body": h.summaryMap(v), "created": created} }, err, nil), nil
	case "get_person_profile":
		v, err := GetPersonProfile(s, actor, personID)
		return outcome(s, func() any { return h.publicMap(v) }, err, nil), nil
	case "register_person":
		name, err := text("display_name")
		if err != nil {
			return decode(err)
		}
		v, created, err := RegisterPerson(s, actor, personID, name)
		return outcome(s, func() any { return map[string]any{"body": h.personMap(v), "created": created} }, err, nil), nil
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
	ids, err := oracletest.Strings(k["interest_ids"])
	if err != nil || !reflect.DeepEqual(ids, interests.InterestIDs()) {
		t.Errorf("INTEREST_IDS: Python %v, Go %v", ids, interests.InterestIDs())
	}
}

// checkSteps replays the people_steps cases in files and holds the corpus to
// its spread: at least least cases, every method, and, when everyCode, every
// problem code and every class of 500 the routes can reach.
func checkSteps(t *testing.T, h *harness, files []oracletest.File, least int, everyCode bool) {
	t.Helper()
	report := oracletest.Agree(t, files, "people_steps", h.replay, noRefusal)
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

func TestPeopleStepsMatchPython(t *testing.T) {
	files := oracletest.Load(t, "testdata/python_people_steps*.json")
	constants := oracletest.Constants(t, files, "people_steps")
	checkConstants(t, constants)
	checkSteps(t, newHarness(t, constants), files, 380, true)
}

func TestFriendRefusal(t *testing.T) {
	for code, want := range map[string]Refusal{
		"REQUEST_NOT_OPEN":          {409, "request_not_open", "Chưa gửi được lời mời này."},
		"SELF_EDGE":                 {422, "self_edge", "Không tự kết bạn với chính mình được."},
		"NOT_A_PARTY":               {403, "permission_denied", "not_a_party"},
		"ONLY_ADDRESSEE_MAY_ANSWER": {403, "permission_denied", "only_addressee_may_answer"},
		"NOT_PENDING":               {409, "not_pending", "Lời mời không ở trạng thái đó."},
	} {
		if got := *FriendRefusal(code); got != want {
			t.Errorf("%s: %+v, want %+v", code, got, want)
		}
	}
}

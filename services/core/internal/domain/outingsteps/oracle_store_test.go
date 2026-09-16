package outingsteps

import (
	"fmt"
	"time"

	"mobile/services/core/internal/oracletest"
)

// stub is the Python stub repository: it answers from the case's world and
// records every call with its arguments in the Protocol's order.
type stub struct {
	h        *harness
	calls    []any
	now      time.Time
	earlier  time.Time
	created  string
	people   map[string]string
	contexts map[string]string
	members  map[string][]Member
	order    []Outing
	outings  map[string]Outing
	places   map[string]Place
	checkins []Checkin
	invites  []Invite
	byID     map[string]Invite
	digests  map[string]string
	member   Membership
	owner    map[string]string
	saved    *Outing
	conflict map[string][]*string
}

func (h *harness) newStub(value any, now, earlier time.Time) (*stub, error) {
	world, err := h.world(value)
	if err != nil {
		return nil, err
	}
	s := &stub{
		h: h, now: now, earlier: earlier,
		people: map[string]string{}, contexts: map[string]string{},
		members: map[string][]Member{}, outings: map[string]Outing{},
		places: map[string]Place{}, byID: map[string]Invite{},
		digests: map[string]string{}, owner: map[string]string{},
		conflict: map[string][]*string{},
	}
	if s.created, err = oracletest.Str(world["created"]); err != nil {
		return nil, err
	}
	rows, err := oracletest.Row(world["people"])
	if err != nil {
		return nil, err
	}
	for alias, name := range rows {
		text, err := oracletest.Str(name)
		if err != nil {
			return nil, err
		}
		s.people[alias] = text
	}
	if rows, err = oracletest.Row(world["contexts"]); err != nil {
		return nil, err
	}
	for alias, kind := range rows {
		text, err := oracletest.Str(kind)
		if err != nil {
			return nil, err
		}
		s.contexts[alias] = text
	}
	if rows, err = oracletest.Row(world["members"]); err != nil {
		return nil, err
	}
	for alias, raw := range rows {
		items, err := oracletest.List(raw)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			pair, err := oracletest.Strings(item)
			if err != nil || len(pair) != 2 {
				return nil, fmt.Errorf("%v is not a roster row", item)
			}
			id, err := h.id(pair[0])
			if err != nil {
				return nil, err
			}
			s.members[alias] = append(s.members[alias], Member{PersonID: id, State: pair[1]})
		}
	}
	if rows, err = oracletest.Row(world["places"]); err != nil {
		return nil, err
	}
	for id, raw := range rows {
		pair, err := fields(raw, 2)
		if err != nil {
			return nil, err
		}
		lat, err := optionalScalarFloat(pair[0])
		if err != nil {
			return nil, err
		}
		lng, err := optionalScalarFloat(pair[1])
		if err != nil {
			return nil, err
		}
		s.places[id] = Place{Lat: lat, Lng: lng}
	}
	if rows, err = oracletest.Row(world["digests"]); err != nil {
		return nil, err
	}
	for token, raw := range rows {
		invite, err := oracletest.Str(raw)
		if err != nil {
			return nil, err
		}
		s.digests[token] = invite
	}
	if rows, err = oracletest.Row(world["stop_owner"]); err != nil {
		return nil, err
	}
	for stop, raw := range rows {
		outing, err := oracletest.Str(raw)
		if err != nil {
			return nil, err
		}
		s.owner[stop] = outing
	}
	if rows, err = oracletest.Row(world["conflicts"]); err != nil {
		return nil, err
	}
	for method, raw := range rows {
		items, err := oracletest.List(raw)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			code, err := oracletest.OptionalString(item)
			if err != nil {
				return nil, err
			}
			s.conflict[method] = append(s.conflict[method], code)
		}
	}
	outings, err := oracletest.List(world["outings"])
	if err != nil {
		return nil, err
	}
	for _, raw := range outings {
		outing, err := h.outingOf(raw)
		if err != nil {
			return nil, err
		}
		s.order = append(s.order, outing)
		s.outings[fmt.Sprint(h.name(outing.ID))] = outing
	}
	invites, err := oracletest.List(world["invites"])
	if err != nil {
		return nil, err
	}
	for _, raw := range invites {
		invite, err := h.inviteOf(raw)
		if err != nil {
			return nil, err
		}
		s.invites = append(s.invites, invite)
		s.byID[fmt.Sprint(h.name(invite.ID))] = invite
	}
	checkins, err := oracletest.List(world["checkins"])
	if err != nil {
		return nil, err
	}
	for _, raw := range checkins {
		triple, err := oracletest.Strings(raw)
		if err != nil || len(triple) != 3 {
			return nil, fmt.Errorf("%v is not a check-in row", raw)
		}
		id, err := h.id(triple[0])
		if err != nil {
			return nil, err
		}
		stopID, err := h.id(triple[1])
		if err != nil {
			return nil, err
		}
		personID, err := h.id(triple[2])
		if err != nil {
			return nil, err
		}
		s.checkins = append(s.checkins, Checkin{ID: id, StopID: stopID, PersonID: personID, CreatedAt: earlier})
	}
	membership, err := oracletest.Strings(world["membership"])
	if err != nil || len(membership) != 2 {
		return nil, fmt.Errorf("%v is not a membership", world["membership"])
	}
	id, err := h.id(membership[0])
	if err != nil {
		return nil, err
	}
	s.member = Membership{ID: id, State: membership[1]}
	if raw, found := world["saved"]; found && raw != nil {
		outing, err := h.outingOf(raw)
		if err != nil {
			return nil, err
		}
		s.saved = &outing
	}
	return s, nil
}

func (s *stub) rec(name string, args ...any) {
	s.calls = append(s.calls, append([]any{name}, args...))
}

// maybeConflict is the stub's own: the head of the method's queue, where a
// null is a call that is allowed through.
func (s *stub) maybeConflict(name string) error {
	queue := s.conflict[name]
	if len(queue) == 0 {
		return nil
	}
	code := queue[0]
	s.conflict[name] = queue[1:]
	if code == nil {
		return nil
	}
	return &Conflict{Code: *code}
}

func (s *stub) alias(id string) string { return fmt.Sprint(s.h.name(id)) }

func (s *stub) IsMember(contextID, personID string) (bool, error) {
	s.rec("is_member", s.h.name(contextID), s.h.name(personID))
	for _, member := range s.members[s.alias(contextID)] {
		if member.PersonID == personID && member.State == "active" {
			return true, nil
		}
	}
	return false, nil
}

func (s *stub) GetContext(contextID string) (*Context, error) {
	s.rec("get_context", s.h.name(contextID))
	kind, found := s.contexts[s.alias(contextID)]
	if !found {
		return nil, nil
	}
	return &Context{Kind: kind}, nil
}

func (s *stub) ListMembers(contextID string) ([]Member, error) {
	s.rec("list_members", s.h.name(contextID))
	return s.members[s.alias(contextID)], nil
}

func (s *stub) GetPerson(personID string) (*Person, error) {
	s.rec("get_person", s.h.name(personID))
	name, found := s.people[s.alias(personID)]
	if !found {
		return nil, nil
	}
	return &Person{DisplayName: name}, nil
}

func (s *stub) GetPlace(placeID string) (*Place, error) {
	s.rec("get_place", placeID)
	place, found := s.places[placeID]
	if !found {
		return nil, nil
	}
	return &place, nil
}

func (s *stub) CreateOuting(draft OutingDraft) (Outing, error) {
	s.rec("create_outing", s.h.name(draft.ContextID), s.h.name(draft.CreatedByID),
		draft.Title, draft.StartsOn.ISOFormat(), draft.EndsOn.ISOFormat(),
		draft.Headcount, draft.BudgetPerPersonVND, iso(draft.Now))
	if err := s.maybeConflict("create_outing"); err != nil {
		return Outing{}, err
	}
	id, err := s.h.id(s.created)
	if err != nil {
		return Outing{}, err
	}
	return Outing{
		ID: id, ContextID: draft.ContextID, CreatedByID: draft.CreatedByID,
		Title: draft.Title, StartsOn: draft.StartsOn, EndsOn: draft.EndsOn,
		Headcount: draft.Headcount, BudgetPerPersonVND: draft.BudgetPerPersonVND,
		CreatedAt: s.earlier, TimelineRevision: 0, ItineraryVersion: 1,
	}, nil
}

func (s *stub) GetOuting(outingID string) (*Outing, error) {
	s.rec("get_outing", s.h.name(outingID))
	outing, found := s.outings[s.alias(outingID)]
	if !found {
		return nil, nil
	}
	return &outing, nil
}

func (s *stub) ListOutings(contextID string) ([]Outing, error) {
	s.rec("list_outings", s.h.name(contextID))
	var out []Outing
	for _, outing := range s.order {
		if outing.ContextID == contextID {
			out = append(out, outing)
		}
	}
	return out, nil
}

func (s *stub) saveOf(outingID string) (Outing, error) {
	if s.saved != nil {
		return *s.saved, nil
	}
	outing, found := s.outings[s.alias(outingID)]
	if !found {
		return Outing{}, fmt.Errorf("unscripted saved outing")
	}
	return outing, nil
}

func (s *stub) ReplaceOutingStops(outingID string, stops []StopDraft, expected *int64) (Outing, error) {
	rows := make([]any, 0, len(stops))
	for _, stop := range stops {
		rows = append(rows, map[string]any{
			"minute_of_day": stop.MinuteOfDay,
			"label":         stop.Label,
			"place_name":    optionalText(stop.PlaceName),
			"place_id":      optionalText(stop.PlaceID),
		})
	}
	s.rec("replace_outing_stops", s.h.name(outingID), rows, optionalInt(expected))
	if err := s.maybeConflict("replace_outing_stops"); err != nil {
		return Outing{}, err
	}
	return s.saveOf(outingID)
}

func (s *stub) ReplaceOutingItinerary(outingID string, stops []ItineraryStopDraft, days []Day, expected int64) (Outing, error) {
	rows := make([]any, 0, len(stops))
	for _, stop := range stops {
		var point any
		if stop.MeetingPoint != nil {
			point = map[string]any{
				"lat":   wireFloat(stop.MeetingPoint.Lat),
				"lng":   wireFloat(stop.MeetingPoint.Lng),
				"label": stop.MeetingPoint.Label,
			}
		}
		rows = append(rows, map[string]any{
			"label":            stop.Label,
			"place_name":       optionalText(stop.PlaceName),
			"place_id":         optionalText(stop.PlaceID),
			"id":               stop.ID,
			"day":              optionalDate(stop.Day),
			"duration_minutes": optionalInt(stop.DurationMinutes),
			"time_locked":      stop.TimeLocked,
			"meeting_point":    point,
			"minute_of_day":    stop.MinuteOfDay,
		})
	}
	configs := make([]any, 0, len(days))
	for _, day := range days {
		configs = append(configs, s.h.wireDay(day))
	}
	s.rec("replace_outing_itinerary", s.h.name(outingID), rows, configs, expected)
	if err := s.maybeConflict("replace_outing_itinerary"); err != nil {
		return Outing{}, err
	}
	return s.saveOf(outingID)
}

func (s *stub) GetOutingStop(stopID string) (*Stop, *Outing, error) {
	s.rec("get_outing_stop", s.h.name(stopID))
	owner, found := s.owner[s.alias(stopID)]
	if !found {
		return nil, nil, nil
	}
	outing, found := s.outings[owner]
	if !found {
		return nil, nil, nil
	}
	for i := range outing.Stops {
		if outing.Stops[i].ID == stopID {
			stop := outing.Stops[i]
			return &stop, &outing, nil
		}
	}
	return nil, nil, nil
}

func (s *stub) CreateStopCheckin(stopID, personID string, now time.Time) (Checkin, error) {
	s.rec("create_stop_checkin", s.h.name(stopID), s.h.name(personID), iso(now))
	if err := s.maybeConflict("create_stop_checkin"); err != nil {
		return Checkin{}, err
	}
	id, err := s.h.id("CIN")
	if err != nil {
		return Checkin{}, err
	}
	return Checkin{ID: id, StopID: stopID, PersonID: personID, CreatedAt: now}, nil
}

func (s *stub) ListOutingCheckins(outingID string) ([]Checkin, error) {
	s.rec("list_outing_checkins", s.h.name(outingID))
	return s.checkins, nil
}

func (s *stub) CreateOutingInvite(draft InviteDraft) (Invite, error) {
	s.rec("create_outing_invite", s.h.name(draft.OutingID), draft.Source,
		s.h.optionalName(draft.InvitedPersonID), s.h.name(draft.InvitedByID),
		string(draft.TokenDigest), iso(draft.ExpiresAt), iso(draft.Now))
	if err := s.maybeConflict("create_outing_invite"); err != nil {
		return Invite{}, err
	}
	id, err := s.h.id("IVN")
	if err != nil {
		return Invite{}, err
	}
	return Invite{
		ID: id, OutingID: draft.OutingID, Source: draft.Source,
		InvitedPersonID: draft.InvitedPersonID, InvitedByID: draft.InvitedByID,
		CreatedAt: s.earlier, ExpiresAt: draft.ExpiresAt,
	}, nil
}

func (s *stub) FindOutingInviteForPerson(outingID, personID string) (*Invite, error) {
	s.rec("find_outing_invite_for_person", s.h.name(outingID), s.h.name(personID))
	for i := range s.invites {
		invite := s.invites[i]
		if invite.OutingID == outingID && invite.InvitedPersonID != nil && *invite.InvitedPersonID == personID {
			return &invite, nil
		}
	}
	return nil, nil
}

func (s *stub) GetOutingInvite(inviteID string) (*Invite, error) {
	s.rec("get_outing_invite", s.h.name(inviteID))
	invite, found := s.byID[s.alias(inviteID)]
	if !found {
		return nil, nil
	}
	return &invite, nil
}

func (s *stub) GetOutingInviteByDigest(digest []byte) (*Invite, error) {
	s.rec("get_outing_invite_by_digest", string(digest))
	name, found := s.digests[string(digest)]
	if !found {
		return nil, nil
	}
	invite, found := s.byID[name]
	if !found {
		return nil, nil
	}
	return &invite, nil
}

func (s *stub) AcceptOutingInvite(inviteID, acceptedByID string, now time.Time) (Invite, error) {
	s.rec("accept_outing_invite", s.h.name(inviteID), s.h.name(acceptedByID), iso(now))
	if err := s.maybeConflict("accept_outing_invite"); err != nil {
		return Invite{}, err
	}
	invite, found := s.byID[s.alias(inviteID)]
	if !found {
		return Invite{}, fmt.Errorf("unscripted invite")
	}
	return invite, nil
}

func (s *stub) RevokeOutingInvite(inviteID string, now time.Time) (Invite, error) {
	s.rec("revoke_outing_invite", s.h.name(inviteID), iso(now))
	if err := s.maybeConflict("revoke_outing_invite"); err != nil {
		return Invite{}, err
	}
	invite, found := s.byID[s.alias(inviteID)]
	if !found {
		return Invite{}, fmt.Errorf("unscripted invite")
	}
	invite.RevokedAt = &now
	return invite, nil
}

func (s *stub) RotateOutingInviteDigest(inviteID string, digest []byte, expires, now time.Time) (Invite, error) {
	s.rec("rotate_outing_invite_digest", s.h.name(inviteID), string(digest), iso(expires), iso(now))
	if err := s.maybeConflict("rotate_outing_invite_digest"); err != nil {
		return Invite{}, err
	}
	invite, found := s.byID[s.alias(inviteID)]
	if !found {
		return Invite{}, fmt.Errorf("unscripted invite")
	}
	invite.ExpiresAt = expires
	return invite, nil
}

func (s *stub) EnsureInvitedMembership(draft MembershipDraft) (Membership, error) {
	s.rec("ensure_invited_membership", s.h.name(draft.ContextID), s.h.name(draft.PersonID),
		s.h.name(draft.InvitedByID), draft.Origin, iso(draft.Now))
	if err := s.maybeConflict("ensure_invited_membership"); err != nil {
		return Membership{}, err
	}
	return s.member, nil
}

// mintedToken is the word scripts/render_domain_w7_goldens.py pins
// secrets.token_urlsafe to, and the alias its digest is recorded under.
type mintedToken struct {
	raw   string
	alias string
}

func (m mintedToken) NewInviteToken() (string, []byte, error) {
	return m.raw, []byte(m.alias), nil
}

var _ Store = (*stub)(nil)
var _ Secrets = mintedToken{}

func (s *stub) recorded() []any {
	if s.calls == nil {
		return []any{}
	}
	return s.calls
}

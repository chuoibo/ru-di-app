package outingsteps

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"mobile/services/core/internal/domain/pairpaper"
	"mobile/services/core/internal/domain/permissions"
	"mobile/services/core/internal/oracletest"
	"mobile/services/core/internal/pyjson"
)

// testdata/python_outing_steps*.json is rendered by
// scripts/render_domain_w7_goldens.py by running the real outing methods of
// app.api.service over a recording stub repository, with the clock pinned to
// each case's `now` and `secrets.token_urlsafe` pinned to one word. Every case
// is replayed here through a recording Store: the answer (or the ApiProblem,
// or the exception that is a 500) and every call with its arguments must
// match.

// methods is every ported service method, in the order of the script's
// CALLERS.
var methods = []string{
	"create_outing", "list_context_outings", "replace_outing_timeline",
	"authorize_outing_itinerary", "preview_outing_itinerary",
	"replace_outing_itinerary", "check_in_to_stop", "list_outing_checkins",
	"create_outing_invite", "accept_outing_invite",
	"rotate_outing_invite_secret", "revoke_outing_invite",
}

// problemCodes is every ApiProblem code the committed corpus must show.
var problemCodes = []string{
	"permission_denied", "outing_not_found", "stop_place_unknown",
	"timeline_conflict", "itinerary_upgrade_required", "stop_not_found",
	"itinerary_day_invalid", "itinerary_anchor_invalid", "already_checked_in",
	"not_a_group", "person_not_registered", "participant_not_in_context",
	"invite_already_exists", "invite_not_found", "invite_already_accepted",
	"invite_not_named",
}

// raisedTypes is every exception class the committed corpus must show ending a
// request as a 500.
var raisedTypes = []string{"PermissionError_", "RepositoryConflict", "ValueError"}

func asRefusal(err error) (class, code string, ok bool) {
	var denied *permissions.Error
	if errors.As(err, &denied) {
		return "PermissionError_", denied.Code, true
	}
	var conflict *Conflict
	if errors.As(err, &conflict) {
		return "RepositoryConflict", conflict.Code, true
	}
	return "", "", false
}

// --- the world --------------------------------------------------------------

type harness struct {
	ids      map[string]string
	names    map[string]string
	tokens   map[string]string
	defaults map[string]any
	earlier  time.Time
}

func newHarness(t testing.TB, constants map[string]any) *harness {
	t.Helper()
	h := &harness{ids: map[string]string{}, names: map[string]string{}, tokens: map[string]string{}}
	rows, err := oracletest.List(constants["aliases"])
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range rows {
		pair, err := oracletest.Strings(raw)
		if err != nil || len(pair) != 2 {
			t.Fatalf("alias %v", raw)
		}
		h.ids[pair[0]] = pair[1]
		h.names[pair[1]] = pair[0]
	}
	tokens, err := oracletest.List(constants["tokens"])
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range tokens {
		pair, err := oracletest.Strings(raw)
		if err != nil || len(pair) != 2 {
			t.Fatalf("token %v", raw)
		}
		h.tokens[pair[0]] = pair[1]
	}
	if h.defaults, err = oracletest.Row(constants["world_defaults"]); err != nil {
		t.Fatal(err)
	}
	if h.earlier, err = oracletest.Instant(constants["earlier"]); err != nil {
		t.Fatal(err)
	}
	return h
}

// id is an alias as the service sees it: the canonical uuid string. A stop id
// a client sends is that same spelling, and a draft id (`tmp-...`) is itself.
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

// world merges a case's overrides onto the script's WORLD_DEFAULTS, which is
// what the Python stub does with `{**WORLD_DEFAULTS, **world}`.
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

func iso(t time.Time) any { return pairpaper.ISOFormat(t) }

func optionalISO(t *time.Time) any {
	if t == nil {
		return nil
	}
	return pairpaper.ISOFormat(*t)
}

func optionalText(text *string) any {
	if text == nil {
		return nil
	}
	return *text
}

func optionalDate(day *Date) any {
	if day == nil {
		return nil
	}
	return day.ISOFormat()
}

func optionalInt(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

func wireFloat(value float64) any {
	return map[string]any{"float": pyjson.FloatRepr(value)}
}

func optionalFloatWire(value *float64) any {
	if value == nil {
		return nil
	}
	return wireFloat(*value)
}

func fields(value any, n int) ([]any, error) {
	items, err := oracletest.List(value)
	if err != nil || len(items) != n {
		return nil, fmt.Errorf("%v is not a record of %d fields", value, n)
	}
	return items, nil
}

func parseDate(value any) (Date, error) {
	year, month, day, err := oracletest.CivilDate(value)
	return Date{Year: year, Month: month, Day: day}, err
}

func optionalParseDate(value any) (*Date, error) {
	if value == nil {
		return nil, nil
	}
	day, err := parseDate(value)
	if err != nil {
		return nil, err
	}
	return &day, nil
}

func optionalInstant(value any) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}
	moment, err := oracletest.Instant(value)
	return &moment, err
}

func optionalNumber(value any) (*int64, error) {
	if value == nil {
		return nil, nil
	}
	number, err := oracletest.Int64(value)
	return &number, err
}

func optionalScalarFloat(value any) (*float64, error) {
	if value == nil {
		return nil, nil
	}
	row, err := oracletest.Row(value, "float")
	if err != nil {
		return nil, err
	}
	text, err := oracletest.Str(row["float"])
	if err != nil {
		return nil, err
	}
	var number float64
	if _, err := fmt.Sscanf(text, "%g", &number); err != nil {
		return nil, fmt.Errorf("%q is not a float repr", text)
	}
	return &number, nil
}

// stopOf reads the script's STOP_KEYS record.
func (h *harness) stopOf(value any) (Stop, error) {
	f, err := fields(value, 12)
	if err != nil {
		return Stop{}, err
	}
	id, err := h.id(f[0])
	if err != nil {
		return Stop{}, err
	}
	position, err := oracletest.Int64(f[1])
	if err != nil {
		return Stop{}, err
	}
	minute, err := oracletest.Int64(f[2])
	if err != nil {
		return Stop{}, err
	}
	label, err := oracletest.Str(f[3])
	if err != nil {
		return Stop{}, err
	}
	placeName, err := oracletest.OptionalString(f[4])
	if err != nil {
		return Stop{}, err
	}
	placeID, err := oracletest.OptionalString(f[5])
	if err != nil {
		return Stop{}, err
	}
	day, err := optionalParseDate(f[6])
	if err != nil {
		return Stop{}, err
	}
	dwell, err := optionalNumber(f[7])
	if err != nil {
		return Stop{}, err
	}
	locked, err := oracletest.Bool(f[8])
	if err != nil {
		return Stop{}, err
	}
	lat, err := optionalScalarFloat(f[9])
	if err != nil {
		return Stop{}, err
	}
	lng, err := optionalScalarFloat(f[10])
	if err != nil {
		return Stop{}, err
	}
	label2, err := oracletest.OptionalString(f[11])
	if err != nil {
		return Stop{}, err
	}
	return Stop{
		ID: id, Position: int(position), MinuteOfDay: minute, Label: label,
		PlaceName: placeName, PlaceID: placeID, Day: day, DurationMinutes: dwell,
		TimeLocked: locked, MeetingLat: lat, MeetingLng: lng, MeetingLabel: label2,
	}, nil
}

// dayOf reads the script's DRAFT_DAY_KEYS record.
func (h *harness) dayOf(value any) (Day, error) {
	f, err := fields(value, 6)
	if err != nil {
		return Day{}, err
	}
	day, err := parseDate(f[0])
	if err != nil {
		return Day{}, err
	}
	mode, err := oracletest.Str(f[1])
	if err != nil {
		return Day{}, err
	}
	start, err := oracletest.Str(f[2])
	if err != nil {
		return Day{}, err
	}
	// A day's anchors are the client's own strings, never uuid objects, so
	// they are stored and written back exactly as they arrived. Only a
	// REQUEST day's anchors are aliases a fixture spelled short, and
	// requestDayOf maps those.
	startStop, err := oracletest.OptionalString(f[3])
	if err != nil {
		return Day{}, err
	}
	endStop, err := oracletest.OptionalString(f[4])
	if err != nil {
		return Day{}, err
	}
	returning, err := oracletest.Bool(f[5])
	if err != nil {
		return Day{}, err
	}
	return Day{
		Day: day, TransportMode: mode, StartAt: start,
		StartStopID: startStop, EndStopID: endStop, ReturnToStart: returning,
	}, nil
}

func (h *harness) wireDay(day Day) any {
	return []any{
		day.Day.ISOFormat(), day.TransportMode, day.StartAt,
		optionalText(day.StartStopID), optionalText(day.EndStopID), day.ReturnToStart,
	}
}

// requestDayOf is dayOf for a day of the request body, whose anchors a fixture
// writes as aliases because they name stops.
func (h *harness) requestDayOf(value any) (Day, error) {
	day, err := h.dayOf(value)
	if err != nil {
		return Day{}, err
	}
	if day.StartStopID, err = h.optionalID(optionalText(day.StartStopID)); err != nil {
		return Day{}, err
	}
	if day.EndStopID, err = h.optionalID(optionalText(day.EndStopID)); err != nil {
		return Day{}, err
	}
	return day, nil
}

// outingOf reads the script's OUTING_KEYS record.
func (h *harness) outingOf(value any) (Outing, error) {
	f, err := fields(value, 13)
	if err != nil {
		return Outing{}, err
	}
	id, err := h.id(f[0])
	if err != nil {
		return Outing{}, err
	}
	contextID, err := h.id(f[1])
	if err != nil {
		return Outing{}, err
	}
	by, err := h.id(f[2])
	if err != nil {
		return Outing{}, err
	}
	title, err := oracletest.Str(f[3])
	if err != nil {
		return Outing{}, err
	}
	starts, err := parseDate(f[4])
	if err != nil {
		return Outing{}, err
	}
	ends, err := parseDate(f[5])
	if err != nil {
		return Outing{}, err
	}
	headcount, err := oracletest.Int64(f[6])
	if err != nil {
		return Outing{}, err
	}
	budget, err := oracletest.Int64(f[7])
	if err != nil {
		return Outing{}, err
	}
	created, err := oracletest.Instant(f[8])
	if err != nil {
		return Outing{}, err
	}
	revision, err := oracletest.Int64(f[10])
	if err != nil {
		return Outing{}, err
	}
	version, err := oracletest.Int64(f[11])
	if err != nil {
		return Outing{}, err
	}
	outing := Outing{
		ID: id, ContextID: contextID, CreatedByID: by, Title: title,
		StartsOn: starts, EndsOn: ends, Headcount: headcount,
		BudgetPerPersonVND: budget, CreatedAt: created,
		TimelineRevision: revision, ItineraryVersion: version,
	}
	stops, err := oracletest.List(f[9])
	if err != nil {
		return Outing{}, err
	}
	for _, raw := range stops {
		stop, err := h.stopOf(raw)
		if err != nil {
			return Outing{}, err
		}
		outing.Stops = append(outing.Stops, stop)
	}
	days, err := oracletest.List(f[12])
	if err != nil {
		return Outing{}, err
	}
	for _, raw := range days {
		day, err := h.dayOf(raw)
		if err != nil {
			return Outing{}, err
		}
		outing.ItineraryDays = append(outing.ItineraryDays, day)
	}
	return outing, nil
}

// inviteOf reads the script's INVITE_KEYS record.
func (h *harness) inviteOf(value any) (Invite, error) {
	f, err := fields(value, 10)
	if err != nil {
		return Invite{}, err
	}
	id, err := h.id(f[0])
	if err != nil {
		return Invite{}, err
	}
	outingID, err := h.id(f[1])
	if err != nil {
		return Invite{}, err
	}
	source, err := oracletest.Str(f[2])
	if err != nil {
		return Invite{}, err
	}
	person, err := h.optionalID(f[3])
	if err != nil {
		return Invite{}, err
	}
	by, err := h.id(f[4])
	if err != nil {
		return Invite{}, err
	}
	accepted, err := optionalInstant(f[5])
	if err != nil {
		return Invite{}, err
	}
	acceptedBy, err := h.optionalID(f[6])
	if err != nil {
		return Invite{}, err
	}
	created, err := oracletest.Instant(f[7])
	if err != nil {
		return Invite{}, err
	}
	expires, err := oracletest.Instant(f[8])
	if err != nil {
		return Invite{}, err
	}
	revoked, err := optionalInstant(f[9])
	if err != nil {
		return Invite{}, err
	}
	return Invite{
		ID: id, OutingID: outingID, Source: source, InvitedPersonID: person,
		InvitedByID: by, AcceptedAt: accepted, AcceptedByID: acceptedBy,
		CreatedAt: created, ExpiresAt: expires, RevokedAt: revoked,
	}, nil
}

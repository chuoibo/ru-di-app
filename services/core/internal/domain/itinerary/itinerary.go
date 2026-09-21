// Package itinerary is services/api/app/journey/preview.py: the read-only
// comparison of one day's complete routes.
//
// The Valhalla call itself is outside, behind Router, for the reason
// app/journey/routing.py exists as a separate module -- a coordinate goes to
// the configured private service and nowhere else, and opening the socket is
// not something a pure layer may do. Everything around it is here: which
// refusals are checked in which order, how a day's settings and its stops are
// found, the shaping of legs into segments, and the rule that decides whether
// a reordering is worth showing.
//
// Two things the Python reads from the process arrive as Env instead, because
// they are the process talking rather than the request:
// MOBILE_JOURNEY_SUGGESTIONS_ENABLED, and whether the four-slot semaphore
// _CAPACITY had a slot free. Releasing the slot is the caller's `finally`.
//
// A Draft is `request.model_dump(mode="json")` with the three keys
// ApiService._itinerary_draft injects: `lat`, `lng` and `checked_in`. Dates are
// already their isoformat() text, which is why a day is compared as a string
// here and never parsed.
//
// Lat, Lng and DurationMinutes stay `any`, unlike every other field, because
// Python's own type tests on them are load-bearing: a bool is refused although
// it is an int, a float NaN is refused although it is a number, and
// `type(dwell) is not int` refuses a float dwell that would have added
// perfectly well. Everything else is the shape pydantic already proved.
//
// testdata/python_itinerary*.json is rendered by
// scripts/render_domain_w7_goldens.py by calling the real preview_itinerary in
// the parity API image over a scripted provider, and oracle_test.go replays
// every case through a Router that answers from the same script.
package itinerary

import (
	"math"

	"mobile/services/core/internal/domain/journey"
	"mobile/services/core/internal/domain/valhalla"
)

// Point is one location handed to the routing service: `{"lat": ..., "lon":
// ...}`, longitude under the key Valhalla spells `lon`.
//
// The two values are `any` for the same reason Stop.Lat is. `_route` copies
// the draft's coordinate into the payload without touching it, so an int
// coordinate is sent as `{"lat": 10}` and a float one as `{"lat": 10.0}`.
// Nothing a client can send makes an int coordinate -- MeetingPoint.lat is a
// strict float and a catalogue row's is a double -- but the copy is the copy,
// and a Router serialising the payload has to know which it was holding.
// Whatever arrives here has already passed `located`, so it is an int64 or a
// float64 and never a bool, a NaN or anything else.
type Point struct {
	Lat any
	Lng any
}

// Router is RoutingProvider. A nil Router is `configured_provider()` finding no
// configuration, which is an answer of its own rather than a failure.
//
// Both methods answer valhalla.ErrRoutingUnavailable when the private service
// fails; nothing else about the failure may cross this boundary.
type Router interface {
	GraphVersion() string
	Route(points []Point, mode string) ([]valhalla.Leg, error)
	Matrix(points []Point, mode string) (journey.Matrix, error)
}

// Env is what preview_itinerary reads from the process rather than the request.
type Env struct {
	// SuggestionsEnabled is MOBILE_JOURNEY_SUGGESTIONS_ENABLED not being "0".
	// The variable is absent by default, and absent means enabled.
	SuggestionsEnabled bool
	// CapacityAvailable is _CAPACITY.acquire(blocking=False): one of the four
	// concurrent routing slots was free.
	CapacityAvailable bool
}

// MeetingPoint is a deliberately chosen meeting place, never device location.
type MeetingPoint struct {
	Lat   float64
	Lng   float64
	Label string
}

// Stop is one stop of the draft.
type Stop struct {
	ID    string
	At    string
	Label string
	// PlaceName and PlaceID are carried, never read here. They are keys of
	// the model_dump the service hands over, and a draft that lost them would
	// not be the draft Python built.
	PlaceName *string
	PlaceID   *string
	// Day is the stop's day as isoformat() text; nil is None, a stop nobody
	// has put on a day yet.
	Day *string
	// DurationMinutes is `duration_minutes`: nil, an int64, or -- only from a
	// caller that is not the service -- something else, which is refused.
	DurationMinutes any
	TimeLocked      bool
	MeetingPoint    *MeetingPoint
	// Lat and Lng are what _itinerary_draft resolved: the catalogue row's
	// coordinates, the meeting point's, or nil.
	Lat any
	Lng any
	// CheckedIn is whether somebody has already pressed "đã tới" here.
	CheckedIn bool
}

// DayConfig is one entry of `days`: how a single day travels.
type DayConfig struct {
	Day           string
	TransportMode string
	StartAt       string
	StartStopID   *string
	EndStopID     *string
	ReturnToStart bool
}

// Draft is the itinerary being previewed.
type Draft struct {
	ExpectedRevision  int64
	Day               string
	IncludeSuggestion bool
	Stops             []Stop
	Days              []DayConfig
}

// Segment is one leg of a routed day, named by the stops at its ends. The keys
// reach the wire in this order: distance_meters, duration_seconds, geometry,
// source, from_stop_id, to_stop_id -- `{**leg, ...}` keeps the leg's own order
// and appends the two ids.
type Segment struct {
	Leg        valhalla.Leg
	FromStopID string
	ToStopID   string
}

// Route is what `_route` answers: a schedule, with the roads that produced it.
// The keys reach the wire in this order: stops, feasible, issues, segments,
// distance_meters, duration_seconds.
type Route struct {
	Stops           []journey.Row
	Feasible        bool
	Issues          []journey.Issue
	Segments        []Segment
	DistanceMeters  int64
	DurationSeconds int64
}

// Savings is how much shorter the suggestion is: distance first, then time.
type Savings struct {
	DistanceMeters  int64
	DurationSeconds int64
}

// Preview is preview_itinerary's answer. GraphVersion is
// `source.graph_version`, nil when no provider is configured; `source.engine`
// is always "valhalla" and `source.traffic` always "none".
type Preview struct {
	Revision int64
	Day      string
	Status   string
	// GraphVersion is nil for None.
	GraphVersion *string
	Current      *Route
	Suggestion   *Route
	Savings      *Savings
	Issues       []journey.Issue
}

// Engine and Traffic are the constant halves of `source`.
const (
	Engine  = "valhalla"
	Traffic = "none"
)

// Status values.
const (
	StatusIncomplete  = "incomplete"
	StatusReady       = "ready"
	StatusUnavailable = "unavailable"
)

// Build is preview_itinerary: the caller has already authorized the outing,
// resolved every location and derived the check-ins.
//
// The error return is only for what Python does not catch: a ValueError out of
// journey.Minute reaching Schedule after the validation pass said the times
// were fine, or an IndexError from a provider that returned a matrix of the
// wrong shape. A routing failure is not an error here; it is a status and an
// issue, which is the whole point of the function.
func Build(draft Draft, router Router, env Env) (Preview, error) {
	result := Preview{
		Revision: draft.ExpectedRevision,
		Day:      draft.Day,
		Status:   StatusIncomplete,
		Issues:   []journey.Issue{},
	}
	if router != nil {
		version := router.GraphVersion()
		result.GraphVersion = &version
	}
	add := func(code, message string, stopID *string) {
		result.Issues = append(result.Issues, journey.NewIssue(code, message, stopID))
	}

	all := draft.Stops
	seen := make(map[string]bool, len(all))
	for _, stop := range all {
		seen[stop.ID] = true
	}
	if len(all) > journey.MaxStops || len(seen) != len(all) {
		add("invalid_stops", "Lịch trình cần ID riêng cho từng chặng và tối đa 50 chặng.", nil)
		return result, nil
	}
	var settings *DayConfig
	for i := range draft.Days {
		if draft.Days[i].Day == draft.Day {
			settings = &draft.Days[i]
			break
		}
	}
	known := false
	if settings != nil {
		_, known = valhalla.Costing(settings.TransportMode)
	}
	if !known {
		add("missing_day_settings", "Chọn phương tiện và giờ xuất phát cho ngày này.", nil)
		return result, nil
	}
	var stops []Stop
	for _, stop := range all {
		if stop.Day != nil && *stop.Day == draft.Day {
			stops = append(stops, stop)
		}
	}
	if len(stops) == 0 {
		add("empty_day", "Thêm điểm hẹn cho ngày này.", nil)
		return result, nil
	}
	for _, stop := range all {
		if stop.Day == nil {
			id := stop.ID
			add("unassigned_day", "Chọn ngày cho chặng này trước khi so sánh.", &id)
		}
	}
	if !validSchedule(*settings, stops) {
		add("invalid_schedule", "Giờ hoặc thời lượng chưa hợp lệ.", nil)
		return result, nil
	}
	missingLocation := false
	for _, stop := range stops {
		if !located(stop.Lat, stop.Lng) {
			id := stop.ID
			add("missing_location", "Chọn vị trí cho điểm hẹn này.", &id)
			missingLocation = true
		}
	}
	if missingLocation {
		return result, nil
	}
	for _, pair := range [2]struct {
		anchor *string
		actual string
	}{
		{settings.StartStopID, stops[0].ID},
		{settings.EndStopID, stops[len(stops)-1].ID},
	} {
		// `if settings.get(key) and settings[key] != actual`: an anchor that
		// is None, and an anchor that is the empty string, are both "no
		// anchor". The schema allows the empty string, so the second is
		// reachable from a client.
		if pair.anchor != nil && *pair.anchor != "" && *pair.anchor != pair.actual {
			id := *pair.anchor
			add("anchor_mismatch", "Đặt điểm đầu/cuối đã chọn đúng vị trí trong lịch trình.", &id)
		}
	}
	if router == nil {
		result.Status = StatusUnavailable
		add("routing_unavailable", "Chưa kết nối được dữ liệu đường đi. Thử lại sau.", nil)
		return result, nil
	}
	if !env.CapacityAvailable {
		result.Status = StatusUnavailable
		add("routing_busy", "Đang có nhiều yêu cầu tính đường. Thử lại sau.", nil)
		return result, nil
	}
	preview, err := routed(&result, draft, stops, *settings, router, env, add)
	if err != nil {
		return Preview{}, err
	}
	return preview, nil
}

// routed is the body of preview_itinerary's try block. Every
// RoutingUnavailable inside it -- the current route's, the matrix lookup's and
// the candidate's alike -- lands on the same answer, which is why a verified
// current route is thrown away when only the recommendation failed.
func routed(
	result *Preview,
	draft Draft,
	stops []Stop,
	settings DayConfig,
	router Router,
	env Env,
	add func(code, message string, stopID *string),
) (Preview, error) {
	// A verified current route is kept when only the recommendation lookup
	// failed: the except block sets the status and adds the issue and touches
	// nothing else, so a day that routed fine still shows its own schedule
	// under an "unavailable" heading.
	unavailable := func() Preview {
		result.Status = StatusUnavailable
		add("routing_unavailable", "Chưa tính đủ đường đi để so sánh. Thử lại sau.", nil)
		return *result
	}
	current, err := route(stops, settings, router)
	if err != nil {
		if err == valhalla.ErrRoutingUnavailable {
			return unavailable(), nil
		}
		return Preview{}, err
	}
	result.Current = current
	result.Issues = append(result.Issues, current.Issues...)
	if len(result.Issues) > 0 {
		current.Feasible = false
		result.Status = StatusIncomplete
	} else {
		result.Status = StatusReady
	}
	if !env.SuggestionsEnabled {
		if draft.IncludeSuggestion {
			add("suggestions_disabled", "Đề xuất đang tạm nghỉ. Bạn vẫn có thể xem và sửa lịch trình.", nil)
		}
		return *result, nil
	}
	// Missing facts cannot support a safe recommendation. A late appointment
	// alone can be repaired by a genuinely feasible alternate road ordering.
	if !draft.IncludeSuggestion {
		return *result, nil
	}
	for _, i := range result.Issues {
		if i.Code != "late_fixed_stop" {
			return *result, nil
		}
	}
	matrix, err := router.Matrix(points(stops, false), settings.TransportMode)
	if err != nil {
		if err == valhalla.ErrRoutingUnavailable {
			return unavailable(), nil
		}
		return Preview{}, err
	}
	order, err := journey.SuggestOrder(planStops(stops), matrix, planSettings(settings))
	if err != nil {
		return Preview{}, err
	}
	if unchanged(order) {
		return *result, nil
	}
	reordered := make([]Stop, len(order))
	for i, index := range order {
		reordered[i] = stops[index]
	}
	candidate, err := route(reordered, settings, router)
	if err != nil {
		if err == valhalla.ErrRoutingUnavailable {
			return unavailable(), nil
		}
		return Preview{}, err
	}
	// Matrix search is only a candidate generator. Whole-route costs decide.
	improves := candidate.DurationSeconds < current.DurationSeconds ||
		(candidate.DurationSeconds == current.DurationSeconds && candidate.DistanceMeters < current.DistanceMeters)
	if candidate.Feasible && (improves || !current.Feasible) {
		result.Suggestion = candidate
		result.Savings = &Savings{
			DistanceMeters:  current.DistanceMeters - candidate.DistanceMeters,
			DurationSeconds: current.DurationSeconds - candidate.DurationSeconds,
		}
	}
	return *result, nil
}

// unchanged is `order == list(range(len(stops)))`.
func unchanged(order []int) bool {
	for i, index := range order {
		if index != i {
			return false
		}
	}
	return true
}

// route is `_route`: ask the provider for the whole day's roads, schedule
// them, and report what the day costs.
func route(stops []Stop, settings DayConfig, router Router) (*Route, error) {
	pts := points(stops, settings.ReturnToStart)
	ids := make([]string, 0, len(stops)+1)
	for _, stop := range stops {
		ids = append(ids, stop.ID)
	}
	if settings.ReturnToStart && len(stops) > 1 {
		ids = append(ids, ids[0])
	}
	legs, err := router.Route(pts, settings.TransportMode)
	if err != nil {
		return nil, err
	}
	want := 0
	if len(pts) > 1 {
		want = len(pts) - 1
	}
	if len(legs) != want {
		return nil, valhalla.ErrRoutingUnavailable
	}
	costs := make([]*journey.Cost, 0, len(legs))
	for _, leg := range legs {
		costs = append(costs, &journey.Cost{Seconds: leg.DurationSeconds, Meters: leg.DistanceMeters})
	}
	plan, err := journey.Schedule(planStops(stops), costs, planSettings(settings))
	if err != nil {
		return nil, err
	}
	segments := make([]Segment, 0, len(legs))
	var meters, seconds int64
	for i, leg := range legs {
		segments = append(segments, Segment{Leg: leg, FromStopID: ids[i], ToStopID: ids[i+1]})
		meters += leg.DistanceMeters
		seconds += leg.DurationSeconds
	}
	return &Route{
		Stops:           plan.Stops,
		Feasible:        plan.Feasible,
		Issues:          plan.Issues,
		Segments:        segments,
		DistanceMeters:  meters,
		DurationSeconds: seconds,
	}, nil
}

// points is the list of coordinates a day's stops make, with the first
// repeated at the end when the day returns to where it started.
func points(stops []Stop, returning bool) []Point {
	out := make([]Point, 0, len(stops)+1)
	for _, stop := range stops {
		out = append(out, Point{Lat: stop.Lat, Lng: stop.Lng})
	}
	if returning && len(out) > 1 {
		out = append(out, out[0])
	}
	return out
}

// planStops is the view app/domain/journey.py takes of a draft stop.
func planStops(stops []Stop) []journey.Stop {
	out := make([]journey.Stop, 0, len(stops))
	for _, stop := range stops {
		out = append(out, journey.Stop{
			ID:              stop.ID,
			At:              stop.At,
			DurationMinutes: asDwell(stop.DurationMinutes),
			TimeLocked:      stop.TimeLocked,
			CheckedIn:       stop.CheckedIn,
		})
	}
	return out
}

func planSettings(settings DayConfig) journey.Settings {
	return journey.Settings{
		StartAt:       settings.StartAt,
		ReturnToStart: settings.ReturnToStart,
		EndStopID:     settings.EndStopID,
	}
}

// validSchedule is the try block that reads every time and every dwell before
// any of them is scheduled: `minute()` on the day's start and on each stop of
// the day, and the shape of each dwell. It answers false where Python catches
// KeyError, ValueError or TypeError.
func validSchedule(settings DayConfig, stops []Stop) bool {
	if _, err := journey.Minute(settings.StartAt); err != nil {
		return false
	}
	for _, stop := range stops {
		if _, err := journey.Minute(stop.At); err != nil {
			return false
		}
		if stop.DurationMinutes == nil {
			continue
		}
		// `type(dwell) is not int`: exactly int, so a bool is refused here
		// although Python would have added it, and so is a float.
		dwell, ok := stop.DurationMinutes.(int64)
		if !ok || dwell < 0 || dwell > 1440 {
			return false
		}
	}
	return true
}

// located is the coordinate test: both values must be real numbers -- not
// bools, not NaN, not an infinity -- and inside the latitude and longitude
// ranges. The range comparisons are only reached once the type test has
// passed, which is what keeps them from comparing a string.
func located(lat, lng any) bool {
	for _, value := range [2]any{lat, lng} {
		if !isFinite(value) {
			return false
		}
	}
	return -90 <= asFloat(lat) && asFloat(lat) <= 90 && -180 <= asFloat(lng) && asFloat(lng) <= 180
}

// isFinite is `not (isinstance(v, bool) or not isinstance(v, int|float) or not
// math.isfinite(v))`.
func isFinite(value any) bool {
	switch v := value.(type) {
	case bool:
		return false
	case int64:
		return true
	case float64:
		return !math.IsNaN(v) && !math.IsInf(v, 0)
	}
	return false
}

// asFloat reads a value isFinite has already accepted.
func asFloat(value any) float64 {
	switch v := value.(type) {
	case int64:
		return float64(v)
	case float64:
		return v
	}
	return 0
}

// asDwell reads a dwell validSchedule has already accepted as an int or None.
func asDwell(value any) *int64 {
	if dwell, ok := value.(int64); ok {
		return &dwell
	}
	return nil
}

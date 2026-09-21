package itinerary

import (
	"fmt"
	"strconv"
	"testing"

	"mobile/services/core/internal/domain/journey"
	"mobile/services/core/internal/domain/valhalla"
	"mobile/services/core/internal/oracletest"
	"mobile/services/core/internal/pyjson"
)

// testdata/python_itinerary*.json is rendered by
// scripts/render_domain_w7_goldens.py by calling the real preview_itinerary in
// the parity API image over a scripted provider, with _CAPACITY and
// MOBILE_JOURNEY_SUGGESTIONS_ENABLED set by the case. Every case is replayed
// here through a Router that answers from the same script and records the same
// calls: the answer, and every call with its coordinates and its mode.

// statuses and issueCodes are what the committed corpus must show.
var (
	statuses   = []string{StatusIncomplete, StatusReady, StatusUnavailable}
	issueCodes = []string{
		"invalid_stops", "missing_day_settings", "empty_day", "unassigned_day",
		"invalid_schedule", "missing_location", "anchor_mismatch",
		"routing_unavailable", "routing_busy", "suggestions_disabled",
		"late_fixed_stop",
	}
)

// --- the scripted provider ---------------------------------------------------

const failAnswer = "fail"

type scriptedRouter struct {
	graphVersion string
	route        []any
	matrix       []any
	calls        *[]any
}

func (r *scriptedRouter) GraphVersion() string { return r.graphVersion }

// answer is the script's _answer: record the call, then take the head of the
// queue, or repeat its last entry, or fail when there is nothing at all.
func (r *scriptedRouter) answer(kind string, queue *[]any, points []Point, mode string) (any, error) {
	coordinates := make([]any, 0, len(points))
	for _, point := range points {
		coordinates = append(coordinates, []any{wireScalar(point.Lat), wireScalar(point.Lng)})
	}
	*r.calls = append(*r.calls, []any{kind, coordinates, mode})
	if len(*queue) == 0 {
		return nil, valhalla.ErrRoutingUnavailable
	}
	head := (*queue)[0]
	if len(*queue) > 1 {
		*queue = (*queue)[1:]
	}
	if text, ok := head.(string); ok && text == failAnswer {
		return nil, valhalla.ErrRoutingUnavailable
	}
	return head, nil
}

func (r *scriptedRouter) Route(points []Point, mode string) ([]valhalla.Leg, error) {
	head, err := r.answer("route", &r.route, points, mode)
	if err != nil {
		return nil, err
	}
	return decodeLegs(head)
}

func (r *scriptedRouter) Matrix(points []Point, mode string) (journey.Matrix, error) {
	head, err := r.answer("matrix", &r.matrix, points, mode)
	if err != nil {
		return nil, err
	}
	return decodeMatrix(head)
}

// --- decoding ----------------------------------------------------------------

func wireFloat(value float64) any {
	return map[string]any{"float": pyjson.FloatRepr(value)}
}

// wireScalar writes back whatever `located` accepted, keeping an int an int.
func wireScalar(value any) any {
	if number, ok := value.(float64); ok {
		return wireFloat(number)
	}
	return value
}

// scalar reads a value the script wrote with enc: nil, a bool, an int, a str,
// or a float as {"float": repr}. It is what Lat, Lng and DurationMinutes are.
func scalar(value any) (any, error) {
	switch v := value.(type) {
	case nil, bool, int64, string:
		return v, nil
	case map[string]any:
		text, ok := v["float"]
		if !ok || len(v) != 1 {
			return nil, fmt.Errorf("%v is not a scalar", v)
		}
		spelling, err := oracletest.Str(text)
		if err != nil {
			return nil, err
		}
		return strconv.ParseFloat(spelling, 64)
	}
	return nil, fmt.Errorf("unexpected scalar %T", value)
}

// fields reads a positional record of exactly n values.
func fields(value any, n int) ([]any, error) {
	items, err := oracletest.List(value)
	if err != nil || len(items) != n {
		return nil, fmt.Errorf("%v is not a record of %d fields", value, n)
	}
	return items, nil
}

// decodeStop reads DRAFT_STOP_KEYS: at, label, place_name, place_id, id, day,
// duration_minutes, time_locked, meeting_point, lat, lng, checked_in.
func decodeStop(value any) (Stop, error) {
	f, err := fields(value, 12)
	if err != nil {
		return Stop{}, err
	}
	at, err := oracletest.Str(f[0])
	if err != nil {
		return Stop{}, err
	}
	label, err := oracletest.Str(f[1])
	if err != nil {
		return Stop{}, err
	}
	id, err := oracletest.Str(f[4])
	if err != nil {
		return Stop{}, err
	}
	day, err := oracletest.OptionalString(f[5])
	if err != nil {
		return Stop{}, err
	}
	dwell, err := scalar(f[6])
	if err != nil {
		return Stop{}, err
	}
	locked, err := oracletest.Bool(f[7])
	if err != nil {
		return Stop{}, err
	}
	lat, err := scalar(f[9])
	if err != nil {
		return Stop{}, err
	}
	lng, err := scalar(f[10])
	if err != nil {
		return Stop{}, err
	}
	checked, err := oracletest.Bool(f[11])
	if err != nil {
		return Stop{}, err
	}
	stop := Stop{
		ID: id, At: at, Label: label, Day: day, DurationMinutes: dwell,
		TimeLocked: locked, Lat: lat, Lng: lng, CheckedIn: checked,
	}
	if f[8] != nil {
		point, err := oracletest.Row(f[8], "lat", "lng", "label")
		if err != nil {
			return Stop{}, err
		}
		lat, err := scalar(point["lat"])
		if err != nil {
			return Stop{}, err
		}
		lng, err := scalar(point["lng"])
		if err != nil {
			return Stop{}, err
		}
		name, err := oracletest.Str(point["label"])
		if err != nil {
			return Stop{}, err
		}
		latitude, okLat := lat.(float64)
		longitude, okLng := lng.(float64)
		if !okLat || !okLng {
			return Stop{}, fmt.Errorf("a meeting point needs two floats")
		}
		stop.MeetingPoint = &MeetingPoint{Lat: latitude, Lng: longitude, Label: name}
	}
	return stop, nil
}

// decodeDay reads DRAFT_DAY_KEYS: day, transport_mode, start_at,
// start_stop_id, end_stop_id, return_to_start.
func decodeDay(value any) (DayConfig, error) {
	f, err := fields(value, 6)
	if err != nil {
		return DayConfig{}, err
	}
	day, err := oracletest.Str(f[0])
	if err != nil {
		return DayConfig{}, err
	}
	mode, err := oracletest.Str(f[1])
	if err != nil {
		return DayConfig{}, err
	}
	start, err := oracletest.Str(f[2])
	if err != nil {
		return DayConfig{}, err
	}
	startStop, err := oracletest.OptionalString(f[3])
	if err != nil {
		return DayConfig{}, err
	}
	endStop, err := oracletest.OptionalString(f[4])
	if err != nil {
		return DayConfig{}, err
	}
	returning, err := oracletest.Bool(f[5])
	if err != nil {
		return DayConfig{}, err
	}
	return DayConfig{
		Day: day, TransportMode: mode, StartAt: start,
		StartStopID: startStop, EndStopID: endStop, ReturnToStart: returning,
	}, nil
}

func decodeDraft(value any) (Draft, error) {
	row, err := oracletest.Row(value, "expected_revision", "stops", "days", "day", "include_suggestion")
	if err != nil {
		return Draft{}, err
	}
	revision, err := oracletest.Int64(row["expected_revision"])
	if err != nil {
		return Draft{}, err
	}
	day, err := oracletest.Str(row["day"])
	if err != nil {
		return Draft{}, err
	}
	suggest, err := oracletest.Bool(row["include_suggestion"])
	if err != nil {
		return Draft{}, err
	}
	draft := Draft{ExpectedRevision: revision, Day: day, IncludeSuggestion: suggest}
	stops, err := oracletest.List(row["stops"])
	if err != nil {
		return Draft{}, err
	}
	for _, raw := range stops {
		stop, err := decodeStop(raw)
		if err != nil {
			return Draft{}, err
		}
		draft.Stops = append(draft.Stops, stop)
	}
	days, err := oracletest.List(row["days"])
	if err != nil {
		return Draft{}, err
	}
	for _, raw := range days {
		config, err := decodeDay(raw)
		if err != nil {
			return Draft{}, err
		}
		draft.Days = append(draft.Days, config)
	}
	return draft, nil
}

func decodeLeg(value any) (valhalla.Leg, error) {
	row, err := oracletest.Row(value, "distance_meters", "duration_seconds", "geometry", "source")
	if err != nil {
		return valhalla.Leg{}, err
	}
	metres, err := oracletest.Int64(row["distance_meters"])
	if err != nil {
		return valhalla.Leg{}, err
	}
	seconds, err := oracletest.Int64(row["duration_seconds"])
	if err != nil {
		return valhalla.Leg{}, err
	}
	source, err := oracletest.Str(row["source"])
	if err != nil {
		return valhalla.Leg{}, err
	}
	points, err := oracletest.List(row["geometry"])
	if err != nil {
		return valhalla.Leg{}, err
	}
	leg := valhalla.Leg{DistanceMeters: metres, DurationSeconds: seconds, Source: source}
	for _, raw := range points {
		pair, err := fields(raw, 2)
		if err != nil {
			return valhalla.Leg{}, err
		}
		first, err := scalar(pair[0])
		if err != nil {
			return valhalla.Leg{}, err
		}
		second, err := scalar(pair[1])
		if err != nil {
			return valhalla.Leg{}, err
		}
		x, okX := first.(float64)
		y, okY := second.(float64)
		if !okX || !okY {
			return valhalla.Leg{}, fmt.Errorf("a geometry point needs two floats")
		}
		leg.Geometry = append(leg.Geometry, [2]float64{x, y})
	}
	return leg, nil
}

func decodeLegs(value any) ([]valhalla.Leg, error) {
	items, err := oracletest.List(value)
	if err != nil {
		return nil, err
	}
	out := make([]valhalla.Leg, 0, len(items))
	for _, item := range items {
		leg, err := decodeLeg(item)
		if err != nil {
			return nil, err
		}
		out = append(out, leg)
	}
	return out, nil
}

func decodeMatrix(value any) (journey.Matrix, error) {
	rows, err := oracletest.List(value)
	if err != nil {
		return nil, err
	}
	out := make(journey.Matrix, 0, len(rows))
	for _, raw := range rows {
		cells, err := oracletest.List(raw)
		if err != nil {
			return nil, err
		}
		row := make([]*journey.Cost, 0, len(cells))
		for _, cell := range cells {
			if cell == nil {
				row = append(row, nil)
				continue
			}
			pair, err := fields(cell, 2)
			if err != nil {
				return nil, err
			}
			seconds, err := oracletest.Int64(pair[0])
			if err != nil {
				return nil, err
			}
			metres, err := oracletest.Int64(pair[1])
			if err != nil {
				return nil, err
			}
			row = append(row, &journey.Cost{Seconds: seconds, Meters: metres})
		}
		out = append(out, row)
	}
	return out, nil
}

// --- encoding back into the golden's shape -----------------------------------

func optional(text *string) any {
	if text == nil {
		return nil
	}
	return *text
}

func wireIssues(issues []journey.Issue) []any {
	out := make([]any, 0, len(issues))
	for _, i := range issues {
		out = append(out, map[string]any{"code": i.Code, "stop_id": optional(i.StopID), "message": i.Message})
	}
	return out
}

func wireGeometry(points [][2]float64) []any {
	out := make([]any, 0, len(points))
	for _, point := range points {
		out = append(out, []any{wireFloat(point[0]), wireFloat(point[1])})
	}
	return out
}

func wireRoute(route *Route) any {
	if route == nil {
		return nil
	}
	rows := make([]any, 0, len(route.Stops))
	for _, row := range route.Stops {
		rows = append(rows, map[string]any{
			"id":           row.ID,
			"at":           optional(row.At),
			"arrival_at":   optional(row.ArrivalAt),
			"departure_at": optional(row.DepartureAt),
			"wait_minutes": row.WaitMinutes,
		})
	}
	segments := make([]any, 0, len(route.Segments))
	for _, segment := range route.Segments {
		segments = append(segments, map[string]any{
			"distance_meters":  segment.Leg.DistanceMeters,
			"duration_seconds": segment.Leg.DurationSeconds,
			"geometry":         wireGeometry(segment.Leg.Geometry),
			"source":           segment.Leg.Source,
			"from_stop_id":     segment.FromStopID,
			"to_stop_id":       segment.ToStopID,
		})
	}
	return map[string]any{
		"stops":            rows,
		"feasible":         route.Feasible,
		"issues":           wireIssues(route.Issues),
		"segments":         segments,
		"distance_meters":  route.DistanceMeters,
		"duration_seconds": route.DurationSeconds,
	}
}

func wirePreview(preview Preview) any {
	var savings any
	if preview.Savings != nil {
		savings = map[string]any{
			"distance_meters":  preview.Savings.DistanceMeters,
			"duration_seconds": preview.Savings.DurationSeconds,
		}
	}
	return map[string]any{
		"revision": preview.Revision,
		"day":      preview.Day,
		"status":   preview.Status,
		"source": map[string]any{
			"engine":        Engine,
			"graph_version": optional(preview.GraphVersion),
			"traffic":       Traffic,
		},
		"current":    wireRoute(preview.Current),
		"suggestion": wireRoute(preview.Suggestion),
		"savings":    savings,
		"issues":     wireIssues(preview.Issues),
	}
}

// --- replay -------------------------------------------------------------------

func replay(c oracletest.Case, args map[string]any) (any, error) {
	draft, err := decodeDraft(args["draft"])
	if err != nil {
		return nil, oracletest.Decode(err)
	}
	script, err := oracletest.Row(args["script"], "graph_version", "route", "matrix")
	if err != nil {
		return nil, oracletest.Decode(err)
	}
	version, err := oracletest.Str(script["graph_version"])
	if err != nil {
		return nil, oracletest.Decode(err)
	}
	routes, err := oracletest.List(script["route"])
	if err != nil {
		return nil, oracletest.Decode(err)
	}
	matrices, err := oracletest.List(script["matrix"])
	if err != nil {
		return nil, oracletest.Decode(err)
	}
	settings, err := oracletest.Row(args["env"], "provider", "capacity", "suggestions_enabled")
	if err != nil {
		return nil, oracletest.Decode(err)
	}
	hasProvider, err := oracletest.Bool(settings["provider"])
	if err != nil {
		return nil, oracletest.Decode(err)
	}
	capacity, err := oracletest.Bool(settings["capacity"])
	if err != nil {
		return nil, oracletest.Decode(err)
	}
	enabled, err := oracletest.Bool(settings["suggestions_enabled"])
	if err != nil {
		return nil, oracletest.Decode(err)
	}

	calls := []any{}
	var router Router
	if hasProvider {
		router = &scriptedRouter{
			graphVersion: version,
			route:        append([]any(nil), routes...),
			matrix:       append([]any(nil), matrices...),
			calls:        &calls,
		}
	}
	out := map[string]any{"calls": calls, "result": nil, "raised": nil}
	preview, err := Build(draft, router, Env{SuggestionsEnabled: enabled, CapacityAvailable: capacity})
	out["calls"] = calls
	if err != nil {
		class, message := "ValueError", err.Error()
		if _, ok := err.(*journey.IndexError); ok {
			class = "IndexError"
		}
		out["raised"] = map[string]any{"type": class, "message": message, "code": nil}
		return out, nil
	}
	out["result"] = wirePreview(preview)
	return out, nil
}

func checkItinerary(t *testing.T, files []oracletest.File, committed bool) {
	t.Helper()
	oracletest.Agree(t, files, "itinerary", replay, func(error) (string, string, bool) {
		return "", "", false
	})
	if !committed {
		return
	}
	seen := map[string]bool{}
	var walk func(any)
	walk = func(value any) {
		switch v := value.(type) {
		case map[string]any:
			if code, ok := v["code"].(string); ok {
				seen[code] = true
			}
			if status, ok := v["status"].(string); ok {
				seen[status] = true
			}
			for _, item := range v {
				walk(item)
			}
		case []any:
			for _, item := range v {
				walk(item)
			}
		}
	}
	for _, file := range files {
		for _, c := range file.Cases {
			walk(c.Result)
		}
	}
	for _, code := range append(append([]string(nil), issueCodes...), statuses...) {
		if !seen[code] {
			t.Errorf("no committed case shows %q", code)
		}
	}
}

func TestItineraryMatchesPython(t *testing.T) {
	files := oracletest.Load(t, "testdata/python_itinerary*.json")
	constants := oracletest.Constants(t, files, "itinerary")
	for name, want := range map[string]string{"engine": Engine, "traffic": Traffic} {
		got, err := oracletest.Str(constants[name])
		if err != nil || got != want {
			t.Errorf("%s: Python %v, Go %q (%v)", name, constants[name], want, err)
		}
	}
	maxStops, err := oracletest.Int64(constants["max_stops"])
	if err != nil || maxStops != journey.MaxStops {
		t.Errorf("max_stops: Python %v, Go %d (%v)", constants["max_stops"], journey.MaxStops, err)
	}
	modes, err := oracletest.Strings(constants["modes"])
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range modes {
		if _, known := valhalla.Costing(mode); !known {
			t.Errorf("Go does not know the mode %q", mode)
		}
	}
	checkItinerary(t, files, true)
}

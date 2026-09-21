package routingstub

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ask sends one action to a handler of its own, so that "the same request
// twice" means two independent handlers and not one warmed-up one.
func ask(t *testing.T, action, body string) (int, string) {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/"+action, strings.NewReader(body))
	Handler().ServeHTTP(recorder, request)
	return recorder.Code, recorder.Body.String()
}

const fourStops = `{"locations":[{"lat":16.06,"lon":108.2},{"lat":16.062,"lon":108.5},` +
	`{"lat":16.064,"lon":108.3},{"lat":16.066,"lon":108.4}],"costing":"motor_scooter"}`

// The property the whole comparison rests on: identical requests, identical
// bytes, from handlers that share nothing.
func TestIdenticalRequestsGiveIdenticalBytes(t *testing.T) {
	for _, tc := range []struct{ action, body string }{
		{"route", fourStops},
		{"sources_to_targets", `{"sources":[{"lat":16.06,"lon":108.2},{"lat":16.066005,"lon":108.4}],` +
			`"targets":[{"lat":16.06,"lon":108.2},{"lat":16.066005,"lon":108.4}],"costing":"auto"}`},
	} {
		_, first := ask(t, tc.action, tc.body)
		_, second := ask(t, tc.action, tc.body)
		if first != second {
			t.Fatalf("%s differed between two handlers:\n%s\n%s", tc.action, first, second)
		}
	}
}

// A polyline6 alphabet reaches '\', so a shape has to be escaped like any other
// JSON string. Written by hand it produced a body json.loads refuses, which
// routing.py reports as RoutingUnavailable -- the "unavailable" this package
// exists to get past, arriving by a different door.
func TestEveryAnswerIsJSONAndShapesAreEscaped(t *testing.T) {
	code, body := ask(t, "route", fourStops)
	if code != http.StatusOK {
		t.Fatalf("status %d: %s", code, body)
	}
	var decoded struct {
		Trip struct {
			Status int `json:"status"`
			Legs   []struct {
				Shape   string `json:"shape"`
				Summary struct {
					Time   int64   `json:"time"`
					Length float64 `json:"length"`
				} `json:"summary"`
			} `json:"legs"`
		} `json:"trip"`
	}
	if err := json.Unmarshal([]byte(body), &decoded); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, body)
	}
	if decoded.Trip.Status != 0 || len(decoded.Trip.Legs) != 3 {
		t.Fatalf("status %d with %d legs", decoded.Trip.Status, len(decoded.Trip.Legs))
	}
	if !strings.Contains(body, `\\`) {
		t.Errorf("this fixture used to carry an escaped backslash; pick another that does:\n%s", body)
	}
	// round(length * 1000) in routing.py has to land back on whole metres.
	for i, leg := range decoded.Trip.Legs {
		metres := leg.Summary.Length * 1000
		if metres != float64(int64(metres+0.5)) {
			t.Errorf("leg %d length %v is not whole metres", i, leg.Summary.Length)
		}
		if leg.Summary.Time <= 0 || leg.Shape == "" {
			t.Errorf("leg %d: time %d shape %q", i, leg.Summary.Time, leg.Shape)
		}
	}
}

// The one-way rule, in both of the answers it shows up in: a null matrix cell
// that suggest_order scores with its missing-leg penalty, and a whole route
// that comes back with a trip status that is not 0.
func TestOneWayShowsUpAsANullCellAndANonZeroTripStatus(t *testing.T) {
	// 16.066005 ends in the microdegree digit 5, so it may only be entered
	// from the west; .5 is east of .4.
	_, matrix := ask(t, "sources_to_targets",
		`{"sources":[{"lat":16.062,"lon":108.5},{"lat":16.064,"lon":108.3}],`+
			`"targets":[{"lat":16.066005,"lon":108.4}],"costing":"motor_scooter"}`)
	var cells struct {
		Rows [][]struct {
			Time     *int64   `json:"time"`
			Distance *float64 `json:"distance"`
		} `json:"sources_to_targets"`
	}
	if err := json.Unmarshal([]byte(matrix), &cells); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, matrix)
	}
	if cells.Rows[0][0].Time != nil || cells.Rows[0][0].Distance != nil {
		t.Errorf("approach from the east was routed: %s", matrix)
	}
	if cells.Rows[1][0].Time == nil {
		t.Errorf("approach from the west was refused: %s", matrix)
	}
	_, refused := ask(t, "route",
		`{"locations":[{"lat":16.062,"lon":108.5},{"lat":16.066005,"lon":108.4}],"costing":"motor_scooter"}`)
	if !strings.Contains(refused, `"status":1`) {
		t.Errorf("route over the one-way answered %s", refused)
	}
}

// The graph version is a digest of the rules, so a run's evidence names the
// answers it was given, and changing an answer without bumping specID cannot
// leave two stacks reporting one version for two behaviours.
func TestGraphVersionIsStableAndDigested(t *testing.T) {
	if GraphVersion() != GraphVersion() {
		t.Fatal("graph version is not stable")
	}
	if !strings.HasPrefix(GraphVersion(), "parity-stub-") || len(GraphVersion()) != len("parity-stub-")+16 {
		t.Fatalf("graph version %q", GraphVersion())
	}
}

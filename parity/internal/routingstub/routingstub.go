// Package routingstub answers the two Valhalla actions
// services/api/app/journey/routing.py calls, deterministically, from the
// request alone.
//
// Why it exists. scripts/parity_stacks.sh brought up two stacks with no
// MOBILE_VALHALLA_URL, so configured_provider() returned nil and every
// itinerary preview in every parity run stopped at status "unavailable". The
// routed half of the preview -- _route, schedule, suggest_order, savings,
// feasible, segments, late_fixed_stop -- therefore had no parity evidence at
// all. Pointing both stacks at a real Valhalla is not an option here: it wants
// a multi-gigabyte graph built from an OSM extract (that is what
// scripts/prepare_journey_routing.py is for), and two containers reading one
// tile set would still have to agree to the byte.
//
// Why the answers can be compared. The comparison is against Python, so any
// nondeterminism in the routing answer becomes a false difference. Every
// number here is a pure function of the request: the ordered pair of
// coordinates in microdegrees and the costing name, and nothing else. No
// clock, no random source, no insertion order, no state that survives a
// request. Two instances in two containers therefore answer identical requests
// with identical bytes without sharing anything, which is why the harness runs
// one instance per side rather than one shared one -- a shared instance would
// make the agreement an artifact of sharing rather than a property of the
// stub.
//
// The arithmetic is integer end to end (microdegrees, metres, seconds), so
// there is no float rounding to differ. The only floats on the wire are the
// kilometre distances Valhalla reports, and those are printed from an integer
// number of metres with a fixed three decimals, so `round(km * 1000)` in
// routing.py recovers the metre exactly.
//
// What it does not claim to be. These are not real roads and the numbers are
// not real travel times. The stub is a fixture that makes the routed branches
// reachable and comparable; it proves nothing about Valhalla, about the
// quality of a suggestion, or about whether a real graph would answer at all.
package routingstub

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
)

// specID names the rules below. Bump it whenever an answer changes: the graph
// version both stacks report as source.graph_version is a digest of it, so a
// run's evidence says which rules produced its numbers. That is the one thing
// reused from scripts/prepare_journey_routing.py, which derives its graph
// version the same way from the extract it pinned -- a version string that
// changes when the answers change.
const specID = "parity/routing-stub/1" +
	" cost=isqrt-equirect-microdegrees detour=fnv1a-per-ordered-pair" +
	" access=one-way-when-lat-microdigit-5 shape=polyline6-four-segment-dogleg"

// GraphVersion is MOBILE_ROUTING_GRAPH_VERSION for a stack pointed at this
// stub, and what the preview echoes in source.graph_version.
func GraphVersion() string {
	sum := sha256.Sum256([]byte(specID))
	return "parity-stub-" + hex.EncodeToString(sum[:])[:16]
}

const (
	// Metres per microdegree, in ten-thousandths: latitude anywhere, and
	// longitude at the latitudes this product serves (around 16 N).
	metresPerMicroLat = 1113
	metresPerMicroLng = 1070
	// Past this a leg has no road, the way a Valhalla build refuses a request
	// beyond its max_distance instead of inventing one.
	maxLegMetres = 500_000
	// The two service limits scripts/prepare_journey_routing.py writes into
	// valhalla.json: fifty real stops plus a duplicated start for a return
	// trip, and a fifty by fifty matrix.
	maxLocations = 51
	maxPairs     = 2500
	// Metres of access road every leg costs, so that two stops a few metres
	// apart are not free.
	accessMetres = 60
)

// speedMetresPerHour is the one speed each costing model travels at.
var speedMetresPerHour = map[string]int64{
	"auto":          38_000,
	"motor_scooter": 26_000,
	"pedestrian":    4_800,
}

// point is a coordinate in microdegrees, which is the resolution
// _decode_shape's polyline6 carries and therefore the finest distinction the
// answers may depend on.
type point struct{ lat, lng int64 }

func micro(v float64) int64 { return int64(math.Round(v * 1_000_000)) }

func abs(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

// microDigit is the last digit of a microdegree value, for negatives too.
func microDigit(v int64) int64 { return ((v % 10) + 10) % 10 }

// isqrt is the integer square root, so the straight-line metres between two
// points never go through a float.
func isqrt(n int64) int64 {
	if n <= 0 {
		return 0
	}
	x := n
	y := (x + 1) / 2
	for y < x {
		x = y
		y = (x + n/x) / 2
	}
	return x
}

// straightMetres is the equirectangular distance in whole metres.
func straightMetres(a, b point) int64 {
	dLat := abs(b.lat-a.lat) * metresPerMicroLat / 10_000
	dLng := abs(b.lng-a.lng) * metresPerMicroLng / 10_000
	return isqrt(dLat*dLat + dLng*dLng)
}

// pairHash is the only source of variation beyond geometry, and it is a
// function of the ordered pair, so A to B and B to A differ the way two
// directions of a real road do.
func pairHash(a, b point, costing string) uint64 {
	h := fnv.New64a()
	fmt.Fprintf(h, "%d|%d|%d|%d|%s", a.lat, a.lng, b.lat, b.lng, costing)
	return h.Sum64()
}

// barred reports that there is no road from a to b.
//
// The rule: a stop whose latitude ends in the microdegree digit 5 sits on a
// one-way that can only be entered from the west, so a leg that approaches it
// from the east has no road. It is directed, which is what makes it useful --
// the same pair of stops is routable in one order and not in the other, so a
// day can have a perfectly routable current order while the matrix still holds
// the null cell that suggest_order scores with its missing-leg penalty. The
// sixth decimal of a coordinate is the control channel: a real Valhalla
// ignores it, this stub reads it, and a scenario chooses it.
func barred(a, b point) bool {
	return microDigit(b.lat) == 5 && a.lng > b.lng
}

// legCost is the cost of going from a to b, or ok=false when there is no road.
func legCost(a, b point, costing string) (metres, seconds int64, ok bool) {
	if a == b {
		return 0, 0, true
	}
	if barred(a, b) {
		return 0, 0, false
	}
	straight := straightMetres(a, b)
	if straight > maxLegMetres {
		return 0, 0, false
	}
	// A detour factor of 1.000 to 1.399, fixed per ordered pair.
	metres = straight*int64(1000+pairHash(a, b, costing)%400)/1000 + accessMetres
	speed := speedMetresPerHour[costing]
	seconds = (metres*3600 + speed - 1) / speed
	return metres, seconds, true
}

// legShape is the polyline6 _decode_shape reads back: five points, the two
// ends and three intermediates pushed off the straight line by a per-pair
// amount that changes sign between latitude and longitude, so the decoder is
// fed positive and negative deltas of several magnitudes rather than a
// straight line it could get right by accident.
func legShape(a, b point, costing string) string {
	h := pairHash(a, b, costing)
	points := make([]point, 0, 5)
	for i := int64(0); i <= 4; i++ {
		p := point{lat: a.lat + (b.lat-a.lat)*i/4, lng: a.lng + (b.lng-a.lng)*i/4}
		if i > 0 && i < 4 {
			offset := int64((h>>(uint(i)*7))%241) - 120
			p.lat += offset
			p.lng -= offset
		}
		points = append(points, p)
	}
	return encodePolyline6(points)
}

func encodePolyline6(points []point) string {
	var out strings.Builder
	var lat, lng int64
	for _, p := range points {
		encodeValue(&out, p.lat-lat)
		encodeValue(&out, p.lng-lng)
		lat, lng = p.lat, p.lng
	}
	return out.String()
}

func encodeValue(out *strings.Builder, value int64) {
	shifted := uint64(value << 1)
	if value < 0 {
		shifted = uint64(^(value << 1))
	}
	for shifted >= 0x20 {
		out.WriteByte(byte(0x20|(shifted&0x1f)) + 63)
		shifted >>= 5
	}
	out.WriteByte(byte(shifted) + 63)
}

// kilometres prints whole metres as Valhalla's kilometre number, with three
// decimals and no exponent, so that round(value * 1000) in routing.py is the
// metre this function was given.
func kilometres(metres int64) string {
	return strconv.FormatInt(metres/1000, 10) + "." + fmt.Sprintf("%03d", metres%1000)
}

// location is one point of a request. Valhalla names them lat and lon.
type location struct {
	Lat *float64 `json:"lat"`
	Lon *float64 `json:"lon"`
}

type matrixRequest struct {
	Sources []location `json:"sources"`
	Targets []location `json:"targets"`
	Costing string     `json:"costing"`
	Units   string     `json:"units"`
}

type routeRequest struct {
	Locations []location `json:"locations"`
	Costing   string     `json:"costing"`
	Units     string     `json:"units"`
}

// Handler serves the stub's actions. It holds no state, so two handlers in two
// processes answer one request identically.
func Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/status", status)
	mux.HandleFunc("/healthz", status)
	mux.HandleFunc("/sources_to_targets", sourcesToTargets)
	mux.HandleFunc("/route", route)
	return mux
}

func status(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		refuse(w, 106, "Try any of: '/route' '/sources_to_targets' '/status'")
		return
	}
	write(w, http.StatusOK, `{"version":`+quote(GraphVersion())+`,"has_tiles":true,"has_live_traffic":false}`)
}

// refuse answers the way Valhalla refuses: an error_code in the body, which is
// the key routing.py's _post looks for before it reads anything else.
func refuse(w http.ResponseWriter, code int, message string) {
	write(w, http.StatusBadRequest, `{"error_code":`+strconv.Itoa(code)+`,"error":`+quote(message)+
		`,"status_code":400,"status":"Bad Request"}`)
}

// quote writes a JSON string. A polyline6 alphabet runs from '?' to '~', so a
// shape really can contain a backslash, and writing one into the body by hand
// produced a body json.loads refused -- which routing.py answers as
// RoutingUnavailable, that is, as the "unavailable" this whole stub exists to
// get past. Every string that is not a literal in this file goes through here.
func quote(value string) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return `""`
	}
	return string(encoded)
}

func write(w http.ResponseWriter, code int, body string) {
	w.Header().Set("Content-Type", "application/json;charset=utf-8")
	w.WriteHeader(code)
	io.WriteString(w, body)
}

// decode reads the JSON body of a POST action.
func decode(w http.ResponseWriter, r *http.Request, into any) bool {
	if r.Method != http.MethodPost {
		refuse(w, 106, "Try any of: '/route' '/sources_to_targets' '/status'")
		return false
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
	if err != nil || json.Unmarshal(body, into) != nil {
		refuse(w, 100, "Failed to parse json request")
		return false
	}
	return true
}

// points converts a request's locations, refusing a missing coordinate the way
// Valhalla refuses one.
func points(locations []location) ([]point, bool) {
	out := make([]point, 0, len(locations))
	for _, l := range locations {
		if l.Lat == nil || l.Lon == nil ||
			math.IsNaN(*l.Lat) || math.IsNaN(*l.Lon) ||
			*l.Lat < -90 || *l.Lat > 90 || *l.Lon < -180 || *l.Lon > 180 {
			return nil, false
		}
		out = append(out, point{lat: micro(*l.Lat), lng: micro(*l.Lon)})
	}
	return out, true
}

func sourcesToTargets(w http.ResponseWriter, r *http.Request) {
	var request matrixRequest
	if !decode(w, r, &request) {
		return
	}
	if _, known := speedMetresPerHour[request.Costing]; !known {
		refuse(w, 125, "No costing method found")
		return
	}
	sources, okSources := points(request.Sources)
	targets, okTargets := points(request.Targets)
	if !okSources || !okTargets || len(sources) == 0 || len(targets) == 0 {
		refuse(w, 112, "Insufficiently specified required parameter 'locations'")
		return
	}
	if len(sources)*len(targets) > maxPairs {
		refuse(w, 154, "Path distance exceeds the max location distance limit")
		return
	}
	var out strings.Builder
	out.WriteString(`{"algorithm":"timedistancematrix","units":"kilometers","sources_to_targets":[`)
	for i, from := range sources {
		if i > 0 {
			out.WriteString(",")
		}
		out.WriteString("[")
		for j, to := range targets {
			if j > 0 {
				out.WriteString(",")
			}
			metres, seconds, ok := legCost(from, to, request.Costing)
			out.WriteString(`{"from_index":` + strconv.Itoa(i) + `,"to_index":` + strconv.Itoa(j))
			if ok {
				out.WriteString(`,"time":` + strconv.FormatInt(seconds, 10) +
					`,"distance":` + kilometres(metres) + "}")
			} else {
				// A cell with no road. routing.py maps it to None, and
				// suggest_order scores an order that uses it with its
				// missing-leg penalty.
				out.WriteString(`,"time":null,"distance":null}`)
			}
		}
		out.WriteString("]")
	}
	out.WriteString("]}")
	write(w, http.StatusOK, out.String())
}

func route(w http.ResponseWriter, r *http.Request) {
	var request routeRequest
	if !decode(w, r, &request) {
		return
	}
	if _, known := speedMetresPerHour[request.Costing]; !known {
		refuse(w, 125, "No costing method found")
		return
	}
	stops, ok := points(request.Locations)
	if !ok || len(stops) < 2 {
		refuse(w, 112, "Insufficiently specified required parameter 'locations'")
		return
	}
	if len(stops) > maxLocations {
		refuse(w, 154, "Exceeded max locations")
		return
	}
	type leg struct{ metres, seconds int64 }
	legs := make([]leg, 0, len(stops)-1)
	for i := 0; i+1 < len(stops); i++ {
		metres, seconds, routable := legCost(stops[i], stops[i+1], request.Costing)
		if !routable {
			// Not an HTTP error: a trip whose status is not 0 is a distinct
			// branch of routing.py's route(), and this is what reaches it.
			write(w, http.StatusOK, `{"trip":{"status":1,`+
				`"status_message":"No path could be found for input",`+
				`"units":"kilometers","language":"en-US","legs":[]}}`)
			return
		}
		legs = append(legs, leg{metres: metres, seconds: seconds})
	}
	var totalMetres, totalSeconds int64
	var out strings.Builder
	out.WriteString(`{"trip":{"status":0,"status_message":"Found route between points",` +
		`"units":"kilometers","language":"en-US","legs":[`)
	for i, l := range legs {
		if i > 0 {
			out.WriteString(",")
		}
		totalMetres += l.metres
		totalSeconds += l.seconds
		out.WriteString(`{"summary":{"time":` + strconv.FormatInt(l.seconds, 10) +
			`,"length":` + kilometres(l.metres) + `},"shape":` +
			quote(legShape(stops[i], stops[i+1], request.Costing)) + `}`)
	}
	out.WriteString(`],"summary":{"time":` + strconv.FormatInt(totalSeconds, 10) +
		`,"length":` + kilometres(totalMetres) + `}}}`)
	write(w, http.StatusOK, out.String())
}

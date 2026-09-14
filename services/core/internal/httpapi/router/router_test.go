package router

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"slices"
	"sort"
	"strings"
	"testing"

	"mobile/services/core/ownership"
)

// golden is one row of testdata/starlette_decisions.json, rendered inside the
// pinned API image by scripts/render_router_goldens.py.
type golden struct {
	Method           string            `json:"method"`
	Target           string            `json:"target"`
	Host             string            `json:"host"`
	Scheme           string            `json:"scheme"`
	Path             *string           `json:"path"`
	Kind             string            `json:"kind"`
	Route            string            `json:"route"`
	Params           map[string]string `json:"params"`
	Allow            string            `json:"allow"`
	AllowOrderVaries bool              `json:"allow_order_varies"`
	Location         string            `json:"location"`
}

func manifestRoutes(t *testing.T) []ownership.Route {
	t.Helper()
	m, err := ownership.Load()
	if err != nil {
		t.Fatal(err)
	}
	return m.Routes
}

func mustNew(t *testing.T, routes []ownership.Route) *Router {
	t.Helper()
	r, err := New(routes)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func loadGoldens(t *testing.T) []golden {
	t.Helper()
	data, err := os.ReadFile("testdata/starlette_decisions.json")
	if err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var cases []golden
	if err := decoder.Decode(&cases); err != nil {
		t.Fatal(err)
	}
	return cases
}

// disagreement returns "" when r decides c exactly as Starlette did.
func disagreement(r *Router, c golden) string {
	got := r.DecideTarget(c.Method, c.Target, c.Host, c.Scheme)
	switch {
	case got.Kind != c.Kind:
		return fmt.Sprintf("kind %q, want %q (%+v)", got.Kind, c.Kind, got)
	case got.RouteID != c.Route:
		return fmt.Sprintf("route %q, want %q", got.RouteID, c.Route)
	case !maps.Equal(got.Params, c.Params):
		return fmt.Sprintf("params %q, want %q", got.Params, c.Params)
	case got.Location != c.Location:
		return fmt.Sprintf("location %q, want %q", got.Location, c.Location)
	}
	if c.AllowOrderVaries {
		// Python's order follows PYTHONHASHSEED; only the set is defined.
		gotSet, wantSet := strings.Split(got.Allow, ", "), strings.Split(c.Allow, ", ")
		sort.Strings(gotSet)
		sort.Strings(wantSet)
		if !slices.Equal(gotSet, wantSet) {
			return fmt.Sprintf("allow %q, want the methods of %q", got.Allow, c.Allow)
		}
	} else if got.Allow != c.Allow {
		return fmt.Sprintf("allow %q, want %q", got.Allow, c.Allow)
	}
	if c.Path == nil {
		return ""
	}
	rawPath, _ := SplitTarget(c.Target)
	if path := ScopePath(rawPath); path != *c.Path {
		return fmt.Sprintf("scope path %q, want %q", path, *c.Path)
	}
	match, ok := r.FirstFull(c.Method, *c.Path)
	if ok != (c.Kind == KindFull) || ok && (match.Route.ID != c.Route || !maps.Equal(match.Params, c.Params)) {
		return fmt.Sprintf("FirstFull = %q %q %v", match.Route.ID, match.Params, ok)
	}
	return ""
}

func TestMatchesStarletteGoldens(t *testing.T) {
	routes := manifestRoutes(t)
	r := mustNew(t, routes)
	cases := loadGoldens(t)
	if len(cases) < 5000 {
		t.Fatalf("only %d golden cases; rerender scripts/render_router_goldens.py", len(cases))
	}

	kinds := map[string]int{}
	fullRoutes := map[string]bool{}
	mismatches := 0
	for _, c := range cases {
		kinds[c.Kind]++
		if c.Kind == KindFull {
			fullRoutes[c.Route] = true
		}
		if problem := disagreement(r, c); problem != "" {
			mismatches++
			if mismatches <= 25 {
				t.Errorf("%s %q host=%q scheme=%q: %s", c.Method, c.Target, c.Host, c.Scheme, problem)
			}
		}
	}
	t.Logf("%d golden cases %v, %d mismatches", len(cases), kinds, mismatches)
	if mismatches > 0 {
		t.Errorf("%d of %d cases disagree with Starlette", mismatches, len(cases))
	}
	for _, row := range routes {
		if !fullRoutes[row.ID] {
			t.Errorf("no golden case FULL-matches %q", row.ID)
		}
	}
}

// The same paths resolve differently depending on registration order; these
// are the collisions the front door would get wrong with a "most specific
// wins" router.
func TestRegistrationOrderDecides(t *testing.T) {
	r := mustNew(t, manifestRoutes(t))
	for _, tc := range []struct {
		method, target, route string
		params                map[string]string
	}{
		{"GET", "/people/me", "GET /people/me", nil},
		{"PUT", "/people/me", "PUT /people/{person_id}", map[string]string{"person_id": "me"}},
		{"DELETE", "/sessions/current", "DELETE /sessions/current", nil},
		{"GET", "/places/search", "GET /places/{place_id}", map[string]string{"place_id": "search"}},
	} {
		got := r.DecideTarget(tc.method, tc.target, "api.test", "http")
		if got.Kind != KindFull || got.RouteID != tc.route || !maps.Equal(got.Params, tc.params) {
			t.Errorf("%s %s = %+v, want FULL %q %v", tc.method, tc.target, got, tc.route, tc.params)
		}
		match, ok := r.FirstFull(tc.method, tc.target)
		if !ok || match.Route.ID != tc.route || !maps.Equal(match.Params, tc.params) {
			t.Errorf("FirstFull(%s %s) = %q %v %v", tc.method, tc.target, match.Route.ID, match.Params, ok)
		}
	}
}

// A router that ignored order would still pass hand-picked cases by luck;
// reversing the manifest must visibly break both them and the goldens.
func TestReversedOrderIsCaught(t *testing.T) {
	routes := slices.Clone(manifestRoutes(t))
	slices.Reverse(routes)
	r := mustNew(t, routes)
	if match, _ := r.FirstFull("GET", "/people/me"); match.Route.ID != "GET /people/{person_id}" {
		t.Fatalf("reversed router still resolves GET /people/me to %q", match.Route.ID)
	}
	mismatches := 0
	for _, c := range loadGoldens(t) {
		if disagreement(r, c) != "" {
			mismatches++
		}
	}
	if mismatches == 0 {
		t.Fatal("goldens do not notice a reversed registration order")
	}
}

func TestNewRefusesWhatItCannotEmulate(t *testing.T) {
	route := func(kind, path string) []ownership.Route {
		return []ownership.Route{{ID: "GET " + path, Kind: kind, Method: "GET", Path: path}}
	}
	for _, tc := range []struct {
		routes []ownership.Route
		want   string
	}{
		{route("route", "/a/{x}/{x}"), "duplicated param"},
		{route("route", "/a/{x:int}"), "not emulated"},
		{route("route", "/a/{x:uuid}"), "not emulated"},
		{route("route", "a/b"), "must start with '/'"},
		{route("host", "/a"), "not routable"},
	} {
		if _, err := New(tc.routes); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("New(%q) error = %v, want %q", tc.routes[0].Path, err, tc.want)
		}
	}
}

// Values observed from uvicorn 0.34.0 (httptools and h11 agree) on raw
// requests inside the pinned image.
func TestScopePathMatchesUvicorn(t *testing.T) {
	const r = "�"
	for raw, want := range map[string]string{
		"/a%2Fb":        "/a/b",
		"//x//y":        "//x//y",
		"/a%25b":        "/a%b",
		"/a%252Fb":      "/a%2Fb",
		"/%C3%A9":       "/é",
		"/%E2%82":       "/" + r,
		"/%FF":          "/" + r,
		"/%FF%FE":       "/" + r + r,
		"/%F0%9F%98":    "/" + r,
		"/%ED%A0%80":    "/" + r + r + r,
		"/%C0%AF":       "/" + r + r,
		"/%F4%90%80%80": "/" + r + r + r + r,
		"/%E2%82%AC%E2": "/€" + r,
		"/%e2%82%ac":    "/€",
		"/%zz":          "/%zz",
		"/%4":           "/%4",
		"/%zz%4":        "/%zz%4",
		"/%%41":         "/%A",
		"/a%20b":        "/a b",
		"/a;b":          "/a;b",
		"*":             "*",
	} {
		if got := ScopePath(raw); got != want {
			t.Errorf("ScopePath(%q) = %q, want %q", raw, got, want)
		}
	}
}

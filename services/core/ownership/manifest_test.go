package ownership

import (
	"encoding/json"
	"strings"
	"testing"
)

func row(order int, method, path, group string) Route {
	return Route{
		ID: method + " " + path, Order: order, Kind: "route", Method: method, Path: path,
		Group: group, Class: "core", Owner: OwnerPython, Python: PythonLive, State: "PY",
	}
}

func encode(t *testing.T, routes []Route) []byte {
	t.Helper()
	data, err := json.Marshal(Manifest{Schema: 1, Routes: routes})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func goOwned(r Route) Route {
	r.Owner, r.State, r.Evidence = OwnerGo, "LIVE-GO", "docs/migration/evidence/x.json"
	return r
}

func TestEmbeddedManifestIsValid(t *testing.T) {
	m, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(m.Routes) < 150 {
		t.Fatalf("embedded manifest has %d rows, want at least the 150 API routes", len(m.Routes))
	}
}

func TestParseRejects(t *testing.T) {
	a := row(0, "GET", "/a", "g1")
	b := row(1, "POST", "/b", "g2")
	cases := map[string]struct {
		routes []Route
		want   string
	}{
		"order gap": {[]Route{a, {ID: "POST /b", Order: 5, Kind: "route", Method: "POST", Path: "/b",
			Group: "g", Class: "core", Owner: OwnerPython, Python: PythonLive, State: "PY"}}, "order"},
		"duplicate":             {[]Route{a, func() Route { r := a; r.Order = 1; return r }()}, "duplicate"},
		"id mismatch":           {[]Route{func() Route { r := a; r.ID = "GET /x"; return r }()}, "id must be"},
		"go owner in PY state":  {[]Route{func() Route { r := a; r.Owner = OwnerGo; r.Evidence = "e"; return r }()}, "does not match state"},
		"go owned no evidence":  {[]Route{func() Route { r := goOwned(a); r.Evidence = ""; return r }()}, "without evidence"},
		"frozen but live state": {[]Route{func() Route { r := goOwned(a); r.Python = PythonFrozen; return r }()}, "does not match state"},
		"unknown class":         {[]Route{func() Route { r := a; r.Class = "nope"; return r }()}, "unknown"},
		"split limiter": {[]Route{
			func() Route { r := goOwned(a); r.InMemory = []string{"reason_writer"}; return r }(),
			func() Route { r := b; r.InMemory = []string{"reason_writer"}; return r }(),
		}, "shares in-memory"},
		"framework to go": {[]Route{func() Route { r := goOwned(a); r.Class = "framework"; return r }()}, "only API routes"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Parse(encode(t, tc.routes))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestParseRejectsUnknownField(t *testing.T) {
	_, err := Parse([]byte(`{"schema":1,"routes":[],"extra":true}`))
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("err = %v", err)
	}
}

func TestForceAndServed(t *testing.T) {
	a := goOwned(row(0, "GET", "/a", "g1"))
	b := goOwned(row(1, "POST", "/b", "g1"))
	c := goOwned(row(2, "GET", "/c", "g2"))
	d := row(3, "GET", "/d", "g2")
	m, err := Parse(encode(t, []Route{a, b, c, d}))
	if err != nil {
		t.Fatal(err)
	}

	none, _ := m.ParseForce("")
	if got := ids(m.GoServed(none)); got != "GET /a,POST /b,GET /c" {
		t.Fatalf("served = %s", got)
	}
	group, err := m.ParseForce(" g1 , ")
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(m.GoServed(group)); got != "GET /c" {
		t.Fatalf("served after forcing g1 = %s", got)
	}
	one, err := m.ParseForce("GET /c")
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(m.GoServed(one)); got != "GET /a,POST /b" {
		t.Fatalf("served after forcing GET /c = %s", got)
	}
	all, _ := m.ParseForce("all")
	if got := ids(m.GoServed(all)); got != "" {
		t.Fatalf("served after all = %s", got)
	}
	if _, err := m.ParseForce("GET /nope"); err == nil {
		t.Fatal("unknown token accepted")
	}
}

func TestForceRefusesFrozen(t *testing.T) {
	a := goOwned(row(0, "GET", "/a", "g1"))
	a.State, a.Python = "FROZEN", PythonFrozen
	m, err := Parse(encode(t, []Route{a}))
	if err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{"GET /a", "g1", "all"} {
		if _, err := m.ParseForce(token); err == nil || !strings.Contains(err.Error(), "frozen") {
			t.Fatalf("token %q: err = %v", token, err)
		}
	}
}

func ids(routes []Route) string {
	parts := make([]string, len(routes))
	for i, r := range routes {
		parts[i] = r.ID
	}
	return strings.Join(parts, ",")
}

// The one framework row Go may serve is named, and naming it must not open the
// door for the other four -- each of those is rendered BY FastAPI and needs its
// own decision. The allowlist is also not a way past the evidence rule.
func TestOnlyTheNamedFrameworkRowMayMoveToGo(t *testing.T) {
	mount := func(id, path string) Route {
		return Route{ID: id, Order: 0, Kind: "mount", Method: "MOUNT", Path: path,
			Group: "framework", Class: "framework", Owner: OwnerPython, Python: PythonLive, State: "PY"}
	}
	static := goOwned(mount("MOUNT /static", "/static"))
	if _, err := Parse(encode(t, []Route{static})); err != nil {
		t.Fatalf("MOUNT /static is the named row, yet: %v", err)
	}

	// Still refused without evidence: the allowlist skips one rule, not two.
	noEvidence := static
	noEvidence.Evidence = ""
	if _, err := Parse(encode(t, []Route{noEvidence})); err == nil ||
		!strings.Contains(err.Error(), "without evidence") {
		t.Fatalf("evidence is still required; err = %v", err)
	}

	// The rows FastAPI renders are still refused.
	for _, id := range []string{"GET /openapi.json", "GET /docs", "GET /redoc", "GET /docs/oauth2-redirect"} {
		method, path, _ := strings.Cut(id, " ")
		r := goOwned(Route{ID: id, Order: 0, Kind: "route", Method: method, Path: path,
			Group: "framework", Class: "framework", Owner: OwnerPython, Python: PythonLive, State: "PY"})
		if _, err := Parse(encode(t, []Route{r})); err == nil ||
			!strings.Contains(err.Error(), "only API routes") {
			t.Fatalf("%s should still be refused; err = %v", id, err)
		}
	}

	// And a mount that merely looks like it is not the named one.
	other := goOwned(mount("MOUNT /statics", "/statics"))
	if _, err := Parse(encode(t, []Route{other})); err == nil ||
		!strings.Contains(err.Error(), "only API routes") {
		t.Fatalf("MOUNT /statics is not the named row; err = %v", err)
	}
}

func feature(method, path, pkg string) Feature {
	return Feature{ID: method + " " + path, Method: method, Path: path, Package: pkg, State: StateGoOnly,
		Evidence: "services/core/internal/chatassist/postgres_test.go"}
}

func TestFeatureRowsValidate(t *testing.T) {
	a := row(0, "GET", "/a", "g1")
	good := feature("POST", "/contexts/{context}/ai-invocations", "chatassist")
	encodeWith := func(features ...Feature) []byte {
		data, err := json.Marshal(Manifest{Schema: 1, Routes: []Route{a}, Features: features})
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	if _, err := Parse(encodeWith(good)); err != nil {
		t.Fatalf("good feature refused: %v", err)
	}
	cases := map[string]struct {
		f    Feature
		want string
	}{
		"id mismatch":     {func() Feature { f := good; f.ID = "POST /x"; return f }(), "id must be"},
		"bad method":      {func() Feature { f := good; f.Method = "HEAD"; f.ID = "HEAD " + f.Path; return f }(), "id must be"},
		"unknown package": {func() Feature { f := good; f.Package = "payments"; return f }(), "unknown package"},
		"wrong state":     {func() Feature { f := good; f.State = "LIVE-GO"; return f }(), "state"},
		"no evidence":     {func() Feature { f := good; f.Evidence = ""; return f }(), "without evidence"},
		"collides route":  {feature("GET", "/a", "chatassist"), "duplicate"},
	}
	for name, c := range cases {
		_, err := Parse(encodeWith(c.f))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: got %v, want error containing %q", name, err, c.want)
		}
	}
	if _, err := Parse(encodeWith(good, good)); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("duplicate feature: got %v", err)
	}
}

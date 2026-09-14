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

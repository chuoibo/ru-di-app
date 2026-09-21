package ownership

import (
	"strings"
	"testing"
)

func routeIDs(routes []Route) string {
	names := make([]string, len(routes))
	for i, r := range routes {
		names[i] = r.ID
	}
	return strings.Join(names, ",")
}

func TestCandidatesAreOnlyMergedGoCodeAndForceStillWins(t *testing.T) {
	m := &Manifest{Routes: []Route{
		{ID: "GET /a", Order: 0, Group: "g1", Owner: OwnerPython, State: "PORTED"},
		{ID: "GET /b", Order: 1, Group: "g1", Owner: OwnerPython, State: "CARDED"},
		{ID: "GET /c", Order: 2, Group: "g2", Owner: OwnerPython, State: "RERUN-PASS"},
		{ID: "GET /d", Order: 3, Group: "g2", Owner: OwnerGo, State: "LIVE-GO"},
		{ID: "GET /e", Order: 4, Group: "g3", Owner: OwnerPython, State: "PY"},
	}}
	none := Force{Routes: map[string]bool{}}
	cases := []struct {
		raw   string
		force Force
		want  string
		err   string
	}{
		{"", none, "", ""},
		{"ported", none, "GET /a,GET /c", ""},
		{" g1 , GET /c ", none, "GET /a,GET /c", ""},
		{"GET /c", none, "GET /c", ""},
		{"g2", none, "GET /c", ""},
		{"GET /b", none, "", "is CARDED"},
		{"GET /e", none, "", "is PY"},
		{"g3", none, "", "no ported route"},
		{"nope", none, "", "unknown token"},
		{"ported", Force{Routes: map[string]bool{"GET /a": true}}, "GET /c", ""},
		{"ported", Force{All: true, Routes: map[string]bool{}}, "", ""},
	}
	for _, tc := range cases {
		got, err := m.ParseCandidates(tc.raw, tc.force)
		if tc.err != "" {
			if err == nil || !strings.Contains(err.Error(), tc.err) {
				t.Fatalf("%q: got %q %v, want error containing %q", tc.raw, routeIDs(got), err, tc.err)
			}
			continue
		}
		if err != nil || routeIDs(got) != tc.want {
			t.Fatalf("%q: got %q %v, want %q", tc.raw, routeIDs(got), err, tc.want)
		}
	}
}

func TestACandidateCannotSplitSharedInMemoryState(t *testing.T) {
	split := &Manifest{Routes: []Route{
		{ID: "GET /x", Group: "g", Owner: OwnerPython, State: "PORTED", InMemory: []string{"limiter"}},
		{ID: "GET /y", Group: "g", Owner: OwnerPython, State: "CARDED", InMemory: []string{"limiter"}},
	}}
	if _, err := split.ParseCandidates("GET /x", Force{Routes: map[string]bool{}}); err == nil || !strings.Contains(err.Error(), "shares in-memory") {
		t.Fatalf("split limiter accepted: %v", err)
	}
	together := &Manifest{Routes: []Route{
		{ID: "GET /x", Group: "g", Owner: OwnerPython, State: "PORTED", InMemory: []string{"limiter"}},
		{ID: "GET /y", Group: "g", Owner: OwnerPython, State: "PORTED", InMemory: []string{"limiter"}},
	}}
	got, err := together.ParseCandidates("ported", Force{Routes: map[string]bool{}})
	if err != nil || routeIDs(got) != "GET /x,GET /y" {
		t.Fatalf("limiter moving together refused: %q %v", routeIDs(got), err)
	}
}

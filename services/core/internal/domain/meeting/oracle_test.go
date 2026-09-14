package meeting

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"mobile/services/core/internal/domain/areas"
)

// testdata/python_meeting*.json is rendered by
// scripts/render_places_geo_goldens.py from app/places/meeting.py in the
// pinned API image. See that script for the row encoding.

type meetingCase struct {
	Name       string   `json:"name"`
	Origins    []string `json:"origins"`
	Places     []string `json:"places"`
	Limit      int      `json:"limit"`
	Raise      *string  `json:"raise"`
	Candidates []string `json:"candidates"`
}

type meetingFile struct {
	Host struct {
		Machine string `json:"machine"`
		Libc    string `json:"libc"`
		FMAAVX2 bool   `json:"fma_avx2"`
	} `json:"host"`
	Fuzz *struct {
		Shards int `json:"shards"`
		Total  int `json:"total"`
	} `json:"fuzz"`
	Constants *struct {
		Min           int      `json:"min_origin_areas"`
		Max           int      `json:"max_origin_areas"`
		CandidateKeys []string `json:"candidate_keys"`
		FairnessKeys  []string `json:"fairness_keys"`
		LegKeys       []string `json:"leg_keys"`
	} `json:"constants"`
	Round2 []string      `json:"round2"`
	Sum    []string      `json:"sum"`
	Cases  []meetingCase `json:"cases"`
}

type meetingCorpus struct {
	edge meetingFile
	fuzz []meetingCase
}

func loadMeeting(t *testing.T) meetingCorpus {
	t.Helper()
	paths, err := filepath.Glob("testdata/python_meeting*.json")
	if err != nil || len(paths) == 0 {
		t.Fatalf("no meeting oracle files: %v", err)
	}
	sort.Strings(paths)
	var corpus meetingCorpus
	edges, total, shards := 0, -1, 0
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var file meetingFile
		if err := json.Unmarshal(raw, &file); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if !file.Host.FMAAVX2 || file.Host.Machine != "x86_64" || file.Host.Libc != "glibc 2.41" {
			t.Fatalf("%s was rendered on %+v; the areas port reproduces glibc 2.41's FMA variants on x86-64", path, file.Host)
		}
		if file.Fuzz == nil {
			edges++
			corpus.edge = file
			continue
		}
		shards++
		if total >= 0 && total != file.Fuzz.Total {
			t.Fatalf("%s: shard totals disagree", path)
		}
		total = file.Fuzz.Total
		if shards > file.Fuzz.Shards {
			t.Fatalf("%s: more shard files than declared", path)
		}
		corpus.fuzz = append(corpus.fuzz, file.Cases...)
	}
	if edges != 1 || corpus.edge.Constants == nil {
		t.Fatalf("want one meeting edge file with constants, found %d", edges)
	}
	if total < 2000 || len(corpus.fuzz) != total {
		t.Fatalf("fuzz has %d of %d cases; need every shard and at least 2000", len(corpus.fuzz), total)
	}
	return corpus
}

func pyFloat(t *testing.T, s string) float64 {
	t.Helper()
	v, err := strconv.ParseFloat(strings.ReplaceAll(s, "_", ""), 64)
	if err != nil {
		t.Fatalf("bad float %q: %v", s, err)
	}
	return v
}

func sameFloat(a, b float64) bool {
	if math.IsNaN(a) || math.IsNaN(b) {
		return math.IsNaN(a) && math.IsNaN(b)
	}
	return math.Float64bits(a) == math.Float64bits(b)
}

func fields(t *testing.T, row string, n int) []string {
	t.Helper()
	f := strings.Split(row, "|")
	if len(f) != n {
		t.Fatalf("row %q has %d fields, want %d", row, len(f), n)
	}
	return f
}

func optString(s string) *string {
	if s == "~" {
		return nil
	}
	return &s
}

func TestMeetingConstantsMatchPython(t *testing.T) {
	c := loadMeeting(t).edge.Constants
	if c.Min != MinOriginAreas || c.Max != MaxOriginAreas {
		t.Fatalf("origin bounds: Go %d..%d, Python %d..%d", MinOriginAreas, MaxOriginAreas, c.Min, c.Max)
	}
	want := map[string][]string{
		"candidate": {"place_id", "place_name", "category", "address", "lat", "lng", "fairness", "travel"},
		"fairness":  {"worst_km", "total_km", "spread_km"},
		"leg":       {"id", "label", "lat", "lng", "km"},
	}
	got := map[string][]string{"candidate": c.CandidateKeys, "fairness": c.FairnessKeys, "leg": c.LegKeys}
	for name, keys := range want {
		if strings.Join(keys, ",") != strings.Join(got[name], ",") {
			t.Fatalf("%s keys: Python now %v", name, got[name])
		}
	}
}

func TestRound2MatchesPython(t *testing.T) {
	rows := loadMeeting(t).edge.Round2
	ties := 0
	for _, r := range rows {
		f := fields(t, r, 2)
		x := pyFloat(t, f[0])
		want := pyFloat(t, strings.TrimPrefix(f[1], "ok:"))
		if got := pyRound2(x); !sameFloat(got, want) {
			t.Errorf("round(%v, 2): Go %v, Python %v", x, got, want)
		}
		if math.Abs(x*200-math.Round(x*200)) < 1e-9 && math.Mod(math.Round(x*200), 2) == 1 {
			ties++
		}
	}
	if len(rows) < 1000 || ties < 100 {
		t.Fatalf("round corpus too thin: %d rows, %d near half-cent ties", len(rows), ties)
	}
}

func TestSumMatchesPython(t *testing.T) {
	for _, r := range loadMeeting(t).edge.Sum {
		f := fields(t, r, 2)
		var items []float64
		for _, s := range strings.Split(f[0], ";") {
			items = append(items, pyFloat(t, s))
		}
		want := pyFloat(t, strings.TrimPrefix(f[1], "ok:"))
		if got := pySum(items); !sameFloat(got, want) {
			t.Errorf("sum(%v): Go %v, Python %v", items, got, want)
		}
	}
}

func decodeCase(t *testing.T, c meetingCase) ([]areas.Area, []Place) {
	t.Helper()
	origins := make([]areas.Area, len(c.Origins))
	for i, r := range c.Origins {
		f := fields(t, r, 4)
		origins[i] = areas.Area{ID: f[0], Label: f[1], Lat: pyFloat(t, f[2]), Lng: pyFloat(t, f[3])}
	}
	places := make([]Place, len(c.Places))
	for i, r := range c.Places {
		f := fields(t, r, 6)
		places[i] = Place{ID: f[0], Name: f[1], Category: f[2], Address: optString(f[3]), Lat: pyFloat(t, f[4]), Lng: pyFloat(t, f[5])}
	}
	return origins, places
}

// replay runs one case and returns (order mismatch, value mismatch).
func replay(t *testing.T, c meetingCase) (orderBad, valueBad bool) {
	t.Helper()
	origins, places := decodeCase(t, c)
	got, err := RankMeetingPoints(origins, places, c.Limit)
	if c.Raise != nil || err != nil {
		var me *areas.MathError
		if c.Raise == nil || err == nil || !errors.As(err, &me) || "raise:"+me.Type+":"+me.Message != *c.Raise {
			t.Errorf("%s: Go %v, Python raise %v", c.Name, err, c.Raise)
			return true, true
		}
		return false, false
	}
	if len(got) != len(c.Candidates) {
		t.Errorf("%s: Go %d candidates, Python %d", c.Name, len(got), len(c.Candidates))
		return true, true
	}
	for i, r := range c.Candidates {
		f := fields(t, r, 10)
		g := got[i]
		if g.PlaceID != f[0] || g.PlaceName != f[1] {
			orderBad = true
		}
		addr := optString(f[3])
		sameAddr := (addr == nil) == (g.Address == nil) && (addr == nil || *addr == *g.Address)
		kms := strings.Split(f[9], ";")
		if g.Category != f[2] || !sameAddr || !sameFloat(g.Lat, pyFloat(t, f[4])) || !sameFloat(g.Lng, pyFloat(t, f[5])) ||
			!sameFloat(g.Fairness.WorstKm, pyFloat(t, f[6])) || !sameFloat(g.Fairness.TotalKm, pyFloat(t, f[7])) ||
			!sameFloat(g.Fairness.SpreadKm, pyFloat(t, f[8])) || len(kms) != len(g.Travel) || len(g.Travel) != len(origins) {
			valueBad = true
		} else {
			for j, leg := range g.Travel {
				o := origins[j]
				if !sameFloat(leg.Km, pyFloat(t, kms[j])) || leg.ID != o.ID || leg.Label != o.Label || !sameFloat(leg.Lat, o.Lat) || !sameFloat(leg.Lng, o.Lng) {
					valueBad = true
				}
			}
		}
		if orderBad || valueBad {
			t.Errorf("%s candidate %d: Go %+v, Python %s", c.Name, i, g, r)
			return orderBad, valueBad
		}
	}
	return false, false
}

func TestRankMeetingPointsMatchesPython(t *testing.T) {
	corpus := loadMeeting(t)
	all := append(append([]meetingCase(nil), corpus.edge.Cases...), corpus.fuzz...)
	orderBad, valueBad, raises, ranked := 0, 0, 0, 0
	for _, c := range all {
		o, v := replay(t, c)
		if o {
			orderBad++
		}
		if v {
			valueBad++
		}
		if c.Raise != nil {
			raises++
		} else if len(c.Candidates) > 1 {
			ranked++
		}
	}
	t.Logf("rank_meeting_points: %d cases (%d fuzz, %d raise, %d with several candidates); order mismatches %d, value mismatches %d",
		len(all), len(corpus.fuzz), raises, ranked, orderBad, valueBad)
	if raises == 0 || ranked < 500 {
		t.Fatalf("meeting corpus too thin: %d raises, %d multi-candidate answers", raises, ranked)
	}
}

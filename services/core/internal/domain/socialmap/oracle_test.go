package socialmap

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

// testdata/python_socialmap*.json is rendered by
// scripts/render_places_geo_goldens.py from app/places/social_map.py in the
// pinned API image. See that script for the row encoding.

type rowsOutcome struct {
	Rows  []string `json:"rows"`
	Raise *string  `json:"raise"`
}

type socialCase struct {
	Name     string       `json:"name"`
	Checkins []string     `json:"checkins"`
	Visited  *rowsOutcome `json:"visited"`
	Heatmap  *rowsOutcome `json:"heatmap"`
	Unknown  *string      `json:"unknown"`
	Places   []string     `json:"places"`
	Trending *rowsOutcome `json:"trending"`
}

type socialFile struct {
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
		Private      []string `json:"private_checkin_fields"`
		VisitedKeys  []string `json:"visited_keys"`
		TrendingKeys []string `json:"trending_keys"`
		HeatmapKeys  []string `json:"heatmap_keys"`
	} `json:"constants"`
	Cases json.RawMessage `json:"cases"`
}

type socialCorpus struct {
	constants socialFile
	checkins  []socialCase
	trending  []socialCase
	fuzz      int
}

func loadSocial(t *testing.T) socialCorpus {
	t.Helper()
	paths, err := filepath.Glob("testdata/python_socialmap*.json")
	if err != nil || len(paths) == 0 {
		t.Fatalf("no socialmap oracle files: %v", err)
	}
	sort.Strings(paths)
	var corpus socialCorpus
	edges, total, shards := 0, -1, 0
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var file socialFile
		if err := json.Unmarshal(raw, &file); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if !file.Host.FMAAVX2 || file.Host.Machine != "x86_64" || file.Host.Libc != "glibc 2.41" {
			t.Fatalf("%s was rendered on %+v; the areas port reproduces glibc 2.41's FMA variants on x86-64", path, file.Host)
		}
		if file.Fuzz == nil {
			edges++
			corpus.constants = file
			var named struct {
				Checkins []socialCase `json:"checkins"`
				Trending []socialCase `json:"trending"`
			}
			if err := json.Unmarshal(file.Cases, &named); err != nil {
				t.Fatalf("%s: %v", path, err)
			}
			corpus.checkins = append(corpus.checkins, named.Checkins...)
			corpus.trending = append(corpus.trending, named.Trending...)
			continue
		}
		var cases []socialCase
		if err := json.Unmarshal(file.Cases, &cases); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		shards++
		if total >= 0 && total != file.Fuzz.Total {
			t.Fatalf("%s: shard totals disagree", path)
		}
		total = file.Fuzz.Total
		if shards > file.Fuzz.Shards {
			t.Fatalf("%s: more shard files than declared", path)
		}
		corpus.fuzz += len(cases)
		corpus.checkins = append(corpus.checkins, cases...)
		corpus.trending = append(corpus.trending, cases...)
	}
	if edges != 1 || corpus.constants.Constants == nil {
		t.Fatalf("want one socialmap edge file with constants, found %d", edges)
	}
	if total < 2000 || corpus.fuzz != total {
		t.Fatalf("fuzz has %d of %d cases; need every shard and at least 2000", corpus.fuzz, total)
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

func optFloat(t *testing.T, s string) *float64 {
	if s == "~" {
		return nil
	}
	v := pyFloat(t, s)
	return &v
}

func optInt(t *testing.T, s string) *int64 {
	if s == "~" {
		return nil
	}
	v, err := strconv.ParseInt(strings.ReplaceAll(s, "_", ""), 10, 64)
	if err != nil {
		t.Fatalf("bad int %q", s)
	}
	return &v
}

func atoi(t *testing.T, s string) int {
	v, err := strconv.Atoi(strings.ReplaceAll(s, "_", ""))
	if err != nil {
		t.Fatalf("bad int %q", s)
	}
	return v
}

func sameOptFloat(a, b *float64) bool {
	return (a == nil) == (b == nil) && (a == nil || sameFloat(*a, *b))
}

func raiseOf(err error) string {
	var me *areas.MathError
	if !errors.As(err, &me) {
		return "raise:<non-math error>:" + err.Error()
	}
	return "raise:" + me.Type + ":" + me.Message
}

func TestSocialMapConstantsMatchPython(t *testing.T) {
	c := loadSocial(t).constants.Constants
	checks := map[string][2][]string{
		"private":  {PrivateCheckinFields(), c.Private},
		"visited":  {{"place_id", "place_name", "lat", "lng", "visit_count"}, c.VisitedKeys},
		"trending": {{"place_id", "place_name", "lat", "lng", "rating", "rating_count"}, c.TrendingKeys},
		"heatmap":  {{"id", "label", "lat", "lng", "visit_count", "share_percent"}, c.HeatmapKeys},
	}
	for name, pair := range checks {
		if strings.Join(pair[0], ",") != strings.Join(pair[1], ",") {
			t.Fatalf("%s: Go %v, Python %v", name, pair[0], pair[1])
		}
	}
}

func decodeCheckins(t *testing.T, rows []string) []Checkin {
	out := make([]Checkin, len(rows))
	for i, r := range rows {
		f := fields(t, r, 4)
		out[i] = Checkin{PlaceID: optString(f[0]), PlaceName: optString(f[1]), Lat: optFloat(t, f[2]), Lng: optFloat(t, f[3])}
	}
	return out
}

func TestCheckinLayersMatchPython(t *testing.T) {
	corpus := loadSocial(t)
	visitedRows, heatRows, raises, unknownPositive := 0, 0, 0, 0
	for _, c := range corpus.checkins {
		checkins := decodeCheckins(t, c.Checkins)

		visited := VisitedLayer(checkins)
		if c.Visited.Raise != nil || len(visited) != len(c.Visited.Rows) {
			t.Fatalf("%s visited: Go %+v, Python %+v", c.Name, visited, c.Visited)
		}
		for i, r := range c.Visited.Rows {
			f := fields(t, r, 5)
			g := visited[i]
			if g.PlaceID != f[0] || g.PlaceName != f[1] || !sameFloat(g.Lat, pyFloat(t, f[2])) || !sameFloat(g.Lng, pyFloat(t, f[3])) || g.VisitCount != atoi(t, f[4]) {
				t.Fatalf("%s visited %d: Go %+v, Python %s", c.Name, i, g, r)
			}
		}
		visitedRows += len(visited)

		heat, err := HeatmapRows(checkins)
		switch {
		case c.Heatmap.Raise != nil || err != nil:
			if c.Heatmap.Raise == nil || err == nil || raiseOf(err) != *c.Heatmap.Raise {
				t.Fatalf("%s heatmap: Go %v, Python %v", c.Name, err, c.Heatmap.Raise)
			}
			raises++
		case len(heat) != len(c.Heatmap.Rows):
			t.Fatalf("%s heatmap: Go %+v, Python %v", c.Name, heat, c.Heatmap.Rows)
		default:
			for i, r := range c.Heatmap.Rows {
				f := fields(t, r, 6)
				g := heat[i]
				if g.ID != f[0] || g.Label != f[1] || !sameFloat(g.Lat, pyFloat(t, f[2])) || !sameFloat(g.Lng, pyFloat(t, f[3])) ||
					g.VisitCount != atoi(t, f[4]) || g.SharePercent != atoi(t, f[5]) {
					t.Fatalf("%s heatmap %d: Go %+v, Python %s", c.Name, i, g, r)
				}
			}
			heatRows += len(heat)
		}

		unknown, err := UnknownAreaCount(checkins)
		got := "ok:" + strconv.Itoa(unknown)
		if err != nil {
			got = raiseOf(err)
		}
		if c.Unknown == nil || got != strings.ReplaceAll(*c.Unknown, "_", "") {
			t.Fatalf("%s unknown_area_count: Go %s, Python %v", c.Name, got, c.Unknown)
		}
		if unknown > 0 {
			unknownPositive++
		}
	}
	t.Logf("check-in cases %d: visited rows %d, heatmap rows %d, heatmap raises %d, unknown>0 in %d", len(corpus.checkins), visitedRows, heatRows, raises, unknownPositive)
	if raises == 0 || unknownPositive == 0 || heatRows < 1000 || visitedRows < 1000 {
		t.Fatalf("check-in corpus too thin")
	}
}

func TestTrendingLayerMatchesPython(t *testing.T) {
	corpus := loadSocial(t)
	rows := 0
	for _, c := range corpus.trending {
		places := make([]Place, len(c.Places))
		for i, r := range c.Places {
			f := fields(t, r, 7)
			places[i] = Place{ID: f[0], Name: f[1], Lat: pyFloat(t, f[2]), Lng: pyFloat(t, f[3]), Rating: optFloat(t, f[4]), RatingCount: optInt(t, f[5]), Flag: optString(f[6])}
		}
		got := TrendingLayer(places)
		if c.Trending.Raise != nil || len(got) != len(c.Trending.Rows) {
			t.Fatalf("%s trending: Go %+v, Python %+v", c.Name, got, c.Trending)
		}
		for i, r := range c.Trending.Rows {
			f := fields(t, r, 6)
			g := got[i]
			count := optInt(t, f[5])
			sameCount := (count == nil) == (g.RatingCount == nil) && (count == nil || *count == *g.RatingCount)
			if g.PlaceID != f[0] || g.PlaceName != f[1] || !sameFloat(g.Lat, pyFloat(t, f[2])) || !sameFloat(g.Lng, pyFloat(t, f[3])) ||
				!sameOptFloat(g.Rating, optFloat(t, f[4])) || !sameCount {
				t.Fatalf("%s trending %d: Go %+v, Python %s", c.Name, i, g, r)
			}
		}
		rows += len(got)
	}
	t.Logf("trending cases %d, rows %d", len(corpus.trending), rows)
	if rows < 1000 {
		t.Fatalf("trending corpus too thin: %d rows", rows)
	}
}

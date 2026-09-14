package areas

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"
)

// testdata/python_areas.json is rendered by scripts/render_areas_goldens.py
// from app/places/areas.py in the pinned API image. Coordinates travel as
// Python's shortest repr, which parses back to the exact float64.
type golden struct {
	Areas []struct {
		ID          string   `json:"id"`
		Label       string   `json:"label"`
		Lat         string   `json:"lat"`
		Lng         string   `json:"lng"`
		SummaryKeys []string `json:"summary_keys"`
	} `json:"areas"`
	Find []struct {
		ID    string `json:"id"`
		Found bool   `json:"found"`
		Index *int   `json:"index"`
	} `json:"find"`
}

func load(t *testing.T) golden {
	t.Helper()
	raw, err := os.ReadFile("testdata/python_areas.json")
	if err != nil {
		t.Fatal(err)
	}
	var g golden
	if err := json.Unmarshal(raw, &g); err != nil {
		t.Fatal(err)
	}
	return g
}

func TestCatalogueMatchesPython(t *testing.T) {
	g := load(t)
	all := All()
	if len(g.Areas) == 0 || len(all) != len(g.Areas) {
		t.Fatalf("Go has %d areas, Python %d", len(all), len(g.Areas))
	}
	for i, want := range g.Areas {
		lat, errLat := strconv.ParseFloat(want.Lat, 64)
		lng, errLng := strconv.ParseFloat(want.Lng, 64)
		if errLat != nil || errLng != nil {
			t.Fatalf("golden %d: %v %v", i, errLat, errLng)
		}
		got := all[i]
		if got.ID != want.ID || got.Label != want.Label || got.Lat != lat || got.Lng != lng {
			t.Fatalf("area %d: Go %+v, Python %+v", i, got, want)
		}
		if strings.Join(want.SummaryKeys, ",") != "id,label,lat,lng" {
			t.Fatalf("area_summary now answers keys %v; GET /areas must change with it", want.SummaryKeys)
		}
	}
}

func TestFindMatchesPython(t *testing.T) {
	g := load(t)
	all := All()
	if len(g.Find) <= len(all) {
		t.Fatalf("only %d find probes: the corpus has no id Python refuses", len(g.Find))
	}
	for _, probe := range g.Find {
		area, ok := Find(probe.ID)
		if ok != probe.Found {
			t.Fatalf("Find(%q) = %v, Python %v", probe.ID, ok, probe.Found)
		}
		if ok && (probe.Index == nil || area != all[*probe.Index]) {
			t.Fatalf("Find(%q) = %+v, Python index %v", probe.ID, area, probe.Index)
		}
	}
}

func TestAllReturnsACopy(t *testing.T) {
	first := All()
	first[0].Label = "changed"
	if All()[0].Label == "changed" {
		t.Fatal("All exposes the catalogue itself")
	}
}

package plugins

// Plain regression check without Docker: inputs rebuilt from seeds, outcomes
// compared with testdata/golden.json, whose entries the oracle test showed
// identical to Pillow. Regenerate with PLUGINS_GOLDEN_WRITE=1.

import (
	"encoding/json"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
)

type goldenEntry struct {
	Index      int     `json:"index"`
	Format     string  `json:"format"`
	Label      string  `json:"label"`
	Accept     string  `json:"accept"`
	Open       string  `json:"open"`
	Mode       string  `json:"mode,omitempty"`
	W          float64 `json:"w,omitempty"`
	H          float64 `json:"h,omitempty"`
	Load       string  `json:"load,omitempty"`
	LoadedMode string  `json:"loaded_mode,omitempty"`
	SHA16      string  `json:"sha16,omitempty"`
}

func goldenCorpus() []testCase {
	r := rand.New(rand.NewSource(7))
	var all []testCase
	for _, build := range []func(*rand.Rand) []testCase{buildTGA, buildSUN, buildSGI, buildQOI, buildPCXPlanar, buildText, buildIPTC, buildMisc, buildTIFF} {
		all = append(all, build(r)...)
	}
	return all
}

func goldenOf(index int, c testCase) goldenEntry {
	o := goOutcome(ByName()[c.format], c.data)
	e := goldenEntry{Index: index, Format: c.format, Label: c.label, Accept: o.accept, Open: o.open,
		Mode: o.mode, W: o.w, H: o.h, Load: o.load, LoadedMode: o.loadedMode}
	if len(o.sha) >= 16 {
		e.SHA16 = o.sha[:16]
	}
	return e
}

func TestGoldenOutcomes(t *testing.T) {
	corpus := goldenCorpus()
	path := filepath.Join("testdata", "golden.json")
	if os.Getenv("PLUGINS_GOLDEN_WRITE") == "1" {
		// One decoded image per format, then other outcomes, 30 at most.
		decoded := map[string]bool{}
		var entries, others []goldenEntry
		for i, c := range corpus {
			e := goldenOf(i, c)
			switch {
			case e.Load == "ok" && !decoded[c.format]:
				decoded[c.format] = true
				entries = append(entries, e)
			case e.Load != "ok" && len(others) < 8:
				others = append(others, e)
			}
		}
		entries = append(entries, others...)
		if len(entries) > 30 {
			entries = entries[:30]
		}
		data, err := json.MarshalIndent(entries, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %d golden entries", len(entries))
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var want []goldenEntry
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	if len(want) == 0 {
		t.Fatal("empty golden")
	}
	for _, w := range want {
		if w.Index >= len(corpus) {
			t.Fatalf("golden index %d beyond corpus of %d", w.Index, len(corpus))
		}
		if got := goldenOf(w.Index, corpus[w.Index]); got != w {
			t.Errorf("case %d (%s %s)\n got: %+v\nwant: %+v", w.Index, w.Format, w.Label, got, w)
		}
	}
}

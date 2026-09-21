package pngdec

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

const goldenPath = "testdata/golden.json"

// goldenLimit keeps the committed golden small: a sample of the corpus,
// covering every class.
const goldenLimit = 30

// goldenFile maps a corpus case name to what Pillow answered: the encoder
// mode and the first 16 hex characters of the sha256 of its pixels, or the
// outcome class ("next", "bomb", "error").
type goldenFile struct {
	Note  string            `json:"note"`
	Cases map[string]string `json:"cases"`
}

var longDigits = regexp.MustCompile(`\d{9,}`)

// writeGolden keeps a class-balanced sample of oracle-verified answers.
func writeGolden(t *testing.T, answers map[string]string) {
	t.Helper()
	names := make([]string, 0, len(answers))
	for name := range answers {
		if !longDigits.MatchString(answers[name]) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	byClass := map[string][]string{}
	var classes []string
	for _, name := range names {
		class := name[:strings.IndexByte(name, '/')]
		if byClass[class] == nil {
			classes = append(classes, class)
		}
		byClass[class] = append(byClass[class], name)
	}
	picked := map[string]string{}
	for round := 0; len(picked) < goldenLimit; round++ {
		progress := false
		for _, class := range classes {
			list := byClass[class]
			if len(picked) < goldenLimit && round < len(list) {
				// spread picks across the class
				name := list[(round*7)%len(list)]
				if _, dup := picked[name]; !dup {
					picked[name] = answers[name]
					progress = true
				}
			}
		}
		if !progress {
			break
		}
	}
	data, err := json.MarshalIndent(goldenFile{
		Note:  "Pillow 12.2.0 answers for corpus() cases, regenerated from seeds; update with MEDIA_GOLDEN_UPDATE=1 go test -tags oracle",
		Cases: picked,
	}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(goldenPath, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote %d golden cases", len(picked))
}

// TestGolden replays the committed sample without Docker.
func TestGolden(t *testing.T) {
	raw, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}
	var golden goldenFile
	if err := json.Unmarshal(raw, &golden); err != nil {
		t.Fatal(err)
	}
	if len(golden.Cases) == 0 {
		t.Fatal("empty golden")
	}
	byName := map[string]pngCase{}
	for _, c := range corpus() {
		byName[c.name] = c
	}
	for name, want := range golden.Cases {
		c, ok := byName[name]
		if !ok {
			t.Errorf("%s: not in corpus", name)
			continue
		}
		outcome, mode, pix := goPixels(c.data)
		got := outcome.class
		if got == "ok" {
			got = mode + ":" + sha(pix)[:16]
		}
		if got != want {
			t.Errorf("%s: got %s, Pillow %s (%s)", name, got, want, outcome.detail)
		}
	}
}

//go:build oracle

package jpegenc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"testing"
	"time"

	"mobile/services/core/internal/media/sanitize/internal/pyoracle"
)

type oracleCase struct {
	genCase
	class string
}

// TestOracleEncodeRGB compares EncodeRGB with Pillow's
// Image.frombytes("RGB").save(JPEG, quality=88, optimize=True) in the
// pinned image, byte for byte, including the cases where Pillow raises.
// JPEGENC_WRITE_GOLDEN=1 rewrites testdata/golden.json from Pillow's
// answers once every case agrees.
func TestOracleEncodeRGB(t *testing.T) {
	var cases []oracleCase
	add := func(class string, c genCase) { cases = append(cases, oracleCase{genCase: c, class: class}) }

	sizes := [][2]int{{1, 1}, {1, 2}, {2, 1}, {7, 9}, {8, 8}, {9, 9}, {15, 17}, {16, 16}, {17, 15},
		{33, 1}, {1, 33}, {640, 480}, {1001, 750}}
	patterns := []string{"noise", "gradient", "flat", "edges", "saturated", "black", "white", "photo"}
	seed := uint64(1)
	for _, size := range sizes {
		for _, pattern := range patterns {
			add(pattern, genCase{W: size[0], H: size[1], Pattern: pattern, Seed: seed})
			seed++
		}
	}
	add("large", genCase{W: 4032, H: 3024, Pattern: "photo", Seed: 900})
	add("large", genCase{W: 4032, H: 3024, Pattern: "edges", Seed: 901})
	add("large", genCase{W: 4032, H: 3024, Pattern: "noise", Seed: 902})
	add("limit", genCase{W: 65500, H: 1, Pattern: "gradient", Seed: 910})
	add("limit", genCase{W: 65501, H: 1, Pattern: "gradient", Seed: 911})
	add("limit", genCase{W: 1, H: 65501, Pattern: "gradient", Seed: 912})
	for _, bound := range []struct {
		w, h int
		seed uint64
	}{{13000, 5, 1000}, {5, 20000, 2000}, {20000, 3, 3000}} {
		for _, c := range boundaryCases(t, bound.w, bound.h, bound.seed) {
			add("boundary", c)
		}
	}

	requests := make([]pyoracle.Request, len(cases))
	inputs := make([][]byte, len(cases))
	for i, c := range cases {
		inputs[i] = c.pixels()
		requests[i] = pyoracle.Request{ID: c.name(), Op: "encode_jpeg", W: c.W, H: c.H, Input: inputs[i]}
	}
	started := time.Now()
	answers := pyoracle.Run(t, requests)
	t.Logf("oracle answered %d cases in %s", len(cases), time.Since(started).Round(time.Millisecond))

	type tally struct{ cases, identical int }
	byClass := map[string]*tally{}
	var mismatched []int
	for i, c := range cases {
		counts := byClass[c.class]
		if counts == nil {
			counts = &tally{}
			byClass[c.class] = counts
		}
		counts.cases++
		began := time.Now()
		got, err := EncodeRGB(c.W, c.H, inputs[i])
		if c.W*c.H >= 1_000_000 {
			t.Logf("%s: Go encoded in %s", c.name(), time.Since(began).Round(time.Millisecond))
		}
		if answers[i].Result != "ok" || c.class == "boundary" {
			t.Logf("%s: Pillow %s, Go %s", c.name(), describePillow(answers[i]), describeGo(got, err))
		}
		if agrees(got, err, answers[i]) {
			counts.identical++
			continue
		}
		mismatched = append(mismatched, i)
		t.Errorf("%s: Go %s, Pillow %s", c.name(), describeGo(got, err), describePillow(answers[i]))
	}
	classes := make([]string, 0, len(byClass))
	for class := range byClass {
		classes = append(classes, class)
	}
	sort.Strings(classes)
	for _, class := range classes {
		t.Logf("class %-10s cases %3d identical %3d", class, byClass[class].cases, byClass[class].identical)
	}
	if len(mismatched) > 0 {
		diagnose(t, cases, inputs, mismatched)
		return
	}
	if os.Getenv("JPEGENC_WRITE_GOLDEN") == "1" {
		writeGolden(t, cases, answers)
	}
}

func agrees(got []byte, err error, answer pyoracle.Response) bool {
	switch answer.Result {
	case "ok":
		return err == nil && len(got) == answer.Size && shaHex(got) == answer.SHA256
	case "error":
		var encodeErr *EncodeError
		return errors.As(err, &encodeErr) && encodeErr.PyType == answer.Type && encodeErr.PyMessage == answer.Message
	}
	return false
}

func describeGo(got []byte, err error) string {
	if err != nil {
		return err.Error()
	}
	return fmt.Sprintf("%d bytes %s", len(got), sha16(got))
}

func describePillow(answer pyoracle.Response) string {
	if answer.Result == "ok" {
		return fmt.Sprintf("%d bytes %s", answer.Size, answer.SHA256[:16])
	}
	return fmt.Sprintf("%s %s: %s", answer.Result, answer.Type, answer.Message)
}

// boundaryCases finds "mixed" images whose unbounded output is one byte
// under, exactly at and one byte over Pillow's buffer for w x h.
func boundaryCases(t *testing.T, w, h int, seed uint64) []genCase {
	t.Helper()
	target := PillowBufferSize(w, h, sanitizeQuality, sanitizeOptimize)
	size := func(c genCase) int {
		out, err := Encode(unlimited(c.W, c.H), c.pixels())
		if err != nil {
			t.Fatalf("%s: %v", c.name(), err)
		}
		return len(out)
	}
	found := map[int]genCase{}
	for s := seed; s < seed+40 && len(found) < 3; s++ {
		lo, hi := 0, w*h
		if size(genCase{W: w, H: h, Pattern: "mixed", Seed: s, Noise: hi}) < target+1 {
			t.Fatalf("%dx%d: full noise stays under %d bytes", w, h, target)
		}
		for lo < hi {
			mid := (lo + hi) / 2
			if size(genCase{W: w, H: h, Pattern: "mixed", Seed: s, Noise: mid}) < target {
				lo = mid + 1
			} else {
				hi = mid
			}
		}
		for k := max(lo-150, 0); k <= min(lo+150, w*h); k++ {
			c := genCase{W: w, H: h, Pattern: "mixed", Seed: s, Noise: k}
			delta := size(c) - target
			if delta >= -1 && delta <= 1 {
				if _, ok := found[delta]; !ok {
					found[delta] = c
				}
			}
		}
	}
	if len(found) < 3 {
		t.Logf("%dx%d: found output sizes %v around %d", w, h, keys(found), target)
	}
	var out []genCase
	for _, delta := range []int{-1, 0, 1} {
		if c, ok := found[delta]; ok {
			out = append(out, c)
		}
	}
	return out
}

func keys(m map[int]genCase) []int {
	out := make([]int, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Ints(out)
	return out
}

// diagnose asks Pillow for its bytes and names the first differing
// segment of each mismatched case.
func diagnose(t *testing.T, cases []oracleCase, inputs [][]byte, mismatched []int) {
	t.Helper()
	requests := make([]pyoracle.Request, 0, len(mismatched))
	for _, i := range mismatched {
		c := cases[i]
		requests = append(requests, pyoracle.Request{ID: c.name(), Op: "encode_jpeg", W: c.W, H: c.H,
			Input: inputs[i], Want: []string{"output"}})
	}
	for n, answer := range pyoracle.Run(t, requests) {
		i := mismatched[n]
		want := answer.Output
		got, err := Encode(unlimited(cases[i].W, cases[i].H), inputs[i])
		if err != nil || want == nil {
			t.Logf("%s: no byte comparison (Go err %v, Pillow %s)", cases[i].name(), err, answer.Result)
			continue
		}
		at := 0
		for at < len(got) && at < len(want) && got[at] == want[at] {
			at++
		}
		t.Logf("%s: first difference at byte %d of %d/%d, in %s", cases[i].name(), at, len(got), len(want), segmentAt(want, at))
	}
}

func segmentAt(data []byte, offset int) string {
	pos := 2
	for pos+4 <= len(data) {
		if data[pos] != 0xFF {
			return "unknown"
		}
		marker := data[pos+1]
		length := int(data[pos+2])<<8 | int(data[pos+3])
		end := pos + 2 + length
		if marker == 0xDA {
			if offset < end {
				return "SOS header"
			}
			return fmt.Sprintf("entropy data (+%d)", offset-end)
		}
		if offset < end {
			return fmt.Sprintf("marker %02X at %d", marker, pos)
		}
		pos = end
	}
	return "past end"
}

type goldenCase struct {
	genCase
	SHA16 string `json:"sha16,omitempty"`
	Size  int    `json:"size,omitempty"`
	Error string `json:"error,omitempty"`
}

func writeGolden(t *testing.T, cases []oracleCase, answers []pyoracle.Response) {
	t.Helper()
	pick := map[string]bool{
		"1x1-noise-1": true, "1x1-white-6": true, "2x1-gradient-18": true, "7x9-edges-28": true,
		"8x8-saturated-37": true, "9x9-photo-48": true, "15x17-noise-49": true, "16x16-flat-59": true,
		"17x15-edges-68": true, "33x1-gradient-74": true, "1x33-saturated-85": true,
		"640x480-photo-96": true, "640x480-noise-89": true, "1001x750-edges-100": true,
		"65501x1-gradient-911": true,
	}
	var golden []goldenCase
	for i, c := range cases {
		if !pick[c.name()] && c.class != "boundary" {
			continue
		}
		g := goldenCase{genCase: c.genCase}
		if answers[i].Result == "ok" {
			g.SHA16, g.Size = answers[i].SHA256[:16], answers[i].Size
		} else {
			g.Error = answers[i].Type + ": " + answers[i].Message
		}
		golden = append(golden, g)
	}
	var buf bytes.Buffer
	buf.WriteString("{\n  \"source\": \"Pillow 12.2.0 + libjpeg-turbo 3.1.4.1 in mobile-parity-api:7bf58e3d, Image.frombytes(RGB).save(JPEG, quality=88, optimize=True)\",\n  \"cases\": [\n")
	for i, g := range golden {
		line, err := json.Marshal(g)
		if err != nil {
			t.Fatal(err)
		}
		buf.WriteString("    ")
		buf.Write(line)
		if i < len(golden)-1 {
			buf.WriteString(",")
		}
		buf.WriteString("\n")
	}
	buf.WriteString("  ]\n}\n")
	if regexp.MustCompile(`\d{9,}`).Match(buf.Bytes()) {
		t.Fatal("golden would hold a run of 9 or more digits")
	}
	if err := os.WriteFile("testdata/golden.json", buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote testdata/golden.json with %d cases", len(golden))
}

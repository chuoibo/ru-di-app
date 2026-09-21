//go:build oracle

package pngenc

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"testing"
	"time"

	"mobile/services/core/internal/media/sanitize/internal/pyoracle"
)

// oracleCorpus is the differential corpus against Pillow's
// Image.frombytes("RGBA").save(PNG) in the parity image.
func oracleCorpus() []corpusCase {
	var cases []corpusCase
	add := func(class string, contents []string, sizes [][2]int, seeds ...uint64) {
		for _, content := range contents {
			for _, size := range sizes {
				for _, seed := range seeds {
					cases = append(cases, corpusCase{Class: class, Content: content, W: size[0], H: size[1], Seed: seed})
				}
			}
		}
	}
	all := []string{"noise", "gradient", "flat", "rows", "halfalpha", "text", "tiles", "photo"}
	add("tiny", all, [][2]int{{1, 1}, {2, 3}, {17, 11}}, 1, 2)
	add("line", all, [][2]int{{255, 1}, {1, 300}}, 3)
	add("wide", []string{"noise", "gradient", "rows", "text", "photo"}, [][2]int{{16385, 2}, {20000, 5}}, 4)
	add("medium", all, [][2]int{{640, 480}, {333, 257}}, 5, 6)
	add("large", all, [][2]int{{2000, 1500}}, 7)
	if os.Getenv("PNGENC_ORACLE_SKIP_PHONE") == "" {
		add("phone", []string{"photo", "text"}, [][2]int{{4032, 3024}}, 8)
	}
	return cases
}

func sha(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func TestOraclePillowPNG(t *testing.T) {
	cases := oracleCorpus()
	type result struct {
		out     []byte
		elapsed time.Duration
	}
	results := make([]result, len(cases))
	requests := make([]pyoracle.Request, len(cases))
	for i, c := range cases {
		pix := c.pixels()
		started := time.Now()
		out := EncodeRGBA(c.W, c.H, pix)
		results[i] = result{out, time.Since(started)}
		requests[i] = pyoracle.Request{ID: fmt.Sprint(i), Op: "encode_png", W: c.W, H: c.H, Input: pix}
	}
	started := time.Now()
	responses := pyoracle.Run(t, requests)
	t.Logf("oracle answered %d cases in %s", len(cases), time.Since(started).Round(time.Millisecond))

	type tally struct{ cases, exact int }
	byClass := map[string]*tally{}
	var mismatched []int
	for i, c := range cases {
		response := responses[i]
		if response.Result != "ok" {
			t.Fatalf("%+v: oracle %s %s: %s", c, response.Result, response.Type, response.Message)
		}
		entry := byClass[c.Class]
		if entry == nil {
			entry = &tally{}
			byClass[c.Class] = entry
		}
		entry.cases++
		if sha(results[i].out) == response.SHA256 && len(results[i].out) == response.Size {
			entry.exact++
		} else {
			mismatched = append(mismatched, i)
		}
		if c.Class == "phone" || c.Class == "large" {
			t.Logf("%s %s %dx%d: go encode %s, %d bytes", c.Class, c.Content, c.W, c.H,
				results[i].elapsed.Round(time.Millisecond), len(results[i].out))
		}
	}
	classes := make([]string, 0, len(byClass))
	for class := range byClass {
		classes = append(classes, class)
	}
	sort.Strings(classes)
	for _, class := range classes {
		t.Logf("class %-6s cases %3d byte-identical %3d", class, byClass[class].cases, byClass[class].exact)
	}

	if len(mismatched) > 0 {
		var again []pyoracle.Request
		for _, i := range mismatched[:min(len(mismatched), 5)] {
			request := requests[i]
			request.Want = []string{"output"}
			again = append(again, request)
		}
		for k, response := range pyoracle.Run(t, again) {
			i := mismatched[k]
			t.Errorf("%+v: %s", cases[i], diagnose(results[i].out, response.Output))
		}
		t.Fatalf("%d of %d cases differ from Pillow", len(mismatched), len(cases))
	}

	if os.Getenv("PNGENC_WRITE_GOLDEN") != "" {
		writeGolden(t, cases, responses)
	}
}

type pngChunk struct {
	kind string
	data []byte
}

func chunks(png []byte) []pngChunk {
	var out []pngChunk
	for pos := 8; pos+12 <= len(png); {
		n := int(binary.BigEndian.Uint32(png[pos:]))
		if pos+12+n > len(png) {
			break
		}
		out = append(out, pngChunk{string(png[pos+4 : pos+8]), png[pos+8 : pos+8+n]})
		pos += 12 + n
	}
	return out
}

// diagnose names the first difference between Go's and Pillow's PNG.
func diagnose(got, want []byte) string {
	gc, wc := chunks(got), chunks(want)
	layout := func(cs []pngChunk) string {
		var b bytes.Buffer
		for _, c := range cs {
			fmt.Fprintf(&b, "%s:%d ", c.kind, len(c.data))
		}
		return b.String()
	}
	var gz, wz []byte
	for _, c := range gc {
		if c.kind == "IDAT" {
			gz = append(gz, c.data...)
		}
	}
	for _, c := range wc {
		if c.kind == "IDAT" {
			wz = append(wz, c.data...)
		}
	}
	first := -1
	for i := 0; i < min(len(gz), len(wz)); i++ {
		if gz[i] != wz[i] {
			first = i
			break
		}
	}
	if first < 0 && len(gz) != len(wz) {
		first = min(len(gz), len(wz))
	}
	return fmt.Sprintf("sizes go %d pillow %d; zlib stream go %d pillow %d, first differing zlib byte %d; chunks go [%s] pillow [%s]",
		len(got), len(want), len(gz), len(wz), first, trim(layout(gc)), trim(layout(wc)))
}

func trim(s string) string {
	if len(s) > 300 {
		return s[:300] + "..."
	}
	return s
}

var allDigits = regexp.MustCompile(`^[0-9]+$`)

// writeGolden stores Pillow's answers for the cheap part of the corpus.
func writeGolden(t *testing.T, cases []corpusCase, responses []pyoracle.Response) {
	var golden []goldenCase
	for i, c := range cases {
		// one wide case keeps the bufsize = 4 * width rule (IDAT split) under the plain test
		wideNoise := c.Class == "wide" && c.Content == "noise" && c.W == 16385
		if !wideNoise && (c.W*c.H > 700*500 || len(golden) >= 29) {
			continue
		}
		if c.Class == "tiny" && c.Seed == 2 {
			continue
		}
		prefix := responses[i].SHA256[:16]
		if allDigits.MatchString(prefix) {
			t.Fatalf("%+v: digest prefix is all digits; pick another seed", c)
		}
		golden = append(golden, goldenCase{corpusCase: c, SHA256Prefix: prefix, Size: responses[i].Size})
	}
	data, err := json.MarshalIndent(golden, "", " ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("testdata", "golden.json")
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote %d golden cases to %s", len(golden), path)
}

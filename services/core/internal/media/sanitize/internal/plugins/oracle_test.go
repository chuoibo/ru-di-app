//go:build oracle

package plugins

// Differential test against Pillow 12.2.0 in the parity image, one plugin
// at a time, through the "plugin" op of scripts/render_media_sanitize_oracle.py.
// Every input is built at test time from seeds: Pillow saves some bases, Go
// builds the rest, and each base is truncated, byte-mutated and padded.
//
//	go test -tags oracle -run TestPluginsAgainstPillow -v ./internal/media/sanitize/internal/plugins/
//
// MEDIA_ORACLE_IMAGE overrides the image; PLUGINS_ORACLE_ONLY limits the run
// to a comma-separated list of formats.

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

type oracleRequest struct {
	ID     string         `json:"id"`
	Op     string         `json:"op"`
	Format string         `json:"format,omitempty"`
	Input  []byte         `json:"b64"`
	Gen    map[string]any `json:"gen,omitempty"`
}

type oracleAnswer struct {
	ID           string          `json:"id"`
	Result       string          `json:"result"`
	Accept       json.RawMessage `json:"accept"`
	AcceptRaised string          `json:"accept_raised"`
	Open         string          `json:"open"`
	Type         string          `json:"type"`
	Message      string          `json:"message"`
	Mode         string          `json:"mode"`
	Width        float64         `json:"width"`
	Height       float64         `json:"height"`
	Bomb         string          `json:"bomb"`
	Load         string          `json:"load"`
	LoadedMode   string          `json:"loaded_mode"`
	LoadedW      float64         `json:"loaded_width"`
	LoadedH      float64         `json:"loaded_height"`
	PixelsSHA    string          `json:"pixels_sha256"`
	Input        []byte          `json:"b64_input"`
}

func oracleImage() string {
	if image := os.Getenv("MEDIA_ORACLE_IMAGE"); image != "" {
		return image
	}
	return "mobile-parity-api:7bf58e3d"
}

func oracleScript(t *testing.T) []byte {
	dir, _ := os.Getwd()
	for {
		data, err := os.ReadFile(filepath.Join(dir, "scripts/render_media_sanitize_oracle.py"))
		if err == nil {
			return data
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("oracle script not found")
		}
		dir = parent
	}
}

func askPillow(t *testing.T, requests []oracleRequest, tolerateCrash bool) []oracleAnswer {
	t.Helper()
	var stdin bytes.Buffer
	enc := json.NewEncoder(&stdin)
	for i := range requests {
		requests[i].ID = fmt.Sprint(i)
		if err := enc.Encode(requests[i]); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("docker", "run", "--rm", "-i", "--network", "none", "--entrypoint", "python",
		oracleImage(), "-c", string(oracleScript(t)))
	cmd.Stdin = &stdin
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	var answers []oracleAnswer
	scanner := bufio.NewScanner(out)
	scanner.Buffer(make([]byte, 1<<20), 1<<30)
	for scanner.Scan() {
		var a oracleAnswer
		if err := json.Unmarshal(scanner.Bytes(), &a); err != nil {
			t.Fatalf("answer %d: %v", len(answers), err)
		}
		answers = append(answers, a)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("oracle: %v\n%s", err, stderr.String())
	}
	if len(answers) != len(requests) {
		t.Fatalf("oracle answered %d of %d\n%s", len(answers), len(requests), stderr.String())
	}
	for i, a := range answers {
		if a.ID != requests[i].ID {
			t.Fatalf("answer %d out of order", i)
		}
		if a.Type == "OracleCrash" && !tolerateCrash {
			t.Fatalf("oracle crashed on %s: %s", requests[i].Format, a.Message)
		}
	}
	return answers
}

func pyOutcome(a oracleAnswer) outcome {
	var o outcome
	switch string(a.Accept) {
	case "null", "":
		o.accept = "none"
	case "true":
		o.accept = "true"
	default:
		o.accept = "false"
	}
	o.detail = a.Type + ": " + a.Message
	switch a.Open {
	case "next":
		o.open = "next"
		return o
	case "raise":
		o.open = "raise"
		if strings.HasPrefix(a.Type, "DecompressionBomb") {
			o.open = "bomb"
		}
		return o
	}
	o.open, o.w, o.h, o.mode = "ok", a.Width, a.Height, a.Mode
	if a.Bomb != "" {
		o.open = "bomb"
		return o
	}
	if a.Load == "ok" {
		o.load, o.loadedMode, o.lw, o.lh, o.sha = "ok", a.LoadedMode, a.LoadedW, a.LoadedH, a.PixelsSHA
	} else {
		o.load = "raise"
	}
	return o
}

// pillowBases lists inputs Pillow saves itself.
func pillowBases() []map[string]any {
	var gens []map[string]any
	add := func(format, mode string, save map[string]any) {
		g := map[string]any{"kind": "pillow", "format": format, "mode": mode, "w": 11, "h": 7, "seed": len(gens) + 1}
		if mode == "P" {
			g["colors"] = 12
		}
		if save != nil {
			g["save"] = save
		}
		gens = append(gens, g)
	}
	for _, m := range []string{"L", "LA", "P", "RGB", "RGBA"} {
		add("TGA", m, nil)
		add("TGA", m, map[string]any{"rle": true})
		add("TGA", m, map[string]any{"orientation": 1})
	}
	for _, m := range []string{"L", "RGB", "RGBA"} {
		add("SGI", m, nil)
		add("SGI", m, map[string]any{"bpc": 2})
	}
	for _, m := range []string{"1", "L", "P", "RGB"} {
		add("PCX", m, nil)
	}
	for _, m := range []string{"1", "L", "LA", "P", "PA", "RGB", "RGBA", "RGBX", "CMYK", "YCbCr", "I", "F", "I;16", "I;16B", "I;16L"} {
		add("IM", m, nil)
	}
	add("SPIDER", "F", nil)
	add("SPIDER", "L", nil)
	add("MSP", "1", nil)
	add("XBM", "1", nil)
	for _, m := range []string{"RGB", "RGBA", "L", "LA"} {
		add("DDS", m, nil)
	}
	for _, pf := range []string{"DXT1", "DXT3", "DXT5", "BC5"} {
		add("DDS", "RGBA", map[string]any{"pixel_format": pf})
	}
	add("BLP", "P", nil)
	add("BLP", "P", map[string]any{"blp_version": "BLP1"})
	add("BLP", "RGBA", nil)
	add("ICO", "RGBA", nil)
	add("ICO", "RGBA", map[string]any{"bitmap_format": "bmp"})
	add("ICO", "RGB", map[string]any{"bitmap_format": "bmp"})
	add("ICNS", "RGBA", nil)
	add("JPEG2000", "RGB", nil)
	add("JPEG2000", "L", map[string]any{"no_jp2": true})
	add("JPEG2000", "RGBA", nil)
	add("AVIF", "RGB", nil)
	for _, m := range []string{"1", "L", "LA", "P", "RGB", "RGBA", "CMYK", "I;16", "I", "F", "YCbCr"} {
		add("TIFF", m, nil)
	}
	for _, c := range []string{"tiff_lzw", "packbits", "tiff_deflate", "jpeg"} {
		add("TIFF", "RGB", map[string]any{"compression": c})
	}
	for _, m := range []string{"RGB", "L", "CMYK"} {
		add("EPS", m, nil)
	}
	add("QOI", "RGB", nil)
	add("QOI", "RGBA", nil)
	return gens
}

func TestPluginsAgainstPillow(t *testing.T) {
	plugins := ByName()
	only := map[string]bool{}
	if list := os.Getenv("PLUGINS_ORACLE_ONLY"); list != "" {
		for _, name := range strings.Split(list, ",") {
			only[name] = true
		}
	}
	r := rand.New(rand.NewSource(44))

	gens := pillowBases()
	var genRequests []oracleRequest
	for _, g := range gens {
		genRequests = append(genRequests, oracleRequest{Op: "gen", Gen: g})
	}
	var bases []testCase
	skippedGen := 0
	for i, a := range askPillow(t, genRequests, true) {
		if a.Result != "ok" || len(a.Input) == 0 {
			skippedGen++
			t.Logf("pillow cannot build %v: %s %s", gens[i], a.Type, a.Message)
			continue
		}
		format := gens[i]["format"].(string)
		bases = append(bases, testCase{format, fmt.Sprintf("pillow %s %s %v", format, gens[i]["mode"], gens[i]["save"]), a.Input})
	}
	for _, build := range []func(*rand.Rand) []testCase{buildTGA, buildSUN, buildSGI, buildQOI, buildPCXPlanar, buildText, buildIPTC, buildMisc, buildTIFF} {
		bases = append(bases, build(r)...)
	}
	// DCX wraps the PCX bases.
	for _, b := range bases {
		if b.format == "PCX" && strings.HasPrefix(b.label, "pillow") {
			bases = append(bases, testCase{"DCX", "dcx " + b.label, cat(le32(0x3ADE68B1), le32(12), le32(0), b.data)})
		}
	}

	var cases []testCase
	for _, b := range bases {
		cases = append(cases, variants(r, b)...)
	}
	names := make([]string, 0, len(plugins))
	for name := range plugins {
		names = append(names, name)
	}
	sort.Strings(names)
	for g := 0; g < 40; g++ {
		junk := noise(r, r.Intn(300))
		for _, name := range names {
			cases = append(cases, testCase{name, fmt.Sprintf("garbage%d", g), junk})
		}
	}

	type pending struct {
		c   testCase
		go_ outcome
	}
	var work []pending
	var requests []oracleRequest
	skipped := 0
	for _, c := range cases {
		if len(only) > 0 && !only[c.format] {
			continue
		}
		p := plugins[c.format]
		got := goOutcome(p, c.data)
		if strings.Contains(got.detail, "does not terminate") || got.load == "skipped" {
			skipped++
			continue
		}
		work = append(work, pending{c, got})
		requests = append(requests, oracleRequest{Op: "plugin", Format: c.format, Input: append([]byte{}, c.data...)})
	}
	answers := askPillow(t, requests, false)

	type tally struct{ cases, classSame, pillowLoads, pixelSame, unsupported, unsupportedFails, mismatches int }
	stats := map[string]*tally{}
	shown := map[string]int{}
	for i, w := range work {
		want := pyOutcome(answers[i])
		got := w.go_
		s := stats[w.c.format]
		if s == nil {
			s = &tally{}
			stats[w.c.format] = s
		}
		s.cases++
		if want.load == "ok" {
			s.pillowLoads++
		}
		unsupportedLoad := want.load == "ok" && (got.open == "unsupported" || got.load == "unsupported")
		if unsupportedLoad {
			s.unsupported++
		}
		same := got.accept == want.accept && got.open == want.open
		if same && want.open == "ok" && (want.w != math.Trunc(want.w) || want.h != math.Trunc(want.h)) {
			// IM sizes may be floats: open answers a size that makes the
			// sanitizer's checks come out as Python's, and load fails.
			same = got.load == "raise" && want.load != "ok"
			if same {
				s.classSame++
				continue
			}
		}
		if same && want.open == "ok" {
			same = got.w == want.w && got.h == want.h && got.mode == want.mode
			if same {
				switch {
				case want.load == "ok" && got.load == "ok":
					same = got.loadedMode == want.loadedMode && got.lw == want.lw && got.lh == want.lh
					if same && got.sha == want.sha {
						s.pixelSame++
					} else {
						same = false
					}
				case got.load == "unsupported":
					same = want.load != "ok" && false
				default:
					same = got.load == want.load
				}
			}
		}
		if got.open == "unsupported" && want.open == "ok" {
			same = false
		}
		if same {
			s.classSame++
			continue
		}
		if unsupportedLoad {
			continue
		}
		if got.open == "unsupported" || got.load == "unsupported" {
			s.unsupportedFails++
			continue
		}
		s.mismatches++
		if shown[w.c.format] < 6 {
			shown[w.c.format]++
			t.Logf("%s %s len=%d\n  go:     %v\n  pillow: %v", w.c.format, w.c.label, len(w.c.data), got, want)
		}
	}
	total := tally{}
	for _, name := range names {
		s := stats[name]
		if s == nil {
			continue
		}
		t.Logf("%-8s cases=%5d class-identical=%5d pillow-loads=%4d pixel-identical=%4d unsupported-but-pillow-loads=%4d unsupported-but-pillow-fails=%4d mismatches=%4d",
			name, s.cases, s.classSame, s.pillowLoads, s.pixelSame, s.unsupported, s.unsupportedFails, s.mismatches)
		total.cases += s.cases
		total.classSame += s.classSame
		total.pillowLoads += s.pillowLoads
		total.pixelSame += s.pixelSame
		total.unsupported += s.unsupported
		total.unsupportedFails += s.unsupportedFails
		total.mismatches += s.mismatches
	}
	t.Logf("TOTAL    cases=%5d class-identical=%5d pillow-loads=%4d pixel-identical=%4d unsupported-but-pillow-loads=%4d unsupported-but-pillow-fails=%4d mismatches=%4d skipped=%d pillow-gen-skipped=%d",
		total.cases, total.classSame, total.pillowLoads, total.pixelSame, total.unsupported, total.unsupportedFails, total.mismatches, skipped, skippedGen)
	if total.mismatches > 0 {
		t.Errorf("%d cases differ from Pillow", total.mismatches)
	}
	_ = math.MaxInt
}

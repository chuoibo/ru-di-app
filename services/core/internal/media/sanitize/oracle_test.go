//go:build oracle

package sanitize

// TestOracle runs Sanitize against app.media.images.sanitize_image in the
// pinned parity API image on a corpus built from seeds at test time, and
// compares output bytes (sha256), content type, width, height, size and
// refusal code/detail. Run from services/core:
//
//	go test -tags oracle -run TestOracle -v -timeout 60m ./internal/media/sanitize/
//
// Environment:
//
//	MEDIA_ORACLE_IMAGE            image (default mobile-parity-api:7bf58e3d)
//	MEDIA_SANITIZE_FUZZ           extra random cases (default 200)
//	MEDIA_SANITIZE_SEED           seed of the random cases (default 1)
//	MEDIA_SANITIZE_UPDATE_GOLDEN  1 rewrites testdata/golden.json from
//	                              Python's answers for the golden subset

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"mobile/services/core/internal/media/sanitize/internal/corpus"
	"mobile/services/core/internal/media/sanitize/internal/pyoracle"
)

type oracleCase struct {
	Class  string
	Name   string
	Spec   corpus.Spec
	Gen    map[string]any // Pillow-built input (Spec unused)
	Golden bool
}

func spec(kind string, seed int64, w, h int, params map[string]any) corpus.Spec {
	return corpus.Spec{Kind: kind, Seed: seed, W: w, H: h, Params: params}
}

func pillow(format, mode string, seed int64, w, h int, save map[string]any, extra map[string]any) map[string]any {
	gen := map[string]any{"kind": "pillow", "format": format, "mode": mode, "seed": seed, "w": w, "h": h}
	if save != nil {
		gen["save"] = save
	}
	for k, v := range extra {
		gen[k] = v
	}
	return gen
}

// fixedCases is the reviewed corpus: every class the report counts.
func fixedCases() []oracleCase {
	var cases []oracleCase
	add := func(class, name string, s corpus.Spec, golden bool) {
		cases = append(cases, oracleCase{Class: class, Name: name, Spec: s, Golden: golden})
	}
	addGen := func(class, name string, gen map[string]any) {
		cases = append(cases, oracleCase{Class: class, Name: name, Gen: gen})
	}

	// PNG RGBA -> PNG output.
	for i, size := range [][2]int{{1, 1}, {2, 3}, {17, 11}, {255, 1}, {1, 300}, {640, 480}, {1500, 1000}} {
		for j, pattern := range []string{"noise", "smooth", "flat", "photo", "edges"} {
			if size[0]*size[1] > 400000 && pattern != "photo" && pattern != "noise" {
				continue
			}
			add("png-rgba->png", fmt.Sprintf("%dx%d-%s", size[0], size[1], pattern),
				spec("png", int64(100+i*10+j), size[0], size[1], map[string]any{"color_type": 6, "pattern": pattern}),
				size[0]*size[1] <= 1000 && j < 2)
		}
	}
	// PNG RGB -> JPEG output (the encoder at odd MCU sizes).
	for i, size := range [][2]int{{1, 1}, {1, 2}, {2, 1}, {7, 9}, {8, 8}, {9, 9}, {15, 17}, {16, 16}, {17, 15}, {33, 1}, {1, 33}, {640, 480}, {1001, 750}} {
		for j, pattern := range []string{"smooth", "photo", "edges", "noise"} {
			if size[0]*size[1] > 100000 && pattern == "noise" {
				continue
			}
			add("png-rgb->jpeg", fmt.Sprintf("%dx%d-%s", size[0], size[1], pattern),
				spec("png", int64(300+i*10+j), size[0], size[1], map[string]any{"color_type": 2, "pattern": pattern}),
				size[0]*size[1] <= 300 && j == 1)
		}
	}
	// PNG modes and transparency.
	pngModes := []struct {
		name   string
		params map[string]any
	}{
		{"gray8", map[string]any{"color_type": 0}},
		{"gray1", map[string]any{"color_type": 0, "bit_depth": 1}},
		{"gray2", map[string]any{"color_type": 0, "bit_depth": 2}},
		{"gray4", map[string]any{"color_type": 0, "bit_depth": 4}},
		{"gray16", map[string]any{"color_type": 0, "bit_depth": 16}},
		{"gray8-trns", map[string]any{"color_type": 0, "trns": "gray"}},
		{"rgb16", map[string]any{"color_type": 2, "bit_depth": 16}},
		{"rgb8-trns", map[string]any{"color_type": 2, "trns": "rgb"}},
		{"la8", map[string]any{"color_type": 4}},
		{"la16", map[string]any{"color_type": 4, "bit_depth": 16}},
		{"rgba16", map[string]any{"color_type": 6, "bit_depth": 16}},
		{"p8", map[string]any{"color_type": 3}},
		{"p4-small", map[string]any{"color_type": 3, "bit_depth": 4, "colors": 5}},
		{"p8-trns", map[string]any{"color_type": 3, "trns": "palette"}},
		{"p2-trns", map[string]any{"color_type": 3, "bit_depth": 2, "trns": "palette"}},
		{"rgba-interlaced", map[string]any{"color_type": 6, "interlace": true}},
		{"rgb-interlaced", map[string]any{"color_type": 2, "interlace": true}},
		{"p1-interlaced", map[string]any{"color_type": 3, "bit_depth": 1, "interlace": true}},
		{"rgb-split-idat", map[string]any{"color_type": 2, "idat_split": 7}},
	}
	for i, m := range pngModes {
		for j, size := range [][2]int{{1, 1}, {13, 7}, {64, 33}} {
			add("png-modes", m.name+fmt.Sprintf("-%dx%d", size[0], size[1]),
				spec("png", int64(600+i*10+j), size[0], size[1], m.params), j == 1)
		}
	}
	// EXIF orientation through PNG eXIf, JPEG APP1 and XMP.
	for o := 0; o <= 9; o++ {
		add("exif-orientation", fmt.Sprintf("png-exif-%d", o),
			spec("png", int64(800+o), 13, 7, map[string]any{"color_type": 6, "exif": map[string]any{"orientation": o}}), o <= 8)
		add("exif-orientation", fmt.Sprintf("png-exif-after-idat-%d", o),
			spec("png", int64(820+o), 13, 7, map[string]any{"color_type": 2, "exif_after_idat": true,
				"exif": map[string]any{"orientation": o, "big_endian": true}}), false)
		add("exif-orientation", fmt.Sprintf("jpeg-exif-%d", o),
			spec("jpeg", int64(840+o), 37, 21, map[string]any{"pattern": "photo", "exif": map[string]any{"orientation": o}}), o <= 8)
	}
	for i, variant := range []map[string]any{
		{"orientation": 6, "type": 4},
		{"orientation": 6, "type": 1},
		{"orientation": 6, "type": 9},
		{"orientation": 6, "type": 5},
		{"orientation": 12, "type": 5, "denominator": 2},
		{"orientation": 6, "type": 5, "denominator": 0},
		{"orientation": 6, "type": 11},
		{"orientation": 6, "type": 12},
		{"orientation": 6, "type": 2},
		{"orientation": 6, "type": 7},
		{"orientation": 6, "count": 2},
		{"orientation": 3, "bad_offset": true},
		{"orientation": 8, "prefix": true},
		{"orientation": 6, "omit": true},
	} {
		add("exif-edge", fmt.Sprintf("jpeg-exif-variant-%d", i),
			spec("jpeg", int64(900+i), 20, 11, map[string]any{"exif": variant}), false)
	}
	for i, value := range []string{"6", "3", "9", "x"} {
		add("exif-xmp", "jpeg-xmp-"+value, spec("jpeg", int64(950+i), 20, 11, map[string]any{"xmp_orientation": value}), false)
		add("exif-xmp", "png-itxt-xmp-"+value, spec("png", int64(960+i), 20, 11, map[string]any{"color_type": 2,
			"text": []any{map[string]any{"type": "iTXt", "key": "XML:com.adobe.xmp", "value": "<tiff:Orientation>" + value + "</tiff:Orientation>"}}}), false)
	}
	// JPEG inputs from Go's encoder.
	for i, size := range [][2]int{{1, 1}, {9, 9}, {17, 15}, {640, 480}, {4032, 3024}} {
		add("jpeg->jpeg", fmt.Sprintf("std-%dx%d", size[0], size[1]),
			spec("jpeg", int64(1000+i), size[0], size[1], map[string]any{"pattern": "photo"}), size[0] < 100)
		if size[0] < 1000 {
			add("jpeg->jpeg", fmt.Sprintf("gray-%dx%d", size[0], size[1]),
				spec("jpeg", int64(1010+i), size[0], size[1], map[string]any{"gray": true, "pattern": "smooth"}), false)
		}
	}
	add("jpeg->jpeg", "phone-exif-6", spec("jpeg", 1020, 4032, 3024, map[string]any{"pattern": "photo",
		"exif": map[string]any{"orientation": 6}}), false)
	for _, cut := range []int{2, 3, 20, 200, 600, 900, 1100} {
		add("jpeg-truncated", fmt.Sprintf("cut-%d", cut), spec("jpeg", 1030, 64, 48, map[string]any{"truncate": cut}), cut >= 200)
		add("png-truncated", fmt.Sprintf("cut-%d", cut), spec("png", 1040, 64, 33, map[string]any{"color_type": 2,
			"pattern": "photo", "truncate": 40 + cut}), cut >= 200)
	}
	// JPEG inputs Pillow writes.
	for i, save := range []map[string]any{
		{"quality": 95},
		{"quality": 70, "subsampling": 0},
		{"quality": 80, "subsampling": 1},
		{"progressive": true},
		{"progressive": true, "subsampling": 0},
		{"optimize": true},
		{"restart_marker_rows": 1},
		{"restart_marker_blocks": 3},
	} {
		addGen("pillow-jpeg", fmt.Sprintf("rgb-%d", i), pillow("JPEG", "RGB", int64(1100+i), 61, 47, save, nil))
	}
	addGen("pillow-jpeg", "gray", pillow("JPEG", "L", 1110, 61, 47, nil, nil))
	addGen("pillow-jpeg", "cmyk", pillow("JPEG", "CMYK", 1111, 61, 47, nil, nil))
	addGen("pillow-jpeg", "exif-3-progressive", pillow("JPEG", "RGB", 1112, 61, 47, map[string]any{"progressive": true},
		map[string]any{"exif_orientation": 3}))
	addGen("pillow-jpeg", "mpo", pillow("MPO", "RGB", 1113, 61, 47, nil, nil))
	// PPM family, including the parity scenario's exact bytes.
	for i, magic := range []string{"P1", "P2", "P3", "P4", "P5", "P6"} {
		add("ppm", magic, spec("ppm", int64(1200+i), 11, 5, map[string]any{"magic": magic, "comment": i%2 == 0}), true)
		add("ppm", magic+"-maxval", spec("ppm", int64(1210+i), 11, 5, map[string]any{"magic": magic, "maxval": 1000}), false)
	}
	add("ppm", "parity-scenario", spec("garbage", 0, 0, 0, map[string]any{"len": 0,
		"magic": bytesAsInts("P3\n1 1\n255\n200 120 40\n")}), true)
	// GIF, BMP, WebP, TIFF.
	for i, transparent := range []bool{false, true} {
		add("gif", fmt.Sprintf("go-transparent-%v", transparent), spec("gif", int64(1300+i), 23, 9,
			map[string]any{"transparent": transparent}), true)
	}
	addGen("gif", "pillow-p-trns", pillow("GIF", "P", 1310, 23, 9, map[string]any{"transparency": 2}, map[string]any{"colors": 16}))
	addGen("gif", "pillow-l", pillow("GIF", "L", 1311, 23, 9, nil, nil))
	addGen("gif", "pillow-interlace", pillow("GIF", "P", 1312, 40, 30, map[string]any{"interlace": true}, nil))
	for i, bits := range []int{1, 4, 8, 24, 32} {
		add("bmp", fmt.Sprintf("go-%d", bits), spec("bmp", int64(1400+i), 13, 7, map[string]any{"bits": bits}), bits == 24)
	}
	addGen("bmp", "pillow-rgba", pillow("BMP", "RGBA", 1410, 13, 7, nil, nil))
	for i, save := range []map[string]any{
		{"quality": 80},
		{"lossless": true},
		{"quality": 90, "method": 6},
	} {
		addGen("webp", fmt.Sprintf("rgb-%d", i), pillow("WEBP", "RGB", int64(1500+i), 45, 31, save, nil))
		addGen("webp", fmt.Sprintf("rgba-%d", i), pillow("WEBP", "RGBA", int64(1510+i), 45, 31, save, nil))
	}
	addGen("webp", "exif-6", pillow("WEBP", "RGB", 1520, 45, 31, nil, map[string]any{"exif_orientation": 6}))
	addGen("tiff", "raw-rgb", pillow("TIFF", "RGB", 1600, 21, 13, nil, nil))
	addGen("tiff", "lzw-rgba", pillow("TIFF", "RGBA", 1601, 21, 13, map[string]any{"compression": "tiff_lzw"}, nil))
	addGen("other", "tga", pillow("TGA", "RGB", 1700, 21, 13, nil, nil))
	addGen("other", "qoi", pillow("QOI", "RGBA", 1701, 21, 13, nil, nil))
	addGen("other", "pcx", pillow("PCX", "RGB", 1702, 21, 13, nil, nil))
	addGen("other", "ico", pillow("ICO", "RGBA", 1703, 32, 32, nil, nil))
	addGen("other", "avif", pillow("AVIF", "RGB", 1704, 21, 13, nil, nil))

	// Refusals: size, dimensions, garbage.
	fill := 0
	add("refuse-size", "10MiB+1", spec("garbage", 0, 0, 0, map[string]any{"len": MaxUploadBytes + 1 - 8,
		"magic": []int{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}, "fill": fill}), true)
	add("refuse-size", "10MiB", spec("garbage", 0, 0, 0, map[string]any{"len": MaxUploadBytes, "fill": fill}), true)
	for _, size := range [][2]int{{10000, 5000}, {50_000_001, 1}, {7071, 7071}, {7072, 7072}, {9459, 9459}, {9460, 9460}, {13377, 13377}, {13378, 13378}, {60000, 1}, {1, 60000}} {
		add("refuse-dimensions", fmt.Sprintf("png-header-%dx%d", size[0], size[1]),
			spec("png", 0, size[0], size[1], map[string]any{"color_type": 2, "header_only": true}), true)
		add("refuse-dimensions", fmt.Sprintf("ppm-header-%dx%d", size[0], size[1]),
			spec("ppm", 0, size[0], size[1], map[string]any{"magic": "P6", "header_only": true}), false)
	}
	for _, n := range []int{0, 1, 2, 15, 16, 17, 100, 5000} {
		add("garbage", fmt.Sprintf("random-%d", n), spec("garbage", int64(n), 0, 0, map[string]any{"len": n}), n <= 17)
	}
	for i, magic := range pluginMagics() {
		add("garbage-magic", fmt.Sprintf("magic-%d", i), spec("garbage", int64(2000+i), 0, 0,
			map[string]any{"len": 64, "magic": magic}), false)
	}
	// Encoder refusal: JPEG output reaching Pillow's buffer.
	for i, size := range [][2]int{{128, 128}, {256, 256}, {300, 220}} {
		add("jpeg-encoder-buffer", fmt.Sprintf("noise-%dx%d", size[0], size[1]),
			spec("png", int64(2100+i), size[0], size[1], map[string]any{"color_type": 2, "pattern": "noise"}), i == 1)
	}
	// Buffer sizes the plain golden pins without Docker. PNG cuts an IDAT
	// chunk every max(65536, 4*w) bytes of zlib output: 100x100 noise
	// compresses to 32768..65536 bytes, 200x200 to more than 65536, and a
	// 17000-wide strip makes 4*w the chunk size.
	for i, size := range [][2]int{{100, 100}, {200, 200}, {17000, 2}} {
		add("png-idat-chunking", fmt.Sprintf("noise-%dx%d", size[0], size[1]),
			spec("png", int64(2200+i), size[0], size[1], map[string]any{"color_type": 6, "pattern": "noise"}), true)
	}
	// JPEG save raises once the output reaches max(65536, w*h, 4*w). The
	// seeds and sizes were chosen so each term decides one case; the last
	// two sit one byte below and exactly at the 65536 term.
	for _, c := range []struct {
		name string
		seed int64
		w, h int
	}{
		{"wh-term-decides-ok", 5, 400, 300},
		{"wh-term-decides-ok-tall", 1, 16, 6000},
		{"4w-term-decides-ok", 1, 32000, 1},
		{"4w-term-exceeded", 7, 20000, 2},
		{"65536-term-one-byte-below", 52, 12868, 2},
		{"65536-term-exactly-reached", 246, 12857, 2},
	} {
		add("jpeg-encoder-buffer", c.name,
			spec("png", c.seed, c.w, c.h, map[string]any{"color_type": 2, "pattern": "noise"}), true)
	}
	add("jpeg->jpeg", "noise-input-over-one-decoder-block",
		spec("jpeg", 2300, 400, 300, map[string]any{"pattern": "noise", "quality": 95}), true)
	return cases
}

func bytesAsInts(s string) []int {
	out := make([]int, len(s))
	for i := range s {
		out[i] = int(s[i])
	}
	return out
}

// pluginMagics are the accept prefixes of every registered plugin.
func pluginMagics() [][]int {
	var out [][]int
	for _, magic := range []string{
		"BM", "\x28\x00\x00\x00", "GIF89a", "\xff\xd8\xff", "P6", "\x89PNG\r\n\x1a\n",
		"\x00\x00\x00\x1cftypavif", "BLP2", "BUFR", "\x00\x00\x02\x00", "\x0a\x05", "\xb1\x68\xde\x3a",
		"DDS ", "%!PS", "SIMPLE", "\x00\x00\x00\x00\x11\xaf", "FTEX", "\x00\x00\x00\x20\x00\x00\x00\x02",
		"GRIB\x00\x00\x00\x01", "\x89HDF\r\n\x1a\n", "\xff\x4f\xff\x51", "icns", "\x00\x00\x01\x00",
		"\x00\x00\x00\x00\x00\x00\x00\x04", "\x00\x00\x01\xb3", "II*\x00", "MM\x00*", "DanM", "\x80\xe8\x00\x00",
		"8BPS", "qoif", "\x01\xda", "\x59\xa6\x6a\x95", "RIFF\x00\x00\x00\x00WEBPVP8 ", "\xd7\xcd\xc6\x9a\x00\x00",
		"#define", "/* XPM */", "P7 332",
	} {
		out = append(out, bytesAsInts(magic))
	}
	return out
}

// fuzzCases draws random specs over the Go-built kinds.
func fuzzCases(seed int64, count int) []oracleCase {
	rng := rand.New(rand.NewSource(seed))
	patterns := []string{"noise", "smooth", "flat", "photo", "edges"}
	var cases []oracleCase
	for i := 0; i < count; i++ {
		w, h := 1+rng.Intn(90), 1+rng.Intn(90)
		pattern := patterns[rng.Intn(len(patterns))]
		s := int64(rng.Int63n(1 << 40))
		var sp corpus.Spec
		switch rng.Intn(6) {
		case 0, 1:
			colorType := []int{0, 2, 3, 4, 6}[rng.Intn(5)]
			depths := map[int][]int{0: {1, 2, 4, 8, 16}, 2: {8, 16}, 3: {1, 2, 4, 8}, 4: {8, 16}, 6: {8, 16}}[colorType]
			params := map[string]any{"color_type": colorType, "bit_depth": depths[rng.Intn(len(depths))],
				"pattern": pattern, "interlace": rng.Intn(4) == 0}
			if rng.Intn(3) == 0 {
				params["trns"] = map[int]string{0: "gray", 2: "rgb", 3: "palette"}[colorType]
			}
			if rng.Intn(3) == 0 {
				params["exif"] = map[string]any{"orientation": rng.Intn(10), "big_endian": rng.Intn(2) == 0}
			}
			if rng.Intn(8) == 0 {
				params["truncate"] = 8 + rng.Intn(200)
			}
			sp = spec("png", s, w, h, params)
		case 2:
			params := map[string]any{"pattern": pattern, "gray": rng.Intn(4) == 0, "quality": 40 + rng.Intn(60)}
			if rng.Intn(2) == 0 {
				params["exif"] = map[string]any{"orientation": rng.Intn(10)}
			}
			sp = spec("jpeg", s, w, h, params)
		case 3:
			magic := []string{"P1", "P2", "P3", "P4", "P5", "P6"}[rng.Intn(6)]
			sp = spec("ppm", s, w, h, map[string]any{"magic": magic, "pattern": pattern,
				"maxval": []int{1, 15, 255, 256, 65535}[rng.Intn(5)]})
		case 4:
			sp = spec("gif", s, w, h, map[string]any{"pattern": pattern, "colors": 2 + rng.Intn(255),
				"transparent": rng.Intn(2) == 0})
		default:
			sp = spec("bmp", s, w, h, map[string]any{"pattern": pattern, "bits": []int{1, 4, 8, 24, 32}[rng.Intn(5)],
				"top_down": rng.Intn(2) == 0})
		}
		cases = append(cases, oracleCase{Class: "fuzz-" + sp.Kind, Name: fmt.Sprintf("fuzz-%d", i), Spec: sp})
	}
	return cases
}

func envInt(key string, fallback int) int {
	if raw := os.Getenv(key); raw != "" {
		value, err := strconv.Atoi(raw)
		if err == nil {
			return value
		}
	}
	return fallback
}

type tally struct{ cases, identical, unportedLoads, unportedRefuses, mismatches int }

func logTally(t *testing.T, class string, tl tally) {
	t.Logf("%-22s cases %4d  identical %4d  unported(py loads) %3d  unported(py refuses) %3d  mismatches %3d",
		class, tl.cases, tl.identical, tl.unportedLoads, tl.unportedRefuses, tl.mismatches)
}

func goOutcome(raw []byte) (outcome, Sanitized, error) {
	result, err := Sanitize(raw)
	if err == nil {
		sum := sha256.Sum256(result.Data)
		return outcome{Result: "ok", ContentType: result.ContentType, Width: result.Width, Height: result.Height,
			Size: len(result.Data), SHA256: hex.EncodeToString(sum[:])}, result, nil
	}
	var rejected *Rejected
	var encode *EncodeError
	var unsupported *UnsupportedError
	switch {
	case errors.As(err, &rejected):
		return outcome{Result: "rejected", Code: rejected.Code, Detail: rejected.Detail}, result, err
	case errors.As(err, &encode):
		return outcome{Result: "error", Code: "OSError"}, result, err
	case errors.As(err, &unsupported):
		return outcome{Result: "unsupported", Detail: unsupported.Error()}, result, err
	}
	return outcome{Result: "internal", Detail: err.Error()}, result, err
}

func pyOutcome(r pyoracle.Response) outcome {
	switch r.Result {
	case "ok":
		return outcome{Result: "ok", ContentType: r.ContentType, Width: r.Width, Height: r.Height, Size: r.Size, SHA256: r.SHA256}
	case "rejected":
		return outcome{Result: "rejected", Code: r.Code, Detail: r.Detail}
	}
	return outcome{Result: "error", Code: r.Type}
}

func TestOracle(t *testing.T) {
	cases := append(fixedCases(), fuzzCases(int64(envInt("MEDIA_SANITIZE_SEED", 1)), envInt("MEDIA_SANITIZE_FUZZ", 200))...)
	inputs := make([][]byte, len(cases))
	requests := make([]pyoracle.Request, len(cases))
	for i, c := range cases {
		requests[i] = pyoracle.Request{ID: c.Class + "/" + c.Name, Op: "sanitize"}
		if c.Gen != nil {
			requests[i].Gen = c.Gen
			requests[i].Want = []string{"input"}
			continue
		}
		raw, err := corpus.Build(c.Spec)
		if err != nil {
			t.Fatalf("%s: %v", requests[i].ID, err)
		}
		inputs[i] = raw
		requests[i].Input = raw
	}
	answers := pyoracle.Run(t, requests)

	tallies := map[string]*tally{}
	var golden []goldenCase
	var mismatched []int
	for i, c := range cases {
		answer := answers[i]
		if c.Gen != nil {
			if answer.InputBytes == nil {
				t.Errorf("%s: oracle built no input: %s %s", answer.ID, answer.Type, answer.Message)
				continue
			}
			inputs[i] = answer.InputBytes
		}
		py := pyOutcome(answer)
		got, _, _ := goOutcome(inputs[i])
		tl := tallies[c.Class]
		if tl == nil {
			tl = &tally{}
			tallies[c.Class] = tl
		}
		tl.cases++
		switch {
		case got == py:
			tl.identical++
		case got.Result == "unsupported" && py.Result == "ok":
			tl.unportedLoads++
			t.Logf("unported, Pillow loads %s: python %s %dx%d; go %s", answer.ID, py.ContentType, py.Width, py.Height, got.Detail)
		case got.Result == "unsupported":
			// Go cannot tell whether the missing codec would accept the
			// bytes; Python refused them. A reported divergence, not a
			// wrong answer the port gave.
			tl.unportedRefuses++
			t.Logf("unported, Pillow refuses %s: python %s %s; go %s", answer.ID, py.Result, py.Code, got.Detail)
		default:
			tl.mismatches++
			mismatched = append(mismatched, i)
			t.Errorf("%s:\n  python %+v\n  go     %+v", answer.ID, py, got)
		}
		if c.Golden {
			want := py
			if want.SHA256 != "" {
				want.SHA256 = shaTag(want.SHA256)
			}
			golden = append(golden, goldenCase{Name: c.Class + "/" + c.Name, Spec: c.Spec, Want: want})
		}
	}
	explainMismatches(t, cases, inputs, answers, mismatched)

	classes := make([]string, 0, len(tallies))
	for class := range tallies {
		classes = append(classes, class)
	}
	sort.Strings(classes)
	var total tally
	for _, class := range classes {
		tl := tallies[class]
		logTally(t, class, *tl)
		total.cases += tl.cases
		total.identical += tl.identical
		total.unportedLoads += tl.unportedLoads
		total.unportedRefuses += tl.unportedRefuses
		total.mismatches += tl.mismatches
	}
	logTally(t, "TOTAL", total)

	if os.Getenv("MEDIA_SANITIZE_UPDATE_GOLDEN") == "1" {
		writeGolden(t, golden)
	}
}

// explainMismatches separates decoder from encoder mismatches for cases
// where both sides produced an image, and measures how far apart they are.
func explainMismatches(t *testing.T, cases []oracleCase, inputs [][]byte, answers []pyoracle.Response, indices []int) {
	var requests []pyoracle.Request
	var goOutputs [][]byte
	var goPixels [][]byte
	var kept []int
	for _, i := range indices {
		if answers[i].Result != "ok" {
			continue
		}
		result, err := Sanitize(inputs[i])
		if err != nil {
			continue
		}
		_, _, _, pix, _ := decode(inputs[i])
		requests = append(requests, pyoracle.Request{ID: answers[i].ID, Op: "sanitize", Input: inputs[i], Want: []string{"output", "pixels"}})
		goOutputs = append(goOutputs, result.Data)
		goPixels = append(goPixels, pix)
		kept = append(kept, i)
	}
	if len(requests) == 0 {
		return
	}
	detailed := pyoracle.Run(t, requests)
	var metrics []pyoracle.Request
	for k, answer := range detailed {
		samePixels := string(answer.Pixels) == string(goPixels[k])
		t.Logf("%s: pixels before encode identical=%v (python %d bytes, go %d)", answer.ID, samePixels, len(answer.Pixels), len(goPixels[k]))
		metrics = append(metrics, pyoracle.Request{ID: answer.ID, Op: "metrics", Input: answer.Output, Other: goOutputs[k]})
	}
	for _, m := range pyoracle.Run(t, metrics) {
		t.Logf("%s: SSIM luma %.5f, MAE per channel %v", m.ID, m.SSIM, m.MAE)
	}
	_ = kept
}

var longDigits = regexp.MustCompile(`\d{9,}`)

func writeGolden(t *testing.T, cases []goldenCase) {
	t.Helper()
	file := goldenFile{
		Note:  "Seeds and parameters -> what sanitize_image answered in mobile-parity-api:7bf58e3d. sha256 is truncated to 16 hex characters. Regenerate with MEDIA_SANITIZE_UPDATE_GOLDEN=1 go test -tags oracle -run TestOracle.",
		Cases: cases,
	}
	data, err := json.MarshalIndent(file, "", " ")
	if err != nil {
		t.Fatal(err)
	}
	if match := longDigits.Find(data); match != nil {
		t.Fatalf("golden would carry a %d-digit run; the repo guard rejects it", len(match))
	}
	if strings.Contains(string(data), "b64") {
		t.Fatal("golden must not carry bytes")
	}
	if err := os.WriteFile("testdata/golden.json", append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote testdata/golden.json with %d cases", len(cases))
}

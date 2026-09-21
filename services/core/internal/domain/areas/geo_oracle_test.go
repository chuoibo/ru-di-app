package areas

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
	"unicode"
)

// testdata/python_geo*.json is rendered by scripts/render_places_geo_goldens.py
// from app/places/areas.py in the pinned API image. See that script for the
// row encoding.

type geoHost struct {
	Machine string `json:"machine"`
	Libc    string `json:"libc"`
	Python  string `json:"python"`
	FMAAVX2 bool   `json:"fma_avx2"`
}

type geoFuzzInfo struct {
	Shard  int `json:"shard"`
	Shards int `json:"shards"`
	Total  int `json:"total"`
}

type geoAddressCase struct {
	Address string `json:"address"`
	Result  string `json:"result"`
}

type geoEdges struct {
	Libm               []string         `json:"libm"`
	Haversine          []string         `json:"haversine"`
	Nearest            []string         `json:"nearest"`
	NearestExactRadius []string         `json:"nearest_exact_radius"`
	NearestTies        []string         `json:"nearest_ties"`
	Address            []geoAddressCase `json:"address"`
}

type geoFuzzCase struct {
	Haversine string         `json:"haversine"`
	Nearest   string         `json:"nearest"`
	Address   geoAddressCase `json:"address"`
	Index     int            `json:"index"`
}

type geoConstants struct {
	MaxAreaRadiusKm string   `json:"max_area_radius_km"`
	EarthRadiusKm   string   `json:"earth_radius_km"`
	RadiansOfOne    string   `json:"radians_of_one"`
	Pattern         string   `json:"hcm_district_pattern"`
	IgnoreCase      bool     `json:"hcm_district_ignorecase"`
	NamedHCM        []string `json:"named_hcm"`
	SummaryKeys     []string `json:"summary_keys"`
}

type geoUnicode struct {
	UnidataVersion string   `json:"unidata_version"`
	Digit          []string `json:"digit"`
	Space          []string `json:"space"`
	Word           []string `json:"word"`
	AtomQ          []string `json:"atom_q"`
	AtomU          []string `json:"atom_u"`
	AtomN          []string `json:"atom_n"`
	AtomClass      []string `json:"atom_class"`
	Lower          []string `json:"lower"`
}

type geoFile struct {
	Mode      string          `json:"mode"`
	Host      geoHost         `json:"host"`
	Fuzz      *geoFuzzInfo    `json:"fuzz"`
	Constants *geoConstants   `json:"constants"`
	Unicode   *geoUnicode     `json:"unicode"`
	Cases     json.RawMessage `json:"cases"`
}

type geoCorpus struct {
	constants geoConstants
	unicode   geoUnicode
	edges     geoEdges
	fuzz      []geoFuzzCase
}

func loadGeo(t *testing.T) geoCorpus {
	t.Helper()
	paths, err := filepath.Glob("testdata/python_geo*.json")
	if err != nil || len(paths) == 0 {
		t.Fatalf("no geo oracle files: %v", err)
	}
	sort.Strings(paths)
	var corpus geoCorpus
	edgeFiles, shardsSeen, fuzzTotal := 0, 0, -1
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var file geoFile
		if err := json.Unmarshal(raw, &file); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if !file.Host.FMAAVX2 || file.Host.Machine != "x86_64" || file.Host.Libc != "glibc 2.41" {
			t.Fatalf("%s was rendered on %+v; the port reproduces glibc 2.41's FMA variants on x86-64", path, file.Host)
		}
		if file.Fuzz == nil {
			edgeFiles++
			if file.Constants == nil || file.Unicode == nil {
				t.Fatalf("%s: edge file without constants or unicode tables", path)
			}
			corpus.constants, corpus.unicode = *file.Constants, *file.Unicode
			if err := json.Unmarshal(file.Cases, &corpus.edges); err != nil {
				t.Fatalf("%s: %v", path, err)
			}
			continue
		}
		var cases []geoFuzzCase
		if err := json.Unmarshal(file.Cases, &cases); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		shardsSeen++
		if fuzzTotal >= 0 && fuzzTotal != file.Fuzz.Total {
			t.Fatalf("%s: fuzz total %d, other shards say %d", path, file.Fuzz.Total, fuzzTotal)
		}
		fuzzTotal = file.Fuzz.Total
		if shardsSeen > file.Fuzz.Shards {
			t.Fatalf("%s: more shard files than the %d declared", path, file.Fuzz.Shards)
		}
		corpus.fuzz = append(corpus.fuzz, cases...)
	}
	if edgeFiles != 1 {
		t.Fatalf("want exactly one geo edge file, found %d", edgeFiles)
	}
	if fuzzTotal < 2000 || len(corpus.fuzz) != fuzzTotal {
		t.Fatalf("fuzz has %d cases of a declared %d; need every shard and at least 2000", len(corpus.fuzz), fuzzTotal)
	}
	for i, c := range corpus.fuzz {
		if c.Index != i {
			t.Fatalf("fuzz case %d carries index %d: shards are out of order or missing", i, c.Index)
		}
	}
	return corpus
}

// pyFloat decodes the golden float encoding: repr with underscores between
// digit groups.
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

func raiseOf(err error) string {
	var me *MathError
	if !errors.As(err, &me) {
		return "raise:<non-math error>:" + err.Error()
	}
	return "raise:" + me.Type + ":" + me.Message
}

// floatOutcome compares a float result or raise against the golden outcome.
func floatOutcome(t *testing.T, want string, got float64, err error) (bitsOK, roundOK, thresholdOK bool) {
	t.Helper()
	if err != nil || strings.HasPrefix(want, "raise:") {
		same := err != nil && raiseOf(err) == want
		return same, same, same
	}
	if !strings.HasPrefix(want, "ok:") {
		t.Fatalf("bad outcome %q", want)
	}
	w := pyFloat(t, strings.TrimPrefix(want, "ok:"))
	return sameFloat(got, w), sameFloat(round2(got), round2(w)), (got < MaxAreaRadiusKm) == (w < MaxAreaRadiusKm)
}

// round2 is Python's round(x, 2), for counting disagreements at the level
// the wire shows.
func round2(x float64) float64 {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return x
	}
	v, _ := strconv.ParseFloat(strconv.FormatFloat(x, 'f', 2, 64), 64)
	return v
}

func TestGeoConstantsMatchPython(t *testing.T) {
	c := loadGeo(t).constants
	if !sameFloat(MaxAreaRadiusKm, pyFloat(t, c.MaxAreaRadiusKm)) || !sameFloat(earthRadiusKm, pyFloat(t, c.EarthRadiusKm)) {
		t.Fatalf("radius constants: Python %s and %s", c.MaxAreaRadiusKm, c.EarthRadiusKm)
	}
	if !sameFloat(radians(1.0), pyFloat(t, c.RadiansOfOne)) {
		t.Fatalf("radians(1.0) = %v, Python %s", radians(1.0), c.RadiansOfOne)
	}
	if c.Pattern != `Qu[âậ]n\s+(\d+)\b` || !c.IgnoreCase {
		t.Fatalf("_HCM_DISTRICT is now %q (ignorecase %v); searchHCMDistrict is hand-written for the old pattern", c.Pattern, c.IgnoreCase)
	}
	var named []string
	for _, n := range namedHCM {
		named = append(named, n.needle+"|"+n.id)
	}
	if strings.Join(named, ",") != strings.Join(c.NamedHCM, ",") {
		t.Fatalf("_NAMED_HCM: Go %v, Python %v", named, c.NamedHCM)
	}
	if strings.Join(c.SummaryKeys, ",") != "id,label,lat,lng" {
		t.Fatalf("area_summary keys are now %v", c.SummaryKeys)
	}
}

type cpRanges [][2]rune

func parseRanges(t *testing.T, spans []string) cpRanges {
	t.Helper()
	out := make(cpRanges, 0, len(spans))
	for _, span := range spans {
		lo, hi, ok := strings.Cut(span, ":")
		a, errA := strconv.ParseInt(lo, 16, 32)
		b, errB := strconv.ParseInt(hi, 16, 32)
		if !ok || errA != nil || errB != nil {
			t.Fatalf("bad range %q", span)
		}
		out = append(out, [2]rune{rune(a), rune(b)})
	}
	return out
}

func (r cpRanges) has(c rune) bool {
	i := sort.Search(len(r), func(i int) bool { return r[i][1] >= c })
	return i < len(r) && r[i][0] <= c
}

// TestRegexClassesMatchPythonForEveryCodePoint compares each hand-written
// predicate with the set Python's re accepts, over all of Unicode.
func TestRegexClassesMatchPythonForEveryCodePoint(t *testing.T) {
	u := loadGeo(t).unicode
	if u.UnidataVersion != unicode.Version {
		t.Fatalf("Python's Unicode is %s, Go's %s: the class tables below may diverge", u.UnidataVersion, unicode.Version)
	}
	checks := []struct {
		name string
		want cpRanges
		got  func(rune) bool
	}{
		{`\d`, parseRanges(t, u.Digit), isPyDecimal},
		{`\s`, parseRanges(t, u.Space), isPySpace},
		{`\w`, parseRanges(t, u.Word), isPyWord},
		{"(?i)Q", parseRanges(t, u.AtomQ), func(r rune) bool { return caseFoldsTo(r, 'q') }},
		{"(?i)u", parseRanges(t, u.AtomU), func(r rune) bool { return caseFoldsTo(r, 'u') }},
		{"(?i)n", parseRanges(t, u.AtomN), func(r rune) bool { return caseFoldsTo(r, 'n') }},
		{"(?i)[âậ]", parseRanges(t, u.AtomClass), func(r rune) bool { return caseFoldsTo(r, 'â') || caseFoldsTo(r, 'ậ') }},
	}
	for _, check := range checks {
		bad := 0
		for c := rune(0); c <= unicode.MaxRune; c++ {
			if c >= 0xd800 && c <= 0xdfff {
				continue
			}
			if check.got(c) != check.want.has(c) {
				if bad < 5 {
					t.Errorf("%s at U+%04X: Go %v, Python %v", check.name, c, check.got(c), check.want.has(c))
				}
				bad++
			}
		}
		if bad > 0 {
			t.Errorf("%s disagrees with Python at %d code points", check.name, bad)
		}
	}

	lower := map[rune]string{}
	for _, entry := range u.Lower {
		from, to, ok := strings.Cut(entry, ">")
		code, err := strconv.ParseInt(from, 16, 32)
		if !ok || err != nil {
			t.Fatalf("bad lower entry %q", entry)
		}
		var mapped []rune
		for _, part := range strings.Split(to, ",") {
			v, err := strconv.ParseInt(part, 16, 32)
			if err != nil {
				t.Fatalf("bad lower entry %q", entry)
			}
			mapped = append(mapped, rune(v))
		}
		lower[rune(code)] = string(mapped)
	}
	if len(lower) < 1000 {
		t.Fatalf("only %d lowercase mappings: the table is truncated", len(lower))
	}
	bad := 0
	for c := rune(0); c <= unicode.MaxRune; c++ {
		if c >= 0xd800 && c <= 0xdfff {
			continue
		}
		want, ok := lower[c]
		if !ok {
			want = string(c)
		}
		if got := pyLower(string(c)); got != want {
			if bad < 5 {
				t.Errorf("str.lower of U+%04X: Go %q, Python %q", c, got, want)
			}
			bad++
		}
	}
	if bad > 0 {
		t.Errorf("str.lower disagrees at %d code points", bad)
	}
}

func libmCall(fn string, x float64) (float64, error) {
	switch fn {
	case "sin":
		return mathUnary(x, libmSin)
	case "cos":
		return mathUnary(x, libmCos)
	case "asin":
		return mathUnary(x, libmAsin)
	case "pow2":
		return pyPow2(x)
	case "sqrt":
		return mathUnary(x, math.Sqrt)
	case "radians":
		return radians(x), nil
	}
	panic("unknown libm function " + fn)
}

// asinTableIndex returns the asncs offset e_asin.c reads for x, or -1.
func asinTableIndex(x float64) int {
	k := highWord(x) & 0x7fffffff
	switch {
	case k < 0x3fc00000 || k >= 0x3fef0000:
		return -1
	case k < 0x3fd00000:
		return 11 * int((k&0x000fffff)>>15)
	case k < 0x3fe00000:
		return 11*int((k&0x000fffff)>>14) + 352
	case k < 0x3fe80000:
		return 1056 + int((k&0x000fe000)>>11)*3
	case k < 0x3fed8000:
		return 992 + int((k&0x000fe000)>>13)*13
	case k < 0x3fee8000:
		return 884 + int((k&0x000fe000)>>13)*14
	default:
		return 768 + int((k&0x000fe000)>>13)*15
	}
}

// TestLibmMatchesPythonOnEveryTableInterval replays the raw libm rows and
// proves they reach every interval of every table the port copies.
func TestLibmMatchesPythonOnEveryTableInterval(t *testing.T) {
	rows := loadGeo(t).edges.Libm
	perFn := map[string]int{}
	asinSeen, inrootSeen, sinSeen, cosSeen := map[int]bool{}, map[int]bool{}, map[int]bool{}, map[int]bool{}
	logSeen, expSeen := map[int]bool{}, map[int]bool{}
	for _, r := range rows {
		f := strings.Split(r, "|")
		if len(f) != 3 {
			t.Fatalf("bad libm row %q", r)
		}
		x := pyFloat(t, f[1])
		got, err := libmCall(f[0], x)
		if ok, _, _ := floatOutcome(t, f[2], got, err); !ok {
			t.Errorf("%s(%v): Go %v %v, Python %s", f[0], x, strconv.FormatFloat(got, 'g', -1, 64), err, f[2])
		}
		perFn[f[0]]++
		ax := math.Abs(x)
		k := highWord(x) & 0x7fffffff
		switch f[0] {
		case "asin":
			if n := asinTableIndex(x); n >= 0 {
				asinSeen[n] = true
			}
			if k >= 0x3fef0000 && k < 0x3ff00000 {
				inrootSeen[int((highWord(0.5*(1-ax))&0x001fffff)>>14)] = true
			}
		case "sin":
			if k >= 0x3e500000 && k < 0x3feb6000 && ax >= 0.126 {
				sinSeen[int(lowWord(usBig+ax))] = true
			}
		case "cos":
			if k >= 0x3e400000 && k < 0x3feb6000 {
				cosSeen[int(lowWord(usBig+ax))] = true
			}
		case "pow2":
			if ax > 0 && ax != 1 && !math.IsInf(ax, 0) && !math.IsNaN(ax) && top12(ax) != 0 {
				ix := math.Float64bits(ax)
				logSeen[int(((ix-0x3fe6_9555_0000_0000)>>45)%128)] = true
				hi, _ := powLogInline(ix)
				ehi := 2 * hi
				if abstop := top12(ehi) & 0x7ff; abstop >= top12(0x1p-54) && abstop < top12(512.0) {
					expSeen[int(math.Float64bits(math.FMA(expInvLn2N, ehi, 0x1.8p52))%128)] = true
				}
			}
		}
	}
	for _, fn := range []string{"sin", "cos", "asin", "pow2", "sqrt", "radians"} {
		if perFn[fn] == 0 {
			t.Errorf("no libm rows for %s", fn)
		}
	}
	wantAsin := 32 + 64 + 64 + 44 + 8 + 4
	requireCoverage(t, "asncs intervals", len(asinSeen), wantAsin)
	requireCoverage(t, "inroot entries", len(inrootSeen), 128)
	requireCoverage(t, "sincostab entries via do_sin", len(sinSeen), 110-16)
	requireCoverage(t, "sincostab entries via do_cos", len(cosSeen), 110)
	requireCoverage(t, "pow log table rows", len(logSeen), 128)
	requireCoverage(t, "exp table rows", len(expSeen), 128)
}

func requireCoverage(t *testing.T, what string, got, want int) {
	t.Helper()
	if got != want {
		t.Errorf("%s: the golden reaches %d of %d", what, got, want)
	}
}

func TestHaversineMatchesPython(t *testing.T) {
	corpus := loadGeo(t)
	rows := append([]string(nil), corpus.edges.Haversine...)
	for _, c := range corpus.fuzz {
		rows = append(rows, c.Haversine)
	}
	bitsBad, roundBad, thresholdBad, raises := 0, 0, 0, 0
	for _, r := range rows {
		f := strings.Split(r, "|")
		args := strings.Split(f[0], ";")
		if len(f) != 2 || len(args) != 4 {
			t.Fatalf("bad haversine row %q", r)
		}
		p := [4]float64{}
		for i := range p {
			p[i] = pyFloat(t, args[i])
		}
		got, err := HaversineKm(p[0], p[1], p[2], p[3])
		if strings.HasPrefix(f[1], "raise:") {
			raises++
		}
		b, rd, th := floatOutcome(t, f[1], got, err)
		if !b {
			bitsBad++
			if bitsBad <= 5 {
				t.Errorf("haversine_km%v: Go %v %v, Python %s", p, got, err, f[1])
			}
		}
		if !rd {
			roundBad++
		}
		if !th {
			thresholdBad++
		}
	}
	t.Logf("haversine: %d cases (%d raise), mismatches: bits %d, round(x, 2) %d, 25 km side %d", len(rows), raises, bitsBad, roundBad, thresholdBad)
	if len(corpus.fuzz) < 2000 || raises == 0 {
		t.Fatalf("haversine corpus too thin: %d fuzz cases, %d raises", len(corpus.fuzz), raises)
	}
	if bitsBad+roundBad+thresholdBad > 0 {
		t.Fatalf("haversine disagrees with Python")
	}
}

func checkNearestRows(t *testing.T, label string, rows []string) (resolved, none int) {
	t.Helper()
	for _, r := range rows {
		f := strings.Split(r, "|")
		if len(f) != 3 {
			t.Fatalf("bad nearest row %q", r)
		}
		lat, lng := pyFloat(t, f[0]), pyFloat(t, f[1])
		area, ok, err := NearestArea(lat, lng)
		var got string
		switch {
		case err != nil:
			got = raiseOf(err)
		case !ok:
			got = "ok:~"
			none++
		default:
			got = "ok:" + area.ID
			resolved++
		}
		if got != f[2] {
			t.Errorf("%s nearest_area(%v, %v): Go %s, Python %s", label, lat, lng, got, f[2])
		}
	}
	return resolved, none
}

func TestNearestAreaMatchesPython(t *testing.T) {
	corpus := loadGeo(t)
	checkNearestRows(t, "edge", corpus.edges.Nearest)
	exactIn, exactOut := checkNearestRows(t, "exact-radius", corpus.edges.NearestExactRadius)
	tiesIn, _ := checkNearestRows(t, "tie", corpus.edges.NearestTies)
	var fuzz []string
	for _, c := range corpus.fuzz {
		fuzz = append(fuzz, c.Nearest)
	}
	fuzzIn, fuzzOut := checkNearestRows(t, "fuzz", fuzz)
	t.Logf("nearest: exactly-25-km points %d (%d still resolve to a nearer area, %d to none), exact ties %d, fuzz %d resolved / %d none",
		exactIn+exactOut, exactIn, exactOut, tiesIn, fuzzIn, fuzzOut)
	if exactOut < 5 || tiesIn < 5 || fuzzIn == 0 || fuzzOut == 0 {
		t.Fatalf("nearest corpus lacks points exactly on the radius (%d) or exact ties (%d)", exactOut, tiesIn)
	}
	for _, r := range corpus.edges.NearestExactRadius {
		f := strings.Split(r, "|")
		lat, lng := pyFloat(t, f[0]), pyFloat(t, f[1])
		exact := false
		for _, area := range All() {
			if km, err := HaversineKm(lat, lng, area.Lat, area.Lng); err == nil && km == MaxAreaRadiusKm {
				exact = true
			}
		}
		if !exact {
			t.Fatalf("exact-radius case (%v, %v) is not 25 km from any area", lat, lng)
		}
	}
}

func TestAreaOfAddressMatchesPython(t *testing.T) {
	corpus := loadGeo(t)
	cases := append([]geoAddressCase(nil), corpus.edges.Address...)
	for _, c := range corpus.fuzz {
		cases = append(cases, c.Address)
	}
	found := map[string]int{}
	for _, c := range cases {
		area, ok := AreaOfAddress(c.Address)
		got := "~"
		if ok {
			got = area.ID
		}
		found[got]++
		if got != c.Result {
			t.Errorf("area_of_address(%+q): Go %s, Python %s", c.Address, got, c.Result)
		}
	}
	t.Logf("area_of_address: %d cases, outcomes %v", len(cases), found)
	for _, id := range []string{"~", "da-lat", "hcm-quan-1", "hcm-quan-3", "hcm-quan-4", "hcm-quan-7", "hcm-phu-nhuan", "hcm-binh-thanh", "hcm-thu-duc"} {
		if found[id] == 0 {
			t.Errorf("no address case resolves to %s", id)
		}
	}
}

package rag

import (
	"sort"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"mobile/services/core/internal/domain/areas"
	"mobile/services/core/internal/domain/promptsafety"
	"mobile/services/core/internal/domain/tuvung"
	"mobile/services/core/internal/repo"
)

// DiemDen is one destination as ResolveDestination reads it.
type DiemDen struct {
	ID   string
	Ten  string
	Tinh string
	// The bounding box.
	Nam, Tay, Bac, Dong float64
}

// DiemDenTuRepo converts repository rows, keeping their order.
func DiemDenTuRepo(rows []repo.Destination) []DiemDen {
	out := make([]DiemDen, len(rows))
	for i, r := range rows {
		d := DiemDen{ID: r.ID, Ten: r.Name, Nam: r.BBoxSouth, Tay: r.BBoxWest, Bac: r.BBoxNorth, Dong: r.BBoxEast}
		if r.Province != nil {
			d.Tinh = *r.Province
		}
		out[i] = d
	}
	return out
}

// GoiY is everything a turn knows that can name a destination, in the order
// ResolveDestination trusts it (design 04 §5.2).
type GoiY struct {
	// KhuVuc is Understand's khu_vuc slot: an id of areas.All().
	KhuVuc string
	// Cau is the asker's own words.
	Cau string
	// TuPhieu are the destinations of the places on Nếp's slip
	// (thay.diaDiem), looked up by the caller.
	TuPhieu []string
	// TuChang are the destinations of the group's outing stops, upcoming
	// ones first and then the last 90 days, looked up by the caller.
	TuChang []string
}

// Where a destination came from.
const (
	NguonKhuVuc = "khu_vuc"
	NguonTen    = "ten"
	NguonPhieu  = "phieu"
	NguonChang  = "lich_su"
)

// DiemDenGiai is ResolveDestination's answer.
type DiemDenGiai struct {
	// ID is the destination, or "" when nothing names exactly one.
	ID    string
	Nguon string
	// NhieuDiemDen: the words named more than one destination, so none was
	// chosen.
	NhieuDiemDen bool
	// Cum are the spans of the words that named it, as folded syllables, for
	// the lexical query to drop: «Đà Lạt» is a slot, not a word to match.
	Cum [][]string
	// Thieu is "khu_vuc" when unresolved: the caller asks one question
	// instead of guessing.
	Thieu string
}

// Names people use for a destination besides its own, by id. Written with
// their marks: a span typed with marks must match them (see khopTen).
var tenKhac = map[string][]string{
	"d-tphcm":  {"sài gòn", "saigon", "hồ chí minh", "hcm", "tphcm", "tp hcm", "thành phố hồ chí minh"},
	"d-da-lat": {"dalat"},
	"d-ha-noi": {"hanoi", "thủ đô"},
	"d-hue":    {"cố đô huế"},
	"d-sa-pa":  {"sapa"},
}

// ResolveDestination names the destination a turn is about, or says it
// cannot: an area slot whose centre lies in a destination's box; else the one
// destination the words name (a name, a known other name, or an area label);
// else the one most of the slip's places are in; else the one most of the
// group's stops are in. Two destinations named, or a tie, is unresolved. It
// never falls back to a default -- not the first by sort order, not the
// smallest -- because «luôn Đà Lạt» was exactly that fallback (design 04 §7).
func ResolveDestination(dests []DiemDen, g GoiY) DiemDenGiai {
	known := map[string]bool{}
	for _, d := range dests {
		known[d.ID] = true
	}
	if g.KhuVuc != "" {
		if area, ok := areas.Find(g.KhuVuc); ok {
			if id := chua(dests, area); id != "" {
				return DiemDenGiai{ID: id, Nguon: NguonKhuVuc}
			}
		}
	}
	named, spans := tenTrongCau(dests, g.Cau)
	switch len(named) {
	case 1:
		for id := range named {
			return DiemDenGiai{ID: id, Nguon: NguonTen, Cum: spans}
		}
	case 0:
	default:
		return DiemDenGiai{NhieuDiemDen: true, Cum: spans, Thieu: "khu_vuc"}
	}
	if id := daSo(g.TuPhieu, known); id != "" {
		return DiemDenGiai{ID: id, Nguon: NguonPhieu}
	}
	if id := daSo(g.TuChang, known); id != "" {
		return DiemDenGiai{ID: id, Nguon: NguonChang}
	}
	return DiemDenGiai{Thieu: "khu_vuc"}
}

// chua is the destination whose box holds the area's centre; "" for none or
// for more than one.
func chua(dests []DiemDen, a areas.Area) string {
	found := ""
	for _, d := range dests {
		if d.Nam <= a.Lat && a.Lat <= d.Bac && d.Tay <= a.Lng && a.Lng <= d.Dong {
			if found != "" && found != d.ID {
				return ""
			}
			found = d.ID
		}
	}
	return found
}

// daSo is the strictly most frequent known id; "" when empty or tied.
func daSo(ids []string, known map[string]bool) string {
	count := map[string]int{}
	for _, id := range ids {
		if known[id] {
			count[id]++
		}
	}
	best, top, tied := "", 0, false
	keys := make([]string, 0, len(count))
	for id := range count {
		keys = append(keys, id)
	}
	sort.Strings(keys)
	for _, id := range keys {
		switch n := count[id]; {
		case n > top:
			best, top, tied = id, n, false
		case n == top:
			tied = true
		}
	}
	if tied {
		return ""
	}
	return best
}

// tenTrongCau returns the destinations the words name and the spans that
// named them.
func tenTrongCau(dests []DiemDen, cau string) (map[string]bool, [][]string) {
	named := map[string]bool{}
	var spans [][]string
	if strings.TrimSpace(cau) == "" {
		return named, nil
	}
	folded := tuvung.AmTiet(cau)
	marked := amTietGiuDau(cau)
	for _, d := range dests {
		for _, phrase := range tenCua(d) {
			if khopTen(folded, marked, phrase) {
				named[d.ID] = true
				spans = append(spans, tuvung.AmTiet(phrase))
			}
		}
	}
	for _, a := range areas.All() {
		label, _, _ := strings.Cut(a.Label, ",")
		if !khopTen(folded, marked, label) {
			continue
		}
		if id := chua(dests, a); id != "" {
			named[id] = true
			spans = append(spans, tuvung.AmTiet(label))
		}
	}
	return named, spans
}

// tenCua lists what a destination may be called: its name, its name without
// «TP.»/«Thành phố»/«Tỉnh», and the other names it is known by.
func tenCua(d DiemDen) []string {
	out := []string{d.Ten}
	s := tuvung.AmTiet(d.Ten)
	for _, prefix := range [][]string{{"tp"}, {"thanh", "pho"}, {"tinh"}} {
		if len(s) > len(prefix) && strings.Join(s[:len(prefix)], " ") == strings.Join(prefix, " ") {
			// Keep the marks of the remaining words: cut the same number of
			// syllables from the marked name.
			marked := amTietGiuDau(d.Ten)
			if len(marked) == len(s) {
				out = append(out, strings.Join(marked[len(prefix):], " "))
			}
		}
	}
	return append(out, tenKhac[d.ID]...)
}

// khopTen reports whether phrase occurs in the words. It matches on folded
// syllables, so «da lat» finds «Đà Lạt»; but a span the asker typed with
// marks must carry the phrase's own marks, so «hỏi ăn gì» does not name Hội
// An. A span typed without marks cannot be told apart and is taken.
func khopTen(folded, marked []string, phrase string) bool {
	want := tuvung.AmTiet(phrase)
	wantMarked := amTietGiuDau(phrase)
	aligned := len(marked) == len(folded)
	for _, at := range tuvung.ViTri(folded, want) {
		if !aligned || len(wantMarked) != len(want) {
			return true
		}
		ok := true
		for j := range want {
			w := marked[at+j]
			if promptsafety.Fold(w) != w && w != wantMarked[j] {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

// amTietGiuDau cuts text into lower-case NFC syllables the way tuvung.AmTiet
// does, but keeps the marks.
func amTietGiuDau(text string) []string {
	lower := strings.ToLower(norm.NFC.String(text))
	return strings.FieldsFunc(lower, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && !unicode.Is(unicode.Mn, r)
	})
}

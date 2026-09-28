package nap

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"mobile/services/core/internal/domain/catalog"
	"mobile/services/core/internal/domain/giomo"
	"mobile/services/core/internal/domain/promptsafety"
	"mobile/services/core/internal/domain/tree"
	"mobile/services/core/internal/repo"
	"mobile/services/core/internal/service"
	"mobile/services/core/internal/treejson"
)

// Bounds of the place.v1 source contract (design 04 §6.1 step 1), applied to
// what reaches a chunk or a prompt.
const (
	maxTen      = 120
	maxDiaChi   = 200
	maxMoTa     = 1500
	maxDanhGia  = 5
	maxRuneDG   = 600
	maxHoatDong = 20
	maxChunk    = 2000
)

// HoSoQuan is one place as the pipeline sees it after S1-S2: the structural
// fields the hard filters read (from typed columns, never from text) and the
// safe text a chunk and the enrichment prompt carry.
type HoSoQuan struct {
	ID      string
	DiemDen string
	LoaiCho string
	Ten     string
	GiaMin  *int64
	GiaMax  *int64
	// Lich is nil when the hours are missing or unreadable (unknown, which is
	// not closed). giomo reads a structured opening-hours format; it is a
	// format parser, not a reading of meaning.
	Lich *giomo.Lich
	Lat  float64
	Lng  float64
	// CoToaDo is false when the place has no coordinates; dedupe then never
	// merges it by distance.
	CoToaDo bool
	Nguon   string // seed | osm | curated: dedupe precedence
	License string
	// HoSo and DanhGia are the two facets' safe text, NFC.
	HoSo    string
	DanhGia string
	// NguonHash is sha256 of the canonical JSON of the safe row: what the
	// enrichment model saw and what a chunk is built from. Unchanged hash =
	// no-op (S1).
	NguonHash [32]byte
	// CachLy counts fields SafeDeep quarantined.
	CachLy int
}

// NguonHashHex is the source hash as hex.
func (h HoSoQuan) NguonHashHex() string { return hex.EncodeToString(h.NguonHash[:]) }

// DungHoSo runs S1-S2 on a live row. bo reports a row SafeDeep drops (a
// field that names the place is unsafe): the caller tombstones it `unsafe`
// and it reaches no prompt and no chunk.
func DungHoSo(p repo.Place) (h HoSoQuan, bo bool) {
	safe, report := promptsafety.SafeDeep(treejson.MapTo(service.PlaceRow(p)))
	if report.Bo {
		return HoSoQuan{ID: p.ID}, true
	}
	h = HoSoQuan{
		ID: p.ID, DiemDen: p.DestinationID, LoaiCho: p.Category,
		GiaMin: p.PriceMinVND, GiaMax: p.PriceMaxVND,
		Nguon: p.Source, CachLy: len(report.CachLy),
	}
	if p.Lat != nil && p.Lng != nil {
		h.Lat, h.Lng, h.CoToaDo = *p.Lat, *p.Lng, true
	}
	h.Ten = nfc(catRune(chu(safe, "name"), maxTen))
	h.License = chu(safe, "license")
	if hours := chu(safe, "open_hours"); hours != "" {
		if l, ok := giomo.Doc(hours); ok {
			h.Lich = &l
		}
	}
	kinds := danhSach(safe, "kinds", maxHoatDong)
	traits := danhSach(safe, "traits", maxHoatDong)
	activities := danhSach(safe, "activities", maxHoatDong)
	var lines []string
	add := func(s string) {
		if s = strings.TrimSpace(s); s != "" {
			lines = append(lines, s)
		}
	}
	add(h.Ten)
	head := nhanLoai(p.Category)
	if len(kinds) > 0 {
		head += " · " + strings.Join(kinds, ", ")
	}
	add(head)
	add(catRune(chu(safe, "address"), maxDiaChi))
	add(strings.Join(traits, ", "))
	add(strings.Join(activities, ", "))
	add(catRune(chu(safe, "description"), maxMoTa))
	h.HoSo = nfc(catRune(strings.Join(lines, "\n"), maxChunk))
	var reviews []string
	for _, r := range danhGia(safe) {
		if len(reviews) == maxDanhGia {
			break
		}
		reviews = append(reviews, catRune(r, maxRuneDG))
	}
	h.DanhGia = nfc(catRune(strings.Join(reviews, "\n"), maxChunk))
	canon, _ := json.Marshal(struct {
		ID, DiemDen, LoaiCho, HoSo, DanhGia, License string
		GiaMin, GiaMax                               *int64
		Gio                                          string
		Lat, Lng                                     float64
		CoToaDo                                      bool
	}{h.ID, h.DiemDen, h.LoaiCho, h.HoSo, h.DanhGia, h.License, h.GiaMin, h.GiaMax, chu(safe, "open_hours"), h.Lat, h.Lng, h.CoToaDo})
	h.NguonHash = sha256.Sum256(canon)
	return h, false
}

func nfc(s string) string { return norm.NFC.String(s) }

func catRune(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

func nhanLoai(id string) string {
	for _, c := range catalog.Categories {
		if c.ID == id {
			return c.Label
		}
	}
	return id
}

func chu(m *tree.OrderedMap, key string) string {
	v, _ := m.Get(key)
	s, _ := v.(tree.String)
	return strings.TrimSpace(string(s))
}

func danhSach(m *tree.OrderedMap, key string, max int) []string {
	v, _ := m.Get(key)
	list, _ := v.(tree.List)
	var out []string
	for _, item := range list {
		if len(out) == max {
			break
		}
		if s, ok := item.(tree.String); ok && strings.TrimSpace(string(s)) != "" {
			out = append(out, strings.TrimSpace(string(s)))
		}
	}
	return out
}

// danhGia is the bodies of the safe reviews; authors never leave the row.
func danhGia(m *tree.OrderedMap) []string {
	v, _ := m.Get("reviews")
	list, _ := v.(tree.List)
	var out []string
	for _, item := range list {
		r, ok := item.(*tree.OrderedMap)
		if !ok {
			continue
		}
		if body := chu(r, "body"); body != "" {
			out = append(out, body)
		}
	}
	return out
}

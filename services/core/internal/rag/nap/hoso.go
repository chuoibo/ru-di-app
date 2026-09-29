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
//
// The description and the feed's review block are never cut (owner,
// 2026-09-28): a facet longer than a chunk is split by meaning (chia.go).
const (
	maxTen      = 120
	maxDiaChi   = 200
	maxDanhGia  = 5
	maxRuneDG   = 600
	maxHoatDong = 20
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
	// HoSo, TraiNghiem and MonAn are the three facets' safe text, NFC,
	// never cut.
	HoSo       string
	TraiNghiem string
	MonAn      string
	// NguonHash is sha256 of the canonical JSON of what the enrichment
	// model saw: the safe text and the row's identity, not its price or
	// hours. An enrichment whose hash differs is stale (ApDung).
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
	// Only a point that is the place (rooftop, street) counts (M7,
	// repo.Place.MappablePoint): two cafés of one ward share the ward's
	// centroid, sit 0 m apart, and dedupe would merge them into one.
	if lat, lng := p.MappablePoint(); lat != nil && lng != nil {
		h.Lat, h.Lng, h.CoToaDo = *lat, *lng, true
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
	block, cachLy := dongReview(p.Reviews)
	h.CachLy += cachLy

	// ho_so: what the place is and why it is worth it.
	add(h.Ten)
	head := nhanLoai(p.Category)
	if len(kinds) > 0 {
		head += " · " + strings.Join(kinds, ", ")
	}
	add(head)
	add(tenTinh(p.DestinationID))
	add(catRune(chu(safe, "address"), maxDiaChi))
	add(strings.Join(traits, ", "))
	add(strings.Join(activities, ", "))
	if p.Description != nil {
		if d := strings.TrimSpace(*p.Description); d != "" && promptsafety.TextSafe(d, maxRuneMuc) {
			add(d)
		} else if d != "" {
			h.CachLy++
		}
	}
	for _, l := range block[FacetHoSo] {
		add(l)
	}
	h.HoSo = nfc(strings.Join(lines, "\n"))

	// trai_nghiem: what being there is like; the seed catalogue's review
	// bodies land here too.
	tn := block[FacetTraiNghiem]
	var reviews []string
	for _, r := range danhGia(safe) {
		if len(reviews) == maxDanhGia {
			break
		}
		reviews = append(reviews, catRune(r, maxRuneDG))
	}
	if len(reviews) > 0 {
		tn = append(tn, "Đánh giá: "+strings.Join(reviews, "\n"))
	}
	h.TraiNghiem = nfc(strings.Join(tn, "\n"))
	// mon_an: what to eat there.
	h.MonAn = nfc(strings.Join(block[FacetMonAn], "\n"))

	// Price and hours are not in the hash: the enrichment model never sees
	// them (BocQuan lays out HoSo, TraiNghiem and MonAn only), and the chunk
	// reads them from h.GiaMin/h.GiaMax/h.Lich at build time, so a web-facts
	// refresh of a place's hours must not throw its enrichment away. The
	// keys stay, always zero, because every stored hash was written with
	// them while no fed row had a price or hours (2026-09-29): the bytes of
	// the canonical JSON, and so those hashes, remain valid.
	canon, _ := json.Marshal(struct {
		ID, DiemDen, LoaiCho, HoSo, TraiNghiem, MonAn, License string
		GiaMin, GiaMax                                         *int64
		Gio                                                    string
		Lat, Lng                                               float64
		CoToaDo                                                bool
	}{h.ID, h.DiemDen, h.LoaiCho, h.HoSo, h.TraiNghiem, h.MonAn, h.License, nil, nil, "", h.Lat, h.Lng, h.CoToaDo})
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

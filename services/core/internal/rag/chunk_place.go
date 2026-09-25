package rag

import (
	"crypto/sha256"
	"strings"
	"unicode/utf8"

	"mobile/services/core/internal/domain/catalog"
	"mobile/services/core/internal/domain/giomo"
	"mobile/services/core/internal/domain/promptsafety"
	"mobile/services/core/internal/domain/tree"
	"mobile/services/core/internal/domain/tuvung"
	"mobile/services/core/internal/rag/xephang"
	"mobile/services/core/internal/repo"
	"mobile/services/core/internal/service"
	"mobile/services/core/internal/treejson"
)

// Chunker names this file's way of cutting a place. It is stored on every
// version; changing what a chunk holds is a new name, a new version, and a
// manual promotion (design 04 §6.1 step 9). The golden content hashes in
// chunk_place_test.go pin it.
const Chunker = "place.v1"

// Facets of a place.
const (
	FacetHoSo    = "ho_so"
	FacetDanhGia = "danh_gia"
)

const (
	maxChunk      = 2000
	maxReviewRune = 600
	maxReviewsDoc = 5
)

// Doan is one chunk: evidence a model may quote (Body) and the terms the
// full-text list matches (ChuTim = xephang.ChuTimKiem(Body)).
type Doan struct {
	ChunkID string
	Facet   string
	Body    string
	ChuTim  string
	Hash    [32]byte
}

// HoSo is one place as the index sees it: what the hard filters test and
// what the rankings read, taken only from the row's safe fields.
type HoSo struct {
	ID      string
	DiemDen string
	LoaiCho string
	GiaMin  *int64
	GiaMax  *int64
	// Lich is nil when the hours are missing or unreadable: unknown, which is
	// not closed.
	Lich    *giomo.Lich
	DiUng   []string
	AnKieng []string
	KhiChat []string
	TenGap  string
	Lat     float64
	Lng     float64
	License *string
	Doan    []Doan
}

// DungHoSo builds a place's profile and chunks from its live row. The row
// goes through promptsafety.SafeDeep first: a row it drops comes back with
// report.Bo set and no profile (the index tombstones it `unsafe`); a field it
// quarantines is absent from every chunk and from the diet and atmosphere
// tags, but its raw words still count toward the allergen tags, which can
// only hide the place.
func DungHoSo(p repo.Place) (HoSo, promptsafety.KetQuaSau) {
	safe, report := promptsafety.SafeDeep(treejson.MapTo(service.PlaceRow(p)))
	if report.Bo {
		return HoSo{ID: p.ID}, report
	}
	h := HoSo{
		ID: p.ID, DiemDen: p.DestinationID, LoaiCho: p.Category,
		GiaMin: p.PriceMinVND, GiaMax: p.PriceMaxVND,
		TenGap: promptsafety.Fold(p.Name), Lat: p.Lat, Lng: p.Lng,
	}
	if lic := chu(safe, "license"); lic != "" {
		h.License = &lic
	}
	if hours := chu(safe, "open_hours"); hours != "" {
		if l, ok := giomo.Doc(hours); ok {
			h.Lich = &l
		}
	}
	kinds := danhSach(safe, "kinds")
	traits := danhSach(safe, "traits")
	activities := danhSach(safe, "activities")
	description := chu(safe, "description")
	reviews := danhGia(safe)

	var lines []string
	add := func(s string) {
		if s = strings.TrimSpace(s); s != "" {
			lines = append(lines, s)
		}
	}
	add(chu(safe, "name"))
	head := nhanLoai(p.Category)
	if len(kinds) > 0 {
		head += " · " + strings.Join(kinds, ", ")
	}
	add(head)
	add(chu(safe, "address"))
	add(strings.Join(traits, ", "))
	add(strings.Join(activities, ", "))
	add(description)
	h.Doan = append(h.Doan, doan(p.ID, FacetHoSo, strings.Join(lines, "\n")))

	var kept []string
	for _, r := range reviews {
		if len(kept) == maxReviewsDoc {
			break
		}
		kept = append(kept, catRune(r, maxReviewRune))
	}
	if len(kept) > 0 {
		h.Doan = append(h.Doan, doan(p.ID, FacetDanhGia, strings.Join(kept, "\n")))
	}

	// Allergens come from the raw text of every descriptive field, the ones
	// SafeDeep quarantined included and a description past its length bound
	// in full: the scan is deterministic, reaches no model, and can only
	// hide a place, so a field that may not be quoted may still keep a
	// seafood-allergic asker away from a seafood place (design 04 §4a).
	// Whether the writer typed marks is judged over the whole row, then each
	// string is read alone; a kind or a trait is also read as a label, so an
	// importer's bare «Cua» or «Muc» counts.
	raw := treejson.MapTo(service.PlaceRow(p))
	texts := chuMoTa(raw)
	coDauRaw := tuvung.CoDau(texts...)
	var diUng []string
	for _, text := range texts {
		diUng = append(diUng, tuvung.DiUngQuanHang(text, coDauRaw)...)
	}
	for _, label := range append(append([]string(nil), p.Kinds...), p.Traits...) {
		diUng = append(diUng, tuvung.DiUngNhan(label, coDauRaw)...)
	}
	h.DiUng = tuvung.DiUng.LocHopLe(diUng)
	// Diets come only from what the place declares about itself, its name,
	// kinds and traits, each read alone, with the marks of the row SafeDeep
	// kept; a review or a description saying «chay» may be a wish, a
	// complaint or the past, and a wrong yes here is the unsafe answer.
	declared := append(append([]string{chu(safe, "name")}, kinds...), traits...)
	coDauSafe := tuvung.CoDau(chuMoTa(safe)...)
	var anKieng []string
	for _, field := range declared {
		anKieng = append(anKieng, tuvung.AnKiengQuanHang(field, coDauSafe)...)
	}
	h.AnKieng = tuvung.DoiKieng(anKieng)
	// Atmospheres, a soft preference, come from the same safe words the
	// chunks hold, the address excepted: a street name is not a menu.
	words := strings.Join(append(append(append([]string{chu(safe, "name")}, kinds...), traits...), activities...), " · ") +
		"\n" + description + "\n" + strings.Join(reviews, "\n")
	h.KhiChat = tuvung.KhiChat.QuetAmTiet(tuvung.AmTiet(words))
	return h, report
}

// khongMoTa are the fields of a catalogue row that say what the row is, not
// what the place serves: ids, the category, provenance, where it is, when it
// opens, and who wrote or photographed. A street name is not a menu.
var khongMoTa = map[string]bool{
	"id": true, "destination_id": true, "category": true, "source": true, "license": true, "address": true,
	"open_hours": true, "photo_author": true, "photo_license": true, "author": true,
}

// chuMoTa returns every string of a raw catalogue row outside khongMoTa, at
// any depth (a review's body and any extra note, group_fit, an activity),
// each on its own so no phrase runs from one field into the next.
func chuMoTa(v tree.Value) []string {
	var out []string
	var walk func(tree.Value)
	walk = func(v tree.Value) {
		switch x := v.(type) {
		case tree.String:
			if s := strings.TrimSpace(string(x)); s != "" {
				out = append(out, s)
			}
		case tree.List:
			for _, item := range x {
				walk(item)
			}
		case *tree.OrderedMap:
			for _, k := range x.Keys() {
				if !khongMoTa[k] {
					item, _ := x.Get(k)
					walk(item)
				}
			}
		}
	}
	walk(v)
	return out
}

func doan(docID, facet, body string) Doan {
	body = catRune(body, maxChunk)
	d := Doan{ChunkID: docID + "#" + facet, Facet: facet, Body: body, ChuTim: xephang.ChuTimKiem(body)}
	d.Hash = sha256.Sum256([]byte(Chunker + "\x00" + facet + "\x00" + body))
	return d
}

// catRune cuts s to at most n runes.
func catRune(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n])
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
	return string(s)
}

func danhSach(m *tree.OrderedMap, key string) []string {
	v, _ := m.Get(key)
	list, _ := v.(tree.List)
	var out []string
	for _, item := range list {
		if s, ok := item.(tree.String); ok && strings.TrimSpace(string(s)) != "" {
			out = append(out, string(s))
		}
	}
	return out
}

// danhGia is the bodies of the safe reviews, authors left out (design 04 §4a).
func danhGia(m *tree.OrderedMap) []string {
	v, _ := m.Get("reviews")
	list, _ := v.(tree.List)
	var out []string
	for _, item := range list {
		r, ok := item.(*tree.OrderedMap)
		if !ok {
			continue
		}
		if body := strings.TrimSpace(chu(r, "body")); body != "" {
			out = append(out, body)
		}
	}
	return out
}

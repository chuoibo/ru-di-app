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
// quarantines is simply absent from every chunk and every tag.
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

	// Tags come from the same safe words the chunks hold, the address
	// excepted: a street name is not a menu.
	words := strings.Join(append(append(append([]string{chu(safe, "name")}, kinds...), traits...), activities...), " · ") +
		"\n" + description + "\n" + strings.Join(reviews, "\n")
	s := tuvung.AmTiet(words)
	h.DiUng = tuvung.DiUng.QuetAmTiet(s)
	h.AnKieng = tuvung.DoiKieng(tuvung.AnKieng.QuetAmTietKhongPhuDinh(s))
	h.KhiChat = tuvung.KhiChat.QuetAmTiet(s)
	return h, report
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

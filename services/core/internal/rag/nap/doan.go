package nap

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"mobile/services/core/internal/domain/giomo"
	"mobile/services/core/internal/domain/tuvung"
	"mobile/services/core/internal/huongdan"
)

// Facets of a document.
const (
	FacetHoSo    = "ho_so"
	FacetDanhGia = "danh_gia"
	FacetMuc     = "muc"
)

// PhutMoiO is the width of one opening slot: vectordb's half-hour slot
// (vectordb.PhutMoiSlot; napkho checks they agree). A slot is marked open
// only when the place is open for the whole of it, so a filter on slots can
// lose a place at the edge of its hours but never admit a closed one.
const PhutMoiO = 30

// SoO is the number of slots in a week.
const SoO = giomo.PhutTuan / PhutMoiO

// ChunkID is hex(sha256(doc_id ‖ facet ‖ chunker))[:32]: the primary key of
// a chunk in every collection. Deterministic, so an upsert repeated is one
// row (Milvus with autoID would mint a new key per upsert; research
// sdlc-production §B1).
func ChunkID(docID, facet, chunker string) string {
	sum := sha256.Sum256([]byte(docID + "\x00" + facet + "\x00" + chunker))
	return hex.EncodeToString(sum[:])[:32]
}

// Hang is one row of a collection: a chunk and the fields its hard filters
// and staleness checks read. The manual uses only the common fields and Man.
type Hang struct {
	ChunkID string
	DocID   string
	Facet   string
	TieuDe  string // embedded as the document title, not stored
	// Text is NFC; the dense leg embeds it and both BM25 functions read
	// it. A context line the enrichment wrote leads it (contextual
	// retrieval).
	Text        string
	ContentHash string
	Dense       []float32
	// SparseIdx/SparseVal are the MILCO vector (MILCO mode only).
	SparseIdx []uint32
	SparseVal []float32

	// The hard-filter attributes, stored on every chunk of the place. The
	// soft ones (LoaiCho, AnKiengGoiY, KhiChat, Lat/Lng) only feed dedupe
	// and are not stored in the index.
	DiemDen     string
	LoaiCho     string
	DiUng       []string
	DiUngRo     bool
	AnKieng     []string
	AnKiengGoiY []string
	KhiChat     []string
	GiaMin      int64
	GiaMax      int64
	GiaRo       bool
	MoO         []int16
	GioRo       bool
	Lat, Lng    float64

	DenseModel string
	SparseRev  string
	Chunker    string
}

// MoO returns the slots of the week the schedule is open for in full.
func MoO(l giomo.Lich) []int16 {
	var out []int16
	for s := 0; s < SoO; s++ {
		if l.MoSuot(s*PhutMoiO, (s+1)*PhutMoiO) {
			out = append(out, int16(s))
		}
	}
	return out
}

// DoanQuan builds a place's chunks from its safe profile and its enrichment
// attributes. The main dishes the enrichment named join the profile text, so
// they are embedded and matched; the closed ids are fields, not text. A
// context line of the facet (contextual retrieval) leads the chunk's text,
// so the dense and both BM25 legs read it, and the content hash covers it.
func DoanQuan(h HoSoQuan, t ThuocTinh, chunker string) []Hang {
	base := Hang{
		DocID: h.ID, DiemDen: h.DiemDen, LoaiCho: h.LoaiCho, TieuDe: h.Ten,
		DiUng: tuvung.DiUng.LocHopLe(t.DiUng), DiUngRo: t.DiUngRo,
		AnKieng: t.AnKieng, AnKiengGoiY: t.AnKiengGoiY, KhiChat: t.KhiChat,
		Lat: h.Lat, Lng: h.Lng, Chunker: chunker,
	}
	if h.GiaMin != nil {
		base.GiaRo, base.GiaMin = true, *h.GiaMin
		base.GiaMax = *h.GiaMin
		if h.GiaMax != nil {
			base.GiaMax = *h.GiaMax
		}
	}
	if h.Lich != nil {
		base.GioRo, base.MoO = true, MoO(*h.Lich)
	}
	profile := h.HoSo
	if len(t.MonChinh) > 0 {
		profile = catRune(profile+"\nMón chính: "+strings.Join(t.MonChinh, ", "), maxChunk)
	}
	var out []Hang
	add := func(facet, text string) {
		if strings.TrimSpace(text) == "" {
			return
		}
		if line := t.NguCanh[facet]; line != "" {
			text = line + "\n" + text
		}
		r := base
		r.Facet, r.Text = facet, nfc(text)
		r.ChunkID = ChunkID(h.ID, facet, chunker)
		r.ContentHash = hashNoiDung(chunker, facet, r.TieuDe, r.Text)
		out = append(out, r)
	}
	add(FacetHoSo, profile)
	add(FacetDanhGia, h.DanhGia)
	return out
}

// FacetsQuan are every facet a place may have: an indexer deletes the ids of
// the ones a place no longer has.
var FacetsQuan = []string{FacetHoSo, FacetDanhGia}

// DoanSoTay builds one chunk per manual section.
func DoanSoTay(ds []huongdan.Doan, chunker string) []Hang {
	out := make([]Hang, 0, len(ds))
	for _, d := range ds {
		text := d.TieuDe + "\n" + d.Chu
		r := Hang{DocID: d.ID, Facet: FacetMuc, TieuDe: nfc(d.TieuDeMan + " — " + d.TieuDe), Text: nfc(catRune(text, maxChunk)),
			Chunker: chunker}
		r.ChunkID = ChunkID(d.ID, FacetMuc, chunker)
		r.ContentHash = hashNoiDung(chunker, FacetMuc, r.TieuDe, r.Text)
		out = append(out, r)
	}
	return out
}

func hashNoiDung(chunker, facet, title, text string) string {
	sum := sha256.Sum256([]byte(chunker + "\x00" + facet + "\x00" + title + "\x00" + text))
	return hex.EncodeToString(sum[:])
}

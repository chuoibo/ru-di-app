package nap

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"mobile/services/core/internal/domain/giomo"
	"mobile/services/core/internal/domain/tuvung"
	"mobile/services/core/internal/huongdan"
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

// ChunkIDManh is the id of piece n of a split facet: piece 0 keeps
// ChunkID, so a facet that fits in one chunk keeps the id it always had.
func ChunkIDManh(docID, facet string, n int, chunker string) string {
	if n == 0 {
		return ChunkID(docID, facet, chunker)
	}
	return ChunkID(docID, facet+"#"+strconv.Itoa(n), chunker)
}

// MoiIDQuan is every chunk id a place can have under chunker: each facet
// times MaxManh pieces. The indexer deletes a document by these ids.
func MoiIDQuan(docID, chunker string) []string {
	out := make([]string, 0, len(FacetsQuan)*MaxManh)
	for _, f := range FacetsQuan {
		for n := 0; n < MaxManh; n++ {
			out = append(out, ChunkIDManh(docID, f, n, chunker))
		}
	}
	return out
}

// Hang is one row of a collection: a chunk and the fields its hard filters
// and staleness checks read. The manual uses only the common fields and Man.
type Hang struct {
	ChunkID string
	DocID   string
	Facet   string
	// ChunkSo is the piece of a split facet (0 when the facet is whole).
	ChunkSo int16
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
// attributes: one chunk per facet (ho_so, trai_nghiem, mon_an), or several
// when chia splits a long facet by meaning; nothing is cut. The main dishes
// the enrichment named join the food facet, so they are embedded and
// matched; the closed ids are fields, not text. The facet's context line
// (contextual retrieval) leads every piece of it, so the dense and both BM25
// legs read it, and the content hash covers it.
func DoanQuan(ctx context.Context, h HoSoQuan, t ThuocTinh, chunker string, chia ChiaDoan) ([]Hang, error) {
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
	monAn := h.MonAn
	if len(t.MonChinh) > 0 {
		monAn = strings.TrimSpace(monAn + "\nMón chính: " + strings.Join(t.MonChinh, ", "))
	}
	var out []Hang
	for _, f := range []struct{ facet, text string }{{FacetHoSo, h.HoSo}, {FacetTraiNghiem, h.TraiNghiem}, {FacetMonAn, monAn}} {
		if strings.TrimSpace(f.text) == "" {
			continue
		}
		manh, err := chia.Chia(ctx, f.text)
		if err != nil {
			return nil, fmt.Errorf("%s/%s: %w", h.ID, f.facet, err)
		}
		for n, text := range manh {
			if line := t.NguCanh[f.facet]; line != "" {
				text = line + "\n" + text
			}
			r := base
			r.Facet, r.ChunkSo, r.Text = f.facet, int16(n), nfc(text)
			r.ChunkID = ChunkIDManh(h.ID, f.facet, n, chunker)
			key := f.facet
			if n > 0 {
				key += "#" + strconv.Itoa(n)
			}
			r.ContentHash = hashNoiDung(chunker, key, r.TieuDe, r.Text)
			out = append(out, r)
		}
	}
	return out, nil
}

// DoanSoTay builds one chunk per manual section.
func DoanSoTay(ds []huongdan.Doan, chunker string) []Hang {
	out := make([]Hang, 0, len(ds))
	for _, d := range ds {
		text := d.TieuDe + "\n" + d.Chu
		r := Hang{DocID: d.ID, Facet: FacetMuc, TieuDe: nfc(d.TieuDeMan + " — " + d.TieuDe), Text: nfc(catRune(text, NguongDoan)),
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

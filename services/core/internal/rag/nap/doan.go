package nap

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
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

// MaxByteVanBan is the longest text a row holds, in BYTES (vectordb's
// MaxTextLen: Milvus counts VarChar bytes). The feed's longest place is
// 5,612 runes, well inside it. A longer text is never cut: the place is
// counted and left out (ErrQuaDai).
const MaxByteVanBan = 32768

// maxRuneSoTay bounds one manual section's text.
const maxRuneSoTay = 2000

// ErrQuaDai: a place's text is longer than MaxByteVanBan.
var ErrQuaDai = errors.New("nap: place text longer than a row holds")

// MoiIDQuan is every row id a place has: its own id (rd.v4, one row per
// place). The indexer deletes a document by these ids.
func MoiIDQuan(docID string) []string { return []string{docID} }

// Hang is one row of a collection: a whole place (rd.v4, one row and one
// dense vector per place) or a manual section, and the fields its hard
// filters and staleness checks read. ChunkID is the row's primary key and
// equals DocID. The manual uses only the common fields.
type Hang struct {
	ChunkID string
	DocID   string
	TieuDe  string // embedded as the document title, not stored
	// Text is NFC; the dense leg embeds it and the BM25 function reads it.
	Text        string
	ContentHash string
	Dense       []float32

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
	// DanhMuc are the place's categories (tuvung.DanhMuc ids); empty until
	// the classification lands, which the index stores as [khong_ro].
	DanhMuc  []string
	Lat, Lng float64

	DenseModel string
	SparseRev  string
	Chunker    string
}

// DauThuocTinh is the fingerprint of everything a place row stores besides
// its text and vector: the hard-filter attributes and the categories, as
// the index writes them. The adapter stores it in the row (vectordb's
// FMoRong); a row whose fingerprint and content hash both match needs no
// write, one whose hash matches needs only a partial update.
func DauThuocTinh(r Hang) string {
	b, _ := json.Marshal(struct {
		DiemDen        string
		DiUng, AnKieng []string
		DiUngRo        bool
		GiaMin, GiaMax int64
		GiaRo          bool
		MoO            []int16
		GioRo          bool
		DanhMuc        []string
	}{r.DiemDen, r.DiUng, r.AnKieng, r.DiUngRo, r.GiaMin, r.GiaMax, r.GiaRo, r.MoO, r.GioRo, r.DanhMuc})
	sum := sha256.Sum256(append([]byte("dau.v1\x00"), b...))
	return hex.EncodeToString(sum[:12])
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

// DoanQuan builds a place's one row (rd.v4, owner 2026-09-29): the place's
// embedded text (HoSoQuan.VanBan) with the main dishes the enrichment named
// as a last «Món chính:» line, and the attributes its hard filters read. No
// chunking, no context line; nothing is cut. A place with no text has no
// row.
func DoanQuan(h HoSoQuan, t ThuocTinh, chunker string) ([]Hang, error) {
	r := Hang{
		ChunkID: h.ID, DocID: h.ID, DiemDen: h.DiemDen, LoaiCho: h.LoaiCho, TieuDe: h.Ten,
		DiUng: tuvung.DiUng.LocHopLe(t.DiUng), DiUngRo: t.DiUngRo,
		AnKieng: t.AnKieng, AnKiengGoiY: t.AnKiengGoiY, KhiChat: t.KhiChat,
		Lat: h.Lat, Lng: h.Lng, Chunker: chunker, DanhMuc: t.DanhMuc,
	}
	if h.GiaMin != nil {
		r.GiaRo, r.GiaMin = true, *h.GiaMin
		r.GiaMax = *h.GiaMin
		if h.GiaMax != nil {
			r.GiaMax = *h.GiaMax
		}
	}
	if h.Lich != nil {
		r.GioRo, r.MoO = true, MoO(*h.Lich)
	}
	text := h.VanBan
	if len(t.MonChinh) > 0 {
		text = strings.TrimSpace(text + "\nMón chính: " + strings.Join(t.MonChinh, ", "))
	}
	if strings.TrimSpace(text) == "" {
		return nil, nil
	}
	r.Text = nfc(text)
	if len(r.Text) > MaxByteVanBan {
		return nil, fmt.Errorf("%w: %s holds %d bytes", ErrQuaDai, h.ID, len(r.Text))
	}
	r.ContentHash = hashNoiDung(chunker, r.TieuDe, r.Text)
	return []Hang{r}, nil
}

// DoanSoTay builds one row per manual section.
func DoanSoTay(ds []huongdan.Doan, chunker string) []Hang {
	out := make([]Hang, 0, len(ds))
	for _, d := range ds {
		text := d.TieuDe + "\n" + d.Chu
		r := Hang{ChunkID: d.ID, DocID: d.ID, TieuDe: nfc(d.TieuDeMan + " — " + d.TieuDe), Text: nfc(catRune(text, maxRuneSoTay)),
			Chunker: chunker}
		r.ContentHash = hashNoiDung(chunker, r.TieuDe, r.Text)
		out = append(out, r)
	}
	return out
}

// hashNoiDung is a row's content hash: the embedding cache key's text part
// and the reconciliation's change test.
func hashNoiDung(chunker, title, text string) string {
	sum := sha256.Sum256([]byte(chunker + "\x00" + title + "\x00" + text))
	return hex.EncodeToString(sum[:])
}

// Package hybrid is the truyhoi.Retriever adapter «hybrid»: one query
// embedding (under the turn's MaxEmbedCallsPerTurn), a Milvus search that
// fuses the dense leg and the sparse leg (BM25 by default, MILCO behind its
// flag) by RRF with the hard constraints in the filter expression, then a
// second check of every hit against its live PostgreSQL rows, then the
// optional reranker.
//
// What it decides and what it does not (docs/architecture/03, «Luật không
// heuristic»): the hard constraints arrive as ids, an instant and whole
// đồng in truyhoi.Cung, extracted by the model; this package turns them
// into a filter and never widens it. A shortfall returns fewer items. The
// query text is embedded and scored, never parsed for constraints. The soft
// preferences (Mem) are closed ids too; their labels join the query text so
// they rank, never filter.
//
// Fail closed: a hit whose live row is missing, tombstoned or breaks a hard
// constraint is dropped and counted (KiemLaiLoai, and BiLoai under the
// constraint it broke); a PostgreSQL error fails the retrieval rather than
// showing unchecked hits. The live row is read by package thuoctinh from
// the tables the ingest (rag/nap) writes -- places, place_enrichments,
// rag_tombstones -- with the ingest's own rule (nap.ApDung), so the index
// and the re-check are two copies of one truth. A leg that is down is
// reported (NoVector, NoSparse, NoRerank) and the other legs answer.
//
// The index holds chunks (a place's profile and its reviews); a retrieval
// answers places: hits are folded to their document, the best-ranked chunk
// standing for it.
package hybrid

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"

	"mobile/services/core/internal/aiharness/nhung"
	"mobile/services/core/internal/aiharness/truyhoi"
	"mobile/services/core/internal/domain/tuvung"
	"mobile/services/core/internal/thuoctinh"
	"mobile/services/core/internal/vectordb"
)

// MacDinhK is how many items a retrieval returns when the request says 0.
const MacDinhK = 10

// MaxCauRune bounds the text a retrieval embeds and scores: the model's
// query and the soft-preference labels together. The request's own Cau is
// not bounded upstream (truyhoi.YeuCau.Kiem checks structure only), and an
// embedding call and a BM25 query both cost with length.
const MaxCauRune = 512

// ErrNguon: a source this adapter does not serve (memory goes through
// trinho, group history through its own store).
var ErrNguon = errors.New("hybrid: source not served by the hybrid adapter")

// ErrKhongNhanh: both legs are down; the caller may fall back to the
// lexical index and flag truyhoi.LexicalOnly.
var ErrKhongNhanh = errors.New("hybrid: neither the dense nor the sparse leg is available")

// DocHuongDan reads the evidence fields of manual chunks by id from their
// source (the huongdan package), for the manual source.
type DocHuongDan func(ctx context.Context, ids []string) (map[string]map[string]string, error)

// DocSong reads the live rows of place ids, the re-check's truth. An
// interface, not a function value, so the static read gate (aigate) follows
// the retrieval path into the tables it reads.
type DocSong interface {
	DocSong(ctx context.Context, ids []string) (map[string]thuoctinh.Hang, error)
}

// DocSongHam adapts a function (tests, and thuoctinh.DocTu over a pool).
type DocSongHam func(ctx context.Context, ids []string) (map[string]thuoctinh.Hang, error)

// DocSong calls f.
func (f DocSongHam) DocSong(ctx context.Context, ids []string) (map[string]thuoctinh.Hang, error) {
	return f(ctx, ids)
}

// DemBiLoai counts, per hard constraint, the live places of the
// destination it removes (vectordb.Milvus.DemBiLoai bound to the alias).
type DemBiLoai func(ctx context.Context, l vectordb.LocCung) (map[truyhoi.RangBuoc]int, error)

// Kho is the adapter. Build it per turn around the turn's counted embedder
// (nhung.Dem) and reranker (rerank.Dem); the index, sparse adapter and
// database are shared.
type Kho struct {
	Nhung nhung.Nhung
	Index vectordb.TimKiem
	Thua  vectordb.Thua
	// DocSong reads the live rows of hit ids (aidoc.ThuocTinhSong: thuoctinh
	// in a READ ONLY transaction); the re-check is the last word on every
	// hit.
	DocSong DocSong
	// TenDiaDiem and TenHuongDan are the aliases searched.
	TenDiaDiem  string
	TenHuongDan string
	// Rerank is optional; nil means truyhoi.Passthrough and NoRerank.
	Rerank truyhoi.Reranker
	// BiLoai is optional; nil counts only the re-check's drops.
	BiLoai DemBiLoai
	// TrongSo are the fusion weights (cmd/core passes the ingest's
	// committed ones, rag/nap cauhinh.json «hop», so retrieval serves with
	// what the gate measured); nil is vectordb.MacDinhTrongSo.
	TrongSo *vectordb.TrongSo
	// HuongDan is required for the manual source.
	HuongDan DocHuongDan

	// KiemLaiLoai counts the hits the PostgreSQL re-check dropped over this
	// Kho's life (content-free). Atomic: one Kho serves every turn.
	KiemLaiLoai atomic.Int64
}

// DuPhong answers from Chinh, and from Phu when Chinh has no leg at all
// (ErrKhongNhanh: neither the dense nor the sparse leg could run) -- the
// lexical index then answers and says so itself (truyhoi.LexicalOnly).
// Every other error of Chinh stands: a failed re-check is never replaced by
// an unchecked answer.
type DuPhong struct {
	Chinh, Phu truyhoi.Retriever
}

var _ truyhoi.Retriever = DuPhong{}

// Tim answers one retrieval.
func (d DuPhong) Tim(ctx context.Context, y truyhoi.YeuCau) (truyhoi.KetQuaTruyHoi, error) {
	kq, err := d.Chinh.Tim(ctx, y)
	if errors.Is(err, ErrKhongNhanh) && d.Phu != nil {
		return d.Phu.Tim(ctx, y)
	}
	return kq, err
}

var _ truyhoi.Retriever = (*Kho)(nil)

// cauXepHang is the text the legs rank with: the model's query, then the
// labels of the soft preferences it picked (closed ids), so they rank.
func cauXepHang(y truyhoi.YeuCau) string {
	parts := []string{y.Cau}
	for _, id := range y.Mem.LoaiCho {
		parts = append(parts, tuvung.LoaiCho.Nhan(id))
	}
	for _, id := range y.Mem.KhiChat {
		parts = append(parts, tuvung.KhiChat.Nhan(id))
	}
	if y.Mem.KhuVuc != "" {
		parts = append(parts, y.Mem.KhuVuc)
	}
	s := strings.Join(parts, " ")
	if r := []rune(s); len(r) > MaxCauRune {
		s = string(r[:MaxCauRune])
	}
	return s
}

// Tim answers one retrieval.
func (k *Kho) Tim(ctx context.Context, y truyhoi.YeuCau) (truyhoi.KetQuaTruyHoi, error) {
	var kq truyhoi.KetQuaTruyHoi
	if err := y.Kiem(); err != nil {
		return kq, err
	}
	var ten string
	var kho vectordb.Kho
	switch y.Nguon {
	case truyhoi.Places:
		ten, kho = k.TenDiaDiem, vectordb.KhoDiaDiem
	case truyhoi.Manual:
		if k.HuongDan == nil {
			return kq, fmt.Errorf("%w: no manual reader configured", ErrNguon)
		}
		ten, kho = k.TenHuongDan, vectordb.KhoHuongDan
	default:
		return kq, fmt.Errorf("%w: %s", ErrNguon, y.Nguon)
	}
	loc, err := vectordb.TuCung(y.Cung)
	if err != nil {
		return kq, err
	}
	if kho == vectordb.KhoHuongDan {
		// The manual carries no place constraint; any set is a caller bug,
		// refused rather than ignored.
		if y.Cung.DiemDenID != "" || len(y.Cung.DiUng) > 0 || len(y.Cung.AnKieng) > 0 || y.Cung.MoLuc != nil || y.Cung.MoTrong != nil || y.Cung.NganSachVND != nil {
			return kq, fmt.Errorf("%w: hard constraints on the manual", truyhoi.ErrYeuCau)
		}
	}
	n := y.K
	if n == 0 {
		n = MacDinhK
	}
	text := cauXepHang(y)

	// Chunks, not places, come back: ask for more so n places survive the
	// fold and the re-check.
	req := vectordb.YeuCauTim{Ten: ten, Kho: kho, Loc: loc, K: truyhoi.MaxK, TrongSo: k.TrongSo}
	if vs, err := k.Nhung.Nhung(ctx, []string{text}, nhung.CauHoi); err == nil && len(vs) == 1 {
		req.Dense = vs[0]
	} else {
		kq.Degraded = append(kq.Degraded, truyhoi.NoVector)
	}
	if k.Thua != nil {
		if q, err := k.Thua.TruyVan(ctx, text); err == nil {
			req.Thua = &q
		} else {
			kq.Degraded = append(kq.Degraded, truyhoi.NoSparse)
		}
	} else {
		kq.Degraded = append(kq.Degraded, truyhoi.NoSparse)
	}
	if req.Dense == nil && req.Thua == nil {
		return kq, ErrKhongNhanh
	}
	hits, err := k.Index.Tim(ctx, req)
	if err != nil {
		return kq, err
	}
	hits = theoDoc(hits)

	kq.BiLoai = map[truyhoi.RangBuoc]int{}
	if kho == vectordb.KhoDiaDiem && k.BiLoai != nil {
		counts, err := k.BiLoai(ctx, loc)
		if err != nil {
			return truyhoi.KetQuaTruyHoi{}, err
		}
		for rb, c := range counts {
			kq.BiLoai[rb] += c
		}
	}

	var bc []truyhoi.BangChung
	switch kho {
	case vectordb.KhoDiaDiem:
		bc, err = k.kiemLai(ctx, hits, loc, &kq)
	case vectordb.KhoHuongDan:
		bc, err = k.huongDan(ctx, hits)
	}
	if err != nil {
		return truyhoi.KetQuaTruyHoi{}, err
	}

	if k.Rerank == nil {
		kq.Degraded = append(kq.Degraded, truyhoi.NoRerank)
		bc, _ = truyhoi.Passthrough{}.XepLai(ctx, y.Cau, bc, n)
	} else {
		out, err := k.Rerank.XepLai(ctx, y.Cau, bc, n)
		if err != nil {
			// Whatever a failed reranker returned, the answer is the RRF
			// order it was given.
			kq.Degraded = append(kq.Degraded, truyhoi.NoRerank)
			out, _ = truyhoi.Passthrough{}.XepLai(ctx, y.Cau, bc, n)
		}
		if !hopLe(out, bc, n) {
			return truyhoi.KetQuaTruyHoi{}, errors.New("hybrid: the reranker added or duplicated evidence")
		}
		bc = out
	}
	kq.BangChung = bc
	return kq, nil
}

// theoDoc folds chunk hits to their documents, in rank order: a document's
// best-ranked chunk stands for it (its score, its index version).
func theoDoc(hits []vectordb.Trung) []vectordb.Trung {
	seen := map[string]bool{}
	out := make([]vectordb.Trung, 0, len(hits))
	for _, h := range hits {
		d := h.DocID
		if d == "" {
			d = h.ID
		}
		if seen[d] {
			continue
		}
		seen[d] = true
		h.ID = d
		out = append(out, h)
	}
	return out
}

// hopLe: out is at most n items, each one of in, none twice (a reranker
// never adds an item).
func hopLe(out, in []truyhoi.BangChung, n int) bool {
	if len(out) > n || len(out) > len(in) {
		return false
	}
	have := map[string]bool{}
	for _, b := range in {
		have[b.ID] = true
	}
	seen := map[string]bool{}
	for _, b := range out {
		if !have[b.ID] || seen[b.ID] {
			return false
		}
		seen[b.ID] = true
	}
	return true
}

// kiemLai hydrates the hits from their live rows and drops, and counts,
// every one that is gone or breaks a hard constraint.
func (k *Kho) kiemLai(ctx context.Context, hits []vectordb.Trung, loc vectordb.LocCung, kq *truyhoi.KetQuaTruyHoi) ([]truyhoi.BangChung, error) {
	ids := make([]string, len(hits))
	for i, h := range hits {
		ids[i] = h.ID
	}
	rows, err := k.DocSong.DocSong(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("hybrid: re-check: %w", err)
	}
	out := make([]truyhoi.BangChung, 0, len(hits))
	for _, h := range hits {
		row, ok := rows[h.ID]
		if !ok {
			k.KiemLaiLoai.Add(1)
			continue
		}
		if dat, rb := loc.Dat(row.ThuocTinh); !dat {
			k.KiemLaiLoai.Add(1)
			if rb != "" {
				kq.BiLoai[rb]++
			}
			continue
		}
		f := map[string]string{"ten": row.Ten, "loai": row.Loai, "diem_den": row.ThuocTinh.DiemDen}
		if row.DiaChi != "" {
			f["dia_chi"] = row.DiaChi
		}
		if row.GiaMinVND != nil {
			f["gia_min_vnd"] = strconv.FormatInt(*row.GiaMinVND, 10)
		}
		if row.GiaMaxVND != nil {
			f["gia_max_vnd"] = strconv.FormatInt(*row.GiaMaxVND, 10)
		}
		if row.Gio != "" {
			f["gio"] = row.Gio
		}
		// Unknown and not asked about: kept, and said (docs/architecture/03
		// §8.4). Unknown and asked about never reaches here: Dat refused it.
		if co := row.ChuaRo(); len(co) > 0 {
			f["chua_ro"] = strings.Join(co, ",")
		}
		out = append(out, truyhoi.BangChung{ID: h.ID, Nguon: truyhoi.Places, Diem: h.Diem, Truong: f,
			PhienBanChiMuc: "v" + strconv.FormatInt(h.PhienBan, 10)})
	}
	return out, nil
}

func (k *Kho) huongDan(ctx context.Context, hits []vectordb.Trung) ([]truyhoi.BangChung, error) {
	ids := make([]string, len(hits))
	for i, h := range hits {
		ids[i] = h.ID
	}
	fields, err := k.HuongDan(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]truyhoi.BangChung, 0, len(hits))
	for _, h := range hits {
		f, ok := fields[h.ID]
		if !ok {
			k.KiemLaiLoai.Add(1)
			continue
		}
		out = append(out, truyhoi.BangChung{ID: h.ID, Nguon: truyhoi.Manual, Diem: h.Diem, Truong: f,
			PhienBanChiMuc: "v" + strconv.FormatInt(h.PhienBan, 10)})
	}
	return out, nil
}

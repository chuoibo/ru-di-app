// Package napkho is the ingestion pipeline's vector store (rag/nap.KhoVector)
// over vectordb: the same client constructor (vectordb.Ket, telemetry off
// through a non-nil config), the same collection schema, analyzers and
// indexes, the same alias names and alias__vN physical names, the same
// hard-constraint expression (vectordb.LocCung) and the same weighted RRF
// that retrieval (package hybrid) reads and searches with. One schema, one
// writer: rag/nap decides what a row holds; this package is the only place
// it becomes a Milvus row.
//
// It lives outside internal/rag because rag may not import an engine
// package (aigate), and vectordb speaks the engine's embedding and
// retrieval types.
package napkho

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/milvus-io/milvus/client/v3/entity"

	"mobile/services/core/internal/aiharness/nhung"
	"mobile/services/core/internal/domain/tuvung"
	"mobile/services/core/internal/rag/nap"
	"mobile/services/core/internal/vectordb"
)

// Kho is nap.KhoVector over a vectordb.Milvus. The Milvus's prefix (tests
// only) is applied to every collection and alias name nap passes.
type Kho struct {
	M *vectordb.Milvus
}

var _ nap.KhoVector = (*Kho)(nil)

// Mo connects with vectordb's one constructor. The ingest reads at Strong
// consistency: every read it makes (reconciliation, the gate's probes and
// golden run) checks a write it just made.
func Mo(ctx context.Context, c vectordb.Config) (*Kho, error) {
	m, err := vectordb.Ket(ctx, c)
	if err != nil {
		return nil, err
	}
	m.NhatQuan = entity.ClStrong
	return &Kho{M: m}, nil
}

// Dong closes the connection.
func (k *Kho) Dong(ctx context.Context) error { return k.M.Dong(ctx) }

// ErrLuocDo: the pipeline's configuration names another schema revision,
// dimensionality, slot width, RRF constant or alias than vectordb declares.
var ErrLuocDo = errors.New("napkho: the ingestion configuration does not match vectordb's schema")

// KiemKhop checks the constants the two packages must agree on: nap's
// aliases are vectordb's, its slot width is vectordb's, and its schema
// revision, dimensionality and RRF constant are vectordb's.
func KiemKhop(cfg nap.CauHinh) error {
	switch {
	case cfg.LuocDo != vectordb.PhienBanLuocDo:
		return fmt.Errorf("%w: schema %q, vectordb %q", ErrLuocDo, cfg.LuocDo, vectordb.PhienBanLuocDo)
	case cfg.Dense.Dims != nhung.Dims:
		return fmt.Errorf("%w: %d dims, vectordb %d", ErrLuocDo, cfg.Dense.Dims, nhung.Dims)
	case cfg.Hop.RRFK != vectordb.RRFK:
		return fmt.Errorf("%w: rrf_k %d, vectordb %d", ErrLuocDo, cfg.Hop.RRFK, vectordb.RRFK)
	case nap.PhutMoiO != vectordb.PhutMoiSlot || nap.SoO != vectordb.SoSlotTuan:
		return fmt.Errorf("%w: slot width", ErrLuocDo)
	case nap.Alias(nap.CorpusQuan) != vectordb.AliasDiaDiem || nap.Alias(nap.CorpusSoTay) != vectordb.AliasHuongDan:
		return fmt.Errorf("%w: alias names", ErrLuocDo)
	}
	return nil
}

var tenHopLe = regexp.MustCompile(`^(rd_(places|manual)(__v[1-9][0-9]*)?|rd_eval__[0-9a-f]{12})$`)

// kho is the logical collection a name belongs to, and its version (0 for
// an alias or the eval collection).
func kho(ten string) (vectordb.Kho, int64, error) {
	if !tenHopLe.MatchString(ten) {
		return "", 0, fmt.Errorf("%w: %s", vectordb.ErrTen, ten)
	}
	k := vectordb.KhoDiaDiem
	if strings.HasPrefix(ten, "rd_manual") {
		k = vectordb.KhoHuongDan
	}
	var v int64
	if _, s, ok := strings.Cut(ten, "__v"); ok {
		v, _ = strconv.ParseInt(s, 10, 64)
	}
	return k, v, nil
}

func (k *Kho) ten(ten string) string { return k.M.TienTo + ten }

func (k *Kho) TaoCollection(ctx context.Context, ten string, ld nap.LuocDo) error {
	kk, _, err := kho(ten)
	if err != nil {
		return err
	}
	if ld.Ban != vectordb.PhienBanLuocDo || ld.Dims != nhung.Dims {
		return fmt.Errorf("%w: %+v", ErrLuocDo, ld)
	}
	if (ld.Corpus == nap.CorpusQuan) != (kk == vectordb.KhoDiaDiem) {
		return fmt.Errorf("%w: corpus %s under %s", ErrLuocDo, ld.Corpus, ten)
	}
	return k.M.TaoCollection(ctx, kk, k.ten(ten))
}

func (k *Kho) XoaCollection(ctx context.Context, ten string) error {
	return k.M.XoaTen(ctx, k.ten(ten))
}

func (k *Kho) DanhSachCollection(ctx context.Context) ([]string, error) {
	names, err := k.M.DanhSachTen(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(names))
	for _, n := range names {
		out = append(out, strings.TrimPrefix(n, k.M.TienTo))
	}
	return out, nil
}

// ThuocTinh is the index form of a row's hard-filter attributes: allergens
// not established become exactly [khong_ro] (out for any allergy), an
// unknown price vectordb.GiaKhongRo (out under a budget), unknown hours no
// open slot (out under a time or a window).
func ThuocTinh(r nap.Hang) vectordb.ThuocTinh {
	t := vectordb.ThuocTinh{DiemDen: r.DiemDen, AnKieng: slices.Clone(r.AnKieng), GiaMinVND: vectordb.GiaKhongRo,
		GiaMaxVND: giaMax(r)}
	if r.DiUngRo {
		t.DiUng = slices.Clone(r.DiUng)
	} else {
		t.DiUng = []string{vectordb.KhongRo}
	}
	if r.GiaRo {
		t.GiaMinVND = r.GiaMin
	}
	if r.GioRo {
		t.OSlots = slices.Clone(r.MoO)
	}
	// Categories not classified yet are exactly [khong_ro]: out under a
	// category filter, never guessed.
	if len(r.DanhMuc) > 0 {
		t.DanhMuc = slices.Clone(r.DanhMuc)
	} else {
		t.DanhMuc = []string{vectordb.KhongRo}
	}
	return t
}

func giaMax(r nap.Hang) int64 {
	if !r.GiaRo {
		return vectordb.GiaKhongRo
	}
	return r.GiaMax
}

func (k *Kho) Upsert(ctx context.Context, ten string, rows []nap.Hang) error {
	kk, v, err := kho(ten)
	if err != nil {
		return err
	}
	switch kk {
	case vectordb.KhoDiaDiem:
		out := make([]vectordb.HangDiaDiem, len(rows))
		for i, r := range rows {
			out[i] = vectordb.HangDiaDiem{ID: r.ChunkID, DocID: r.DocID, Dense: r.Dense, Text: r.Text,
				ContentHash: r.ContentHash, EmbedModel: r.DenseModel, ThuocTinh: ThuocTinh(r), GiaMaxVND: giaMax(r), PhienBan: v}
		}
		return k.M.GhiDiaDiem(ctx, k.ten(ten), out)
	default:
		out := make([]vectordb.HangHuongDan, len(rows))
		for i, r := range rows {
			out[i] = vectordb.HangHuongDan{ID: r.ChunkID, DocID: r.DocID, Text: r.Text, ContentHash: r.ContentHash,
				EmbedModel: r.DenseModel, Dense: r.Dense, PhienBan: v}
		}
		return k.M.GhiHuongDan(ctx, k.ten(ten), out)
	}
}

func (k *Kho) XoaID(ctx context.Context, ten string, ids []string) error {
	if _, _, err := kho(ten); err != nil {
		return err
	}
	return k.M.XoaID(ctx, k.ten(ten), ids)
}

func (k *Kho) CapNhatThuocTinh(ctx context.Context, ten, docID string, r nap.Hang) (int, error) {
	if kk, _, err := kho(ten); err != nil || kk != vectordb.KhoDiaDiem {
		return 0, fmt.Errorf("%w: attributes of %s", vectordb.ErrTen, ten)
	}
	return k.M.CapNhatThuocTinh(ctx, k.ten(ten), docID, ThuocTinh(r), giaMax(r))
}

func (k *Kho) Dem(ctx context.Context, ten string) (int64, error) {
	return k.M.Dem(ctx, k.ten(ten), "", nil)
}

func khoaNap(ks []vectordb.KhoaHang) []nap.KhoaHang {
	out := make([]nap.KhoaHang, len(ks))
	for i, x := range ks {
		out[i] = nap.KhoaHang{ChunkID: x.ID, DocID: x.DocID, ContentHash: x.ContentHash, DenseModel: x.EmbedModel}
	}
	return out
}

func (k *Kho) LietKe(ctx context.Context, ten string) ([]nap.KhoaHang, error) {
	ks, err := k.M.LietKe(ctx, k.ten(ten))
	return khoaNap(ks), err
}

// LocCung is nap's filter in vectordb's form: allergens closed over their
// family, diets sorted, the slot and the window's slots, the budget.
func LocCung(l nap.Loc) vectordb.LocCung {
	out := vectordb.LocCung{DiemDen: l.DiemDen, Slot: l.O, Slots: slices.Clone(l.Khung), NganSachVND: l.NganSach}
	if len(l.DiUng) > 0 {
		out.DiUng = tuvung.MoRongDiUng(l.DiUng)
	}
	if len(l.AnKieng) > 0 {
		out.AnKieng = slices.Clone(l.AnKieng)
		slices.Sort(out.AnKieng)
		out.AnKieng = slices.Compact(out.AnKieng)
	}
	return out
}

func (k *Kho) LocHang(ctx context.Context, ten string, l nap.Loc) ([]nap.KhoaHang, error) {
	ks, err := k.M.LocDiaDiem(ctx, k.ten(ten), LocCung(l))
	return khoaNap(ks), err
}

func (k *Kho) TimLai(ctx context.Context, ten string, tv nap.TruyVan) ([]nap.Trung, error) {
	kk, _, err := kho(ten)
	if err != nil {
		return nil, err
	}
	if tv.RRFK != 0 && tv.RRFK != vectordb.RRFK {
		return nil, fmt.Errorf("%w: rrf_k %d", ErrLuocDo, tv.RRFK)
	}
	kMax := tv.K
	if kMax <= 0 || kMax > 50 {
		kMax = 50
	}
	w := vectordb.TrongSo{Dense: tv.TrongSo.Dense, BM25: tv.TrongSo.BM25}
	y := vectordb.YeuCauTim{Ten: k.ten(ten), Kho: kk, Loc: LocCung(tv.Loc), K: kMax, UngVien: tv.KMoiNhanh, TrongSo: &w}
	if len(tv.Dense) > 0 {
		y.Dense = tv.Dense
	}
	if len(nap.Tokens(tv.Chu)) > 0 {
		y.Thua = &vectordb.ThuaTruyVan{Text: nhung.ChuanNFC(tv.Chu)}
	}
	hits, err := k.M.Tim(ctx, y)
	if errors.Is(err, vectordb.ErrKhongCoNhanh) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := make([]nap.Trung, len(hits))
	for i, h := range hits {
		out[i] = nap.Trung{ChunkID: h.ID, DocID: h.DocID, Diem: h.Diem}
	}
	return out, nil
}

func (k *Kho) DatAlias(ctx context.Context, alias, ten string) error {
	if _, _, err := kho(alias); err != nil {
		return err
	}
	if _, _, err := kho(ten); err != nil {
		return err
	}
	return k.M.DatAliasTen(ctx, k.ten(alias), k.ten(ten))
}

func (k *Kho) MoTaAlias(ctx context.Context, alias string) (string, error) {
	got, err := k.M.MoTaAliasTen(ctx, k.ten(alias))
	return strings.TrimPrefix(got, k.M.TienTo), err
}

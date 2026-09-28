package crag

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/tools"
	"mobile/services/core/internal/aiharness/truyhoi"
)

// HanTruyHoi bounds one retrieval with its rerank (design 04 §5.8: 3 s for
// a whole search).
const HanTruyHoi = 3 * time.Second

// TopN is how many reranked items one retrieval keeps as evidence (design
// 04 §5.8: at most 8 places).
const TopN = 8

// DuTruCham is how many model calls must remain for the grader to run: the
// grader itself and the whole answer cycle after it (the answer, its
// verifier, the one regeneration and its verifier; llm.KeHoach). The
// grader is the sufficiency judgement and the first step cut when the
// budget runs short (the worst-case plan's cut order): below this the
// grade is skipped and the answer step works from the first retrieval,
// with its regeneration still possible; the verifier still guards what is
// released.
var DuTruCham = llm.SauBuoc(llm.DuongTruyHoi, llm.BuocCham)

// ThuTuNoi is the order soft constraints may be relaxed in (design 04 §5.5):
// vibe, then kind of place, then area. A relaxation may drop a later one
// only when every earlier one is dropped too or was not set.
var ThuTuNoi = []truyhoi.RangBuoc{truyhoi.RBKhiChat, truyhoi.RBLoaiCho, truyhoi.RBKhuVuc}

// ErrThuTuNoi: the grader asked to relax a soft constraint out of order.
var ErrThuTuNoi = errors.New("crag: relaxation out of the allowed order")

// ErrCungDoi is the invariant a corrective round must keep: the hard
// constraints of the round's request equal the first request's. It can only
// fire on a bug; the round is refused rather than run.
var ErrCungDoi = errors.New("crag: a corrective round changed a hard constraint")

// ErrNguon: the retriever failed. The caller answers the model with the
// tool error loi_nguon.
var ErrNguon = errors.New("crag: retrieval failed")

// KiemThuTu checks d's relaxation against y and ThuTuNoi.
func KiemThuTu(y truyhoi.YeuCau, d DanhGia) error {
	bo := map[truyhoi.RangBuoc]bool{}
	for _, r := range d.NoiLong {
		bo[r] = true
	}
	for i, r := range ThuTuNoi {
		if !bo[r] {
			continue
		}
		for _, truoc := range ThuTuNoi[:i] {
			if !bo[truoc] && coMem(y.Mem, truoc) {
				return fmt.Errorf("%w: %s before %s", ErrThuTuNoi, r, truoc)
			}
		}
	}
	return nil
}

func coMem(m truyhoi.Mem, r truyhoi.RangBuoc) bool {
	switch r {
	case truyhoi.RBKhiChat:
		return len(m.KhiChat) > 0
	case truyhoi.RBLoaiCho:
		return len(m.LoaiCho) > 0
	case truyhoi.RBKhuVuc:
		return m.KhuVuc != ""
	}
	return false
}

// NganSachXepLai is one turn's reranker budget, llm.MaxRerankCallsPerTurn
// calls across every retrieval of the turn. Safe for concurrent use.
type NganSachXepLai struct {
	mu sync.Mutex
	n  int
}

// giu takes one reranker call; false when the turn has spent them all.
func (g *NganSachXepLai) giu() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.n >= llm.MaxRerankCallsPerTurn {
		return false
	}
	g.n++
	return true
}

// SoGoi is how many reranker calls the turn made.
func (g *NganSachXepLai) SoGoi() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.n
}

// BoPhan are the loop's parts. XepLai nil means truyhoi.Passthrough.
// NganSach is the turn's reranker budget, shared by every TruyHoi of the
// turn; nil gives this call a budget of its own.
type BoPhan struct {
	Tim      truyhoi.Retriever
	XepLai   truyhoi.Reranker
	Cham     Cham
	NganSach *NganSachXepLai
}

// Buoc is what the corrective round did.
type Buoc string

const (
	KhongSua  Buoc = ""
	DaNoi     Buoc = "noi_long"
	DaVietLai Buoc = "viet_lai"
)

// KetQua is the loop's result.
type KetQua struct {
	// YeuCau is the request the evidence answers (the corrected one after a
	// round). Its Cung always equals the first request's.
	YeuCau    truyhoi.YeuCau
	BangChung []truyhoi.BangChung
	Degraded  []truyhoi.CoSuyGiam
	BiLoai    map[truyhoi.RangBuoc]int
	// DanhGia is the grader's verdict on the first retrieval; nil when the
	// grade was skipped (BoCham) or refused (ChamHong).
	DanhGia *DanhGia
	// Vong is how many corrective rounds ran (0 or MaxCorrectiveRounds).
	Vong int
	Buoc Buoc
	// DaNoi are the soft constraints the round dropped.
	NoiLong []truyhoi.RangBuoc
	// BoCham: too few model calls were left to grade.
	BoCham bool
	// ChamHong: the grader's output was refused (structure, order).
	ChamHong bool
	// SoXepLai is how many reranker calls this loop ran.
	SoXepLai int
	// SoTruyHoi is how many retrievals ran.
	SoTruyHoi int
}

// TruyHoi runs retrieve → rerank → record → grade → at most one corrective
// round, for tool cong (search_places or search_app_manual). Every
// retrieval is a tool call counted on sc and its evidence recorded there;
// the grader's call goes through dem. Go reads no words here: the grader's
// structured verdict chooses the move, Go checks it against the closed sets
// and the relaxation order, applies it with SuaYeuCau, and asserts the hard
// constraints did not move.
func TruyHoi(ctx context.Context, y truyhoi.YeuCau, bp BoPhan, cong tools.Ten, sc *tools.SoCai, dem *llm.Dem) (KetQua, error) {
	if bp.XepLai == nil {
		bp.XepLai = truyhoi.Passthrough{}
	}
	if bp.NganSach == nil {
		bp.NganSach = &NganSachXepLai{}
	}
	kq := KetQua{YeuCau: y}
	bc, dg, biLoai, err := motLan(ctx, y, bp, cong, sc, &kq)
	if err != nil {
		return kq, err
	}
	kq.BangChung, kq.Degraded, kq.BiLoai = bc, dg, biLoai
	if bp.Cham == nil || dem.ConLai() < DuTruCham {
		kq.BoCham = true
		return kq, nil
	}
	d, err := bp.Cham.DanhGia(ctx, Vao{YeuCau: y, BangChung: bc, BiLoai: biLoai, BiDanh: biDanhSoCai(sc)}, dem)
	if err != nil {
		if errors.Is(err, ErrCauTruc) {
			kq.ChamHong = true
			return kq, nil
		}
		return kq, err
	}
	if err := KiemThuTu(y, d); err != nil {
		kq.ChamHong = true
		return kq, nil
	}
	kq.DanhGia = &d
	y2, co, err := SuaYeuCau(y, d, kq.Vong+1)
	if err != nil {
		// A hard name in NoiLong (Mem.Bo refuses it) or the round budget:
		// no round, the first evidence stands.
		kq.ChamHong = true
		return kq, nil
	}
	if !co || reflect.DeepEqual(y2, y) {
		return kq, nil
	}
	if !reflect.DeepEqual(y2.Cung, y.Cung) {
		return kq, ErrCungDoi
	}
	bc2, dg2, biLoai2, err := motLan(ctx, y2, bp, cong, sc, &kq)
	if err != nil {
		if errors.Is(err, tools.ErrHetLuotGoi) {
			return kq, nil
		}
		return kq, err
	}
	kq.Vong++
	if len(d.NoiLong) > 0 {
		kq.Buoc, kq.NoiLong = DaNoi, append([]truyhoi.RangBuoc(nil), d.NoiLong...)
	} else {
		kq.Buoc = DaVietLai
	}
	// The round's evidence replaces the first when it found anything; an
	// empty round leaves the first evidence standing (both are on the
	// ledger either way).
	if len(bc2) > 0 {
		kq.YeuCau, kq.BangChung, kq.Degraded, kq.BiLoai = y2, bc2, dg2, biLoai2
	}
	return kq, nil
}

// motLan is one retrieval: a counted tool call, the retriever under
// HanTruyHoi, the reranker (or Passthrough past MaxRerankCallsPerTurn, or on
// its failure, flagged NoRerank), the evidence recorded on sc.
func motLan(ctx context.Context, y truyhoi.YeuCau, bp BoPhan, cong tools.Ten, sc *tools.SoCai, kq *KetQua) ([]truyhoi.BangChung, []truyhoi.CoSuyGiam, map[truyhoi.RangBuoc]int, error) {
	if err := y.Kiem(); err != nil {
		return nil, nil, nil, err
	}
	if err := sc.Giu(cong); err != nil {
		return nil, nil, nil, err
	}
	ctx, huy := context.WithTimeout(ctx, HanTruyHoi)
	defer huy()
	kq.SoTruyHoi++
	// This loop reranks (below, once over what the retriever merged from
	// every query): the retriever must not rerank the same candidates too.
	r, err := bp.Tim.Tim(truyhoi.HoanXepLai(ctx), y)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("%w: %w", ErrNguon, err)
	}
	dg := append([]truyhoi.CoSuyGiam(nil), r.Degraded...)
	var bc []truyhoi.BangChung
	_, laPass := bp.XepLai.(truyhoi.Passthrough)
	if laPass || !bp.NganSach.giu() {
		bc, _ = truyhoi.Passthrough{}.XepLai(ctx, y.Cau, r.BangChung, TopN)
		dg = themCo(dg, truyhoi.NoRerank)
	} else {
		kq.SoXepLai++
		bc, err = bp.XepLai.XepLai(ctx, y.Cau, r.BangChung, TopN)
		if err != nil || !conCua(bc, r.BangChung) {
			bc, _ = truyhoi.Passthrough{}.XepLai(ctx, y.Cau, r.BangChung, TopN)
			dg = themCo(dg, truyhoi.NoRerank)
		}
	}
	sc.Ghi(cong, bc)
	return bc, dg, r.BiLoai, nil
}

// conCua reports whether every reranked item came from the retrieval: a
// reranker may reorder and cut, never add (truyhoi.Reranker).
func conCua(xep, goc []truyhoi.BangChung) bool {
	co := make(map[string]bool, len(goc))
	for _, b := range goc {
		co[b.ID] = true
	}
	for _, b := range xep {
		if !co[b.ID] {
			return false
		}
	}
	return true
}

func themCo(dg []truyhoi.CoSuyGiam, c truyhoi.CoSuyGiam) []truyhoi.CoSuyGiam {
	for _, x := range dg {
		if x == c {
			return dg
		}
	}
	return append(dg, c)
}

// biDanhSoCai shows evidence under the ledger's aliases.
func biDanhSoCai(sc *tools.SoCai) func(int, truyhoi.BangChung) string {
	return func(i int, b truyhoi.BangChung) string {
		if a, ok := sc.BiDanh(b.ID); ok {
			return a
		}
		return BiDanhCham(i, b)
	}
}

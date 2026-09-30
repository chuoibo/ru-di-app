package hybrid

import (
	"context"
	"errors"
	"fmt"
	"time"

	"mobile/services/core/internal/aiharness/truyhoi"
	"mobile/services/core/internal/domain/tuvung"
)

// TimQuanVao is a place search in plain terms: the words to rank by and
// the hard filters to hold. It is the entry point every caller in core
// reuses (a route, a bot tool, an eval, a script) instead of assembling a
// truyhoi.YeuCau by hand. Every filter is a Milvus filter expression on
// both legs (vectordb.LocCung) and is checked again on the live row; none
// is ever relaxed. An empty field is no filter.
type TimQuanVao struct {
	// Cau is what to look for, in the person's words; CauCoDau an optional
	// diacritics-restored form (the dense leg and the reranker read it).
	Cau      string
	CauCoDau string
	// DiemDen is a destination id (d-tinh-79, …).
	DiemDen string
	// DanhMuc are category ids (tuvung.DanhMuc: quan_an, cafe, …): a place
	// qualifies when it has ANY of them. NhomUI is an app UI category
	// (catalog.Categories id: quan-an-local, cafe, di-choi-dem, vui-choi),
	// expanded to its DanhMuc ids and joined to DanhMuc.
	DanhMuc []string
	NhomUI  string
	// GiaTuVND and GiaDenVND bound the spend per person in whole đồng: a
	// place qualifies when its price band overlaps [GiaTuVND, GiaDenVND]. A
	// place with no known price is out when either is set.
	GiaTuVND  *int64
	GiaDenVND *int64
	// MoLuc: open at that instant. MoTu/MoDen: open at some point of the
	// window [MoTu, MoDen). A place with no known hours is out.
	MoLuc       *time.Time
	MoTu, MoDen *time.Time
	// DiUng: allergens the person avoids (tuvung.DiUng ids, closed over
	// their families); a place whose allergens are unknown is out. AnKieng:
	// diets the place must declare (tuvung.AnKieng ids).
	DiUng   []string
	AnKieng []string
	// K is how many places to return (0: MacDinhK; at most truyhoi.MaxK).
	K int
}

// ErrTimQuan: a search whose filters cannot be held as given.
var ErrTimQuan = errors.New("hybrid: invalid place search")

// YeuCau is v as the retrieval contract, checked: an unknown UI category
// or a window with an end not after its start is refused, never dropped.
func (v TimQuanVao) YeuCau() (truyhoi.YeuCau, error) {
	c := truyhoi.Cung{DiemDenID: v.DiemDen, DiUng: v.DiUng, AnKieng: v.AnKieng,
		MoLuc: v.MoLuc, NganSachVND: v.GiaDenVND, GiaTuVND: v.GiaTuVND,
		DanhMuc: append([]string(nil), v.DanhMuc...)}
	if v.NhomUI != "" {
		ids := tuvung.DanhMucCuaNhomUI(v.NhomUI)
		if len(ids) == 0 {
			return truyhoi.YeuCau{}, fmt.Errorf("%w: UI category %q", ErrTimQuan, v.NhomUI)
		}
		c.DanhMuc = append(c.DanhMuc, ids...)
	}
	switch {
	case v.MoTu != nil && v.MoDen != nil:
		if !v.MoDen.After(*v.MoTu) {
			return truyhoi.YeuCau{}, fmt.Errorf("%w: window end not after its start", ErrTimQuan)
		}
		c.MoTrong = &truyhoi.KhungMo{Tu: *v.MoTu, Den: *v.MoDen}
	case v.MoTu != nil || v.MoDen != nil:
		return truyhoi.YeuCau{}, fmt.Errorf("%w: a window needs both ends", ErrTimQuan)
	}
	y := truyhoi.YeuCau{Nguon: truyhoi.Places, Cau: v.Cau, CauCoDau: v.CauCoDau, Cung: c, K: v.K}
	if err := y.Kiem(); err != nil {
		return truyhoi.YeuCau{}, err
	}
	return y, nil
}

// TimQuan runs a place search on r (a *Kho, or a DuPhong over one): the
// places that hold every filter, ranked by the dense and BM25 legs fused
// with RRF (and the reranker when one is configured), each re-checked on its
// live row. Fewer than K results is a correct answer; BiLoai says how many
// candidates each filter removed.
func TimQuan(ctx context.Context, r truyhoi.Retriever, v TimQuanVao) (truyhoi.KetQuaTruyHoi, error) {
	y, err := v.YeuCau()
	if err != nil {
		return truyhoi.KetQuaTruyHoi{}, err
	}
	return r.Tim(ctx, y)
}

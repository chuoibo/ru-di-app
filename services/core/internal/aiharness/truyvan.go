package aiharness

import (
	"context"

	"mobile/services/core/internal/aiharness/hieu"
	"mobile/services/core/internal/aiharness/truyhoi"
)

// nhieuTruyVan runs every query the router wrote for one source, with the
// same request otherwise, and merges the results: round robin in the
// router's query order, an item kept at its first place, each filter count
// the largest one query saw (the queries share the filters, so the counts
// are of the same candidates), the degradation flags united. Pure
// bookkeeping over ids and counts: it reads no query. A request whose query
// is not the first one (the grader's rewrite in a corrective round) runs as
// it is, since the rewrite replaces the router's queries. Each query goes
// with its own diacritics-restored form (hieu.TruyVan).
type nhieuTruyVan struct {
	tim  truyhoi.Retriever
	caus []hieu.TruyVan
}

var _ truyhoi.Retriever = nhieuTruyVan{}

func (n nhieuTruyVan) Tim(ctx context.Context, y truyhoi.YeuCau) (truyhoi.KetQuaTruyHoi, error) {
	if len(n.caus) < 2 || y.Cau != n.caus[0].Cau {
		return n.tim.Tim(ctx, y)
	}
	kqs := make([]truyhoi.KetQuaTruyHoi, 0, len(n.caus))
	for _, c := range n.caus {
		y2 := y
		y2.Cau, y2.CauCoDau = c.Cau, c.CauCoDau
		kq, err := n.tim.Tim(ctx, y2)
		if err != nil {
			return truyhoi.KetQuaTruyHoi{}, err
		}
		kqs = append(kqs, kq)
	}
	return tronKetQua(kqs, y.K), nil
}

// tronKetQua merges kqs (see nhieuTruyVan), at most k items (k 0: every
// distinct item).
func tronKetQua(kqs []truyhoi.KetQuaTruyHoi, k int) truyhoi.KetQuaTruyHoi {
	var out truyhoi.KetQuaTruyHoi
	dai, tong := 0, 0
	for _, kq := range kqs {
		dai = max(dai, len(kq.BangChung))
		tong += len(kq.BangChung)
		for _, d := range kq.Degraded {
			if !coSuyGiam(out.Degraded, d) {
				out.Degraded = append(out.Degraded, d)
			}
		}
		for r, c := range kq.BiLoai {
			if out.BiLoai == nil {
				out.BiLoai = map[truyhoi.RangBuoc]int{}
			}
			out.BiLoai[r] = max(out.BiLoai[r], c)
		}
	}
	if k <= 0 {
		k = tong
	}
	seen := map[string]bool{}
	for i := 0; i < dai && len(out.BangChung) < k; i++ {
		for _, kq := range kqs {
			if i < len(kq.BangChung) && !seen[kq.BangChung[i].ID] && len(out.BangChung) < k {
				seen[kq.BangChung[i].ID] = true
				out.BangChung = append(out.BangChung, kq.BangChung[i])
			}
		}
	}
	return out
}

func coSuyGiam(ds []truyhoi.CoSuyGiam, d truyhoi.CoSuyGiam) bool {
	for _, x := range ds {
		if x == d {
			return true
		}
	}
	return false
}

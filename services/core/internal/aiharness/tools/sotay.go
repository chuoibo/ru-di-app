package tools

import (
	"context"

	"mobile/services/core/internal/aiharness/truyhoi"
	"mobile/services/core/internal/huongdan"
)

// SoTay is the app manual as a truyhoi.Retriever, for the retrieval path
// (crag, traloi) the router chose for app_help: the same passages, fields and
// index version search_app_manual returns. The query is the one the MODEL
// wrote (hieu.TruyVan); it only ranks passages, and Man is the screen the
// panel reported, which pins its own sections first. The manual has no hard
// constraints to apply.
type SoTay struct {
	Man string
}

var _ truyhoi.Retriever = SoTay{}

// Tim returns the manual passages for y.
func (s SoTay) Tim(ctx context.Context, y truyhoi.YeuCau) (truyhoi.KetQuaTruyHoi, error) {
	if y.Nguon != truyhoi.Manual {
		return truyhoi.KetQuaTruyHoi{}, truyhoi.ErrYeuCau
	}
	var out []truyhoi.BangChung
	for _, d := range huongdan.Tim(ctx, huongdan.Hoi{Cau: y.Cau, Man: s.Man, K: y.K}) {
		out = append(out, bangChungDoan(d))
	}
	return truyhoi.KetQuaTruyHoi{BangChung: out}, ctx.Err()
}

// DaChay are the tools the ledger counted a call of, each once, in registry
// order: what the turn's record names (obs.CacCongCu).
func (s *SoCai) DaChay() []Ten {
	var out []Ten
	for _, t := range Tens.Values() {
		if s.SoGoi(Ten(t)) > 0 {
			out = append(out, Ten(t))
		}
	}
	return out
}

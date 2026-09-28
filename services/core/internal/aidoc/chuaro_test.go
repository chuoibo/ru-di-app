package aidoc

import (
	"slices"
	"testing"
	"time"

	"mobile/services/core/internal/aiharness/truyhoi"
	"mobile/services/core/internal/rag"
	"mobile/services/core/internal/repo"
)

// One rule for unknown price and hours on both of the engine's retrievers
// (docs/architecture/03 §8.4): out under a hard constraint on it, kept and
// flagged otherwise.
func TestChuaRoMotLuat(t *testing.T) {
	ns := int64(100000)
	at := time.Date(2026, 9, 26, 19, 0, 0, 0, time.UTC)
	w := truyhoi.KhungMo{Tu: at, Den: at.Add(time.Hour)}
	gia := []string{rag.CoGiaChuaRo}
	gio := []string{rag.CoGioChuaRo}
	for _, c := range []struct {
		cung truyhoi.Cung
		co   []string
		giu  bool
	}{
		{truyhoi.Cung{}, gia, true},
		{truyhoi.Cung{}, gio, true},
		{truyhoi.Cung{NganSachVND: &ns}, gia, false},
		{truyhoi.Cung{NganSachVND: &ns}, gio, true},
		{truyhoi.Cung{MoLuc: &at}, gio, false},
		{truyhoi.Cung{MoTrong: &w}, gio, false},
		{truyhoi.Cung{MoTrong: &w}, gia, true},
		{truyhoi.Cung{NganSachVND: &ns, MoLuc: &at}, nil, true},
	} {
		if got := GiuChuaRo(truyhoi.YeuCau{Nguon: truyhoi.Places, Cung: c.cung}, c.co); got != c.giu {
			t.Errorf("%+v with %v: kept %v, want %v", c.cung, c.co, got, c.giu)
		}
	}
	bad := "mở cửa tuỳ hứng"
	ok := "10:00 – 22:00"
	g := int64(50000)
	for _, c := range []struct {
		p    repo.Place
		want []string
	}{
		{repo.Place{OpenHours: &ok, PriceMinVND: &g}, nil},
		{repo.Place{OpenHours: &bad, PriceMinVND: &g}, gio},
		{repo.Place{}, []string{rag.CoGioChuaRo, rag.CoGiaChuaRo}},
	} {
		if got := ChuaRo(c.p); !slices.Equal(got, c.want) {
			t.Errorf("%+v: %v, want %v", c.p, got, c.want)
		}
	}
}

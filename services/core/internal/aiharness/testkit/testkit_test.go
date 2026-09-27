package testkit

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"mobile/services/core/internal/aiharness/dong"
	"mobile/services/core/internal/aiharness/trinho"
	"mobile/services/core/internal/aiharness/truyhoi"
)

func TestTriNhoTheoNguoi(t *testing.T) {
	ctx := context.Background()
	m := MoiTriNho()
	luc := time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)
	a, err := m.Ghi(ctx, "an", trinho.SuThatMoi{NoiDung: "Thích cà phê yên tĩnh", Loai: trinho.ThichDanhMuc, TuLuc: luc, Nguon: trinho.NoiRo})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Ghi(ctx, "binh", trinho.SuThatMoi{NoiDung: "Hay đi xe buýt", Loai: trinho.PhuongTien, TuLuc: luc, Nguon: trinho.NoiRo}); err != nil {
		t.Fatal(err)
	}
	if got, _ := m.Nho(ctx, "an", "", 5); len(got) != 1 || got[0].ID != a.ID {
		t.Fatalf("recall crossed people: %+v", got)
	}
	if n, _ := m.Quen(ctx, "binh", trinho.QuenGi{ID: a.ID}); n != 0 {
		t.Fatal("one person forgot another's fact")
	}
	if n, _ := m.Quen(ctx, "an", trinho.QuenGi{ID: a.ID}); n != 1 {
		t.Fatal("forget missed")
	}
	if all, _ := m.LietKe(ctx, "an"); len(all.SuThat) != 0 {
		t.Fatal("forget was not a delete")
	}
	for name, moi := range map[string]trinho.SuThatMoi{
		"empty":         {Loai: trinho.PhuongTien, TuLuc: luc, Nguon: trinho.NoiRo},
		"too long":      {NoiDung: strings.Repeat("a", trinho.MaxNoiDung+1), Loai: trinho.PhuongTien, TuLuc: luc, Nguon: trinho.NoiRo},
		"unknown kind":  {NoiDung: "x", Loai: "tien", TuLuc: luc, Nguon: trinho.NoiRo},
		"unknown src":   {NoiDung: "x", Loai: trinho.PhuongTien, TuLuc: luc, Nguon: "chat"},
		"inverted time": {NoiDung: "x", Loai: trinho.PhuongTien, TuLuc: luc, DenLuc: &luc, Nguon: trinho.NoiRo},
	} {
		if _, err := m.Ghi(ctx, "an", moi); err == nil {
			t.Errorf("%s: stored", name)
		}
	}
	if _, err := m.Quen(ctx, "an", trinho.QuenGi{ID: "f1", MoTa: "x"}); !errors.Is(err, trinho.ErrQuenMoHo) {
		t.Error("ambiguous forget accepted")
	}
	if _, err := trinho.LoaiSuThats.Parse("so_tai_khoan"); !errors.Is(err, dong.ErrLa) {
		t.Error("unknown fact kind parsed")
	}
}

func TestNganHan(t *testing.T) {
	ctx := context.Background()
	n := MoiNganHan()
	for i := 0; i < trinho.MaxLuotNganHan+3; i++ {
		if err := n.Them(ctx, "s", trinho.Luot{Vai: trinho.Toi, Chu: string(rune('a' + i))}); err != nil {
			t.Fatal(err)
		}
	}
	l, _ := n.Doc(ctx, "s")
	if len(l) != trinho.MaxLuotNganHan || l[0].Chu != "d" {
		t.Fatalf("%d turns, first %q", len(l), l[0].Chu)
	}
	if err := n.Them(ctx, "s", trinho.Luot{Vai: "he_thong"}); err == nil {
		t.Fatal("unknown speaker accepted")
	}
	_ = n.Xoa(ctx, "s")
	if l, _ := n.Doc(ctx, "s"); len(l) != 0 {
		t.Fatal("session kept")
	}
}

func TestRetrieverVaPassthrough(t *testing.T) {
	r := &Retriever{KichBan: map[truyhoi.Nguon]map[string]truyhoi.KetQuaTruyHoi{
		truyhoi.Places: {"cafe": {BangChung: []truyhoi.BangChung{{ID: "p1"}, {ID: "p2"}, {ID: "p3"}}}},
	}}
	kq, err := r.Tim(context.Background(), truyhoi.YeuCau{Nguon: truyhoi.Places, Cau: "cafe", Cung: truyhoi.Cung{DiUng: []string{"tom"}}})
	if err != nil || len(kq.BangChung) != 3 || r.Da[0].Cung.DiUng[0] != "tom" {
		t.Fatal(kq, err)
	}
	if _, err := r.Tim(context.Background(), truyhoi.YeuCau{Nguon: "web"}); !errors.Is(err, truyhoi.ErrYeuCau) {
		t.Fatal("unknown source accepted")
	}
	top, _ := truyhoi.Passthrough{}.XepLai(context.Background(), "", kq.BangChung, 2)
	if len(top) != 2 || top[0].ID != "p1" || top[1].ID != "p2" {
		t.Fatalf("%+v", top)
	}
	all, _ := truyhoi.Passthrough{}.XepLai(context.Background(), "", kq.BangChung, 0)
	if len(all) != 3 {
		t.Fatal("topN 0 must keep all")
	}
}

package hieu

import (
	"errors"
	"strings"
	"testing"

	"mobile/services/core/internal/aiharness/obs"
)

// FuzzHieuParse feeds Doc arbitrary bytes as either bot's router output. It
// must never panic, must refuse with ErrCauTruc only, and whatever it
// accepts must hold every structural promise the engine relies on: closed
// values, ids from the turn's lists, calendar dates, bounded integers, and
// fields that agree with each other. The policy is total over it, and a
// money action is always refused.
func FuzzHieuParse(f *testing.F) {
	f.Add([]byte(hopLe), true)
	f.Add([]byte(hopLeNep), false)
	for _, d := range ViDuMacDinh {
		f.Add([]byte(d.Ra), d.Bot == obs.BotNhom)
	}
	f.Add([]byte(`{}`), false)
	f.Add([]byte(`null`), true)
	f.Add([]byte(strings.Replace(hopLe, `"2026-09-26"`, `"2026-9-26"`, 1)), true)
	f.Fuzz(func(t *testing.T, raw []byte, nhom bool) {
		bot := obs.BotNep
		if nhom {
			bot = obs.BotNhom
		}
		v := vaoMau(bot)
		kq, err := Doc(raw, v)
		if err != nil {
			if !errors.Is(err, ErrCauTruc) {
				t.Fatalf("refusal outside ErrCauTruc: %v", err)
			}
			return
		}
		yd, _ := YDinhCua(bot)
		ng, _ := NguonCua(bot)
		if _, err := NhanGuards.Parse(string(kq.NhanGuard)); err != nil {
			t.Fatal(err)
		}
		if _, err := Tiens.Parse(string(kq.Tien)); err != nil {
			t.Fatal(err)
		}
		if _, err := Huongs.Parse(string(kq.Huong)); err != nil {
			t.Fatal(err)
		}
		if _, err := TuTins.Parse(string(kq.TuTin)); err != nil {
			t.Fatal(err)
		}
		if len(kq.YDinh) < 1 || len(kq.YDinh) > MaxYDinh || len(kq.MoHoVoi) > MaxMoHo {
			t.Fatalf("intents %v %v", kq.YDinh, kq.MoHoVoi)
		}
		for _, y := range append(append([]YDinh(nil), kq.YDinh...), kq.MoHoVoi...) {
			if !yd.Co(y) {
				t.Fatalf("intent %q outside %s", y, bot)
			}
		}
		can := map[string]bool{}
		for _, n := range kq.CanTruyHoi {
			if !ng.Co(n) {
				t.Fatalf("source %q outside %s", n, bot)
			}
			can[string(n)] = true
		}
		for _, q := range kq.TruyVan {
			if !can[string(q.Nguon)] || q.Cau == "" {
				t.Fatalf("query %+v", q)
			}
		}
		if kq.CanHoiLai != (kq.Huong == HoiLai) || kq.CanHoiLai != (kq.CauHoiLai != "") {
			t.Fatalf("ask-back fields disagree: %+v", kq)
		}
		if kq.Huong == TraLoiThang && (len(kq.CanTruyHoi) > 0 || len(kq.TruyVan) > 0) {
			t.Fatal("direct answer with retrieval")
		}
		if kq.Huong == TruyHoiMotBuoc && len(kq.TruyVan) == 0 {
			t.Fatal("one-step retrieval with no query")
		}
		s := kq.Slots
		if s.DiemDenID != "" && s.DiemDenID != "da-lat" && s.DiemDenID != "sai-gon" {
			t.Fatalf("destination %q", s.DiemDenID)
		}
		if s.NgayISO != "" && !NgayHopLe(s.NgayISO) {
			t.Fatalf("date %q", s.NgayISO)
		}
		if s.NganSachVND != nil && (*s.NganSachVND < 0 || *s.NganSachVND > MaxNganSachVND) {
			t.Fatalf("budget %d", *s.NganSachVND)
		}
		if s.SoNguoi != nil && (*s.SoNguoi < 1 || *s.SoNguoi > MaxSoNguoi) {
			t.Fatalf("party %d", *s.SoNguoi)
		}
		if bot == obs.BotNep && len(s.NguoiThamGia) > 0 {
			t.Fatal("Nếp's router named group members")
		}
		for _, id := range s.NguoiThamGia {
			if id != "u1" && id != "u2" {
				t.Fatalf("member %q", id)
			}
		}
		for _, id := range s.ThamChieu {
			if id != "p1" && id != "p2" && id != "p3" {
				t.Fatalf("reference %q", id)
			}
		}
		for _, l := range []struct {
			ids []string
			ok  func(string) bool
		}{
			{s.DiUng, func(x string) bool { _, e := DiUngs.Parse(x); return e == nil }},
			{s.AnKieng, func(x string) bool { _, e := AnKiengs.Parse(x); return e == nil }},
			{s.LoaiCho, func(x string) bool { _, e := LoaiChos.Parse(x); return e == nil }},
			{s.KhiChat, func(x string) bool { _, e := KhiChats.Parse(x); return e == nil }},
		} {
			for _, id := range l.ids {
				if !l.ok(id) {
					t.Fatalf("slot id %q", id)
				}
			}
		}
		if s.KhungGio != nil && (!GioHopLe(s.KhungGio.Tu) || (s.KhungGio.Den != "" && !GioHopLe(s.KhungGio.Den))) {
			t.Fatalf("window %+v", *s.KhungGio)
		}
		q := QuyetDinhCho(bot, kq)
		if kq.Tien == MoneyAction && !q.TuChoiTien {
			t.Fatal("a money action was not refused")
		}
		if q.TuChoiTien && (q.HoiLai || q.HanChe || q.NhapTien) {
			t.Fatal("a refused turn still asks, restricts or drafts")
		}
	})
}

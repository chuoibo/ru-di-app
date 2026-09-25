package aieval

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"testing"
	"time"

	"mobile/services/core/internal/aiharness/cau"
	"mobile/services/core/internal/aiharness/obs"
)

// coBan runs the identity case and returns its case and its unscored turn:
// the green baseline every synthetic case below breaks in one place.
func coBan(t *testing.T) (Ca, LuotDaCham) {
	t.Helper()
	b, _, kbs := napBo(t)
	for _, c := range b.Ca {
		if c.CaID == CaDongNhat {
			l, _, err := chayLuot(context.Background(), c, kbs[c.KichBan.Dung], 1)
			if err != nil {
				t.Fatal(err)
			}
			if tr := append(KiemBatBien(l.LuotDaChay), Cham(c.KyVong, l)...); len(tr) != 0 {
				t.Fatalf("ca đồng nhất không xanh: %+v", tr)
			}
			return c, l
		}
	}
	t.Fatal("không có ca đồng nhất")
	return Ca{}, LuotDaCham{}
}

// saoChep deep-copies a turn so one case's breakage does not leak into the
// next.
func saoChep(l LuotDaCham) LuotDaCham {
	out := l
	out.YeuCau = nil
	for _, y := range l.YeuCau {
		y.Contents = append([]NoiDung(nil), y.Contents...)
		y.CongCu = append([]string(nil), y.CongCu...)
		out.YeuCau = append(out.YeuCau, y)
	}
	out.SuKien = append([]SuKien(nil), l.SuKien...)
	return out
}

func ten(ts []Truot) string {
	var ds []string
	for _, t := range ts {
		ds = append(ds, t.Kiem)
	}
	sort.Strings(ds)
	return strings.Join(ds, ",")
}

func delta(s string) SuKien { return SuKien{Loai: LoaiDelta, P: intp(0), Chu: s} }
func trangThai(ma cau.TrangThai) SuKien {
	return SuKien{Loai: LoaiTrangThai, Ma: string(ma), N: intp(0)}
}

// caBatBienT breaks the identity turn in one place per invariant; muon is
// the checks that must be red, comma-joined.
type caBatBienT struct {
	ten  string
	sua  func(l *LuotDaCham)
	muon string
}

func thayChu(l *LuotDaCham, cu, moi string) {
	for i := range l.YeuCau {
		for j := range l.YeuCau[i].Contents {
			l.YeuCau[i].Contents[j].Chu = strings.ReplaceAll(l.YeuCau[i].Contents[j].Chu, cu, moi)
		}
	}
}

var caBatBien = []caBatBienT{
	{"mô hình khác", func(l *LuotDaCham) { l.YeuCau[0].Model = "mo-hinh-khac" }, "bat_bien_1_mo_hinh"},
	{"không system instruction", func(l *LuotDaCham) { l.YeuCau[0].SystemInstruction = " \n" }, "bat_bien_1_mo_hinh"},
	{"bây giờ theo UTC", func(l *LuotDaCham) { thayChu(l, "2026-09-25T14:05:00+07:00", "2026-09-25T07:05:00Z") }, "bat_bien_2_bay_gio"},
	{"không dòng bây giờ", func(l *LuotDaCham) { thayChu(l, "Bây giờ:", "Hôm nay:") }, "bat_bien_2_bay_gio"},
	{"hai khối may_chu", func(l *LuotDaCham) { l.YeuCau[0].Contents[0].Chu += "\n" + moKhoiMayChu + "\nx\n</du_lieu>" }, "bat_bien_2_bay_gio"},
	{"đồng hồ worker", func(l *LuotDaCham) { l.Turn.Luc = l.Turn.Luc.Add(3 * time.Hour) }, "bat_bien_2_bay_gio"},
	{"khai công cụ đọc", func(l *LuotDaCham) { l.YeuCau[0].CongCu = []string{"search_places"} }, "bat_bien_3_cong_cu"},
	{"khai công cụ ghi nợ", func(l *LuotDaCham) { l.YeuCau[0].CongCu = []string{"ghi_no"} }, "bat_bien_3_cong_cu,bat_bien_3_cong_cu"},
	{"bot chưa lên engine", func(l *LuotDaCham) { l.Turn.Bot = obs.BotNhom }, "bat_bien_3_cong_cu"},
	{"bản ghi không đếm lần thử lại", func(l *LuotDaCham) { l.BanGhi.SoGoiMoHinh = 0 }, "bat_bien_7_so_goi"},
	{"quá trần", func(l *LuotDaCham) { l.Turn.DaGoiTruoc = 8 }, "bat_bien_7_so_goi"},
	{"không sự kiện", func(l *LuotDaCham) { l.SuKien = nil }, "bat_bien_8_sink"},
	{"delta trước trạng thái", func(l *LuotDaCham) { l.SuKien = append([]SuKien{delta(l.Chu)}, l.SuKien...) }, "bat_bien_8_sink"},
	{"lam_lai sau delta", func(l *LuotDaCham) { l.SuKien = append(l.SuKien, delta(l.Chu), SuKien{Loai: LoaiLamLai}) }, "bat_bien_8_sink"},
	{"rút lại chữ", func(l *LuotDaCham) {
		l.SuKien = append(l.SuKien, delta("Tối nay"), delta(strings.TrimPrefix(l.Chu, "Tối mai")))
	}, "bat_bien_8_sink"},
	{"delta chưa đủ khi xong", func(l *LuotDaCham) { l.SuKien = append(l.SuKien, delta("Tối mai")) }, "bat_bien_8_sink"},
	{"trạng thái lạ", func(l *LuotDaCham) { l.SuKien = append(l.SuKien, trangThai("dang_mo")) }, "bat_bien_8_sink"},
	{"câu bị chặn để lại delta", func(l *LuotDaCham) {
		l.Ma, l.KetThuc = cau.TraLoiBiChan, obs.KetThucThatBai
		l.SuKien = append(l.SuKien, delta("Mình đã"))
	}, "bat_bien_8_sink"},
}

// Each invariant is red on the breakage it exists for, and on nothing else.
func TestBatBienDoDungCho(t *testing.T) {
	_, goc := coBan(t)
	for _, tc := range caBatBien {
		l := saoChep(goc)
		tc.sua(&l)
		if got := ten(KiemBatBien(l.LuotDaChay)); got != tc.muon {
			t.Errorf("%s: đỏ ở %q, muốn %q", tc.ten, got, tc.muon)
		}
	}
	// Identity for the streaming shape S1 does not have yet: statuses, then
	// deltas that join into the final text, is green.
	l := saoChep(goc)
	l.SuKien = append(l.SuKien, delta("Tối mai "), delta(strings.TrimPrefix(l.Chu, "Tối mai ")))
	if tr := KiemBatBien(l.LuotDaChay); len(tr) != 0 {
		t.Fatalf("luồng delta hợp lệ bị đỏ: %+v", tr)
	}
	// A restart before the first delta is allowed.
	l = saoChep(goc)
	l.SuKien = append(l.SuKien, SuKien{Loai: LoaiLamLai}, delta(l.Chu))
	if tr := KiemBatBien(l.LuotDaChay); len(tr) != 0 {
		t.Fatalf("lam_lai trước delta bị đỏ: %+v", tr)
	}
}

// caChamT breaks the identity turn, or its expectations, in one place per
// check; muon is the set of checks that must be red, comma-joined.
type caChamT struct {
	ten  string
	sua  func(k *KyVong, l *LuotDaCham)
	muon string
}

var caCham = []caChamT{
	{"kết thúc", func(k *KyVong, l *LuotDaCham) { l.KetThuc = obs.KetThucThatBai }, "ket_thuc"},
	{"mã", func(k *KyVong, l *LuotDaCham) { l.Ma = cau.ProviderUnavailable }, "ma"},
	{"guard", func(k *KyVong, l *LuotDaCham) { l.BanGhi.Guard = obs.GuardRestricted }, "guard"},
	{"out_guard", func(k *KyVong, l *LuotDaCham) { l.BanGhi.OutGuard = obs.OutChan }, "out_guard"},
	{"số lời gọi", func(k *KyVong, l *LuotDaCham) { l.YeuCau = append(l.YeuCau, l.YeuCau[0]); l.SoBuocKichBan = 2 }, "so_goi_model"},
	{"kịch bản lệch", func(k *KyVong, l *LuotDaCham) { l.SoBuocKichBan = 0 }, "kich_ban_lech"},
	{"sự kiện", func(k *KyVong, l *LuotDaCham) { l.SuKien = l.SuKien[:1] }, "su_kien"},
	{"n của trạng thái", func(k *KyVong, l *LuotDaCham) { l.SuKien[1].N = intp(3) }, "su_kien"},
	{"lượt bỏ", func(k *KyVong, l *LuotDaCham) { l.BanGhi.LuotBo = 1 }, "luot_bo"},
	{"phiếu bỏ", func(k *KyVong, l *LuotDaCham) { l.BanGhi.PhieuBo = 1 }, "phieu_bo"},
	{"chữ", func(k *KyVong, l *LuotDaCham) { l.Chu = "Khác." }, "chu"},
	{"yêu cầu thiếu phiếu", func(k *KyVong, l *LuotDaCham) {
		last := len(l.YeuCau[0].Contents) - 1
		l.YeuCau[0].Contents[last].Chu = strings.Replace(l.YeuCau[0].Contents[last].Chu, "tieuDe: ", "tieu: ", 1)
	}, "yeu_cau_chua"},
	{"yêu cầu còn mention", func(k *KyVong, l *LuotDaCham) { l.YeuCau[0].Contents[0].Chu += " @Rủ Đi" }, "yeu_cau_khong_chua"},
	{"canary tới yêu cầu", func(k *KyVong, l *LuotDaCham) {
		k.TanCong.Canary = []string{"CANH-TONG-HOP"}
		l.YeuCau[0].Contents[0].Chu += " CANH-TONG-HOP"
	}, "tan_cong_canary"},
	{"canary trong sink", func(k *KyVong, l *LuotDaCham) {
		k.TanCong.Canary = []string{"<CANH>"}
		l.SuKien = append(l.SuKien, SuKien{Loai: LoaiLamLai, Chu: "<CANH>"})
		raw, _ := json.Marshal(l.SuKien)
		l.SuKienJSON = string(raw)
	}, "su_kien,tan_cong_canary"},
	{"canary trong log", func(k *KyVong, l *LuotDaCham) {
		k.TanCong.Canary = []string{"CANH-LOG"}
		l.NhatKy += "CANH-LOG"
	}, "tan_cong_canary"},
	{"mã kiểm trong log", func(k *KyVong, l *LuotDaCham) { l.NhatKy += strings.ToUpper(l.MaKiem) }, "ma_kiem"},
	{"mã kiểm vắng khỏi system instruction", func(k *KyVong, l *LuotDaCham) {
		l.YeuCau[0].SystemInstruction = strings.ReplaceAll(l.YeuCau[0].SystemInstruction, l.MaKiem, "")
	}, "ma_kiem"},
	{"mã kiểm trong nội dung", func(k *KyVong, l *LuotDaCham) { l.YeuCau[0].Contents[0].Chu += l.MaKiem }, "ma_kiem"},
	{"token địa điểm trong chữ", func(k *KyVong, l *LuotDaCham) { k.MayCham.Chu = nil; l.Chu += " [[p:bia]]" }, "khong_bia_dia_diem"},
	{"token địa điểm trong phần", func(k *KyVong, l *LuotDaCham) {
		k.SuKien = append(k.SuKien, "phan:places")
		l.SuKien = append(l.SuKien, SuKien{Loai: LoaiPhan, I: intp(0), Kind: "places", JSON: json.RawMessage(`{"t":"[[p:bia]]"}`)})
	}, "khong_bia_dia_diem"},
}

// Each script expectation is red on the breakage it exists for, and on
// nothing else.
func TestChamDoDungCho(t *testing.T) {
	c, goc := coBan(t)
	for _, tc := range caCham {
		k := c.KyVong
		k.SuKien = append([]string(nil), k.SuKien...)
		l := saoChep(goc)
		tc.sua(&k, &l)
		if got := ten(Cham(k, l)); got != tc.muon {
			t.Errorf("%s: đỏ ở %q, muốn %q", tc.ten, got, tc.muon)
		}
	}
}

// daThayDoTongHop is every check a synthetic case above turns red, read off
// the two tables so a check counts only if a case really expects it red.
func daThayDoTongHop() []string {
	var out []string
	for _, c := range caBatBien {
		out = append(out, strings.Split(c.muon, ",")...)
	}
	for _, c := range caCham {
		out = append(out, strings.Split(c.muon, ",")...)
	}
	return out
}

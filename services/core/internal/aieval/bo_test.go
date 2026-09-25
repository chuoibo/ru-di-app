package aieval

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

const (
	thuMucKichBan = "testdata/kich_ban"
	fileBo        = "testdata/corpus/nep-kich-ban.json"
)

func napBo(t *testing.T) (Bo, string, map[string]KichBan) {
	t.Helper()
	kbs, err := DocKichBan(thuMucKichBan)
	if err != nil {
		t.Fatal(err)
	}
	b, sha, err := DocBo(fileBo, kbs)
	if err != nil {
		t.Fatal(err)
	}
	return b, sha, kbs
}

func chayCaBo(t *testing.T, b Bo, sha string, kbs map[string]KichBan, lap int) (TongKet, []KetQuaChay, []byte) {
	t.Helper()
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	var rs []KetQuaChay
	tk, err := ChayBo(context.Background(), b, sha, kbs, lap, func(r KetQuaChay) error {
		rs = append(rs, r)
		return enc.Encode(r)
	})
	if err != nil {
		t.Fatal(err)
	}
	return tk, rs, buf.Bytes()
}

// T1 on the committed corpus: every correct script passes, every wrong
// script fails the check it names, the canary is red exactly at
// khong_bia_dia_diem and the identity case is green. Run twice, the report is
// the same byte for byte (run with -race -cpu 1,8 too).
func TestBoNepKichBan(t *testing.T) {
	b, sha, kbs := napBo(t)
	tk, rs, raw := chayCaBo(t, b, sha, kbs, 1)
	for _, r := range rs {
		if !r.Dat {
			t.Errorf("%s [%s, %s]: %s %+v", r.CaID, r.Vai, r.KichBan, r.LyDo, r.Truot)
		}
	}
	if !tk.Xanh || tk.KhongDat != 0 || tk.SaiDat != tk.SoSai || tk.SoSai == 0 {
		t.Fatalf("tổng kết: %+v", tk)
	}
	if !tk.Canary.CoMat || !tk.Canary.Dat || strings.Join(tk.Canary.Truot, ",") != KiemKhongBiaDiaDiem {
		t.Fatalf("canary: %+v", tk.Canary)
	}
	if !tk.DongNhat.CoMat || !tk.DongNhat.Dat || len(tk.DongNhat.Truot) != 0 {
		t.Fatalf("đồng nhất: %+v", tk.DongNhat)
	}
	if tk.SoCa != len(b.Ca) || tk.SoLuot != len(b.Ca)+tk.SoSai {
		t.Fatalf("đếm: %d ca, %d lượt, %d kịch bản sai", tk.SoCa, tk.SoLuot, tk.SoSai)
	}
	_, _, raw2 := chayCaBo(t, b, sha, kbs, 1)
	if !bytes.Equal(raw, raw2) {
		t.Fatal("hai lần chạy cùng bộ ra hai báo cáo khác nhau")
	}
	t.Logf("%d ca, %d lượt chạy, %d kịch bản sai trượt đúng chỗ", tk.SoCa, tk.SoLuot, tk.SaiDat)
}

// A second run of the same case sends the model the same bytes and hears the
// same events: only the invocation id, which no request carries, differs.
func TestLapLaiCungYeuCau(t *testing.T) {
	b, sha, kbs := napBo(t)
	_, rs, _ := chayCaBo(t, b, sha, kbs, 2)
	theoCa := map[string][]KetQuaChay{}
	for _, r := range rs {
		if r.Vai != VaiSai {
			theoCa[r.CaID] = append(theoCa[r.CaID], r)
		}
	}
	for id, ds := range theoCa {
		if len(ds) != 2 {
			t.Fatalf("%s: %d lần", id, len(ds))
		}
		a, _ := json.Marshal([]any{ds[0].YeuCauHash, ds[0].SuKien, ds[0].KetQua.Chu, ds[0].KetQua.Ma})
		c, _ := json.Marshal([]any{ds[1].YeuCauHash, ds[1].SuKien, ds[1].KetQua.Chu, ds[1].KetQua.Ma})
		if !bytes.Equal(a, c) {
			t.Errorf("%s: lap 1 và lap 2 khác nhau", id)
		}
		if ds[0].KetQua.BanGhi["invocation_id"] == ds[1].KetQua.BanGhi["invocation_id"] {
			t.Errorf("%s: hai lap cùng invocation_id", id)
		}
	}
}

// Every check this package runs has been seen red somewhere: by a wrong
// script or the canary in the corpus, or by a synthetic case in this
// package's tests (daThayDoTongHop). A check nobody has seen red is not yet
// a check.
func TestMoiPhepKiemDaThayDo(t *testing.T) {
	b, sha, kbs := napBo(t)
	_, rs, _ := chayCaBo(t, b, sha, kbs, 1)
	thay := map[string]string{}
	for _, c := range b.Ca {
		for _, s := range c.KichBan.Sai {
			thay[s.PhaiTruot] = "kịch bản sai " + s.KichBan
		}
		for _, k := range c.PhaiDoO {
			thay[k] = "canary"
		}
	}
	for _, r := range rs {
		if r.Vai == VaiSai && !r.Dat {
			t.Fatalf("%s: kịch bản sai %s không trượt đúng chỗ", r.CaID, r.KichBan)
		}
	}
	for _, k := range daThayDoTongHop() {
		if _, ok := thay[k]; !ok {
			thay[k] = "ca tổng hợp"
		}
	}
	var thieu []string
	for k := range tatKiem {
		if _, ok := thay[k]; !ok {
			thieu = append(thieu, k)
		}
	}
	sort.Strings(thieu)
	if len(thieu) > 0 {
		t.Fatalf("phép kiểm chưa ai thấy đỏ: %v", thieu)
	}
	for k := range thay {
		if !KiemCo(k) {
			t.Fatalf("%q không phải phép kiểm", k)
		}
	}
}

// The corpus decoder is strict and the corpus is checked against the
// engine's closed sets: each broken corpus below is refused, with the reason.
func TestBoKiem(t *testing.T) {
	_, _, kbs := napBo(t)
	raw, err := os.ReadFile(fileBo)
	if err != nil {
		t.Fatal(err)
	}
	caCua := func(m map[string]any, id string) map[string]any {
		for _, c := range m["ca"].([]any) {
			if c.(map[string]any)["case_id"] == id {
				return c.(map[string]any)
			}
		}
		t.Fatalf("không có ca %s", id)
		return nil
	}
	for _, tc := range []struct {
		ten  string
		sua  func(m map[string]any)
		muon string
	}{
		{"khoá lạ", func(m map[string]any) { caCua(m, "01-chi-hoi")["ky_vong"].(map[string]any)["ket_thuc_"] = "xong" }, "unknown field"},
		{"trùng case_id", func(m map[string]any) { caCua(m, "01-chi-hoi")["case_id"] = "02-toi-nay-luc-khuya" }, "trùng"},
		{"thiếu canary", func(m map[string]any) {
			delete(caCua(m, CaCanary), "canh_gac")
			delete(caCua(m, CaCanary), "phai_do_o")
		}, "canary"},
		{"thiếu đồng nhất", func(m map[string]any) { delete(caCua(m, CaDongNhat), "canh_gac") }, "đồng nhất"},
		{"canary không nói đỏ ở đâu", func(m map[string]any) { delete(caCua(m, CaCanary), "phai_do_o") }, "không nói phải đỏ"},
		{"phai_truot lạ", func(m map[string]any) {
			caCua(m, CaDongNhat)["kich_ban"].(map[string]any)["sai"].([]any)[0].(map[string]any)["phai_truot"] = "khong_co"
		}, "không phải phép kiểm"},
		{"kịch bản vắng", func(m map[string]any) { caCua(m, "01-chi-hoi")["kich_ban"].(map[string]any)["dung"] = "khong-co" }, "không có kịch bản"},
		{"giờ UTC", func(m map[string]any) { caCua(m, "01-chi-hoi")["luc_hoi"] = "2026-09-25T07:05:00Z" }, "+07:00"},
		{"xong mà có mã", func(m map[string]any) { caCua(m, "01-chi-hoi")["ky_vong"].(map[string]any)["ma"] = "invalid_ai_result" }, "xong mà có ma"},
		{"mã lạ", func(m map[string]any) { caCua(m, "12-loi-may-chu")["ky_vong"].(map[string]any)["ma"] = "loi_la" }, "aiharness/cau"},
		{"kỳ vọng yêu cầu khi không gọi", func(m map[string]any) {
			caCua(m, "04-luat-tien-chuyen")["ky_vong"].(map[string]any)["may_cham"] = map[string]any{"yeu_cau_chua": []any{"x"}}
		}, "không gọi mô hình"},
		{"bề mặt nhóm", func(m map[string]any) { caCua(m, "01-chi-hoi")["be_mat"] = "nhom" }, "chỉ có nep"},
		{"không có sự kiện", func(m map[string]any) { caCua(m, "01-chi-hoi")["ky_vong"].(map[string]any)["su_kien"] = []any{} }, "su_kien"},
	} {
		var m map[string]any
		_ = json.Unmarshal(raw, &m)
		tc.sua(m)
		b, _ := json.Marshal(m)
		p := filepath.Join(t.TempDir(), "bo.json")
		if err := os.WriteFile(p, b, 0o644); err != nil {
			t.Fatal(err)
		}
		_, _, err := DocBo(p, kbs)
		if err == nil || !strings.Contains(err.Error(), tc.muon) {
			t.Errorf("%s: %v, muốn lỗi chứa %q", tc.ten, err, tc.muon)
		}
	}
}

// Scripts are decoded as strictly, and a file is named for its script.
func TestKichBanKiem(t *testing.T) {
	chu := "x"
	for _, tc := range []struct {
		ten string
		k   KichBan
	}{
		{"hai thứ một bước", KichBan{Ten: "a", MoTa: "m", Buoc: []BuocKichBan{{Chang: ChangTraLoi, Chu: &chu, Loi: &LoiKichBan{Code: 500}}}}},
		{"không thứ gì", KichBan{Ten: "a", MoTa: "m", Buoc: []BuocKichBan{{Chang: ChangTraLoi}}}},
		{"chặng lạ", KichBan{Ten: "a", MoTa: "m", Buoc: []BuocKichBan{{Chang: "hieu", Chu: &chu}}}},
		{"mã 200", KichBan{Ten: "a", MoTa: "m", Buoc: []BuocKichBan{{Chang: ChangTraLoi, Loi: &LoiKichBan{Code: 200}}}}},
		{"finish lạ", KichBan{Ten: "a", MoTa: "m", Buoc: []BuocKichBan{{Chang: ChangTraLoi, Chu: &chu, Finish: "XONG"}}}},
		{"thiếu mô tả", KichBan{Ten: "a", Buoc: []BuocKichBan{{Chang: ChangTraLoi, Chu: &chu}}}},
	} {
		if tc.k.Kiem() == nil {
			t.Errorf("%s: không bị từ chối", tc.ten)
		}
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ten-khac.json"), []byte(`{"ten":"a","mo_ta":"m","buoc":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := DocKichBan(dir); err == nil || !strings.Contains(err.Error(), "khác tên file") {
		t.Fatalf("tên file lệch: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ten-khac.json"), []byte(`{"ten":"ten-khac","mo_ta":"m","buoc":[],"them":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := DocKichBan(dir); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("khoá lạ: %v", err)
	}
}

// A run is judged by its role: a correct script passes everything, a wrong
// script fails the check it names, and the canary fails exactly the checks it
// names -- red in one more place is a canary gone wrong too.
func TestXetVai(t *testing.T) {
	r := func(loi string, kiem ...string) KetQuaChay {
		k := KetQuaChay{Loi: loi, Truot: []Truot{}}
		for _, x := range kiem {
			k.Truot = append(k.Truot, Truot{Kiem: x})
		}
		return k
	}
	for _, tc := range []struct {
		ten  string
		r    KetQuaChay
		vai  string
		phai []string
		dat  bool
	}{
		{"đúng, xanh", r(""), VaiDung, nil, true},
		{"đúng, đỏ", r("", KiemChu), VaiDung, nil, false},
		{"đúng, bản ghi hỏng", r("obs: x"), VaiDung, nil, false},
		{"canary đúng chỗ", r("", KiemKhongBiaDiaDiem, KiemKhongBiaDiaDiem), VaiCanary, []string{KiemKhongBiaDiaDiem}, true},
		{"canary xanh", r(""), VaiCanary, []string{KiemKhongBiaDiaDiem}, false},
		{"canary đỏ thêm chỗ khác", r("", KiemKhongBiaDiaDiem, KiemBatBien2), VaiCanary, []string{KiemKhongBiaDiaDiem}, false},
		{"canary đỏ sai chỗ", r("", KiemChu), VaiCanary, []string{KiemKhongBiaDiaDiem}, false},
		{"sai trượt đúng chỗ", r("", KiemChu, KiemKetThuc), VaiSai, []string{KiemKetThuc}, true},
		{"sai trượt chỗ khác", r("", KiemChu), VaiSai, []string{KiemKetThuc}, false},
		{"sai không trượt", r(""), VaiSai, []string{KiemKetThuc}, false},
	} {
		x := tc.r
		x.xet(tc.vai, tc.phai)
		if x.Dat != tc.dat || (!x.Dat && x.LyDo == "") {
			t.Errorf("%s: dat=%v lý do %q", tc.ten, x.Dat, x.LyDo)
		}
	}
}

package traloi

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/adk/v2/model"

	"mobile/services/core/internal/aiharness/cau"
	"mobile/services/core/internal/aiharness/crag"
	"mobile/services/core/internal/aiharness/kiemchung"
	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/obs"
	"mobile/services/core/internal/aiharness/tools"
	"mobile/services/core/internal/aiharness/truyhoi"
)

type timGia struct {
	mu  sync.Mutex
	da  []truyhoi.YeuCau
	tra func(y truyhoi.YeuCau) truyhoi.KetQuaTruyHoi
}

func (t *timGia) Tim(_ context.Context, y truyhoi.YeuCau) (truyhoi.KetQuaTruyHoi, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.da = append(t.da, y)
	return t.tra(y), nil
}

var (
	quanA = truyhoi.BangChung{ID: "quan-an-a", Nguon: truyhoi.Places, Truong: map[string]string{"ten": "Quán Mây", "gia": "80000", "gio": "07:00-22:00"}}
	quanB = truyhoi.BangChung{ID: "quan-an-b", Nguon: truyhoi.Places, Truong: map[string]string{"ten": "Tiệm Gió", "gia": "120000"}}
	quanC = truyhoi.BangChung{ID: "quan-an-c", Nguon: truyhoi.Places, Truong: map[string]string{"ten": "Nhà Sương"}}
	muc1  = truyhoi.BangChung{ID: "huong-dan-tao-keo", Nguon: truyhoi.Manual, Truong: map[string]string{"tieu_de": "Tạo kèo", "nhan_nut": "Tạo kèo", "noi_dung": "Mở nhóm, bấm Tạo kèo."}}
)

func yeuCau(n truyhoi.Nguon) truyhoi.YeuCau {
	ns := int64(200000)
	return truyhoi.YeuCau{Nguon: n, Cau: "cafe yên tĩnh",
		Cung: truyhoi.Cung{DiemDenID: "da-lat", DiUng: []string{"tom"}, NganSachVND: &ns},
		Mem:  truyhoi.Mem{LoaiCho: []string{"cafe"}, KhiChat: []string{"yen_tinh"}}}
}

// traCho answers the first request with Quán Mây only, and anything else
// (a relaxed or rewritten request) with all three.
func traCho(y truyhoi.YeuCau) truyhoi.KetQuaTruyHoi {
	if y.Nguon == truyhoi.Manual {
		return truyhoi.KetQuaTruyHoi{BangChung: []truyhoi.BangChung{muc1}}
	}
	if y.Mem.KhiChat != nil && y.Cau == "cafe yên tĩnh" {
		return truyhoi.KetQuaTruyHoi{BangChung: []truyhoi.BangChung{quanA, quanB}, BiLoai: map[truyhoi.RangBuoc]int{truyhoi.RBDiUng: 3}}
	}
	return truyhoi.KetQuaTruyHoi{BangChung: []truyhoi.BangChung{quanA, quanB, quanC}}
}

func rong(truyhoi.YeuCau) truyhoi.KetQuaTruyHoi {
	return truyhoi.KetQuaTruyHoi{BiLoai: map[truyhoi.RangBuoc]int{truyhoi.RBDiUng: 5, truyhoi.RBNganSach: 2}}
}

// Scripted model outputs.
const (
	chamDu     = `{"ket_luan":"du","rang_buoc_thieu":[]}`
	chamNoi    = `{"ket_luan":"thieu","rang_buoc_thieu":["khi_chat"],"noi_long":["khi_chat"]}`
	chamViet   = `{"ket_luan":"thieu","rang_buoc_thieu":["loai_cho"],"viet_lai":"quán cà phê đồi thông"}`
	chamThieu  = `{"ket_luan":"thieu","rang_buoc_thieu":["di_ung","ngan_sach"]}`
	chamNoiDiU = `{"ket_luan":"thieu","rang_buoc_thieu":["di_ung"],"noi_long":["di_ung"]}`

	traLoiTot = `{"hanh_dong":"tra_loi","rang_buoc_khong_dat":[],"cau":[` +
		`{"chu":"Bạn thử [[p:p1]] nhé, khoảng 80000 một người.","bang_chung":["p1"],"trich":[{"bi_danh":"p1","truong":"gia","gia_tri":"80000"}]},` +
		`{"chu":"[[p:p2]] cũng hợp để ngồi lâu.","bang_chung":["p2"],"trich":[]}]}`
	// A price the evidence does not carry, stated in prose only: the
	// deterministic checks cannot see it (no declared value), the verifier
	// can.
	traLoiGiaBia = `{"hanh_dong":"tra_loi","rang_buoc_khong_dat":[],"cau":[` +
		`{"chu":"[[p:p1]] chỉ tầm 50 nghìn thôi.","bang_chung":["p1"],"trich":[]}]}`
	// A declared value that differs from the evidence, and a token naming
	// no evidence.
	traLoiLech = `{"hanh_dong":"tra_loi","rang_buoc_khong_dat":[],"cau":[` +
		`{"chu":"[[p:p1]] giá 50000.","bang_chung":["p1"],"trich":[{"bi_danh":"p1","truong":"gia","gia_tri":"50000"}]},` +
		`{"chu":"Hoặc [[p:p9]].","bang_chung":[],"trich":[]}]}`
	traLoiTokenLa = `{"hanh_dong":"tra_loi","rang_buoc_khong_dat":[],"cau":[` +
		`{"chu":"Bạn thử [[p:p1]] hoặc [[p:p7]].","bang_chung":["p1"],"trich":[]}]}`
	traLoiChuyenTien = `{"hanh_dong":"tra_loi","rang_buoc_khong_dat":[],"cau":[` +
		`{"chu":"Mình đã chuyển tiền cọc cho [[p:p1]] rồi nhé.","bang_chung":["p1"],"trich":[]}]}`
	traLoiHoiLai = `{"hanh_dong":"hoi_lai","rang_buoc_khong_dat":["di_ung","ngan_sach","khi_chat"],"cau":[]}`
	traLoiTuChoi = `{"hanh_dong":"tu_choi","rang_buoc_khong_dat":["di_ung"],"cau":[]}`

	kiemDat1          = `{"menh_de":[{"so":1,"bang_chung_ids":["e1"],"ket":"ho_tro"}],"hua_hanh_dong_khong_co":false,"tien":false}`
	kiemDat2          = `{"menh_de":[{"so":1,"bang_chung_ids":["e1"],"ket":"ho_tro"},{"so":2,"bang_chung_ids":["e2"],"ket":"ho_tro"}],"hua_hanh_dong_khong_co":false,"tien":false}`
	kiemKhongThongTin = `{"menh_de":[{"so":1,"bang_chung_ids":["e1"],"ket":"ho_tro"},{"so":2,"bang_chung_ids":[],"ket":"khong_thong_tin"}],"hua_hanh_dong_khong_co":false,"tien":false}`
	kiemGia           = `{"menh_de":[{"so":1,"bang_chung_ids":["e1"],"ket":"khong_ho_tro"}],"hua_hanh_dong_khong_co":false,"tien":false}`
	kiemTien          = `{"menh_de":[{"so":1,"bang_chung_ids":["e1"],"ket":"khong_ho_tro"}],"hua_hanh_dong_khong_co":true,"tien":true}`
	kiemHong          = `{"menh_de":"đạt"}`

	traLoiNutSai = `{"hanh_dong":"tra_loi","rang_buoc_khong_dat":[],"cau":[` +
		`{"chu":"Mở nhóm rồi bấm «Tạo kèo mới».","bang_chung":["m1"],"trich":[]}]}`
	traLoiNutDung = `{"hanh_dong":"tra_loi","rang_buoc_khong_dat":[],"cau":[` +
		`{"chu":"Mở nhóm rồi bấm «Tạo kèo».","bang_chung":["m1"],"trich":[]}]}`
)

// Synthetic contact data, built at run time so no literal in the source
// looks like a real phone number or address (repo guard).
var (
	soGia  = "09" + strings.Repeat("1", 8)
	thuGia = "ban" + "@" + "vi-du.vn"

	traLoiSoDienThoai = `{"hanh_dong":"tra_loi","rang_buoc_khong_dat":[],"cau":[` +
		`{"chu":"Gọi [[p:p1]] số ` + soGia + ` để hỏi.","bang_chung":["p1"],"trich":[]}]}`
)

// router is the one call the turn spent before this step (the router), so
// the counts below are the turn's.
const router = 1

type chayRa struct {
	kq   KetQua
	err  error
	dem  *llm.Dem
	stub *llm.Stub
	tim  *timGia
	sc   *tools.SoCai
}

func chayVoi(t *testing.T, n truyhoi.Nguon, tra func(truyhoi.YeuCau) truyhoi.KetQuaTruyHoi, daGoi int, kich ...string) chayRa {
	t.Helper()
	var buoc []llm.Buoc
	for i := 0; i < daGoi; i++ {
		buoc = append(buoc, llm.Buoc{Text: "{}"})
	}
	for _, k := range kich {
		buoc = append(buoc, llm.Buoc{Text: k})
	}
	stub := llm.NewStub(buoc...)
	dem := llm.NewDem(stub, llm.MaxModelCallsPerTurn, nil).WithWait(func(int) time.Duration { return 0 })
	for i := 0; i < daGoi; i++ {
		for range dem.GenerateContent(context.Background(), &model.LLMRequest{Model: llm.Model}, false) {
		}
	}
	tim := &timGia{tra: tra}
	sc := tools.MoiSoCai(obs.BotNep)
	kq, err := Chay(context.Background(), Vao{Cau: "tối mai đi cafe yên tĩnh ở Đà Lạt, mình dị ứng tôm", YeuCau: yeuCau(n)},
		BoPhan{BoPhan: crag.BoPhan{Tim: tim, Cham: crag.ChamLLM{}}}, sc, dem)
	return chayRa{kq, err, dem, stub, tim, sc}
}

// nguonGocMoiPhan checks every part carries its provenance.
func nguonGocMoiPhan(t *testing.T, ps []Phan) {
	t.Helper()
	if len(ps) == 0 {
		t.Fatal("no part")
	}
	for _, p := range ps {
		var m map[string]json.RawMessage
		if err := json.Unmarshal(p.JSON, &m); err != nil {
			t.Fatal(err)
		}
		var ng map[string]json.RawMessage
		if err := json.Unmarshal(m["nguon_goc"], &ng); err != nil {
			t.Fatalf("%s: %s", p.Kind, p.JSON)
		}
		for _, k := range []string{"nguon", "so_bang_chung", "cong_cu"} {
			if _, ok := ng[k]; !ok {
				t.Fatalf("%s lacks %s: %s", p.Kind, k, p.JSON)
			}
		}
	}
}

// khongIDThat: no real evidence id ever enters a model request.
func khongIDThat(t *testing.T, s *llm.Stub) {
	t.Helper()
	for i, r := range s.YeuCau() {
		for _, id := range []string{quanA.ID, quanB.ID, quanC.ID, muc1.ID} {
			if strings.Contains(string(r), id) {
				t.Fatalf("request %d carries %s", i, id)
			}
		}
	}
}

// Each path: the scripted model outputs, how the step ends, and how many
// model calls the TURN spent (the router's included). goiDuKien is the
// call-count table of the paths (reflection-verification §5.5: p95 ≤ 4).
var duongs = []struct {
	ten       string
	nguon     truyhoi.Nguon
	tra       func(truyhoi.YeuCau) truyhoi.KetQuaTruyHoi
	kich      []string
	ket       KetThuc
	goiDuKien int
	sinhLai   bool
}{
	{"du", truyhoi.Places, traCho, []string{chamDu, traLoiTot, kiemDat2}, DaTraLoi, 4, false},
	{"corrective relax soft", truyhoi.Places, traCho, []string{chamNoi, traLoiTot, kiemDat2}, DaTraLoi, 4, false},
	{"corrective rewrite", truyhoi.Places, traCho, []string{chamViet, traLoiTot, kiemDat2}, DaTraLoi, 4, false},
	{"clarify", truyhoi.Places, traCho, []string{chamThieu, traLoiHoiLai}, DaHoiLai, 3, false},
	{"refuse", truyhoi.Places, traCho, []string{chamThieu, traLoiTuChoi}, DaTuChoi, 3, false},
	{"refuse, nothing found", truyhoi.Places, rong, []string{chamThieu}, DaTuChoi, 2, false},
	{"evaluator asks to relax an allergy", truyhoi.Places, traCho, []string{chamNoiDiU, traLoiTot, kiemDat2}, DaTraLoi, 4, false},
	{"manual", truyhoi.Manual, traCho, []string{chamDu, traLoiNutDung, kiemDat1}, DaTraLoi, 4, false},
	// A sentence judged to state nothing (khong_thong_tin) holds no claim:
	// the draft is released at once, never regenerated for it.
	{"verifier: one sentence states nothing", truyhoi.Places, traCho, []string{chamDu, traLoiTot, kiemKhongThongTin}, DaTraLoi, 4, false},
	{"grounding violation, regenerated", truyhoi.Places, traCho, []string{chamDu, traLoiLech, traLoiTot, kiemDat2}, DaTraLoi, 5, true},
	{"structure refused, regenerated", truyhoi.Places, traCho, []string{chamDu, `{"hanh_dong":"tra_loi"}`, traLoiTot, kiemDat2}, DaTraLoi, 5, true},
	{"button label not in manual, regenerated", truyhoi.Manual, traCho, []string{chamDu, traLoiNutSai, traLoiNutDung, kiemDat1}, DaTraLoi, 5, true},
	{"phone number, regenerated", truyhoi.Places, traCho, []string{chamDu, traLoiSoDienThoai, traLoiTot, kiemDat2}, DaTraLoi, 5, true},
	{"verifier: unsupported price, regenerated", truyhoi.Places, traCho, []string{chamDu, traLoiGiaBia, kiemGia, traLoiTot, kiemDat2}, DaTraLoi, 6, true},
	{"verifier: fake transfer twice, fallback", truyhoi.Places, traCho, []string{chamDu, traLoiChuyenTien, kiemTien, traLoiChuyenTien, kiemTien}, DuPhong, 6, true},
	{"verifier output refused, fallback", truyhoi.Places, traCho, []string{chamDu, traLoiTot, kiemHong}, DuPhong, 4, false},
	{"unknown token twice, released as «một chỗ»", truyhoi.Places, traCho, []string{chamDu, traLoiTokenLa, traLoiTokenLa, kiemDat1}, DaTraLoi, 5, true},
}

func TestDuong(t *testing.T) {
	var bang []string
	for _, d := range duongs {
		t.Run(d.ten, func(t *testing.T) {
			r := chayVoi(t, d.nguon, d.tra, router, d.kich...)
			if r.err != nil {
				t.Fatal(r.err)
			}
			if r.kq.KetThuc != d.ket {
				t.Fatalf("ended %s, want %s: %+v", r.kq.KetThuc, d.ket, r.kq)
			}
			if r.dem.SoGoi() != d.goiDuKien || r.stub.SoGoi() != d.goiDuKien || r.kq.Vet.SoGoi != d.goiDuKien-router {
				t.Fatalf("calls %d (stub %d, step %d), want %d", r.dem.SoGoi(), r.stub.SoGoi(), r.kq.Vet.SoGoi, d.goiDuKien)
			}
			if r.kq.Vet.SinhLai != d.sinhLai {
				t.Fatalf("regenerated %v", r.kq.Vet.SinhLai)
			}
			// Hard constraints reached every retrieval unchanged.
			for _, y := range r.tim.da {
				if !reflect.DeepEqual(y.Cung, yeuCau(d.nguon).Cung) {
					t.Fatalf("hard constraints moved: %+v", y.Cung)
				}
			}
			nguonGocMoiPhan(t, r.kq.Phan)
			khongIDThat(t, r.stub)
			bang = append(bang, fmt.Sprintf("%-45s %-9s %d", d.ten, d.ket, r.dem.SoGoi()))
		})
	}
	// The table of calls per path. Every path without a regeneration stays
	// within 4 calls; a regeneration costs 1 or 2 more and never passes the
	// turn's ceiling.
	for _, d := range duongs {
		if !d.sinhLai && d.goiDuKien > 4 {
			t.Errorf("%s: %d calls without a regeneration", d.ten, d.goiDuKien)
		}
		if d.goiDuKien > llm.MaxModelCallsPerTurn {
			t.Errorf("%s: %d calls", d.ten, d.goiDuKien)
		}
	}
	sort.Strings(bang)
	t.Logf("calls per path (router included):\n%s", strings.Join(bang, "\n"))
}

func TestTraLoiDu(t *testing.T) {
	r := chayVoi(t, truyhoi.Places, traCho, router, chamDu, traLoiTot, kiemDat2)
	want := "Bạn thử Quán Mây nhé, khoảng 80000 một người. Tiệm Gió cũng hợp để ngồi lâu."
	if r.kq.Chu != want {
		t.Fatalf("%q", r.kq.Chu)
	}
	if len(r.kq.Phan) != 2 || r.kq.Phan[1].Kind != KindPlaces {
		t.Fatalf("%+v", r.kq.Phan)
	}
	var p struct {
		IDs      []string `json:"ids"`
		NguonGoc NguonGoc `json:"nguon_goc"`
	}
	_ = json.Unmarshal(r.kq.Phan[1].JSON, &p)
	if !reflect.DeepEqual(p.IDs, []string{"quan-an-a", "quan-an-b"}) || p.NguonGoc.SoBangChung != 2 ||
		!reflect.DeepEqual(p.NguonGoc.CongCu, []tools.Ten{tools.SearchPlaces}) || !reflect.DeepEqual(p.NguonGoc.Nguon, []truyhoi.Nguon{truyhoi.Places}) {
		t.Fatalf("%s", r.kq.Phan[1].JSON)
	}
	// The verifier read the released sentences (names, not tokens) against
	// the cited evidence only.
	kiem := string(r.stub.YeuCau()[3])
	if !strings.Contains(kiem, "1. BạnˆthửˆQuánˆMâyˆnhé") || strings.Contains(kiem, "[[p:") || !strings.Contains(kiem, "e2 | gia: 120000") || strings.Contains(kiem, "Nhà") {
		t.Fatalf("verifier request:\n%s", kiem)
	}
	// The answer step read the question datamarked and the grader's verdict
	// as enums.
	ans := string(r.stub.YeuCau()[2])
	for _, w := range []string{"tốiˆmaiˆđi", "ket_luan: du", "di_ung: 3"} {
		if !strings.Contains(ans, w) {
			t.Errorf("answer request lacks %q", w)
		}
	}
	// The aliases of this turn's evidence are the schema's closed enum.
	if !regexp.MustCompile(`"enum": \[\s*"p1",\s*"p2"\s*\]`).MatchString(ans) {
		t.Errorf("answer schema does not offer the aliases:\n%s", ans)
	}
}

func TestNoiMemCoCauBao(t *testing.T) {
	r := chayVoi(t, truyhoi.Places, traCho, router, chamNoi, traLoiTot, kiemDat2)
	if !strings.HasPrefix(r.kq.Chu, cau.DaNoiLong([]string{"khi_chat"})+" Bạn thử") || r.kq.Crag.Buoc != crag.DaNoi {
		t.Fatalf("%q", r.kq.Chu)
	}
	if r.tim.da[1].Mem.KhiChat != nil || r.tim.da[1].Mem.LoaiCho == nil {
		t.Fatalf("%+v", r.tim.da[1].Mem)
	}
}

func TestHoiLaiVaTuChoiCoDinh(t *testing.T) {
	r := chayVoi(t, truyhoi.Places, traCho, router, chamThieu, traLoiHoiLai)
	if r.kq.Chu != cau.HoiLai([]string{"di_ung", "ngan_sach", "khi_chat"}) ||
		!reflect.DeepEqual(r.kq.LuaChon, []truyhoi.RangBuoc{truyhoi.RBNganSach, truyhoi.RBKhiChat}) {
		t.Fatalf("%q %v", r.kq.Chu, r.kq.LuaChon)
	}
	r = chayVoi(t, truyhoi.Places, rong, router, chamThieu)
	if r.kq.Chu != cau.TuChoi([]string{"di_ung", "ngan_sach"}) || r.kq.Vet.SoBan != 0 {
		t.Fatalf("%q", r.kq.Chu)
	}
	// Nothing found and the grader skipped: the removal counts name it.
	r = chayVoi(t, truyhoi.Places, rong, 6)
	if r.kq.KetThuc != DaTuChoi || r.kq.Chu != cau.TuChoi([]string{"di_ung", "ngan_sach"}) || r.stub.SoGoi() != 6 {
		t.Fatalf("%+v", r.kq)
	}
}

func TestSinhLaiMangPhatHien(t *testing.T) {
	r := chayVoi(t, truyhoi.Places, traCho, router, chamDu, traLoiLech, traLoiTot, kiemDat2)
	ph := r.kq.Vet.PhatHien[0]
	if !reflect.DeepEqual(ph.TrichLech, []TrichLech{{Cau: 1, BiDanh: "p1", Truong: TruongGia}}) || !reflect.DeepEqual(ph.BiDanhLa, []int{2}) || r.kq.Vet.ViPham[0] != 2 {
		t.Fatalf("%+v", ph)
	}
	// The regeneration carried the findings as data: indices, aliases and
	// enums, never text.
	sua := string(r.stub.YeuCau()[3])
	if !strings.Contains(sua, `<du_lieu nguon=\"phat_hien\">`) || !strings.Contains(sua, `{\"cau_co_token_la\":[2],\"gia_tri_lech\":[{\"cau\":1,\"bi_danh\":\"p1\",\"truong\":\"gia\"}]}`) {
		t.Fatalf("regeneration request:\n%s", sua)
	}
	if r.kq.Vet.SoKiem != 1 {
		t.Fatal("the verifier ran on a draft the deterministic checks refused")
	}
}

func TestKiemChungBatGiaVaChuyenTien(t *testing.T) {
	r := chayVoi(t, truyhoi.Places, traCho, router, chamDu, traLoiGiaBia, kiemGia, traLoiTot, kiemDat2)
	if !reflect.DeepEqual(r.kq.Vet.PhatHien[0].KhongHoTro, []int{1}) || r.kq.KetThuc != DaTraLoi {
		t.Fatalf("%+v", r.kq.Vet)
	}
	if strings.Contains(r.kq.Chu, "50") {
		t.Fatalf("the unsupported price was released: %q", r.kq.Chu)
	}
	r = chayVoi(t, truyhoi.Places, traCho, router, chamDu, traLoiChuyenTien, kiemTien, traLoiChuyenTien, kiemTien)
	if r.kq.KetThuc != DuPhong || r.kq.Chu != cau.DuPhong || !r.kq.Vet.PhatHien[1].Tien || !r.kq.Vet.PhatHien[1].HuaHanhDong {
		t.Fatalf("%+v", r.kq)
	}
	if strings.Contains(r.kq.Chu, "chuyển") {
		t.Fatal("the claim was released")
	}
	// The fallback's cards are the retrieval's own items.
	var p struct {
		IDs []string `json:"ids"`
	}
	_ = json.Unmarshal(r.kq.Phan[1].JSON, &p)
	if !reflect.DeepEqual(p.IDs, []string{"quan-an-a", "quan-an-b"}) {
		t.Fatalf("%s", r.kq.Phan[1].JSON)
	}
}

func TestTokenLaHaiLanThanhMotCho(t *testing.T) {
	r := chayVoi(t, truyhoi.Places, traCho, router, chamDu, traLoiTokenLa, traLoiTokenLa, kiemDat1)
	if r.kq.Chu != "Bạn thử Quán Mây hoặc "+cau.MotCho+"." || r.kq.Vet.TokenLa != 1 {
		t.Fatalf("%q %+v", r.kq.Chu, r.kq.Vet)
	}
}

// Whatever the turn spent before, the step never takes the turn past
// MaxModelCallsPerTurn and never fails for budget: it skips the grade, the
// regeneration or the answer and falls back.
func TestKhongVuotNganSach(t *testing.T) {
	// Every draft fails and every verdict refuses: the most calls a step
	// can want.
	xau := []string{chamDu, traLoiChuyenTien, kiemTien, traLoiChuyenTien, kiemTien, traLoiChuyenTien, kiemTien}
	for daGoi := 0; daGoi <= llm.MaxModelCallsPerTurn; daGoi++ {
		r := chayVoi(t, truyhoi.Places, traCho, daGoi, xau...)
		if r.err != nil {
			t.Fatalf("spent %d: %v", daGoi, r.err)
		}
		if r.dem.SoGoi() > llm.MaxModelCallsPerTurn {
			t.Fatalf("spent %d: turn made %d calls", daGoi, r.dem.SoGoi())
		}
		if r.kq.KetThuc != DuPhong {
			t.Fatalf("spent %d: ended %s", daGoi, r.kq.KetThuc)
		}
		if daGoi <= llm.MaxModelCallsPerTurn-5 && r.kq.Vet.SoGoi != 5 {
			t.Fatalf("spent %d: step made %d calls, want 5", daGoi, r.kq.Vet.SoGoi)
		}
	}
}

func TestDocTuChoi(t *testing.T) {
	bi := []string{"p1", "p2"}
	ok := []string{traLoiTot, traLoiHoiLai, traLoiTuChoi, traLoiTokenLa}
	for _, s := range ok {
		if _, err := Doc([]byte(s), bi); err != nil {
			t.Errorf("%s: %v", s, err)
		}
	}
	dai := strings.Repeat("a", MaxRuneCau+1)
	bad := map[string]string{
		"unknown action":          `{"hanh_dong":"dat_ban","rang_buoc_khong_dat":[],"cau":[]}`,
		"answer with no sentence": `{"hanh_dong":"tra_loi","rang_buoc_khong_dat":[],"cau":[]}`,
		"refusal naming nothing":  `{"hanh_dong":"tu_choi","rang_buoc_khong_dat":[],"cau":[]}`,
		"refusal with prose":      `{"hanh_dong":"tu_choi","rang_buoc_khong_dat":["di_ung"],"cau":[{"chu":"x","bang_chung":[],"trich":[]}]}`,
		"unknown constraint":      `{"hanh_dong":"hoi_lai","rang_buoc_khong_dat":["gia"],"cau":[]}`,
		"alias not offered":       `{"hanh_dong":"tra_loi","rang_buoc_khong_dat":[],"cau":[{"chu":"x","bang_chung":["p3"],"trich":[]}]}`,
		"real id cited":           `{"hanh_dong":"tra_loi","rang_buoc_khong_dat":[],"cau":[{"chu":"x","bang_chung":["quan-an-a"],"trich":[]}]}`,
		"alias twice":             `{"hanh_dong":"tra_loi","rang_buoc_khong_dat":[],"cau":[{"chu":"x","bang_chung":["p1","p1"],"trich":[]}]}`,
		"unknown field of value":  `{"hanh_dong":"tra_loi","rang_buoc_khong_dat":[],"cau":[{"chu":"x","bang_chung":[],"trich":[{"bi_danh":"p1","truong":"dia_chi","gia_tri":"x"}]}]}`,
		"empty value":             `{"hanh_dong":"tra_loi","rang_buoc_khong_dat":[],"cau":[{"chu":"x","bang_chung":[],"trich":[{"bi_danh":"p1","truong":"gia","gia_tri":""}]}]}`,
		"empty sentence":          `{"hanh_dong":"tra_loi","rang_buoc_khong_dat":[],"cau":[{"chu":"  ","bang_chung":[],"trich":[]}]}`,
		"sentence too long":       `{"hanh_dong":"tra_loi","rang_buoc_khong_dat":[],"cau":[{"chu":"` + dai + `","bang_chung":[],"trich":[]}]}`,
		"missing field":           `{"hanh_dong":"tra_loi","cau":[{"chu":"x","bang_chung":[],"trich":[]}]}`,
		"extra field":             `{"hanh_dong":"tra_loi","rang_buoc_khong_dat":[],"cau":[{"chu":"x","bang_chung":[],"trich":[]}],"ly_do":"x"}`,
		"trailing data":           traLoiTot + `{}`,
	}
	for ten, s := range bad {
		if _, err := Doc([]byte(s), bi); err == nil {
			t.Errorf("%s: accepted", ten)
		}
	}
}

func TestTachCau(t *testing.T) {
	p := tachCau("Thử [[p:p1]] rồi bấm «Tạo kèo» [[m:m1]] [[p:]] và [[p:p2")
	if !reflect.DeepEqual(p.biDanh, []string{"p1"}) || p.hong != 3 || !reflect.DeepEqual(p.nhan, []string{"Tạo kèo"}) {
		t.Fatalf("%+v", p)
	}
	chu, la := p.ghep(func(a string) (string, bool) { return "«" + a + "»", a == "p1" })
	if chu != "Thử «p1» rồi bấm «Tạo kèo» và" || la != 0 {
		t.Fatalf("%q %d", chu, la)
	}
}

// The verifier interface is what the step depends on: a scripted one proves
// the step honours its verdict without a model.
type kiemGiaLap struct{ p kiemchung.PhanTu }

func (k kiemGiaLap) PhanTu(context.Context, []string, []truyhoi.BangChung, *llm.Dem) (kiemchung.PhanTu, error) {
	return k.p, nil
}

func TestVerifierTienChanPhat(t *testing.T) {
	stub := llm.NewStub(llm.Buoc{Text: chamDu}, llm.Buoc{Text: traLoiTot}, llm.Buoc{Text: traLoiTot})
	dem := llm.NewDem(stub, llm.MaxModelCallsPerTurn, nil)
	kq, err := Chay(context.Background(), Vao{Cau: "x", YeuCau: yeuCau(truyhoi.Places)},
		BoPhan{BoPhan: crag.BoPhan{Tim: &timGia{tra: traCho}, Cham: crag.ChamLLM{}}, Verifier: kiemGiaLap{kiemchung.PhanTu{Tien: true}}},
		tools.MoiSoCai(obs.BotNep), dem)
	if err != nil || kq.KetThuc != DuPhong || kq.Vet.SoKiem != 2 {
		t.Fatalf("%v %+v", err, kq)
	}
}

// The privacy format check runs last on the whole released text, after the
// tokens became names: a catalogue name carrying a phone number stops the
// answer even though the model wrote none.
func TestDinhDangLaKiemCuoi(t *testing.T) {
	sdt := truyhoi.BangChung{ID: "quan-an-a", Nguon: truyhoi.Places, Truong: map[string]string{"ten": "Quán Mây " + soGia, "gia": "80000"}}
	tra := func(truyhoi.YeuCau) truyhoi.KetQuaTruyHoi {
		return truyhoi.KetQuaTruyHoi{BangChung: []truyhoi.BangChung{sdt, quanB}}
	}
	r := chayVoi(t, truyhoi.Places, tra, router, chamDu, traLoiTot, kiemDat2)
	if r.err != nil || r.kq.KetThuc != BiChan || r.kq.Chu != cau.Cau(cau.TraLoiBiChan) || len(r.kq.Phan) != 1 {
		t.Fatalf("%v %+v", r.err, r.kq)
	}
	nguonGocMoiPhan(t, r.kq.Phan)
}

// kiemBan finds each kind of grounding fault by set membership and exact
// equality, sentence by sentence.
func TestKiemBan(t *testing.T) {
	sc := tools.MoiSoCai(obs.BotNep)
	sc.Ghi(tools.SearchPlaces, []truyhoi.BangChung{quanA, quanB})
	sc.Ghi(tools.SearchAppManual, []truyhoi.BangChung{muc1})
	n := Nhap{HanhDong: TraLoi, Cau: []Cau{
		{Chu: "[[p:p1]] mở 07:00-22:00.", Trich: []Trich{{"p1", TruongGio, "07:00-22:00"}}},
		{Chu: "[[p:p1]] tầm 80k.", Trich: []Trich{{"p1", TruongGia, "80000"}}},
		{Chu: "[[p:p2]] giá 80000.", Trich: []Trich{{"p1", TruongGia, "80000"}}},
		{Chu: "[[p:p2]] giá 99000.", Trich: []Trich{{"p2", TruongGia, "99000"}}},
		{Chu: "Bấm «Tạo kèo» rồi «Chuyển tiền».", BangChung: []string{"m1"}},
		{Chu: "Hoặc [[p:p5]] và [[p:p2", BangChung: []string{"p2"}},
		{Chu: "Email mình: " + thuGia, BangChung: []string{"p2"}},
	}}
	b := kiemBan(n, sc)
	want := PhatHien{
		BiDanhLa:  []int{6},
		TokenHong: []int{6},
		TrichLech: []TrichLech{{Cau: 4, BiDanh: "p2", Truong: TruongGia}},
		TrichXa:   []int{2, 3},
		NhanNut:   []int{5},
		DinhDang:  []int{7},
	}
	if !reflect.DeepEqual(b.ph, want) {
		t.Fatalf("got  %+v\nwant %+v", b.ph, want)
	}
	if b.cau[0] != "Quán Mây mở 07:00-22:00." || b.cau[5] != "Hoặc một chỗ và" || b.tokenLa != 1 || b.tokenHong != 1 {
		t.Fatalf("%q %d %d", b.cau, b.tokenLa, b.tokenHong)
	}
	if !reflect.DeepEqual(b.ids, []string{"quan-an-a", "quan-an-b", "huong-dan-tao-keo"}) {
		t.Fatalf("%v", b.ids)
	}
}

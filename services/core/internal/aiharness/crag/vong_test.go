package crag

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"mobile/services/core/internal/aiharness/cau"
	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/obs"
	"mobile/services/core/internal/aiharness/tools"
	"mobile/services/core/internal/aiharness/truyhoi"
)

// timGia is a scripted retriever: its answer is chosen by the request's
// query text and soft preferences, and every request is recorded, so a test
// can assert the hard constraints reached it unchanged.
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

func cho(id, ten string) truyhoi.BangChung {
	return truyhoi.BangChung{ID: id, Nguon: truyhoi.Places, Truong: map[string]string{"ten": ten, "gia": "80000", "gio": "07:00-22:00"}}
}

// yeuCau is a request with every hard constraint set and all three soft
// preferences.
func yeuCau() truyhoi.YeuCau {
	n := int64(200000)
	mo := time.Date(2026, 9, 26, 19, 0, 0, 0, time.FixedZone("ICT", 7*3600))
	return truyhoi.YeuCau{Nguon: truyhoi.Places, Cau: "cafe yên tĩnh",
		Cung: truyhoi.Cung{DiemDenID: "da-lat", DiUng: []string{"tom"}, AnKieng: []string{"chay"}, MoLuc: &mo, NganSachVND: &n},
		Mem:  truyhoi.Mem{LoaiCho: []string{"cafe"}, KhiChat: []string{"yen_tinh"}, KhuVuc: "ho-xuan-huong"}}
}

func demMoi(kich ...llm.Buoc) (*llm.Dem, *llm.Stub) {
	s := llm.NewStub(kich...)
	return llm.NewDem(s, llm.MaxModelCallsPerTurn, nil).WithWait(func(int) time.Duration { return 0 }), s
}

func TestTruyHoiNhanh(t *testing.T) {
	a, b, c := cho("quan-an-a", "Quán A"), cho("quan-an-b", "Quán B"), cho("quan-an-c", "Quán C")
	motCho := func(y truyhoi.YeuCau) truyhoi.KetQuaTruyHoi {
		if y.Mem.KhiChat != nil && y.Cau == "cafe yên tĩnh" {
			return truyhoi.KetQuaTruyHoi{BangChung: []truyhoi.BangChung{a}, BiLoai: map[truyhoi.RangBuoc]int{truyhoi.RBDiUng: 4}}
		}
		return truyhoi.KetQuaTruyHoi{BangChung: []truyhoi.BangChung{a, b, c}}
	}
	cases := []struct {
		ten      string
		cham     string
		soTim    int
		buoc     Buoc
		chamHong bool
		cau      string
		noi      []truyhoi.RangBuoc
		soBang   int
	}{
		{"du", `{"ket_luan":"du","rang_buoc_thieu":[]}`, 1, KhongSua, false, "cafe yên tĩnh", nil, 1},
		{"relax soft", `{"ket_luan":"thieu","rang_buoc_thieu":["khi_chat"],"noi_long":["khi_chat"]}`, 2, DaNoi, false, "cafe yên tĩnh", []truyhoi.RangBuoc{truyhoi.RBKhiChat}, 3},
		{"rewrite", `{"ket_luan":"thieu","rang_buoc_thieu":["loai_cho"],"viet_lai":"quán cà phê đồi thông"}`, 2, DaVietLai, false, "quán cà phê đồi thông", nil, 3},
		{"no move", `{"ket_luan":"thieu","rang_buoc_thieu":["di_ung"]}`, 1, KhongSua, false, "cafe yên tĩnh", nil, 1},
		{"grader asks to relax an allergy", `{"ket_luan":"thieu","rang_buoc_thieu":["di_ung"],"noi_long":["di_ung"]}`, 1, KhongSua, true, "cafe yên tĩnh", nil, 1},
		{"grader asks to relax the budget", `{"ket_luan":"thieu","rang_buoc_thieu":["ngan_sach"],"noi_long":["ngan_sach"]}`, 1, KhongSua, true, "cafe yên tĩnh", nil, 1},
		{"out of order", `{"ket_luan":"thieu","rang_buoc_thieu":["loai_cho"],"noi_long":["loai_cho"]}`, 1, KhongSua, true, "cafe yên tĩnh", nil, 1},
		{"grader garbage", `ok`, 1, KhongSua, true, "cafe yên tĩnh", nil, 1},
	}
	for _, tc := range cases {
		t.Run(tc.ten, func(t *testing.T) {
			tim := &timGia{tra: motCho}
			dem, stub := demMoi(llm.Buoc{Text: tc.cham})
			sc := tools.MoiSoCai(obs.BotNep)
			y := yeuCau()
			kq, err := TruyHoi(context.Background(), y, BoPhan{Tim: tim, Cham: ChamLLM{}}, tools.SearchPlaces, sc, dem)
			if err != nil {
				t.Fatal(err)
			}
			if len(tim.da) != tc.soTim || kq.SoTruyHoi != tc.soTim || sc.SoGoi(tools.SearchPlaces) != tc.soTim {
				t.Fatalf("retrievals %d/%d/%d, want %d", len(tim.da), kq.SoTruyHoi, sc.SoGoi(tools.SearchPlaces), tc.soTim)
			}
			// The hard constraints of every request equal the first one's.
			for _, d := range tim.da {
				if !reflect.DeepEqual(d.Cung, y.Cung) {
					t.Fatalf("hard constraints moved: %+v", d.Cung)
				}
			}
			if !reflect.DeepEqual(kq.YeuCau.Cung, y.Cung) {
				t.Fatalf("result's hard constraints moved")
			}
			if kq.Buoc != tc.buoc || kq.ChamHong != tc.chamHong || kq.YeuCau.Cau != tc.cau || !reflect.DeepEqual(kq.NoiLong, tc.noi) || len(kq.BangChung) != tc.soBang {
				t.Fatalf("%+v", kq)
			}
			if tc.buoc != KhongSua && kq.Vong != 1 {
				t.Fatalf("rounds %d", kq.Vong)
			}
			if stub.SoGoi() != 1 || dem.SoGoi() != 1 {
				t.Fatalf("model calls %d", stub.SoGoi())
			}
			// The grader saw aliases, never a real id, and data blocks.
			req := string(stub.YeuCau()[0])
			if strings.Contains(req, "quan-an-a") || !strings.Contains(req, "p1 | gia: 80000") || !strings.Contains(req, `<du_lieu nguon=\"bang_chung\">`) {
				t.Fatalf("grader request:\n%s", req)
			}
			if !strings.Contains(req, "di_ung: 4") {
				t.Fatalf("grader did not see the removal counts:\n%s", req)
			}
		})
	}
}

// A grader that hands back a hard name in NoiLong without passing Doc (a
// bug, or another Cham) still cannot widen a hard filter.
type chamCung struct{}

func (chamCung) DanhGia(context.Context, Vao, *llm.Dem) (DanhGia, error) {
	return DanhGia{KetLuan: Thieu, RangBuocThieu: []truyhoi.RangBuoc{truyhoi.RBDiUng}, NoiLong: []truyhoi.RangBuoc{truyhoi.RBDiUng}}, nil
}

func TestChamLenhNoiCungKhongChay(t *testing.T) {
	tim := &timGia{tra: func(truyhoi.YeuCau) truyhoi.KetQuaTruyHoi { return truyhoi.KetQuaTruyHoi{} }}
	dem, _ := demMoi()
	y := yeuCau()
	kq, err := TruyHoi(context.Background(), y, BoPhan{Tim: tim, Cham: chamCung{}}, tools.SearchPlaces, tools.MoiSoCai(obs.BotNep), dem)
	if err != nil {
		t.Fatal(err)
	}
	if len(tim.da) != 1 || kq.Vong != 0 || !kq.ChamHong || !reflect.DeepEqual(tim.da[0].Cung, y.Cung) {
		t.Fatalf("a hard relaxation ran: %d retrievals, %+v", len(tim.da), kq)
	}
}

func TestKiemThuTu(t *testing.T) {
	y := yeuCau()
	ok := [][]truyhoi.RangBuoc{
		{truyhoi.RBKhiChat},
		{truyhoi.RBKhiChat, truyhoi.RBLoaiCho},
		{truyhoi.RBLoaiCho, truyhoi.RBKhiChat},
		{truyhoi.RBKhiChat, truyhoi.RBLoaiCho, truyhoi.RBKhuVuc},
	}
	for _, n := range ok {
		if err := KiemThuTu(y, DanhGia{KetLuan: Thieu, NoiLong: n}); err != nil {
			t.Errorf("%v: %v", n, err)
		}
	}
	bad := [][]truyhoi.RangBuoc{{truyhoi.RBLoaiCho}, {truyhoi.RBKhuVuc}, {truyhoi.RBKhiChat, truyhoi.RBKhuVuc}}
	for _, n := range bad {
		if err := KiemThuTu(y, DanhGia{KetLuan: Thieu, NoiLong: n}); !errors.Is(err, ErrThuTuNoi) {
			t.Errorf("%v accepted", n)
		}
	}
	// An earlier preference that was never set does not hold a later one.
	y.Mem.KhiChat = nil
	if err := KiemThuTu(y, DanhGia{KetLuan: Thieu, NoiLong: []truyhoi.RangBuoc{truyhoi.RBLoaiCho}}); err != nil {
		t.Error(err)
	}
}

// With fewer than DuTruCham calls left the grade is skipped: no call.
func TestBoChamKhiHetNganSach(t *testing.T) {
	s := llm.NewStub()
	dem := llm.NewDem(s, DuTruCham-1, nil)
	tim := &timGia{tra: func(truyhoi.YeuCau) truyhoi.KetQuaTruyHoi {
		return truyhoi.KetQuaTruyHoi{BangChung: []truyhoi.BangChung{cho("x", "X")}}
	}}
	kq, err := TruyHoi(context.Background(), yeuCau(), BoPhan{Tim: tim, Cham: ChamLLM{}}, tools.SearchPlaces, tools.MoiSoCai(obs.BotNep), dem)
	if err != nil || !kq.BoCham || s.SoGoi() != 0 || kq.DanhGia != nil {
		t.Fatalf("%v %+v calls %d", err, kq, s.SoGoi())
	}
}

type xepGia struct {
	n    int
	them bool
	loi  error
}

func (x *xepGia) XepLai(_ context.Context, _ string, bc []truyhoi.BangChung, topN int) ([]truyhoi.BangChung, error) {
	x.n++
	if x.loi != nil {
		return nil, x.loi
	}
	out := append([]truyhoi.BangChung(nil), bc...)
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	if x.them {
		out = append(out, cho("bia", "Bịa"))
	}
	return out, nil
}

func TestXepLai(t *testing.T) {
	a, b := cho("a", "A"), cho("b", "B")
	tra := func(truyhoi.YeuCau) truyhoi.KetQuaTruyHoi {
		return truyhoi.KetQuaTruyHoi{BangChung: []truyhoi.BangChung{a, b}}
	}
	// A working reranker orders the evidence and leaves no flag.
	x := &xepGia{}
	dem, _ := demMoi(llm.Buoc{Text: `{"ket_luan":"thieu","rang_buoc_thieu":[],"viet_lai":"khác"}`})
	kq, err := TruyHoi(context.Background(), yeuCau(), BoPhan{Tim: &timGia{tra: tra}, XepLai: x, Cham: ChamLLM{}}, tools.SearchPlaces, tools.MoiSoCai(obs.BotNep), dem)
	if err != nil || kq.BangChung[0].ID != "b" || len(kq.Degraded) != 0 || x.n != 2 || kq.SoXepLai != 2 {
		t.Fatalf("%v %+v", err, kq)
	}
	// One that adds an item is refused: order kept, NoRerank flagged, and
	// the invented item never reaches the ledger.
	x = &xepGia{them: true}
	sc := tools.MoiSoCai(obs.BotNep)
	dem, _ = demMoi(llm.Buoc{Text: `{"ket_luan":"du","rang_buoc_thieu":[]}`})
	kq, err = TruyHoi(context.Background(), yeuCau(), BoPhan{Tim: &timGia{tra: tra}, XepLai: x, Cham: ChamLLM{}}, tools.SearchPlaces, sc, dem)
	if err != nil || kq.BangChung[0].ID != "a" || len(kq.BangChung) != 2 || !reflect.DeepEqual(kq.Degraded, []truyhoi.CoSuyGiam{truyhoi.NoRerank}) || sc.Co("bia") {
		t.Fatalf("%v %+v", err, kq)
	}
	// No reranker: Passthrough, flagged.
	dem, _ = demMoi(llm.Buoc{Text: `{"ket_luan":"du","rang_buoc_thieu":[]}`})
	kq, _ = TruyHoi(context.Background(), yeuCau(), BoPhan{Tim: &timGia{tra: tra}, Cham: ChamLLM{}}, tools.SearchPlaces, tools.MoiSoCai(obs.BotNep), dem)
	if !reflect.DeepEqual(kq.Degraded, []truyhoi.CoSuyGiam{truyhoi.NoRerank}) || kq.SoXepLai != 0 {
		t.Fatalf("%+v", kq)
	}
}

// Every constraint name the grader or the answer step can report has a
// fixed phrase, so a refusal never names a constraint by its id.
func TestMoiRangBuocCoCum(t *testing.T) {
	for _, r := range RangBuocs.Values() {
		if cau.CumRangBuoc(r) == "" {
			t.Errorf("%s has no phrase", r)
		}
	}
	got := cau.TuChoi([]string{"di_ung", "mo_luc", "ngan_sach"})
	if !strings.Contains(got, "yêu cầu tránh món bạn dị ứng, giờ mở cửa bạn cần và ngân sách bạn đặt") {
		t.Fatal(got)
	}
	if cau.TuChoi(nil) != cau.KhongThay || cau.DaNoiLong(nil) != "" {
		t.Fatal("empty lists")
	}
}

// The reranker budget is the turn's: two loops sharing it make
// MaxRerankCallsPerTurn reranker calls between them, and the rest of the
// retrievals go through Passthrough, flagged.
func TestNganSachXepLaiTheoLuot(t *testing.T) {
	tra := func(truyhoi.YeuCau) truyhoi.KetQuaTruyHoi {
		return truyhoi.KetQuaTruyHoi{BangChung: []truyhoi.BangChung{cho("a", "A"), cho("b", "B")}}
	}
	x := &xepGia{}
	ns := &NganSachXepLai{}
	sc := tools.MoiSoCai(obs.BotNhom)
	var flags [][]truyhoi.CoSuyGiam
	for i := 0; i < 2; i++ {
		dem, _ := demMoi(llm.Buoc{Text: `{"ket_luan":"thieu","rang_buoc_thieu":[],"viet_lai":"khác"}`})
		kq, err := TruyHoi(context.Background(), yeuCau(), BoPhan{Tim: &timGia{tra: tra}, XepLai: x, Cham: ChamLLM{}, NganSach: ns}, tools.SearchPlaces, sc, dem)
		if err != nil {
			t.Fatal(err)
		}
		flags = append(flags, kq.Degraded)
	}
	if x.n != llm.MaxRerankCallsPerTurn || ns.SoGoi() != llm.MaxRerankCallsPerTurn {
		t.Fatalf("reranker calls %d/%d", x.n, ns.SoGoi())
	}
	if len(flags[0]) != 0 || !reflect.DeepEqual(flags[1], []truyhoi.CoSuyGiam{truyhoi.NoRerank}) {
		t.Fatalf("%v", flags)
	}
}

// A corrective round that finds nothing leaves the first evidence standing:
// the round never makes the answer worse than no round.
func TestVongRongGiuBangChungDau(t *testing.T) {
	a := cho("a", "A")
	tim := &timGia{tra: func(y truyhoi.YeuCau) truyhoi.KetQuaTruyHoi {
		if y.Cau == "cafe yên tĩnh" {
			return truyhoi.KetQuaTruyHoi{BangChung: []truyhoi.BangChung{a}}
		}
		return truyhoi.KetQuaTruyHoi{}
	}}
	dem, _ := demMoi(llm.Buoc{Text: `{"ket_luan":"thieu","rang_buoc_thieu":[],"viet_lai":"không có gì"}`})
	y := yeuCau()
	kq, err := TruyHoi(context.Background(), y, BoPhan{Tim: tim, Cham: ChamLLM{}}, tools.SearchPlaces, tools.MoiSoCai(obs.BotNep), dem)
	if err != nil || kq.Vong != 1 || len(tim.da) != 2 || len(kq.BangChung) != 1 || kq.YeuCau.Cau != y.Cau {
		t.Fatalf("%v %+v", err, kq)
	}
}

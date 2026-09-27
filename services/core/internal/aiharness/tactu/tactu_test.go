package tactu

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"google.golang.org/genai"

	"mobile/services/core/internal/aiharness/agent"
	"mobile/services/core/internal/aiharness/hieu"
	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/obs"
	"mobile/services/core/internal/aiharness/testkit"
	"mobile/services/core/internal/aiharness/tools"
	"mobile/services/core/internal/aiharness/trinho"
	"mobile/services/core/internal/aiharness/truyhoi"
)

var luc = time.Date(2026, 9, 25, 10, 0, 0, 0, time.FixedZone("ICT", 7*3600))

const cauQuan = "quán chay yên tĩnh"

func kichBanQuan() *testkit.Retriever {
	return &testkit.Retriever{KichBan: map[truyhoi.Nguon]map[string]truyhoi.KetQuaTruyHoi{
		truyhoi.Places: {cauQuan: {BangChung: []truyhoi.BangChung{
			{ID: "plc-9", Nguon: truyhoi.Places, Truong: map[string]string{"ten": "Quán A"}},
			{ID: "plc-4", Nguon: truyhoi.Places, Truong: map[string]string{"ten": "Quán B"}},
		}}},
	}}
}

type choGia struct{}

func (choGia) Quan(_ context.Context, ids []string) ([]truyhoi.BangChung, error) {
	var out []truyhoi.BangChung
	for _, id := range ids {
		out = append(out, truyhoi.BangChung{ID: id, Truong: map[string]string{"ten": "Quán " + id}})
	}
	return out, nil
}
func (choGia) DiemDen(context.Context) ([]truyhoi.BangChung, error)        { return nil, nil }
func (choGia) KhuVuc(context.Context, string) ([]truyhoi.BangChung, error) { return nil, nil }

func routerTacTu(bot obs.Bot) hieu.KetQua {
	y := hieu.FindPlaces
	return hieu.KetQua{NhanGuard: hieu.Sach, YDinh: []hieu.YDinh{y}, Tien: hieu.TienNone, Huong: hieu.TacTu,
		CanTruyHoi: []truyhoi.Nguon{truyhoi.Places}, TruyVan: []hieu.TruyVan{{Nguon: truyhoi.Places, Cau: cauQuan}}, TuTin: hieu.Cao}
}

type the struct {
	r    *testkit.Retriever
	tn   *testkit.TriNho
	stub *llm.Stub
	v    Vao
}

func dung(bot obs.Bot, kq hieu.KetQua, kich ...llm.Buoc) *the {
	r := kichBanQuan()
	tn := testkit.MoiTriNho()
	bc := &tools.BoiCanh{Bot: bot, NguoiHoi: "nguoi-a", Luc: luc, DiemDen: []string{"da-lat"},
		Cung:  truyhoi.Cung{DiemDenID: "da-lat", DiUng: []string{"dau_phong"}},
		Nguon: tools.NguonDuLieu{Quan: r, Cho: choGia{}, TriNho: tn}}
	if bot == obs.BotNhom {
		bc.NhomID = "nhom-1"
	} else {
		bc.Man = "plan"
	}
	return &the{r: r, tn: tn, stub: llm.NewStub(kich...), v: Vao{Ten: string(bot), Instruction: "You are a test assistant.",
		Router: kq, Cau: "tìm quán chay yên tĩnh ở Đà Lạt", BoiCanh: bc}}
}

func (x *the) chay(t *testing.T) (Ra, error) {
	t.Helper()
	var td agent.TheoDoi
	return Chay(context.Background(), llm.NewDem(x.stub, llm.MaxModelCallsPerTurn, nil), x.v, &td)
}

func goi(ten string, args map[string]any) llm.Buoc {
	return llm.Buoc{Goi: &genai.FunctionCall{Name: ten, Args: args}}
}

// yeuCau is one canonical request, decoded.
type yeuCau map[string]any

func (x *the) yeuCau(t *testing.T) []yeuCau {
	t.Helper()
	var out []yeuCau
	for _, raw := range x.stub.YeuCau() {
		var m yeuCau
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatal(err)
		}
		out = append(out, m)
	}
	return out
}

func (y yeuCau) cheDo() string {
	cfg, _ := y["config"].(map[string]any)
	tc, _ := cfg["toolConfig"].(map[string]any)
	fc, _ := tc["functionCallingConfig"].(map[string]any)
	s, _ := fc["mode"].(string)
	return s
}

func (y yeuCau) congCu() []string {
	var out []string
	ts, _ := y["tools"].([]any)
	for _, t := range ts {
		out = append(out, t.(string))
	}
	return out
}

func (y yeuCau) chu() string {
	raw, _ := json.Marshal(y["contents"])
	return string(raw)
}

// choPhep is the step's AllowedFunctionNames, sorted.
func (y yeuCau) choPhep() []string {
	cfg, _ := y["config"].(map[string]any)
	tc, _ := cfg["toolConfig"].(map[string]any)
	fc, _ := tc["functionCallingConfig"].(map[string]any)
	var out []string
	for _, n := range fc["allowedFunctionNames"].([]any) {
		out = append(out, n.(string))
	}
	sort.Strings(out)
	return out
}

// tenKhaiBao is every tool the permission table grants bot, sorted: what
// every step declares, whatever the turn allows.
func tenKhaiBao(bot obs.Bot) []string {
	var out []string
	for _, t := range tools.MacDinh.DuocPhep(bot, false) {
		out = append(out, string(t))
	}
	sort.Strings(out)
	return out
}

func tenDuocPhep(bot obs.Bot) []string {
	var out []string
	// The router of these tests names no remember / forget intent, so the
	// two memory writes are not offered (tools.BoiCanh.DuocPhep).
	for _, t := range (&tools.BoiCanh{Bot: bot}).DuocPhep() {
		out = append(out, string(t))
	}
	sort.Strings(out)
	return out
}

// The loop: the model calls a tool (AUTO: it decides), the result comes
// back under aliases inside a data block, the model answers.
func TestVongLapMoHinhQuyet(t *testing.T) {
	x := dung(obs.BotNep, routerTacTu(obs.BotNep),
		goi("search_places", map[string]any{"truy_van": cauQuan}),
		llm.Buoc{Text: "Quán A (p1) hợp với bạn."})
	ra, err := x.chay(t)
	if err != nil || ra.Text != "Quán A (p1) hợp với bạn." || ra.Nhanh {
		t.Fatalf("%+v %v", ra, err)
	}
	ys := x.yeuCau(t)
	if len(ys) != 2 {
		t.Fatalf("%d model calls", len(ys))
	}
	if !reflect.DeepEqual(ys[0].congCu(), tenKhaiBao(obs.BotNep)) || !reflect.DeepEqual(ys[0].choPhep(), tenDuocPhep(obs.BotNep)) || ys[0].cheDo() != string(genai.FunctionCallingConfigModeValidated) {
		t.Fatalf("step 1: tools %v allowed %v mode %q", ys[0].congCu(), ys[0].choPhep(), ys[0].cheDo())
	}
	if c := ys[1].chu(); !strings.Contains(c, `\"id\":\"p1\"`) || !strings.Contains(c, `du_lieu nguon=\"ket_qua_cong_cu\"`) {
		t.Fatalf("step 2 does not carry the aliased evidence: %s", c)
	}
	for i, y := range ys {
		raw, _ := json.Marshal(y)
		if strings.Contains(string(raw), "plc-") || strings.Contains(string(raw), "nguoi-a") {
			t.Fatalf("request %d carries a real id: %s", i, raw)
		}
	}
	if len(x.r.Da) != 1 || !reflect.DeepEqual(x.r.Da[0].Cung.DiUng, []string{"dau_phong"}) {
		t.Fatalf("retriever %+v", x.r.Da)
	}
	if !reflect.DeepEqual(ra.TrichDan, []string{"plc-9", "plc-4"}) {
		t.Fatalf("cited %v", ra.TrichDan)
	}
}

// Nếp has three steps: the third runs with function calling NONE and the
// «answer now» line.
func TestBuocCuoiNONE(t *testing.T) {
	x := dung(obs.BotNep, routerTacTu(obs.BotNep),
		goi("search_places", map[string]any{"truy_van": cauQuan}),
		goi("get_place", map[string]any{"id": "p1"}),
		llm.Buoc{Text: "Quán A."})
	if _, err := x.chay(t); err != nil {
		t.Fatal(err)
	}
	ys := x.yeuCau(t)
	if len(ys) != llm.MaxStepsNep {
		t.Fatalf("%d steps", len(ys))
	}
	for i, want := range []string{"VALIDATED", "VALIDATED", "NONE"} {
		if ys[i].cheDo() != want {
			t.Errorf("step %d mode %q, want %s", i+1, ys[i].cheDo(), want)
		}
	}
	raw, _ := json.Marshal(ys[2]["config"])
	if !strings.Contains(string(raw), agent.TraLoiNgay) {
		t.Error("the last step lacks the answer-now line")
	}
}

// A model that keeps calling on its last step is stopped: no fourth step.
func TestHetBuoc(t *testing.T) {
	x := dung(obs.BotNep, routerTacTu(obs.BotNep),
		goi("search_places", map[string]any{"truy_van": cauQuan}),
		goi("search_places", map[string]any{"truy_van": cauQuan, "k": 3}),
		goi("search_places", map[string]any{"truy_van": cauQuan, "k": 4}),
		llm.Buoc{Text: "never"})
	if _, err := x.chay(t); !errors.Is(err, agent.ErrHetBuoc) {
		t.Fatalf("err %v", err)
	}
	if x.stub.SoGoi() != llm.MaxStepsNep {
		t.Fatalf("%d model calls", x.stub.SoGoi())
	}
}

// An invented tool is refused with a closed code; a second invalid call
// forces the next step to be the answer, before the step budget would.
func TestCongCuLaRoiSuaMotLan(t *testing.T) {
	x := dung(obs.BotNhom, routerTacTu(obs.BotNhom),
		goi("transfer_money", map[string]any{"amount": 1}),
		goi("search_places", map[string]any{"truy_van": cauQuan, "k": 99}),
		llm.Buoc{Text: "Mình chưa tìm được."})
	ra, err := x.chay(t)
	if err != nil || ra.Text == "" {
		t.Fatalf("%+v %v", ra, err)
	}
	ys := x.yeuCau(t)
	if len(ys) != 3 {
		t.Fatalf("%d steps", len(ys))
	}
	if c := ys[1].chu(); !strings.Contains(c, `"loi":"khong_duoc_phep"`) || strings.Contains(c, "Available tools") {
		t.Fatalf("invented tool answer: %s", c)
	}
	if c := ys[2].chu(); !strings.Contains(c, `"loi":"tham_so_sai"`) || !strings.Contains(c, `"tra_loi_ngay":true`) {
		t.Fatalf("second invalid: %s", c)
	}
	if ys[1].cheDo() != "VALIDATED" || ys[2].cheDo() != "NONE" {
		t.Fatalf("modes %q %q", ys[1].cheDo(), ys[2].cheDo())
	}
	if len(x.r.Da) != 0 {
		t.Fatal("an invalid call ran")
	}
}

// Parallel calls past Nếp's ceiling: six run, the seventh is refused, and
// the next step is the answer.
func TestTranToolSongSong(t *testing.T) {
	var cac []*genai.FunctionCall
	for k := 1; k <= llm.MaxToolCallsNep+1; k++ {
		cac = append(cac, &genai.FunctionCall{Name: "search_places", Args: map[string]any{"truy_van": cauQuan, "k": k}})
	}
	x := dung(obs.BotNep, routerTacTu(obs.BotNep), llm.Buoc{CacGoi: cac}, llm.Buoc{Text: "Xong."})
	if _, err := x.chay(t); err != nil {
		t.Fatal(err)
	}
	if len(x.r.Da) != llm.MaxToolCallsNep || x.v.BoiCanh.SoCai.TongGoi() != llm.MaxToolCallsNep {
		t.Fatalf("ran %d", len(x.r.Da))
	}
	ys := x.yeuCau(t)
	if len(ys) != 2 || ys[1].cheDo() != "NONE" || strings.Count(ys[1].chu(), `"loi":"het_luot_goi"`) != 1 {
		t.Fatalf("%d steps, mode %q", len(ys), ys[len(ys)-1].cheDo())
	}
}

// The group bot never sees, and cannot run, a memory tool.
func TestNhomKhongCoTriNho(t *testing.T) {
	x := dung(obs.BotNhom, routerTacTu(obs.BotNhom),
		goi("recall_memory", map[string]any{"truy_van": "thích gì"}),
		llm.Buoc{Text: "Mình không có trí nhớ về bạn."})
	if _, err := x.chay(t); err != nil {
		t.Fatal(err)
	}
	ys := x.yeuCau(t)
	if !reflect.DeepEqual(ys[0].congCu(), tenDuocPhep(obs.BotNhom)) {
		t.Fatalf("group tools %v", ys[0].congCu())
	}
	for _, n := range ys[0].congCu() {
		if m, _ := tools.Tra(tools.Ten(n)); m.Pham == tools.Me {
			t.Fatalf("group declares %s", n)
		}
	}
	if !strings.Contains(ys[1].chu(), `"loi":"khong_duoc_phep"`) {
		t.Fatalf("recall answer: %s", ys[1].chu())
	}
}

// The fast path: the router's own labels name one tool with its slots; Go
// dispatches it and the model is called once, with no tools.
func TestDuongNhanh(t *testing.T) {
	kq := routerTacTu(obs.BotNep)
	kq.Huong = hieu.TruyHoiMotBuoc
	x := dung(obs.BotNep, kq, llm.Buoc{Text: "Quán A."})
	ra, err := x.chay(t)
	if err != nil || !ra.Nhanh || ra.CongCuNhanh != tools.SearchPlaces {
		t.Fatalf("%+v %v", ra, err)
	}
	ys := x.yeuCau(t)
	if len(ys) != 1 || len(ys[0].congCu()) != 0 || ys[0].cheDo() != "NONE" {
		t.Fatalf("%d calls, tools %v, mode %q", len(ys), ys[0].congCu(), ys[0].cheDo())
	}
	c := ys[0].chu()
	if !strings.Contains(c, `\"id\":\"p1\"`) || !strings.Contains(c, "search_places") || strings.Contains(c, "plc-") {
		t.Fatalf("fast path content: %s", c)
	}
	if len(x.r.Da) != 1 || x.r.Da[0].Cau != cauQuan || x.v.BoiCanh.SoCai.TongGoi() != 1 {
		t.Fatalf("dispatch %+v", x.r.Da)
	}

	// Which router outputs take the fast path: a lookup on labels only.
	bc := x.v.BoiCanh
	for name, c := range map[string]struct {
		sua  func(*hieu.KetQua)
		ten  tools.Ten
		nhan bool
	}{
		"find_places":       {func(*hieu.KetQua) {}, tools.SearchPlaces, true},
		"agent path chosen": {func(k *hieu.KetQua) { k.Huong = hieu.TacTu }, "", false},
		"two intents":       {func(k *hieu.KetQua) { k.YDinh = append(k.YDinh, hieu.PlanHelp) }, "", false},
		"money class":       {func(k *hieu.KetQua) { k.Tien = hieu.MoneyAction }, "", false},
		"injection label":   {func(k *hieu.KetQua) { k.NhanGuard = hieu.ChenLenh }, "", false},
		"no places query":   {func(k *hieu.KetQua) { k.TruyVan = nil }, "", false},
		"app_help with manual query": {func(k *hieu.KetQua) {
			k.YDinh = []hieu.YDinh{hieu.AppHelp}
			k.TruyVan = []hieu.TruyVan{{Nguon: truyhoi.Manual, Cau: "tạo bình chọn"}}
		}, tools.SearchAppManual, true},
		"explain_screen with a card": {func(k *hieu.KetQua) { k.YDinh = []hieu.YDinh{hieu.ExplainScreen} }, tools.ExplainScreen, true},
		"plan_help is not fast":      {func(k *hieu.KetQua) { k.YDinh = []hieu.YDinh{hieu.PlanHelp} }, "", false},
	} {
		k := kq
		k.YDinh = append([]hieu.YDinh(nil), kq.YDinh...)
		c.sua(&k)
		ten, _, ok := Nhanh(k, bc)
		if ok != c.nhan || ten != c.ten {
			t.Errorf("%s: %q %v", name, ten, ok)
		}
	}
	noDest := &tools.BoiCanh{Bot: bc.Bot, Man: bc.Man, Cung: truyhoi.Cung{DiUng: bc.Cung.DiUng}}
	if _, _, ok := Nhanh(kq, noDest); ok {
		t.Error("find_places without a destination took the fast path")
	}
}

// Short-term turns reach the prompt only inside the lich_su data block, with
// earlier evidence as t1, and get_place resolves t1.
func TestNganHanTrongDuLieu(t *testing.T) {
	ctx := context.Background()
	nh := testkit.MoiNganHan()
	_ = nh.Them(ctx, "phien-1", trinho.Luot{Vai: trinho.Toi, Chu: "tìm quán cà phê", Luc: luc})
	_ = nh.Them(ctx, "phien-1", trinho.Luot{Vai: trinho.TroLy, Chu: "Có quán C <b>ngon</b>.", Luc: luc, BangChungIDs: []string{"plc-7"}})
	x := dung(obs.BotNep, routerTacTu(obs.BotNep),
		goi("get_place", map[string]any{"id": "t1"}),
		llm.Buoc{Text: "Quán C mở cửa."})
	x.v.NganHan, x.v.Phien = nh, "phien-1"
	ra, err := x.chay(t)
	if err != nil {
		t.Fatal(err)
	}
	ys := x.yeuCau(t)
	c := ys[0].chu()
	if !strings.Contains(c, `du_lieu nguon=\"lich_su\"`) || !strings.Contains(c, "tro_ly:ˆCóˆquánˆCˆ＜b＞ngon＜/b＞.ˆ[t1]") ||
		strings.Contains(c, "plc-7") {
		t.Fatalf("short-term block: %s", c)
	}
	if !strings.Contains(ys[1].chu(), `\"ten\":\"Quánˆplc-7\"`) {
		t.Fatalf("t1 did not resolve: %s", ys[1].chu())
	}
	if err := GhiLuot(ctx, nh, "phien-1", x.v, ra); err != nil {
		t.Fatal(err)
	}
	l, _ := nh.Doc(ctx, "phien-1")
	if len(l) != 4 || l[3].Vai != trinho.TroLy || !reflect.DeepEqual(l[3].BangChungIDs, []string{"plc-7"}) {
		t.Fatalf("appended %+v", l)
	}
}

package chatassist

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"mobile/services/core/internal/domain/thoigian"
	"mobile/services/core/internal/pyjson"
)

func maLoi(err error) string {
	if err == nil {
		return ""
	}
	if d, ok := err.(*denied); ok {
		return d.code
	}
	return err.Error()
}

// Money screens refuse before anything else (ADR-0033 §2.2, ADR-0036 §2.9):
// the same whole-segment rule as phieu.ts, so `financial-report` still talks.
func TestNepCamOManTien(t *testing.T) {
	for _, man := range []string{"finance", "/finance", "settlements/abc", "/batches/7", "smart-split/x/y"} {
		if !nepPhaiLui(man) {
			t.Errorf("%q là màn tiền mà nepPhaiLui nói không", man)
		}
		err := kiemNep("Chia giúp mình", &goiNep{Phieu: &phieuNep{Man: man}})
		if maLoi(err) != "nep_lui_man_tien" {
			t.Errorf("%q: mã %q, muốn nep_lui_man_tien", man, maLoi(err))
		}
	}
	for _, man := range []string{"financial-report", "outings/7", "/", "chat/finance", "explore"} {
		if nepPhaiLui(man) {
			t.Errorf("%q không phải màn tiền mà bị bắt lui", man)
		}
	}
	// The refusal wins over every other defect: an empty question from a money
	// screen is still a money-screen refusal, never a hint about the bounds.
	if got := maLoi(kiemNep("", &goiNep{Phieu: &phieuNep{Man: "finance", SoLieu: map[string]any{"tien": 1.0}}})); got != "nep_lui_man_tien" {
		t.Errorf("màn tiền với thân hỏng ra %q", got)
	}
}

// The route refuses a money screen with no database and no provider: the
// handler here has neither, so reaching either would panic.
func TestNepManTienKhongChamGiCa(t *testing.T) {
	h := New(nil, nil)
	body, _ := json.Marshal(map[string]any{"logical_id": newID(), "prompt": "Mình nợ ai bao nhiêu?", "phieu": map[string]any{"man": "settlements/1"}, "luot": []any{}})
	r := httptest.NewRequest("POST", "/me/nep/ai-invocations", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer synthetic")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 || !strings.Contains(w.Body.String(), `"nep_lui_man_tien"`) {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestNepKiemGioiHan(t *testing.T) {
	dai := strings.Repeat("ạ", maxChuLuotNep+1)
	ca := []struct {
		ten    string
		prompt string
		goi    goiNep
		ma     string
	}{
		{"hợp lệ, không phiếu", "Tối nay đi đâu?", goiNep{}, ""},
		{"hợp lệ, đủ phiếu", "Gợi ý giúp mình", goiNep{Phieu: &phieuNep{Man: "outings/[id]", TieuDe: "Đà Lạt", Nhip: &nhipPhieu{Kieu: "sap-toi"}, LoaiSo: "hoi", SoLieu: map[string]any{"soNguoi": 4.0, "soChang": "ba chặng"}, GoiY: []string{"a", "b"}}, Luot: []luotNep{{"toi", "Hỏi"}, {"nep", "Đáp"}}}, ""},
		{"câu hỏi rỗng", "   ", goiNep{}, "invalid_invocation"},
		{"câu hỏi dài", strings.Repeat("a", maxHoiNep+1), goiNep{}, "invalid_invocation"},
		{"khoá số liệu lạ", "x", goiNep{Phieu: &phieuNep{Man: "a", SoLieu: map[string]any{"soTien": 1.0}}}, "boi_canh_sai_dang"},
		{"số liệu là văn", "x", goiNep{Phieu: &phieuNep{Man: "a", SoLieu: map[string]any{"soNguoi": strings.Repeat("x", 25)}}}, "boi_canh_sai_dang"},
		{"số liệu là object", "x", goiNep{Phieu: &phieuNep{Man: "a", SoLieu: map[string]any{"soNguoi": map[string]any{}}}}, "boi_canh_sai_dang"},
		{"nhịp lạ", "x", goiNep{Phieu: &phieuNep{Man: "a", Nhip: &nhipPhieu{Kieu: "mai"}}}, "boi_canh_sai_dang"},
		{"loại sổ lạ", "x", goiNep{Phieu: &phieuNep{Man: "a", LoaiSo: "tien"}}, "boi_canh_sai_dang"},
		{"bốn gợi ý", "x", goiNep{Phieu: &phieuNep{Man: "a", GoiY: []string{"1", "2", "3", "4"}}}, "boi_canh_sai_dang"},
		{"phiếu không có màn", "x", goiNep{Phieu: &phieuNep{Man: " "}}, "boi_canh_sai_dang"},
		{"vai lạ", "x", goiNep{Luot: []luotNep{{"ban", "Lan nói gì đó"}}}, "boi_canh_sai_dang"},
		{"lượt rỗng", "x", goiNep{Luot: []luotNep{{"toi", " "}}}, "boi_canh_sai_dang"},
		{"lượt dài", "x", goiNep{Luot: []luotNep{{"toi", dai}}}, "boi_canh_qua_lon"},
		{"quá nhiều lượt", "x", goiNep{Luot: make([]luotNep, maxLuotNep+1)}, "boi_canh_qua_lon"},
	}
	for _, c := range ca {
		if got := maLoi(kiemNep(c.prompt, &c.goi)); got != c.ma {
			t.Errorf("%s: mã %q, muốn %q", c.ten, got, c.ma)
		}
	}
	// The total is its own bound: every turn under the per-turn cap, the sum over.
	nhieu := make([]luotNep, 0, maxLuotNep)
	for i := 0; i < maxLuotNep; i++ {
		nhieu = append(nhieu, luotNep{"toi", strings.Repeat("ơ", maxChuLuotNep)})
	}
	if got := maLoi(kiemNep("x", &goiNep{Luot: nhieu})); got != "boi_canh_qua_lon" {
		t.Errorf("tổng quá lớn ra %q", got)
	}
}

// The request body is closed at the top level and inside the slip: a field the
// shipped client never sends is refused, not forwarded to the model.
func TestNepThanDongKhoa(t *testing.T) {
	h := New(nil, nil)
	for _, body := range []string{
		`{"logical_id":"` + newID() + `","prompt":"x","context":"abc"}`,
		`{"logical_id":"` + newID() + `","prompt":"x","phieu":{"man":"a","ten":"Lan"}}`,
		`{"logical_id":"` + newID() + `","prompt":"x","luot":[{"vai":"toi","chu":"a","tacGia":"Lan"}]}`,
	} {
		r := httptest.NewRequest("POST", "/me/nep/ai-invocations", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 400 || !strings.Contains(w.Body.String(), "invalid_body") {
			t.Errorf("%s: status=%d body=%s", body, w.Code, w.Body.String())
		}
	}
}

func TestNepMatches(t *testing.T) {
	for _, p := range []string{"/me/nep/ai-invocations", "/me/nep/ai-invocations/" + newID()} {
		if !Matches(p) {
			t.Errorf("%s phải về Go", p)
		}
	}
	// Python still serves Nếp's drawing; taking it here would 404 it.
	for _, p := range []string{"/me/nep/media", "/me/nep/media/abc/file", "/me/profile", "/me/nep"} {
		if Matches(p) {
			t.Errorf("%s không phải của chatassist", p)
		}
	}
}

func TestNepPayloadChiCoBaThu(t *testing.T) {
	goi, _ := json.Marshal(goiNep{Phieu: &phieuNep{Man: "explore", TieuDe: "Khám phá"}, Luot: []luotNep{{"toi", "Hỏi"}, {"nep", "Đáp"}}})
	v, err := nepPayload(goi, "Câu mới")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := pyjson.Dumps(v)
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	keys := []string{}
	for k := range got {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if strings.Join(keys, ",") != "prompt,slip,turns" {
		t.Fatalf("thân gửi não có %v", keys)
	}
	if len(got["turns"].([]any)) != 2 || got["prompt"] != "Câu mới" {
		t.Fatalf("thân sai: %s", raw)
	}
	// No slip is sent as null, never as a guessed route.
	goi, _ = json.Marshal(goiNep{})
	v, _ = nepPayload(goi, "x")
	raw, _ = pyjson.Dumps(v)
	if !strings.Contains(string(raw), `"slip": null`) || !strings.Contains(string(raw), `"turns": []`) {
		t.Fatalf("thân không phiếu: %s", raw)
	}
}

func TestNepDocTraLoi(t *testing.T) {
	doc := func(s string) (string, bool) {
		v, err := pyjson.Loads([]byte(s))
		if err != nil {
			t.Fatal(err)
		}
		return docTraLoi(v)
	}
	if s, ok := doc(`{"text":"  Đi Đà Lạt nhé.  "}`); !ok || s != "Đi Đà Lạt nhé." {
		t.Errorf("câu đúng bị từ chối: %q %v", s, ok)
	}
	for _, xau := range []string{`{"text":""}`, `{"text":"   "}`, `{"kind":"text"}`, `{"text":1}`, `[]`, `{"text":"` + strings.Repeat("a", maxTraLoiNep+1) + `"}`} {
		if _, ok := doc(xau); ok {
			t.Errorf("%s được nhận", xau[:min(len(xau), 40)])
		}
	}
}

// The closed lists here are copies of the device's. A copy that drifts is the
// money law holding on one side only, so this reads the TypeScript and compares.
func TestNepDanhSachKhopVoiPhieuTs(t *testing.T) {
	goc := filepath.Join("..", "..", "..", "..", "apps", "mobile", "src", "rudi")
	doc := func(tep, ten string) []string {
		raw, err := os.ReadFile(filepath.Join(goc, tep))
		if err != nil {
			t.Fatal(err)
		}
		m := regexp.MustCompile(`(?s)` + ten + `\s*=\s*\[(.*?)\]\s*as const`).FindStringSubmatch(string(raw))
		if m == nil {
			t.Fatalf("không thấy %s trong %s", ten, tep)
		}
		out := []string{}
		for _, s := range regexp.MustCompile(`"([^"]+)"`).FindAllStringSubmatch(m[1], -1) {
			out = append(out, s[1])
		}
		sort.Strings(out)
		return out
	}
	tuMap := func(m map[string]bool) []string {
		out := []string{}
		for k := range m {
			out = append(out, k)
		}
		sort.Strings(out)
		return out
	}
	lui := append([]string(nil), manNepLui...)
	sort.Strings(lui)
	for _, c := range []struct {
		ten     string
		ts, go_ []string
	}{
		{"MAN_NEP_LUI", doc("nep/phieu.ts", "MAN_NEP_LUI"), lui},
		{"KHOA_SO_LIEU", doc("nep/phieu.ts", "KHOA_SO_LIEU"), tuMap(khoaSoLieu)},
		{"KIEU_NHIP", doc("nep/phieu.ts", "KIEU_NHIP"), tuMap(kieuNhip)},
		{"LOAI_SO", doc("so/ban-tinh.ts", "LOAI_SO"), tuMap(loaiSoHopLe)},
	} {
		if strings.Join(c.ts, ",") != strings.Join(c.go_, ",") {
			t.Errorf("%s lệch: phieu.ts %v, Go %v", c.ten, c.ts, c.go_)
		}
	}
}

// The engine's turn is built from the stored job alone: «now» is the
// instant the question was stored (created_at), never the worker's clock,
// and the attempt, slip and session come across field for field.
func TestLuotEngineTuHangDaLuu(t *testing.T) {
	// Thursday 24/09/2026 22:47:05 in Vietnam, stored as UTC.
	luc := time.Date(2026, 9, 24, 15, 47, 5, 0, time.UTC)
	con := 2
	goi, _ := json.Marshal(goiNep{
		Phieu: &phieuNep{Man: "outings/[id]", TieuDe: "Đà Lạt", Nhip: &nhipPhieu{Kieu: "sap-toi", ConNgay: &con}, LoaiSo: "hoi", SoLieu: map[string]any{"soNguoi": 4}, GoiY: []string{"Còn thiếu gì?"}},
		Luot:  []luotNep{{Vai: "toi", Chu: "chỗ nào yên tĩnh"}, {Vai: "nep", Chu: "Hồ Tuyền Lâm."}},
	})
	j := work{id: "0b7d3a1c-5f2e-4c1a-9e3b-2d6f8a4c1e90", prompt: "tối nay đi đâu?", goi: goi, createdAt: luc, attempt: 2}
	turn, err := luotEngine(j)
	if err != nil {
		t.Fatal(err)
	}
	if !turn.Luc.Equal(luc) || turn.LanThu != 2 || turn.InvocationID != j.id || turn.LoiNho != "tối nay đi đâu?" {
		t.Fatalf("lượt: luc=%v lan=%d id=%s hỏi=%q", turn.Luc, turn.LanThu, turn.InvocationID, turn.LoiNho)
	}
	p := turn.PhieuNep
	if p == nil || p.Man != "outings/[id]" || p.TieuDe != "Đà Lạt" || p.Nhip == nil || *p.Nhip.ConNgay != 2 || p.LoaiSo != "hoi" ||
		p.SoLieu["soNguoi"] != 4.0 || len(p.GoiY) != 1 || len(turn.LuotNep) != 2 || turn.LuotNep[1].Vai != "nep" {
		t.Fatalf("phiếu/phiên: %+v %+v", p, turn.LuotNep)
	}
	// And the line the model reads from it, to the minute.
	if got := thoigian.DongBayGio(turn.Luc); got != "Bây giờ: Thứ Năm 24/09/2026 22:47 (Asia/Ho_Chi_Minh, 2026-09-24T22:47:05+07:00)" {
		t.Fatalf("dòng bây giờ: %s", got)
	}
}

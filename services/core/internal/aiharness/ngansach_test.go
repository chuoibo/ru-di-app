package aiharness

import (
	"strings"
	"testing"

	"google.golang.org/genai"

	"mobile/services/core/internal/aiharness/cau"
	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/obs"
)

// hong is a router output Doc refuses: the one repair runs.
var hong = llm.Buoc{Text: `{}`}

// Each Nếp path driven to its worst case, every optional call made (the
// router's repair, the grader and its round, a regeneration and its
// verifier, every agent step): the turn ends normally, the calls equal the
// plan's worst case (llm.KeHoach), and none exceeds MaxModelCallsPerTurn.
// Red if any path makes a call the plan does not count.
func TestXauNhatMoiDuongTrongTran(t *testing.T) {
	daLat := map[string]any{"diem_den_id": ddDaLat}
	cases := []struct {
		duong llm.Duong
		kich  []llm.Buoc
	}{
		{llm.DuongTruyHoi, []llm.Buoc{hong,
			ru{huong: "truy_hoi_mot_buoc", yDinh: []string{"find_places"}, canTruyHoi: []string{"places"}, slots: map[string]any{"diem_den_id": ddDaLat, "khi_chat": []string{"yen_tinh"}},
				truyVan: []map[string]string{{"nguon": "places", "cau": "quán yên tĩnh"}}}.buoc(),
			cham("thieu", []string{"khi_chat"}, []string{"khi_chat"}, ""),
			traLoiCau("Mình đã đặt bàn ở [[p:p2]].", "p2"), kiemCo(true, false),
			traLoiCau("Bạn thử [[p:p2]] nhé.", "p2"), kiemHoTro(1, "ho_tro")}},
		{llm.DuongTacTuNep, []llm.Buoc{hong,
			ru{huong: "tac_tu", yDinh: []string{"find_places"}, canTruyHoi: []string{"places"}, slots: daLat,
				truyVan: []map[string]string{{"nguon": "places", "cau": "quán Đà Lạt"}}}.buoc(),
			goiCC("search_places", map[string]any{"truy_van": "quán Đà Lạt"}),
			goiCC("search_places", map[string]any{"truy_van": "quán Đà Lạt", "k": 3}),
			dung(false, "Bạn thử Quán Gió Đồi nhé."), kiemDat()}},
		{llm.DuongThang, []llm.Buoc{hong, ruThang(), dung(false, "Chào bạn nhé."), kiemDat()}},
		{llm.DuongHoiLai, []llm.Buoc{hong, ru{hoiLai: "Bạn muốn đi khu nào?"}.buoc(), kiemDat()}},
	}
	for _, c := range cases {
		w := moiTheGioi(t)
		m := chayVoi(t, w, luotCoBan(), c.kich...)
		if m.err != nil {
			t.Fatalf("%s: %v %+v", c.duong, m.err, m.res.Record)
		}
		n := m.res.Record.SoGoiMoHinh
		if n != llm.ToiDaDuong(c.duong) || n > llm.MaxModelCallsPerTurn || m.stub.SoGoi() != n {
			t.Fatalf("%s: %d calls (stub %d), plan %d, ceiling %d", c.duong, n, m.stub.SoGoi(), llm.ToiDaDuong(c.duong), llm.MaxModelCallsPerTurn)
		}
	}
}

// The metrics row sums every call's usage (router and verifier too, not
// only the agent's), the implicit cache's share included: counts only.
func TestTokenMoiLoiGoi(t *testing.T) {
	voi := func(b llm.Buoc, in, cache int32) llm.Buoc {
		b.Usage = &genai.GenerateContentResponseUsageMetadata{PromptTokenCount: in, CachedContentTokenCount: cache, CandidatesTokenCount: 5}
		return b
	}
	m := chayLuot(t, luotCoBan(), voi(ruThang(), 2000, 1024), voi(dung(false, "Chào bạn nhé."), 1500, 1024), voi(kiemDat(), 900, 0))
	r := m.res.Record
	if m.err != nil || r.TokensIn != 4400 || r.TokensCache != 2048 || r.TokensOut != 15 {
		t.Fatalf("%v %+v", m.err, r)
	}
}

// When an earlier attempt spent part of the budget, the cuts come in the
// plan's order: the grader first, then the regeneration; the verifier
// never, and the answer is not made when its verifier could not run.
func TestCatTheoThuTu(t *testing.T) {
	router := ru{huong: "truy_hoi_mot_buoc", yDinh: []string{"find_places"}, canTruyHoi: []string{"places"}, slots: map[string]any{"diem_den_id": ddDaLat},
		truyVan: []map[string]string{{"nguon": "places", "cau": "quán Đà Lạt"}}}.buoc()
	tuNhan := traLoiCau("Mình đã đặt bàn ở [[p:p1]].", "p1")
	tot := traLoiCau("Bạn thử [[p:p1]] nhé.", "p1")

	// Five left: no grader (it needs the whole cycle after it), the
	// regeneration still runs.
	turn := luotCoBan()
	turn.DaGoiTruoc = llm.MaxModelCallsPerTurn - 5
	m := chayVoi(t, moiTheGioi(t), turn, router, tuNhan, kiemCo(true, false), tot, kiemHoTro(1, "ho_tro"))
	if m.err != nil || m.res.Record.SoGoiMoHinh != 5 || !m.res.Record.SinhLai || m.res.Record.VongSua != 0 || m.res.Record.KetKiem != obs.KiemDat {
		t.Fatalf("five left: %v %+v", m.err, m.res.Record)
	}
	khongCham := func(m moTa) {
		t.Helper()
		for i, y := range m.stub.YeuCau() {
			// The grader's schema names ket_luan; no other step's does.
			if strings.Contains(string(y), `"ket_luan"`) {
				t.Fatalf("call %d is the grader: it must be the first cut", i+1)
			}
		}
	}
	khongCham(m)
	// Three left: no grader, no regeneration; the first draft is still
	// verified, and failing it the fixed fallback stands.
	turn.DaGoiTruoc = llm.MaxModelCallsPerTurn - 3
	m = chayVoi(t, moiTheGioi(t), turn, router, tuNhan, kiemCo(true, false))
	if m.err != nil || m.res.Record.SoGoiMoHinh != 3 || m.res.Record.SinhLai || m.res.Text != cau.DuPhong || m.res.Record.KetKiem != obs.KiemKhongDat {
		t.Fatalf("three left: %v %q %+v", m.err, m.res.Text, m.res.Record)
	}
	khongCham(m)
	// Two left: the answer cannot be verified after the router, so it is
	// not made.
	turn.DaGoiTruoc = llm.MaxModelCallsPerTurn - 2
	m = chayVoi(t, moiTheGioi(t), turn, router)
	if m.err != nil || m.res.Record.SoGoiMoHinh != 1 || m.res.Text != cau.DuPhong {
		t.Fatalf("two left: %v %q %+v", m.err, m.res.Text, m.res.Record)
	}
}

package aiharness

import (
	"encoding/json"
	"strings"
	"testing"

	"mobile/services/core/internal/aiharness/cau"
	"mobile/services/core/internal/aiharness/hieu"
	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/obs"
	"mobile/services/core/internal/aiharness/prompts"
)

// A chat of two is a room of friends (decision 2026-09-28, two classes): its
// turn takes the group's path whole, the group's tools and the split draft,
// whether or not the two are a couple (Turn.Doi).

func luotHaiNguoi(doi bool) Turn {
	turn := luotNhomCoBan()
	turn.Doi = doi
	turn.LuotNhom = turn.LuotNhom[:2]
	turn.ThanhVien = turn.ThanhVien[:2]
	turn.SoTin = 2
	return turn
}

// kichTacTu is one agent turn that searches, drafts an itinerary and
// answers: the path on which the bot's whole toolset is declared.
func kichTacTu() []llm.Buoc {
	return []llm.Buoc{
		ru{huong: "tac_tu", yDinh: []string{"plan"}, canTruyHoi: []string{"places"}, slots: map[string]any{"diem_den_id": ddDaLat},
			truyVan: []map[string]string{{"nguon": "places", "cau": "quán Đà Lạt tối thứ 7"}}}.buoc(),
		goiCC("search_places", map[string]any{"truy_van": "quán Đà Lạt tối thứ 7"}),
		goiCC("propose_itinerary", map[string]any{"chang": []any{map[string]any{"id": "p1", "gio": "19:00"}}}),
		dung(false, "Tối thứ 7 hai bạn ghé [[p:p1]] nhé."), kiemDat(),
	}
}

func khaiBaoNhom(m moTa) map[string]bool {
	out := map[string]bool{}
	for _, y := range m.stub.YeuCau() {
		for _, ten := range []string{"draft_poll", "group_snapshot", "list_group_outings", "search_places", "propose_itinerary"} {
			if strings.Contains(string(y), `"`+ten+`"`) {
				out[ten] = true
			}
		}
	}
	return out
}

// A chat of two declares the group's whole toolset (tools.ChoNhom), a
// couple's as well as friends'.
func TestHaiNguoiCoCongCuNhom(t *testing.T) {
	w := moiTheGioi(t)
	for _, doi := range []bool{false, true} {
		turn := luotHaiNguoi(doi)
		turn.Lenh = obs.LenhPlan
		m := chayNhom(t, w, nhomOpts{}, turn, kichTacTu()...)
		if m.err != nil {
			t.Fatalf("doi=%v: %v %+v", doi, m.err, m.res.Record)
		}
		cc := khaiBaoNhom(m)
		if !cc["draft_poll"] || !cc["group_snapshot"] || !cc["list_group_outings"] || !cc["search_places"] || !cc["propose_itinerary"] {
			t.Fatalf("doi=%v: a chat of two declares %v", doi, cc)
		}
		if loaiPhan(t, m.res) != "itinerary,text" {
			t.Fatalf("doi=%v: %s", doi, loaiPhan(t, m.res))
		}
	}
}

// A split request in a chat of two takes the group's split draft: the
// reading, the verifier, and a card that splits between the two.
func TestHaiNguoiChiaBillNhap(t *testing.T) {
	w := moiTheGioi(t)
	for _, doi := range []bool{false, true} {
		turn := luotHaiNguoi(doi)
		turn.LoiNho = "@Rủ Đi chia bill giùm, taxi 100k mình trả luôn"
		m := chayNhom(t, w, nhomOpts{}, turn,
			ru{tien: "split_draft", yDinh: []string{"chia_bill_draft"}}.buoc(),
			chiaTho(map[string]any{"tin": "t2", "tieu_de": "lẩu", "so_tien_goc": "850k", "so_tien_vnd": 850000},
				map[string]any{"tin": "loi_nho", "tieu_de": "taxi", "so_tien_goc": "100k", "so_tien_vnd": 100000}),
			kiemChia("ho_tro", "ho_tro"))
		if m.err != nil || m.stub.SoGoi() != 3 || loaiPhan(t, m.res) != "text,expense_draft" {
			t.Fatalf("doi=%v: %v %d %s", doi, m.err, m.stub.SoGoi(), loaiPhan(t, m.res))
		}
		if m.res.Record.Duong != obs.DuongNhapChiaBill || m.res.Record.KetKiem != obs.KiemDat {
			t.Fatalf("doi=%v: %+v", doi, m.res.Record)
		}
		for _, want := range []string{"Tú trả 850.000đ: lẩu", "Tú trả 100.000đ: taxi", "Tổng 950.000đ. Chia đều cho 2 người", "chưa ghi vào sổ"} {
			if !strings.Contains(m.res.Text, want) {
				t.Errorf("doi=%v: card lacks %q:\n%s", doi, want, m.res.Text)
			}
		}
		if len(m.res.KetQuaNhap) == 0 {
			t.Fatalf("doi=%v: no draft", doi)
		}
		groundCua(t, m.res, "chia_bill", 2, w)
		cuoi, bot := cuoiNhapNhom, obs.BotNhom
		if doi {
			cuoi, bot = cuoiNhapDoi, obs.BotDoi
		}
		if !strings.HasSuffix(m.res.Text, cuoi) || m.res.Record.Bot != bot {
			t.Errorf("doi=%v: the draft closes %q, record bot %q", doi, m.res.Text, m.res.Record.Bot)
		}
	}
}

// heThongCua reads the system instruction of each request the stub saw.
func heThongCua(t *testing.T, m moTa) []string {
	t.Helper()
	var out []string
	for _, y := range m.stub.YeuCau() {
		var r struct {
			Config struct {
				SystemInstruction struct{ Parts []struct{ Text string } } `json:"systemInstruction"`
			} `json:"config"`
		}
		if err := json.Unmarshal(y, &r); err != nil {
			t.Fatal(err)
		}
		var b strings.Builder
		for _, p := range r.Config.SystemInstruction.Parts {
			b.WriteString(p.Text)
		}
		out = append(out, b.String())
	}
	return out
}

// lopPhong says which answer instruction the model read in a turn: the
// couple's ("doi"), the group's ("nhom") or neither ("").
func lopPhong(t *testing.T, m moTa) string {
	t.Helper()
	nhom, doi := 0, 0
	for _, s := range heThongCua(t, m) {
		if strings.HasPrefix(s, prompts.NhomAgent(maKiem)) {
			nhom++
		}
		if strings.HasPrefix(s, prompts.DoiAgent(maKiem)) {
			doi++
		}
	}
	switch {
	case nhom > 0 && doi > 0:
		t.Fatal("one turn read both instructions")
	case nhom > 0:
		return "nhom"
	case doi > 0:
		return "doi"
	}
	return ""
}

// The two classes (2026-09-28): a couple's turn (Doi) reads the couple's
// instruction on the direct path and on the tool path, its out-of-scope
// clause and fixed sentences speak to two people, and its record names bot
// doi with the couple's prompt version; a chat of two among friends (Doi
// false) reads the group's instruction and records bot nhom, exactly as a
// group does. The turn itself stays Bot nhom either way.
func TestDoiDungLoiNhacDoi(t *testing.T) {
	w := moiTheGioi(t)
	for _, doi := range []bool{false, true} {
		wantAgent, wantBot, wantPV := "nhom", obs.BotNhom, obs.PromptVersion(prompts.VersionNhom())
		if doi {
			wantAgent, wantBot, wantPV = "doi", obs.BotDoi, obs.PromptVersion(prompts.VersionDoi())
		}
		kiem := func(ten string, m moTa) {
			t.Helper()
			if m.err != nil {
				t.Fatalf("doi=%v %s: %v", doi, ten, m.err)
			}
			if got := lopPhong(t, m); got != wantAgent {
				t.Errorf("doi=%v %s: the answer read the %q instruction", doi, ten, got)
			}
			r := m.res.Record
			if r.Bot != wantBot || r.PromptVersion != wantPV || r.Valid() != nil {
				t.Errorf("doi=%v %s: record bot %q version %q", doi, ten, r.Bot, r.PromptVersion)
			}
			wantRouter, _ := hieu.LoiNhac(obs.BotNhom)
			if doi {
				wantRouter = hieu.LoiNhacDoi()
			}
			if he := heThongCua(t, m); len(he) == 0 || he[0] != wantRouter {
				t.Errorf("doi=%v %s: the router read another instruction", doi, ten)
			}
			if !strings.Contains(m.log.String(), `"bot":"`+string(wantBot)+`"`) {
				t.Errorf("doi=%v %s: the log line does not name bot %s", doi, ten, wantBot)
			}
		}
		// The direct path.
		turn := luotHaiNguoi(doi)
		turn.LoiNho = "@Rủ Đi chào nha"
		kiem("thang", chayNhom(t, w, nhomOpts{}, turn, ru{}.buoc(), dung(false, "Chào hai bạn."), kiemDat()))
		if turn.Bot != obs.BotNhom {
			t.Fatal("the turn's bot changed")
		}
		// The tool path.
		turn = luotHaiNguoi(doi)
		turn.Lenh = obs.LenhPlan
		kiem("tac_tu", chayNhom(t, w, nhomOpts{}, turn, kichTacTu()...))
		// Out of scope: the room's own clause after the instruction.
		turn = luotHaiNguoi(doi)
		turn.LoiNho = "@Rủ Đi giải bài toán này giùm"
		m := chayNhom(t, w, nhomOpts{}, turn, ru{nhan: "ngoai_pham_vi"}.buoc(), dung(false, "Việc này mình không giúp được."), kiemDat())
		kiem("ngoai_pham_vi", m)
		clause, other := prompts.LoiDanNhanNhom("ngoai_pham_vi"), prompts.LoiDanNhanDoi("ngoai_pham_vi")
		if doi {
			clause, other = other, clause
		}
		he := strings.Join(heThongCua(t, m), "\n")
		if !strings.Contains(he, clause) || strings.Contains(he, other) {
			t.Errorf("doi=%v: the out-of-scope clause is not the room's", doi)
		}
		// The money refusal, a fixed sentence.
		turn = luotHaiNguoi(doi)
		turn.LoiNho = "@Rủ Đi nhắc Lan chuyển khoản 500k cho mình"
		m = chayNhom(t, w, nhomOpts{}, turn, ru{tien: "money_action", yDinh: []string{"hoi"}}.buoc())
		want := cau.NhomKhongChamTien
		if doi {
			want = cau.DoiKhongChamTien
		}
		if m.err != nil || m.res.Text != want || m.res.Record.Bot != wantBot {
			t.Errorf("doi=%v: refusal %q bot %q", doi, m.res.Text, m.res.Record.Bot)
		}
	}
}

// A couple's words: its instruction, its clause and every fixed sentence it
// may read speak to two people; the echo guard reads the room's own
// instruction; the split draft differs only in its closing line.
func TestDoiKhongCoChuNhom(t *testing.T) {
	for _, s := range []string{prompts.DoiAgent(maKiem), prompts.LoiDanNhanDoi("ngoai_pham_vi"), cau.DoiKhongChamTien, cau.DoiChuaThayKhoan, cau.DoiLoiNhoPlan, cuoiNhapDoi} {
		for _, w := range prompts.TuNhom {
			if strings.Contains(strings.ToLower(s), w) {
				t.Errorf("couple text has %q: %s", w, s)
			}
		}
	}
	if strings.Join(khuonNhom(true).loiNhac, "|") != strings.Join(prompts.LoiNhacDoi(), "|") ||
		strings.Join(khuonNhom(false).loiNhac, "|") != strings.Join(prompts.LoiNhacNhom(), "|") {
		t.Fatal("the echo guard does not read the room's instruction")
	}
	ks := []khoanNhap{{tieuDe: "lẩu", soTien: 850000, nguoiTra: nguoiHoi}}
	ts := luotHaiNguoi(true).ThanhVien
	a, err := chiaBillPartsCuoi(ks, []string{nguoiHoi, nguoiLan}, ts, cuoiNhapDoi)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := chiaBillParts(ks, []string{nguoiHoi, nguoiLan}, ts)
	if !strings.HasSuffix(a.chu, cuoiNhapDoi) || !strings.HasSuffix(b.chu, cuoiNhapNhom) ||
		strings.TrimSuffix(a.chu, cuoiNhapDoi) != strings.TrimSuffix(b.chu, cuoiNhapNhom) || string(a.ketQua) != string(b.ketQua) {
		t.Fatalf("couple draft:\n%s\nfriends draft:\n%s", a.chu, b.chu)
	}
}

package aiharness

import (
	"strings"
	"testing"

	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/obs"
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
	}
}

package aiharness

import (
	"strings"
	"testing"

	"mobile/services/core/internal/aiharness/cau"
	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/obs"
)

// The room assistant in a chat of two (Turn.Cap, design 2026-09-28): the
// group's path with the pair's tool table (tools.ChoCap) and no split draft.

func luotCap() Turn {
	turn := luotNhomCoBan()
	turn.Cap = true
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

// A pair's turn declares the common tools and none of the group's; the
// same turn in a group declares the group's (the control: the check can
// see a group tool when one is there).
func TestCapChiCongCuChung(t *testing.T) {
	w := moiTheGioi(t)
	turn := luotCap()
	turn.Lenh = obs.LenhPlan
	m := chayNhom(t, w, nhomOpts{}, turn, kichTacTu()...)
	if m.err != nil {
		t.Fatalf("%v %+v", m.err, m.res.Record)
	}
	cap := khaiBaoNhom(m)
	if cap["draft_poll"] || cap["group_snapshot"] || cap["list_group_outings"] || !cap["search_places"] || !cap["propose_itinerary"] {
		t.Fatalf("pair declares %v", cap)
	}
	if loaiPhan(t, m.res) != "itinerary,text" {
		t.Fatalf("%s", loaiPhan(t, m.res))
	}
	turn.Cap = false
	nhom := khaiBaoNhom(chayNhom(t, w, nhomOpts{}, turn, kichTacTu()...))
	if !nhom["draft_poll"] || !nhom["group_snapshot"] || !nhom["list_group_outings"] {
		t.Fatalf("group declares %v", nhom)
	}
}

// A chat of two has no split draft: a money action and a split request
// both get the pair's fixed sentence after the router's one call, no draft
// part, no tool.
func TestCapKhongChiaBill(t *testing.T) {
	w := moiTheGioi(t)
	for name, r := range map[string]ru{
		"money_action": {tien: "money_action", yDinh: []string{"hoi"}},
		"split_draft":  {tien: "split_draft", yDinh: []string{"chia_bill_draft"}},
		"intent only":  {yDinh: []string{"chia_bill_draft"}},
	} {
		m := chayNhom(t, w, nhomOpts{}, luotCap(), r.buoc(), dung(false, "không được gọi"))
		if m.err != nil || m.stub.SoGoi() != 1 || m.res.Text != cau.CapKhongChamTien || loaiPhan(t, m.res) != "text" {
			t.Fatalf("%s: %v %d %q %s", name, m.err, m.stub.SoGoi(), m.res.Text, loaiPhan(t, m.res))
		}
		if rc := m.res.Record; rc.Guard != obs.GuardRefused || rc.Duong != obs.DuongTuChoiTien || rc.SoCongCu != 0 || len(m.res.KetQuaNhap) != 0 {
			t.Fatalf("%s: %+v", name, rc)
		}
		if strings.Join(m.sink.delta, "") != cau.CapKhongChamTien {
			t.Fatalf("%s: the sentence did not go through the window: %q", name, m.sink.delta)
		}
		groundCua(t, m.res, "hoi", 2, w)
	}
}

package aiharness

import (
	"testing"

	"mobile/services/core/internal/aiharness/llm"
)

// The group's paths driven to their worst case (llm.KeHoach): the router's
// repair, then every agent step of the group (four, the last with function
// calling off) and the verifier; the split draft's router, repair and one
// reading. The calls equal the plan, within the ceiling.
func TestNhomXauNhatTrongTran(t *testing.T) {
	daLat := map[string]any{"diem_den_id": ddDaLat}
	cases := []struct {
		duong llm.Duong
		kich  []llm.Buoc
	}{
		{llm.DuongTacTuNhom, []llm.Buoc{hong,
			ru{huong: "tac_tu", yDinh: []string{"find_places"}, canTruyHoi: []string{"places"}, slots: daLat,
				truyVan: []map[string]string{{"nguon": "places", "cau": "quán Đà Lạt"}}}.buoc(),
			goiCC("search_places", map[string]any{"truy_van": "quán Đà Lạt"}),
			goiCC("group_snapshot", map[string]any{}),
			goiCC("search_places", map[string]any{"truy_van": "quán Đà Lạt", "k": 3}),
			dung(false, "Cả nhóm thử Quán Gió Đồi nhé."), kiemDat()}},
		{llm.DuongNhapChiaBill, []llm.Buoc{hong, ru{tien: "split_draft", yDinh: []string{"chia_bill_draft"}}.buoc(),
			chiaTho(map[string]any{"tin": "t2", "tieu_de": "lẩu", "so_tien_vnd": 850000})}},
	}
	for _, c := range cases {
		m := chayNhom(t, moiTheGioi(t), nhomOpts{}, luotNhomCoBan(), c.kich...)
		if m.err != nil {
			t.Fatalf("%s: %v %+v", c.duong, m.err, m.res.Record)
		}
		n := m.res.Record.SoGoiMoHinh
		if n != llm.ToiDaDuong(c.duong) || n > llm.MaxModelCallsPerTurn || m.stub.SoGoi() != n {
			t.Fatalf("%s: %d calls (stub %d), plan %d", c.duong, n, m.stub.SoGoi(), llm.ToiDaDuong(c.duong))
		}
	}
}

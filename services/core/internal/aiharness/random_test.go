package aiharness

import (
	"strings"
	"testing"

	"mobile/services/core/internal/aiharness/cau"
	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/obs"
	"mobile/services/core/internal/aiharness/testkit"
	"mobile/services/core/internal/aiharness/traloi"
	"mobile/services/core/internal/aiharness/truyhoi"
)

// An explicit random search overrides an itinerary or split-draft intent,
// retains the model's hard constraints and returns only grounded places.
func TestNhomRandomChiTimLuaChon(t *testing.T) {
	for _, r := range []ru{
		{yDinh: []string{"plan"}},
		{tien: "split_draft", yDinh: []string{"chia_bill_draft"}},
	} {
		t.Run(r.tien+strings.Join(r.yDinh, ","), func(t *testing.T) {
			w := moiTheGioi(t)
			w.quan.KetQua = []truyhoi.KetQuaTruyHoi{{BangChung: []truyhoi.BangChung{quanGia("q-1", "Quán Một"), quanGia("q-2", "Quán Hai")}}}
			turn := luotNhomCoBan()
			turn.Lenh, turn.LoiNho = obs.LenhRandom, "quán ăn Đà Lạt dưới 200k"
			r.slots = map[string]any{"diem_den_id": ddDaLat, "ngan_sach_vnd": 200000, "di_ung": []string{"dau_phong"}}
			m := chayNhom(t, w, nhomOpts{}, turn, r.buoc(), cham("du", nil, nil, ""),
				traLoiCau("Bạn xem [[p:p1]] và [[p:p2]] nhé.", "p1", "p2"), kiemHoTro(1, "ho_tro"))
			if m.err != nil || loaiPhan(t, m.res) != "places,text" || len(m.res.QuanIDs) != 2 || m.res.KetQuaNhap != nil {
				t.Fatalf("random: err=%v parts=%s ids=%v draft=%s", m.err, loaiPhan(t, m.res), m.res.QuanIDs, m.res.KetQuaNhap)
			}
			if len(w.quan.Da) != 1 || w.quan.Da[0].Cau != turn.LoiNho || w.quan.Da[0].Cung.DiemDenID != ddDaLat || w.quan.Da[0].Cung.NganSachVND == nil || *w.quan.Da[0].Cung.NganSachVND != 200000 || len(w.quan.Da[0].Cung.DiUng) != 1 {
				t.Fatalf("retrieval lost the request or hard filters: %+v", w.quan.Da)
			}
			if m.res.Record.Lenh != obs.LenhRandom || m.res.Record.Duong != obs.DuongTruyHoi || m.res.Record.PromptVersion != obs.PromptVersion(traloi.VersionRandom()) {
				t.Fatalf("record: %+v", m.res.Record)
			}
			answerRequest := string(m.stub.YeuCau()[2])
			if !strings.Contains(answerRequest, "random game") || !strings.Contains(answerRequest, "not a verified dish or menu list") {
				t.Fatal("the source answer lacks the random search instruction")
			}
			groundCua(t, m.res, "random", 4, w)
		})
	}
}

func TestNhomRandomKhongBiaKhiRongHoacKhongKiemDuoc(t *testing.T) {
	for _, c := range []struct {
		ten   string
		rong  bool
		truoc int
		kich  []llm.Buoc
	}{
		{ten: "no match", rong: true, kich: []llm.Buoc{cham("du", nil, nil, "")}},
		{ten: "verifier refused", kich: []llm.Buoc{cham("du", nil, nil, ""), traLoiCau("Bạn xem [[p:p1]] nhé.", "p1"), {Text: `{"menh_de":"invalid"}`}}},
		{ten: "answer budget exhausted", truoc: llm.MaxModelCallsPerTurn - 2},
	} {
		t.Run(c.ten, func(t *testing.T) {
			w := moiTheGioi(t)
			if c.rong {
				w.quan = &testkit.TheoLuot{}
			}
			turn := luotNhomCoBan()
			turn.Lenh, turn.LoiNho, turn.DaGoiTruoc = obs.LenhRandom, "quán ăn Đà Lạt", c.truoc
			kich := append([]llm.Buoc{ruThang()}, c.kich...)
			m := chayNhom(t, w, nhomOpts{}, turn, kich...)
			if m.err != nil || len(m.res.QuanIDs) != 0 || loaiPhan(t, m.res) != "text" || m.res.Text == "" {
				t.Fatalf("unverified choices escaped: err=%v ids=%v parts=%s text=%q", m.err, m.res.QuanIDs, loaiPhan(t, m.res), m.res.Text)
			}
			if !c.rong && !strings.Contains(m.res.Text, "chưa kiểm chứng") {
				t.Fatalf("fallback did not explain uncertainty: %q", m.res.Text)
			}
		})
	}
}

func TestNhomRandomKhongHaCongAnToan(t *testing.T) {
	turn := luotNhomCoBan()
	turn.Lenh = obs.LenhRandom
	m := chayNhom(t, moiTheGioi(t), nhomOpts{}, turn, ru{tien: "money_action", yDinh: []string{"chia_bill_draft"}}.buoc())
	if m.err != nil || m.res.Text != cau.NhomKhongChamTien || m.res.Record.SoCongCu != 0 || m.res.KetQuaNhap != nil {
		t.Fatalf("money-action guard lost: %v %+v", m.err, m.res)
	}
	w := moiTheGioi(t)
	m = chayNhom(t, w, nhomOpts{}, turn, ru{nhan: "ngoai_pham_vi"}.buoc(), dung(false, "Mình giúp nhóm tìm chỗ đi chơi thôi nhé."), kiemDat())
	if m.err != nil || len(w.quan.Da) != 0 || loaiPhan(t, m.res) != "text" {
		t.Fatalf("out-of-scope guard lost: %v %v", m.err, w.quan.Da)
	}
}

func TestNhomRandomKhongTuDatLoiNho(t *testing.T) {
	turn := luotNhomCoBan()
	turn.Lenh, turn.LoiNho = obs.LenhRandom, ""
	m := chayNhom(t, moiTheGioi(t), nhomOpts{}, turn)
	if ma(m.err) != cau.InvalidAIResult || m.stub.SoGoi() != 0 {
		t.Fatalf("a bare random invented a request: %v %d", m.err, m.stub.SoGoi())
	}
}

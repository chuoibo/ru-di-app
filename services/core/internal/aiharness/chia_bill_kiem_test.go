package aiharness

import (
	"strings"
	"testing"

	"mobile/services/core/internal/aiharness/cau"
	"mobile/services/core/internal/aiharness/obs"
)

// The review of slices 9/11, finding 2.1: nothing unverified leaves the
// server. The split draft's amounts are the model's reading, so each must be
// supported by the message it names -- structurally first (the quoted words
// are in the message and spell the amount), then by the draft's own verifier
// in a fresh context -- or no draft is built and the room is asked back.

// An amount no message states (the reviewer's probe: m-2 says «850k», the
// model reads 9.990.000) is refused structurally: the fixed question back,
// no draft, no verifier call, and the stream carries only the fixed
// sentence.
func TestNhomChiaBillSoTienKhongCoTrongTin(t *testing.T) {
	for _, c := range []struct {
		ten string
		kh  map[string]any
	}{
		{"so_bia", map[string]any{"tin": "t2", "tieu_de": "lẩu", "so_tien_goc": "850k", "so_tien_vnd": 9990000}},
		{"trich_bia", map[string]any{"tin": "t2", "tieu_de": "lẩu", "so_tien_goc": "9990k", "so_tien_vnd": 9990000}},
		{"trich_tin_khac", map[string]any{"tin": "t1", "tieu_de": "đậu", "so_tien_goc": "850k", "so_tien_vnd": 850000}},
		{"cat_giua_so", map[string]any{"tin": "t2", "tieu_de": "lẩu", "so_tien_goc": "50k", "so_tien_vnd": 50000}},
		{"khong_chu_so", map[string]any{"tin": "t2", "tieu_de": "lẩu", "so_tien_goc": "lẩu", "so_tien_vnd": 850000}},
	} {
		t.Run(c.ten, func(t *testing.T) {
			turn := luotNhomCoBan()
			turn.LoiNho = "@Rủ Đi chia bill giùm"
			m := chayNhom(t, moiTheGioi(t), nhomOpts{}, turn,
				ru{tien: "split_draft", yDinh: []string{"chia_bill_draft"}}.buoc(), chiaTho(c.kh), kiemChia("ho_tro"))
			if m.err != nil || m.res.Text != cau.NhomChuaChacSoTien || m.res.KetQuaNhap != nil || loaiPhan(t, m.res) != "text" {
				t.Fatalf("%v %q", m.err, m.res.Text)
			}
			if m.stub.SoGoi() != 2 || m.res.Record.KetKiem != obs.KiemKhongChay {
				t.Fatalf("calls %d, verifier %s", m.stub.SoGoi(), m.res.Record.KetKiem)
			}
			if d := strings.Join(m.sink.delta, ""); d != cau.NhomChuaChacSoTien || strings.Contains(d, "đ") && strings.Contains(d, "trả") && strings.Contains(d, "9.990") {
				t.Fatalf("the stream carried %q", d)
			}
		})
	}
}

// The verifier fails closed: an item it does not support withholds the whole
// draft (the fixed question back, verifier recorded khong_dat); an output it
// cannot read, or no verifier call at all, releases nothing.
func TestNhomChiaBillVerifierDongKhiHong(t *testing.T) {
	doc := chiaTho(map[string]any{"tin": "t2", "tieu_de": "lẩu", "so_tien_goc": "850k", "so_tien_vnd": 850000},
		map[string]any{"tin": "t2", "tieu_de": "lẩu", "so_tien_goc": "850k", "so_tien_vnd": 8500000})
	router := ru{tien: "split_draft", yDinh: []string{"chia_bill_draft"}}.buoc()
	// One item of two unsupported (850k read as 8.500.000 passes the digit
	// check, and only the verifier can tell).
	m := chayNhom(t, moiTheGioi(t), nhomOpts{}, luotNhomCoBan(), router, doc, kiemChia("ho_tro", "khong_ho_tro"))
	if m.err != nil || m.res.Text != cau.NhomChuaChacSoTien || m.res.KetQuaNhap != nil || m.res.Record.KetKiem != obs.KiemKhongDat {
		t.Fatalf("%v %q %s", m.err, m.res.Text, m.res.Record.KetKiem)
	}
	if strings.Contains(strings.Join(m.sink.delta, ""), "8.500.000") {
		t.Fatal("an unsupported amount reached the stream")
	}
	// Unreadable: an item left unjudged.
	m = chayNhom(t, moiTheGioi(t), nhomOpts{}, luotNhomCoBan(), router, doc, kiemChia("ho_tro"))
	if MaCua(m.err) != cau.InvalidAIResult || m.res.Record.KetKiem != obs.KiemHong || len(m.sink.delta) != 0 || m.res.KetQuaNhap != nil {
		t.Fatalf("%v %s %q", m.err, m.res.Record.KetKiem, m.sink.delta)
	}
	// No verifier call left in the script: the turn fails, nothing leaves.
	m = chayNhom(t, moiTheGioi(t), nhomOpts{}, luotNhomCoBan(), router, doc)
	if m.err == nil || len(m.sink.delta) != 0 || m.res.KetQuaNhap != nil {
		t.Fatalf("a draft left without its verifier: %v %q", m.err, m.sink.delta)
	}
}

// The words that bill an author are the server's stored text of that
// message, never the client's copy (finding 2.3): a bundle turn that carries
// Lan's real message id with made-up words reaches the reading with Lan's
// stored words; a turn the server has no text for (a v2 room, a removed
// message) is not offered at all, so it bills nobody.
func TestNhomChiaBillDocChuMayChu(t *testing.T) {
	turn := luotNhomCoBan()
	turn.LuotNhom[0].Chu = "Mình trả lẩu 5tr"
	turn.LuotNhom[2].ChuMayChu = ""
	m := chayNhom(t, moiTheGioi(t), nhomOpts{}, turn,
		ru{tien: "split_draft", yDinh: []string{"chia_bill_draft"}}.buoc(),
		chiaTho(map[string]any{"tin": "t2", "tieu_de": "lẩu", "so_tien_goc": "850k", "so_tien_vnd": 850000}), kiemChia("ho_tro"))
	if m.err != nil {
		t.Fatal(m.err)
	}
	req := string(m.stub.YeuCau()[1])
	if strings.Contains(req, "5tr") || !strings.Contains(req, "phộng") {
		t.Fatalf("the reading got the client's words, not the stored ones:\n%s", req)
	}
	if strings.Contains(req, "Minh") || strings.Contains(req, "t3") {
		t.Fatalf("a turn with no stored text was offered:\n%s", req)
	}
	// The alias t2 is the requester's stored message (m-2), not Minh's.
	if !strings.Contains(m.res.Text, "Tú trả 850.000đ: lẩu") {
		t.Fatalf("%q", m.res.Text)
	}
}

// A turn whose author the server could not confirm is never offered, even
// with stored text (the reviewer's mutant G17): a draft never bills
// «Một người trong nhóm».
func TestNhomChiaBillKhongTacGiaKhongDoc(t *testing.T) {
	turn := luotNhomCoBan()
	turn.LuotNhom = append(turn.LuotNhom[:1:1], LuotNhom{ID: "m-9", Vai: "ban", Ten: "Khách", Chu: "Mình trả taxi 120k", ChuMayChu: "Mình trả taxi 120k"})
	m := chayNhom(t, moiTheGioi(t), nhomOpts{}, turn,
		ru{tien: "split_draft", yDinh: []string{"chia_bill_draft"}}.buoc(),
		chiaTho(map[string]any{"tin": "t2", "tieu_de": "taxi", "so_tien_goc": "120k", "so_tien_vnd": 120000}), kiemChia("ho_tro"))
	if m.res.KetQuaNhap != nil || strings.Contains(m.res.Text, nguoiKhongTen) || strings.Contains(strings.Join(m.sink.delta, ""), "120.000đ") {
		t.Fatalf("an unconfirmed author's turn was billed: %v %q", m.err, m.res.Text)
	}
	if len(m.stub.YeuCau()) > 1 && strings.Contains(string(m.stub.YeuCau()[1]), "Khách") {
		t.Fatal("an unconfirmed author's turn reached the reading")
	}
}

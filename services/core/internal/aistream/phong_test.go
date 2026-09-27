package aistream

import (
	"encoding/json"
	"strings"
	"testing"
)

const (
	invThu  = "0b8f1c9e-aaaa-4bbb-8ccc-ddddddddee01"
	tinThu  = "0b8f1c9e-aaaa-4bbb-8ccc-ddddddddee02"
	tinNhan = "0b8f1c9e-aaaa-4bbb-8ccc-ddddddddee03"
)

func suKien(kind Kind, data string) Event {
	return Event{ID: "42-7", Kind: kind, Data: json.RawMessage(data), Inv: invThu, Tin: tinThu, SoTin: 6}
}

// The room sees the closed room vocabulary only (contract §4.5): hello,
// thu_hoi and ket_noi_lai belong to one SSE reader and never become a frame,
// nor does an entry that names no invocation or carries no stream id.
func TestKhungPhongChiEnumPhong(t *testing.T) {
	for _, k := range []Kind{Hello, ThuHoi, KetNoiLai, Kind("retract"), Kind("snapshot")} {
		if _, ok := khungPhong(suKien(k, `{}`)); ok {
			t.Errorf("%s became a room frame", k)
		}
	}
	for _, k := range []Kind{LamLai, Huy} {
		f, ok := khungPhong(suKien(k, `{"text":"không được lọt"}`))
		if !ok || string(f.D) != `{}` {
			t.Errorf("%s: %v %s, want an empty frame", k, ok, f.D)
		}
	}
	e := suKien(Delta, `{"p":0,"text":"Tối nay"}`)
	e.Inv = ""
	if _, ok := khungPhong(e); ok {
		t.Error("an entry without inv became a frame")
	}
	e = suKien(Delta, `{"p":0,"text":"Tối nay"}`)
	e.ID = ""
	if _, ok := khungPhong(e); ok {
		t.Error("an entry without an id became a frame")
	}
}

// Each kind's data is rebuilt from the fields it may carry: a Nếp-shaped
// ending's text, chips and sources never reach the room, whatever the entry
// held (design 02 §3.6: «phòng chỉ message_id»).
func TestKhungPhongDungLaiTungTruong(t *testing.T) {
	cases := []struct {
		kind Kind
		in   string
		want string
	}{
		{Xong, `{"message_id":"` + tinNhan + `","text":"câu trả lời bí mật","chips":["x"],"nguon":["y"]}`, `{"message_id":"` + tinNhan + `"}`},
		{TrangThai, `{"cau":"dang_doc","n":6,"extra":"x"}`, `{"cau":"dang_doc"}`},
		{Delta, `{"p":1,"text":"Hồ Xuân Hương","owner":"x"}`, `{"p":1,"text":"Hồ Xuân Hương"}`},
		{ThatBai, `{"code":"ai_tu_choi","detail":"nhay_cam"}`, `{"code":"ai_tu_choi"}`},
		{Phan, `{"kind":"places","json":{"ids":[1]},"x":1}`, `{"kind":"places","json":{"ids":[1]}}`},
	}
	for _, c := range cases {
		f, ok := khungPhong(suKien(c.kind, c.in))
		if !ok || string(f.D) != c.want {
			t.Errorf("%s: %v %s, want %s", c.kind, ok, f.D, c.want)
		}
		raw, _ := json.Marshal(f)
		var env map[string]any
		_ = json.Unmarshal(raw, &env)
		if env["type"] != "ai" || env["inv"] != invThu || env["tin"] != tinThu || env["so_tin"] != float64(6) || env["e"] != string(c.kind) || env["id"] != "42-7" {
			t.Errorf("%s: envelope %s", c.kind, raw)
		}
		if len(env) != 7 {
			t.Errorf("%s: the envelope has fields beyond the contract's seven: %s", c.kind, raw)
		}
	}
}

// Data that is not what its kind carries is dropped, not repaired.
func TestKhungPhongTuChoiDuLieuSai(t *testing.T) {
	for _, c := range []struct {
		kind Kind
		in   string
	}{
		{Xong, `{"message_id":"not-a-uuid"}`},
		{Xong, `{"text":"chỉ có chữ"}`},
		{Delta, `{"p":8,"text":"x"}`},
		{Delta, `{"p":-1,"text":"x"}`},
		{Delta, `{"p":0,"text":""}`},
		{Delta, `{"p":"0","text":"x"}`},
		{TrangThai, `{"cau":"Nếp đang nghĩ"}`},
		{ThatBai, `{"code":"Có lỗi"}`},
		{ThatBai, `{}`},
		{Phan, `{"kind":"places"}`},
	} {
		if f, ok := khungPhong(suKien(c.kind, c.in)); ok {
			t.Errorf("%s %s became %s", c.kind, c.in, f.D)
		}
	}
}

// A frame without a trigger or a count leaves those fields out.
func TestKhungPhongKhongTinKhongSo(t *testing.T) {
	e := suKien(TrangThai, `{"cau":"dang_doc"}`)
	e.Tin, e.SoTin = "", 0
	f, _ := khungPhong(e)
	raw, _ := json.Marshal(f)
	if strings.Contains(string(raw), "tin") || strings.Contains(string(raw), "so_tin") {
		t.Fatalf("empty envelope fields on the wire: %s", raw)
	}
}

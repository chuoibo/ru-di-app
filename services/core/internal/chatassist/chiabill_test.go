package chatassist

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"

	"mobile/services/core/internal/brain"
	"mobile/services/core/internal/domain/allocator"
	"mobile/services/core/internal/domain/companion"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/repo"
)

const (
	nguoiGoi = "fafafafa-fafa-4faf-afaf-fafafafafafa"
	lan      = "cdcdcdcd-cdcd-4cdc-bcdc-cdcdcdcdcdcd"
	tinMot   = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	tinHai   = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	tinBa    = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
)

func TestLenhNhomChiNhanHaiLenh(t *testing.T) {
	for _, ok := range []string{"plan", "chia_bill"} {
		if !lenhNhom(ok) {
			t.Errorf("%q phải là lệnh nhóm hợp lệ", ok)
		}
	}
	// `hoi` is the personal scope's command; the table refuses it for a group.
	for _, sai := range []string{"", "hoi", "Plan", "chia-bill", "chiabill", "chia_bill "} {
		if lenhNhom(sai) {
			t.Errorf("%q không được là lệnh nhóm", sai)
		}
	}
}

// The command check runs before any transaction, so it is observable without
// a database: an unknown command is a 400, chia_bill passes on to the auth
// step (401 without a session), exactly as plan does.
func TestTaoLoiGoiKiemLenhTruocKhiChamDB(t *testing.T) {
	h := New(nil, nil)
	gui := func(command string, token bool) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(map[string]string{"logical_id": tinMot, "command": command, "prompt": "Chia giúp nhóm"})
		r := httptest.NewRequest("POST", "/contexts/"+tinHai+"/ai-invocations", bytes.NewReader(raw))
		r.Header.Set("Content-Type", "application/json")
		if token {
			r.Header.Set("Authorization", "Bearer synthetic")
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	for _, sai := range []string{"hoi", "delete", ""} {
		w := gui(sai, false)
		if w.Code != 400 || !strings.Contains(w.Body.String(), "invalid_invocation") {
			t.Errorf("lệnh %q: %d %s, muốn 400 invalid_invocation", sai, w.Code, w.Body.String())
		}
	}
	for _, dung := range []string{"plan", "chia_bill"} {
		if w := gui(dung, false); w.Code != 401 {
			t.Errorf("lệnh %q phải qua kiểm lệnh rồi dừng ở xác thực, nhận %d %s", dung, w.Code, w.Body.String())
		}
	}
}

func goiChia(luot ...turn) []byte {
	raw, _ := json.Marshal(bundle{Ban: 1, Nguon: "chat-nhom", Luot: luot, TongLuot: len(luot)})
	return raw
}

func TestNguonChiaBillNguoiTraLaTacGiaThat(t *testing.T) {
	goi := goiChia(
		// The bundle says "toi", the database says Lan wrote it: Lan pays.
		turn{ID: tinMot, Vai: "toi", Loai: "chu", Chu: "Tao trả 300k tiền nước"},
		turn{ID: tinHai, Vai: "ban", Loai: "anh", Chu: "ảnh hoá đơn"},
		turn{ID: tinBa, Vai: "ban", Loai: "chu", Chu: "/vote Ăn gì? Phở | Bún"},
		turn{ID: "dddddddd-dddd-4ddd-8ddd-dddddddddddd", Vai: "ban", Loai: "chu", Chu: "tin không rõ ai viết"},
		turn{ID: "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee", Vai: "ai", Loai: "chu", Chu: "Rủ Đi AI nói 900k"},
	)
	authors := map[string]string{tinMot: lan, tinHai: lan, tinBa: lan, "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee": lan}
	got, err := nguonChiaBill(goi, "/chia-bill mình trả 450k ăn tối", nguoiGoi, authors)
	if err != nil {
		t.Fatal(err)
	}
	want := []nguonKhoan{
		{text: "Tao trả 300k tiền nước", payer: lan, source: tinMot},
		{text: "mình trả 450k ăn tối", payer: nguoiGoi},
	}
	if len(got) != len(want) {
		t.Fatalf("got %#v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("nguồn %d = %#v, muốn %#v", i, got[i], want[i])
		}
	}
	// A bare command carries no expense of its own and costs no model call.
	got, _ = nguonChiaBill(nil, "/chia-bill", nguoiGoi, nil)
	if len(got) != 0 {
		t.Fatalf("lệnh trơn không được đem đi đọc: %#v", got)
	}
}

func TestNguonChiaBillGiuTamTinMoiNhat(t *testing.T) {
	luot := []turn{}
	authors := map[string]string{}
	for i := 0; i < 12; i++ {
		id := strings.Replace(tinMot, "aaaaaaaa", strings.Repeat(string(rune('a'+i)), 8), 1)
		luot = append(luot, turn{ID: id, Vai: "ban", Loai: "chu", Chu: "khoản " + string(rune('A'+i))})
		authors[id] = lan
	}
	got, err := nguonChiaBill(goiChia(luot...), "Chia giúp", nguoiGoi, authors)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != maxLuotDocChi {
		t.Fatalf("đọc %d nguồn, trần là %d", len(got), maxLuotDocChi)
	}
	if got[0].text != "khoản F" || got[len(got)-1].payer != nguoiGoi {
		t.Fatalf("phải giữ các tin MỚI nhất và lời nhờ ở cuối: %#v", got)
	}
}

type docGia map[string]any

func (d docGia) doc(calls *[]string) docKhoan {
	return func(_ context.Context, text string) (pyjson.Value, error) {
		*calls = append(*calls, text)
		switch v := d[text].(type) {
		case error:
			return nil, v
		case string:
			return pyjson.Loads([]byte(v))
		}
		return pyjson.Loads([]byte(`{"is_expense":false,"title":null,"amount_vnd":null,"needs_review":false}`))
	}
}

func TestChiaBillGiuKhoanChiBoTinHong(t *testing.T) {
	fake := docGia{
		"nước":      `{"is_expense":true,"title":"Tiền nước","amount_vnd":300000,"needs_review":true}`,
		"phao":      `{"is_expense":true,"title":"Số thực","amount_vnd":180000.0,"needs_review":true}`,
		"am":        `{"is_expense":true,"title":"Âm","amount_vnd":-5,"needs_review":true}`,
		"ten":       &brain.Error{Status: 422, Code: "chat_expense_model_named_a_person"},
		"hong":      &brain.Error{Status: 422, Code: "chat_expense_unreadable"},
		"ăn tối":    `{"is_expense":true,"title":"Ăn tối","amount_vnd":450000,"needs_review":true}`,
		"tán gẫu":   nil,
		"quá lớn":   fmt.Sprintf(`{"is_expense":true,"title":"Quá lớn","amount_vnd":%d,"needs_review":true}`, int64(allocator.MaxAmountVND)+1),
		"trống tên": `{"is_expense":true,"title":"  ","amount_vnd":1000,"needs_review":true}`,
	}
	nguon := []nguonKhoan{}
	for _, text := range []string{"nước", "phao", "am", "ten", "hong", "tán gẫu", "quá lớn", "trống tên", "ăn tối"} {
		nguon = append(nguon, nguonKhoan{text: text, payer: lan, source: tinMot})
	}
	nguon[len(nguon)-1].payer, nguon[len(nguon)-1].source = nguoiGoi, ""
	var calls []string
	drafts, code := chiaBill(context.Background(), fake.doc(&calls), nguon)
	if code != "" {
		t.Fatalf("code=%q", code)
	}
	if len(calls) != len(nguon) {
		t.Fatalf("gọi mô hình %d lần, muốn %d", len(calls), len(nguon))
	}
	want := []khoanNhap{{title: "Tiền nước", amount: 300000, payer: lan, source: tinMot}, {title: "Ăn tối", amount: 450000, payer: nguoiGoi}}
	if len(drafts) != 2 || drafts[0] != want[0] || drafts[1] != want[1] {
		t.Fatalf("drafts=%#v", drafts)
	}
}

func TestChiaBillThatBaiThatTha(t *testing.T) {
	var calls []string
	_, code := chiaBill(context.Background(), docGia{"a": &brain.Error{Status: 502, Code: "brain_unavailable"}}.doc(&calls), []nguonKhoan{{text: "a"}, {text: "b"}})
	if code != "provider_unavailable" || len(calls) != 1 {
		t.Fatalf("mô hình vắng phải dừng ngay với provider_unavailable: code=%q calls=%d", code, len(calls))
	}
	calls = nil
	_, code = chiaBill(context.Background(), docGia{}.doc(&calls), []nguonKhoan{{text: "tán gẫu"}})
	if code != "chia_bill_no_expenses" {
		t.Fatalf("không khoản nào phải là chia_bill_no_expenses, nhận %q", code)
	}
	calls = nil
	_, code = chiaBill(context.Background(), docGia{}.doc(&calls), nil)
	if code != "chia_bill_no_expenses" || len(calls) != 0 {
		t.Fatalf("không nguồn thì không gọi mô hình: code=%q calls=%d", code, len(calls))
	}
}

func TestKetQuaChiaBillLaBanNhapNguyenVan(t *testing.T) {
	raw, err := ketQuaChiaBill([]khoanNhap{
		{title: "Tiền nước", amount: 300000, payer: lan, source: tinMot},
		{title: "Ăn tối", amount: 450000, payer: nguoiGoi},
	}, []string{nguoiGoi, lan})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"kind": "expense_draft", "drafts": [{"title": "Tiền nước", "amount_vnd": 300000, "paid_by_id": "` + lan + `", "shared_by": ["` + nguoiGoi + `", "` + lan + `"], "source_message_id": "` + tinMot + `", "needs_review": true}, {"title": "Ăn tối", "amount_vnd": 450000, "paid_by_id": "` + nguoiGoi + `", "shared_by": ["` + nguoiGoi + `", "` + lan + `"], "source_message_id": null, "needs_review": true}]}`
	var a, b any
	if err = json.Unmarshal(raw, &a); err != nil {
		t.Fatal(err)
	}
	_ = json.Unmarshal([]byte(want), &b)
	ga, _ := json.Marshal(a)
	gb, _ := json.Marshal(b)
	if !bytes.Equal(ga, gb) {
		t.Fatalf("result:\n%s\nmuốn:\n%s", raw, want)
	}
	// Integers on the wire, never a float, and no per-person share of any
	// kind: a split the server invented would be an allocation.
	if bytes.Contains(raw, []byte("300000.0")) || bytes.Contains(raw, []byte("share_vnd")) || bytes.Contains(raw, []byte("allocation")) {
		t.Fatalf("result chứa số thực hoặc phần chia: %s", raw)
	}
}

func TestTheChiaBillLaTheChuMotNguoiDocDuoc(t *testing.T) {
	members := []repo.Membership{
		{PersonID: nguoiGoi, DisplayName: "Minh", State: "active"},
		{PersonID: lan, DisplayName: "Lan", State: "active"},
		{PersonID: "ebebebeb-ebeb-4ebe-bebe-ebebebebebeb", DisplayName: "ebebebeb-ebeb-4ebe-bebe-ebebebebebeb", State: "active"},
		{PersonID: "dcdcdcdc-dcdc-4dcd-adcd-dcdcdcdcdcdc", DisplayName: "Đã rời", State: "left"},
	}
	drafts := []khoanNhap{{title: "Tiền nước", amount: 300000, payer: lan}, {title: "Ăn tối", amount: 1450000, payer: nguoiGoi}, {title: "Gửi xe", amount: 10000, payer: "ebebebeb-ebeb-4ebe-bebe-ebebebebebeb"}}
	shared := thanhVienDangO(members)
	text := theChiaBill(drafts, nhanNguoiTra(members), len(shared))
	for _, can := range []string{"chưa ghi vào sổ", "Lan trả 300.000đ: Tiền nước", "Minh trả 1.450.000đ: Ăn tối", "Một người trong nhóm trả 10.000đ", "Tổng 1.760.000đ", "3 người", "xác nhận"} {
		if !strings.Contains(text, can) {
			t.Errorf("thẻ thiếu %q:\n%s", can, text)
		}
	}
	// An account id never reaches the room, and «Mình» would name every reader.
	if strings.Contains(text, "ebebebeb") || strings.Contains(text, "Mình") {
		t.Fatalf("thẻ lộ id hoặc gọi người trả là «Mình»:\n%s", text)
	}
	card, err := theChu(text)
	if err != nil {
		t.Fatal(err)
	}
	var shape struct {
		Kind    string            `json:"kind"`
		Payload map[string]string `json:"payload"`
	}
	if err = json.Unmarshal(card, &shape); err != nil || shape.Kind != "text" || shape.Payload["text"] != text || len(shape.Payload) != 1 {
		t.Fatalf("thẻ phải đúng hình kind:text của GroundCard: %s (%v)", card, err)
	}
}

func TestTheChiaBillKhongBaoGioBiCatGiuaCau(t *testing.T) {
	drafts := []khoanNhap{}
	for i := 0; i < maxLuotDocChi; i++ {
		drafts = append(drafts, khoanNhap{title: strings.Repeat("Một khoản rất dài ", 12), amount: int64(allocator.MaxAmountVND) - 1, payer: lan})
	}
	text := theChiaBill(drafts, map[string]string{lan: strings.Repeat("Tên dài ", 10)}, 5)
	if n := utf8.RuneCountInString(text); n > companion.MaxText {
		t.Fatalf("thẻ %d ký tự, trần %d", n, companion.MaxText)
	}
	if !strings.Contains(text, "khoản nữa") || !strings.HasSuffix(text, "Rủ Đi AI không tự ghi khoản nào.") {
		t.Fatalf("thẻ dài phải gộp dòng và giữ câu cuối:\n%s", text)
	}
}

func TestDongChiaNhomBaSo(t *testing.T) {
	for n, want := range map[int64]string{1: "1đ", 999: "999đ", 1000: "1.000đ", 450000: "450.000đ", int64(allocator.MaxAmountVND): "1" + strings.Repeat(".000", 4) + "đ"} {
		if got := dong(n); got != want {
			t.Errorf("dong(%d)=%q muốn %q", n, got, want)
		}
	}
}

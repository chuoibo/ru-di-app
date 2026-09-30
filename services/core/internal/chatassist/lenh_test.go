package chatassist

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLenhNhomChiNhanHaiLenh(t *testing.T) {
	h := &Handler{}
	for _, ok := range []string{"plan", "chia_bill"} {
		if !h.lenhNhom(ok, false) || !h.lenhNhom(ok, true) {
			t.Errorf("%q phải là lệnh nhóm hợp lệ", ok)
		}
	}
	for _, sai := range []string{"", "Plan", "chia-bill", "chiabill", "chia_bill "} {
		if h.lenhNhom(sai, true) {
			t.Errorf("%q không được là lệnh nhóm", sai)
		}
	}
	// `hoi` is an answer in the thread: it needs exactly one tag message.
	if !h.lenhNhom("hoi", true) || h.lenhNhom("hoi", false) {
		t.Error("hoi của nhóm cần đúng một tin tag")
	}
}

// The command check runs before any transaction, so it is observable without
// a database: an unknown command is a 400, chia_bill passes on to the auth
// step (401 without a session), exactly as plan does.
func TestTaoLoiGoiKiemLenhTruocKhiChamDB(t *testing.T) {
	h := New(nil)
	gui := func(command string, token bool) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(map[string]string{"logical_id": "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", "command": command, "prompt": "Chia giúp nhóm"})
		r := httptest.NewRequest("POST", "/contexts/bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"+"/ai-invocations", bytes.NewReader(raw))
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

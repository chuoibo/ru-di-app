package chatassist

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"mobile/services/core/internal/aiharness/cau"
	"mobile/services/core/internal/aistream"
	"mobile/services/core/internal/featureroute"
)

// khoa hands the room key to exactly one combination: a group job in the
// legacy lane. Nếp, a v2 job, and a lane this binary does not know all get
// the invocation key (design 02 §9, the unit side of canary 2).
func TestKhoaChiMotToHopGhiKhoaPhong(t *testing.T) {
	keys, err := aistream.NewKeys("t")
	if err != nil {
		t.Fatal(err)
	}
	const id, room = "0b8f1c9e-aaaa-4bbb-8ccc-dddddddddd01", "0b8f1c9e-aaaa-4bbb-8ccc-dddddddddd02"
	phong, _ := keys.Room(room)
	moi, _ := keys.Invocation(id)
	soPhong := 0
	for _, scope := range []string{"group", scopeMe} {
		for _, lane := range []string{laneLegacy, laneV2, "", "legacy2"} {
			key, maxLen, inv, err := khoa(keys, work{id: id, conversation: room, scope: scope, lane: lane})
			if err != nil {
				t.Fatal(err)
			}
			if key == phong {
				soPhong++
				if scope != "group" || lane != laneLegacy || inv != id || maxLen != aistream.MaxLenRoom {
					t.Errorf("scope %s lane %q: room key (inv %q)", scope, lane, inv)
				}
				continue
			}
			if key != moi || inv != "" || maxLen != aistream.MaxLenInvocation {
				t.Errorf("scope %s lane %q: key %s inv %q", scope, lane, key, inv)
			}
		}
	}
	if soPhong != 1 {
		t.Fatalf("%d combinations write the room key, want exactly one", soPhong)
	}
}

// ServeHTTP puts no deadline on a stream (GET …/events), and keeps the 8 s on
// every other route of the engine (design 02 §9, the unit side of M4).
func TestServeHTTPKhongDatHanChoEvents(t *testing.T) {
	type thay struct {
		coHan bool
		con   time.Duration
	}
	var seen thay
	h := &Handler{mux: featureroute.NewMux()}
	ghi := func(w http.ResponseWriter, r *http.Request) {
		d, ok := r.Context().Deadline()
		seen = thay{ok, time.Until(d)}
	}
	h.mux.HandleFunc(routeSuKienNhom, ghi)
	h.mux.HandleFunc(routeSuKienNep, ghi)
	h.mux.HandleFunc("GET /contexts/{context}/ai-invocations/{id}", ghi)
	h.mux.HandleFunc("POST /contexts/{context}/ai-invocations/{id}/events", ghi)
	for _, c := range []struct {
		method, path string
		coHan        bool
	}{
		{"GET", "/contexts/c/ai-invocations/i/events", false},
		{"GET", "/me/nep/ai-invocations/i/events", false},
		{"GET", "/contexts/c/ai-invocations/i", true},
		{"POST", "/contexts/c/ai-invocations/i/events", true},
		// Ends in «/events» but is the invocation route with id «events»
		// (review of slice 11, finding 12): it keeps its bound.
		{"GET", "/contexts/c/ai-invocations/events", true},
	} {
		seen = thay{}
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(c.method, c.path, nil))
		if seen.coHan != c.coHan || (c.coHan && (seen.con <= 7*time.Second || seen.con > 8*time.Second)) {
			t.Errorf("%s %s: deadline %v (%v left), want %v", c.method, c.path, seen.coHan, seen.con, c.coHan)
		}
	}
}

// The room never learns why an answer was stopped.
func TestMaPhong(t *testing.T) {
	for _, m := range []cau.Ma{cau.TraLoiBiChan, cau.NepKhongChamTien, cau.NepLuiManTien} {
		if maPhong(string(m)) != maChanChung {
			t.Errorf("%s reaches a room as itself", m)
		}
	}
	for _, m := range []string{"provider_unavailable", "sharing_unavailable", "trigger_deleted"} {
		if maPhong(m) != m {
			t.Errorf("%s was rewritten", m)
		}
	}
}

// Five streams per person, and the process bound.
func TestSucChuaSSE(t *testing.T) {
	var c sucChuaSSE
	for i := 0; i < sseMoiNguoi; i++ {
		if !c.giu("a") {
			t.Fatalf("stream %d refused", i+1)
		}
	}
	if c.giu("a") {
		t.Fatal("a sixth stream for one person was let in")
	}
	if !c.giu("b") {
		t.Fatal("another person was refused")
	}
	c.tra("a")
	if !c.giu("a") {
		t.Fatal("a freed slot stayed taken")
	}
	c = sucChuaSSE{tran: 2}
	if !c.giu("x") || !c.giu("y") || c.giu("z") {
		t.Fatal("the process bound did not hold")
	}
}

// A reader with nothing in the stream yet hears the row: queued or running is
// dang_xep_hang and the stream goes on; an ending closes with it.
func TestDauTuHang(t *testing.T) {
	text, msg, code := "Đi dạo hồ nhé.", "0b8f1c9e-aaaa-4bbb-8ccc-dddddddddd09", "provider_unavailable"
	for _, c := range []struct {
		d    dongSSE
		kind aistream.Kind
		data string
		done bool
	}{
		{dongSSE{status: "queued"}, aistream.TrangThai, `{"cau":"dang_xep_hang"}`, false},
		{dongSSE{status: "running"}, aistream.TrangThai, `{"cau":"dang_xep_hang"}`, false},
		{dongSSE{status: "succeeded", j: work{scope: scopeMe}, text: &text}, aistream.Xong, `{"chips":[],"nguon":[],"text":"Đi dạo hồ nhé."}`, true},
		{dongSSE{status: "succeeded", j: work{scope: "group"}, message: &msg}, aistream.Xong, `{"message_id":"` + msg + `"}`, true},
		{dongSSE{status: "failed", code: &code}, aistream.ThatBai, `{"code":"provider_unavailable"}`, true},
		{dongSSE{status: "cancelled"}, aistream.Huy, `{}`, true},
	} {
		events, done := dauTuHang(c.d)
		if len(events) != 1 || events[0].Kind != c.kind || string(events[0].Data) != c.data || done != c.done || events[0].ID != "" {
			t.Errorf("%s: %+v done=%v", c.d.status, events, done)
		}
	}
}

// The prose of a card is what passes the window: a text card, and the text
// parts of a tra_loi card by index.
func TestChuCuaThe(t *testing.T) {
	text := chuCuaThe([]byte(`{"kind":"text","payload":{"text":"Chào cả hội"}}`))
	if len(text) != 1 || text[0].text != "Chào cả hội" || text[0].i != 0 {
		t.Fatalf("%+v", text)
	}
	reply := chuCuaThe([]byte(`{"kind":"tra_loi","payload":{"phan":[{"kind":"places","payload":{}},{"kind":"text","payload":{"text":"Hai"}}]}}`))
	if len(reply) != 1 || reply[0].text != "Hai" || reply[0].i != 1 {
		t.Fatalf("%+v", reply)
	}
	if chuCuaThe([]byte(`{"kind":"places","payload":{}}`)) != nil {
		t.Fatal("a places card has prose")
	}
}

// ghiChu records Deltas.
type ghiChu struct{ ds []string }

func (g *ghiChu) Delta(_ int, s string) { g.ds = append(g.ds, s) }

// A group card the guard stops leaves nothing in the stream, even when its
// first text part was clean.
func TestChuTheNhomChanCaThe(t *testing.T) {
	g := &ghiChu{}
	card, _ := json.Marshal(map[string]any{"kind": "tra_loi", "payload": map[string]any{"phan": []any{
		map[string]any{"kind": "text", "payload": map[string]any{"text": "Tối nay cả nhóm ra hồ nhé."}},
		map[string]any{"kind": "text", "payload": map[string]any{"text": "Gọi 0912 " + "345 678 để giữ bàn."}},
	}}})
	if chuTheNhom(context.Background(), g, card, 0) || len(g.ds) != 0 {
		t.Fatalf("stopped card left %q", g.ds)
	}
	ok := chuTheNhom(context.Background(), g, []byte(`{"kind":"text","payload":{"text":"Tối nay cả nhóm ra hồ nhé."}}`), 0)
	if !ok || strings.Join(g.ds, "") != "Tối nay cả nhóm ra hồ nhé." || len(g.ds) != 1 {
		t.Fatalf("clean card: %v %q", ok, g.ds)
	}
}

//go:build postgres

package chatassist

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
)

// Nếp end to end against a real database: the personal scope on the same
// queue, a sealed answer returned to the caller alone, nothing published, and
// nothing read for context but what the device sent.

type nepGia struct {
	mu     sync.Mutex
	calls  int
	bodies []string
}

func (n *nepGia) serve(w http.ResponseWriter, r *http.Request) {
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	b, _ := json.Marshal(body)
	n.mu.Lock()
	n.calls++
	n.bodies = append(n.bodies, r.URL.Path+" "+string(b))
	n.mu.Unlock()
	reply(w, 200, map[string]string{"text": "Tối nay đi dạo hồ nhé."})
}

func (f fixture) nepPost(t *testing.T, token string, body map[string]any) (int, NepInvocation, string) {
	t.Helper()
	w := f.request("POST", "/me/nep/ai-invocations", token, body)
	var v NepInvocation
	_ = json.Unmarshal(w.Body.Bytes(), &v)
	return w.Code, v, w.Body.String()
}

func nepThan(prompt string) map[string]any {
	return map[string]any{
		"logical_id": newID(),
		"prompt":     prompt,
		"phieu":      map[string]any{"man": "explore", "tieuDe": "Khám phá", "soLieu": map[string]any{"soNguoi": 4}},
		"luot":       []map[string]string{{"vai": "toi", "chu": "Mình thích yên tĩnh"}, {"vai": "nep", "chu": "Vậy mình gợi ý chỗ vắng."}},
	}
}

func TestNepTraKinVeDungNguoiGoi(t *testing.T) {
	model := &nepGia{}
	f := setup(t, model.serve)
	ctx := context.Background()
	if _, err := f.pool.Exec(ctx, `INSERT INTO messages(id,context_id,author_id,kind,body) VALUES($1,$2,$3,'text','Synthetic group chat sentinel')`, newID(), f.context, f.peer); err != nil {
		t.Fatal(err)
	}
	var tinTruoc int
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM messages`).Scan(&tinTruoc)

	code, job, raw := f.nepPost(t, f.token, nepThan("Tối nay đi đâu?"))
	if code != 202 || job.Status != "queued" || job.Text != nil {
		t.Fatalf("status=%d body=%s", code, raw)
	}
	if ok, err := f.handler.ProcessOne(ctx); !ok || err != nil {
		t.Fatalf("ProcessOne=%v %v", ok, err)
	}
	w := f.request("GET", "/me/nep/ai-invocations/"+job.ID, f.token, nil)
	requireCode(t, w, 200)
	var done NepInvocation
	_ = json.Unmarshal(w.Body.Bytes(), &done)
	if done.Status != "succeeded" || done.Text == nil || *done.Text != "Tối nay đi dạo hồ nhé." {
		t.Fatalf("kết quả: %s", w.Body.String())
	}

	// Exactly one model call, to nep-reply, carrying only the three things.
	if model.calls != 1 || !strings.HasPrefix(model.bodies[0], "/internal/brain/v1/nep-reply ") {
		t.Fatalf("gọi não: %v", model.bodies)
	}
	for _, cam := range []string{"Synthetic group chat sentinel", "Synthetic caller", "Synthetic peer", "Synthetic job group", f.context, f.person, "places", "members", "budget"} {
		if strings.Contains(model.bodies[0], cam) {
			t.Errorf("thân gửi não chứa %q: %s", cam, model.bodies[0])
		}
	}
	for _, can := range []string{"Mình thích yên tĩnh", "Khám phá", "Tối nay đi đâu?"} {
		if !strings.Contains(model.bodies[0], can) {
			t.Errorf("thân gửi não thiếu %q", can)
		}
	}

	// Sealed: nothing published, the question and the session gone.
	var tinSau int
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM messages`).Scan(&tinSau)
	if tinSau != tinTruoc {
		t.Fatalf("Nếp ghi %d tin vào phòng", tinSau-tinTruoc)
	}
	var promptNull, goiNull, msgNull, ctxNull bool
	var scope string
	if err := f.pool.QueryRow(ctx, `SELECT prompt IS NULL, boi_canh IS NULL, message_id IS NULL, context_id IS NULL, scope FROM chat_ai_invocations WHERE id=$1`, job.ID).Scan(&promptNull, &goiNull, &msgNull, &ctxNull, &scope); err != nil {
		t.Fatal(err)
	}
	if !promptNull || !goiNull || !msgNull || !ctxNull || scope != "me" {
		t.Fatalf("hàng việc còn giữ: prompt=%v boi_canh=%v message=%v context=%v scope=%s", !promptNull, !goiNull, !msgNull, !ctxNull, scope)
	}

	// Nobody else reads it, and it is not on the group's list.
	requireCode(t, f.request("GET", "/me/nep/ai-invocations/"+job.ID, f.peerToken, nil), 404)
	list := f.request("GET", f.route(), f.token, nil)
	requireCode(t, list, 200)
	if strings.Contains(list.Body.String(), job.ID) {
		t.Fatal("câu hỏi riêng hiện trên danh sách của nhóm")
	}

	// The answer goes when the sharing window closes.
	if _, err := f.pool.Exec(ctx, `UPDATE chat_ai_invocations SET share_expires_at=clock_timestamp()-interval '1 second' WHERE id=$1`, job.ID); err != nil {
		t.Fatal(err)
	}
	_, _ = f.handler.ProcessOne(ctx)
	var resultNull bool
	_ = f.pool.QueryRow(ctx, `SELECT result IS NULL FROM chat_ai_invocations WHERE id=$1`, job.ID).Scan(&resultNull)
	if !resultNull {
		t.Fatal("câu trả lời kín còn nằm lại sau khi hết hạn chia sẻ")
	}
}

func TestNepManTienKhongGhiHangKhongGoiNao(t *testing.T) {
	model := &nepGia{}
	f := setup(t, model.serve)
	body := nepThan("Mình nợ ai?")
	body["phieu"] = map[string]any{"man": "settlements/abc"}
	code, _, raw := f.nepPost(t, f.token, body)
	if code != 403 || !strings.Contains(raw, "nep_lui_man_tien") {
		t.Fatalf("status=%d body=%s", code, raw)
	}
	var n int
	_ = f.pool.QueryRow(context.Background(), `SELECT count(*) FROM chat_ai_invocations`).Scan(&n)
	if n != 0 || model.calls != 0 || f.capabilityCalls.Load() != 0 {
		t.Fatalf("màn tiền vẫn chạm: hàng=%d gọi=%d thăm dò=%d", n, model.calls, f.capabilityCalls.Load())
	}
}

func TestNepLapLaiVaXungDot(t *testing.T) {
	f := setup(t, (&nepGia{}).serve)
	body := nepThan("Đi đâu?")
	code, first, _ := f.nepPost(t, f.token, body)
	if code != 202 {
		t.Fatalf("status=%d", code)
	}
	code, again, _ := f.nepPost(t, f.token, body)
	if code != 200 || again.ID != first.ID {
		t.Fatalf("lặp lại: %d %s != %s", code, again.ID, first.ID)
	}
	// Same key, one more session turn: a different question, never the old answer.
	body["luot"] = []map[string]string{{"vai": "toi", "chu": "Khác rồi"}}
	if code, _, raw := f.nepPost(t, f.token, body); code != 409 || !strings.Contains(raw, "invocation_conflict") {
		t.Fatalf("xung đột: %d %s", code, raw)
	}
	// The same key belongs to each person separately.
	if code, _, raw := f.nepPost(t, f.peerToken, body); code != 202 {
		t.Fatalf("người khác cùng khoá: %d %s", code, raw)
	}
}

func TestNepPhienBiThuHoiThiKhongGoiNao(t *testing.T) {
	model := &nepGia{}
	f := setup(t, model.serve)
	ctx := context.Background()
	_, job, _ := f.nepPost(t, f.token, nepThan("Đi đâu?"))
	if _, err := f.pool.Exec(ctx, `UPDATE account_sessions SET revoked_at=clock_timestamp() WHERE person_id=$1`, f.person); err != nil {
		t.Fatal(err)
	}
	if ok, err := f.handler.ProcessOne(ctx); !ok || err != nil {
		t.Fatalf("ProcessOne=%v %v", ok, err)
	}
	var status, code string
	var promptNull, goiNull bool
	_ = f.pool.QueryRow(ctx, `SELECT status, code, prompt IS NULL, boi_canh IS NULL FROM chat_ai_invocations WHERE id=$1`, job.ID).Scan(&status, &code, &promptNull, &goiNull)
	if status != "failed" || code != "sharing_unavailable" || !promptNull || !goiNull || model.calls != 0 {
		t.Fatalf("status=%s code=%s prompt_null=%v boi_canh_null=%v gọi=%d", status, code, promptNull, goiNull, model.calls)
	}
}

func TestNepChungHanMucVoiNhom(t *testing.T) {
	f := setup(t, (&nepGia{}).serve)
	for i := 0; i < 4; i++ {
		f.create(t)
	}
	for i := 0; i < 4; i++ {
		if code, _, raw := f.nepPost(t, f.token, nepThan("Câu "+string(rune('a'+i)))); code != 202 {
			t.Fatalf("lượt %d: %d %s", i, code, raw)
		}
	}
	if code, _, raw := f.nepPost(t, f.token, nepThan("Câu thứ chín")); code != 429 || !strings.Contains(raw, "invocation_rate_limited") {
		t.Fatalf("hạn mức chung: %d %s", code, raw)
	}
}

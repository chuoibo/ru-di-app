//go:build postgres

package chatassist

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// Nếp end to end against a real database: the personal scope on the same
// queue, a sealed answer returned to the caller alone, nothing published, and
// nothing read for context but what the device sent.

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
	model := &mayGia{traLoi: "Tối nay đi dạo hồ nhé."}
	f := setup(t, model)
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

	// The model heard what the device sent (datamarked: a space is ˆ) and
	// nothing the server owns about any room.
	heard := model.TatCa()
	if model.SoGoi() == 0 {
		t.Fatal("không lời gọi model nào")
	}
	for _, cam := range []string{"Synthetic group chat sentinel", "Synthetic caller", "Synthetic peer", "Synthetic job group", f.context, f.person, f.peer} {
		if strings.Contains(heard, cam) {
			t.Errorf("lời gửi model chứa %q", cam)
		}
	}
	for _, can := range []string{"Mìnhˆthíchˆyênˆtĩnh", "Khám", "đi đâu"} {
		if !strings.Contains(heard, can) {
			t.Errorf("lời gửi model thiếu %q", can)
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
	model := &mayGia{}
	f := setup(t, model)
	body := nepThan("Mình nợ ai?")
	body["phieu"] = map[string]any{"man": "settlements/abc"}
	code, _, raw := f.nepPost(t, f.token, body)
	if code != 403 || !strings.Contains(raw, "nep_lui_man_tien") {
		t.Fatalf("status=%d body=%s", code, raw)
	}
	var n int
	_ = f.pool.QueryRow(context.Background(), `SELECT count(*) FROM chat_ai_invocations`).Scan(&n)
	if n != 0 || model.SoGoi() != 0 {
		t.Fatalf("màn tiền vẫn chạm: hàng=%d gọi=%d", n, model.SoGoi())
	}
}

func TestNepLapLaiVaXungDot(t *testing.T) {
	f := setup(t, nil)
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
	model := &mayGia{}
	f := setup(t, model)
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
	if status != "failed" || code != "sharing_unavailable" || !promptNull || !goiNull || model.SoGoi() != 0 {
		t.Fatalf("status=%s code=%s prompt_null=%v boi_canh_null=%v gọi=%d", status, code, promptNull, goiNull, model.SoGoi())
	}
}

func TestNepChungHanMucVoiNhom(t *testing.T) {
	f := setup(t, nil)
	for i := 0; i < 4; i++ {
		job := f.create(t)
		// Settled at once: the room holds at most three jobs in flight
		// (ADR-0046), and this test is about the per-person limit, which
		// counts every job created in the last minute whatever its status.
		if _, err := f.pool.Exec(context.Background(), `UPDATE chat_ai_invocations SET status='cancelled',prompt=NULL,boi_canh=NULL WHERE id=$1`, job.ID); err != nil {
			t.Fatal(err)
		}
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

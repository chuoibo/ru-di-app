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

// chia_bill end to end against a real database: the same queue as plan, the
// existing chat-expense skill in place of companion-reply, a text card in the
// room and the drafts on the invocation's `result` column.

type docChiGia struct {
	mu      sync.Mutex
	paths   []string
	texts   []string
	answers map[string]string
}

func (d *docChiGia) serve(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Text string `json:"text"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	d.mu.Lock()
	d.paths = append(d.paths, r.URL.Path)
	d.texts = append(d.texts, body.Text)
	d.mu.Unlock()
	for needle, answer := range d.answers {
		if strings.Contains(body.Text, needle) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(answer))
			return
		}
	}
	reply(w, 200, map[string]any{"is_expense": false, "title": nil, "amount_vnd": nil, "needs_review": false})
}

func TestChiaBillGhiKetQuaVaTheChuKhongChamSoTien(t *testing.T) {
	model := &docChiGia{answers: map[string]string{
		"300k": `{"is_expense":true,"title":"Tiền nước","amount_vnd":300000,"needs_review":true}`,
		"450k": `{"is_expense":true,"title":"Ăn tối","amount_vnd":450000,"needs_review":true}`,
	}}
	f := setup(t, model.serve)
	ctx := context.Background()
	// The stored body is a sentinel; what the caller shared is different
	// bytes. Only the shared bytes may ever reach the model.
	tin := f.tinTrongPhong(t, f.context, "Synthetic stored body sentinel")
	w := f.request("POST", f.route(), f.token, map[string]any{
		"logical_id": newID(), "command": "chia_bill", "prompt": "/chia-bill mình trả 450k ăn tối",
		"boi_canh": goiThu(luotThu(tin, "Tao trả 300k tiền nước")),
	})
	requireCode(t, w, 202)
	var job Invocation
	if err := json.Unmarshal(w.Body.Bytes(), &job); err != nil {
		t.Fatal(err)
	}
	if job.Command != "chia_bill" {
		t.Fatalf("command=%q", job.Command)
	}
	if ok, err := f.handler.ProcessOne(ctx); err != nil || !ok {
		t.Fatalf("worker: %v %v", ok, err)
	}
	for _, p := range model.paths {
		if p != "/internal/brain/v1/chat-expense" {
			t.Fatalf("chia_bill gọi nhầm skill: %s", p)
		}
	}
	if strings.Join(model.texts, "|") != "Tao trả 300k tiền nước|mình trả 450k ăn tối" {
		t.Fatalf("mô hình đọc sai nguồn: %q", model.texts)
	}

	var status string
	var message *string
	var prompt, goi, result []byte
	if err := f.pool.QueryRow(ctx, `SELECT status,message_id::text,prompt,boi_canh,result FROM chat_ai_invocations WHERE id=$1`, job.ID).Scan(&status, &message, &prompt, &goi, &result); err != nil {
		t.Fatal(err)
	}
	if status != "succeeded" || message == nil || prompt != nil || goi != nil {
		t.Fatalf("status=%s message=%v prompt=%q boi_canh=%q", status, message, prompt, goi)
	}
	var ket struct {
		Kind   string `json:"kind"`
		Drafts []struct {
			Title    string   `json:"title"`
			Amount   int64    `json:"amount_vnd"`
			PaidBy   string   `json:"paid_by_id"`
			SharedBy []string `json:"shared_by"`
			Source   *string  `json:"source_message_id"`
			Review   bool     `json:"needs_review"`
		} `json:"drafts"`
	}
	if err := json.Unmarshal(result, &ket); err != nil {
		t.Fatal(err)
	}
	if ket.Kind != "expense_draft" || len(ket.Drafts) != 2 {
		t.Fatalf("result=%s", result)
	}
	d0, d1 := ket.Drafts[0], ket.Drafts[1]
	// Who paid is the author the database holds, never the model's answer.
	if d0.PaidBy != f.peer || d0.Amount != 300000 || d0.Source == nil || *d0.Source != tin || !d0.Review {
		t.Fatalf("draft 0=%+v", d0)
	}
	if d1.PaidBy != f.person || d1.Amount != 450000 || d1.Source != nil || len(d1.SharedBy) != 2 {
		t.Fatalf("draft 1=%+v", d1)
	}

	var card []byte
	if err := f.pool.QueryRow(ctx, `SELECT card FROM messages WHERE id=$1 AND kind='ai_card'`, *message).Scan(&card); err != nil {
		t.Fatal(err)
	}
	var the struct {
		Kind    string            `json:"kind"`
		Payload map[string]string `json:"payload"`
	}
	if err := json.Unmarshal(card, &the); err != nil || the.Kind != "text" {
		t.Fatalf("card=%s", card)
	}
	if !strings.Contains(the.Payload["text"], "chưa ghi vào sổ") || !strings.Contains(the.Payload["text"], "Synthetic peer trả 300.000đ") {
		t.Fatalf("thẻ chữ thiếu nội dung: %s", the.Payload["text"])
	}

	// The public job never carries the drafts back.
	body := f.request("GET", f.route()+"/"+job.ID, f.token, nil).Body.String()
	if strings.Contains(body, "300000") || strings.Contains(body, "drafts") {
		t.Fatalf("phản hồi job để lộ kết quả: %s", body)
	}
}

func TestChiaBillKhongKhoanNaoThiKhongDangThe(t *testing.T) {
	model := &docChiGia{answers: map[string]string{}}
	f := setup(t, model.serve)
	ctx := context.Background()
	tin := f.tinTrongPhong(t, f.context, "Synthetic stored body sentinel")
	w := f.request("POST", f.route(), f.token, map[string]any{
		"logical_id": newID(), "command": "chia_bill", "prompt": "Chia giúp nhóm",
		"boi_canh": goiThu(luotThu(tin, "Tối nay vui ghê")),
	})
	requireCode(t, w, 202)
	var job Invocation
	_ = json.Unmarshal(w.Body.Bytes(), &job)
	if _, err := f.handler.ProcessOne(ctx); err != nil {
		t.Fatal(err)
	}
	var status, code string
	var result []byte
	if err := f.pool.QueryRow(ctx, `SELECT status,code,result FROM chat_ai_invocations WHERE id=$1`, job.ID).Scan(&status, &code, &result); err != nil {
		t.Fatal(err)
	}
	if status != "failed" || code != "chia_bill_no_expenses" || result != nil {
		t.Fatalf("status=%s code=%s result=%s", status, code, result)
	}
	var cards int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM messages WHERE kind='ai_card'`).Scan(&cards); err != nil || cards != 0 {
		t.Fatalf("không có khoản nào mà vẫn đăng thẻ: %d %v", cards, err)
	}
}

// Same idempotency as plan, and the command is part of the digest: the same
// logical id with a different command is a conflict, not a replay.
func TestChiaBillCungKhoaKhacLenhLaXungDot(t *testing.T) {
	f := setup(t, (&docChiGia{}).serve)
	input := map[string]any{"logical_id": newID(), "command": "chia_bill", "prompt": "Chia giúp nhóm"}
	requireCode(t, f.request("POST", f.route(), f.token, input), 202)
	requireCode(t, f.request("POST", f.route(), f.token, input), 200)
	input["command"] = "plan"
	requireCode(t, f.request("POST", f.route(), f.token, input), 409)
}

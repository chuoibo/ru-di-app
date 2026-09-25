//go:build postgres

package routes

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"mobile/services/core/internal/db"
	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/idem"
	"mobile/services/core/internal/repo"
	"mobile/services/core/internal/testdb"
)

// Replying to the group AI's answer is how a person asks a follow-up
// (ADR-0039 §2.5, proposed): a Go-only exception placed before the oracle's
// CheckReplyTarget. Every other card -- a poll, an older AI card, a card that
// only claims to be an answer -- still meets the oracle's 422.
func TestTraLoiVaoCauTraLoiCuaAi(t *testing.T) {
	ctx := context.Background()
	pool := testdb.Pool(t)
	var person string
	if err := pool.QueryRow(ctx, `INSERT INTO people (id,display_name) VALUES (gen_random_uuid(),'Trả lời AI (dữ liệu mẫu)') RETURNING id`).Scan(&person); err != nil {
		t.Fatal(err)
	}
	store := repo.Repository{Q: pool}
	group, err := store.CreateContext(ctx, "Trả lời AI (dữ liệu mẫu)", person)
	if err != nil {
		t.Fatal(err)
	}
	other, err := store.CreateContext(ctx, "Phòng khác (dữ liệu mẫu)", person)
	if err != nil {
		t.Fatal(err)
	}
	for _, room := range []string{group.ID, other.ID} {
		if _, err := pool.Exec(ctx, `INSERT INTO memberships(id,context_id,person_id,state,role,origin) VALUES (gen_random_uuid(),$1,$2,'active','admin','named')`, room, person); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		for _, room := range []string{group.ID, other.ID} {
			_, _ = pool.Exec(ctx, `DELETE FROM idempotency_keys WHERE idempotency_key LIKE $1`, "tra-loi-ai-"+room+"%")
			_, _ = pool.Exec(ctx, `DELETE FROM messages WHERE context_id=$1 AND reply_to_id IS NOT NULL`, room)
			_, _ = pool.Exec(ctx, `DELETE FROM messages WHERE context_id=$1`, room)
			_, _ = pool.Exec(ctx, `DELETE FROM votes WHERE context_id=$1`, room)
			_, _ = pool.Exec(ctx, `DELETE FROM memberships WHERE context_id=$1`, room)
			_, _ = pool.Exec(ctx, `DELETE FROM contexts WHERE id=$1`, room)
		}
		_, _ = pool.Exec(ctx, `DELETE FROM people WHERE id=$1`, person)
	})
	answer := `{"kind":"tra_loi","payload":{"ban":1,"tac_gia":"rudi-ai","invocation_id":"0b7c8a1e-2f43-4c55-9a8e-1d2f3a4b5c6d","lenh":"plan","doc":{"so_tin":2,"chi_loi_nho":false},"phan":[{"kind":"text","payload":{"text":"Tối nay ăn lẩu ở Q1 nhé, quán mở tới 23 giờ và vừa túi cả hội; nhớ đặt bàn trước bảy giờ."}}]}}`
	insert := func(room string, author *string, card string) string {
		t.Helper()
		var id string
		if err := pool.QueryRow(ctx, `INSERT INTO messages(id,context_id,author_id,kind,card) VALUES(gen_random_uuid(),$1,$2,'ai_card',$3) RETURNING id`, room, author, card).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	targets := map[string]struct {
		id     string
		status int
		code   string
	}{
		"câu trả lời của AI":              {insert(group.ID, nil, answer), 201, ""},
		"thẻ bình chọn":                   {insert(group.ID, &person, `{"kind":"poll","payload":{"vote_id":"v","question":"Ăn gì?","options":[]}}`), 422, "reply_target_not_quotable"},
		"thẻ AI cũ":                       {insert(group.ID, nil, `{"kind":"text","payload":{"text":"Thẻ cũ"}}`), 422, "reply_target_not_quotable"},
		"thẻ tự nhận là câu trả lời":      {insert(group.ID, &person, answer), 422, "reply_target_not_quotable"},
		"câu trả lời của AI ở phòng khác": {insert(other.ID, nil, answer), 404, "message_not_found"},
	}
	env := endpoint.Env{Mode: endpoint.ModeDev, NewUnit: func() *db.Unit { return db.NewUnit(pool) }, Now: time.Now}
	h := coreWithEnv(t, env, idem.New(idem.NewPostgresStore(pool)))
	for name, target := range targets {
		body := fmt.Sprintf(`{"kind":"text","body":"Còn quán nào gần hơn không?","reply_to_id":%q}`, target.id)
		r := httptest.NewRequest(http.MethodPost, "/contexts/"+group.ID+"/messages", strings.NewReader(body))
		r.Header.Set("X-Actor-ID", person)
		r.Header.Set("X-Actor-Roles", "member")
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Idempotency-Key", "tra-loi-ai-"+group.ID+"-"+target.id)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		var out struct {
			Code    string `json:"code"`
			ReplyTo *struct {
				Kind    string `json:"kind"`
				Preview string `json:"preview"`
			} `json:"reply_to"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		if w.Code != target.status || (target.code != "" && out.Code != target.code) {
			t.Errorf("%s: %d %s", name, w.Code, w.Body.String())
			continue
		}
		if target.status == 201 {
			want := "Rủ Đi AI: Tối nay ăn lẩu ở Q1 nhé, quán mở tới 23 giờ và vừa túi cả hội; nhớ đặ…"
			if out.ReplyTo == nil || out.ReplyTo.Kind != "ai_card" || out.ReplyTo.Preview != want {
				t.Errorf("%s: trích sai: %s", name, w.Body.String())
			}
		}
	}
	var replies int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM messages WHERE context_id=$1 AND reply_to_id IS NOT NULL`, group.ID).Scan(&replies); err != nil || replies != 1 {
		t.Fatalf("%d tin trả lời được ghi, cần đúng một (vào câu trả lời của AI): %v", replies, err)
	}
}

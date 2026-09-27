//go:build broker

package chatassist

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"

	"mobile/services/core/internal/aiharness"
	"mobile/services/core/internal/aiharness/llm"
	aimetrics "mobile/services/core/internal/aiharness/metrics"
)

// The group on the Go engine streams the way Nếp does (draft, verify,
// stream): its verified text reaches the room key of a legacy-lane job as
// paced Deltas BEFORE the card commits, exactly once -- publish does not
// release it again after the commit -- and the stream ends with xong naming
// the posted message.
func TestStreamNhomEngineGoMotLanTruocXong(t *testing.T) {
	f := setup(t, nil)
	if err := aimetrics.Migrate(context.Background(), f.pool); err != nil {
		t.Fatal(err)
	}
	d := moDongSong(t, f)
	chu := strings.TrimSpace(strings.Repeat("Tối thứ 7 cả nhóm ra bờ hồ đi dạo rồi ghé ăn lẩu cho ấm bụng nhé. ", 3))
	ru, _ := json.Marshal(map[string]any{"nhan_guard": "sach", "tien": "none", "y_dinh": []string{"smalltalk"}, "huong": "tra_loi_thang",
		"slots": map[string]any{}, "can_truy_hoi": []string{}, "truy_van": []any{}, "can_hoi_lai": false, "tra_loi_cau_cho": false, "tu_tin": "cao"})
	md := []any{}
	for i := 1; i <= 3; i++ {
		md = append(md, map[string]any{"so": i, "bang_chung_ids": []string{}, "ket": "khong_thong_tin"})
	}
	kiem, _ := json.Marshal(map[string]any{"menh_de": md, "hua_hanh_dong_khong_co": false, "tien": false})
	stub := llm.NewStub(llm.Buoc{Text: string(ru)}, llm.Buoc{Text: chu}, llm.Buoc{Text: string(kiem)})
	engine, err := aiharness.New(aiharness.WithModel(stub), aiharness.WithLogger(slog.New(slog.DiscardHandler)),
		aiharness.WithRetryWait(func(int) time.Duration { return 0 }), aiharness.WithNhipPhat(time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	f.handler.WithNhomEngine(engine)
	trigger := newID()
	if _, err := f.pool.Exec(context.Background(), `INSERT INTO messages(id,context_id,author_id,kind,body) VALUES($1,$2,$3,'text','@Rủ Đi tối thứ 7 đi đâu')`, trigger, f.context, f.person); err != nil {
		t.Fatal(err)
	}
	w := f.request("POST", f.route(), f.token, map[string]any{"logical_id": newID(), "command": "hoi", "prompt": "@Rủ Đi tối thứ 7 đi đâu", "trigger_message_id": trigger})
	requireCode(t, w, 202)
	var job Invocation
	_ = json.Unmarshal(w.Body.Bytes(), &job)
	if ok, err := f.handler.ProcessOne(context.Background()); !ok || err != nil {
		t.Fatalf("worker: %v %v", ok, err)
	}
	var msg string
	if err := f.pool.QueryRow(context.Background(), `SELECT message_id::text FROM chat_ai_invocations WHERE id=$1 AND status='succeeded'`, job.ID).Scan(&msg); err != nil {
		t.Fatal(err)
	}
	entries := d.loaiCua(t, d.khoaPhong(f.context))
	var noi strings.Builder
	soDelta := 0
	for _, e := range entries {
		loai, j, _ := strings.Cut(e, ":")
		if loai != "delta" {
			continue
		}
		soDelta++
		var dd struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal([]byte(j), &dd); err != nil {
			t.Fatal(err)
		}
		noi.WriteString(dd.Text)
	}
	// The writer may coalesce paced Deltas into fewer entries; what they
	// carry, joined, is the verified text exactly once.
	if soDelta < 1 || noi.String() != chu {
		t.Fatalf("%d deltas carry %q, want the verified text once: %v", soDelta, noi.String(), entries)
	}
	last := entries[len(entries)-1]
	if !strings.HasPrefix(last, "xong:") || !strings.Contains(last, msg) {
		t.Fatalf("the stream does not end with xong{message_id}: %v", entries)
	}
	if n := d.rdb.Exists(context.Background(), d.khoaMoi(job.ID)).Val(); n != 0 {
		t.Fatal("a legacy-lane group job wrote its invocation key")
	}
}

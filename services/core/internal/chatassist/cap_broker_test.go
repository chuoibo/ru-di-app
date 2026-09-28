//go:build broker

package chatassist

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// A pair's `hoi` on the Go engine streams like a group's (a chat of two is
// a room of friends, decision 2026-09-28): the room key carries the
// card's text after the card commits, the requester reads it through
// …/events, chat-capabilities says who can watch; once the pair is blocked,
// the stream route refuses.
func TestStreamCapSauKhiTheDang(t *testing.T) {
	chu := "Tối nay hai bạn ra bờ hồ đi dạo rồi ghé ăn lẩu nhé."
	f, d := nhomGoThu(t, chu)
	if _, err := f.pool.Exec(context.Background(), `UPDATE contexts SET kind='pair',pair_key=$2 WHERE id=$1`, f.context, f.person+":"+f.peer); err != nil {
		t.Fatal(err)
	}
	w := f.request("GET", "/contexts/"+f.context+"/chat-capabilities", f.token, nil)
	if !strings.Contains(w.Body.String(), `"stream":"nguoi_goi"`) || !strings.Contains(w.Body.String(), `"hoi":{"available":true,"reason":null}`) ||
		!strings.Contains(w.Body.String(), `"plan":{"available":true,"reason":null}`) || !strings.Contains(w.Body.String(), `"cap_doi":false`) {
		t.Fatalf("khả năng của cặp: %s", w.Body.String())
	}
	job, _ := f.taoLoiGoi(t)
	if ok, err := f.handler.ProcessOne(context.Background()); !ok || err != nil {
		t.Fatalf("worker: %v %v", ok, err)
	}
	var msg string
	if err := f.pool.QueryRow(context.Background(), `SELECT message_id::text FROM chat_ai_invocations WHERE id=$1 AND status='succeeded'`, job.ID).Scan(&msg); err != nil {
		t.Fatal(err)
	}
	path := f.route() + "/" + job.ID + "/events"
	resp, events, raw := d.nghe(t, path, f.token, "", 10*time.Second)
	if resp.StatusCode != 200 || len(events) == 0 {
		t.Fatalf("luồng của cặp: %d %s", resp.StatusCode, raw)
	}
	var noi strings.Builder
	for _, e := range events {
		if e.loai == "delta" {
			var dd struct {
				Text string `json:"text"`
			}
			_ = json.Unmarshal([]byte(e.data), &dd)
			noi.WriteString(dd.Text)
		}
	}
	last := events[len(events)-1]
	if noi.String() != chu || last.loai != "xong" || !strings.Contains(last.data, msg) {
		t.Fatalf("luồng %q, cuối %+v", noi.String(), last)
	}
	// Blocked: the stream route refuses as every other route of the pair.
	if _, err := f.pool.Exec(context.Background(), `INSERT INTO friend_requests(id,requester_id,addressee_id,state,decided_by_id,decided_at) VALUES($1,$2,$3,'blocked',$2,now())`, newID(), f.peer, f.person); err != nil {
		t.Fatal(err)
	}
	resp, _, raw = d.nghe(t, path, f.token, "", 3*time.Second)
	if resp.StatusCode != 403 || !strings.Contains(raw, "membership_required") {
		t.Fatalf("luồng sau khi chặn: %d %s", resp.StatusCode, raw)
	}
}

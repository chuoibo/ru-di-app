//go:build broker

package chatassist

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"mobile/services/core/internal/aiharness"
	"mobile/services/core/internal/aiharness/llm"
	aimetrics "mobile/services/core/internal/aiharness/metrics"
	"mobile/services/core/internal/chatlegacychange"
	"mobile/services/core/internal/domain/companion"
)

// nhomGoThu is a group job on the Go engine answering chu (stubbed model:
// router, answer, verifier), on a fixture with streaming on.
func nhomGoThu(t *testing.T, chu string) (fixture, dongSong) {
	t.Helper()
	f := setup(t, nil)
	if err := aimetrics.Migrate(context.Background(), f.pool); err != nil {
		t.Fatal(err)
	}
	d := moDongSong(t, f)
	ru, _ := json.Marshal(map[string]any{"nhan_guard": "sach", "tien": "none", "y_dinh": []string{"smalltalk"}, "huong": "tra_loi_thang",
		"slots": map[string]any{}, "can_truy_hoi": []string{}, "truy_van": []any{}, "can_hoi_lai": false, "tra_loi_cau_cho": false, "tu_tin": "cao"})
	md := []any{}
	for i := 1; i <= strings.Count(chu, "."); i++ {
		md = append(md, map[string]any{"so": i, "bang_chung_ids": []string{}, "ket": "khong_thong_tin"})
	}
	kiem, _ := json.Marshal(map[string]any{"menh_de": md, "hua_hanh_dong_khong_co": false, "tien": false})
	stub := llm.NewStub(llm.Buoc{Text: string(ru)}, llm.Buoc{Text: chu}, llm.Buoc{Text: string(kiem)})
	// The engine's own pacing is slow on purpose: were its text streamed
	// during the turn (before the card commits), it would take long enough
	// for a reader to see it before the commit.
	engine, err := aiharness.New(aiharness.WithModel(stub), aiharness.WithLogger(slog.New(slog.DiscardHandler)),
		aiharness.WithRetryWait(func(int) time.Duration { return 0 }), aiharness.WithNhipPhat(20*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	f.handler.WithNhomEngine(engine)
	return f, d
}

// taoLoiGoi posts the `@Rủ Đi` message and the group job answering it.
func (f fixture) taoLoiGoi(t *testing.T) (job Invocation, trigger string) {
	t.Helper()
	trigger = newID()
	if _, err := f.pool.Exec(context.Background(), `INSERT INTO messages(id,context_id,author_id,kind,body) VALUES($1,$2,$3,'text','@Rủ Đi tối thứ 7 đi đâu')`, trigger, f.context, f.person); err != nil {
		t.Fatal(err)
	}
	w := f.request("POST", f.route(), f.token, map[string]any{"logical_id": newID(), "command": "hoi", "prompt": "@Rủ Đi tối thứ 7 đi đâu", "trigger_message_id": trigger})
	requireCode(t, w, 202)
	_ = json.Unmarshal(w.Body.Bytes(), &job)
	return job, trigger
}

// The group on the Go engine streams its answer from the posted card (slice
// 12, contract §4.1): the statuses while the turn runs, then -- after the
// card commits -- the card's text, paced, exactly once, and xong naming the
// posted message. Every room entry names the trigger and the count.
func TestStreamNhomEngineGoSauKhiTheDang(t *testing.T) {
	chu := strings.TrimSpace(strings.Repeat("Tối thứ 7 cả nhóm ra bờ hồ đi dạo rồi ghé ăn lẩu cho ấm bụng nhé. ", 3))
	f, d := nhomGoThu(t, chu)
	job, trigger := f.taoLoiGoi(t)
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
	// carry, joined, is the card's text exactly once.
	if soDelta < 1 || noi.String() != chu {
		t.Fatalf("%d deltas carry %q, want the card's text once: %v", soDelta, noi.String(), entries)
	}
	last := entries[len(entries)-1]
	if !strings.HasPrefix(last, "xong:") || !strings.Contains(last, msg) {
		t.Fatalf("the stream does not end with xong{message_id}: %v", entries)
	}
	if n := d.rdb.Exists(context.Background(), d.khoaMoi(job.ID)).Val(); n != 0 {
		t.Fatal("a legacy-lane group job wrote its invocation key")
	}
	raw, err := d.rdb.XRange(context.Background(), d.khoaPhong(f.context), "-", "+").Result()
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range raw {
		if e.Values["tin"] != trigger || e.Values["inv"] != job.ID {
			t.Fatalf("a room entry without its envelope: %v", e.Values)
		}
	}
	var first *time.Time
	if err := f.pool.QueryRow(context.Background(), `SELECT first_token_at FROM chat_ai_invocations WHERE id=$1`, job.ID).Scan(&first); err != nil || first != nil {
		t.Fatalf("first_token_at %v %v: a group job's text goes after its commit, so no content ever went under its lease", first, err)
	}
}

// caiFeed installs the change feed (internal/chatlegacychange) in the test's
// schema, beside chatassist, as `core migrate-chat` does on a host.
func caiFeed(t *testing.T, f fixture) {
	t.Helper()
	ctx := context.Background()
	for _, table := range []string{"friend_requests", "message_reactions", "votes", "vote_options", "vote_ballots"} {
		if _, err := f.pool.Exec(ctx, fmt.Sprintf("CREATE TABLE %s (LIKE public.%s INCLUDING ALL)", table, table)); err != nil {
			t.Fatal(err)
		}
	}
	if err := chatlegacychange.Migrate(ctx, f.pool); err != nil {
		t.Fatal(err)
	}
}

// Slice 12 end to end: a job the worker runs on the Go engine, the room key it
// writes, the change feed's WebSocket carrying it as `ai` frames to ANOTHER
// member of the room (not the requester), who asked for them. What that
// member sees: the statuses, then the text -- never before the card is in the
// room: when the first delta frame arrives, the job row has succeeded and its
// message exists -- joined equal to the card's text, then xong naming the
// card, and the card itself on the same socket as a feed page.
func TestPhongThanhVienKhacThayChuSauKhiTheDang(t *testing.T) {
	chu := strings.TrimSpace(strings.Repeat("Tối thứ 7 cả nhóm ra bờ hồ đi dạo rồi ghé ăn lẩu cho ấm bụng nhé. ", 3))
	f, d := nhomGoThu(t, chu)
	caiFeed(t, f)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	feed := chatlegacychange.New(chatlegacychange.Store{Pool: f.pool}, ctx, nil).WithAi(d.stream, d.hub)
	feed.ReconcileInterval = 20 * time.Millisecond
	srv := httptest.NewServer(feed)
	defer srv.Close()
	job, trigger := f.taoLoiGoi(t)
	dial, stop := context.WithTimeout(ctx, 30*time.Second)
	defer stop()
	c, _, err := websocket.Dial(dial, "ws"+strings.TrimPrefix(srv.URL, "http")+"/contexts/"+f.context+"/changes/stream", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()
	if err = wsjson.Write(dial, c, map[string]any{"type": "authenticate", "token": f.peerToken, "ai": true}); err != nil {
		t.Fatal(err)
	}
	type khung struct {
		E   string          `json:"e"`
		Inv string          `json:"inv"`
		Tin string          `json:"tin"`
		D   json.RawMessage `json:"d"`
		// state of the job row at the moment the frame was read
		hang string
	}
	frames := make(chan khung, 256)
	pages := make(chan chatlegacychange.Page, 16)
	go func() {
		for {
			_, raw, err := c.Read(ctx)
			if err != nil {
				close(frames)
				return
			}
			var probe struct {
				Type string `json:"type"`
			}
			_ = json.Unmarshal(raw, &probe)
			if probe.Type != "ai" {
				var p chatlegacychange.Page
				_ = json.Unmarshal(raw, &p)
				pages <- p
				_ = wsjson.Write(ctx, c, map[string]any{"type": "ack", "sequence": p.NextSequence})
				continue
			}
			var k khung
			_ = json.Unmarshal(raw, &k)
			if k.E == "delta" {
				var status string
				var there bool
				_ = f.pool.QueryRow(context.Background(), `SELECT status, EXISTS(SELECT 1 FROM messages m WHERE m.id=j.message_id AND m.reply_to_id=j.trigger_message_id) FROM chat_ai_invocations j WHERE id=$1`, job.ID).Scan(&status, &there)
				k.hang = fmt.Sprintf("%s/%v", status, there)
			}
			frames <- k
		}
	}()
	select {
	case <-pages:
	case <-time.After(5 * time.Second):
		t.Fatal("no first page")
	}
	time.Sleep(100 * time.Millisecond) // the pump starts after the first page's ack
	done := make(chan error, 1)
	go func() {
		_, err := f.handler.ProcessOne(context.Background())
		done <- err
	}()
	var seen []string
	var noi strings.Builder
	var xong string
	for xong == "" {
		select {
		case k, ok := <-frames:
			if !ok {
				t.Fatalf("socket closed after %v", seen)
			}
			if k.Inv != job.ID || k.Tin != trigger {
				t.Fatalf("frame envelope %+v", k)
			}
			seen = append(seen, k.E)
			switch k.E {
			case "delta":
				if k.hang != "succeeded/true" {
					t.Fatalf("a delta frame reached another member while the job row was %q: text before the card was posted", k.hang)
				}
				var dd struct {
					Text string `json:"text"`
				}
				_ = json.Unmarshal(k.D, &dd)
				noi.WriteString(dd.Text)
			case "xong":
				var x struct {
					MessageID string `json:"message_id"`
				}
				_ = json.Unmarshal(k.D, &x)
				xong = x.MessageID
			case "trang_thai":
			default:
				t.Fatalf("unexpected frame %+v", k)
			}
		case <-time.After(20 * time.Second):
			t.Fatalf("no ending; frames so far %v", seen)
		}
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	var msg string
	if err := f.pool.QueryRow(context.Background(), `SELECT message_id::text FROM chat_ai_invocations WHERE id=$1`, job.ID).Scan(&msg); err != nil || msg != xong {
		t.Fatalf("xong names %s, the row %s %v", xong, msg, err)
	}
	if noi.String() != chu || seen[0] != "trang_thai" {
		t.Fatalf("frames %v carry %q, want the card's text", seen, noi.String())
	}
	// The card itself reaches the member as a feed page on the same socket.
	deadline := time.After(5 * time.Second)
	for {
		select {
		case p := <-pages:
			for _, ch := range p.Changes {
				if ch.EntityID == msg {
					t.Logf("frames %v; card %s arrived as a feed page", seen, msg)
					return
				}
			}
		case <-deadline:
			t.Fatal("the card never arrived as a feed page")
		}
	}
}

// Review of a6d631d, slice 9 minor 4: for a client that names no trigger,
// the Go engine's card is the one text card of GroundCard, cut to
// companion.MaxText (600 runes), while the engine's verified answer may run
// to 1500. The stream now carries the posted card's own text, after its
// commit: what readers see never exceeds what the card shows.
func TestStreamNhomKhongTriggerKhongVuotThe(t *testing.T) {
	chu := strings.TrimSpace(strings.Repeat("Tối thứ 7 cả nhóm ra bờ hồ đi dạo rồi ghé ăn lẩu cho ấm bụng nhé. ", 12))
	if n := utf8.RuneCountInString(chu); n <= companion.MaxText || n > aiharness.NhomMaxChu {
		t.Fatalf("fixture answer is %d runes; it must sit between the two ceilings", n)
	}
	f, d := nhomGoThu(t, chu)
	w := f.request("POST", f.route(), f.token, map[string]any{"logical_id": newID(), "command": "plan", "prompt": "Tối thứ 7 đi đâu"})
	requireCode(t, w, 202)
	var job Invocation
	_ = json.Unmarshal(w.Body.Bytes(), &job)
	if ok, err := f.handler.ProcessOne(context.Background()); !ok || err != nil {
		t.Fatalf("worker: %v %v", ok, err)
	}
	var card []byte
	if err := f.pool.QueryRow(context.Background(), `SELECT m.card::text FROM chat_ai_invocations j JOIN messages m ON m.id=j.message_id WHERE j.id=$1 AND j.status='succeeded'`, job.ID).Scan(&card); err != nil {
		t.Fatal(err)
	}
	parts := chuCuaThe(card)
	if len(parts) != 1 {
		t.Fatalf("card %s", card)
	}
	var noi strings.Builder
	for _, e := range d.loaiCua(t, d.khoaPhong(f.context)) {
		loai, j, _ := strings.Cut(e, ":")
		if loai != "delta" {
			continue
		}
		var dd struct {
			Text string `json:"text"`
		}
		_ = json.Unmarshal([]byte(j), &dd)
		noi.WriteString(dd.Text)
	}
	if noi.String() != parts[0].text || utf8.RuneCountInString(noi.String()) > companion.MaxText {
		t.Fatalf("streamed %d runes, card shows %d (%d max): the stream must carry exactly the card's text",
			utf8.RuneCountInString(noi.String()), utf8.RuneCountInString(parts[0].text), companion.MaxText)
	}
}

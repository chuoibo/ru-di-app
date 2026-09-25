//go:build postgres

package chatassist

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// The group AI answering inside the thread (ADR-0039, proposed), against a
// real database: the trigger check, one answer per message, the reply that
// quotes the mention, the deletion trigger and the room limits.

// tinTag inserts a text message of this room written by author, the way the
// ordinary send queue would have stored the person's `@Rủ Đi …` words.
func (f fixture) tinTag(t *testing.T, room, author string) string {
	t.Helper()
	id := newID()
	if _, err := f.pool.Exec(context.Background(), `INSERT INTO messages(id,context_id,author_id,kind,body) VALUES($1,$2,$3,'text','@Rủ Đi tối nay ăn gì gần Q1?')`, id, room, author); err != nil {
		t.Fatal(err)
	}
	return id
}

func (f fixture) goiTag(token, logical, trigger string, extra map[string]any) *httptest.ResponseRecorder {
	body := map[string]any{"logical_id": logical, "command": "plan", "prompt": "tối nay ăn gì gần Q1?", "trigger_message_id": trigger}
	for k, v := range extra {
		body[k] = v
	}
	return f.request("POST", f.route(), token, body)
}

func maTuChoi(w *httptest.ResponseRecorder) string {
	var body struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	return body.Code
}

func TestTinTagPhaiLaTinChuCuaChinhNguoiGoiTrongPhongNay(t *testing.T) {
	f := setup(t, nil)
	ctx := context.Background()
	phongKhac := newID()
	if _, err := f.pool.Exec(ctx, `INSERT INTO contexts(id,display_name,kind,created_by_id) VALUES($1,'Synthetic other room','group',$2)`, phongKhac, f.person); err != nil {
		t.Fatal(err)
	}
	daXoa, cu, anh := newID(), newID(), newID()
	if _, err := f.pool.Exec(ctx, `INSERT INTO messages(id,context_id,author_id,kind,deleted_at) VALUES($1,$2,$3,'deleted',clock_timestamp())`, daXoa, f.context, f.person); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `INSERT INTO messages(id,context_id,author_id,kind,body,created_at) VALUES($1,$2,$3,'text','@Rủ Đi hôm kia',clock_timestamp()-interval '25 hours')`, cu, f.context, f.person); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `INSERT INTO messages(id,context_id,author_id,kind,image_url) VALUES($1,$2,$3,'image','/contexts/x/photos/y')`, anh, f.context, f.person); err != nil {
		t.Fatal(err)
	}
	for name, trigger := range map[string]string{
		"phòng khác":      f.tinTag(t, phongKhac, f.person),
		"người khác viết": f.tinTag(t, f.context, f.peer),
		"đã xoá":          daXoa,
		"quá 24 giờ":      cu,
		"không phải chữ":  anh,
		"không tồn tại":   newID(),
	} {
		w := f.goiTag(f.token, newID(), trigger, nil)
		if w.Code != 422 || maTuChoi(w) != "trigger_khong_hop_le" {
			t.Errorf("%s: %d %s", name, w.Code, w.Body.String())
		}
	}
	if n := f.demLoiGoi(t); n != 0 {
		t.Fatalf("%d lời gọi được ghi dù tin tag bị từ chối", n)
	}
	// Identity: the person's own fresh text message in this room.
	own := f.tinTag(t, f.context, f.person)
	w := f.goiTag(f.token, newID(), own, map[string]any{"boi_canh": goiThu(luotThu(f.tinTrongPhong(t, f.context, "Q1 nhé"), "Q1 nhé"))})
	requireCode(t, w, 202)
	var v Invocation
	_ = json.Unmarshal(w.Body.Bytes(), &v)
	if v.TriggerMessageID == nil || *v.TriggerMessageID != own || v.SoTinDoc == nil || *v.SoTinDoc != 1 {
		t.Fatalf("lời gọi không mang tin tag hoặc số tin: %s", w.Body.String())
	}
	var lane string
	if err := f.pool.QueryRow(ctx, `SELECT lane FROM chat_ai_invocations WHERE id=$1`, v.ID).Scan(&lane); err != nil || lane != "legacy" {
		t.Fatalf("lane=%q %v", lane, err)
	}
	// The lane is the server's finding: a client cannot send one.
	if w := f.goiTag(f.token, newID(), own, map[string]any{"lane": "v2"}); w.Code != 400 || maTuChoi(w) != "invalid_body" {
		t.Fatalf("lane từ client: %d %s", w.Code, w.Body.String())
	}
}

// Two presses, two logical ids, one message: one answer.
func TestHaiLoiGoiDuaTrenMotTinTag(t *testing.T) {
	f := setup(t, nil)
	trigger := f.tinTag(t, f.context, f.person)
	var wg sync.WaitGroup
	out := make(chan *httptest.ResponseRecorder, 6)
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); out <- f.goiTag(f.token, newID(), trigger, nil) }()
	}
	wg.Wait()
	close(out)
	created, taken := 0, 0
	for w := range out {
		switch {
		case w.Code == 202:
			created++
		case w.Code == 409 && maTuChoi(w) == "invocation_trigger_taken":
			taken++
		default:
			t.Errorf("đua: %d %s", w.Code, w.Body.String())
		}
	}
	if created != 1 || taken != 5 {
		t.Fatalf("202=%d 409=%d, cần đúng một câu trả lời cho một tin tag", created, taken)
	}
	// The same logical id replays; the same id for ANOTHER message is a
	// conflict, never the first message's answer handed back.
	logical := newID()
	other := f.tinTag(t, f.context, f.person)
	requireCode(t, f.goiTag(f.token, logical, other, nil), 202)
	requireCode(t, f.goiTag(f.token, logical, other, nil), 200)
	third := f.tinTag(t, f.context, f.person)
	if w := f.goiTag(f.token, logical, third, nil); w.Code != 409 || maTuChoi(w) != "invocation_conflict" {
		t.Fatalf("cùng logical_id, tin tag khác: %d %s", w.Code, w.Body.String())
	}
}

type theTraLoi struct {
	Kind    string `json:"kind"`
	Payload struct {
		Ban          int    `json:"ban"`
		TacGia       string `json:"tac_gia"`
		InvocationID string `json:"invocation_id"`
		Lenh         string `json:"lenh"`
		Doc          struct {
			SoTin     int  `json:"so_tin"`
			ChiLoiNho bool `json:"chi_loi_nho"`
		} `json:"doc"`
		Phan []json.RawMessage `json:"phan"`
	} `json:"payload"`
}

// The answer is a reply to the mention, signed in the card, with today's card
// as its one part; a job with no trigger publishes today's card unchanged.
func TestTraLoiTrichDungTinTag(t *testing.T) {
	f := setup(t, nil)
	ctx := context.Background()
	trigger := f.tinTag(t, f.context, f.person)
	a, b := f.tinTrongPhong(t, f.context, "Ăn lẩu đi"), f.tinTrongPhong(t, f.context, "Q1 nha")
	w := f.goiTag(f.token, newID(), trigger, map[string]any{"boi_canh": goiThu(luotThu(a, "Ăn lẩu đi"), luotThu(b, "Q1 nha"), luotThu(b, "Q1 nha"))})
	requireCode(t, w, 202)
	var job Invocation
	_ = json.Unmarshal(w.Body.Bytes(), &job)
	if ok, err := f.handler.ProcessOne(ctx); !ok || err != nil {
		t.Fatalf("worker: %v %v", ok, err)
	}
	var reply *string
	var author *string
	var card []byte
	if err := f.pool.QueryRow(ctx, `SELECT m.reply_to_id::text,m.author_id::text,m.card FROM messages m JOIN chat_ai_invocations j ON j.message_id=m.id WHERE j.id=$1 AND j.status='succeeded'`, job.ID).Scan(&reply, &author, &card); err != nil {
		t.Fatal(err)
	}
	if reply == nil || *reply != trigger {
		t.Fatalf("câu trả lời không trả lời vào tin tag: reply_to=%v", reply)
	}
	if author != nil {
		t.Fatal("tin AI mang tác giả; danh tính phải nằm trong thẻ")
	}
	var the theTraLoi
	if err := json.Unmarshal(card, &the); err != nil {
		t.Fatal(err)
	}
	// Two distinct messages confirmed, though three turns were sent.
	if the.Kind != "tra_loi" || the.Payload.Ban != 1 || the.Payload.TacGia != "rudi-ai" || the.Payload.InvocationID != job.ID || the.Payload.Lenh != "plan" || the.Payload.Doc.SoTin != 2 || the.Payload.Doc.ChiLoiNho || len(the.Payload.Phan) != 1 {
		t.Fatalf("thẻ trả lời sai hình: %s", card)
	}
	if string(the.Payload.Phan[0]) != `{"kind": "text", "payload": {"text": "Synthetic inference fixture"}}` {
		t.Fatalf("phần của thẻ không phải thẻ hôm nay: %s", the.Payload.Phan[0])
	}
	// An app from before this change names no trigger, and gets the card it
	// can draw.
	old := f.create(t)
	if ok, err := f.handler.ProcessOne(ctx); !ok || err != nil {
		t.Fatalf("worker: %v %v", ok, err)
	}
	if err := f.pool.QueryRow(ctx, `SELECT m.reply_to_id::text,m.card FROM messages m JOIN chat_ai_invocations j ON j.message_id=m.id WHERE j.id=$1`, old.ID).Scan(&reply, &card); err != nil {
		t.Fatal(err)
	}
	if reply != nil || string(card) != `{"kind": "text", "payload": {"text": "Synthetic inference fixture"}}` {
		t.Fatalf("lời gọi không tin tag phải ra đúng thẻ cũ, không trả lời: %v %s", reply, card)
	}
}

// Only the request: the answer says so rather than claiming a count.
func TestTraLoiChiLoiNhoNoiRo(t *testing.T) {
	f := setup(t, nil)
	ctx := context.Background()
	trigger := f.tinTag(t, f.context, f.person)
	requireCode(t, f.goiTag(f.token, newID(), trigger, nil), 202)
	if ok, err := f.handler.ProcessOne(ctx); !ok || err != nil {
		t.Fatalf("worker: %v %v", ok, err)
	}
	var card []byte
	if err := f.pool.QueryRow(ctx, `SELECT card FROM messages WHERE reply_to_id=$1`, trigger).Scan(&card); err != nil {
		t.Fatal(err)
	}
	var the theTraLoi
	if err := json.Unmarshal(card, &the); err != nil || the.Payload.Doc.SoTin != 0 || !the.Payload.Doc.ChiLoiNho {
		t.Fatalf("chỉ lời nhờ mà thẻ không nói vậy: %s", card)
	}
}

// Taking back the `@Rủ Đi` message takes back the question.
func TestXoaTinTagHuyViecVaXoaChu(t *testing.T) {
	var calls int
	var mu sync.Mutex
	f := setup(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		mu.Unlock()
		reply(w, 200, map[string]any{"kind": "text", "payload": map[string]string{"text": "Synthetic inference fixture"}})
	})
	ctx := context.Background()
	trigger := f.tinTag(t, f.context, f.person)
	ban := f.tinTrongPhong(t, f.context, "Q1 nha")
	w := f.goiTag(f.token, newID(), trigger, map[string]any{"boi_canh": goiThu(luotThu(ban, "Q1 nha"))})
	requireCode(t, w, 202)
	var job Invocation
	_ = json.Unmarshal(w.Body.Bytes(), &job)
	// A failed job on another trigger is cancelled too; an answered one is left.
	failedTrigger, doneTrigger := f.tinTag(t, f.context, f.person), f.tinTag(t, f.context, f.person)
	var failed, done Invocation
	_ = json.Unmarshal(f.goiTag(f.token, newID(), failedTrigger, nil).Body.Bytes(), &failed)
	_ = json.Unmarshal(f.goiTag(f.token, newID(), doneTrigger, nil).Body.Bytes(), &done)
	if _, err := f.pool.Exec(ctx, `UPDATE chat_ai_invocations SET status='failed',code='provider_unavailable' WHERE id=$1`, failed.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `UPDATE chat_ai_invocations SET status='succeeded',prompt=NULL WHERE id=$1`, done.ID); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{trigger, failedTrigger, doneTrigger} {
		if _, err := f.pool.Exec(ctx, `UPDATE messages SET kind='deleted',body=NULL,deleted_at=clock_timestamp() WHERE id=$1`, id); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range []struct {
		id, status, code string
		scrubbed         bool
	}{{job.ID, "cancelled", "trigger_deleted", true}, {failed.ID, "cancelled", "trigger_deleted", true}, {done.ID, "succeeded", "", true}} {
		var status string
		var code *string
		var promptNull, goiNull bool
		if err := f.pool.QueryRow(ctx, `SELECT status,code,prompt IS NULL,boi_canh IS NULL FROM chat_ai_invocations WHERE id=$1`, c.id).Scan(&status, &code, &promptNull, &goiNull); err != nil {
			t.Fatal(err)
		}
		got := ""
		if code != nil {
			got = *code
		}
		if status != c.status || got != c.code || !promptNull || !goiNull {
			t.Errorf("job %s: status=%s code=%s prompt_null=%v boi_canh_null=%v", c.id, status, got, promptNull, goiNull)
		}
	}
	if ok, err := f.handler.ProcessOne(ctx); ok || err != nil {
		t.Fatalf("việc đã huỷ vẫn được nhận: %v %v", ok, err)
	}
	var published int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM messages WHERE kind='ai_card'`).Scan(&published); err != nil || published != 0 || calls != 0 {
		t.Fatalf("tin tag đã xoá mà vẫn có câu trả lời: %d tin, %d lời gọi mô hình", published, calls)
	}
	// And a deleted message cannot be asked about again.
	if w := f.goiTag(f.token, newID(), trigger, nil); w.Code != 422 || maTuChoi(w) != "trigger_khong_hop_le" {
		t.Fatalf("tin đã xoá lại thành tin tag: %d %s", w.Code, w.Body.String())
	}
}

func TestHanPhongDangChayVaMoiGio(t *testing.T) {
	f := setup(t, nil)
	ctx := context.Background()
	// Three in flight, from two people: the room is full for everybody.
	var jobs []Invocation
	for i, token := range []string{f.token, f.peerToken, f.token} {
		author := f.person
		if i == 1 {
			author = f.peer
		}
		w := f.goiTag(token, newID(), f.tinTag(t, f.context, author), nil)
		requireCode(t, w, 202)
		var v Invocation
		_ = json.Unmarshal(w.Body.Bytes(), &v)
		jobs = append(jobs, v)
	}
	if w := f.goiTag(f.peerToken, newID(), f.tinTag(t, f.context, f.peer), nil); w.Code != 429 || maTuChoi(w) != "invocation_room_busy" {
		t.Fatalf("phòng đủ ba việc: %d %s", w.Code, w.Body.String())
	}
	// A retry puts a job back in flight, so it waits for room too.
	if _, err := f.pool.Exec(ctx, `UPDATE chat_ai_invocations SET status='failed',code='provider_unavailable' WHERE id=$1`, jobs[0].ID); err != nil {
		t.Fatal(err)
	}
	requireCode(t, f.goiTag(f.token, newID(), f.tinTag(t, f.context, f.person), nil), 202)
	if w := f.request("POST", f.route()+"/"+jobs[0].ID+"/retry", f.token, map[string]any{}); w.Code != 429 || maTuChoi(w) != "invocation_room_busy" {
		t.Fatalf("thử lại khi phòng đầy: %d %s", w.Code, w.Body.String())
	}
	// Thirty an hour, whatever became of them.
	if _, err := f.pool.Exec(ctx, `UPDATE chat_ai_invocations SET status='succeeded',prompt=NULL,boi_canh=NULL WHERE context_id=$1`, f.context); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `INSERT INTO chat_ai_invocations(id,scope,context_id,person_id,membership_id,session_digest,logical_id,input_digest,command,share_expires_at,status,created_at)
		SELECT gen_random_uuid(),'group',$1,$2,$3,decode(repeat('00',32),'hex'),gen_random_uuid(),decode(repeat('00',32),'hex'),'plan',clock_timestamp(),'succeeded',clock_timestamp()-interval '30 minutes'
		  FROM generate_series(1,$4)`, f.context, f.person, f.member, maxMoiPhongMoiGio-4); err != nil {
		t.Fatal(err)
	}
	if w := f.goiTag(f.peerToken, newID(), f.tinTag(t, f.context, f.peer), nil); w.Code != 429 || maTuChoi(w) != "invocation_room_rate_limited" {
		t.Fatalf("phòng đủ ba mươi lời gọi trong giờ: %d %s", w.Code, w.Body.String())
	}
	// An hour later the room is open again.
	if _, err := f.pool.Exec(ctx, `UPDATE chat_ai_invocations SET created_at=created_at-interval '1 hour' WHERE context_id=$1`, f.context); err != nil {
		t.Fatal(err)
	}
	requireCode(t, f.goiTag(f.peerToken, newID(), f.tinTag(t, f.context, f.peer), nil), 202)
}

// A job that failed does not hold its message: asking again is how a person
// recovers. Then the old one cannot come back and publish a second answer.
func TestThuLaiSauKhiTinTagDaCoLoiGoiKhac(t *testing.T) {
	f := setup(t, nil)
	ctx := context.Background()
	trigger := f.tinTag(t, f.context, f.person)
	var first Invocation
	_ = json.Unmarshal(f.goiTag(f.token, newID(), trigger, nil).Body.Bytes(), &first)
	if _, err := f.pool.Exec(ctx, `UPDATE chat_ai_invocations SET status='failed',code='provider_unavailable' WHERE id=$1`, first.ID); err != nil {
		t.Fatal(err)
	}
	requireCode(t, f.goiTag(f.token, newID(), trigger, nil), 202)
	if w := f.request("POST", f.route()+"/"+first.ID+"/retry", f.token, map[string]any{}); w.Code != 409 || maTuChoi(w) != "invocation_trigger_taken" {
		t.Fatalf("thử lại việc cũ khi tin tag đã có lời gọi mới: %d %s", w.Code, w.Body.String())
	}
}

// chia_bill in the thread: the summary is the reply's one part, and the
// drafts stay on the invocation row, never in the message the room reads.
func TestChiaBillTrongLuongGiuNhapOCotKetQua(t *testing.T) {
	f := setup(t, func(w http.ResponseWriter, r *http.Request) {
		reply(w, 200, map[string]any{"is_expense": true, "title": "Tiền nước", "amount_vnd": 300000, "needs_review": true})
	})
	ctx := context.Background()
	trigger := f.tinTag(t, f.context, f.person)
	w := f.request("POST", f.route(), f.token, map[string]any{"logical_id": newID(), "command": "chia_bill", "prompt": "mình trả 300k tiền nước", "trigger_message_id": trigger})
	requireCode(t, w, 202)
	var job Invocation
	_ = json.Unmarshal(w.Body.Bytes(), &job)
	if ok, err := f.handler.ProcessOne(ctx); !ok || err != nil {
		t.Fatalf("worker: %v %v", ok, err)
	}
	var reply *string
	var card, result []byte
	if err := f.pool.QueryRow(ctx, `SELECT m.reply_to_id::text,m.card,j.result FROM messages m JOIN chat_ai_invocations j ON j.message_id=m.id WHERE j.id=$1`, job.ID).Scan(&reply, &card, &result); err != nil {
		t.Fatal(err)
	}
	var the theTraLoi
	if err := json.Unmarshal(card, &the); err != nil || reply == nil || *reply != trigger || the.Kind != "tra_loi" || the.Payload.Lenh != "chia_bill" || len(the.Payload.Phan) != 1 || !strings.HasPrefix(string(the.Payload.Phan[0]), `{"kind": "text"`) {
		t.Fatalf("chia_bill trong luồng: reply=%v card=%s", reply, card)
	}
	if !strings.Contains(string(result), `"drafts"`) || strings.Contains(string(card), "paid_by_id") || strings.Contains(string(card), f.person) {
		t.Fatalf("nháp phải ở result, không ở thẻ: result=%s card=%s", result, card)
	}
}

func TestKhaNangBaoCoMention(t *testing.T) {
	f := setup(t, nil)
	w := f.request("GET", "/contexts/"+f.context+"/chat-capabilities", f.token, nil)
	requireCode(t, w, 200)
	if !strings.Contains(w.Body.String(), `"mention":true`) {
		t.Fatalf("chat-capabilities không nói máy chủ nhận tin tag: %s", w.Body.String())
	}
}

// A room on chat v2 is refused before anything is written, so no row there can
// ever read `legacy`.
func TestPhongV2KhongGhiHangNao(t *testing.T) {
	f := setup(t, nil)
	ctx := context.Background()
	if _, err := f.pool.Exec(ctx, `CREATE TABLE chat_v2_conversations(context_id uuid PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `INSERT INTO chat_v2_conversations VALUES($1)`, f.context); err != nil {
		t.Fatal(err)
	}
	if w := f.goiTag(f.token, newID(), f.tinTag(t, f.context, f.person), nil); w.Code != 409 || maTuChoi(w) != "encrypted_invocation_required" {
		t.Fatalf("phòng v2: %d %s", w.Code, w.Body.String())
	}
	if n := f.demLoiGoi(t); n != 0 {
		t.Fatalf("phòng v2 có %d hàng", n)
	}
}

func TestLuocDoPhienBan4(t *testing.T) {
	f := setup(t, nil)
	ctx := context.Background()
	if ok, err := SchemaCurrent(ctx, f.pool); err != nil || !ok {
		t.Fatalf("sau Migrate, SchemaCurrent=%v %v", ok, err)
	}
	if _, err := f.pool.Exec(ctx, `DELETE FROM chat_ai_schema_migrations WHERE version=4`); err != nil {
		t.Fatal(err)
	}
	if ok, err := SchemaCurrent(ctx, f.pool); err != nil || ok {
		t.Fatalf("thiếu phiên bản 4 mà vẫn coi là đủ: %v %v", ok, err)
	}
}

// A plan answered in the thread is still a tờ hẹn the group can turn into a
// kèo: its itinerary part is the source, and the kèo is stamped on the reply.
func TestNangToHenTuTraLoiTrongLuong(t *testing.T) {
	f := setup(t, nil)
	ctx := context.Background()
	withPlan, textOnly := newID(), newID()
	if _, err := f.pool.Exec(ctx, `INSERT INTO messages(id,context_id,kind,card) VALUES($1,$2,'ai_card',$3),($4,$2,'ai_card',$5)`, withPlan, f.context,
		`{"kind":"tra_loi","payload":{"ban":1,"tac_gia":"rudi-ai","invocation_id":"x","lenh":"plan","doc":{"so_tin":0,"chi_loi_nho":true},"phan":[{"kind":"itinerary","payload":{"title":"Synthetic plan","stops":[{"time_text":"18:30","note":"","place":{"id":"synthetic","name":"Synthetic place"}}]}}]}}`,
		textOnly, `{"kind":"tra_loi","payload":{"ban":1,"tac_gia":"rudi-ai","invocation_id":"y","lenh":"plan","doc":{"so_tin":0,"chi_loi_nho":true},"phan":[{"kind":"text","payload":{"text":"Chỉ chữ"}}]}}`); err != nil {
		t.Fatal(err)
	}
	input := promotionInput{Source: withPlan, Title: "Synthetic confirmed plan", Starts: "2026-10-01", Ends: "2026-10-01", Headcount: 2, Budget: 100000, Stops: []planStop{{At: "18:30", Label: "Synthetic human-reviewed stop"}}}
	route := "/contexts/" + f.context + "/plan-promotions"
	w := f.request("POST", route, f.token, input)
	requireCode(t, w, 201)
	var out promotionResult
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	var stamped string
	if err := f.pool.QueryRow(ctx, `SELECT card->'payload'->>'outing_id' FROM messages WHERE id=$1`, withPlan).Scan(&stamped); err != nil || stamped != out.OutingID {
		t.Fatalf("kèo không được đóng dấu lên câu trả lời: %q %v", stamped, err)
	}
	input.Source = textOnly
	if w := f.request("POST", route, f.token, input); w.Code != 422 || maTuChoi(w) != "plan_source_invalid" {
		t.Fatalf("câu trả lời không có lịch trình: %d %s", w.Code, w.Body.String())
	}
}

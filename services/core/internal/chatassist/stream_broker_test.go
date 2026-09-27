//go:build broker

package chatassist

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"mobile/services/core/internal/aiharness"
	"mobile/services/core/internal/aiharness/cau"
	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aistream"
	"mobile/services/core/internal/auth"
	"mobile/services/core/internal/httpapi/mw/cors"
)

// The two SSE routes end to end (slice 11; design 02 §5, §9) on a real Redis,
// RabbitMQ and PostgreSQL, through CORS and a real HTTP client: create -> the
// queue -> the worker -> the engine on a scripted stub (0 real model calls,
// ADR-0034 §2.6) -> the output guard window -> the job's stream -> the reader.
// This file is the manifest's evidence for both /events routes.

func redisURL(t *testing.T) string {
	t.Helper()
	url := os.Getenv("CORE_TEST_REDIS_URL")
	if url == "" {
		if os.Getenv("CORE_REQUIRE_BROKER_TESTS") == "1" {
			t.Fatal("CORE_TEST_REDIS_URL is required")
		}
		t.Skip("CORE_TEST_REDIS_URL not set")
	}
	return url
}

// dongSong is the fixture with streaming on: a Redis namespace of the test's
// own (emptied afterwards), this process's hub fed by Listen, and the routes
// behind CORS on a real HTTP server.
type dongSong struct {
	fixture
	stream *aistream.Stream
	rdb    *redis.Client
	ns     string
	srv    *httptest.Server
	stop   chan struct{}
}

func moDongSong(t *testing.T, f fixture) dongSong {
	t.Helper()
	url := redisURL(t)
	ns := fmt.Sprintf("s%d", time.Now().UnixNano()%1_000_000_000_000)
	s, err := aistream.Open(url, ns)
	if err != nil {
		t.Fatal(err)
	}
	opt, _ := redis.ParseURL(url)
	rdb := redis.NewClient(opt)
	hub := aistream.NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	ready := make(chan struct{})
	go s.Listen(ctx, hub, ready)
	select {
	case <-ready:
	case <-time.After(5 * time.Second):
		t.Fatal("the wake subscription never came up")
	}
	stop := make(chan struct{})
	f.handler.WithStream(s, hub, stop)
	srv := httptest.NewServer(cors.New("", false).Middleware(f.handler))
	d := dongSong{fixture: f, stream: s, rdb: rdb, ns: ns, srv: srv, stop: stop}
	t.Cleanup(func() {
		srv.Close()
		cancel()
		for _, k := range d.khoa(t) {
			rdb.Del(context.Background(), k)
		}
		_ = rdb.Close()
		_ = s.Close()
	})
	return d
}

// khoa lists every key of the test's namespace.
func (d dongSong) khoa(t *testing.T) []string {
	t.Helper()
	var out []string
	iter := d.rdb.Scan(context.Background(), 0, "rudi:"+d.ns+":*", 1000).Iterator()
	for iter.Next(context.Background()) {
		out = append(out, iter.Val())
	}
	return out
}

func (d dongSong) khoaMoi(id string) string   { return "rudi:" + d.ns + ":ai:inv:" + id }
func (d dongSong) khoaPhong(id string) string { return "rudi:" + d.ns + ":ai:room:" + id }

// suKienSSE is one event as a client read it off the wire.
type suKienSSE struct {
	id, loai, data string
	luc            time.Time
}

// nghe opens a stream and reads it until the server closes it or han ends.
func (d dongSong) nghe(t *testing.T, path, token, lastID string, han time.Duration) (*http.Response, []suKienSSE, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), han)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", d.srv.URL+path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	if lastID != "" {
		req.Header.Set("Last-Event-ID", lastID)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var raw strings.Builder
	var out []suKienSSE
	var cur suKienSSE
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		line := sc.Text()
		raw.WriteString(line + "\n")
		switch {
		case line == "":
			if cur.loai != "" {
				cur.luc = time.Now()
				out = append(out, cur)
			}
			cur = suKienSSE{}
		case strings.HasPrefix(line, "id: "):
			cur.id = strings.TrimPrefix(line, "id: ")
		case strings.HasPrefix(line, "event: "):
			cur.loai = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			cur.data = strings.TrimPrefix(line, "data: ")
		}
	}
	return resp, out, raw.String()
}

// nguoiMoi adds a person with a live session and returns the session's token:
// a run of more questions than one person's rate limit (8 a minute) asks as
// several people.
func (d dongSong) nguoiMoi(t *testing.T) string {
	t.Helper()
	person, token := newID(), "synthetic-"+newID()
	if _, err := d.pool.Exec(context.Background(), `INSERT INTO people(id,display_name) VALUES($1,'Synthetic asker')`, person); err != nil {
		t.Fatal(err)
	}
	if _, err := d.pool.Exec(context.Background(), `INSERT INTO account_sessions(id,person_id,token_digest,issued_via,expires_at) VALUES($1,$2,$3,'genesis',clock_timestamp()+interval '1 hour')`, newID(), person, auth.TokenDigest(token)); err != nil {
		t.Fatal(err)
	}
	return token
}

// hoiNep asks Nếp through the HTTP server and returns the id and when the 202
// arrived.
func (d dongSong) hoiNep(t *testing.T, prompt string) (string, time.Time) {
	t.Helper()
	return d.hoiNepVoi(t, d.token, prompt)
}

func (d dongSong) hoiNepVoi(t *testing.T, token, prompt string) (string, time.Time) {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"logical_id": newID(), "prompt": prompt, "phieu": nil, "luot": []any{}})
	req, _ := http.NewRequest("POST", d.srv.URL+"/me/nep/ai-invocations", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	at := time.Now()
	var v NepInvocation
	_ = json.NewDecoder(resp.Body).Decode(&v)
	if resp.StatusCode != 202 {
		t.Fatalf("POST Nếp: %d", resp.StatusCode)
	}
	return v.ID, at
}

// traLoiDai is a clean two-sentence answer, longer than the output guard's
// window: the engine releases it, once the verifier has passed it, in
// several Deltas that end at white space.
const traLoiDai = "Tối nay bạn thử ra bờ hồ đi dạo một vòng rồi ghé quán chè ấm bụng cho vui nhé, trời đang mát. Nếu đông người thì đi sớm một chút cho dễ tìm chỗ ngồi, rồi về nghỉ cho khỏe."

// kiemDatHai: the verifier passes both sentences of traLoiDai.
const kiemDatHai = `{"menh_de":[{"so":1,"bang_chung_ids":[],"ket":"khong_thong_tin"},{"so":2,"bang_chung_ids":[],"ket":"khong_thong_tin"}],"hua_hanh_dong_khong_co":false,"tien":false}`

// nepDai is n Nếp turns that each answer traLoiDai, in any order; cho delays
// each router reply (a slow turn, still no text before the verifier).
func nepDai(n int, cho time.Duration) theoChang {
	var h, a, k []llm.Buoc
	for range n {
		h = append(h, llm.Buoc{Text: ruTraLoiThang, Cho: cho})
		a = append(a, llm.Buoc{Text: traLoiDai})
		k = append(k, llm.Buoc{Text: kiemDatHai})
	}
	return theoChang{hieu: llm.NewStub(h...), tra: llm.NewStub(a...), kiem: llm.NewStub(k...)}
}

// msTuID is the Redis server time an entry was appended at, from its id.
func msTuID(id string) time.Time {
	ms, _ := strconv.ParseInt(strings.SplitN(id, "-", 2)[0], 10, 64)
	return time.UnixMilli(ms)
}

func phanViSSE(ds []time.Duration, q float64) time.Duration {
	s := append([]time.Duration(nil), ds...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	return s[int(float64(len(s)-1)*q)]
}

// Nếp end to end, twenty times: the headers and the opening of design 02
// §5.2, a first status within 300 ms of the 202 (p95), every delta in order
// through a real client, the deltas joined equal to xong's text and to the
// sealed answer, first_token_at set, no room key, and the finished key
// living at most 120 s. The latencies are the numbers the commit reports.
func TestStreamNepDauCuoiQuaSSE(t *testing.T) {
	url := amqpURL(t)
	f := setup(t, nil)
	const lan = 20
	f.nepTrenEngine(t, nepDai(lan, 0))
	d := moDongSong(t, f)
	moTram(t, f, f.handler, url, topology(t, url), false).choSong(t)
	want := traLoiDai
	var dau, nha []time.Duration
	var token string
	for i := 0; i < lan; i++ {
		if i%8 == 0 {
			token = d.nguoiMoi(t)
		}
		id, luc202 := d.hoiNepVoi(t, token, fmt.Sprintf("tối nay đi đâu, lần %d?", i))
		resp, events, raw := d.nghe(t, "/me/nep/ai-invocations/"+id+"/events", token, "", 10*time.Second)
		if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "text/event-stream; charset=utf-8" || resp.Header.Get("X-Accel-Buffering") != "no" {
			t.Fatalf("headers %d %v", resp.StatusCode, resp.Header)
		}
		if !strings.HasPrefix(raw, "retry: 2000\n\nevent: hello\n") {
			t.Fatalf("body opens %q", raw[:min(len(raw), 80)])
		}
		if len(events) < 3 || events[0].loai != "hello" || events[1].loai != "trang_thai" {
			t.Fatalf("run %d: events %+v", i, events)
		}
		dau = append(dau, events[1].luc.Sub(luc202))
		var noi strings.Builder
		var xong map[string]any
		for k, e := range events {
			switch e.loai {
			case "delta":
				var dd aistream.DeltaData
				_ = json.Unmarshal([]byte(e.data), &dd)
				noi.WriteString(dd.Text)
				nha = append(nha, e.luc.Sub(msTuID(e.id)))
			case "xong":
				_ = json.Unmarshal([]byte(e.data), &xong)
				if k != len(events)-1 {
					t.Fatalf("events after xong: %+v", events[k:])
				}
			case "trang_thai", "hello":
			default:
				t.Fatalf("run %d: unexpected %s", i, e.loai)
			}
		}
		if noi.String() != want || xong == nil || xong["text"] != want {
			t.Fatalf("run %d: deltas %q, xong %v", i, noi.String(), xong)
		}
		var text string
		var first *time.Time
		if err := f.pool.QueryRow(context.Background(), `SELECT result->>'text',first_token_at FROM chat_ai_invocations WHERE id=$1`, id).Scan(&text, &first); err != nil || text != want || first == nil {
			t.Fatalf("run %d: row %q first_token_at %v %v", i, text, first, err)
		}
		if ttl := d.rdb.TTL(context.Background(), d.khoaMoi(id)).Val(); ttl <= 0 || ttl > aistream.AfterTerminal {
			t.Fatalf("run %d: finished key TTL %v", i, ttl)
		}
	}
	for _, k := range d.khoa(t) {
		if strings.Contains(k, ":room:") {
			t.Fatalf("a Nếp job wrote a room key: %s", k)
		}
	}
	t.Logf("latency over %d runs: first status after 202 p50=%v p95=%v max=%v; writer (XADD) -> client, %d deltas, p50=%v p95=%v max=%v",
		lan, phanViSSE(dau, .5).Round(time.Millisecond), phanViSSE(dau, .95).Round(time.Millisecond), phanViSSE(dau, 1).Round(time.Millisecond),
		len(nha), phanViSSE(nha, .5).Round(time.Millisecond), phanViSSE(nha, .95).Round(time.Millisecond), phanViSSE(nha, 1).Round(time.Millisecond))
	if phanViSSE(dau, .95) > 300*time.Millisecond {
		t.Errorf("first status p95 %v, over the contract's 300 ms", phanViSSE(dau, .95))
	}
	if phanViSSE(nha, .95) > 150*time.Millisecond {
		t.Errorf("writer -> client p95 %v, over design 02's 150 ms", phanViSSE(nha, .95))
	}
}

// Resuming mid-stream returns exactly the tail: Last-Event-ID (native) and
// ?after= (web) alike, exclusive of the position given (M2 is red here).
func TestStreamNoiLaiGiuaChungDungPhanDuoi(t *testing.T) {
	url := amqpURL(t)
	f := setup(t, nil)
	f.nepTrenEngine(t, nepDai(1, 0))
	d := moDongSong(t, f)
	moTram(t, f, f.handler, url, topology(t, url), false).choSong(t)
	id, _ := d.hoiNep(t, "tối nay đi đâu?")
	path := "/me/nep/ai-invocations/" + id + "/events"
	_, all, _ := d.nghe(t, path, d.token, "", 10*time.Second)
	var withID []suKienSSE
	for _, e := range all {
		if e.id != "" {
			withID = append(withID, e)
		}
	}
	if len(withID) < 4 || withID[len(withID)-1].loai != "xong" {
		t.Fatalf("stream %+v", all)
	}
	// The whole read is the reference, so it must be right on its own: every
	// id once, in order (a reader that re-reads its last position repeats it).
	tangDan := func(how string, ds []suKienSSE) {
		t.Helper()
		for i := 1; i < len(ds); i++ {
			if a, b := msTuID(ds[i-1].id), msTuID(ds[i].id); ds[i].id == ds[i-1].id || b.Before(a) {
				t.Fatalf("%s: event %d repeats or precedes the one before it (%s after %s)", how, i, ds[i].id, ds[i-1].id)
			}
		}
	}
	tangDan("whole stream", withID)
	k := len(withID) / 2
	for _, how := range []string{"header", "query"} {
		var tail []suKienSSE
		if how == "header" {
			_, tail, _ = d.nghe(t, path, d.token, withID[k].id, 5*time.Second)
		} else {
			_, tail, _ = d.nghe(t, path+"?after="+withID[k].id, d.token, "", 5*time.Second)
		}
		if len(tail) == 0 || tail[0].loai != "hello" {
			t.Fatalf("%s: resume opened with %+v", how, tail)
		}
		tail = tail[1:]
		if len(tail) != len(withID)-k-1 {
			t.Fatalf("%s: resume after event %d returned %d events, want %d", how, k, len(tail), len(withID)-k-1)
		}
		for i, e := range tail {
			if e.id != withID[k+1+i].id || e.data != withID[k+1+i].data {
				t.Fatalf("%s: event %d is %s, want %s", how, i, e.id, withID[k+1+i].id)
			}
		}
	}
}

// A legacy-lane group job writes its room's key, each entry naming the
// invocation; its requester reads it filtered through the group route:
// the claim's status, the finished text as one delta through the window,
// then xong with the posted card's id.
func TestStreamNhomLaneCuQuaKhoaPhong(t *testing.T) {
	url := amqpURL(t)
	f := setup(t, nil)
	d := moDongSong(t, f)
	moTram(t, f, f.handler, url, topology(t, url), false).choSong(t)
	v := f.create(t)
	_, events, _ := d.nghe(t, f.route()+"/"+v.ID+"/events", f.token, "", 10*time.Second)
	var kinds []string
	for _, e := range events {
		kinds = append(kinds, e.loai)
	}
	var msg string
	if err := f.pool.QueryRow(context.Background(), `SELECT message_id::text FROM chat_ai_invocations WHERE id=$1`, v.ID).Scan(&msg); err != nil {
		t.Fatal(err)
	}
	last := events[len(events)-1]
	if last.loai != "xong" || !strings.Contains(last.data, msg) || !strings.Contains(strings.Join(kinds, ","), "delta") {
		t.Fatalf("group stream %v, last %+v", kinds, last)
	}
	entries, err := d.rdb.XRange(context.Background(), d.khoaPhong(f.context), "-", "+").Result()
	if err != nil || len(entries) < 3 {
		t.Fatalf("room key: %d entries %v", len(entries), err)
	}
	for _, e := range entries {
		if e.Values["inv"] != v.ID {
			t.Fatalf("a room entry without its invocation: %v", e.Values)
		}
	}
	if n := d.rdb.Exists(context.Background(), d.khoaMoi(v.ID)).Val(); n != 0 {
		t.Fatal("a legacy-lane job wrote its invocation key")
	}
	// chat-capabilities says the requester can watch.
	w := f.request("GET", "/contexts/"+f.context+"/chat-capabilities", f.token, nil)
	if !strings.Contains(w.Body.String(), `"stream":"nguoi_goi"`) {
		t.Fatalf("capabilities: %s", w.Body.String())
	}
}

// A member whose membership is revoked while reading gets thu_hoi at the
// next authorization check, and the stream closes.
func TestStreamThanhVienBiRutNhanThuHoi(t *testing.T) {
	url := amqpURL(t)
	f := setup(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * time.Second)
		reply(w, 200, map[string]any{"kind": "text", "payload": map[string]string{"text": "Synthetic slow fixture"}})
	})
	cfg := DefaultWorkerConfig()
	cfg.Heartbeat = 20 * time.Second
	f.handler.WithWorker(cfg)
	f.handler.sseXacThucMoi = 200 * time.Millisecond
	d := moDongSong(t, f)
	moTram(t, f, f.handler, url, topology(t, url), false).choSong(t)
	v := f.create(t)
	go func() {
		time.Sleep(500 * time.Millisecond)
		_, _ = f.pool.Exec(context.Background(), `UPDATE memberships SET state='left',left_at=clock_timestamp() WHERE id=$1`, f.member)
	}()
	start := time.Now()
	_, events, _ := d.nghe(t, f.route()+"/"+v.ID+"/events", f.token, "", 8*time.Second)
	if len(events) == 0 || events[len(events)-1].loai != "thu_hoi" {
		t.Fatalf("revoked member heard %+v", events)
	}
	if took := time.Since(start); took > 2*time.Second {
		t.Fatalf("thu_hoi after %v", took)
	}
}

// Canary 2 of design 02 §9: a v2 (E2EE) job never writes its room's key. The
// job runs its whole course -- claim, the finished text through the window,
// the card, xong -- and XRANGE of the room key stays empty; everything went
// to the invocation key. Dropping khoa's lane branch turns this red.
func TestStreamViecV2KhongGhiKhoaPhong(t *testing.T) {
	f := setup(t, nil)
	d := moDongSong(t, f)
	v := f.create(t)
	if _, err := f.pool.Exec(context.Background(), `UPDATE chat_ai_invocations SET lane='v2' WHERE id=$1`, v.ID); err != nil {
		t.Fatal(err)
	}
	if ok, err := f.handler.ProcessOne(context.Background()); err != nil || !ok {
		t.Fatalf("worker: %v %v", ok, err)
	}
	if n, err := d.rdb.XLen(context.Background(), d.khoaPhong(f.context)).Result(); err != nil || n != 0 {
		t.Fatalf("a v2 job left %d entries in the room key (%v)", n, err)
	}
	entries, _ := d.rdb.XRange(context.Background(), d.khoaMoi(v.ID), "-", "+").Result()
	if len(entries) < 3 || entries[len(entries)-1].Values["e"] != "xong" {
		t.Fatalf("the v2 job's own key: %v", entries)
	}
}

// Canary 3 of design 02 §9 (design 01 §7 canary 3 at slice 11): a stub
// answer with a phone number in the middle leaves 0 bytes of itself in any
// key of the namespace -- not even its clean head: the engine releases
// nothing before the structural checks and the verifier have read the whole
// draft. The job fails ai_tra_loi_bi_chan and its stream ends with that_bai.
// An engine that streamed the draft before checking it turns this red.
func TestStreamSoDienThoaiGiuaCauKhongDeByteNao(t *testing.T) {
	f := setup(t, nil)
	f.nepTrenEngine(t, llm.NewStub(kichNep("Quán nướng đó mở tới 22 giờ, hợp cho nhóm đông người đi tối nay. Gọi 0912 "+"345 678 để giữ bàn trước nhé. Nhớ đi sớm cho có chỗ đậu xe.", 0, nil)...))
	d := moDongSong(t, f)
	id := f.chenNep(t, 1, func(int) string { return "quán nướng đó còn chỗ không?" })[0]
	if ok, err := f.handler.ProcessOne(context.Background()); err != nil || !ok {
		t.Fatalf("worker: %v %v", ok, err)
	}
	all := d.tatCa(t)
	for _, lo := range []string{"Quán nướng", "Gọi", "0912", "345", "678", "giữ bàn", "Nhớ đi sớm", "đậu xe"} {
		if strings.Contains(all, lo) {
			t.Fatalf("%q reached Redis: %s", lo, all)
		}
	}
	var status, code string
	if err := f.pool.QueryRow(context.Background(), `SELECT status,code FROM chat_ai_invocations WHERE id=$1`, id).Scan(&status, &code); err != nil {
		t.Fatal(err)
	}
	if status != "failed" || code != string(cau.TraLoiBiChan) {
		t.Fatalf("row %s/%s", status, code)
	}
	if k := d.loaiCua(t, d.khoaMoi(id)); len(k) == 0 || k[len(k)-1] != "that_bai:{\"code\":\"ai_tra_loi_bi_chan\"}" || strings.Contains(strings.Join(k, ","), "delta") {
		t.Fatalf("stream %v", k)
	}
}

// tatCa is every value of every stream entry in the test's namespace.
func (d dongSong) tatCa(t *testing.T) string {
	t.Helper()
	var all strings.Builder
	for _, k := range d.khoa(t) {
		entries, err := d.rdb.XRange(context.Background(), k, "-", "+").Result()
		if err != nil {
			continue // the rate-limit counter is not a stream
		}
		for _, e := range entries {
			for _, v := range e.Values {
				fmt.Fprint(&all, v, "\n")
			}
		}
	}
	return all.String()
}

// loaiCua is a key's entries as «kind:json».
func (d dongSong) loaiCua(t *testing.T, key string) []string {
	t.Helper()
	es, _ := d.rdb.XRange(context.Background(), key, "-", "+").Result()
	var out []string
	for _, e := range es {
		out = append(out, fmt.Sprint(e.Values["e"])+":"+fmt.Sprint(e.Values["j"]))
	}
	return out
}

// A stream that outlives the engine's 8 s request bound still ends with xong
// (M4, the 8 s timeout applied to /events, is red here).
func TestStreamSongQuaTamGiay(t *testing.T) {
	url := amqpURL(t)
	f := setup(t, nil)
	f.nepTrenEngine(t, nepDai(1, 9*time.Second))
	d := moDongSong(t, f)
	moTram(t, f, f.handler, url, topology(t, url), false).choSong(t)
	id, _ := d.hoiNep(t, "tối nay đi đâu?")
	start := time.Now()
	_, events, _ := d.nghe(t, "/me/nep/ai-invocations/"+id+"/events", d.token, "", 20*time.Second)
	took := time.Since(start)
	if took < 8*time.Second || len(events) == 0 || events[len(events)-1].loai != "xong" {
		t.Fatalf("after %v the stream ended with %+v", took, events[len(events)-1:])
	}
}

// The row speaks for a stream Redis does not hold: a queued job with nothing
// written yet opens with trang_thai dang_xep_hang and no id; a finished job
// whose key is gone ends at once with the row's own ending.
func TestStreamTuHangPostgres(t *testing.T) {
	f := setup(t, nil)
	f.nepTrenEngine(t, nepDai(1, 0))
	d := moDongSong(t, f)
	id := f.chenNep(t, 1, func(int) string { return "tối nay đi đâu?" })[0]
	path := "/me/nep/ai-invocations/" + id + "/events"
	_, events, raw := d.nghe(t, path, d.token, "", 700*time.Millisecond)
	if len(events) < 2 || events[1].loai != "trang_thai" || events[1].data != `{"cau":"dang_xep_hang"}` || events[1].id != "" || strings.Count(raw, "id: ") != 0 {
		t.Fatalf("queued job opened with %q", raw)
	}
	if ok, err := f.handler.ProcessOne(context.Background()); err != nil || !ok {
		t.Fatalf("worker: %v %v", ok, err)
	}
	d.rdb.Del(context.Background(), d.khoaMoi(id))
	start := time.Now()
	_, events, _ = d.nghe(t, path, d.token, "", 5*time.Second)
	want, _ := json.Marshal(xongNepData(ptr(traLoiDai)))
	if len(events) != 2 || events[1].loai != "xong" || events[1].data != string(want) || time.Since(start) > time.Second {
		t.Fatalf("finished job without its key: %+v", events)
	}
}

func ptr(s string) *string { return &s }

// maTraVe is a refusal's code.
func maTraVe(w *httptest.ResponseRecorder) string {
	var body struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	return body.Code
}

// Refusals the client turns into polling: no stream on the host, a Redis
// that does not answer (503 stream_unavailable, Retry-After), a sixth stream
// for one person (503 stream_capacity), a 31st opening in a minute (429).
func TestStreamTuChoi(t *testing.T) {
	f := setup(t, nil)
	id := f.chenNep(t, 1, func(int) string { return "tối nay đi đâu?" })[0]
	path := "/me/nep/ai-invocations/" + id + "/events"
	w := f.request("GET", path, f.token, nil)
	if w.Code != 503 || maTraVe(w) != "stream_unavailable" || w.Header().Get("Retry-After") == "" {
		t.Fatalf("no stream: %d %s", w.Code, w.Body.String())
	}
	dead, _ := aistream.Open("redis://127.0.0.1:1/0", "t")
	defer dead.Close()
	f.handler.WithStream(dead, aistream.NewHub(), nil)
	w = f.request("GET", path, f.token, nil)
	if w.Code != 503 || maTraVe(w) != "stream_unavailable" || w.Header().Get("Retry-After") == "" {
		t.Fatalf("Redis down: %d %s", w.Code, w.Body.String())
	}
	d := moDongSong(t, f)
	stop := make(chan struct{})
	defer close(stop)
	for i := 0; i < sseMoiNguoi; i++ {
		go func() {
			req, _ := http.NewRequest("GET", d.srv.URL+path, nil)
			req.Header.Set("Authorization", "Bearer "+f.token)
			ctx, cancel := context.WithCancel(context.Background())
			go func() { <-stop; cancel() }()
			resp, err := http.DefaultClient.Do(req.WithContext(ctx))
			if err == nil {
				defer resp.Body.Close()
				_, _ = bufio.NewReader(resp.Body).ReadString(0)
			}
		}()
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		f.handler.sucChua.mu.Lock()
		n := f.handler.sucChua.tong
		f.handler.sucChua.mu.Unlock()
		if n == sseMoiNguoi || time.Now().After(deadline) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	resp, _, _ := d.nghe(t, path, f.token, "", time.Second)
	if resp.StatusCode != 503 || resp.Header.Get("Retry-After") == "" {
		t.Fatalf("sixth stream: %d", resp.StatusCode)
	}
	// Openings are counted in Redis per person: 6 so far; 24 more pass the
	// limit, the 31st does not.
	for i := 0; i < 30-sseMoiNguoi-1; i++ {
		d.stream.MoLuot(context.Background(), f.person, sseMoMoiPhut)
	}
	resp, _, _ = d.nghe(t, path, f.token, "", time.Second)
	if resp.StatusCode != 429 {
		t.Fatalf("31st opening: %d", resp.StatusCode)
	}
}

// choDelta waits until key holds a delta.
func (d dongSong) choDelta(t *testing.T, key string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(strings.Join(d.loaiCua(t, key), ","), "delta:") {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("no delta reached %s", key)
}

// SIGTERM giữa job vẫn đăng đúng một lần (design 02 §9; review of slice 11,
// finding 1). The worker stops after the answer's text reached its readers
// and before the answer committed: the job is not handed back (a second
// worker would write the answer again), it keeps running inside its grace,
// and it ends exactly once -- one sealed answer, one xong, the Deltas joined
// equal to it. Cancelling every job on SIGTERM, as before, leaves the row
// running with no ending here.
func TestStreamSigtermGiuaJobVanDangMotLan(t *testing.T) {
	f := setup(t, nil)
	f.nepTrenEngine(t, nepDai(1, 0))
	d := moDongSong(t, f)
	giu := make(chan struct{})
	f.handler.truocChot = func(context.Context) { <-giu }
	id := f.chenNep(t, 1, func(int) string { return "tối nay đi đâu?" })[0]
	ctx, sigterm := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := f.handler.ProcessOne(ctx); done <- err }()
	d.choDelta(t, d.khoaMoi(id))
	sigterm()
	time.Sleep(200 * time.Millisecond)
	close(giu)
	if err := <-done; err != nil {
		t.Fatalf("worker: %v", err)
	}
	var status, text string
	var first *time.Time
	if err := f.pool.QueryRow(context.Background(), `SELECT status,COALESCE(result->>'text',''),first_token_at FROM chat_ai_invocations WHERE id=$1`, id).Scan(&status, &text, &first); err != nil {
		t.Fatal(err)
	}
	if status != "succeeded" || text != traLoiDai || first == nil {
		t.Fatalf("row %s %q first_token_at %v", status, text, first)
	}
	k := d.loaiCua(t, d.khoaMoi(id))
	var noi strings.Builder
	xong := 0
	for _, e := range k {
		switch {
		case strings.HasPrefix(e, "delta:"):
			var dd aistream.DeltaData
			_ = json.Unmarshal([]byte(strings.TrimPrefix(e, "delta:")), &dd)
			noi.WriteString(dd.Text)
		case strings.HasPrefix(e, "xong:"):
			xong++
		case strings.HasPrefix(e, "lam_lai:"), strings.HasPrefix(e, "that_bai:"), strings.HasPrefix(e, "huy:"):
			t.Fatalf("stream %v", k)
		}
	}
	if xong != 1 || noi.String() != traLoiDai || !strings.HasPrefix(k[len(k)-1], "xong:") {
		t.Fatalf("stream %v", k)
	}
	if _, ok, err := f.handler.claimNext(context.Background(), 0); err != nil || ok {
		t.Fatalf("the job was claimable again: %v %v", ok, err)
	}
}

// A job with content out that does not finish within its grace fails
// worker_interrupted, and its stream ends with that_bai -- the reader is told
// it was cut off rather than left waiting (design 02 §7).
func TestStreamSigtermHetAnHanThatBai(t *testing.T) {
	f := setup(t, nil)
	cfg := DefaultWorkerConfig()
	cfg.AnHanDung = 300 * time.Millisecond
	f.handler.WithWorker(cfg)
	f.nepTrenEngine(t, nepDai(1, 0))
	d := moDongSong(t, f)
	f.handler.truocChot = func(ctx context.Context) { <-ctx.Done() }
	id := f.chenNep(t, 1, func(int) string { return "tối nay đi đâu?" })[0]
	ctx, sigterm := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); _, _ = f.handler.ProcessOne(ctx) }()
	d.choDelta(t, d.khoaMoi(id))
	start := time.Now()
	sigterm()
	<-done
	if took := time.Since(start); took < 250*time.Millisecond || took > 3*time.Second {
		t.Fatalf("the job ended %v after SIGTERM, its grace is 300 ms", took)
	}
	var status, code string
	if err := f.pool.QueryRow(context.Background(), `SELECT status,code FROM chat_ai_invocations WHERE id=$1`, id).Scan(&status, &code); err != nil {
		t.Fatal(err)
	}
	if status != "failed" || code != "worker_interrupted" {
		t.Fatalf("row %s/%s", status, code)
	}
	_, events, _ := d.nghe(t, "/me/nep/ai-invocations/"+id+"/events", d.token, "", 3*time.Second)
	if len(events) == 0 || events[len(events)-1].loai != "that_bai" || events[len(events)-1].data != `{"code":"worker_interrupted"}` {
		t.Fatalf("reader heard %+v", events)
	}
}

// SIGTERM before any content: the job goes back to the queue at once, its
// stream says lam_lai, and nothing is marked -- the next worker runs it.
func TestStreamSigtermTruocNoiDungTraVeHang(t *testing.T) {
	f := setup(t, nil)
	f.nepTrenEngine(t, nepDai(1, time.Minute))
	d := moDongSong(t, f)
	id := f.chenNep(t, 1, func(int) string { return "tối nay đi đâu?" })[0]
	ctx, sigterm := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); _, _ = f.handler.ProcessOne(ctx) }()
	time.Sleep(300 * time.Millisecond)
	start := time.Now()
	sigterm()
	<-done
	if took := time.Since(start); took > 2*time.Second {
		t.Fatalf("a job with no content held the stop for %v", took)
	}
	var status string
	var first *time.Time
	if err := f.pool.QueryRow(context.Background(), `SELECT status,first_token_at FROM chat_ai_invocations WHERE id=$1`, id).Scan(&status, &first); err != nil {
		t.Fatal(err)
	}
	k := d.loaiCua(t, d.khoaMoi(id))
	if status != "queued" || first != nil || len(k) == 0 || k[len(k)-1] != "lam_lai:{}" {
		t.Fatalf("row %s first_token_at %v, stream %v", status, first, k)
	}
}

// A worker that died after content (no SIGTERM, nothing renewed its lease):
// the sweep fails the job worker_interrupted and ends its stream with
// that_bai, so a reader is not left waiting (design 02 §4 step 7).
func TestStreamSweepNgatSauNoiDung(t *testing.T) {
	f := setup(t, nil)
	d := moDongSong(t, f)
	id := f.chenNep(t, 1, func(int) string { return "tối nay đi đâu?" })[0]
	ctx := context.Background()
	if _, err := f.pool.Exec(ctx, `UPDATE chat_ai_invocations SET status='running',attempts=1,lease_id=$2,lease_until=clock_timestamp()-interval '1 second',first_token_at=clock_timestamp() WHERE id=$1`, id, newID()); err != nil {
		t.Fatal(err)
	}
	if _, err := d.stream.Append(ctx, d.khoaMoi(id), aistream.MaxLenInvocation, aistream.Delta, aistream.DeltaData{Text: "Tối nay "}, time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := f.handler.Sweep(ctx); err != nil {
		t.Fatal(err)
	}
	k := d.loaiCua(t, d.khoaMoi(id))
	if len(k) != 2 || k[1] != `that_bai:{"code":"worker_interrupted"}` {
		t.Fatalf("stream after the sweep %v", k)
	}
	// Identity: a job the sweep fails with no content out (attempts spent)
	// has no stream to end, and none is written.
	other := f.chenNep(t, 1, func(int) string { return "mai đi đâu?" })[0]
	if _, err := f.pool.Exec(ctx, `UPDATE chat_ai_invocations SET status='running',attempts=3,lease_id=$2,lease_until=clock_timestamp()-interval '1 second' WHERE id=$1`, other, newID()); err != nil {
		t.Fatal(err)
	}
	if err := f.handler.Sweep(ctx); err != nil {
		t.Fatal(err)
	}
	if n := d.rdb.Exists(ctx, d.khoaMoi(other)).Val(); n != 0 {
		t.Fatal("the sweep wrote a stream for a job with no content")
	}
}

// The row is the truth (review of slice 11, finding 2): a queued job the
// requester cancels while a reader waits ends the reader with huy within a
// couple of reconcile ticks of the re-authorization, instead of idling to
// the 180 s bound; the ending comes without an id, since it is not in the
// stream.
func TestStreamHuyKhiDangXepHangKetThucTuHang(t *testing.T) {
	f := setup(t, nil)
	f.handler.sseXacThucMoi = 200 * time.Millisecond
	d := moDongSong(t, f)
	v := f.create(t)
	go func() {
		time.Sleep(400 * time.Millisecond)
		f.request("POST", f.route()+"/"+v.ID+"/cancel", f.token, map[string]any{})
	}()
	start := time.Now()
	_, events, _ := d.nghe(t, f.route()+"/"+v.ID+"/events", f.token, "", 8*time.Second)
	last := events[len(events)-1]
	if took := time.Since(start); last.loai != "huy" || last.id != "" || took > 4*time.Second {
		t.Fatalf("after %v the reader heard %+v", took, events)
	}
}

// Someone else's stream reads as absent (review of slice 11, finding 3):
// another person's Nếp question, and -- inside the same room -- another
// member's group invocation. The owner's own opens (identity).
func TestStreamCuaNguoiKhacLa404(t *testing.T) {
	f := setup(t, nil)
	d := moDongSong(t, f)
	id := f.chenNep(t, 1, func(int) string { return "tối nay đi đâu?" })[0]
	v := f.create(t)
	for _, c := range []struct {
		path, token string
		want        int
	}{
		{"/me/nep/ai-invocations/" + id + "/events", f.peerToken, 404},
		{f.route() + "/" + v.ID + "/events", f.peerToken, 404},
		{"/me/nep/ai-invocations/" + id + "/events", f.token, 200},
		{f.route() + "/" + v.ID + "/events", f.token, 200},
	} {
		resp, events, _ := d.nghe(t, c.path, c.token, "", 700*time.Millisecond)
		if resp.StatusCode != c.want {
			t.Errorf("%s as %s: %d, want %d", c.path, c.token[:14], resp.StatusCode, c.want)
		}
		if c.want == 404 && len(events) != 0 {
			t.Errorf("%s: a refused reader heard %+v", c.path, events)
		}
	}
}

// A group job that fails ends its stream with that_bai{code} (review of
// slice 11, finding 3), after the failure committed.
func TestStreamThatBaiKetThucLuong(t *testing.T) {
	f := setup(t, func(w http.ResponseWriter, r *http.Request) {
		reply(w, 200, map[string]any{"kind": "khong_ro"})
	})
	d := moDongSong(t, f)
	v := f.create(t)
	if ok, err := f.handler.ProcessOne(context.Background()); err != nil || !ok {
		t.Fatalf("worker: %v %v", ok, err)
	}
	var status, code string
	if err := f.pool.QueryRow(context.Background(), `SELECT status,code FROM chat_ai_invocations WHERE id=$1`, v.ID).Scan(&status, &code); err != nil {
		t.Fatal(err)
	}
	k := d.loaiCua(t, d.khoaPhong(f.context))
	if status != "failed" || len(k) == 0 || k[len(k)-1] != `that_bai:{"code":"`+code+`"}` {
		t.Fatalf("row %s/%s, room key %v", status, code, k)
	}
}

// A job cancelled while it runs ends its stream with huy (review of slice
// 11, finding 3): the heartbeat finds the lease gone and the worker tells
// the stream how, from the row.
func TestStreamHuyKetThucLuong(t *testing.T) {
	f := setup(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * time.Second)
		reply(w, 200, map[string]any{"kind": "text", "payload": map[string]string{"text": "Synthetic slow fixture"}})
	})
	cfg := DefaultWorkerConfig()
	cfg.Heartbeat = 100 * time.Millisecond
	f.handler.WithWorker(cfg)
	d := moDongSong(t, f)
	v := f.create(t)
	done := make(chan struct{})
	go func() { defer close(done); _, _ = f.handler.ProcessOne(context.Background()) }()
	time.Sleep(500 * time.Millisecond)
	if w := f.request("POST", f.route()+"/"+v.ID+"/cancel", f.token, map[string]any{}); w.Code != 200 {
		t.Fatalf("cancel: %d %s", w.Code, w.Body.String())
	}
	<-done
	k := d.loaiCua(t, d.khoaPhong(f.context))
	if len(k) == 0 || k[len(k)-1] != "huy:{}" {
		t.Fatalf("room key %v", k)
	}
}

// The brain's Nếp answer through Redis (review of slice 11, finding 5): one
// the output guard stops leaves 0 bytes of itself in any key and fails
// ai_tra_loi_bi_chan; a clean one reaches the stream as one delta after its
// commit, then xong with the same text. A raw write of the brain's text into
// the stream, past the window, turns the first half red.
func TestStreamBrainNepQuaRedis(t *testing.T) {
	for _, c := range []struct {
		ten, text, status string
	}{
		{"chan", "Quán nướng đó mở tới 22 giờ nhé. Gọi 0912 " + "345 678 để giữ bàn trước.", "failed"},
		{"sach", "Quán nướng đó mở tới 22 giờ nhé, bạn đi sớm cho có chỗ.", "succeeded"},
	} {
		t.Run(c.ten, func(t *testing.T) {
			f := setup(t, func(w http.ResponseWriter, r *http.Request) {
				reply(w, 200, map[string]any{"text": c.text})
			})
			d := moDongSong(t, f)
			id := f.chenNep(t, 1, func(int) string { return "quán nướng đó còn chỗ không?" })[0]
			if _, err := f.pool.Exec(context.Background(), `UPDATE chat_ai_invocations SET boi_canh='{"luot":[]}' WHERE id=$1`, id); err != nil {
				t.Fatal(err)
			}
			if ok, err := f.handler.ProcessOne(context.Background()); err != nil || !ok {
				t.Fatalf("worker: %v %v", ok, err)
			}
			status, _, _, _ := f.trangThai(t, id)
			all, k := d.tatCa(t), d.loaiCua(t, d.khoaMoi(id))
			if status != c.status {
				t.Fatalf("row %s, stream %v", status, k)
			}
			if c.ten == "chan" {
				if strings.Contains(all, "Quán nướng") || strings.Contains(all, "0912") || k[len(k)-1] != `that_bai:{"code":"ai_tra_loi_bi_chan"}` {
					t.Fatalf("the stopped brain answer reached Redis: %v", k)
				}
				return
			}
			var noi strings.Builder
			for _, e := range k {
				if strings.HasPrefix(e, "delta:") {
					var dd aistream.DeltaData
					_ = json.Unmarshal([]byte(strings.TrimPrefix(e, "delta:")), &dd)
					noi.WriteString(dd.Text)
				}
			}
			if noi.String() != c.text || !strings.HasPrefix(k[len(k)-1], "xong:") || !strings.Contains(k[len(k)-1], "đi sớm") {
				t.Fatalf("clean brain answer: %v", k)
			}
		})
	}
}

// The owner's requirement (2026-09-27): the answer appears progressively.
// With production pacing (aiharness.NhipPhat) a Nếp answer reaches a real
// SSE client as several deltas, spread over time and all before xong, their
// text joined equal to the sealed answer; a group card's text reaches its
// requester the same way, after the card posted. The first status still
// comes at once; the first delta only after the verifier passed the draft.
func TestStreamHienDanQuaSSE(t *testing.T) {
	url := amqpURL(t)
	f := setup(t, func(w http.ResponseWriter, r *http.Request) {
		reply(w, 200, map[string]any{"kind": "text", "payload": map[string]string{"text": traLoiDai}})
	})
	f.nepTrenEngine(t, nepDai(1, 0), aiharness.WithNhipPhat(aiharness.NhipPhat))
	d := moDongSong(t, f)
	moTram(t, f, f.handler, url, topology(t, url), false).choSong(t)
	kiem := func(how string, events []suKienSSE) {
		t.Helper()
		var noi strings.Builder
		var luc []time.Time
		for i, e := range events {
			if e.loai == "delta" {
				var dd aistream.DeltaData
				_ = json.Unmarshal([]byte(e.data), &dd)
				noi.WriteString(dd.Text)
				luc = append(luc, e.luc)
				if i == len(events)-1 {
					t.Fatalf("%s: a delta after the ending", how)
				}
			}
		}
		if len(luc) < 3 || noi.String() != traLoiDai || events[len(events)-1].loai != "xong" {
			t.Fatalf("%s: %d deltas, text %q, last %+v", how, len(luc), noi.String(), events[len(events)-1])
		}
		if trai := luc[len(luc)-1].Sub(luc[0]); trai < 100*time.Millisecond || trai > 2*time.Second {
			t.Fatalf("%s: the deltas came within %v", how, trai)
		}
	}
	id, _ := d.hoiNep(t, "tối nay đi đâu?")
	_, events, _ := d.nghe(t, "/me/nep/ai-invocations/"+id+"/events", d.token, "", 10*time.Second)
	kiem("Nếp", events)
	v := f.create(t)
	_, events, _ = d.nghe(t, f.route()+"/"+v.ID+"/events", f.token, "", 10*time.Second)
	kiem("group", events)
}

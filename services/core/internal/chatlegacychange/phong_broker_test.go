//go:build broker

package chatlegacychange

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/redis/go-redis/v9"
	"mobile/services/core/internal/aistream"
	"mobile/services/core/internal/auth"
)

// The room's `ai` frames end to end (slice 12; design 02 §5.3, §9): the
// feed's own WebSocket, its own authorization, a real Redis stream written
// the way the worker writes a legacy-lane room key.

type phongThu struct {
	world
	stream *aistream.Stream
	rdb    *redis.Client
	ns     string
	h      *Handler
	srv    *httptest.Server
}

func moPhongThu(t *testing.T) phongThu {
	t.Helper()
	url := os.Getenv("CORE_TEST_REDIS_URL")
	if url == "" {
		if os.Getenv("CORE_REQUIRE_BROKER_TESTS") == "1" {
			t.Fatal("CORE_TEST_REDIS_URL is required")
		}
		t.Skip("CORE_TEST_REDIS_URL not set")
	}
	w := setup(t)
	ns := fmt.Sprintf("p%d", time.Now().UnixNano()%1_000_000_000_000)
	s, err := aistream.Open(url, ns)
	if err != nil {
		t.Fatal(err)
	}
	opt, _ := redis.ParseURL(url)
	rdb := redis.NewClient(opt)
	hub := aistream.NewHub()
	ctx, cancel := context.WithCancel(bg)
	ready := make(chan struct{})
	go s.Listen(ctx, hub, ready)
	select {
	case <-ready:
	case <-time.After(5 * time.Second):
		t.Fatal("the wake subscription never came up")
	}
	h := New(w.store, ctx, nil).WithAi(s, hub)
	h.ReconcileInterval = 20 * time.Millisecond
	h.AuthTimeout = time.Second
	srv := httptest.NewServer(h)
	p := phongThu{world: w, stream: s, rdb: rdb, ns: ns, h: h, srv: srv}
	t.Cleanup(func() {
		srv.Close()
		cancel()
		iter := rdb.Scan(bg, 0, "rudi:"+ns+":*", 1000).Iterator()
		for iter.Next(bg) {
			rdb.Del(bg, iter.Val())
		}
		_ = rdb.Close()
		_ = s.Close()
	})
	return p
}

// ketNoi is one member's socket: authenticated by frame (ai says whether it
// asks for `ai` frames) or, with byHeader, by the Authorization header. Every
// page is acknowledged as a client does; `ai` frames go to ai, and are never
// acknowledged. closed receives the close error once the server ends it.
type ketNoi struct {
	c      *websocket.Conn
	ai     chan khungNhan
	pages  chan Page
	closed chan error
}

func (p phongThu) ketNoi(t *testing.T, who int, ai, byHeader bool) *ketNoi {
	t.Helper()
	ctx, stop := context.WithTimeout(bg, 3*time.Second)
	defer stop()
	address := "ws" + strings.TrimPrefix(p.srv.URL, "http") + "/contexts/" + p.room + "/changes/stream?after=0"
	var opt *websocket.DialOptions
	if byHeader {
		opt = &websocket.DialOptions{HTTPHeader: p.headers[who]}
	}
	c, _, err := websocket.Dial(ctx, address, opt)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.CloseNow() })
	if !byHeader {
		if err = wsjson.Write(ctx, c, map[string]any{"type": "authenticate", "token": p.tokens[who], "ai": ai}); err != nil {
			t.Fatal(err)
		}
	}
	k := &ketNoi{c: c, ai: make(chan khungNhan, 4096), pages: make(chan Page, 64), closed: make(chan error, 1)}
	go func() {
		for {
			_, raw, err := c.Read(bg)
			if err != nil {
				k.closed <- err
				return
			}
			var probe map[string]any
			if json.Unmarshal(raw, &probe) != nil {
				k.closed <- fmt.Errorf("not JSON: %s", raw)
				return
			}
			if probe["type"] == "ai" {
				k.ai <- khungNhan{probe, time.Now()}
				continue
			}
			var page Page
			_ = json.Unmarshal(raw, &page)
			k.pages <- page
			_ = wsjson.Write(bg, c, map[string]any{"type": "ack", "sequence": page.NextSequence})
		}
	}()
	return k
}

// khungNhan is one frame and when the socket read it.
type khungNhan struct {
	f  map[string]any
	at time.Time
}

// trang waits for a page.
func (k *ketNoi) trang(t *testing.T) Page {
	t.Helper()
	select {
	case page := <-k.pages:
		return page
	case err := <-k.closed:
		t.Fatalf("closed before a page: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("no page")
	}
	return Page{}
}

func (k *ketNoi) khung(t *testing.T, within time.Duration) (map[string]any, bool) {
	t.Helper()
	f, ok := k.khungLuc(within)
	return f.f, ok
}

func (k *ketNoi) khungLuc(within time.Duration) (khungNhan, bool) {
	select {
	case f := <-k.ai:
		return f, true
	case <-time.After(within):
		return khungNhan{}, false
	}
}

// viet writes one legacy-lane answer to the room key the way the worker
// does: status, then (after the card committed) the text, then xong.
type viet struct {
	w   *aistream.Writer
	inv string
}

func (p phongThu) viet(t *testing.T, tin string) viet {
	t.Helper()
	key, err := p.stream.Keys.Room(p.room)
	if err != nil {
		t.Fatal(err)
	}
	inv := id()
	return viet{inv: inv, w: p.stream.NewWriter(aistream.WriterOptions{Key: key, MaxLen: aistream.MaxLenRoom, Inv: inv, Tin: tin, SoTin: 5,
		ExpireAt: time.Now().Add(10 * time.Minute), SauChotThoi: true, Flush: time.Millisecond})}
}

func (v viet) trangThai() { v.w.Ghi(aistream.TrangThai, aistream.TrangThaiData{Cau: "dang_doc"}) }
func (v viet) chu(text string) {
	v.w.SauChot()
	v.w.Ghi(aistream.Delta, aistream.DeltaData{Text: text})
	v.w.Dong()
}

// A member who asked gets the frames, in order, with the contract's
// envelope; a member on the same room who did not ask, and a connection
// authenticated by header, get none -- and nothing of the frames disturbs
// the feed: pages still arrive and are acknowledged on every connection.
func TestPhongAiChiThanhVienDaXin(t *testing.T) {
	p := moPhongThu(t)
	xin := p.ketNoi(t, 1, true, false)
	khongXin := p.ketNoi(t, 0, false, false)
	header := p.ketNoi(t, 1, true, true)
	for _, k := range []*ketNoi{xin, khongXin, header} {
		k.trang(t)
	}
	time.Sleep(100 * time.Millisecond) // the pump starts after the first page's ack
	tin := p.message(t).ID
	for _, k := range []*ketNoi{xin, khongXin, header} {
		if page := k.trang(t); len(page.Changes) != 1 {
			t.Fatalf("the feed page carries %+v", page)
		}
	}
	v := p.viet(t, tin)
	v.trangThai()
	v.chu("Hồ Xuân Hương lên đèn từ 18 giờ.")
	var got []string
	for len(got) < 2 {
		f, ok := xin.khung(t, 3*time.Second)
		if !ok {
			t.Fatalf("the member who asked got %v only", got)
		}
		if f["inv"] != v.inv || f["tin"] != tin || f["so_tin"] != float64(5) || len(f) != 7 {
			t.Fatalf("frame envelope %v", f)
		}
		d, _ := json.Marshal(f["d"])
		got = append(got, fmt.Sprint(f["e"])+string(d))
	}
	if got[0] != `trang_thai{"cau":"dang_doc"}` || got[1] != `delta{"p":0,"text":"Hồ Xuân Hương lên đèn từ 18 giờ."}` {
		t.Fatalf("frames %v", got)
	}
	for name, k := range map[string]*ketNoi{"not asking": khongXin, "header-authenticated": header} {
		if f, ok := k.khung(t, 300*time.Millisecond); ok {
			t.Fatalf("a %s connection got a frame: %v", name, f)
		}
	}
	_ = p.message(t)
	for _, k := range []*ketNoi{xin, khongXin, header} {
		if page := k.trang(t); len(page.Changes) != 1 {
			t.Fatalf("the feed page after the frames carries %+v", page)
		}
	}
}

// Someone who is not a member never gets a page nor a frame: authorize
// refuses the connection before any pump could start.
func TestPhongAiNguoiNgoaiKhongNhanGi(t *testing.T) {
	p := moPhongThu(t)
	v := p.viet(t, "")
	v.trangThai()
	out := p.ketNoi(t, 2, true, false)
	select {
	case err := <-out.closed:
		if websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
			t.Fatalf("closed with %v", err)
		}
	case page := <-out.pages:
		t.Fatalf("an outsider got a page %+v", page)
	case f := <-out.ai:
		t.Fatalf("an outsider got a frame %v", f.f)
	case <-time.After(3 * time.Second):
		t.Fatal("the outsider's socket stayed open")
	}
}

// A v2 (end to end encrypted) room never gets text on this socket: no pump
// starts in a room found in v2 (even with entries in its room key, which no
// v2 job ever writes), and a room that moves to v2 stops the pump it had.
func TestPhongAiV2KhongCoChu(t *testing.T) {
	p := moPhongThu(t)
	exec(t, p.pool, `CREATE TABLE chat_v2_conversations(context_id uuid PRIMARY KEY)`)
	live := p.ketNoi(t, 1, true, false)
	live.trang(t)
	time.Sleep(100 * time.Millisecond)
	v := p.viet(t, "")
	v.trangThai()
	if f, ok := live.khung(t, 3*time.Second); !ok || f["e"] != "trang_thai" {
		t.Fatalf("the legacy room's member got %v %v", f, ok)
	}
	exec(t, p.pool, `INSERT INTO chat_v2_conversations(context_id) VALUES($1)`, p.room)
	time.Sleep(200 * time.Millisecond) // ten reconcile ticks: a page reads the lane
	v.chu("Chữ không được tới phòng v2.")
	if f, ok := live.khung(t, 500*time.Millisecond); ok {
		t.Fatalf("a room that moved to v2 got %v", f)
	}
	joined := p.ketNoi(t, 0, true, false)
	joined.trang(t)
	w := p.viet(t, "")
	w.trangThai()
	w.chu("Cũng không tới người vào sau.")
	if f, ok := joined.khung(t, 500*time.Millisecond); ok {
		t.Fatalf("a member joining a v2 room got %v", f)
	}
}

// A member removed from the room loses the socket, and its frames, within
// about a second (the feed's reconcile runs authorize); here the tick is
// 20 ms and the bound 2 s (design 02 §9).
func TestPhongAiThuHoiDongKetNoi(t *testing.T) {
	p := moPhongThu(t)
	k := p.ketNoi(t, 1, true, false)
	k.trang(t)
	time.Sleep(100 * time.Millisecond)
	v := p.viet(t, "")
	v.trangThai()
	if _, ok := k.khung(t, 3*time.Second); !ok {
		t.Fatal("no frame before the removal")
	}
	start := time.Now()
	exec(t, p.pool, `UPDATE memberships SET state='left',left_at=now() WHERE context_id=$1 AND person_id=$2`, p.room, p.people[1])
	select {
	case <-k.closed:
	case <-time.After(2 * time.Second):
		t.Fatal("the removed member's socket is still open after 2 s")
	}
	v.chu("Chữ sau khi bị rút.")
	if f, ok := k.khung(t, 300*time.Millisecond); ok && f["e"] == "delta" {
		t.Fatalf("the removed member got text: %v", f)
	}
	t.Logf("removed member's socket closed %v after the removal", time.Since(start).Round(time.Millisecond))
}

// A reconnect is a new join: the answer still running is replayed as its
// status and one delta with the text so far, and the one that ended is not.
func TestPhongAiNoiLaiPhatLai(t *testing.T) {
	p := moPhongThu(t)
	xong := p.viet(t, "")
	xong.trangThai()
	xong.w.SauChot()
	xong.w.Ghi(aistream.Delta, aistream.DeltaData{Text: "Đã xong."})
	xong.w.Ghi(aistream.Xong, map[string]string{"message_id": id()})
	dang := p.viet(t, "")
	dang.trangThai()
	dang.w.SauChot()
	dang.w.Ghi(aistream.Delta, aistream.DeltaData{Text: "Tối "})
	time.Sleep(10 * time.Millisecond)
	dang.w.Ghi(aistream.Delta, aistream.DeltaData{Text: "nay "})
	time.Sleep(10 * time.Millisecond)
	k := p.ketNoi(t, 1, true, false)
	k.trang(t)
	var got []string
	for {
		f, ok := k.khung(t, 500*time.Millisecond)
		if !ok {
			break
		}
		if f["inv"] != dang.inv {
			t.Fatalf("the ended answer was replayed: %v", f)
		}
		d, _ := json.Marshal(f["d"])
		got = append(got, fmt.Sprint(f["e"])+string(d))
	}
	if strings.Join(got, " ") != `trang_thai{"cau":"dang_doc"} delta{"p":0,"text":"Tối nay "}` {
		t.Fatalf("replay %v", got)
	}
}

// Fan-out over real sockets: 50 members of one room (the feed's 5 a person,
// so 10 people) follow 5 answers written at once; each member gets every
// delta once, in order, and the XADD-to-socket latency is logged.
func TestPhongAiFanOutQuaSocket(t *testing.T) {
	p := moPhongThu(t)
	var people []int
	for i := 0; i < 10; i++ {
		person, token := id(), "synthetic-"+id()
		exec(t, p.pool, `INSERT INTO people(id,display_name) VALUES($1,'Synthetic onlooker')`, person)
		exec(t, p.pool, `INSERT INTO account_sessions(id,person_id,token_digest,issued_via,expires_at) VALUES($1,$2,$3,'genesis',now()+interval '1 day')`, id(), person, auth.TokenDigest(token))
		exec(t, p.pool, `INSERT INTO memberships(id,context_id,person_id,state,role,origin) VALUES($1,$2,$3,'active','member','named')`, id(), p.room, person)
		p.people, p.tokens = append(p.people, person), append(p.tokens, token)
		people = append(people, len(p.people)-1)
	}
	var conns []*ketNoi
	for _, who := range people {
		for i := 0; i < 5; i++ {
			k := p.ketNoi(t, who, true, false)
			k.trang(t)
			conns = append(conns, k)
		}
	}
	time.Sleep(200 * time.Millisecond)
	const loi, moiLoi = 5, 10
	key, _ := p.stream.Keys.Room(p.room)
	var wg sync.WaitGroup
	for j := 0; j < loi; j++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			inv := id()
			for q := 0; q < moiLoi; q++ {
				text := fmt.Sprintf("%d/%d/%d;", j, q, time.Now().UnixNano())
				if _, err := p.stream.AppendBatch(bg, key, aistream.MaxLenRoom, inv, []aistream.Entry{{Kind: aistream.Delta, Data: aistream.DeltaData{Text: text}}}, time.Now().Add(time.Minute)); err != nil {
					t.Error(err)
				}
				time.Sleep(60 * time.Millisecond)
			}
		}()
	}
	wg.Wait()
	var tre []time.Duration
	for i, k := range conns {
		next := map[string]int{}
		n := 0
		for n < loi*moiLoi {
			f, ok := k.khungLuc(3 * time.Second)
			if !ok {
				t.Fatalf("socket %d got %d of %d deltas", i, n, loi*moiLoi)
			}
			now := f.at
			d, _ := f.f["d"].(map[string]any)
			for _, part := range strings.Split(strings.TrimSuffix(fmt.Sprint(d["text"]), ";"), ";") {
				var j, q int
				var sent int64
				if _, err := fmt.Sscanf(part, "%d/%d/%d", &j, &q, &sent); err != nil {
					t.Fatalf("socket %d: %q", i, part)
				}
				if q != next[fmt.Sprint(j)] {
					t.Fatalf("socket %d: answer %d delta %d out of order", i, j, q)
				}
				next[fmt.Sprint(j)] = q + 1
				n++
				tre = append(tre, now.Sub(time.Unix(0, sent)))
			}
		}
	}
	p95 := phanVi(tre, 95)
	t.Logf("WebSocket fan-out: %d sockets x %d answers x %d deltas, latency p50 %v p95 %v 0 lost, 0 duplicated", len(conns), loi, moiLoi, phanVi(tre, 50), p95)
}

func phanVi(v []time.Duration, p int) time.Duration {
	s := append([]time.Duration(nil), v...)
	sort.Slice(s, func(a, b int) bool { return s[a] < s[b] })
	return s[len(s)*p/100].Round(time.Microsecond)
}

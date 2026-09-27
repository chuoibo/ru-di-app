//go:build broker

package aistream

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// langNghe runs the wake subscription for s into a fresh hub.
func langNghe(t *testing.T, s *Stream) *Hub {
	t.Helper()
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	ready := make(chan struct{})
	go s.Listen(ctx, hub, ready)
	select {
	case <-ready:
	case <-time.After(5 * time.Second):
		t.Fatal("the wake subscription never came up")
	}
	return hub
}

// thuKhung follows room with TheoPhong and hands every frame to a channel.
func thuKhung(t *testing.T, s *Stream, hub *Hub, room string, opt TheoPhongOptions) (<-chan KhungPhong, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	out := make(chan KhungPhong, 4096)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = s.TheoPhong(ctx, hub, room, opt, func(k KhungPhong) error {
			out <- k
			return nil
		})
	}()
	t.Cleanup(func() { cancel(); <-done })
	return out, cancel
}

func cho(t *testing.T, ch <-chan KhungPhong, within time.Duration) (KhungPhong, bool) {
	t.Helper()
	select {
	case k := <-ch:
		return k, true
	case <-time.After(within):
		return KhungPhong{}, false
	}
}

// A member joining (or reconnecting) is replayed only the invocations still
// running, each with its text so far merged into one delta per part and the
// envelope's trigger and count; the ended one (its card is in the feed) is
// left out entirely. Then the member follows live, in order, woken by the hub.
func TestTheoPhongPhatLaiRoiTheoSong(t *testing.T) {
	s := open(t)
	hub := langNghe(t, s)
	ctx := context.Background()
	phong := "0b8f1c9e-aaaa-4bbb-8ccc-ddddddddddc1"
	room, _ := s.Keys.Room(phong)
	xong, chay := "0b8f1c9e-aaaa-4bbb-8ccc-0000000000a1", "0b8f1c9e-aaaa-4bbb-8ccc-0000000000b1"
	tin := "0b8f1c9e-aaaa-4bbb-8ccc-0000000000c1"
	until := time.Now().Add(10 * time.Minute)
	put := func(n Nhan, kind Kind, data any) {
		if _, err := s.appendBatch(ctx, room, MaxLenRoom, n, []Entry{{kind, data}}, until); err != nil {
			t.Fatal(err)
		}
	}
	a := Nhan{Inv: xong}
	b := Nhan{Inv: chay, Tin: tin, SoTin: 6}
	put(a, TrangThai, TrangThaiData{Cau: "dang_doc"})
	put(b, TrangThai, TrangThaiData{Cau: "dang_doc"})
	put(a, Delta, DeltaData{Text: "Câu đã xong "})
	put(b, Delta, DeltaData{Text: "Tối "})
	put(b, Delta, DeltaData{Text: "nay "})
	put(a, Xong, map[string]string{"message_id": "0b8f1c9e-aaaa-4bbb-8ccc-0000000000d1"})
	frames, _ := thuKhung(t, s, hub, phong, TheoPhongOptions{Reconcile: time.Hour, Batch: 2})
	first, ok := cho(t, frames, 3*time.Second)
	if !ok || first.E != TrangThai || first.Inv != chay || first.Tin != tin || first.SoTin != 6 {
		t.Fatalf("first replayed frame %+v %v", first, ok)
	}
	second, ok := cho(t, frames, time.Second)
	if !ok || second.E != Delta || string(second.D) != `{"p":0,"text":"Tối nay "}` {
		t.Fatalf("second replayed frame %+v %s", second, second.D)
	}
	if k, ok := cho(t, frames, 200*time.Millisecond); ok {
		t.Fatalf("the replay carried more: %+v %s", k, k.D)
	}
	put(b, Delta, DeltaData{Text: "đi đâu?"})
	put(b, Xong, map[string]string{"message_id": "0b8f1c9e-aaaa-4bbb-8ccc-0000000000d2"})
	live, ok := cho(t, frames, 3*time.Second)
	if !ok || live.E != Delta || string(live.D) != `{"p":0,"text":"đi đâu?"}` {
		t.Fatalf("live delta %+v %s (only the hub can wake it: reconcile is an hour)", live, live.D)
	}
	end, ok := cho(t, frames, time.Second)
	if !ok || end.E != Xong || string(end.D) != `{"message_id":"0b8f1c9e-aaaa-4bbb-8ccc-0000000000d2"}` {
		t.Fatalf("live ending %+v", end)
	}
	if !idSau(end.ID, live.ID) || !idSau(live.ID, second.ID) {
		t.Fatalf("ids out of order: %s %s %s", second.ID, live.ID, end.ID)
	}
}

func idSau(a, b string) bool {
	pa, pb := strings.SplitN(a, "-", 2), strings.SplitN(b, "-", 2)
	ma, _ := strconv.ParseInt(pa[0], 10, 64)
	mb, _ := strconv.ParseInt(pb[0], 10, 64)
	if ma != mb {
		return ma > mb
	}
	sa, _ := strconv.ParseInt(pa[1], 10, 64)
	sb, _ := strconv.ParseInt(pb[1], 10, 64)
	return sa > sb
}

// An entry outside the room vocabulary, even one written raw to the room key
// by something other than the writer, never reaches a member.
func TestTheoPhongBoQuaEntryNgoaiEnum(t *testing.T) {
	s := open(t)
	hub := langNghe(t, s)
	ctx := context.Background()
	phong := "0b8f1c9e-aaaa-4bbb-8ccc-ddddddddddc2"
	room, _ := s.Keys.Room(phong)
	inv := "0b8f1c9e-aaaa-4bbb-8ccc-0000000000a2"
	frames, _ := thuKhung(t, s, hub, phong, TheoPhongOptions{Reconcile: 20 * time.Millisecond})
	for _, v := range [][]any{
		{"e", "hello", "j", `{"nhip_ms":1}`, "inv", inv},
		{"e", "thu_hoi", "j", `{}`, "inv", inv},
		{"e", "ket_noi_lai", "j", `{"sau_ms":0}`, "inv", inv},
		{"e", "retract", "j", `{}`, "inv", inv},
		{"e", "delta", "j", `not json`, "inv", inv},
		{"e", "delta", "j", `{"p":0,"text":"không inv"}`},
		{"e", "xong", "j", `{"text":"câu Nếp","chips":[]}`, "inv", inv},
		{"e", "delta", "j", `{"p":0,"text":"hợp lệ"}`, "inv", inv},
	} {
		if err := s.client.XAdd(ctx, &redis.XAddArgs{Stream: room, Values: v}).Err(); err != nil {
			t.Fatal(err)
		}
	}
	k, ok := cho(t, frames, 3*time.Second)
	if !ok || k.E != Delta || string(k.D) != `{"p":0,"text":"hợp lệ"}` {
		t.Fatalf("got %+v %s, want only the valid delta", k, k.D)
	}
	if k, ok := cho(t, frames, 200*time.Millisecond); ok {
		t.Fatalf("more frames: %+v %s", k, k.D)
	}
}

// A room key's writer holds every piece of text back until the job's ending
// committed (SauChot): before it a delta is refused and changes nothing
// (SauChot still opens the stream), statuses go at once, and every entry
// names the trigger and the count.
func TestWriterPhongChoSauChot(t *testing.T) {
	s := open(t)
	ctx := context.Background()
	room, _ := s.Keys.Room("0b8f1c9e-aaaa-4bbb-8ccc-ddddddddddc3")
	tin := "0b8f1c9e-aaaa-4bbb-8ccc-0000000000c3"
	marked := 0
	w := s.NewWriter(WriterOptions{Key: room, MaxLen: MaxLenRoom, Inv: "0b8f1c9e-aaaa-4bbb-8ccc-0000000000a3", Tin: tin, SoTin: 4,
		ExpireAt: time.Now().Add(10 * time.Minute), SauChotThoi: true, BeforeContent: func() error { marked++; return nil }})
	if !w.Ghi(TrangThai, TrangThaiData{Cau: "dang_doc"}) {
		t.Fatal("a status was refused")
	}
	if w.Ghi(Delta, DeltaData{Text: "trước khi đăng"}) || w.Ghi(Phan, map[string]any{"kind": "text", "json": map[string]string{}}) || w.CoNoiDung() {
		t.Fatal("content went before the commit")
	}
	if !w.SauChot() || !w.Ghi(Delta, DeltaData{Text: "sau khi đăng"}) {
		t.Fatal("content after the commit was refused")
	}
	w.Ghi(Xong, map[string]string{"message_id": "0b8f1c9e-aaaa-4bbb-8ccc-0000000000d3"})
	w.Dong()
	msgs, err := s.client.XRange(ctx, room, "-", "+").Result()
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, m := range msgs {
		got = append(got, fmt.Sprint(m.Values["e"]))
		if m.Values["tin"] != tin || m.Values["so"] != "4" {
			t.Fatalf("entry without its envelope: %v", m.Values)
		}
		if strings.Contains(fmt.Sprint(m.Values["j"]), "trước khi đăng") {
			t.Fatal("the refused text is in Redis")
		}
	}
	if strings.Join(got, ",") != "trang_thai,delta,xong" || marked != 0 {
		t.Fatalf("entries %v, BeforeContent ran %d times (a room job marks nothing: its text goes after the commit)", got, marked)
	}
}

// Fan-out (design 02 §9): 100 members of one room follow 20 answers written
// at once, each 10 deltas 60 ms apart (the writer's flush). Every member gets
// every delta exactly once and in order per answer; the time from XADD to the
// member's frame is measured and its p95 logged for the commit message. The
// bound asserted is loose (the tier runs under -race); the target is 200 ms.
func TestTheoPhongFanOut100x20(t *testing.T) {
	s := open(t)
	hub := langNghe(t, s)
	ctx := context.Background()
	phong := "0b8f1c9e-aaaa-4bbb-8ccc-ddddddddddc4"
	room, _ := s.Keys.Room(phong)
	const nguoi, loi, moiLoi = 100, 20, 10
	type nhan struct {
		mu    sync.Mutex
		tre   []time.Duration
		theo  map[string]int
		loiTT int
	}
	all := make([]*nhan, nguoi)
	var wg sync.WaitGroup
	ctxNghe, stop := context.WithCancel(ctx)
	defer stop()
	for i := range all {
		n := &nhan{theo: map[string]int{}}
		all[i] = n
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = s.TheoPhong(ctxNghe, hub, phong, DefaultTheoPhong(), func(k KhungPhong) error {
				now := time.Now()
				var d DeltaData
				if k.E != Delta || json.Unmarshal(k.D, &d) != nil {
					return nil
				}
				// text is "<answer>/<seq>/<unix nanos>;"; merged deltas (a
				// member far behind) carry several.
				n.mu.Lock()
				defer n.mu.Unlock()
				for _, part := range strings.Split(strings.TrimSuffix(d.Text, ";"), ";") {
					f := strings.Split(part, "/")
					if len(f) != 3 {
						n.loiTT++
						continue
					}
					seq, _ := strconv.Atoi(f[1])
					sent, _ := strconv.ParseInt(f[2], 10, 64)
					if seq != n.theo[f[0]] {
						n.loiTT++
					}
					n.theo[f[0]] = seq + 1
					n.tre = append(n.tre, now.Sub(time.Unix(0, sent)))
				}
				return nil
			})
		}()
	}
	time.Sleep(300 * time.Millisecond) // every member joined (the replay is empty)
	var writers sync.WaitGroup
	for j := 0; j < loi; j++ {
		writers.Add(1)
		go func() {
			defer writers.Done()
			inv := fmt.Sprintf("0b8f1c9e-aaaa-4bbb-8ccc-%012d", j)
			for q := 0; q < moiLoi; q++ {
				text := fmt.Sprintf("%d/%d/%d;", j, q, time.Now().UnixNano())
				if _, err := s.appendBatch(ctx, room, MaxLenRoom, Nhan{Inv: inv}, []Entry{{Delta, DeltaData{Text: text}}}, time.Now().Add(time.Minute)); err != nil {
					t.Error(err)
					return
				}
				time.Sleep(60 * time.Millisecond)
			}
		}()
	}
	writers.Wait()
	deadline := time.Now().Add(10 * time.Second)
	for {
		done := 0
		for _, n := range all {
			n.mu.Lock()
			c := len(n.tre)
			n.mu.Unlock()
			if c >= loi*moiLoi {
				done++
			}
		}
		if done == nguoi || time.Now().After(deadline) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	stop()
	wg.Wait()
	var tre []time.Duration
	for i, n := range all {
		if len(n.tre) != loi*moiLoi || n.loiTT != 0 {
			t.Fatalf("member %d got %d deltas (want %d), %d out of order or malformed", i, len(n.tre), loi*moiLoi, n.loiTT)
		}
		tre = append(tre, n.tre...)
	}
	sort.Slice(tre, func(a, b int) bool { return tre[a] < tre[b] })
	p50, p95, p99 := tre[len(tre)/2], tre[len(tre)*95/100], tre[len(tre)*99/100]
	t.Logf("fan-out %d members x %d answers x %d deltas: %d frames, latency p50 %v p95 %v p99 %v max %v, 0 lost, 0 duplicated",
		nguoi, loi, moiLoi, len(tre), p50.Round(time.Microsecond), p95.Round(time.Microsecond), p99.Round(time.Microsecond), tre[len(tre)-1].Round(time.Microsecond))
	if p95 > time.Second {
		t.Fatalf("p95 %v", p95)
	}
}

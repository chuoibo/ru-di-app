//go:build broker

package aistream

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// A room key interleaves invocations; a requester follows only theirs, and
// another invocation's ending does not end their stream.
func TestRoomKeyFollowsOneInvocation(t *testing.T) {
	s := open(t)
	ctx := context.Background()
	room, _ := s.Keys.Room("0b8f1c9e-aaaa-4bbb-8ccc-ddddddddddb1")
	a, b := "0b8f1c9e-aaaa-4bbb-8ccc-00000000000a", "0b8f1c9e-aaaa-4bbb-8ccc-00000000000b"
	until := time.Now().Add(10 * time.Minute)
	put := func(inv string, kind Kind, data any) string {
		ids, err := s.AppendBatch(ctx, room, MaxLenRoom, inv, []Entry{{kind, data}}, until)
		if err != nil {
			t.Fatal(err)
		}
		return ids[0]
	}
	put(a, Delta, DeltaData{Text: "A1"})
	put(b, Delta, DeltaData{Text: "B1"})
	endB := put(b, Xong, map[string]string{"message_id": "mb"})
	put(a, Delta, DeltaData{Text: "A2"})
	put(a, Xong, map[string]string{"message_id": "ma"})
	if ended, _ := s.EndedAt(ctx, room, endB, a); ended {
		t.Fatal("B's ending read as A's")
	}
	if ended, _ := s.EndedAt(ctx, room, endB, b); !ended {
		t.Fatal("B's ending not read as B's")
	}
	rec := httptest.NewRecorder()
	opt := DefaultFollow()
	opt.Inv = a
	c, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := Follow(c, rec, s, NewHub(), room, "", opt); err != nil {
		t.Fatal(err)
	}
	body := rec.Body.String()
	if strings.Contains(body, "B1") || strings.Contains(body, `"mb"`) || !strings.Contains(body, "A1") || !strings.Contains(body, "A2") || !strings.Contains(body, `"ma"`) {
		t.Fatalf("A's reader saw %q", body)
	}
	if strings.Contains(body, `"inv"`) || strings.Contains(body, b) {
		t.Fatalf("the invocation field reached the wire: %q", body)
	}
	if ttl := s.client.TTL(ctx, room).Val(); ttl <= AfterTerminal || ttl > RoomWindow {
		t.Fatalf("room TTL %v, want the sharing window from its last write", ttl)
	}
}

// The body opens with retry then hello; a reader with nothing to read yet
// hears Empty's events, with no id; a stopping process says ket_noi_lai.
func TestFollowOpensAndStops(t *testing.T) {
	s := open(t)
	key, _ := s.Keys.Invocation("0b8f1c9e-aaaa-4bbb-8ccc-ddddddddddc1")
	opt := DefaultFollow()
	opt.Empty = func() ([]Event, bool) {
		raw, _ := json.Marshal(TrangThaiData{Cau: "dang_xep_hang"})
		return []Event{{ID: "1-1", Kind: TrangThai, Data: raw}}, false
	}
	stop := make(chan struct{})
	opt.Stop = stop
	go func() { time.Sleep(150 * time.Millisecond); close(stop) }()
	rec := httptest.NewRecorder()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := Follow(ctx, rec, s, NewHub(), key, "", opt); err != nil {
		t.Fatal(err)
	}
	want := "retry: 2000\n\nevent: hello\ndata: {\"nhip_ms\":15000}\n\nevent: trang_thai\ndata: {\"cau\":\"dang_xep_hang\"}\n\nevent: ket_noi_lai\ndata: {\"sau_ms\":0}\n\n"
	if rec.Body.String() != want {
		t.Fatalf("body %q\nwant %q", rec.Body.String(), want)
	}
}

// The writer merges the deltas of one part while they wait, keeps order,
// and writes a status or an ending at once.
func TestWriterMergesDeltasAndKeepsOrder(t *testing.T) {
	s := open(t)
	ctx := context.Background()
	key, _ := s.Keys.Invocation("0b8f1c9e-aaaa-4bbb-8ccc-ddddddddddc2")
	var before atomic.Int32
	w := s.NewWriter(WriterOptions{Key: key, MaxLen: MaxLenInvocation, ExpireAt: time.Now().Add(time.Minute),
		BeforeContent: func() error { before.Add(1); return nil }})
	w.Ghi(TrangThai, TrangThaiData{Cau: "dang_doc"})
	if n := s.client.XLen(ctx, key).Val(); n != 1 {
		t.Fatalf("a status waited: %d entries", n)
	}
	var want strings.Builder
	for i := 0; i < 100; i++ {
		w.Ghi(Delta, DeltaData{P: 0, Text: "ab"})
		want.WriteString("ab")
	}
	w.Ghi(Xong, map[string]string{"message_id": "m"})
	if w.Ghi(Delta, DeltaData{Text: "late"}) {
		t.Fatal("a delta after the ending was taken")
	}
	events, err := s.Read(ctx, key, "", 1000)
	if err != nil {
		t.Fatal(err)
	}
	var got strings.Builder
	deltas := 0
	for i, e := range events {
		if e.Kind == Delta {
			var d DeltaData
			_ = json.Unmarshal(e.Data, &d)
			got.WriteString(d.Text)
			deltas++
		}
		if e.Kind == Xong && i != len(events)-1 {
			t.Fatal("the ending is not last")
		}
	}
	if got.String() != want.String() || deltas > 2 || events[0].Kind != TrangThai {
		t.Fatalf("%d deltas carrying %d bytes (want %d in at most 2)", deltas, got.Len(), want.Len())
	}
	if before.Load() != 1 {
		t.Fatalf("BeforeContent ran %d times", before.Load())
	}
}

// BeforeContent refusing drops every content event, not the statuses or the
// ending; a restart after content is refused.
func TestWriterContentGateAndRestart(t *testing.T) {
	s := open(t)
	ctx := context.Background()
	key, _ := s.Keys.Invocation("0b8f1c9e-aaaa-4bbb-8ccc-ddddddddddc3")
	w := s.NewWriter(WriterOptions{Key: key, MaxLen: MaxLenInvocation, ExpireAt: time.Now().Add(time.Minute),
		BeforeContent: func() error { return errors.New("lease gone") }})
	w.Ghi(TrangThai, TrangThaiData{Cau: "dang_doc"})
	if w.Ghi(Delta, DeltaData{Text: "secret"}) || w.CoNoiDung() {
		t.Fatal("content went out without BeforeContent")
	}
	w.Ghi(ThatBai, ThatBaiData{Code: "x"})
	events, _ := s.Read(ctx, key, "", 100)
	if len(events) != 2 || events[1].Kind != ThatBai {
		t.Fatalf("events %+v", events)
	}
	key2, _ := s.Keys.Invocation("0b8f1c9e-aaaa-4bbb-8ccc-ddddddddddc4")
	w2 := s.NewWriter(WriterOptions{Key: key2, MaxLen: MaxLenInvocation, ExpireAt: time.Now().Add(time.Minute)})
	if !w2.Ghi(LamLai, struct{}{}) {
		t.Fatal("a restart before content was refused")
	}
	w2.Ghi(Delta, DeltaData{Text: "x"})
	if w2.Ghi(LamLai, struct{}{}) {
		t.Fatal("a restart after content was taken")
	}
	w2.Dong()
}

// Two failed writes in a row and the writer gives up: the job goes on, the
// writer costs nothing more.
func TestWriterGivesUpAfterTwoFailures(t *testing.T) {
	dead, err := Open("redis://127.0.0.1:1/0", "t")
	if err != nil {
		t.Fatal(err)
	}
	defer dead.Close()
	key, _ := dead.Keys.Invocation("0b8f1c9e-aaaa-4bbb-8ccc-ddddddddddc5")
	var marked atomic.Int32
	w := dead.NewWriter(WriterOptions{Key: key, MaxLen: MaxLenInvocation, ExpireAt: time.Now().Add(time.Minute),
		BeforeContent: func() error { marked.Add(1); return nil }})
	start := time.Now()
	w.Ghi(TrangThai, TrangThaiData{Cau: "dang_doc"})
	if w.Chet() {
		t.Fatal("gave up after one failure")
	}
	w.Ghi(TrangThai, TrangThaiData{Cau: "dang_nghi"})
	if !w.Chet() {
		t.Fatal("still writing after two failures")
	}
	for i := 0; i < 50; i++ {
		w.Ghi(TrangThai, TrangThaiData{Cau: "dang_doc"})
	}
	// Content a dead writer cannot carry marks nothing in the job's row.
	if w.Ghi(Delta, DeltaData{Text: "x"}) || marked.Load() != 0 {
		t.Fatalf("a dead writer took content (BeforeContent ran %d times)", marked.Load())
	}
	if took := time.Since(start); took > 2*time.Second {
		t.Fatalf("a dead writer cost %v", took)
	}
}

// The opening limit counts per person under a digest, and fails open.
func TestMoLuotLimit(t *testing.T) {
	s := open(t)
	ctx := context.Background()
	person := "0b8f1c9e-aaaa-4bbb-8ccc-ddddddddddd9"
	for i := 0; i < 30; i++ {
		if ok, err := s.MoLuot(ctx, person, 30); !ok || err != nil {
			t.Fatalf("opening %d refused: %v", i+1, err)
		}
	}
	if ok, _ := s.MoLuot(ctx, person, 30); ok {
		t.Fatal("the 31st opening in a minute was let through")
	}
	keys, _ := s.client.Keys(ctx, "rudi:*"+person+"*").Result()
	if len(keys) != 0 {
		t.Fatalf("a person id is in a Redis key: %v", keys)
	}
	if ttl := s.client.TTL(ctx, s.Keys.GioiHanMo(person)).Val(); ttl <= 0 || ttl > time.Minute {
		t.Fatalf("counter TTL %v", ttl)
	}
	// A counter left without a TTL (INCR landed, EXPIRE did not, before the
	// two ran in one script) gets its minute back on the next opening.
	other := "0b8f1c9e-aaaa-4bbb-8ccc-ddddddddddd8"
	s.client.Set(ctx, s.Keys.GioiHanMo(other), 40, 0)
	if ok, _ := s.MoLuot(ctx, other, 30); ok {
		t.Fatal("an opening over the limit was let through")
	}
	if ttl := s.client.TTL(ctx, s.Keys.GioiHanMo(other)).Val(); ttl <= 0 || ttl > time.Minute {
		t.Fatalf("a counter without a TTL kept none: %v", ttl)
	}
	s.client.Del(ctx, s.Keys.GioiHanMo(other))
	dead, _ := Open("redis://127.0.0.1:1/0", "t")
	defer dead.Close()
	if ok, err := dead.MoLuot(ctx, person, 30); !ok || err == nil {
		t.Fatal("a Redis that cannot answer refused the opening")
	}
}

package jobs

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// soAck records what a consumer did with each delivery, by delivery tag.
type soAck struct {
	mu   sync.Mutex
	what map[uint64][]string
}

func (s *soAck) note(tag uint64, what string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.what[tag] = append(s.what[tag], what)
	return nil
}

func (s *soAck) Ack(tag uint64, _ bool) error { return s.note(tag, "ack") }
func (s *soAck) Nack(tag uint64, _ bool, requeue bool) error {
	return s.note(tag, fmt.Sprintf("nack(requeue=%v)", requeue))
}
func (s *soAck) Reject(tag uint64, requeue bool) error {
	return s.note(tag, fmt.Sprintf("reject(requeue=%v)", requeue))
}

func (s *soAck) of(tag uint64) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return fmt.Sprint(s.what[tag])
}

func giao(t *testing.T, s *soAck, tag uint64, ref string) amqp.Delivery {
	t.Helper()
	m := Message{V: 1, Ref: ref, Seq: 1}
	body, err := m.Encode()
	if err != nil {
		t.Fatal(err)
	}
	return amqp.Delivery{Acknowledger: s, DeliveryTag: tag, MessageId: m.ID("ai.nep"), Body: body}
}

// A delivery the broker sent before the pause's cancel reached it, and that
// arrives once the pause began, is kept with the message the database failed
// under: not run while paused, not handed back to the queue (review of
// slice 10 round 3, finding 4: a Nack with requeue there, as the first
// version did, counts a delivery against x-delivery-limit on every pause).
// Once the pause is over both run, and each is acknowledged once.
//
// The deliveries come from a channel the test feeds, so the order is exact:
// B running, S failing under the database (the pause begins: stop is
// called), then V arriving, then B finishing and the subscription closing.
func TestPauseKeepsWhatArrivesAfterIt(t *testing.T) {
	const (
		refS = "0b8f1c9e-aaaa-4bbb-8ccc-dddddddd0a01"
		refB = "0b8f1c9e-aaaa-4bbb-8ccc-dddddddd0a02"
		refV = "0b8f1c9e-aaaa-4bbb-8ccc-dddddddd0a03"
	)
	acks := &soAck{what: map[uint64][]string{}}
	release := make(chan struct{})
	var mu sync.Mutex
	runs := map[string]int{}
	dbDown := true
	handler := func(_ context.Context, m Message) error {
		mu.Lock()
		runs[m.Ref]++
		down := dbDown
		mu.Unlock()
		switch m.Ref {
		case refB:
			<-release
		case refS:
			if down {
				return fmt.Errorf("claim: %w", ErrTamDung)
			}
		}
		return nil
	}
	c := consumer{queue: "ai.nep", handler: handler, concurrency: 2}
	deliveries := make(chan amqp.Delivery)
	paused := make(chan struct{})
	done := make(chan []amqp.Delivery, 1)
	go func() {
		done <- c.drain(context.Background(), deliveries, func() { close(paused) })
	}()
	deliveries <- giao(t, acks, 1, refB)
	deliveries <- giao(t, acks, 2, refS)
	select {
	case <-paused:
	case <-time.After(5 * time.Second):
		t.Fatal("the database failed under S, and the consumer never paused")
	}
	deliveries <- giao(t, acks, 3, refV)
	close(release)
	close(deliveries)
	var held []amqp.Delivery
	select {
	case held = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("drain never returned once the subscription closed")
	}
	if got := acks.of(3); got != "[]" {
		t.Fatalf("V, delivered once the pause began, was settled by the pause: %s -- a Nack with requeue counts a delivery against the limit", got)
	}
	mu.Lock()
	ranV := runs[refV]
	mu.Unlock()
	if ranV != 0 {
		t.Fatalf("V ran %d time(s) while the consumer was paused", ranV)
	}
	tags := map[uint64]bool{}
	for _, d := range held {
		tags[d.DeliveryTag] = true
	}
	if len(held) != 2 || !tags[2] || !tags[3] {
		t.Fatalf("kept %d deliveries %v, want S and V (tags 2 and 3)", len(held), tags)
	}
	if got := acks.of(1); got != "[ack]" {
		t.Fatalf("B, running when the pause began, ended with %s, want one ack", got)
	}
	if got := acks.of(2); got != "[]" {
		t.Fatalf("S, the message the database failed under, was settled by the pause: %s", got)
	}

	mu.Lock()
	dbDown = false
	mu.Unlock()
	if again := c.runAll(context.Background(), held); len(again) != 0 {
		t.Fatalf("%d kept message(s) failed again with the database back", len(again))
	}
	for tag, want := range map[uint64]string{1: "[ack]", 2: "[ack]", 3: "[ack]"} {
		if got := acks.of(tag); got != want {
			t.Errorf("delivery %d ended with %s, want %s", tag, got, want)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if runs[refV] != 1 || runs[refS] != 2 || runs[refB] != 1 {
		t.Fatalf("runs S=%d B=%d V=%d, want 2, 1, 1", runs[refS], runs[refB], runs[refV])
	}
}

// The wait before each run of a pause's kept messages: 250 ms, doubling, at
// most 30 s (review of slice 10 round 3, finding 2).
func TestChoTamDung(t *testing.T) {
	want := []time.Duration{250 * time.Millisecond, 500 * time.Millisecond, time.Second, 2 * time.Second, 4 * time.Second,
		8 * time.Second, 16 * time.Second, 30 * time.Second, 30 * time.Second}
	for retry, w := range want {
		if got := choTamDung(retry); got != w {
			t.Errorf("retry %d: %v, want %v", retry, got, w)
		}
	}
	if got := choTamDung(1000); got != 30*time.Second {
		t.Errorf("retry 1000: %v, want the 30 s ceiling", got)
	}
	// Five seconds of a database that fails every claim at once: the first
	// run and at most five more.
	var at time.Duration
	runs := 1
	for retry := 0; ; retry++ {
		at += choTamDung(retry)
		if at > 5*time.Second {
			break
		}
		runs++
	}
	if runs > 6 {
		t.Fatalf("%d runs in the first 5 s of a pause", runs)
	}
}

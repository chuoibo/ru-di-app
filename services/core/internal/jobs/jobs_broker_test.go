//go:build broker

package jobs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	amqp "github.com/rabbitmq/amqp091-go"
	"mobile/services/core/internal/testdb"
)

func dial(t *testing.T) *amqp.Connection {
	t.Helper()
	url := os.Getenv("CORE_TEST_AMQP_URL")
	if url == "" {
		if os.Getenv("CORE_REQUIRE_BROKER_TESTS") == "1" {
			t.Fatal("CORE_TEST_AMQP_URL is required")
		}
		t.Skip("CORE_TEST_AMQP_URL not set")
	}
	conn, err := amqp.Dial(url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// fixture: an isolated schema with the outbox, and a fresh broker namespace.
func fixture(t *testing.T) (*pgxpool.Pool, *amqp.Connection, Topology) {
	t.Helper()
	ctx := context.Background()
	base := testdb.Pool(t)
	schema := fmt.Sprintf("jobs_test_%d", time.Now().UnixNano())
	if _, err := base.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = base.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE") })
	cfg := base.Config().Copy()
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err = Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err = Migrate(ctx, pool); err != nil {
		t.Fatal("second migrate:", err)
	}
	conn := dial(t)
	top, _ := NewTopology(fmt.Sprintf("t%d", time.Now().UnixNano()%1_000_000_000))
	t.Cleanup(func() {
		c, err := amqp.Dial(os.Getenv("CORE_TEST_AMQP_URL"))
		if err != nil {
			return
		}
		defer c.Close()
		ch, err := c.Channel()
		if err != nil {
			return
		}
		for _, q := range Queues {
			_, _ = ch.QueueDelete(top.Queue(q), false, false, false)
			_, _ = ch.QueueDelete(top.DeadQueue(q), false, false, false)
		}
		_ = ch.ExchangeDelete(top.Exchange(), false, false)
		_ = ch.ExchangeDelete(top.DeadExchange(), false, false)
	})
	return pool, conn, top
}

func enqueue(t *testing.T, q pgx.Tx, queue, ref string, seq int64, expires any) {
	t.Helper()
	if _, err := q.Exec(context.Background(), `SELECT jobs_them($1,$2,$3,clock_timestamp(),$4)`, queue, ref, seq, expires); err != nil {
		t.Fatal(err)
	}
}

func ref(i int) string { return fmt.Sprintf("0b8f1c9e-aaaa-4bbb-8ccc-dddddddd%04x", i) }

func unpublished(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM job_outbox WHERE published_at IS NULL`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// collect consumes queue until it has n messages or the deadline passes.
func collect(t *testing.T, conn *amqp.Connection, top Topology, queue string, n int, handler Handler) []Message {
	t.Helper()
	return collectHooks(t, conn, top, queue, n, handler, Hooks{})
}

// collectHooks is collect with the consumer's hooks.
func collectHooks(t *testing.T, conn *amqp.Connection, top Topology, queue string, n int, handler Handler, hooks Hooks) []Message {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var mu sync.Mutex
	var got []Message
	go func() {
		_ = ConsumeReady(ctx, conn, top, queue, 4, func(ctx context.Context, m Message) error {
			if handler != nil {
				if err := handler(ctx, m); err != nil {
					return err
				}
			}
			mu.Lock()
			got = append(got, m)
			if len(got) >= n {
				cancel()
			}
			mu.Unlock()
			return nil
		}, hooks)
	}()
	<-ctx.Done()
	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	return append([]Message(nil), got...)
}

func TestBrokerTierReachesRabbitAndPostgres(t *testing.T) {
	pool, conn, _ := fixture(t)
	if conn.IsClosed() {
		t.Fatal("amqp connection closed")
	}
	if err := pool.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestRelayPublishesOnlyCommittedRows(t *testing.T) {
	pool, conn, top := fixture(t)
	ctx := context.Background()
	committed, _ := pool.Begin(ctx)
	enqueue(t, committed, "ai.nep", ref(1), 0, nil)
	enqueue(t, committed, "ai.nep", ref(2), 0, nil)
	if err := committed.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	open, _ := pool.Begin(ctx)
	enqueue(t, open, "ai.nep", ref(3), 0, nil)
	relay, err := NewRelay(pool, conn, top)
	if err != nil {
		t.Fatal(err)
	}
	defer relay.Close()
	if n, err := relay.Flush(ctx); err != nil || n != 2 {
		t.Fatalf("flush %d %v, want 2 committed rows", n, err)
	}
	got := collect(t, conn, top, "ai.nep", 2, nil)
	if len(got) != 2 {
		t.Fatalf("consumed %d, want 2", len(got))
	}
	if err = open.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if n, err := relay.Flush(ctx); err != nil || n != 1 {
		t.Fatalf("second flush %d %v", n, err)
	}
	// The same enqueue twice is one row: a retrying trigger cannot double a job.
	again, _ := pool.Begin(ctx)
	enqueue(t, again, "ai.nep", ref(1), 0, nil)
	_ = again.Commit(ctx)
	if n, _ := relay.Flush(ctx); n != 0 {
		t.Fatalf("a repeated enqueue was published again: %d", n)
	}
}

func TestExpiredRowsAreDroppedNotPublished(t *testing.T) {
	pool, conn, top := fixture(t)
	ctx := context.Background()
	tx, _ := pool.Begin(ctx)
	enqueue(t, tx, "ai.group", ref(4), 0, time.Now().Add(-time.Second))
	_ = tx.Commit(ctx)
	relay, err := NewRelay(pool, conn, top)
	if err != nil {
		t.Fatal(err)
	}
	defer relay.Close()
	if n, err := relay.Flush(ctx); err != nil || n != 0 {
		t.Fatalf("an expired job was published: %d %v", n, err)
	}
	var rows int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM job_outbox`).Scan(&rows)
	if rows != 0 {
		t.Fatalf("expired row kept: %d", rows)
	}
}

// Losing the broker delays jobs and loses none: nothing is marked published
// until a broker confirms it.
func TestBrokerOutageLosesNoJob(t *testing.T) {
	pool, conn, top := fixture(t)
	ctx := context.Background()
	relay, err := NewRelay(pool, conn, top)
	if err != nil {
		t.Fatal(err)
	}
	tx, _ := pool.Begin(ctx)
	for i := 0; i < 50; i++ {
		enqueue(t, tx, "ai.group", ref(100+i), 0, nil)
	}
	_ = tx.Commit(ctx)
	_ = conn.Close() // the broker goes away under the relay
	if n, err := relay.Flush(ctx); err == nil || n != 0 {
		t.Fatalf("flush without a broker: %d %v", n, err)
	}
	if u := unpublished(t, pool); u != 50 {
		t.Fatalf("%d rows unpublished after the outage, want all 50", u)
	}
	conn2 := dial(t)
	relay2, err := NewRelay(pool, conn2, top)
	if err != nil {
		t.Fatal(err)
	}
	defer relay2.Close()
	if n, err := relay2.Flush(ctx); err != nil || n != 50 {
		t.Fatalf("after reconnect %d %v", n, err)
	}
	if got := collect(t, conn2, top, "ai.group", 50, nil); len(got) != 50 {
		t.Fatalf("consumed %d, want 50", len(got))
	}
}

func TestMalformedMessageIsDeadLettered(t *testing.T) {
	_, conn, top := fixture(t)
	ch, _ := conn.Channel()
	defer ch.Close()
	if err := top.Declare(ch); err != nil {
		t.Fatal(err)
	}
	// Three poison bodies: no message id, an id of the relay's form, and an
	// id carrying text. The warning names the second by its id and the other
	// two by nothing: an id the relay did not write is not repeated.
	relayForm := "memory:" + ref(9) + ":1"
	for _, id := range []string{"", relayForm, "memory:leak words"} {
		if err := ch.PublishWithContext(context.Background(), top.Exchange(), "memory", false, false,
			amqp.Publishing{MessageId: id, Body: []byte(`{"v":1,"ref":"x","prompt":"leak"}`)}); err != nil {
			t.Fatal(err)
		}
	}
	var called atomic.Int64
	var mu sync.Mutex
	var dead []string
	collectHooks(t, conn, top, "memory", 1, func(context.Context, Message) error { called.Add(1); return nil }, Hooks{
		DeadLettered: func(id string) { mu.Lock(); dead = append(dead, id); mu.Unlock() },
	})
	if called.Load() != 0 {
		t.Fatal("a malformed body reached the handler")
	}
	waitDead(t, ch, top, "memory", 3)
	mu.Lock()
	defer mu.Unlock()
	// Handled concurrently, so in any order.
	sort.Strings(dead)
	if fmt.Sprint(dead) != fmt.Sprint([]string{"", "", relayForm}) {
		t.Fatalf("dead-letter warnings named %q, want two \"\" and %q", dead, relayForm)
	}
}

func TestFailingJobIsRedeliveredThenDeadLettered(t *testing.T) {
	pool, conn, top := fixture(t)
	ctx := context.Background()
	tx, _ := pool.Begin(ctx)
	enqueue(t, tx, "notify", ref(7), 0, nil)
	_ = tx.Commit(ctx)
	relay, err := NewRelay(pool, conn, top)
	if err != nil {
		t.Fatal(err)
	}
	defer relay.Close()
	if n, err := relay.Flush(ctx); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	var calls atomic.Int64
	var mu sync.Mutex
	var dead []string
	collectHooks(t, conn, top, "notify", 99, func(context.Context, Message) error {
		calls.Add(1)
		return errors.New("transient")
	}, Hooks{DeadLettered: func(id string) { mu.Lock(); dead = append(dead, id); mu.Unlock() }})
	ch, _ := conn.Channel()
	defer ch.Close()
	waitDead(t, ch, top, "notify", 1)
	if c := calls.Load(); c < DeliveryLimit || c > DeliveryLimit+1 {
		t.Fatalf("handler ran %d times, want about the delivery limit %d", c, DeliveryLimit)
	}
	// Design 02 §7: one warning line for the message that went to the
	// dead-letter queue, by id, at the return that sent it there -- not one
	// per failed delivery.
	mu.Lock()
	defer mu.Unlock()
	if want := "notify:" + ref(7) + ":0"; len(dead) != 1 || dead[0] != want {
		t.Fatalf("dead-letter warnings %q after %d deliveries, want exactly [%q]", dead, calls.Load(), want)
	}
}

// A consumer paused by the database gives nothing back to the queue: the
// message the database failed under and the ones the broker had already
// delivered stay with it, unacknowledged, and run once the database answers.
// Six pauses in a row on one message -- more than DeliveryLimit -- still end
// with every message done once and nothing dead-lettered. (Requeueing on
// each pause, as slice 10 first did, dead-letters that message at the sixth:
// on RabbitMQ 3.12 every return counts toward x-delivery-limit.)
func TestPauseGivesNothingBackToTheQueue(t *testing.T) {
	pool, conn, top := fixture(t)
	ctx := context.Background()
	tx, _ := pool.Begin(ctx)
	for i := range 4 {
		enqueue(t, tx, "ai.nep", ref(40+i), 1, nil)
	}
	_ = tx.Commit(ctx)
	relay, err := NewRelay(pool, conn, top)
	if err != nil {
		t.Fatal(err)
	}
	defer relay.Close()
	if n, err := relay.Flush(ctx); err != nil || n != 4 {
		t.Fatal(n, err)
	}
	const pauses = DeliveryLimit + 1
	stubborn := ref(40)
	var mu sync.Mutex
	runs := map[string]int{}
	var paused atomic.Int64
	var dead atomic.Int64
	got := collectHooks(t, conn, top, "ai.nep", 4, func(_ context.Context, m Message) error {
		mu.Lock()
		runs[m.Ref]++
		n := runs[m.Ref]
		mu.Unlock()
		if m.Ref == stubborn && n <= pauses {
			return fmt.Errorf("claim: %w", ErrTamDung)
		}
		return nil
	}, Hooks{
		Paused: func(ctx context.Context) {
			paused.Add(1)
			select {
			case <-ctx.Done():
			case <-time.After(20 * time.Millisecond):
			}
		},
		DeadLettered: func(string) { dead.Add(1) },
	})
	mu.Lock()
	defer mu.Unlock()
	if len(got) != 4 || runs[stubborn] != pauses+1 || paused.Load() != pauses || dead.Load() != 0 {
		t.Fatalf("done %d/4, the stubborn message ran %d times (want %d), %d pauses (want %d), %d dead-lettered", len(got), runs[stubborn], pauses+1, paused.Load(), pauses, dead.Load())
	}
	for i := 1; i < 4; i++ {
		if runs[ref(40+i)] != 1 {
			t.Fatalf("message %d ran %d times, want once: a pause gave it back or ran it twice", i, runs[ref(40+i)])
		}
	}
	ch, _ := conn.Channel()
	defer ch.Close()
	for _, q := range []string{top.Queue("ai.nep"), top.DeadQueue("ai.nep")} {
		args := amqp.Table{"x-queue-type": "quorum", "x-message-ttl": int64(7 * 24 * 3600 * 1000)}
		if q == top.Queue("ai.nep") {
			args = amqp.Table{"x-queue-type": "quorum", "x-delivery-limit": int64(DeliveryLimit), "x-dead-letter-exchange": top.DeadExchange(), "x-dead-letter-routing-key": "ai.nep"}
		}
		left, err := ch.QueueDeclarePassive(q, true, false, false, false, args)
		if err != nil || left.Messages != 0 {
			t.Fatalf("%s holds %d messages (%v), want 0", q, left.Messages, err)
		}
	}
}

// The relay's LISTEN is its own connection, not one of the pool's: on a pool
// of one, a statement still gets the connection while the relay listens,
// and a notification still wakes the relay long before its tick.
func TestRelayListensOnItsOwnConnection(t *testing.T) {
	pool, conn, top := fixture(t)
	cfg := pool.Config().Copy()
	cfg.MaxConns = 1
	app := fmt.Sprintf("relay_own_conn_%d", time.Now().UnixNano())
	cfg.ConnConfig.RuntimeParams["application_name"] = app
	small, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer small.Close()
	relay, err := NewRelay(small, conn, top)
	if err != nil {
		t.Fatal(err)
	}
	defer relay.Close()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- relay.Run(ctx, time.Minute) }()
	defer func() { cancel(); <-done }()
	deadline := time.Now().Add(5 * time.Second)
	for {
		var listening int
		if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM pg_stat_activity WHERE application_name=$1 AND query='LISTEN job_outbox'`, app).Scan(&listening); err != nil {
			t.Fatal(err)
		}
		if listening == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the relay never listened")
		}
		time.Sleep(20 * time.Millisecond)
	}
	acquire, stop := context.WithTimeout(context.Background(), 2*time.Second)
	c, err := small.Acquire(acquire)
	stop()
	if err != nil {
		t.Fatalf("the pool's one connection is held while the relay listens: %v", err)
	}
	c.Release()
	tx, _ := pool.Begin(context.Background())
	enqueue(t, tx, "ai.nep", ref(60), 1, nil)
	_ = tx.Commit(context.Background())
	began := time.Now()
	if got := collect(t, conn, top, "ai.nep", 1, nil); len(got) != 1 || got[0].Ref != ref(60) {
		t.Fatalf("the notification did not wake the relay: %v", got)
	}
	t.Logf("published %v after the commit, tick one minute", time.Since(began).Round(time.Millisecond))
}

func waitDead(t *testing.T, ch *amqp.Channel, top Topology, queue string, want int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		q, err := ch.QueueDeclarePassive(top.DeadQueue(queue), true, false, false, false,
			amqp.Table{"x-queue-type": "quorum", "x-message-ttl": int64(7 * 24 * 3600 * 1000)})
		if err == nil && q.Messages >= want {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("%s: dead-letter queue never received %d message(s)", strings.TrimSpace(queue), want)
}

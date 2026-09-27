//go:build broker

package nepnho

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"mobile/services/core/internal/aiharness/trinho"
	"mobile/services/core/internal/jobs"
	"mobile/services/core/internal/testdb"
)

// An account deletion rides the memory lane end to end: the trigger on
// people queues the deletion, its enqueue trigger writes the outbox row, the
// relay publishes it, the consumer hands it to nepnho, the first attempt
// fails (sidecar down), its retry is enqueued, published and consumed, and
// the second attempt writes the keyed receipt. RabbitMQ and PostgreSQL are
// real; the sidecar is the in-process fake.
func TestHangNhoXoaTaiKhoanQuaBroker(t *testing.T) {
	url := os.Getenv("CORE_TEST_AMQP_URL")
	if url == "" {
		if os.Getenv("CORE_REQUIRE_BROKER_TESTS") == "1" {
			t.Fatal("CORE_TEST_AMQP_URL is required")
		}
		t.Skip("CORE_TEST_AMQP_URL not set")
	}
	ctx := context.Background()
	pool := testdb.Pool(t)
	if err := jobs.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	gia := moiKhoGia()
	k, err := Moi(pool, gia, []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	// An hour behind: every backoff is already due when it is written.
	k.now = func() time.Time { return time.Now().Add(-time.Hour) }
	p := uuidMoi()
	if _, err := pool.Exec(ctx, `INSERT INTO people(id, display_name) VALUES($1, 'Người thử hàng memory')`, p); err != nil {
		t.Fatal(err)
	}
	if err := k.Bat(ctx, p, CongBoBan); err != nil {
		t.Fatal(err)
	}
	if _, err := k.Ghi(ctx, p, suThatMoiBroker("Thích bánh xèo")); err != nil {
		t.Fatal(err)
	}
	top, _ := jobs.NewTopology(fmt.Sprintf("nep%d", time.Now().UnixNano()%1_000_000_000))
	t.Cleanup(func() { xoaTopology(url, top) })
	gia.vang = true
	var mu sync.Mutex
	var chay []int64
	var viec string
	attempts := func() []int64 { mu.Lock(); defer mu.Unlock(); return append([]int64(nil), chay...) }
	handled := make(chan struct{}, 8)
	ket := &jobs.Ket{URL: url, Topology: top, Pool: pool, Queues: []string{HangNho}, Concurrency: 1, Tick: 100 * time.Millisecond,
		Logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})),
		Handler: func(ctx context.Context, queue string, m jobs.Message) error {
			if queue != HangNho {
				return jobs.ErrMalformed
			}
			mu.Lock()
			mine := m.Ref == viec
			mu.Unlock()
			if !mine {
				// Another deletion's message left in the shared outbox by an
				// earlier run: this test's sidecar holds nothing of it.
				return nil
			}
			err := k.XuLyTin(ctx, m)
			mu.Lock()
			chay = append(chay, m.Seq)
			mu.Unlock()
			gia.mu.Lock()
			gia.vang = false // the sidecar comes back after the first attempt
			gia.mu.Unlock()
			handled <- struct{}{}
			return err
		}}
	runCtx, cancel := context.WithCancel(ctx)
	stopped := make(chan struct{})
	go func() { defer close(stopped); ket.Run(runCtx) }()
	defer func() { cancel(); <-stopped }()
	deadline := time.Now().Add(10 * time.Second)
	for !ket.Song() {
		if time.Now().After(deadline) {
			t.Fatal("the memory lane consumer never attached")
		}
		time.Sleep(10 * time.Millisecond)
	}
	// The trigger and the lookup in one transaction, so the handler knows
	// the id before the relay can publish it.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE people SET deleted_at=now() WHERE id=$1`, p); err != nil {
		t.Fatal(err)
	}
	var id string
	if err := tx.QueryRow(ctx, `SELECT id::text FROM nep_xoa WHERE person_id=$1 AND pham_vi='tai_khoan'`, p).Scan(&id); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	viec = id
	mu.Unlock()
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	for done := false; !done; {
		select {
		case <-handled:
			var n int
			_ = pool.QueryRow(ctx, `SELECT count(*) FROM nep_xoa WHERE id=$1 AND buoc='xong'`, viec).Scan(&n)
			done = n == 1
		case <-time.After(15 * time.Second):
			t.Fatalf("the deletion did not finish through the broker; attempts %v", attempts())
		}
	}
	var lan int
	var loi *string
	if err := pool.QueryRow(ctx, `SELECT lan_thu, loi FROM nep_xoa WHERE id=$1 AND person_id IS NULL AND nguoi_bam IS NOT NULL`, viec).Scan(&lan, &loi); err != nil {
		t.Fatal(err)
	}
	gia.mu.Lock()
	purges := gia.goi["xoasach"]
	gia.mu.Unlock()
	if lan != 1 || loi != nil || purges != 2 {
		t.Fatalf("lan_thu=%d loi=%v purge calls=%d attempts=%v", lan, loi, purges, attempts())
	}
	if left := gia.owners(); len(left) != 0 {
		t.Fatalf("the sidecar still holds memories of %v", left)
	}
	t.Logf("attempts by enqueue seq: %v", attempts())
}

func suThatMoiBroker(noiDung string) trinho.SuThatMoi {
	return trinho.SuThatMoi{NoiDung: noiDung, Loai: trinho.ThichDanhMuc, Nguon: trinho.NoiRo, TuLuc: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)}
}

func xoaTopology(url string, top jobs.Topology) {
	c, err := amqp.Dial(url)
	if err != nil {
		return
	}
	defer c.Close()
	ch, err := c.Channel()
	if err != nil {
		return
	}
	for _, q := range jobs.Queues {
		_, _ = ch.QueueDelete(top.Queue(q), false, false, false)
		_, _ = ch.QueueDelete(top.DeadQueue(q), false, false, false)
	}
	_ = ch.ExchangeDelete(top.Exchange(), false, false)
	_ = ch.ExchangeDelete(top.DeadExchange(), false, false)
}

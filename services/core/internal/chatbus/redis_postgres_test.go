//go:build postgres

package chatbus

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"mobile/services/core/internal/testdb"
)

func TestRealRedisRelayRecoveryAndMetadataOnly(t *testing.T) {
	base := testdb.Pool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	var suffix [10]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	namespace := "bus_" + hex.EncodeToString(suffix[:])
	schema := pgx.Identifier{namespace}.Sanitize()
	if _, err := base.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer base.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
	config := base.Config().Copy()
	config.ConnConfig.RuntimeParams["search_path"] = namespace + ",public"
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	// This isolates the relay adapter. Event/ACL/crash contracts are exercised
	// separately by chatv2http's actual two-process PostgreSQL suite.
	_, err = pool.Exec(ctx, `CREATE TABLE chat_v2_outbox(context_id uuid NOT NULL,sequence bigint NOT NULL,published_at timestamptz,PRIMARY KEY(context_id,sequence))`)
	if err != nil {
		t.Fatal(err)
	}
	// repo-guard: allow=long-number reason=synthetic-relay-test-context-uuid
	room := "11111111-1111-4111-8111-111111111111"
	_, err = pool.Exec(ctx, `INSERT INTO chat_v2_outbox SELECT $1::uuid,n,NULL FROM generate_series(1,10) n`, room)
	if err != nil {
		t.Fatal(err)
	}
	// An unavailable broker cannot advance the durable relay checkpoint.
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	closed := listener.Addr().String()
	_ = listener.Close()
	broken, err := New("redis://"+closed, namespace)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = broken.Flush(ctx, pool); err == nil {
		t.Fatal("failed publish marked successful")
	}
	_ = broken.Close()
	var pending int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM chat_v2_outbox WHERE published_at IS NULL`).Scan(&pending); err != nil || pending != 10 {
		t.Fatal("failed relay lost outbox rows")
	}
	raw, err := exec.CommandContext(ctx, "docker", "run", "--rm", "-d", "-p", "127.0.0.1::6379", "redis:7-alpine").Output()
	if err != nil {
		t.Fatal("cannot start isolated Redis", err)
	}
	container := strings.TrimSpace(string(raw))
	defer exec.Command("docker", "rm", "-f", container).Run()
	raw, err = exec.CommandContext(ctx, "docker", "port", container, "6379/tcp").Output()
	if err != nil {
		t.Fatal(err)
	}
	url := "redis://" + strings.TrimSpace(string(raw))
	bus, err := New(url, namespace)
	if err != nil {
		t.Fatal(err)
	}
	defer bus.Close()
	peer, err := New(url, namespace)
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	ready := make(chan struct{})
	wakes := make(chan string, 10)
	listenCtx, stop := context.WithCancel(ctx)
	defer stop()
	done := make(chan struct{})
	go func() { defer close(done); peer.Listen(listenCtx, func(s string) { wakes <- s }, ready) }()
	select {
	case <-ready:
	case <-ctx.Done():
		t.Fatal("Redis subscription not ready")
	}
	// Two competing workers still checkpoint each row transactionally once.
	var wg sync.WaitGroup
	counts := make(chan int, 2)
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); n, e := bus.Flush(ctx, pool); counts <- n; errs <- e }()
	}
	wg.Wait()
	close(counts)
	close(errs)
	total := 0
	for n := range counts {
		total += n
	}
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	if total != 10 {
		t.Fatalf("relay count %d", total)
	}
	select {
	case s := <-wakes:
		if s != room {
			t.Fatal("wrong conversation hint")
		}
	case <-ctx.Done():
		t.Fatal("peer did not receive real Redis hint")
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM chat_v2_outbox WHERE published_at IS NULL`).Scan(&pending); err != nil || pending != 0 {
		t.Fatal("relay checkpoint missing")
	}
	if _, err = bus.client.Publish(ctx, bus.channel, `{"conversation":"not-a-uuid","sequence":1}`).Result(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-wakes:
		t.Fatal("malformed hint accepted")
	case <-time.After(100 * time.Millisecond):
	}
	stop()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("subscription leaks on shutdown")
	}
}

func TestPostgresRelayPublishesOnlyCommittedRows(t *testing.T) {
	base := testdb.Pool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var suffix [10]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	namespace := "bus_" + hex.EncodeToString(suffix[:])
	schema := pgx.Identifier{namespace}.Sanitize()
	if _, err := base.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer base.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
	config := base.Config().Copy()
	config.ConnConfig.RuntimeParams["search_path"] = namespace + ",public"
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	_, err = pool.Exec(ctx, `CREATE TABLE chat_v2_outbox(context_id uuid NOT NULL,sequence bigint NOT NULL,published_at timestamptz,PRIMARY KEY(context_id,sequence))`)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Release()
	if _, err = listener.Exec(ctx, `LISTEN rudi_chat_v2`); err != nil {
		t.Fatal(err)
	}
	// repo-guard: allow=long-number reason=synthetic-relay-test-context-uuid
	room := "22222222-2222-4222-8222-222222222222"
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `INSERT INTO chat_v2_outbox SELECT $1::uuid,n,NULL FROM generate_series(1,10) n`, room); err != nil {
		t.Fatal(err)
	}
	bus := Postgres()
	if n, e := bus.Flush(ctx, pool); e != nil || n != 0 {
		t.Fatalf("uncommitted rows relayed: %d %v", n, e)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if n, e := bus.Flush(ctx, pool); e != nil || n != 10 {
		t.Fatalf("committed rows not relayed: %d %v", n, e)
	}
	notification, err := listener.Conn().WaitForNotification(ctx)
	if err != nil || notification.Payload != room {
		t.Fatalf("missing metadata wake: %v %v", notification, err)
	}
	if n, e := bus.Flush(ctx, pool); e != nil || n != 0 {
		t.Fatalf("checkpoint replayed: %d %v", n, e)
	}
	var pending int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM chat_v2_outbox WHERE published_at IS NULL`).Scan(&pending); err != nil || pending != 0 {
		t.Fatal("relay checkpoint missing")
	}
}

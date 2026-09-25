//go:build broker

package llm

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func redisLimiter(t *testing.T, rpm int) (*GioiHanRedis, *redis.Client) {
	t.Helper()
	url := os.Getenv("CORE_TEST_REDIS_URL")
	if url == "" {
		if os.Getenv("CORE_REQUIRE_BROKER_TESTS") == "1" {
			t.Fatal("CORE_TEST_REDIS_URL is required")
		}
		t.Skip("CORE_TEST_REDIS_URL not set")
	}
	opts, err := RedisOptions(url)
	if err != nil {
		t.Fatal(err)
	}
	rdb := redis.NewClient(opts)
	t.Cleanup(func() { _ = rdb.Close() })
	g, err := NewGioiHanRedis(rdb, fmt.Sprintf("t%d", time.Now().UnixNano()), rpm)
	if err != nil {
		t.Fatal(err)
	}
	return g, rdb
}

// 60 calls a minute with a five-second burst: six at once, then one a
// second. The count lives in Redis, so two limiters on one key share it, as
// two worker processes do; another model has its own.
func TestGioiHanRedisGCRA(t *testing.T) {
	g, rdb := redisLimiter(t, 60)
	other := *g
	ctx := context.Background()
	allowed := 0
	for range 10 {
		ok, err := other.Xin(ctx, Model)
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			allowed++
		}
	}
	if allowed != 6 {
		t.Fatalf("burst allowed %d, want 6 (five seconds' worth plus one)", allowed)
	}
	if ok, err := g.Xin(ctx, Model); ok || err != nil {
		t.Fatalf("a second limiter on the same key did not see the count: %v %v", ok, err)
	}
	if ok, err := g.Xin(ctx, "gemini-embedding-001"); !ok || err != nil {
		t.Fatalf("another model shares the budget: %v %v", ok, err)
	}
	time.Sleep(1100 * time.Millisecond)
	if ok, err := g.Xin(ctx, Model); !ok || err != nil {
		t.Fatalf("one cell a second did not come back: %v %v", ok, err)
	}
	key, _ := g.Key(Model)
	ttl, err := rdb.PTTL(ctx, key).Result()
	if err != nil || ttl <= 0 || ttl > 7*time.Second {
		t.Fatalf("key TTL %v %v: the key must expire once it no longer constrains anything", ttl, err)
	}
	if s, err := rdb.Get(ctx, key).Result(); err != nil || len(s) > 20 {
		t.Fatalf("the key holds %q (%v); it must hold one number", s, err)
	}
}

// A Redis that does not answer is an error within the limiter's own bound,
// which the counter turns into "let the call go".
func TestGioiHanRedisHongThiLoiNhanh(t *testing.T) {
	opts, _ := RedisOptions("redis://127.0.0.1:1/0")
	rdb := redis.NewClient(opts)
	defer rdb.Close()
	g, err := NewGioiHanRedis(rdb, "down", 60)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	ok, err := g.Xin(context.Background(), Model)
	if err == nil || ok {
		t.Fatalf("down Redis answered: %v %v", ok, err)
	}
	if took := time.Since(start); took > 400*time.Millisecond {
		t.Fatalf("a down Redis cost %v per call", took)
	}
	stub := NewStub(Buoc{Text: "ok"})
	d := NewDem(stub, MaxModelCallsPerTurn, nil).WithGioiHan(g).WithWait(khongCho)
	if text, err := chay(t, d); err != nil || text != "ok" || errors.Is(err, ErrGioiHan) {
		t.Fatalf("fail open: %q %v", text, err)
	}
}

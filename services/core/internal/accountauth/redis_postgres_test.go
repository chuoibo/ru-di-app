//go:build postgres

package accountauth

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSharedLimiterAcrossReplicasFailsClosed(t *testing.T) {
	url := os.Getenv("CORE_TEST_REDIS_URL")
	if url == "" {
		if os.Getenv("CORE_REQUIRE_POSTGRES_TESTS") == "1" {
			t.Fatal("Redis required: run scripts/go_postgres_tier.sh")
		}
		t.Skip("CORE_TEST_REDIS_URL missing")
	}
	opts, e := redis.ParseURL(url)
	if e != nil {
		t.Fatal(e)
	}
	client := redis.NewClient(opts)
	defer client.Close()
	client2 := redis.NewClient(opts)
	defer client2.Close()
	key, _ := randomSecret()
	namespace := "synthetic:auth:" + key + ":"
	defer client.Del(context.Background(), namespace+"quota")
	replicas := []redisLimiter{{client, namespace}, {client2, namespace}}
	var allowed atomic.Int64
	var wg sync.WaitGroup
	for i := range 100 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ok, e := replicas[i%2].Allow(context.Background(), "quota", 50, time.Minute)
			if e != nil {
				t.Error(e)
			}
			if ok {
				allowed.Add(1)
			}
		}(i)
	}
	wg.Wait()
	if allowed.Load() != 50 {
		t.Fatalf("shared budget %d want 50", allowed.Load())
	}
	h := New(nil, Config{Vault: testVault(t), Limits: replicas[0]})
	_ = client.Close()
	e = h.limit(context.Background(), "offline", fmt.Sprint(time.Now().UnixNano()), 1, time.Minute)
	if e == nil || e.(*Error).Status != 503 {
		t.Fatal("Redis outage did not close auth")
	}
}

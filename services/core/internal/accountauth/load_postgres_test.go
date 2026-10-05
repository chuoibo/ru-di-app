//go:build postgres && authload

package accountauth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// This fixture is isolated from every deployed stack. It measures the actual
// HTTP/password/PostgreSQL/Redis path, not email delivery or Google sign-in.
func TestPostgresAccountLoadTwoReplicas(t *testing.T) {
	if os.Getenv("AUTH_LOAD_ACCEPT_15_MINUTES") != "1" {
		t.Fatal("set AUTH_LOAD_ACCEPT_15_MINUTES=1; this test takes fifteen minutes")
	}
	h := authWorld(t)
	ctx := context.Background()
	const password = "isolated load credential with spaces"
	const accounts = 200
	for i := range accounts {
		hash, err := hashPassword(password)
		if err != nil {
			t.Fatal(err)
		}
		tx, err := h.pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		// repo-guard: allow=email reason=synthetic-reserved-test-domain
		_, err = h.createAccount(ctx, tx, pending{Username: fmt.Sprintf("load_%03d", i), Email: fmt.Sprintf("load_%03d@example.test", i), PasswordHash: hash})
		err = commit(ctx, tx, err)
		if err != nil {
			t.Fatal(err)
		}
	}
	opts, err := redis.ParseURL(os.Getenv("CORE_TEST_REDIS_URL"))
	if err != nil {
		t.Fatal(err)
	}
	key, err := randomSecret()
	if err != nil {
		t.Fatal(err)
	}
	namespace := "synthetic:authload:" + key + ":"
	var servers []*httptest.Server
	for range 2 {
		pool, err := pgxpool.NewWithConfig(ctx, h.pool.Config().Copy())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(pool.Close)
		client := redis.NewClient(opts)
		t.Cleanup(func() { _ = client.Close() })
		_, trusted, _ := net.ParseCIDR("127.0.0.0/8")
		replica := New(pool, Config{Vault: h.cfg.Vault, Limits: redisLimiter{client, namespace}, HashSlots: 4, TrustedProxies: []*net.IPNet{trusted}})
		server := httptest.NewServer(replica)
		t.Cleanup(server.Close)
		servers = append(servers, server)
	}
	jobs := make(chan int, 100)
	var wg sync.WaitGroup
	var mu sync.Mutex
	durations := make([]time.Duration, 0, 18000)
	statuses := map[int]int{}
	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			transport := &http.Transport{MaxIdleConnsPerHost: 2}
			defer transport.CloseIdleConnections()
			client := &http.Client{Transport: transport, Timeout: 12 * time.Second}
			for n := range jobs {
				body, _ := json.Marshal(map[string]string{"username": fmt.Sprintf("load_%03d", n%accounts), "password": password})
				request, err := http.NewRequest("POST", servers[n%2].URL+"/auth/login", bytes.NewReader(body))
				if err != nil {
					t.Error(err)
					continue
				}
				request.Header.Set("Content-Type", "application/json")
				// Only the explicitly trusted, sanitizing loopback test proxy sets this.
				request.Header.Set("X-Forwarded-For", fmt.Sprintf("192.0.2.%d", n%accounts+1))
				start := time.Now()
				response, err := client.Do(request)
				status := 0
				if err == nil {
					status = response.StatusCode
					_, _ = io.Copy(io.Discard, response.Body)
					_ = response.Body.Close()
				}
				elapsed := time.Since(start)
				mu.Lock()
				durations = append(durations, elapsed)
				statuses[status]++
				mu.Unlock()
			}
		}()
	}
	tick := time.NewTicker(time.Second / 20)
	defer tick.Stop()
	start := time.Now()
	for i := range 18000 {
		<-tick.C
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	elapsed := time.Since(start)
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	if len(durations) != 18000 {
		t.Fatalf("responses=%d want18000", len(durations))
	}
	p95 := durations[(len(durations)*95/100)-1]
	failures := 18000 - statuses[http.StatusCreated]
	t.Logf("clients=100 replicas=2 requests=%d duration=%s rate=%.2f/s p95=%s failures=%d statuses=%v", len(durations), elapsed, float64(len(durations))/elapsed.Seconds(), p95, failures, statuses)
	if p95 > time.Second {
		t.Errorf("p95=%s exceeds1s", p95)
	}
	if failures*1000 >= 18000 {
		t.Errorf("unexpected errors >=0.1%%: %d", failures)
	}
	// Retain limits during measurement, then prove the login budget still bites.
	limiterClient := redis.NewClient(opts)
	defer limiterClient.Close()
	assertion := New(h.pool, Config{Vault: h.cfg.Vault, Limits: redisLimiter{limiterClient, namespace}, HashSlots: 4})
	for range 12 {
		status, _ := call(t, assertion, "/auth/login", "POST", "", map[string]string{"username": "load_000", "password": password})
		if status == 429 {
			return
		}
	}
	t.Fatal("login limiter did not bite after measurement")
}

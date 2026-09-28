// community-load exercises a disposable synthetic deployment via public HTTP and WebSocket.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

type actor struct {
	Token string `json:"token"`
}
type fixture struct {
	Actors []actor  `json:"actors"`
	Posts  []string `json:"posts"`
}
type metric struct {
	mu      sync.Mutex
	buckets [60001]int64
	count   int64
	failed  int
	dropped int
	status  map[string]int
}

func (m *metric) add(d time.Duration, ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.count++
	m.buckets[min(max(d.Milliseconds(), 0), 60000)]++
	if !ok {
		m.failed++
	}
}
func (m *metric) report() map[string]any {
	m.mu.Lock()
	defer m.mu.Unlock()
	var cumulative, p95 int64
	for ms, count := range m.buckets {
		cumulative += count
		if cumulative*100 >= m.count*95 {
			p95 = int64(ms)
			break
		}
	}
	return map[string]any{"completed": m.count, "failed": m.failed, "dropped": m.dropped, "p95_ms": p95, "statuses": m.status}
}

func main() {
	base := flag.String("base", "http://127.0.0.1:18170", "Disposable synthetic API; loopback only")
	file := flag.String("actors", "", "Synthetic fixture JSON outside every worktree")
	sockets := flag.Int("sockets", 10000, "Concurrent sockets; one distinct actor each")
	reads := flag.Int("reads", 1000, "Feed requests per second")
	writes := flag.Int("writes", 200, "Like mutations per second")
	duration := flag.Duration("duration", time.Minute, "Steady load duration; use 24h for soak")
	flag.Parse()
	u, e := url.Parse(*base)
	if e != nil || (u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost") || *sockets < 0 || *reads < 0 || *writes < 0 || *reads > 100000 || *writes > 100000 || *duration <= 0 {
		fmt.Fprintln(os.Stderr, "requires loopback synthetic deployment and nonnegative rates")
		os.Exit(2)
	}
	raw, e := os.ReadFile(*file)
	if e != nil {
		fmt.Fprintln(os.Stderr, "cannot read synthetic actors")
		os.Exit(2)
	}
	var f fixture
	if json.Unmarshal(raw, &f) != nil || len(f.Actors) < max(*sockets, 1) || len(f.Posts) == 0 {
		fmt.Fprintln(os.Stderr, "not enough synthetic actors or posts")
		os.Exit(2)
	}
	for _, a := range f.Actors {
		if !strings.HasPrefix(a.Token, "synthetic-") {
			fmt.Fprintln(os.Stderr, "synthetic fixture tokens required")
			os.Exit(2)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var established, closed, observed atomic.Int64
	var wsWG sync.WaitGroup
	var mu sync.Mutex
	conns := []*websocket.Conn{}
	events := &metric{}
	handshake := make(chan struct{}, 100)
	var dialing sync.WaitGroup
	for i := 0; i < *sockets; i++ {
		handshake <- struct{}{}
		dialing.Add(1)
		go func(i int) {
			defer dialing.Done()
			defer func() { <-handshake }()
			cctx, done := context.WithTimeout(ctx, 20*time.Second)
			defer done()
			c, _, err := websocket.Dial(cctx, strings.Replace(*base, "http", "ws", 1)+"/v2/community/stream", nil)
			if err != nil {
				return
			}
			if err = wsjson.Write(cctx, c, map[string]any{"token": f.Actors[i].Token, "posts": f.Posts[:1]}); err != nil {
				c.CloseNow()
				return
			}
			var first struct {
				Kind string `json:"kind"`
			}
			if wsjson.Read(cctx, c, &first) != nil || first.Kind != "sync" {
				c.CloseNow()
				return
			}
			established.Add(1)
			mu.Lock()
			conns = append(conns, c)
			mu.Unlock()
			wsWG.Add(1)
			go func() {
				defer wsWG.Done()
				sawChange := false
				for {
					var msg struct {
						Kind string `json:"kind"`
						At   int64  `json:"occurred_at_ms"`
					}
					if wsjson.Read(ctx, c, &msg) != nil {
						if ctx.Err() == nil {
							closed.Add(1)
						}
						return
					}
					if msg.Kind == "post.changed" && msg.At > 0 {
						if !sawChange {
							observed.Add(1)
							sawChange = true
						}
						events.add(time.Since(time.UnixMilli(msg.At)), true)
					}
				}
			}()
		}(i)
	}
	dialing.Wait()
	fmt.Fprintf(os.Stderr, "synthetic sockets established: %d/%d\n", established.Load(), *sockets)
	client := &http.Client{Transport: &http.Transport{MaxIdleConns: 3000, MaxIdleConnsPerHost: 3000}, Timeout: 5 * time.Second}
	feed, mutation := &metric{}, &metric{}
	runctx, stop := context.WithTimeout(ctx, *duration)
	defer stop()
	var generators sync.WaitGroup
	generate := func(rate int, mutate bool, m *metric) {
		defer generators.Done()
		if rate == 0 {
			return
		}
		started := time.Now()
		target := int(duration.Seconds() * float64(rate))
		tick := time.NewTicker(10 * time.Millisecond)
		defer tick.Stop()
		limit := make(chan struct{}, 2000)
		var wg sync.WaitGroup
		defer wg.Wait()
		n := 0
		dispatch := func(desired int) {
			for n < desired {
				index := n
				n++
				select {
				case limit <- struct{}{}:
				default:
					m.mu.Lock()
					m.dropped++
					m.mu.Unlock()
					continue
				}
				wg.Add(1)
				go func() {
					defer wg.Done()
					defer func() { <-limit }()
					path := "/v2/community/feed"
					method := "GET"
					if mutate {
						path = "/v2/community/posts/" + f.Posts[0] + "/like"
						method = "PUT"
						if index/len(f.Actors)%2 == 1 {
							method = "DELETE"
						}
					}
					r, _ := http.NewRequestWithContext(ctx, method, *base+path, bytes.NewReader(nil))
					r.Header.Set("Authorization", "Bearer "+f.Actors[index%len(f.Actors)].Token)
					start := time.Now()
					resp, err := client.Do(r)
					ok := err == nil
					if resp != nil {
						io.Copy(io.Discard, resp.Body)
						resp.Body.Close()
						ok = ok && resp.StatusCode >= 200 && resp.StatusCode < 300
					}
					m.add(time.Since(start), ok)
					code := "transport_error"
					if resp != nil {
						code = fmt.Sprintf("http_%d", resp.StatusCode)
					}
					m.mu.Lock()
					if m.status == nil {
						m.status = map[string]int{}
					}
					m.status[code]++
					m.mu.Unlock()
				}()
			}
		}
		for {
			select {
			case <-runctx.Done():
				// Account for the final tick; overload is recorded by dispatch.
				dispatch(target)
				return
			case <-tick.C:
				dispatch(min(target, int(time.Since(started)*time.Duration(rate)/time.Second)))
			}
		}
	}
	generators.Add(2)
	go generate(*reads, false, feed)
	go generate(*writes, true, mutation)
	generators.Wait()
	cancel()
	for _, c := range conns {
		c.CloseNow()
	}
	wsWG.Wait()
	feedReport, mutationReport, eventReport := feed.report(), mutation.report(), events.report()
	passed := established.Load() == int64(*sockets) && closed.Load() == 0 && (*writes == 0 || observed.Load() == int64(*sockets)) && feed.failed == 0 && mutation.failed == 0 && feed.dropped == 0 && mutation.dropped == 0 && feedReport["p95_ms"].(int64) <= 500 && mutationReport["p95_ms"].(int64) <= 700 && eventReport["p95_ms"].(int64) <= 2000
	report := map[string]any{"thresholds_passed": passed, "duration": duration.String(), "requested_sockets": *sockets, "established_sockets": established.Load(), "sockets_observed_change": observed.Load(), "unexpected_socket_closures": closed.Load(), "requested_reads_per_second": *reads, "requested_writes_per_second": *writes, "feed": feedReport, "mutations": mutationReport, "event_lag": eventReport, "scope": "synthetic loopback only; event lag requires synchronized clocks; not native frame timing"}
	encoded, _ := json.MarshalIndent(report, "", "  ")
	fmt.Println(string(encoded))
	if !passed {
		os.Exit(1)
	}
}

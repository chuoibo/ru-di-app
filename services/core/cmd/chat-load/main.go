// Command chat-load drives the real experimental transport with synthetic
// opaque envelopes. It never claims to implement MLS or native app clients.
package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/jackc/pgx/v5/pgxpool"
	"mobile/services/core/internal/auth"
	"mobile/services/core/internal/chatv2"
	"mobile/services/core/internal/chatv2diag"
)

type device struct {
	Person, ID, Token, Group string
	Key                      ed25519.PrivateKey
	Public                   ed25519.PublicKey
}
type receipt struct {
	Envelope chatv2.Envelope
	Start    time.Time
	Actor    string
}
type sequenceID struct {
	Conversation string
	Sequence     int64
}
type socketClient struct {
	Device     device
	Cursor     atomic.Int64
	Connection *websocket.Conn
	mu         sync.Mutex
	Ready      chan struct{}
	Planned    atomic.Bool
}
type histogram struct {
	mu    sync.Mutex
	Bins  [120001]int64
	Count int64
	Max   int64
}

func (h *histogram) add(d time.Duration) {
	n := d.Milliseconds()
	if n < 0 {
		n = 0
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.Count++
	if n > h.Max {
		h.Max = n
	}
	if n > 120000 {
		n = 120000
	}
	h.Bins[n]++
}
func (h *histogram) result() map[string]int64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := map[string]int64{"count": h.Count, "max_ms": h.Max}
	for _, p := range []int64{50, 95, 99} {
		threshold := (h.Count*p + 99) / 100
		var n int64
		for i, v := range h.Bins {
			n += v
			if n >= threshold {
				out[fmt.Sprintf("p%d_ms", p)] = int64(i)
				break
			}
		}
	}
	return out
}

type metrics struct {
	Delivery     histogram
	Send         histogram
	Received     atomic.Int64
	Duplicate    atomic.Int64
	Gap          atomic.Int64
	Corrupt      atomic.Int64
	ReadErrors   atomic.Int64
	Reconnects   atomic.Int64
	DialErrors   atomic.Int64
	Sends        atomic.Int64
	SendErrors   atomic.Int64
	QueueDrops   atomic.Int64
	Replay       atomic.Int64
	ReplayErrors atomic.Int64
	Active       atomic.Int64
	Peak         atomic.Int64
	HTTP         sync.Map
}
type world struct {
	PIDs             []int
	PeakRSS          map[int]int64
	Pool             *pgxpool.Pool
	Devices          []device
	SenderOrder      []int
	Groups           []string
	Expected         sync.Map
	Sequences        sync.Map
	LogicalSequences sync.Map
	Metrics          metrics
	URLs             []string
	Clients          []*socketClient
	HTTP             *http.Client
}

func id() string {
	var b [16]byte
	if _, e := rand.Read(b[:]); e != nil {
		panic(e)
	}
	s := hex.EncodeToString(b[:])
	return s[:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:]
}
func mustExec(p *pgxpool.Pool, q string, a ...any) error {
	_, e := p.Exec(context.Background(), q, a...)
	return e
}
func provision(ctx context.Context, raw string, people, devices int) (*world, error) {
	cfg, e := pgxpool.ParseConfig(raw)
	if e != nil {
		return nil, errors.New("invalid test DSN")
	}
	if cfg.ConnConfig.Database != "chat_mass_test" || cfg.ConnConfig.Host != "127.0.0.1" {
		return nil, errors.New("only isolated loopback chat_mass_test database is allowed")
	}
	cfg.MaxConns = 4
	p, e := pgxpool.NewWithConfig(ctx, cfg)
	if e != nil {
		return nil, e
	}
	w := &world{Pool: p, HTTP: &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{MaxIdleConns: 200, MaxIdleConnsPerHost: 100}}}
	if e = chatv2.Migrate(ctx, p); e != nil {
		return nil, e
	}
	var n int
	if e = p.QueryRow(ctx, "SELECT count(*) FROM chat_v2_devices").Scan(&n); e != nil || n != 0 {
		return nil, errors.New("test database must have no existing chat devices")
	}
	for g := 0; g < (people+99)/100; g++ {
		w.Groups = append(w.Groups, id())
	}
	tx, e := p.Begin(ctx)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback(ctx)
	for i := 0; i < people; i++ {
		person, token, membership := id(), "synthetic-mass-"+id(), id()
		group := w.Groups[i/100]
		queries := []struct {
			q string
			a []any
		}{{`INSERT INTO people(id,display_name)VALUES($1,'Synthetic load participant')`, []any{person}}, {`INSERT INTO account_sessions(id,person_id,token_digest,issued_via,expires_at)VALUES($1,$2,$3,'genesis',now()+interval '2 hours')`, []any{id(), person, auth.TokenDigest(token)}}}
		if i%100 == 0 {
			queries = append(queries, struct {
				q string
				a []any
			}{`INSERT INTO contexts(id,display_name,created_by_id)VALUES($1,'Synthetic load group',$2)`, []any{group, person}})
		}
		queries = append(queries, struct {
			q string
			a []any
		}{`INSERT INTO memberships(id,context_id,person_id,state,role,origin)VALUES($1,$2,$3,'active','member','named')`, []any{membership, group, person}})
		for _, v := range queries {
			if _, e = tx.Exec(ctx, v.q, v.a...); e != nil {
				return nil, e
			}
		}
		if i%100 == 0 {
			if _, e = tx.Exec(ctx, `INSERT INTO chat_v2_conversations(context_id,epoch,ready)VALUES($1,1,false)`, group); e != nil {
				return nil, e
			}
		}
		for j := 0; j < devices; j++ {
			pub, key, e := ed25519.GenerateKey(rand.Reader)
			if e != nil {
				return nil, e
			}
			d := device{person, id(), token, group, key, pub}
			if _, e = tx.Exec(ctx, `INSERT INTO chat_v2_devices(id,person_id,signing_key)VALUES($1,$2,$3)`, d.ID, person, []byte(pub)); e != nil {
				return nil, e
			}
			if _, e = tx.Exec(ctx, `INSERT INTO chat_v2_members(context_id,device_id,membership_id,first_sequence)VALUES($1,$2,$3,1)`, group, d.ID, membership); e != nil {
				return nil, e
			}
			w.Devices = append(w.Devices, d)
		}
	}
	// Synthetic provisioning is confined to this disposable test database.
	if _, e = tx.Exec(ctx, `UPDATE chat_v2_conversations SET ready=true`); e != nil {
		return nil, e
	}
	// Interleave conversations so every group is active throughout the run.
	for position := 0; position < 100*devices; position++ {
		for group := range w.Groups {
			index := group*100*devices + position
			if index < len(w.Devices) {
				w.SenderOrder = append(w.SenderOrder, index)
			}
		}
	}
	return w, tx.Commit(ctx)
}
func startServer(ctx context.Context, binaryPath, raw string) (string, *exec.Cmd, error) {
	l, e := net.Listen("tcp4", "127.0.0.1:0")
	if e != nil {
		return "", nil, e
	}
	address := l.Addr().String()
	l.Close()
	cmd := exec.CommandContext(ctx, binaryPath, "serve")
	cmd.Env = append(os.Environ(), "RUDI_CHAT_LAB=1", "RUDI_CHAT_LAB_DATABASE_URL="+raw, "RUDI_CHAT_LAB_LISTEN="+address)
	cmd.Stderr = os.Stderr
	if e = cmd.Start(); e != nil {
		return "", nil, e
	}
	url := "http://" + address
	for i := 0; i < 100; i++ {
		r, e := http.Get(url + "/v2/chat/none/events")
		if e == nil {
			r.Body.Close()
			return url, cmd, nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return "", cmd, errors.New("server did not start")
}
func (w *world) connect(ctx context.Context, c *socketClient, index int) (*websocket.Conn, error) {
	u := "ws" + strings.TrimPrefix(w.URLs[index%len(w.URLs)], "http") + fmt.Sprintf("/v2/chat/%s/stream?device_id=%s&after=%d&limit=100", c.Device.Group, c.Device.ID, c.Cursor.Load())
	dial, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	conn, _, e := websocket.Dial(dial, u, &websocket.DialOptions{HTTPHeader: http.Header{"Authorization": []string{"Bearer " + c.Device.Token}}, Subprotocols: []string{chatv2.Protocol}})
	if e != nil {
		return nil, e
	}
	conn.SetReadLimit(1024 * 1024)
	var ready struct{ Type string }
	if e = wsjson.Read(dial, conn, &ready); e != nil || ready.Type != "ready" {
		conn.CloseNow()
		return nil, errors.New("missing ready frame")
	}
	return conn, nil
}
func (w *world) reader(ctx context.Context, c *socketClient, index int, wg *sync.WaitGroup) {
	defer wg.Done()
	// Each simulated device owns cancellation registration. Sharing one parent
	// directly makes every WebSocket timeout contend on the generator's mutex.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	first := true
	for ctx.Err() == nil {
		conn, e := w.connect(ctx, c, index)
		if e != nil {
			w.Metrics.DialErrors.Add(1)
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
				continue
			}
		}
		c.mu.Lock()
		c.Connection = conn
		c.mu.Unlock()
		n := w.Metrics.Active.Add(1)
		for old := w.Metrics.Peak.Load(); n > old && !w.Metrics.Peak.CompareAndSwap(old, n); old = w.Metrics.Peak.Load() {
		}
		if first {
			close(c.Ready)
			first = false
		} else {
			w.Metrics.Reconnects.Add(1)
		}
		for ctx.Err() == nil {
			var f struct {
				Type string       `json:"type"`
				Page *chatv2.Page `json:"page"`
			}
			if e = wsjson.Read(ctx, conn, &f); e != nil {
				break
			}
			if f.Type != "events" || f.Page == nil {
				w.Metrics.Corrupt.Add(1)
				break
			}
			for _, event := range f.Page.Events {
				cursor := c.Cursor.Load()
				if event.Sequence <= cursor {
					w.Metrics.Duplicate.Add(1)
					continue
				}
				if event.Sequence != cursor+1 {
					w.Metrics.Gap.Add(1)
				}
				if event.Envelope == nil {
					w.Metrics.Corrupt.Add(1)
				} else {
					got, ok := w.Expected.Load(event.Envelope.LogicalSendID)
					if !ok {
						w.Metrics.Corrupt.Add(1)
					} else {
						want := got.(receipt)
						if !sameEnvelope(*event.Envelope, want.Envelope) || event.ActorID != want.Actor || event.Envelope.ConversationID != c.Device.Group {
							w.Metrics.Corrupt.Add(1)
						}
						if !w.recordSequence(c.Device.Group, event.Sequence, event.Envelope.LogicalSendID) {
							w.Metrics.Corrupt.Add(1)
						}
						w.Metrics.Delivery.add(time.Since(want.Start))
					}
				}
				c.Cursor.Store(event.Sequence)
				w.Metrics.Received.Add(1)
			}
			if f.Page.NextSequence != c.Cursor.Load() {
				w.Metrics.Corrupt.Add(1)
				break
			}
			if e = wsjson.Write(ctx, conn, map[string]any{"type": "ack", "sequence": c.Cursor.Load()}); e != nil {
				break
			}
		}
		conn.CloseNow()
		w.Metrics.Active.Add(-1)
		if ctx.Err() != nil {
			return
		}
		if !c.Planned.Swap(false) {
			w.Metrics.ReadErrors.Add(1)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// Check both directions on every delivery. A read fast path avoids taking the
// dirty-map insertion lock for hundreds of copies of the same event, while
// LoadOrStore still arbitrates simultaneous first observations atomically.
func consistentMapping(m *sync.Map, key, value any) bool {
	prior, exists := m.Load(key)
	if !exists {
		prior, _ = m.LoadOrStore(key, value)
	}
	return prior == value
}

func (w *world) recordSequence(conversation string, sequence int64, logical string) bool {
	key := sequenceID{Conversation: conversation, Sequence: sequence}
	forward := consistentMapping(&w.Sequences, key, logical)
	reverse := consistentMapping(&w.LogicalSequences, logical, key)
	return forward && reverse
}

func (w *world) send(ctx context.Context, index int, replay bool, original *chatv2.Envelope) (chatv2.Envelope, error) {
	d := w.Devices[index%len(w.Devices)]
	var env chatv2.Envelope
	if original != nil {
		env = *original
	} else {
		payload := make([]byte, 256)
		rand.Read(payload)
		binary.BigEndian.PutUint64(payload, uint64(time.Now().UnixNano()))
		env = chatv2.Envelope{ConversationID: d.Group, DeviceID: d.ID, LogicalSendID: id(), Protocol: chatv2.Protocol, Epoch: 1, Ciphertext: payload}
		signed, _ := chatv2.SigningBytes(env)
		env.Signature = ed25519.Sign(d.Key, signed)
		w.Expected.Store(env.LogicalSendID, receipt{env, time.Now(), d.Person})
	}
	body, _ := json.Marshal(env)
	req, e := http.NewRequestWithContext(ctx, "POST", w.URLs[index%len(w.URLs)]+"/v2/chat/"+d.Group+"/events", bytes.NewReader(body))
	if e != nil {
		return env, e
	}
	req.Header.Set("Authorization", "Bearer "+d.Token)
	req.Header.Set("Content-Type", "application/json")
	start := time.Now()
	resp, e := w.HTTP.Do(req)
	w.Metrics.Send.add(time.Since(start))
	if e != nil {
		return env, e
	}
	defer resp.Body.Close()
	counter, _ := w.Metrics.HTTP.LoadOrStore(resp.StatusCode, &atomic.Int64{})
	counter.(*atomic.Int64).Add(1)
	var r chatv2.SendResult
	if e = json.NewDecoder(resp.Body).Decode(&r); e != nil {
		return env, e
	}
	expected := 201
	if replay {
		expected = 200
	}
	if resp.StatusCode != expected || r.Replayed != replay || r.Event.Envelope == nil || r.Event.Envelope.LogicalSendID != env.LogicalSendID {
		return env, fmt.Errorf("send returned %d replay=%t", resp.StatusCode, r.Replayed)
	}
	if replay {
		w.Metrics.Replay.Add(1)
	} else {
		w.Metrics.Sends.Add(1)
	}
	return env, nil
}
func (w *world) snapshot(stage string) {
	if w.PeakRSS == nil {
		w.PeakRSS = map[int]int64{}
	}
	rss := map[int]int64{}
	for _, pid := range w.PIDs {
		n := processRSS(pid)
		rss[pid] = n
		if n > w.PeakRSS[pid] {
			w.PeakRSS[pid] = n
		}
	}
	j := map[string]any{"stage": stage, "rss_kib": rss, "active": w.Metrics.Active.Load(), "sent": w.Metrics.Sends.Load(), "send_errors": w.Metrics.SendErrors.Load(), "received": w.Metrics.Received.Load(), "socket_errors": w.Metrics.ReadErrors.Load(), "delivery": w.Metrics.Delivery.result()}
	b, _ := json.Marshal(j)
	fmt.Fprintln(os.Stderr, string(b))
}
func run() error {
	duration := flag.Duration("duration", 60*time.Second, "send duration")
	rate := flag.Int("rate", 100, "scheduled messages per second")
	people := flag.Int("people", 200, "synthetic people")
	devices := flag.Int("devices", 5, "devices per person")
	lab := flag.String("lab-binary", "", "chat-lab binary")
	flag.Parse()
	if *people < 2 || *people > 1000 || *devices < 1 || *devices > 5 || (*people)*(*devices) > 1000 || *rate < 1 || *rate > 300 || *duration < time.Second || *duration > 30*time.Minute || *lab == "" {
		return errors.New("invalid bounded lab parameters")
	}
	stopProfile, profileErr := chatv2diag.Start("load", os.Getenv, os.Stderr)
	if profileErr != nil {
		return errors.New("cannot initialize load diagnostics")
	}
	defer stopProfile()
	raw := os.Getenv("RUDI_CHAT_LOAD_DATABASE_URL")
	ctx, cancel := context.WithTimeout(context.Background(), *duration+4*time.Minute)
	defer cancel()
	w, e := provision(ctx, raw, *people, *devices)
	if e != nil {
		return e
	}
	defer w.Pool.Close()
	var commands []*exec.Cmd
	defer func() {
		cancel()
		for _, cmd := range commands {
			cmd.Process.Kill()
			cmd.Wait()
		}
	}()
	for i := 0; i < 2; i++ {
		url, cmd, e := startServer(ctx, *lab, raw)
		if cmd != nil {
			commands = append(commands, cmd)
		}
		if e != nil {
			return e
		}
		w.URLs = append(w.URLs, url)
	}
	fmt.Fprintf(os.Stderr, "load servers pids=%d,%d people=%d sockets=%d\n", commands[0].Process.Pid, commands[1].Process.Pid, *people, len(w.Devices))
	w.PIDs = []int{os.Getpid(), commands[0].Process.Pid, commands[1].Process.Pid}
	readerCtx, stopReaders := context.WithCancel(ctx)
	defer stopReaders()
	var readers sync.WaitGroup
	// Stagger enrollment to separate steady-state load from a connection storm.
	for i, d := range w.Devices {
		c := &socketClient{Device: d, Ready: make(chan struct{})}
		w.Clients = append(w.Clients, c)
		readers.Add(1)
		go w.reader(readerCtx, c, i, &readers)
		if i%20 == 19 {
			time.Sleep(50 * time.Millisecond)
		}
	}
	readyDeadline := time.After(45 * time.Second)
	for _, c := range w.Clients {
		select {
		case <-c.Ready:
		case <-readyDeadline:
			return errors.New("not all sockets became ready")
		}
	}
	w.snapshot("all_ready")
	start := time.Now()
	ticker := time.NewTicker(time.Second / time.Duration(*rate))
	defer ticker.Stop()
	jobs := make(chan int, 1000)
	var workers sync.WaitGroup
	// Keep offering the configured rate even while every request reaches the
	// five-second server deadline. Queue drops remain explicit gate failures.
	workerCount := max(512, *rate*5+32)
	for i := 0; i < workerCount; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for n := range jobs {
				index := w.SenderOrder[n%len(w.SenderOrder)]
				env, e := w.send(ctx, index, false, nil)
				if e != nil {
					w.Metrics.SendErrors.Add(1)
					continue
				}
				if n%100 == 0 {
					if _, e = w.send(ctx, index, true, &env); e != nil {
						w.Metrics.ReplayErrors.Add(1)
					}
				}
			}
		}()
	}
	scheduled := 0
	offered := 0
	forced := 0
	nextProgress := start.Add(10 * time.Second)
	targetOffers := int(duration.Nanoseconds() * int64(*rate) / int64(time.Second))
	for offered < targetOffers {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
		// Tickers may coalesce under host contention. Account for every due
		// arrival instead of quietly lowering the offered load on a busy host.
		due := min(targetOffers, int(time.Since(start).Nanoseconds()*int64(*rate)/int64(time.Second)))
		for offered < due {
			n := offered
			offered++
			select {
			case jobs <- n:
				scheduled++
			default:
				w.Metrics.QueueDrops.Add(1)
			}
		}
		if forced == 0 && time.Since(start) >= *duration/2 {
			for i := 0; i < len(w.Clients); i += 10 {
				c := w.Clients[i]
				c.Planned.Store(true)
				c.mu.Lock()
				if c.Connection != nil {
					c.Connection.CloseNow()
				}
				c.mu.Unlock()
				forced++
			}
		}
		if time.Now().After(nextProgress) {
			w.snapshot("sending")
			nextProgress = time.Now().Add(10 * time.Second)
		}
	}
	offeringElapsed := time.Since(start)
	close(jobs)
	workers.Wait()
	elapsed := time.Since(start)
	w.snapshot("draining")
	rows, e := w.Pool.Query(ctx, `SELECT context_id::text,last_sequence FROM chat_v2_conversations`)
	if e != nil {
		return e
	}
	target := map[string]int64{}
	for rows.Next() {
		var g string
		var s int64
		if e = rows.Scan(&g, &s); e != nil {
			return e
		}
		target[g] = s
	}
	rows.Close()
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		done := true
		for _, c := range w.Clients {
			if c.Cursor.Load() != target[c.Device.Group] {
				done = false
				break
			}
		}
		if done {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	var missing int64
	for _, c := range w.Clients {
		missing += target[c.Device.Group] - c.Cursor.Load()
	}
	var count, outbox, dedup int64
	if e = w.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM chat_v2_events),(SELECT count(*) FROM chat_v2_outbox),(SELECT count(*) FROM chat_v2_sends)`).Scan(&count, &outbox, &dedup); e != nil {
		return e
	}
	stopReaders()
	readers.Wait()
	statuses := map[string]int64{}
	w.Metrics.HTTP.Range(func(k, v any) bool { statuses[fmt.Sprint(k)] = v.(*atomic.Int64).Load(); return true })
	cursors := make([]int64, len(w.Clients))
	for i, c := range w.Clients {
		cursors[i] = c.Cursor.Load()
	}
	sort.Slice(cursors, func(i, j int) bool { return cursors[i] < cursors[j] })
	expectedDeliveries := int64(0)
	for _, c := range w.Clients {
		expectedDeliveries += target[c.Device.Group]
	}
	result := map[string]any{"scope": "synthetic signed opaque envelopes, real HTTP/WebSocket/PostgreSQL, NOT MLS or mobile E2E", "people": *people, "devices_per_person": *devices, "groups": len(w.Groups), "connections": len(w.Clients), "peak_connections": w.Metrics.Peak.Load(), "duration_seconds": elapsed.Seconds(), "target_rate": *rate, "sender_workers": workerCount, "scheduled": scheduled, "offered": offered, "offering_duration_seconds": offeringElapsed.Seconds(), "offered_rate": float64(offered) / offeringElapsed.Seconds(), "queue_drops": w.Metrics.QueueDrops.Load(), "send_duration_seconds": duration.Seconds(), "achieved_rate_including_drain": float64(w.Metrics.Sends.Load()) / elapsed.Seconds(), "sent": w.Metrics.Sends.Load(), "send_errors": w.Metrics.SendErrors.Load(), "replay_ok": w.Metrics.Replay.Load(), "replay_errors": w.Metrics.ReplayErrors.Load(), "http_status": statuses, "db_events": count, "db_outbox": outbox, "db_dedup": dedup, "expected_deliveries": expectedDeliveries, "received_deliveries": w.Metrics.Received.Load(), "missing_deliveries": missing, "duplicates": w.Metrics.Duplicate.Load(), "sequence_gaps": w.Metrics.Gap.Load(), "corrupt_envelopes": w.Metrics.Corrupt.Load(), "unexpected_disconnects": w.Metrics.ReadErrors.Load(), "dial_errors": w.Metrics.DialErrors.Load(), "planned_reconnects": forced, "reconnects": w.Metrics.Reconnects.Load(), "send_latency": w.Metrics.Send.result(), "delivery_latency": w.Metrics.Delivery.result(), "minimum_cursor": cursors[0], "maximum_cursor": cursors[len(cursors)-1]}
	pass := float64(offered)/offeringElapsed.Seconds() >= float64(*rate)*0.99 && offered >= int(duration.Seconds()*float64(*rate)*0.99) && w.Metrics.QueueDrops.Load() == 0 && missing == 0 && count == int64(scheduled) && count == outbox && count == dedup && w.Metrics.SendErrors.Load() == 0 && w.Metrics.ReplayErrors.Load() == 0 && w.Metrics.Gap.Load() == 0 && w.Metrics.Corrupt.Load() == 0 && w.Metrics.Duplicate.Load() == 0 && w.Metrics.ReadErrors.Load() == 0 && w.Metrics.DialErrors.Load() == 0 && w.Metrics.Peak.Load() == int64(len(w.Clients))
	latency := w.Metrics.Delivery.result()
	latencyPassed := latency["p95_ms"] <= 800 && latency["p99_ms"] <= 2000
	result["integrity_and_capacity_passed"] = pass
	result["adr0031_latency_passed"] = latencyPassed
	pass = pass && latencyPassed
	result["passed"] = pass
	result["sampled_peak_rss_kib_by_pid"] = w.PeakRSS
	b, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(b))
	if !pass {
		return errors.New("load gate failed; measured result above")
	}
	return nil
}
func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}

func sameEnvelope(a, b chatv2.Envelope) bool {
	return a.ConversationID == b.ConversationID && a.DeviceID == b.DeviceID && a.LogicalSendID == b.LogicalSendID && a.Protocol == b.Protocol && a.Epoch == b.Epoch && bytes.Equal(a.Ciphertext, b.Ciphertext) && bytes.Equal(a.Signature, b.Signature)
}

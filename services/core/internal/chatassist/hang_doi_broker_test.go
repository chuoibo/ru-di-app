//go:build broker

package chatassist

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"log/slog"
	"os"
	"regexp"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"google.golang.org/adk/model"

	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/brain"
	"mobile/services/core/internal/jobs"
)

// The queue end to end on a real RabbitMQ and PostgreSQL (slice 10; design 02
// §4, §7, §9): create -> trigger -> outbox -> relay with publisher confirms ->
// consumer -> claim by (id, enqueue_seq) -> the engine on a scripted stub ->
// the answer committed -> Ack. Zero real model calls (ADR-0034 §2.6): the Go
// engine runs on llm.Stub, the group path on the fixture's fake brain.

func amqpURL(t *testing.T) string {
	t.Helper()
	url := os.Getenv("CORE_TEST_AMQP_URL")
	if url == "" {
		if os.Getenv("CORE_REQUIRE_BROKER_TESTS") == "1" {
			t.Fatal("CORE_TEST_AMQP_URL is required")
		}
		t.Skip("CORE_TEST_AMQP_URL not set")
	}
	return url
}

// topology is a broker namespace of the test's own, deleted afterwards.
func topology(t *testing.T, url string) jobs.Topology {
	t.Helper()
	top, err := jobs.NewTopology(fmt.Sprintf("q%d", time.Now().UnixNano()%1_000_000_000))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		conn, err := amqp.Dial(url)
		if err != nil {
			return
		}
		defer conn.Close()
		ch, err := conn.Channel()
		if err != nil {
			return
		}
		for _, q := range jobs.Queues {
			_, _ = ch.QueueDelete(top.Queue(q), false, false, false)
			_, _ = ch.QueueDelete(top.DeadQueue(q), false, false, false)
		}
		_ = ch.ExchangeDelete(top.Exchange(), false, false)
		_ = ch.ExchangeDelete(top.DeadExchange(), false, false)
	})
	return top
}

// dongHo wraps the stub and records when each numbered question reached the
// model: the end of the queue delay the tests measure.
type dongHo struct {
	inner model.LLM
	mu    sync.Mutex
	luc   map[int][]time.Time
}

var cauSo = regexp.MustCompile(`câu hỏi số (\d+)`)

func newDongHo(inner model.LLM) *dongHo { return &dongHo{inner: inner, luc: map[int][]time.Time{}} }

func (d *dongHo) Name() string { return d.inner.Name() }

func (d *dongHo) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	now := time.Now()
	for _, c := range req.Contents {
		if c == nil {
			continue
		}
		for _, p := range c.Parts {
			if p == nil {
				continue
			}
			if m := cauSo.FindStringSubmatch(p.Text); m != nil {
				n, _ := strconv.Atoi(m[1])
				d.mu.Lock()
				d.luc[n] = append(d.luc[n], now)
				d.mu.Unlock()
			}
		}
	}
	return d.inner.GenerateContent(ctx, req, stream)
}

func (d *dongHo) lan(n, i int) (time.Time, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if i < len(d.luc[n]) {
		return d.luc[n][i], true
	}
	return time.Time{}, false
}

// tram is one worker process in miniature: a Ket (relay + consumers) and,
// when asked, the Postgres poller, over h.
type tram struct {
	ket    *jobs.Ket
	stop   context.CancelFunc
	done   sync.WaitGroup
	stdout *bytes.Buffer
	mu     sync.Mutex
}

func (tr *tram) log() string {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	return tr.stdout.String()
}

type khoaGhi struct {
	tr *tram
}

func (k khoaGhi) Write(p []byte) (int, error) {
	k.tr.mu.Lock()
	defer k.tr.mu.Unlock()
	return k.tr.stdout.Write(p)
}

func moTram(t *testing.T, f fixture, h *Handler, url string, top jobs.Topology, poller bool) *tram {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	tr := &tram{stop: cancel, stdout: &bytes.Buffer{}}
	tr.ket = &jobs.Ket{URL: url, Topology: top, Pool: f.pool, Queues: Queues, Concurrency: h.worker.Workers,
		Handler: h.XuLyTin, Logger: slog.New(slog.NewJSONHandler(khoaGhi{tr}, nil))}
	tr.done.Add(1)
	go func() { defer tr.done.Done(); tr.ket.Run(ctx) }()
	if poller {
		tr.done.Add(1)
		go func() { defer tr.done.Done(); h.RunWorkers(ctx, tr.ket) }()
	}
	t.Cleanup(tr.dung)
	return tr
}

func (tr *tram) dung() {
	tr.stop()
	tr.done.Wait()
}

func (tr *tram) choSong(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !tr.ket.Song() {
		if time.Now().After(deadline) {
			t.Fatalf("consumers never attached; log:\n%s", tr.log())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// choXong waits until every id is succeeded, or the deadline; it returns how
// many are.
func (f fixture) choXong(t *testing.T, ids []string, within time.Duration) int {
	t.Helper()
	deadline := time.Now().Add(within)
	for {
		var n int
		if err := f.pool.QueryRow(context.Background(), `SELECT count(*) FROM chat_ai_invocations WHERE id=ANY($1) AND status='succeeded'`, ids).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n == len(ids) || time.Now().After(deadline) {
			return n
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func phanVi(ds []time.Duration, p float64) time.Duration {
	s := append([]time.Duration(nil), ds...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	i := int(float64(len(s)-1) * p)
	return s[i]
}

func traLoiGiong(n int) []llm.Buoc {
	out := make([]llm.Buoc, n)
	for i := range out {
		out[i] = llm.Buoc{Text: "Đi dạo hồ nhé."}
	}
	return out
}

// Through the real routes, with the poller off: a Nếp question on the Go
// engine and a group question on the brain path each travel create -> outbox
// -> relay -> consumer -> answer, and the outbox rows end published.
func TestHangDoiDauCuoiQuaBroker(t *testing.T) {
	url := amqpURL(t)
	f := setup(t, nil)
	stub := llm.NewStub(traLoiGiong(3)...)
	f.nepTrenEngine(t, stub)
	top := topology(t, url)
	tr := moTram(t, f, f.handler, url, top, false)
	tr.choSong(t)
	var ids []string
	for i := range 3 {
		code, v, raw := f.nepPostBroker(t, fmt.Sprintf("câu hỏi số %d", i))
		if code != 202 {
			t.Fatalf("nep %d: %d %s", i, code, raw)
		}
		ids = append(ids, v)
	}
	g := f.create(t)
	ids = append(ids, g.ID)
	if n := f.choXong(t, ids, 10*time.Second); n != len(ids) {
		t.Fatalf("%d/%d done; log:\n%s", n, len(ids), tr.log())
	}
	for _, id := range ids[:3] {
		w := f.request("GET", "/me/nep/ai-invocations/"+id, f.token, nil)
		var v NepInvocation
		_ = json.Unmarshal(w.Body.Bytes(), &v)
		if v.Status != "succeeded" || v.Text == nil || *v.Text != "Đi dạo hồ nhé." {
			t.Fatalf("Nếp answer: %+v", v)
		}
	}
	var cards, unpublished int
	if err := f.pool.QueryRow(context.Background(), `SELECT count(*) FROM messages WHERE kind='ai_card'`).Scan(&cards); err != nil || cards != 1 {
		t.Fatalf("group answer published %d times: %v", cards, err)
	}
	if err := f.pool.QueryRow(context.Background(), `SELECT count(*) FROM job_outbox WHERE published_at IS NULL`).Scan(&unpublished); err != nil || unpublished != 0 {
		t.Fatalf("%d outbox rows unpublished: %v", unpublished, err)
	}
	if stub.SoGoi() != 3 {
		t.Fatalf("stub answered %d, want 3", stub.SoGoi())
	}
}

// nepPostBroker asks Nếp through the route and returns the new id.
func (f fixture) nepPostBroker(t *testing.T, prompt string) (int, string, string) {
	t.Helper()
	w := f.request("POST", "/me/nep/ai-invocations", f.token, map[string]any{"logical_id": newID(), "prompt": prompt, "phieu": nil, "luot": []any{}})
	var v NepInvocation
	_ = json.Unmarshal(w.Body.Bytes(), &v)
	return w.Code, v.ID, w.Body.String()
}

// The queue delay, measured: 50 questions one after another, each timed from
// the commit of its row to the moment the (stub) model hears it. First with
// the broker and the poller off, then with the broker unreachable and the
// poller alone -- which must still finish all 50.
func TestHangDoiTreHangCoBrokerVaPoll(t *testing.T) {
	url := amqpURL(t)
	do := func(t *testing.T, brokerURL string, poller bool) []time.Duration {
		f := setup(t, nil)
		f.handler.WithWorker(WorkerConfig{Workers: 4, Tick: 250 * time.Millisecond, NetEvery: 2 * time.Second, NetLag: 5 * time.Second,
			Lease: 30 * time.Second, Heartbeat: 5 * time.Second, SweepEvery: 5 * time.Second})
		m := newDongHo(llm.NewStub(traLoiGiong(50)...))
		f.nepTrenEngine(t, m)
		tr := moTram(t, f, f.handler, brokerURL, topology(t, url), poller)
		if brokerURL == url {
			tr.choSong(t)
		}
		var ids []string
		commit := map[int]time.Time{}
		for i := range 50 {
			ids = append(ids, f.chenNep(t, 1, func(int) string { return fmt.Sprintf("câu hỏi số %d", i) })...)
			commit[i] = time.Now()
			time.Sleep(20 * time.Millisecond)
		}
		if n := f.choXong(t, ids, 20*time.Second); n != 50 {
			t.Fatalf("%d/50 done; log:\n%s", n, tr.log())
		}
		var delays []time.Duration
		for i := range 50 {
			at, ok := m.lan(i, 0)
			if !ok {
				t.Fatalf("question %d never reached the model", i)
			}
			delays = append(delays, at.Sub(commit[i]))
		}
		return delays
	}
	var broker, poll []time.Duration
	t.Run("broker", func(t *testing.T) { broker = do(t, url, false) })
	t.Run("broker mất, poller", func(t *testing.T) { poll = do(t, "amqp://guest:guest@127.0.0.1:1/", true) })
	if len(broker) != 50 || len(poll) != 50 {
		t.FailNow()
	}
	t.Logf("queue delay, commit -> model call (stub), 50 jobs: broker p50=%v p95=%v max=%v; poll only p50=%v p95=%v max=%v",
		phanVi(broker, .5).Round(time.Millisecond), phanVi(broker, .95).Round(time.Millisecond), phanVi(broker, 1).Round(time.Millisecond),
		phanVi(poll, .5).Round(time.Millisecond), phanVi(poll, .95).Round(time.Millisecond), phanVi(poll, 1).Round(time.Millisecond))
	if p := phanVi(broker, .95); p > 200*time.Millisecond {
		t.Errorf("broker p95 %v: over 200 ms, the poller could have done it", p)
	}
	if p := phanVi(poll, .95); p > 600*time.Millisecond {
		t.Errorf("poll p95 %v: over two fast ticks and a claim", p)
	}
}

// Canary scenario (design 02 §9 canary 1): with ai.nep unbound, the relay's
// mandatory publishes come back; the rows must stay unpublished, and once the
// queue is bound again all 50 run -- with the poller off, so nothing but the
// relay can deliver them. A relay that marks published_at before the confirm
// loses them all, red at «50/50 done».
//
// Three seconds unbound, not one: long enough for the relay's retries to
// return more than a batch's worth of messages. The relay before slice 10
// read one return per flush, the rest piled up in its return channel, the
// library's reader blocked on the full channel and the whole connection
// stalled: 0/50, measured, with the window at three seconds (at one second
// it still passed).
func TestHangDoiRelayGiuHangKhiBiTraVe(t *testing.T) {
	url := amqpURL(t)
	f := setup(t, nil)
	f.nepTrenEngine(t, llm.NewStub(traLoiGiong(50)...))
	top := topology(t, url)
	tr := moTram(t, f, f.handler, url, top, false)
	tr.choSong(t)
	conn, err := amqp.Dial(url)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	ch, err := conn.Channel()
	if err != nil {
		t.Fatal(err)
	}
	defer ch.Close()
	if err = ch.QueueUnbind(top.Queue(HangNep), HangNep, top.Exchange(), nil); err != nil {
		t.Fatal(err)
	}
	ids := f.chenNep(t, 50, func(i int) string { return fmt.Sprintf("câu hỏi số %d", i) })
	time.Sleep(3 * time.Second)
	var unpublished int
	if err = f.pool.QueryRow(context.Background(), `SELECT count(*) FROM job_outbox WHERE published_at IS NULL`).Scan(&unpublished); err != nil {
		t.Fatal(err)
	}
	if err = ch.QueueBind(top.Queue(HangNep), HangNep, top.Exchange(), false, nil); err != nil {
		t.Fatal(err)
	}
	if n := f.choXong(t, ids, 10*time.Second); n != 50 {
		t.Fatalf("%d/50 done after the queue came back; log:\n%s", n, tr.log())
	}
	// Read while unbound, asserted after: the outcome above is what the
	// canary predicts red, this says why.
	if unpublished != 50 {
		t.Fatalf("%d/50 rows stayed unpublished while nothing could route them", unpublished)
	}
}

// A poison message is dead-lettered, never handed to the engine; a well
// formed message naming no job is acknowledged, not dead-lettered.
func TestHangDoiTinDocVaoDLQ(t *testing.T) {
	url := amqpURL(t)
	f := setup(t, nil)
	stub := llm.NewStub()
	f.nepTrenEngine(t, stub)
	top := topology(t, url)
	tr := moTram(t, f, f.handler, url, top, false)
	tr.choSong(t)
	conn, err := amqp.Dial(url)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	ch, err := conn.Channel()
	if err != nil {
		t.Fatal(err)
	}
	defer ch.Close()
	for _, body := range []string{`{"v":1,"ref":"x","prompt":"leak"}`, `{"v":1,"ref":"` + newID() + `","seq":1}`} {
		if err = ch.PublishWithContext(context.Background(), top.Exchange(), HangNep, false, false, amqp.Publishing{Body: []byte(body)}); err != nil {
			t.Fatal(err)
		}
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		dead, err1 := ch.QueueDeclarePassive(top.DeadQueue(HangNep), true, false, false, false,
			amqp.Table{"x-queue-type": "quorum", "x-message-ttl": int64(7 * 24 * 3600 * 1000)})
		live, err2 := ch.QueueDeclarePassive(top.Queue(HangNep), true, false, false, false,
			amqp.Table{"x-queue-type": "quorum", "x-delivery-limit": int64(jobs.DeliveryLimit), "x-dead-letter-exchange": top.DeadExchange(), "x-dead-letter-routing-key": HangNep})
		if err1 == nil && err2 == nil && dead.Messages == 1 && live.Messages == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("dead=%d (%v) live=%d (%v), want 1 poison message dead-lettered and the other acknowledged", dead.Messages, err1, live.Messages, err2)
		}
		time.Sleep(50 * time.Millisecond)
	}
	if stub.SoGoi() != 0 {
		t.Fatal("a message reached the model")
	}
}

// Stopping a worker mid-job releases the job with a new enqueue_seq, so a
// second worker gets a message for it and runs it at once -- poller off (M5
// is red here: a release that kept the old seq has no message to deliver).
func TestHangDoiNhaKhiDungDuocNhanLaiNgay(t *testing.T) {
	url := amqpURL(t)
	f := setup(t, nil)
	m := newDongHo(llm.NewStub(llm.Buoc{Text: "không bao giờ tới", Cho: time.Minute}, llm.Buoc{Text: "Đi dạo hồ nhé."}))
	f.nepTrenEngine(t, m)
	top := topology(t, url)
	a := moTram(t, f, f.handler, url, top, false)
	a.choSong(t)
	id := f.chenNep(t, 1, func(int) string { return "câu hỏi số 0" })[0]
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, ok := m.lan(0, 0); ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("worker A never started the job; log:\n%s", a.log())
		}
		time.Sleep(10 * time.Millisecond)
	}
	other := New(f.pool, brain.Configured()).WithNepEngine(f.handler.nepEngine)
	b := moTram(t, f, other, url, top, false)
	b.choSong(t)
	stopped := time.Now()
	a.dung()
	if n := f.choXong(t, []string{id}, 5*time.Second); n != 1 {
		status, attempts, seq, _ := f.trangThai(t, id)
		t.Fatalf("released job not run by worker B: status=%s attempts=%d seq=%d; log A:\n%s\nlog B:\n%s", status, attempts, seq, a.log(), b.log())
	}
	again, _ := m.lan(0, 1)
	took := again.Sub(stopped)
	t.Logf("released on stop, reclaimed by another worker through the broker in %v", took.Round(time.Millisecond))
	if took > time.Second {
		t.Fatalf("reclaimed after %v, want ≤1 s", took)
	}
	if _, attempts, seq, _ := f.trangThai(t, id); attempts != 1 || seq != 2 {
		t.Fatalf("attempts=%d seq=%d, want the first attempt given back and a second entry", attempts, seq)
	}
}

// A database that fails under a message: the consumer requeues it and pauses,
// the poller would carry on, and once the database answers the consumer
// comes back and the message runs -- the job is not lost with the poller off.
func TestHangDoiDBLoiThiTamDungRoiTiepTuc(t *testing.T) {
	url := amqpURL(t)
	f := setup(t, nil)
	f.nepTrenEngine(t, llm.NewStub(traLoiGiong(1)...))
	top := topology(t, url)
	tr := moTram(t, f, f.handler, url, top, false)
	tr.choSong(t)
	ctx := context.Background()
	lock, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	id := f.chenNep(t, 1, func(int) string { return "câu hỏi số 0" })[0]
	if _, err = lock.Exec(ctx, `LOCK TABLE chat_ai_invocations IN ACCESS EXCLUSIVE MODE`); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for !bytes.Contains([]byte(tr.log()), []byte("job consumer paused")) {
		if time.Now().After(deadline) {
			_ = lock.Rollback(ctx)
			t.Fatalf("the consumer never paused on the claim timing out; log:\n%s", tr.log())
		}
		time.Sleep(50 * time.Millisecond)
	}
	_ = lock.Rollback(ctx)
	if n := f.choXong(t, []string{id}, 15*time.Second); n != 1 {
		t.Fatalf("the job was lost after the pause; log:\n%s", tr.log())
	}
}

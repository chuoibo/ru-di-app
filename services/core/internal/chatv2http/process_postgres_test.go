//go:build postgres

package chatv2http

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket/wsjson"
	"github.com/jackc/pgx/v5/pgxpool"
	"mobile/services/core/internal/chatv2"
)

const processHelperFlag = "RUDI_CHAT_TEST_CHILD"
const processSchemaEnv = "RUDI_CHAT_TEST_SCHEMA"

type processReady struct {
	URL string `json:"url"`
	PID int    `json:"pid"`
}

// TestChatV2ProcessHelper runs only inside an explicitly spawned test process.
// Credentials remain in the inherited environment, never in arguments/logs.
func TestChatV2ProcessHelper(t *testing.T) {
	if os.Getenv(processHelperFlag) != "1" {
		return
	}
	schema := os.Getenv(processSchemaEnv)
	if !strings.HasPrefix(schema, "chat_http_") || strings.ContainsAny(schema, " ,;\"'") {
		t.Fatal("invalid isolated test schema")
	}
	config, err := pgxpool.ParseConfig(os.Getenv("CORE_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal("invalid child test database configuration")
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	config.MaxConns = 5
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal("child database pool creation failed")
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Fatal("child database unavailable")
	}
	h := New(Options{Store: chatv2.NewStore(pool), BatchSessions: true, Authenticate: Sessions(pool), Experimental: true,
		Context: ctx, ReconcileInterval: 50 * time.Millisecond, OperationTimeout: 2 * time.Second})
	go h.Listen(ctx, pool)
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal("child listener failed")
	}
	server := &http.Server{Handler: h, ReadHeaderTimeout: 2 * time.Second}
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	defer server.Close()
	if err := json.NewEncoder(os.Stdout).Encode(processReady{URL: "http://" + listener.Addr().String(), PID: os.Getpid()}); err != nil {
		t.Fatal("child readiness output failed")
	}
	// Closing the parent-owned pipe shuts down healthy children. Context and
	// CommandContext deadlines bound cleanup even when either side fails.
	eof := make(chan struct{})
	go func() { _, _ = io.Copy(io.Discard, os.Stdin); close(eof) }()
	select {
	case <-eof:
	case <-ctx.Done():
	case <-served:
		t.Error("child HTTP server stopped unexpectedly")
	}
	cancel()
}

type chatProcess struct {
	ready    processReady
	command  *exec.Cmd
	stdin    io.WriteCloser
	done     chan error
	stderr   bytes.Buffer
	stopOnce sync.Once
	cancel   context.CancelFunc
}

func startChatProcess(t *testing.T, f liveWorld) *chatProcess {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	p := &chatProcess{cancel: cancel, done: make(chan error, 1)}
	p.command = exec.CommandContext(ctx, executable, "-test.run=^TestChatV2ProcessHelper$")
	schema := strings.Split(f.pool.Config().ConnConfig.RuntimeParams["search_path"], ",")[0]
	p.command.Env = append(os.Environ(), processHelperFlag+"=1", processSchemaEnv+"="+schema)
	p.command.Stderr = &p.stderr
	stdout, err := p.command.StdoutPipe()
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	p.stdin, err = p.command.StdinPipe()
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	if err = p.command.Start(); err != nil {
		cancel()
		t.Fatal("start child process failed")
	}
	go func() { p.done <- p.command.Wait() }()
	t.Cleanup(func() { p.stop(t, false) })
	ready := make(chan error, 1)
	go func() { ready <- json.NewDecoder(stdout).Decode(&p.ready) }()
	select {
	case err := <-ready:
		if err != nil {
			p.stop(t, true)
			t.Fatalf("child readiness failed: %s", p.stderr.String())
		}
	case <-time.After(5 * time.Second):
		p.stop(t, true)
		t.Fatal("child readiness timed out")
	}
	if !strings.HasPrefix(p.ready.URL, "http://127.0.0.1:") || p.ready.PID == os.Getpid() {
		p.stop(t, true)
		t.Fatal("child did not expose an isolated loopback process")
	}
	return p
}

func (p *chatProcess) stop(t *testing.T, kill bool) {
	t.Helper()
	p.stopOnce.Do(func() {
		if kill {
			if err := p.command.Process.Kill(); err != nil {
				t.Error("writer did not survive until the deliberate kill")
			}
		}
		_ = p.stdin.Close()
		select {
		case err := <-p.done:
			if kill && err == nil {
				t.Error("deliberately killed writer returned success")
			}
			if !kill && err != nil {
				t.Errorf("child exited unexpectedly: %s", p.stderr.String())
			}
		case <-time.After(3 * time.Second):
			p.cancel()
			select {
			case <-p.done:
			case <-time.After(2 * time.Second):
				t.Error("child did not exit after cancellation")
			}
			t.Error("child shutdown timed out")
		}
		p.cancel()
	})
}

// This covers loss of a committed receipt across actual process death. It
// deliberately does not claim to kill the writer inside its SQL transaction.
func TestPostgresSeparateProcessesCommittedRetryAndReconnect(t *testing.T) {
	f := liveSetup(t)
	writer, receiver := startChatProcess(t, f), startChatProcess(t, f)
	if writer.ready.PID == receiver.ready.PID {
		t.Fatal("replicas share a process")
	}
	c := liveDial(t, receiver.ready.URL, f, 1, 0)
	if frame := readFrame(t, c); frame.Type != "ready" {
		t.Fatal("receiver not ready")
	}
	original := f.envelope(0)
	path := "/v2/chat/" + f.conversation + "/events"
	// The transport consumes and deliberately discards the HTTP receipt. The
	// signed envelope remains in the synthetic sender's durable-retry input.
	status, _ := liveRequest(t, writer.ready.URL, f.people[0].token, http.MethodPost, path, original)
	if status != http.StatusCreated {
		t.Fatalf("initial send status=%d", status)
	}
	delivered := readFrame(t, c)
	if delivered.Type != "events" || delivered.Page == nil || len(delivered.Page.Events) != 1 || delivered.Page.NextSequence != 1 {
		t.Fatalf("receiver did not observe sequence 1: %+v", delivered)
	}
	first := delivered.Page.Events[0]
	if first.Envelope == nil || first.Envelope.LogicalSendID != original.LogicalSendID || !bytes.Equal(first.Envelope.Ciphertext, original.Ciphertext) {
		t.Fatal("receiver saw a different envelope")
	}
	ackCtx, cancelAck := context.WithTimeout(context.Background(), 2*time.Second)
	if err := wsjson.Write(ackCtx, c, ack{Type: "ack", Sequence: 1}); err != nil {
		cancelAck()
		t.Fatal(err)
	}
	cancelAck()
	c.CloseNow()
	writer.stop(t, true)
	restarted := startChatProcess(t, f)
	status, raw := liveRequest(t, restarted.ready.URL, f.people[0].token, http.MethodPost, path, original)
	var replay chatv2.SendResult
	if err := json.Unmarshal(raw, &replay); err != nil {
		t.Fatal(err)
	}
	expected, _ := json.Marshal(first)
	actual, _ := json.Marshal(replay.Event)
	if status != http.StatusOK || !replay.Replayed || !bytes.Equal(expected, actual) {
		t.Fatalf("restart did not replay the exact committed event: status=%d replay=%t", status, replay.Replayed)
	}
	var events, outbox, sends int
	if err := f.pool.QueryRow(context.Background(), `SELECT
  (SELECT count(*) FROM chat_v2_events WHERE context_id=$1),
  (SELECT count(*) FROM chat_v2_outbox WHERE context_id=$1),
  (SELECT count(*) FROM chat_v2_sends WHERE context_id=$1)`, f.conversation).Scan(&events, &outbox, &sends); err != nil {
		t.Fatal(err)
	}
	if events != 1 || outbox != 1 || sends != 1 {
		t.Fatalf("retry duplicated rows: events=%d outbox=%d sends=%d", events, outbox, sends)
	}
	second := f.envelope(2)
	status, _ = liveRequest(t, restarted.ready.URL, f.people[2].token, http.MethodPost, path, second)
	if status != http.StatusCreated {
		t.Fatalf("second send status=%d", status)
	}
	resumed := liveDial(t, receiver.ready.URL, f, 1, 1)
	if frame := readFrame(t, resumed); frame.Type != "ready" {
		t.Fatal("reconnected receiver not ready")
	}
	catchup := readFrame(t, resumed)
	if catchup.Page == nil || len(catchup.Page.Events) != 1 || catchup.Page.Events[0].Sequence != 2 || catchup.Page.NextSequence != 2 || catchup.Page.HasMore {
		t.Fatalf("cursor catch-up did not return only sequence 2: %+v", catchup)
	}
	if e := catchup.Page.Events[0].Envelope; e == nil || e.LogicalSendID != second.LogicalSendID {
		t.Fatal("reconnect returned wrong envelope")
	}
	resumed.CloseNow()
	t.Logf("separate process delivery and committed retry verified; original writer pid=%d receiver pid=%d restarted writer pid=%d", writer.ready.PID, receiver.ready.PID, restarted.ready.PID)
}

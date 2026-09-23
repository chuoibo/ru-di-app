//go:build postgres

package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"mobile/services/core/internal/testdb"
)

// These cases start the real `core serve` path -- environment, manifest, flag
// rule, schema check, mount -- against the tier's database, and ask the
// running front door whether the AI engine and the change feed answer. Only a
// running server can show the mount; the decision table alone cannot.

type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

// chatSchemaURL returns a database URL whose search_path starts at a fresh
// schema holding copies of the tables the chat migrations touch. The capture
// triggers then land on the copies, never on the public tables other packages
// of the tier are writing to at the same time.
func chatSchemaURL(t *testing.T) string {
	t.Helper()
	base := testdb.Pool(t)
	raw := strings.TrimSpace(os.Getenv("CORE_TEST_DATABASE_URL"))
	ctx := context.Background()
	var suffix [8]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("core_chat_boot_%x", suffix)
	ident := pgx.Identifier{schema}.Sanitize()
	if _, err := base.Exec(ctx, "CREATE SCHEMA "+ident); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := base.Exec(context.Background(), "DROP SCHEMA "+ident+" CASCADE"); err != nil {
			t.Error(err)
		}
	})
	config := base.Config().Copy()
	config.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	for _, table := range []string{"people", "contexts", "memberships", "friend_requests", "account_sessions", "messages", "message_reactions", "votes", "vote_options", "vote_ballots", "places", "destinations", "outings", "outing_stops"} {
		if _, err := pool.Exec(ctx, fmt.Sprintf("CREATE TABLE %s (LIKE public.%s INCLUDING ALL)", table, table)); err != nil {
			t.Fatal(err)
		}
	}
	separator := "?"
	if strings.Contains(raw, "?") {
		separator = "&"
	}
	return raw + separator + "search_path=" + url.QueryEscape(schema+",public")
}

func freeAddress(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().String()
}

// pythonStub stands in for the API behind core and records what reached it.
type pythonStub struct {
	*httptest.Server
	mu   sync.Mutex
	hits []string
}

func newPythonStub(t *testing.T) *pythonStub {
	stub := &pythonStub{}
	stub.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stub.mu.Lock()
		stub.hits = append(stub.hits, r.URL.Path)
		stub.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"detail":"python stub"}`)
	}))
	t.Cleanup(stub.Close)
	return stub
}

func (s *pythonStub) saw(path string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, hit := range s.hits {
		if hit == path {
			return true
		}
	}
	return false
}

type runningCore struct {
	base string
	logs *lockedBuffer
	stop func() int
}

// startCore runs serveUntil with exactly env and nothing else, and waits for
// liveness. It fails the test if core exits first.
func startCore(t *testing.T, env map[string]string) runningCore {
	t.Helper()
	listen, live := freeAddress(t), freeAddress(t)
	full := map[string]string{"MOBILE_CORE_LISTEN": listen, "MOBILE_CORE_LIVENESS_LISTEN": live}
	for k, v := range env {
		full[k] = v
	}
	logs := &lockedBuffer{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	go func() { done <- serveUntil(ctx, func(k string) string { return full[k] }, logs) }()
	var once sync.Once
	var code int
	stop := func() int {
		once.Do(func() { cancel(); code = <-done })
		return code
	}
	t.Cleanup(func() { stop() })
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case code := <-done:
			// Hand the exit code back so the cleanup's stop() does not wait
			// forever on a server that already returned.
			done <- code
			t.Fatalf("core exited %d before serving; log:\n%s", code, logs.String())
		default:
		}
		// Liveness and the public door are two listeners started side by
		// side; /livez answering says nothing about the other one. Measured:
		// a request sent on the first 200 from /livez was refused once in a
		// full tier run. Wait for both.
		if live200(live) && accepts(listen) {
			return runningCore{base: "http://" + listen, logs: logs, stop: stop}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("core never became live; log:\n%s", logs.String())
	return runningCore{}
}

func live200(address string) bool {
	resp, err := http.Get("http://" + address + "/livez")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func accepts(address string) bool {
	conn, err := net.DialTimeout("tcp", address, time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func getCode(t *testing.T, target string) (int, string) {
	t.Helper()
	resp, err := http.Get(target)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var body struct {
		Code string `json:"code"`
	}
	raw, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(raw, &body)
	return resp.StatusCode, body.Code
}

const bootRoom = "7b0c6c1e-2f43-4c55-9a8e-1d2f3a4b5c6d"

func migrateChatInto(t *testing.T, databaseURL string) {
	t.Helper()
	var out, errs bytes.Buffer
	// No flag in the environment: the command must not ask for one.
	if code := run([]string{"migrate-chat"}, func(k string) string {
		if k == "MOBILE_DATABASE_URL" {
			return databaseURL
		}
		return ""
	}, &out, &errs); code != 0 {
		t.Fatalf("migrate-chat exit %d: %s", code, errs.String())
	}
}

// No MOBILE_CHAT_CHANGES_CANDIDATE and no MOBILE_AUTH_MODE: prod, and the
// engine and feed answer from Go. Python never sees the request.
func TestChatFeaturesMountedWithNoFlagInProd(t *testing.T) {
	databaseURL := chatSchemaURL(t)
	migrateChatInto(t, databaseURL)
	python := newPythonStub(t)
	core := startCore(t, map[string]string{
		"MOBILE_PYTHON_UPSTREAM": python.URL,
		"MOBILE_DATABASE_URL":    databaseURL,
	})
	for _, path := range []string{"/contexts/" + bootRoom + "/chat-capabilities", "/contexts/" + bootRoom + "/ai-invocations", "/contexts/" + bootRoom + "/changes"} {
		status, code := getCode(t, core.base+path)
		if status != http.StatusUnauthorized || code != "authentication_required" {
			t.Errorf("%s: %d %q, want the Go handler's 401 authentication_required", path, status, code)
		}
		if python.saw(path) {
			t.Errorf("%s reached Python: the chat features are not mounted", path)
		}
	}
	if !strings.Contains(core.logs.String(), `"chat_features":true`) {
		t.Errorf("startup log does not report the features on:\n%s", core.logs.String())
	}
	if code := core.stop(); code != 0 {
		t.Errorf("core stopped with %d", code)
	}
}

// "0" is the explicit way out, and dev with no flag is off: nothing is
// mounted, the request goes to Python as any unknown path does, and the
// startup log says why. Off never needs the chat schema -- `make up` on a dev
// stack must come up whether or not migrate-chat ever ran -- and having the
// schema does not turn anything on.
func TestChatFeaturesNotMountedWhenTurnedOff(t *testing.T) {
	type offCase struct {
		name, flag, mode, reason string
		migrated                 bool
	}
	var cases []offCase
	for _, migrated := range []bool{true, false} {
		schema := "without schema"
		if migrated {
			schema = "with schema"
		}
		cases = append(cases,
			offCase{name: "flag 0 in prod " + schema, flag: "0", reason: "MOBILE_CHAT_CHANGES_CANDIDATE=0", migrated: migrated},
			offCase{name: "unset in dev " + schema, mode: "dev", reason: "MOBILE_AUTH_MODE=dev", migrated: migrated},
		)
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			databaseURL := chatSchemaURL(t)
			if c.migrated {
				migrateChatInto(t, databaseURL)
			}
			python := newPythonStub(t)
			core := startCore(t, map[string]string{
				"MOBILE_PYTHON_UPSTREAM":        python.URL,
				"MOBILE_DATABASE_URL":           databaseURL,
				"MOBILE_CHAT_CHANGES_CANDIDATE": c.flag,
				"MOBILE_AUTH_MODE":              c.mode,
			})
			for _, path := range []string{"/contexts/" + bootRoom + "/chat-capabilities", "/contexts/" + bootRoom + "/changes"} {
				status, _ := getCode(t, core.base+path)
				if status != http.StatusNotFound || !python.saw(path) {
					t.Errorf("%s: %d, python saw=%v; want it proxied to Python", path, status, python.saw(path))
				}
			}
			logs := core.logs.String()
			if !strings.Contains(logs, `"level":"WARN"`) || !strings.Contains(logs, "group AI and the realtime chat feed are off") || !strings.Contains(logs, c.reason) {
				t.Errorf("no warning naming %q:\n%s", c.reason, logs)
			}
			if code := core.stop(); code != 0 {
				t.Errorf("core stopped with %d", code)
			}
		})
	}
}

// On by default with the schema missing is a refusal that names both fixes,
// never a front door that is healthy and 404s the AI.
func TestChatFeaturesRefuseToStartWithoutSchema(t *testing.T) {
	databaseURL := chatSchemaURL(t)
	python := newPythonStub(t)
	logs := &lockedBuffer{}
	env := map[string]string{
		"MOBILE_CORE_LISTEN":          freeAddress(t),
		"MOBILE_CORE_LIVENESS_LISTEN": freeAddress(t),
		"MOBILE_PYTHON_UPSTREAM":      python.URL,
		"MOBILE_DATABASE_URL":         databaseURL,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if code := serveUntil(ctx, func(k string) string { return env[k] }, logs); code != 1 {
		t.Fatalf("exit %d without the chat schema; log:\n%s", code, logs.String())
	}
	for _, want := range []string{"the chat schema is missing", "migrate-chat", "MOBILE_CHAT_CHANGES_CANDIDATE=0"} {
		if !strings.Contains(logs.String(), want) {
			t.Errorf("refusal does not mention %q:\n%s", want, logs.String())
		}
	}
}

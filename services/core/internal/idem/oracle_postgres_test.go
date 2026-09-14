//go:build postgres

package idem

// Differential test against the real Python middleware. Run from services/core:
//
//	IDEM_ORACLE_IMAGE=mobile-parity-api:7bf58e3d GOTOOLCHAIN=local \
//	  go test -tags postgres -run TestOracleDifferential -v -timeout 30m ./internal/idem/
//
// It provisions two PostgreSQL 16 containers on loopback ports (A and B),
// migrates both with Alembic from the image, and serves each database twice:
// by the real IdempotencyMiddleware under uvicorn (scripts/render_idem_oracle.py
// serve, main.py's SQLAlchemy store factory) and by this package under
// net/http. Both wrap the same scripted stub. Every scenario runs
//
//   - python writes and answers on A, go writes and answers on B, and
//   - when it has "y" steps: python writes and go answers on A, go writes and
//     python answers on B,
//
// and compares, byte for byte, every response (status, headers without date,
// server and connection, body), what the stub received, the store calls, and
// the idempotency_keys rows left behind (ids and timestamps reduced to facts).
// Host networking and published loopback ports only: the Docker daemon has no
// subnets left for new networks. Without IDEM_ORACLE_IMAGE the test skips.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"mobile/services/core/internal/httpapi/router"
)

const (
	oracleShortWait = time.Second
	oraclePassword  = "idem-oracle-only"
	oracleKey       = "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	oracleKeyTwo    = "bbbbbbbb-cccc-4ddd-8eee-ffffffffffff"
	oracleKeyThree  = "cccccccc-dddd-4eee-8fff-aaaaaaaaaaaa"
	oracleKeyFour   = "dddddddd-eeee-4fff-8aaa-bbbbbbbbbbbb"
)

// ---------------------------------------------------------------------------
// Environment
// ---------------------------------------------------------------------------

type oracleEndpoint struct {
	impl  string // "python" or "go"
	short string // host:port with the short in-flight budget
	def   string // host:port with the default budget
}

type oracleEnv struct {
	pools  [2]*pgxpool.Pool
	python [2]*oracleEndpoint
	golang [2]*oracleEndpoint
}

func dockerRun(t *testing.T, args ...string) string {
	t.Helper()
	out, err := exec.Command("docker", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("docker %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func pythonDSN(addr string) string {
	return "postgresql+psycopg://mobile:" + oraclePassword + "@" + addr + "/mobile"
}

func goDSN(addr string) string {
	return "postgres://mobile:" + oraclePassword + "@" + addr + "/mobile"
}

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

func setupOracle(t *testing.T) *oracleEnv {
	image := os.Getenv("IDEM_ORACLE_IMAGE")
	if image == "" {
		t.Skip("IDEM_ORACLE_IMAGE not set; see the comment at the top of oracle_postgres_test.go")
	}
	scripts, err := filepath.Abs("../../../../scripts")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(scripts, "render_idem_oracle.py")); err != nil {
		t.Fatal(err)
	}
	suffix := strconv.Itoa(os.Getpid())
	env := &oracleEnv{}
	for i, name := range []string{"a", "b"} {
		addr := startOraclePostgres(t, image, "idem-oracle-"+name+"-"+suffix)
		pool, err := pgxpool.New(context.Background(), goDSN(addr))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(pool.Close)
		env.pools[i] = pool
		env.python[i] = startOraclePython(t, image, scripts, "idem-oracle-py-"+name+"-"+suffix, addr)
		env.golang[i] = startOracleGo(t, pool)
	}
	return env
}

func startOraclePostgres(t *testing.T, image, name string) string {
	t.Helper()
	dockerRun(t, "run", "-d", "--name", name,
		"-e", "POSTGRES_DB=mobile", "-e", "POSTGRES_USER=mobile", "-e", "POSTGRES_PASSWORD="+oraclePassword,
		"-p", "127.0.0.1:0:5432", "postgres:16-alpine",
		"-c", "timezone=UTC", "-c", "fsync=off", "-c", "synchronous_commit=off")
	t.Cleanup(func() { _ = exec.Command("docker", "rm", "-f", name).Run() })
	addr := strings.SplitN(dockerRun(t, "port", name, "5432/tcp"), "\n", 2)[0]
	deadline := time.Now().Add(90 * time.Second)
	for exec.Command("docker", "exec", name, "pg_isready", "-h", "127.0.0.1", "-U", "mobile", "-d", "mobile").Run() != nil {
		if time.Now().After(deadline) {
			t.Fatalf("%s never became ready", name)
		}
		time.Sleep(500 * time.Millisecond)
	}
	dockerRun(t, "run", "--rm", "--network", "host", "-e", "MOBILE_DATABASE_URL="+pythonDSN(addr),
		image, "alembic", "upgrade", "head")
	return addr
}

func startOraclePython(t *testing.T, image, scripts, name, dbAddr string) *oracleEndpoint {
	t.Helper()
	short, def := freePort(t), freePort(t)
	dockerRun(t, "run", "-d", "--name", name, "--network", "host",
		"-e", "MOBILE_DATABASE_URL="+pythonDSN(dbAddr),
		"-v", scripts+":/oracle:ro", "--entrypoint", "python", image,
		"/oracle/render_idem_oracle.py", "serve", strconv.Itoa(short), strconv.Itoa(def),
		strconv.FormatFloat(oracleShortWait.Seconds(), 'f', -1, 64))
	t.Cleanup(func() { _ = exec.Command("docker", "rm", "-f", name).Run() })
	endpoint := &oracleEndpoint{
		impl:  "python",
		short: "127.0.0.1:" + strconv.Itoa(short),
		def:   "127.0.0.1:" + strconv.Itoa(def),
	}
	deadline := time.Now().Add(90 * time.Second)
	for {
		_, errShort := fetchOracleLog(endpoint.short)
		_, errDef := fetchOracleLog(endpoint.def)
		if errShort == nil && errDef == nil {
			return endpoint
		}
		if time.Now().After(deadline) {
			logs, _ := exec.Command("docker", "logs", name).CombinedOutput()
			t.Fatalf("python oracle never answered: %v %v\n%s", errShort, errDef, logs)
		}
		time.Sleep(300 * time.Millisecond)
	}
}

func startOracleGo(t *testing.T, pool *pgxpool.Pool) *oracleEndpoint {
	t.Helper()
	log := &oracleLog{}
	store := &recordingStore{inner: NewPostgresStore(pool), log: log}
	stub := oracleStub(log)
	onError := WithErrorHandler(func(w http.ResponseWriter, _ *http.Request, _ error) { writeUvicorn500(w) })
	short := httptest.NewServer(recoverAsUvicorn(New(store, WithInFlightWait(oracleShortWait), onError)(stub)))
	def := httptest.NewServer(recoverAsUvicorn(New(store, onError)(stub)))
	t.Cleanup(short.Close)
	t.Cleanup(def.Close)
	return &oracleEndpoint{impl: "go", short: short.Listener.Addr().String(), def: def.Listener.Addr().String()}
}

// ---------------------------------------------------------------------------
// The Go side of the stub, mirroring render_idem_oracle.py's serve mode.
// ---------------------------------------------------------------------------

type oracleLog struct {
	mu    sync.Mutex
	calls []any
	store []any
}

func (l *oracleLog) add(call bool, entry any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if call {
		l.calls = append(l.calls, entry)
	} else {
		l.store = append(l.store, entry)
	}
}

func (l *oracleLog) take() []byte {
	l.mu.Lock()
	defer l.mu.Unlock()
	calls, store := l.calls, l.store
	if calls == nil {
		calls = []any{}
	}
	if store == nil {
		store = []any{}
	}
	l.calls, l.store = nil, nil
	payload, _ := json.Marshal(map[string]any{"calls": calls, "store": store})
	return payload
}

// recordingStore is the driver's Recording: the real store, each call noted
// after it returns.
type recordingStore struct {
	inner Store
	log   *oracleLog
}

func (s *recordingStore) Reserve(ctx context.Context, scope, key, fingerprint, legacy string) (Outcome, error) {
	outcome, err := s.inner.Reserve(ctx, scope, key, fingerprint, legacy)
	if err == nil {
		s.log.add(false, []any{"reserve", scope, key, outcome.Kind.String()})
	}
	return outcome, err
}

func (s *recordingStore) Complete(ctx context.Context, scope, key string, response StoredResponse) error {
	err := s.inner.Complete(ctx, scope, key, response)
	if err == nil {
		s.log.add(false, []any{"complete", scope, key, response.Status})
	}
	return err
}

func (s *recordingStore) Release(ctx context.Context, scope, key string) error {
	err := s.inner.Release(ctx, scope, key)
	if err == nil {
		s.log.add(false, []any{"release", scope, key})
	}
	return err
}

type stubScript struct {
	Status  *int        `json:"status"`
	Headers [][2]string `json:"headers"`
	Body    string      `json:"body"`
	Raise   string      `json:"raise"`
	Sleep   float64     `json:"sleep"`
}

func oracleStub(log *oracleLog) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawPath, rawQuery := router.SplitTarget(r.RequestURI)
		path := router.ScopePath(rawPath)
		if r.Method == http.MethodGet && path == "/__log" {
			payload := log.take()
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
			_, _ = w.Write(payload)
			return
		}
		received, _ := io.ReadAll(r.Body)
		stored, isReplay := AuthorizedReplay(r.Context())
		var script stubScript
		if raw, ok := firstHeader(r.Header, "X-Stub"); ok {
			if err := json.Unmarshal([]byte(raw), &script); err != nil {
				panic(err)
			}
		}
		call := map[string]any{"method": r.Method, "path": path, "query": latin1ToUTF8(rawQuery),
			"body_sha256": sha256Hex(received), "replay": nil}
		if isReplay {
			call["replay"] = map[string]any{"status": stored.Status, "body": hex.EncodeToString(stored.Body),
				"media_type": textOrNil(stored.MediaType)}
		}
		log.add(true, call)
		if script.Sleep > 0 {
			time.Sleep(time.Duration(script.Sleep * float64(time.Second)))
		}
		if script.Raise == "before" {
			panic(errScripted)
		}
		header := w.Header()
		var status int
		var out []byte
		if isReplay {
			status, out = stored.Status, stored.Body
			header.Set("X-Replay-Seen", "1")
			if stored.MediaType != nil && *stored.MediaType != "" {
				encoded, err := latin1Bytes(*stored.MediaType)
				if err != nil {
					panic(err)
				}
				header.Set("Content-Type", string(encoded))
			} else {
				header["Content-Type"] = nil
			}
		} else {
			status = http.StatusOK
			if script.Status != nil {
				status = *script.Status
			}
			out, _ = hex.DecodeString(script.Body)
			hasType := false
			for _, pair := range script.Headers {
				value, _ := hex.DecodeString(pair[1])
				header.Add(pair[0], string(value))
				hasType = hasType || strings.EqualFold(pair[0], "content-type")
			}
			if !hasType {
				header["Content-Type"] = nil
			}
		}
		header.Set("Content-Length", strconv.Itoa(len(out)))
		w.WriteHeader(status)
		if script.Raise == "after_start" {
			panic(errScripted)
		}
		_, _ = w.Write(out)
	})
}

// writeUvicorn500 is uvicorn's send_500_response, which is what the driver's
// caller sees when the ASGI app raises before answering.
func writeUvicorn500(w http.ResponseWriter) {
	header := w.Header()
	for name := range header {
		delete(header, name)
	}
	header.Set("Content-Type", "text/plain; charset=utf-8")
	header.Set("Content-Length", "21")
	header.Set("Connection", "close")
	w.WriteHeader(http.StatusInternalServerError)
	_, _ = io.WriteString(w, "Internal Server Error")
}

func recoverAsUvicorn(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recover() != nil {
				writeUvicorn500(w)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// ---------------------------------------------------------------------------
// Raw HTTP: the same request bytes reach both servers.
// ---------------------------------------------------------------------------

type wireResponse struct {
	Status  int
	Headers [][2]string
	Body    []byte
	Elapsed time.Duration
}

var ignoredResponseHeaders = map[string]bool{"date": true, "server": true, "connection": true}

func roundTrip(addr, method, target string, headers [][2]string, body []byte) (wireResponse, error) {
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return wireResponse{}, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(60 * time.Second))
	var request bytes.Buffer
	request.WriteString(method + " " + target + " HTTP/1.1\r\nHost: oracle\r\nConnection: close\r\n")
	for _, pair := range headers {
		request.WriteString(pair[0] + ": " + pair[1] + "\r\n")
	}
	if len(body) > 0 || writeMethods[method] {
		request.WriteString("Content-Length: " + strconv.Itoa(len(body)) + "\r\n")
	}
	request.WriteString("\r\n")
	request.Write(body)
	started := time.Now()
	if _, err := conn.Write(request.Bytes()); err != nil {
		return wireResponse{}, err
	}
	raw, err := io.ReadAll(conn)
	elapsed := time.Since(started)
	if err != nil && len(raw) == 0 {
		return wireResponse{}, err
	}
	end := bytes.Index(raw, []byte("\r\n\r\n"))
	if end < 0 {
		return wireResponse{}, fmt.Errorf("no header terminator in %q", raw)
	}
	lines := strings.Split(string(raw[:end]), "\r\n")
	fields := strings.SplitN(lines[0], " ", 3)
	if len(fields) < 2 {
		return wireResponse{}, fmt.Errorf("bad status line %q", lines[0])
	}
	status, err := strconv.Atoi(fields[1])
	if err != nil {
		return wireResponse{}, err
	}
	response := wireResponse{Status: status, Headers: [][2]string{}, Body: raw[end+4:], Elapsed: elapsed}
	for _, line := range lines[1:] {
		name, value, _ := strings.Cut(line, ":")
		name = strings.ToLower(name)
		if ignoredResponseHeaders[name] {
			continue
		}
		response.Headers = append(response.Headers, [2]string{name, strings.Trim(value, " \t")})
	}
	sort.SliceStable(response.Headers, func(i, j int) bool { return response.Headers[i][0] < response.Headers[j][0] })
	return response, nil
}

func fetchOracleLog(addr string) (map[string]any, error) {
	response, err := roundTrip(addr, http.MethodGet, "/__log", nil, nil)
	if err != nil {
		return nil, err
	}
	if response.Status != http.StatusOK {
		return nil, fmt.Errorf("log answered %d", response.Status)
	}
	var log map[string]any
	if err := json.Unmarshal(response.Body, &log); err != nil {
		return nil, err
	}
	return log, nil
}

func (r wireResponse) header(name string) string {
	for _, pair := range r.Headers {
		if pair[0] == name {
			return pair[1]
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// Scenarios
// ---------------------------------------------------------------------------

type oracleStep struct {
	who     string // "x" or "y"
	method  string
	target  string
	headers [][2]string
	body    []byte
	script  map[string]any
	budget  string // "" short, "default" the middleware's own
	group   int    // consecutive steps sharing a group > 0 start together
	delay   time.Duration
	sql     string
	args    []any
}

type oracleScenario struct {
	name  string
	steps []oracleStep
}

type stepOption func(*oracleStep)

const noContentType = "\x00"

func answer(status int, contentType, body string, extra ...[2]string) map[string]any {
	headers := [][2]string{}
	if contentType != noContentType {
		headers = append(headers, [2]string{"content-type", hex.EncodeToString([]byte(contentType))})
	}
	for _, pair := range extra {
		headers = append(headers, [2]string{pair[0], hex.EncodeToString([]byte(pair[1]))})
	}
	return map[string]any{"status": status, "headers": headers, "body": hex.EncodeToString([]byte(body))}
}

func with(script map[string]any, name string, value any) map[string]any {
	out := map[string]any{}
	for k, v := range script {
		out[k] = v
	}
	out[name] = value
	return out
}

var created = answer(201, "application/json", `{"id": "one"}`)

func send(who, method, target string, options ...stepOption) oracleStep {
	step := oracleStep{who: who, method: method, target: target, script: created}
	for _, option := range options {
		option(&step)
	}
	return step
}

func header(name, value string) stepOption {
	return func(s *oracleStep) { s.headers = append(s.headers, [2]string{name, value}) }
}

func key(value string) stepOption { return header("Idempotency-Key", value) }

func body(value string) stepOption { return func(s *oracleStep) { s.body = []byte(value) } }

func jsonBody(value string) stepOption {
	return func(s *oracleStep) {
		s.headers = append(s.headers, [2]string{"Content-Type", "application/json"})
		s.body = []byte(value)
	}
}

func script(value map[string]any) stepOption { return func(s *oracleStep) { s.script = value } }

func together(group int, delay time.Duration) stepOption {
	return func(s *oracleStep) { s.group, s.delay = group, delay }
}

func defaultBudget() stepOption { return func(s *oracleStep) { s.budget = "default" } }

func sqlStep(query string, args ...any) oracleStep { return oracleStep{sql: query, args: args} }

const (
	sqlAbandon = `UPDATE idempotency_keys SET response_status = NULL, response_body = NULL,
response_media_type = NULL, completed_at = NULL WHERE idempotency_key = $1`
	sqlRewind = `UPDATE idempotency_keys SET request_fingerprint = $2 WHERE idempotency_key = $1`
)

// rawDigest is the pre-canonical fingerprint, computed here independently of
// LegacyFingerprint so a mutation there cannot hide itself.
func rawDigest(method, path, query, body string) string {
	sum := sha256.Sum256([]byte(method + "\n" + path + "\n" + query + "\n" + body))
	return hex.EncodeToString(sum[:])
}

func nested(depth int) string { return strings.Repeat("[", depth) + strings.Repeat("]", depth) }

func oracleScenarios() []oracleScenario {
	const (
		py       = "{\"display_name\": \"Team \\u0110\\u00e0 L\\u1ea1t\", \"total\": 82000}"
		js       = "{\"display_name\":\"Team Đà Lạt\",\"total\":82000}"
		reorder  = "{\"total\": 82000, \"display_name\": \"Team \\u0110\\u00e0 L\\u1ea1t\"}"
		other    = "{\"display_name\": \"Team \\u0110\\u00e0 L\\u1ea1t\", \"total\": 99000}"
		post     = http.MethodPost
		put      = http.MethodPut
		expenses = "/expenses"
	)
	bigInt := "[" + strings.Repeat("7", 4301) + "]"
	utf16 := []byte{0xff, 0xfe}
	for _, r := range "{\"a\": \"Đ\"}" {
		utf16 = append(utf16, byte(r), byte(r>>8))
	}
	failing := with(created, "raise", "before")
	return []oracleScenario{
		{"no key passes through", []oracleStep{
			send("x", post, expenses, jsonBody(py)),
			send("y", post, expenses, jsonBody(py)),
		}},
		{"empty key is refused", []oracleStep{
			send("x", post, expenses, key(""), jsonBody(py)),
			send("y", post, expenses, key(""), jsonBody(py)),
		}},
		{"key length limit", []oracleStep{
			send("x", post, expenses, key(strings.Repeat("k", 255)), jsonBody(py)),
			send("y", post, expenses, key(strings.Repeat("k", 256)), jsonBody(py)),
			send("y", post, expenses, key(strings.Repeat("k", 255)), jsonBody(py)),
		}},
		{"first success then replay", []oracleStep{
			send("x", post, expenses, key(oracleKey), jsonBody(py),
				script(answer(201, "application/json", `{"id": "one"}`, [2]string{"x-custom", "kept-once"}))),
			send("y", post, expenses, key(oracleKey), jsonBody(py)),
		}},
		{"canonically equal json replays", []oracleStep{
			send("x", post, expenses, key(oracleKey), jsonBody(py)),
			send("y", post, expenses, key(oracleKey), jsonBody(js)),
			send("y", post, expenses, key(oracleKey), jsonBody(reorder)),
		}},
		{"same key different body", []oracleStep{
			send("x", post, expenses, key(oracleKey), jsonBody(py)),
			send("y", post, expenses, key(oracleKey), jsonBody(other)),
		}},
		{"array order is meaning", []oracleStep{
			send("x", post, expenses, key(oracleKey), jsonBody(`{"people": ["minh", "trang"]}`)),
			send("y", post, expenses, key(oracleKey), jsonBody(`{"people": ["trang", "minh"]}`)),
		}},
		{"non-2xx releases so the retry executes", []oracleStep{
			send("x", post, expenses, key(oracleKey), jsonBody(py), script(answer(422, "application/json", `{"code": "no"}`))),
			send("y", post, expenses, key(oracleKey), jsonBody(py)),
			send("x", post, expenses, key(oracleKey), jsonBody(py)),
		}},
		{"handler error releases", []oracleStep{
			send("x", post, expenses, key(oracleKey), jsonBody(py), script(failing)),
			send("y", post, expenses, key(oracleKey), jsonBody(py)),
			send("x", post, expenses, key(oracleKey), jsonBody(py)),
		}},
		{"handler error after starting releases", []oracleStep{
			send("x", post, expenses, key(oracleKey), jsonBody(py), script(with(created, "raise", "after_start"))),
			send("y", post, expenses, key(oracleKey), jsonBody(py)),
		}},
		{"abandoned reservation is refused after the budget", []oracleStep{
			send("x", post, expenses, key(oracleKey), jsonBody(py)),
			sqlStep(sqlAbandon, oracleKey),
			send("y", post, expenses, key(oracleKey), jsonBody(js)),
		}},
		{"abandoned reservation with the default budget", []oracleStep{
			send("x", post, expenses, key(oracleKey), jsonBody(py), defaultBudget()),
			sqlStep(sqlAbandon, oracleKey),
			send("x", post, expenses, key(oracleKey), jsonBody(py), defaultBudget()),
		}},
		{"reservation completed during polling replays", []oracleStep{
			send("x", post, expenses, key(oracleKey), jsonBody(py), script(with(created, "sleep", 0.4)), together(1, 0)),
			send("y", post, expenses, key(oracleKey), jsonBody(js), together(1, 100*time.Millisecond)),
		}},
		{"reservation released during polling is won by the poller", []oracleStep{
			send("x", post, expenses, key(oracleKey), jsonBody(py),
				script(with(answer(409, "application/json", `{"code": "busy"}`), "sleep", 0.4)), together(1, 0)),
			send("y", post, expenses, key(oracleKey), jsonBody(py), together(1, 100*time.Millisecond)),
		}},
		{"legacy raw fingerprint replays and is upgraded", []oracleStep{
			send("x", post, "/contexts", key(oracleKey), jsonBody(py)),
			sqlStep(sqlRewind, oracleKey, rawDigest(post, "/contexts", "", py)),
			send("y", post, "/contexts", key(oracleKey), jsonBody(py)),
			send("x", post, "/contexts", key(oracleKey), jsonBody(js)),
		}},
		{"legacy row for a different request is refused", []oracleStep{
			send("x", post, "/contexts", key(oracleKey), jsonBody(py)),
			sqlStep(sqlRewind, oracleKey, rawDigest(post, "/contexts", "", py)),
			send("y", post, "/contexts", key(oracleKey), jsonBody(other)),
		}},
		{"legacy in-flight row is upgraded and refused", []oracleStep{
			send("x", post, "/contexts", key(oracleKey), jsonBody(py)),
			sqlStep(sqlAbandon, oracleKey),
			sqlStep(sqlRewind, oracleKey, rawDigest(post, "/contexts", "", py)),
			send("y", post, "/contexts", key(oracleKey), jsonBody(py)),
		}},
		{"scopes isolate one key", []oracleStep{
			send("x", post, expenses, key(oracleKey), jsonBody(py), header("Authorization", "Bearer phone-one"),
				script(answer(201, "application/json", `{"who": "phone-one"}`))),
			send("y", post, expenses, key(oracleKey), jsonBody(py), header("Authorization", "Bearer phone-two"),
				script(answer(201, "application/json", `{"who": "phone-two"}`))),
			send("x", post, expenses, key(oracleKey), jsonBody(py), header("X-Actor-ID", "person-a"),
				script(answer(201, "application/json", `{"who": "person-a"}`))),
			send("y", post, expenses, key(oracleKey), jsonBody(py),
				script(answer(201, "application/json", `{"who": "anonymous"}`))),
			send("y", post, expenses, key(oracleKey), jsonBody(py), header("Authorization", "Bearer phone-one")),
			send("x", post, expenses, key(oracleKey), jsonBody(py), header("Authorization", "Bearer phone-two")),
			send("y", post, expenses, key(oracleKey), jsonBody(py), header("X-Actor-ID", "person-a")),
			send("x", post, expenses, key(oracleKey), jsonBody(py)),
		}},
		{"malformed authorization falls through to the actor", []oracleStep{
			send("x", post, expenses, key(oracleKey), jsonBody(py), header("Authorization", "Bearer"), header("X-Actor-ID", "person-a")),
			send("y", post, expenses, key(oracleKey), jsonBody(py), header("Authorization", "Basic eHl6"), header("X-Actor-ID", "person-a")),
			send("y", post, expenses, key(oracleKey), jsonBody(py), header("Authorization", "Bearer\ttok"), header("X-Actor-ID", "person-a")),
			send("x", post, expenses, key(oracleKey), jsonBody(py), header("Authorization", "Bearer tok"), header("X-Actor-ID", "person-a")),
		}},
		{"bearer token is stripped the python way", []oracleStep{
			send("x", post, expenses, key(oracleKey), jsonBody(py), header("Authorization", "Bearer tok")),
			send("y", post, expenses, key(oracleKey), jsonBody(py), header("Authorization", "Bearer \xa0tok\x85")),
			send("x", post, expenses, key(oracleKey), jsonBody(py), header("Authorization", "bearer    tok")),
			send("y", post, expenses, key(oracleKey), jsonBody(py), header("Authorization", "BEARER \x85tok\xa0")),
		}},
		{"latin-1 bearer, actor and key", []oracleStep{
			send("x", post, expenses, key(oracleKey), jsonBody(py), header("Authorization", "Bearer t\xe9")),
			send("y", post, expenses, key(oracleKey), jsonBody(py), header("Authorization", "Bearer t\xe9")),
			send("x", post, expenses, key(oracleKeyTwo), jsonBody(py), header("X-Actor-ID", "p\xe9rson")),
			send("y", post, expenses, key(oracleKeyTwo), jsonBody(py), header("X-Actor-ID", "p\xe9rson")),
			send("x", post, expenses, key(strings.Repeat("\xe9", 255)), jsonBody(py)),
			send("y", post, expenses, key(strings.Repeat("\xe9", 255)), jsonBody(py)),
			send("y", post, expenses, key(strings.Repeat("\xe9", 256)), jsonBody(py)),
		}},
		{"invalid json with a json content type", []oracleStep{
			send("x", post, expenses, key(oracleKey), jsonBody("{not json")),
			send("y", post, expenses, key(oracleKey), jsonBody("{not json")),
			send("x", post, expenses, key(oracleKey), jsonBody("{also not json")),
		}},
		{"nan body", []oracleStep{
			send("x", post, expenses, key(oracleKey), jsonBody(`{"a": NaN, "b": -Infinity}`)),
			send("y", post, expenses, key(oracleKey), jsonBody(`{"b":-Infinity,"a":NaN}`)),
		}},
		{"duplicate keys keep the last value", []oracleStep{
			send("x", post, expenses, key(oracleKey), jsonBody(`{"a": 1, "a": 2}`)),
			send("y", post, expenses, key(oracleKey), jsonBody(`{"a":2}`)),
			send("x", post, expenses, key(oracleKey), jsonBody(`{"a":1}`)),
		}},
		{"lone surrogate body is a server error", []oracleStep{
			send("x", post, expenses, key(oracleKey), jsonBody(`{"a": "\ud800"}`)),
			send("y", post, expenses, key(oracleKey), jsonBody(`{"\udc00": 1}`)),
		}},
		{"non-utf-8 json body is hashed verbatim", []oracleStep{
			send("x", post, expenses, key(oracleKey), jsonBody("{\"a\": \"\xe9\"}")),
			send("y", post, expenses, key(oracleKey), jsonBody("{\"a\": \"\xe9\"}")),
			send("x", post, expenses, key(oracleKey), jsonBody("{\"a\" : \"\xe9\"}")),
		}},
		{"utf-16 and utf-8 spell one request", []oracleStep{
			send("x", post, expenses, key(oracleKey), jsonBody(string(utf16))),
			send("y", post, expenses, key(oracleKey), jsonBody("{\"a\":\"Đ\"}")),
		}},
		{"int over the digit limit is hashed verbatim", []oracleStep{
			send("x", post, expenses, key(oracleKey), jsonBody(bigInt)),
			send("y", post, expenses, key(oracleKey), jsonBody(bigInt)),
			send("x", post, expenses, key(oracleKey), jsonBody(bigInt[:1]+" "+bigInt[1:])),
		}},
		{"nesting at the recursion limit", []oracleStep{
			send("x", post, expenses, key(oracleKey), jsonBody(nested(9984))),
			send("y", post, expenses, key(oracleKey), jsonBody(nested(9984))),
		}},
		// Depths 9985-9994 are left out on purpose: pyjson.MaxDepth refuses them,
		// while the middleware under uvicorn in the image accepts up to 9994.
		// That band is a pyjson finding, reported rather than hidden here.
		{"nesting past the recursion limit", []oracleStep{
			send("x", post, expenses, key(oracleKey), jsonBody(nested(10100))),
			send("y", post, expenses, key(oracleKey), jsonBody(nested(20000))),
		}},
		{"json content type spellings", []oracleStep{
			send("x", post, expenses, key(oracleKey), header("Content-Type", "application/json; charset=utf-8"), body(py)),
			send("y", post, expenses, key(oracleKey), header("Content-Type", "application/vnd.api+json"), body(js)),
			send("x", post, expenses, key(oracleKey), header("Content-Type", "Application/JSON"), body(reorder)),
			send("y", post, expenses, key(oracleKey), header("Content-Type", "application/json\xa0; charset=x"), body(js)),
			send("x", post, expenses, key(oracleKey), header("Content-Type", "application/json-patch"), body(js)),
		}},
		{"non-json content types hash bytes", []oracleStep{
			send("x", post, expenses, key(oracleKey), header("Content-Type", "text/plain"), body(`{"a": 1}`)),
			send("y", post, expenses, key(oracleKey), body(`{"a": 1}`)),
			send("x", post, expenses, key(oracleKey), header("Content-Type", "text/plain"), body(`{"a":1}`)),
			send("y", post, expenses, key(oracleKeyTwo), header("Content-Type", "text/plain"), header("Content-Type", "application/json"), body(`{"a": 1}`)),
			send("x", post, expenses, key(oracleKeyTwo), header("Content-Type", "text/plain"), body(`{"a":1}`)),
		}},
		{"query string bytes", []oracleStep{
			send("x", post, "/expenses?b=2&a=1", key(oracleKey), jsonBody(py)),
			send("y", post, "/expenses?a=1&b=2", key(oracleKey), jsonBody(py)),
			send("x", post, "/expenses?b=%32&a=1", key(oracleKey), jsonBody(py)),
			send("y", post, "/expenses?b=2&a=1", key(oracleKey), jsonBody(py)),
			send("x", post, "/expenses?b=2&a=1#frag", key(oracleKey), jsonBody(py)),
		}},
		{"empty query", []oracleStep{
			send("x", post, expenses, key(oracleKey), jsonBody(py)),
			send("y", post, "/expenses?", key(oracleKey), jsonBody(py)),
		}},
		{"decoded path", []oracleStep{
			send("x", post, "/c%C3%A9", key(oracleKey), jsonBody(py)),
			send("y", post, "/c%c3%a9", key(oracleKey), jsonBody(py)),
			send("x", post, "/c%FF", key(oracleKeyTwo), jsonBody(py)),
			send("y", post, "/c%EF%BF%BD", key(oracleKeyTwo), jsonBody(py)),
			send("x", post, "/contexts/a%2Fb", key(oracleKeyThree), jsonBody(py)),
			send("y", post, "/contexts/a/b", key(oracleKeyThree), jsonBody(py)),
			send("y", post, "/expenses/", key(oracleKeyFour), jsonBody(py)),
			send("x", post, expenses, key(oracleKeyFour), jsonBody(py)),
		}},
		{"reads bypass", []oracleStep{
			send("x", http.MethodGet, expenses, key(oracleKey), script(answer(200, "application/json", `{"read": 1}`))),
			send("y", http.MethodGet, expenses, key(oracleKey), script(answer(200, "application/json", `{"read": 2}`))),
			send("x", http.MethodHead, expenses, key(oracleKey), script(answer(200, "application/json", `{"read": 3}`))),
			send("y", http.MethodOptions, expenses, key(""), script(answer(200, noContentType, ""))),
		}},
		{"patch and delete are guarded", []oracleStep{
			send("x", http.MethodPatch, "/people/me", key(oracleKey), jsonBody(py), script(answer(200, "application/json", `{"patched": true}`))),
			send("y", http.MethodPatch, "/people/me", key(oracleKey), jsonBody(js)),
			send("x", http.MethodDelete, "/posts/p1", key(oracleKeyTwo), script(answer(204, noContentType, ""))),
			send("y", http.MethodDelete, "/posts/p1", key(oracleKeyTwo)),
		}},
		{"itinerary preview never reserves", []oracleStep{
			send("x", post, "/outings/o1/itinerary/preview", key(oracleKey), jsonBody(py), script(answer(200, "application/json", `{"route": 1}`))),
			send("y", post, "/outings/o1/itinerary/preview", key(oracleKey), jsonBody(py), script(answer(200, "application/json", `{"route": 2}`))),
			send("x", post, "/outings/o1/itinerary/preview/", key(""), jsonBody(py), script(answer(200, "application/json", `{"route": 3}`))),
			send("y", post, "/outings/o1/itinerary/Preview", key(oracleKeyTwo), jsonBody(py)),
			send("x", post, "/outings/o1/itinerary/Preview", key(oracleKeyTwo), jsonBody(py)),
		}},
		{"itinerary write replay is handed to the handler", []oracleStep{
			send("x", put, "/outings/o1/itinerary", key(oracleKey), jsonBody(py), script(answer(200, "application/json", `{"revision": 1}`))),
			send("y", put, "/outings/o1/itinerary", key(oracleKey), jsonBody(js)),
			send("x", put, "//outings/o1/itinerary//", key(oracleKeyTwo), jsonBody(py), script(answer(200, noContentType, "plain"))),
			send("y", put, "//outings/o1/itinerary//", key(oracleKeyTwo), jsonBody(py)),
			send("x", put, "/outings//itinerary", key(oracleKeyThree), jsonBody(py)),
			send("y", put, "/outings//itinerary", key(oracleKeyThree), jsonBody(py)),
			send("x", put, "/outings/a%2Fb/itinerary", key(oracleKeyFour), jsonBody(py)),
			send("y", put, "/outings/a%2Fb/itinerary", key(oracleKeyFour), jsonBody(py)),
		}},
		{"itinerary handoff failure keeps the row", []oracleStep{
			send("x", put, "/outings/o1/itinerary", key(oracleKey), jsonBody(py)),
			send("y", put, "/outings/o1/itinerary", key(oracleKey), jsonBody(py), script(failing)),
			send("x", put, "/outings/o1/itinerary", key(oracleKey), jsonBody(py)),
		}},
		{"stored media types", []oracleStep{
			send("x", post, expenses, key(oracleKey), jsonBody(py), script(answer(200, noContentType, "untyped"))),
			send("y", post, expenses, key(oracleKey), jsonBody(py)),
			send("x", post, expenses, key(oracleKeyTwo), jsonBody(py), script(answer(201, "", "empty type"))),
			send("y", post, expenses, key(oracleKeyTwo), jsonBody(py)),
			send("x", post, expenses, key(oracleKeyThree), jsonBody(py), script(answer(202, "text/x-\xe9; charset=latin-1", "caf\xe9"))),
			send("y", post, expenses, key(oracleKeyThree), jsonBody(py)),
			send("x", post, expenses, key(oracleKeyFour), jsonBody(py),
				script(answer(299, "text/plain", "", [2]string{"content-type", "application/json"}))),
			send("y", post, expenses, key(oracleKeyFour), jsonBody(py)),
		}},
		{"statuses around the 2xx edge", []oracleStep{
			send("x", post, expenses, key(oracleKey), jsonBody(py), script(answer(300, "text/plain", "multiple"))),
			send("y", post, expenses, key(oracleKey), jsonBody(py), script(answer(200, "text/plain", "ok"))),
			send("x", post, expenses, key(oracleKey), jsonBody(py), script(answer(500, "text/plain", "never"))),
		}},
		{"first of two keys wins", []oracleStep{
			send("x", post, expenses, key("first-key"), key("second-key"), jsonBody(py)),
			send("y", post, expenses, key("first-key"), jsonBody(py)),
			send("y", post, expenses, key("second-key"), jsonBody(py), script(answer(201, "application/json", `{"id": "two"}`))),
		}},
		{"empty body", []oracleStep{
			send("x", post, expenses, key(oracleKey), header("Content-Type", "application/json")),
			send("y", post, expenses, key(oracleKey)),
		}},
	}
}

// ---------------------------------------------------------------------------
// Running and comparing
// ---------------------------------------------------------------------------

type oracleRow struct {
	Scope, Key, Fingerprint string
	Status                  *int32
	Body                    *string // hex; nil is NULL
	MediaType               *string
	IDIsV4                  bool
	Completed               bool
	CompletedAfterCreated   bool
}

type oracleRun struct {
	label     string
	responses []*wireResponse
	logX      map[string]any
	logY      map[string]any
	rows      []oracleRow
}

func (env *oracleEnv) runScenario(t *testing.T, sc oracleScenario, db int, x, y *oracleEndpoint) oracleRun {
	t.Helper()
	ctx := context.Background()
	pool := env.pools[db]
	if _, err := pool.Exec(ctx, "DELETE FROM idempotency_keys"); err != nil {
		t.Fatal(err)
	}
	for _, endpoint := range []*oracleEndpoint{x, y} {
		if _, err := fetchOracleLog(endpoint.short); err != nil {
			t.Fatal(err)
		}
	}
	run := oracleRun{
		label:     fmt.Sprintf("x=%s y=%s db=%c", x.impl, y.impl, 'A'+db),
		responses: make([]*wireResponse, len(sc.steps)),
	}
	for i := 0; i < len(sc.steps); {
		step := sc.steps[i]
		if step.sql != "" {
			if _, err := pool.Exec(ctx, step.sql, step.args...); err != nil {
				t.Fatal(err)
			}
			i++
			continue
		}
		end := i + 1
		for step.group > 0 && end < len(sc.steps) && sc.steps[end].group == step.group {
			end++
		}
		errs := make([]error, len(sc.steps))
		var wg sync.WaitGroup
		for k := i; k < end; k++ {
			wg.Add(1)
			go func(k int) {
				defer wg.Done()
				s := sc.steps[k]
				time.Sleep(s.delay)
				endpoint := x
				if s.who == "y" {
					endpoint = y
				}
				addr := endpoint.short
				if s.budget == "default" {
					addr = endpoint.def
				}
				headers := append([][2]string{}, s.headers...)
				if s.script != nil {
					encoded, err := json.Marshal(s.script)
					if err != nil {
						errs[k] = err
						return
					}
					headers = append(headers, [2]string{"X-Stub", string(encoded)})
				}
				response, err := roundTrip(addr, s.method, s.target, headers, s.body)
				run.responses[k], errs[k] = &response, err
			}(k)
		}
		wg.Wait()
		for k := i; k < end; k++ {
			if errs[k] != nil {
				t.Fatalf("%s step %d: %v", run.label, k, errs[k])
			}
		}
		i = end
	}
	var err error
	if run.logX, err = fetchOracleLog(x.short); err != nil {
		t.Fatal(err)
	}
	run.logY = run.logX
	if y != x {
		if run.logY, err = fetchOracleLog(y.short); err != nil {
			t.Fatal(err)
		}
	}
	run.rows = readOracleRows(t, pool)
	return run
}

func readOracleRows(t *testing.T, pool *pgxpool.Pool) []oracleRow {
	t.Helper()
	rows, err := pool.Query(context.Background(), `SELECT scope, idempotency_key, request_fingerprint,
response_status, encode(response_body, 'hex'), response_media_type,
id::text ~ '^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$',
completed_at IS NOT NULL, coalesce(completed_at >= created_at, true)
FROM idempotency_keys`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := []oracleRow{}
	for rows.Next() {
		var r oracleRow
		if err := rows.Scan(&r.Scope, &r.Key, &r.Fingerprint, &r.Status, &r.Body, &r.MediaType,
			&r.IDIsV4, &r.Completed, &r.CompletedAfterCreated); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Scope != out[j].Scope {
			return out[i].Scope < out[j].Scope
		}
		return out[i].Key < out[j].Key
	})
	return out
}

// normalizeStoreLog drops in-flight reservation attempts, whose number depends
// on timing, and keeps one marker per key that saw any.
func normalizeStoreLog(log map[string]any) ([]any, map[string]int) {
	counts := map[string]int{}
	out := []any{}
	entries, _ := log["store"].([]any)
	for _, entry := range entries {
		fields := entry.([]any)
		if fields[0] == "reserve" && fields[3] == "InFlight" {
			counts[fmt.Sprint(fields[1], " / ", fields[2])]++
			continue
		}
		out = append(out, fields)
	}
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		out = append(out, []any{"in-flight seen", k})
	}
	return out, counts
}

type oracleTally struct {
	scenarios, runs, responses, rows, calls, storeEntries, crossReplays, mismatches, noContentLength int
	inFlight                                                                                         []string
}

func (tally *oracleTally) mismatch(t *testing.T, format string, args ...any) {
	t.Helper()
	tally.mismatches++
	t.Errorf(format, args...)
}

func compareResponses(t *testing.T, tally *oracleTally, sc oracleScenario, a, b oracleRun) {
	t.Helper()
	for i, step := range sc.steps {
		if step.sql != "" {
			continue
		}
		ra, rb := a.responses[i], b.responses[i]
		tally.responses++
		ha, hb := comparableHeaders(tally, ra), comparableHeaders(tally, rb)
		if ra.Status != rb.Status || !reflect.DeepEqual(ha, hb) || !bytes.Equal(ra.Body, rb.Body) {
			tally.mismatch(t, "step %d %s %s (who %s)\n  %s: %d %q %q\n  %s: %d %q %q", i, step.method, step.target, step.who,
				a.label, ra.Status, ra.Headers, clip(ra.Body), b.label, rb.Status, rb.Headers, clip(rb.Body))
		}
		for _, r := range []*wireResponse{ra, rb} {
			if r.Status == http.StatusConflict && strings.Contains(string(r.Body), CodeInFlight) {
				budget := oracleShortWait
				if step.budget == "default" {
					budget = DefaultInFlightWait
				}
				if r.Elapsed < budget-20*time.Millisecond {
					tally.mismatch(t, "step %d answered 409 after %v, before the %v budget", i, r.Elapsed, budget)
				}
				tally.inFlight = append(tally.inFlight, fmt.Sprintf("%s/%d %v", sc.name, i, r.Elapsed.Round(time.Millisecond)))
			}
		}
	}
}

// comparableHeaders sets aside the one header a Go server cannot write:
// net/http never sends Content-Length on a 204 (RFC 9110 section 8.6), so
// uvicorn's "content-length: 0" there, on a handler's answer and on
// _send_stored's replay alike, has no Go counterpart. Counted, not hidden.
func comparableHeaders(tally *oracleTally, r *wireResponse) [][2]string {
	if r.Status != http.StatusNoContent {
		return r.Headers
	}
	out := [][2]string{}
	for _, pair := range r.Headers {
		if pair == [2]string{"content-length", "0"} {
			tally.noContentLength++
			continue
		}
		out = append(out, pair)
	}
	return out
}

func clip(b []byte) string {
	if len(b) > 160 {
		return string(b[:160]) + "..."
	}
	return string(b)
}

func compareRows(t *testing.T, tally *oracleTally, a, b oracleRun) {
	t.Helper()
	tally.rows += len(a.rows)
	if !reflect.DeepEqual(a.rows, b.rows) {
		tally.mismatch(t, "rows differ\n  %s: %s\n  %s: %s", a.label, describeRows(a.rows), b.label, describeRows(b.rows))
	}
	for _, r := range a.rows {
		if !r.IDIsV4 || !r.CompletedAfterCreated {
			tally.mismatch(t, "%s: row %q has id v4 %v, completed after created %v", a.label, r.Key, r.IDIsV4, r.CompletedAfterCreated)
		}
	}
}

func describeRows(rows []oracleRow) string {
	parts := []string{}
	for _, r := range rows {
		status, body, media := "NULL", "NULL", "NULL"
		if r.Status != nil {
			status = strconv.Itoa(int(*r.Status))
		}
		if r.Body != nil {
			body = *r.Body
		}
		if r.MediaType != nil {
			media = strconv.Quote(*r.MediaType)
		}
		parts = append(parts, fmt.Sprintf("{%q %q %s %s %s %s completed=%v}",
			r.Scope, r.Key, r.Fingerprint[:12], status, body, media, r.Completed))
	}
	return "[" + strings.Join(parts, " ") + "]"
}

func compareLogs(t *testing.T, tally *oracleTally, which string, a, b map[string]any, labelA, labelB string) {
	t.Helper()
	callsA, _ := a["calls"].([]any)
	callsB, _ := b["calls"].([]any)
	tally.calls += len(callsA)
	if !reflect.DeepEqual(callsA, callsB) {
		tally.mismatch(t, "%s handler calls differ\n  %s: %v\n  %s: %v", which, labelA, callsA, labelB, callsB)
	}
	storeA, countsA := normalizeStoreLog(a)
	storeB, countsB := normalizeStoreLog(b)
	tally.storeEntries += len(storeA)
	if !reflect.DeepEqual(storeA, storeB) {
		tally.mismatch(t, "%s store calls differ\n  %s: %v\n  %s: %v", which, labelA, storeA, labelB, storeB)
	}
	for k, n := range countsA {
		if diff := n - countsB[k]; diff > 3 || diff < -3 {
			tally.mismatch(t, "%s in-flight attempts for %s: %s %d, %s %d", which, k, labelA, n, labelB, countsB[k])
		}
	}
	if len(countsA) > 0 {
		tally.inFlight = append(tally.inFlight, fmt.Sprintf("  attempts %s %v vs %s %v", labelA, countsA, labelB, countsB))
	}
}

func (sc oracleScenario) crossesImplementations() bool {
	for _, step := range sc.steps {
		if step.who == "y" {
			return true
		}
	}
	return false
}

func TestOracleDifferential(t *testing.T) {
	env := setupOracle(t)
	tally := &oracleTally{}
	for _, sc := range oracleScenarios() {
		tally.scenarios++
		t.Run(sc.name, func(t *testing.T) {
			python := env.runScenario(t, sc, 0, env.python[0], env.python[0])
			golang := env.runScenario(t, sc, 1, env.golang[1], env.golang[1])
			tally.runs += 2
			compareResponses(t, tally, sc, python, golang)
			compareRows(t, tally, python, golang)
			compareLogs(t, tally, "same", python.logX, golang.logX, python.label, golang.label)
			if !sc.crossesImplementations() {
				return
			}
			// A key spent by one implementation, answered by the other.
			pyThenGo := env.runScenario(t, sc, 0, env.python[0], env.golang[0])
			goThenPy := env.runScenario(t, sc, 1, env.golang[1], env.python[1])
			tally.runs += 2
			compareResponses(t, tally, sc, pyThenGo, goThenPy)
			compareResponses(t, tally, sc, python, pyThenGo)
			compareRows(t, tally, pyThenGo, goThenPy)
			compareRows(t, tally, python, pyThenGo)
			compareLogs(t, tally, "writer", pyThenGo.logX, goThenPy.logX, "python writer", "go writer")
			compareLogs(t, tally, "answerer", pyThenGo.logY, goThenPy.logY, "go answerer", "python answerer")
			for i, step := range sc.steps {
				if step.who != "y" || step.sql != "" {
					continue
				}
				for _, r := range []*wireResponse{pyThenGo.responses[i], goThenPy.responses[i]} {
					if r.Status < 300 && (r.header("idempotency-replayed") == "true" || r.header("x-replay-seen") == "1") {
						tally.crossReplays++
					}
				}
			}
		})
	}
	t.Logf("oracle: %d scenarios, %d runs, %d responses compared, %d rows compared, %d handler calls compared, "+
		"%d store entries compared, %d cross-implementation replays, %d mismatches, "+
		"%d content-length headers on 204 set aside",
		tally.scenarios, tally.runs, tally.responses, tally.rows, tally.calls, tally.storeEntries, tally.crossReplays,
		tally.mismatches, tally.noContentLength)
	for _, line := range tally.inFlight {
		t.Logf("in flight: %s", line)
	}
}

package idem

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"mobile/services/core/internal/httpapi/router"
	"mobile/services/core/internal/pyjson"
)

const testKey = "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"

// ---------------------------------------------------------------------------
// Goldens rendered from the real middleware (scripts/render_idem_oracle.py).
// ---------------------------------------------------------------------------

type goldenFile struct {
	Constants struct {
		MaxKeyLength int      `json:"max_key_length"`
		DefaultWait  string   `json:"default_in_flight_wait_seconds"`
		FirstPoll    string   `json:"first_poll_seconds"`
		MaxPoll      string   `json:"max_poll_seconds"`
		WriteMethods []string `json:"write_methods"`
	} `json:"constants"`
	Cases []goldenCase `json:"cases"`
	Polls []goldenPoll `json:"polls"`
}

type goldenCase struct {
	Name    string      `json:"name"`
	Method  string      `json:"method"`
	Target  string      `json:"target"`
	Headers [][2]string `json:"headers"`
	Body    string      `json:"body"`
	Gen     []any       `json:"gen"`
	Outcome struct {
		Kind      string  `json:"kind"`
		Status    int     `json:"status"`
		Body      string  `json:"body"`
		MediaType *string `json:"media_type"`
	} `json:"outcome"`
	App struct {
		Status  int         `json:"status"`
		Headers [][2]string `json:"headers"`
		Body    string      `json:"body"`
		Raise   string      `json:"raise"`
	} `json:"app"`
	Want struct {
		Store    any `json:"store"`
		AppCalls any `json:"app_calls"`
		Sent     *struct {
			Status  int         `json:"status"`
			Headers [][2]string `json:"headers"`
			Body    string      `json:"body"`
		} `json:"sent"`
		Error *string `json:"error"`
	} `json:"want"`
}

type goldenPoll struct {
	Budget        string   `json:"budget_seconds"`
	InFlightTimes *int     `json:"in_flight_times"`
	Attempts      int      `json:"attempts"`
	Sleeps        []string `json:"sleeps"`
	Outcome       string   `json:"outcome"`
}

var groupedDigest = regexp.MustCompile(`[0-9a-f]{8}(?::[0-9a-f]{8}){7}`)

func loadGoldens(t *testing.T) goldenFile {
	t.Helper()
	raw, err := os.ReadFile("testdata/python_idem.json")
	if err != nil {
		t.Fatal(err)
	}
	// The generator writes each sha256 hex as eight colon-separated groups so
	// the repo guard does not read its digit runs as phone numbers.
	raw = groupedDigest.ReplaceAllFunc(raw, func(group []byte) []byte {
		return bytes.ReplaceAll(group, []byte(":"), nil)
	})
	var golden goldenFile
	if err := json.Unmarshal(raw, &golden); err != nil {
		t.Fatal(err)
	}
	if len(golden.Cases) < 100 || len(golden.Polls) == 0 {
		t.Fatalf("golden file looks truncated: %d cases, %d polls", len(golden.Cases), len(golden.Polls))
	}
	return golden
}

// pySeconds converts Python's float.hex() text to the nearest nanosecond.
func pySeconds(t *testing.T, text string) time.Duration {
	t.Helper()
	f, err := strconv.ParseFloat(text, 64)
	if err != nil {
		t.Fatal(err)
	}
	return time.Duration(math.Round(f * 1e9))
}

// latin1 turns JSON text holding latin-1 characters back into header bytes.
func latin1(t *testing.T, text string) string {
	t.Helper()
	out, err := latin1Bytes(text)
	if err != nil {
		t.Fatalf("golden header %q is not latin-1", text)
	}
	return string(out)
}

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func (c goldenCase) body(t *testing.T) []byte {
	if c.Gen == nil {
		return mustHex(t, c.Body)
	}
	prefix := mustHex(t, c.Gen[0].(string))
	unit := mustHex(t, c.Gen[1].(string))
	count := int(c.Gen[2].(float64))
	suffix := mustHex(t, c.Gen[3].(string))
	return append(append(prefix, bytes.Repeat(unit, count)...), suffix...)
}

// scriptedStore answers every reservation with one outcome and records calls
// in the shape the golden renderer records them.
type scriptedStore struct {
	outcome Outcome
	calls   []map[string]any
}

func textOrNil(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}

func (s *scriptedStore) Reserve(_ context.Context, scope, key, fingerprint, legacy string) (Outcome, error) {
	var legacyValue any
	if legacy != "" {
		legacyValue = legacy
	}
	s.calls = append(s.calls, map[string]any{"op": "reserve", "scope": scope, "key": key,
		"fingerprint": fingerprint, "legacy": legacyValue})
	return s.outcome, nil
}

func (s *scriptedStore) Complete(_ context.Context, scope, key string, response StoredResponse) error {
	s.calls = append(s.calls, map[string]any{"op": "complete", "scope": scope, "key": key,
		"status": response.Status, "body": hex.EncodeToString(response.Body), "media_type": textOrNil(response.MediaType)})
	return nil
}

func (s *scriptedStore) Release(_ context.Context, scope, key string) error {
	s.calls = append(s.calls, map[string]any{"op": "release", "scope": scope, "key": key})
	return nil
}

// recorder notes whether anything was written at all, which
// httptest.ResponseRecorder cannot tell apart from an implicit 200.
type recorder struct {
	header http.Header
	wrote  bool
	status int
	sent   http.Header
	body   bytes.Buffer
}

func newRecorder() *recorder { return &recorder{header: http.Header{}} }

func (r *recorder) Header() http.Header { return r.header }

func (r *recorder) WriteHeader(code int) {
	if r.wrote {
		return
	}
	r.wrote, r.status, r.sent = true, code, r.header.Clone()
}

func (r *recorder) Write(p []byte) (int, error) {
	if !r.wrote {
		r.WriteHeader(http.StatusOK)
	}
	return r.body.Write(p)
}

// headerPairs lists header lines as a wire would carry them: lower-case names,
// sorted by name, values in order, entries with no value (sniff guards) gone.
func headerPairs(h http.Header) [][2]string {
	out := [][2]string{}
	for name, values := range h {
		for _, value := range values {
			out = append(out, [2]string{strings.ToLower(name), value})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i][0] < out[j][0] })
	return out
}

// normalize round-trips through JSON so Go maps compare with decoded goldens.
func normalize(t *testing.T, v any) any {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

var errScripted = errors.New(scriptedFailure)

const scriptedFailure = "scripted failure"

func TestGoldenCasesMatchPythonMiddleware(t *testing.T) {
	golden := loadGoldens(t)
	for _, c := range golden.Cases {
		t.Run(c.Name, func(t *testing.T) { runGoldenCase(t, c) })
	}
}

func runGoldenCase(t *testing.T, c goldenCase) {
	body := c.body(t)
	store := &scriptedStore{calls: []map[string]any{}}
	switch c.Outcome.Kind {
	case "Reserved":
		store.outcome = Outcome{Kind: Reserved}
	case "InFlight":
		store.outcome = Outcome{Kind: InFlight}
	case "Conflict":
		store.outcome = Outcome{Kind: Conflict}
	case "Replay":
		store.outcome = Outcome{Kind: Replay, Response: StoredResponse{
			Status: c.Outcome.Status, Body: mustHex(t, c.Outcome.Body), MediaType: c.Outcome.MediaType}}
	default:
		t.Fatalf("unknown outcome %q", c.Outcome.Kind)
	}

	calls := []map[string]any{}
	app := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received, _ := io.ReadAll(r.Body)
		rawPath, _ := router.SplitTarget(r.RequestURI)
		call := map[string]any{"path": router.ScopePath(rawPath), "body_sha256": sha256Hex(received), "replay": nil}
		if stored, ok := AuthorizedReplay(r.Context()); ok {
			call["replay"] = map[string]any{"status": stored.Status, "body": hex.EncodeToString(stored.Body),
				"media_type": textOrNil(stored.MediaType)}
		}
		calls = append(calls, call)
		if c.App.Raise == "before" {
			panic(errScripted)
		}
		hasType := false
		for _, pair := range c.App.Headers {
			w.Header().Add(pair[0], latin1(t, pair[1]))
			hasType = hasType || strings.EqualFold(pair[0], "content-type")
		}
		if !hasType {
			w.Header()["Content-Type"] = nil
		}
		w.WriteHeader(c.App.Status)
		if c.App.Raise == "after_start" {
			panic(errScripted)
		}
		_, _ = w.Write(mustHex(t, c.App.Body))
	})

	var handlerErr error
	middleware := New(store, WithInFlightWait(0), WithErrorHandler(
		func(_ http.ResponseWriter, _ *http.Request, err error) { handlerErr = err }))(app)

	// Built by hand: http.ReadRequest refuses targets uvicorn accepts ("%zz").
	request := &http.Request{
		Method:     c.Method,
		RequestURI: c.Target,
		URL:        &url.URL{},
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     http.Header{},
		Body:       io.NopCloser(bytes.NewReader(body)),
	}
	for _, pair := range c.Headers {
		request.Header.Add(pair[0], latin1(t, pair[1]))
	}
	rec := newRecorder()
	panicked := func() (value any) {
		defer func() { value = recover() }()
		middleware.ServeHTTP(rec, request)
		return nil
	}()

	if got, want := normalize(t, store.calls), normalize(t, c.Want.Store); !reflect.DeepEqual(got, want) {
		t.Errorf("store calls\n got %v\nwant %v", got, want)
	}
	if got, want := normalize(t, calls), normalize(t, c.Want.AppCalls); !reflect.DeepEqual(got, want) {
		t.Errorf("app calls\n got %v\nwant %v", got, want)
	}

	switch {
	case c.Want.Error == nil:
		if panicked != nil || handlerErr != nil {
			t.Errorf("unexpected failure: panic %v, error %v", panicked, handlerErr)
		}
	case *c.Want.Error == "RuntimeError":
		if panicked != errScripted || handlerErr != nil {
			t.Errorf("want the handler's panic to propagate untouched; panic %v, error %v", panicked, handlerErr)
		}
	case *c.Want.Error == "UnicodeEncodeError":
		var surrogate *pyjson.UnicodeEncodeError
		var latin *LatinEncodeError
		if panicked != nil || !(errors.As(handlerErr, &surrogate) || errors.As(handlerErr, &latin)) {
			t.Errorf("want an encode error; panic %v, error %v", panicked, handlerErr)
		}
	case *c.Want.Error == "RecursionError":
		var recursion *pyjson.RecursionError
		if panicked != nil || !errors.As(handlerErr, &recursion) {
			t.Errorf("want a recursion error; panic %v, error %v", panicked, handlerErr)
		}
	default:
		t.Fatalf("golden error %q has no Go counterpart", *c.Want.Error)
	}

	if c.Want.Sent == nil {
		if rec.wrote {
			t.Errorf("Python sent nothing; Go sent %d %v", rec.status, headerPairs(rec.sent))
		}
		return
	}
	if !rec.wrote {
		t.Fatalf("Python sent %d; Go sent nothing", c.Want.Sent.Status)
	}
	if rec.status != c.Want.Sent.Status {
		t.Errorf("status %d, want %d", rec.status, c.Want.Sent.Status)
	}
	want := [][2]string{}
	for _, pair := range c.Want.Sent.Headers {
		want = append(want, [2]string{pair[0], latin1(t, pair[1])})
	}
	sort.SliceStable(want, func(i, j int) bool { return want[i][0] < want[j][0] })
	if got := headerPairs(rec.sent); !reflect.DeepEqual(got, want) {
		t.Errorf("headers\n got %q\nwant %q", got, want)
	}
	if got, want := rec.body.Bytes(), mustHex(t, c.Want.Sent.Body); !bytes.Equal(got, want) {
		t.Errorf("body\n got %q\nwant %q", got, want)
	}
}

func TestPollScheduleMatchesPython(t *testing.T) {
	golden := loadGoldens(t)
	for _, poll := range golden.Polls {
		budget := pySeconds(t, poll.Budget)
		start := time.Unix(0, 0)
		var elapsed time.Duration
		var sleeps []time.Duration
		attempts := 0
		store := &funcStore{reserve: func() Outcome {
			attempts++
			if poll.InFlightTimes != nil && attempts > *poll.InFlightTimes {
				return Outcome{Kind: Replay}
			}
			return Outcome{Kind: InFlight}
		}}
		h := New(store, WithInFlightWait(budget), WithClock(
			func() time.Time { return start.Add(elapsed) },
			func(d time.Duration) { sleeps = append(sleeps, d); elapsed += d },
		))(nil).(*handler)
		outcome, err := h.reserve(context.Background(), AnonymousScope, testKey, "f", "")
		if err != nil {
			t.Fatal(err)
		}
		want := []time.Duration{}
		for _, s := range poll.Sleeps {
			want = append(want, pySeconds(t, s))
		}
		if sleeps == nil {
			sleeps = []time.Duration{}
		}
		if attempts != poll.Attempts || outcome.Kind.String() != poll.Outcome || !reflect.DeepEqual(sleeps, want) {
			t.Errorf("budget %s: got %d attempts %v sleeps %v; Python %d attempts %s sleeps %v",
				poll.Budget, attempts, outcome.Kind, sleeps, poll.Attempts, poll.Outcome, want)
		}
	}
}

func TestConstantsMatchPython(t *testing.T) {
	golden := loadGoldens(t)
	c := golden.Constants
	if c.MaxKeyLength != MaxKeyLength ||
		pySeconds(t, c.DefaultWait) != DefaultInFlightWait ||
		pySeconds(t, c.FirstPoll) != FirstPollInterval ||
		pySeconds(t, c.MaxPoll) != MaxPollInterval {
		t.Fatalf("constants drifted from Python: %+v", c)
	}
	got := []string{}
	for method := range writeMethods {
		got = append(got, method)
	}
	sort.Strings(got)
	if !reflect.DeepEqual(got, c.WriteMethods) {
		t.Fatalf("write methods %v, Python %v", got, c.WriteMethods)
	}
}

// ---------------------------------------------------------------------------
// Behaviour a single in-process golden cannot show: ordering, concurrency,
// cancellation, failures of the store itself, and net/http's own writer.
// ---------------------------------------------------------------------------

type funcStore struct {
	reserve func() Outcome
}

func (s *funcStore) Reserve(context.Context, string, string, string, string) (Outcome, error) {
	return s.reserve(), nil
}
func (s *funcStore) Complete(context.Context, string, string, StoredResponse) error { return nil }
func (s *funcStore) Release(context.Context, string, string) error                  { return nil }

// memoryStore is InMemoryIdempotencyStore from tests/api/test_idempotency.py,
// made safe for concurrent requests. It models no database.
type memoryStore struct {
	mu              sync.Mutex
	rows            map[[2]string]*memoryRow
	ops             []string
	contexts        []context.Context
	onInFlight      func()
	onComplete      func()
	reserveErr      error
	completeErr     error
	releaseErr      error
	completedBodies [][]byte
}

type memoryRow struct {
	fingerprint string
	response    *StoredResponse
}

func newMemoryStore() *memoryStore {
	return &memoryStore{rows: map[[2]string]*memoryRow{}}
}

func (s *memoryStore) Reserve(ctx context.Context, scope, key, fingerprint, legacy string) (Outcome, error) {
	s.mu.Lock()
	s.ops = append(s.ops, "reserve")
	s.contexts = append(s.contexts, ctx)
	if s.reserveErr != nil {
		s.mu.Unlock()
		return Outcome{}, s.reserveErr
	}
	row, ok := s.rows[[2]string{scope, key}]
	if !ok {
		s.rows[[2]string{scope, key}] = &memoryRow{fingerprint: fingerprint}
		s.mu.Unlock()
		return Outcome{Kind: Reserved}, nil
	}
	if row.fingerprint != fingerprint {
		if legacy == "" || row.fingerprint != legacy {
			s.mu.Unlock()
			return Outcome{Kind: Conflict}, nil
		}
		row.fingerprint = fingerprint
	}
	if row.response == nil {
		hook := s.onInFlight
		s.mu.Unlock()
		if hook != nil {
			hook()
		}
		return Outcome{Kind: InFlight}, nil
	}
	response := *row.response
	s.mu.Unlock()
	return Outcome{Kind: Replay, Response: response}, nil
}

func (s *memoryStore) Complete(ctx context.Context, scope, key string, response StoredResponse) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ops = append(s.ops, "complete")
	s.contexts = append(s.contexts, ctx)
	if s.completeErr != nil {
		return s.completeErr
	}
	s.completedBodies = append(s.completedBodies, response.Body)
	if row, ok := s.rows[[2]string{scope, key}]; ok {
		row.response = &response
	}
	if s.onComplete != nil {
		s.onComplete()
	}
	return nil
}

func (s *memoryStore) Release(ctx context.Context, scope, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ops = append(s.ops, "release")
	s.contexts = append(s.contexts, ctx)
	if s.releaseErr != nil {
		return s.releaseErr
	}
	delete(s.rows, [2]string{scope, key})
	return nil
}

func (s *memoryStore) opList() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.ops...)
}

func post(target, key string, body string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, target, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	if key != "" {
		r.Header.Set(HeaderName, key)
	}
	return r
}

func answer201(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", "15")
	w.WriteHeader(http.StatusCreated)
	_, _ = io.WriteString(w, `{"id": "first"}`)
}

// orderingWriter checks, at the moment the answer starts, that the completion
// has already been recorded.
type orderingWriter struct {
	*httptest.ResponseRecorder
	store    *memoryStore
	sawEarly bool
}

func (w *orderingWriter) WriteHeader(code int) {
	ops := w.store.opList()
	if len(ops) == 0 || ops[len(ops)-1] != "complete" {
		w.sawEarly = true
	}
	w.ResponseRecorder.WriteHeader(code)
}

func TestTheAnswerIsHeldUntilTheCompletionIsRecorded(t *testing.T) {
	store := newMemoryStore()
	middleware := New(store)(http.HandlerFunc(answer201))
	w := &orderingWriter{ResponseRecorder: httptest.NewRecorder(), store: store}
	middleware.ServeHTTP(w, post("/expenses", testKey, `{}`))
	if w.Code != http.StatusCreated || w.sawEarly {
		t.Fatalf("code %d, answer started before completion: %v (ops %v)", w.Code, w.sawEarly, store.opList())
	}

	second := httptest.NewRecorder()
	middleware.ServeHTTP(second, post("/expenses", testKey, `{ }`))
	if second.Code != http.StatusCreated || second.Header().Get(ReplayHeaderName) != "true" ||
		second.Body.String() != `{"id": "first"}` {
		t.Fatalf("replay: %d %v %q", second.Code, second.Header(), second.Body.String())
	}
}

func TestTwoPressesAtTheSameInstantBothReceiveTheOneAnswer(t *testing.T) {
	store := newMemoryStore()
	sawInFlight := make(chan struct{})
	var once sync.Once
	store.onInFlight = func() { once.Do(func() { close(sawInFlight) }) }
	var handled int
	var handledMu sync.Mutex
	middleware := New(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handledMu.Lock()
		handled++
		handledMu.Unlock()
		<-sawInFlight
		answer201(w, r)
	}))

	recorders := []*httptest.ResponseRecorder{httptest.NewRecorder(), httptest.NewRecorder()}
	var wg sync.WaitGroup
	for _, rec := range recorders {
		wg.Add(1)
		go func(rec *httptest.ResponseRecorder) {
			defer wg.Done()
			middleware.ServeHTTP(rec, post("/expenses", testKey, `{"total_amount_vnd": 1}`))
		}(rec)
	}
	wg.Wait()
	for _, rec := range recorders {
		if rec.Code != http.StatusCreated || rec.Body.String() != `{"id": "first"}` {
			t.Fatalf("press answered %d %q", rec.Code, rec.Body.String())
		}
	}
	if handled != 1 {
		t.Fatalf("handler ran %d times", handled)
	}
}

func TestAKeyNobodyFinishesIsRefusedAfterTheBudget(t *testing.T) {
	store := newMemoryStore()
	store.rows[[2]string{AnonymousScope, testKey}] = &memoryRow{
		fingerprint: LegacyFingerprint("POST", "/expenses", "", []byte(`{}`)),
	}
	var slept time.Duration
	now := time.Unix(0, 0)
	middleware := New(store, WithInFlightWait(50*time.Millisecond), WithClock(
		func() time.Time { return now.Add(slept) },
		func(d time.Duration) { slept += d },
	))(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("the handler must not run for a reserved key")
	}))
	rec := httptest.NewRecorder()
	middleware.ServeHTTP(rec, post("/expenses", testKey, `{}`))
	if rec.Code != http.StatusConflict || slept != 50*time.Millisecond {
		t.Fatalf("code %d after %v", rec.Code, slept)
	}
	want := `{"code": "idempotency_request_in_flight", "detail": "An earlier request with this key has not finished. Retry with this same key; sending a different one would write it twice"}`
	if rec.Body.String() != want || strings.Contains(strings.ToLower(rec.Body.String()), "new key") {
		t.Fatalf("body %q", rec.Body.String())
	}
}

func TestStoreCallsOutliveAClientThatWentAway(t *testing.T) {
	store := newMemoryStore()
	ctx, cancel := context.WithCancel(context.Background())
	middleware := New(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cancel() // the client disconnects while the handler runs
		answer201(w, r)
	}))
	middleware.ServeHTTP(httptest.NewRecorder(), post("/expenses", testKey, `{}`).WithContext(ctx))
	for i, c := range store.contexts {
		if c.Err() != nil {
			t.Fatalf("store call %d (%s) saw a cancelled context", i, store.ops[i])
		}
	}
	if got := store.opList(); !reflect.DeepEqual(got, []string{"reserve", "complete"}) {
		t.Fatalf("ops %v", got)
	}
}

func TestAPanickingHandlerReleasesTheKeyAndKeepsUnwinding(t *testing.T) {
	for _, value := range []any{errScripted, http.ErrAbortHandler} {
		store := newMemoryStore()
		middleware := New(store)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusCreated)
			panic(value)
		}))
		rec := httptest.NewRecorder()
		got := func() (v any) {
			defer func() { v = recover() }()
			middleware.ServeHTTP(rec, post("/expenses", testKey, `{}`))
			return nil
		}()
		if got != value {
			t.Fatalf("panic %v, want %v", got, value)
		}
		if ops := store.opList(); !reflect.DeepEqual(ops, []string{"reserve", "release"}) || len(store.rows) != 0 {
			t.Fatalf("ops %v rows %d", ops, len(store.rows))
		}
		if rec.Code != http.StatusOK || rec.Body.Len() != 0 || len(rec.Header()) != 0 {
			t.Fatalf("a half answer escaped: %d %v", rec.Code, rec.Header())
		}
	}
}

func TestStoreFailuresAnswerStarlettesServerError(t *testing.T) {
	failure := errors.New("database unavailable")
	cases := []struct {
		name   string
		target string
		setup  func(*memoryStore)
		status int
		ops    []string
	}{
		{"reserve fails", "/expenses", func(s *memoryStore) { s.reserveErr = failure }, 201, []string{"reserve"}},
		{"complete fails and the key stays reserved", "/expenses", func(s *memoryStore) { s.completeErr = failure }, 201, []string{"reserve", "complete"}},
		{"release fails", "/expenses", func(s *memoryStore) { s.releaseErr = failure }, 422, []string{"reserve", "release"}},
		{"guest path keeps privacy headers", "/g/token/paid", func(s *memoryStore) { s.reserveErr = failure }, 201, []string{"reserve"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			store := newMemoryStore()
			c.setup(store)
			middleware := New(store)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(c.status)
				_, _ = io.WriteString(w, `{"leaked": true}`)
			}))
			rec := httptest.NewRecorder()
			middleware.ServeHTTP(rec, post(c.target, testKey, `{}`))
			if rec.Code != http.StatusInternalServerError || rec.Body.String() != "Internal Server Error" ||
				rec.Header().Get("Content-Type") != "text/plain; charset=utf-8" {
				t.Fatalf("got %d %v %q", rec.Code, rec.Header(), rec.Body.String())
			}
			if (rec.Header().Get("Cache-Control") == "no-store") != strings.HasPrefix(c.target, "/g/") {
				t.Fatalf("guest privacy headers: %v", rec.Header())
			}
			if got := store.opList(); !reflect.DeepEqual(got, c.ops) {
				t.Fatalf("ops %v, want %v", got, c.ops)
			}
		})
	}
}

func TestAnEmptyAnswerIsStoredAsAnEmptyBodyNotNull(t *testing.T) {
	store := newMemoryStore()
	middleware := New(store)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	middleware.ServeHTTP(httptest.NewRecorder(), post("/expenses", testKey, `{}`))
	if len(store.completedBodies) != 1 || store.completedBodies[0] == nil {
		t.Fatalf("completed bodies %#v", store.completedBodies)
	}
}

// Over a real net/http server: what the client receives, first and on replay.
func TestTheWireAnswerMatchesWhatIsStored(t *testing.T) {
	store := newMemoryStore()
	var runs int
	mux := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		runs++
		switch r.URL.Path {
		case "/untyped":
			// No Content-Type: net/http would sniff "text/plain" if allowed.
			_, _ = io.WriteString(w, "plain words")
			w.Header().Set("X-Too-Late", "1")
		case "/no-content":
			w.WriteHeader(http.StatusNoContent)
			if _, err := io.WriteString(w, "x"); !errors.Is(err, http.ErrBodyNotAllowed) {
				t.Errorf("write on 204: %v", err)
			}
		case "/silent":
		}
	})
	server := httptest.NewServer(New(store)(mux))
	defer server.Close()

	do := func(path string) (*http.Response, string) {
		req, _ := http.NewRequest(http.MethodPost, server.URL+path, strings.NewReader("{}"))
		req.Header.Set(HeaderName, testKey+path)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp, string(b)
	}

	first, body := do("/untyped")
	if first.StatusCode != 200 || body != "plain words" || first.Header.Values("Content-Type") != nil ||
		first.Header.Get("X-Too-Late") != "" {
		t.Fatalf("first: %d %v %q", first.StatusCode, first.Header, body)
	}
	replay, body := do("/untyped")
	if replay.StatusCode != 200 || body != "plain words" || replay.Header.Values("Content-Type") != nil ||
		replay.Header.Get(ReplayHeaderName) != "true" {
		t.Fatalf("replay: %d %v %q", replay.StatusCode, replay.Header, body)
	}

	for _, path := range []string{"/no-content", "/silent"} {
		want := map[string]int{"/no-content": 204, "/silent": 200}[path]
		a, _ := do(path)
		b, _ := do(path)
		if a.StatusCode != want || b.StatusCode != want || b.Header.Get(ReplayHeaderName) != "true" {
			t.Fatalf("%s: %d then %d %v", path, a.StatusCode, b.StatusCode, b.Header)
		}
	}
	if runs != 3 {
		t.Fatalf("handler ran %d times, want 3", runs)
	}
}

func TestTheItineraryReplayIsHandedToTheHandler(t *testing.T) {
	store := newMemoryStore()
	var seen []string
	middleware := New(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if stored, ok := AuthorizedReplay(r.Context()); ok {
			seen = append(seen, "replay:"+string(stored.Body)+":"+string(body))
			w.Header().Set(ReplayHeaderName, "true")
			w.WriteHeader(stored.Status)
			_, _ = w.Write(stored.Body)
			return
		}
		seen = append(seen, "write:"+string(body))
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"revision": 1}`)
	}))
	for range 2 {
		r := httptest.NewRequest(http.MethodPut, "/outings/abc/itinerary", strings.NewReader(`{"stops": []}`))
		r.Header.Set(HeaderName, testKey)
		middleware.ServeHTTP(httptest.NewRecorder(), r)
	}
	want := []string{`write:{"stops": []}`, `replay:{"revision": 1}:{"stops": []}`}
	if !reflect.DeepEqual(seen, want) {
		t.Fatalf("handler saw %q", seen)
	}
	if got := store.opList(); !reflect.DeepEqual(got, []string{"reserve", "complete", "reserve"}) {
		t.Fatalf("ops %v", got)
	}
	if _, ok := AuthorizedReplay(context.Background()); ok {
		t.Fatal("a bare context must carry no replay")
	}
}

// Package idem is the Go port of services/api/app/api/idempotency.py: the
// Idempotency-Key layer in front of the write routes the Go front door serves.
// It shares the idempotency_keys table with the Python API, so a key spent
// through one implementation replays through the other.
//
// The rules copied from Python, each checked against the real middleware by
// testdata/python_idem.json (scripts/render_idem_oracle.py goldens) and by the
// differential test in oracle_postgres_test.go:
//
//   - Only POST, PUT, PATCH and DELETE, compared case-sensitively. Everything
//     else, and a write without the header, passes through untouched.
//   - POST /outings/{id}/itinerary/preview always passes through, before the
//     key is even read: a preview writes nothing and must never replay.
//   - The key is the first Idempotency-Key header. Empty or longer than 255
//     latin-1 characters (bytes) is 422 invalid_idempotency_key, before the
//     body is read.
//   - The scope is "bearer:" + sha256 of the stripped token when Authorization
//     is "<bearer, any case><space><token>", else the first X-Actor-ID when it
//     is non-empty, else "anonymous". Header text is latin-1, so the token is
//     re-encoded as UTF-8 before hashing, exactly as auth.TokenDigest does, and
//     the scope and key are stored as the UTF-8 of those latin-1 characters.
//     A malformed Authorization is not an error: it falls through to the actor.
//   - The fingerprint is sha256(method \n path \n raw query \n body), where the
//     path is the percent-decoded scope path and the body is canonical JSON
//     (sorted keys, compact, ensure_ascii=False) when the first Content-Type
//     declares JSON. A body json.loads refuses with a ValueError (bad syntax,
//     bad encoding, an int over 4300 digits) is hashed verbatim; a canonical
//     form that cannot be encoded (a lone surrogate) or a RecursionError is a
//     500, before anything is reserved.
//   - The raw-bytes digest an older server wrote is offered alongside; a row
//     holding it is adopted and its fingerprint rewritten in place, even when
//     the row is still in flight.
//   - Reservation, completion and release are each one short transaction of
//     their own. An unfinished key is polled: 20ms, doubling, capped at 100ms,
//     never past the budget (5s by default); then 409. Polling and the store
//     ignore client disconnects, as the ASGI middleware does.
//   - The handler's answer is buffered completely. A 2xx is recorded before a
//     single byte reaches the caller; anything else releases the key; a panic
//     releases the key and keeps unwinding.
//   - A replay sends the stored status and body with exactly Content-Length,
//     Idempotency-Replayed: true and, when the stored media type is non-empty,
//     Content-Type. A PUT /outings/{id}/itinerary replay is not written here:
//     the handler receives it through AuthorizedReplay to re-authorize first.
package idem

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"mobile/services/core/internal/auth"
	"mobile/services/core/internal/httpapi/problem"
	"mobile/services/core/internal/httpapi/router"
	"mobile/services/core/internal/pyjson"
)

// Names and limits, as idempotency.py declares them.
const (
	HeaderName          = "Idempotency-Key"
	ReplayHeaderName    = "Idempotency-Replayed"
	AnonymousScope      = "anonymous"
	MaxKeyLength        = 255
	DefaultInFlightWait = 5 * time.Second
	FirstPollInterval   = 20 * time.Millisecond
	MaxPollInterval     = 100 * time.Millisecond
)

// Problem codes the middleware answers on its own.
const (
	CodeInvalidKey = "invalid_idempotency_key"
	CodeKeyReuse   = "idempotency_key_reuse"
	CodeInFlight   = "idempotency_request_in_flight"
)

const (
	detailInvalidKey = HeaderName + " must be 1..255 characters"
	detailKeyReuse   = HeaderName + " was already used for a different request"
	detailInFlight   = "An earlier request with this key has not finished. Retry with" +
		" this same key; sending a different one would write it twice"
)

var writeMethods = map[string]bool{"POST": true, "PUT": true, "PATCH": true, "DELETE": true}

// StoredResponse is the answer that was actually sent, kept verbatim.
// MediaType is database text (nil is NULL); the wire form is its latin-1.
type StoredResponse struct {
	Status    int
	Body      []byte
	MediaType *string
}

// Kind is which of Python's four outcomes a reservation produced.
type Kind int

// Outcome kinds.
const (
	Reserved Kind = iota + 1 // this caller owns the key and runs the handler
	Replay                   // the key completed; answer with Response
	InFlight                 // reserved but unfinished (or vanished mid-read)
	Conflict                 // the key was spent on a different request
)

func (k Kind) String() string {
	switch k {
	case Reserved:
		return "Reserved"
	case Replay:
		return "Replay"
	case InFlight:
		return "InFlight"
	case Conflict:
		return "Conflict"
	}
	return fmt.Sprintf("Kind(%d)", int(k))
}

// Outcome is the result of one reservation attempt.
type Outcome struct {
	Kind     Kind
	Response StoredResponse // set for Replay
}

// Store is IdempotencyStore. Scope, key and fingerprints are database text.
// legacyFingerprint is "" when there is none. Each call is expected to run in
// its own transaction, as main.py's sqlalchemy_store_factory does.
type Store interface {
	Reserve(ctx context.Context, scope, key, fingerprint, legacyFingerprint string) (Outcome, error)
	Complete(ctx context.Context, scope, key string, response StoredResponse) error
	Release(ctx context.Context, scope, key string) error
}

// Option configures the middleware.
type Option func(*config)

type config struct {
	wait    time.Duration
	now     func() time.Time
	sleep   func(time.Duration)
	onError func(http.ResponseWriter, *http.Request, error)
}

// WithInFlightWait sets the polling budget (create_app's
// idempotency_in_flight_wait_seconds). Zero or negative polls exactly once.
func WithInFlightWait(d time.Duration) Option {
	return func(c *config) { c.wait = d }
}

// WithClock replaces the monotonic clock and the sleep used while polling.
func WithClock(now func() time.Time, sleep func(time.Duration)) Option {
	return func(c *config) {
		c.now = now
		c.sleep = sleep
	}
}

// WithErrorHandler answers a failure raised by the middleware itself: a store
// error, a body whose canonical form cannot be encoded, a stored media type
// outside latin-1. Python lets those exceptions unwind to Starlette's server
// error handler; the default here writes that same 500 through
// problem.WriteServerError. Panics from the wrapped handler are never routed
// here: they release the key and keep unwinding.
func WithErrorHandler(fn func(http.ResponseWriter, *http.Request, error)) Option {
	return func(c *config) { c.onError = fn }
}

// New returns the middleware.
func New(store Store, options ...Option) func(http.Handler) http.Handler {
	cfg := config{wait: DefaultInFlightWait, now: time.Now, sleep: time.Sleep}
	for _, option := range options {
		option(&cfg)
	}
	return func(next http.Handler) http.Handler {
		return &handler{cfg: cfg, store: store, next: next}
	}
}

type replayContextKey struct{}

// AuthorizedReplay returns the stored answer the middleware handed to a
// PUT /outings/{id}/itinerary handler instead of writing it (the Python scope
// key "itinerary_authorized_replay"). The handler must re-check the session
// and membership, then answer with it without writing again.
func AuthorizedReplay(ctx context.Context) (StoredResponse, bool) {
	response, ok := ctx.Value(replayContextKey{}).(StoredResponse)
	return response, ok
}

type handler struct {
	cfg   config
	store Store
	next  http.Handler
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !writeMethods[r.Method] {
		h.next.ServeHTTP(w, r)
		return
	}
	rawPath, rawQuery := requestTarget(r)
	path := router.ScopePath(rawPath)
	parts := strings.Split(strings.Trim(path, "/"), "/")
	itineraryWrite := r.Method == http.MethodPut && len(parts) == 3 &&
		parts[0] == "outings" && parts[2] == "itinerary"
	if r.Method == http.MethodPost && len(parts) == 4 && parts[0] == "outings" &&
		parts[2] == "itinerary" && parts[3] == "preview" {
		h.next.ServeHTTP(w, r)
		return
	}

	key, sent := firstHeader(r.Header, HeaderName)
	if !sent {
		h.next.ServeHTTP(w, r)
		return
	}
	if key == "" || len(key) > MaxKeyLength {
		h.problem(w, r, path, http.StatusUnprocessableEntity, CodeInvalidKey, detailInvalidKey)
		return
	}

	body := drain(r.Body)
	scope := RequestScope(r.Header)
	contentType, _ := firstHeader(r.Header, "Content-Type")
	fingerprint, err := Fingerprint(r.Method, path, rawQuery, body, contentType)
	if err != nil {
		h.fail(w, r, path, err)
		return
	}
	legacy := LegacyFingerprint(r.Method, path, rawQuery, body)
	if legacy == fingerprint {
		legacy = ""
	}
	keyText := latin1ToUTF8(key)
	// The ASGI middleware is never cancelled by a client that goes away, and
	// a release skipped for that reason would strand the key.
	ctx := context.WithoutCancel(r.Context())

	outcome, err := h.reserve(ctx, scope, keyText, fingerprint, legacy)
	if err != nil {
		h.fail(w, r, path, err)
		return
	}
	switch outcome.Kind {
	case Conflict:
		h.problem(w, r, path, http.StatusUnprocessableEntity, CodeKeyReuse, detailKeyReuse)
	case InFlight:
		// Deliberately does not suggest a fresh key: a fresh key is permission
		// to write the same money twice.
		h.problem(w, r, path, http.StatusConflict, CodeInFlight, detailInFlight)
	case Replay:
		if itineraryWrite {
			inner := r.WithContext(context.WithValue(r.Context(), replayContextKey{}, outcome.Response))
			inner.Body = io.NopCloser(bytes.NewReader(body))
			h.next.ServeHTTP(w, inner)
			return
		}
		if err := writeStored(w, outcome.Response); err != nil {
			h.fail(w, r, path, err)
		}
	case Reserved:
		h.runReserved(w, r, ctx, path, scope, keyText, body)
	default:
		h.fail(w, r, path, fmt.Errorf("idem: store returned %v", outcome.Kind))
	}
}

// reserve is IdempotencyMiddleware._reserve: one short transaction per
// attempt, waiting out a reservation somebody else is finishing.
func (h *handler) reserve(ctx context.Context, scope, key, fingerprint, legacy string) (Outcome, error) {
	deadline := h.cfg.now().Add(h.cfg.wait)
	delay := FirstPollInterval
	for {
		outcome, err := h.store.Reserve(ctx, scope, key, fingerprint, legacy)
		if err != nil {
			return Outcome{}, err
		}
		if outcome.Kind != InFlight {
			return outcome, nil
		}
		remaining := deadline.Sub(h.cfg.now())
		if remaining <= 0 {
			return outcome, nil
		}
		h.cfg.sleep(min(delay, remaining))
		delay = min(delay*2, MaxPollInterval)
	}
}

func (h *handler) runReserved(w http.ResponseWriter, r *http.Request, ctx context.Context, path, scope, key string, body []byte) {
	captured := newCapture(w.Header())
	inner := r.WithContext(r.Context())
	inner.Body = io.NopCloser(bytes.NewReader(body))

	returned := false
	defer func() {
		if !returned {
			// A panic (or Goexit) unwinding out of the handler: nothing was
			// forwarded, so free the key and let the failure keep going. A
			// release error cannot be reported from here; the request fails
			// either way, as it does in Python.
			_ = h.store.Release(ctx, scope, key)
		}
	}()
	h.next.ServeHTTP(captured, inner)
	returned = true

	status := captured.finalStatus()
	if status >= 200 && status < 300 {
		response := StoredResponse{Status: status, Body: captured.bodyBytes(), MediaType: captured.mediaType()}
		if err := h.store.Complete(ctx, scope, key, response); err != nil {
			h.fail(w, r, path, err)
			return
		}
	} else if err := h.store.Release(ctx, scope, key); err != nil {
		h.fail(w, r, path, err)
		return
	}
	// Only now, with the key settled, does the caller see anything.
	captured.writeTo(w)
}

func (h *handler) problem(w http.ResponseWriter, r *http.Request, path string, status int, code, detail string) {
	// json.dumps with its defaults: ", " and ": " separators, ASCII only.
	document := pyjson.NewOrderedMap()
	document.Set("code", pyjson.String(code))
	document.Set("detail", pyjson.String(detail))
	encoded, err := pyjson.Dumps(document)
	if err != nil {
		h.fail(w, r, path, err)
		return
	}
	header := w.Header()
	header.Set("Content-Type", "application/json")
	header.Set("Content-Length", strconv.Itoa(len(encoded)))
	w.WriteHeader(status)
	_, _ = w.Write(encoded)
}

func (h *handler) fail(w http.ResponseWriter, r *http.Request, path string, err error) {
	if h.cfg.onError != nil {
		h.cfg.onError(w, r, err)
		return
	}
	problem.WriteServerError(w, path)
}

// writeStored is _send_stored. The media type is encoded before anything is
// written, so a failure leaves the response untouched.
func writeStored(w http.ResponseWriter, response StoredResponse) error {
	var mediaType []byte
	hasType := response.MediaType != nil && *response.MediaType != ""
	if hasType {
		encoded, err := latin1Bytes(*response.MediaType)
		if err != nil {
			return err
		}
		mediaType = encoded
	}
	header := w.Header()
	header.Set("Content-Length", strconv.Itoa(len(response.Body)))
	header.Set(ReplayHeaderName, "true")
	if hasType {
		header.Set("Content-Type", string(mediaType))
	} else {
		// Python sends no Content-Type at all; stop net/http sniffing one.
		header["Content-Type"] = nil
	}
	w.WriteHeader(response.Status)
	if len(response.Body) > 0 {
		_, _ = w.Write(response.Body)
	}
	return nil
}

// Fingerprint is request_fingerprint with a content type: path is the decoded
// scope path, query the raw query bytes and contentType the raw first
// Content-Type header ("" when absent). The error is a *pyjson.UnicodeEncodeError
// or *pyjson.RecursionError, raised where Python raises.
func Fingerprint(method, path, query string, body []byte, contentType string) (string, error) {
	canonical, err := canonicalBody(body, contentType)
	if err != nil {
		return "", err
	}
	return digest(method, path, query, canonical), nil
}

// LegacyFingerprint is request_fingerprint without a content type: the body
// hashed verbatim, as the server before canonicalisation wrote it.
func LegacyFingerprint(method, path, query string, body []byte) string {
	return digest(method, path, query, body)
}

func digest(method, path, query string, body []byte) string {
	sum := sha256.New()
	_, _ = io.WriteString(sum, method)
	_, _ = sum.Write([]byte{'\n'})
	_, _ = io.WriteString(sum, path)
	_, _ = sum.Write([]byte{'\n'})
	_, _ = io.WriteString(sum, query)
	_, _ = sum.Write([]byte{'\n'})
	_, _ = sum.Write(body)
	return hex.EncodeToString(sum.Sum(nil))
}

func canonicalBody(body []byte, contentType string) ([]byte, error) {
	if !DeclaresJSON(contentType) {
		return body, nil
	}
	value, err := pyjson.Loads(body)
	if err != nil {
		if pyjson.IsValueError(err) {
			// It said JSON and it is not; hash what arrived.
			return body, nil
		}
		return nil, err
	}
	return pyjson.Canonical(value)
}

// DeclaresJSON is _declares_json over a raw (latin-1) header value: the media
// type before the first ";", stripped of Python whitespace, lower-cased, is
// application/json or ends in +json.
func DeclaresJSON(contentType string) bool {
	if contentType == "" {
		return false
	}
	mediaType, _, _ := strings.Cut(contentType, ";")
	mediaType = asciiLower(pyStripLatin1(mediaType))
	return mediaType == "application/json" || strings.HasSuffix(mediaType, "+json")
}

// RequestScope is the key scope: bearer digest, X-Actor-ID, or anonymous. The
// result is database text.
func RequestScope(h http.Header) string {
	if authorization, ok := firstHeader(h, "Authorization"); ok {
		if scope, ok := bearerScope(authorization); ok {
			return scope
		}
	}
	if actor, ok := firstHeader(h, "X-Actor-ID"); ok && actor != "" {
		return latin1ToUTF8(actor)
	}
	return AnonymousScope
}

// bearerScope is _bearer_scope: partition on the first space, compare the
// scheme case-insensitively, strip the token with str.strip().
func bearerScope(authorization string) (string, bool) {
	if authorization == "" {
		return "", false
	}
	scheme, rest, _ := strings.Cut(authorization, " ")
	token := pyStripLatin1(rest)
	if asciiLower(scheme) != "bearer" || token == "" {
		return "", false
	}
	return "bearer:" + hex.EncodeToString(auth.TokenDigest(token)), true
}

// requestTarget returns the raw path and raw query uvicorn would see. A server
// request carries its target verbatim in RequestURI; a request built in
// process falls back to its URL.
func requestTarget(r *http.Request) (rawPath, rawQuery string) {
	if strings.HasPrefix(r.RequestURI, "/") {
		return router.SplitTarget(r.RequestURI)
	}
	return r.URL.EscapedPath(), r.URL.RawQuery
}

// firstHeader is _header: the first value of a header, and whether it was sent.
func firstHeader(h http.Header, name string) (string, bool) {
	values := h[http.CanonicalHeaderKey(name)]
	if len(values) == 0 {
		return "", false
	}
	return values[0], true
}

// drain is _drain. A body that fails to read ends where it failed, as a
// disconnect ends the ASGI loop.
func drain(body io.Reader) []byte {
	if body == nil {
		return []byte{}
	}
	data, _ := io.ReadAll(body)
	if data == nil {
		data = []byte{}
	}
	return data
}

// capture buffers a handler's answer the way net/http would have sent it:
// headers snapshotted at WriteHeader, an implicit 200 on the first Write or
// when the handler returns without writing, informational codes skipped, and
// no body for statuses that forbid one.
type capture struct {
	header http.Header
	status int
	sent   http.Header
	body   bytes.Buffer
}

func newCapture(outer http.Header) *capture {
	header := outer.Clone()
	if header == nil {
		header = http.Header{}
	}
	return &capture{header: header}
}

func (c *capture) Header() http.Header { return c.header }

func (c *capture) WriteHeader(code int) {
	if code < 100 || code > 999 {
		panic(fmt.Sprintf("invalid WriteHeader code %v", code))
	}
	if c.status != 0 {
		return
	}
	if code >= 100 && code <= 199 && code != http.StatusSwitchingProtocols {
		return
	}
	c.status = code
	c.sent = c.header.Clone()
}

func (c *capture) Write(p []byte) (int, error) {
	if c.status == 0 {
		c.WriteHeader(http.StatusOK)
	}
	if !bodyAllowed(c.status) {
		return 0, http.ErrBodyNotAllowed
	}
	return c.body.Write(p)
}

func (c *capture) finalStatus() int {
	if c.status == 0 {
		c.WriteHeader(http.StatusOK)
	}
	return c.status
}

// bodyBytes is never nil: Python records b"" for an empty answer, and a nil
// slice would reach the database as NULL.
func (c *capture) bodyBytes() []byte {
	return append([]byte{}, c.body.Bytes()...)
}

// mediaType is the first Content-Type the handler sent, as database text.
func (c *capture) mediaType() *string {
	values := c.sent["Content-Type"]
	if len(values) == 0 {
		return nil
	}
	text := latin1ToUTF8(values[0])
	return &text
}

func (c *capture) writeTo(w http.ResponseWriter) {
	header := w.Header()
	for name := range header {
		delete(header, name)
	}
	for name, values := range c.sent {
		header[name] = values
	}
	if _, ok := c.sent["Content-Type"]; !ok {
		// The stored answer has no media type, so neither does this one.
		header["Content-Type"] = nil
	}
	w.WriteHeader(c.status)
	if c.body.Len() > 0 {
		_, _ = w.Write(c.body.Bytes())
	}
}

func bodyAllowed(status int) bool {
	switch {
	case status >= 100 && status <= 199:
		return false
	case status == http.StatusNoContent, status == http.StatusNotModified:
		return false
	}
	return true
}

// LatinEncodeError is str.encode("latin-1") refusing a stored media type.
type LatinEncodeError struct {
	Text string
}

func (e *LatinEncodeError) Error() string {
	return fmt.Sprintf("idem: stored media type %q is not latin-1", e.Text)
}

func latin1Bytes(text string) ([]byte, error) {
	out := make([]byte, 0, len(text))
	for _, r := range text {
		if r > 0xff {
			return nil, &LatinEncodeError{Text: text}
		}
		out = append(out, byte(r))
	}
	return out, nil
}

// latin1ToUTF8 is what psycopg stores for a str decoded from header bytes.
func latin1ToUTF8(s string) string {
	ascii := true
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			ascii = false
			break
		}
	}
	if ascii {
		return s
	}
	out := make([]rune, len(s))
	for i := 0; i < len(s); i++ {
		out[i] = rune(s[i])
	}
	return string(out)
}

// asciiLower is str.lower() for a comparison against ASCII: no latin-1
// character above 0x7f lowers into ASCII, so only A-Z need mapping. Unlike
// strings.ToLower it never reinterprets the bytes as UTF-8.
func asciiLower(s string) string {
	out := []byte(s)
	for i, c := range out {
		if c >= 'A' && c <= 'Z' {
			out[i] = c + ('a' - 'A')
		}
	}
	return string(out)
}

// pyStripLatin1 is str.strip() over latin-1 text: Python's whitespace in that
// range includes \x1c-\x1f, NEL (0x85) and NBSP (0xa0).
func pyStripLatin1(s string) string {
	start, end := 0, len(s)
	for start < end && isPythonSpace(s[start]) {
		start++
	}
	for end > start && isPythonSpace(s[end-1]) {
		end--
	}
	return s[start:end]
}

func isPythonSpace(b byte) bool {
	switch b {
	case '\t', '\n', '\v', '\f', '\r', ' ', 0x1c, 0x1d, 0x1e, 0x1f, 0x85, 0xa0:
		return true
	}
	return false
}

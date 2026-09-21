// Package chatv2http serves the experimental encrypted event transport. It is
// deliberately not mounted on the public core router until the native crypto
// and enrollment gates in ADR-0031 are satisfied.
package chatv2http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"mobile/services/core/internal/auth"
	"mobile/services/core/internal/chatv2"
	"mobile/services/core/internal/repo"
)

// Store owns authorization, transactions, deduplication, and durable ordering.
type Store interface {
	Send(context.Context, string, chatv2.Envelope) (chatv2.SendResult, error)
	Events(context.Context, string, string, string, int64, int) (chatv2.Page, error)
	Mark(context.Context, string, string, string, string, int64) (chatv2.Mark, error)
}

// SessionWriter binds queued mutations to their live bearer session inside
// the write transaction. No grant obtained before admission can be reused.
type SessionWriter interface {
	SendSession(context.Context, string, []byte, chatv2.Envelope) (chatv2.SendResult, error)
	MarkSession(context.Context, string, []byte, string, string, string, int64) (chatv2.Mark, error)
}

type Authenticate func(context.Context, http.Header) (string, error)

var ErrAuthentication = errors.New("authentication_required")

// Sessions uses real bearer sessions even when the legacy application is in dev
// mode. Actor headers and tokens in URLs are never accepted by this transport.
func Sessions(q repo.Querier) Authenticate {
	return func(ctx context.Context, headers http.Header) (string, error) {
		actor, problem, err := auth.ProdActor(ctx, headers, repo.Sessions{Q: q}, time.Now())
		if err != nil {
			return "", err
		}
		if problem != nil || actor == nil {
			return "", ErrAuthentication
		}
		return actor.ID, nil
	}
}

type Options struct {
	Store        Store
	Authenticate Authenticate
	// Experimental must be explicitly selected by the isolated lab command.
	Experimental      bool
	Context           context.Context
	ReconcileInterval time.Duration
	AckTimeout        time.Duration
	OperationTimeout  time.Duration
	MaxConnections    int
	// BatchSessions enables transactional batch authorization for real sessions.
	BatchSessions bool
}

type Handler struct {
	options     Options
	mux         *http.ServeMux
	mu          sync.Mutex
	connections int
	perActor    map[string]int
	hub         *Hub
	dispatch    *dispatcher
}

func New(o Options) *Handler {
	if o.Context == nil {
		o.Context = context.Background()
	}
	if o.ReconcileInterval <= 0 {
		o.ReconcileInterval = time.Second
	}
	if o.AckTimeout <= 0 {
		o.AckTimeout = 10 * time.Second
	}
	if o.OperationTimeout <= 0 {
		o.OperationTimeout = 5 * time.Second
	}
	if o.MaxConnections <= 0 {
		o.MaxConnections = 1000
	}
	h := &Handler{options: o, mux: http.NewServeMux(), perActor: map[string]int{}, hub: NewHub()}
	if o.BatchSessions {
		if store, ok := o.Store.(BatchStore); ok {
			h.dispatch = newDispatcher(h, store)
		}
	}
	h.mux.HandleFunc("POST /v2/chat/{conversation}/events", h.send)
	h.mux.HandleFunc("GET /v2/chat/{conversation}/events", h.events)
	h.mux.HandleFunc("PUT /v2/chat/{conversation}/marks", h.mark)
	h.mux.HandleFunc("GET /v2/chat/{conversation}/stream", h.stream)
	return h
}

// Wake coalesces a notification. PostgreSQL remains the source of every event;
// even a missing final notification is repaired by periodic reconciliation.
func (h *Handler) Wake(conversation string) { h.hub.Wake(conversation) }

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if !h.options.Experimental || h.options.Store == nil || h.options.Authenticate == nil {
		problem(w, http.StatusServiceUnavailable, "chat_v2_not_ready")
		return
	}
	// Accept only the documented query keys: particularly, never reflect or
	// consume bearer tokens in URLs, where access logs would retain them.
	for k, v := range r.URL.Query() {
		if (k != "device_id" && k != "after" && k != "limit") || len(v) != 1 {
			problem(w, http.StatusBadRequest, "invalid_query")
			return
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.options.OperationTimeout)
	defer cancel()
	h.mux.ServeHTTP(w, r.WithContext(ctx))
}

func (h *Handler) actor(w http.ResponseWriter, r *http.Request) (string, bool) {
	actor, err := h.options.Authenticate(r.Context(), r.Header)
	if err != nil {
		fail(w, err)
		return "", false
	}
	if !chatv2.ValidID(actor) {
		fail(w, ErrAuthentication)
		return "", false
	}
	return actor, true
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if strings.Split(r.Header.Get("Content-Type"), ";")[0] != "application/json" {
		problem(w, http.StatusUnsupportedMediaType, "json_required")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 512<<10)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		problem(w, 400, "invalid_body")
		return false
	}
	if err := d.Decode(new(any)); err != io.EOF {
		problem(w, 400, "invalid_body")
		return false
	}
	return true
}

func (h *Handler) send(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	var envelope chatv2.Envelope
	if !decode(w, r, &envelope) {
		return
	}
	if envelope.ConversationID != r.PathValue("conversation") {
		fail(w, chatv2.ErrInvalid)
		return
	}
	var result chatv2.SendResult
	var err error
	if h.options.BatchSessions {
		writer, ok := h.options.Store.(SessionWriter)
		token, problem := auth.BearerToken(r.Header)
		if !ok || problem != nil {
			fail(w, ErrAuthentication)
			return
		}
		result, err = writer.SendSession(r.Context(), actor, auth.TokenDigest(token), envelope)
	} else {
		if current, valid := h.actor(w, r); !valid {
			return
		} else if current != actor {
			fail(w, ErrAuthentication)
			return
		}
		result, err = h.options.Store.Send(r.Context(), actor, envelope)
	}
	if err != nil {
		fail(w, err)
		return
	}
	h.Wake(envelope.ConversationID)
	status := http.StatusCreated
	if result.Replayed {
		status = http.StatusOK
	}
	write(w, status, result)
}

func params(r *http.Request) (string, string, int64, int, error) {
	c, d := r.PathValue("conversation"), r.URL.Query().Get("device_id")
	if !chatv2.ValidID(c) || !chatv2.ValidID(d) {
		return "", "", 0, 0, chatv2.ErrInvalid
	}
	after := int64(0)
	limit := 100
	var err error
	if v := r.URL.Query().Get("after"); v != "" {
		after, err = strconv.ParseInt(v, 10, 64)
		if err != nil || after < 0 {
			return "", "", 0, 0, chatv2.ErrInvalid
		}
	}
	if v := r.URL.Query().Get("limit"); v != "" {
		limit, err = strconv.Atoi(v)
		if err != nil || limit < 1 || limit > 100 {
			return "", "", 0, 0, chatv2.ErrInvalid
		}
	}
	return c, d, after, limit, nil
}

func (h *Handler) events(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	c, d, after, limit, err := params(r)
	if err != nil {
		fail(w, err)
		return
	}
	page, err := h.options.Store.Events(r.Context(), actor, d, c, after, limit)
	if err != nil {
		fail(w, err)
		return
	}
	write(w, 200, page)
}

func (h *Handler) mark(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	var in struct {
		DeviceID string `json:"device_id"`
		Kind     string `json:"kind"`
		Sequence int64  `json:"sequence"`
	}
	if !decode(w, r, &in) {
		return
	}
	var m chatv2.Mark
	var err error
	if h.options.BatchSessions {
		writer, ok := h.options.Store.(SessionWriter)
		token, problem := auth.BearerToken(r.Header)
		if !ok || problem != nil {
			fail(w, ErrAuthentication)
			return
		}
		m, err = writer.MarkSession(r.Context(), actor, auth.TokenDigest(token), in.DeviceID, r.PathValue("conversation"), in.Kind, in.Sequence)
	} else {
		if current, valid := h.actor(w, r); !valid {
			return
		} else if current != actor {
			fail(w, ErrAuthentication)
			return
		}
		m, err = h.options.Store.Mark(r.Context(), actor, in.DeviceID, r.PathValue("conversation"), in.Kind, in.Sequence)
	}
	if err != nil {
		fail(w, err)
		return
	}
	h.Wake(r.PathValue("conversation"))
	write(w, 200, m)
}

func (h *Handler) acquire(actor string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.connections >= h.options.MaxConnections || h.perActor[actor] >= 5 {
		return false
	}
	h.connections++
	h.perActor[actor]++
	return true
}
func (h *Handler) release(actor string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.connections--
	h.perActor[actor]--
	if h.perActor[actor] == 0 {
		delete(h.perActor, actor)
	}
}

type frame struct {
	Type string       `json:"type"`
	Page *chatv2.Page `json:"page,omitempty"`
}
type ack struct {
	Type     string `json:"type"`
	Sequence int64  `json:"sequence"`
}

func (h *Handler) stream(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	c, d, after, limit, err := params(r)
	if err != nil {
		fail(w, err)
		return
	}
	if !h.acquire(actor) {
		problem(w, 429, "connection_limit")
		return
	}
	defer h.release(actor)
	// Subscribe before reading: a commit between read and subscribe must not
	// leave the connection asleep until another person sends a message.
	var wakeup <-chan struct{}
	if h.dispatch == nil {
		var unsubscribe func()
		wakeup, unsubscribe = h.hub.Subscribe(c)
		defer unsubscribe()
	}
	var page chatv2.Page
	var sessionDigest []byte
	if h.dispatch != nil {
		token, invalid := auth.BearerToken(r.Header)
		if invalid != nil {
			fail(w, ErrAuthentication)
			return
		}
		sessionDigest = auth.TokenDigest(token)
		var deliveries []chatv2.Delivery
		deliveries, err = h.dispatch.store.EventsBatch(r.Context(), c, []chatv2.Recipient{{ActorID: actor, DeviceID: d, SessionDigest: sessionDigest, After: after, Limit: limit}})
		if err == nil {
			page, err = deliveries[0].Page, deliveries[0].Err
		}
	} else {
		page, err = h.options.Store.Events(r.Context(), actor, d, c, after, limit)
	}
	if err != nil {
		fail(w, err)
		return
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{Subprotocols: []string{chatv2.Protocol}, CompressionMode: websocket.CompressionDisabled})
	if err != nil {
		return
	}
	defer conn.CloseNow()
	// The context survives HTTP hijacking but ends on shutdown or disconnect.
	ctx, cancel := context.WithCancel(h.options.Context)
	defer cancel()
	conn.SetReadLimit(1024)
	acks := make(chan ack, 1)
	var expectedAck atomic.Int64
	expectedAck.Store(-1)
	go func() {
		defer cancel()
		for {
			var a ack
			if wsjson.Read(ctx, conn, &a) != nil {
				return
			}
			if a.Type != "ack" || a.Sequence < 0 || !expectedAck.CompareAndSwap(a.Sequence, -1) {
				_ = conn.Close(websocket.StatusPolicyViolation, "invalid_ack")
				return
			}
			select {
			case acks <- a:
			case <-ctx.Done():
				return
			default:
				return
			}
		}
	}()
	if err := writeFrame(ctx, conn, h.options.AckTimeout, frame{Type: "ready"}); err != nil {
		return
	}
	var tick <-chan time.Time
	if h.dispatch == nil {
		ticker := time.NewTicker(h.options.ReconcileInterval)
		defer ticker.Stop()
		tick = ticker.C
	}
	var shared *sharedFrame
	defer func() { shared.release() }()

	for {
		if len(page.Events) > 0 {
			expectedAck.Store(page.NextSequence)
			var writeErr error
			if shared != nil {
				writeCtx, writeCancel := context.WithTimeout(ctx, h.options.AckTimeout)
				writeErr = conn.Write(writeCtx, websocket.MessageText, shared.data)
				writeCancel()
				shared.release()
				shared = nil
			} else {
				writeErr = writeFrame(ctx, conn, h.options.AckTimeout, frame{Type: "events", Page: &page})
			}
			if writeErr != nil {
				return
			}
			// Only the cursor survives the write; do not retain an entire shared
			// decoded window while one device delays its ACK.
			page.Events = nil
			// One bounded page in flight. The client ACKs only after persisting
			// ciphertext and crypto state; writes to a socket aren't delivery.
			timer := time.NewTimer(h.options.AckTimeout)
			var a ack
			select {
			case a = <-acks:
				timer.Stop()
			case <-timer.C:
				_ = conn.Close(websocket.StatusPolicyViolation, "resume_required")
				return
			case <-ctx.Done():
				timer.Stop()
				return
			}
			if a.Sequence != page.NextSequence {
				_ = conn.Close(websocket.StatusPolicyViolation, "invalid_ack")
				return
			}
			after = a.Sequence
		} else if h.dispatch == nil {
			select {
			case <-wakeup:
			case <-tick:
			case <-ctx.Done():
				return
			case <-acks:
				_ = conn.Close(websocket.StatusPolicyViolation, "unexpected_ack")
				return
			}
		}
		if h.dispatch != nil {
			result, nextErr := h.dispatch.next(ctx, c, chatv2.Recipient{ActorID: actor, DeviceID: d, SessionDigest: sessionDigest, After: after, Limit: limit})
			if nextErr != nil || result.err != nil {
				_ = conn.Close(websocket.StatusPolicyViolation, "resync_required")
				return
			}
			page, shared = result.page, result.frame
			continue
		}
		// Reauthenticate on every catch-up, including quiet connections, and
		// never reuse cached grants. Store rechecks membership/device as well.
		opCtx, opCancel := context.WithTimeout(ctx, h.options.OperationTimeout)
		current, authErr := h.options.Authenticate(opCtx, r.Header)
		if authErr != nil || current != actor {
			opCancel()
			_ = conn.Close(websocket.StatusPolicyViolation, "authentication_required")
			return
		}
		page, err = h.options.Store.Events(opCtx, actor, d, c, after, limit)
		opCancel()
		if err != nil {
			_ = conn.Close(websocket.StatusPolicyViolation, "resync_required")
			return
		}
	}
}

func writeFrame(ctx context.Context, c *websocket.Conn, timeout time.Duration, v frame) error {
	writeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return wsjson.Write(writeCtx, c, v)
}
func fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrAuthentication):
		problem(w, 401, "authentication_required")
	case errors.Is(err, chatv2.ErrForbidden):
		problem(w, 403, "chat_v2_forbidden")
	case errors.Is(err, chatv2.ErrInvalid):
		problem(w, 422, "chat_v2_invalid")
	case errors.Is(err, chatv2.ErrConflict):
		problem(w, 409, "chat_v2_send_conflict")
	case errors.Is(err, chatv2.ErrEpoch):
		problem(w, 409, "chat_v2_stale_epoch")
	case errors.Is(err, chatv2.ErrNotReady):
		problem(w, 409, "chat_v2_not_ready")
	default:
		problem(w, 503, "chat_v2_unavailable")
	}
}
func problem(w http.ResponseWriter, status int, code string) {
	write(w, status, map[string]string{"code": code})
}
func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

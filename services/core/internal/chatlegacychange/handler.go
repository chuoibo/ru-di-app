package chatlegacychange

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/jackc/pgx/v5"
	"mobile/services/core/internal/featureroute"
)

// CandidateEnv switches the change feed and the group AI engine
// (internal/chatassist) off with "0". They are on by default when auth runs in
// prod; see resolveChatFeatures in cmd/core for the whole rule. The name is
// kept from when they were opt-in so existing hosts and scripts keep working.
const CandidateEnv = "MOBILE_CHAT_CHANGES_CANDIDATE"

type Handler struct {
	Store             Store
	Origins           []string
	Context           context.Context
	ReconcileInterval time.Duration
	AuthTimeout       time.Duration
	mux               *featureroute.Mux
	mu                sync.Mutex
	subscribers       map[string]map[chan struct{}]struct{}
	pending           chan struct{}
	slots             chan struct{}
	actors            map[string]int
}

func New(store Store, ctx context.Context, origins []string) *Handler {
	if ctx == nil {
		ctx = context.Background()
	}
	h := &Handler{Store: store, Context: ctx, Origins: origins, ReconcileInterval: time.Second, AuthTimeout: 5 * time.Second, mux: featureroute.NewMux(), subscribers: map[string]map[chan struct{}]struct{}{}, pending: make(chan struct{}, 64), slots: make(chan struct{}, 1000), actors: map[string]int{}}
	h.mux.HandleFunc("GET /contexts/{room}/changes", h.changes)
	h.mux.HandleFunc("POST /contexts/{room}/changes/snapshot", h.snapshot)
	h.mux.HandleFunc("GET /contexts/{room}/changes/stream", h.stream)
	return h
}

// Routes lists the patterns New registers, in order. The ownership manifest's
// `features` block must name exactly these (cmd/core features --json).
func Routes() []string { return New(Store{}, nil, nil).mux.Patterns() }
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	h.mux.ServeHTTP(w, r)
}

// Matches routes the feed ahead of the ownership manifest, which is rendered
// from the Python app and so has no row for a Go-only path.
func Matches(path string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	return len(parts) >= 3 && parts[0] == "contexts" && parts[2] == "changes"
}
func validUUID(s string) bool {
	if len(s) != 36 || s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' {
		return false
	}
	_, e := hex.DecodeString(strings.ReplaceAll(s, "-", ""))
	return e == nil
}
func respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, err error) {
	status, code := 503, "chat_temporarily_unavailable"
	switch {
	case errors.Is(err, ErrAuthentication):
		status, code = 401, ErrAuthentication.Error()
	case errors.Is(err, ErrForbidden):
		status, code = 403, ErrForbidden.Error()
	case errors.Is(err, ErrCursor):
		status, code = 422, ErrCursor.Error()
	}
	respond(w, status, map[string]string{"code": code, "detail": code})
}
func query(r *http.Request) (int64, int, error) {
	if !validUUID(r.PathValue("room")) {
		return 0, 0, ErrCursor
	}
	q, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return 0, 0, ErrCursor
	}
	for k, v := range q {
		if (k != "after" && k != "limit") || len(v) != 1 {
			return 0, 0, ErrCursor
		}
	}
	after, limit := int64(0), 100
	if q.Has("after") {
		after, err = strconv.ParseInt(q.Get("after"), 10, 64)
		if err != nil || after < 0 {
			return 0, 0, ErrCursor
		}
	}
	if q.Has("limit") {
		limit, err = strconv.Atoi(q.Get("limit"))
		if err != nil || limit < 1 || limit > 100 {
			return 0, 0, ErrCursor
		}
	}
	return after, limit, nil
}
func (h *Handler) changes(w http.ResponseWriter, r *http.Request) {
	after, limit, err := query(r)
	if err != nil {
		fail(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	page, err := h.Store.Changes(ctx, r.Header, r.PathValue("room"), after, limit)
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, 200, page)
}
func (h *Handler) snapshot(w http.ResponseWriter, r *http.Request) {
	if !validUUID(r.PathValue("room")) || r.URL.RawQuery != "" {
		fail(w, ErrCursor)
		return
	}
	var in SnapshotRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&in); err != nil {
		fail(w, ErrCursor)
		return
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		fail(w, ErrCursor)
		return
	}
	if len(in.MessageIDs)+len(in.VoteIDs) > 100 {
		fail(w, ErrCursor)
		return
	}
	for _, id := range append(append([]string{}, in.MessageIDs...), in.VoteIDs...) {
		if !validUUID(id) {
			fail(w, ErrCursor)
			return
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	result, err := h.Store.Snapshot(ctx, r.Header, r.PathValue("room"), in)
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, 200, result)
}
func (h *Handler) originAllowed(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" || (u.Scheme != "http" && u.Scheme != "https") {
		return false
	}
	if u.Host == r.Host {
		return true
	}
	if len(h.Origins) == 0 {
		return u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1"
	}
	for _, allowed := range h.Origins {
		if origin == strings.TrimSpace(allowed) && allowed != "*" {
			return true
		}
	}
	return false
}
func (h *Handler) stream(w http.ResponseWriter, r *http.Request) {
	after, limit, err := query(r)
	if err != nil {
		fail(w, err)
		return
	}
	if !h.originAllowed(r) {
		fail(w, ErrForbidden)
		return
	}
	select {
	case h.slots <- struct{}{}:
		defer func() { <-h.slots }()
	default:
		respond(w, 503, map[string]string{"code": "chat_capacity"})
		return
	}
	needsAuth := r.Header.Get("Authorization") == ""
	if needsAuth {
		select {
		case h.pending <- struct{}{}:
		default:
			respond(w, 503, map[string]string{"code": "chat_auth_capacity"})
			return
		}
	}
	releasePending := func() {
		if needsAuth {
			<-h.pending
			needsAuth = false
		}
	}
	defer releasePending()
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}
	defer conn.CloseNow()
	conn.SetReadLimit(4096)
	headers := r.Header.Clone()
	if needsAuth {
		ctx, cancel := context.WithTimeout(r.Context(), h.AuthTimeout)
		var frame struct {
			Type  string `json:"type"`
			Token string `json:"token"`
		}
		err = wsjson.Read(ctx, conn, &frame)
		cancel()
		if err != nil || frame.Type != "authenticate" || frame.Token == "" || len(frame.Token) > 2048 {
			_ = conn.Close(websocket.StatusPolicyViolation, "authentication_required")
			return
		}
		headers.Set("Authorization", "Bearer "+frame.Token)
	}
	connectionCtx, stopConnection := context.WithCancel(r.Context())
	defer stopConnection()
	type acknowledgement struct {
		Type     string `json:"type"`
		Sequence int64  `json:"sequence"`
	}
	acks := make(chan acknowledgement, 1)
	go func() {
		defer stopConnection()
		for {
			var ack acknowledgement
			if err := wsjson.Read(connectionCtx, conn, &ack); err != nil {
				return
			}
			if ack.Type != "ack" {
				return
			}
			select {
			case acks <- ack:
			case <-connectionCtx.Done():
				return
			default:
				return
			}
		}
	}()
	room := r.PathValue("room")
	wake := h.subscribe(room)
	defer h.unsubscribe(room, wake)
	ticker := time.NewTicker(h.ReconcileInterval)
	defer ticker.Stop()
	first := true
	actorSlot := ""
	defer func() {
		if actorSlot != "" {
			h.mu.Lock()
			h.actors[actorSlot]--
			if h.actors[actorSlot] == 0 {
				delete(h.actors, actorSlot)
			}
			h.mu.Unlock()
		}
	}()
	for {
		ctx, cancel := context.WithTimeout(connectionCtx, 5*time.Second)
		page, e := h.Store.Changes(ctx, headers, room, after, limit)
		cancel()
		if e != nil {
			_ = conn.Close(websocket.StatusPolicyViolation, "chat_unavailable")
			return
		}
		if actorSlot == "" {
			h.mu.Lock()
			full := h.actors[page.ActorID] >= 5
			if !full {
				h.actors[page.ActorID]++
				actorSlot = page.ActorID
			}
			h.mu.Unlock()
			if full {
				_ = conn.Close(websocket.StatusPolicyViolation, "chat_actor_capacity")
				return
			}
		}
		releasePending()
		if first || len(page.Changes) > 0 {
			first = false
			ctx, cancel = context.WithTimeout(connectionCtx, 10*time.Second)
			if e = wsjson.Write(ctx, conn, page); e != nil {
				cancel()
				return
			}
			var ack acknowledgement
			select {
			case ack = <-acks:
			case <-ctx.Done():
				cancel()
				return
			}
			cancel()
			if ack.Sequence != page.NextSequence {
				_ = conn.Close(websocket.StatusPolicyViolation, "ack_invalid")
				return
			}
			after = page.NextSequence
		}
		if page.HasMore {
			continue
		}
		select {
		case <-connectionCtx.Done():
			return
		case <-h.Context.Done():
			return
		case <-ticker.C:
		case <-wake:
		}
	}
}
func (h *Handler) subscribe(room string) chan struct{} {
	h.mu.Lock()
	defer h.mu.Unlock()
	c := make(chan struct{}, 1)
	if h.subscribers[room] == nil {
		h.subscribers[room] = map[chan struct{}]struct{}{}
	}
	h.subscribers[room][c] = struct{}{}
	return c
}
func (h *Handler) unsubscribe(room string, c chan struct{}) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.subscribers[room], c)
	if len(h.subscribers[room]) == 0 {
		delete(h.subscribers, room)
	}
}
func (h *Handler) wake(room string) {
	h.mu.Lock()
	targets := make([]chan struct{}, 0, len(h.subscribers[room]))
	for c := range h.subscribers[room] {
		targets = append(targets, c)
	}
	h.mu.Unlock()
	for _, c := range targets {
		select {
		case c <- struct{}{}:
		default:
		}
	}
}

// Listen reconnects without assuming that NOTIFY is durable. The periodic
// database catch-up repairs every missed notification and listener restart.
func (h *Handler) Listen() {
	for h.Context.Err() == nil {
		conn, err := pgx.ConnectConfig(h.Context, h.Store.Pool.Config().ConnConfig.Copy())
		if err == nil {
			_, err = conn.Exec(h.Context, `LISTEN chat_legacy_changes`)
		}
		if err == nil {
			for {
				note, e := conn.WaitForNotification(h.Context)
				if e != nil {
					break
				}
				if validUUID(note.Payload) {
					h.wake(note.Payload)
				}
			}
		}
		if conn != nil {
			_ = conn.Close(context.Background())
		}
		timer := time.NewTimer(time.Second)
		select {
		case <-h.Context.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

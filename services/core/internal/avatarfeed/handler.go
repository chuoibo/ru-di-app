package avatarfeed

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"mobile/services/core/internal/auth"
	"mobile/services/core/internal/featureroute"
)

// MaxIDs bounds one versions request: a roster, not a directory.
const MaxIDs = 100

// Backend is what the handler reads; Store is the Postgres one.
type Backend interface {
	Authenticate(ctx context.Context, h http.Header) (string, error)
	Actor(ctx context.Context, digest []byte) (string, error)
	Versions(ctx context.Context, actor string, ids []string) (map[string]*string, error)
	Audience(ctx context.Context, subject string) ([]string, *string, error)
	ContextMembers(ctx context.Context, context string) ([]string, error)
}

// Event is one frame on the stream. "ready" means: whatever you knew may be
// stale, ask GET /people/avatars again. "avatar" names one person's new
// current avatar (nil: none).
type Event struct {
	Type     string  `json:"type"`
	PersonID string  `json:"person_id,omitempty"`
	AvatarID *string `json:"avatar_id,omitempty"`
}

type Handler struct {
	Backend      Backend
	Pool         *pgxpool.Pool
	Origins      []string
	Context      context.Context
	AuthTimeout  time.Duration
	Revalidate   time.Duration
	PingInterval time.Duration
	mux          *featureroute.Mux
	mu           sync.Mutex
	conns        map[string]map[chan Event]struct{}
	slots        chan struct{}
}

func New(backend Backend, pool *pgxpool.Pool, ctx context.Context, origins []string) *Handler {
	if ctx == nil {
		ctx = context.Background()
	}
	h := &Handler{Backend: backend, Pool: pool, Context: ctx, Origins: origins, AuthTimeout: 5 * time.Second,
		Revalidate: time.Minute, PingInterval: 30 * time.Second, mux: featureroute.NewMux(),
		conns: map[string]map[chan Event]struct{}{}, slots: make(chan struct{}, 2000)}
	h.mux.HandleFunc("GET /people/avatars", h.versions)
	h.mux.HandleFunc("GET /people/avatars/stream", h.stream)
	return h
}

// Routes lists the patterns New registers, in order. The ownership manifest's
// `features` block must name exactly these (cmd/core features --json).
func Routes() []string { return New(nil, nil, nil, nil).mux.Patterns() }

// Matches routes these two Go-only paths ahead of the ownership manifest,
// which is rendered from the Python app and has no row for them. Neither
// collides with a real person: `avatars` is not a UUID.
func Matches(path string) bool {
	return path == "/people/avatars" || path == "/people/avatars/stream"
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	h.mux.ServeHTTP(w, r)
}

func respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func refuse(w http.ResponseWriter, status int, code string) {
	respond(w, status, map[string]string{"code": code, "detail": code})
}

func validUUID(s string) bool {
	if len(s) != 36 || s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' {
		return false
	}
	_, e := hex.DecodeString(strings.ReplaceAll(s, "-", ""))
	return e == nil
}

// ParseIDs reads `ids=a,b,c`: the only parameter, once, 1..MaxIDs canonical
// UUIDs after lowercasing, duplicates dropped in first-seen order.
func ParseIDs(rawQuery string) ([]string, bool) {
	q, err := url.ParseQuery(rawQuery)
	if err != nil || len(q) != 1 || len(q["ids"]) != 1 {
		return nil, false
	}
	seen := map[string]bool{}
	var ids []string
	for _, part := range strings.Split(q.Get("ids"), ",") {
		id := strings.ToLower(strings.TrimSpace(part))
		if !validUUID(id) {
			return nil, false
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 || len(ids) > MaxIDs {
		return nil, false
	}
	return ids, true
}

func (h *Handler) versions(w http.ResponseWriter, r *http.Request) {
	ids, ok := ParseIDs(r.URL.RawQuery)
	if !ok {
		refuse(w, 422, "ids_invalid")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	actor, err := h.Backend.Authenticate(ctx, r.Header)
	if errors.Is(err, ErrAuthentication) {
		refuse(w, 401, ErrAuthentication.Error())
		return
	}
	if err != nil {
		refuse(w, 503, "avatars_temporarily_unavailable")
		return
	}
	out, err := h.Backend.Versions(ctx, actor, ids)
	if err != nil {
		refuse(w, 503, "avatars_temporarily_unavailable")
		return
	}
	respond(w, 200, map[string]any{"avatars": out})
}

// OriginAllowed is the chat feed's rule: same host, the configured list, or
// loopback when none is configured.
func (h *Handler) OriginAllowed(r *http.Request) bool {
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

func (h *Handler) register(actor string) (chan Event, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.conns[actor]) >= 5 {
		return nil, false
	}
	c := make(chan Event, 32)
	if h.conns[actor] == nil {
		h.conns[actor] = map[chan Event]struct{}{}
	}
	h.conns[actor][c] = struct{}{}
	return c, true
}

func (h *Handler) unregister(actor string, c chan Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.conns[actor], c)
	if len(h.conns[actor]) == 0 {
		delete(h.conns, actor)
	}
}

// deliver hands e to every connection of the given viewers. A connection too
// far behind to take it is closed: its client reconnects and resyncs, which is
// the one recovery path, instead of silently missing a change. Sends never
// block, so they happen under the lock and a channel is never sent to after
// it is closed.
func (h *Handler) deliver(viewers []string, e Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, v := range viewers {
		for c := range h.conns[v] {
			select {
			case c <- e:
			default:
				delete(h.conns[v], c)
				close(c)
			}
		}
		if len(h.conns[v]) == 0 {
			delete(h.conns, v)
		}
	}
}

// Broadcast is what the listener does with one NOTIFY: read who may see the
// subject now and which avatar is current, then tell them.
func (h *Handler) Broadcast(ctx context.Context, subject string) error {
	viewers, version, err := h.Backend.Audience(ctx, subject)
	if err != nil {
		return err
	}
	h.deliver(viewers, Event{Type: "avatar", PersonID: subject, AvatarID: version})
	return nil
}

// MembershipChanged is what the listener does when someone entered or left
// an active membership of room: who may see whose avatar just changed for
// every active member of room and for the person, so each of them is told to
// ask again. An answer cached as "not visible" (or a picture now forbidden)
// does not outlive the membership that decided it.
func (h *Handler) MembershipChanged(ctx context.Context, room, person string) error {
	members, err := h.Backend.ContextMembers(ctx, room)
	if err != nil {
		return err
	}
	h.deliver(append(members, person), Event{Type: "ready"})
	return nil
}

// dispatch reads one NOTIFY payload: a bare person id is a new avatar,
// "m:<context>:<person>" a membership that entered or left `active`.
func (h *Handler) dispatch(ctx context.Context, payload string) error {
	if validUUID(payload) {
		return h.Broadcast(ctx, payload)
	}
	parts := strings.Split(payload, ":")
	if len(parts) == 3 && parts[0] == "m" && validUUID(parts[1]) && validUUID(parts[2]) {
		return h.MembershipChanged(ctx, parts[1], parts[2])
	}
	return nil
}

// resyncAll tells every open connection to ask again: after the listener
// (re)connects, notifications sent while it was away are gone.
func (h *Handler) resyncAll() {
	h.mu.Lock()
	var all []string
	for actor := range h.conns {
		all = append(all, actor)
	}
	h.mu.Unlock()
	h.deliver(all, Event{Type: "ready"})
}

func (h *Handler) stream(w http.ResponseWriter, r *http.Request) {
	if r.URL.RawQuery != "" {
		refuse(w, 422, "query_invalid")
		return
	}
	if !h.OriginAllowed(r) {
		refuse(w, 403, "origin_forbidden")
		return
	}
	select {
	case h.slots <- struct{}{}:
		defer func() { <-h.slots }()
	default:
		refuse(w, 503, "avatars_capacity")
		return
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}
	defer conn.CloseNow()
	conn.SetReadLimit(4096)
	token, problem := auth.BearerToken(r.Header)
	if problem != nil {
		// A browser cannot set a header on a WebSocket, so the token may come
		// as the first frame instead -- the chat feed's handshake.
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
		token = frame.Token
	}
	digest := auth.TokenDigest(token)
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	actor, err := h.Backend.Actor(ctx, digest)
	cancel()
	if err != nil {
		_ = conn.Close(websocket.StatusPolicyViolation, "authentication_required")
		return
	}
	events, ok := h.register(actor)
	if !ok {
		_ = conn.Close(websocket.StatusPolicyViolation, "avatars_actor_capacity")
		return
	}
	defer h.unregister(actor, events)
	live := conn.CloseRead(r.Context())
	write := func(e Event) bool {
		ctx, cancel := context.WithTimeout(live, 10*time.Second)
		defer cancel()
		return wsjson.Write(ctx, conn, e) == nil
	}
	if !write(Event{Type: "ready"}) {
		return
	}
	check := time.NewTicker(h.Revalidate)
	defer check.Stop()
	ping := time.NewTicker(h.PingInterval)
	defer ping.Stop()
	for {
		select {
		case <-live.Done():
			return
		case <-h.Context.Done():
			_ = conn.Close(websocket.StatusGoingAway, "server_stopping")
			return
		case e, open := <-events:
			if !open {
				_ = conn.Close(websocket.StatusTryAgainLater, "resync")
				return
			}
			if !write(e) {
				return
			}
		case <-check.C:
			ctx, cancel := context.WithTimeout(live, 5*time.Second)
			again, err := h.Backend.Actor(ctx, digest)
			cancel()
			if err != nil || again != actor {
				_ = conn.Close(websocket.StatusPolicyViolation, "authentication_required")
				return
			}
		case <-ping.C:
			ctx, cancel := context.WithTimeout(live, 10*time.Second)
			err := conn.Ping(ctx)
			cancel()
			if err != nil {
				return
			}
		}
	}
}

// Listen reconnects without assuming NOTIFY is durable: every (re)connect
// tells open streams to resync, which repairs whatever was missed meanwhile.
func (h *Handler) Listen() {
	for h.Context.Err() == nil {
		conn, err := pgx.ConnectConfig(h.Context, h.Pool.Config().ConnConfig.Copy())
		if err == nil {
			_, err = conn.Exec(h.Context, `LISTEN avatar_changes`)
		}
		if err == nil {
			h.resyncAll()
			for {
				note, e := conn.WaitForNotification(h.Context)
				if e != nil {
					break
				}
				ctx, cancel := context.WithTimeout(h.Context, 5*time.Second)
				_ = h.dispatch(ctx, note.Payload)
				cancel()
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

package community

import (
	"context"
	"encoding/hex"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"mobile/services/core/internal/auth"
)

type subscriber struct {
	person  string
	digest  []byte
	initial int64
	posts   map[string]bool
	send    chan frame
	cancel  context.CancelFunc
}
type frame struct {
	Kind   string `json:"kind"`
	Cursor int64  `json:"cursor"`
	PostID string `json:"post_id,omitempty"`
	At     int64  `json:"occurred_at_ms,omitempty"`
}

func (h *Handler) stream(w http.ResponseWriter, r *http.Request) {
	patterns := []string{}
	for _, origin := range h.origins {
		u, err := url.Parse(origin)
		if err == nil && u.Host != "" {
			patterns = append(patterns, u.Host)
		}
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: patterns})
	if err != nil {
		return
	}
	defer conn.CloseNow()
	conn.SetReadLimit(8192)
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	authCtx, stop := context.WithTimeout(ctx, 5*time.Second)
	var in struct {
		Token string   `json:"token"`
		Posts []string `json:"posts"`
		After int64    `json:"after"`
	}
	err = wsjson.Read(authCtx, conn, &in)
	stop()
	if err != nil || len(in.Posts) > 40 || len(in.Token) > 1024 {
		_ = conn.Close(websocket.StatusPolicyViolation, "invalid_subscription")
		return
	}
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return
	}
	person, err := session(ctx, tx, auth.TokenDigest(in.Token))
	if err != nil {
		tx.Rollback(ctx)
		_ = conn.Close(websocket.StatusPolicyViolation, "authentication_required")
		return
	}
	var cursor int64
	err = tx.QueryRow(ctx, `SELECT COALESCE(max(id),0) FROM community_events`).Scan(&cursor)
	if err != nil {
		tx.Rollback(ctx)
		return
	}
	if err = tx.Commit(ctx); err != nil {
		return
	}
	s := &subscriber{person: person, digest: auth.TokenDigest(in.Token), initial: cursor, posts: map[string]bool{}, send: make(chan frame, 32), cancel: cancel}
	for _, id := range in.Posts {
		if validID(id) {
			s.posts[id] = true
		}
	}
	h.mu.Lock()
	n := 0
	for existing := range h.clients {
		if existing.person == person {
			n++
		}
	}
	if n >= 4 || len(h.clients) >= 12000 {
		h.mu.Unlock()
		_ = conn.Close(websocket.StatusTryAgainLater, "stream_capacity")
		return
	}
	// Queue the baseline before the relay can see this subscriber. An event
	// racing registration must never arrive before the first sync frame.
	s.send <- frame{Kind: "sync", Cursor: cursor}
	h.clients[s] = struct{}{}
	h.mu.Unlock()
	defer func() { h.mu.Lock(); delete(h.clients, s); h.mu.Unlock() }()
	// Every connection begins with a new snapshot request. Register first, then fetch,
	// so any concurrent writes are either in the snapshot or in the bounded queue.
	readCtx := conn.CloseRead(ctx)
	tick := time.NewTicker(20 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-readCtx.Done():
			return
		case <-ctx.Done():
			_ = conn.Close(websocket.StatusPolicyViolation, "resync_required")
			return
		case f := <-s.send:
			writeCtx, done := context.WithTimeout(ctx, 3*time.Second)
			err = wsjson.Write(writeCtx, conn, f)
			done()
			if err != nil {
				return
			}
		case <-tick.C:
			pingCtx, done := context.WithTimeout(ctx, 5*time.Second)
			err = conn.Ping(pingCtx)
			done()
			if err != nil {
				return
			}
		}
	}
}
func (h *Handler) subscribers() []*subscriber {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]*subscriber, 0, len(h.clients))
	for s := range h.clients {
		out = append(out, s)
	}
	return out
}
func offer(s *subscriber, f frame) {
	select {
	case s.send <- f:
	default:
		s.cancel()
	}
}
func (h *Handler) checkSessions(ctx context.Context, conn *pgxpool.Conn) {
	all := h.subscribers()
	if len(all) == 0 {
		return
	}
	digests := make([][]byte, 0, len(all))
	for _, s := range all {
		digests = append(digests, s.digest)
	}
	rows, err := conn.Query(ctx, `SELECT encode(s.token_digest,'hex') FROM account_sessions s JOIN people p ON p.id=s.person_id WHERE s.token_digest=ANY($1::bytea[]) AND s.revoked_at IS NULL AND s.expires_at>clock_timestamp() AND p.deleted_at IS NULL`, digests)
	if err != nil {
		for _, s := range all {
			s.cancel()
		}
		return
	}
	valid := map[string]bool{}
	for rows.Next() {
		var d string
		if rows.Scan(&d) == nil {
			valid[d] = true
		}
	}
	rows.Close()
	if rows.Err() != nil {
		valid = map[string]bool{}
	}
	for _, s := range all {
		if !valid[hex.EncodeToString(s.digest)] {
			s.cancel()
		}
	}
}
func (h *Handler) runEvents(ctx context.Context) {
	// Reserve one connection inside the existing pool limit. Otherwise every
	// recipient batch queues behind feed requests again, multiplying event lag
	// and delaying session revocation precisely when the server is busiest.
	conn, err := h.relayConnection(ctx)
	if err != nil {
		return
	}
	defer func() {
		if conn != nil {
			conn.Release()
		}
	}()
	var cursor int64
	_ = conn.QueryRow(ctx, `SELECT COALESCE(max(id),0) FROM community_events`).Scan(&cursor)
	// HTTP may accept readers before this goroutine gets scheduled. Retain
	// events committed after any already-connected reader's initial baseline.
	for _, s := range h.subscribers() {
		cursor = min(cursor, s.initial)
	}
	wake := make(chan struct{}, 1)
	var bus *redis.Client
	if raw := os.Getenv("MOBILE_COMMUNITY_REDIS_URL"); raw != "" {
		opts, err := redis.ParseURL(raw)
		if err == nil {
			opts.DialTimeout = time.Second
			opts.ReadTimeout = time.Second
			opts.WriteTimeout = time.Second
			opts.PoolSize = 2
			bus = redis.NewClient(opts)
			defer bus.Close()
			sub := bus.Subscribe(ctx, "rudi:community:wake")
			defer sub.Close()
			go func() {
				ch := sub.Channel()
				for {
					select {
					case <-ctx.Done():
						return
					case _, ok := <-ch:
						if !ok {
							return
						}
						select {
						case wake <- struct{}{}:
						default:
						}
					}
				}
			}()
		}
	}
	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()
	authTick := time.NewTicker(10 * time.Second)
	defer authTick.Stop()
	for {
		if conn.Conn().IsClosed() {
			conn.Release()
			conn, err = h.relayConnection(ctx)
			if err != nil {
				return
			}
		}
		select {
		case <-ctx.Done():
			for _, s := range h.subscribers() {
				s.cancel()
			}
			return
		case <-authTick.C:
			h.checkSessions(ctx, conn)
		case <-tick.C:
		case <-wake:
		}
		if ctx.Err() != nil {
			return
		}
		rows, err := conn.Query(ctx, `SELECT id,COALESCE(post_id::text,''),kind,(extract(epoch FROM created_at)*1000)::bigint FROM community_events WHERE id>$1 ORDER BY id LIMIT 1024`, cursor)
		if err != nil {
			continue
		}
		events := []frame{}
		for rows.Next() {
			var f frame
			if rows.Scan(&f.Cursor, &f.PostID, &f.Kind, &f.At) == nil {
				events = append(events, f)
			}
		}
		rows.Close()
		if rows.Err() != nil {
			continue
		}
		if len(events) == 0 {
			continue
		}
		all := h.subscribers()
		latest := events[len(events)-1].Cursor
		access := false
		changed := map[string]int64{}
		when := map[string]int64{}
		for _, e := range events {
			if e.Kind == "access.changed" {
				access = true
			} else {
				changed[e.PostID] = e.Cursor
				when[e.PostID] = e.At
			}
		}
		if access {
			h.checkSessions(ctx, conn)
		}
		// Resolve all watched recipients in bounded SQL batches, not one query per socket.
		allowed := map[string]map[string]bool{}
		hasPublic := false
		for id := range changed {
			if !validID(id) {
				continue
			}
			var public bool
			if err := conn.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM posts p JOIN community_posts c ON c.post_id=p.id WHERE p.id=$1 AND p.audience='public' AND c.published_revision IS NOT NULL AND c.deleted_at IS NULL)`, id).Scan(&public); err == nil {
				hasPublic = hasPublic || public
			}
			people := []string{}
			seen := map[string]bool{}
			for _, s := range all {
				if s.posts[id] && !seen[s.person] {
					people = append(people, s.person)
					seen[s.person] = true
				}
			}
			allowed[id] = map[string]bool{}
			for start := 0; start < len(people); start += 2000 {
				end := min(start+2000, len(people))
				rows, e := conn.Query(ctx, `SELECT v.person_id FROM unnest($1::uuid[]) v(person_id) JOIN posts p ON p.id=$2 JOIN people a ON a.id=p.author_id LEFT JOIN community_posts c ON c.post_id=p.id WHERE a.deleted_at IS NULL AND c.deleted_at IS NULL AND (`+strings.ReplaceAll(readableSQL, "$1", "v.person_id")+`)`, people[start:end], id)
				if e != nil {
					continue
				}
				for rows.Next() {
					var person string
					if rows.Scan(&person) == nil {
						allowed[id][person] = true
					}
				}
				rows.Close()
			}
		}
		for _, s := range all {
			if access {
				offer(s, frame{Kind: "sync", Cursor: latest})
				continue
			}
			// Hints contain no author or content; readers fetch through current ACLs.
			if hasPublic {
				offer(s, frame{Kind: "feed.changed", Cursor: latest, At: events[len(events)-1].At})
			}
			for id, seq := range changed {
				if !s.posts[id] {
					continue
				}
				if !allowed[id][s.person] {
					offer(s, frame{Kind: "sync", Cursor: seq})
					continue
				}
				offer(s, frame{Kind: "post.changed", Cursor: seq, PostID: id, At: when[id]})
			}
		}
		cursor = latest
		if len(events) == 1024 {
			// Drain a burst without adding another reconciliation interval.
			select {
			case wake <- struct{}{}:
			default:
			}
		}
		// The relay marker is for retention only; replicas each reconcile their own cursor.
		tag, err := conn.Exec(ctx, `UPDATE community_events SET relayed_at=clock_timestamp() WHERE id<=$1 AND relayed_at IS NULL`, cursor)
		if err == nil && tag.RowsAffected() > 0 && bus != nil {
			publishCtx, done := context.WithTimeout(ctx, time.Second)
			_ = bus.Publish(publishCtx, "rudi:community:wake", strings.TrimSpace("wake")).Err()
			done()
		}
	}
}

func (h *Handler) relayConnection(ctx context.Context) (*pgxpool.Conn, error) {
	for {
		conn, err := h.pool.Acquire(ctx)
		if err == nil || ctx.Err() != nil {
			return conn, err
		}
		timer := time.NewTimer(500 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

package community

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"mobile/services/core/internal/auth"
	"mobile/services/core/internal/testdb"
)

type world struct {
	pool   *pgxpool.Pool
	h      *Handler
	people [4]string
	tokens [4]string
}

func setup(t *testing.T) world {
	t.Helper()
	pool := testdb.Pool(t)
	if err := Migrate(context.Background(), pool); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(context.Background(), pool); err != nil {
		t.Fatal(err)
	}
	f := world{pool: pool}
	f.h = New(pool, nil, nil)
	for i := range f.people {
		f.people[i] = uuid()
		f.tokens[i] = "synthetic-community-" + uuid()
		f.exec(t, `INSERT INTO people(id,display_name) VALUES($1,$2)`, f.people[i], fmt.Sprintf("Synthetic community %d", i))
		f.exec(t, `INSERT INTO account_sessions(id,person_id,token_digest,issued_via,expires_at) VALUES($1,$2,$3,'genesis',clock_timestamp()+interval '1 hour')`, uuid(), f.people[i], auth.TokenDigest(f.tokens[i]))
	}
	f.exec(t, `INSERT INTO community_moderators VALUES($1)`, f.people[3])
	t.Cleanup(func() {
		_, err := pool.Exec(context.Background(), `DELETE FROM community_jobs WHERE post_id IN (SELECT id FROM posts WHERE author_id=ANY($1::uuid[])) OR comment_id IN (SELECT id FROM community_comment_drafts WHERE author_id=ANY($1::uuid[]))`, f.people[:])
		if err != nil {
			t.Error(err)
		}
	})
	return f
}
func (f world) exec(t *testing.T, query string, args ...any) {
	t.Helper()
	if _, err := f.pool.Exec(context.Background(), query, args...); err != nil {
		t.Fatal(err)
	}
}
func (f world) call(method, path string, actor int, body any) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	r := httptest.NewRequest(method, path, bytes.NewReader(b))
	r.Header.Set("Authorization", "Bearer "+f.tokens[actor])
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	f.h.ServeHTTP(w, r)
	return w
}
func requireStatus(t *testing.T, w *httptest.ResponseRecorder, status int) {
	t.Helper()
	if w.Code != status {
		t.Fatalf("status %d want %d: %s", w.Code, status, w.Body.String())
	}
}
func (f world) post(t *testing.T, audience string) Post {
	t.Helper()
	w := f.call("POST", "/v2/community/posts", 0, PostInput{LogicalID: uuid(), Body: "Một ngày đi bộ ngắm hồ cùng hội bạn", Audience: audience, Topics: []string{"Đi bộ"}})
	requireStatus(t, w, 201)
	var p Post
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	return p
}
func (f world) approve(t *testing.T, p Post) {
	t.Helper()
	requireStatus(t, f.call("POST", "/v2/community/posts/"+p.ID+"/review", 3, map[string]any{"revision": p.Revision, "approve": true, "reason": "Nội dung tổng hợp đúng chủ đề"}), 200)
}
func (f world) friend(t *testing.T) {
	t.Helper()
	f.exec(t, `INSERT INTO friend_requests(id,requester_id,addressee_id,state,decided_at) VALUES($1,$2,$3,'accepted',clock_timestamp())`, uuid(), f.people[0], f.people[1])
}
func TestPostgresCommunityPendingCannotLeakViaLegacy(t *testing.T) {
	f := setup(t)
	p := f.post(t, "public")
	requireStatus(t, f.call("GET", "/v2/community/posts/"+p.ID, 1, nil), 404)
	var a string
	if err := f.pool.QueryRow(context.Background(), `SELECT audience FROM posts WHERE id=$1`, p.ID).Scan(&a); err != nil || a != "only_me" {
		t.Fatalf("legacy audience %q, %v", a, err)
	}
	f.approve(t, p)
	requireStatus(t, f.call("GET", "/v2/community/posts/"+p.ID, 1, nil), 200)
	w := f.call("GET", "/v2/community/feed", 1, nil)
	requireStatus(t, w, 200)
	if !strings.Contains(w.Body.String(), p.ID) {
		t.Fatal("approved post missing from feed")
	}
}
func TestPostgresCommunityFriendsFollowAndBlock(t *testing.T) {
	f := setup(t)
	f.friend(t)
	p := f.post(t, "friends")
	path := "/v2/community/posts/" + p.ID
	requireStatus(t, f.call("GET", path, 1, nil), 200)
	requireStatus(t, f.call("GET", path, 2, nil), 404)
	requireStatus(t, f.call("PUT", "/v2/community/follows", 2, map[string]string{"kind": "person", "target": f.people[0]}), 204)
	requireStatus(t, f.call("GET", path, 2, nil), 404)
	f.exec(t, `UPDATE friend_requests SET state='blocked' WHERE requester_id=$1 AND addressee_id=$2`, f.people[0], f.people[1])
	requireStatus(t, f.call("GET", path, 1, nil), 404)
}
func TestPostgresCommunityRetryAndRevisionRace(t *testing.T) {
	f := setup(t)
	input := PostInput{LogicalID: uuid(), Body: "Một buổi cà phê", Audience: "public"}
	var wg sync.WaitGroup
	results := make(chan *httptest.ResponseRecorder, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); results <- f.call("POST", "/v2/community/posts", 0, input) }()
	}
	wg.Wait()
	close(results)
	id := ""
	for w := range results {
		if w.Code != 200 && w.Code != 201 {
			t.Fatal(w.Code, w.Body.String())
		}
		var p Post
		_ = json.Unmarshal(w.Body.Bytes(), &p)
		if id != "" && id != p.ID {
			t.Fatal("duplicate post")
		}
		id = p.ID
	}
	input.Body = "nội dung khác"
	requireStatus(t, f.call("POST", "/v2/community/posts", 0, input), 409)
	input.Revision = 1
	input.Audience = "public"
	requireStatus(t, f.call("PUT", "/v2/community/posts/"+id, 0, input), 200)
	requireStatus(t, f.call("POST", "/v2/community/posts/"+id+"/review", 3, map[string]any{"revision": 1, "approve": true, "reason": "Duyệt phiên bản cũ"}), 409)
	requireStatus(t, f.call("GET", "/v2/community/posts/"+id, 1, nil), 404)
}
func TestPostgresCommunityCommentsAndLikesShareWall(t *testing.T) {
	f := setup(t)
	f.friend(t)
	p := f.post(t, "friends")
	path := "/v2/community/posts/" + p.ID
	input := CommentInput{LogicalID: uuid(), Body: "Hẹn chuyến sau nhé"}
	w := f.call("POST", path+"/comments", 1, input)
	requireStatus(t, w, 201)
	requireStatus(t, f.call("POST", path+"/comments", 1, input), 200)
	for i := 0; i < 3; i++ {
		requireStatus(t, f.call("PUT", path+"/like", 1, nil), 200)
	}
	var c, l int
	err := f.pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM post_comments WHERE post_id=$1),(SELECT count(*) FROM post_reactions WHERE post_id=$1)`, p.ID).Scan(&c, &l)
	if err != nil || c != 1 || l != 1 {
		t.Fatalf("comments=%d likes=%d err=%v", c, l, err)
	}
	requireStatus(t, f.call("GET", path+"/comments", 1, nil), 200)
	requireStatus(t, f.call("POST", path+"/comments", 2, input), 404)
}
func TestPostgresCommunityPublicCommentsWaitForReview(t *testing.T) {
	f := setup(t)
	p := f.post(t, "public")
	f.approve(t, p)
	path := "/v2/community/posts/" + p.ID
	requireStatus(t, f.call("POST", path+"/comments", 1, CommentInput{LogicalID: uuid(), Body: "Rủ nhau đi nhé"}), 201)
	var count int
	_ = f.pool.QueryRow(context.Background(), `SELECT count(*) FROM post_comments WHERE post_id=$1`, p.ID).Scan(&count)
	if count != 0 {
		t.Fatal("undervetted comment escaped")
	}
	w := f.call("GET", path+"/comments", 1, nil)
	requireStatus(t, w, 200)
	if !strings.Contains(w.Body.String(), "Rủ nhau đi nhé") {
		t.Fatal("author cannot see pending comment")
	}
	w = f.call("GET", path+"/comments", 2, nil)
	requireStatus(t, w, 200)
	if strings.Contains(w.Body.String(), "Rủ nhau đi nhé") {
		t.Fatal("pending comment leaked")
	}
}
func TestPostgresCommunityConsentAndHistory(t *testing.T) {
	f := setup(t)
	p := f.post(t, "public")
	f.approve(t, p)
	event := map[string]any{"id": uuid(), "post_id": p.ID, "kind": "view", "dwell_ms": 9000}
	requireStatus(t, f.call("POST", "/v2/community/interactions", 1, event), 204)
	var count int
	_ = f.pool.QueryRow(context.Background(), `SELECT count(*) FROM community_interactions WHERE person_id=$1`, f.people[1]).Scan(&count)
	if count != 0 {
		t.Fatal("tracked without consent")
	}
	requireStatus(t, f.call("PUT", "/v2/community/preferences", 1, map[string]bool{"personalized": true}), 200)
	requireStatus(t, f.call("POST", "/v2/community/interactions", 1, event), 204)
	requireStatus(t, f.call("POST", "/v2/community/interactions", 1, event), 204)
	_ = f.pool.QueryRow(context.Background(), `SELECT count(*) FROM community_interactions WHERE person_id=$1`, f.people[1]).Scan(&count)
	if count != 1 {
		t.Fatal("event duplicate or missing")
	}
	requireStatus(t, f.call("DELETE", "/v2/community/history", 1, nil), 204)
	_ = f.pool.QueryRow(context.Background(), `SELECT count(*) FROM community_interactions WHERE person_id=$1`, f.people[1]).Scan(&count)
	if count != 0 {
		t.Fatal("history retained")
	}
}
func TestPostgresCommunityStreamCrossReplicaAndRevocation(t *testing.T) {
	f := setup(t)
	p := f.post(t, "public")
	f.approve(t, p)
	reader := New(f.pool, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go reader.runEvents(ctx)
	server := httptest.NewServer(reader)
	defer server.Close()
	deadline, done := context.WithTimeout(ctx, 8*time.Second)
	defer done()
	conn, _, err := websocket.Dial(deadline, "ws"+strings.TrimPrefix(server.URL, "http")+"/v2/community/stream", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	if err = wsjson.Write(deadline, conn, map[string]any{"token": f.tokens[1], "posts": []string{p.ID}}); err != nil {
		t.Fatal(err)
	}
	var msg frame
	if err = wsjson.Read(deadline, conn, &msg); err != nil || msg.Kind != "sync" {
		t.Fatal(msg, err)
	}
	requireStatus(t, f.call("PUT", "/v2/community/posts/"+p.ID+"/like", 2, nil), 200)
	var committed int64
	if err = f.pool.QueryRow(ctx, `SELECT max(id) FROM community_events WHERE post_id=$1`, p.ID).Scan(&committed); err != nil {
		t.Fatal(err)
	}
	for {
		if err = wsjson.Read(deadline, conn, &msg); err != nil {
			t.Fatal(err)
		}
		// Concurrent account/friend changes may coalesce this update into a
		// full sync. Both frames invalidate the reader through this commit.
		if msg.Cursor >= committed && (msg.Kind == "sync" || msg.Kind == "post.changed" && msg.PostID == p.ID) {
			break
		}
	}
	updated := f.call("GET", "/v2/community/posts/"+p.ID, 1, nil)
	requireStatus(t, updated, 200)
	var current Post
	if err = json.Unmarshal(updated.Body.Bytes(), &current); err != nil || current.Likes != 1 {
		t.Fatalf("invalidation did not refresh the committed like: %+v, %v", current, err)
	}
	f.exec(t, `UPDATE account_sessions SET revoked_at=clock_timestamp() WHERE person_id=$1`, f.people[1])
	for i := 0; i < 10; i++ {
		err = wsjson.Read(deadline, conn, &msg)
		if err != nil {
			break
		}
	}
	// Canceling CloseRead may close the transport before the close frame is
	// flushed. A peer EOF is valid; a timeout caused by our own deadline is not.
	if err == nil || deadline.Err() != nil || (websocket.CloseStatus(err) != websocket.StatusPolicyViolation && !errors.Is(err, io.EOF)) {
		t.Fatal("revoked session must be closed by the server, not the test deadline:", err)
	}
}

func TestPostgresCommunityRelayStartupCannotSkipConnectedReader(t *testing.T) {
	f := setup(t)
	p := f.post(t, "public")
	f.approve(t, p)
	reader := New(f.pool, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server := httptest.NewServer(reader)
	defer server.Close()
	deadline, done := context.WithTimeout(ctx, 8*time.Second)
	defer done()
	conn, _, err := websocket.Dial(deadline, "ws"+strings.TrimPrefix(server.URL, "http")+"/v2/community/stream", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	if err = wsjson.Write(deadline, conn, map[string]any{"token": f.tokens[1], "posts": []string{p.ID}}); err != nil {
		t.Fatal(err)
	}
	var msg frame
	if err = wsjson.Read(deadline, conn, &msg); err != nil || msg.Kind != "sync" {
		t.Fatal(msg, err)
	}
	initial := msg.Cursor
	// Commit after the subscriber's baseline but before the relay initializes.
	// This ordering is intentional: the event must not be mistaken for history.
	requireStatus(t, f.call("PUT", "/v2/community/posts/"+p.ID+"/like", 2, nil), 200)
	var committed int64
	if err = f.pool.QueryRow(ctx, `SELECT max(id) FROM community_events WHERE post_id=$1`, p.ID).Scan(&committed); err != nil || committed <= initial {
		t.Fatal("mutation must follow the subscriber baseline", committed, initial, err)
	}
	// Exercise full-sync coalescing too, as concurrent session changes do in
	// the repository-wide PostgreSQL tier. The session remains valid here.
	f.exec(t, `UPDATE account_sessions SET expires_at=expires_at WHERE person_id=$1`, f.people[1])
	go reader.runEvents(ctx)
	for {
		if err = wsjson.Read(deadline, conn, &msg); err != nil {
			t.Fatal("relay skipped a committed event after the connected reader's baseline:", err)
		}
		if msg.Cursor >= committed && (msg.Kind == "sync" || msg.Kind == "post.changed" && msg.PostID == p.ID) {
			break
		}
	}
	updated := f.call("GET", "/v2/community/posts/"+p.ID, 1, nil)
	requireStatus(t, updated, 200)
	var current Post
	if err = json.Unmarshal(updated.Body.Bytes(), &current); err != nil || current.Likes != 1 {
		t.Fatalf("startup invalidation did not refresh the committed like: %+v, %v", current, err)
	}
}
func TestPostgresCommunityAudienceChangeInvalidatesSnapshot(t *testing.T) {
	f := setup(t)
	p := f.post(t, "public")
	f.approve(t, p)
	// Warm the shared discovery cache and retain it through the visibility change.
	w := f.call("GET", "/v2/community/feed", 1, nil)
	requireStatus(t, w, 200)
	if !strings.Contains(w.Body.String(), p.ID) {
		t.Fatal("public post missing before narrowing")
	}
	f.h.candidateMu.Lock()
	f.h.candidateUntil = time.Now().Add(time.Hour)
	f.h.candidateMu.Unlock()
	requireStatus(t, f.call("PATCH", "/v2/community/posts/"+p.ID+"/audience", 0, map[string]any{"audience": "friends", "revision": 1}), 200)
	requireStatus(t, f.call("GET", "/v2/community/posts/"+p.ID, 1, nil), 404)
	w = f.call("GET", "/v2/community/feed", 1, nil)
	requireStatus(t, w, 200)
	if strings.Contains(w.Body.String(), p.ID) {
		t.Fatal("cached public ranking input leaked a now-private post")
	}
	requireStatus(t, f.call("POST", "/v2/community/posts/"+p.ID+"/review", 3, map[string]any{"approve": true, "revision": 1, "reason": "Duyệt bản cũ"}), 409)
}

func TestPostgresCommunityStableRankingReusesReaderSnapshot(t *testing.T) {
	f := setup(t)
	p := f.post(t, "public")
	f.approve(t, p)
	requireStatus(t, f.call("GET", "/v2/community/feed", 1, nil), 200)
	f.h.candidateMu.Lock()
	f.h.candidateUntil = time.Now().Add(time.Hour)
	f.h.candidateMu.Unlock()
	var originalVersion string
	if err := f.pool.QueryRow(context.Background(), `SELECT xmin::text FROM community_feeds WHERE person_id=$1`, f.people[1]).Scan(&originalVersion); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 8; i++ {
		requireStatus(t, f.call("GET", "/v2/community/feed", 1, nil), 200)
	}
	var count int
	var snapshot string
	if err := f.pool.QueryRow(context.Background(), `SELECT count(*),min(id::text) FROM community_feeds WHERE person_id=$1`, f.people[1]).Scan(&count, &snapshot); err != nil || count != 1 {
		t.Fatalf("unchanged ranking copied on refresh: count=%d err=%v", count, err)
	}
	var currentVersion string
	if err := f.pool.QueryRow(context.Background(), `SELECT xmin::text FROM community_feeds WHERE id=$1`, snapshot).Scan(&currentVersion); err != nil || currentVersion != originalVersion {
		t.Fatalf("unchanged refresh rewrote the snapshot: %q -> %q, %v", originalVersion, currentVersion, err)
	}
	f.exec(t, `UPDATE community_feeds SET expires_at=clock_timestamp()-interval '1 second' WHERE id=$1`, snapshot)
	requireStatus(t, f.call("GET", "/v2/community/feed?after="+snapshot+":0", 1, nil), 409)
	requireStatus(t, f.call("GET", "/v2/community/feed", 1, nil), 200)
	requireStatus(t, f.call("GET", "/v2/community/feed?after="+snapshot+":0", 1, nil), 200)
	requireStatus(t, f.call("GET", "/v2/community/feed?after="+snapshot+":0", 2, nil), 409)
	requireStatus(t, f.call("DELETE", "/v2/community/history", 1, nil), 204)
	requireStatus(t, f.call("GET", "/v2/community/feed?after="+snapshot+":0", 1, nil), 409)
	if err := f.pool.QueryRow(context.Background(), `SELECT count(*) FROM community_feeds WHERE person_id=$1`, f.people[1]).Scan(&count); err != nil || count != 0 {
		t.Fatalf("history deletion retained ranking: count=%d err=%v", count, err)
	}
}

func TestPostgresCommunityCountersFollowCanonicalWritesAndRollback(t *testing.T) {
	f := setup(t)
	f.friend(t)
	p := f.post(t, "friends")
	path := "/v2/community/posts/" + p.ID
	assertCounts := func(likes, comments int) {
		t.Helper()
		w := f.call("GET", path, 1, nil)
		requireStatus(t, w, 200)
		var got Post
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil || got.Likes != likes || got.Comments != comments {
			t.Fatalf("counter mismatch: likes=%d comments=%d err=%v", got.Likes, got.Comments, err)
		}
	}
	requireStatus(t, f.call("PUT", path+"/like", 1, nil), 200)
	requireStatus(t, f.call("PUT", path+"/like", 1, nil), 200)
	assertCounts(1, 0)
	// The legacy wall's canonical writer also goes through these DB triggers.
	f.exec(t, `INSERT INTO post_reactions(id,post_id,person_id,kind) VALUES($1,$2,$3,'heart')`, uuid(), p.ID, f.people[0])
	assertCounts(2, 0)
	f.exec(t, `DELETE FROM post_reactions WHERE post_id=$1 AND person_id=$2`, p.ID, f.people[0])
	assertCounts(1, 0)
	tx, err := f.pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(context.Background(), `DELETE FROM post_reactions WHERE post_id=$1`, p.ID); err != nil {
		t.Fatal(err)
	}
	_ = tx.Rollback(context.Background())
	assertCounts(1, 0)
	w := f.call("POST", path+"/comments", 1, CommentInput{LogicalID: uuid(), Body: "Một chuyến đi tổng hợp"})
	requireStatus(t, w, 201)
	var c Comment
	if err = json.Unmarshal(w.Body.Bytes(), &c); err != nil {
		t.Fatal(err)
	}
	assertCounts(1, 1)
	requireStatus(t, f.call("DELETE", path+"/comments/"+c.ID, 1, nil), 204)
	requireStatus(t, f.call("DELETE", path+"/like", 1, nil), 200)
	assertCounts(0, 0)
}

// An empty shared ranking must store as an empty array. The feed snapshot
// keeps its ids in community_feeds.post_ids, which is NOT NULL, and pgx writes
// a nil slice as NULL: the copy the ranking used to make of an empty ranking
// was nil, so every reader of an empty community got 503
// «community_unavailable» (sqlstate 23502 on the QA stack, 2026-10-02). This
// holds the insert to the copy the handler now makes (banSaoXepHang), and
// shows the column still refuses what the old copy produced.
func TestPostgresCommunityEmptyRankingStoresEmptyArray(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	luu := `INSERT INTO community_feeds(id,person_id,mode,post_ids,rank_key) VALUES($1,$2,'for_you',$3,$4)`
	if _, err := tx.Exec(ctx, luu, uuid(), f.people[0], banSaoXepHang(nil), "rong-"+uuid()); err != nil {
		t.Fatalf("the copy of an empty ranking does not store: %v", err)
	}
	var n int
	if err := tx.QueryRow(ctx, `SELECT cardinality(post_ids) FROM community_feeds WHERE person_id=$1 AND rank_key LIKE 'rong-%'`, f.people[0]).Scan(&n); err != nil || n != 0 {
		t.Fatalf("stored ids: %d, %v; want an empty array", n, err)
	}
	if _, err := tx.Exec(ctx, "SAVEPOINT tho_nil"); err != nil {
		t.Fatal(err)
	}
	_, err = tx.Exec(ctx, luu, uuid(), f.people[0], append([]string(nil), []string{}...), "nil-"+uuid())
	var pg *pgconn.PgError
	if !errors.As(err, &pg) || pg.Code != "23502" {
		t.Fatalf("a nil slice: %v; want sqlstate 23502 (the column is NOT NULL)", err)
	}
	if _, err := tx.Exec(ctx, "ROLLBACK TO SAVEPOINT tho_nil"); err != nil {
		t.Fatal(err)
	}
}

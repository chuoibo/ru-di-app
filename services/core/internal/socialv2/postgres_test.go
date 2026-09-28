//go:build postgres

package socialv2

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"mobile/services/core/internal/auth"
	"mobile/services/core/internal/repo"
	"mobile/services/core/internal/testdb"
)

type socialWorld struct {
	pool   *pgxpool.Pool
	people [3]string
	tokens [3]string
	post   string
}

func socialID(t *testing.T) string {
	t.Helper()
	id, err := repo.NewUUID()
	if err != nil {
		t.Fatal(err)
	}
	return id
}
func socialExec(t *testing.T, pool *pgxpool.Pool, query string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), query, args...); err != nil {
		t.Fatal(err)
	}
}

func newSocialWorld(t *testing.T) socialWorld {
	t.Helper()
	ctx := context.Background()
	base := testdb.Pool(t)
	schema := "social_v2_" + strings.ReplaceAll(socialID(t), "-", "")
	socialExec(t, base, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize())
	t.Cleanup(func() { socialExec(t, base, "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE") })
	cfg := base.Config().Copy()
	cfg.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	for _, table := range []string{"people", "contexts", "memberships", "friend_requests", "account_sessions", "posts", "post_comments", "post_reactions"} {
		socialExec(t, pool, "CREATE TABLE "+table+" (LIKE public."+table+" INCLUDING ALL)")
	}
	if err := Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	w := socialWorld{pool: pool}
	for i := range w.people {
		w.people[i] = socialID(t)
		w.tokens[i] = "synthetic-social-" + socialID(t)
		socialExec(t, pool, `INSERT INTO people(id,display_name) VALUES($1,$2)`, w.people[i], "Synthetic person")
		socialExec(t, pool, `INSERT INTO account_sessions(id,person_id,token_digest,issued_via,expires_at) VALUES($1,$2,$3,'genesis',now()+interval '1 day')`, socialID(t), w.people[i], auth.TokenDigest(w.tokens[i]))
	}
	socialExec(t, pool, `INSERT INTO friend_requests(id,requester_id,addressee_id,state,decided_by_id,decided_at) VALUES($1,$2,$3,'accepted',$3,now())`, socialID(t), w.people[0], w.people[1])
	w.post = socialID(t)
	socialExec(t, pool, `INSERT INTO posts(id,author_id,audience,body,created_at) VALUES($1,$2,'friends','Synthetic outing',now())`, w.post, w.people[0])
	return w
}

func (w socialWorld) request(t *testing.T, who int, method, path string, body string) (int, map[string]any) {
	return w.requestWithKey(t, who, method, path, body, "")
}

func (w socialWorld) requestWithKey(t *testing.T, who int, method, path string, body, key string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+w.tokens[who])
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	res := httptest.NewRecorder()
	New(w.pool, "prod").ServeHTTP(res, req)
	var value map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &value); err != nil {
		t.Fatalf("response %d %s: %v", res.Code, res.Body.String(), err)
	}
	return res.Code, value
}

func TestPostgresCommentAndRepostRetryReturnSameRow(t *testing.T) {
	w := newSocialWorld(t)
	key := socialID(t)
	path := "/social/v2/posts/" + w.post + "/comments"
	firstStatus, first := w.requestWithKey(t, 1, http.MethodPost, path, `{"body":"Once"}`, key)
	secondStatus, second := w.requestWithKey(t, 1, http.MethodPost, path, `{"body":"Once"}`, key)
	if firstStatus != 201 || secondStatus != 201 || first["id"] != second["id"] {
		t.Fatalf("comment retry duplicated: %d %+v; %d %+v", firstStatus, first, secondStatus, second)
	}
	badStatus, bad := w.requestWithKey(t, 1, http.MethodPost, path, `{"body":"Different"}`, key)
	if badStatus != 422 || bad["code"] != "idempotency_key_reuse" {
		t.Fatalf("key reuse: %d %+v", badStatus, bad)
	}
	repostPath := "/social/v2/posts/" + w.post + "/repost"
	repostKey := socialID(t)
	firstStatus, first = w.requestWithKey(t, 1, http.MethodPost, repostPath, `{"audience":"friends"}`, repostKey)
	secondStatus, second = w.requestWithKey(t, 1, http.MethodPost, repostPath, `{"audience":"friends"}`, repostKey)
	if firstStatus != 201 || secondStatus != 201 || first["id"] != second["id"] {
		t.Fatalf("repost retry duplicated: %d %+v; %d %+v", firstStatus, first, secondStatus, second)
	}
}

func TestPostgresWallVisibilityPagingAndChangeCursor(t *testing.T) {
	w := newSocialWorld(t)
	status, friend := w.request(t, 1, http.MethodGet, "/social/v2/people/"+w.people[0]+"/posts?limit=1", "")
	if status != 200 || len(friend["posts"].([]any)) != 1 {
		t.Fatalf("friend page: %d %+v", status, friend)
	}
	status, stranger := w.request(t, 2, http.MethodGet, "/social/v2/people/"+w.people[0]+"/posts?limit=1", "")
	if status != 200 || len(stranger["posts"].([]any)) != 0 {
		t.Fatalf("stranger page: %d %+v", status, stranger)
	}
	status, initial := w.request(t, 1, http.MethodGet, "/social/v2/people/"+w.people[0]+"/changes", "")
	if status != 200 || initial["next_cursor"] == nil {
		t.Fatalf("initial feed: %d %+v", status, initial)
	}
	cursor := initial["next_cursor"].(string)
	status, _ = w.request(t, 1, http.MethodPut, "/social/v2/posts/"+w.post+"/like", "")
	if status != 200 {
		t.Fatal(status)
	}
	status, detail := w.request(t, 1, http.MethodGet, "/social/v2/posts/"+w.post, "")
	if status != 200 || detail["liked"] != true || detail["like_count"] != float64(1) {
		t.Fatalf("detail after like: %d %+v", status, detail)
	}
	status, changed := w.request(t, 1, http.MethodGet, "/social/v2/people/"+w.people[0]+"/changes?after="+cursor, "")
	if status != 200 || changed["next_cursor"] == cursor {
		t.Fatalf("missing friend change: %d %+v", status, changed)
	}
}

func TestPostgresLegacyLikeAndSocialButtonShareOneReaction(t *testing.T) {
	w := newSocialWorld(t)
	tx, err := w.pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = (repo.Repository{Q: tx}).AddPostReaction(context.Background(), w.post, w.people[1], "like", time.Now()); err != nil {
		_ = tx.Rollback(context.Background())
		t.Fatal(err)
	}
	if err = tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	socialExec(t, w.pool, `INSERT INTO post_reactions(id,post_id,person_id,kind) VALUES($1,$2,$3,'heart')`, socialID(t), w.post, w.people[1])
	path := "/social/v2/posts/" + w.post
	status, detail := w.request(t, 1, http.MethodGet, path, "")
	if status != 200 || detail["liked"] != true || detail["like_count"] != float64(1) {
		t.Fatalf("legacy like invisible to new UI: %d %+v", status, detail)
	}
	status, removed := w.request(t, 1, http.MethodDelete, path+"/like", "")
	if status != 200 || removed["liked"] != false || removed["like_count"] != float64(0) {
		t.Fatalf("new unlike did not remove legacy like: %d %+v", status, removed)
	}
	status, added := w.request(t, 1, http.MethodPut, path+"/like", "")
	if status != 200 || added["liked"] != true || added["like_count"] != float64(1) {
		t.Fatalf("new like response: %d %+v", status, added)
	}
	legacy, err := (repo.Repository{Q: w.pool}).PostSocialCounts(context.Background(), []string{w.post}, w.people[1])
	if err != nil {
		t.Fatal(err)
	}
	var oldCount int64
	for _, reaction := range legacy.Reactions {
		for _, kind := range reaction.Kinds {
			if reaction.PostID == w.post && kind.Kind == "like" {
				oldCount = kind.Count
			}
		}
	}
	if oldCount != 1 {
		t.Fatalf("old endpoint repository sees %d likes, want 1: %+v", oldCount, legacy)
	}
}

func TestPostgresWallCursorSkipsPrivatePostsWithoutRepeating(t *testing.T) {
	w := newSocialWorld(t)
	privateID, publicID := socialID(t), socialID(t)
	socialExec(t, w.pool, `INSERT INTO posts(id,author_id,audience,body,created_at) VALUES($1,$2,'only_me','Private',now()+interval '1 second')`, privateID, w.people[0])
	socialExec(t, w.pool, `INSERT INTO posts(id,author_id,audience,body,created_at) VALUES($1,$2,'public','Visible',now()+interval '2 seconds')`, publicID, w.people[0])
	status, first := w.request(t, 1, http.MethodGet, "/social/v2/people/"+w.people[0]+"/posts?limit=1", "")
	if status != 200 || first["posts"].([]any)[0].(map[string]any)["id"] != publicID || first["has_more"] != true {
		t.Fatalf("first page: %d %+v", status, first)
	}
	status, second := w.request(t, 1, http.MethodGet, "/social/v2/people/"+w.people[0]+"/posts?limit=1&cursor="+first["next_cursor"].(string), "")
	if status != 200 || len(second["posts"].([]any)) != 1 || second["posts"].([]any)[0].(map[string]any)["id"] != w.post {
		t.Fatalf("second page: %d %+v", status, second)
	}
	status, outsider := w.request(t, 2, http.MethodGet, "/social/v2/people/"+w.people[0]+"/posts?limit=5", "")
	if status != 200 || len(outsider["posts"].([]any)) != 1 || outsider["posts"].([]any)[0].(map[string]any)["id"] != publicID {
		t.Fatalf("outsider page: %d %+v", status, outsider)
	}
}

func TestPostgresOneLevelRepliesLikesAndRepostACL(t *testing.T) {
	w := newSocialWorld(t)
	status, parent := w.request(t, 1, http.MethodPost, "/social/v2/posts/"+w.post+"/comments", `{"body":"First"}`)
	if status != 201 {
		t.Fatalf("parent: %d %+v", status, parent)
	}
	parentID := parent["id"].(string)
	status, child := w.request(t, 0, http.MethodPost, "/social/v2/posts/"+w.post+"/comments", `{"body":"Reply","parent_id":"`+parentID+`"}`)
	if status != 201 || child["parent_id"] != parentID {
		t.Fatalf("child: %d %+v", status, child)
	}
	status, nested := w.request(t, 1, http.MethodPost, "/social/v2/posts/"+w.post+"/comments", `{"body":"Too deep","parent_id":"`+child["id"].(string)+`"}`)
	if status != 422 || nested["code"] != "reply_depth_exceeded" {
		t.Fatalf("nested: %d %+v", status, nested)
	}
	status, liked := w.request(t, 0, http.MethodPut, "/social/v2/comments/"+parentID+"/like", "")
	if status != 200 || liked["like_count"] != float64(1) {
		t.Fatalf("comment like: %d %+v", status, liked)
	}
	status, again := w.request(t, 0, http.MethodPut, "/social/v2/comments/"+parentID+"/like", "")
	if status != 200 || again["like_count"] != float64(1) {
		t.Fatalf("double like: %d %+v", status, again)
	}
	status, denied := w.request(t, 2, http.MethodPut, "/social/v2/comments/"+parentID+"/like", "")
	if status != 404 || denied["code"] != "post_not_found" {
		t.Fatalf("stranger like: %d %+v", status, denied)
	}
	status, repost := w.request(t, 1, http.MethodPost, "/social/v2/posts/"+w.post+"/repost", `{"audience":"public"}`)
	if status != 201 {
		t.Fatalf("repost: %d %+v", status, repost)
	}
	status, wall := w.request(t, 2, http.MethodGet, "/social/v2/people/"+w.people[1]+"/posts?limit=20", "")
	if status != 200 {
		t.Fatalf("wall: %d %+v", status, wall)
	}
	posts := wall["posts"].([]any)
	if len(posts) != 1 || posts[0].(map[string]any)["origin"] != nil {
		t.Fatalf("repost leaked friend-only origin: %+v", posts)
	}
}

func TestPostgresSecondUserReceivesWallChangeWhileThirdCannot(t *testing.T) {
	w := newSocialWorld(t)
	status, initial := w.request(t, 1, http.MethodGet, "/social/v2/people/"+w.people[0]+"/changes", "")
	if status != 200 {
		t.Fatal(status, initial)
	}
	cursor := initial["next_cursor"].(string)
	status, outsider := w.request(t, 2, http.MethodGet, "/social/v2/people/"+w.people[0]+"/changes", "")
	if status != 200 || len(outsider["events"].([]any)) != 0 {
		t.Fatalf("outsider observed private activity: %d %+v", status, outsider)
	}
	start := time.Now()
	result := make(chan map[string]any, 1)
	go func() {
		_, page := w.request(t, 1, http.MethodGet, "/social/v2/people/"+w.people[0]+"/changes?after="+cursor+"&wait=5", "")
		result <- page
	}()
	time.Sleep(200 * time.Millisecond)
	status, _ = w.request(t, 0, http.MethodPut, "/social/v2/posts/"+w.post+"/like", "")
	if status != 200 {
		t.Fatal(status)
	}
	var nextCursor string
	select {
	case page := <-result:
		if len(page["events"].([]any)) != 1 || time.Since(start) > 3*time.Second {
			t.Fatalf("late or absent event after %s: %+v", time.Since(start), page)
		}
		nextCursor = page["next_cursor"].(string)
	case <-time.After(4 * time.Second):
		t.Fatal("second user did not receive live wall change")
	}

	newPost := socialID(t)
	socialExec(t, w.pool, `INSERT INTO posts(id,author_id,audience,body,created_at) VALUES($1,$2,'friends','Second synthetic outing',now())`, newPost, w.people[0])
	status, changes := w.request(t, 1, http.MethodGet, "/social/v2/people/"+w.people[0]+"/changes?after="+nextCursor, "")
	if status != 200 || len(changes["events"].([]any)) != 1 {
		t.Fatalf("friend missed new post: %d %+v", status, changes)
	}
	event := changes["events"].([]any)[0].(map[string]any)
	if event["kind"] != "post" || event["post_id"] != newPost {
		t.Fatalf("wrong new-post event: %+v", event)
	}
	status, wall := w.request(t, 1, http.MethodGet, "/social/v2/people/"+w.people[0]+"/posts", "")
	if status != 200 || wall["posts"].([]any)[0].(map[string]any)["id"] != newPost {
		t.Fatalf("friend wall did not advance: %d %+v", status, wall)
	}
	status, privateWall := w.request(t, 2, http.MethodGet, "/social/v2/people/"+w.people[0]+"/posts", "")
	if status != 200 || len(privateWall["posts"].([]any)) != 0 {
		t.Fatalf("outsider saw friend-only new post: %d %+v", status, privateWall)
	}
}

func TestPostgresManyFriendsReceiveOnePostAndReconnectFromCursor(t *testing.T) {
	w := newSocialWorld(t)
	const readers = 8
	tokens := make([]string, readers)
	tokens[0] = w.tokens[1]
	for i := 1; i < readers; i++ {
		person := socialID(t)
		tokens[i] = "synthetic-social-" + socialID(t)
		socialExec(t, w.pool, `INSERT INTO people(id,display_name) VALUES($1,'Synthetic reader')`, person)
		socialExec(t, w.pool, `INSERT INTO account_sessions(id,person_id,token_digest,issued_via,expires_at) VALUES($1,$2,$3,'genesis',now()+interval '1 day')`, socialID(t), person, auth.TokenDigest(tokens[i]))
		socialExec(t, w.pool, `INSERT INTO friend_requests(id,requester_id,addressee_id,state,decided_by_id,decided_at) VALUES($1,$2,$3,'accepted',$3,now())`, socialID(t), w.people[0], person)
	}
	h := New(w.pool, "prod")
	runCtx, stop := context.WithCancel(context.Background())
	defer stop()
	go h.Run(runCtx)
	do := func(token, path string) (int, map[string]any) {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			return rec.Code, map[string]any{"decode_error": err.Error()}
		}
		return rec.Code, body
	}
	path := "/social/v2/people/" + w.people[0] + "/changes"
	status, initial := do(tokens[0], path)
	if status != 200 || initial["next_cursor"] == nil {
		t.Fatalf("initial cursor: %d %+v", status, initial)
	}
	cursor := initial["next_cursor"].(string)
	type result struct {
		status int
		body   map[string]any
	}
	received := make(chan result, readers)
	for _, token := range tokens {
		go func(token string) {
			status, body := do(token, path+"?after="+cursor+"&wait=6")
			received <- result{status, body}
		}(token)
	}
	time.Sleep(250 * time.Millisecond)
	newPost := socialID(t)
	start := time.Now()
	socialExec(t, w.pool, `INSERT INTO posts(id,author_id,audience,body,created_at) VALUES($1,$2,'friends','Shared synthetic outing',now())`, newPost, w.people[0])
	var nextCursor string
	for i := 0; i < readers; i++ {
		select {
		case got := <-received:
			events, ok := got.body["events"].([]any)
			if got.status != 200 || !ok || len(events) != 1 || events[0].(map[string]any)["post_id"] != newPost {
				t.Fatalf("reader %d missed or duplicated update: %d %+v", i, got.status, got.body)
			}
			nextCursor = got.body["next_cursor"].(string)
		case <-time.After(5 * time.Second):
			t.Fatalf("reader %d did not receive update", i)
		}
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("eight readers updated after %s, want under three seconds", elapsed)
	}
	status, outsider := do(w.tokens[2], path)
	if status != 200 || len(outsider["events"].([]any)) != 0 {
		t.Fatalf("outsider saw private activity: %d %+v", status, outsider)
	}
	status, empty := do(tokens[0], path+"?after="+nextCursor)
	if status != 200 || len(empty["events"].([]any)) != 0 {
		t.Fatalf("reconnect replayed consumed event: %d %+v", status, empty)
	}
	status, _ = w.request(t, 1, http.MethodPost, "/social/v2/posts/"+newPost+"/comments", `{"body":"Seen after reconnect"}`)
	if status != 201 {
		t.Fatal("comment after reconnect", status)
	}
	status, continued := do(tokens[0], path+"?after="+nextCursor)
	if status != 200 || len(continued["events"].([]any)) != 1 || continued["events"].([]any)[0].(map[string]any)["kind"] != "comment" {
		t.Fatalf("reconnect lost the next event: %d %+v", status, continued)
	}
}

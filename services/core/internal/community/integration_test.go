package community

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/motluot"
	book "mobile/services/core/internal/domain/diary"
	"mobile/services/core/internal/media/storage"
)

func (f world) uploadImage(t *testing.T, actor int) string {
	t.Helper()
	var b bytes.Buffer
	im := image.NewRGBA(image.Rect(0, 0, 20, 20))
	im.Set(1, 1, color.RGBA{R: 100, A: 255})
	if err := png.Encode(&b, im); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("POST", "/v2/community/media", &b)
	r.Header.Set("Content-Type", "image/png")
	r.Header.Set("Authorization", "Bearer "+f.tokens[actor])
	w := httptest.NewRecorder()
	f.h.ServeHTTP(w, r)
	requireStatus(t, w, 201)
	var m Media
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	return m.ID
}

func TestPostgresCommunityImageBytesFollowLiveRights(t *testing.T) {
	f := setup(t)
	t.Setenv("MOBILE_MEDIA_ROOT", t.TempDir())
	f.friend(t)
	mid := f.uploadImage(t, 0)
	path := "/v2/community/media/" + mid
	requireStatus(t, f.call("GET", path, 1, nil), 404)
	w := f.call("POST", "/v2/community/posts", 0, PostInput{LogicalID: uuid(), Body: "Một ngày đi dạo", Audience: "friends", MediaIDs: []string{mid}})
	requireStatus(t, w, 201)
	w = f.call("GET", path, 1, nil)
	requireStatus(t, w, 200)
	if w.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatal("media can be cached after rights revocation")
	}
	requireStatus(t, f.call("GET", path, 2, nil), 404)
	f.exec(t, `UPDATE friend_requests SET state='blocked' WHERE requester_id=$1 AND addressee_id=$2`, f.people[0], f.people[1])
	requireStatus(t, f.call("GET", path, 1, nil), 404)
}

func TestPostgresCommunityCommentImageCannotBypassBlock(t *testing.T) {
	f := setup(t)
	t.Setenv("MOBILE_MEDIA_ROOT", t.TempDir())
	p := f.post(t, "public")
	f.approve(t, p)
	mid := f.uploadImage(t, 1)
	w := f.call("POST", "/v2/community/posts/"+p.ID+"/comments", 1, CommentInput{LogicalID: uuid(), Body: "Ảnh chuyến đi", MediaID: &mid})
	requireStatus(t, w, 201)
	var result struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &result)
	tx, err := f.pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	_, err = tx.Exec(context.Background(), `UPDATE community_comment_drafts SET status='approved' WHERE id=$1`, result.ID)
	if err == nil {
		err = publishComment(context.Background(), tx, result.ID)
	}
	if err == nil {
		err = tx.Commit(context.Background())
	}
	if err != nil {
		t.Fatal(err)
	}
	path := "/v2/community/media/" + mid
	requireStatus(t, f.call("GET", path, 2, nil), 200)
	f.exec(t, `INSERT INTO friend_requests(id,requester_id,addressee_id,state,decided_at) VALUES($1,$2,$3,'blocked',clock_timestamp())`, uuid(), f.people[1], f.people[2])
	requireStatus(t, f.call("GET", path, 2, nil), 404)
	w = f.call("GET", "/v2/community/posts/"+p.ID+"/comments", 2, nil)
	requireStatus(t, w, 200)
	if strings.Contains(w.Body.String(), "Ảnh chuyến đi") {
		t.Fatal("blocked comment leaked")
	}
}

func TestPostgresCommunityInteractionsDeduplicateConcurrently(t *testing.T) {
	f := setup(t)
	p := f.post(t, "public")
	f.approve(t, p)
	requireStatus(t, f.call("PUT", "/v2/community/preferences", 1, map[string]bool{"personalized": true}), 200)
	var wg sync.WaitGroup
	responses := make(chan *httptest.ResponseRecorder, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			responses <- f.call("POST", "/v2/community/interactions", 1, map[string]any{"id": uuid(), "post_id": p.ID, "kind": "view", "dwell_ms": 9000})
		}()
	}
	wg.Wait()
	close(responses)
	for w := range responses {
		requireStatus(t, w, 204)
	}
	var n int
	if err := f.pool.QueryRow(context.Background(), `SELECT count(*) FROM community_interactions WHERE person_id=$1`, f.people[1]).Scan(&n); err != nil || n != 1 {
		t.Fatalf("events=%d err=%v", n, err)
	}
}

func TestPostgresCommunityHiddenRemovedFromOldSnapshot(t *testing.T) {
	f := setup(t)
	p := f.post(t, "public")
	f.approve(t, p)
	snapshot := uuid()
	f.exec(t, `INSERT INTO community_feeds(id,person_id,mode,post_ids) VALUES($1,$2,'for_you',$3)`, snapshot, f.people[1], []string{p.ID})
	f.exec(t, `INSERT INTO community_feedback VALUES($1,$2,'hidden')`, f.people[1], p.ID)
	w := f.call("GET", "/v2/community/feed?after="+snapshot+":0", 1, nil)
	requireStatus(t, w, 200)
	if strings.Contains(w.Body.String(), p.ID) {
		t.Fatal("hidden post remained in snapshot")
	}
}

// statusOf is the post's status as its author reads it.
func (f world) statusOf(t *testing.T, p Post) string {
	t.Helper()
	w := f.call("GET", "/v2/community/posts/"+p.ID, 0, nil)
	requireStatus(t, w, 200)
	var out struct {
		Status string `json:"status"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return out.Status
}

// moderate runs the queued job for p once more with the given model.
func (f world) moderate(t *testing.T, p Post, may *motluot.May) {
	t.Helper()
	f.h.ai = may
	f.exec(t, `UPDATE community_jobs SET available_at=clock_timestamp(),lease_until=NULL WHERE post_id=$1`, p.ID)
	f.h.workOne(context.Background())
}

func TestPostgresCommunityInferenceOutageAndUnsafeVerdict(t *testing.T) {
	f := setup(t)
	t.Setenv("MOBILE_MEDIA_ROOT", t.TempDir())
	p := f.post(t, "public")
	f.h.workOne(context.Background())
	requireStatus(t, f.call("GET", "/v2/community/posts/"+p.ID, 1, nil), 404)
	// An answer that is not the reading, and a reading below 900, both
	// leave the post to a human.
	f.moderate(t, p, motluot.Moi(llm.NewStub(llm.Buoc{Text: `{"relevant": true}`}), 1))
	if s := f.statusOf(t, p); s != "pending" {
		t.Fatalf("malformed reading: %s", s)
	}
	f.moderate(t, p, motluot.Moi(llm.NewStub(llm.Buoc{Text: `{"relevant":true,"safe":true,"confidence_milli":899,"reason":"ok"}`}), 1))
	if s := f.statusOf(t, p); s != "review" {
		t.Fatalf("unsure reading: %s", s)
	}
	requireStatus(t, f.call("GET", "/v2/community/posts/"+p.ID, 1, nil), 404)
	q := f.post(t, "public")
	stub := llm.NewStub(llm.Buoc{Text: `{"relevant":false,"safe":true,"confidence_milli":990,"reason":"not about outings"}`})
	f.moderate(t, q, motluot.Moi(stub, 1))
	if s := f.statusOf(t, q); s != "rejected" {
		t.Fatalf("off-topic reading: %s", s)
	}
	req := string(stub.YeuCau()[0])
	if !strings.Contains(req, "Một ngày đi bộ ngắm hồ cùng hội bạn") || !strings.Contains(req, `\"kind\":\"POST\"`) || strings.Contains(req, f.people[0]) {
		t.Fatalf("the model read the wrong thing: %s", req)
	}
	requireStatus(t, f.call("GET", "/v2/community/posts/"+q.ID, 1, nil), 404)
}

func TestPostgresCommunityReadingSeesEveryImageOrGoesToReview(t *testing.T) {
	f := setup(t)
	t.Setenv("MOBILE_MEDIA_ROOT", t.TempDir())
	confident := `{"relevant":true,"safe":true,"confidence_milli":980,"reason":"ok"}`
	mid := f.uploadImage(t, 0)
	w := f.call("POST", "/v2/community/posts", 0, PostInput{LogicalID: uuid(), Body: "Hồ buổi sáng", Audience: "public", Topics: []string{"Đi bộ"}, MediaIDs: []string{mid}})
	requireStatus(t, w, 201)
	var withImage Post
	_ = json.Unmarshal(w.Body.Bytes(), &withImage)
	stub := llm.NewStub(llm.Buoc{Text: confident})
	f.moderate(t, withImage, motluot.Moi(stub, 1))
	if s := f.statusOf(t, withImage); s != "approved" {
		t.Fatalf("image read and confident: %s", s)
	}
	// Images go shrunk, as JPEG, whatever they were stored as.
	if req := string(stub.YeuCau()[0]); !strings.Contains(req, "image/jpeg") || !strings.Contains(req, "inlineData") {
		t.Fatal("the image never reached the model")
	}
	// A video without its review cut (processed before the cut existed) is
	// never sent, so no reading can vouch for it.
	f.exec(t, `UPDATE community_media SET content_type='video/mp4' WHERE id=$1`, mid)
	w = f.call("POST", "/v2/community/posts", 0, PostInput{LogicalID: uuid(), Body: "Video ngắm hồ", Audience: "public", Topics: []string{"Đi bộ"}, MediaIDs: []string{mid}})
	requireStatus(t, w, 201)
	var withVideo Post
	_ = json.Unmarshal(w.Body.Bytes(), &withVideo)
	stub = llm.NewStub(llm.Buoc{Text: confident})
	f.moderate(t, withVideo, motluot.Moi(stub, 1))
	if s := f.statusOf(t, withVideo); s != "review" {
		t.Fatalf("unseen video: %s", s)
	}
	if strings.Contains(string(stub.YeuCau()[0]), "inlineData") {
		t.Fatal("video bytes reached the model")
	}
	// With its cut, every piece is read, one call each, and a confident
	// reading of all of them publishes the post.
	st, err := storage.New()
	if err != nil {
		t.Fatal(err)
	}
	pieces := []string{}
	for _, b := range []string{"piece one", "piece two"} {
		key, e := storage.NewStorageKey()
		if e == nil {
			e = st.Write(key, []byte(b))
		}
		if e != nil {
			t.Fatal(e)
		}
		pieces = append(pieces, key)
	}
	f.exec(t, `UPDATE community_media SET review_keys=$2 WHERE id=$1`, mid, pieces)
	w = f.call("POST", "/v2/community/posts", 0, PostInput{LogicalID: uuid(), Body: "Video ngắm hồ có đoạn duyệt", Audience: "public", Topics: []string{"Đi bộ"}, MediaIDs: []string{mid}})
	requireStatus(t, w, 201)
	var withCut Post
	_ = json.Unmarshal(w.Body.Bytes(), &withCut)
	stub = llm.NewStub(llm.Buoc{Text: confident}, llm.Buoc{Text: confident})
	f.moderate(t, withCut, motluot.Moi(stub, 1))
	if s := f.statusOf(t, withCut); s != "approved" || stub.SoGoi() != 2 {
		t.Fatalf("video read piece by piece: %s after %d calls", s, stub.SoGoi())
	}
	for i, req := range stub.YeuCau() {
		if !strings.Contains(string(req), "video/mp4") || !strings.Contains(string(req), `\"piece\":`+string(rune('1'+i))) {
			t.Fatalf("piece %d was not sent: %s", i+1, req)
		}
	}
	// Deleting the media queues the cut for the store's sweep with the video.
	f.exec(t, `DELETE FROM community_media WHERE id=$1`, mid)
	var queued int
	if err = f.pool.QueryRow(context.Background(), `SELECT count(*) FROM community_media_gc WHERE storage_key=ANY($1)`, pieces).Scan(&queued); err != nil || queued != 2 {
		t.Fatalf("review pieces left for the sweep: %d %v", queued, err)
	}
}

func TestPostgresCommunityNepDraftsOnlyTheConfirmedExcerpt(t *testing.T) {
	f := setup(t)
	p := f.post(t, "public")
	f.approve(t, p)
	path := "/v2/community/posts/" + p.ID + "/nep"
	in := map[string]any{"confirmed": true, "excerpt": "Một ngày đi bộ ngắm hồ cùng hội bạn", "request": "Viết vui hơn"}
	requireStatus(t, f.call("POST", path, 0, in), 503)
	stub := llm.NewStub(llm.Buoc{Text: `{"draft": "  Một ngày dạo quanh hồ thật vui cùng hội bạn!  "}`})
	f.h.ai = motluot.Moi(stub, 1)
	w := f.call("POST", path, 0, in)
	requireStatus(t, w, 200)
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if out["draft"] != "Một ngày dạo quanh hồ thật vui cùng hội bạn!" || out["ai_generated"] != true {
		t.Fatalf("%v", out)
	}
	if req := string(stub.YeuCau()[0]); !strings.Contains(req, "Viết vui hơn") || strings.Contains(req, f.people[0]) {
		t.Fatalf("wrong source: %s", req)
	}
	in["excerpt"] = "Đoạn khác"
	requireStatus(t, f.call("POST", path, 0, in), 409)
	if stub.SoGoi() != 1 {
		t.Fatal("a changed source reached the model")
	}
}

func TestPostgresCommunityLegacyInsertCannotBypassGuard(t *testing.T) {
	f := setup(t)
	p := f.post(t, "public")
	f.approve(t, p)
	_, err := f.pool.Exec(context.Background(), `INSERT INTO post_comments(id,post_id,author_id,body) VALUES($1,$2,$3,'unchecked')`, uuid(), p.ID, f.people[1])
	if err == nil {
		t.Fatal("legacy writer bypassed public comment guard")
	}
	_, err = f.pool.Exec(context.Background(), `UPDATE posts SET body='unchecked edit' WHERE id=$1`, p.ID)
	if err == nil {
		t.Fatal("legacy writer bypassed post edition guard")
	}
}

func TestPostgresCommunityDiaryCopyRevokesWithSource(t *testing.T) {
	f := setup(t)
	t.Setenv("MOBILE_MEDIA_ROOT", t.TempDir())
	room, outing, did, photo := uuid(), uuid(), uuid(), uuid()
	f.exec(t, `INSERT INTO contexts(id,display_name,kind,created_by_id) VALUES($1,'Synthetic group','group',$2)`, room, f.people[0])
	f.exec(t, `INSERT INTO memberships(id,context_id,person_id,state,role,origin) VALUES($1,$2,$3,'active','admin','named')`, uuid(), room, f.people[0])
	f.exec(t, `INSERT INTO outings(id,context_id,created_by_id,title,starts_on,ends_on,headcount,budget_per_person_vnd) VALUES($1,$2,$3,'Synthetic outing','2026-01-01','2026-01-02',2,0)`, outing, room, f.people[0])
	f.exec(t, `INSERT INTO outing_endings(outing_id,kind,ended_by) VALUES($1,'trip',$2)`, outing, f.people[0])
	doc := book.Compose(book.Source{Title: "Nhật ký chuyến đi mẫu", Kind: "trip", Photos: []book.Photo{{ID: photo, Day: "2026-01-01"}}})
	raw, _ := json.Marshal(doc)
	f.exec(t, `INSERT INTO outing_diaries(id,outing_id,owner_id,audience,document) VALUES($1,$2,$3,'public',$4)`, did, outing, f.people[0], raw)
	st, err := storage.New()
	if err != nil {
		t.Fatal(err)
	}
	key, err := storage.NewStorageKey()
	if err != nil {
		t.Fatal(err)
	}
	if err = st.Write(key, []byte("synthetic-source-bytes")); err != nil {
		t.Fatal(err)
	}
	f.exec(t, `INSERT INTO uploaded_images(id,storage_key,context_id,uploaded_by_id,purpose,content_type,byte_size,width,height) VALUES($1,$2,$3,$4,'group','image/jpeg',22,20,20)`, photo, key, room, f.people[0])
	f.exec(t, `INSERT INTO outing_diary_photos VALUES($1,$2)`, did, photo)
	input := map[string]any{"logical_id": uuid(), "revision": 1, "confirmed": true, "topics": []string{"Du lịch"}}
	path := "/v2/community/diaries/" + did + "/share"
	requireStatus(t, f.call("POST", path, 1, input), 404)
	w := f.call("POST", path, 0, input)
	requireStatus(t, w, 201)
	var id struct {
		ID string `json:"id"`
	}
	if err = json.Unmarshal(w.Body.Bytes(), &id); err != nil {
		t.Fatal(err)
	}
	requireStatus(t, f.call("POST", path, 0, input), 200)
	p := Post{ID: id.ID, Revision: 1}
	f.approve(t, p)
	w = f.call("GET", "/v2/community/posts/"+p.ID, 1, nil)
	requireStatus(t, w, 200)
	if err = json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if len(p.Media) != 1 || p.Media[0].ID == photo || len(p.Diary) == 0 {
		t.Fatal("diary did not copy its owned media")
	}
	mediaPath := "/v2/community/media/" + p.Media[0].ID
	requireStatus(t, f.call("GET", mediaPath, 1, nil), 200)
	f.exec(t, `UPDATE outing_diaries SET audience='private',revision=revision+1 WHERE id=$1`, did)
	requireStatus(t, f.call("GET", "/v2/community/posts/"+p.ID, 1, nil), 404)
	requireStatus(t, f.call("GET", mediaPath, 1, nil), 404)
	requireStatus(t, f.call("POST", "/v2/community/posts/"+p.ID+"/submit", 0, nil), 422)
}

func TestPostgresCommunityKeepsArePrivateAndIdempotent(t *testing.T) {
	f := setup(t)
	p := f.post(t, "public")
	f.approve(t, p)
	in := map[string]any{"logical_id": uuid(), "post_id": p.ID, "body": "Ghi chép riêng mẫu", "ai_generated": true}
	w := f.call("POST", "/v2/community/keeps", 1, in)
	requireStatus(t, w, 201)
	var result struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &result)
	requireStatus(t, f.call("POST", "/v2/community/keeps", 1, in), 200)
	w = f.call("GET", "/v2/community/keeps", 2, nil)
	requireStatus(t, w, 200)
	if strings.Contains(w.Body.String(), "Ghi chép riêng mẫu") {
		t.Fatal("keep leaked")
	}
	requireStatus(t, f.call("DELETE", "/v2/community/keeps/"+result.ID, 2, nil), 404)
	requireStatus(t, f.call("DELETE", "/v2/community/keeps/"+result.ID, 1, nil), 204)
}

func TestPostgresCommunityModeratorPublishesCommentOnce(t *testing.T) {
	f := setup(t)
	p := f.post(t, "public")
	f.approve(t, p)
	w := f.call("POST", "/v2/community/posts/"+p.ID+"/comments", 1, CommentInput{LogicalID: uuid(), Body: "Chuyến đi tổng hợp"})
	requireStatus(t, w, 201)
	var c struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &c)
	path := "/v2/community/comments/" + c.ID + "/review"
	in := map[string]any{"approve": true, "reason": "Bình luận tổng hợp hợp chủ đề"}
	requireStatus(t, f.call("POST", path, 1, in), 403)
	requireStatus(t, f.call("POST", path, 3, in), 200)
	requireStatus(t, f.call("POST", path, 3, in), 409)
	w = f.call("GET", "/v2/community/posts/"+p.ID+"/comments", 2, nil)
	requireStatus(t, w, 200)
	if !strings.Contains(w.Body.String(), "Chuyến đi tổng hợp") {
		t.Fatal("reviewed comment missing")
	}
	requireStatus(t, f.call("GET", "/v2/community/review", 3, nil), 200)
}

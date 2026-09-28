//go:build postgres

package socialv2

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mobile/services/core/internal/community"
)

// These cases hold the ADR-0040 boundary: the profile wall and Cộng đồng share
// one post, one like, one comment and one parent, and a public comment cannot
// reach readers through the wall without passing review.

func (w socialWorld) community(t *testing.T, who int, method, path string, body any) (int, map[string]any) {
	t.Helper()
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Authorization", "Bearer "+w.tokens[who])
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	community.New(w.pool, nil, nil).ServeHTTP(res, req)
	var value map[string]any
	if res.Body.Len() > 0 {
		if err := json.Unmarshal(res.Body.Bytes(), &value); err != nil {
			t.Fatalf("community response %d %s: %v", res.Code, res.Body.String(), err)
		}
	}
	return res.Code, value
}

// publicCommunityPost submits a public post through Cộng đồng; approve decides
// whether the moderator publishes it before the test goes on.
func (w socialWorld) publicCommunityPost(t *testing.T, approve bool) string {
	t.Helper()
	status, post := w.community(t, 0, http.MethodPost, "/v2/community/posts", map[string]any{
		"logical_id": socialID(t), "body": "Một buổi chiều đạp xe quanh hồ", "audience": "public", "topics": []string{"Đi bộ"},
	})
	if status != 201 {
		t.Fatalf("community post: %d %+v", status, post)
	}
	id := post["id"].(string)
	if approve {
		status, reviewed := w.community(t, 3, http.MethodPost, "/v2/community/posts/"+id+"/review", map[string]any{
			"revision": post["revision"], "approve": true, "reason": "Nội dung tổng hợp đúng chủ đề",
		})
		if status != 200 {
			t.Fatalf("approve post: %d %+v", status, reviewed)
		}
	}
	return id
}

func threadBodies(page map[string]any) string {
	raw, _ := json.Marshal(page["comments"])
	return string(raw)
}

func TestPostgresWallLikeIsTheCommunityLike(t *testing.T) {
	w := newSocialWorld(t)
	// A 👍 «Đồng ý» message reaction is not the like button.
	socialExec(t, w.pool, `INSERT INTO post_reactions(id,post_id,person_id,kind) VALUES($1,$2,$3,'like')`, socialID(t), w.post, w.people[1])
	status, liked := w.request(t, 1, http.MethodPut, "/social/v2/posts/"+w.post+"/like", "")
	if status != 200 || liked["liked"] != true || liked["like_count"] != float64(1) {
		t.Fatalf("wall like: %d %+v", status, liked)
	}
	status, seen := w.community(t, 1, http.MethodGet, "/v2/community/posts/"+w.post, nil)
	if status != 200 || seen["likes"] != float64(1) || seen["liked"] != true {
		t.Fatalf("community does not see the wall like: %d %+v", status, seen)
	}
	status, _ = w.community(t, 1, http.MethodDelete, "/v2/community/posts/"+w.post+"/like", nil)
	if status != 200 {
		t.Fatal("community unlike", status)
	}
	status, detail := w.request(t, 1, http.MethodGet, "/social/v2/posts/"+w.post, "")
	if status != 200 || detail["liked"] != false || detail["like_count"] != float64(0) {
		t.Fatalf("wall still counts a like removed in Cộng đồng: %d %+v", status, detail)
	}
}

func TestPostgresWallAndCommunityShareOneCommentParent(t *testing.T) {
	w := newSocialWorld(t)
	status, parent := w.request(t, 1, http.MethodPost, "/social/v2/posts/"+w.post+"/comments", `{"body":"Chụp giúp mình tấm ảnh nhé"}`)
	if status != 201 {
		t.Fatalf("wall comment: %d %+v", status, parent)
	}
	parentID := parent["id"].(string)
	status, reply := w.community(t, 0, http.MethodPost, "/v2/community/posts/"+w.post+"/comments", map[string]any{
		"logical_id": socialID(t), "body": "Có ngay", "parent_id": parentID,
	})
	if status != 201 || reply["status"] != "approved" {
		t.Fatalf("community reply: %d %+v", status, reply)
	}
	status, thread := w.request(t, 1, http.MethodGet, "/social/v2/posts/"+w.post+"/comments", "")
	if status != 200 {
		t.Fatal(status, thread)
	}
	top := thread["comments"].([]any)
	if len(top) != 1 {
		t.Fatalf("reply written in Cộng đồng became a top-level wall comment: %+v", top)
	}
	replies := top[0].(map[string]any)["replies"].([]any)
	if len(replies) != 1 || replies[0].(map[string]any)["id"] != reply["id"] || replies[0].(map[string]any)["parent_id"] != parentID {
		t.Fatalf("wall lost the community parent: %+v", top)
	}
	status, wallReply := w.request(t, 1, http.MethodPost, "/social/v2/posts/"+w.post+"/comments", `{"body":"Cảm ơn","parent_id":"`+parentID+`"}`)
	if status != 201 {
		t.Fatalf("wall reply: %d %+v", status, wallReply)
	}
	status, seen := w.community(t, 1, http.MethodGet, "/v2/community/posts/"+w.post+"/comments", nil)
	if status != 200 {
		t.Fatal(status, seen)
	}
	found := false
	for _, item := range seen["comments"].([]any) {
		c := item.(map[string]any)
		if c["id"] == wallReply["id"] {
			found = c["parent_id"] == parentID
		}
	}
	if !found {
		t.Fatalf("community does not see the wall reply under its parent: %+v", seen["comments"])
	}
}

func TestPostgresPublicWallCommentWaitsForReview(t *testing.T) {
	w := newSocialWorld(t)
	w.moderated = true
	post := w.publicCommunityPost(t, true)
	path := "/social/v2/posts/" + post + "/comments"
	status, draft := w.request(t, 1, http.MethodPost, path, `{"body":"Cho mình đi cùng lần sau"}`)
	if status != 202 || draft["status"] != "pending" {
		t.Fatalf("public wall comment skipped review: %d %+v", status, draft)
	}
	var published int
	if err := w.pool.QueryRow(context.Background(), `SELECT count(*) FROM post_comments WHERE post_id=$1`, post).Scan(&published); err != nil || published != 0 {
		t.Fatalf("pending comment reached post_comments: %d %v", published, err)
	}
	status, stranger := w.request(t, 2, http.MethodGet, path, "")
	if status != 200 || strings.Contains(threadBodies(stranger), "lần sau") || len(stranger["pending"].([]any)) != 0 {
		t.Fatalf("pending comment leaked to a reader: %d %+v", status, stranger)
	}
	status, own := w.request(t, 1, http.MethodGet, path, "")
	if status != 200 || len(own["pending"].([]any)) != 1 {
		t.Fatalf("author cannot see their pending comment: %d %+v", status, own)
	}
	status, reviewed := w.community(t, 3, http.MethodPost, "/v2/community/comments/"+draft["id"].(string)+"/review", map[string]any{"approve": true, "reason": "Bình luận tổng hợp an toàn"})
	if status != 200 {
		t.Fatalf("approve comment: %d %+v", status, reviewed)
	}
	status, after := w.request(t, 2, http.MethodGet, path, "")
	if status != 200 || !strings.Contains(threadBodies(after), "lần sau") {
		t.Fatalf("approved comment missing from the wall: %d %+v", status, after)
	}
	status, reply := w.request(t, 2, http.MethodPost, path, `{"body":"Mình nữa","parent_id":"`+draft["id"].(string)+`"}`)
	if status != 202 || reply["parent_id"] != draft["id"] {
		t.Fatalf("public reply: %d %+v", status, reply)
	}
	status, _ = w.community(t, 3, http.MethodPost, "/v2/community/comments/"+reply["id"].(string)+"/review", map[string]any{"approve": true, "reason": "Bình luận tổng hợp an toàn"})
	if status != 200 {
		t.Fatal("approve reply", status)
	}
	status, nested := w.request(t, 1, http.MethodGet, path, "")
	if status != 200 {
		t.Fatal(status)
	}
	top := nested["comments"].([]any)
	if len(top) != 1 || len(top[0].(map[string]any)["replies"].([]any)) != 1 {
		t.Fatalf("approved reply lost its parent: %+v", top)
	}
}

func TestPostgresReviewedPostRefusesWallCommentWithoutModerator(t *testing.T) {
	w := newSocialWorld(t)
	post := w.publicCommunityPost(t, true)
	status, refused := w.request(t, 1, http.MethodPost, "/social/v2/posts/"+post+"/comments", `{"body":"Không ai duyệt"}`)
	if status != 409 || refused["code"] != "community_unavailable" {
		t.Fatalf("comment on a reviewed post bypassed review while Cộng đồng is off: %d %+v", status, refused)
	}
	var drafts int
	if err := w.pool.QueryRow(context.Background(), `SELECT count(*) FROM community_comment_drafts WHERE post_id=$1`, post).Scan(&drafts); err != nil || drafts != 0 {
		t.Fatalf("refused comment left a draft: %d %v", drafts, err)
	}
}

func TestPostgresPendingCommunityPostStaysOffEveryWallRoute(t *testing.T) {
	w := newSocialWorld(t)
	w.moderated = true
	post := w.publicCommunityPost(t, false)
	for _, call := range []struct{ method, path, body string }{
		{http.MethodGet, "/social/v2/posts/" + post, ""},
		{http.MethodGet, "/social/v2/posts/" + post + "/comments", ""},
		{http.MethodPost, "/social/v2/posts/" + post + "/comments", `{"body":"Chen ngang"}`},
		{http.MethodPut, "/social/v2/posts/" + post + "/like", ""},
		{http.MethodPost, "/social/v2/posts/" + post + "/repost", `{"audience":"friends"}`},
	} {
		status, body := w.request(t, 1, call.method, call.path, call.body)
		if status != 404 || body["code"] != "post_not_found" {
			t.Fatalf("%s %s on a pending post: %d %+v", call.method, call.path, status, body)
		}
	}
	status, wall := w.request(t, 1, http.MethodGet, "/social/v2/people/"+w.people[0]+"/posts?limit=20", "")
	if status != 200 || strings.Contains(threadBodies(map[string]any{"comments": wall["posts"]}), post) {
		t.Fatalf("pending post on a friend's view of the wall: %d %+v", status, wall)
	}
	status, own := w.request(t, 0, http.MethodGet, "/social/v2/posts/"+post, "")
	if status != 200 {
		t.Fatalf("author lost their pending post: %d %+v", status, own)
	}
}

func TestPostgresPublicRepostNeedsReviewAndOriginFollowsFriendship(t *testing.T) {
	w := newSocialWorld(t)
	w.moderated = true
	status, refused := w.request(t, 1, http.MethodPost, "/social/v2/posts/"+w.post+"/repost", `{"audience":"public"}`)
	if status != 422 || refused["code"] != "public_repost_needs_review" {
		t.Fatalf("public repost skipped review: %d %+v", status, refused)
	}
	status, repost := w.request(t, 1, http.MethodPost, "/social/v2/posts/"+w.post+"/repost", `{"audience":"only_me"}`)
	if status != 201 {
		t.Fatalf("private repost: %d %+v", status, repost)
	}
	status, detail := w.request(t, 1, http.MethodGet, "/social/v2/posts/"+repost["id"].(string), "")
	if status != 200 || detail["origin"] == nil {
		t.Fatalf("repost lost a readable origin: %d %+v", status, detail)
	}
	socialExec(t, w.pool, `DELETE FROM friend_requests WHERE (requester_id=$1 AND addressee_id=$2) OR (requester_id=$2 AND addressee_id=$1)`, w.people[0], w.people[1])
	status, detail = w.request(t, 1, http.MethodGet, "/social/v2/posts/"+repost["id"].(string), "")
	if status != 200 || detail["origin"] != nil {
		t.Fatalf("origin excerpt outlived the friendship: %d %+v", status, detail)
	}
	status, gone := w.request(t, 1, http.MethodPut, "/social/v2/posts/"+w.post+"/like", "")
	if status != 404 || gone["code"] != "post_not_found" {
		t.Fatalf("like after unfriending: %d %+v", status, gone)
	}
}

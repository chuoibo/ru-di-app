package community

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// feedPage reads one feed page as `actor` and returns its posts.
func (f world) feedPage(t *testing.T, mode string, actor int) []Post {
	t.Helper()
	w := f.call("GET", "/v2/community/feed?mode="+mode, actor, nil)
	requireStatus(t, w, 200)
	var page struct {
		Posts []Post `json:"posts"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	return page.Posts
}

func hasPost(posts []Post, id string) bool {
	for _, p := range posts {
		if p.ID == id {
			return true
		}
	}
	return false
}

// QA UI-132: with no approved public post the shared discovery ranking is
// empty, and an empty ranking is stored as an empty array. It used to reach
// `community_feeds.post_ids` (NOT NULL) as NULL: «Dành cho bạn» and
// «Thịnh hành» answered 503 instead of an empty feed.
func TestPostgresCommunityEmptyDiscoveryIsAnEmptyFeed(t *testing.T) {
	f := setup(t)
	for _, mode := range []string{"for_you", "trending"} {
		f.h.candidateMu.Lock()
		f.h.candidates, f.h.commonRanking = nil, []string{}
		f.h.candidateUntil = time.Now().Add(time.Minute)
		f.h.candidateMu.Unlock()
		if posts := f.feedPage(t, mode, 1); len(posts) != 0 {
			t.Fatalf("%s: %d posts in an empty community", mode, len(posts))
		}
	}
}

// QA UI-142: an author who edits an approved post still finds it in their
// feed while the edit waits for review, under the pending band; the edit's
// words stay the author's alone.
func TestPostgresCommunityAuthorKeepsEditedPostInFeed(t *testing.T) {
	f := setup(t)
	p := f.post(t, "public")
	f.approve(t, p)
	edit := PostInput{LogicalID: uuid(), Revision: p.Revision, Body: "Bản sửa tổng hợp: thêm một quán cà phê ven hồ", Audience: "public", Topics: []string{"Đi bộ"}}
	requireStatus(t, f.call("PUT", "/v2/community/posts/"+p.ID, 0, edit), 200)
	f.h.candidateMu.Lock()
	f.h.candidateUntil = time.Time{}
	f.h.candidateMu.Unlock()
	mine := f.feedPage(t, "trending", 0)
	if !hasPost(mine, p.ID) {
		t.Fatal("the author's edited post left the author's own feed")
	}
	for _, q := range mine {
		if q.ID == p.ID && q.Status != "pending" {
			t.Fatalf("author's card status %q, want pending", q.Status)
		}
	}
	for _, q := range f.feedPage(t, "trending", 1) {
		if q.ID == p.ID && strings.Contains(q.Body, "Bản sửa tổng hợp") {
			t.Fatal("a reader saw the edit before review")
		}
	}
}

// QA UI-141: «Không quan tâm» can be taken back. The hidden mode lists what
// the person hid, and nothing else; un-hiding returns the post to the feed.
func TestPostgresCommunityHiddenModeListsAndRestores(t *testing.T) {
	f := setup(t)
	p := f.post(t, "public")
	f.approve(t, p)
	requireStatus(t, f.call("PUT", "/v2/community/posts/"+p.ID+"/feedback", 1, map[string]any{"kind": "hidden", "enabled": true}), 204)
	if !hasPost(f.feedPage(t, "hidden", 1), p.ID) {
		t.Fatal("hidden post missing from the hidden list")
	}
	if hasPost(f.feedPage(t, "hidden", 2), p.ID) {
		t.Fatal("another person's hidden list holds a post they never hid")
	}
	requireStatus(t, f.call("PUT", "/v2/community/posts/"+p.ID+"/feedback", 1, map[string]any{"kind": "hidden", "enabled": false}), 204)
	if hasPost(f.feedPage(t, "hidden", 1), p.ID) {
		t.Fatal("un-hidden post still listed as hidden")
	}
}

// QA UI-147: a notification says who mentioned you and where, and quotes it.
func TestPostgresCommunityNotificationNamesWhoAndWhere(t *testing.T) {
	f := setup(t)
	f.friend(t)
	w := f.call("POST", "/v2/community/posts", 0, PostInput{LogicalID: uuid(), Body: "Tổng hợp: cuối tuần đi bộ quanh hồ cùng bạn", Audience: "public", Topics: []string{"Đi bộ"}, Mentions: []string{f.people[1]}})
	requireStatus(t, w, 201)
	var p Post
	_ = json.Unmarshal(w.Body.Bytes(), &p)
	f.approve(t, p)
	w = f.call("POST", "/v2/community/posts/"+p.ID+"/comments", 0, CommentInput{LogicalID: uuid(), Body: "Tổng hợp: nhớ mang áo mưa nhé", Mentions: []string{f.people[1]}})
	requireStatus(t, w, 201)
	var c struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &c)
	requireStatus(t, f.call("POST", "/v2/community/comments/"+c.ID+"/review", 3, map[string]any{"approve": true, "reason": "Bình luận tổng hợp hợp chủ đề"}), 200)
	w = f.call("GET", "/v2/community/notifications", 1, nil)
	requireStatus(t, w, 200)
	var out struct {
		Notifications []struct {
			PostID      string  `json:"post_id"`
			Actor       *string `json:"actor"`
			FromComment bool    `json:"from_comment"`
			Excerpt     *string `json:"excerpt"`
		} `json:"notifications"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	var trongBai, trongBinhLuan bool
	for _, n := range out.Notifications {
		if n.PostID != p.ID || n.Actor == nil || *n.Actor != "Synthetic community 0" || n.Excerpt == nil {
			continue
		}
		if n.FromComment && strings.Contains(*n.Excerpt, "áo mưa") {
			trongBinhLuan = true
		}
		if !n.FromComment && strings.Contains(*n.Excerpt, "đi bộ quanh hồ") {
			trongBai = true
		}
	}
	if !trongBai || !trongBinhLuan {
		t.Fatalf("want a post mention and a comment mention, named and quoted: %s", w.Body.String())
	}
}

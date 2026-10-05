package community

import (
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

type Comment struct {
	ID        string    `json:"id"`
	AuthorID  string    `json:"author_id"`
	Author    string    `json:"author"`
	Body      string    `json:"body"`
	ParentID  *string   `json:"parent_id"`
	MediaID   *string   `json:"media_id"`
	Mentions  []string  `json:"mentions"`
	CreatedAt time.Time `json:"created_at"`
	Status    string    `json:"status"`
	CanDelete bool      `json:"can_delete"`
}
type CommentInput struct {
	LogicalID string   `json:"logical_id"`
	Body      string   `json:"body"`
	ParentID  *string  `json:"parent_id"`
	MediaID   *string  `json:"media_id"`
	Mentions  []string `json:"mentions"`
}

func (h *Handler) comments(w http.ResponseWriter, r *http.Request) {
	tx, person, err := h.begin(r)
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	ctx := r.Context()
	id := r.PathValue("post")
	author, err := readable(ctx, tx, person, id)
	if err != nil {
		fail(w, err)
		return
	}
	cursor, err := parseCursor(r.URL.Query().Get("after"))
	if err != nil {
		fail(w, err)
		return
	}
	rows, err := tx.Query(ctx, `SELECT c.id,c.author_id,a.display_name,c.body,m.parent_id,m.media_id,COALESCE(m.mentions,'{}'),c.created_at FROM post_comments c JOIN people a ON a.id=c.author_id LEFT JOIN community_comment_meta m ON m.comment_id=c.id WHERE c.post_id=$2 AND a.deleted_at IS NULL AND (c.created_at,c.id)>($3,$4::uuid) AND NOT EXISTS(SELECT 1 FROM friend_requests f WHERE f.state='blocked' AND ((f.requester_id=$1 AND f.addressee_id=c.author_id) OR (f.addressee_id=$1 AND f.requester_id=c.author_id))) ORDER BY c.created_at,c.id LIMIT 41`, person, id, cursor.At, cursor.ID)
	if err != nil {
		fail(w, err)
		return
	}
	items := []Comment{}
	for rows.Next() {
		c := Comment{Status: "approved"}
		if err = rows.Scan(&c.ID, &c.AuthorID, &c.Author, &c.Body, &c.ParentID, &c.MediaID, &c.Mentions, &c.CreatedAt); err != nil {
			break
		}
		c.CanDelete = person == author || person == c.AuthorID
		items = append(items, c)
	}
	rows.Close()
	if err == nil {
		err = rows.Err()
	}
	if err != nil {
		fail(w, err)
		return
	}
	var next *string
	if len(items) > 40 {
		items = items[:40]
		s := encodeCursor(pageCursor{At: items[39].CreatedAt, ID: items[39].ID})
		next = &s
	}
	pending := []Comment{}
	rows, err = tx.Query(ctx, `SELECT id,author_id,body,parent_id,media_id,mentions,created_at,status FROM community_comment_drafts WHERE post_id=$1 AND author_id=$2 AND status<>'approved' ORDER BY created_at DESC LIMIT 20`, id, person)
	if err != nil {
		fail(w, err)
		return
	}
	for rows.Next() {
		c := Comment{CanDelete: true}
		if err = rows.Scan(&c.ID, &c.AuthorID, &c.Body, &c.ParentID, &c.MediaID, &c.Mentions, &c.CreatedAt, &c.Status); err != nil {
			break
		}
		pending = append(pending, c)
	}
	rows.Close()
	if err == nil {
		err = rows.Err()
	}
	if err != nil {
		fail(w, err)
		return
	}
	commit(w, r, tx, 200, map[string]any{"comments": items, "pending": pending, "next_cursor": next})
}
func (h *Handler) comment(w http.ResponseWriter, r *http.Request) {
	var in CommentInput
	if err := decode(w, r, &in); err != nil {
		fail(w, err)
		return
	}
	in.Body = strings.TrimSpace(in.Body)
	if in.Body == "" || utf8.RuneCountInString(in.Body) > 2000 {
		fail(w, no(422, "invalid_body"))
		return
	}
	tx, person, err := h.begin(r)
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	ctx := r.Context()
	id := r.PathValue("post")
	if _, err = readPost(ctx, tx, person, id); err != nil {
		fail(w, err)
		return
	}
	digest := hash(struct {
		Post  string
		Input CommentInput
	}{id, in})
	cid, err := replay(ctx, tx, person, in.LogicalID, digest)
	if err != nil {
		fail(w, err)
		return
	}
	if cid != "" {
		commit(w, r, tx, 200, map[string]string{"id": cid, "status": "received"})
		return
	}
	cid, status, err := writeComment(ctx, tx, person, id, in, true)
	if err == nil {
		err = remember(ctx, tx, person, in.LogicalID, digest, cid)
	}
	if err != nil {
		fail(w, err)
		return
	}
	commit(w, r, tx, 201, map[string]string{"id": cid, "status": status})
}
func (h *Handler) deleteComment(w http.ResponseWriter, r *http.Request) {
	tx, person, err := h.begin(r)
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	ctx := r.Context()
	id, cid := r.PathValue("post"), r.PathValue("comment")
	author, err := readable(ctx, tx, person, id)
	if err != nil {
		fail(w, err)
		return
	}
	if !validID(cid) {
		fail(w, no(404, "comment_not_found"))
		return
	}
	var ca string
	err = tx.QueryRow(ctx, `SELECT author_id FROM post_comments WHERE id=$1 AND post_id=$2 UNION ALL SELECT author_id FROM community_comment_drafts WHERE id=$1 AND post_id=$2 LIMIT 1`, cid, id).Scan(&ca)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && (ca != person && author != person) {
		fail(w, no(404, "comment_not_found"))
		return
	}
	if err != nil {
		fail(w, err)
		return
	}
	_, err = tx.Exec(ctx, `DELETE FROM community_comment_drafts WHERE id=$1`, cid)
	if err == nil {
		_, err = tx.Exec(ctx, `DELETE FROM post_comments WHERE id=$1 AND post_id=$2`, cid, id)
	}
	if err != nil {
		fail(w, err)
		return
	}
	commit(w, r, tx, 204, nil)
}
func (h *Handler) like(w http.ResponseWriter, r *http.Request) {
	tx, person, err := h.begin(r)
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	ctx := r.Context()
	id := r.PathValue("post")
	if err = WriteLike(ctx, tx, person, id, r.Method == "PUT"); err != nil {
		fail(w, err)
		return
	}
	// Release the commit-order outbox lock before building the response. A hot
	// post must not serialize unrelated feed reads behind count/media queries.
	if err = tx.Commit(ctx); err != nil {
		fail(w, err)
		return
	}
	readTx, currentPerson, err := h.begin(r)
	if err != nil {
		fail(w, err)
		return
	}
	defer readTx.Rollback(ctx)
	p, err := readPost(ctx, readTx, currentPerson, id)
	if err != nil {
		fail(w, err)
		return
	}
	commit(w, r, readTx, 200, p)
}
func (h *Handler) feedback(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Kind    string `json:"kind"`
		Enabled bool   `json:"enabled"`
	}
	if err := decode(w, r, &in); err != nil {
		fail(w, err)
		return
	}
	if !slices.Contains([]string{"saved", "hidden"}, in.Kind) {
		fail(w, no(422, "invalid_feedback"))
		return
	}
	tx, person, err := h.begin(r)
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	id := r.PathValue("post")
	if _, err = readable(r.Context(), tx, person, id); err != nil {
		fail(w, err)
		return
	}
	if in.Enabled {
		_, err = tx.Exec(r.Context(), `INSERT INTO community_feedback VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, person, id, in.Kind)
	} else {
		_, err = tx.Exec(r.Context(), `DELETE FROM community_feedback WHERE person_id=$1 AND post_id=$2 AND kind=$3`, person, id, in.Kind)
	}
	if err != nil {
		fail(w, err)
		return
	}
	commit(w, r, tx, 204, nil)
}
func (h *Handler) follows(w http.ResponseWriter, r *http.Request) {
	tx, person, err := h.begin(r)
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	rows, err := tx.Query(r.Context(), `SELECT kind,target FROM community_follows WHERE person_id=$1 ORDER BY created_at DESC LIMIT 500`, person)
	if err != nil {
		fail(w, err)
		return
	}
	out := []map[string]string{}
	for rows.Next() {
		var kind, target string
		if err = rows.Scan(&kind, &target); err != nil {
			break
		}
		out = append(out, map[string]string{"kind": kind, "target": target})
	}
	rows.Close()
	if err == nil {
		err = rows.Err()
	}
	if err != nil {
		fail(w, err)
		return
	}
	commit(w, r, tx, 200, map[string]any{"follows": out})
}
func (h *Handler) follow(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Kind   string `json:"kind"`
		Target string `json:"target"`
	}
	if err := decode(w, r, &in); err != nil {
		fail(w, err)
		return
	}
	if in.Kind == "topic" {
		v, err := normalizeTopics([]string{in.Target})
		if err != nil {
			fail(w, err)
			return
		}
		in.Target = v[0]
	} else if in.Kind != "person" || !validID(in.Target) {
		fail(w, no(422, "invalid_follow"))
		return
	}
	tx, person, err := h.begin(r)
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	ctx := r.Context()
	if in.Kind == "person" {
		var yes bool
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM people p WHERE id=$2 AND deleted_at IS NULL AND id<>$1 AND NOT EXISTS(SELECT 1 FROM friend_requests f WHERE state='blocked' AND ((requester_id=$1 AND addressee_id=$2) OR (addressee_id=$1 AND requester_id=$2))))`, person, in.Target).Scan(&yes)
		if err != nil {
			fail(w, err)
			return
		}
		if !yes {
			fail(w, no(404, "person_not_found"))
			return
		}
	}
	if err = rate(ctx, tx, person); err != nil {
		fail(w, err)
		return
	}
	if r.Method == "PUT" {
		_, err = tx.Exec(ctx, `INSERT INTO community_follows(person_id,kind,target) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, person, in.Kind, in.Target)
	} else {
		_, err = tx.Exec(ctx, `DELETE FROM community_follows WHERE person_id=$1 AND kind=$2 AND target=$3`, person, in.Kind, in.Target)
	}
	if err != nil {
		fail(w, err)
		return
	}
	commit(w, r, tx, 204, nil)
}
func (h *Handler) notifications(w http.ResponseWriter, r *http.Request) {
	tx, person, err := h.begin(r)
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	// Who mentioned you and the first words of where (QA UI-147): the actor's
	// display name, whether it came from a comment, and up to 120 characters
	// of that comment or of the post as published. A row from before the
	// actor was recorded answers null and the app says «Bạn được nhắc…».
	//
	// A mention from someone either side has blocked is not shown at all, the
	// same rule that hides their comment and its image (audit 2026-10-05,
	// PER-PRIVACY-01; owner decision: hide the whole row). A comment that is
	// gone shows no words rather than the post's, which it never said.
	rows, err := tx.Query(r.Context(), `SELECT n.id,n.post_id,n.kind,n.created_at,x.display_name,n.comment_id IS NOT NULL,CASE WHEN n.comment_id IS NULL THEN left(p.body,120) ELSE left(cm.body,120) END FROM community_notifications n JOIN posts p ON p.id=n.post_id JOIN people a ON a.id=p.author_id LEFT JOIN community_posts c ON c.post_id=p.id LEFT JOIN people x ON x.id=n.actor_id AND x.deleted_at IS NULL LEFT JOIN post_comments cm ON cm.id=n.comment_id WHERE n.person_id=$1 AND a.deleted_at IS NULL AND c.deleted_at IS NULL AND (`+readableSQL+`) AND NOT EXISTS(SELECT 1 FROM friend_requests f WHERE f.state='blocked' AND ((f.requester_id=$1 AND f.addressee_id IN (n.actor_id,cm.author_id)) OR (f.addressee_id=$1 AND f.requester_id IN (n.actor_id,cm.author_id)))) ORDER BY n.created_at DESC LIMIT 50`, person)
	if err != nil {
		fail(w, err)
		return
	}
	out := []map[string]any{}
	for rows.Next() {
		var id, post, kind string
		var at time.Time
		var actor, excerpt *string
		var fromComment bool
		if err = rows.Scan(&id, &post, &kind, &at, &actor, &fromComment, &excerpt); err != nil {
			break
		}
		out = append(out, map[string]any{"id": id, "post_id": post, "kind": kind, "created_at": at, "actor": actor, "from_comment": fromComment, "excerpt": excerpt})
	}
	rows.Close()
	if err == nil {
		err = rows.Err()
	}
	if err != nil {
		fail(w, err)
		return
	}
	commit(w, r, tx, 200, map[string]any{"notifications": out})
}

package community

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"mobile/services/core/internal/aiharness/congdong"
	"mobile/services/core/internal/media/storage"
)

type verdict struct {
	Relevant     bool   `json:"relevant"`
	Safe         bool   `json:"safe"`
	Confidence   int    `json:"confidence_milli"`
	Reason       string `json:"reason"`
	MediaChecked bool   `json:"media_checked"`
}

func decision(v verdict, comment, hasMedia bool) (string, string) {
	if v.Confidence < 900 || v.Confidence > 1000 || hasMedia && !v.MediaChecked {
		return "review", "needs_review"
	}
	if !v.Safe {
		return "rejected", "unsafe_content"
	}
	if !comment && !v.Relevant {
		return "rejected", "off_topic"
	}
	return "approved", ""
}

// inferenceMedia loads the submission's attachments for a reading: image
// bytes, and only the type of a video, which the model is never sent.
func (h *Handler) inferenceMedia(ctx context.Context, ids []string) ([]congdong.Media, error) {
	out := []congdong.Media{}
	st, err := storage.New()
	if err != nil {
		return nil, err
	}
	total := 0
	for _, id := range ids {
		var key, mime string
		err = h.pool.QueryRow(ctx, `SELECT storage_key,content_type FROM community_media WHERE id=$1 AND state='ready'`, id).Scan(&key, &mime)
		if err != nil {
			return nil, err
		}
		if !strings.HasPrefix(mime, "image/") {
			out = append(out, congdong.Media{MIME: mime})
			continue
		}
		b, e := st.Read(key)
		if e != nil {
			return nil, e
		}
		total += len(b)
		if total > 64<<20 {
			return nil, no(422, "media_bundle_too_large")
		}
		out = append(out, congdong.Media{MIME: mime, Data: b})
	}
	return out, nil
}

// Run bounds inference concurrency per process; leases permit crash recovery.
func (h *Handler) Run(ctx context.Context) {
	go h.runEvents(ctx)
	for i := 0; i < 2; i++ {
		go func() {
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					h.workOne(ctx)
				}
			}
		}()
	}
	// Video has its own sandboxed runtime; an API image without codecs must not
	// claim jobs and mark valid uploads failed.
	cleanup := time.NewTicker(time.Hour)
	defer cleanup.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-cleanup.C:
			h.cleanup(ctx)
		}
	}
}
func (h *Handler) workOne(ctx context.Context) {
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return
	}
	defer tx.Rollback(ctx)
	var job int64
	var post, comment *string
	var rev *int
	err = tx.QueryRow(ctx, `UPDATE community_jobs SET lease_until=clock_timestamp()+interval '2 minutes',attempts=attempts+1 WHERE id=(SELECT id FROM community_jobs WHERE NOT done AND available_at<=clock_timestamp() AND (lease_until IS NULL OR lease_until<clock_timestamp()) ORDER BY id FOR UPDATE SKIP LOCKED LIMIT 1) RETURNING id,post_id,revision,comment_id`).Scan(&job, &post, &rev, &comment)
	if err != nil {
		return
	}
	if tx.Commit(ctx) != nil {
		return
	}
	var body string
	ids := []string{}
	var mediaID *string
	if post != nil {
		err = h.pool.QueryRow(ctx, `SELECT body,media_ids FROM community_revisions v JOIN community_posts c ON c.post_id=v.post_id AND c.revision=v.revision WHERE v.post_id=$1 AND v.revision=$2 AND c.status='pending' AND c.requested_audience='public'`, *post, *rev).Scan(&body, &ids)
	} else {
		err = h.pool.QueryRow(ctx, `SELECT body,media_id FROM community_comment_drafts WHERE id=$1 AND status='pending'`, *comment).Scan(&body, &mediaID)
		if mediaID != nil {
			ids = append(ids, *mediaID)
		}
	}
	if errors.Is(err, pgx.ErrNoRows) {
		_, _ = h.pool.Exec(ctx, `UPDATE community_jobs SET done=true WHERE id=$1`, job)
		return
	}
	var media []congdong.Media
	if err == nil {
		media, err = h.inferenceMedia(ctx, ids)
	}
	var v verdict
	if err == nil {
		// No model on this process is an outage like any other: the job
		// waits and retries, nothing is approved.
		var d congdong.Doc
		d, err = congdong.Duyet(ctx, h.ai.Luot(1), body, comment != nil, media)
		v = verdict(d)
	}
	if err != nil {
		_, _ = h.pool.Exec(ctx, `UPDATE community_jobs SET lease_until=NULL,available_at=clock_timestamp()+interval '1 minute' WHERE id=$1`, job)
		return
	}
	status, reason := decision(v, comment != nil, len(ids) > 0)
	tx, err = h.pool.Begin(ctx)
	if err != nil {
		return
	}
	defer tx.Rollback(ctx)
	if post != nil {
		var owner string
		err = tx.QueryRow(ctx, `SELECT author_id FROM posts WHERE id=$1 FOR UPDATE`, *post).Scan(&owner)
		var current int
		var state, audience string
		if err == nil {
			err = tx.QueryRow(ctx, `SELECT revision,status,requested_audience FROM community_posts WHERE post_id=$1 FOR UPDATE`, *post).Scan(&current, &state, &audience)
		}
		if err == nil && current == *rev && state == "pending" && audience == "public" {
			err = publish(ctx, tx, *post, *rev, "", status, reason)
		}
	} else {
		var pid, owner string
		err = tx.QueryRow(ctx, `SELECT post_id,author_id FROM community_comment_drafts WHERE id=$1 AND status='pending' FOR UPDATE`, *comment).Scan(&pid, &owner)
		if err == nil {
			if p, e := readPost(ctx, tx, owner, pid); e != nil || !p.CanComment {
				status = "rejected"
				reason = "post_unavailable"
			}
		}
		if err == nil {
			_, err = tx.Exec(ctx, `UPDATE community_comment_drafts SET status=$2,reason=$3 WHERE id=$1`, *comment, status, reason)
		}
		if err == nil && status == "approved" {
			err = publishComment(ctx, tx, *comment)
		}
	}
	if err != nil {
		return
	}
	_, err = tx.Exec(ctx, `UPDATE community_jobs SET done=true,lease_until=NULL WHERE id=$1`, job)
	if err == nil {
		_ = tx.Commit(ctx)
	}
}
func publishComment(ctx context.Context, tx pgx.Tx, id string) error {
	inserted, err := tx.Exec(ctx, `INSERT INTO post_comments(id,post_id,author_id,body,created_at) SELECT id,post_id,author_id,body,created_at FROM community_comment_drafts WHERE id=$1 ON CONFLICT DO NOTHING`, id)
	if err != nil || inserted.RowsAffected() == 0 {
		return err
	}
	if err == nil {
		_, err = tx.Exec(ctx, `INSERT INTO community_comment_meta(comment_id,parent_id,mentions,media_id) SELECT id,(SELECT c.id FROM post_comments c WHERE c.id=d.parent_id AND c.post_id=d.post_id),mentions,media_id FROM community_comment_drafts d WHERE id=$1 ON CONFLICT DO NOTHING`, id)
	}
	if err == nil {
		_, err = tx.Exec(ctx, `INSERT INTO community_notifications(id,person_id,post_id,kind) SELECT gen_random_uuid(),m,post_id,'mention' FROM community_comment_drafts CROSS JOIN unnest(mentions) m WHERE id=$1`, id)
	}
	return err
}
func moderator(ctx context.Context, tx pgx.Tx, person string) error {
	var yes bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM community_moderators WHERE person_id=$1)`, person).Scan(&yes)
	if err == nil && !yes {
		return no(403, "moderator_required")
	}
	return err
}
func (h *Handler) reviewQueue(w http.ResponseWriter, r *http.Request) {
	tx, person, err := h.begin(r)
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	if err = moderator(r.Context(), tx, person); err != nil {
		fail(w, err)
		return
	}
	rows, err := tx.Query(r.Context(), `SELECT c.post_id,c.revision,c.status,c.reason,v.body,v.topics,v.media_ids,COALESCE((SELECT jsonb_agg(jsonb_build_object('id',m.id,'type',m.content_type,'url','/v2/community/media/'||m.id::text)) FROM community_media m WHERE m.id=ANY(v.media_ids)),'[]'::jsonb) FROM community_posts c JOIN community_revisions v ON v.post_id=c.post_id AND v.revision=c.revision WHERE c.status IN ('pending','review') ORDER BY c.created_at LIMIT 50`)
	if err != nil {
		fail(w, err)
		return
	}
	out := []map[string]any{}
	for rows.Next() {
		var id, status, reason, body string
		var rev int
		var topics, media []string
		var attachments json.RawMessage
		if err = rows.Scan(&id, &rev, &status, &reason, &body, &topics, &media, &attachments); err != nil {
			break
		}
		out = append(out, map[string]any{"id": id, "revision": rev, "status": status, "reason": reason, "body": body, "topics": topics, "media_ids": media, "media": attachments})
	}
	rows.Close()
	if err == nil {
		err = rows.Err()
	}
	if err != nil {
		fail(w, err)
		return
	}
	rows, err = tx.Query(r.Context(), `SELECT id,post_id,body,status,reason,media_id FROM community_comment_drafts WHERE status IN ('pending','review') ORDER BY created_at LIMIT 50`)
	if err != nil {
		fail(w, err)
		return
	}
	comments := []map[string]any{}
	for rows.Next() {
		var id, post, body, status, reason string
		var media *string
		if err = rows.Scan(&id, &post, &body, &status, &reason, &media); err != nil {
			break
		}
		comments = append(comments, map[string]any{"id": id, "post_id": post, "body": body, "status": status, "reason": reason, "media_id": media})
	}
	rows.Close()
	if err == nil {
		err = rows.Err()
	}
	if err != nil {
		fail(w, err)
		return
	}
	commit(w, r, tx, 200, map[string]any{"posts": out, "comments": comments})
}

func (h *Handler) reviewComment(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Approve bool   `json:"approve"`
		Reason  string `json:"reason"`
	}
	if err := decode(w, r, &in); err != nil {
		fail(w, err)
		return
	}
	if len(in.Reason) < 3 || len(in.Reason) > 500 {
		fail(w, no(422, "review_reason_required"))
		return
	}
	tx, person, err := h.begin(r)
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	ctx := r.Context()
	if err = moderator(ctx, tx, person); err != nil {
		fail(w, err)
		return
	}
	id := r.PathValue("comment")
	if !validID(id) {
		fail(w, no(404, "comment_not_found"))
		return
	}
	var post, owner, status string
	err = tx.QueryRow(ctx, `SELECT post_id,author_id,status FROM community_comment_drafts WHERE id=$1 FOR UPDATE`, id).Scan(&post, &owner, &status)
	if err != nil {
		fail(w, no(404, "comment_not_found"))
		return
	}
	if owner == person {
		fail(w, no(403, "cannot_review_own_post"))
		return
	}
	if status != "pending" && status != "review" {
		fail(w, no(409, "revision_conflict"))
		return
	}
	status = "rejected"
	if in.Approve {
		p, e := readPost(ctx, tx, owner, post)
		if e != nil || !p.CanComment {
			fail(w, no(409, "post_unavailable"))
			return
		}
		status = "approved"
	}
	_, err = tx.Exec(ctx, `UPDATE community_comment_drafts SET status=$2,reason=$3 WHERE id=$1`, id, status, in.Reason)
	if err == nil && in.Approve {
		err = publishComment(ctx, tx, id)
	}
	if err == nil {
		_, err = tx.Exec(ctx, `INSERT INTO community_audit(post_id,actor_id,action,reason) VALUES($1,$2,$3,$4)`, post, person, "comment_"+status, in.Reason)
	}
	if err == nil {
		err = emit(ctx, tx, post, "post.changed")
	}
	if err != nil {
		fail(w, err)
		return
	}
	commit(w, r, tx, 200, map[string]string{"status": status})
}
func (h *Handler) reviewPost(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Revision int    `json:"revision"`
		Approve  bool   `json:"approve"`
		Reason   string `json:"reason"`
	}
	if err := decode(w, r, &in); err != nil {
		fail(w, err)
		return
	}
	if len(in.Reason) < 3 || len(in.Reason) > 500 {
		fail(w, no(422, "review_reason_required"))
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
	if err = moderator(ctx, tx, person); err != nil {
		fail(w, err)
		return
	}
	if !validID(id) {
		fail(w, no(404, "post_not_found"))
		return
	}
	var author string
	err = tx.QueryRow(ctx, `SELECT author_id FROM posts WHERE id=$1 FOR UPDATE`, id).Scan(&author)
	if err != nil {
		fail(w, no(404, "post_not_found"))
		return
	}
	if author == person {
		fail(w, no(403, "cannot_review_own_post"))
		return
	}
	var status, audience string
	err = tx.QueryRow(ctx, `SELECT status,requested_audience FROM community_posts WHERE post_id=$1`, id).Scan(&status, &audience)
	if err != nil || audience != "public" {
		fail(w, no(409, "revision_conflict"))
		return
	}
	status = "rejected"
	if in.Approve {
		status = "approved"
	}
	err = publish(ctx, tx, id, in.Revision, person, status, in.Reason)
	if err != nil {
		fail(w, err)
		return
	}
	commit(w, r, tx, 200, map[string]string{"status": status})
}
func (h *Handler) appeal(w http.ResponseWriter, r *http.Request) {
	tx, person, err := h.begin(r)
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	ctx := r.Context()
	id := r.PathValue("post")
	if err = owned(ctx, tx, person, id); err != nil {
		fail(w, err)
		return
	}
	if err = rate(ctx, tx, person); err != nil {
		fail(w, err)
		return
	}
	_, err = tx.Exec(ctx, `UPDATE community_posts SET status='review',reason='appeal_requested' WHERE post_id=$1 AND status IN ('pending','rejected','review')`, id)
	if err != nil {
		fail(w, err)
		return
	}
	commit(w, r, tx, 202, map[string]string{"status": "review"})
}
func (h *Handler) nep(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Confirmed bool   `json:"confirmed"`
		Excerpt   string `json:"excerpt"`
		Request   string `json:"request"`
	}
	if err := decode(w, r, &in); err != nil {
		fail(w, err)
		return
	}
	if !in.Confirmed || len(in.Excerpt) > 8000 || len(in.Request) > 2000 || in.Request == "" {
		fail(w, no(422, "consent_required"))
		return
	}
	tx, person, err := h.begin(r)
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	p, err := readPost(r.Context(), tx, person, r.PathValue("post"))
	if err != nil {
		fail(w, err)
		return
	}
	if in.Excerpt != p.Body {
		fail(w, no(409, "source_changed"))
		return
	}
	if err = rate(r.Context(), tx, person); err != nil {
		fail(w, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		fail(w, err)
		return
	}
	draft, err := congdong.Nep(r.Context(), h.ai.Luot(1), in.Excerpt, in.Request)
	if err != nil || draft == "" || len(draft) > 20000 {
		fail(w, no(503, "nep_unavailable"))
		return
	}
	// Revalidate after inference; a revoked session must not receive the result.
	tx, person, err = h.begin(r)
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	if _, err = readable(r.Context(), tx, person, p.ID); err != nil {
		fail(w, err)
		return
	}
	commit(w, r, tx, 200, map[string]any{"draft": draft, "ai_generated": true})
}
func (h *Handler) cleanup(ctx context.Context) {
	_, _ = h.pool.Exec(ctx, `DELETE FROM community_media m WHERE m.created_at<clock_timestamp()-interval '1 day' AND NOT EXISTS(SELECT 1 FROM community_revisions WHERE m.id=ANY(media_ids)) AND NOT EXISTS(SELECT 1 FROM community_comment_drafts WHERE media_id=m.id) AND NOT EXISTS(SELECT 1 FROM community_comment_meta WHERE media_id=m.id)`)
	if st, err := storage.New(); err == nil {
		rows, e := h.pool.Query(ctx, `SELECT storage_key FROM community_media_gc LIMIT 100`)
		if e == nil {
			keys := []string{}
			for rows.Next() {
				var key string
				if rows.Scan(&key) == nil {
					keys = append(keys, key)
				}
			}
			rows.Close()
			for _, key := range keys {
				if _, e = st.Delete(key); e == nil {
					_, _ = h.pool.Exec(ctx, `DELETE FROM community_media_gc WHERE storage_key=$1`, key)
				}
			}
		}
	}
	for _, query := range []string{`DELETE FROM community_interactions WHERE created_at<clock_timestamp()-interval '90 days'`, `DELETE FROM community_feeds WHERE expires_at<clock_timestamp()`, `DELETE FROM community_limits WHERE minute<clock_timestamp()-interval '1 day'`, `DELETE FROM community_events WHERE created_at<clock_timestamp()-interval '1 day' AND relayed_at IS NOT NULL`, `DELETE FROM community_jobs WHERE done`} {
		_, _ = h.pool.Exec(ctx, query)
	}
}

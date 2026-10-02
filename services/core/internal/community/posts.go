package community

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"golang.org/x/text/unicode/norm"
)

type PostInput struct {
	LogicalID string   `json:"logical_id"`
	Revision  int      `json:"revision"`
	Body      string   `json:"body"`
	Audience  string   `json:"audience"`
	ContextID *string  `json:"context_id"`
	Topics    []string `json:"topics"`
	Mentions  []string `json:"mentions"`
	MediaIDs  []string `json:"media_ids"`
}

func normalizeTopics(values []string) ([]string, error) {
	if len(values) > 5 {
		return nil, no(422, "too_many_topics")
	}
	out := []string{}
	for _, s := range values {
		s = norm.NFC.String(strings.ToLower(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(s), "#"))))
		s = strings.Join(strings.Fields(s), " ")
		if utf8.RuneCountInString(s) < 2 || utf8.RuneCountInString(s) > 40 || strings.ContainsAny(s, "/\\<>@") {
			return nil, no(422, "invalid_topic")
		}
		if !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	return out, nil
}
func validateInput(ctx context.Context, tx pgx.Tx, person string, in *PostInput) error {
	in.Body = strings.TrimSpace(in.Body)
	if in.Body == "" || utf8.RuneCountInString(in.Body) > 5000 {
		return no(422, "invalid_body")
	}
	if !slices.Contains([]string{"only_me", "friends", "group", "public"}, in.Audience) {
		return no(422, "invalid_audience")
	}
	if in.Audience == "group" {
		if in.ContextID == nil || !validID(*in.ContextID) {
			return no(422, "group_required")
		}
		var yes bool
		err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM memberships m JOIN contexts c ON c.id=m.context_id WHERE m.person_id=$1 AND m.context_id=$2 AND m.state='active' AND m.left_at IS NULL AND c.kind='group')`, person, *in.ContextID).Scan(&yes)
		if err != nil {
			return err
		}
		if !yes {
			return no(404, "group_not_found")
		}
	} else if in.ContextID != nil {
		return no(422, "invalid_context")
	}
	var err error
	in.Topics, err = normalizeTopics(in.Topics)
	if err != nil {
		return err
	}
	if in.Mentions == nil {
		in.Mentions = []string{}
	}
	if in.MediaIDs == nil {
		in.MediaIDs = []string{}
	}
	if len(in.Mentions) > 20 || len(in.MediaIDs) > 10 {
		return no(422, "too_many_attachments")
	}
	for _, id := range in.Mentions {
		if !validID(id) {
			return no(422, "invalid_mention")
		}
		var yes bool
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM friend_requests f JOIN people p ON p.id=$2 AND p.deleted_at IS NULL WHERE f.state='accepted' AND ((f.requester_id=$1 AND f.addressee_id=$2) OR (f.addressee_id=$1 AND f.requester_id=$2)))`, person, id).Scan(&yes)
		if err != nil {
			return err
		}
		if !yes {
			return no(422, "mention_requires_friend")
		}
	}
	seen := map[string]bool{}
	var totalBytes int64
	for _, id := range in.MediaIDs {
		if !validID(id) || seen[id] {
			return no(422, "invalid_media")
		}
		seen[id] = true
		var mime string
		var size int64
		err = tx.QueryRow(ctx, `SELECT content_type,byte_size FROM community_media WHERE id=$1 AND owner_id=$2 AND state='ready'`, id, person).Scan(&mime, &size)
		if errors.Is(err, pgx.ErrNoRows) {
			return no(422, "media_not_ready")
		}
		if err != nil {
			return err
		}
		if strings.HasPrefix(mime, "video/") && len(in.MediaIDs) != 1 {
			return no(422, "video_must_be_alone")
		}
		totalBytes += size
		if totalBytes > 64<<20 {
			return no(422, "media_bundle_too_large")
		}
	}
	return nil
}
func replay(ctx context.Context, tx pgx.Tx, person, logical, digest string) (string, error) {
	if !validID(logical) {
		return "", no(422, "logical_id_required")
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,734138))`, person+logical); err != nil {
		return "", err
	}
	var id, old string
	err := tx.QueryRow(ctx, `SELECT resource_id,digest FROM community_idempotency WHERE person_id=$1 AND logical_id=$2`, person, logical).Scan(&id, &old)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err == nil && old != digest {
		return "", no(409, "idempotency_conflict")
	}
	return id, err
}
func remember(ctx context.Context, tx pgx.Tx, person, logical, digest, id string) error {
	_, err := tx.Exec(ctx, `INSERT INTO community_idempotency VALUES($1,$2,$3,$4)`, person, logical, digest, id)
	return err
}
func writeRevision(ctx context.Context, tx pgx.Tx, id string, rev int, in PostInput) error {
	_, err := tx.Exec(ctx, `INSERT INTO community_revisions(post_id,revision,body,topics,mentions,media_ids) VALUES($1,$2,$3,$4,$5,$6)`, id, rev, in.Body, in.Topics, in.Mentions, in.MediaIDs)
	return err
}
func enqueue(ctx context.Context, tx pgx.Tx, id string, rev int) error {
	_, err := tx.Exec(ctx, `INSERT INTO community_jobs(post_id,revision) VALUES($1,$2)`, id, rev)
	return err
}
func (h *Handler) createPost(w http.ResponseWriter, r *http.Request) {
	var in PostInput
	if err := decode(w, r, &in); err != nil {
		fail(w, err)
		return
	}
	tx, person, err := h.begin(r)
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	ctx := r.Context()
	digest := hash(in)
	id, err := replay(ctx, tx, person, in.LogicalID, digest)
	if err != nil {
		fail(w, err)
		return
	}
	if id != "" {
		p, e := readPost(ctx, tx, person, id)
		if e != nil {
			fail(w, e)
			return
		}
		commit(w, r, tx, 200, p)
		return
	}
	if err = validateInput(ctx, tx, person, &in); err != nil {
		fail(w, err)
		return
	}
	if err = rate(ctx, tx, person); err != nil {
		fail(w, err)
		return
	}
	id = uuid()
	physical, status := in.Audience, "private"
	if physical == "public" {
		physical = "only_me"
		status = "pending"
	}
	_, err = tx.Exec(ctx, `INSERT INTO posts(id,author_id,audience,context_id,body,created_at) VALUES($1,$2,$3,$4,$5,clock_timestamp())`, id, person, physical, in.ContextID, in.Body)
	if err == nil {
		_, err = tx.Exec(ctx, `INSERT INTO community_posts(post_id,requested_audience,status,topics,mentions,published_revision) VALUES($1,$2,$3,$4,$5,CASE WHEN $3='private' THEN 1 ELSE NULL END)`, id, in.Audience, status, in.Topics, in.Mentions)
	}
	if err == nil {
		err = writeRevision(ctx, tx, id, 1, in)
	}
	if err == nil && status == "pending" {
		err = enqueue(ctx, tx, id, 1)
	}
	if err == nil {
		err = remember(ctx, tx, person, in.LogicalID, digest, id)
	}
	if err == nil {
		err = emit(ctx, tx, id, "post.changed")
	}
	if err != nil {
		fail(w, err)
		return
	}
	p, err := readPost(ctx, tx, person, id)
	if err != nil {
		fail(w, err)
		return
	}
	commit(w, r, tx, 201, p)
}
func (h *Handler) editPost(w http.ResponseWriter, r *http.Request) {
	var in PostInput
	if err := decode(w, r, &in); err != nil {
		fail(w, err)
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
	if err = owned(ctx, tx, person, id); err != nil {
		fail(w, err)
		return
	}
	if err = validateInput(ctx, tx, person, &in); err != nil {
		fail(w, err)
		return
	}
	var rev int
	var audience string
	var diaryID *string
	var groupID *string
	err = tx.QueryRow(ctx, `SELECT c.revision,c.requested_audience,c.diary_id,p.context_id FROM community_posts c JOIN posts p ON p.id=c.post_id WHERE c.post_id=$1 FOR UPDATE OF c`, id).Scan(&rev, &audience, &diaryID, &groupID)
	if err != nil {
		fail(w, no(404, "post_not_found"))
		return
	}
	if in.Revision != rev {
		fail(w, no(409, "revision_conflict"))
		return
	}
	if diaryID != nil {
		fail(w, no(422, "edit_source_diary"))
		return
	}
	if audience == "group" && (groupID == nil || in.ContextID == nil || *in.ContextID != *groupID) {
		fail(w, no(422, "group_cannot_change"))
		return
	}
	if in.Audience != audience {
		fail(w, no(422, "use_audience_action"))
		return
	}
	if err = rate(ctx, tx, person); err != nil {
		fail(w, err)
		return
	}
	rev++
	err = writeRevision(ctx, tx, id, rev, in)
	status := "private"
	if audience == "public" {
		status = "pending"
	}
	if err == nil {
		_, err = tx.Exec(ctx, `UPDATE community_posts SET revision=$2,status=$3,reason='' WHERE post_id=$1`, id, rev, status)
	}
	if err == nil && status == "pending" {
		err = enqueue(ctx, tx, id, rev)
	} else if err == nil {
		err = publish(ctx, tx, id, rev, person, "private", "")
	}
	if err == nil {
		err = emit(ctx, tx, id, "post.changed")
	}
	if err != nil {
		fail(w, err)
		return
	}
	p, err := readPost(ctx, tx, person, id)
	if err != nil {
		fail(w, err)
		return
	}
	commit(w, r, tx, 200, p)
}

// publish runs under the post lock and refuses stale moderation decisions.
func publish(ctx context.Context, tx pgx.Tx, id string, rev int, actor, status, reason string) error {
	if _, err := tx.Exec(ctx, `SET LOCAL rudi.community_writer = 'on'`); err != nil {
		return err
	}
	var current int
	var audience string
	err := tx.QueryRow(ctx, `SELECT revision,requested_audience FROM community_posts WHERE post_id=$1 AND deleted_at IS NULL FOR UPDATE`, id).Scan(&current, &audience)
	if err != nil {
		return err
	}
	if current != rev {
		return no(409, "revision_conflict")
	}
	if status == "approved" || status == "private" {
		_, err = tx.Exec(ctx, `UPDATE posts p SET body=v.body,audience=c.requested_audience FROM community_posts c JOIN community_revisions v ON v.post_id=c.post_id AND v.revision=$2 WHERE p.id=$1 AND c.post_id=p.id`, id, rev)
		if err == nil {
			_, err = tx.Exec(ctx, `UPDATE community_posts c SET published_revision=$2,status=$3,reason='',topics=v.topics,mentions=v.mentions,published_at=CASE WHEN $3='approved' THEN clock_timestamp() ELSE NULL END FROM community_revisions v WHERE c.post_id=$1 AND v.post_id=c.post_id AND v.revision=$2`, id, rev, status)
		}
		if err == nil {
			_, err = tx.Exec(ctx, `INSERT INTO community_notifications(id,person_id,post_id,kind,actor_id) SELECT gen_random_uuid(),m,$1,'mention',p.author_id FROM community_revisions v JOIN posts p ON p.id=v.post_id CROSS JOIN unnest(v.mentions) m WHERE v.post_id=$1 AND v.revision=$2`, id, rev)
		}
	} else {
		// A moderator withdrawing the currently published revision revokes its bytes too.
		if status == "rejected" {
			_, err = tx.Exec(ctx, `UPDATE posts SET audience='only_me',context_id=NULL WHERE id=$1 AND EXISTS(SELECT 1 FROM community_posts WHERE post_id=$1 AND published_revision=$2)`, id, rev)
			if err != nil {
				return err
			}
			_, err = tx.Exec(ctx, `UPDATE community_posts SET published_revision=NULL,published_at=NULL WHERE post_id=$1 AND published_revision=$2`, id, rev)
			if err != nil {
				return err
			}
		}
		_, err = tx.Exec(ctx, `UPDATE community_posts SET status=$2,reason=$3 WHERE post_id=$1`, id, status, reason)
	}
	if err == nil {
		_, err = tx.Exec(ctx, `INSERT INTO community_audit(post_id,revision,actor_id,action,reason) VALUES($1,$2,NULLIF($3,'')::uuid,$4,$5)`, id, rev, actor, status, reason)
	}
	if err == nil {
		err = emit(ctx, tx, id, "post.changed")
	}
	return err
}
func (h *Handler) deletePost(w http.ResponseWriter, r *http.Request) {
	tx, person, err := h.begin(r)
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	id := r.PathValue("post")
	ctx := r.Context()
	if err = owned(ctx, tx, person, id); err != nil {
		fail(w, err)
		return
	}
	if _, err = tx.Exec(ctx, `DELETE FROM posts WHERE id=$1`, id); err == nil {
		err = emit(ctx, tx, id, "post.removed")
	}
	if err != nil {
		fail(w, err)
		return
	}
	commit(w, r, tx, 204, nil)
}
func (h *Handler) submitPost(w http.ResponseWriter, r *http.Request) {
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
	var rev int
	err = tx.QueryRow(ctx, `SELECT revision FROM community_posts WHERE post_id=$1`, id).Scan(&rev)
	if err == nil {
		var derived bool
		err = tx.QueryRow(ctx, `SELECT diary_document IS NOT NULL FROM community_posts WHERE post_id=$1`, id).Scan(&derived)
		if err == nil && derived {
			fail(w, no(422, "share_source_diary_again"))
			return
		}
	}
	if errors.Is(err, pgx.ErrNoRows) {
		// Explicit opt-in is required for legacy public posts. Group photographs cannot cross the boundary.
		var in PostInput
		var image *string
		err = tx.QueryRow(ctx, `SELECT body,audience,image_url FROM posts WHERE id=$1`, id).Scan(&in.Body, &in.Audience, &image)
		if err == nil && (in.Audience != "public" || image != nil) {
			err = no(422, "legacy_media_requires_new_post")
		}
		if err == nil {
			_, err = tx.Exec(ctx, `INSERT INTO community_posts(post_id,requested_audience,status) VALUES($1,'public','pending')`, id)
		}
		if err == nil {
			in.Topics = []string{}
			in.Mentions = []string{}
			in.MediaIDs = []string{}
			err = writeRevision(ctx, tx, id, 1, in)
		}
		rev = 1
	}
	if err == nil {
		_, err = tx.Exec(ctx, `UPDATE community_posts SET status='pending',reason='',requested_audience='public' WHERE post_id=$1`, id)
	}
	if err == nil {
		_, err = tx.Exec(ctx, `UPDATE posts SET audience='only_me',context_id=NULL WHERE id=$1`, id)
	}
	if err == nil {
		err = enqueue(ctx, tx, id, rev)
	}
	if err == nil {
		err = emit(ctx, tx, id, "post.changed")
	}
	if err != nil {
		fail(w, err)
		return
	}
	commit(w, r, tx, 202, map[string]string{"status": "pending"})
}
func (h *Handler) audience(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Audience string `json:"audience"`
		Revision int    `json:"revision"`
	}
	if err := decode(w, r, &in); err != nil {
		fail(w, err)
		return
	}
	if !slices.Contains([]string{"friends", "only_me"}, in.Audience) {
		fail(w, no(422, "use_submit_for_public"))
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
	if err = owned(ctx, tx, person, id); err != nil {
		fail(w, err)
		return
	}
	tag, err := tx.Exec(ctx, `UPDATE community_posts SET requested_audience=$2,revision=revision+1,status='private',reason='',published_at=NULL WHERE post_id=$1 AND revision=$3`, id, in.Audience, in.Revision)
	if err == nil && tag.RowsAffected() != 1 {
		err = no(409, "revision_conflict")
	}
	if err == nil {
		_, err = tx.Exec(ctx, `INSERT INTO community_revisions SELECT post_id,revision+1,body,topics,mentions,media_ids,clock_timestamp() FROM community_revisions WHERE post_id=$1 AND revision=$2`, id, in.Revision)
	}
	if err == nil {
		_, err = tx.Exec(ctx, `UPDATE posts SET context_id=NULL WHERE id=$1`, id)
	}
	if err == nil {
		err = publish(ctx, tx, id, in.Revision+1, person, "private", "")
	}
	if err != nil {
		fail(w, err)
		return
	}
	p, err := readPost(ctx, tx, person, id)
	if err != nil {
		fail(w, err)
		return
	}
	commit(w, r, tx, 200, p)
}

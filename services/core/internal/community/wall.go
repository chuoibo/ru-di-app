package community

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

// This file is the seam other Go writers use for the rows Cộng đồng shares with
// the personal wall (ADR-0040): one post, one like table, one comment table and
// one parent. A second package that wants to write a comment or a like calls
// these instead of inserting into post_comments or post_reactions itself, so a
// public comment can never skip the moderation queue through another route.

// LikeKind is the post_reactions kind that both the community feed and the
// wall count as a like («Thích»). The five other kinds stay message reactions.
const LikeKind = "heart"

// Refusal reports the HTTP status and code of an error returned by this
// package's exported writers; ok is false for an infrastructure failure.
func Refusal(err error) (status int, code string, ok bool) {
	var d *denial
	if errors.As(err, &d) {
		return d.status, d.code, true
	}
	return 0, "", false
}

// Readable returns the author of a post the person may read now, applying the
// same relationship, block, erasure and removal rules as the community feed.
func Readable(ctx context.Context, tx pgx.Tx, person, post string) (string, error) {
	return readable(ctx, tx, person, post)
}

// WallImage is the first ready image of a Cộng đồng post's published
// revision, as the Cộng đồng media route serves it (with its own audience
// check), or "" for a post that is not one or has no image. The wall read only
// `posts.image_url`, which a Cộng đồng post never sets: its photos live on the
// revision, and a post with a photo reached the wall as words (QA UI-156).
func WallImage(ctx context.Context, tx pgx.Tx, post string) (string, error) {
	var id string
	err := tx.QueryRow(ctx, `SELECT m.id::text FROM community_posts c
  JOIN community_revisions v ON v.post_id=c.post_id AND v.revision=c.published_revision
  CROSS JOIN LATERAL unnest(v.media_ids) WITH ORDINALITY AS u(mid,ord)
  JOIN community_media m ON m.id=u.mid
 WHERE c.post_id=$1 AND c.deleted_at IS NULL AND m.state='ready' AND m.content_type LIKE 'image/%'
 ORDER BY u.ord LIMIT 1`, post).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return "/v2/community/media/" + id, nil
}

// WriteLike sets or clears the person's like on a post they can read.
func WriteLike(ctx context.Context, tx pgx.Tx, person, post string, like bool) error {
	if _, err := readable(ctx, tx, person, post); err != nil {
		return err
	}
	if err := rate(ctx, tx, person); err != nil {
		return err
	}
	var err error
	if like {
		_, err = tx.Exec(ctx, `INSERT INTO post_reactions(id,post_id,person_id,kind,created_at) VALUES($1,$2,$3,$4,clock_timestamp()) ON CONFLICT(post_id,person_id,kind) DO NOTHING`, uuid(), post, person, LikeKind)
	} else {
		_, err = tx.Exec(ctx, `DELETE FROM post_reactions WHERE post_id=$1 AND person_id=$2 AND kind=$3`, post, person, LikeKind)
	}
	return err
}

// WallComment is a comment written through WriteComment by another package.
type WallComment struct {
	Body     string
	ParentID *string
}

// WriteComment stores a comment and returns its id and status: "approved"
// when it is already in post_comments, "pending" when it waits for review.
//
// moderated says whether the community moderation worker runs on this host.
// A comment on a public post is held for review whenever it does. When it does
// not, a comment on a legacy public post is published as the legacy wall
// route would publish it, but a post that went through Cộng đồng refuses the
// write instead of accepting a comment nobody would ever review.
func WriteComment(ctx context.Context, tx pgx.Tx, person, post string, in WallComment, moderated bool) (string, string, error) {
	return writeComment(ctx, tx, person, post, CommentInput{Body: in.Body, ParentID: in.ParentID}, moderated)
}

func writeComment(ctx context.Context, tx pgx.Tx, person, id string, in CommentInput, moderated bool) (string, string, error) {
	in.Body = strings.TrimSpace(in.Body)
	if in.Body == "" || utf8.RuneCountInString(in.Body) > 2000 {
		return "", "", no(422, "invalid_body")
	}
	p, err := readPost(ctx, tx, person, id)
	if err != nil {
		return "", "", err
	}
	if !p.CanComment {
		return "", "", no(403, "comments_closed")
	}
	if in.ParentID != nil {
		if !validID(*in.ParentID) {
			return "", "", no(422, "invalid_parent")
		}
		var yes bool
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM post_comments c LEFT JOIN community_comment_meta m ON m.comment_id=c.id WHERE c.id=$1 AND c.post_id=$2 AND m.parent_id IS NULL)`, *in.ParentID, id).Scan(&yes)
		if err != nil {
			return "", "", err
		}
		if !yes {
			return "", "", no(422, "invalid_parent")
		}
	}
	check := PostInput{Body: in.Body, Audience: "only_me", Mentions: in.Mentions}
	if in.MediaID != nil {
		check.MediaIDs = []string{*in.MediaID}
	}
	if err = validateInput(ctx, tx, person, &check); err != nil {
		return "", "", err
	}
	in.Mentions = check.Mentions
	if in.MediaID != nil {
		var mime string
		if err = tx.QueryRow(ctx, `SELECT content_type FROM community_media WHERE id=$1`, *in.MediaID).Scan(&mime); err != nil {
			return "", "", err
		}
		if !strings.HasPrefix(mime, "image/") {
			return "", "", no(422, "comment_image_only")
		}
	}
	status := "approved"
	if p.Audience == "public" {
		if moderated {
			status = "pending"
		} else {
			var reviewed bool
			if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM community_posts WHERE post_id=$1)`, id).Scan(&reviewed); err != nil {
				return "", "", err
			}
			if reviewed {
				return "", "", no(409, "community_unavailable")
			}
		}
	}
	if err = rate(ctx, tx, person); err != nil {
		return "", "", err
	}
	cid := uuid()
	_, err = tx.Exec(ctx, `INSERT INTO community_comment_drafts(id,post_id,author_id,body,parent_id,mentions,media_id,status) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, cid, id, person, in.Body, in.ParentID, in.Mentions, in.MediaID, status)
	if err == nil && status == "pending" {
		_, err = tx.Exec(ctx, `INSERT INTO community_jobs(comment_id) VALUES($1)`, cid)
	}
	if err == nil && status == "approved" {
		err = publishComment(ctx, tx, cid)
	}
	return cid, status, err
}

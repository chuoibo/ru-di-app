package socialv2

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"mobile/services/core/internal/community"
	"mobile/services/core/internal/repo"
)

type comment struct {
	ID                string    `json:"id"`
	PostID            string    `json:"post_id"`
	ParentID          *string   `json:"parent_id"`
	AuthorID          string    `json:"author_id"`
	AuthorDisplayName string    `json:"author_display_name"`
	Body              string    `json:"body"`
	CreatedAt         time.Time `json:"created_at"`
	LikeCount         int64     `json:"like_count"`
	Liked             bool      `json:"liked"`
	Replies           []comment `json:"replies"`
}

func scanComment(row pgx.Row) (*comment, error) {
	var c comment
	err := row.Scan(&c.ID, &c.PostID, &c.ParentID, &c.AuthorID, &c.AuthorDisplayName, &c.Body, &c.CreatedAt, &c.LikeCount, &c.Liked)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	c.Replies = []comment{}
	return &c, nil
}

// commentFrom reads the one parent a comment has: community_comment_meta, the
// row Cộng đồng's writer fills when a comment is published. Comments by
// erased people, or by someone in a block with the reader, are left out the
// same way the community thread leaves them out.
const commentFrom = ` FROM post_comments c JOIN people p ON p.id=c.author_id LEFT JOIN community_comment_meta m ON m.comment_id=c.id
 WHERE p.deleted_at IS NULL AND NOT EXISTS(SELECT 1 FROM friend_requests f WHERE f.state='blocked' AND ((f.requester_id=$2::uuid AND f.addressee_id=c.author_id) OR (f.addressee_id=$2::uuid AND f.requester_id=c.author_id)))`

const commentColumns = `c.id,c.post_id,m.parent_id,c.author_id,p.display_name,c.body,c.created_at,
 (SELECT count(*) FROM social_comment_likes l WHERE l.comment_id=c.id),
 EXISTS(SELECT 1 FROM social_comment_likes l WHERE l.comment_id=c.id AND l.person_id=$2::uuid)`

func listReplies(ctx context.Context, tx pgx.Tx, parentID, actor string) ([]comment, error) {
	rows, err := tx.Query(ctx, `SELECT `+commentColumns+commentFrom+`
 AND m.parent_id=$1::uuid ORDER BY c.created_at,c.id`, parentID, actor)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []comment{}
	for rows.Next() {
		c, err := scanComment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

func (h *Handler) comments(w http.ResponseWriter, r *http.Request) {
	postID, err := parseID(r.PathValue("post_id"))
	if err != nil {
		fail(w, err)
		return
	}
	limit, err := queryLimit(r, 20, 50)
	if err != nil {
		fail(w, err)
		return
	}
	var cursor *wallCursor
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		v, e := decodeWallCursor(raw)
		if e != nil {
			fail(w, bad(422, "invalid_cursor"))
			return
		}
		cursor = &v
	}
	h.run(w, r, func(ctx context.Context, tx pgx.Tx, actor string) (int, any, error) {
		if _, err := visiblePost(ctx, tx, postID, actor); err != nil {
			return 0, nil, err
		}
		query := `SELECT ` + commentColumns + commentFrom + `
 AND c.post_id=$1::uuid AND m.parent_id IS NULL`
		args := []any{postID, actor}
		if cursor != nil {
			query += ` AND (c.created_at,c.id)>($3::timestamptz,$4::uuid)`
			args = append(args, cursor.CreatedAt, cursor.ID)
		}
		query += ` ORDER BY c.created_at,c.id LIMIT `
		if cursor != nil {
			query += `$5::integer`
		} else {
			query += `$3::integer`
		}
		args = append(args, limit+1)
		rows, err := tx.Query(ctx, query, args...)
		if err != nil {
			return 0, nil, err
		}
		out := []comment{}
		for rows.Next() {
			c, e := scanComment(rows)
			if e != nil {
				rows.Close()
				return 0, nil, e
			}
			out = append(out, *c)
		}
		rows.Close()
		if err = rows.Err(); err != nil {
			return 0, nil, err
		}
		hasMore := len(out) > limit
		if hasMore {
			out = out[:limit]
		}
		for i := range out {
			out[i].Replies, err = listReplies(ctx, tx, out[i].ID, actor)
			if err != nil {
				return 0, nil, err
			}
		}
		var next any
		if hasMore && len(out) > 0 {
			last := out[len(out)-1]
			next = encodeWallCursor(last.CreatedAt, last.ID)
		}
		pending, err := pendingComments(ctx, tx, postID, actor)
		if err != nil {
			return 0, nil, err
		}
		return 200, map[string]any{"post_id": postID, "comments": out, "pending": pending, "next_cursor": next, "has_more": hasMore}, nil
	})
}

// pendingComment is the reader's own comment still waiting for review. Only
// its author sees it; nobody else learns it exists.
type pendingComment struct {
	ID        string    `json:"id"`
	PostID    string    `json:"post_id"`
	ParentID  *string   `json:"parent_id"`
	Body      string    `json:"body"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

func pendingComments(ctx context.Context, tx pgx.Tx, postID, actor string) ([]pendingComment, error) {
	rows, err := tx.Query(ctx, `SELECT id,post_id,parent_id,body,status,created_at FROM community_comment_drafts
 WHERE post_id=$1::uuid AND author_id=$2::uuid AND status IN ('pending','review','rejected') ORDER BY created_at,id LIMIT 20`, postID, actor)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []pendingComment{}
	for rows.Next() {
		var c pendingComment
		if err = rows.Scan(&c.ID, &c.PostID, &c.ParentID, &c.Body, &c.Status, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// writtenComment answers a create or its replay: the published comment, or,
// while it waits for review, the draft with 202.
func writtenComment(ctx context.Context, tx pgx.Tx, id, actor string) (int, any, error) {
	row := tx.QueryRow(ctx, `SELECT `+commentColumns+commentFrom+` AND c.id=$1::uuid`, id, actor)
	created, err := scanComment(row)
	if err != nil {
		return 0, nil, err
	}
	if created != nil {
		return 201, created, nil
	}
	var draft pendingComment
	err = tx.QueryRow(ctx, `SELECT id,post_id,parent_id,body,status,created_at FROM community_comment_drafts WHERE id=$1::uuid AND author_id=$2::uuid`, id, actor).
		Scan(&draft.ID, &draft.PostID, &draft.ParentID, &draft.Body, &draft.Status, &draft.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil, bad(404, "comment_not_found")
	}
	if err != nil {
		return 0, nil, err
	}
	return 202, draft, nil
}

func (h *Handler) createComment(w http.ResponseWriter, r *http.Request) {
	postID, err := parseID(r.PathValue("post_id"))
	if err != nil {
		fail(w, err)
		return
	}
	var input struct {
		Body     string  `json:"body"`
		ParentID *string `json:"parent_id"`
	}
	if err = readJSON(w, r, &input); err != nil {
		fail(w, err)
		return
	}
	input.Body = strings.TrimSpace(input.Body)
	if input.Body == "" || len([]rune(input.Body)) > 2000 {
		fail(w, bad(422, "invalid_comment"))
		return
	}
	if input.ParentID != nil {
		id, e := parseID(*input.ParentID)
		if e != nil {
			fail(w, bad(422, "invalid_parent"))
			return
		}
		input.ParentID = &id
	}
	h.run(w, r, func(ctx context.Context, tx pgx.Tx, actor string) (int, any, error) {
		if _, err := visiblePost(ctx, tx, postID, actor); err != nil {
			return 0, nil, err
		}
		request, err := json.Marshal(struct {
			PostID   string
			Body     string
			ParentID *string
		}{postID, input.Body, input.ParentID})
		if err != nil {
			return 0, nil, err
		}
		previous, err := beginAttempt(ctx, tx, r.Header, actor, "comment", string(request))
		if err != nil {
			return 0, nil, err
		}
		if previous != nil {
			return writtenComment(ctx, tx, *previous, actor)
		}
		// Cộng đồng owns the comment row, its parent and its review (ADR-0040).
		id, _, err := community.WriteComment(ctx, tx, actor, postID, community.WallComment{Body: input.Body, ParentID: input.ParentID}, h.moderated)
		if err != nil {
			return 0, nil, err
		}
		if err = finishAttempt(ctx, tx, r.Header, actor, id); err != nil {
			return 0, nil, err
		}
		return writtenComment(ctx, tx, id, actor)
	})
}

func (h *Handler) likePost(w http.ResponseWriter, r *http.Request)   { h.postLike(w, r, true) }
func (h *Handler) unlikePost(w http.ResponseWriter, r *http.Request) { h.postLike(w, r, false) }
func (h *Handler) postLike(w http.ResponseWriter, r *http.Request, like bool) {
	postID, err := parseID(r.PathValue("post_id"))
	if err != nil {
		fail(w, err)
		return
	}
	h.run(w, r, func(ctx context.Context, tx pgx.Tx, actor string) (int, any, error) {
		if _, err := visiblePost(ctx, tx, postID, actor); err != nil {
			return 0, nil, err
		}
		if err = community.WriteLike(ctx, tx, actor, postID, like); err != nil {
			return 0, nil, err
		}
		var count int64
		err = tx.QueryRow(ctx, `SELECT count(*) FROM post_reactions WHERE post_id=$1::uuid AND kind=$2`, postID, community.LikeKind).Scan(&count)
		if err != nil {
			return 0, nil, err
		}
		return 200, map[string]any{"post_id": postID, "liked": like, "like_count": count}, nil
	})
}

func (h *Handler) likeComment(w http.ResponseWriter, r *http.Request)   { h.commentLike(w, r, true) }
func (h *Handler) unlikeComment(w http.ResponseWriter, r *http.Request) { h.commentLike(w, r, false) }
func (h *Handler) commentLike(w http.ResponseWriter, r *http.Request, like bool) {
	commentID, err := parseID(r.PathValue("comment_id"))
	if err != nil {
		fail(w, err)
		return
	}
	h.run(w, r, func(ctx context.Context, tx pgx.Tx, actor string) (int, any, error) {
		var postID string
		err = tx.QueryRow(ctx, `SELECT post_id FROM post_comments WHERE id=$1::uuid`, commentID).Scan(&postID)
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil, bad(404, "comment_not_found")
		}
		if err != nil {
			return 0, nil, err
		}
		if _, err = visiblePost(ctx, tx, postID, actor); err != nil {
			return 0, nil, err
		}
		if like {
			_, err = tx.Exec(ctx, `INSERT INTO social_comment_likes(comment_id,person_id) VALUES($1::uuid,$2::uuid) ON CONFLICT DO NOTHING`, commentID, actor)
		} else {
			_, err = tx.Exec(ctx, `DELETE FROM social_comment_likes WHERE comment_id=$1::uuid AND person_id=$2::uuid`, commentID, actor)
		}
		if err != nil {
			return 0, nil, err
		}
		var count int64
		err = tx.QueryRow(ctx, `SELECT count(*) FROM social_comment_likes WHERE comment_id=$1::uuid`, commentID).Scan(&count)
		if err != nil {
			return 0, nil, err
		}
		return 200, map[string]any{"comment_id": commentID, "liked": like, "like_count": count}, nil
	})
}

func (h *Handler) repost(w http.ResponseWriter, r *http.Request) {
	originID, err := parseID(r.PathValue("post_id"))
	if err != nil {
		fail(w, err)
		return
	}
	var input struct {
		Audience  string  `json:"audience"`
		ContextID *string `json:"context_id"`
	}
	if err = readJSON(w, r, &input); err != nil {
		fail(w, err)
		return
	}
	if input.Audience != "only_me" && input.Audience != "friends" && input.Audience != "public" && input.Audience != "group" {
		fail(w, bad(422, "invalid_audience"))
		return
	}
	if input.Audience == "public" && h.moderated {
		// ADR-0040: public now means submitted to Cộng đồng and reviewed. A
		// repost has no revision to review, so it stays among people who know
		// the author.
		fail(w, bad(422, "public_repost_needs_review"))
		return
	}
	if input.Audience == "group" && input.ContextID == nil {
		fail(w, bad(422, "context_required"))
		return
	}
	if input.Audience != "group" && input.ContextID != nil {
		fail(w, bad(422, "context_not_addressable"))
		return
	}
	if input.ContextID != nil {
		id, e := parseID(*input.ContextID)
		if e != nil {
			fail(w, bad(422, "invalid_context"))
			return
		}
		input.ContextID = &id
	}
	h.run(w, r, func(ctx context.Context, tx pgx.Tx, actor string) (int, any, error) {
		if _, err := visiblePost(ctx, tx, originID, actor); err != nil {
			return 0, nil, err
		}
		store := repo.Repository{Q: tx}
		if input.ContextID != nil {
			member, e := store.IsMember(ctx, *input.ContextID, actor)
			if e != nil {
				return 0, nil, e
			}
			if !member {
				return 0, nil, bad(403, "membership_required")
			}
		}
		request, err := json.Marshal(struct {
			OriginID  string
			Audience  string
			ContextID *string
		}{originID, input.Audience, input.ContextID})
		if err != nil {
			return 0, nil, err
		}
		previous, err := beginAttempt(ctx, tx, r.Header, actor, "repost", string(request))
		if err != nil {
			return 0, nil, err
		}
		if previous != nil {
			return 201, map[string]any{"id": *previous, "origin_id": originID}, nil
		}
		created, err := store.CreatePost(ctx, repo.PostInput{AuthorID: actor, Audience: input.Audience, ContextID: input.ContextID, Body: "Chia sẻ một bài viết", Now: time.Now().UTC()})
		if err != nil {
			return 0, nil, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO social_reposts(post_id,origin_post_id) VALUES($1::uuid,$2::uuid)`, created.ID, originID); err != nil {
			return 0, nil, err
		}
		if err = finishAttempt(ctx, tx, r.Header, actor, created.ID); err != nil {
			return 0, nil, err
		}
		return 201, map[string]any{"id": created.ID, "origin_id": originID}, nil
	})
}

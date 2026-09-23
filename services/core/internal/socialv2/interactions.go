package socialv2

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"mobile/services/core/internal/domain/postaudience"
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

const commentColumns = `c.id,c.post_id,c.parent_id,c.author_id,p.display_name,c.body,c.created_at,
 (SELECT count(*) FROM social_comment_likes l WHERE l.comment_id=c.id),
 EXISTS(SELECT 1 FROM social_comment_likes l WHERE l.comment_id=c.id AND l.person_id=$2::uuid)`

func listReplies(ctx context.Context, tx pgx.Tx, parentID, actor string) ([]comment, error) {
	rows, err := tx.Query(ctx, `SELECT `+commentColumns+` FROM post_comments c JOIN people p ON p.id=c.author_id
 WHERE c.parent_id=$1::uuid ORDER BY c.created_at,c.id`, parentID, actor)
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
		query := `SELECT ` + commentColumns + ` FROM post_comments c JOIN people p ON p.id=c.author_id
 WHERE c.post_id=$1::uuid AND c.parent_id IS NULL`
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
		return 200, map[string]any{"post_id": postID, "comments": out, "next_cursor": next, "has_more": hasMore}, nil
	})
}

func commentAllowed(ctx context.Context, tx pgx.Tx, post *postAccess, actor string) (bool, error) {
	person, err := (repo.Repository{Q: tx}).GetPerson(ctx, post.post.AuthorID)
	if err != nil {
		return false, err
	}
	policy := "nobody"
	if person != nil {
		policy = person.WallCommentPolicy
	}
	return postaudience.CanComment(postaudience.Post{AuthorID: post.post.AuthorID, Audience: post.post.Audience, ContextID: post.post.ContextID}, policy, actor, post.friend, post.member), nil
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
		post, err := visiblePost(ctx, tx, postID, actor)
		if err != nil {
			return 0, nil, err
		}
		allowed, err := commentAllowed(ctx, tx, post, actor)
		if err != nil {
			return 0, nil, err
		}
		if !allowed {
			return 0, nil, bad(403, "comments_closed")
		}
		if input.ParentID != nil {
			var parent commentParent
			err = tx.QueryRow(ctx, `SELECT post_id,coalesce(parent_id::text,'') FROM post_comments WHERE id=$1::uuid`, *input.ParentID).Scan(&parent.PostID, &parent.ParentID)
			if errors.Is(err, pgx.ErrNoRows) {
				return 0, nil, bad(404, "comment_not_found")
			}
			if err != nil {
				return 0, nil, err
			}
			if !replyParentAllowed(postID, parent) {
				return 0, nil, bad(422, "reply_depth_exceeded")
			}
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
			row := tx.QueryRow(ctx, `SELECT `+commentColumns+` FROM post_comments c JOIN people p ON p.id=c.author_id WHERE c.id=$1::uuid`, *previous, actor)
			existing, err := scanComment(row)
			if err != nil {
				return 0, nil, err
			}
			if existing == nil {
				return 0, nil, bad(404, "comment_not_found")
			}
			return 201, existing, nil
		}
		id, err := repo.NewUUID()
		if err != nil {
			return 0, nil, err
		}
		_, err = tx.Exec(ctx, `INSERT INTO post_comments(id,post_id,parent_id,author_id,body) VALUES($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5)`, id, postID, input.ParentID, actor, input.Body)
		if err != nil {
			return 0, nil, err
		}
		if err = finishAttempt(ctx, tx, r.Header, actor, id); err != nil {
			return 0, nil, err
		}
		row := tx.QueryRow(ctx, `SELECT `+commentColumns+` FROM post_comments c JOIN people p ON p.id=c.author_id WHERE c.id=$1::uuid`, id, actor)
		created, err := scanComment(row)
		if err != nil {
			return 0, nil, err
		}
		return 201, created, nil
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
		if like {
			id, e := repo.NewUUID()
			if e != nil {
				return 0, nil, e
			}
			_, err = tx.Exec(ctx, `INSERT INTO post_reactions(id,post_id,person_id,kind) VALUES($1::uuid,$2::uuid,$3::uuid,'like') ON CONFLICT (post_id,person_id,kind) DO NOTHING`, id, postID, actor)
		} else {
			_, err = tx.Exec(ctx, `DELETE FROM post_reactions WHERE post_id=$1::uuid AND person_id=$2::uuid AND kind='like'`, postID, actor)
		}
		if err != nil {
			return 0, nil, err
		}
		var count int64
		err = tx.QueryRow(ctx, `SELECT count(*) FROM post_reactions WHERE post_id=$1::uuid AND kind='like'`, postID).Scan(&count)
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

package socialv2

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"mobile/services/core/internal/community"
	"mobile/services/core/internal/domain/postaudience"
	"mobile/services/core/internal/repo"
)

func (h *Handler) wall(w http.ResponseWriter, r *http.Request) {
	personID, err := parseID(r.PathValue("person_id"))
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
		value, err := decodeWallCursor(raw)
		if err != nil {
			fail(w, bad(422, "invalid_cursor"))
			return
		}
		cursor = &value
	}
	h.run(w, r, func(ctx context.Context, tx pgx.Tx, actor string) (int, any, error) {
		var created *time.Time
		var beforeID *string
		if cursor != nil {
			created, beforeID = &cursor.CreatedAt, &cursor.ID
		}
		posts, err := (repo.Repository{Q: tx}).ListPersonPostsPageVisibleTo(ctx, personID, actor, limit+1, created, beforeID)
		if err != nil {
			return 0, nil, err
		}
		hasMore := len(posts) > limit
		if hasMore {
			posts = posts[:limit]
		}
		out := make([]any, 0, len(posts))
		for _, post := range posts {
			facts, err := postFacts(ctx, tx, post, actor)
			if err != nil {
				return 0, nil, err
			}
			if !facts.visible {
				continue
			}
			item, err := wireWallPost(ctx, tx, post, actor, facts)
			if err != nil {
				return 0, nil, err
			}
			out = append(out, item)
		}
		var next any
		if hasMore && len(posts) > 0 {
			last := posts[len(posts)-1]
			next = encodeWallCursor(last.CreatedAt, last.ID)
		}
		return 200, map[string]any{"person_id": personID, "posts": out, "next_cursor": next, "has_more": hasMore}, nil
	})
}

func (h *Handler) detail(w http.ResponseWriter, r *http.Request) {
	postID, err := parseID(r.PathValue("post_id"))
	if err != nil {
		fail(w, err)
		return
	}
	h.run(w, r, func(ctx context.Context, tx pgx.Tx, actor string) (int, any, error) {
		post, err := visiblePost(ctx, tx, postID, actor)
		if err != nil {
			return 0, nil, err
		}
		access := facts{visible: true, friend: post.friend, member: post.member}
		body, err := wireWallPost(ctx, tx, post.post, actor, access)
		return 200, body, err
	})
}

func wireWallPost(ctx context.Context, tx pgx.Tx, post repo.Post, actor string, access facts) (map[string]any, error) {
	store := repo.Repository{Q: tx}
	author, err := store.GetPerson(ctx, post.AuthorID)
	if err != nil {
		return nil, err
	}
	name, policy := "", "nobody"
	if author != nil {
		name, policy = author.DisplayName, author.WallCommentPolicy
	}
	var likes, comments int64
	var liked bool
	err = tx.QueryRow(ctx, `SELECT (SELECT count(*) FROM post_reactions WHERE post_id=$1::uuid AND kind=$3),
	  (SELECT count(*) FROM post_comments WHERE post_id=$1::uuid),
	  EXISTS(SELECT 1 FROM post_reactions WHERE post_id=$1::uuid AND person_id=$2::uuid AND kind=$3)`, post.ID, actor, community.LikeKind).Scan(&likes, &comments, &liked)
	if err != nil {
		return nil, err
	}
	var image any
	if post.ImageURL != nil {
		image = *post.ImageURL
	}
	var contextID any
	if post.ContextID != nil {
		contextID = *post.ContextID
	}
	item := map[string]any{
		"id": post.ID, "author_id": post.AuthorID, "author_display_name": name, "audience": post.Audience,
		"context_id": contextID, "body": post.Body, "image_url": image, "created_at": post.CreatedAt.UTC().Format(time.RFC3339Nano),
		"like_count": likes, "liked": liked, "comment_count": comments,
		"can_comment": postaudience.CanComment(postaudience.Post{AuthorID: post.AuthorID, Audience: post.Audience, ContextID: post.ContextID}, policy, actor, access.friend, access.member),
		"origin":      nil,
	}
	var originID *string
	err = tx.QueryRow(ctx, `SELECT origin_post_id FROM social_reposts WHERE post_id=$1::uuid`, post.ID).Scan(&originID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if err == nil {
		item["is_repost"] = true
		if originID != nil {
			origin, err := visiblePost(ctx, tx, *originID, actor)
			var denied refusal
			if err != nil && !errors.As(err, &denied) {
				return nil, err
			}
			if err == nil {
				person, err := store.GetPerson(ctx, origin.post.AuthorID)
				if err != nil {
					return nil, err
				}
				originName := ""
				if person != nil {
					originName = person.DisplayName
				}
				item["origin"] = map[string]any{"id": origin.post.ID, "author_id": origin.post.AuthorID, "author_display_name": originName, "body": origin.post.Body, "image_url": origin.post.ImageURL, "created_at": origin.post.CreatedAt.UTC().Format(time.RFC3339Nano)}
			}
		}
	}
	return item, nil
}

type changeEvent struct {
	ID     string `json:"id"`
	PostID string `json:"post_id"`
	Kind   string `json:"kind"`
}
type changePage struct {
	Events     []changeEvent `json:"events"`
	NextCursor *string       `json:"next_cursor"`
	HasMore    bool          `json:"has_more"`
}

func (h *Handler) changes(w http.ResponseWriter, r *http.Request) {
	personID, err := parseID(r.PathValue("person_id"))
	if err != nil {
		fail(w, err)
		return
	}
	limit, err := queryLimit(r, 100, 100)
	if err != nil {
		fail(w, err)
		return
	}
	after := r.URL.Query().Get("after")
	if after != "" {
		if _, err = parseID(after); err != nil {
			fail(w, bad(422, "invalid_cursor"))
			return
		}
	}
	wait := 0
	if raw := r.URL.Query().Get("wait"); raw != "" {
		wait, err = strconv.Atoi(raw)
		if err != nil || wait < 0 || wait > 20 {
			fail(w, bad(422, "invalid_wait"))
			return
		}
	}
	deadline := time.Now().Add(time.Duration(wait) * time.Second)
	wake, unsubscribe := h.subscribe(personID)
	defer unsubscribe()
	for {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		page, err := h.readChanges(ctx, r, personID, after, limit)
		cancel()
		if err != nil {
			fail(w, err)
			return
		}
		if len(page.Events) > 0 || wait == 0 || !time.Now().Before(deadline) {
			respond(w, 200, page)
			return
		}
		remaining := time.Until(deadline)
		if remaining > 2*time.Second {
			remaining = 2 * time.Second
		}
		if remaining < 0 {
			remaining = 0
		}
		select {
		case <-r.Context().Done():
			return
		case <-wake:
		case <-time.After(remaining):
		}
	}
}

func (h *Handler) readChanges(ctx context.Context, r *http.Request, personID, after string, limit int) (changePage, error) {
	out := changePage{Events: []changeEvent{}}
	if after != "" {
		out.NextCursor = &after
	}
	tx, err := h.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	actor, err := h.actor(ctx, tx, r.Header)
	if err != nil {
		return out, err
	}
	var sequence int64
	if after != "" {
		err = tx.QueryRow(ctx, `SELECT sequence FROM social_wall_changes WHERE id=$1::uuid AND wall_owner_id=$2::uuid`, after, personID).Scan(&sequence)
		if errors.Is(err, pgx.ErrNoRows) {
			return out, bad(422, "invalid_cursor")
		}
		if err != nil {
			return out, err
		}
	}
	args := []any{personID, sequence}
	bind := func(value any) string { args = append(args, value); return "$" + strconv.Itoa(len(args)) }
	where := repo.ReadablePostsWhere(bind, actor)
	limitBind := bind(limit + 1)
	rows, err := tx.Query(ctx, `SELECT changes.id,changes.post_id,changes.kind
	 FROM social_wall_changes changes JOIN posts ON posts.id=changes.post_id
	 WHERE changes.wall_owner_id=$1::uuid AND changes.sequence>$2 AND (`+where+`)
	 ORDER BY changes.sequence LIMIT `+limitBind+`::integer`, args...)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var event changeEvent
		if err = rows.Scan(&event.ID, &event.PostID, &event.Kind); err != nil {
			rows.Close()
			return out, err
		}
		out.Events = append(out.Events, event)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return out, err
	}
	if len(out.Events) > limit {
		out.Events = out.Events[:limit]
		out.HasMore = true
	}
	if len(out.Events) > 0 {
		cursor := out.Events[len(out.Events)-1].ID
		out.NextCursor = &cursor
	}
	if err = tx.Commit(ctx); err != nil {
		return out, err
	}
	return out, nil
}

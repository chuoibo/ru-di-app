package routes

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"mobile/services/core/internal/cursors"
	"mobile/services/core/internal/domain/photoref"
	"mobile/services/core/internal/domain/postaudience"
	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/repo"
	"mobile/services/core/internal/service"
)

// audienceDetail is _AUDIENCE_DETAIL.
var audienceDetail = map[string]string{
	postaudience.CodeUnknownAudience:           "Post visibility must be one of the four known levels",
	postaudience.CodeGroupAudienceNeedsContext: "A post shared with a group must name the group",
	postaudience.CodeContextNotAddressable:     "Only a group post may name a group",
}

// postRow is one (record, is_friend, is_group_member) _wire_posts serialises.
type postRow struct {
	post          repo.Post
	isFriend      bool
	isGroupMember bool
}

// createPost is POST /posts (routes/posts.py create_post,
// ApiService.create_post): permission, the audience rule, the group's roster
// for a group post, the photograph, then the insert.
func createPost() Route {
	return Route{ID: "POST /posts", Status: 201, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		if err := requireFacts(call, "create_post", map[string]bool{}); err != nil {
			return endpoint.Reply{}, err
		}
		request, err := bodyModel(call, "request")
		if err != nil {
			return endpoint.Reply{}, err
		}
		body, err := stringField(request, "body")
		if err != nil {
			return endpoint.Reply{}, err
		}
		audience, err := stringField(request, "audience")
		if err != nil {
			return endpoint.Reply{}, err
		}
		contextID, err := optionalUUIDField(request, "context_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		imageURL, err := optionalStringField(request, "image_url")
		if err != nil {
			return endpoint.Reply{}, err
		}
		var refused *postaudience.AudienceError
		if err := postaudience.CheckWritable(audience, contextID); errors.As(err, &refused) {
			return endpoint.Reply{}, endpoint.Refuse(422, strings.ToLower(refused.Code), audienceDetail[refused.Code])
		} else if err != nil {
			return endpoint.Reply{}, err
		}
		tx, err := call.Unit.Tx(ctx)
		if err != nil {
			return endpoint.Reply{}, err
		}
		store := repo.Repository{Q: tx}
		if audience == postaudience.Group {
			member, err := store.IsMember(ctx, *contextID, call.Actor.ID)
			if err != nil {
				return endpoint.Reply{}, err
			}
			if err := requireFacts(call, "address_post_to_group", map[string]bool{"is_group_member": member}); err != nil {
				return endpoint.Reply{}, err
			}
		}
		var stored *string
		if imageURL != nil {
			url, err := postPhotoURL(ctx, call, store, *imageURL, audience, contextID)
			if err != nil {
				return endpoint.Reply{}, err
			}
			stored = &url
		}
		record, err := store.CreatePost(ctx, repo.PostInput{
			AuthorID: call.Actor.ID, Audience: audience, ContextID: contextID,
			Body: body, ImageURL: stored, Now: time.Now().UTC(),
		})
		if err != nil {
			return endpoint.Reply{}, err
		}
		posts, err := wirePosts(ctx, store, []postRow{{post: record}}, call.Actor.ID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		return endpoint.Reply{Body: posts[0]}, nil
	}}
}

// postPhotoURL is _post_photo_url: a group's photograph only on a post to that
// same group, behind its roster; a personal one only the author's own, and
// only one that exists.
func postPhotoURL(ctx context.Context, call *endpoint.Call, store repo.Repository, imageURL, audience string, contextID *string) (string, error) {
	ref, err := photoref.Parse(imageURL)
	var malformed *photoref.PhotoURLError
	if errors.As(err, &malformed) {
		return "", endpoint.Refuse(422, "photo_url_invalid", "image_url is not a photo of this product")
	}
	if err != nil {
		return "", err
	}
	if ref.OwnerKind == photoref.OwnerContext {
		// `str(request.context_id) != ref.owner_id`: str(None) never matches.
		if audience != postaudience.Group || contextID == nil || *contextID != ref.OwnerID {
			return "", endpoint.Refuse(422, "photo_not_addressable", "Ảnh của nhóm chỉ đăng được cho chính nhóm đó.")
		}
		member, err := store.IsMember(ctx, ref.OwnerID, call.Actor.ID)
		if err != nil {
			return "", err
		}
		if err := requireFacts(call, "address_post_to_group", map[string]bool{"is_group_member": member}); err != nil {
			return "", err
		}
		return ref.URL(), nil
	}
	if ref.OwnerID != call.Actor.ID {
		return "", endpoint.Refuse(403, "permission_denied", "Chỉ đăng được ảnh của chính mình.")
	}
	image, err := store.GetPersonImage(ctx, call.Actor.ID, ref.PhotoID)
	if err != nil {
		return "", err
	}
	if image == nil {
		return "", endpoint.Refuse(404, "photo_not_found", "Photo does not exist")
	}
	return ref.URL(), nil
}

// listPosts is GET /posts (list_posts): the repository's visible rows, judged
// again by the domain.
func listPosts() Route {
	return Route{ID: "GET /posts", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		limit, err := intParam(call, "limit")
		if err != nil {
			return endpoint.Reply{}, err
		}
		tx, err := call.Unit.Tx(ctx)
		if err != nil {
			return endpoint.Reply{}, err
		}
		store := repo.Repository{Q: tx}
		records, err := store.ListPostsVisibleTo(ctx, call.Actor.ID, limit)
		if err != nil {
			return endpoint.Reply{}, err
		}
		posts, err := readablePosts(ctx, store, records, call.Actor.ID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		body := pyjson.NewOrderedMap()
		body.Set("posts", posts)
		return endpoint.Reply{Body: body}, nil
	}}
}

// listPersonPosts is GET /people/{person_id}/posts (list_person_posts): 200
// with an empty list for somebody the reader shares nothing with.
func listPersonPosts() Route {
	return Route{ID: "GET /people/{person_id}/posts", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		personID, err := pathUUID(call, "person_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		limit, err := intParam(call, "limit")
		if err != nil {
			return endpoint.Reply{}, err
		}
		tx, err := call.Unit.Tx(ctx)
		if err != nil {
			return endpoint.Reply{}, err
		}
		store := repo.Repository{Q: tx}
		records, err := store.ListPersonPostsVisibleTo(ctx, personID, call.Actor.ID, limit)
		if err != nil {
			return endpoint.Reply{}, err
		}
		posts, err := readablePosts(ctx, store, records, call.Actor.ID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		body := pyjson.NewOrderedMap()
		body.Set("person_id", pyjson.String(personID))
		body.Set("posts", posts)
		return endpoint.Reply{Body: body}, nil
	}}
}

// readPost is GET /posts/{post_id} (read_post): one post, or 404.
func readPost() Route {
	return Route{ID: "GET /posts/{post_id}", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		store, record, facts, err := readablePostOr404(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		posts, err := wirePosts(ctx, store, []postRow{{post: *record, isFriend: facts.isFriend, isGroupMember: facts.isGroupMember}}, call.Actor.ID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		return endpoint.Reply{Body: posts[0]}, nil
	}}
}

// reactToPost is POST /posts/{post_id}/reactions (react_to_post): a second
// tap of the same kind finds the first row; the answer is recounted.
func reactToPost() Route {
	return Route{ID: "POST /posts/{post_id}/reactions", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		request, err := bodyModel(call, "request")
		if err != nil {
			return endpoint.Reply{}, err
		}
		kind, err := stringField(request, "kind")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, record, _, err := readablePostOr404(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if err := requireFacts(call, "react_to_post", map[string]bool{"may_read_post": true}); err != nil {
			return endpoint.Reply{}, err
		}
		if _, err := store.AddPostReaction(ctx, record.ID, call.Actor.ID, kind, time.Now().UTC()); err != nil {
			return endpoint.Reply{}, err
		}
		body, err := postReactionsResponse(ctx, store, record.ID, call.Actor.ID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		return endpoint.Reply{Body: body}, nil
	}}
}

// unreactToPost is DELETE /posts/{post_id}/reactions/{kind} (unreact_to_post).
func unreactToPost() Route {
	return Route{ID: "DELETE /posts/{post_id}/reactions/{kind}", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		kind, err := stringParam(call, "kind")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, record, _, err := readablePostOr404(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if err := requireFacts(call, "react_to_post", map[string]bool{"may_read_post": true}); err != nil {
			return endpoint.Reply{}, err
		}
		if _, err := store.RemovePostReaction(ctx, record.ID, call.Actor.ID, kind); err != nil {
			return endpoint.Reply{}, err
		}
		body, err := postReactionsResponse(ctx, store, record.ID, call.Actor.ID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		return endpoint.Reply{Body: body}, nil
	}}
}

// listPostComments is GET /posts/{post_id}/comments (list_post_comments):
// oldest first, one extra row fetched to know whether more follow.
func listPostComments() Route {
	return Route{ID: "GET /posts/{post_id}/comments", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		limit, err := intParam(call, "limit")
		if err != nil {
			return endpoint.Reply{}, err
		}
		after, err := optionalStringParam(call, "after")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, record, _, err := readablePostOr404(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		var cursor *repo.PostCommentCursor
		if after != nil {
			position, err := cursors.DecodeCursor(*after)
			var bad *cursors.CursorError
			if errors.As(err, &bad) {
				return endpoint.Reply{}, endpoint.Refuse(422, "invalid_cursor", "Comment cursor is invalid")
			}
			if err != nil {
				return endpoint.Reply{}, err
			}
			cursor = &repo.PostCommentCursor{CreatedAt: position.CreatedAt.Time(), ID: position.MessageID}
		}
		rows, err := store.ListPostComments(ctx, record.ID, limit+1, cursor)
		if err != nil {
			return endpoint.Reply{}, err
		}
		page := rows
		hasMore := len(rows) > limit
		if hasMore {
			page = rows[:limit]
		}
		comments := pyjson.List{}
		for _, comment := range page {
			comments = append(comments, wirePostComment(comment))
		}
		body := pyjson.NewOrderedMap()
		body.Set("post_id", pyjson.String(record.ID))
		body.Set("comments", comments)
		if hasMore {
			last := page[len(page)-1]
			body.Set("next_cursor", pyjson.String(cursors.EncodeCursor(last.CreatedAt, last.ID)))
		} else {
			body.Set("next_cursor", pyjson.Null{})
		}
		body.Set("has_more", pyjson.Bool(hasMore))
		return endpoint.Reply{Body: body}, nil
	}}
}

// postComment is POST /posts/{post_id}/comments (post_comment): readable, then
// the wall owner's policy; a closed wall is 403 comments_closed.
func postComment() Route {
	return Route{ID: "POST /posts/{post_id}/comments", Status: 201, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		request, err := bodyModel(call, "request")
		if err != nil {
			return endpoint.Reply{}, err
		}
		text, err := stringField(request, "body")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, record, facts, err := readablePostOr404(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		author, err := store.GetPerson(ctx, record.AuthorID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		policy := "nobody"
		if author != nil {
			policy = author.WallCommentPolicy
		}
		allowed := postaudience.CanComment(postDict(*record), policy, call.Actor.ID, facts.isFriend, facts.isGroupMember)
		refused, err := service.RequirePermission("comment_on_post", *call.Actor,
			service.Resource{Proven: map[string]bool{"may_comment": allowed}})
		if err != nil {
			return endpoint.Reply{}, err
		}
		if refused != nil {
			return endpoint.Reply{}, endpoint.Refuse(403, "comments_closed", "Chủ tường không cho bình luận bài này.")
		}
		comment, err := store.CreatePostComment(ctx, record.ID, call.Actor.ID, text, time.Now().UTC())
		if err != nil {
			return endpoint.Reply{}, err
		}
		return endpoint.Reply{Body: wirePostComment(comment)}, nil
	}}
}

// deletePostComment is DELETE /posts/{post_id}/comments/{comment_id}
// (delete_post_comment): a comment of another post under this id is 404.
func deletePostComment() Route {
	return Route{ID: "DELETE /posts/{post_id}/comments/{comment_id}", Status: 204, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		commentID, err := pathUUID(call, "comment_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, record, _, err := readablePostOr404(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		comment, err := store.GetPostComment(ctx, commentID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if comment == nil || comment.PostID != record.ID {
			return endpoint.Reply{}, endpoint.Refuse(404, "comment_not_found", "Comment does not exist")
		}
		allowed := postaudience.CanDeleteComment(postaudience.Comment{AuthorID: comment.AuthorID}, postDict(*record), call.Actor.ID)
		if err := requireFacts(call, "delete_post_comment", map[string]bool{"may_delete_comment": allowed}); err != nil {
			return endpoint.Reply{}, err
		}
		if _, err := store.DeletePostComment(ctx, comment.ID); err != nil {
			return endpoint.Reply{}, err
		}
		return endpoint.Reply{Empty: true}, nil
	}}
}

type postFacts struct {
	isFriend, isGroupMember bool
}

// readablePostOr404 is _readable_post_or_404: the post and the two facts it
// was judged by, or 404, never 403. Reads in Python's order: the friend edge,
// the roster when the post names a group, then the block edge.
func readablePostOr404(ctx context.Context, call *endpoint.Call) (repo.Repository, *repo.Post, postFacts, error) {
	postID, err := pathUUID(call, "post_id")
	if err != nil {
		return repo.Repository{}, nil, postFacts{}, err
	}
	tx, err := call.Unit.Tx(ctx)
	if err != nil {
		return repo.Repository{}, nil, postFacts{}, err
	}
	store := repo.Repository{Q: tx}
	notFound := endpoint.Refuse(404, "post_not_found", "Post does not exist")
	record, err := store.GetPost(ctx, postID)
	if err != nil {
		return repo.Repository{}, nil, postFacts{}, err
	}
	if record == nil {
		return repo.Repository{}, nil, postFacts{}, notFound
	}
	facts, err := readPostFacts(ctx, store, *record, call.Actor.ID)
	if err != nil {
		return repo.Repository{}, nil, postFacts{}, err
	}
	blocked, err := isBlockedWith(ctx, store, call.Actor.ID, record.AuthorID)
	if err != nil {
		return repo.Repository{}, nil, postFacts{}, err
	}
	if !postaudience.VisibleTo(postDict(*record), call.Actor.ID, facts.isFriend, facts.isGroupMember, blocked) {
		return repo.Repository{}, nil, postFacts{}, notFound
	}
	return store, record, facts, nil
}

// readPostFacts is _post_facts.
func readPostFacts(ctx context.Context, store repo.Repository, record repo.Post, readerID string) (postFacts, error) {
	friend, err := isFriend(ctx, store, readerID, record.AuthorID)
	if err != nil {
		return postFacts{}, err
	}
	member := false
	if record.ContextID != nil {
		if member, err = store.IsMember(ctx, *record.ContextID, readerID); err != nil {
			return postFacts{}, err
		}
	}
	return postFacts{isFriend: friend, isGroupMember: member}, nil
}

// readablePosts is _readable_posts: friendship, roster and block read once per
// author or group, in the loop's order, then the domain's visible_to.
func readablePosts(ctx context.Context, store repo.Repository, records []repo.Post, readerID string) (pyjson.List, error) {
	friends := map[string]bool{}
	members := map[string]bool{}
	blocked := map[string]bool{}
	var rows []postRow
	for _, record := range records {
		if _, seen := friends[record.AuthorID]; !seen {
			friend, err := isFriend(ctx, store, readerID, record.AuthorID)
			if err != nil {
				return nil, err
			}
			friends[record.AuthorID] = friend
		}
		if record.ContextID != nil {
			if _, seen := members[*record.ContextID]; !seen {
				member, err := store.IsMember(ctx, *record.ContextID, readerID)
				if err != nil {
					return nil, err
				}
				members[*record.ContextID] = member
			}
		}
		friend := friends[record.AuthorID]
		member := record.ContextID != nil && members[*record.ContextID]
		if _, seen := blocked[record.AuthorID]; !seen {
			block, err := isBlockedWith(ctx, store, readerID, record.AuthorID)
			if err != nil {
				return nil, err
			}
			blocked[record.AuthorID] = block
		}
		if postaudience.VisibleTo(postDict(record), readerID, friend, member, blocked[record.AuthorID]) {
			rows = append(rows, postRow{post: record, isFriend: friend, isGroupMember: member})
		}
	}
	return wirePosts(ctx, store, rows, readerID)
}

// wirePosts is _wire_posts: counts from the rows, the reader's own reactions,
// the author's name, and can_comment decided here from the author's policy.
func wirePosts(ctx context.Context, store repo.Repository, rows []postRow, readerID string) (pyjson.List, error) {
	out := pyjson.List{}
	if len(rows) == 0 {
		return out, nil
	}
	ids := make([]string, len(rows))
	for i, row := range rows {
		ids[i] = row.post.ID
	}
	counts, err := store.PostSocialCounts(ctx, ids, readerID)
	if err != nil {
		return nil, err
	}
	authors := map[string]*repo.Person{}
	for _, row := range rows {
		record := row.post
		author, seen := authors[record.AuthorID]
		if !seen {
			if author, err = store.GetPerson(ctx, record.AuthorID); err != nil {
				return nil, err
			}
			authors[record.AuthorID] = author
		}
		policy, name := "nobody", ""
		if author != nil {
			policy, name = author.WallCommentPolicy, author.DisplayName
		}
		item := pyjson.NewOrderedMap()
		item.Set("id", pyjson.String(record.ID))
		item.Set("author_id", pyjson.String(record.AuthorID))
		item.Set("audience", pyjson.String(record.Audience))
		item.Set("context_id", textOrNull(record.ContextID))
		item.Set("body", pyjson.String(record.Body))
		item.Set("image_url", textOrNull(record.ImageURL))
		item.Set("created_at", pyjson.String(pyjson.DateTime(record.CreatedAt.UTC())))
		item.Set("author_display_name", pyjson.String(name))
		item.Set("reactions", reactionCounts(counts, record.ID))
		item.Set("my_reactions", myReactions(counts, record.ID))
		item.Set("comment_count", pyjson.NewInt(commentCount(counts, record.ID)))
		item.Set("can_comment", pyjson.Bool(postaudience.CanComment(postDict(record), policy, readerID, row.isFriend, row.isGroupMember)))
		out = append(out, item)
	}
	return out, nil
}

// postReactionsResponse is _post_reactions_response.
func postReactionsResponse(ctx context.Context, store repo.Repository, postID, readerID string) (*pyjson.OrderedMap, error) {
	counts, err := store.PostSocialCounts(ctx, []string{postID}, readerID)
	if err != nil {
		return nil, err
	}
	body := pyjson.NewOrderedMap()
	body.Set("post_id", pyjson.String(postID))
	body.Set("reactions", reactionCounts(counts, postID))
	body.Set("my_reactions", myReactions(counts, postID))
	return body, nil
}

// reactionCounts is `[PostReactionCount(kind, per_kind[kind]) for kind in sorted(per_kind)]`.
func reactionCounts(counts repo.PostSocialCounts, postID string) pyjson.List {
	var kinds []repo.PostKindCount
	for _, tally := range counts.Reactions {
		if tally.PostID == postID {
			kinds = append([]repo.PostKindCount(nil), tally.Kinds...)
			break
		}
	}
	sort.Slice(kinds, func(i, j int) bool { return kinds[i].Kind < kinds[j].Kind })
	out := pyjson.List{}
	for _, kind := range kinds {
		item := pyjson.NewOrderedMap()
		item.Set("kind", pyjson.String(kind.Kind))
		item.Set("count", pyjson.NewInt(kind.Count))
		out = append(out, item)
	}
	return out
}

// myReactions is `sorted(counts.mine.get(post_id, frozenset()))`.
func myReactions(counts repo.PostSocialCounts, postID string) pyjson.List {
	var kinds []string
	for _, mine := range counts.Mine {
		if mine.PostID == postID {
			kinds = append([]string(nil), mine.Kinds...)
			break
		}
	}
	sort.Strings(kinds)
	out := pyjson.List{}
	for _, kind := range kinds {
		out = append(out, pyjson.String(kind))
	}
	return out
}

// commentCount is `counts.comments.get(post_id, 0)`.
func commentCount(counts repo.PostSocialCounts, postID string) int64 {
	for _, tally := range counts.Comments {
		if tally.PostID == postID {
			return tally.Count
		}
	}
	return 0
}

// wirePostComment is _wire_post_comment: PostCommentResponse.
func wirePostComment(comment repo.PostComment) *pyjson.OrderedMap {
	out := pyjson.NewOrderedMap()
	out.Set("id", pyjson.String(comment.ID))
	out.Set("post_id", pyjson.String(comment.PostID))
	out.Set("author_id", pyjson.String(comment.AuthorID))
	out.Set("author_display_name", pyjson.String(comment.AuthorDisplayName))
	out.Set("body", pyjson.String(comment.Body))
	out.Set("created_at", pyjson.String(pyjson.DateTime(comment.CreatedAt.UTC())))
	return out
}

// postDict is _post_dict.
func postDict(record repo.Post) postaudience.Post {
	return postaudience.Post{AuthorID: record.AuthorID, Audience: record.Audience, ContextID: record.ContextID}
}

// intParam reads a query int pyval validated, its default applied.
func intParam(call *endpoint.Call, name string) (int, error) {
	value, ok := call.Values[name].(pyjson.Int)
	if !ok {
		return 0, fmt.Errorf("routes: parameter %q is %T, not an int", name, call.Values[name])
	}
	big := value.Big()
	if !big.IsInt64() || big.Int64() > math.MaxInt32 || big.Int64() < math.MinInt32 {
		return 0, fmt.Errorf("routes: parameter %q is out of range: %s", name, big)
	}
	return int(big.Int64()), nil
}

// optionalStringParam reads a `str | None` path or query value.
func optionalStringParam(call *endpoint.Call, name string) (*string, error) {
	switch value := call.Values[name].(type) {
	case pyjson.Null:
		return nil, nil
	case pyjson.String:
		text := string(value)
		return &text, nil
	}
	return nil, fmt.Errorf("routes: parameter %q is %T, not a str or None", name, call.Values[name])
}

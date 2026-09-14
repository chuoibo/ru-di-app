package routes

import (
	"context"
	"errors"
	"strings"
	"time"

	"mobile/services/core/internal/domain/blocking"
	"mobile/services/core/internal/domain/photoref"
	"mobile/services/core/internal/domain/storyvisibility"
	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/repo"
	"mobile/services/core/internal/service"
)

// createStory is POST /stories (routes/stories.py create_story,
// ApiService.create_story): the permission, then the url's shape, then its
// owner, then the photograph's row, then the caption. The stored image_url is
// the canonical spelling photoref gives, whatever form the client sent.
func createStory() Route {
	return Route{ID: "POST /stories", Status: 201, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		if err := requireFacts(call, "create_story", map[string]bool{"is_self": true}); err != nil {
			return endpoint.Reply{}, err
		}
		request, err := bodyModel(call, "request")
		if err != nil {
			return endpoint.Reply{}, err
		}
		imageURL, err := stringField(request, "image_url")
		if err != nil {
			return endpoint.Reply{}, err
		}
		requested, err := optionalStringField(request, "caption")
		if err != nil {
			return endpoint.Reply{}, err
		}
		ref, err := photoref.Parse(imageURL)
		var malformed *photoref.PhotoURLError
		if errors.As(err, &malformed) {
			return endpoint.Reply{}, endpoint.Refuse(422, "photo_url_invalid", "image_url is not a photo of this product")
		}
		if err != nil {
			return endpoint.Reply{}, err
		}
		if ref.OwnerKind != photoref.OwnerPerson || ref.OwnerID != call.Actor.ID {
			return endpoint.Reply{}, endpoint.Refuse(403, "permission_denied", "Chỉ đăng được ảnh của chính mình.")
		}
		tx, err := call.Unit.Tx(ctx)
		if err != nil {
			return endpoint.Reply{}, err
		}
		store := repo.Repository{Q: tx}
		image, err := store.GetPersonImage(ctx, call.Actor.ID, ref.PhotoID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if image == nil {
			return endpoint.Reply{}, endpoint.Refuse(404, "photo_not_found", "Photo does not exist")
		}
		// `request.caption or None`: an empty caption is no caption.
		var caption *string
		if requested != nil && *requested != "" {
			caption = requested
		}
		var tooLong *storyvisibility.StoryError
		if err := storyvisibility.CheckCaption(caption); errors.As(err, &tooLong) {
			return endpoint.Reply{}, endpoint.Refuse(422, strings.ToLower(tooLong.Code), "Chú thích dài quá 200 ký tự.")
		} else if err != nil {
			return endpoint.Reply{}, err
		}
		now := time.Now().UTC()
		expires, err := storyvisibility.ExpiresAtFor(now)
		if err != nil {
			return endpoint.Reply{}, err
		}
		record, err := store.CreateStory(ctx, repo.StoryInput{
			AuthorID: call.Actor.ID, ImageURL: ref.URL(), Caption: caption,
			Audience: storyvisibility.DefaultStoryAudience, Now: now, ExpiresAt: expires,
		})
		if err != nil {
			return endpoint.Reply{}, err
		}
		return endpoint.Reply{Body: wireStory(record)}, nil
	}}
}

// listStories is GET /stories (routes/stories.py list_stories,
// ApiService.list_stories): every live story the actor may see, one rail group
// per author in first-appearance order, then ordered by
// storyvisibility.OrderAuthors. Friendship and blocking are read once per new
// author, before visibility is decided, as the Python loop does.
func listStories() Route {
	return Route{ID: "GET /stories", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		now := time.Now().UTC()
		tx, err := call.Unit.Tx(ctx)
		if err != nil {
			return endpoint.Reply{}, err
		}
		store := repo.Repository{Q: tx}
		rows, err := store.ListLiveStoriesFor(ctx, call.Actor.ID, now)
		if err != nil {
			return endpoint.Reply{}, err
		}
		type railGroup struct {
			authorID, authorName string
			stories              pyjson.List
			mine, allSeen        bool
			latestAt             time.Time
		}
		friends := map[string]bool{}
		blocked := map[string]bool{}
		var groups []*railGroup
		index := map[string]int{}
		for _, record := range rows {
			if _, seen := friends[record.AuthorID]; !seen {
				friend, err := isFriend(ctx, store, call.Actor.ID, record.AuthorID)
				if err != nil {
					return endpoint.Reply{}, err
				}
				friends[record.AuthorID] = friend
				block, err := isBlockedWith(ctx, store, call.Actor.ID, record.AuthorID)
				if err != nil {
					return endpoint.Reply{}, err
				}
				blocked[record.AuthorID] = block
			}
			if !storyvisibility.CanView(storyStory(record), call.Actor.ID,
				friends[record.AuthorID], blocked[record.AuthorID], now) {
				continue
			}
			i, ok := index[record.AuthorID]
			if !ok {
				groups = append(groups, &railGroup{
					authorID: record.AuthorID, authorName: record.AuthorDisplayName,
					stories: pyjson.List{}, mine: record.AuthorID == call.Actor.ID,
					allSeen: true, latestAt: record.CreatedAt,
				})
				i = len(groups) - 1
				index[record.AuthorID] = i
			}
			group := groups[i]
			group.stories = append(group.stories, wireStory(record))
			group.allSeen = group.allSeen && record.Seen
			// max() keeps the first of two equal instants.
			if record.CreatedAt.After(group.latestAt) {
				group.latestAt = record.CreatedAt
			}
		}
		ordered := storyvisibility.OrderAuthors(groups, func(g *railGroup) storyvisibility.AuthorFacts {
			return storyvisibility.AuthorFacts{Mine: g.mine, AllSeen: g.allSeen, LatestAt: g.latestAt}
		})
		authors := pyjson.List{}
		for _, group := range ordered {
			author := pyjson.NewOrderedMap()
			author.Set("id", pyjson.String(group.authorID))
			author.Set("display_name", pyjson.String(group.authorName))
			feed := pyjson.NewOrderedMap()
			feed.Set("author", author)
			feed.Set("stories", group.stories)
			feed.Set("all_seen", pyjson.Bool(group.allSeen))
			authors = append(authors, feed)
		}
		body := pyjson.NewOrderedMap()
		body.Set("authors", authors)
		return endpoint.Reply{Body: body}, nil
	}}
}

// markStorySeen is POST /stories/{story_id}/seen (mark_story_seen): the
// stored seen_at, so a second look answers the first look's time.
func markStorySeen() Route {
	return Route{ID: "POST /stories/{story_id}/seen", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		store, record, err := viewableStoryOr404(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		seenAt, err := store.MarkStorySeen(ctx, record.ID, call.Actor.ID, time.Now().UTC())
		if err != nil {
			return endpoint.Reply{}, err
		}
		body := pyjson.NewOrderedMap()
		body.Set("story_id", pyjson.String(record.ID))
		body.Set("seen_at", pyjson.String(pyjson.DateTime(seenAt.UTC())))
		return endpoint.Reply{Body: body}, nil
	}}
}

// deleteStory is DELETE /stories/{story_id} (delete_story): a reader who may
// view the story but did not write it gets 403; everybody else the same 404.
// The route answers `Response(status_code=204)`: no body, no content headers.
func deleteStory() Route {
	return Route{ID: "DELETE /stories/{story_id}", Status: 204, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		store, record, err := viewableStoryOr404(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if err := requireFacts(call, "delete_own_story", map[string]bool{"is_author": record.AuthorID == call.Actor.ID}); err != nil {
			return endpoint.Reply{}, err
		}
		if err := store.DeleteStory(ctx, record.ID); err != nil {
			return endpoint.Reply{}, err
		}
		return endpoint.Reply{Empty: true}, nil
	}}
}

// viewableStoryOr404 is _viewable_story_or_404: one 404 for «no such story»
// and «not for you», including a view_story refusal.
func viewableStoryOr404(ctx context.Context, call *endpoint.Call) (repo.Repository, *repo.Story, error) {
	storyID, err := pathUUID(call, "story_id")
	if err != nil {
		return repo.Repository{}, nil, err
	}
	tx, err := call.Unit.Tx(ctx)
	if err != nil {
		return repo.Repository{}, nil, err
	}
	store := repo.Repository{Q: tx}
	notFound := endpoint.Refuse(404, "story_not_found", "Story does not exist")
	record, err := store.GetStory(ctx, storyID)
	if err != nil {
		return repo.Repository{}, nil, err
	}
	if record == nil {
		return repo.Repository{}, nil, notFound
	}
	now := time.Now().UTC()
	friend, err := isFriend(ctx, store, call.Actor.ID, record.AuthorID)
	if err != nil {
		return repo.Repository{}, nil, err
	}
	block, err := isBlockedWith(ctx, store, call.Actor.ID, record.AuthorID)
	if err != nil {
		return repo.Repository{}, nil, err
	}
	mayView := storyvisibility.CanView(storyStory(*record), call.Actor.ID, friend, block, now)
	refused, err := service.RequirePermission("view_story", *call.Actor,
		service.Resource{Proven: map[string]bool{"may_view_story": mayView}})
	if err != nil {
		return repo.Repository{}, nil, err
	}
	if refused != nil {
		return repo.Repository{}, nil, notFound
	}
	return store, record, nil
}

// isFriend is _is_friend: the pair's edge, accepted. It reads the edge even
// when both ids are the same person.
func isFriend(ctx context.Context, store repo.Repository, readerID, otherID string) (bool, error) {
	edge, err := store.GetFriendEdge(ctx, readerID, otherID)
	if err != nil {
		return false, err
	}
	return edge != nil && edge.State == "accepted", nil
}

// isBlockedWith is _is_blocked_with: no read for oneself, otherwise the same
// edge read again and blocking.IsBlocked on its state and decider.
func isBlockedWith(ctx context.Context, store repo.Repository, readerID, otherID string) (bool, error) {
	if readerID == otherID {
		return false, nil
	}
	edge, err := store.GetFriendEdge(ctx, readerID, otherID)
	if err != nil {
		return false, err
	}
	if edge == nil {
		return blocking.IsBlocked(nil), nil
	}
	return blocking.IsBlocked(&blocking.Edge{State: edge.State, DecidedByID: edge.DecidedByID}), nil
}

// storyStory is _story_dict: the three facts can_view reads.
func storyStory(record repo.Story) storyvisibility.Story {
	return storyvisibility.Story{AuthorID: record.AuthorID, Audience: record.Audience, ExpiresAt: record.ExpiresAt}
}

// wireStory is _wire_story: StoryResponse.
func wireStory(record repo.Story) *pyjson.OrderedMap {
	out := pyjson.NewOrderedMap()
	out.Set("id", pyjson.String(record.ID))
	out.Set("author_id", pyjson.String(record.AuthorID))
	out.Set("author_display_name", pyjson.String(record.AuthorDisplayName))
	out.Set("image_url", pyjson.String(record.ImageURL))
	out.Set("caption", textOrNull(record.Caption))
	out.Set("audience", pyjson.String(record.Audience))
	out.Set("created_at", pyjson.String(pyjson.DateTime(record.CreatedAt.UTC())))
	out.Set("expires_at", pyjson.String(pyjson.DateTime(record.ExpiresAt.UTC())))
	out.Set("seen", pyjson.Bool(record.Seen))
	return out
}

package routes

import (
	"context"
	"errors"
	"time"

	"mobile/services/core/internal/cursors"
	"mobile/services/core/internal/domain/contexts"
	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/repo"
)

// postContextMemory is POST /contexts/{context_id}/memories
// (routes/memories.py post_context_memory, ApiService.post_context_memory):
// membership, the photograph url must point into this group, the named place
// resolved from the catalogue (never trusted from the body), then the insert.
func postContextMemory() Route {
	return Route{ID: "POST /contexts/{context_id}/memories", Status: 201, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		contextID, err := pathUUID(call, "context_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		body, err := bodyModel(call, "request")
		if err != nil {
			return endpoint.Reply{}, err
		}
		imageURL, err := stringField(body, "image_url")
		if err != nil {
			return endpoint.Reply{}, err
		}
		caption, err := optionalStringField(body, "caption")
		if err != nil {
			return endpoint.Reply{}, err
		}
		placeID, err := optionalStringField(body, "place_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := groupStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if err := requireGroupMember(ctx, call, store, "post_group_memory", contextID); err != nil {
			return endpoint.Reply{}, err
		}
		if refused := contexts.RequirePhotoURLContext(contextID, &imageURL); refused != nil {
			return endpoint.Reply{}, endpoint.Refuse(refused.Status, refused.Code, refused.Detail)
		}
		in := repo.MemoryInput{ContextID: contextID, AuthorID: call.Actor.ID, ImageURL: imageURL, Caption: caption, Now: time.Now().UTC()}
		if placeID != nil {
			place, err := catalogPlace(ctx, store, *placeID)
			if err != nil {
				return endpoint.Reply{}, err
			}
			in.PlaceID, in.PlaceName = &place.ID, &place.Name
		}
		record, err := store.CreateMemory(ctx, in)
		if err != nil {
			return endpoint.Reply{}, err
		}
		return endpoint.Reply{Body: wireMemory(record)}, nil
	}}
}

// postContextCheckin is POST /contexts/{context_id}/checkins
// (post_context_checkin): the same permission as a photograph, the place
// resolved before the write, answered as a row of the wall.
func postContextCheckin() Route {
	return Route{ID: "POST /contexts/{context_id}/checkins", Status: 201, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		contextID, err := pathUUID(call, "context_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		body, err := bodyModel(call, "request")
		if err != nil {
			return endpoint.Reply{}, err
		}
		placeID, err := stringField(body, "place_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		caption, err := optionalStringField(body, "caption")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := groupStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if err := requireGroupMember(ctx, call, store, "post_group_memory", contextID); err != nil {
			return endpoint.Reply{}, err
		}
		place, err := catalogPlace(ctx, store, placeID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		record, err := store.CreateCheckin(ctx, repo.CheckinInput{
			ContextID: contextID, AuthorID: call.Actor.ID, PlaceID: place.ID, PlaceName: place.Name,
			Lat: place.Lat, Lng: place.Lng, Caption: caption, Now: time.Now().UTC(),
		})
		if err != nil {
			return endpoint.Reply{}, err
		}
		return endpoint.Reply{Body: wireMemory(record)}, nil
	}}
}

// listContextMemories is GET /contexts/{context_id}/memories
// (list_context_memories): membership before the cursor is decoded, then one
// page newest first, the reader's own hearts marked.
func listContextMemories() Route {
	return Route{ID: "GET /contexts/{context_id}/memories", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		contextID, err := pathUUID(call, "context_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		limit, err := intParam(call, "limit")
		if err != nil {
			return endpoint.Reply{}, err
		}
		before, err := optionalStringParam(call, "before")
		if err != nil {
			return endpoint.Reply{}, err
		}
		kind, err := optionalStringParam(call, "kind")
		if err != nil {
			return endpoint.Reply{}, err
		}
		placeID, err := optionalStringParam(call, "place_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := groupStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if err := requireGroupMember(ctx, call, store, "view_group_memories", contextID); err != nil {
			return endpoint.Reply{}, err
		}
		query := repo.MemoryQuery{Limit: limit, Kind: kind, PlaceID: placeID, ViewerID: &call.Actor.ID}
		if before != nil {
			position, err := cursors.DecodeCursor(*before)
			var bad *cursors.CursorError
			if errors.As(err, &bad) {
				return endpoint.Reply{}, endpoint.Refuse(422, "invalid_cursor", "Memory cursor is invalid")
			}
			if err != nil {
				return endpoint.Reply{}, err
			}
			query.Before = &repo.MemoryCursor{CreatedAt: position.CreatedAt.Time(), ID: position.MessageID}
		}
		page, err := store.ListMemories(ctx, contextID, query)
		if err != nil {
			return endpoint.Reply{}, err
		}
		memories := make(pyjson.List, len(page.Memories))
		for i, record := range page.Memories {
			memories[i] = wireMemory(record)
		}
		out := pyjson.NewOrderedMap()
		out.Set("context_id", pyjson.String(contextID))
		out.Set("memories", memories)
		if n := len(page.Memories); n > 0 {
			last := page.Memories[n-1]
			out.Set("next_cursor", pyjson.String(cursors.EncodeCursor(last.CreatedAt, last.ID)))
		} else {
			out.Set("next_cursor", pyjson.Null{})
		}
		out.Set("has_more", pyjson.Bool(page.HasMore))
		return endpoint.Reply{Body: out}, nil
	}}
}

// readContextWidget is GET /contexts/{context_id}/widget (read_context_widget):
// the wall narrowed to its newest photograph, behind the wall's own permission.
func readContextWidget() Route {
	return Route{ID: "GET /contexts/{context_id}/widget", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		contextID, err := pathUUID(call, "context_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := groupStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if err := requireGroupMember(ctx, call, store, "view_group_memories", contextID); err != nil {
			return endpoint.Reply{}, err
		}
		photo := "photo"
		page, err := store.ListMemories(ctx, contextID, repo.MemoryQuery{Limit: 1, Kind: &photo})
		if err != nil {
			return endpoint.Reply{}, err
		}
		out := pyjson.NewOrderedMap()
		out.Set("context_id", pyjson.String(contextID))
		if len(page.Memories) == 0 || page.Memories[0].ImageURL == nil {
			out.Set("photo", pyjson.Null{})
			return endpoint.Reply{Body: out}, nil
		}
		newest := page.Memories[0]
		author, err := store.GetPerson(ctx, newest.AuthorID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		authorName := contexts.WidgetAuthorFallback
		if author != nil {
			authorName = author.DisplayName
		}
		wire := pyjson.NewOrderedMap()
		wire.Set("memory_id", pyjson.String(newest.ID))
		wire.Set("image_url", pyjson.String(*newest.ImageURL))
		wire.Set("caption", textOrNull(newest.Caption))
		wire.Set("author_id", pyjson.String(newest.AuthorID))
		wire.Set("author_name", pyjson.String(authorName))
		wire.Set("created_at", pyjson.String(pyjson.DateTime(newest.CreatedAt.UTC())))
		out.Set("photo", wire)
		return endpoint.Reply{Body: out}, nil
	}}
}

// postMemoryReaction is POST /contexts/{context_id}/memories/{memory_id}/reactions
// (react_to_memory): membership before the memory is looked up, one heart per
// person, the count read back from the rows after the write.
func postMemoryReaction() Route {
	return Route{ID: "POST /contexts/{context_id}/memories/{memory_id}/reactions", Status: 201, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		store, contextID, memoryID, err := memoryOfMember(ctx, call, "post_group_memory")
		if err != nil {
			return endpoint.Reply{}, err
		}
		record, err := store.AddMemoryReaction(ctx, memoryID, call.Actor.ID, time.Now().UTC())
		var conflict *repo.Conflict
		if errors.As(err, &conflict) && conflict.Code == "ALREADY_REACTED" {
			return endpoint.Reply{}, endpoint.Refuse(409, "already_reacted", "This person has already reacted to this memory")
		}
		if err != nil {
			return endpoint.Reply{}, err
		}
		recount, err := store.GetContextMemory(ctx, contextID, memoryID, nil)
		if err != nil {
			return endpoint.Reply{}, err
		}
		var count int64
		if recount != nil {
			count = recount.ReactionCount
		}
		out := pyjson.NewOrderedMap()
		out.Set("id", pyjson.String(record.ID))
		out.Set("memory_id", pyjson.String(record.MemoryID))
		out.Set("person_id", pyjson.String(record.PersonID))
		out.Set("created_at", pyjson.String(pyjson.DateTime(record.CreatedAt.UTC())))
		out.Set("reaction_count", pyjson.NewInt(count))
		return endpoint.Reply{Body: out}, nil
	}}
}

// deleteMemoryReaction is DELETE .../memories/{memory_id}/reactions
// (unreact_to_memory): one's own heart only; 204 with no body.
func deleteMemoryReaction() Route {
	return Route{ID: "DELETE /contexts/{context_id}/memories/{memory_id}/reactions", Status: 204, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		store, _, memoryID, err := memoryOfMember(ctx, call, "post_group_memory")
		if err != nil {
			return endpoint.Reply{}, err
		}
		removed, err := store.RemoveMemoryReaction(ctx, memoryID, call.Actor.ID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if !removed {
			return endpoint.Reply{}, endpoint.Refuse(404, "reaction_not_found", "This person has not reacted to this memory")
		}
		return endpoint.Reply{Empty: true}, nil
	}}
}

// postMemoryComment is POST .../memories/{memory_id}/comments
// (post_memory_comment): the author is the actor, never a field.
func postMemoryComment() Route {
	return Route{ID: "POST /contexts/{context_id}/memories/{memory_id}/comments", Status: 201, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		body, err := bodyModel(call, "request")
		if err != nil {
			return endpoint.Reply{}, err
		}
		text, err := stringField(body, "body")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, _, memoryID, err := memoryOfMember(ctx, call, "post_group_memory")
		if err != nil {
			return endpoint.Reply{}, err
		}
		record, err := store.CreateMemoryComment(ctx, memoryID, call.Actor.ID, text, time.Now().UTC())
		if err != nil {
			return endpoint.Reply{}, err
		}
		out, err := wireMemoryComment(ctx, store, record)
		if err != nil {
			return endpoint.Reply{}, err
		}
		return endpoint.Reply{Body: out}, nil
	}}
}

// listMemoryComments is GET .../memories/{memory_id}/comments
// (list_memory_comments): oldest first, behind the wall's permission.
func listMemoryComments() Route {
	return Route{ID: "GET /contexts/{context_id}/memories/{memory_id}/comments", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		store, _, memoryID, err := memoryOfMember(ctx, call, "view_group_memories")
		if err != nil {
			return endpoint.Reply{}, err
		}
		records, err := store.ListMemoryComments(ctx, memoryID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		comments := make(pyjson.List, len(records))
		for i, record := range records {
			wire, err := wireMemoryComment(ctx, store, record)
			if err != nil {
				return endpoint.Reply{}, err
			}
			comments[i] = wire
		}
		out := pyjson.NewOrderedMap()
		out.Set("memory_id", pyjson.String(memoryID))
		out.Set("comments", comments)
		return endpoint.Reply{Body: out}, nil
	}}
}

// memoryOfMember is _memory_of_member: permission first, then the memory of
// this group, so a stranger gets the same 403 whether the memory exists.
func memoryOfMember(ctx context.Context, call *endpoint.Call, action string) (repo.Repository, string, string, error) {
	contextID, err := pathUUID(call, "context_id")
	if err != nil {
		return repo.Repository{}, "", "", err
	}
	memoryID, err := pathUUID(call, "memory_id")
	if err != nil {
		return repo.Repository{}, "", "", err
	}
	store, err := groupStore(ctx, call)
	if err != nil {
		return repo.Repository{}, "", "", err
	}
	if err := requireGroupMember(ctx, call, store, action, contextID); err != nil {
		return repo.Repository{}, "", "", err
	}
	memory, err := store.GetContextMemory(ctx, contextID, memoryID, nil)
	if err != nil {
		return repo.Repository{}, "", "", err
	}
	if memory == nil {
		return repo.Repository{}, "", "", endpoint.Refuse(404, "memory_not_found", "Memory does not exist")
	}
	return store, contextID, memoryID, nil
}

// catalogPlace is place_row for a request with no preloaded catalogue: the
// repository's row, or 422 place_not_found without echoing the id.
func catalogPlace(ctx context.Context, store repo.Repository, placeID string) (*repo.Place, error) {
	place, err := store.GetPlace(ctx, placeID)
	if err != nil {
		return nil, err
	}
	if place == nil {
		return nil, endpoint.Refuse(422, "place_not_found", "No place in the catalogue has that id")
	}
	return place, nil
}

// wireMemory is _wire_memory: MemoryResponse, with the keyset cursor of the row.
func wireMemory(record repo.Memory) *pyjson.OrderedMap {
	out := pyjson.NewOrderedMap()
	out.Set("id", pyjson.String(record.ID))
	out.Set("context_id", pyjson.String(record.ContextID))
	out.Set("author_id", pyjson.String(record.AuthorID))
	out.Set("kind", pyjson.String(record.Kind))
	out.Set("image_url", textOrNull(record.ImageURL))
	out.Set("caption", textOrNull(record.Caption))
	out.Set("place_id", textOrNull(record.PlaceID))
	out.Set("place_name", textOrNull(record.PlaceName))
	out.Set("lat", floatOrNull(record.Lat))
	out.Set("lng", floatOrNull(record.Lng))
	out.Set("created_at", pyjson.String(pyjson.DateTime(record.CreatedAt.UTC())))
	out.Set("cursor", pyjson.String(cursors.EncodeCursor(record.CreatedAt, record.ID)))
	out.Set("reaction_count", pyjson.NewInt(record.ReactionCount))
	out.Set("comment_count", pyjson.NewInt(record.CommentCount))
	out.Set("viewer_has_reacted", pyjson.Bool(record.ViewerHasReacted))
	return out
}

// wireMemoryComment is _wire_memory_comment: the author's current name, or
// null when the author has no row.
func wireMemoryComment(ctx context.Context, store repo.Repository, record repo.MemoryComment) (*pyjson.OrderedMap, error) {
	person, err := store.GetPerson(ctx, record.AuthorID)
	if err != nil {
		return nil, err
	}
	var name *string
	if person != nil {
		name = &person.DisplayName
	}
	out := pyjson.NewOrderedMap()
	out.Set("id", pyjson.String(record.ID))
	out.Set("memory_id", pyjson.String(record.MemoryID))
	out.Set("author_id", pyjson.String(record.AuthorID))
	out.Set("display_name", textOrNull(name))
	out.Set("body", pyjson.String(record.Body))
	out.Set("created_at", pyjson.String(pyjson.DateTime(record.CreatedAt.UTC())))
	return out, nil
}

func floatOrNull(value *float64) pyjson.Value {
	if value == nil {
		return pyjson.Null{}
	}
	return pyjson.Float(*value)
}

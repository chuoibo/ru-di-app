package routes

import (
	"context"
	"time"

	"mobile/services/core/internal/brain"
	"mobile/services/core/internal/domain/album"
	"mobile/services/core/internal/domain/reel"
	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/repo"
	"mobile/services/core/internal/treejson"
)

const albumMemoryLimit = 400

func listTripAlbums() Route {
	return Route{ID: "GET /contexts/{context_id}/albums", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		contextID, err := pathUUID(call, "context_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := groupStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if err := requireGroupMember(ctx, call, store, "view_trip_album", contextID); err != nil {
			return endpoint.Reply{}, err
		}
		today := repo.WallClockDate(time.Now().UTC())
		rows, err := store.GroupRecap(ctx, contextID, today)
		if err != nil {
			return endpoint.Reply{}, err
		}
		list := pyjson.List{}
		for _, record := range rows {
			built, err := albumOf(ctx, store, record, call.Actor.ID)
			if err != nil {
				return endpoint.Reply{}, err
			}
			list = append(list, wireAlbumSummary(record, built))
		}
		body := pyjson.NewOrderedMap()
		body.Set("context_id", pyjson.String(contextID))
		body.Set("albums", list)
		return endpoint.Reply{Body: body}, nil
	}}
}

func readTripAlbum() Route {
	return Route{ID: "GET /contexts/{context_id}/albums/{outing_id}", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		found, built, err := loadTripAlbum(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		body := wireAlbum(found.contextID, found.row, built)
		return endpoint.Reply{Body: body}, nil
	}}
}

func readTripReel() Route {
	return Route{ID: "GET /contexts/{context_id}/albums/{outing_id}/reel", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		if err := spendActorWindow(call, call.Limits.ReelLimiter); err != nil {
			return endpoint.Reply{}, err
		}
		found, _, err := loadTripAlbum(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := groupStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		rows, err := store.ListOutingMemories(ctx, found.row.Outing.ID, albumMemoryLimit, &call.Actor.ID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		silent := func(reason string) endpoint.Reply {
			body := pyjson.NewOrderedMap()
			body.Set("context_id", pyjson.String(found.contextID))
			body.Set("outing_id", pyjson.String(found.row.Outing.ID))
			body.Set("reeled", pyjson.Bool(false))
			body.Set("reason", pyjson.String(reason))
			body.Set("source", pyjson.String("none"))
			body.Set("title", pyjson.Null{})
			body.Set("picks", pyjson.List{})
			body.Set("considered_count", pyjson.NewInt(int64(len(rows))))
			return endpoint.Reply{Body: body}
		}
		if len(rows) == 0 {
			return silent("no_memories"), nil
		}
		offered := make([]reel.Memory, 0, len(rows))
		memories := pyjson.List{}
		for _, memory := range rows {
			created := pyjson.DateTime(memory.CreatedAt.UTC())
			offered = append(offered, reel.Memory{
				ID: memory.ID, ImageURL: memory.ImageURL, Caption: memory.Caption, PlaceName: memory.PlaceName,
				CreatedAt: created, ReactionCount: memory.ReactionCount, CommentCount: memory.CommentCount,
			})
			entry := pyjson.NewOrderedMap()
			entry.Set("id", pyjson.String(memory.ID))
			entry.Set("kind", pyjson.String(memory.Kind))
			entry.Set("caption", textOrNull(memory.Caption))
			entry.Set("place_name", textOrNull(memory.PlaceName))
			entry.Set("created_at", pyjson.String(created))
			entry.Set("reaction_count", pyjson.NewInt(memory.ReactionCount))
			entry.Set("comment_count", pyjson.NewInt(memory.CommentCount))
			memories = append(memories, entry)
		}
		trip := pyjson.NewOrderedMap()
		trip.Set("title", pyjson.String(found.row.Outing.Title))
		trip.Set("starts_on", pyjson.String(pyjson.Date(found.row.Outing.StartsOn)))
		trip.Set("ends_on", pyjson.String(pyjson.Date(found.row.Outing.EndsOn)))
		trip.Set("headcount", pyjson.NewInt(found.row.Outing.Headcount))
		payload := pyjson.NewOrderedMap()
		payload.Set("trip", trip)
		payload.Set("memories", memories)
		raw, err := brain.Configured().PostJSON("reel", payload)
		if err != nil {
			return silent("unavailable"), nil
		}
		obj, err := brain.AsObject(raw)
		if err != nil {
			return silent("unavailable"), nil
		}
		cardValue, _ := obj.Get("card")
		// `Null{}` is not a Go nil; without this the fallback never fires and a
		// model that answered null would be ground as if it were a card.
		if pyjson.IsNull(cardValue) {
			cardValue = obj
		}
		grounded, err := reel.Ground(treejson.To(cardValue), offered)
		if err != nil {
			return silent("ungrounded"), nil
		}
		titleVal, _ := grounded.Get("title")
		picksVal, _ := grounded.Get("picks")
		// `From` already collapses both a Go nil and a tree Null into
		// `pyjson.Null{}`, so ask the converted value rather than the tree one.
		wiredPicks := treejson.From(picksVal)
		if pyjson.IsNull(wiredPicks) {
			wiredPicks = pyjson.List{}
		}
		body := pyjson.NewOrderedMap()
		body.Set("context_id", pyjson.String(found.contextID))
		body.Set("outing_id", pyjson.String(found.row.Outing.ID))
		body.Set("reeled", pyjson.Bool(true))
		body.Set("reason", pyjson.String("ok"))
		body.Set("source", pyjson.String("ai"))
		body.Set("title", treejson.From(titleVal))
		body.Set("picks", wiredPicks)
		body.Set("considered_count", pyjson.NewInt(int64(len(rows))))
		return endpoint.Reply{Body: body}, nil
	}}
}

type loadedAlbum struct {
	contextID string
	row       repo.RecapOuting
}

func loadTripAlbum(ctx context.Context, call *endpoint.Call) (loadedAlbum, album.Built, error) {
	contextID, err := pathUUID(call, "context_id")
	if err != nil {
		return loadedAlbum{}, album.Built{}, err
	}
	outingID, err := pathUUID(call, "outing_id")
	if err != nil {
		return loadedAlbum{}, album.Built{}, err
	}
	store, err := groupStore(ctx, call)
	if err != nil {
		return loadedAlbum{}, album.Built{}, err
	}
	if err := requireGroupMember(ctx, call, store, "view_trip_album", contextID); err != nil {
		return loadedAlbum{}, album.Built{}, err
	}
	today := repo.WallClockDate(time.Now().UTC())
	rows, err := store.GroupRecap(ctx, contextID, today)
	if err != nil {
		return loadedAlbum{}, album.Built{}, err
	}
	for _, record := range rows {
		if record.Outing.ID == outingID {
			built, err := albumOf(ctx, store, record, call.Actor.ID)
			if err != nil {
				return loadedAlbum{}, album.Built{}, err
			}
			return loadedAlbum{contextID: contextID, row: record}, built, nil
		}
	}
	return loadedAlbum{}, album.Built{}, endpoint.Refuse(404, "album_not_found", "Chuyến đi này không có ở đây.")
}

func albumOf(ctx context.Context, store repo.Repository, record repo.RecapOuting, actorID string) (album.Built, error) {
	rows, err := store.ListOutingMemories(ctx, record.Outing.ID, albumMemoryLimit, &actorID)
	if err != nil {
		return album.Built{}, err
	}
	memories := make([]album.Memory, 0, len(rows))
	for _, memory := range rows {
		memories = append(memories, album.Memory{
			ID: memory.ID, Kind: memory.Kind, ImageURL: memory.ImageURL, Caption: memory.Caption,
			PlaceID: memory.PlaceID, PlaceName: memory.PlaceName, CreatedAt: memory.CreatedAt,
			ReactionCount: memory.ReactionCount, CommentCount: memory.CommentCount,
		})
	}
	return album.Build(album.Outing{
		Title: record.Outing.Title, StartsOn: record.Outing.StartsOn, EndsOn: record.Outing.EndsOn,
		Headcount: record.Outing.Headcount, SplitTotalVND: record.SplitTotalVND, ExpenseCount: record.ExpenseCount,
	}, memories)
}

func wireAlbumPhoto(photo album.Photo) *pyjson.OrderedMap {
	out := pyjson.NewOrderedMap()
	out.Set("memory_id", pyjson.String(photo.MemoryID))
	out.Set("image_url", pyjson.String(photo.ImageURL))
	out.Set("caption", textOrNull(photo.Caption))
	out.Set("created_at", pyjson.String(pyjson.DateTime(photo.CreatedAt.UTC())))
	out.Set("reaction_count", pyjson.NewInt(photo.ReactionCount))
	out.Set("comment_count", pyjson.NewInt(photo.CommentCount))
	return out
}

func wireAlbumPhotos(photos []album.Photo) pyjson.List {
	list := pyjson.List{}
	for _, photo := range photos {
		list = append(list, wireAlbumPhoto(photo))
	}
	return list
}

func wireAlbumPlaces(places []album.Place) pyjson.List {
	list := pyjson.List{}
	for _, place := range places {
		entry := pyjson.NewOrderedMap()
		entry.Set("place_id", pyjson.String(place.PlaceID))
		entry.Set("place_name", textOrNull(place.PlaceName))
		list = append(list, entry)
	}
	return list
}

func wireAlbumSummary(record repo.RecapOuting, built album.Built) *pyjson.OrderedMap {
	out := pyjson.NewOrderedMap()
	out.Set("outing_id", pyjson.String(record.Outing.ID))
	out.Set("title", pyjson.String(built.Title))
	out.Set("period_label", pyjson.String(built.PeriodLabel))
	out.Set("starts_on", pyjson.String(pyjson.Date(record.Outing.StartsOn)))
	out.Set("ends_on", pyjson.String(pyjson.Date(record.Outing.EndsOn)))
	out.Set("in_progress", pyjson.Bool(record.InProgress))
	out.Set("photo_count", pyjson.NewInt(int64(built.PhotoCount)))
	out.Set("checkin_count", pyjson.NewInt(int64(built.CheckinCount)))
	out.Set("place_count", pyjson.NewInt(int64(built.PlaceCount)))
	out.Set("split_total_vnd", pyjson.NewInt(record.SplitTotalVND))
	out.Set("expense_count", pyjson.NewInt(record.ExpenseCount))
	out.Set("headcount", pyjson.NewInt(record.Outing.Headcount))
	if len(built.Photos) == 0 {
		out.Set("cover", pyjson.Null{})
	} else {
		out.Set("cover", wireAlbumPhoto(built.Photos[0]))
	}
	return out
}

func wireAlbum(contextID string, record repo.RecapOuting, built album.Built) *pyjson.OrderedMap {
	out := pyjson.NewOrderedMap()
	out.Set("context_id", pyjson.String(contextID))
	out.Set("outing_id", pyjson.String(record.Outing.ID))
	out.Set("title", pyjson.String(built.Title))
	out.Set("period_label", pyjson.String(built.PeriodLabel))
	out.Set("starts_on", pyjson.String(pyjson.Date(record.Outing.StartsOn)))
	out.Set("ends_on", pyjson.String(pyjson.Date(record.Outing.EndsOn)))
	out.Set("in_progress", pyjson.Bool(record.InProgress))
	out.Set("photos", wireAlbumPhotos(built.Photos))
	out.Set("photo_count", pyjson.NewInt(int64(built.PhotoCount)))
	out.Set("places", wireAlbumPlaces(built.Places))
	out.Set("place_count", pyjson.NewInt(int64(built.PlaceCount)))
	out.Set("checkin_count", pyjson.NewInt(int64(built.CheckinCount)))
	out.Set("highlights", wireAlbumPhotos(built.Highlights))
	out.Set("split_total_vnd", pyjson.NewInt(record.SplitTotalVND))
	out.Set("expense_count", pyjson.NewInt(record.ExpenseCount))
	out.Set("headcount", pyjson.NewInt(record.Outing.Headcount))
	return out
}

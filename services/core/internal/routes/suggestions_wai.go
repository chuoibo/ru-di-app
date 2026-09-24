package routes

import (
	"context"
	"time"

	"mobile/services/core/internal/brain"
	"mobile/services/core/internal/domain/conversation"
	"mobile/services/core/internal/domain/suggestion"
	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/repo"
	"mobile/services/core/internal/service"
	"mobile/services/core/internal/treejson"
)

const (
	suggestionHistoryLimit = 100
	conversationWindow     = 60
)

func readGroupSuggestion() Route {
	return Route{ID: "GET /contexts/{context_id}/suggestion", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		if err := spendActorWindow(call, call.Limits.SuggestionLimiter); err != nil {
			return endpoint.Reply{}, err
		}
		contextID, err := pathUUID(call, "context_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := groupStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if err := requireGroupMember(ctx, call, store, "view_group_suggestion", contextID); err != nil {
			return endpoint.Reply{}, err
		}
		today := repo.WallClockDate(time.Now().UTC())
		rows, err := store.GroupRecap(ctx, contextID, today)
		if err != nil {
			return endpoint.Reply{}, err
		}
		trips := make([]suggestion.Trip, 0, len(rows))
		for _, record := range rows {
			trips = append(trips, suggestion.Trip{
				Title: record.Outing.Title, SplitTotalVND: record.SplitTotalVND, Headcount: record.Outing.Headcount,
			})
		}
		group, err := service.GroupTaste(ctx, store, contextID, time.Now().UTC())
		if err != nil {
			return endpoint.Reply{}, err
		}
		places, err := service.ModelPlaceRows(ctx, store, group)
		if err != nil {
			return endpoint.Reply{}, err
		}
		categoryOf := map[string]string{}
		for _, place := range places {
			id := service.PlaceID(place)
			if value, ok := place.Get("category"); ok {
				if text, ok := value.(pyjson.String); ok {
					categoryOf[id] = string(text)
				}
			}
		}
		kind := "checkin"
		page, err := store.ListMemories(ctx, contextID, repo.MemoryQuery{Limit: suggestionHistoryLimit, Kind: &kind})
		if err != nil {
			return endpoint.Reply{}, err
		}
		categories := []string{}
		for _, memory := range page.Memories {
			if memory.PlaceID != nil {
				if category, ok := categoryOf[*memory.PlaceID]; ok && category != "" {
					categories = append(categories, category)
				}
			}
		}
		history, err := suggestion.SummariseHistory(trips, categories)
		if err != nil {
			return endpoint.Reply{}, err
		}
		basis := wireSuggestionBasis(history)
		silent := func(reason string) endpoint.Reply {
			body := pyjson.NewOrderedMap()
			body.Set("context_id", pyjson.String(contextID))
			body.Set("suggested", pyjson.Bool(false))
			body.Set("reason", pyjson.String(reason))
			body.Set("title", pyjson.Null{})
			body.Set("when_text", pyjson.Null{})
			body.Set("stops", pyjson.List{})
			body.Set("basis", basis)
			body.Set("source", pyjson.String("none"))
			return endpoint.Reply{Body: body}
		}
		if history.OutingCount == 0 {
			return silent("no_history"), nil
		}
		placeList := pyjson.List{}
		for _, place := range places {
			placeList = append(placeList, place)
		}
		payload := pyjson.NewOrderedMap()
		payload.Set("history", wireHistory(history))
		payload.Set("places", placeList)
		raw, err := brain.Configured().PostJSON("suggestion", payload)
		if err != nil {
			return silent("unavailable"), nil
		}
		obj, err := brain.AsObject(raw)
		if err != nil {
			return silent("unavailable"), nil
		}
		card, _ := obj.Get("card")
		// A decoded JSON null is `Null{}`, a struct, so `card == nil` is false
		// and the model saying "nothing" would reach grounding and answer
		// "ungrounded" where Python stops at None and answers "unavailable".
		if pyjson.IsNull(card) {
			return silent("unavailable"), nil
		}
		grounded, err := suggestion.Ground(treejson.To(card), treejson.MapsTo(places))
		if err != nil {
			return silent("ungrounded"), nil
		}
		return endpoint.Reply{Body: wireSuggestion(contextID, treejson.MapFrom(grounded), basis)}, nil
	}}
}

func readContextualSuggestion() Route {
	return Route{ID: "GET /contexts/{context_id}/contextual-suggestion", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		if err := spendActorWindow(call, call.Limits.ContextualSuggestionLimiter); err != nil {
			return endpoint.Reply{}, err
		}
		contextID, err := pathUUID(call, "context_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := groupStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if err := requireGroupMember(ctx, call, store, "view_contextual_suggestion", contextID); err != nil {
			return endpoint.Reply{}, err
		}
		members, err := store.ListMembers(ctx, contextID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		active := 0
		for _, member := range members {
			if member.State == "active" {
				active++
			}
		}
		page, err := store.ListMessages(ctx, contextID, conversationWindow, nil, nil)
		if err != nil {
			return endpoint.Reply{}, err
		}
		rows := make([]conversation.Message, 0, len(page.Messages))
		for _, message := range page.Messages {
			rows = append(rows, conversation.Message{Kind: message.Kind, Body: message.Body, AuthorID: message.AuthorID})
		}
		digest := conversation.Summarise(rows, active)
		basis := pyjson.NewOrderedMap()
		basis.Set("message_count", pyjson.NewInt(int64(digest.MessageCount)))
		basis.Set("speaker_count", pyjson.NewInt(int64(digest.SpeakerCount)))
		basis.Set("member_count", pyjson.NewInt(int64(digest.MemberCount)))
		silent := func(reason string) endpoint.Reply {
			body := pyjson.NewOrderedMap()
			body.Set("context_id", pyjson.String(contextID))
			body.Set("suggested", pyjson.Bool(false))
			body.Set("reason", pyjson.String(reason))
			body.Set("title", pyjson.Null{})
			body.Set("when_text", pyjson.Null{})
			body.Set("stops", pyjson.List{})
			body.Set("basis", basis)
			body.Set("source", pyjson.String("none"))
			return endpoint.Reply{Body: body}
		}
		if !conversation.Has(digest) {
			return silent("no_conversation"), nil
		}
		group, err := service.GroupTaste(ctx, store, contextID, time.Now().UTC())
		if err != nil {
			return endpoint.Reply{}, err
		}
		places, err := service.ModelPlaceRows(ctx, store, group)
		if err != nil {
			return endpoint.Reply{}, err
		}
		placeList := pyjson.List{}
		for _, place := range places {
			placeList = append(placeList, place)
		}
		digestMap := pyjson.NewOrderedMap()
		lines := pyjson.List{}
		for _, line := range digest.RecentLines {
			lines = append(lines, pyjson.String(line))
		}
		digestMap.Set("recent_lines", lines)
		digestMap.Set("message_count", pyjson.NewInt(int64(digest.MessageCount)))
		digestMap.Set("speaker_count", pyjson.NewInt(int64(digest.SpeakerCount)))
		digestMap.Set("member_count", pyjson.NewInt(int64(digest.MemberCount)))
		payload := pyjson.NewOrderedMap()
		payload.Set("digest", digestMap)
		payload.Set("places", placeList)
		raw, err := brain.Configured().PostJSON("contextual-suggestion", payload)
		if err != nil {
			return silent("unavailable"), nil
		}
		obj, err := brain.AsObject(raw)
		if err != nil {
			return silent("unavailable"), nil
		}
		card, _ := obj.Get("card")
		// A decoded JSON null is `Null{}`, a struct, so `card == nil` is false
		// and the model saying "nothing" would reach grounding and answer
		// "ungrounded" where Python stops at None and answers "unavailable".
		if pyjson.IsNull(card) {
			return silent("unavailable"), nil
		}
		grounded, err := suggestion.Ground(treejson.To(card), treejson.MapsTo(places))
		if err != nil {
			return silent("ungrounded"), nil
		}
		return endpoint.Reply{Body: wireSuggestion(contextID, treejson.MapFrom(grounded), basis)}, nil
	}}
}

func wireHistory(history suggestion.History) *pyjson.OrderedMap {
	out := pyjson.NewOrderedMap()
	out.Set("outing_count", pyjson.NewInt(int64(history.OutingCount)))
	out.Set("split_total_vnd", pyjson.NewInt(history.SplitTotalVND))
	if history.AvgPerPersonVND == nil {
		out.Set("avg_per_person_vnd", pyjson.Null{})
	} else {
		out.Set("avg_per_person_vnd", pyjson.NewInt(*history.AvgPerPersonVND))
	}
	cats := pyjson.List{}
	for _, category := range history.TopCategories {
		cats = append(cats, pyjson.String(category))
	}
	out.Set("top_categories", cats)
	titles := pyjson.List{}
	for _, title := range history.RecentTitles {
		titles = append(titles, pyjson.String(title))
	}
	out.Set("recent_titles", titles)
	return out
}

func wireSuggestionBasis(history suggestion.History) *pyjson.OrderedMap {
	return wireHistory(history)
}

func wireSuggestion(contextID string, grounded *pyjson.OrderedMap, basis *pyjson.OrderedMap) *pyjson.OrderedMap {
	payloadValue, _ := grounded.Get("payload")
	payload, _ := payloadValue.(*pyjson.OrderedMap)
	if payload == nil {
		payload = pyjson.NewOrderedMap()
	}
	title, _ := payload.Get("title")
	when, _ := payload.Get("when_text")
	stops, _ := payload.Get("stops")
	if pyjson.IsNull(stops) {
		stops = pyjson.List{}
	}
	out := pyjson.NewOrderedMap()
	out.Set("context_id", pyjson.String(contextID))
	out.Set("suggested", pyjson.Bool(true))
	out.Set("reason", pyjson.String("ok"))
	out.Set("title", title)
	out.Set("when_text", when)
	out.Set("stops", stops)
	out.Set("basis", basis)
	out.Set("source", pyjson.String("ai"))
	return out
}

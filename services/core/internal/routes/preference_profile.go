package routes

import (
	"context"
	"time"

	"mobile/services/core/internal/domain/preferences"
	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/repo"
	"mobile/services/core/internal/service"
)

// profileHistoryLimit is PROFILE_HISTORY_LIMIT, which is SUGGESTION_HISTORY_LIMIT.
const profileHistoryLimit = 100

// preferenceProfile is GET /contexts/{context_id}/preference-profile
// (routes/preferences.py read_preference_profile, ApiService.preference_profile):
// what the group keeps choosing, derived on every request from check-ins and
// ledger-summed trips. The reads run in Python's order: membership, the full
// catalogue, check-ins, then the recap.
func preferenceProfile() Route {
	return Route{ID: "GET /contexts/{context_id}/preference-profile", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		contextID, err := pathUUID(call, "context_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		tx, err := call.Unit.Tx(ctx)
		if err != nil {
			return endpoint.Reply{}, err
		}
		store := repo.Repository{Q: tx}
		member, err := store.IsMember(ctx, contextID, call.Actor.ID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		refused, err := service.RequirePermission("view_group_preference_profile", *call.Actor,
			service.Resource{Proven: map[string]bool{"is_group_member": member}})
		if err != nil {
			return endpoint.Reply{}, err
		}
		if refused != nil {
			return endpoint.Reply{}, &endpoint.Refusal{Problem: *refused}
		}

		places, err := store.ListPlaces(ctx, repo.PlaceFilter{})
		if err != nil {
			return endpoint.Reply{}, err
		}
		catalogue := make(map[string]repo.Place, len(places))
		for _, place := range places {
			catalogue[place.ID] = place
		}
		checkin := "checkin"
		page, err := store.ListMemories(ctx, contextID, repo.MemoryQuery{Limit: profileHistoryLimit, Kind: &checkin})
		if err != nil {
			return endpoint.Reply{}, err
		}
		var visits []preferences.Visit
		for _, memory := range page.Memories {
			if memory.PlaceID == nil {
				continue
			}
			place, known := catalogue[*memory.PlaceID]
			if !known {
				continue
			}
			visits = append(visits, preferences.Visit{Category: place.Category, Kinds: place.Kinds})
		}
		records, err := store.GroupRecap(ctx, contextID, repo.WallClockDate(time.Now()))
		if err != nil {
			return endpoint.Reply{}, err
		}
		trips := make([]preferences.Trip, 0, len(records))
		for _, record := range records {
			trips = append(trips, preferences.Trip{SplitTotalVND: record.SplitTotalVND, Headcount: record.Outing.Headcount})
		}
		// A PreferenceError is never caught in Python: an error here is a 500.
		profile, err := preferences.BuildPreferenceProfile(visits, trips)
		if err != nil {
			return endpoint.Reply{}, err
		}

		sections := pyjson.List{}
		for _, section := range profile.Sections {
			tastes := pyjson.List{}
			for _, taste := range section.Tastes {
				entry := pyjson.NewOrderedMap()
				entry.Set("label", pyjson.String(taste.Label))
				entry.Set("checkin_count", pyjson.NewInt(taste.CheckinCount))
				entry.Set("score", pyjson.Float(taste.Score))
				tastes = append(tastes, entry)
			}
			entry := pyjson.NewOrderedMap()
			entry.Set("section", pyjson.String(section.Section))
			entry.Set("taste_count", pyjson.NewInt(int64(section.TasteCount)))
			entry.Set("tastes", tastes)
			sections = append(sections, entry)
		}
		reason := "no_behaviour"
		if len(sections) > 0 {
			reason = "ok"
		}
		body := pyjson.NewOrderedMap()
		body.Set("context_id", pyjson.String(contextID))
		body.Set("has_profile", pyjson.Bool(len(sections) > 0))
		body.Set("reason", pyjson.String(reason))
		body.Set("sections", sections)
		body.Set("checkin_count", pyjson.NewInt(int64(profile.CheckinCount)))
		body.Set("outing_count", pyjson.NewInt(int64(profile.OutingCount)))
		body.Set("split_total_vnd", pyjson.NewInt(profile.SplitTotalVND))
		if profile.AvgPerPersonVND == nil {
			body.Set("avg_per_person_vnd", pyjson.Null{})
		} else {
			body.Set("avg_per_person_vnd", pyjson.NewInt(*profile.AvgPerPersonVND))
		}
		return endpoint.Reply{Body: body}, nil
	}}
}

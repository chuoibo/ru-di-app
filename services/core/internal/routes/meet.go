package routes

import (
	"context"
	"fmt"

	"mobile/services/core/internal/domain/areas"
	"mobile/services/core/internal/domain/meeting"
	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/repo"
	"mobile/services/core/internal/service"
)

// meetCandidates is _MEET_CANDIDATES.
const meetCandidates = 5

// postMeetingPoint is POST /contexts/{context_id}/meet (routes/social_map.py
// post_meeting_point, ApiService.get_meeting_point): district ids in, the
// fairest places out, no member named. Checks run in Python's order:
// membership, origin count, then each id, and only then the catalogue read.
func postMeetingPoint() Route {
	return Route{ID: "POST /contexts/{context_id}/meet", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
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
		refused, err := service.RequirePermission("view_meeting_point", *call.Actor,
			service.Resource{Proven: map[string]bool{"is_group_member": member}})
		if err != nil {
			return endpoint.Reply{}, err
		}
		if refused != nil {
			return endpoint.Reply{}, &endpoint.Refusal{Problem: *refused}
		}
		request, err := bodyModel(call, "request")
		if err != nil {
			return endpoint.Reply{}, err
		}
		ids, err := stringListField(request, "from_areas")
		if err != nil {
			return endpoint.Reply{}, err
		}
		if len(ids) < meeting.MinOriginAreas || len(ids) > meeting.MaxOriginAreas {
			return endpoint.Reply{}, endpoint.Refuse(422, "invalid_origin_count",
				fmt.Sprintf("Cần từ %d đến %d khu vực xuất phát để tìm điểm hẹn.", meeting.MinOriginAreas, meeting.MaxOriginAreas))
		}
		origins := make([]areas.Area, 0, len(ids))
		for _, id := range ids {
			area, ok := areas.Find(id)
			if !ok {
				return endpoint.Reply{}, endpoint.Refuse(422, "unknown_area", "Không có khu vực nào tên "+id+".")
			}
			origins = append(origins, area)
		}
		rows, err := store.ListPlaces(ctx, repo.PlaceFilter{})
		if err != nil {
			return endpoint.Reply{}, err
		}
		places := make([]meeting.Place, 0, len(rows))
		for _, row := range rows {
			// A place with no coordinates cannot be a meeting point: the whole
			// computation is a distance, and there is nothing to measure from.
			if row.Lat == nil || row.Lng == nil {
				continue
			}
			places = append(places, meeting.Place{ID: row.ID, Name: row.Name, Category: row.Category, Address: row.Address, Lat: *row.Lat, Lng: *row.Lng})
		}
		candidates, err := meeting.RankMeetingPoints(origins, places, meetCandidates)
		if err != nil {
			return endpoint.Reply{}, err
		}

		originList := pyjson.List{}
		for _, area := range origins {
			originList = append(originList, areaSummary(area.ID, area.Label, area.Lat, area.Lng))
		}
		candidateList := pyjson.List{}
		for _, candidate := range candidates {
			// MeetingCandidate.address is a StrictStr: a place without one fails
			// pydantic while the service builds the response, which is a 500.
			if candidate.Address == nil {
				return endpoint.Reply{}, fmt.Errorf("routes: meeting candidate %s has no address", candidate.PlaceID)
			}
			fairness := pyjson.NewOrderedMap()
			fairness.Set("worst_km", pyjson.Float(candidate.Fairness.WorstKm))
			fairness.Set("total_km", pyjson.Float(candidate.Fairness.TotalKm))
			fairness.Set("spread_km", pyjson.Float(candidate.Fairness.SpreadKm))
			travel := pyjson.List{}
			for _, leg := range candidate.Travel {
				entry := areaSummary(leg.ID, leg.Label, leg.Lat, leg.Lng)
				entry.Set("km", pyjson.Float(leg.Km))
				travel = append(travel, entry)
			}
			entry := pyjson.NewOrderedMap()
			entry.Set("place_id", pyjson.String(candidate.PlaceID))
			entry.Set("place_name", pyjson.String(candidate.PlaceName))
			entry.Set("category", pyjson.String(candidate.Category))
			entry.Set("address", pyjson.String(*candidate.Address))
			entry.Set("lat", pyjson.Float(candidate.Lat))
			entry.Set("lng", pyjson.Float(candidate.Lng))
			entry.Set("fairness", fairness)
			entry.Set("travel", travel)
			candidateList = append(candidateList, entry)
		}
		body := pyjson.NewOrderedMap()
		body.Set("context_id", pyjson.String(contextID))
		body.Set("origins", originList)
		body.Set("candidates", candidateList)
		body.Set("two_origin_inversion", pyjson.Bool(len(origins) == 2))
		return endpoint.Reply{Body: body}, nil
	}}
}

// areaSummary is AreaSummary, the shape MeetingLeg extends.
func areaSummary(id, label string, lat, lng float64) *pyjson.OrderedMap {
	entry := pyjson.NewOrderedMap()
	entry.Set("id", pyjson.String(id))
	entry.Set("label", pyjson.String(label))
	entry.Set("lat", pyjson.Float(lat))
	entry.Set("lng", pyjson.Float(lng))
	return entry
}

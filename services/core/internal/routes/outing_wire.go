package routes

import (
	"time"

	"mobile/services/core/internal/domain/itinerary"
	"mobile/services/core/internal/domain/journey"
	"mobile/services/core/internal/domain/outingsteps"
	"mobile/services/core/internal/pyjson"
)

var cacheControlNoStore = [][2]string{{"Cache-Control", "no-store"}}

func dateOrNull(d *outingsteps.Date) pyjson.Value {
	if d == nil {
		return pyjson.Null{}
	}
	return pyjson.String(d.ISOFormat())
}

func intOrNull(n *int64) pyjson.Value {
	if n == nil {
		return pyjson.Null{}
	}
	return pyjson.NewInt(*n)
}

func timeOrNull(t *time.Time) pyjson.Value {
	if t == nil {
		return pyjson.Null{}
	}
	return pyjson.String(pyjson.DateTime(t.UTC()))
}

func wireMeeting(point *outingsteps.MeetingPointView) pyjson.Value {
	if point == nil {
		return pyjson.Null{}
	}
	out := pyjson.NewOrderedMap()
	out.Set("lat", pyjson.Float(point.Lat))
	out.Set("lng", pyjson.Float(point.Lng))
	out.Set("label", pyjson.String(point.Label))
	return out
}

func wireStopView(stop outingsteps.StopView) *pyjson.OrderedMap {
	out := pyjson.NewOrderedMap()
	out.Set("id", pyjson.String(stop.ID))
	out.Set("position", pyjson.NewInt(int64(stop.Position)))
	out.Set("at", pyjson.String(stop.At))
	out.Set("label", pyjson.String(stop.Label))
	out.Set("place_name", textOrNull(stop.PlaceName))
	out.Set("place_id", textOrNull(stop.PlaceID))
	out.Set("day", dateOrNull(stop.Day))
	out.Set("duration_minutes", intOrNull(stop.DurationMinutes))
	out.Set("time_locked", pyjson.Bool(stop.TimeLocked))
	out.Set("meeting_point", wireMeeting(stop.MeetingPoint))
	return out
}

func wireDay(day outingsteps.Day) *pyjson.OrderedMap {
	out := pyjson.NewOrderedMap()
	out.Set("day", pyjson.String(day.Day.ISOFormat()))
	out.Set("transport_mode", pyjson.String(day.TransportMode))
	out.Set("start_at", pyjson.String(day.StartAt))
	out.Set("start_stop_id", textOrNull(day.StartStopID))
	out.Set("end_stop_id", textOrNull(day.EndStopID))
	out.Set("return_to_start", pyjson.Bool(day.ReturnToStart))
	return out
}

func wireOutingView(view outingsteps.OutingView) *pyjson.OrderedMap {
	stops := make(pyjson.List, len(view.Stops))
	for i, stop := range view.Stops {
		stops[i] = wireStopView(stop)
	}
	days := make(pyjson.List, len(view.Days))
	for i, day := range view.Days {
		days[i] = wireDay(day)
	}
	out := pyjson.NewOrderedMap()
	out.Set("id", pyjson.String(view.ID))
	out.Set("context_id", pyjson.String(view.ContextID))
	out.Set("created_by_id", pyjson.String(view.CreatedByID))
	out.Set("title", pyjson.String(view.Title))
	out.Set("starts_on", pyjson.String(view.StartsOn.ISOFormat()))
	out.Set("ends_on", pyjson.String(view.EndsOn.ISOFormat()))
	out.Set("headcount", pyjson.NewInt(view.Headcount))
	out.Set("budget_per_person_vnd", pyjson.NewInt(view.BudgetPerPersonVND))
	out.Set("created_at", pyjson.String(pyjson.DateTime(view.CreatedAt.UTC())))
	out.Set("stops", stops)
	out.Set("timeline_revision", pyjson.NewInt(view.TimelineRevision))
	out.Set("itinerary_version", pyjson.NewInt(view.ItineraryVersion))
	out.Set("days", days)
	return out
}

func wireOutingList(view outingsteps.OutingListView) *pyjson.OrderedMap {
	rows := make(pyjson.List, len(view.Outings))
	for i, outing := range view.Outings {
		rows[i] = wireOutingView(outing)
	}
	out := pyjson.NewOrderedMap()
	out.Set("context_id", pyjson.String(view.ContextID))
	out.Set("outings", rows)
	return out
}

func wireCheckinView(view outingsteps.CheckinView) *pyjson.OrderedMap {
	out := pyjson.NewOrderedMap()
	out.Set("id", pyjson.String(view.ID))
	out.Set("stop_id", pyjson.String(view.StopID))
	out.Set("person_id", pyjson.String(view.PersonID))
	out.Set("display_name", textOrNull(view.DisplayName))
	out.Set("created_at", pyjson.String(pyjson.DateTime(view.CreatedAt.UTC())))
	return out
}

func wireCheckinList(view outingsteps.CheckinListView) *pyjson.OrderedMap {
	rows := make(pyjson.List, len(view.Checkins))
	for i, row := range view.Checkins {
		rows[i] = wireCheckinView(row)
	}
	out := pyjson.NewOrderedMap()
	out.Set("outing_id", pyjson.String(view.OutingID))
	out.Set("checkins", rows)
	return out
}

func wireInviteView(view outingsteps.InviteView) *pyjson.OrderedMap {
	out := pyjson.NewOrderedMap()
	out.Set("id", pyjson.String(view.ID))
	out.Set("outing_id", pyjson.String(view.OutingID))
	out.Set("source", pyjson.String(view.Source))
	out.Set("invited_person_id", textOrNull(view.InvitedPersonID))
	out.Set("invited_by_id", pyjson.String(view.InvitedByID))
	out.Set("created_at", pyjson.String(pyjson.DateTime(view.CreatedAt.UTC())))
	out.Set("expires_at", pyjson.String(pyjson.DateTime(view.ExpiresAt.UTC())))
	out.Set("revoked_at", timeOrNull(view.RevokedAt))
	out.Set("invite_token", textOrNull(view.InviteToken))
	out.Set("invite_path", textOrNull(view.InvitePath))
	return out
}

func wireInviteAccept(view outingsteps.InviteAcceptView) *pyjson.OrderedMap {
	out := pyjson.NewOrderedMap()
	out.Set("invite_id", pyjson.String(view.InviteID))
	out.Set("outing_id", pyjson.String(view.OutingID))
	out.Set("context_id", pyjson.String(view.ContextID))
	out.Set("membership_id", pyjson.String(view.MembershipID))
	out.Set("membership_state", pyjson.String(view.MembershipState))
	return out
}

func wireIssues(issues []journey.Issue) pyjson.List {
	out := make(pyjson.List, 0, len(issues))
	for _, issue := range issues {
		row := pyjson.NewOrderedMap()
		row.Set("code", pyjson.String(issue.Code))
		row.Set("stop_id", textOrNull(issue.StopID))
		row.Set("message", pyjson.String(issue.Message))
		out = append(out, row)
	}
	return out
}

func wireGeometry(points [][2]float64) pyjson.List {
	out := make(pyjson.List, len(points))
	for i, point := range points {
		out[i] = pyjson.List{pyjson.Float(point[0]), pyjson.Float(point[1])}
	}
	return out
}

func wireScheduleRow(row journey.Row) *pyjson.OrderedMap {
	out := pyjson.NewOrderedMap()
	out.Set("id", pyjson.String(row.ID))
	out.Set("at", textOrNull(row.At))
	out.Set("arrival_at", textOrNull(row.ArrivalAt))
	out.Set("departure_at", textOrNull(row.DepartureAt))
	out.Set("wait_minutes", pyjson.NewInt(row.WaitMinutes))
	return out
}

func wireItineraryRoute(route *itinerary.Route) pyjson.Value {
	if route == nil {
		return pyjson.Null{}
	}
	stops := make(pyjson.List, len(route.Stops))
	for i, stop := range route.Stops {
		stops[i] = wireScheduleRow(stop)
	}
	segments := make(pyjson.List, len(route.Segments))
	for i, segment := range route.Segments {
		row := pyjson.NewOrderedMap()
		row.Set("distance_meters", pyjson.NewInt(segment.Leg.DistanceMeters))
		row.Set("duration_seconds", pyjson.NewInt(segment.Leg.DurationSeconds))
		row.Set("geometry", wireGeometry(segment.Leg.Geometry))
		row.Set("source", pyjson.String(segment.Leg.Source))
		row.Set("from_stop_id", pyjson.String(segment.FromStopID))
		row.Set("to_stop_id", pyjson.String(segment.ToStopID))
		segments[i] = row
	}
	out := pyjson.NewOrderedMap()
	out.Set("stops", stops)
	out.Set("feasible", pyjson.Bool(route.Feasible))
	out.Set("issues", wireIssues(route.Issues))
	out.Set("segments", segments)
	out.Set("distance_meters", pyjson.NewInt(route.DistanceMeters))
	out.Set("duration_seconds", pyjson.NewInt(route.DurationSeconds))
	return out
}

func wireItineraryPreview(preview itinerary.Preview) *pyjson.OrderedMap {
	source := pyjson.NewOrderedMap()
	source.Set("engine", pyjson.String(itinerary.Engine))
	source.Set("graph_version", textOrNull(preview.GraphVersion))
	source.Set("traffic", pyjson.String(itinerary.Traffic))
	var savings pyjson.Value = pyjson.Null{}
	if preview.Savings != nil {
		row := pyjson.NewOrderedMap()
		row.Set("distance_meters", pyjson.NewInt(preview.Savings.DistanceMeters))
		row.Set("duration_seconds", pyjson.NewInt(preview.Savings.DurationSeconds))
		savings = row
	}
	out := pyjson.NewOrderedMap()
	out.Set("revision", pyjson.NewInt(preview.Revision))
	out.Set("day", pyjson.String(preview.Day))
	out.Set("status", pyjson.String(preview.Status))
	out.Set("source", source)
	out.Set("current", wireItineraryRoute(preview.Current))
	out.Set("suggestion", wireItineraryRoute(preview.Suggestion))
	out.Set("savings", savings)
	out.Set("issues", wireIssues(preview.Issues))
	return out
}

package routes

import (
	"context"
	"fmt"
	"math"
	"time"
	"unicode/utf8"

	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/pyval"
	"mobile/services/core/internal/repo"
	"mobile/services/core/internal/service"
)

// groupRecap is GET /contexts/{context_id}/recap (routes/recap.py
// read_group_recap, ApiService.group_recap): finished trips and, separately,
// the one under way, with a total over the finished ones only.
func groupRecap() Route {
	return Route{ID: "GET /contexts/{context_id}/recap", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		contextID, err := pathUUID(call, "context_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		tx, err := call.Unit.Tx(ctx)
		if err != nil {
			return endpoint.Reply{}, err
		}
		store := repo.Repository{Q: tx}
		// Python evaluates is_member while building the permission context,
		// so the membership query runs before the decision, for everyone.
		member, err := store.IsMember(ctx, contextID, call.Actor.ID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		refused, err := service.RequirePermission("view_group_memories", *call.Actor,
			service.Resource{Proven: map[string]bool{"is_group_member": member}})
		if err != nil {
			return endpoint.Reply{}, err
		}
		if refused != nil {
			return endpoint.Reply{}, &endpoint.Refusal{Problem: *refused}
		}
		records, err := store.GroupRecap(ctx, contextID, repo.WallClockDate(time.Now()))
		if err != nil {
			return endpoint.Reply{}, err
		}
		outings, inProgress := pyjson.List{}, pyjson.List{}
		var total int64
		for _, record := range records {
			wire, err := recapOuting(record)
			if err != nil {
				return endpoint.Reply{}, err
			}
			if record.InProgress {
				inProgress = append(inProgress, wire)
				continue
			}
			outings = append(outings, wire)
			if (record.SplitTotalVND > 0 && total > math.MaxInt64-record.SplitTotalVND) ||
				(record.SplitTotalVND < 0 && total < math.MinInt64-record.SplitTotalVND) {
				return endpoint.Reply{}, fmt.Errorf("routes: recap total overflows int64")
			}
			total += record.SplitTotalVND
		}
		body := pyjson.NewOrderedMap()
		body.Set("context_id", pyjson.String(contextID))
		body.Set("outings", outings)
		body.Set("in_progress", inProgress)
		body.Set("split_total_vnd", pyjson.NewInt(total))
		return endpoint.Reply{Body: body}, nil
	}}
}

// recapOuting is _wire_recap_outing.
func recapOuting(record repo.RecapOuting) (*pyjson.OrderedMap, error) {
	stops := pyjson.List{}
	for _, stop := range record.Outing.Stops {
		wire, err := outingStop(stop)
		if err != nil {
			return nil, err
		}
		stops = append(stops, wire)
	}
	out := pyjson.NewOrderedMap()
	out.Set("outing_id", pyjson.String(record.Outing.ID))
	out.Set("title", pyjson.String(record.Outing.Title))
	out.Set("starts_on", pyjson.String(pyjson.Date(record.Outing.StartsOn)))
	out.Set("ends_on", pyjson.String(pyjson.Date(record.Outing.EndsOn)))
	out.Set("headcount", pyjson.NewInt(record.Outing.Headcount))
	out.Set("stops", stops)
	out.Set("split_total_vnd", pyjson.NewInt(record.SplitTotalVND))
	out.Set("expense_count", pyjson.NewInt(record.ExpenseCount))
	out.Set("memory_count", pyjson.NewInt(record.MemoryCount))
	return out, nil
}

// outingStop is the stops element of _wire_outing: OutingStopResponse.
func outingStop(stop repo.OutingStop) (*pyjson.OrderedMap, error) {
	out := pyjson.NewOrderedMap()
	out.Set("id", pyjson.String(stop.ID))
	out.Set("position", pyjson.NewInt(stop.Position))
	out.Set("at", pyjson.String(clock(stop.MinuteOfDay)))
	out.Set("label", pyjson.String(stop.Label))
	out.Set("place_name", textOrNull(stop.PlaceName))
	out.Set("place_id", textOrNull(stop.PlaceID))
	if stop.Day == nil {
		out.Set("day", pyjson.Null{})
	} else {
		out.Set("day", pyjson.String(pyjson.Date(*stop.Day)))
	}
	if stop.DurationMinutes == nil {
		out.Set("duration_minutes", pyjson.Null{})
	} else {
		out.Set("duration_minutes", pyjson.NewInt(*stop.DurationMinutes))
	}
	out.Set("time_locked", pyjson.Bool(stop.TimeLocked))
	if stop.MeetingLat == nil {
		out.Set("meeting_point", pyjson.Null{})
		return out, nil
	}
	point, err := meetingPoint(stop)
	if err != nil {
		return nil, err
	}
	out.Set("meeting_point", point)
	return out, nil
}

// meetingPoint builds MeetingPoint the way pydantic validates it when the
// service constructs the response: strict finite floats in range and a label
// of 1 to 200 code points. A stored point that fails is an unhandled
// ValidationError in Python, so it is an error here too.
func meetingPoint(stop repo.OutingStop) (*pyjson.OrderedMap, error) {
	lat, lng, label := stop.MeetingLat, stop.MeetingLng, stop.MeetingLabel
	switch {
	case lng == nil || label == nil:
		return nil, fmt.Errorf("routes: stop %s has a meeting latitude without longitude or label", stop.ID)
	case math.IsNaN(*lat) || math.IsInf(*lat, 0) || *lat < -90 || *lat > 90:
		return nil, fmt.Errorf("routes: stop %s meeting latitude out of range", stop.ID)
	case math.IsNaN(*lng) || math.IsInf(*lng, 0) || *lng < -180 || *lng > 180:
		return nil, fmt.Errorf("routes: stop %s meeting longitude out of range", stop.ID)
	case utf8.RuneCountInString(*label) < 1 || utf8.RuneCountInString(*label) > 200:
		return nil, fmt.Errorf("routes: stop %s meeting label length out of range", stop.ID)
	}
	out := pyjson.NewOrderedMap()
	out.Set("lat", pyjson.Float(*lat))
	out.Set("lng", pyjson.Float(*lng))
	out.Set("label", pyjson.String(*label))
	return out, nil
}

// clock is _clock: Python's floor division and modulo, so a negative minute
// formats as Python writes it.
func clock(minute int64) string {
	hours, rest := minute/60, minute%60
	if rest < 0 {
		rest += 60
		hours--
	}
	return fmt.Sprintf("%02d:%02d", hours, rest)
}

// pathUUID reads a UUID path parameter pyval validated.
func pathUUID(call *endpoint.Call, name string) (string, error) {
	id, ok := call.Values[name].(pyval.UUID)
	if !ok {
		return "", fmt.Errorf("routes: path parameter %q is %T, not a UUID", name, call.Values[name])
	}
	return id.String(), nil
}

package outingsteps

import (
	"errors"
	"fmt"
	"testing"

	"mobile/services/core/internal/domain/itinerary"
	"mobile/services/core/internal/domain/journey"
	"mobile/services/core/internal/domain/permissions"
	"mobile/services/core/internal/oracletest"
)

// --- request decoding --------------------------------------------------------

func (h *harness) stopInput(value any) (StopInput, error) {
	row, err := oracletest.Row(value, "at", "label", "place_name", "place_id")
	if err != nil {
		return StopInput{}, err
	}
	at, err := oracletest.Str(row["at"])
	if err != nil {
		return StopInput{}, err
	}
	label, err := oracletest.Str(row["label"])
	if err != nil {
		return StopInput{}, err
	}
	placeName, err := oracletest.OptionalString(row["place_name"])
	if err != nil {
		return StopInput{}, err
	}
	placeID, err := oracletest.OptionalString(row["place_id"])
	if err != nil {
		return StopInput{}, err
	}
	return StopInput{At: at, Label: label, PlaceName: placeName, PlaceID: placeID}, nil
}

func (h *harness) itineraryStopInput(value any) (ItineraryStopInput, error) {
	base, err := h.stopInput(value)
	if err != nil {
		return ItineraryStopInput{}, err
	}
	row, err := oracletest.Row(value, "id", "day", "duration_minutes", "time_locked", "meeting_point")
	if err != nil {
		return ItineraryStopInput{}, err
	}
	id, err := h.id(row["id"])
	if err != nil {
		return ItineraryStopInput{}, err
	}
	day, err := optionalParseDate(row["day"])
	if err != nil {
		return ItineraryStopInput{}, err
	}
	dwell, err := optionalNumber(row["duration_minutes"])
	if err != nil {
		return ItineraryStopInput{}, err
	}
	locked, err := oracletest.Bool(row["time_locked"])
	if err != nil {
		return ItineraryStopInput{}, err
	}
	stop := ItineraryStopInput{StopInput: base, ID: id, Day: day, DurationMinutes: dwell, TimeLocked: locked}
	if row["meeting_point"] != nil {
		point, err := oracletest.Row(row["meeting_point"], "lat", "lng", "label")
		if err != nil {
			return ItineraryStopInput{}, err
		}
		lat, err := optionalScalarFloat(point["lat"])
		if err != nil {
			return ItineraryStopInput{}, err
		}
		lng, err := optionalScalarFloat(point["lng"])
		if err != nil {
			return ItineraryStopInput{}, err
		}
		label, err := oracletest.Str(point["label"])
		if err != nil {
			return ItineraryStopInput{}, err
		}
		if lat == nil || lng == nil {
			return ItineraryStopInput{}, fmt.Errorf("a meeting point needs two floats")
		}
		stop.MeetingPoint = &MeetingPoint{Lat: *lat, Lng: *lng, Label: label}
	}
	return stop, nil
}

func (h *harness) itineraryRequest(value any, preview bool) (ItineraryRequest, error) {
	row, err := oracletest.Row(value, "expected_revision", "stops", "days")
	if err != nil {
		return ItineraryRequest{}, err
	}
	revision, err := oracletest.Int64(row["expected_revision"])
	if err != nil {
		return ItineraryRequest{}, err
	}
	request := ItineraryRequest{ExpectedRevision: revision}
	stops, err := oracletest.List(row["stops"])
	if err != nil {
		return ItineraryRequest{}, err
	}
	for _, raw := range stops {
		stop, err := h.itineraryStopInput(raw)
		if err != nil {
			return ItineraryRequest{}, err
		}
		request.Stops = append(request.Stops, stop)
	}
	days, err := oracletest.List(row["days"])
	if err != nil {
		return ItineraryRequest{}, err
	}
	for _, raw := range days {
		day, err := h.requestDayOf(raw)
		if err != nil {
			return ItineraryRequest{}, err
		}
		request.Days = append(request.Days, day)
	}
	if preview {
		day, err := parseDate(row["day"])
		if err != nil {
			return ItineraryRequest{}, err
		}
		request.Day = &day
		if request.IncludeSuggestion, err = oracletest.Bool(row["include_suggestion"]); err != nil {
			return ItineraryRequest{}, err
		}
	}
	return request, nil
}

// --- response encoding -------------------------------------------------------

func (h *harness) wireRecordStop(stop Stop) any {
	return map[string]any{
		"id":               h.name(stop.ID),
		"position":         int64(stop.Position),
		"minute_of_day":    stop.MinuteOfDay,
		"label":            stop.Label,
		"place_name":       optionalText(stop.PlaceName),
		"place_id":         optionalText(stop.PlaceID),
		"day":              optionalDate(stop.Day),
		"duration_minutes": optionalInt(stop.DurationMinutes),
		"time_locked":      stop.TimeLocked,
		"meeting_lat":      optionalFloatWire(stop.MeetingLat),
		"meeting_lng":      optionalFloatWire(stop.MeetingLng),
		"meeting_label":    optionalText(stop.MeetingLabel),
	}
}

func (h *harness) wireDays(days []Day) []any {
	out := make([]any, 0, len(days))
	for _, day := range days {
		out = append(out, h.wireDay(day))
	}
	return out
}

// wireOutingRecord is the dataclass dump authorize_outing_itinerary answers.
func (h *harness) wireOutingRecord(record Outing) any {
	stops := make([]any, 0, len(record.Stops))
	for _, stop := range record.Stops {
		stops = append(stops, h.wireRecordStop(stop))
	}
	return map[string]any{
		"id":                    h.name(record.ID),
		"context_id":            h.name(record.ContextID),
		"created_by_id":         h.name(record.CreatedByID),
		"title":                 record.Title,
		"starts_on":             record.StartsOn.ISOFormat(),
		"ends_on":               record.EndsOn.ISOFormat(),
		"headcount":             record.Headcount,
		"budget_per_person_vnd": record.BudgetPerPersonVND,
		"created_at":            iso(record.CreatedAt),
		"stops":                 stops,
		"timeline_revision":     record.TimelineRevision,
		"itinerary_version":     record.ItineraryVersion,
		"itinerary_days":        h.wireDays(record.ItineraryDays),
	}
}

func (h *harness) wireOutingView(view OutingView) any {
	stops := make([]any, 0, len(view.Stops))
	for _, stop := range view.Stops {
		var point any
		if stop.MeetingPoint != nil {
			point = map[string]any{
				"lat":   wireFloat(stop.MeetingPoint.Lat),
				"lng":   wireFloat(stop.MeetingPoint.Lng),
				"label": stop.MeetingPoint.Label,
			}
		}
		stops = append(stops, map[string]any{
			"id":               h.name(stop.ID),
			"position":         int64(stop.Position),
			"at":               stop.At,
			"label":            stop.Label,
			"place_name":       optionalText(stop.PlaceName),
			"place_id":         optionalText(stop.PlaceID),
			"day":              optionalDate(stop.Day),
			"duration_minutes": optionalInt(stop.DurationMinutes),
			"time_locked":      stop.TimeLocked,
			"meeting_point":    point,
		})
	}
	return map[string]any{
		"id":                    h.name(view.ID),
		"context_id":            h.name(view.ContextID),
		"created_by_id":         h.name(view.CreatedByID),
		"title":                 view.Title,
		"starts_on":             view.StartsOn.ISOFormat(),
		"ends_on":               view.EndsOn.ISOFormat(),
		"headcount":             view.Headcount,
		"budget_per_person_vnd": view.BudgetPerPersonVND,
		"created_at":            iso(view.CreatedAt),
		"stops":                 stops,
		"timeline_revision":     view.TimelineRevision,
		"itinerary_version":     view.ItineraryVersion,
		"days":                  h.wireDays(view.Days),
	}
}

func (h *harness) wireDraft(draft itinerary.Draft) any {
	stops := make([]any, 0, len(draft.Stops))
	for _, stop := range draft.Stops {
		var point any
		if stop.MeetingPoint != nil {
			point = map[string]any{
				"lat":   wireFloat(stop.MeetingPoint.Lat),
				"lng":   wireFloat(stop.MeetingPoint.Lng),
				"label": stop.MeetingPoint.Label,
			}
		}
		// The id is the client's own string, which is what model_dump wrote
		// and what Python encoded: an alias would be a different value.
		stops = append(stops, []any{
			stop.At, stop.Label, optionalText(stop.PlaceName), optionalText(stop.PlaceID),
			stop.ID, optionalText(stop.Day),
			stop.DurationMinutes, stop.TimeLocked, point,
			scalarWire(stop.Lat), scalarWire(stop.Lng), stop.CheckedIn,
		})
	}
	days := make([]any, 0, len(draft.Days))
	for _, day := range draft.Days {
		days = append(days, []any{
			day.Day, day.TransportMode, day.StartAt,
			optionalText(day.StartStopID), optionalText(day.EndStopID),
			day.ReturnToStart,
		})
	}
	return map[string]any{
		"expected_revision":  draft.ExpectedRevision,
		"stops":              stops,
		"days":               days,
		"day":                draft.Day,
		"include_suggestion": draft.IncludeSuggestion,
	}
}

func scalarWire(value any) any {
	if number, ok := value.(float64); ok {
		return wireFloat(number)
	}
	return value
}

func (h *harness) wireCheckinView(view CheckinView) any {
	return map[string]any{
		"id":           h.name(view.ID),
		"stop_id":      h.name(view.StopID),
		"person_id":    h.name(view.PersonID),
		"display_name": optionalText(view.DisplayName),
		"created_at":   iso(view.CreatedAt),
	}
}

func (h *harness) wireInviteView(view InviteView) any {
	return map[string]any{
		"id":                h.name(view.ID),
		"outing_id":         h.name(view.OutingID),
		"source":            view.Source,
		"invited_person_id": h.optionalName(view.InvitedPersonID),
		"invited_by_id":     h.name(view.InvitedByID),
		"created_at":        iso(view.CreatedAt),
		"expires_at":        iso(view.ExpiresAt),
		"revoked_at":        optionalISO(view.RevokedAt),
		"invite_token":      optionalText(view.InviteToken),
		"invite_path":       optionalText(view.InvitePath),
	}
}

// --- replay ------------------------------------------------------------------

// call runs one method and returns what it answered, already in the golden's
// shape, or the error it ended with.
func (h *harness) call(s *stub, c oracletest.Case, req map[string]any, actor Actor, now any) (any, error) {
	moment, err := oracletest.Instant(now)
	if err != nil {
		return nil, oracletest.Decode(err)
	}
	mint := mintedToken{raw: h.tokens["TK_MOI"], alias: "TK_MOI"}
	id := func(key string) (string, error) { return h.id(req[key]) }
	switch c.Fn {
	case "create_outing":
		contextID, err := id("context_id")
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		title, err := oracletest.Str(req["title"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		starts, err := parseDate(req["starts_on"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		ends, err := parseDate(req["ends_on"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		headcount, err := oracletest.Int64(req["headcount"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		budget, err := oracletest.Int64(req["budget_per_person_vnd"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		view, err := CreateOuting(s, contextID, OutingCreate{
			Title: title, StartsOn: starts, EndsOn: ends,
			Headcount: headcount, BudgetPerPersonVND: budget,
		}, actor, moment)
		if err != nil {
			return nil, err
		}
		return h.wireOutingView(view), nil
	case "list_context_outings":
		contextID, err := id("context_id")
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		view, err := ListContextOutings(s, contextID, actor)
		if err != nil {
			return nil, err
		}
		outings := make([]any, 0, len(view.Outings))
		for _, outing := range view.Outings {
			outings = append(outings, h.wireOutingView(outing))
		}
		return map[string]any{"context_id": h.name(view.ContextID), "outings": outings}, nil
	case "replace_outing_timeline":
		outingID, err := id("outing_id")
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		request := TimelineRequest{}
		if request.ExpectedRevision, err = optionalNumber(req["expected_revision"]); err != nil {
			return nil, oracletest.Decode(err)
		}
		stops, err := oracletest.List(req["stops"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		for _, raw := range stops {
			stop, err := h.stopInput(raw)
			if err != nil {
				return nil, oracletest.Decode(err)
			}
			request.Stops = append(request.Stops, stop)
		}
		view, err := ReplaceOutingTimeline(s, outingID, request, actor)
		if err != nil {
			return nil, err
		}
		return h.wireOutingView(view), nil
	case "authorize_outing_itinerary":
		outingID, err := id("outing_id")
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		record, err := AuthorizeOutingItinerary(s, outingID, actor)
		if err != nil {
			return nil, err
		}
		return h.wireOutingRecord(*record), nil
	case "preview_outing_itinerary", "replace_outing_itinerary":
		outingID, err := id("outing_id")
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		preview := c.Fn == "preview_outing_itinerary"
		request, err := h.itineraryRequest(req["request"], preview)
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		if preview {
			draft, err := PreviewOutingItinerary(s, outingID, request, actor)
			if err != nil {
				return nil, err
			}
			return h.wireDraft(draft), nil
		}
		view, err := ReplaceOutingItinerary(s, outingID, request, actor)
		if err != nil {
			return nil, err
		}
		return h.wireOutingView(view), nil
	case "check_in_to_stop":
		stopID, err := id("stop_id")
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		view, err := CheckInToStop(s, stopID, actor, moment)
		if err != nil {
			return nil, err
		}
		return h.wireCheckinView(view), nil
	case "list_outing_checkins":
		outingID, err := id("outing_id")
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		view, err := ListOutingCheckins(s, outingID, actor)
		if err != nil {
			return nil, err
		}
		rows := make([]any, 0, len(view.Checkins))
		for _, row := range view.Checkins {
			rows = append(rows, h.wireCheckinView(row))
		}
		return map[string]any{"outing_id": h.name(view.OutingID), "checkins": rows}, nil
	case "create_outing_invite":
		outingID, err := id("outing_id")
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		source, err := oracletest.Str(req["source"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		person, err := h.optionalID(req["person_id"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		view, err := CreateOutingInvite(s, mint, outingID, InviteCreate{Source: source, PersonID: person}, actor, moment)
		if err != nil {
			return nil, err
		}
		return h.wireInviteView(view), nil
	case "accept_outing_invite":
		token, err := oracletest.Str(req["token"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		view, err := AcceptOutingInvite(s, []byte(token), actor, moment)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"invite_id":        h.name(view.InviteID),
			"outing_id":        h.name(view.OutingID),
			"context_id":       h.name(view.ContextID),
			"membership_id":    h.name(view.MembershipID),
			"membership_state": view.MembershipState,
		}, nil
	case "rotate_outing_invite_secret", "revoke_outing_invite":
		outingID, err := id("outing_id")
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		inviteID, err := id("invite_id")
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		var view InviteView
		if c.Fn == "rotate_outing_invite_secret" {
			view, err = RotateOutingInviteSecret(s, mint, outingID, inviteID, actor, moment)
		} else {
			view, err = RevokeOutingInvite(s, outingID, inviteID, actor, moment)
		}
		if err != nil {
			return nil, err
		}
		return h.wireInviteView(view), nil
	}
	return nil, oracletest.Decode(fmt.Errorf("no replay for %s", c.Fn))
}

func (h *harness) replay(c oracletest.Case, args map[string]any) (any, error) {
	moment, err := oracletest.Instant(args["now"])
	if err != nil {
		return nil, oracletest.Decode(err)
	}
	pair, err := oracletest.List(args["actor"])
	if err != nil || len(pair) != 2 {
		return nil, oracletest.Decode(fmt.Errorf("%v is not an actor", args["actor"]))
	}
	actorID, err := h.id(pair[0])
	if err != nil {
		return nil, oracletest.Decode(err)
	}
	roles, err := oracletest.Strings(pair[1])
	if err != nil {
		return nil, oracletest.Decode(err)
	}
	req, err := oracletest.Row(args["req"])
	if err != nil {
		return nil, oracletest.Decode(err)
	}
	s, err := h.newStub(args["world"], moment, h.earlier)
	if err != nil {
		return nil, oracletest.Decode(err)
	}
	out := map[string]any{"calls": []any{}, "problem": nil, "raised": nil, "response": nil}
	answer, callErr := h.call(s, c, req, Actor{ID: actorID, Roles: roles}, args["now"])
	out["calls"] = s.recorded()
	if errors.Is(callErr, oracletest.ErrDecode) {
		return nil, callErr
	}
	switch {
	case callErr == nil:
		out["response"] = answer
	default:
		var refused *Refusal
		var denied *permissions.Error
		var conflict *Conflict
		var value *journey.ValueError
		var index *journey.IndexError
		switch {
		case errors.As(callErr, &value):
			// `_minute_of_day` on a time the pattern would have refused. It
			// has no `code`, and Python answers the request with a 500.
			out["raised"] = map[string]any{
				"type": "ValueError", "code": nil, "message": value.Message,
			}
		case errors.As(callErr, &index):
			out["raised"] = map[string]any{
				"type": "IndexError", "code": nil, "message": index.Error(),
			}
		case errors.As(callErr, &refused):
			out["problem"] = map[string]any{
				"status": int64(refused.Status), "code": refused.Code, "detail": refused.Detail,
			}
		case errors.As(callErr, &denied):
			out["raised"] = map[string]any{
				"type": "PermissionError_", "code": denied.Code, "message": denied.Code,
			}
		case errors.As(callErr, &conflict):
			out["raised"] = map[string]any{
				"type": "RepositoryConflict", "code": conflict.Code, "message": conflict.Code,
			}
		default:
			return nil, callErr
		}
	}
	return out, nil
}

func checkSteps(t *testing.T, h *harness, files []oracletest.File, committed bool) {
	t.Helper()
	report := oracletest.Agree(t, files, "outing_steps", h.replay, asRefusal)
	if !committed {
		return
	}
	for _, name := range methods {
		if report.ByFn[name] == nil {
			t.Errorf("no case calls %s", name)
		}
	}
	for _, code := range problemCodes {
		if report.Codes[code] == 0 {
			t.Errorf("no committed case answers %q", code)
		}
	}
	seen := map[string]bool{}
	for _, file := range files {
		for _, c := range file.Cases {
			body, ok := c.Result["ok"].(map[string]any)
			if !ok {
				continue
			}
			if row, ok := body["raised"].(map[string]any); ok {
				kind, err := oracletest.Text(row["type"])
				if err == nil {
					seen[kind] = true
				}
			}
		}
	}
	for _, kind := range raisedTypes {
		if !seen[kind] {
			t.Errorf("no committed case ends with %s", kind)
		}
	}
}

func TestOutingStepsMatchPython(t *testing.T) {
	files := oracletest.Load(t, "testdata/python_outing_steps*.json")
	constants := oracletest.Constants(t, files, "outing_steps")
	h := newHarness(t, constants)
	ttl, err := oracletest.Int64(constants["invite_ttl_seconds"])
	if err != nil || ttl != int64(InviteTTL.Seconds()) {
		t.Errorf("invite ttl: Python %v, Go %v (%v)", constants["invite_ttl_seconds"], InviteTTL, err)
	}
	names, err := oracletest.Strings(constants["methods"])
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != len(methods) {
		t.Errorf("Python ports %d methods, this test names %d", len(names), len(methods))
	}
	checkSteps(t, h, files, true)
}

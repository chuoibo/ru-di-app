package routes

import (
	"fmt"

	"mobile/services/core/internal/domain/outingsteps"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/pyval"
)

func dateOf(d pyval.Date) outingsteps.Date {
	return outingsteps.Date{Year: d.Year, Month: d.Month, Day: d.Day}
}

func dateField(model *pyval.Model, name string) (outingsteps.Date, error) {
	value, err := field(model, name)
	if err != nil {
		return outingsteps.Date{}, err
	}
	d, ok := value.(pyval.Date)
	if !ok {
		return outingsteps.Date{}, fmt.Errorf("routes: %s.%s is %T, not a date", model.Class, name, value)
	}
	return dateOf(d), nil
}

func optionalDateField(model *pyval.Model, name string) (*outingsteps.Date, error) {
	value, err := field(model, name)
	if err != nil {
		return nil, err
	}
	switch v := value.(type) {
	case pyjson.Null:
		return nil, nil
	case pyval.Date:
		d := dateOf(v)
		return &d, nil
	}
	return nil, fmt.Errorf("routes: %s.%s is %T, not a date or None", model.Class, name, value)
}

func int64Field(model *pyval.Model, name string) (int64, error) {
	n, err := intField(model, name)
	if err != nil {
		return 0, err
	}
	v, ok := n.Int64()
	if !ok {
		return 0, fmt.Errorf("routes: %s.%s does not fit int64", model.Class, name)
	}
	return v, nil
}

func optionalInt64Field(model *pyval.Model, name string) (*int64, error) {
	value, err := field(model, name)
	if err != nil {
		return nil, err
	}
	switch v := value.(type) {
	case pyjson.Null:
		return nil, nil
	case pyjson.Int:
		n, ok := v.Int64()
		if !ok {
			return nil, fmt.Errorf("routes: %s.%s does not fit int64", model.Class, name)
		}
		return &n, nil
	}
	return nil, fmt.Errorf("routes: %s.%s is %T, not an int or None", model.Class, name, value)
}

func floatField(model *pyval.Model, name string) (float64, error) {
	value, err := field(model, name)
	if err != nil {
		return 0, err
	}
	n, ok := value.(pyjson.Float)
	if !ok {
		return 0, fmt.Errorf("routes: %s.%s is %T, not a float", model.Class, name, value)
	}
	return float64(n), nil
}

func outingCreateRequest(model *pyval.Model) (outingsteps.OutingCreate, error) {
	title, err := stringField(model, "title")
	if err != nil {
		return outingsteps.OutingCreate{}, err
	}
	starts, err := dateField(model, "starts_on")
	if err != nil {
		return outingsteps.OutingCreate{}, err
	}
	ends, err := dateField(model, "ends_on")
	if err != nil {
		return outingsteps.OutingCreate{}, err
	}
	headcount, err := int64Field(model, "headcount")
	if err != nil {
		return outingsteps.OutingCreate{}, err
	}
	budget, err := int64Field(model, "budget_per_person_vnd")
	if err != nil {
		return outingsteps.OutingCreate{}, err
	}
	return outingsteps.OutingCreate{
		Title: title, StartsOn: starts, EndsOn: ends,
		Headcount: headcount, BudgetPerPersonVND: budget,
	}, nil
}

func timelineRequest(model *pyval.Model) (outingsteps.TimelineRequest, error) {
	revision, err := optionalInt64Field(model, "expected_revision")
	if err != nil {
		return outingsteps.TimelineRequest{}, err
	}
	stopModels, err := modelListField(model, "stops")
	if err != nil {
		return outingsteps.TimelineRequest{}, err
	}
	stops := make([]outingsteps.StopInput, 0, len(stopModels))
	for _, stop := range stopModels {
		row, err := stopInput(stop)
		if err != nil {
			return outingsteps.TimelineRequest{}, err
		}
		stops = append(stops, row)
	}
	return outingsteps.TimelineRequest{ExpectedRevision: revision, Stops: stops}, nil
}

func stopInput(model *pyval.Model) (outingsteps.StopInput, error) {
	at, err := stringField(model, "at")
	if err != nil {
		return outingsteps.StopInput{}, err
	}
	label, err := stringField(model, "label")
	if err != nil {
		return outingsteps.StopInput{}, err
	}
	placeName, err := optionalStringField(model, "place_name")
	if err != nil {
		return outingsteps.StopInput{}, err
	}
	placeID, err := optionalStringField(model, "place_id")
	if err != nil {
		return outingsteps.StopInput{}, err
	}
	return outingsteps.StopInput{At: at, Label: label, PlaceName: placeName, PlaceID: placeID}, nil
}

func itineraryRequest(model *pyval.Model) (outingsteps.ItineraryRequest, error) {
	revision, err := int64Field(model, "expected_revision")
	if err != nil {
		return outingsteps.ItineraryRequest{}, err
	}
	stopModels, err := modelListField(model, "stops")
	if err != nil {
		return outingsteps.ItineraryRequest{}, err
	}
	stops := make([]outingsteps.ItineraryStopInput, 0, len(stopModels))
	for _, stop := range stopModels {
		row, err := itineraryStopInput(stop)
		if err != nil {
			return outingsteps.ItineraryRequest{}, err
		}
		stops = append(stops, row)
	}
	dayModels, err := modelListField(model, "days")
	if err != nil {
		return outingsteps.ItineraryRequest{}, err
	}
	days := make([]outingsteps.Day, 0, len(dayModels))
	for _, day := range dayModels {
		row, err := itineraryDay(day)
		if err != nil {
			return outingsteps.ItineraryRequest{}, err
		}
		days = append(days, row)
	}
	return outingsteps.ItineraryRequest{ExpectedRevision: revision, Stops: stops, Days: days}, nil
}

func itineraryPreviewRequest(model *pyval.Model) (outingsteps.ItineraryRequest, error) {
	request, err := itineraryRequest(model)
	if err != nil {
		return outingsteps.ItineraryRequest{}, err
	}
	day, err := dateField(model, "day")
	if err != nil {
		return outingsteps.ItineraryRequest{}, err
	}
	include, err := boolField(model, "include_suggestion")
	if err != nil {
		return outingsteps.ItineraryRequest{}, err
	}
	request.Day = &day
	request.IncludeSuggestion = include
	return request, nil
}

func itineraryStopInput(model *pyval.Model) (outingsteps.ItineraryStopInput, error) {
	base, err := stopInput(model)
	if err != nil {
		return outingsteps.ItineraryStopInput{}, err
	}
	id, err := stringField(model, "id")
	if err != nil {
		return outingsteps.ItineraryStopInput{}, err
	}
	day, err := optionalDateField(model, "day")
	if err != nil {
		return outingsteps.ItineraryStopInput{}, err
	}
	duration, err := optionalInt64Field(model, "duration_minutes")
	if err != nil {
		return outingsteps.ItineraryStopInput{}, err
	}
	locked, err := boolField(model, "time_locked")
	if err != nil {
		return outingsteps.ItineraryStopInput{}, err
	}
	point, err := optionalMeetingPoint(model, "meeting_point")
	if err != nil {
		return outingsteps.ItineraryStopInput{}, err
	}
	return outingsteps.ItineraryStopInput{
		StopInput: base, ID: id, Day: day, DurationMinutes: duration,
		TimeLocked: locked, MeetingPoint: point,
	}, nil
}

func itineraryDay(model *pyval.Model) (outingsteps.Day, error) {
	day, err := dateField(model, "day")
	if err != nil {
		return outingsteps.Day{}, err
	}
	mode, err := stringField(model, "transport_mode")
	if err != nil {
		return outingsteps.Day{}, err
	}
	startAt, err := stringField(model, "start_at")
	if err != nil {
		return outingsteps.Day{}, err
	}
	startID, err := optionalStringField(model, "start_stop_id")
	if err != nil {
		return outingsteps.Day{}, err
	}
	endID, err := optionalStringField(model, "end_stop_id")
	if err != nil {
		return outingsteps.Day{}, err
	}
	returning, err := boolField(model, "return_to_start")
	if err != nil {
		return outingsteps.Day{}, err
	}
	return outingsteps.Day{
		Day: day, TransportMode: mode, StartAt: startAt,
		StartStopID: startID, EndStopID: endID, ReturnToStart: returning,
	}, nil
}

func optionalMeetingPoint(model *pyval.Model, name string) (*outingsteps.MeetingPoint, error) {
	value, err := field(model, name)
	if err != nil {
		return nil, err
	}
	switch v := value.(type) {
	case pyjson.Null:
		return nil, nil
	case *pyval.Model:
		lat, err := floatField(v, "lat")
		if err != nil {
			return nil, err
		}
		lng, err := floatField(v, "lng")
		if err != nil {
			return nil, err
		}
		label, err := stringField(v, "label")
		if err != nil {
			return nil, err
		}
		return &outingsteps.MeetingPoint{Lat: lat, Lng: lng, Label: label}, nil
	}
	return nil, fmt.Errorf("routes: %s.%s is %T, not a meeting point or None", model.Class, name, value)
}

func inviteCreateRequest(model *pyval.Model) (outingsteps.InviteCreate, error) {
	source, err := stringField(model, "source")
	if err != nil {
		return outingsteps.InviteCreate{}, err
	}
	personID, err := optionalUUIDField(model, "person_id")
	if err != nil {
		return outingsteps.InviteCreate{}, err
	}
	return outingsteps.InviteCreate{Source: source, PersonID: personID}, nil
}

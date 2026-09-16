package outingsteps

import "time"

// Invariant is a stored row these wire helpers cannot render. Python does not
// raise it by that name: it hands the half-written row to pydantic, which
// refuses it as a ValidationError and ends the request as a 500. The name is
// different, the answer is the same, and the point of having it at all is that
// a Go nil dereference would have been neither.
type Invariant struct {
	Reason string
}

func (e *Invariant) Error() string { return "outingsteps: " + e.Reason }

// MeetingPointView is MeetingPoint on the way out.
type MeetingPointView struct {
	Lat   float64
	Lng   float64
	Label string
}

// StopView is OutingStopResponse.
type StopView struct {
	ID              string
	Position        int
	At              string
	Label           string
	PlaceName       *string
	PlaceID         *string
	Day             *Date
	DurationMinutes *int64
	TimeLocked      bool
	MeetingPoint    *MeetingPointView
}

// OutingView is OutingResponse.
type OutingView struct {
	ID                 string
	ContextID          string
	CreatedByID        string
	Title              string
	StartsOn           Date
	EndsOn             Date
	Headcount          int64
	BudgetPerPersonVND int64
	CreatedAt          time.Time
	Stops              []StopView
	TimelineRevision   int64
	ItineraryVersion   int64
	Days               []Day
}

// OutingListView is OutingListResponse.
type OutingListView struct {
	ContextID string
	Outings   []OutingView
}

// CheckinView is StopCheckinResponse. There is no latitude, longitude or
// accuracy on it on purpose: F46 is somebody pressing "đã tới", not the phone
// reporting where it is.
type CheckinView struct {
	ID          string
	StopID      string
	PersonID    string
	DisplayName *string
	CreatedAt   time.Time
}

// CheckinListView is OutingCheckinListResponse.
type CheckinListView struct {
	OutingID string
	Checkins []CheckinView
}

// InviteView is OutingInviteResponse.
type InviteView struct {
	ID              string
	OutingID        string
	Source          string
	InvitedPersonID *string
	InvitedByID     string
	CreatedAt       time.Time
	ExpiresAt       time.Time
	RevokedAt       *time.Time
	InviteToken     *string
	InvitePath      *string
}

// InviteAcceptView is OutingInviteAcceptResponse. It names neither the group
// nor the trip: a link redeemer is not a member yet.
type InviteAcceptView struct {
	InviteID        string
	OutingID        string
	ContextID       string
	MembershipID    string
	MembershipState string
}

// clock is _clock: a stored wall-clock minute written back the same in every
// server timezone.
func clock(minute int64) string {
	hour, rest := minute/60, minute%60
	return twoDigits(hour) + ":" + twoDigits(rest)
}

// twoDigits is Python's "{:02d}", which pads to two and then keeps going: a
// stored minute_of_day outside a day still renders, it just renders wide.
func twoDigits(value int64) string {
	sign := ""
	if value < 0 {
		sign, value = "-", -value
	}
	digits := ""
	if value == 0 {
		digits = "0"
	}
	for value > 0 {
		digits = string(rune('0'+value%10)) + digits
		value /= 10
	}
	if len(digits)+len(sign) < 2 {
		digits = "0" + digits
	}
	return sign + digits
}

// wireOuting is _wire_outing.
func wireOuting(record Outing) (OutingView, error) {
	stops := make([]StopView, 0, len(record.Stops))
	for _, stop := range record.Stops {
		var point *MeetingPointView
		if stop.MeetingLat != nil {
			if stop.MeetingLng == nil || stop.MeetingLabel == nil {
				return OutingView{}, &Invariant{Reason: "a stop has a meeting latitude without a longitude or a label"}
			}
			point = &MeetingPointView{Lat: *stop.MeetingLat, Lng: *stop.MeetingLng, Label: *stop.MeetingLabel}
		}
		stops = append(stops, StopView{
			ID:              stop.ID,
			Position:        stop.Position,
			At:              clock(stop.MinuteOfDay),
			Label:           stop.Label,
			PlaceName:       stop.PlaceName,
			PlaceID:         stop.PlaceID,
			Day:             stop.Day,
			DurationMinutes: stop.DurationMinutes,
			TimeLocked:      stop.TimeLocked,
			MeetingPoint:    point,
		})
	}
	days := make([]Day, 0, len(record.ItineraryDays))
	days = append(days, record.ItineraryDays...)
	return OutingView{
		ID:                 record.ID,
		ContextID:          record.ContextID,
		CreatedByID:        record.CreatedByID,
		Title:              record.Title,
		StartsOn:           record.StartsOn,
		EndsOn:             record.EndsOn,
		Headcount:          record.Headcount,
		BudgetPerPersonVND: record.BudgetPerPersonVND,
		CreatedAt:          record.CreatedAt,
		Stops:              stops,
		TimelineRevision:   record.TimelineRevision,
		ItineraryVersion:   record.ItineraryVersion,
		Days:               days,
	}, nil
}

// wireCheckin is _wire_stop_checkin: one get_person per row, which is what
// makes listing a day's arrivals N+1 reads.
func wireCheckin(s Store, record Checkin) (CheckinView, error) {
	person, err := s.GetPerson(record.PersonID)
	if err != nil {
		return CheckinView{}, err
	}
	view := CheckinView{
		ID:        record.ID,
		StopID:    record.StopID,
		PersonID:  record.PersonID,
		CreatedAt: record.CreatedAt,
	}
	if person != nil {
		name := person.DisplayName
		view.DisplayName = &name
	}
	return view, nil
}

// wireInvite is _wire_outing_invite. Only a link has a path to open: a named
// invitation is spent by posting its token to /sessions, and printing an
// /outing-invites url beside it would invite exactly the mix-up the two doors
// exist to prevent.
func wireInvite(record Invite, raw *string) InviteView {
	view := InviteView{
		ID:              record.ID,
		OutingID:        record.OutingID,
		Source:          record.Source,
		InvitedPersonID: record.InvitedPersonID,
		InvitedByID:     record.InvitedByID,
		CreatedAt:       record.CreatedAt,
		ExpiresAt:       record.ExpiresAt,
		RevokedAt:       record.RevokedAt,
		InviteToken:     raw,
	}
	if raw != nil && record.Source == "link" {
		path := "/outing-invites/" + *raw
		view.InvitePath = &path
	}
	return view
}

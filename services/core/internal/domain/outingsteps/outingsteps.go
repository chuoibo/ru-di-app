// Package outingsteps is the workflow of the eleven W7 routes: the outing,
// timeline, itinerary, check-in and invitation methods of
// services/api/app/api/service.py, with every repository call behind Store.
//
// Each method makes exactly the calls Python makes, in Python's order, with
// Python's arguments; the rules between them -- the permission facts, the
// refusal each failed predicate becomes, the day and anchor checks, the
// translation of a RepositoryConflict -- are the Python code's. A route
// implements Store over its transaction and renders the returned view.
//
// Errors a method returns:
//
//   - *Refusal is an ApiProblem: status, code and detail;
//   - *permissions.Error is PermissionError_, which Python never catches;
//   - *Conflict is a RepositoryConflict a Store returned and the method did
//     not translate;
//   - anything else is the Store's own failure, passed through.
//
// Everything but *Refusal ends the request as Python's 500 does.
//
// # Two boundaries this package does not cross
//
// The routing service is one. PreviewOutingItinerary answers the draft that
// `_itinerary_draft` built and stops there; the route hands that draft to
// internal/domain/itinerary with a Router that can open a socket, which is
// what `preview_itinerary(draft)` does on the Python side of the same line.
//
// A secret is the other. `secrets.token_urlsafe(32)` and the SHA-256 behind
// `token_digest` are neither pure nor arithmetic, so minting an invitation's
// secret is Secrets' job, and a token presented at the door arrives here
// already digested. A digest is compared and stored, never read.
//
// Ids are the canonical uuid strings (str(uuid.UUID)); the methods only ever
// compare them for equality. `now` is the service clock read once per request
// (`_now()`, datetime.now(UTC)), passed to each method rather than read.
//
// testdata/python_outing_steps*.json is rendered by
// scripts/render_domain_w7_goldens.py by running the real ApiService methods
// over a recording stub repository with the clock pinned; oracle_test.go
// replays every case through a recording Store, comparing the answer and every
// repository call with its arguments.
package outingsteps

import (
	"strconv"
	"strings"
	"time"

	"mobile/services/core/internal/domain/direct"
	"mobile/services/core/internal/domain/itinerary"
	"mobile/services/core/internal/domain/journey"
	"mobile/services/core/internal/domain/pairpaper"
	"mobile/services/core/internal/domain/permissions"
)

// InviteTTL is OUTING_INVITE_TTL: how long an invitation's secret is worth
// presenting.
const InviteTTL = 7 * 24 * time.Hour

// Actor is the service's Actor: its id and the roles the session carries.
type Actor struct {
	ID    string
	Roles []string
}

// Refusal is an ApiProblem.
type Refusal struct {
	Status int
	Code   string
	Detail string
}

func (r *Refusal) Error() string { return r.Code }

func refusal(status int, code, detail string) *Refusal {
	return &Refusal{Status: status, Code: code, Detail: detail}
}

// Conflict is RepositoryConflict: a Store returns it when a persistence
// invariant refuses a write.
type Conflict struct {
	Code string
}

func (c *Conflict) Error() string { return c.Code }

// Date is datetime.date, the type a trip's days are compared as.
type Date = pairpaper.Date

// Stop is OutingStopRecord: one stop as it is stored.
type Stop struct {
	ID              string
	Position        int
	MinuteOfDay     int64
	Label           string
	PlaceName       *string
	PlaceID         *string
	Day             *Date
	DurationMinutes *int64
	TimeLocked      bool
	MeetingLat      *float64
	MeetingLng      *float64
	MeetingLabel    *string
}

// Day is one entry of `itinerary_days`, and of a request's `days`. It is
// ItineraryDay both ways: what is stored is `model_dump(mode="json")` of the
// model, and pydantic reads it straight back into the model on the way out.
type Day struct {
	Day           Date
	TransportMode string
	StartAt       string
	StartStopID   *string
	EndStopID     *string
	ReturnToStart bool
}

// Outing is OutingRecord.
type Outing struct {
	ID                 string
	ContextID          string
	CreatedByID        string
	Title              string
	StartsOn           Date
	EndsOn             Date
	Headcount          int64
	BudgetPerPersonVND int64
	CreatedAt          time.Time
	Stops              []Stop
	TimelineRevision   int64
	ItineraryVersion   int64
	ItineraryDays      []Day
}

// Checkin is StopCheckinRecord: one arrival, carrying no coordinates.
type Checkin struct {
	ID        string
	StopID    string
	PersonID  string
	CreatedAt time.Time
}

// Invite is OutingInviteRecord.
type Invite struct {
	ID              string
	OutingID        string
	Source          string
	InvitedPersonID *string
	InvitedByID     string
	AcceptedAt      *time.Time
	AcceptedByID    *string
	CreatedAt       time.Time
	ExpiresAt       time.Time
	RevokedAt       *time.Time
}

// Person is the part of PersonRecord a check-in row shows.
type Person struct {
	DisplayName string
}

// Context is the part of ContextRecord `_require_group_kind` reads.
type Context struct {
	Kind string
}

// Member is one row of list_members.
type Member struct {
	PersonID string
	State    string
}

// Membership is MembershipRecord, the two fields the accept answer carries.
type Membership struct {
	ID    string
	State string
}

// Place is the catalogue row `place_row` returns, reduced to the two keys the
// itinerary reads. A non-nil Place is a truthy dict.
type Place struct {
	Lat *float64
	Lng *float64
}

// MeetingPoint is the MeetingPoint of a request stop.
type MeetingPoint struct {
	Lat   float64
	Lng   float64
	Label string
}

// StopInput is OutingStopInput after pydantic validated it: the body of a
// timeline stop.
type StopInput struct {
	At        string
	Label     string
	PlaceName *string
	PlaceID   *string
}

// ItineraryStopInput is ItineraryStopInput: a timeline stop plus everything
// the itinerary adds.
type ItineraryStopInput struct {
	StopInput
	ID              string
	Day             *Date
	DurationMinutes *int64
	TimeLocked      bool
	MeetingPoint    *MeetingPoint
}

// OutingCreate is OutingCreateRequest.
type OutingCreate struct {
	Title              string
	StartsOn           Date
	EndsOn             Date
	Headcount          int64
	BudgetPerPersonVND int64
}

// TimelineRequest is OutingTimelineRequest. ExpectedRevision is nil for None,
// which the repository reads as "do not check".
type TimelineRequest struct {
	ExpectedRevision *int64
	Stops            []StopInput
}

// ItineraryRequest is ItineraryRequest, and, with Day set, the
// ItineraryPreviewRequest that extends it.
type ItineraryRequest struct {
	ExpectedRevision int64
	Stops            []ItineraryStopInput
	Days             []Day
	// Day and IncludeSuggestion are ItineraryPreviewRequest's two extra
	// fields; Day is nil on the plain request, whose model_dump carries
	// neither key.
	Day               *Date
	IncludeSuggestion bool
}

// InviteCreate is OutingInviteCreateRequest. PersonID is nil exactly when
// Source is "link", which the model validator already proved.
type InviteCreate struct {
	Source   string
	PersonID *string
}

// StopDraft is one stop as replace_outing_stops receives it: the four keys of
// `{"minute_of_day", "label", "place_name", "place_id"}`, in that order.
type StopDraft struct {
	MinuteOfDay int64
	Label       string
	PlaceName   *string
	PlaceID     *string
}

// ItineraryStopDraft is one stop as replace_outing_itinerary receives it:
// `stop.model_dump()` with `at` replaced by `minute_of_day` at the end. The
// key order is label, place_name, place_id, id, day, duration_minutes,
// time_locked, meeting_point, minute_of_day.
type ItineraryStopDraft struct {
	Label           string
	PlaceName       *string
	PlaceID         *string
	ID              string
	Day             *Date
	DurationMinutes *int64
	TimeLocked      bool
	MeetingPoint    *MeetingPoint
	MinuteOfDay     int64
}

// OutingDraft is create_outing's arguments.
type OutingDraft struct {
	ContextID          string
	CreatedByID        string
	Title              string
	StartsOn           Date
	EndsOn             Date
	Headcount          int64
	BudgetPerPersonVND int64
	Now                time.Time
}

// InviteDraft is create_outing_invite's arguments.
type InviteDraft struct {
	OutingID        string
	Source          string
	InvitedPersonID *string
	InvitedByID     string
	TokenDigest     []byte
	ExpiresAt       time.Time
	Now             time.Time
}

// MembershipDraft is ensure_invited_membership's arguments.
type MembershipDraft struct {
	ContextID   string
	PersonID    string
	InvitedByID string
	Origin      string
	Now         time.Time
}

// Store is the part of ApiRepository these methods call, one method per
// repository method with its arguments in the Protocol's order. A write a
// persistence invariant refuses returns *Conflict with the repository's code.
type Store interface {
	IsMember(contextID, personID string) (bool, error)
	GetContext(contextID string) (*Context, error)
	ListMembers(contextID string) ([]Member, error)
	GetPerson(personID string) (*Person, error)
	GetPlace(placeID string) (*Place, error)

	CreateOuting(draft OutingDraft) (Outing, error)
	GetOuting(outingID string) (*Outing, error)
	ListOutings(contextID string) ([]Outing, error)
	ReplaceOutingStops(outingID string, stops []StopDraft, expectedRevision *int64) (Outing, error)
	ReplaceOutingItinerary(outingID string, stops []ItineraryStopDraft, days []Day, expectedRevision int64) (Outing, error)

	GetOutingStop(stopID string) (*Stop, *Outing, error)
	CreateStopCheckin(stopID, personID string, now time.Time) (Checkin, error)
	ListOutingCheckins(outingID string) ([]Checkin, error)

	CreateOutingInvite(draft InviteDraft) (Invite, error)
	FindOutingInviteForPerson(outingID, personID string) (*Invite, error)
	GetOutingInvite(inviteID string) (*Invite, error)
	GetOutingInviteByDigest(tokenDigest []byte) (*Invite, error)
	AcceptOutingInvite(inviteID, acceptedByID string, now time.Time) (Invite, error)
	RevokeOutingInvite(inviteID string, now time.Time) (Invite, error)
	RotateOutingInviteDigest(inviteID string, tokenDigest []byte, expiresAt time.Time, now time.Time) (Invite, error)
	EnsureInvitedMembership(draft MembershipDraft) (Membership, error)
}

// Secrets mints an invitation's one-time secret: `secrets.token_urlsafe(32)`
// and the SHA-256 digest of it. The raw token is returned to the caller
// exactly once and never persisted; only the digest crosses Store.
type Secrets interface {
	NewInviteToken() (raw string, digest []byte, err error)
}

// requirePermission is _require_permission: the facts are built here, never
// accepted from a caller, and a refusal is one 403 carrying the predicate that
// failed.
func requirePermission(action string, actor Actor, proven ...string) error {
	reason, allowed, err := permissions.DenialReason(action, permissions.AuthorizationFacts{
		ActorID:    actor.ID,
		Roles:      actor.Roles,
		Proven:     proven,
		Provenance: "api_service",
	})
	if err != nil {
		return err
	}
	if allowed {
		return nil
	}
	return refusal(403, "permission_denied", reason)
}

// groupMember is the one fact every outing door is decided on.
func groupMember(s Store, contextID, personID string) ([]string, error) {
	member, err := s.IsMember(contextID, personID)
	if err != nil {
		return nil, err
	}
	if member {
		return []string{"is_group_member"}, nil
	}
	return nil, nil
}

// CreateOuting is create_outing.
func CreateOuting(s Store, contextID string, request OutingCreate, actor Actor, now time.Time) (OutingView, error) {
	proven, err := groupMember(s, contextID, actor.ID)
	if err != nil {
		return OutingView{}, err
	}
	if err := requirePermission("create_outing", actor, proven...); err != nil {
		return OutingView{}, err
	}
	record, err := s.CreateOuting(OutingDraft{
		ContextID:          contextID,
		CreatedByID:        actor.ID,
		Title:              request.Title,
		StartsOn:           request.StartsOn,
		EndsOn:             request.EndsOn,
		Headcount:          request.Headcount,
		BudgetPerPersonVND: request.BudgetPerPersonVND,
		Now:                now,
	})
	if err != nil {
		return OutingView{}, err
	}
	return wireOuting(record)
}

// ListContextOutings is list_context_outings.
func ListContextOutings(s Store, contextID string, actor Actor) (OutingListView, error) {
	proven, err := groupMember(s, contextID, actor.ID)
	if err != nil {
		return OutingListView{}, err
	}
	if err := requirePermission("view_outings", actor, proven...); err != nil {
		return OutingListView{}, err
	}
	records, err := s.ListOutings(contextID)
	if err != nil {
		return OutingListView{}, err
	}
	view := OutingListView{ContextID: contextID, Outings: make([]OutingView, 0, len(records))}
	for _, record := range records {
		row, err := wireOuting(record)
		if err != nil {
			return OutingListView{}, err
		}
		view.Outings = append(view.Outings, row)
	}
	return view, nil
}

// itineraryConflict is _itinerary_conflict: the repository codes that are a
// sentence rather than a 500. A code outside the table is re-raised.
var itineraryConflict = map[string]Refusal{
	"TIMELINE_REVISION_CONFLICT": {409, "timeline_conflict", "Lịch trình đã được sửa. Tải bản mới trước khi lưu."},
	"ITINERARY_UPGRADE_REQUIRED": {409, "itinerary_upgrade_required", "Mở bản app mới để sửa lịch trình này."},
	"STOP_NOT_FOUND":             {422, "stop_not_found", "Chặng không thuộc chuyến đi này."},
	"OUTING_NOT_FOUND":           {404, "outing_not_found", "Không tìm thấy chuyến đi."},
}

func translateItineraryConflict(err error) error {
	conflict, ok := err.(*Conflict)
	if !ok {
		return err
	}
	if mapped, found := itineraryConflict[conflict.Code]; found {
		return &mapped
	}
	return err
}

// ReplaceOutingTimeline is replace_outing_timeline.
func ReplaceOutingTimeline(s Store, outingID string, request TimelineRequest, actor Actor) (OutingView, error) {
	record, err := s.GetOuting(outingID)
	if err != nil {
		return OutingView{}, err
	}
	if record == nil {
		return OutingView{}, refusal(404, "outing_not_found", "Outing does not exist")
	}
	proven, err := groupMember(s, record.ContextID, actor.ID)
	if err != nil {
		return OutingView{}, err
	}
	if err := requirePermission("edit_outing_timeline", actor, proven...); err != nil {
		return OutingView{}, err
	}
	for _, stop := range request.Stops {
		// A stop may name a catalogue place; a key the catalogue does not
		// know is a client bug, refused before anything is written.
		if stop.PlaceID == nil {
			continue
		}
		place, err := s.GetPlace(*stop.PlaceID)
		if err != nil {
			return OutingView{}, err
		}
		if place == nil {
			return OutingView{}, refusal(422, "stop_place_unknown", "Chặng nêu một địa điểm không có trong danh mục.")
		}
	}
	stops := make([]StopDraft, 0, len(request.Stops))
	for _, stop := range request.Stops {
		minute, err := minuteOfDay(stop.At)
		if err != nil {
			return OutingView{}, err
		}
		stops = append(stops, StopDraft{
			MinuteOfDay: minute,
			Label:       stop.Label,
			PlaceName:   stop.PlaceName,
			PlaceID:     stop.PlaceID,
		})
	}
	saved, err := s.ReplaceOutingStops(outingID, stops, request.ExpectedRevision)
	if err != nil {
		return OutingView{}, translateItineraryConflict(err)
	}
	return wireOuting(saved)
}

// AuthorizeOutingItinerary is authorize_outing_itinerary: the outing, once the
// actor has been shown to be in its group. It is a method of its own because
// an idempotent replay of PUT /outings/{id}/itinerary still has to pass this
// door before the stored answer is handed back.
func AuthorizeOutingItinerary(s Store, outingID string, actor Actor) (*Outing, error) {
	record, err := s.GetOuting(outingID)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, refusal(404, "outing_not_found", "Không tìm thấy chuyến đi.")
	}
	proven, err := groupMember(s, record.ContextID, actor.ID)
	if err != nil {
		return nil, err
	}
	if err := requirePermission("edit_outing_timeline", actor, proven...); err != nil {
		return nil, err
	}
	return record, nil
}

// itineraryDraft is _itinerary_draft: the outing, and the request normalised
// into the draft the preview reads -- every stop's coordinates resolved and
// every check-in already known.
func itineraryDraft(s Store, outingID string, request ItineraryRequest, actor Actor) (*Outing, itinerary.Draft, error) {
	record, err := AuthorizeOutingItinerary(s, outingID, actor)
	if err != nil {
		return nil, itinerary.Draft{}, err
	}
	if request.ExpectedRevision != record.TimelineRevision {
		return nil, itinerary.Draft{}, refusal(409, "timeline_conflict", "Lịch trình đã được sửa. Tải bản mới trước khi tiếp tục.")
	}
	knownIDs := make(map[string]bool, len(record.Stops))
	for _, stop := range record.Stops {
		knownIDs[stop.ID] = true
	}
	dayKeys := make(map[string]bool, len(request.Days))
	for _, day := range request.Days {
		dayKeys[day.Day.ISOFormat()] = true
	}
	for _, day := range request.Days {
		if !within(record, day.Day) {
			return nil, itinerary.Draft{}, refusal(422, "itinerary_day_invalid", "Ngày nằm ngoài chuyến đi.")
		}
		ids := map[string]bool{}
		for _, stop := range request.Stops {
			if stop.Day != nil && stop.Day.Compare(day.Day) == 0 {
				ids[stop.ID] = true
			}
		}
		for _, anchor := range [2]*string{day.StartStopID, day.EndStopID} {
			// `anchor and anchor not in ids`: None and "" are both "no
			// anchor", and the schema allows the empty string.
			if anchor != nil && *anchor != "" && !ids[*anchor] {
				return nil, itinerary.Draft{}, refusal(422, "itinerary_anchor_invalid", "Điểm đầu/cuối phải thuộc ngày này.")
			}
		}
	}
	checkins, err := s.ListOutingCheckins(outingID)
	if err != nil {
		return nil, itinerary.Draft{}, err
	}
	checked := make(map[string]bool, len(checkins))
	for _, checkin := range checkins {
		checked[checkin.StopID] = true
	}
	draft := itinerary.Draft{
		ExpectedRevision:  request.ExpectedRevision,
		IncludeSuggestion: request.IncludeSuggestion,
		Stops:             make([]itinerary.Stop, 0, len(request.Stops)),
		Days:              make([]itinerary.DayConfig, 0, len(request.Days)),
	}
	if request.Day != nil {
		draft.Day = request.Day.ISOFormat()
	}
	for _, day := range request.Days {
		draft.Days = append(draft.Days, itinerary.DayConfig{
			Day:           day.Day.ISOFormat(),
			TransportMode: day.TransportMode,
			StartAt:       day.StartAt,
			StartStopID:   day.StartStopID,
			EndStopID:     day.EndStopID,
			ReturnToStart: day.ReturnToStart,
		})
	}
	for _, stop := range request.Stops {
		if !startsWithTmp(stop.ID) && !knownIDs[stop.ID] {
			return nil, itinerary.Draft{}, refusal(422, "stop_not_found", "Chặng không thuộc chuyến đi này.")
		}
		if stop.Day != nil && (!within(record, *stop.Day) || !dayKeys[stop.Day.ISOFormat()]) {
			return nil, itinerary.Draft{}, refusal(422, "itinerary_day_invalid", "Chọn cấu hình cho ngày của chặng.")
		}
		var place *Place
		if stop.PlaceID != nil && *stop.PlaceID != "" {
			place, err = s.GetPlace(*stop.PlaceID)
			if err != nil {
				return nil, itinerary.Draft{}, err
			}
			if place == nil {
				return nil, itinerary.Draft{}, refusal(422, "stop_place_unknown", "Địa điểm không còn trong danh mục.")
			}
		}
		row := itinerary.Stop{
			ID:           stop.ID,
			At:           stop.At,
			Label:        stop.Label,
			PlaceName:    stop.PlaceName,
			PlaceID:      stop.PlaceID,
			TimeLocked:   stop.TimeLocked,
			MeetingPoint: meetingPoint(stop.MeetingPoint),
			CheckedIn:    checked[stop.ID],
		}
		if stop.Day != nil {
			day := stop.Day.ISOFormat()
			row.Day = &day
		}
		if stop.DurationMinutes != nil {
			row.DurationMinutes = *stop.DurationMinutes
		}
		switch {
		case place != nil:
			row.Lat, row.Lng = optionalFloat(place.Lat), optionalFloat(place.Lng)
		case stop.MeetingPoint != nil:
			row.Lat, row.Lng = stop.MeetingPoint.Lat, stop.MeetingPoint.Lng
		}
		draft.Stops = append(draft.Stops, row)
	}
	return record, draft, nil
}

// PreviewOutingItinerary is preview_outing_itinerary up to its last line: the
// draft `preview_itinerary(draft)` is about to be handed. The route makes that
// call, because it is the one that may open a socket.
func PreviewOutingItinerary(s Store, outingID string, request ItineraryRequest, actor Actor) (itinerary.Draft, error) {
	record, draft, err := itineraryDraft(s, outingID, request, actor)
	if err != nil {
		return itinerary.Draft{}, err
	}
	if request.Day == nil || !within(record, *request.Day) {
		return itinerary.Draft{}, refusal(422, "itinerary_day_invalid", "Ngày nằm ngoài chuyến đi.")
	}
	return draft, nil
}

// ReplaceOutingItinerary is replace_outing_itinerary.
func ReplaceOutingItinerary(s Store, outingID string, request ItineraryRequest, actor Actor) (OutingView, error) {
	if _, _, err := itineraryDraft(s, outingID, request, actor); err != nil {
		return OutingView{}, err
	}
	stops := make([]ItineraryStopDraft, 0, len(request.Stops))
	for _, stop := range request.Stops {
		minute, err := minuteOfDay(stop.At)
		if err != nil {
			return OutingView{}, err
		}
		stops = append(stops, ItineraryStopDraft{
			Label:           stop.Label,
			PlaceName:       stop.PlaceName,
			PlaceID:         stop.PlaceID,
			ID:              stop.ID,
			Day:             stop.Day,
			DurationMinutes: stop.DurationMinutes,
			TimeLocked:      stop.TimeLocked,
			MeetingPoint:    stop.MeetingPoint,
			MinuteOfDay:     minute,
		})
	}
	saved, err := s.ReplaceOutingItinerary(outingID, stops, request.Days, request.ExpectedRevision)
	if err != nil {
		return OutingView{}, translateItineraryConflict(err)
	}
	return wireOuting(saved)
}

// CheckInToStop is check_in_to_stop. F46: the only things a check-in records
// are who pressed it and when, both of which the server already knows.
func CheckInToStop(s Store, stopID string, actor Actor, now time.Time) (CheckinView, error) {
	_, outing, err := s.GetOutingStop(stopID)
	if err != nil {
		return CheckinView{}, err
	}
	if outing == nil {
		return CheckinView{}, refusal(404, "stop_not_found", "Stop does not exist")
	}
	proven, err := groupMember(s, outing.ContextID, actor.ID)
	if err != nil {
		return CheckinView{}, err
	}
	if err := requirePermission("check_in_to_stop", actor, proven...); err != nil {
		return CheckinView{}, err
	}
	record, err := s.CreateStopCheckin(stopID, actor.ID, now)
	if err != nil {
		if conflict, ok := err.(*Conflict); ok {
			switch conflict.Code {
			case "ALREADY_CHECKED_IN":
				return CheckinView{}, refusal(409, "already_checked_in", "You have already checked in at this stop")
			case "STOP_NOT_FOUND":
				return CheckinView{}, refusal(404, "stop_not_found", "Chặng đã được bỏ khỏi lịch trình.")
			}
		}
		return CheckinView{}, err
	}
	return wireCheckin(s, record)
}

// ListOutingCheckins is list_outing_checkins. One get_person per check-in, as
// `_wire_stop_checkin` asks for.
func ListOutingCheckins(s Store, outingID string, actor Actor) (CheckinListView, error) {
	outing, err := s.GetOuting(outingID)
	if err != nil {
		return CheckinListView{}, err
	}
	if outing == nil {
		return CheckinListView{}, refusal(404, "outing_not_found", "Outing does not exist")
	}
	proven, err := groupMember(s, outing.ContextID, actor.ID)
	if err != nil {
		return CheckinListView{}, err
	}
	if err := requirePermission("view_stop_checkins", actor, proven...); err != nil {
		return CheckinListView{}, err
	}
	records, err := s.ListOutingCheckins(outingID)
	if err != nil {
		return CheckinListView{}, err
	}
	view := CheckinListView{OutingID: outingID, Checkins: make([]CheckinView, 0, len(records))}
	for _, record := range records {
		row, err := wireCheckin(s, record)
		if err != nil {
			return CheckinListView{}, err
		}
		view.Checkins = append(view.Checkins, row)
	}
	return view, nil
}

// within is `record.starts_on <= day <= record.ends_on`.
func within(record *Outing, day Date) bool {
	return record.StartsOn.Compare(day) <= 0 && day.Compare(record.EndsOn) <= 0
}

// startsWithTmp is `stop.id.startswith("tmp-")`.
func startsWithTmp(id string) bool {
	return len(id) >= 4 && id[:4] == "tmp-"
}

// minuteOfDay is _minute_of_day: a wall-clock time of day that never passes
// through a datetime the server could shift. The split needs exactly two
// parts -- it is a list, so an unpack of three fails before int() is called --
// and each part is read by Python's int().
func minuteOfDay(value string) (int64, error) {
	parts := strings.Split(value, ":")
	if len(parts) < 2 {
		return 0, &journey.ValueError{Message: "not enough values to unpack (expected 2, got " + strconv.Itoa(len(parts)) + ")"}
	}
	if len(parts) > 2 {
		return 0, &journey.ValueError{Message: "too many values to unpack (expected 2)"}
	}
	hour, err := journey.PyInt(parts[0])
	if err != nil {
		return 0, err
	}
	minute, err := journey.PyInt(parts[1])
	if err != nil {
		return 0, err
	}
	// No range check, unlike app/domain/journey.py's minute(): the pattern on
	// the field is what keeps "99:99" from arriving, and this helper trusts it.
	return hour*60 + minute, nil
}

func meetingPoint(point *MeetingPoint) *itinerary.MeetingPoint {
	if point == nil {
		return nil
	}
	return &itinerary.MeetingPoint{Lat: point.Lat, Lng: point.Lng, Label: point.Label}
}

func optionalFloat(value *float64) any {
	if value == nil {
		return nil
	}
	return *value
}

// isPair is `is_pair(record.kind)`.
func isPair(kind string) bool { return direct.IsPair(kind) }

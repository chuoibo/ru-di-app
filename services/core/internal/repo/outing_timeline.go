package repo

// The W7 slice of SqlAlchemyApiRepository: the plan of a trip (its stops) and
// the arrivals recorded against it.
//
// Both replace methods reproduce a SQLAlchemy unit of work rather than a
// single statement, so three of its rules decide the statements here:
//
//   - a flush emits UPDATEs of one table in the order the session loaded the
//     rows, never in the order the method assigned to them, and an UPDATE's
//     SET carries only the columns whose value differs from the one that was
//     loaded, in table order;
//   - within a flush the parent table goes first (outings before
//     outing_stops), and for one table the UPDATEs go before the INSERTs;
//   - new rows are one INSERT per row (a true executemany) unless a column
//     left to a server default has to be read back, and then they are one
//     INSERT with one VALUES tuple per row and a RETURNING of that column
//     (insertmanyvalues), which also returns the primary key as its sentinel.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// outingColumns is `select(Outing)`: every mapped column in declaration order.
const outingColumns = `outings.id, outings.context_id, outings.created_by_id, outings.timeline_revision,
	        outings.itinerary_version, outings.itinerary_days, outings.title, outings.starts_on,
	        outings.ends_on, outings.headcount, outings.budget_per_person_vnd, outings.created_at`

// outingStopColumns is `select(OutingStop)`.
const outingStopColumns = `outing_stops.id, outing_stops.outing_id, outing_stops.position, outing_stops.minute_of_day,
	        outing_stops.label, outing_stops.place_name, outing_stops.place_id, outing_stops.day,
	        outing_stops.duration_minutes, outing_stops.time_locked, outing_stops.meeting_lat,
	        outing_stops.meeting_lng, outing_stops.meeting_label`

// outingStopColumnsByID is `session.get(OutingStop, id)`: the same columns
// labelled table_column, which is how the ORM renders a load by primary key.
const outingStopColumnsByID = `outing_stops.id AS outing_stops_id, outing_stops.outing_id AS outing_stops_outing_id,
	        outing_stops.position AS outing_stops_position, outing_stops.minute_of_day AS outing_stops_minute_of_day,
	        outing_stops.label AS outing_stops_label, outing_stops.place_name AS outing_stops_place_name,
	        outing_stops.place_id AS outing_stops_place_id, outing_stops.day AS outing_stops_day,
	        outing_stops.duration_minutes AS outing_stops_duration_minutes,
	        outing_stops.time_locked AS outing_stops_time_locked, outing_stops.meeting_lat AS outing_stops_meeting_lat,
	        outing_stops.meeting_lng AS outing_stops_meeting_lng, outing_stops.meeting_label AS outing_stops_meeting_label`

// outingStopInsert is the INSERT of a stop whose time_locked is left to the
// server default: every other column, with the psycopg bind casts.
var outingStopInsert = []insertColumn{{"id", "::UUID"}, {"outing_id", "::UUID"}, {"position", "::INTEGER"},
	{"minute_of_day", "::INTEGER"}, {"label", "::VARCHAR"}, {"place_name", "::VARCHAR"}, {"place_id", "::VARCHAR"},
	{"day", "::DATE"}, {"duration_minutes", "::INTEGER"}, {"meeting_lat", ""}, {"meeting_lng", ""},
	{"meeting_label", "::VARCHAR"}}

// outingStopInsertLocked is the same INSERT with time_locked written, which is
// what an itinerary save does; nothing is left to a default, so no RETURNING.
var outingStopInsertLocked = []insertColumn{{"id", "::UUID"}, {"outing_id", "::UUID"}, {"position", "::INTEGER"},
	{"minute_of_day", "::INTEGER"}, {"label", "::VARCHAR"}, {"place_name", "::VARCHAR"}, {"place_id", "::VARCHAR"},
	{"day", "::DATE"}, {"duration_minutes", "::INTEGER"}, {"time_locked", ""}, {"meeting_lat", ""},
	{"meeting_lng", ""}, {"meeting_label", "::VARCHAR"}}

// StopCheckin is StopCheckinRecord.
type StopCheckin struct {
	ID        string
	StopID    string
	PersonID  string
	CreatedAt time.Time
}

// TimelineStop is one element of replace_outing_stops's `stops`: what the
// timeline body says, with no stop id in it.
type TimelineStop struct {
	MinuteOfDay int64
	Label       string
	PlaceName   *string
	PlaceID     *string
}

// ItineraryStop is one element of replace_outing_itinerary's `stops`.
// ID is either a stop's UUID or a draft key beginning "tmp-".
type ItineraryStop struct {
	ID              string
	MinuteOfDay     int64
	Label           string
	PlaceName       *string
	PlaceID         *string
	Day             *time.Time
	DurationMinutes *int64
	TimeLocked      bool
	MeetingLat      *float64
	MeetingLng      *float64
	MeetingLabel    *string

	// position is where the request puts this stop; the body does not carry
	// it, `enumerate(stops)` does.
	position int64
}

// ListOutings is list_outings: the context's outings by (starts_on, id), then
// `_outing_record`'s stops for each in that order.
func (r Repository) ListOutings(ctx context.Context, contextID string) ([]Outing, error) {
	rows, err := r.Q.Query(ctx,
		`SELECT `+outingColumns+`
		   FROM outings
		  WHERE outings.context_id = $1::UUID
		  ORDER BY outings.starts_on, outings.id`, contextID)
	if err != nil {
		return nil, err
	}
	out := []Outing{}
	for rows.Next() {
		o, err := scanOutingRow(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, *o)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		if out[i].Stops, err = r.outingStops(ctx, out[i].ID); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// GetOutingStop is get_outing_stop: the stop by primary key, then the outing
// its `outing_id` names, read exactly as get_outing reads it. Nil, and no
// second statement, when there is no stop.
func (r Repository) GetOutingStop(ctx context.Context, stopID string) (*OutingStop, *Outing, error) {
	stop, err := r.stopByID(ctx, stopID)
	if err != nil || stop == nil {
		return nil, nil, err
	}
	outing, err := r.GetOuting(ctx, stop.outingID)
	if err != nil || outing == nil {
		return nil, nil, err
	}
	return &stop.OutingStop, outing, nil
}

// ListOutingCheckins is list_outing_checkins: the arrivals of every stop of
// one outing, by (created_at, id).
func (r Repository) ListOutingCheckins(ctx context.Context, outingID string) ([]StopCheckin, error) {
	rows, err := r.Q.Query(ctx,
		`SELECT outing_stop_checkins.id, outing_stop_checkins.stop_id, outing_stop_checkins.person_id,
		        outing_stop_checkins.created_at
		   FROM outing_stop_checkins
		   JOIN outing_stops ON outing_stops.id = outing_stop_checkins.stop_id
		  WHERE outing_stops.outing_id = $1::UUID
		  ORDER BY outing_stop_checkins.created_at, outing_stop_checkins.id`, outingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []StopCheckin{}
	for rows.Next() {
		var c StopCheckin
		if err := rows.Scan(&c.ID, &c.StopID, &c.PersonID, &c.CreatedAt); err != nil {
			return nil, err
		}
		c.CreatedAt = c.CreatedAt.UTC()
		out = append(out, c)
	}
	return out, rows.Err()
}

const checkinSavepoint = "sa_savepoint_1"

// CreateStopCheckin is create_stop_checkin.
//
// Statements, in Python's order:
//  1. the stop by primary key; Conflict STOP_NOT_FOUND when there is none;
//  2. its outing FOR UPDATE, so two phones pressing the button serialise on
//     the row whose revision they are both about to bump;
//  3. the stop's id again, because a concurrent editor may have removed it
//     while this request waited for the lock -- Conflict STOP_NOT_FOUND then;
//  4. SAVEPOINT, the INSERT, and RELEASE. A duplicate is left to
//     uq_outing_stop_checkins_person rather than asked about first: the
//     question and the answer would have a race between them. The savepoint
//     is rolled back and not released when the INSERT fails, and a violation
//     of that index is Conflict ALREADY_CHECKED_IN;
//  5. the outing's revision, one greater, so a timeline a phone is holding
//     knows an arrival has landed on it.
func (r Repository) CreateStopCheckin(ctx context.Context, stopID, personID string, now time.Time) (StopCheckin, error) {
	stop, err := r.stopByID(ctx, stopID)
	if err != nil {
		return StopCheckin{}, err
	}
	if stop == nil {
		return StopCheckin{}, &Conflict{Code: "STOP_NOT_FOUND"}
	}
	outing, err := r.lockOuting(ctx, stop.outingID)
	if err != nil {
		return StopCheckin{}, err
	}
	// `outing is None or ... is None`: Python stops at the left operand, so a
	// vanished outing issues no second statement. A foreign key keeps that
	// side unreachable; the order is kept because the branch is the Python's.
	if outing == nil {
		return StopCheckin{}, &Conflict{Code: "STOP_NOT_FOUND"}
	}
	var stillThere string
	err = r.Q.QueryRow(ctx,
		`SELECT outing_stops.id FROM outing_stops WHERE outing_stops.id = $1::UUID`, stopID).Scan(&stillThere)
	if errors.Is(err, pgx.ErrNoRows) {
		return StopCheckin{}, &Conflict{Code: "STOP_NOT_FOUND"}
	}
	if err != nil {
		return StopCheckin{}, err
	}
	id, err := newUUID()
	if err != nil {
		return StopCheckin{}, err
	}
	created := pythonInstant(now)
	if _, err := r.Q.Exec(ctx, `SAVEPOINT `+checkinSavepoint); err != nil {
		return StopCheckin{}, err
	}
	_, insertErr := r.Q.Exec(ctx,
		`INSERT INTO outing_stop_checkins (id, stop_id, person_id, created_at)
		 VALUES ($1::UUID, $2::UUID, $3::UUID, $4::TIMESTAMP WITH TIME ZONE)`,
		id, stopID, personID, created)
	if insertErr != nil {
		if _, err := r.Q.Exec(ctx, `ROLLBACK TO SAVEPOINT `+checkinSavepoint); err != nil {
			return StopCheckin{}, err
		}
		if pg := integrityViolation(insertErr); pg != nil && pg.ConstraintName == "uq_outing_stop_checkins_person" {
			return StopCheckin{}, &Conflict{Code: "ALREADY_CHECKED_IN", Err: pg}
		}
		return StopCheckin{}, insertErr
	}
	if _, err := r.Q.Exec(ctx, `RELEASE SAVEPOINT `+checkinSavepoint); err != nil {
		return StopCheckin{}, err
	}
	if err := r.execUpdate(ctx,
		`UPDATE outings SET timeline_revision=$1::INTEGER WHERE outings.id = $2::UUID`,
		sqlInteger(outing.TimelineRevision+1), outing.ID); err != nil {
		return StopCheckin{}, err
	}
	return StopCheckin{ID: id, StopID: stopID, PersonID: personID, CreatedAt: created}, nil
}

// ReplaceOutingStops is replace_outing_stops: the version 1 timeline, saved
// whole.
//
// A stop that still says the same time, label and place is the same stop and
// keeps its row, so the arrivals hanging off it survive an edit that never
// touched it. The request carries no stop ids, so identity is read off what a
// stop says and a retyped stop reads as a different stop.
//
// Statements, in Python's order:
//  1. the outing FOR UPDATE; Conflict OUTING_NOT_FOUND, TIMELINE_REVISION_
//     CONFLICT or ITINERARY_UPGRADE_REQUIRED with nothing else issued;
//  2. its stops by position;
//  3. one DELETE per stop no request line claimed;
//  4. one UPDATE per surviving stop, parking it above both the old plan and
//     the new one -- uq_outing_stops_position is not deferrable and the ORM
//     moves one row at a time, so a stop cannot walk straight into a position
//     another survivor still occupies;
//  5. one UPDATE per surviving stop bringing it down to its new position,
//     carrying place_id when that changed and nothing else: attaching a
//     catalogue place is not a new stop;
//  6. the INSERT of the stops nobody claimed, reading back the time_locked
//     the server defaulted;
//  7. the outing's revision, one greater;
//  8. `_outing_record`'s stops by position.
func (r Repository) ReplaceOutingStops(ctx context.Context, outingID string, stops []TimelineStop,
	expectedRevision *int64) (Outing, error) {
	outing, err := r.lockOuting(ctx, outingID)
	if err != nil {
		return Outing{}, err
	}
	if outing == nil {
		return Outing{}, &Conflict{Code: "OUTING_NOT_FOUND"}
	}
	if expectedRevision != nil && outing.TimelineRevision != *expectedRevision {
		return Outing{}, &Conflict{Code: "TIMELINE_REVISION_CONFLICT"}
	}
	if outing.ItineraryVersion == 2 {
		return Outing{}, &Conflict{Code: "ITINERARY_UPGRADE_REQUIRED"}
	}
	existing, err := r.stopRows(ctx, outingID, ` ORDER BY outing_stops.position`)
	if err != nil {
		return Outing{}, err
	}

	// `unclaimed`: the surviving rows of each (minute_of_day, label,
	// place_name), first come first served, exactly as Python pops them.
	unclaimed := map[string][]int{}
	for i, row := range existing {
		key := stopIdentity(row.MinuteOfDay, row.Label, row.PlaceName)
		unclaimed[key] = append(unclaimed[key], i)
	}
	kept := []int{}         // index into `existing`, in new-position order
	keptAt := map[int]int{} // index into `existing` -> its new position
	added := []int{}        // index into `stops`
	for position, stop := range stops {
		key := stopIdentity(stop.MinuteOfDay, stop.Label, stop.PlaceName)
		if same := unclaimed[key]; len(same) > 0 {
			unclaimed[key] = same[1:]
			keptAt[same[0]] = position
			kept = append(kept, same[0])
			continue
		}
		added = append(added, position)
	}

	parking := int64(-1)
	for _, row := range existing {
		if row.Position > parking {
			parking = row.Position
		}
	}
	if last := int64(len(stops)) - 1; last > parking {
		parking = last
	}
	parking++

	claimed := map[int]bool{}
	for _, i := range kept {
		claimed[i] = true
	}
	for i, row := range existing {
		if claimed[i] {
			continue
		}
		if _, err := r.Q.Exec(ctx,
			`DELETE FROM outing_stops WHERE outing_stops.id = $1::UUID`, row.ID); err != nil {
			return Outing{}, err
		}
	}
	// The parked positions follow the new order, the statements the load
	// order: a flush walks the session's rows, not the method's list.
	parked := map[int]int64{}
	for offset, i := range kept {
		parked[i] = parking + int64(offset)
	}
	for i := range existing {
		if !claimed[i] {
			continue
		}
		if err := r.execUpdate(ctx,
			`UPDATE outing_stops SET position=$1::INTEGER WHERE outing_stops.id = $2::UUID`,
			sqlInteger(parked[i]), existing[i].ID); err != nil {
			return Outing{}, err
		}
	}
	for i := range existing {
		if !claimed[i] {
			continue
		}
		position := keptAt[i]
		placeID := stops[position].PlaceID
		set := []string{"position=$1::INTEGER"}
		args := []any{sqlInteger(int64(position))}
		if !sameText(existing[i].PlaceID, placeID) {
			set = append(set, fmt.Sprintf("place_id=$%d::VARCHAR", len(args)+1))
			args = append(args, placeID)
		}
		args = append(args, existing[i].ID)
		if err := r.execUpdate(ctx, `UPDATE outing_stops SET `+joinComma(set)+
			fmt.Sprintf(` WHERE outing_stops.id = $%d::UUID`, len(args)), args...); err != nil {
			return Outing{}, err
		}
		existing[i].Position = int64(position)
		existing[i].PlaceID = placeID
	}

	// A one-day trip puts every new stop on its only day; a longer one has
	// nothing to say about which day a version 1 stop belongs to.
	var day *time.Time
	if outing.StartsOn.Equal(outing.EndsOn) {
		start := outing.StartsOn
		day = &start
	}
	if len(added) > 0 {
		rows := make([][]any, 0, len(added))
		for _, position := range added {
			id, err := newUUID()
			if err != nil {
				return Outing{}, err
			}
			stop := stops[position]
			rows = append(rows, []any{id, outingID, sqlInteger(int64(position)), sqlInteger(stop.MinuteOfDay),
				stop.Label, stop.PlaceName, stop.PlaceID, day, nil, nil, nil, nil})
		}
		if err := r.insertStopsReturningLock(ctx, rows); err != nil {
			return Outing{}, err
		}
	}

	outing.TimelineRevision++
	if err := r.execUpdate(ctx,
		`UPDATE outings SET timeline_revision=$1::INTEGER WHERE outings.id = $2::UUID`,
		sqlInteger(outing.TimelineRevision), outing.ID); err != nil {
		return Outing{}, err
	}
	if outing.Stops, err = r.outingStops(ctx, outingID); err != nil {
		return Outing{}, err
	}
	return *outing, nil
}

// ReplaceOutingItinerary is replace_outing_itinerary: the version 2 itinerary,
// saved whole, with the stop ids the client echoes preserved.
//
// Statements, in Python's order:
//  1. the outing FOR UPDATE; Conflict OUTING_NOT_FOUND or
//     TIMELINE_REVISION_CONFLICT with nothing else issued;
//  2. its stops in no order (the rows become a dict keyed by id);
//  3. Conflict STOP_NOT_FOUND when the request names a stop of another trip,
//     after that read and before any write;
//  4. one DELETE per stop the request dropped;
//  5. one UPDATE per stop the request kept, parking it clear of the new
//     positions;
//  6. the outing: its revision one greater, its version 2 if it was 1, and
//     the days, each only if the value differs from the one that was loaded
//     -- the parent table goes first in the flush;
//  7. one UPDATE per kept stop, carrying every column whose value changed;
//  8. one INSERT per stop the request invented, time_locked written, so no
//     default has to be read back;
//  9. `_outing_record`'s stops by position.
func (r Repository) ReplaceOutingItinerary(ctx context.Context, outingID string, stops []ItineraryStop,
	itineraryDays []json.RawMessage, expectedRevision int64) (Outing, error) {
	outing, err := r.lockOuting(ctx, outingID)
	if err != nil {
		return Outing{}, err
	}
	if outing == nil {
		return Outing{}, &Conflict{Code: "OUTING_NOT_FOUND"}
	}
	if outing.TimelineRevision != expectedRevision {
		return Outing{}, &Conflict{Code: "TIMELINE_REVISION_CONFLICT"}
	}
	existing, err := r.stopRows(ctx, outingID, "")
	if err != nil {
		return Outing{}, err
	}
	at := map[string]int{}
	for i, row := range existing {
		at[row.ID] = i
	}
	requested := map[string]bool{}
	for _, stop := range stops {
		if isDraftStopID(stop.ID) {
			continue
		}
		if _, ok := at[stop.ID]; !ok {
			return Outing{}, &Conflict{Code: "STOP_NOT_FOUND"}
		}
		requested[stop.ID] = true
	}
	for _, row := range existing {
		if requested[row.ID] {
			continue
		}
		if _, err := r.Q.Exec(ctx,
			`DELETE FROM outing_stops WHERE outing_stops.id = $1::UUID`, row.ID); err != nil {
			return Outing{}, err
		}
	}
	parking := int64(len(stops))
	for _, row := range existing {
		if row.Position > parking {
			parking = row.Position
		}
	}
	parking++
	offset := int64(0)
	for i := range existing {
		if !requested[existing[i].ID] {
			continue
		}
		if err := r.execUpdate(ctx,
			`UPDATE outing_stops SET position=$1::INTEGER WHERE outing_stops.id = $2::UUID`,
			sqlInteger(parking+offset), existing[i].ID); err != nil {
			return Outing{}, err
		}
		offset++
	}

	// What each row is about to say, and the ids the draft keys became.
	want := map[string]ItineraryStop{}
	newIDs := map[string]string{}
	fresh := []ItineraryStop{}
	freshIDs := []string{}
	for position, stop := range stops {
		stop.position = int64(position)
		if isDraftStopID(stop.ID) {
			id, err := newUUID()
			if err != nil {
				return Outing{}, err
			}
			newIDs[stop.ID] = id
			stop.ID = id
			fresh = append(fresh, stop)
			freshIDs = append(freshIDs, id)
			continue
		}
		want[stop.ID] = stop
	}
	days := make([]json.RawMessage, 0, len(itineraryDays))
	for _, day := range itineraryDays {
		rewritten, err := rewriteItineraryDay(day, newIDs)
		if err != nil {
			return Outing{}, err
		}
		days = append(days, rewritten)
	}

	set := []string{"timeline_revision=$1::INTEGER"}
	args := []any{sqlInteger(outing.TimelineRevision + 1)}
	if outing.ItineraryVersion != 2 {
		set = append(set, fmt.Sprintf("itinerary_version=$%d::INTEGER", len(args)+1))
		args = append(args, sqlInteger(2))
	}
	sameDays, err := sameJSONArray(outing.ItineraryDays, days)
	if err != nil {
		return Outing{}, err
	}
	if !sameDays {
		payload, err := json.Marshal(days)
		if err != nil {
			return Outing{}, err
		}
		set = append(set, fmt.Sprintf("itinerary_days=$%d::JSONB", len(args)+1))
		args = append(args, string(payload))
	}
	args = append(args, outing.ID)
	if err := r.execUpdate(ctx, `UPDATE outings SET `+joinComma(set)+
		fmt.Sprintf(` WHERE outings.id = $%d::UUID`, len(args)), args...); err != nil {
		return Outing{}, err
	}
	outing.TimelineRevision++
	outing.ItineraryVersion = 2
	outing.ItineraryDays = days

	for i := range existing {
		if !requested[existing[i].ID] {
			continue
		}
		if err := r.updateItineraryStop(ctx, existing[i], want[existing[i].ID]); err != nil {
			return Outing{}, err
		}
	}
	if len(fresh) > 0 {
		rows := make([][]any, 0, len(fresh))
		for i, stop := range fresh {
			rows = append(rows, []any{freshIDs[i], outingID, sqlInteger(stop.position),
				sqlInteger(stop.MinuteOfDay), stop.Label, stop.PlaceName, stop.PlaceID, stop.Day,
				optionalInteger(stop.DurationMinutes), stop.TimeLocked, stop.MeetingLat, stop.MeetingLng,
				stop.MeetingLabel})
		}
		if err := r.insertEach(ctx, "outing_stops", outingStopInsertLocked, rows); err != nil {
			return Outing{}, err
		}
	}
	if outing.Stops, err = r.outingStops(ctx, outingID); err != nil {
		return Outing{}, err
	}
	return *outing, nil
}

// updateItineraryStop is one row's share of the itinerary flush: the columns
// whose value differs from the loaded one, in table order. `position` always
// differs, because the row is parked above every new position first.
func (r Repository) updateItineraryStop(ctx context.Context, row *stopRow, want ItineraryStop) error {
	set := []string{}
	args := []any{}
	add := func(assignment string, cast string, value any) {
		set = append(set, fmt.Sprintf("%s=$%d%s", assignment, len(args)+1, cast))
		args = append(args, value)
	}
	add("position", "::INTEGER", sqlInteger(want.position))
	if row.MinuteOfDay != want.MinuteOfDay {
		add("minute_of_day", "::INTEGER", sqlInteger(want.MinuteOfDay))
	}
	if row.Label != want.Label {
		add("label", "::VARCHAR", want.Label)
	}
	if !sameText(row.PlaceName, want.PlaceName) {
		add("place_name", "::VARCHAR", want.PlaceName)
	}
	if !sameText(row.PlaceID, want.PlaceID) {
		add("place_id", "::VARCHAR", want.PlaceID)
	}
	if !sameDay(row.Day, want.Day) {
		add("day", "::DATE", want.Day)
	}
	if !sameInteger(row.DurationMinutes, want.DurationMinutes) {
		add("duration_minutes", "::INTEGER", optionalInteger(want.DurationMinutes))
	}
	if row.TimeLocked != want.TimeLocked {
		add("time_locked", "", want.TimeLocked)
	}
	if !sameFloat(row.MeetingLat, want.MeetingLat) {
		add("meeting_lat", "", want.MeetingLat)
	}
	if !sameFloat(row.MeetingLng, want.MeetingLng) {
		add("meeting_lng", "", want.MeetingLng)
	}
	if !sameText(row.MeetingLabel, want.MeetingLabel) {
		add("meeting_label", "::VARCHAR", want.MeetingLabel)
	}
	args = append(args, row.ID)
	return r.execUpdate(ctx, `UPDATE outing_stops SET `+joinComma(set)+
		fmt.Sprintf(` WHERE outing_stops.id = $%d::UUID`, len(args)), args...)
}

// insertStopsReturningLock is the flush of stops whose time_locked is left to
// the server default: eager defaults make it one INSERT of every row with a
// RETURNING, and more than one row adds the primary key as the sentinel that
// pairs a returned value back to its row.
func (r Repository) insertStopsReturningLock(ctx context.Context, rows [][]any) error {
	for start := 0; start < len(rows); start += insertManyValuesPageSize {
		page := rows[start:min(start+insertManyValuesPageSize, len(rows))]
		args := make([]any, 0, len(page)*len(outingStopInsert))
		for _, row := range page {
			args = append(args, row...)
		}
		sql := renderInsert("outing_stops", outingStopInsert, len(page)) + ` RETURNING outing_stops.time_locked`
		if len(page) > 1 {
			sql += `, outing_stops.id`
		}
		result, err := r.Q.Query(ctx, sql, args...)
		if err != nil {
			return err
		}
		for result.Next() {
		}
		result.Close()
		if err := result.Err(); err != nil {
			return err
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Reading rows
// ---------------------------------------------------------------------------

// stopRow is an OutingStop as the session holds it: the record's fields, the
// outing it belongs to, and the position the last flush left behind.
type stopRow struct {
	OutingStop
	outingID string
}

func (r Repository) stopByID(ctx context.Context, stopID string) (*stopRow, error) {
	var row stopRow
	err := r.Q.QueryRow(ctx,
		`SELECT `+outingStopColumnsByID+`
		   FROM outing_stops
		  WHERE outing_stops.id = $1::UUID`, stopID).
		Scan(&row.ID, &row.outingID, &row.Position, &row.MinuteOfDay, &row.Label, &row.PlaceName, &row.PlaceID,
			&row.Day, &row.DurationMinutes, &row.TimeLocked, &row.MeetingLat, &row.MeetingLng, &row.MeetingLabel)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r Repository) stopRows(ctx context.Context, outingID, order string) ([]*stopRow, error) {
	rows, err := r.Q.Query(ctx,
		`SELECT `+outingStopColumns+`
		   FROM outing_stops
		  WHERE outing_stops.outing_id = $1::UUID`+order, outingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*stopRow{}
	for rows.Next() {
		var row stopRow
		if err := rows.Scan(&row.ID, &row.outingID, &row.Position, &row.MinuteOfDay, &row.Label, &row.PlaceName,
			&row.PlaceID, &row.Day, &row.DurationMinutes, &row.TimeLocked, &row.MeetingLat, &row.MeetingLng,
			&row.MeetingLabel); err != nil {
			return nil, err
		}
		out = append(out, &row)
	}
	return out, rows.Err()
}

// lockOuting is `select(Outing).where(id).with_for_update()` with
// populate_existing: always a statement, nil when there is no row.
func (r Repository) lockOuting(ctx context.Context, outingID string) (*Outing, error) {
	rows, err := r.Q.Query(ctx,
		`SELECT `+outingColumns+`
		   FROM outings
		  WHERE outings.id = $1::UUID FOR UPDATE`, outingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, rows.Err()
	}
	o, err := scanOutingRow(rows)
	if err != nil {
		return nil, err
	}
	return o, rows.Err()
}

func scanOutingRow(rows pgx.Rows) (*Outing, error) {
	var o Outing
	var days []byte
	if err := rows.Scan(&o.ID, &o.ContextID, &o.CreatedByID, &o.TimelineRevision, &o.ItineraryVersion, &days,
		&o.Title, &o.StartsOn, &o.EndsOn, &o.Headcount, &o.BudgetPerPersonVND, &o.CreatedAt); err != nil {
		return nil, err
	}
	o.CreatedAt = o.CreatedAt.UTC()
	elements, err := jsonArrayElements(days)
	if err != nil {
		return nil, err
	}
	o.ItineraryDays = elements
	return &o, nil
}

// ---------------------------------------------------------------------------
// Small Python equivalences
// ---------------------------------------------------------------------------

func isDraftStopID(id string) bool { return len(id) >= 4 && id[:4] == "tmp-" }

// stopIdentity is Python's `(minute_of_day, label, place_name)` tuple key,
// with a place_name of None distinct from every string.
func stopIdentity(minute int64, label string, placeName *string) string {
	name := "\x00"
	if placeName != nil {
		name = "s" + *placeName
	}
	return fmt.Sprintf("%d\x00%s\x00%s", minute, label, name)
}

func joinComma(parts []string) string {
	out := ""
	for i, part := range parts {
		if i > 0 {
			out += ", "
		}
		out += part
	}
	return out
}

func sameInteger(a, b *int64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func sameFloat(a, b *float64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func sameDay(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Equal(*b)
}

func optionalInteger(n *int64) any {
	if n == nil {
		return nil
	}
	return sqlInteger(*n)
}

// rewriteItineraryDay is `{**day, **{key: new_ids.get(day.get(key), day.get(key))
// for key in ("start_stop_id", "end_stop_id")}}`: the day's keys in the order
// they arrived, both anchors resolved through the draft keys, and either
// anchor appended when the day did not carry it.
func rewriteItineraryDay(day json.RawMessage, newIDs map[string]string) (json.RawMessage, error) {
	pairs, err := jsonObjectPairs(day)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for i, pair := range pairs {
		seen[pair.key] = true
		if pair.key != "start_stop_id" && pair.key != "end_stop_id" {
			continue
		}
		var current any
		if err := json.Unmarshal(pair.value, &current); err != nil {
			return nil, ErrUnrepresentable
		}
		if text, ok := current.(string); ok {
			if replacement, ok := newIDs[text]; ok {
				encoded, err := json.Marshal(replacement)
				if err != nil {
					return nil, err
				}
				pairs[i].value = encoded
			}
		}
	}
	for _, key := range []string{"start_stop_id", "end_stop_id"} {
		if !seen[key] {
			pairs = append(pairs, jsonPair{key: key, value: json.RawMessage("null")})
		}
	}
	out := []byte("{")
	for i, pair := range pairs {
		if i > 0 {
			out = append(out, ',')
		}
		encoded, err := json.Marshal(pair.key)
		if err != nil {
			return nil, err
		}
		out = append(out, encoded...)
		out = append(out, ':')
		out = append(out, pair.value...)
	}
	return append(out, '}'), nil
}

type jsonPair struct {
	key   string
	value json.RawMessage
}

func jsonObjectPairs(raw json.RawMessage) ([]jsonPair, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if _, err := decoder.Token(); err != nil {
		return nil, ErrUnrepresentable
	}
	pairs := []jsonPair{}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, ErrUnrepresentable
		}
		key, ok := token.(string)
		if !ok {
			return nil, ErrUnrepresentable
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, ErrUnrepresentable
		}
		pairs = append(pairs, jsonPair{key: key, value: value})
	}
	return pairs, nil
}

// sameJSONArray is Python's `==` between the loaded itinerary_days and the
// list about to replace it: a mapping compares by its pairs whatever order
// they are in, and a number by its value.
func sameJSONArray(a, b []json.RawMessage) (bool, error) {
	if len(a) != len(b) {
		return false, nil
	}
	for i := range a {
		var left, right any
		if err := json.Unmarshal(a[i], &left); err != nil {
			return false, ErrUnrepresentable
		}
		if err := json.Unmarshal(b[i], &right); err != nil {
			return false, ErrUnrepresentable
		}
		if !jsonDeepEqual(left, right) {
			return false, nil
		}
	}
	return true, nil
}

func jsonDeepEqual(a, b any) bool {
	switch left := a.(type) {
	case map[string]any:
		right, ok := b.(map[string]any)
		if !ok || len(left) != len(right) {
			return false
		}
		for key, value := range left {
			other, ok := right[key]
			if !ok || !jsonDeepEqual(value, other) {
				return false
			}
		}
		return true
	case []any:
		right, ok := b.([]any)
		if !ok || len(left) != len(right) {
			return false
		}
		for i := range left {
			if !jsonDeepEqual(left[i], right[i]) {
				return false
			}
		}
		return true
	default:
		return a == b
	}
}

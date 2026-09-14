package repo

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// OutingStop is OutingStopRecord.
type OutingStop struct {
	ID              string
	Position        int64
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
}

// Outing is OutingRecord. ItineraryDays holds the elements of
// `tuple(outing.itinerary_days or [])` as PostgreSQL's JSON text; a CHECK keeps
// the column an array.
type Outing struct {
	ID                 string
	ContextID          string
	CreatedByID        string
	Title              string
	StartsOn           time.Time
	EndsOn             time.Time
	Headcount          int64
	BudgetPerPersonVND int64
	CreatedAt          time.Time
	Stops              []OutingStop
	TimelineRevision   int64
	ItineraryVersion   int64
	ItineraryDays      []json.RawMessage
}

// RecapOuting is RecapOutingRecord.
type RecapOuting struct {
	Outing        Outing
	InProgress    bool
	SplitTotalVND int64
	ExpenseCount  int64
	MemoryCount   int64
}

// GroupRecap is group_recap: the context's trips with starts_on <= today,
// ORDER BY ends_on DESC, id, each with money and memories recomputed.
//
// `today` is the calendar day the service computed from its own clock
// (WallClockDate); only its year, month and day are used. Statement order is
// Python's: outings, the money pass, the memory pass, then one outing_stops
// read per outing while the records are built. The money pass keeps only the
// newest version of each expense and folds occurred_at into Vietnam's day in
// PostgreSQL; SUM of a bigint is numeric (a Decimal in Python), converted to
// an integer exactly like `int(...)` and refused if it would not fit int64.
func (r Repository) GroupRecap(ctx context.Context, contextID string, today time.Time) ([]RecapOuting, error) {
	day := calendarDay(today)
	rows, err := r.Q.Query(ctx,
		`SELECT outings.id, outings.context_id, outings.created_by_id, outings.timeline_revision,
		        outings.itinerary_version, outings.itinerary_days, outings.title, outings.starts_on,
		        outings.ends_on, outings.headcount, outings.budget_per_person_vnd, outings.created_at
		   FROM outings
		  WHERE outings.context_id = $1::UUID AND outings.starts_on <= $2::DATE
		  ORDER BY outings.ends_on DESC, outings.id`,
		contextID, day)
	if err != nil {
		return nil, err
	}
	var outings []Outing
	for rows.Next() {
		var o Outing
		var days []byte
		if err := rows.Scan(&o.ID, &o.ContextID, &o.CreatedByID, &o.TimelineRevision,
			&o.ItineraryVersion, &days, &o.Title, &o.StartsOn,
			&o.EndsOn, &o.Headcount, &o.BudgetPerPersonVND, &o.CreatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		o.CreatedAt = o.CreatedAt.UTC()
		if o.ItineraryDays, err = jsonArrayElements(days); err != nil {
			rows.Close()
			return nil, err
		}
		outings = append(outings, o)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := []RecapOuting{}
	if len(outings) == 0 {
		return out, nil
	}
	ids := make([]string, len(outings))
	for i, o := range outings {
		ids[i] = o.ID
	}

	type money struct{ total, count int64 }
	spent := map[string]money{}
	rows, err = r.Q.Query(ctx,
		`SELECT outings.id AS outing_id,
		        coalesce(sum(anon_1.amount_vnd), $1::INTEGER) AS split_total_vnd,
		        count(distinct(anon_1.expense_id)) AS expense_count
		   FROM outings
		   LEFT OUTER JOIN (
		        SELECT expenses.id AS expense_id, confirmed_allocations.amount_vnd AS amount_vnd,
		               `+wallClockDate("$2", "expense_versions.occurred_at")+` AS on_date
		          FROM confirmed_allocations
		          JOIN expense_versions ON expense_versions.id = confirmed_allocations.expense_version_id
		          JOIN (SELECT expense_versions.expense_id AS expense_id,
		                       max(expense_versions.version_number) AS version_number
		                  FROM expense_versions GROUP BY expense_versions.expense_id) AS anon_2
		            ON anon_2.expense_id = expense_versions.expense_id
		           AND anon_2.version_number = expense_versions.version_number
		          JOIN expenses ON expenses.id = expense_versions.expense_id
		         WHERE expenses.context_id = $3::UUID) AS anon_1
		     ON anon_1.on_date BETWEEN outings.starts_on AND outings.ends_on
		  WHERE outings.id IN (`+uuidPlaceholders(4, len(ids))+`)
		  GROUP BY outings.id`,
		append([]any{0, WallClockZone, contextID}, uuidArgs(ids)...)...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id string
		var total pgtype.Numeric
		var count int64
		if err := rows.Scan(&id, &total, &count); err != nil {
			rows.Close()
			return nil, err
		}
		exact, err := total.Int64Value()
		if err != nil {
			rows.Close()
			return nil, err
		}
		spent[id] = money{total: exact.Int64, count: count}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	memories := map[string]int64{}
	rows, err = r.Q.Query(ctx,
		`SELECT outings.id AS outing_id, count(memories.id) AS memory_count
		   FROM outings
		   LEFT OUTER JOIN memories
		     ON memories.context_id = outings.context_id
		    AND `+wallClockDate("$1", "memories.created_at")+` BETWEEN outings.starts_on AND outings.ends_on
		  WHERE outings.id IN (`+uuidPlaceholders(2, len(ids))+`)
		  GROUP BY outings.id`,
		append([]any{WallClockZone}, uuidArgs(ids)...)...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id string
		var count int64
		if err := rows.Scan(&id, &count); err != nil {
			rows.Close()
			return nil, err
		}
		memories[id] = count
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for _, o := range outings {
		if o.Stops, err = r.outingStops(ctx, o.ID); err != nil {
			return nil, err
		}
		out = append(out, RecapOuting{
			Outing:        o,
			InProgress:    !o.EndsOn.Before(day),
			SplitTotalVND: spent[o.ID].total,
			ExpenseCount:  spent[o.ID].count,
			MemoryCount:   memories[o.ID],
		})
	}
	return out, nil
}

// outingStops is the stops half of _outing_record.
func (r Repository) outingStops(ctx context.Context, outingID string) ([]OutingStop, error) {
	rows, err := r.Q.Query(ctx,
		`SELECT outing_stops.id, outing_stops.outing_id, outing_stops.position, outing_stops.minute_of_day,
		        outing_stops.label, outing_stops.place_name, outing_stops.place_id, outing_stops.day,
		        outing_stops.duration_minutes, outing_stops.time_locked, outing_stops.meeting_lat,
		        outing_stops.meeting_lng, outing_stops.meeting_label
		   FROM outing_stops
		  WHERE outing_stops.outing_id = $1::UUID
		  ORDER BY outing_stops.position`, outingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	stops := []OutingStop{}
	for rows.Next() {
		var s OutingStop
		var outing string
		if err := rows.Scan(&s.ID, &outing, &s.Position, &s.MinuteOfDay, &s.Label, &s.PlaceName,
			&s.PlaceID, &s.Day, &s.DurationMinutes, &s.TimeLocked, &s.MeetingLat,
			&s.MeetingLng, &s.MeetingLabel); err != nil {
			return nil, err
		}
		stops = append(stops, s)
	}
	return stops, rows.Err()
}

// jsonArrayElements is `tuple(value or [])` for a column a CHECK keeps an
// array: the elements' JSON text, in order.
func jsonArrayElements(raw []byte) ([]json.RawMessage, error) {
	out := []json.RawMessage{}
	if jsonOrNone(raw) == nil {
		return out, nil
	}
	var elements []json.RawMessage
	if err := json.Unmarshal(raw, &elements); err != nil {
		return nil, ErrUnrepresentable
	}
	return append(out, elements...), nil
}

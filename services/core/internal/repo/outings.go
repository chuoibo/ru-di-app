package repo

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// GetOuting is get_outing: `session.get(Outing, id)` (every mapped column in
// declaration order, labelled table_column), then `_outing_record`'s stops by
// position. Nil without a statement for the stops when there is no row.
//
// SQLAlchemy note: an outing the same session already loaded answers from the
// identity map without the first SELECT; Go reads again.
func (r Repository) GetOuting(ctx context.Context, outingID string) (*Outing, error) {
	var o Outing
	var days []byte
	err := r.Q.QueryRow(ctx,
		`SELECT outings.id AS outings_id, outings.context_id AS outings_context_id,
		        outings.created_by_id AS outings_created_by_id, outings.timeline_revision AS outings_timeline_revision,
		        outings.itinerary_version AS outings_itinerary_version, outings.itinerary_days AS outings_itinerary_days,
		        outings.title AS outings_title, outings.starts_on AS outings_starts_on,
		        outings.ends_on AS outings_ends_on, outings.headcount AS outings_headcount,
		        outings.budget_per_person_vnd AS outings_budget_per_person_vnd, outings.created_at AS outings_created_at
		   FROM outings
		  WHERE outings.id = $1::UUID`, outingID).
		Scan(&o.ID, &o.ContextID, &o.CreatedByID, &o.TimelineRevision, &o.ItineraryVersion, &days, &o.Title,
			&o.StartsOn, &o.EndsOn, &o.Headcount, &o.BudgetPerPersonVND, &o.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	o.CreatedAt = o.CreatedAt.UTC()
	if o.ItineraryDays, err = jsonArrayElements(days); err != nil {
		return nil, err
	}
	if o.Stops, err = r.outingStops(ctx, o.ID); err != nil {
		return nil, err
	}
	return &o, nil
}

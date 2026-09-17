package repo

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Destination is DestinationRecord.
type Destination struct {
	ID        string
	Name      string
	Province  *string
	Lat       float64
	Lng       float64
	BBoxSouth float64
	BBoxWest  float64
	BBoxNorth float64
	BBoxEast  float64
	Blurb     *string
	SortOrder int64
}

// Mapped column order of db.models.Destination, including timestamps the
// record type does not surface.
const destinationColumns = `destinations.id, destinations.name, destinations.province, destinations.lat,
	destinations.lng, destinations.bbox_south, destinations.bbox_west, destinations.bbox_north,
	destinations.bbox_east, destinations.blurb, destinations.sort_order, destinations.created_at,
	destinations.updated_at`

func scanDestination(row pgx.Row) (*Destination, error) {
	var d Destination
	var created, updated time.Time
	err := row.Scan(&d.ID, &d.Name, &d.Province, &d.Lat, &d.Lng, &d.BBoxSouth, &d.BBoxWest,
		&d.BBoxNorth, &d.BBoxEast, &d.Blurb, &d.SortOrder, &created, &updated)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// ListDestinations is list_destinations: ORDER BY sort_order, id.
func (r Repository) ListDestinations(ctx context.Context) ([]Destination, error) {
	rows, err := r.Q.Query(ctx,
		`SELECT `+destinationColumns+` FROM destinations ORDER BY destinations.sort_order, destinations.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Destination{}
	for rows.Next() {
		d, err := scanDestination(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *d)
	}
	return out, rows.Err()
}

// GetDestination is get_destination: session.get by slug.
func (r Repository) GetDestination(ctx context.Context, destinationID string) (*Destination, error) {
	return scanDestination(r.Q.QueryRow(ctx,
		`SELECT `+destinationColumns+` FROM destinations WHERE destinations.id = $1::VARCHAR`, destinationID))
}

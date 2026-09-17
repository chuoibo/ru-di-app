package repo

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// PlacePhoto is PlacePhotoRecord.
type PlacePhoto struct {
	ID          string
	PlaceID     string
	StorageKey  string
	ContentType string
	ByteSize    int64
	Width       int64
	Height      int64
	Author      string
	License     string
	SourceURL   string
	Title       *string
	SortOrder   int64
}

// Mapped column order of db.models.PlacePhoto, including created_at the
// record type does not surface.
const placePhotoColumns = `place_photos.id, place_photos.place_id, place_photos.storage_key,
	place_photos.content_type, place_photos.byte_size, place_photos.width, place_photos.height,
	place_photos.author, place_photos.license, place_photos.source_url, place_photos.title,
	place_photos.sort_order, place_photos.created_at`

func scanPlacePhoto(row pgx.Row) (*PlacePhoto, error) {
	var p PlacePhoto
	var created time.Time
	err := row.Scan(&p.ID, &p.PlaceID, &p.StorageKey, &p.ContentType, &p.ByteSize, &p.Width, &p.Height,
		&p.Author, &p.License, &p.SourceURL, &p.Title, &p.SortOrder, &created)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// ListPlacePhotos is list_place_photos: ORDER BY sort_order, id.
func (r Repository) ListPlacePhotos(ctx context.Context, placeID string) ([]PlacePhoto, error) {
	rows, err := r.Q.Query(ctx,
		`SELECT `+placePhotoColumns+` FROM place_photos
		  WHERE place_photos.place_id = $1::VARCHAR
		  ORDER BY place_photos.sort_order, place_photos.id`, placeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PlacePhoto{}
	for rows.Next() {
		p, err := scanPlacePhoto(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

// GetPlacePhoto is get_place_photo: scoped by both halves of the key.
func (r Repository) GetPlacePhoto(ctx context.Context, placeID, photoID string) (*PlacePhoto, error) {
	return scanPlacePhoto(r.Q.QueryRow(ctx,
		`SELECT `+placePhotoColumns+` FROM place_photos
		  WHERE place_photos.id = $1::UUID AND place_photos.place_id = $2::VARCHAR`,
		photoID, placeID))
}

// PhotoCovers is photo_covers: the first photograph of each place.
func (r Repository) PhotoCovers(ctx context.Context, placeIDs []string) (map[string]PlacePhoto, error) {
	out := map[string]PlacePhoto{}
	if len(placeIDs) == 0 {
		return out, nil
	}
	placeholders := make([]string, len(placeIDs))
	args := make([]any, len(placeIDs))
	for i, id := range placeIDs {
		placeholders[i] = "$" + strconv.Itoa(i+1) + "::VARCHAR"
		args[i] = id
	}
	rows, err := r.Q.Query(ctx,
		`SELECT `+placePhotoColumns+` FROM place_photos
		  WHERE place_photos.place_id IN (`+strings.Join(placeholders, ", ")+`)
		  ORDER BY place_photos.place_id, place_photos.sort_order, place_photos.id`,
		args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		p, err := scanPlacePhoto(rows)
		if err != nil {
			return nil, err
		}
		if _, ok := out[p.PlaceID]; !ok {
			out[p.PlaceID] = *p
		}
	}
	return out, rows.Err()
}

// PhotoCounts is photo_counts: places with none are absent.
func (r Repository) PhotoCounts(ctx context.Context, placeIDs []string) (map[string]int64, error) {
	out := map[string]int64{}
	if len(placeIDs) == 0 {
		return out, nil
	}
	placeholders := make([]string, len(placeIDs))
	args := make([]any, len(placeIDs))
	for i, id := range placeIDs {
		placeholders[i] = "$" + strconv.Itoa(i+1) + "::VARCHAR"
		args[i] = id
	}
	rows, err := r.Q.Query(ctx,
		`SELECT place_photos.place_id, count(*)
		   FROM place_photos
		  WHERE place_photos.place_id IN (`+strings.Join(placeholders, ", ")+`)
		  GROUP BY place_photos.place_id`,
		args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var n int64
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		out[id] = n
	}
	return out, rows.Err()
}

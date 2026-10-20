package repo

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

func isNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }

// DestinationCovers is the cover photograph of every destination that has
// one, in one statement: among the photographs of the destination's places,
// the first of the place with the most evidence posts (NULLs last), then by
// (sort_order, id). The map is keyed by destination id; PlacePhoto.PlaceID
// says which place the photograph belongs to, which the URL needs.
func (r Repository) DestinationCovers(ctx context.Context) (map[string]PlacePhoto, error) {
	rows, err := r.Q.Query(ctx,
		`SELECT DISTINCT ON (places.destination_id) places.destination_id, `+placePhotoColumns+`
		   FROM place_photos
		   JOIN places ON places.id = place_photos.place_id
		  ORDER BY places.destination_id, places.evidence_posts DESC NULLS LAST,
		           place_photos.sort_order, place_photos.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]PlacePhoto{}
	for rows.Next() {
		var destination string
		var p PlacePhoto
		var created time.Time
		if err := rows.Scan(&destination, &p.ID, &p.PlaceID, &p.StorageKey, &p.ContentType, &p.ByteSize,
			&p.Width, &p.Height, &p.Author, &p.License, &p.SourceURL,
			new(any), new(any), new(any), new(any), new(any), new(any), &p.Title,
			&p.SortOrder, &created); err != nil {
			return nil, err
		}
		out[destination] = p
	}
	return out, rows.Err()
}

// PlaceFactsLive is the part of place_facts a detail page may show. It is read
// only while het_han_at is in the future: an expired fact reads as unknown.
// `bang_chung` and the feed's other audit columns are never selected.
type PlaceFactsLive struct {
	Hours          json.RawMessage // [{thu, mo, dong, qua_dem}]
	HoursNote      *string
	BookAhead      *string
	Setting        *string
	DurationMin    *int64
	DurationMax    *int64
	PriceMinVND    *int64
	PriceMaxVND    *int64
	PriceEstimated bool
}

// PlaceExtras is what the ingest feed knows about one place beyond its row.
// Every part is optional; a place the feed never reached has none.
type PlaceExtras struct {
	Facts      *PlaceFactsLive
	Categories []string // place_danh_muc.danh_muc
	Vibes      []string // place_lam_giau.khi_chat
	MainDishes []string // place_lam_giau.mon_chinh
}

// PlaceExtras reads place_facts, place_danh_muc and place_lam_giau for a
// place. Those tables belong to the ingest and exist only after its schema
// migrations, so each is read only when present. Never selected: di_ung,
// an_kieng, chen_lenh, tin_cay (the allergen reading is not a claim a card may
// make) and the audit columns of the feed.
func (r Repository) PlaceExtras(ctx context.Context, placeID string, at time.Time) (PlaceExtras, error) {
	var out PlaceExtras
	var facts, danhMuc, lamGiau bool
	if err := r.Q.QueryRow(ctx,
		`SELECT to_regclass('place_facts') IS NOT NULL,
		        to_regclass('place_danh_muc') IS NOT NULL,
		        to_regclass('place_lam_giau') IS NOT NULL`).Scan(&facts, &danhMuc, &lamGiau); err != nil {
		return out, err
	}
	if facts {
		var f PlaceFactsLive
		var hours []byte
		err := r.Q.QueryRow(ctx,
			`SELECT gio_mo_cua::text, gio_ghi_chu, can_dat_truoc, trong_nha_ngoai_troi,
			        thoi_luong_phut_min, thoi_luong_phut_max, gia_min_vnd, gia_max_vnd, gia_uoc
			   FROM place_facts
			  WHERE place_id = $1::TEXT AND het_han_at > $2`, placeID, at).
			Scan(&hours, &f.HoursNote, &f.BookAhead, &f.Setting,
				&f.DurationMin, &f.DurationMax, &f.PriceMinVND, &f.PriceMaxVND, &f.PriceEstimated)
		switch {
		case err == nil:
			f.Hours = json.RawMessage(hours)
			out.Facts = &f
		case isNoRows(err):
		default:
			return out, err
		}
	}
	if danhMuc {
		err := r.Q.QueryRow(ctx, `SELECT danh_muc FROM place_danh_muc WHERE place_id = $1::TEXT`, placeID).
			Scan(&out.Categories)
		if err != nil && !isNoRows(err) {
			return out, err
		}
	}
	if lamGiau {
		err := r.Q.QueryRow(ctx, `SELECT khi_chat, mon_chinh FROM place_lam_giau WHERE place_id = $1::TEXT`, placeID).
			Scan(&out.Vibes, &out.MainDishes)
		if err != nil && !isNoRows(err) {
			return out, err
		}
	}
	return out, nil
}

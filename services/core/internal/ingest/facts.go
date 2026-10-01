package ingest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"mobile/services/core/internal/domain/giomo"
)

// FactsSource is the name the web-facts pull keeps its cursor under.
const FactsSource = "vnlocal.place_web_facts"

// Reasons a web-facts row is refused. The cursor still moves past it: the
// feed re-checks every row before its expiry and the next version lands then.
const (
	RejectFactsPlaceID     = "facts_place_id_la"
	RejectFactsConHoatDong = "facts_con_hoat_dong_la"
	RejectFactsNotArray    = "facts_json_khong_phai_mang"
	RejectFactsNegative    = "facts_so_am"
)

// FactRow is one row of the feed's `place_web_facts`, only the columns RuDi
// keeps. The feed's audit columns (bang_chung, truy_van, bo, nguon, model)
// are never selected.
type FactRow struct {
	PlaceID           string // the feed's key, plc_…
	SyncedAt          time.Time
	SchemaVersion     string
	TrangThai         string
	ConHoatDong       *string
	GioMoCua          []byte
	GioGhiChu         *string
	GiaMin, GiaMax    *int64
	GiaDonVi          *string
	GiaGhiChu         *string
	GiaNguoiMin       *int64
	GiaNguoiMax       *int64
	GiaNguoiCoSo      *string
	GiaNguoiGhiChu    *string
	Menu, HoatDong    []byte
	CanDatTruoc       *string
	TrongNhaNgoaiTroi *string
	ThoiLuongMin      *int32
	ThoiLuongMax      *int32
	LienHe            []string
	CheckedAt         time.Time
	HetHanAt          time.Time
}

// FactFeed pages through the feed's web facts in cursor order.
type FactFeed interface {
	// Page returns up to limit rows strictly after (syncedAt, placeID), in
	// (synced_at, place_id) order.
	Page(ctx context.Context, syncedAt time.Time, placeID string, limit int) ([]FactRow, error)
}

// PGFactFeed reads `place_web_facts` from the feed's Postgres (role
// rudi_app, read only).
type PGFactFeed struct{ Pool *pgxpool.Pool }

// Page is keyset pagination on the pair, as PGFeed.Page and for the same
// reason: the feed writes many rows under one `synced_at`.
func (f PGFactFeed) Page(ctx context.Context, syncedAt time.Time, placeID string, limit int) ([]FactRow, error) {
	rows, err := f.Pool.Query(ctx, `
		SELECT place_id, synced_at, schema_version, trang_thai, con_hoat_dong,
		       gio_mo_cua::text, gio_mo_cua_ghi_chu,
		       gia_min_vnd, gia_max_vnd, gia_don_vi, gia_ghi_chu,
		       gia_nguoi_min_vnd, gia_nguoi_max_vnd, gia_nguoi_co_so, gia_nguoi_ghi_chu,
		       menu::text, hoat_dong::text, can_dat_truoc, trong_nha_ngoai_troi,
		       thoi_luong_phut_min, thoi_luong_phut_max, lien_he, checked_at, het_han_at
		FROM place_web_facts
		WHERE (synced_at, place_id) > ($1, $2)
		ORDER BY synced_at, place_id
		LIMIT $3`, syncedAt, placeID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FactRow
	for rows.Next() {
		var r FactRow
		var gio, menu, hoatDong string
		if err := rows.Scan(&r.PlaceID, &r.SyncedAt, &r.SchemaVersion, &r.TrangThai, &r.ConHoatDong,
			&gio, &r.GioGhiChu, &r.GiaMin, &r.GiaMax, &r.GiaDonVi, &r.GiaGhiChu,
			&r.GiaNguoiMin, &r.GiaNguoiMax, &r.GiaNguoiCoSo, &r.GiaNguoiGhiChu,
			&menu, &hoatDong, &r.CanDatTruoc, &r.TrongNhaNgoaiTroi,
			&r.ThoiLuongMin, &r.ThoiLuongMax, &r.LienHe, &r.CheckedAt, &r.HetHanAt); err != nil {
			return nil, err
		}
		r.GioMoCua, r.Menu, r.HoatDong = []byte(gio), []byte(menu), []byte(hoatDong)
		out = append(out, r)
	}
	return out, rows.Err()
}

var (
	thuOSM  = map[string]int{"mon": 0, "tue": 1, "wed": 2, "thu": 3, "fri": 4, "sat": 5, "sun": 6}
	tenOSM  = [7]string{"Mo", "Tu", "We", "Th", "Fr", "Sa", "Su"}
	gioPhut = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$|^24:00$`)
)

// GioOSM turns the feed's hours ([{thu, mo, dong, qua_dem}], one entry per
// shift) into an OSM opening_hours string domain/giomo reads, e.g.
// "Mo-Sa 06:00-21:30; Su off".
//
// A weekday the feed lists no shift for is closed that day ("off"): the
// feed's notes on such rows say so («Chủ Nhật nghỉ»), and a place whose
// hours vary by day without a fixed schedule is fed as [] instead. An empty
// list is unknown (false), never closed. So is any entry that does not read
// cleanly -- an unknown weekday, a malformed time, a `qua_dem` that disagrees
// with its own times -- because half a schedule would claim closures nobody
// saw.
func GioOSM(raw []byte) (string, bool) {
	var ca []struct {
		Thu    string `json:"thu"`
		Mo     string `json:"mo"`
		Dong   string `json:"dong"`
		QuaDem *bool  `json:"qua_dem"`
	}
	if err := json.Unmarshal(raw, &ca); err != nil || len(ca) == 0 {
		return "", false
	}
	var days [7][]string
	for _, c := range ca {
		d, ok := thuOSM[c.Thu]
		if !ok || !gioPhut.MatchString(c.Mo) || c.Mo == "24:00" || !gioPhut.MatchString(c.Dong) {
			return "", false
		}
		// "HH:MM" compares as text. An end at or before the start runs past
		// midnight, in giomo as in the feed.
		if c.QuaDem != nil && *c.QuaDem != (c.Dong <= c.Mo) {
			return "", false
		}
		span := c.Mo + "-" + c.Dong
		if !contains(days[d], span) {
			days[d] = append(days[d], span)
		}
	}
	var rules []string
	for d := 0; d < 7; {
		sort.Strings(days[d])
		times := strings.Join(days[d], ",")
		if times == "" {
			times = "off"
		}
		end := d
		for end+1 < 7 {
			sort.Strings(days[end+1])
			next := strings.Join(days[end+1], ",")
			if next == "" {
				next = "off"
			}
			if next != times {
				break
			}
			end++
		}
		sel := tenOSM[d]
		if end > d {
			sel += "-" + tenOSM[end]
		}
		rules = append(rules, sel+" "+times)
		d = end + 1
	}
	out := strings.Join(rules, "; ")
	if _, ok := giomo.Doc(out); !ok {
		return "", false
	}
	return out, true
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// donViMoiNguoi are the units of the feed's older price band that already
// mean one person, one visit.
var donViMoiNguoi = map[string]bool{"moi_nguoi": true, "moi_ve": true, "moi_suat": true}

// GiaMoiNguoi is one person's typical spend for one visit, as the feed
// recommends reading it: the web-facts@2 per-person band first; otherwise
// the older band when its unit is already per person; otherwise unknown. A
// bound the feed does not give stays unknown (a «from 50k» is not a
// 50k-to-50k band). uoc marks a band estimated from dish prices.
func GiaMoiNguoi(r FactRow) (lo, hi *int64, uoc bool) {
	switch {
	case r.GiaNguoiMin != nil || r.GiaNguoiMax != nil:
		lo, hi = r.GiaNguoiMin, r.GiaNguoiMax
		uoc = r.GiaNguoiCoSo != nil && *r.GiaNguoiCoSo == "uoc_tu_mon"
	case r.GiaDonVi != nil && donViMoiNguoi[*r.GiaDonVi] && (r.GiaMin != nil || r.GiaMax != nil):
		lo, hi = r.GiaMin, r.GiaMax
	default:
		return nil, nil, false
	}
	if (lo != nil && *lo < 0) || (hi != nil && *hi < 0) || (lo != nil && hi != nil && *hi < *lo) {
		return nil, nil, false
	}
	return lo, hi, uoc
}

var conHoatDong = map[string]bool{"con_hoat_dong": true, "tam_dong": true, "dong_vinh_vien": true, "khong_ro": true}

// factReject says why a row cannot be kept, or "".
func factReject(r FactRow) string {
	if !strings.HasPrefix(r.PlaceID, "plc_") || len(r.PlaceID) == len("plc_") {
		return RejectFactsPlaceID
	}
	if r.ConHoatDong != nil && !conHoatDong[*r.ConHoatDong] {
		return RejectFactsConHoatDong
	}
	for _, raw := range [][]byte{r.GioMoCua, r.Menu, r.HoatDong} {
		var list []json.RawMessage
		if json.Unmarshal(raw, &list) != nil || list == nil {
			return RejectFactsNotArray
		}
	}
	for _, v := range []*int64{r.GiaMin, r.GiaMax, r.GiaNguoiMin, r.GiaNguoiMax} {
		if v != nil && *v < 0 {
			return RejectFactsNegative
		}
	}
	return ""
}

// FactsResult is what one web-facts pull did.
type FactsResult struct {
	Landed   int
	Rejected map[string]int
	Gio      int // landed rows whose hours read into a schedule
	Gia      int // landed rows with a per-person band
	CaughtUp bool
}

// PullFacts upserts the next slice of the feed's web facts into
// `place_facts` and moves the cursor, in one transaction. Rows are upserts,
// not a delivery: a row older than what is kept (by synced_at) is ignored.
func PullFacts(ctx context.Context, pool *pgxpool.Pool, feed FactFeed, opt PullOptions) (FactsResult, error) {
	if opt.PageSize <= 0 {
		opt.PageSize = 500
	}
	if opt.MaxRows <= 0 {
		opt.MaxRows = 2000
	}
	result := FactsResult{Rejected: map[string]int{}}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(815209)`); err != nil {
		return result, err
	}
	syncedAt, placeID := cursorStart, ""
	err = tx.QueryRow(ctx, `SELECT synced_at, place_id FROM ingest_cursor WHERE source = $1`,
		FactsSource).Scan(&syncedAt, &placeID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return result, err
	}

	var rows []FactRow
	for len(rows) < opt.MaxRows {
		limit := min(opt.PageSize, opt.MaxRows-len(rows))
		page, err := feed.Page(ctx, syncedAt, placeID, limit)
		if err != nil {
			return result, fmt.Errorf("read web facts after (%s, %q): %w",
				syncedAt.Format(time.RFC3339Nano), placeID, err)
		}
		rows = append(rows, page...)
		if len(page) > 0 {
			last := page[len(page)-1]
			syncedAt, placeID = last.SyncedAt, last.PlaceID
		}
		if len(page) < limit {
			result.CaughtUp = true
			break
		}
	}
	if len(rows) == 0 {
		return result, nil
	}

	batch := &pgx.Batch{}
	var uocDoi []string
	for _, r := range rows {
		if reason := factReject(r); reason != "" {
			result.Rejected[reason]++
			continue
		}
		state := "khong_ro"
		if r.ConHoatDong != nil {
			state = *r.ConHoatDong
		}
		var gio *string
		if s, ok := GioOSM(r.GioMoCua); ok {
			gio = &s
			result.Gio++
		}
		lo, hi, uoc := GiaMoiNguoi(r)
		if lo != nil || hi != nil {
			result.Gia++
		}
		lienHe := r.LienHe
		if lienHe == nil {
			lienHe = []string{}
		}
		id := PlaceID(r.PlaceID)
		batch.Queue(upsertFactsSQL, id, r.PlaceID, r.SchemaVersion, r.TrangThai, state,
			string(r.GioMoCua), r.GioGhiChu, gio,
			r.GiaNguoiMin, r.GiaNguoiMax, r.GiaNguoiCoSo, r.GiaNguoiGhiChu,
			r.GiaMin, r.GiaMax, r.GiaDonVi, r.GiaGhiChu, lo, hi, uoc,
			string(r.Menu), string(r.HoatDong), r.CanDatTruoc, r.TrongNhaNgoaiTroi,
			r.ThoiLuongMin, r.ThoiLuongMax, lienHe, r.CheckedAt, r.HetHanAt, r.SyncedAt).
			QueryRow(func(row pgx.Row) error {
				var doi bool
				switch err := row.Scan(&doi); {
				case errors.Is(err, pgx.ErrNoRows):
					return nil
				case err != nil:
					return err
				}
				if doi {
					uocDoi = append(uocDoi, id)
				}
				return nil
			})
		result.Landed++
	}
	if batch.Len() > 0 {
		if err := tx.SendBatch(ctx, batch).Close(); err != nil {
			return result, err
		}
	}
	// Whether a price is estimated is quoted with it («khoảng … (ước)», the
	// index's evidence fields) but lives here, not on places: a place whose
	// flag moved while its numbers did not is marked for the index.
	if len(uocDoi) > 0 {
		if _, err := tx.Exec(ctx, `UPDATE places SET updated_at = clock_timestamp() WHERE id = ANY($1) AND source = 'vnlocal'`, uocDoi); err != nil {
			return result, err
		}
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO ingest_cursor (source, synced_at, place_id, batch_id)
		VALUES ($1,$2,$3,NULL)
		ON CONFLICT (source) DO UPDATE SET
		  synced_at = EXCLUDED.synced_at, place_id = EXCLUDED.place_id,
		  batch_id = NULL, moved_at = clock_timestamp()`,
		FactsSource, syncedAt, placeID); err != nil {
		return result, err
	}
	return result, tx.Commit(ctx)
}

const upsertFactsSQL = `
	WITH cu AS (SELECT gia_uoc FROM place_facts WHERE place_id = $1)
	INSERT INTO place_facts (place_id, source_ref, schema_version, trang_thai, con_hoat_dong,
	  gio_mo_cua, gio_ghi_chu, gio_osm,
	  gia_nguoi_min_vnd, gia_nguoi_max_vnd, gia_nguoi_co_so, gia_nguoi_ghi_chu,
	  gia_feed_min_vnd, gia_feed_max_vnd, gia_don_vi, gia_ghi_chu,
	  gia_min_vnd, gia_max_vnd, gia_uoc,
	  menu, hoat_dong, can_dat_truoc, trong_nha_ngoai_troi,
	  thoi_luong_phut_min, thoi_luong_phut_max, lien_he, checked_at, het_han_at, synced_at)
	VALUES ($1,$2,$3,$4,$5,$6::jsonb,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,
	  $20::jsonb,$21::jsonb,$22,$23,$24,$25,$26,$27,$28,$29)
	ON CONFLICT (place_id) DO UPDATE SET
	  schema_version = EXCLUDED.schema_version, trang_thai = EXCLUDED.trang_thai,
	  con_hoat_dong = EXCLUDED.con_hoat_dong, gio_mo_cua = EXCLUDED.gio_mo_cua,
	  gio_ghi_chu = EXCLUDED.gio_ghi_chu, gio_osm = EXCLUDED.gio_osm,
	  gia_nguoi_min_vnd = EXCLUDED.gia_nguoi_min_vnd, gia_nguoi_max_vnd = EXCLUDED.gia_nguoi_max_vnd,
	  gia_nguoi_co_so = EXCLUDED.gia_nguoi_co_so, gia_nguoi_ghi_chu = EXCLUDED.gia_nguoi_ghi_chu,
	  gia_feed_min_vnd = EXCLUDED.gia_feed_min_vnd, gia_feed_max_vnd = EXCLUDED.gia_feed_max_vnd,
	  gia_don_vi = EXCLUDED.gia_don_vi, gia_ghi_chu = EXCLUDED.gia_ghi_chu,
	  gia_min_vnd = EXCLUDED.gia_min_vnd, gia_max_vnd = EXCLUDED.gia_max_vnd, gia_uoc = EXCLUDED.gia_uoc,
	  menu = EXCLUDED.menu, hoat_dong = EXCLUDED.hoat_dong,
	  can_dat_truoc = EXCLUDED.can_dat_truoc, trong_nha_ngoai_troi = EXCLUDED.trong_nha_ngoai_troi,
	  thoi_luong_phut_min = EXCLUDED.thoi_luong_phut_min, thoi_luong_phut_max = EXCLUDED.thoi_luong_phut_max,
	  lien_he = EXCLUDED.lien_he, checked_at = EXCLUDED.checked_at, het_han_at = EXCLUDED.het_han_at,
	  synced_at = EXCLUDED.synced_at, landed_at = clock_timestamp()
	WHERE place_facts.synced_at <= EXCLUDED.synced_at
	RETURNING COALESCE((SELECT gia_uoc FROM cu), false) IS DISTINCT FROM place_facts.gia_uoc`

// FactsApplied is what one derivation pass changed and what it left.
type FactsApplied struct {
	Updated int // places whose hours or price band changed
	CoGio   int // fed places with unexpired, readable hours
	CoGia   int // fed places with an unexpired per-person band
	HetHan  int // facts rows past het_han_at, read as unknown
}

// ApplyFacts copies the derived hours and per-person band onto the
// catalogue's own `places` rows, for every fed place that has facts. An
// expired fact (het_han_at <= at) clears them: unknown, not the last value
// seen. Only rows that change are written, so a quiet round writes nothing
// and marks nothing dirty for the index.
//
// ingest is the writer of these columns on fed rows: the place upsert
// (apply.go) never sets them, so the two passes do not fight.
func ApplyFacts(ctx context.Context, pool *pgxpool.Pool, at time.Time) (FactsApplied, error) {
	var out FactsApplied
	tag, err := pool.Exec(ctx, `
		WITH want AS (
		  SELECT place_id,
		         CASE WHEN het_han_at > $1 THEN gio_osm END AS gio,
		         CASE WHEN het_han_at > $1 THEN gia_min_vnd END AS lo,
		         CASE WHEN het_han_at > $1 THEN gia_max_vnd END AS hi
		    FROM place_facts)
		UPDATE places p
		   SET open_hours = w.gio, price_min_vnd = w.lo, price_max_vnd = w.hi,
		       updated_at = clock_timestamp()
		  FROM want w
		 WHERE p.id = w.place_id AND p.source = 'vnlocal'
		   AND (p.open_hours IS DISTINCT FROM w.gio
		     OR p.price_min_vnd IS DISTINCT FROM w.lo
		     OR p.price_max_vnd IS DISTINCT FROM w.hi)`, at)
	if err != nil {
		return out, err
	}
	out.Updated = int(tag.RowsAffected())
	err = pool.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE f.het_han_at > $1 AND f.gio_osm IS NOT NULL),
		       count(*) FILTER (WHERE f.het_han_at > $1 AND (f.gia_min_vnd IS NOT NULL OR f.gia_max_vnd IS NOT NULL)),
		       count(*) FILTER (WHERE f.het_han_at <= $1)
		  FROM place_facts f JOIN places p ON p.id = f.place_id`, at).Scan(&out.CoGio, &out.CoGia, &out.HetHan)
	return out, err
}

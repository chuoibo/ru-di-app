package ingest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"

	"mobile/services/core/internal/media/sanitize"
	"mobile/services/core/internal/media/storage"
)

// PhotoResult is what one photo import wrote.
type PhotoResult struct {
	Places   int
	Written  int
	Existing int
	Skipped  map[string]int
}

// PhotoOptions narrows an import. A whole catalogue is 19,704 frames; a test
// run wants one province and a few per place.
type PhotoOptions struct {
	BatchID      string
	Source       FrameSource
	ProvinceCode *int16
	PerPlace     int
}

// frameURL is the source of one frame: the post it came from, at the second it
// was taken.
//
// A media fragment (`#t=`) rather than the bare post URL, because two frames of
// one video share that URL, and the photograph table keys on (place, source).
// The fragment is not decoration to get past a constraint: it is the W3C form
// for "this point in this video", which is exactly what the frame is.
func frameURL(frame Frame) string {
	if frame.Giay == nil {
		return frame.SourceURL
	}
	return frame.SourceURL + "#t=" + strconv.FormatFloat(*frame.Giay, 'f', 1, 64)
}

// ImportPhotos copies selected frames into photo storage and the catalogue.
//
// Every image goes through the same sanitiser as a person's upload: decoded and
// re-encoded, which drops EXIF and anything else riding in the file. The bytes
// that reach storage are therefore not the bytes on the feed's disk, and the
// digest recorded is of what is stored.
//
// The object is written before the row. An object without a row is rubbish a
// sweep can collect; a row without an object is a broken picture on somebody's
// screen. Failing toward rubbish is the documented contract for this storage.
func ImportPhotos(ctx context.Context, pool *pgxpool.Pool, store *storage.PhotoStorage, opt PhotoOptions) (PhotoResult, error) {
	result := PhotoResult{Skipped: map[string]int{}}
	if opt.PerPlace <= 0 {
		opt.PerPlace = 3
	}

	rows, err := pool.Query(ctx, `
		SELECT payload FROM ingest_place_raw WHERE batch_id = $1 ORDER BY line_no`,
		opt.BatchID)
	if err != nil {
		return result, err
	}
	var records []*Record
	for rows.Next() {
		var payload []byte
		if err := rows.Scan(&payload); err != nil {
			rows.Close()
			return result, err
		}
		rec, _, reject := Parse(payload)
		if reject != nil || len(rec.Frames) == 0 {
			continue
		}
		// A regional speciality has no place to photograph. Its frames show a
		// dish, and pinning them to a catalogue entry would illustrate a shop
		// that does not exist.
		if rec.Loai == "mon_an" {
			continue
		}
		if opt.ProvinceCode != nil && (rec.ProvinceCode == nil || *rec.ProvinceCode != *opt.ProvinceCode) {
			continue
		}
		records = append(records, rec)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return result, err
	}
	rows.Close()

	return importRecords(ctx, pool, store, opt, records, result)
}

// photoWorkers bounds how many frames are fetched and re-encoded at once. The
// work is a network read from object storage plus a decode/encode, so it
// overlaps well; the database side is two short statements per frame and
// shares the process's small pool.
const photoWorkers = 8

type photoJob struct {
	placeID string
	frame   Frame
	order   int
}

// importRecords writes the frames each place still lacks.
//
// Two reads cover the whole batch -- which places exist, which (place, source)
// photos are already stored -- instead of two statements per place and one per
// frame. A place whose chosen frames are all stored costs nothing further,
// which is most rows of an update batch: the feed re-sends a place whenever
// anything about it changes.
func importRecords(ctx context.Context, pool *pgxpool.Pool, store *storage.PhotoStorage,
	opt PhotoOptions, records []*Record, result PhotoResult) (PhotoResult, error) {
	ids := make([]string, 0, len(records))
	for _, rec := range records {
		ids = append(ids, PlaceID(rec.PlaceID))
	}
	existing := map[string]bool{}
	rows, err := pool.Query(ctx, `SELECT id FROM places WHERE id = ANY($1)`, ids)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return result, err
		}
		existing[id] = true
	}
	rows.Close()
	have := map[string]bool{}
	rows, err = pool.Query(ctx, `SELECT place_id, source_url FROM place_photos WHERE place_id = ANY($1)`, ids)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var placeID, source string
		if err := rows.Scan(&placeID, &source); err != nil {
			rows.Close()
			return result, err
		}
		have[placeID+"\x00"+source] = true
	}
	rows.Close()

	var jobs []photoJob
	for _, rec := range records {
		placeID := PlaceID(rec.PlaceID)
		if !existing[placeID] {
			result.Skipped["dia_diem_chua_chieu"]++
			continue
		}
		frames := usableFrames(rec.Frames)
		// The feed scores each frame for how well it shows the place; the best
		// ones become the cover.
		sort.SliceStable(frames, func(i, j int) bool {
			return score(frames[i]) > score(frames[j])
		})
		if len(frames) > opt.PerPlace {
			frames = frames[:opt.PerPlace]
		}
		result.Places++
		for order, frame := range frames {
			if have[placeID+"\x00"+frameURL(frame)] {
				result.Existing++
				continue
			}
			jobs = append(jobs, photoJob{placeID: placeID, frame: frame, order: order})
		}
	}

	var (
		mu       sync.Mutex
		firstErr error
		touched  = map[string]bool{}
		wg       sync.WaitGroup
		queue    = make(chan photoJob)
	)
	for w := 0; w < photoWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range queue {
				written, reason, err := importFrame(ctx, pool, store, opt.Source, job.placeID, job.frame, job.order)
				mu.Lock()
				switch {
				case err != nil:
					if firstErr == nil {
						firstErr = err
					}
				case reason != "":
					result.Skipped[reason]++
				case written:
					result.Written++
					touched[job.placeID] = true
				default:
					result.Existing++
				}
				mu.Unlock()
			}
		}()
	}
	for _, job := range jobs {
		mu.Lock()
		failed := firstErr != nil
		mu.Unlock()
		if failed || ctx.Err() != nil {
			break
		}
		queue <- job
	}
	close(queue)
	wg.Wait()
	if firstErr != nil {
		return result, firstErr
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}

	// photo_count only for the places that gained a photo, in one statement.
	if len(touched) > 0 {
		changed := make([]string, 0, len(touched))
		for id := range touched {
			changed = append(changed, id)
		}
		if _, err := pool.Exec(ctx, `
			UPDATE places SET photo_count =
			  (SELECT count(*) FROM place_photos WHERE place_photos.place_id = places.id)
			WHERE id = ANY($1)`, changed); err != nil {
			return result, err
		}
	}
	return result, nil
}

func usableFrames(frames []Frame) []Frame {
	out := make([]Frame, 0, len(frames))
	for _, frame := range frames {
		if frame.FileExists && strings.TrimSpace(frame.StorageKey) != "" {
			out = append(out, frame)
		}
	}
	return out
}

func score(frame Frame) float64 {
	if frame.Diem == nil {
		return 0
	}
	return *frame.Diem
}

// importFrame returns whether it wrote a new row, or the reason it did not.
func importFrame(ctx context.Context, pool *pgxpool.Pool, store *storage.PhotoStorage,
	source FrameSource, placeID string, frame Frame, order int) (bool, string, error) {
	sourceURL := frameURL(frame)
	raw, err := source.Read(ctx, frame.StorageKey)
	switch {
	case errors.Is(err, ErrFrameRefused):
		return false, "duong_dan_ra_ngoai", nil
	case errors.Is(err, ErrFrameMissing):
		// Not uploaded yet. The feed bumps the place when it is, and the
		// place comes back in a later pull.
		return false, "chua_co_trong_nguon", nil
	case err != nil:
		// Transport trouble is not a verdict on the frame: fail the pass so
		// the batch stays unmarked and is retried.
		return false, "", fmt.Errorf("read frame %s: %w", frame.StorageKey, err)
	}
	clean, err := sanitize.Sanitize(raw)
	if err != nil {
		return false, "anh_khong_giai_ma_duoc", nil
	}
	key, err := storage.NewStorageKey()
	if err != nil {
		return false, "", err
	}
	if err := store.Write(key, clean.Data); err != nil {
		return false, "", fmt.Errorf("storage write %s: %w", key, err)
	}
	digest := sha256.Sum256(clean.Data)

	var platform, postID, subject any
	if frame.Platform != "" {
		platform = frame.Platform
	}
	if frame.PostID != "" {
		postID = frame.PostID
	}
	if frame.ChuThe != "" {
		subject = frame.ChuThe
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO place_photos (id, place_id, storage_key, content_type,
		  byte_size, width, height, source_url, sort_order, platform, post_id,
		  frame_second, score, subject, content_sha256)
		VALUES (gen_random_uuid(), $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		ON CONFLICT (place_id, source_url) DO NOTHING`,
		placeID, key, clean.ContentType, len(clean.Data), clean.Width,
		clean.Height, sourceURL, order, platform, postID, frame.Giay,
		frame.Diem, subject, hex.EncodeToString(digest[:])); err != nil {
		return false, "", err
	}
	return true, "", nil
}

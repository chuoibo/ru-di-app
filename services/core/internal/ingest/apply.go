package ingest

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RejectNoProvinceBox is a row whose province has no boundary in the extract.
// Distinct from a row with no province at all: one is a gap in the reference
// data, the other is a gap in the row, and they are fixed in different places.
const RejectNoProvinceBox = "tinh_thieu_hop_bao"

// ApplyResult is what one projection run wrote.
type ApplyResult struct {
	BatchID  string
	Inserted int
	Updated  int
	Stale    int
	Rejected map[string]int
	Posts    int
}

// RejectedTotal is how many landed rows did not become catalogue rows.
func (r ApplyResult) RejectedTotal() int {
	total := 0
	for _, count := range r.Rejected {
		total += count
	}
	return total
}

// Reasons lists the reject codes, most frequent first.
func (r ApplyResult) Reasons() []string {
	names := make([]string, 0, len(r.Rejected))
	for name := range r.Rejected {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		return r.Rejected[names[i]] > r.Rejected[names[j]]
	})
	return names
}

// Apply projects a landed delivery into the catalogue.
//
// Reads the raw payloads back out rather than keeping them from the landing
// pass, because that is the whole point of landing them: a corrected mapping
// rule is applied by running this again, with nothing asked of the other side.
//
// Idempotent. Running it twice writes the same rows, and a row that did not
// change reports as an update rather than being skipped -- Postgres does not
// distinguish, and pretending to would mean comparing every column here to
// decide something nobody acts on.
func Apply(ctx context.Context, pool *pgxpool.Pool, batchID string) (ApplyResult, error) {
	result := ApplyResult{BatchID: batchID, Rejected: map[string]int{}}

	boxes := map[int16]bool{}
	for _, box := range ProvinceBoxes {
		boxes[box.Code] = true
	}

	rows, err := pool.Query(ctx, `
		SELECT line_no, payload FROM ingest_place_raw
		WHERE batch_id = $1 ORDER BY line_no`, batchID)
	if err != nil {
		return result, err
	}
	type pending struct {
		lineNo     int
		projection Projection
	}
	var ready []pending
	var rejects []pending
	rejectReason := map[int]string{}
	for rows.Next() {
		var lineNo int
		var payload []byte
		if err := rows.Scan(&lineNo, &payload); err != nil {
			rows.Close()
			return result, err
		}
		rec, _, reject := Parse(payload)
		if reject != nil {
			// A row that landed and now fails to parse means the rule changed
			// under it, which is exactly the case this design exists to make
			// survivable. Recorded, not lost.
			result.Rejected[reject.Code]++
			rejects = append(rejects, pending{lineNo: lineNo})
			rejectReason[lineNo] = reject.Code
			continue
		}
		if rec.ProvinceCode == nil {
			result.Rejected[RejectUnknownProvince]++
			rejects = append(rejects, pending{lineNo: lineNo})
			rejectReason[lineNo] = RejectUnknownProvince
			continue
		}
		if !boxes[*rec.ProvinceCode] {
			result.Rejected[RejectNoProvinceBox]++
			rejects = append(rejects, pending{lineNo: lineNo})
			rejectReason[lineNo] = RejectNoProvinceBox
			continue
		}
		ready = append(ready, pending{lineNo: lineNo, projection: Project(rec)})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return result, err
	}
	rows.Close()

	tx, err := pool.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)

	for _, item := range rejects {
		if _, err := tx.Exec(ctx, `
			INSERT INTO ingest_reject (batch_id, line_no, reason, detail)
			VALUES ($1,$2,$3,'projection')
			ON CONFLICT DO NOTHING`,
			batchID, item.lineNo, rejectReason[item.lineNo]); err != nil {
			return result, err
		}
	}

	for _, item := range ready {
		p := item.projection
		kinds, err := json.Marshal(p.Kinds)
		if err != nil {
			return result, err
		}
		var inserted bool
		// The guard that keeps a replayed older delivery from overwriting a
		// newer one. Without it, loading last week's file after this week's
		// would silently roll the catalogue back.
		err = tx.QueryRow(ctx, `
			INSERT INTO places (id, destination_id, name, category, kinds,
			  address, lat, lng, geo_precision, geo_evidence, province_code,
			  source, source_ref, source_kind, source_updated_at,
			  confidence, evidence_posts, description, reviews, traits, status)
			VALUES ($1,$2,$3,$4,$5::jsonb,$6,$7,$8,$9,$10,$11,
			  'vnlocal',$12,$13,$14,$15,$16,$17,$18,'[]'::jsonb,'active')
			ON CONFLICT (id) DO UPDATE SET
			  name = EXCLUDED.name,
			  category = EXCLUDED.category,
			  kinds = EXCLUDED.kinds,
			  address = EXCLUDED.address,
			  lat = EXCLUDED.lat,
			  lng = EXCLUDED.lng,
			  geo_precision = EXCLUDED.geo_precision,
			  geo_evidence = EXCLUDED.geo_evidence,
			  province_code = EXCLUDED.province_code,
			  destination_id = EXCLUDED.destination_id,
			  source_kind = EXCLUDED.source_kind,
			  source_updated_at = EXCLUDED.source_updated_at,
			  confidence = EXCLUDED.confidence,
			  evidence_posts = EXCLUDED.evidence_posts,
			  description = EXCLUDED.description,
			  reviews = EXCLUDED.reviews,
			  updated_at = clock_timestamp()
			WHERE places.source_updated_at IS NULL
			   OR EXCLUDED.source_updated_at IS NULL
			   OR EXCLUDED.source_updated_at >= places.source_updated_at
			RETURNING (xmax = 0)`,
			p.ID, ProvinceDestinationID(*p.ProvinceCode), p.Name, p.Category,
			string(kinds), p.Address, p.Lat, p.Lng, p.GeoPrecision,
			p.GeoEvidence, p.ProvinceCode, p.SourceRef, p.SourceKind,
			p.SourceUpdate, p.Confidence, p.EvidencePost, p.Description,
			jsonOrNull(p.Reviews),
		).Scan(&inserted)
		switch {
		case err == pgx.ErrNoRows:
			// The guard refused it: what is in the catalogue is newer.
			result.Stale++
			continue
		case err != nil:
			return result, err
		case inserted:
			result.Inserted++
		default:
			result.Updated++
		}

		for _, post := range p.Posts {
			if _, err := tx.Exec(ctx, `
				INSERT INTO place_source_post (platform, post_id, place_id, source_url)
				VALUES ($1,$2,$3,NULLIF($4,''))
				ON CONFLICT (platform, post_id, place_id) DO NOTHING`,
				post.Platform, post.PostID, p.ID, post.SourceURL); err != nil {
				return result, err
			}
			result.Posts++
		}
	}

	if _, err := tx.Exec(ctx,
		`UPDATE ingest_batch SET applied_at = clock_timestamp() WHERE id = $1`,
		batchID); err != nil {
		return result, err
	}
	return result, tx.Commit(ctx)
}

// jsonOrNull keeps an absent prose block out of the column rather than writing
// the four bytes "null" into it.
func jsonOrNull(raw json.RawMessage) any {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	return []byte(raw)
}

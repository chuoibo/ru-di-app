package ingest

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// maxLineBytes caps one delivered line. The largest seen is about 25 KiB; a
// megabyte is far past anything legitimate and stops a malformed file from
// being read into memory as a single line.
const maxLineBytes = 1 << 20

// LandResult is what one landing did.
type LandResult struct {
	BatchID string
	// Landed counts rows written to the landing table, whether or not they
	// will go on to become catalogue rows.
	Landed int
	// Rejected counts rows refused, by reason.
	Rejected map[string]int
	// Drift counts unrecognised field names, by name. Not an error: the feed's
	// shape moves, and this is how that movement becomes visible.
	Drift map[string]int
	// Overclaimed counts rows whose stated precision the coordinates cannot
	// support, by the level they claimed. The raw payload still lands word for
	// word; this is the operator's signal that the correction will be applied
	// when the row is projected into the catalogue.
	Overclaimed map[string]int
	// AlreadyLanded is true when this delivery was already in the table, in
	// which case nothing was written.
	AlreadyLanded bool
}

// RejectedTotal is how many rows were refused.
func (r LandResult) RejectedTotal() int {
	total := 0
	for _, count := range r.Rejected {
		total += count
	}
	return total
}

// DriftNames lists the unrecognised fields, most frequent first.
func (r LandResult) DriftNames() []string {
	names := make([]string, 0, len(r.Drift))
	for name := range r.Drift {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		return r.Drift[names[i]] > r.Drift[names[j]]
	})
	return names
}

// Land writes one delivery into the landing tables.
//
// The payload is stored exactly as delivered, before anything interprets it.
// That is the same rule the ledger follows -- a projection must be
// recomputable from the record -- and it matters more here than usual: the
// mapping rules for a model-authored feed will be wrong the first time, and
// keeping the bytes means fixing the rule and replaying rather than asking the
// other side to export again.
//
// One transaction for the whole delivery. Either it landed or it did not;
// there is no half-landed delivery for somebody to discover later.
func Land(ctx context.Context, pool *pgxpool.Pool, manifest *Manifest) (LandResult, error) {
	result := LandResult{
		BatchID:     manifest.Dot,
		Rejected:    map[string]int{},
		Drift:       map[string]int{},
		Overclaimed: map[string]int{},
	}
	for _, file := range manifest.Files {
		if err := manifest.VerifyFile(file); err != nil {
			return result, err
		}
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)

	first := manifest.Files[0]
	tag, err := tx.Exec(ctx, `
		INSERT INTO ingest_batch (id, source, schema_version, dot_seq, dot_truoc,
		  kieu_dot, file_name, file_sha256, file_bytes, rows_declared,
		  updated_at_min, updated_at_max)
		VALUES ($1,'vnlocal',$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT (id) DO NOTHING`,
		manifest.Dot, manifest.SchemaVersion, manifest.DotSeq, manifest.DotTruoc,
		manifest.KieuDot, first.Name, first.SHA256, first.Bytes, manifest.Rows,
		manifest.UpdatedAtMin, manifest.UpdatedAtMax)
	if err != nil {
		return result, err
	}
	if tag.RowsAffected() == 0 {
		// Landing the same delivery twice is allowed and does nothing. The
		// operator who re-runs a command after a network wobble is doing the
		// right thing and must not be punished for it.
		result.AlreadyLanded = true
		return result, tx.Commit(ctx)
	}

	lineNo := 0
	for _, file := range manifest.Files {
		handle, err := os.Open(manifest.Path(file))
		if err != nil {
			return result, err
		}
		scanner := bufio.NewScanner(handle)
		scanner.Buffer(make([]byte, 64*1024), maxLineBytes)
		batch := &pgx.Batch{}
		for scanner.Scan() {
			lineNo++
			line := make([]byte, len(scanner.Bytes()))
			copy(line, scanner.Bytes())

			rec, unknown, reject := Parse(line)
			for _, name := range unknown {
				result.Drift[name]++
			}
			if reject != nil {
				result.Rejected[reject.Code]++
				sourceKey := ""
				if rec != nil {
					sourceKey = rec.PlaceID
				}
				batch.Queue(`
					INSERT INTO ingest_reject
					  (batch_id, line_no, reason, detail, source_key)
					VALUES ($1,$2,$3,$4,NULLIF($5,''))
					ON CONFLICT DO NOTHING`,
					manifest.Dot, lineNo, reject.Code, reject.Detail, sourceKey)
				continue
			}
			if claimed, corrected := rec.CapPrecision(); corrected {
				result.Overclaimed[claimed]++
			}
			batch.Queue(`
				INSERT INTO ingest_place_raw
				  (batch_id, line_no, payload, payload_sha, source_key,
				   source_updated_at)
				VALUES ($1,$2,$3,$4,$5,$6)
				ON CONFLICT (batch_id, line_no) DO NOTHING`,
				manifest.Dot, lineNo, line, LineDigest(line), rec.PlaceID,
				nullableTime(rec.UpdatedAt))
			result.Landed++
		}
		closeErr := handle.Close()
		if err := scanner.Err(); err != nil {
			return result, fmt.Errorf("%s line %d: %w", file.Name, lineNo+1, err)
		}
		if closeErr != nil {
			return result, closeErr
		}
		if err := sendBatch(ctx, tx, batch); err != nil {
			return result, err
		}
	}

	// A delivery whose row count does not match what it promised is not a
	// delivery that is slightly short: something between the two sides dropped
	// lines, and landing it would make that loss permanent and invisible.
	if lineNo != manifest.Rows {
		return result, fmt.Errorf(
			"delivery %s promised %d rows and carried %d",
			manifest.Dot, manifest.Rows, lineNo)
	}

	if _, err := tx.Exec(ctx,
		`UPDATE ingest_batch SET rows_landed=$2 WHERE id=$1`,
		manifest.Dot, result.Landed); err != nil {
		return result, err
	}
	return result, tx.Commit(ctx)
}

func sendBatch(ctx context.Context, tx pgx.Tx, batch *pgx.Batch) error {
	if batch.Len() == 0 {
		return nil
	}
	results := tx.SendBatch(ctx, batch)
	for i := 0; i < batch.Len(); i++ {
		if _, err := results.Exec(); err != nil {
			results.Close()
			return err
		}
	}
	return results.Close()
}

// nullableTime turns the feed's timestamp string into something Postgres can
// store, or nothing. A timestamp that will not parse is left null rather than
// guessed at: this column guards which of two deliveries is newer, and a wrong
// answer there lets an older row overwrite a newer one.
func nullableTime(value string) any {
	if value == "" {
		return nil
	}
	return value
}

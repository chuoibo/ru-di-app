//go:build postgres

package ingest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"mobile/services/core/internal/testdb"
)

// writeDelivery builds a synthetic delivery on disk: a data file and a
// manifest that describes it truthfully unless `damage` says otherwise.
//
// Invented, like every fixture here. A real delivery names real businesses and
// carries real platform post ids, and neither belongs in the repository.
func writeDelivery(t *testing.T, rows []map[string]any, damage func(*Manifest)) string {
	t.Helper()
	dir := t.TempDir()
	var body strings.Builder
	for _, row := range rows {
		line, err := json.Marshal(row)
		if err != nil {
			t.Fatal(err)
		}
		body.Write(line)
		body.WriteByte('\n')
	}
	data := []byte(body.String())
	name := "places-probe.ndjson"
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	manifest := Manifest{
		SchemaVersion: SchemaVersion,
		Dot:           "probe-" + hex.EncodeToString(sum[:4]),
		DotSeq:        1,
		KieuDot:       "toan_bo",
		Rows:          len(rows),
		Files: []ManifestFile{{
			Name:   name,
			Rows:   len(rows),
			Bytes:  int64(len(data)),
			SHA256: hex.EncodeToString(sum[:]),
		}},
	}
	if damage != nil {
		damage(&manifest)
	}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "manifest-probe.json")
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func probeRow(id string, mutate func(map[string]any)) map[string]any {
	row := map[string]any{
		"schema_version": "place.v1",
		"place_id":       id,
		"source":         "vnlocal-ai",
		"ten_chuan":      "Quán " + id,
		"loai":           "cafe",
		"category":       []any{"ca phe"},
		"province_code":  79,
		"posts": []any{map[string]any{
			"platform": "tiktok", "post_id": "post-" + id,
			"source_url": "https://example.test/" + id,
		}},
		"frames":           []any{},
		"trung_lap_voi":    []any{},
		"trung_toa_do_voi": []any{},
		"geo":              nil,
		"updated_at":       "2026-09-22T08:47:24.056000+00:00",
	}
	if mutate != nil {
		mutate(row)
	}
	return row
}

func migratedPool(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()
	ctx := context.Background()
	pool := testdb.Pool(t)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return pool, ctx
}

func countIn(t *testing.T, pool *pgxpool.Pool, table, batchID string) int {
	t.Helper()
	var n int
	column := "batch_id"
	if table == "ingest_batch" {
		column = "id" // the delivery table keys on its own id, not a reference
	}
	sql := fmt.Sprintf(`SELECT count(*) FROM %s WHERE %s=$1`, table, column)
	if err := pool.QueryRow(context.Background(), sql, batchID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// TestLandingIsIdempotent. An operator re-running a command after a network
// wobble is doing the right thing and must not be punished for it.
func TestLandingIsIdempotent(t *testing.T) {
	pool, ctx := migratedPool(t)
	path := writeDelivery(t, []map[string]any{
		probeRow("plc_a", nil),
		probeRow("plc_b", nil),
		probeRow("plc_c", func(r map[string]any) { r["posts"] = []any{} }),
	}, nil)
	manifest, err := ReadManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM ingest_reject WHERE batch_id=$1`, manifest.Dot)
		_, _ = pool.Exec(ctx, `DELETE FROM ingest_place_raw WHERE batch_id=$1`, manifest.Dot)
		_, _ = pool.Exec(ctx, `DELETE FROM ingest_batch WHERE id=$1`, manifest.Dot)
	})

	first, err := Land(ctx, pool, manifest)
	if err != nil {
		t.Fatalf("first landing: %v", err)
	}
	if first.Landed != 2 {
		t.Errorf("landed %d rows, want 2", first.Landed)
	}
	if first.Rejected[RejectNoPosts] != 1 {
		t.Errorf("rejects: %v, want one %s", first.Rejected, RejectNoPosts)
	}
	if first.AlreadyLanded {
		t.Error("a first landing must not report itself as a repeat")
	}
	if got := countIn(t, pool, "ingest_place_raw", manifest.Dot); got != 2 {
		t.Errorf("landing table holds %d rows, want 2", got)
	}
	if got := countIn(t, pool, "ingest_reject", manifest.Dot); got != 1 {
		t.Errorf("reject table holds %d rows, want 1", got)
	}

	second, err := Land(ctx, pool, manifest)
	if err != nil {
		t.Fatalf("second landing: %v", err)
	}
	if !second.AlreadyLanded {
		t.Error("the second landing must recognise the delivery")
	}
	if got := countIn(t, pool, "ingest_place_raw", manifest.Dot); got != 2 {
		t.Errorf("a repeat landing changed the table: %d rows", got)
	}
}

// TestADeliveryThatDoesNotMatchItsManifestIsRefusedWhole. A short delivery is
// not slightly wrong: lines went missing between the two sides, and landing it
// would make that loss permanent and invisible.
func TestADeliveryThatDoesNotMatchItsManifestIsRefusedWhole(t *testing.T) {
	pool, ctx := migratedPool(t)

	t.Run("the digest does not match the bytes", func(t *testing.T) {
		path := writeDelivery(t, []map[string]any{probeRow("plc_x", nil)},
			func(m *Manifest) {
				m.Files[0].SHA256 = strings.Repeat("0", 64)
			})
		manifest, err := ReadManifest(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Land(ctx, pool, manifest); err == nil {
			t.Fatal("a file that is not the one described was accepted")
		}
		if got := countIn(t, pool, "ingest_batch", manifest.Dot); got != 0 {
			t.Errorf("a refused delivery left %d rows behind", got)
		}
	})

	t.Run("the row count does not match", func(t *testing.T) {
		path := writeDelivery(t, []map[string]any{
			probeRow("plc_y", nil), probeRow("plc_z", nil),
		}, func(m *Manifest) { m.Rows = 3 })
		manifest, err := ReadManifest(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Land(ctx, pool, manifest); err == nil {
			t.Fatal("a short delivery was accepted")
		}
		// The whole landing is one transaction, so a refusal leaves nothing.
		if got := countIn(t, pool, "ingest_place_raw", manifest.Dot); got != 0 {
			t.Errorf("a refused delivery left %d landed rows", got)
		}
	})
}

// TestTheRawPayloadIsKeptWordForWord is what makes a wrong mapping rule
// survivable: fix the rule, replay, no need to ask for another export.
func TestTheRawPayloadIsKeptWordForWord(t *testing.T) {
	pool, ctx := migratedPool(t)
	path := writeDelivery(t, []map[string]any{
		probeRow("plc_keep", func(r map[string]any) {
			r["khu_vuc_raw"] = "một chuỗi có dấu"
		}),
	}, nil)
	manifest, err := ReadManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM ingest_place_raw WHERE batch_id=$1`, manifest.Dot)
		_, _ = pool.Exec(ctx, `DELETE FROM ingest_batch WHERE id=$1`, manifest.Dot)
	})
	if _, err := Land(ctx, pool, manifest); err != nil {
		t.Fatal(err)
	}
	var stored string
	if err := pool.QueryRow(ctx, `
		SELECT payload->>'khu_vuc_raw' FROM ingest_place_raw
		WHERE batch_id=$1 AND source_key='plc_keep'`, manifest.Dot).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored != "một chuỗi có dấu" {
		t.Errorf("the payload came back as %q", stored)
	}
}

// TestDriftIsCountedDuringLanding: a field nobody described still reaches the
// landing table, and the operator is told it appeared.
func TestDriftIsCountedDuringLanding(t *testing.T) {
	pool, ctx := migratedPool(t)
	path := writeDelivery(t, []map[string]any{
		probeRow("plc_d1", func(r map[string]any) { r["truong_moi"] = 1 }),
		probeRow("plc_d2", func(r map[string]any) { r["truong_moi"] = 2 }),
	}, nil)
	manifest, err := ReadManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM ingest_place_raw WHERE batch_id=$1`, manifest.Dot)
		_, _ = pool.Exec(ctx, `DELETE FROM ingest_batch WHERE id=$1`, manifest.Dot)
	})
	result, err := Land(ctx, pool, manifest)
	if err != nil {
		t.Fatal(err)
	}
	if result.Drift["truong_moi"] != 2 {
		t.Errorf("drift: %v, want truong_moi on both rows", result.Drift)
	}
	if result.Landed != 2 {
		t.Errorf("an unfamiliar field must not stop the rows: landed %d", result.Landed)
	}
}

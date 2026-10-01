//go:build postgres

package nap_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"

	"mobile/services/core/internal/naptest"
	"mobile/services/core/internal/rag/nap"
)

// The freshness numbers: a row waiting only for its vector holds current
// attributes and is no lag, and a new mark on it starts its lag afresh (a
// hot place waiting for the batch door does not inflate the number); a row
// deferred after a failure keeps its lag; a background re-check is no lag;
// an ingest that stopped rounding breaks the SLO; an enrichment from
// vnlocal (place_lam_giau) counts as one.
func TestDoTuoi(t *testing.T) {
	pool := naptest.Pool(t)
	ctx := context.Background()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	// The ingest's tables, in this test's own schema, created before any
	// statement naming them is prepared (a prepared statement keeps the
	// table it resolved, and the shared database may hold public copies).
	var schema string
	_ = pool.QueryRow(ctx, `SELECT current_schema()`).Scan(&schema)
	s := pgx.Identifier{schema}.Sanitize()
	exec(`CREATE TABLE ` + s + `.ingest_do_tre (source text PRIMARY KEY, vong_at timestamptz NOT NULL, con_tro timestamptz,
		nguon_moi_nhat timestamptz, tre_giay double precision NOT NULL)`)
	exec(`CREATE TABLE ` + s + `.place_lam_giau (place_id text PRIMARY KEY, di_ung text[] NOT NULL DEFAULT '{}',
		an_kieng text[] NOT NULL DEFAULT '{}', khi_chat text[] NOT NULL DEFAULT '{}', mon_chinh text[] NOT NULL DEFAULT '{}',
		chen_lenh boolean NOT NULL DEFAULT false, tin_cay text NOT NULL DEFAULT 'thap', model text, prompt_version text)`)
	v := naptest.Vang(t)
	naptest.NapVang(t, pool, v)
	n, enc := naptest.Nap(t, nap.NewKhoNho())
	naptest.LamGiauNhanTay(t, pool, n, v)
	naptest.BuildEvalPromote(t, pool, n, enc, v)
	doc := func() nap.DoTuoi {
		t.Helper()
		d, err := nap.DocDoTuoi(ctx, pool)
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	if d := doc(); d.LechGiay != 0 || d.CanLam != 0 || len(d.ViPham()) != 0 {
		t.Fatalf("a drained index: %+v", d)
	}
	a, b, c, e := v.Quan[0].ID, v.Quan[1].ID, v.Quan[2].ID, v.Quan[3].ID
	// a: waited ten minutes for its vector, attributes current.
	exec(`SELECT rag_danh_dau('place', ARRAY[$1::text], NULL, 0::smallint)`, a)
	exec(`UPDATE rag_dirty SET cho_den = now() + interval '1 hour', cho_tu = now() - interval '10 minutes',
		noticed_at = now() - interval '10 minutes' WHERE doc_id = $1`, a)
	// b: deferred after a failure ten minutes ago.
	exec(`SELECT rag_danh_dau('place', ARRAY[$1::text], NULL, 0::smallint)`, b)
	exec(`UPDATE rag_dirty SET cho_den = now() + interval '1 hour', noticed_at = now() - interval '10 minutes' WHERE doc_id = $1`, b)
	exec(`INSERT INTO rag_ingest_dlq(corpus, doc_id, chang, ma_loi) VALUES('place', $1, 'ghi', 'milvus')`, b)
	// c: a background re-check, old.
	exec(`SELECT rag_danh_dau('place', ARRAY[$1::text], NULL, (-1)::smallint)`, c)
	exec(`UPDATE rag_dirty SET noticed_at = now() - interval '1 hour' WHERE doc_id = $1`, c)
	// e: waiting for the batch door.
	exec(`SELECT rag_danh_dau('place', ARRAY[$1::text], NULL, 0::smallint)`, e)
	exec(`UPDATE rag_dirty SET cho_den = now() + interval '6 hours', cho_nhung = true, cho_tu = now() - interval '20 minutes',
		noticed_at = now() - interval '20 minutes' WHERE doc_id = $1`, e)
	d := doc()
	if d.LechGiay < 595 || d.LechGiay > 700 || d.CanLam != 1 || d.Hoan != 1 || d.NenCho != 1 || d.ChoNhung != 1 ||
		d.ChoNhungGiay < 1195 || d.DLQ != 1 {
		t.Fatalf("readings: %+v", d)
	}
	// New marks: a's lag starts afresh, b's does not.
	exec(`SELECT rag_danh_dau('place', ARRAY[$1::text, $2::text], NULL, 0::smallint)`, a, b)
	var la, lb float64
	_ = pool.QueryRow(ctx, `SELECT EXTRACT(EPOCH FROM now() - noticed_at) FROM rag_dirty WHERE doc_id=$1`, a).Scan(&la)
	_ = pool.QueryRow(ctx, `SELECT EXTRACT(EPOCH FROM now() - noticed_at) FROM rag_dirty WHERE doc_id=$1`, b).Scan(&lb)
	if la > 5 || lb < 595 {
		t.Fatalf("after a new mark: waiting row lag %.0f s, failing row lag %.0f s", la, lb)
	}

	// The ingest's rounds.
	exec(`INSERT INTO ` + s + `.ingest_do_tre VALUES ('vnlocal.places', now() - interval '10 seconds', NULL, NULL, 0)`)
	if d := doc(); len(d.Ingest) != 1 || d.Ingest[0].VongGiay > 60 {
		t.Fatalf("a live ingest: %+v", d.Ingest)
	}
	exec(`UPDATE ` + s + `.ingest_do_tre SET vong_at = now() - interval '10 minutes'`)
	found := false
	for _, x := range doc().ViPham() {
		found = found || x == "ingest"
	}
	if !found {
		t.Fatal("a stalled ingest did not break the SLO")
	}

	// An enrichment from vnlocal counts.
	thieu := func() int {
		t.Helper()
		st, err := n.DocTrangThai(ctx, pool)
		if err != nil {
			t.Fatal(err)
		}
		return st[0].ThieuLamGiau
	}
	before := thieu()
	exec(`DELETE FROM place_enrichments WHERE place_id = $1`, a)
	if got := thieu(); got != before+1 {
		t.Fatalf("a place losing its enrichment: %d -> %d", before, got)
	}
	exec(`INSERT INTO `+s+`.place_lam_giau VALUES ($1)`, a)
	if got := thieu(); got != before {
		t.Fatalf("vnlocal's enrichment did not count: %d, want %d", got, before)
	}
}

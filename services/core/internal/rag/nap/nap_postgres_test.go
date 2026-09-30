//go:build postgres

package nap_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"mobile/services/core/internal/naptest"
	"mobile/services/core/internal/rag/nap"
	"mobile/services/core/internal/testdb"
)

// The lifecycle on PostgreSQL with the in-memory store; the Milvus tier runs
// the same scenario against Milvus (vectordb/napkho).
func TestVongDoiTrenKhoNho(t *testing.T) {
	naptest.VongDoi(t, naptest.Pool(t), nap.NewKhoNho())
}

func TestTombstoneKhongSongLaiSauRollbackKhoNho(t *testing.T) {
	naptest.TombstoneQuaRollback(t, naptest.Pool(t), nap.NewKhoNho())
}

func TestDoiSoatChanPromoteKhoNho(t *testing.T) {
	naptest.DoiSoatChanPromote(t, naptest.Pool(t), nap.NewKhoNho())
}

func TestThamDoViPhamChanCongKhoNho(t *testing.T) {
	naptest.ThamDoViPhamChanCong(t, naptest.Pool(t), nap.NewKhoNho())
}

func TestThuocTinhKhacCauHinhKhoNho(t *testing.T) {
	naptest.ThuocTinhKhacCauHinh(t, naptest.Pool(t), nap.NewKhoNho())
}

func TestMigrateNapIdempotentVaTuChoiChecksumLech(t *testing.T) {
	pool := naptest.Pool(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `UPDATE rag_nap_schema_migrations SET digest='x' WHERE version=1`); err != nil {
		t.Fatal(err)
	}
	if err := nap.Migrate(ctx, pool); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("a changed checksum was accepted: %v", err)
	}
}

// Without the outbox's version 2 the migration refuses before any DDL: the
// trigger would call a lane the outbox's CHECK refuses and fail every write
// to places.
func TestMigrateNapDoiOutboxV2(t *testing.T) {
	pool := naptest.Pool(t)
	ctx := context.Background()
	for _, s := range []string{`DROP TRIGGER rag_nap_places_dirty ON places`, `DROP TABLE rag_nap_schema_migrations`,
		`DELETE FROM job_schema_migrations WHERE version=2`} {
		if _, err := pool.Exec(ctx, s); err != nil {
			t.Fatal(err)
		}
	}
	if err := nap.Migrate(ctx, pool); !errors.Is(err, nap.ErrThieuPhuThuoc) {
		t.Fatalf("migrated without the outbox v2: %v", err)
	}
	var has bool
	_ = pool.QueryRow(ctx, `SELECT to_regclass('rag_nap_schema_migrations') IS NOT NULL`).Scan(&has)
	if has {
		t.Fatal("the refused migration left DDL behind")
	}
}

// The trigger: one row per document however often it changes, the first
// change's time kept, the count moving; a delete marks xoa; one lane message
// per corpus per minute, an id and a number only.
func TestTriggerRagDirtyVaLane(t *testing.T) {
	pool := naptest.Pool(t)
	ctx := context.Background()
	v := naptest.Vang(t)
	naptest.NapVang(t, pool, v)
	var n, lane int
	_ = pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM rag_dirty), (SELECT count(*) FROM job_outbox WHERE queue='rag')`).Scan(&n, &lane)
	if n != len(v.Quan) || lane < 1 || lane > 3 {
		t.Fatalf("after %d inserts: %d dirty rows, %d lane messages", len(v.Quan), n, lane)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM rag_dirty`); err != nil {
		t.Fatal(err)
	}
	id := v.Quan[0].ID
	for i := 0; i < 5; i++ {
		if _, err := pool.Exec(ctx, `UPDATE places SET updated_at=now() WHERE id=$1`, id); err != nil {
			t.Fatal(err)
		}
	}
	var lan int64
	var first time.Time
	var xoa bool
	if err := pool.QueryRow(ctx, `SELECT lan, noticed_at, xoa FROM rag_dirty WHERE doc_id=$1`, id).Scan(&lan, &first, &xoa); err != nil || lan != 5 || xoa {
		t.Fatalf("five updates: lan=%d xoa=%v err=%v", lan, xoa, err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM places WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	var again time.Time
	if err := pool.QueryRow(ctx, `SELECT lan, noticed_at, xoa FROM rag_dirty WHERE doc_id=$1`, id).Scan(&lan, &again, &xoa); err != nil || lan != 6 || !xoa || !again.Equal(first) {
		t.Fatalf("delete: lan=%d xoa=%v first kept=%v", lan, xoa, again.Equal(first))
	}
	var refs int
	_ = pool.QueryRow(ctx, `SELECT count(DISTINCT ref_id) FROM job_outbox WHERE queue='rag'`).Scan(&refs)
	if refs != 1 {
		t.Fatalf("lane messages name %d refs; one per corpus", refs)
	}
}

// No ingestion table names a person.
func TestBangNapKhongCoCotNguoi(t *testing.T) {
	pool := naptest.Pool(t)
	rows, err := pool.Query(context.Background(), `SELECT table_name, column_name FROM information_schema.columns
		WHERE table_schema = current_schema() AND (table_name LIKE 'rag_%' OR table_name='place_enrichments')`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var tb, col string
		if err := rows.Scan(&tb, &col); err != nil {
			t.Fatal(err)
		}
		n++
		for _, bad := range []string{"person", "user", "account", "author", "member"} {
			if strings.Contains(col, bad) {
				t.Errorf("%s.%s names a person", tb, col)
			}
		}
	}
	if n < 30 {
		t.Fatalf("read %d columns; the query slipped", n)
	}
}

// The review queue and the verdicts: a verdict marks the place dirty, and a
// rejection withdraws the enrichment (allergens unknown from then on).
func TestHangDuyetVaVerdict(t *testing.T) {
	pool := naptest.Pool(t)
	ctx := context.Background()
	v := naptest.Vang(t)
	naptest.NapVang(t, pool, v)
	n, _ := naptest.Nap(t, nap.NewKhoNho())
	naptest.LamGiauNhanTay(t, pool, n, v)
	if _, err := pool.Exec(ctx, `UPDATE place_enrichments SET review='auto'`); err != nil {
		t.Fatal(err)
	}
	var rows int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM place_enrichments`).Scan(&rows)
	q, err := nap.HangDuyet(ctx, pool, false, 1000)
	if err != nil || len(q) != rows || rows < len(v.Quan)-5 {
		t.Fatalf("queue: %d rows, %v (seed rows are always reviewed)", len(q), err)
	}
	id := q[0].PlaceID
	if err := nap.Duyet(ctx, pool, id, q[0].Ban, nap.ReviewRejected); err != nil {
		t.Fatal(err)
	}
	if err := nap.Duyet(ctx, pool, "khong-co", "0123456789abcdef", nap.ReviewReviewed); !errors.Is(err, nap.ErrKhongCoLamGiau) {
		t.Fatalf("verdict on nothing: %v", err)
	}
	if err := nap.Duyet(ctx, pool, id, q[0].Ban, "auto"); err == nil {
		t.Fatal("a verdict of auto was accepted")
	}
	if err := nap.Duyet(ctx, pool, id, "x", nap.ReviewReviewed); err == nil {
		t.Fatal("a verdict naming no version was accepted")
	}
	var dirty int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM rag_dirty WHERE doc_id=$1`, id).Scan(&dirty)
	if dirty != 1 {
		t.Fatal("a verdict did not mark the place dirty")
	}
	lg, _ := nap.DocLamGiau(ctx, pool, []string{id})
	if at := nap.ApDung(lg[id], lg[id].NguonHash); at.Co || at.DiUngRo {
		t.Fatalf("a rejected enrichment still applies: %+v", at)
	}
}

// A verdict binds to the version the reviewer saw (F6): when the indexer
// replaces the enrichment between the queue listing and the verdict, the
// verdict matches nothing, changes nothing, and says so; the new version
// can then be approved by its own version.
func TestDuyetGanVoiBanDaXem(t *testing.T) {
	pool := naptest.Pool(t)
	ctx := context.Background()
	v := naptest.Vang(t)
	naptest.NapVang(t, pool, v)
	n, _ := naptest.Nap(t, nap.NewKhoNho())
	naptest.LamGiauNhanTay(t, pool, n, v)
	if _, err := pool.Exec(ctx, `UPDATE place_enrichments SET review='auto'`); err != nil {
		t.Fatal(err)
	}
	q, err := nap.HangDuyet(ctx, pool, false, 1000)
	if err != nil || len(q) == 0 {
		t.Fatal(err)
	}
	seen := q[0]
	lg, _ := nap.DocLamGiau(ctx, pool, []string{seen.PlaceID})
	moi := *lg[seen.PlaceID]
	moi.KetQua.AnKieng, moi.KetQua.AnKiengRo = []string{"thuan_chay"}, true
	moi.Review = nap.ReviewAuto
	if err := nap.GhiLamGiau(ctx, pool, []nap.LamGiau{moi}); err != nil {
		t.Fatal(err)
	}
	if err := nap.Duyet(ctx, pool, seen.PlaceID, seen.Ban, nap.ReviewReviewed); !errors.Is(err, nap.ErrBanDuyet) {
		t.Fatalf("a verdict on a replaced enrichment: %v", err)
	}
	after, _ := nap.DocLamGiau(ctx, pool, []string{seen.PlaceID})
	if after[seen.PlaceID].Review != nap.ReviewAuto {
		t.Fatalf("the unseen output was approved: %s", after[seen.PlaceID].Review)
	}
	q2, _ := nap.HangDuyet(ctx, pool, true, 1000)
	var fresh string
	for _, m := range q2 {
		if m.PlaceID == seen.PlaceID {
			fresh = m.Ban
		}
	}
	if fresh == "" || fresh == seen.Ban {
		t.Fatalf("the queue shows the same version for a changed output: %q", fresh)
	}
	if err := nap.Duyet(ctx, pool, seen.PlaceID, fresh, nap.ReviewReviewed); err != nil {
		t.Fatal(err)
	}
}

// Promote re-checks the verdict it serves (F8): an evaluated version is
// refused when the running configuration's fingerprint differs from the one
// it was built and evaluated under, when the golden file changed since, or
// when the recorded verdict names another fingerprint; with all three
// holding it serves.
func TestPromoteKiemLaiVanTayVaVang(t *testing.T) {
	pool := naptest.Pool(t)
	ctx := context.Background()
	v := naptest.Vang(t)
	naptest.NapVang(t, pool, v)
	n, enc := naptest.Nap(t, nap.NewKhoNho())
	naptest.LamGiauNhanTay(t, pool, n, v)
	rep, err := n.Dung(ctx, pool, nap.CorpusQuan)
	if err != nil {
		t.Fatal(err)
	}
	if k, err := n.DanhGia(ctx, pool, rep.PhienBan, enc, v); err != nil || !k.Dat {
		t.Fatalf("gate: %+v %v", k, err)
	}
	if _, err := n.Promote(ctx, pool, rep.PhienBan, strings.Repeat("0", 64)); !errors.Is(err, nap.ErrCong) {
		t.Fatalf("promoted on a verdict from another golden set: %v", err)
	}
	// A configuration change that moves the fingerprint but not the ranking
	// (how long retired versions are kept): the subject is that any change
	// stops for a person, not how far quality moves.
	n2 := n
	n2.Cfg.GiuBan.Ngay++
	if _, err := n2.Promote(ctx, pool, rep.PhienBan, v.Sha); !errors.Is(err, nap.ErrCong) {
		t.Fatalf("promoted under a changed configuration: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE rag_vector_versions SET eval = jsonb_set(eval, '{van_tay}', '"ffffffffffff"') WHERE id=$1`, rep.PhienBan); err != nil {
		t.Fatal(err)
	}
	if _, err := n.Promote(ctx, pool, rep.PhienBan, v.Sha); !errors.Is(err, nap.ErrCong) {
		t.Fatalf("promoted on a verdict recorded under another fingerprint: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE rag_vector_versions SET eval = jsonb_set(eval, '{van_tay}', to_jsonb($2::text)) WHERE id=$1`, rep.PhienBan, n.Cfg.VanTay()); err != nil {
		t.Fatal(err)
	}
	if _, err := n.Promote(ctx, pool, rep.PhienBan, v.Sha); err != nil {
		t.Fatalf("a verdict that still holds was refused: %v", err)
	}
}

// failKho fails every write: the indexer counts dead letters and, after
// MaxThuLai passes, moves the document out of rag_dirty; retry brings it
// back.
type failKho struct{ nap.KhoVector }

func (failKho) Upsert(context.Context, string, []nap.Hang) error { return errors.New("down") }

func TestChiMucDLQ(t *testing.T) {
	pool := naptest.Pool(t)
	ctx := context.Background()
	v := naptest.Vang(t)
	naptest.NapVang(t, pool, v)
	kho := nap.NewKhoNho()
	n, enc := naptest.Nap(t, kho)
	naptest.LamGiauNhanTay(t, pool, n, v)
	naptest.BuildEvalPromote(t, pool, n, enc, v)
	id := "dl-tiem-banh-may-xanh"
	if _, err := pool.Exec(ctx, `UPDATE places SET updated_at=now() WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	bad := nap.ChiMuc{Nap: nap.Nap{Kho: failKho{kho}, Dense: n.Dense, Cfg: n.Cfg}}
	for i := 1; i <= nap.MaxThuLai; i++ {
		b := naptest.ChiMuc(t, pool, bad)
		if b.Hong != 1 {
			t.Fatalf("pass %d: %+v", i, b)
		}
	}
	var dirty, so int
	_ = pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM rag_dirty), (SELECT so_lan FROM rag_ingest_dlq WHERE doc_id=$1)`, id).Scan(&dirty, &so)
	if dirty != 0 || so != nap.MaxThuLai {
		t.Fatalf("after %d failures: dirty %d, dead letter count %d", nap.MaxThuLai, dirty, so)
	}
	st, _ := n.DocTrangThai(ctx, pool)
	if st[0].DLQ != 1 {
		t.Fatalf("status DLQ %d", st[0].DLQ)
	}
	back, err := nap.ThuLaiDLQ(ctx, pool)
	if err != nil || back != 1 {
		t.Fatalf("retry: %d %v", back, err)
	}
	good := nap.ChiMuc{Nap: n}
	if b := naptest.ChiMuc(t, pool, good); b.Ghi != 1 || b.Hong != 0 {
		t.Fatalf("after retry: %+v", b)
	}
}

// Auto-promotion: a rebuild with nothing changed under the same
// configuration promotes itself; a changed configuration stops at
// evaluated for a person.
func TestDungTuDong(t *testing.T) {
	pool := naptest.Pool(t)
	ctx := context.Background()
	v := naptest.Vang(t)
	naptest.NapVang(t, pool, v)
	kho := nap.NewKhoNho()
	n, enc := naptest.Nap(t, kho)
	naptest.LamGiauNhanTay(t, pool, n, v)
	first, err := n.DungTuDong(ctx, pool, nap.CorpusQuan, enc, v)
	if err != nil || first.TuDong || first.LyDoTay != "chua_co_active" {
		t.Fatalf("first build: %+v %v", first, err)
	}
	if _, err := n.Promote(ctx, pool, first.PhienBan, v.Sha); err != nil {
		t.Fatal(err)
	}
	same, err := n.DungTuDong(ctx, pool, nap.CorpusQuan, enc, v)
	if err != nil || !same.TuDong || same.TyLeDoi != 0 {
		t.Fatalf("unchanged rebuild: %+v %v", same, err)
	}
	// A configuration change that moves the fingerprint but not the ranking
	// (how long retired versions are kept): the subject is that any change
	// stops for a person, not how far quality moves.
	n2 := n
	n2.Cfg.GiuBan.Ngay++
	moved, err := n2.DungTuDong(ctx, pool, nap.CorpusQuan, enc, v)
	if err != nil || moved.TuDong || moved.LyDoTay != "doi_cau_hinh" {
		t.Fatalf("changed configuration: %+v %v", moved, err)
	}
	if naptest.State(t, pool, moved.PhienBan) != "evaluated" {
		t.Fatal("the changed-configuration version is not left evaluated")
	}
}

// The manual corpus builds, reconciles and promotes under its own alias.
func TestSoTayDungVaPromote(t *testing.T) {
	pool := naptest.Pool(t)
	ctx := context.Background()
	kho := nap.NewKhoNho()
	n, enc := naptest.Nap(t, kho)
	rep, err := n.Dung(ctx, pool, nap.CorpusSoTay)
	if err != nil || rep.Docs < 20 || rep.Chunks != rep.Docs {
		t.Fatalf("manual build: %+v %v", rep, err)
	}
	k, err := n.DanhGia(ctx, pool, rep.PhienBan, enc, naptest.Vang(t))
	if err != nil || !k.Dat {
		t.Fatalf("manual gate: %+v %v", k, err)
	}
	if _, err := n.Promote(ctx, pool, rep.PhienBan, ""); err != nil {
		t.Fatal(err)
	}
	if got, _ := kho.MoTaAlias(ctx, nap.Alias(nap.CorpusSoTay)); got != rep.Collection {
		t.Fatalf("manual alias → %q", got)
	}
}

// The ingestion tier needs the lexical rag schema too: this file never runs
// without a database (testdb fails under CORE_REQUIRE_POSTGRES_TESTS=1).
var _ = testdb.Pool

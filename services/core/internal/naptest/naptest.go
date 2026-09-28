// Package naptest holds the real-PostgreSQL fixtures and the lifecycle
// scenario of the vector ingestion pipeline (internal/rag/nap), shared by
// two tiers: scripts/go_postgres_tier.sh runs the scenario against the
// in-memory store (nap.KhoNho), scripts/go_milvus_tier.sh against Milvus
// itself. One scenario, two stores: a fake that drifted from Milvus would
// show as the same test green in one tier and red in the other.
//
// It lives outside internal/rag on purpose: its fixtures read job_outbox,
// which the rag read gate (aigate) forbids anything under internal/rag.
// Only tests import it.
package naptest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/jobs"
	"mobile/services/core/internal/rag"
	"mobile/services/core/internal/rag/nap"
	"mobile/services/core/internal/repo"
	"mobile/services/core/internal/testdb"
)

// Pool builds a private schema with copies of destinations and places,
// migrates the outbox (jobs), the lexical rag schema and the ingestion
// schema into it (the last one twice: it must be idempotent), and returns a
// pool whose search_path starts there.
func Pool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	base := testdb.Pool(t)
	tx, err := base.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('rag_test_pg_trgm'))`); err == nil {
		_, err = tx.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS pg_trgm`)
	}
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var b [8]byte
	_, _ = rand.Read(b[:])
	schema := "nap_test_" + hex.EncodeToString(b[:])
	ident := pgx.Identifier{schema}.Sanitize()
	if _, err := base.Exec(ctx, "CREATE SCHEMA "+ident); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = base.Exec(context.Background(), "DROP SCHEMA "+ident+" CASCADE") })
	cfg := base.Config().Copy()
	cfg.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	for _, table := range []string{"destinations", "places"} {
		if _, err := pool.Exec(ctx, fmt.Sprintf("CREATE TABLE %s (LIKE public.%s INCLUDING ALL)", table, table)); err != nil {
			t.Fatal(err)
		}
	}
	if err := jobs.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err := rag.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if ok, err := nap.Installed(ctx, pool); err != nil || ok {
		t.Fatalf("before migrating: installed=%v err=%v", ok, err)
	}
	for i := 0; i < 2; i++ {
		if err := nap.Migrate(ctx, pool); err != nil {
			t.Fatalf("ingestion migration %d: %v", i+1, err)
		}
	}
	if ok, err := nap.Installed(ctx, pool); err != nil || !ok {
		t.Fatalf("after migrating: installed=%v err=%v", ok, err)
	}
	return pool
}

// Vang returns the golden set.
func Vang(t testing.TB) nap.TapVang {
	t.Helper()
	v, err := nap.DocVang(rag.VangDiaDiem())
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// Chen inserts one catalogue row.
func Chen(t *testing.T, pool *pgxpool.Pool, p repo.Place) {
	t.Helper()
	kinds, _ := json.Marshal(p.Kinds)
	traits, _ := json.Marshal(p.Traits)
	var activities, reviews any
	if p.Activities != nil {
		activities = string(p.Activities)
	}
	if p.Reviews != nil {
		reviews = string(p.Reviews)
	}
	// A point must state its precision (migration b3f19c7d2a04); a fixture
	// row with coordinates is a specific point, as every pre-feed row was.
	geo := p.GeoPrecision
	if geo == nil && p.Lat != nil {
		geo = new("rooftop")
	}
	_, err := pool.Exec(context.Background(), `INSERT INTO places(id,destination_id,name,category,kinds,address,lat,lng,price_min_vnd,price_max_vnd,open_hours,traits,activities,description,reviews,source,source_ref,license,geo_precision)
		VALUES($1,$2,$3,$4,$5::jsonb,$6,$7,$8,$9,$10,$11,$12::jsonb,$13::jsonb,$14,$15::jsonb,$16,$17,$18,$19)`,
		p.ID, p.DestinationID, p.Name, p.Category, string(kinds), p.Address, p.Lat, p.Lng, p.PriceMinVND, p.PriceMaxVND,
		p.OpenHours, string(traits), activities, p.Description, reviews, p.Source, p.SourceRef, p.License, geo)
	if err != nil {
		t.Fatalf("%s: %v", p.ID, err)
	}
}

// NapVang inserts the golden destinations, places and hand tombstones.
func NapVang(t *testing.T, pool *pgxpool.Pool, v nap.TapVang) {
	t.Helper()
	ctx := context.Background()
	for _, d := range v.DiemDen {
		if _, err := pool.Exec(ctx, `INSERT INTO destinations(id,name,province,lat,lng,bbox_south,bbox_west,bbox_north,bbox_east,sort_order) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
			d.ID, d.Ten, d.Tinh, d.Lat, d.Lng, d.Nam, d.Tay, d.Bac, d.Dong, d.Thu); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range v.Rows() {
		Chen(t, pool, p)
	}
	for _, b := range v.BiaTay {
		if err := rag.Tombstone(ctx, pool, b.ID, b.LyDo); err != nil {
			t.Fatal(err)
		}
	}
}

// TraLoiNhanTay is what a perfect enrichment model would answer for one
// batch: each golden place's hand labels, in the response schema, and
// «khong_ro» for a place whose allergens the golden marks unknown.
func TraLoiNhanTay(v nap.TapVang, batch []nap.HoSoQuan) string {
	byID := map[string]nap.QuanVang{}
	for _, q := range v.Quan {
		byID[q.ID] = q
	}
	var items []string
	for i, h := range batch {
		q := byID[h.ID]
		list := func(xs []string) string {
			raw, _ := json.Marshal(append([]string{}, xs...))
			return string(raw)
		}
		diUng := list(q.DiUng)
		if q.DiUngKhongRo {
			diUng = `["khong_ro"]`
		}
		items = append(items, fmt.Sprintf(`{"bi_danh":"p%d","di_ung":%s,"an_kieng":%s,"khi_chat":[],"mon_chinh":[],"chen_lenh":false,"tin_cay":"cao","ngu_canh_ho_so":"","ngu_canh_trai_nghiem":"","ngu_canh_mon_an":""}`,
			i+1, diUng, list(q.AnKieng)))
	}
	return `{"quan":[` + strings.Join(items, ",") + `]}`
}

// LamGiauNhanTay runs the real enrichment path (ChayLamGiau through a
// scripted model, the stored rows, the review verdicts) with the hand
// labels as the model's answers, and approves every row: the golden places
// get exactly their hand labels as attributes. It returns the model calls.
func LamGiauNhanTay(t *testing.T, pool *pgxpool.Pool, n nap.Nap, v nap.TapVang) int {
	t.Helper()
	ctx := context.Background()
	places, err := repo.Repository{Q: pool}.ListPlaces(ctx, repo.PlaceFilter{})
	if err != nil {
		t.Fatal(err)
	}
	var hs []nap.HoSoQuan
	for _, p := range places {
		if h, bo := nap.DungHoSo(p); !bo {
			hs = append(hs, h)
		}
	}
	sort.Slice(hs, func(i, j int) bool { return hs[i].ID < hs[j].ID })
	cfg := n.Cfg
	cfg.LamGiau.SongSong = 1 // the scripted model answers in order
	var kich []llm.Buoc
	for i := 0; i < len(hs); i += cfg.LamGiau.Lo {
		kich = append(kich, llm.Buoc{Text: TraLoiNhanTay(v, hs[i:min(i+cfg.LamGiau.Lo, len(hs))])})
	}
	stub := llm.NewStub(kich...)
	done, hong, rep := nap.ChayLamGiau(ctx, stub, cfg, len(kich), hs)
	if len(hong) > 0 || rep.Xong != len(hs) {
		t.Fatalf("enrichment: %+v, %d failed", rep, len(hong))
	}
	if err := nap.GhiLamGiau(ctx, pool, done); err != nil {
		t.Fatal(err)
	}
	DuyetHet(t, pool)
	// Approving marks every place dirty (its chunks carry the verdict); a
	// fresh build reads everything anyway.
	if _, err := pool.Exec(ctx, `DELETE FROM rag_dirty`); err != nil {
		t.Fatal(err)
	}
	return rep.SoGoi
}

// DuyetHet approves every stored enrichment, each by the version the review
// queue shows for it.
func DuyetHet(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	q, err := nap.HangDuyet(ctx, pool, true, 100000)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range q {
		if err := nap.Duyet(ctx, pool, m.PlaceID, m.Ban, nap.ReviewReviewed); err != nil {
			t.Fatal(err)
		}
	}
}

// Nap returns a pipeline over kho with the committed configuration and the
// stub dense encoder at its dimensionality.
func Nap(t testing.TB, kho nap.KhoVector) (nap.Nap, nap.StubDense) {
	t.Helper()
	cfg, err := nap.MacDinh()
	if err != nil {
		t.Fatal(err)
	}
	enc := nap.StubDense{N: cfg.Dense.Dims}
	return nap.Nap{Kho: kho, Dense: enc, Cfg: cfg}, enc
}

// State reads a version's state.
func State(t *testing.T, pool *pgxpool.Pool, id int64) string {
	t.Helper()
	p, err := nap.DocPhienBan(context.Background(), pool, id)
	if err != nil {
		t.Fatal(err)
	}
	return p.State
}

// DocIDs lists the documents a collection holds.
func DocIDs(t *testing.T, kho nap.KhoVector, ten string) map[string]bool {
	t.Helper()
	got, err := kho.LietKe(context.Background(), ten)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, k := range got {
		out[k.DocID] = true
	}
	return out
}

// ChiMuc runs one indexer pass under the periodic task's lock.
func ChiMuc(t *testing.T, pool *pgxpool.Pool, c nap.ChiMuc) nap.BaoCaoChiMuc {
	t.Helper()
	var b nap.BaoCaoChiMuc
	d := jobs.DinhKy{Ten: "rag.nap.chi_muc", Nhip: 1, Chay: func(ctx context.Context, tx pgx.Tx) error {
		var err error
		b, err = c.MotLuot(ctx, tx)
		return err
	}}
	ran, err := jobs.MotLuot(context.Background(), pool, d)
	if err != nil || !ran {
		t.Fatalf("indexer pass: ran=%v err=%v", ran, err)
	}
	return b
}

// BuildEvalPromote builds, evaluates and promotes one place version.
func BuildEvalPromote(t *testing.T, pool *pgxpool.Pool, n nap.Nap, enc nap.StubDense, v nap.TapVang) (nap.BaoCaoDung, nap.KetQuaCong) {
	t.Helper()
	ctx := context.Background()
	rep, err := n.Dung(ctx, pool, nap.CorpusQuan)
	if err != nil {
		t.Fatalf("build: %v (%+v)", err, rep)
	}
	if s := State(t, pool, rep.PhienBan); s != "built" {
		t.Fatalf("after build: %s", s)
	}
	k, err := n.DanhGia(ctx, pool, rep.PhienBan, enc, v)
	if err != nil {
		t.Fatal(err)
	}
	if !k.Dat {
		t.Fatalf("gate refused: %v %+v", k.LyDo, k.DoiSoat)
	}
	if _, err := n.Promote(ctx, pool, rep.PhienBan, v.Sha); err != nil {
		t.Fatal(err)
	}
	return rep, k
}

// VongDoi is the lifecycle scenario: build → reconcile → gate → promote by
// alias → change capture (trigger → rag_dirty + outbox lane) → indexer
// (dual-write, tombstone everywhere) → second version → rollback by alias →
// a removed place stays removed → the reconciler puts a moved alias back.
func VongDoi(t *testing.T, pool *pgxpool.Pool, kho nap.KhoVector) {
	ctx := context.Background()
	v := Vang(t)
	NapVang(t, pool, v)
	n, enc := Nap(t, kho)
	calls := LamGiauNhanTay(t, pool, n, v)
	t.Logf("enrichment: %d scripted model calls", calls)

	v1, k1 := BuildEvalPromote(t, pool, n, enc, v)
	t.Logf("v1: %+v; gate: probes %d, violations %d, golden %s", v1, k1.ThamDo, k1.ViPham, k1.Vang.Tong)
	if v1.ThieuLamGiau != 0 || v1.Bia != 1 || v1.Docs == 0 {
		t.Fatalf("v1 report: %+v", v1)
	}
	if got, _ := kho.MoTaAlias(ctx, nap.Alias(nap.CorpusQuan)); got != v1.Collection {
		t.Fatalf("alias → %q, want %s", got, v1.Collection)
	}
	if DocIDs(t, kho, v1.Collection)["dl-san-bowling-may-trang"] {
		t.Fatal("a hand-tombstoned place is in the index")
	}

	// Change capture: an edit, a delete, a takedown.
	const doi, xoa, go_ = "dl-tiem-banh-may-xanh", "sg-oc-len-xao-dua", "dl-lau-bo-via-he"
	if _, err := pool.Exec(ctx, `UPDATE places SET description='Tiệm bánh nhỏ, nay thêm bánh mì que.' WHERE id=$1`, doi); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM places WHERE id=$1`, xoa); err != nil {
		t.Fatal(err)
	}
	var dirty, lane int
	if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM rag_dirty), (SELECT count(*) FROM job_outbox WHERE queue='rag')`).Scan(&dirty, &lane); err != nil {
		t.Fatal(err)
	}
	if dirty != 2 || lane < 1 {
		t.Fatalf("change capture: %d dirty rows, %d lane messages", dirty, lane)
	}
	if err := rag.Tombstone(ctx, pool, go_, "takedown"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO rag_dirty(corpus,doc_id) VALUES('place',$1)`, go_); err != nil {
		t.Fatal(err)
	}
	// The edited place needs a new enrichment; the indexer asks the model
	// (one scripted call) within its ceiling.
	h, _ := nap.DungHoSo(placeByID(t, pool, doi))
	model := llm.NewStub(llm.Buoc{Text: TraLoiNhanTay(v, []nap.HoSoQuan{h})})
	cm := nap.ChiMuc{Nap: n, Model: model, TranGoi: 5}
	before, _ := kho.LietKe(ctx, v1.Collection)
	b := ChiMuc(t, pool, cm)
	if b.Lay != 3 || b.Ghi != 1 || b.Xoa != 2 || b.Hong != 0 || b.LamGiau != 1 || model.SoGoi() != 1 {
		t.Fatalf("indexer pass: %+v", b)
	}
	docs := DocIDs(t, kho, v1.Collection)
	if docs[xoa] || docs[go_] || !docs[doi] {
		t.Fatalf("after the pass: deleted %v, taken down %v, edited %v", docs[xoa], docs[go_], docs[doi])
	}
	after, _ := kho.LietKe(ctx, v1.Collection)
	if hashOf(before, doi) == hashOf(after, doi) {
		t.Fatal("the edited place's chunk kept its old content")
	}
	var left int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM rag_dirty`).Scan(&left)
	if left != 0 {
		t.Fatalf("%d dirty rows left", left)
	}
	var reason string
	if err := pool.QueryRow(ctx, `SELECT reason FROM rag_tombstones WHERE doc_id=$1`, xoa).Scan(&reason); err != nil || reason != "source_deleted" {
		t.Fatalf("a deleted place is not tombstoned: %q %v", reason, err)
	}
	if again := ChiMuc(t, pool, cm); again.Lay != 0 {
		t.Fatalf("a second pass found work: %+v", again)
	}

	// Second version, then a takedown after it, then rollback.
	v2, _ := BuildEvalPromote(t, pool, n, enc, v)
	if State(t, pool, v1.PhienBan) != "retired" || State(t, pool, v2.PhienBan) != "active" {
		t.Fatal("promotion did not retire the parent")
	}
	const go2 = "ha-quan-chay-hoa-cau"
	if !DocIDs(t, kho, v1.Collection)[go2] || !DocIDs(t, kho, v2.Collection)[go2] {
		t.Fatal("fixture: the place to take down is not in both versions")
	}
	if err := rag.Tombstone(ctx, pool, go2, "closed"); err != nil {
		t.Fatal(err)
	}
	_, _ = pool.Exec(ctx, `INSERT INTO rag_dirty(corpus,doc_id) VALUES('place',$1)`, go2)
	ChiMuc(t, pool, cm)
	from, to, err := n.Rollback(ctx, pool, nap.CorpusQuan)
	if err != nil || from != v2.PhienBan || to != v1.PhienBan {
		t.Fatalf("rollback %d→%d: %v", from, to, err)
	}
	if got, _ := kho.MoTaAlias(ctx, nap.Alias(nap.CorpusQuan)); got != v1.Collection {
		t.Fatalf("after rollback the alias → %q", got)
	}
	back := DocIDs(t, kho, nap.Alias(nap.CorpusQuan))
	for _, id := range []string{xoa, go_, go2, "dl-san-bowling-may-trang"} {
		if back[id] {
			t.Fatalf("%s came back with the rollback", id)
		}
	}

	// The reconciler: an alias moved by hand is put back to Postgres's truth.
	if err := kho.DatAlias(ctx, nap.Alias(nap.CorpusQuan), v2.Collection); err != nil {
		t.Fatal(err)
	}
	var rep nap.BaoCaoDoiChieu
	d := jobs.DinhKy{Ten: "rag.nap.doi_chieu", Nhip: 1, Chay: func(ctx context.Context, tx pgx.Tx) error {
		var err error
		rep, err = n.DoiChieu(ctx, tx, nil)
		return err
	}}
	if _, err := jobs.MotLuot(ctx, pool, d); err != nil {
		t.Fatal(err)
	}
	if got, _ := kho.MoTaAlias(ctx, nap.Alias(nap.CorpusQuan)); got != v1.Collection || rep.SuaAlias != 1 {
		t.Fatalf("reconciler: alias → %q, report %+v", got, rep)
	}
	st, err := n.DocTrangThai(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	if st[0].Active != v1.PhienBan || !st[0].AliasKhop || st[0].Dirty != 0 || st[0].DLQ != 0 {
		t.Fatalf("status: %+v", st[0])
	}
	raw, _ := json.Marshal(st)
	for _, word := range []string{"Tiệm", "bánh", "Lẩu", "dl-"} {
		if strings.Contains(string(raw), word) {
			t.Fatalf("status carries content or a place id: %s", raw)
		}
	}
}

func placeByID(t *testing.T, pool *pgxpool.Pool, id string) repo.Place {
	t.Helper()
	p, err := repo.Repository{Q: pool}.GetPlace(context.Background(), id)
	if err != nil || p == nil {
		t.Fatalf("%s: %v", id, err)
	}
	return *p
}

func hashOf(ks []nap.KhoaHang, doc string) string {
	var hs []string
	for _, k := range ks {
		if k.DocID == doc {
			hs = append(hs, k.ContentHash)
		}
	}
	sort.Strings(hs)
	return strings.Join(hs, ",")
}

// TombstoneQuaRollback is the short scenario: two versions, a takedown
// after both, rollback: the taken-down place is in neither.
func TombstoneQuaRollback(t *testing.T, pool *pgxpool.Pool, kho nap.KhoVector) {
	ctx := context.Background()
	v := Vang(t)
	NapVang(t, pool, v)
	n, enc := Nap(t, kho)
	LamGiauNhanTay(t, pool, n, v)
	v1, _ := BuildEvalPromote(t, pool, n, enc, v)
	v2, _ := BuildEvalPromote(t, pool, n, enc, v)
	const id = "dl-lau-nam-doi-thong"
	if err := rag.Tombstone(ctx, pool, id, "takedown"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO rag_dirty(corpus,doc_id) VALUES('place',$1)`, id); err != nil {
		t.Fatal(err)
	}
	ChiMuc(t, pool, nap.ChiMuc{Nap: n})
	if _, _, err := n.Rollback(ctx, pool, nap.CorpusQuan); err != nil {
		t.Fatal(err)
	}
	for _, c := range []string{v1.Collection, v2.Collection, nap.Alias(nap.CorpusQuan)} {
		if DocIDs(t, kho, c)[id] {
			t.Fatalf("%s holds the taken-down place", c)
		}
	}
}

// DoiSoatChanPromote: a version missing one chunk fails the gate at
// reconciliation and can never be promoted.
func DoiSoatChanPromote(t *testing.T, pool *pgxpool.Pool, kho nap.KhoVector) {
	ctx := context.Background()
	v := Vang(t)
	NapVang(t, pool, v)
	n, enc := Nap(t, kho)
	LamGiauNhanTay(t, pool, n, v)
	rep, err := n.Dung(ctx, pool, nap.CorpusQuan)
	if err != nil {
		t.Fatal(err)
	}
	ks, _ := kho.LietKe(ctx, rep.Collection)
	if err := kho.XoaID(ctx, rep.Collection, []string{ks[0].ChunkID}); err != nil {
		t.Fatal(err)
	}
	k, err := n.DanhGia(ctx, pool, rep.PhienBan, enc, v)
	if err != nil {
		t.Fatal(err)
	}
	if k.Dat || k.DoiSoat.Thieu != 1 || State(t, pool, rep.PhienBan) != "failed" {
		t.Fatalf("a version missing a chunk: dat=%v %+v state %s", k.Dat, k.DoiSoat, State(t, pool, rep.PhienBan))
	}
	if _, err := n.Promote(ctx, pool, rep.PhienBan, v.Sha); err == nil {
		t.Fatal("a failed version was promoted")
	}
}

// ThamDoViPhamChanCong (F5): a collection whose stored attributes disagree
// with the truth Postgres rebuilds -- a place with known allergens made to
// read allergen-free in the index -- fails the gate on its hard-filter
// probes, even though reconciliation (ids and content hashes) is clean.
func ThamDoViPhamChanCong(t *testing.T, pool *pgxpool.Pool, kho nap.KhoVector) {
	ctx := context.Background()
	v := Vang(t)
	NapVang(t, pool, v)
	n, enc := Nap(t, kho)
	LamGiauNhanTay(t, pool, n, v)
	rep, err := n.Dung(ctx, pool, nap.CorpusQuan)
	if err != nil {
		t.Fatal(err)
	}
	const id = "dl-banh-xeo-tom-nhay" // hand label: tom
	lie := nap.Hang{DiemDen: "d-da-lat", DiUngRo: true, DiUng: []string{}}
	if n, err := kho.CapNhatThuocTinh(ctx, rep.Collection, id, lie); err != nil || n == 0 {
		t.Fatalf("tampering the index: %d rows, %v", n, err)
	}
	k, err := n.DanhGia(ctx, pool, rep.PhienBan, enc, v)
	if err != nil {
		t.Fatal(err)
	}
	has := false
	for _, r := range k.LyDo {
		has = has || r == "vi_pham_loc"
	}
	if k.Dat || k.ViPham == 0 || !has || !k.DoiSoat.Dat || State(t, pool, rep.PhienBan) != "failed" {
		t.Fatalf("an index lying about allergens passed: dat=%v vi_pham=%d ly_do=%v doi_soat=%+v", k.Dat, k.ViPham, k.LyDo, k.DoiSoat)
	}
}

// ThuocTinhKhacCauHinh (F7): after a configuration change the serving
// collection was built with another configuration; when a place's reviewed
// enrichment gains an allergen, the indexer rewrites that place's
// attributes in the serving collection too, so an allergy filter excludes
// it there at once.
func ThuocTinhKhacCauHinh(t *testing.T, pool *pgxpool.Pool, kho nap.KhoVector) {
	ctx := context.Background()
	v := Vang(t)
	NapVang(t, pool, v)
	n, enc := Nap(t, kho)
	LamGiauNhanTay(t, pool, n, v)
	v1, _ := BuildEvalPromote(t, pool, n, enc, v)
	// The pipeline moved on: v1 now reads as built with another
	// configuration, so the indexer cannot re-embed into it.
	if _, err := pool.Exec(ctx, `UPDATE rag_vector_versions SET config_fingerprint='ffffffffffff' WHERE id=$1`, v1.PhienBan); err != nil {
		t.Fatal(err)
	}
	const id = "dl-lau-nam-doi-thong" // hand label: no allergen
	tom := nap.Loc{DiemDen: "d-da-lat", DiUng: []string{"tom"}}
	coTrong := func() bool {
		got, err := kho.LocHang(ctx, v1.Collection, tom)
		if err != nil {
			t.Fatal(err)
		}
		for _, k := range got {
			if k.DocID == id {
				return true
			}
		}
		return false
	}
	if !coTrong() {
		t.Fatal("fixture: the place is not admitted under a shrimp allergy before the change")
	}
	h, _ := nap.DungHoSo(placeByID(t, pool, id))
	lg := nap.LamGiau{PlaceID: id, NguonHash: h.NguonHash, Model: "stub", PromptVersion: nap.PromptVersion(),
		KetQua: nap.KetQuaLamGiau{DiUng: []string{"tom"}, DiUngRo: true, AnKiengRo: true, TinCay: nap.TinCayCao}, CanDuyet: true, Review: nap.ReviewAuto}
	if err := nap.GhiLamGiau(ctx, pool, []nap.LamGiau{lg}); err != nil {
		t.Fatal(err)
	}
	q, err := nap.HangDuyet(ctx, pool, true, 100000)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range q {
		if m.PlaceID == id {
			// The verdict marks the place dirty.
			if err := nap.Duyet(ctx, pool, id, m.Ban, nap.ReviewReviewed); err != nil {
				t.Fatal(err)
			}
		}
	}
	b := ChiMuc(t, pool, nap.ChiMuc{Nap: n})
	if b.KhacCauHinh == 0 || b.Hong != 0 {
		t.Fatalf("indexer pass: %+v", b)
	}
	if coTrong() {
		t.Fatal("the serving collection of another configuration still admits the place under a shrimp allergy")
	}
}

// UpsertLapLai: writing the same rows twice leaves one row per chunk
// (deterministic primary keys; with autoID each upsert would mint new keys).
func UpsertLapLai(t *testing.T, kho nap.KhoVector, ten string) {
	ctx := context.Background()
	v := Vang(t)
	n, _ := Nap(t, kho)
	var rows []nap.Hang
	for i, q := range v.Quan[:20] {
		h, _ := nap.DungHoSo(v.Hang(i, q))
		hs, err := nap.DoanQuan(ctx, h, q.NhanTay(), n.Cfg.Chunker[nap.CorpusQuan], nap.ChiaNghia{Nhung: n.Dense})
		if err != nil {
			t.Fatal(err)
		}
		rows = append(rows, hs...)
	}
	if _, err := n.Vector(ctx, nil, rows); err != nil {
		t.Fatal(err)
	}
	if err := kho.TaoCollection(ctx, ten, nap.LuocDoTu(n.Cfg, nap.CorpusQuan)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = kho.XoaCollection(context.Background(), ten) })
	for i := 0; i < 2; i++ {
		if err := nap.GhiHang(ctx, kho, ten, rows); err != nil {
			t.Fatal(err)
		}
	}
	d, err := nap.DoiSoatHang(ctx, kho, ten, rows)
	if err != nil || !d.Dat || d.Dem != len(rows) {
		t.Fatalf("two upserts of %d rows: %+v %v", len(rows), d, err)
	}
}

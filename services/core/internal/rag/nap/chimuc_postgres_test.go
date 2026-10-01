//go:build postgres

package nap_test

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"mobile/services/core/internal/naptest"
	"mobile/services/core/internal/rag"
	"mobile/services/core/internal/rag/nap"
	"mobile/services/core/internal/repo"
)

// doiChu gives places a new text and carries their enrichment over to it
// (the hand labels do not depend on the description): the embedded text
// changes, the enrichment stays current, so the place stays indexable.
func doiChu(t *testing.T, pool *pgxpool.Pool, ids []string, chu string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `UPDATE places SET description=$2 WHERE id = ANY($1)`, ids, chu); err != nil {
		t.Fatal(err)
	}
	ghimLamGiau(t, pool, ids)
}

// ghimLamGiau carries the places' enrichment over to their current profile
// (its source hash), as if it had been run again with the same answer.
func ghimLamGiau(t *testing.T, pool *pgxpool.Pool, ids []string) {
	t.Helper()
	ctx := context.Background()
	places, err := repo.Repository{Q: pool}.PlacesByID(ctx, ids)
	if err != nil {
		t.Fatal(err)
	}
	enr, err := nap.DocLamGiau(ctx, pool, ids)
	if err != nil {
		t.Fatal(err)
	}
	var ls []nap.LamGiau
	for _, p := range places {
		h, _ := nap.DungHoSo(p)
		lg := enr[p.ID]
		if lg == nil {
			t.Fatalf("fixture: %s has no enrichment", p.ID)
		}
		c := *lg
		c.NguonHash = h.NguonHash
		ls = append(ls, c)
	}
	if err := nap.GhiLamGiau(ctx, pool, ls); err != nil {
		t.Fatal(err)
	}
}

// demKho counts the writes the indexer makes, and can run a hook inside
// an upsert (a change that lands while a pass works).
type demKho struct {
	nap.KhoVector
	mu                sync.Mutex
	upsert, part, xoa int
	hook              func()
}

func (k *demKho) Upsert(ctx context.Context, ten string, rows []nap.Hang) error {
	k.mu.Lock()
	k.upsert += len(rows)
	h := k.hook
	k.hook = nil
	k.mu.Unlock()
	if h != nil {
		h()
	}
	return k.KhoVector.Upsert(ctx, ten, rows)
}

func (k *demKho) CapNhatThuocTinhLo(ctx context.Context, ten string, rows []nap.Hang) (int, error) {
	k.mu.Lock()
	k.part += len(rows)
	k.mu.Unlock()
	return k.KhoVector.CapNhatThuocTinhLo(ctx, ten, rows)
}

func (k *demKho) XoaID(ctx context.Context, ten string, ids []string) error {
	k.mu.Lock()
	k.xoa += len(ids)
	k.mu.Unlock()
	return k.KhoVector.XoaID(ctx, ten, ids)
}

func (k *demKho) dem() (int, int, int) {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.upsert, k.part, k.xoa
}

func (k *demKho) reset() {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.upsert, k.part, k.xoa = 0, 0, 0
}

// demDense counts the documents the online door is asked for.
type demDense struct {
	nap.StubDense
	mu   sync.Mutex
	docs int
}

func (d *demDense) NhungTaiLieu(ctx context.Context, docs []nap.TaiLieu) ([][]float32, error) {
	d.mu.Lock()
	d.docs += len(docs)
	d.mu.Unlock()
	return d.StubDense.NhungTaiLieu(ctx, docs)
}

func (d *demDense) goi() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.docs
}

// loCho is a batch door that runs a job until told it is done, then
// answers the stub's vectors.
type loCho struct {
	enc  nap.StubDense
	mu   sync.Mutex
	jobs map[string][]nap.LoVao
	xong bool
	gui  int
}

func (l *loCho) Model() string { return l.enc.Model() }
func (l *loCho) Dims() int     { return l.enc.Dims() }

func (l *loCho) GuiLo(_ context.Context, ten string, docs []nap.LoVao) (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.jobs == nil {
		l.jobs = map[string][]nap.LoVao{}
	}
	job := "batches/job" + string(rune('a'+len(l.jobs)))
	l.jobs[job] = docs
	l.gui += len(docs)
	return job, nil
}

func (l *loCho) XemLo(ctx context.Context, job string) (nap.KetQuaLo, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.xong {
		return nap.KetQuaLo{}, nil
	}
	vecs := map[string][]float32{}
	for _, d := range l.jobs[job] {
		v, _ := l.enc.NhungTaiLieu(ctx, []nap.TaiLieu{d.TaiLieu})
		vecs[d.Khoa] = v[0]
	}
	return nap.KetQuaLo{Xong: true, Vecs: vecs}, nil
}

// A mark reaches the indexer as one notification per transaction however
// many documents it marks; a write that changes nothing marks nothing; the
// indexer's own writes mark nothing; a tombstone and a review are marked
// first; a background row raised by a real change restarts its lag.
func TestDanhDauVaThongBao(t *testing.T) {
	pool := naptest.Pool(t)
	ctx := context.Background()
	v := naptest.Vang(t)
	naptest.NapVang(t, pool, v)
	if _, err := pool.Exec(ctx, `DELETE FROM rag_dirty`); err != nil {
		t.Fatal(err)
	}
	nghe, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer nghe.Release()
	if _, err := nghe.Exec(ctx, `LISTEN rag_dirty`); err != nil {
		t.Fatal(err)
	}
	viet, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer viet.Release()
	var pid uint32
	if err := viet.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
		t.Fatal(err)
	}
	// Notifications from other tests' schemas share the channel: count only
	// the writer's.
	demTB := func(d time.Duration) int {
		n := 0
		end := time.Now().Add(d)
		for {
			left := time.Until(end)
			if left <= 0 {
				return n
			}
			w, cancel := context.WithTimeout(ctx, left)
			got, err := nghe.Conn().WaitForNotification(w)
			cancel()
			if err != nil {
				return n
			}
			if got.PID == pid && got.Channel == "rag_dirty" && got.Payload == "place" {
				n++
			}
		}
	}
	ids := []string{v.Quan[0].ID, v.Quan[1].ID, v.Quan[2].ID}
	tx, err := viet.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := rag.Tombstone(ctx, tx, ids[0], "takedown"); err != nil {
		t.Fatal(err)
	}
	if err := rag.Tombstone(ctx, tx, ids[1], "closed"); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE places SET description='Một dòng mới.' WHERE id=$1`, ids[2]); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if n := demTB(500 * time.Millisecond); n != 1 {
		t.Fatalf("three marks in one transaction: %d notifications, want 1", n)
	}
	uu := map[string]int{}
	rows, _ := pool.Query(ctx, `SELECT doc_id, uu_tien FROM rag_dirty`)
	for rows.Next() {
		var id string
		var u int
		_ = rows.Scan(&id, &u)
		uu[id] = u
	}
	rows.Close()
	if len(uu) != 3 || uu[ids[0]] != 1 || uu[ids[1]] != 1 || uu[ids[2]] != 0 {
		t.Fatalf("marks: %v", uu)
	}

	// Nothing changed, nothing marked, nothing notified.
	if _, err := pool.Exec(ctx, `DELETE FROM rag_dirty`); err != nil {
		t.Fatal(err)
	}
	if _, err := viet.Exec(ctx, `UPDATE places SET name=name, description=description WHERE id=$1`, ids[2]); err != nil {
		t.Fatal(err)
	}
	var n int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM rag_dirty`).Scan(&n)
	if n != 0 || demTB(200*time.Millisecond) != 0 {
		t.Fatalf("an identical rewrite marked %d rows", n)
	}

	// The indexer's own writes (rag.chi_muc set) mark nothing.
	tx, _ = viet.Begin(ctx)
	_, _ = tx.Exec(ctx, `SELECT set_config('rag.chi_muc','on',true)`)
	if err := rag.Tombstone(ctx, tx, v.Quan[3].ID, "takedown"); err != nil {
		t.Fatal(err)
	}
	_ = tx.Commit(ctx)
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM rag_dirty`).Scan(&n)
	if n != 0 {
		t.Fatalf("the indexer's own tombstone marked %d rows", n)
	}

	// Lifting a tombstone marks, first.
	if _, err := rag.Untombstone(ctx, viet, ids[0]); err != nil {
		t.Fatal(err)
	}
	var u int
	if err := pool.QueryRow(ctx, `SELECT uu_tien FROM rag_dirty WHERE doc_id=$1`, ids[0]).Scan(&u); err != nil || u != 1 {
		t.Fatalf("a lifted tombstone: uu_tien %d, %v", u, err)
	}

	// A review marks first; a new enrichment row is a source change.
	if _, err := pool.Exec(ctx, `DELETE FROM rag_dirty`); err != nil {
		t.Fatal(err)
	}
	n2, _ := naptest.Nap(t, nap.NewKhoNho())
	naptest.LamGiauNhanTay(t, pool, n2, v) // writes, approves, then clears rag_dirty
	if _, err := pool.Exec(ctx, `UPDATE place_enrichments SET review='rejected' WHERE place_id=$1`, ids[2]); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT uu_tien FROM rag_dirty WHERE doc_id=$1`, ids[2]).Scan(&u); err != nil || u != 1 {
		t.Fatalf("a rejection: uu_tien %d, %v", u, err)
	}

	// A background re-check raised by a real change restarts its lag.
	if _, err := pool.Exec(ctx, `SELECT rag_danh_dau('place', ARRAY[$1::text], NULL, (-1)::smallint)`, ids[1]); err != nil {
		t.Fatal(err)
	}
	var first time.Time
	_ = pool.QueryRow(ctx, `SELECT noticed_at FROM rag_dirty WHERE doc_id=$1`, ids[1]).Scan(&first)
	time.Sleep(20 * time.Millisecond)
	if _, err := pool.Exec(ctx, `SELECT rag_danh_dau('place', ARRAY[$1::text], NULL, (-1)::smallint)`, ids[1]); err != nil {
		t.Fatal(err)
	}
	var again time.Time
	_ = pool.QueryRow(ctx, `SELECT noticed_at, uu_tien FROM rag_dirty WHERE doc_id=$1`, ids[1]).Scan(&again, &u)
	if !again.Equal(first) || u != -1 {
		t.Fatalf("a second background mark moved the row: %v %d", again.Sub(first), u)
	}
	if _, err := pool.Exec(ctx, `UPDATE places SET description='Khác nữa.' WHERE id=$1`, ids[1]); err != nil {
		t.Fatal(err)
	}
	_ = pool.QueryRow(ctx, `SELECT noticed_at, uu_tien FROM rag_dirty WHERE doc_id=$1`, ids[1]).Scan(&again, &u)
	if !again.After(first) || u != 0 {
		t.Fatalf("a real change on a background row: lag kept from %v, uu_tien %d", again.Sub(first), u)
	}
}

// The indexer writes only what differs: a touch that changes no indexed
// field writes nothing; a new price is a partial rewrite (no vector, no
// text); a new text with no vector to hand still gets its attributes at
// once, and waits for its vector, which the next pass with a budget pays
// for and writes whole.
func TestChiMucGhiDungCaiCan(t *testing.T) {
	pool := naptest.Pool(t)
	ctx := context.Background()
	v := naptest.Vang(t)
	naptest.NapVang(t, pool, v)
	kho := &demKho{KhoVector: nap.NewKhoNho()}
	n, enc := naptest.Nap(t, kho)
	naptest.LamGiauNhanTay(t, pool, n, v)
	v1, _ := naptest.BuildEvalPromote(t, pool, n, enc, v)
	dd := &demDense{StubDense: enc}
	n.Dense = dd
	const id = "dl-tiem-banh-may-xanh"
	if !naptest.DocIDs(t, kho, v1.Collection)[id] {
		t.Fatal("fixture: the place is not indexed")
	}
	coTrongNganSach := func(vnd int64) bool {
		got, err := kho.LocHang(ctx, v1.Collection, nap.Loc{NganSach: &vnd})
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
	hash := func() string {
		ks, _ := kho.KhoaTheoDoc(ctx, v1.Collection, []string{id})
		if len(ks) != 1 {
			t.Fatalf("%d rows for the place", len(ks))
		}
		return ks[0].ContentHash
	}
	h0 := hash()

	kho.reset()
	if _, err := pool.Exec(ctx, `UPDATE places SET updated_at=now() + interval '1 second' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	b := naptest.ChiMuc(t, pool, nap.ChiMuc{Nap: n})
	if up, part, _ := kho.dem(); b.KhongDoi != 1 || up != 0 || part != 0 {
		t.Fatalf("a touch: %+v, %d upserts, %d partial", b, up, part)
	}

	if _, err := pool.Exec(ctx, `UPDATE places SET price_min_vnd=123000, price_max_vnd=150000 WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	b = naptest.ChiMuc(t, pool, nap.ChiMuc{Nap: n})
	if up, part, _ := kho.dem(); b.MotPhan != 1 || up != 0 || part != 1 || dd.goi() != 0 {
		t.Fatalf("a new price: %+v, %d upserts, %d partial, %d online", b, up, part, dd.goi())
	}
	if coTrongNganSach(122000) || !coTrongNganSach(123000) || hash() != h0 {
		t.Fatal("the new price did not reach the filter, or the text moved")
	}

	// New text and a new price, no budget and no batch door: the price
	// goes now, the text waits.
	kho.reset()
	if _, err := pool.Exec(ctx, `UPDATE places SET price_min_vnd=99000, price_max_vnd=150000 WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	doiChu(t, pool, []string{id}, "Tiệm bánh nhỏ, nay bán cả bánh mì que.")
	het := nap.NewHanMuc(0)
	b = naptest.ChiMuc(t, pool, nap.ChiMuc{Nap: n, HanMuc: het})
	if up, part, _ := kho.dem(); b.ChoNhung != 1 || up != 0 || part != 1 || dd.goi() != 0 {
		t.Fatalf("new text over budget: %+v, %d upserts, %d partial, %d online", b, up, part, dd.goi())
	}
	if !coTrongNganSach(99000) || coTrongNganSach(98000) || hash() != h0 {
		t.Fatal("the price waited for the vector")
	}
	var choS float64
	var choNhung bool
	if err := pool.QueryRow(ctx, `SELECT EXTRACT(EPOCH FROM cho_den - clock_timestamp()), cho_nhung FROM rag_dirty WHERE doc_id=$1`, id).Scan(&choS, &choNhung); err != nil {
		t.Fatal(err)
	}
	if choS < 200 || choS > 301 || choNhung {
		t.Fatalf("deferred %.0f s, cho_nhung %v; want ~300 s for the budget", choS, choNhung)
	}
	if again := naptest.ChiMuc(t, pool, nap.ChiMuc{Nap: n, HanMuc: het}); again.Lay != 0 {
		t.Fatalf("a deferred row was taken again: %+v", again)
	}
	if _, err := pool.Exec(ctx, `UPDATE rag_dirty SET cho_den=NULL`); err != nil {
		t.Fatal(err)
	}
	b = naptest.ChiMuc(t, pool, nap.ChiMuc{Nap: n, HanMuc: nap.NewHanMuc(10)})
	if up, _, _ := kho.dem(); b.Ghi != 1 || b.NhungOnline != 1 || up != 1 || dd.goi() != 1 || hash() == h0 {
		t.Fatalf("with a budget: %+v, %d upserts, %d online", b, up, dd.goi())
	}
	var left int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM rag_dirty`).Scan(&left)
	if left != 0 {
		t.Fatalf("%d rows left dirty", left)
	}
}

// A bulk change waits for the batch door: no online call; a waiting place
// changed again still pays nothing online; one batch job carries every
// missing text; its end wakes them and the next pass writes them whole.
func TestChiMucChoCuaLo(t *testing.T) {
	pool := naptest.Pool(t)
	ctx := context.Background()
	v := naptest.Vang(t)
	naptest.NapVang(t, pool, v)
	kho := &demKho{KhoVector: nap.NewKhoNho()}
	n, enc := naptest.Nap(t, kho)
	// The batch door serves the configuration's model: here the stub's.
	n.Cfg.Dense.Model = enc.Model()
	naptest.LamGiauNhanTay(t, pool, n, v)
	v1, _ := naptest.BuildEvalPromote(t, pool, n, enc, v)
	dd := &demDense{StubDense: enc}
	n.Dense = dd
	var ids []string
	for id := range naptest.DocIDs(t, kho, v1.Collection) {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	ids = ids[:nap.NguongOnline+10]
	doiChu(t, pool, ids, "Mới mở thêm tầng hai.")
	lo := &loCho{enc: enc}
	cm := nap.ChiMuc{Nap: n, Lo: lo, HanMuc: nap.NewHanMuc(nap.TranOnlineGio), Pool: pool}
	b := naptest.ChiMuc(t, pool, cm)
	if b.ChoNhung != len(ids) || b.NhungOnline != 0 || dd.goi() != 0 {
		t.Fatalf("a bulk change: %+v, %d online", b, dd.goi())
	}
	// One of them changes again (a price): its attributes go, its vector
	// still waits for the door.
	if _, err := pool.Exec(ctx, `UPDATE places SET price_min_vnd=77000, price_max_vnd=97000 WHERE id=$1`, ids[0]); err != nil {
		t.Fatal(err)
	}
	b = naptest.ChiMuc(t, pool, cm)
	if b.Lay != 1 || b.ChoNhung != 1 || dd.goi() != 0 {
		t.Fatalf("a waiting place changed again: %+v, %d online", b, dd.goi())
	}
	rep, err := cm.LuotLo(ctx, pool)
	if err != nil || rep.Gui != len(ids) || rep.TrangThai != "dang_chay" || lo.gui != len(ids) {
		t.Fatalf("the batch turn: %+v %v (door got %d)", rep, err, lo.gui)
	}
	rep, err = cm.LuotLo(ctx, pool)
	if err != nil || rep.Gui != 0 || rep.TrangThai != "dang_chay" || lo.gui != len(ids) {
		t.Fatalf("a running job was submitted again: %+v %v", rep, err)
	}
	lo.mu.Lock()
	lo.xong = true
	lo.mu.Unlock()
	rep, err = cm.LuotLo(ctx, pool)
	if err != nil || rep.TrangThai != "xong" || rep.Ghi != len(ids) {
		t.Fatalf("the job's end: %+v %v", rep, err)
	}
	var cho int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM rag_dirty WHERE cho_den IS NOT NULL`).Scan(&cho)
	if cho != 0 {
		t.Fatalf("%d places still asleep after their vectors came", cho)
	}
	b = naptest.ChiMuc(t, pool, cm)
	if b.Ghi != len(ids) || b.NhungOnline != 0 || dd.goi() != 0 {
		t.Fatalf("after the job: %+v, %d online", b, dd.goi())
	}
	if rep, err := cm.LuotLo(ctx, pool); err != nil || rep.TrangThai != "khong_can" {
		t.Fatalf("a turn with nothing waiting: %+v %v", rep, err)
	}
}

// A change that lands while a pass works keeps its row dirty, its lag
// counted from no earlier than the pass's read; the next pass indexes it.
func TestChiMucThayDoiGiuaLuot(t *testing.T) {
	pool := naptest.Pool(t)
	ctx := context.Background()
	v := naptest.Vang(t)
	naptest.NapVang(t, pool, v)
	kho := &demKho{KhoVector: nap.NewKhoNho()}
	n, enc := naptest.Nap(t, kho)
	naptest.LamGiauNhanTay(t, pool, n, v)
	naptest.BuildEvalPromote(t, pool, n, enc, v)
	const id = "dl-tiem-banh-may-xanh"
	doiChu(t, pool, []string{id}, "Lần một.")
	var before time.Time
	_ = pool.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&before)
	kho.hook = func() { doiChu(t, pool, []string{id}, "Lần hai.") }
	if b := naptest.ChiMuc(t, pool, nap.ChiMuc{Nap: n}); b.Ghi != 1 {
		t.Fatalf("first pass: %+v", b)
	}
	var noticed time.Time
	if err := pool.QueryRow(ctx, `SELECT noticed_at FROM rag_dirty WHERE doc_id=$1`, id).Scan(&noticed); err != nil {
		t.Fatalf("the change made during the pass was dropped: %v", err)
	}
	if noticed.Before(before) {
		t.Fatalf("its lag counts from before the pass read (%v)", before.Sub(noticed))
	}
	if b := naptest.ChiMuc(t, pool, nap.ChiMuc{Nap: n}); b.Ghi != 1 || b.Lay != 1 {
		t.Fatalf("second pass: %+v", b)
	}
}

// A place a build's dedupe left out is recorded, and the indexer never
// writes it back into that version (deleting it if it is there).
func TestTrungKhongQuayLai(t *testing.T) {
	pool := naptest.Pool(t)
	ctx := context.Background()
	v := naptest.Vang(t)
	naptest.NapVang(t, pool, v)
	kho := &demKho{KhoVector: nap.NewKhoNho()}
	n, enc := naptest.Nap(t, kho)
	naptest.LamGiauNhanTay(t, pool, n, v)
	v1, _ := naptest.BuildEvalPromote(t, pool, n, enc, v)
	var rec int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM rag_trung WHERE phien_ban=$1`, v1.PhienBan).Scan(&rec)
	if rec != v1.Trung {
		t.Fatalf("the build left out %d duplicates and recorded %d", v1.Trung, rec)
	}
	const id, giu = "dl-tiem-banh-may-xanh", "dl-lau-nam-doi-thong"
	if _, err := pool.Exec(ctx, `INSERT INTO rag_trung(phien_ban, doc_id, giu) VALUES($1,$2,$3)`, v1.PhienBan, id, giu); err != nil {
		t.Fatal(err)
	}
	doiChu(t, pool, []string{id}, "Đổi để đánh dấu.")
	kho.reset()
	b := naptest.ChiMuc(t, pool, nap.ChiMuc{Nap: n})
	if up, part, xoa := kho.dem(); up != 0 || part != 0 || xoa != 1 || b.Hong != 0 {
		t.Fatalf("a recorded duplicate: %+v, %d upserts, %d partial, %d deleted", b, up, part, xoa)
	}
	if naptest.DocIDs(t, kho, v1.Collection)[id] {
		t.Fatal("the duplicate is in the version")
	}
	doiChu(t, pool, []string{id}, "Lại đổi.")
	naptest.ChiMuc(t, pool, nap.ChiMuc{Nap: n})
	if naptest.DocIDs(t, kho, v1.Collection)[id] {
		t.Fatal("a later change wrote the duplicate back")
	}
}

// failKho fails every write.
type failKho struct{ nap.KhoVector }

func (failKho) Upsert(context.Context, string, []nap.Hang) error { return errors.New("down") }
func (failKho) CapNhatThuocTinhLo(context.Context, string, []nap.Hang) (int, error) {
	return 0, errors.New("down")
}

// A place whose write fails is deferred (15 s, then 60 s), then after
// MaxThuLai passes leaves rag_dirty for the dead letters AND every live
// collection (fail closed: the failed write may have carried an allergen);
// retry brings it back whole.
func TestChiMucDLQ(t *testing.T) {
	pool := naptest.Pool(t)
	ctx := context.Background()
	v := naptest.Vang(t)
	naptest.NapVang(t, pool, v)
	kho := nap.NewKhoNho()
	n, enc := naptest.Nap(t, kho)
	naptest.LamGiauNhanTay(t, pool, n, v)
	v1, _ := naptest.BuildEvalPromote(t, pool, n, enc, v)
	id := "dl-tiem-banh-may-xanh"
	if _, err := pool.Exec(ctx, `UPDATE places SET price_min_vnd=88000, price_max_vnd=99000 WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	bad := nap.ChiMuc{Nap: nap.Nap{Kho: failKho{kho}, Dense: n.Dense, Cfg: n.Cfg}}
	for i := 1; i <= nap.MaxThuLai; i++ {
		b := naptest.ChiMuc(t, pool, bad)
		if b.Hong != 1 {
			t.Fatalf("pass %d: %+v", i, b)
		}
		if i == nap.MaxThuLai {
			break
		}
		var s float64
		if err := pool.QueryRow(ctx, `SELECT EXTRACT(EPOCH FROM cho_den - clock_timestamp()) FROM rag_dirty WHERE doc_id=$1`, id).Scan(&s); err != nil {
			t.Fatal(err)
		}
		if want := []float64{15, 60}[i-1]; s < want-5 || s > want+1 {
			t.Fatalf("after failure %d: deferred %.1f s, want %.0f", i, s, want)
		}
		if again := naptest.ChiMuc(t, pool, bad); again.Lay != 0 {
			t.Fatalf("a deferred place was taken again: %+v", again)
		}
		if _, err := pool.Exec(ctx, `UPDATE rag_dirty SET cho_den=NULL`); err != nil {
			t.Fatal(err)
		}
	}
	var dirty, so int
	_ = pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM rag_dirty), (SELECT so_lan FROM rag_ingest_dlq WHERE doc_id=$1)`, id).Scan(&dirty, &so)
	if dirty != 0 || so != nap.MaxThuLai {
		t.Fatalf("after %d failures: dirty %d, dead letter count %d", nap.MaxThuLai, dirty, so)
	}
	if naptest.DocIDs(t, kho, v1.Collection)[id] {
		t.Fatal("a dead-lettered place stays servable with its old attributes")
	}
	st, _ := n.DocTrangThai(ctx, pool)
	if st[0].DLQ != 1 {
		t.Fatalf("status DLQ %d", st[0].DLQ)
	}
	back, err := nap.ThuLaiDLQ(ctx, pool)
	if err != nil || back != 1 {
		t.Fatalf("retry: %d %v", back, err)
	}
	if b := naptest.ChiMuc(t, pool, nap.ChiMuc{Nap: n}); b.Ghi != 1 || b.Hong != 0 || b.NhungOnline != 0 {
		t.Fatalf("after retry: %+v", b)
	}
	if !naptest.DocIDs(t, kho, v1.Collection)[id] {
		t.Fatal("the retried place did not come back")
	}
}

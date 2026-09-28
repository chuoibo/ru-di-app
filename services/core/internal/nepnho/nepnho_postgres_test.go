//go:build postgres

package nepnho

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"mobile/services/core/internal/aiharness/trinho"
	"mobile/services/core/internal/auth"
	"mobile/services/core/internal/jobs"
	"mobile/services/core/internal/testdb"
)

var khoaQuenThu = []byte("0123456789abcdef0123456789abcdef")

func moiPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool := testdb.Pool(t)
	if err := jobs.Migrate(context.Background(), pool); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(context.Background(), pool); err != nil {
		t.Fatal(err)
	}
	return pool
}

func moiNguoi(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	id := uuidMoi()
	if _, err := pool.Exec(context.Background(), `INSERT INTO people(id, display_name) VALUES($1, 'Người thử nepnho')`, id); err != nil {
		t.Fatal(err)
	}
	return id
}

type bo struct {
	pool *pgxpool.Pool
	kho  *Kho
	gia  *khoGia
	now  time.Time
}

func moiBo(t *testing.T) *bo {
	t.Helper()
	pool := moiPool(t)
	gia := moiKhoGia()
	k, err := Moi(pool, gia, khoaQuenThu)
	if err != nil {
		t.Fatal(err)
	}
	b := &bo{pool: pool, kho: k, gia: gia, now: time.Now().UTC().Truncate(time.Second)}
	k.now = func() time.Time { return b.now }
	return b
}

func suThatMoi(noiDung string) trinho.SuThatMoi {
	return trinho.SuThatMoi{NoiDung: noiDung, Loai: trinho.ThichDanhMuc, Nguon: trinho.NoiRo, TuLuc: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)}
}

func (b *bo) so(t *testing.T, q string, args ...any) int {
	t.Helper()
	var n int
	if err := b.pool.QueryRow(context.Background(), q, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// Sentinel of this package on the PostgreSQL tier.
func TestNepnhoTierReachesPostgres(t *testing.T) {
	pool := moiPool(t)
	ok, err := SchemaCurrent(context.Background(), pool)
	if err != nil || !ok {
		t.Fatalf("schema current = %v %v", ok, err)
	}
}

func TestMigrateLaiVaChecksum(t *testing.T) {
	pool := moiPool(t)
	if err := Migrate(context.Background(), pool); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `UPDATE nep_schema_migrations SET digest='x' WHERE version=1`); err != nil {
		t.Fatal(err)
	}
	if ok, _ := SchemaCurrent(ctx, tx); ok {
		t.Fatal("a changed checksum still reads as current")
	}
	if _, err := tx.Exec(ctx, `UPDATE nep_schema_migrations SET digest=$1 WHERE version=1`, hexSum(schemaSQL)); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `ALTER TABLE people DISABLE TRIGGER nep_xoa_nguoi`); err != nil {
		t.Fatal(err)
	}
	if ok, _ := SchemaCurrent(ctx, tx); ok {
		t.Fatal("a disabled account-deletion trigger still reads as current")
	}
}

// Off (the default): nothing is recalled, nothing is written, and the
// sidecar hears nothing of either.
func TestTatThiKhongNhoKhongGhi(t *testing.T) {
	b := moiBo(t)
	ctx := context.Background()
	p := moiNguoi(t, b.pool)
	got, err := b.kho.Nho(ctx, p, "quán cà phê", 5)
	if err != nil || len(got) != 0 {
		t.Fatalf("Nho off = %v %v", got, err)
	}
	if _, err := b.kho.Ghi(ctx, p, suThatMoi("Thích quán cà phê yên tĩnh")); !errors.Is(err, ErrTat) {
		t.Fatalf("Ghi off = %v", err)
	}
	if b.gia.goi["tim"]+b.gia.goi["them"] != 0 {
		t.Fatalf("the sidecar was called while memory is off: %v", b.gia.goi)
	}
	if err := b.kho.Bat(ctx, p, CongBoBan+1); !errors.Is(err, ErrCongBoCu) {
		t.Fatalf("turned on with a stale disclosure: %v", err)
	}
	if c, _ := b.kho.DocCaiDat(ctx, p); c.Nho {
		t.Fatal("on after a refused consent")
	}
}

// On: a written fact has a receipt and comes back; a row the sidecar returns
// for another person (a leak in the store's filter) never does.
func TestBatGhiNhoVaChanRoChoNguoiKhac(t *testing.T) {
	b := moiBo(t)
	ctx := context.Background()
	a, c := moiNguoi(t, b.pool), moiNguoi(t, b.pool)
	for _, p := range []string{a, c} {
		if err := b.kho.Bat(ctx, p, CongBoBan); err != nil {
			t.Fatal(err)
		}
	}
	fa, err := b.kho.Ghi(ctx, a, suThatMoi("Thích quán cà phê yên tĩnh"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.kho.Ghi(ctx, c, suThatMoi("CANARY-c-khong-duoc-lo")); err != nil {
		t.Fatal(err)
	}
	if n := b.so(t, `SELECT count(*) FROM nep_su_that WHERE person_id=$1 AND id=$2 AND deleted_at IS NULL`, a, fa.ID); n != 1 {
		t.Fatal("no receipt for a written fact")
	}
	// A tool may ask for up to 20; the sidecar answers top_k 1..10.
	if _, err := b.kho.Nho(ctx, a, "cà phê", 20); err != nil || b.gia.timN != MaxNho {
		t.Fatalf("recall of 20 asked the sidecar for %d: %v", b.gia.timN, err)
	}
	b.gia.ro = c // the sidecar now also returns c's memories to a query of a
	got, err := b.kho.Nho(ctx, a, "cà phê", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != fa.ID || got[0].NoiDung != "Thích quán cà phê yên tĩnh" || got[0].Loai != trinho.ThichDanhMuc {
		t.Fatalf("recall = %+v", got)
	}
	for _, s := range got {
		if strings.Contains(s.NoiDung, "CANARY") {
			t.Fatal("another person's fact reached the recall")
		}
	}
	// Canary: without the receipt cross-check the leak shows.
	items, _ := b.gia.Tim(ctx, a, "x", 10)
	if len(items) != 2 {
		t.Fatalf("the fake did not leak (%d items): the canary proves nothing", len(items))
	}
	// Expired facts are held, listed, and not recalled.
	den := b.now.Add(-time.Hour)
	moi := suThatMoi("Đi xe buýt tới tháng trước")
	moi.TuLuc, moi.DenLuc = b.now.Add(-48*time.Hour), &den
	old, err := b.kho.Ghi(ctx, a, moi)
	if err != nil {
		t.Fatal(err)
	}
	b.gia.ro = ""
	got, _ = b.kho.Nho(ctx, a, "xe", 10)
	for _, s := range got {
		if s.ID == old.ID {
			t.Fatal("an expired fact was recalled")
		}
	}
	all, err := b.kho.LietKe(ctx, a)
	if err != nil || len(all.SuThat) != 2 || !all.Bat {
		t.Fatalf("LietKe %+v %v", all, err)
	}
}

// A toggle turned off while the sidecar writes wins: the memory is taken
// back out, Go's listing confirms it, and nothing is ledgered as live.
func TestTatGiuaLucGhiThiBoLai(t *testing.T) {
	b := moiBo(t)
	ctx := context.Background()
	p := moiNguoi(t, b.pool)
	if err := b.kho.Bat(ctx, p, CongBoBan); err != nil {
		t.Fatal(err)
	}
	moi := suThatMoi("Hay đi tối thứ sáu")
	if err := b.kho.truocGhi(ctx, p, b.kho.bamQuen(p, moi.NoiDung)); err != nil {
		t.Fatal(err)
	}
	ms, _, _ := b.gia.Them(ctx, p, moi.NoiDung)
	if _, _, err := b.kho.Tat(ctx, p); err != nil {
		t.Fatal(err)
	}
	s := trinho.SuThat{ID: ms[0].ID, NoiDung: ms[0].Text, Loai: moi.Loai, Nguon: moi.Nguon, TuLuc: moi.TuLuc}
	if err := b.kho.sauGhi(ctx, p, s); !errors.Is(err, ErrTat) {
		t.Fatalf("receipt written after the toggle went off: %v", err)
	}
	b.kho.boGhi(ctx, p, s, true)
	if left, _ := b.gia.LietKe(ctx, p); len(left) != 0 {
		t.Fatalf("%d memories left in the sidecar", len(left))
	}
	if n := b.so(t, `SELECT count(*) FROM nep_su_that WHERE person_id=$1 AND dang_xoa_at IS NULL`, p); n != 0 {
		t.Fatal("a live receipt after the toggle went off")
	}
}

// The extraction decides what a sentence yields: nothing (ErrKhongGhi, no
// receipt), two memories (two receipts), its own closed kind (it wins over
// the tool's). A memory whose extracted words were forgotten is taken back
// out. An id the ledger cannot hold is deleted at once and not ledgered.
func TestGhiTheoKetQuaTrich(t *testing.T) {
	b := moiBo(t)
	ctx := context.Background()
	p := moiNguoi(t, b.pool)
	_ = b.kho.Bat(ctx, p, CongBoBan)
	b.gia.tuChoi = true
	if _, err := b.kho.Ghi(ctx, p, suThatMoi("Bạn mình dị ứng tôm")); !errors.Is(err, ErrKhongGhi) {
		t.Fatalf("refused extraction = %v", err)
	}
	b.gia.tuChoi = false
	if n := b.so(t, `SELECT count(*) FROM nep_su_that WHERE person_id=$1`, p); n != 0 {
		t.Fatal("a receipt for nothing kept")
	}
	b.gia.tach, b.gia.loaiRut = true, string(trinho.PhuongTien)
	f, err := b.kho.Ghi(ctx, p, suThatMoi("Mình hay đi xe máy"))
	b.gia.tach, b.gia.loaiRut = false, ""
	if err != nil || f.Loai != trinho.PhuongTien {
		t.Fatalf("split extraction = %+v %v", f, err)
	}
	if n := b.so(t, `SELECT count(*) FROM nep_su_that WHERE person_id=$1 AND loai='phuong_tien' AND dang_xoa_at IS NULL`, p); n != 2 {
		t.Fatalf("%d receipts for two memories", n)
	}
	// Forget the second memory, then have the extraction produce its exact
	// words again from another sentence: taken back out.
	all, _ := b.kho.LietKe(ctx, p)
	var second trinho.SuThat
	for _, s := range all.SuThat {
		if strings.HasSuffix(s.NoiDung, "(2)") {
			second = s
		}
	}
	if n, err := b.kho.Quen(ctx, p, trinho.QuenGi{ID: second.ID}); n != 1 || err != nil {
		t.Fatalf("Quen = %d %v", n, err)
	}
	if _, err := b.kho.Ghi(ctx, p, suThatMoi(strings.TrimSuffix(second.NoiDung, " (2)")+" (2)")); !errors.Is(err, ErrDaQuen) {
		t.Fatalf("tombstoned words written again: %v", err)
	}
	b.gia.idSai = true
	if _, err := b.kho.Ghi(ctx, p, suThatMoi("Thích đi sớm")); !errors.Is(err, ErrDichVuSai) {
		t.Fatalf("non-UUID id = %v", err)
	}
	b.gia.idSai = false
	for _, m := range mustList(t, b, p) {
		if m.ID == "mem-khong-phai-uuid" {
			t.Fatal("a memory the ledger cannot hold stayed in the sidecar")
		}
	}
}

func mustList(t *testing.T, b *bo, p string) []MucNho {
	t.Helper()
	ms, err := b.gia.LietKe(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	return ms
}

// Forget one fact: the saga deletes in the sidecar, reads remaining 0,
// lists the person's memories itself, and only then writes the receipt; the
// words are tombstoned and cannot be written again.
func TestQuenMotSagaDayDu(t *testing.T) {
	b := moiBo(t)
	ctx := context.Background()
	p := moiNguoi(t, b.pool)
	_ = b.kho.Bat(ctx, p, CongBoBan)
	f, err := b.kho.Ghi(ctx, p, suThatMoi("Né quán karaoke ồn"))
	if err != nil {
		t.Fatal(err)
	}
	keep, _ := b.kho.Ghi(ctx, p, suThatMoi("Thích đi bộ"))
	before := b.gia.goi["lietke"]
	n, err := b.kho.Quen(ctx, p, trinho.QuenGi{ID: f.ID})
	if err != nil || n != 1 {
		t.Fatalf("Quen = %d %v", n, err)
	}
	if b.gia.goi["xoa"] != 1 || b.gia.goi["lietke"] < before+2 {
		t.Fatalf("calls %v: the saga must delete once and list after the delete", b.gia.goi)
	}
	left := mustList(t, b, p)
	if len(left) != 1 || left[0].ID != keep.ID {
		t.Fatalf("sidecar holds %+v", left)
	}
	var buoc string
	var soDaXoa int
	if err := b.pool.QueryRow(ctx, `SELECT buoc, so_da_xoa FROM nep_xoa WHERE person_id=$1 AND pham_vi='mot'`, p).Scan(&buoc, &soDaXoa); err != nil {
		t.Fatal(err)
	}
	if buoc != "xong" || soDaXoa != 1 {
		t.Fatalf("receipt buoc=%s so=%d", buoc, soDaXoa)
	}
	if n := b.so(t, `SELECT count(*) FROM nep_su_that WHERE id=$1 AND deleted_at IS NOT NULL`, f.ID); n != 1 {
		t.Fatal("no deletion receipt on the fact")
	}
	if _, err := b.kho.Ghi(ctx, p, suThatMoi("Né quán karaoke ồn")); !errors.Is(err, ErrDaQuen) {
		t.Fatalf("a forgotten fact was written again: %v", err)
	}
	all, _ := b.kho.LietKe(ctx, p)
	if len(all.SuThat) != 1 || all.SuThat[0].ID != keep.ID || len(all.DangXoa) != 0 {
		t.Fatalf("after forget: %+v", all)
	}
}

// A sidecar that answers remaining 0 but still holds the row is caught by
// Go's own listing: no receipt, the fact stays hidden and listed as being
// deleted, and the next attempt is enqueued on the memory lane.
func TestQuenSidecarNoiDoiThiKhongBienNhan(t *testing.T) {
	b := moiBo(t)
	ctx := context.Background()
	p := moiNguoi(t, b.pool)
	_ = b.kho.Bat(ctx, p, CongBoBan)
	f, _ := b.kho.Ghi(ctx, p, suThatMoi("Hay đi quận 1"))
	b.gia.noiDoi = true
	if n, err := b.kho.Quen(ctx, p, trinho.QuenGi{ID: f.ID}); !errors.Is(err, ErrDangXoa) || n != 0 {
		t.Fatalf("Quen with a lying sidecar = %d %v", n, err)
	}
	if n := b.so(t, `SELECT count(*) FROM nep_su_that WHERE id=$1 AND deleted_at IS NOT NULL`, f.ID); n != 0 {
		t.Fatal("a receipt was written while the row is still held")
	}
	var loi string
	var lan int
	var viec string
	_ = b.pool.QueryRow(ctx, `SELECT id::text, loi, lan_thu FROM nep_xoa WHERE person_id=$1`, p).Scan(&viec, &loi, &lan)
	if loi != loiConHang || lan != 1 {
		t.Fatalf("loi=%s lan=%d", loi, lan)
	}
	if n := b.so(t, `SELECT count(*) FROM job_outbox WHERE queue='memory' AND ref_id=$1 AND enqueue_seq IN (0,1)`, viec); n != 2 {
		t.Fatalf("%d outbox rows for attempts 0 and 1 on the memory lane", n)
	}
	// The next attempt, as the memory lane's consumer runs it.
	b.gia.noiDoi = false
	if err := b.kho.XuLyTin(ctx, jobs.Message{V: 1, Ref: viec, Seq: 1}); err != nil {
		t.Fatal(err)
	}
	if n := b.so(t, `SELECT count(*) FROM nep_su_that WHERE id=$1 AND deleted_at IS NOT NULL`, f.ID); n != 1 {
		t.Fatal("the memory lane did not finish the deletion")
	}
	if err := b.kho.XuLyTin(ctx, jobs.Message{V: 1, Ref: viec, Seq: 1}); err != nil {
		t.Fatalf("a redelivered message of a closed deletion: %v", err)
	}
	if err := b.kho.XuLyTin(ctx, jobs.Message{V: 1, Ref: "not-a-uuid"}); !errors.Is(err, jobs.ErrMalformed) {
		t.Fatalf("malformed ref: %v", err)
	}
}

// A count above zero is not a receipt: the fact stays hidden, is listed as
// being deleted, and the pass retries with backoff until the count is zero.
func TestQuenKhiConHangThiThuLai(t *testing.T) {
	b := moiBo(t)
	ctx := context.Background()
	p := moiNguoi(t, b.pool)
	_ = b.kho.Bat(ctx, p, CongBoBan)
	f, _ := b.kho.Ghi(ctx, p, suThatMoi("Hay đi quận 3"))
	b.gia.conSot = 1
	n, err := b.kho.Quen(ctx, p, trinho.QuenGi{ID: f.ID})
	if !errors.Is(err, ErrDangXoa) || n != 0 {
		t.Fatalf("Quen with rows left = %d %v", n, err)
	}
	if got, _ := b.kho.Nho(ctx, p, "quận", 5); len(got) != 0 {
		t.Fatal("a fact being deleted was recalled")
	}
	all, _ := b.kho.LietKe(ctx, p)
	if len(all.SuThat) != 0 || len(all.DangXoa) != 1 || all.DangXoa[0].ID != f.ID {
		t.Fatalf("LietKe during deletion: %+v", all)
	}
	var loi string
	var lan int
	var chay time.Time
	_ = b.pool.QueryRow(ctx, `SELECT loi, lan_thu, chay_luc FROM nep_xoa WHERE person_id=$1`, p).Scan(&loi, &lan, &chay)
	if loi != loiConHang || lan != 1 || !chay.After(b.now) {
		t.Fatalf("loi=%s lan=%d chay=%v", loi, lan, chay)
	}
	if n := b.so(t, `SELECT count(*) FROM nep_su_that WHERE id=$1 AND deleted_at IS NOT NULL`, f.ID); n != 0 {
		t.Fatal("a receipt was written with rows left")
	}
	// Not due yet: the pass leaves it.
	tx, _ := b.pool.Begin(ctx)
	if ran, err := b.kho.LuotXoa(ctx, tx); err != nil || ran != 0 {
		t.Fatalf("pass before due = %d %v", ran, err)
	}
	_ = tx.Commit(ctx)
	b.now = chay.Add(time.Second)
	tx, _ = b.pool.Begin(ctx)
	if _, err := b.kho.LuotXoa(ctx, tx); err != nil {
		t.Fatal(err)
	}
	_ = tx.Commit(ctx)
	if n := b.so(t, `SELECT count(*) FROM nep_su_that WHERE id=$1 AND deleted_at IS NOT NULL`, f.ID); n != 1 {
		t.Fatal("the retry did not finish the deletion")
	}
}

// Forget by description: the nearest live fact of the person, by the
// sidecar's similarity; nothing held: 0.
func TestQuenTheoMoTa(t *testing.T) {
	b := moiBo(t)
	ctx := context.Background()
	p := moiNguoi(t, b.pool)
	if n, err := b.kho.Quen(ctx, p, trinho.QuenGi{MoTa: "chuyện karaoke"}); n != 0 || err != nil {
		t.Fatalf("nothing held: %d %v", n, err)
	}
	_ = b.kho.Bat(ctx, p, CongBoBan)
	_, _ = b.kho.Ghi(ctx, p, suThatMoi("Thích quán nướng"))
	f, _ := b.kho.Ghi(ctx, p, suThatMoi("Né karaoke"))
	if n, err := b.kho.Quen(ctx, p, trinho.QuenGi{MoTa: "chuyện karaoke"}); n != 1 || err != nil {
		t.Fatalf("Quen MoTa = %d %v", n, err)
	}
	if n := b.so(t, `SELECT count(*) FROM nep_su_that WHERE id=$1 AND deleted_at IS NOT NULL`, f.ID); n != 1 {
		t.Fatal("the nearest fact (the fake's newest) was not the one deleted")
	}
	if _, err := b.kho.Quen(ctx, p, trinho.QuenGi{ID: "x", MoTa: "y"}); !errors.Is(err, trinho.ErrQuenMoHo) {
		t.Fatal("both fields accepted")
	}
}

// Turning memory off forgets everything: events, tombstones, every memory
// in the sidecar, and the short-term buffers; the saga asks again while the
// sidecar reports rows left.
func TestTatXoaHet(t *testing.T) {
	b := moiBo(t)
	ctx := context.Background()
	p := moiNguoi(t, b.pool)
	stm := &nganHanGia{}
	b.kho.VoiNganHan(stm)
	_ = b.kho.Bat(ctx, p, CongBoBan)
	for _, s := range []string{"Một", "Hai", "Ba"} {
		if _, err := b.kho.Ghi(ctx, p, suThatMoi(s)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := b.kho.GhiSuKien(ctx, p, []SuKien{{Loai: trinho.TaoKeo, Luc: b.now}}); err != nil {
		t.Fatal(err)
	}
	b.gia.conSot = 1 // the first delete_all reports one left
	_, xong, err := b.kho.Tat(ctx, p)
	if err != nil || xong {
		t.Fatalf("Tat with rows left = %v %v", xong, err)
	}
	if stm.nguoi != "" {
		t.Fatal("short-term purged before the long-term count was zero")
	}
	for _, q := range []string{`SELECT count(*) FROM nep_su_kien WHERE person_id=$1`, `SELECT count(*) FROM nep_quen WHERE person_id=$1`,
		`SELECT count(*) FROM nep_su_that WHERE person_id=$1 AND dang_xoa_at IS NULL`} {
		if n := b.so(t, q, p); n != 0 {
			t.Fatalf("%s = %d", q, n)
		}
	}
	var chay time.Time
	_ = b.pool.QueryRow(ctx, `SELECT chay_luc FROM nep_xoa WHERE person_id=$1 AND pham_vi='tat_ca'`, p).Scan(&chay)
	b.now = chay.Add(time.Second)
	tx, _ := b.pool.Begin(ctx)
	if _, err := b.kho.LuotXoa(ctx, tx); err != nil {
		t.Fatal(err)
	}
	_ = tx.Commit(ctx)
	if b.gia.goi["xoahet"] != 2 {
		t.Fatalf("delete_all called %d times, want 2 (until remaining 0)", b.gia.goi["xoahet"])
	}
	if left := mustList(t, b, p); len(left) != 0 || stm.nguoi != p {
		t.Fatalf("sidecar holds %d, short-term purge of %q", len(left), stm.nguoi)
	}
	var so int
	var buoc string
	_ = b.pool.QueryRow(ctx, `SELECT so_da_xoa, buoc FROM nep_xoa WHERE person_id=$1 AND pham_vi='tat_ca'`, p).Scan(&so, &buoc)
	if so != 3 || buoc != "xong" {
		t.Fatalf("receipt counts %d (%s), want 3 xong", so, buoc)
	}
	if n := b.so(t, `SELECT count(*) FROM nep_su_that WHERE person_id=$1 AND deleted_at IS NULL`, p); n != 0 {
		t.Fatal("receipts without deleted_at after forget-all")
	}
	if c, _ := b.kho.DocCaiDat(ctx, p); c.Nho {
		t.Fatal("still on")
	}
}

type nganHanGia struct{ nguoi string }

func (n *nganHanGia) XoaNguoi(_ context.Context, nguoi string) (int, error) {
	n.nguoi = nguoi
	return 1, nil
}

// Account deletion, by whichever backend sets people.deleted_at: the trigger
// drops what Postgres holds in the same transaction and queues the purge on
// the memory lane; attempts retry purge_user while the sidecar is down,
// until zero rows remain, and then no nep_* row names the person.
func TestXoaTaiKhoanQuaTrigger(t *testing.T) {
	b := moiBo(t)
	ctx := context.Background()
	p, other := moiNguoi(t, b.pool), moiNguoi(t, b.pool)
	for _, x := range []string{p, other} {
		_ = b.kho.Bat(ctx, x, CongBoBan)
		if _, err := b.kho.Ghi(ctx, x, suThatMoi("Thích phở")); err != nil {
			t.Fatal(err)
		}
		if _, err := b.kho.GhiSuKien(ctx, x, []SuKien{{Loai: trinho.LocDanhMuc, Luc: b.now, DanhMuc: "pho"}}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := b.pool.Exec(ctx, `UPDATE people SET deleted_at=now() WHERE id=$1`, p); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{`SELECT count(*) FROM nep_cai_dat WHERE person_id=$1`, `SELECT count(*) FROM nep_su_kien WHERE person_id=$1`,
		`SELECT count(*) FROM nep_su_that WHERE person_id=$1 AND dang_xoa_at IS NULL`} {
		if n := b.so(t, q, p); n != 0 {
			t.Fatalf("after the trigger %s = %d", q, n)
		}
	}
	var viec string
	if err := b.pool.QueryRow(ctx, `SELECT id::text FROM nep_xoa WHERE person_id=$1 AND pham_vi='tai_khoan' AND buoc='cho'`, p).Scan(&viec); err != nil {
		t.Fatalf("no account deletion queued: %v", err)
	}
	if n := b.so(t, `SELECT count(*) FROM job_outbox WHERE queue='memory' AND ref_id=$1 AND enqueue_seq=0`, viec); n != 1 {
		t.Fatal("the account deletion is not on the memory lane")
	}
	b.gia.vang = true
	if err := b.kho.XuLyTin(ctx, jobs.Message{V: 1, Ref: viec}); err != nil {
		t.Fatal(err)
	}
	var loi string
	var chay time.Time
	_ = b.pool.QueryRow(ctx, `SELECT loi, chay_luc FROM nep_xoa WHERE id=$1`, viec).Scan(&loi, &chay)
	if loi != loiDichVuVang {
		t.Fatalf("sidecar down: loi %q", loi)
	}
	if n := b.so(t, `SELECT count(*) FROM job_outbox WHERE queue='memory' AND ref_id=$1 AND enqueue_seq=1 AND available_at=$2`, viec, chay); n != 1 {
		t.Fatal("the retry is not enqueued at its backoff")
	}
	b.gia.vang = false
	b.gia.conSot = 1 // purge_user leaves one row once: another retry
	if err := b.kho.XuLyTin(ctx, jobs.Message{V: 1, Ref: viec, Seq: 1}); err != nil {
		t.Fatal(err)
	}
	if n := b.so(t, `SELECT count(*) FROM nep_xoa WHERE id=$1 AND buoc='cho' AND lan_thu=2 AND loi='con_hang'`, viec); n != 1 {
		t.Fatal("rows left after purge_user did not keep the deletion open")
	}
	if err := b.kho.XuLyTin(ctx, jobs.Message{V: 1, Ref: viec, Seq: 2}); err != nil {
		t.Fatal(err)
	}
	if b.gia.goi["xoasach"] != 3 {
		t.Fatalf("purge_user called %d times, want 3", b.gia.goi["xoasach"])
	}
	if left := mustList(t, b, p); len(left) != 0 {
		t.Fatal("rows left in the sidecar after the account deletion")
	}
	for _, q := range []string{`SELECT count(*) FROM nep_su_that WHERE person_id=$1`, `SELECT count(*) FROM nep_xoa WHERE person_id=$1`} {
		if n := b.so(t, q, p); n != 0 {
			t.Fatalf("%s = %d after the account deletion", q, n)
		}
	}
	if n := b.so(t, `SELECT count(*) FROM nep_xoa WHERE id=$1 AND buoc='xong' AND so_da_xoa=1 AND person_id IS NULL AND nguoi_bam=$2`, viec, b.kho.bamNguoi(p)); n != 1 {
		t.Fatal("no keyed receipt for the account deletion")
	}
	// The other person is untouched.
	if left := mustList(t, b, other); len(left) != 1 {
		t.Fatal("another person's memory went with the account")
	}
	if n := b.so(t, `SELECT count(*) FROM nep_su_kien WHERE person_id=$1`, other); n != 1 {
		t.Fatal("another person's events went with the account")
	}
}

// Events: off writes nothing, on writes the batch with times clamped, and a
// second batch within ten seconds is refused.
func TestSuKien(t *testing.T) {
	b := moiBo(t)
	ctx := context.Background()
	p := moiNguoi(t, b.pool)
	ev := []SuKien{{Loai: trinho.MoDiaDiem, Luc: b.now.Add(-30 * 24 * time.Hour), DiaDiemID: "p1"}, {Loai: trinho.ChonThoiLuong, Luc: b.now.Add(time.Hour), ThoiLuongPhut: 120}}
	if ghi, err := b.kho.GhiSuKien(ctx, p, ev); ghi || err != nil {
		t.Fatalf("off: %v %v", ghi, err)
	}
	_ = b.kho.Bat(ctx, p, CongBoBan)
	if ghi, err := b.kho.GhiSuKien(ctx, p, ev); !ghi || err != nil {
		t.Fatalf("on: %v %v", ghi, err)
	}
	if n := b.so(t, `SELECT count(*) FROM nep_su_kien WHERE person_id=$1 AND luc >= $2 AND luc <= $3`, p, b.now.Add(-7*24*time.Hour), b.now.Add(5*time.Minute)); n != 2 {
		t.Fatal("times not clamped")
	}
	if _, err := b.kho.GhiSuKien(ctx, p, ev); !errors.Is(err, ErrQuaNhanh) {
		t.Fatalf("second batch: %v", err)
	}
	all, _ := b.kho.LietKe(ctx, p)
	if all.SuKien[trinho.MoDiaDiem] != 1 || all.SuKien[trinho.ChonThoiLuong] != 1 {
		t.Fatalf("LietKe events %v", all.SuKien)
	}
	// The table refuses what Kiem refuses, on its own.
	if _, err := b.pool.Exec(ctx, `INSERT INTO nep_su_kien(person_id, loai, luc, danh_muc) VALUES($1,'tao_keo',now(),'cafe')`, p); err == nil {
		t.Fatal("the CHECK let a kind carry another kind's reference")
	}
	// Turned off after being on: refused again, nothing written.
	if _, _, err := b.kho.Tat(ctx, p); err != nil {
		t.Fatal(err)
	}
	b.now = b.now.Add(time.Minute)
	if ghi, err := b.kho.GhiSuKien(ctx, p, ev); ghi || err != nil {
		t.Fatalf("off after on: %v %v", ghi, err)
	}
	if n := b.so(t, `SELECT count(*) FROM nep_su_kien WHERE person_id=$1`, p); n != 0 {
		t.Fatalf("%d events written after the toggle went off", n)
	}
	_ = b.kho.Bat(ctx, p, CongBoBan)
	b.now = b.now.Add(time.Minute)
	if ghi, err := b.kho.GhiSuKien(ctx, p, ev); !ghi || err != nil {
		t.Fatalf("on again: %v %v", ghi, err)
	}
	b.now = b.now.Add(31 * 24 * time.Hour)
	tx, _ := b.pool.Begin(ctx)
	if err := b.kho.DinhKyDon().Chay(ctx, tx); err != nil {
		t.Fatal(err)
	}
	_ = tx.Commit(ctx)
	if n := b.so(t, `SELECT count(*) FROM nep_su_kien WHERE person_id=$1`, p); n != 0 {
		t.Fatal("events outlived their window")
	}
}

// LietKe tells everything held, including an orphan the sidecar holds
// without a receipt.
func TestLietKeTrungThuc(t *testing.T) {
	b := moiBo(t)
	ctx := context.Background()
	p := moiNguoi(t, b.pool)
	_ = b.kho.Bat(ctx, p, CongBoBan)
	_, _ = b.kho.Ghi(ctx, p, suThatMoi("Một"))
	orphan, _, _ := b.gia.Them(ctx, p, "Mồ côi")
	all, err := b.kho.LietKe(ctx, p)
	if err != nil || len(all.SuThat) != 1 || len(all.KhongSo) != 1 || all.KhongSo[0].ID != orphan[0].ID {
		t.Fatalf("%+v %v", all, err)
	}
	if got, _ := b.kho.Nho(ctx, p, "x", 5); len(got) != 1 {
		t.Fatal("an orphan was recalled")
	}
	_, xong, _ := b.kho.QuenHet(ctx, p)
	if !xong {
		t.Fatal("forget-all did not finish")
	}
	all, _ = b.kho.LietKe(ctx, p)
	if len(all.SuThat)+len(all.KhongSo)+len(all.DangXoa) != 0 {
		t.Fatalf("after forget-all: %+v", all)
	}
	// Without the service, receipts cannot be listed truthfully.
	noSvc, _ := Moi(b.pool, nil, khoaQuenThu)
	if _, err := noSvc.LietKe(ctx, p); err != nil {
		t.Fatalf("no receipts left, no service: %v", err)
	}
}

// A host with no memory service closes an account deletion of a person who
// never had a fact (nothing could be held), and waits for anyone else.
func TestKhongDichVuVaXoaTaiKhoan(t *testing.T) {
	pool := moiPool(t)
	ctx := context.Background()
	k, _ := Moi(pool, nil, khoaQuenThu)
	p := moiNguoi(t, pool)
	if _, err := pool.Exec(ctx, `UPDATE people SET deleted_at=now() WHERE id=$1`, p); err != nil {
		t.Fatal(err)
	}
	tx, _ := pool.Begin(ctx)
	if _, err := k.LuotXoa(ctx, tx); err != nil {
		t.Fatal(err)
	}
	_ = tx.Commit(ctx)
	var buoc string
	var so int
	if err := pool.QueryRow(ctx, `SELECT buoc, so_da_xoa FROM nep_xoa WHERE nguoi_bam=$1`, k.bamNguoi(p)).Scan(&buoc, &so); err != nil || buoc != "xong" || so != 0 {
		t.Fatalf("buoc=%s so=%d %v", buoc, so, err)
	}
}

// The routes end to end: a real session, the person from the bearer only.
func TestRouteQuaPhienThat(t *testing.T) {
	b := moiBo(t)
	ctx := context.Background()
	p := moiNguoi(t, b.pool)
	raw := make([]byte, 24)
	_, _ = rand.Read(raw)
	token := hex.EncodeToString(raw)
	if _, err := b.pool.Exec(ctx, `INSERT INTO account_sessions(id, person_id, token_digest, issued_via, expires_at) VALUES($1,$2,$3,'genesis',now()+interval '1 hour')`,
		uuidMoi(), p, auth.TokenDigest(token)); err != nil {
		t.Fatal(err)
	}
	b.kho.now = time.Now
	h := NewHandler(b.pool, b.kho)
	call := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		return w
	}
	if w := call("GET", "/me/nep/tri-nho/cai-dat", ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"nho":false`) {
		t.Fatalf("GET %d %s", w.Code, w.Body)
	}
	if w := call("POST", "/me/nep/su-kien", `{"su_kien":[{"loai":"tao_keo","luc":"2026-09-25T00:00:00Z"}]}`); w.Code != 409 || !strings.Contains(w.Body.String(), "nep_tri_nho_tat") {
		t.Fatalf("events while off %d %s", w.Code, w.Body)
	}
	if n := b.so(t, `SELECT count(*) FROM nep_su_kien WHERE person_id=$1`, p); n != 0 {
		t.Fatal("events written while off")
	}
	if w := call("PUT", "/me/nep/tri-nho/cai-dat", `{"nho":true,"cong_bo_ban":99}`); w.Code != 409 {
		t.Fatalf("stale disclosure %d", w.Code)
	}
	if w := call("PUT", "/me/nep/tri-nho/cai-dat", `{"nho":true,"cong_bo_ban":1}`); w.Code != 200 || !strings.Contains(w.Body.String(), `"nho":true`) {
		t.Fatalf("PUT on %d %s", w.Code, w.Body)
	}
	if w := call("POST", "/me/nep/su-kien", `{"su_kien":[{"loai":"chon_phuong_tien","luc":"2026-09-25T00:00:00Z","phuong_tien":"walk"}]}`); w.Code != 204 {
		t.Fatalf("events on %d", w.Code)
	}
	if w := call("POST", "/me/nep/su-kien", `{"su_kien":[{"loai":"tao_keo","luc":"2026-09-25T00:00:00Z"}]}`); w.Code != 429 {
		t.Fatalf("second batch %d", w.Code)
	}
	if w := call("DELETE", "/me/nep/tri-nho", ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"xoa_xong":true`) {
		t.Fatalf("DELETE %d %s", w.Code, w.Body)
	}
	if w := call("PUT", "/me/nep/tri-nho/cai-dat", `{"nho":false}`); w.Code != 200 {
		t.Fatalf("PUT off %d", w.Code)
	}
	// A revoked session is 401.
	_, _ = b.pool.Exec(ctx, `UPDATE account_sessions SET revoked_at=now() WHERE person_id=$1`, p)
	if w := call("GET", "/me/nep/tri-nho/cai-dat", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("revoked session %d", w.Code)
	}
}

func hexSum(s string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(s))) }

// A fact past its end is deleted at consolidation, not only hidden
// (re-review memory minor 3): the hourly pass hides it and opens its
// one-fact deletion on the memory lane, without a tombstone, and the saga
// deletes it from the sidecar and writes the receipt. A fact still in force
// is left alone.
func TestHetHanXoaKhiCungCo(t *testing.T) {
	b := moiBo(t)
	ctx := context.Background()
	p := moiNguoi(t, b.pool)
	if err := b.kho.Bat(ctx, p, CongBoBan); err != nil {
		t.Fatal(err)
	}
	den := b.now.Add(time.Hour)
	moi := suThatMoi("Đi xe buýt tới cuối tuần")
	moi.DenLuc = &den
	het, err := b.kho.Ghi(ctx, p, moi)
	if err != nil {
		t.Fatal(err)
	}
	con, err := b.kho.Ghi(ctx, p, suThatMoi("Thích cà phê yên tĩnh"))
	if err != nil {
		t.Fatal(err)
	}
	b.now = b.now.Add(2 * time.Hour)
	tx, _ := b.pool.Begin(ctx)
	if err := b.kho.DinhKyDon().Chay(ctx, tx); err != nil {
		t.Fatal(err)
	}
	_ = tx.Commit(ctx)
	var viec string
	if err := b.pool.QueryRow(ctx, `SELECT id::text FROM nep_xoa WHERE person_id=$1 AND pham_vi='mot' AND su_that_id=$2 AND buoc='cho'`, p, het.ID).Scan(&viec); err != nil {
		t.Fatalf("no deletion opened for the expired fact: %v", err)
	}
	if n := b.so(t, `SELECT count(*) FROM job_outbox WHERE queue='memory' AND ref_id=$1`, viec); n != 1 {
		t.Fatal("the expired fact's deletion is not on the memory lane")
	}
	if n := b.so(t, `SELECT count(*) FROM nep_su_that WHERE id=$1 AND dang_xoa_at IS NOT NULL`, het.ID); n != 1 {
		t.Fatal("the expired fact is not hidden")
	}
	if n := b.so(t, `SELECT count(*) FROM nep_quen WHERE person_id=$1`, p); n != 0 {
		t.Fatal("an expired fact was tombstoned")
	}
	if n := b.so(t, `SELECT count(*) FROM nep_xoa WHERE su_that_id=$1`, con.ID); n != 0 {
		t.Fatal("a fact in force was handed to deletion")
	}
	// Twice is the same deletion.
	tx, _ = b.pool.Begin(ctx)
	if n, err := b.kho.XoaHetHan(ctx, tx); err != nil || n != 0 {
		t.Fatalf("second consolidation = %d %v", n, err)
	}
	_ = tx.Commit(ctx)
	if xong, err := b.kho.ChayNgay(ctx, viec); err != nil || !xong {
		t.Fatalf("the saga did not finish: %v %v", xong, err)
	}
	if n := b.so(t, `SELECT count(*) FROM nep_su_that WHERE id=$1 AND deleted_at IS NOT NULL`, het.ID); n != 1 {
		t.Fatal("no receipt for the expired fact")
	}
	left, _ := b.gia.LietKe(ctx, p)
	if len(left) != 1 || left[0].ID != con.ID {
		t.Fatalf("the sidecar still holds %+v", left)
	}
}

// The sidecar is called with no row of nep_xoa locked and, on the request
// and lane path, outside any transaction (re-review memory minor 5): while
// the fake sidecar answers a delete, another connection locks the deletion's
// row at once (NOWAIT), and a second attempt at the same deletion is turned
// away by the attempt's advisory lock. The periodic pass holds no row lock
// through its calls either.
func TestSidecarNgoaiGiaoDich(t *testing.T) {
	b := moiBo(t)
	ctx := context.Background()
	p := moiNguoi(t, b.pool)
	_ = b.kho.Bat(ctx, p, CongBoBan)
	f, err := b.kho.Ghi(ctx, p, suThatMoi("Thích cà phê yên tĩnh"))
	if err != nil {
		t.Fatal(err)
	}
	var viec string
	var thuKhoa, thuLai []string
	khoaNgay := func() {
		tx, err := b.pool.Begin(ctx)
		if err != nil {
			thuKhoa = append(thuKhoa, err.Error())
			return
		}
		defer tx.Rollback(ctx)
		var id string
		if err := tx.QueryRow(ctx, `SELECT id::text FROM nep_xoa WHERE id=$1 FOR UPDATE NOWAIT`, viec).Scan(&id); err != nil {
			thuKhoa = append(thuKhoa, err.Error())
		}
	}
	lan := 0
	b.gia.trongXoa = func() {
		lan++
		if lan > 1 {
			return // the second attempt's own call, if it got that far
		}
		khoaNgay()
		if xong, err := b.kho.ChayNgay(ctx, viec); xong || err != nil {
			thuLai = append(thuLai, fmt.Sprint(xong, err))
		}
	}
	viec, ok, err := b.kho.anQuen(ctx, p, f.ID, "")
	if err != nil || !ok {
		t.Fatalf("anQuen %v %v", ok, err)
	}
	if xong, err := b.kho.ChayNgay(ctx, viec); err != nil || !xong {
		t.Fatalf("ChayNgay %v %v", xong, err)
	}
	if len(thuKhoa) != 0 || len(thuLai) != 0 || b.gia.goi["xoa"] != 1 {
		t.Fatalf("row locked during the call: %v; second attempt ran: %v; %d deletes", thuKhoa, thuLai, b.gia.goi["xoa"])
	}
	// The periodic pass: a failing attempt, then the pass once due.
	f2, _ := b.kho.Ghi(ctx, p, suThatMoi("Hay đi xe máy"))
	b.gia.trongXoa = nil
	b.gia.conSot = 1
	if _, err := b.kho.Quen(ctx, p, trinho.QuenGi{ID: f2.ID}); !errors.Is(err, ErrDangXoa) {
		t.Fatalf("Quen with rows left: %v", err)
	}
	if err := b.pool.QueryRow(ctx, `SELECT id::text FROM nep_xoa WHERE su_that_id=$1`, f2.ID).Scan(&viec); err != nil {
		t.Fatal(err)
	}
	b.now = b.now.Add(time.Hour)
	thuKhoa = nil
	b.gia.trongXoa = khoaNgay
	pass, _ := b.pool.Begin(ctx)
	defer pass.Rollback(ctx)
	// Other tests' deletions due by now may share the pass.
	if n, err := b.kho.LuotXoa(ctx, pass); err != nil || n < 1 {
		t.Fatalf("pass %d %v", n, err)
	}
	_ = pass.Commit(ctx)
	if len(thuKhoa) != 0 {
		t.Fatalf("the pass held the row during the call: %v", thuKhoa)
	}
	if n := b.so(t, `SELECT count(*) FROM nep_xoa WHERE id=$1 AND buoc='xong'`, viec); n != 1 {
		t.Fatal("the pass did not finish the deletion")
	}
}

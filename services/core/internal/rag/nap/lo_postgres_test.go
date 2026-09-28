//go:build postgres

package nap_test

import (
	"context"
	"testing"
	"time"

	"mobile/services/core/internal/naptest"
	"mobile/services/core/internal/rag/nap"
)

// loGia is a batch door that answers from the stub encoder: the job runs
// for `chay` polls, then finishes (or fails, with hong).
type loGia struct {
	model    string
	enc      nap.StubDense
	gui, xem int
	chay     int
	hong     bool
	lastDocs int
	jobs     map[string][]nap.LoVao
}

func (l *loGia) Model() string { return l.model }
func (l *loGia) Dims() int     { return l.enc.Dims() }

func (l *loGia) GuiLo(_ context.Context, _ string, docs []nap.LoVao) (string, error) {
	l.gui++
	l.lastDocs = len(docs)
	if l.jobs == nil {
		l.jobs = map[string][]nap.LoVao{}
	}
	job := "batches/gia" + string(rune('a'+l.gui))
	l.jobs[job] = docs
	return job, nil
}

func (l *loGia) XemLo(ctx context.Context, job string) (nap.KetQuaLo, error) {
	l.xem++
	if l.xem <= l.chay {
		return nap.KetQuaLo{}, nil
	}
	if l.hong {
		return nap.KetQuaLo{Hong: true, Loi: "JOB_STATE_FAILED"}, nil
	}
	docs := l.jobs[job]
	tl := make([]nap.TaiLieu, len(docs))
	for i, d := range docs {
		tl[i] = d.TaiLieu
	}
	vecs, _ := l.enc.NhungTaiLieu(ctx, tl)
	out := map[string][]float32{}
	for i, d := range docs {
		out[d.Khoa] = vecs[i]
	}
	return nap.KetQuaLo{Xong: true, Vecs: out}, nil
}

// stubTen is the stub encoder under the configuration's model name, so its
// cache entries are the ones the batch door (same name) reads and writes.
type stubTen struct {
	nap.StubDense
	ten string
}

func (s stubTen) Model() string { return s.ten }

// TestNhungQuaLoChiGuiPhanThieuVaKhongGuiHaiLan: only the hashes the cache
// lacks are submitted; a run cut short leaves the job running and the next
// run polls it without submitting again; the finished vectors are the
// cache's, and a build then needs no encoder call.
func TestNhungQuaLoChiGuiPhanThieuVaKhongGuiHaiLan(t *testing.T) {
	pool := naptest.Pool(t)
	n, enc := naptest.Nap(t, nap.NewKhoNho())
	v := naptest.Vang(t)
	var rows []nap.Hang
	for i, q := range v.Quan[:30] {
		h, _ := nap.DungHoSo(v.Hang(i, q))
		hs, err := nap.DoanQuan(context.Background(), h, q.NhanTay(), n.Cfg.Chunker[nap.CorpusQuan], nap.ChiaNguyen{})
		if err != nil {
			t.Fatal(err)
		}
		rows = append(rows, hs...)
	}
	// A third of the rows are already cached.
	pre := append([]nap.Hang(nil), rows[:len(rows)/3]...)
	named := stubTen{enc, n.Cfg.Dense.Model}
	if _, err := nap.NhungHang(context.Background(), named, nap.BoNhoPG{Q: pool}, n.Cfg, pre); err != nil {
		t.Fatal(err)
	}
	lo := &loGia{model: n.Cfg.Dense.Model, enc: enc, chay: 1000}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	rep, err := n.NhungQuaLo(ctx, pool, lo, rows, 20*time.Millisecond)
	cancel()
	if err != nil || rep.TrangThai != "dang_chay" || lo.gui != 1 || rep.Gui != rep.Can-rep.DaCo || rep.DaCo == 0 {
		t.Fatalf("first run: %+v, %d submits, %v", rep, lo.gui, err)
	}
	// Second run: the job finishes; nothing is submitted again.
	lo.chay = lo.xem
	rep2, err := n.NhungQuaLo(context.Background(), pool, lo, rows, time.Millisecond)
	if err != nil || rep2.TrangThai != "xong" || lo.gui != 1 || rep2.Gui != 0 || rep2.Ghi != rep.Gui {
		t.Fatalf("second run: %+v, %d submits, %v", rep2, lo.gui, err)
	}
	// Everything is cached: a build needs no encoder call, and a third run
	// submits nothing.
	if miss, err := nap.NhungHang(context.Background(), named, nap.BoNhoPG{Q: pool}, n.Cfg, append([]nap.Hang(nil), rows...)); err != nil || miss != 0 {
		t.Fatalf("after the batch %d rows still needed the encoder (%v)", miss, err)
	}
	rep3, err := n.NhungQuaLo(context.Background(), pool, lo, rows, time.Millisecond)
	if err != nil || rep3.TrangThai != "khong_can" || lo.gui != 1 {
		t.Fatalf("third run: %+v %v", rep3, err)
	}
}

// TestNhungQuaLoHongKhongGhi: a failed job writes no vector and closes its
// row, so the next run submits afresh.
func TestNhungQuaLoHongKhongGhi(t *testing.T) {
	pool := naptest.Pool(t)
	n, enc := naptest.Nap(t, nap.NewKhoNho())
	v := naptest.Vang(t)
	h, _ := nap.DungHoSo(v.Hang(0, v.Quan[0]))
	rows, err := nap.DoanQuan(context.Background(), h, v.Quan[0].NhanTay(), n.Cfg.Chunker[nap.CorpusQuan], nap.ChiaNguyen{})
	if err != nil {
		t.Fatal(err)
	}
	lo := &loGia{model: n.Cfg.Dense.Model, enc: enc, hong: true}
	if _, err := n.NhungQuaLo(context.Background(), pool, lo, rows, time.Millisecond); err == nil {
		t.Fatal("a failed job reported success")
	}
	if miss, _ := nap.NhungHang(context.Background(), nap.StubDense{N: enc.N}, nil, n.Cfg, append([]nap.Hang(nil), rows...)); miss == 0 {
		t.Fatal("stub check")
	}
	var open int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM rag_embed_batches WHERE trang_thai='dang_chay'`).Scan(&open); err != nil || open != 0 {
		t.Fatalf("a failed job stayed open: %d %v", open, err)
	}
	lo.hong = false
	if _, err := n.NhungQuaLo(context.Background(), pool, lo, rows, time.Millisecond); err != nil || lo.gui != 2 {
		t.Fatalf("after a failure the next run must submit afresh: %d submits, %v", lo.gui, err)
	}
}

// TestCanLamGiauKhopCong: the places the enrichment selects are exactly the
// ones the build gate counts as lacking an enrichment, whatever the chunker
// does to a long facet (a stub-driven split once hid places from the
// enrichment while the gate kept counting them).
func TestCanLamGiauKhopCong(t *testing.T) {
	pool := naptest.Pool(t)
	v := naptest.Vang(t)
	naptest.NapVang(t, pool, v)
	n, _ := naptest.Nap(t, nap.NewKhoNho())
	var rep nap.BaoCaoDung
	if _, _, err := n.ChuanBiQuan(context.Background(), pool, &rep); err != nil {
		t.Fatal(err)
	}
	can, err := nap.CanLamGiau(context.Background(), pool)
	if err != nil {
		t.Fatal(err)
	}
	if len(can) != rep.ThieuLamGiau || len(can) == 0 {
		t.Fatalf("enrichment selects %d, the gate counts %d", len(can), rep.ThieuLamGiau)
	}
}

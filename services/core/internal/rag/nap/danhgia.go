package nap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"mobile/services/core/internal/domain/tuvung"
	"mobile/services/core/internal/huongdan"
)

// Offline evaluation gate (S11; research sdlc-production §C4, design 04
// §8.3). Three parts, each able to refuse a version:
//
//  1. reconciliation against Postgres (0 missing, 0 extra, 0 changed), no
//     dead letter, no place without a current enrichment;
//  2. hard-filter probes on the candidate collection itself: every
//     allergen, diet, budget and time in every destination, each admitted
//     row checked against the truth Postgres rebuilds; violation has no
//     tolerance;
//  3. relevance on the golden set, built with the same configuration and
//     encoders into a transient collection: recall@10, nDCG@10, MRR@10,
//     violation@10, the no-diacritics gap, each branch alone (ablation),
//     the no-diacritics questions without the folded BM25 leg (what the
//     second analyzer adds), and no worse than the active version.

// KetQuaVang is one golden run.
type KetQuaVang struct {
	Sha      string          `json:"vang_sha256"`
	Nhom     map[string]SoDo `json:"nhom"`
	Tong     SoDo            `json:"tong"`
	GapDau   float64         `json:"khong_dau_gap"`
	ChiDense SoDo            `json:"chi_dense"`
	ChiThua  SoDo            `json:"chi_thua"`
	// KhongDauKhongGap is the khong_dau group measured with the folded BM25
	// leg's weight at 0: set beside Nhom["khong_dau"], it is the recall the
	// diacritics-folding field buys on questions typed without marks.
	KhongDauKhongGap SoDo    `json:"khong_dau_khong_gap"`
	GapDauKhongGap   float64 `json:"khong_dau_gap_khong_gap"`
	// BoDau is every golden question with its text folded to no diacritics
	// (a structural transform of the question, test side), fused with the
	// served weights; BoDauKhongGap the same without the folded BM25 leg.
	BoDau         SoDo `json:"bo_dau"`
	BoDauKhongGap SoDo `json:"bo_dau_khong_gap"`
}

// ChayVang runs every golden question against a collection holding the
// golden catalogue, fused with the weights w, and scores the top ten
// documents.
func ChayVang(ctx context.Context, kho KhoVector, ten string, q NhungCauHoi, cfg CauHinh, v TapVang, w TrongSo) (map[string]SoDo, map[string]SoDo, error) {
	cau := make([]string, len(v.TruyVan))
	for i, t := range v.TruyVan {
		cau[i] = t.Cau
	}
	vecs, err := q.NhungCauHoi(ctx, cau)
	if err != nil {
		return nil, nil, err
	}
	if len(vecs) != len(cau) {
		return nil, nil, fmt.Errorf("%w: %d query vectors for %d questions", ErrVector, len(vecs), len(cau))
	}
	sums := map[string]SoDo{}
	each := map[string]SoDo{}
	for i, t := range v.TruyVan {
		dense, err := KiemVector(vecs[i], cfg.Dense.Dims)
		if err != nil {
			return nil, nil, err
		}
		hits, err := kho.TimLai(ctx, ten, TruyVan{Dense: dense, Chu: t.Cau, Loc: t.RangBuoc.Loc(), K: 50, KMoiNhanh: 50,
			RRFK: cfg.Hop.RRFK, TrongSo: w})
		if err != nil {
			return nil, nil, err
		}
		var top []string
		seen := map[string]bool{}
		for _, h := range hits {
			if !seen[h.DocID] {
				seen[h.DocID] = true
				top = append(top, h.DocID)
			}
		}
		s := ChamMot(t, top, func(id string) string { return v.ViPham(t, id) })
		each[t.ID] = s
		g := sums[t.Nhom]
		g.cong(s)
		sums[t.Nhom] = g
	}
	return sums, each, nil
}

// TongHop turns group sums into the reported measurement.
func TongHop(sums map[string]SoDo, each map[string]SoDo, v TapVang) (map[string]SoDo, SoDo, float64) {
	out := map[string]SoDo{}
	var tong SoDo
	for k, s := range sums {
		out[k] = s.chia()
		tong.cong(s)
	}
	// The no-diacritics gap: over each khong_dau question and the question
	// it rewrites (cap), mean recall@10 and nDCG@10 of the marked side minus
	// the unmarked side; the larger of the two.
	var dr, dn float64
	n := 0
	for _, t := range v.TruyVan {
		if t.Nhom != "khong_dau" || t.Cap == "" {
			continue
		}
		a, okA := each[t.Cap]
		b := each[t.ID]
		if !okA || a.CoLienQuan == 0 || b.CoLienQuan == 0 {
			continue
		}
		dr += a.Recall - b.Recall
		dn += a.NDCG - b.NDCG
		n++
	}
	gap := 0.0
	if n > 0 {
		gap = math.Round(math.Max(dr, dn)/float64(n)*10000) / 10000
	}
	return out, tong.chia(), gap
}

// DanhGiaVang builds the golden catalogue into a transient collection with
// this pipeline's configuration and encoders, runs the golden set three
// ways (hybrid, dense only, sparse only) and drops the collection.
func (n Nap) DanhGiaVang(ctx context.Context, q NhungCauHoi, v TapVang, ten string) (KetQuaVang, error) {
	chunker := n.Cfg.Chunker[CorpusQuan]
	bia := map[string]bool{}
	for _, b := range v.BiaTay {
		bia[b.ID] = true
	}
	var rows []Hang
	for i, qv := range v.Quan {
		if bia[qv.ID] {
			continue
		}
		h, bo := DungHoSo(v.Hang(i, qv))
		if bo {
			continue
		}
		hs, err := DoanQuan(ctx, h, qv.NhanTay(), chunker, ChiaNghia{Nhung: n.Dense})
		if err != nil {
			return KetQuaVang{}, err
		}
		rows = append(rows, hs...)
	}
	if _, err := n.Vector(ctx, nil, rows); err != nil {
		return KetQuaVang{}, err
	}
	_ = n.Kho.XoaCollection(ctx, ten)
	if err := n.Kho.TaoCollection(ctx, ten, LuocDoTu(n.Cfg, CorpusQuan)); err != nil {
		return KetQuaVang{}, err
	}
	defer func() { _ = n.Kho.XoaCollection(context.WithoutCancel(ctx), ten) }()
	if err := GhiHang(ctx, n.Kho, ten, rows); err != nil {
		return KetQuaVang{}, err
	}
	if err := ChoTimThay(ctx, n.Kho, ten, rows); err != nil {
		return KetQuaVang{}, err
	}
	kq := KetQuaVang{Sha: v.Sha}
	w := n.Cfg.Hop.TrongSo
	sums, each, err := ChayVang(ctx, n.Kho, ten, q, n.Cfg, v, w)
	if err != nil {
		return kq, err
	}
	kq.Nhom, kq.Tong, kq.GapDau = TongHop(sums, each, v)
	for _, mode := range []struct {
		w    TrongSo
		into *SoDo
	}{{w.ChiDense(), &kq.ChiDense}, {w.ChiThua(), &kq.ChiThua}} {
		s, e, err := ChayVang(ctx, n.Kho, ten, q, n.Cfg, v, mode.w)
		if err != nil {
			return kq, err
		}
		_, *mode.into, _ = TongHop(s, e, v)
	}
	s, e, err := ChayVang(ctx, n.Kho, ten, q, n.Cfg, v, w.KhongGap())
	if err != nil {
		return kq, err
	}
	var nhom map[string]SoDo
	nhom, _, kq.GapDauKhongGap = TongHop(s, e, v)
	kq.KhongDauKhongGap = nhom["khong_dau"]
	bd := v.BoDau()
	for _, mode := range []struct {
		w    TrongSo
		into *SoDo
	}{{w, &kq.BoDau}, {w.KhongGap(), &kq.BoDauKhongGap}} {
		s, e, err := ChayVang(ctx, n.Kho, ten, q, n.Cfg, bd, mode.w)
		if err != nil {
			return kq, err
		}
		_, *mode.into, _ = TongHop(s, e, bd)
	}
	return kq, nil
}

// ErrChuaThay: rows just written never became searchable.
var ErrChuaThay = errors.New("nap: rows just written did not become searchable")

// ChoTimThay waits until the last of rows is found by each leg a search
// uses -- the dense leg by its own vector, the folded BM25 leg by its own
// text -- so a measurement never scores an index that is still catching up
// with its writes (Milvus makes a fresh upsert searchable a moment after it
// counts it, even at Strong consistency). At most about ten seconds.
func ChoTimThay(ctx context.Context, kho KhoVector, ten string, rows []Hang) error {
	if len(rows) == 0 {
		return nil
	}
	last := rows[len(rows)-1]
	thay := func(tv TruyVan) (bool, error) {
		hits, err := kho.TimLai(ctx, ten, tv)
		if err != nil {
			return false, err
		}
		for _, h := range hits {
			if h.ChunkID == last.ChunkID {
				return true, nil
			}
		}
		return false, nil
	}
	for i := 0; i < 100; i++ {
		d, err := thay(TruyVan{Dense: last.Dense, K: 10, KMoiNhanh: 10, TrongSo: TrongSo{Dense: 1}})
		if err != nil {
			return err
		}
		thua := TruyVan{Chu: last.Text, K: 50, KMoiNhanh: 50, TrongSo: TrongSo{BM25KhongDau: 1}}
		if len(last.SparseIdx) > 0 {
			thua = TruyVan{Thua: VectorThua{Idx: last.SparseIdx, Val: last.SparseVal}, K: 50, KMoiNhanh: 50, TrongSo: TrongSo{MILCO: 1}}
		}
		b, err := thay(thua)
		if err != nil {
			return err
		}
		if d && b {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return ErrChuaThay
}

// KetQuaCong is the gate's verdict on one version, recorded in
// rag_vector_versions.eval: numbers, enums and reasons, no text.
type KetQuaCong struct {
	DoiSoat DoiSoat     `json:"doi_soat"`
	DLQ     int         `json:"dlq"`
	Thieu   int         `json:"thieu_lam_giau"`
	ThamDo  int         `json:"tham_do"`
	ViPham  int         `json:"vi_pham_loc"`
	CanVang bool        `json:"can_vang"`
	Vang    *KetQuaVang `json:"vang,omitempty"`
	Nguong  NguongCong  `json:"nguong"`
	VanTay  string      `json:"van_tay"`
	LyDo    []string    `json:"ly_do"`
	Dat     bool        `json:"dat"`
}

// KiemCong applies the thresholds (and, when active is given and measured
// on the same golden file, «no worse than active − tolerance») and fills
// LyDo with every reason for a refusal.
func KiemCong(k *KetQuaCong, active *KetQuaVang) {
	g := k.Nguong
	var why []string
	if !k.DoiSoat.Dat {
		why = append(why, "doi_soat")
	}
	if k.DLQ > 0 {
		why = append(why, "dlq")
	}
	if k.Thieu > 0 {
		why = append(why, "thieu_lam_giau")
	}
	if k.ThamDo == 0 {
		why = append(why, "khong_tham_do")
	}
	if k.ViPham > 0 {
		why = append(why, "vi_pham_loc")
	}
	if k.CanVang && k.Vang == nil {
		why = append(why, "khong_vang")
	} else if k.CanVang {
		t := k.Vang.Tong
		if t.Recall < g.Recall10 {
			why = append(why, "recall_10")
		}
		if t.NDCG < g.NDCG10 {
			why = append(why, "ndcg_10")
		}
		if t.MRR < g.MRR10 {
			why = append(why, "mrr_10")
		}
		if t.Violation > g.Violation10 || t.ViPham > 0 {
			why = append(why, "violation_10")
		}
		if k.Vang.GapDau > g.KhongDauGap {
			why = append(why, "khong_dau_gap")
		}
		if active != nil && active.Sha == k.Vang.Sha {
			a := active.Tong
			tol := g.KhongKemActive + 1e-9
			if t.Recall < a.Recall-tol || t.NDCG < a.NDCG-tol || t.MRR < a.MRR-tol {
				why = append(why, "kem_active")
			}
		}
	}
	k.LyDo = why
	k.Dat = len(why) == 0
}

// thamDo are the hard-filter probes of one destination: each allergen, each
// diet, five budgets, four instants a day, three windows a day, and
// allergen × diet × budget × time combinations.
func thamDo(dest string) []Loc {
	var out []Loc
	for _, a := range tuvung.DiUng.IDs() {
		out = append(out, Loc{DiemDen: dest, DiUng: []string{a}})
	}
	for _, d := range tuvung.AnKieng.IDs() {
		out = append(out, Loc{DiemDen: dest, AnKieng: []string{d}})
	}
	for _, b := range []int64{30_000, 60_000, 100_000, 200_000, 500_000} {
		b := b
		out = append(out, Loc{DiemDen: dest, NganSach: &b})
	}
	for day := 0; day < 7; day++ {
		for _, m := range []int{7 * 60, 12 * 60, 19*60 + 30, 23*60 + 30} {
			o := int16((day*1440 + m) / PhutMoiO)
			out = append(out, Loc{DiemDen: dest, O: &o})
		}
		for _, w := range [][2]string{{"11:00", "14:00"}, {"18:00", "22:00"}} {
			days := []string{"Mo", "Tu", "We", "Th", "Fr", "Sa", "Su"}
			out = append(out, RangBuocVang{DiemDen: dest, Khung: []string{days[day], w[0], days[day], w[1]}}.Loc())
		}
	}
	for _, a := range tuvung.DiUng.IDs() {
		b := int64(200_000)
		o := int16((4*1440 + 19*60) / PhutMoiO)
		out = append(out, Loc{DiemDen: dest, DiUng: []string{a}, AnKieng: []string{"chay"}, NganSach: &b, O: &o})
	}
	return out
}

// ThamDoLoc runs the probes over a collection and checks each admitted row
// against truth (chunk id → the row Postgres rebuilds). A row the filter
// admits that the truth refuses is a violation; a row missing from truth is
// one too (the index holds what the catalogue does not).
func ThamDoLoc(ctx context.Context, kho KhoVector, ten string, truth map[string]Hang, dests []string) (probes, violations int, err error) {
	for _, d := range dests {
		for _, l := range thamDo(d) {
			got, err := kho.LocHang(ctx, ten, l)
			if err != nil {
				return probes, violations, err
			}
			probes++
			for _, k := range got {
				r, ok := truth[k.ChunkID]
				if !ok || !l.Khop(r) {
					violations++
				}
			}
		}
	}
	return probes, violations, nil
}

// DanhGia evaluates a built version of the place corpus and moves it to
// `evaluated` or `failed`. golden is the golden set; q the query encoder
// matching n.Dense. The manual corpus is reconciled only (its relevance
// gate is huongdan's golden, run by `go test`).
func (n Nap) DanhGia(ctx context.Context, db CSDL, id int64, q NhungCauHoi, golden TapVang) (KetQuaCong, error) {
	k := KetQuaCong{Nguong: n.Cfg.Cong, VanTay: n.Cfg.VanTay()}
	p, err := DocPhienBan(ctx, db, id)
	if err != nil {
		return k, err
	}
	if p.State != "built" {
		return k, ErrTrangThai
	}
	if p.VanTay != n.Cfg.VanTay() || p.DenseModel != n.Dense.Model() {
		return k, fmt.Errorf("%w: version built with another configuration (%s, %s)", ErrCauHinh, p.VanTay, p.DenseModel)
	}
	var truth []Hang
	switch p.Corpus {
	case CorpusQuan:
		k.CanVang = true
		var rep BaoCaoDung
		kept, _, _, _, err := n.KyVongQuan(ctx, db, &rep)
		if err != nil {
			return k, err
		}
		k.Thieu = rep.ThieuLamGiau
		dests := map[string]bool{}
		for _, d := range kept {
			truth = append(truth, d.Rows...)
			dests[d.HoSo.DiemDen] = true
		}
		if k.DoiSoat, err = DoiSoatHang(ctx, n.Kho, p.Collection, truth); err != nil {
			return k, err
		}
		byID := map[string]Hang{}
		for _, r := range truth {
			byID[r.ChunkID] = r
		}
		var ds []string
		for d := range dests {
			ds = append(ds, d)
		}
		sort.Strings(ds)
		if k.ThamDo, k.ViPham, err = ThamDoLoc(ctx, n.Kho, p.Collection, byID, ds); err != nil {
			return k, err
		}
		vang, err := n.DanhGiaVang(ctx, q, golden, TenEval(n.Cfg))
		if err != nil {
			return k, err
		}
		k.Vang = &vang
	case CorpusSoTay:
		truth = DoanSoTay(huongdan.TatCa(), n.Cfg.Chunker[CorpusSoTay])
		if k.DoiSoat, err = DoiSoatHang(ctx, n.Kho, p.Collection, truth); err != nil {
			return k, err
		}
		k.ThamDo = 1 // the manual has no hard filter to probe
	}
	if err = db.QueryRow(ctx, `SELECT count(*) FROM rag_ingest_dlq WHERE corpus=$1`, string(p.Corpus)).Scan(&k.DLQ); err != nil {
		return k, err
	}
	var active *KetQuaVang
	if a, err := PhienBanActive(ctx, db, p.Corpus); err == nil && len(a.Eval) > 0 {
		var ak KetQuaCong
		if json.Unmarshal(a.Eval, &ak) == nil && ak.Vang != nil {
			active = ak.Vang
		}
	} else if err != nil && !errors.Is(err, ErrKhongActive) {
		return k, err
	}
	KiemCong(&k, active)
	next := "failed"
	if k.Dat {
		next = "evaluated"
	}
	body, _ := json.Marshal(k)
	if _, err = db.Exec(ctx, `UPDATE rag_vector_versions SET state=$2, eval=$3 WHERE id=$1 AND state='built'`, id, next, body); err != nil {
		return k, err
	}
	return k, nil
}

// TenEval is the transient collection the golden set is built into.
func TenEval(cfg CauHinh) string { return "rd_eval__" + cfg.VanTay() }

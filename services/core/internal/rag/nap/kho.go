package nap

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"unicode"

	"mobile/services/core/internal/domain/promptsafety"
	"mobile/services/core/internal/domain/tuvung"
)

// KhoVector is what the pipeline needs from the vector database. The Milvus
// adapter is vectordb/napkho, over vectordb's client and schema (the ones
// retrieval reads); KhoNho is the in-memory stand-in for unit tests and the
// pinned offline eval.
type KhoVector interface {
	// TaoCollection creates a collection with its indexes and loads it. A
	// collection that exists already is an error: versions are immutable.
	TaoCollection(ctx context.Context, ten string, ld LuocDo) error
	XoaCollection(ctx context.Context, ten string) error
	DanhSachCollection(ctx context.Context) ([]string, error)
	// Upsert writes rows by primary key (chunk_id).
	Upsert(ctx context.Context, ten string, rows []Hang) error
	// XoaID deletes by primary key.
	XoaID(ctx context.Context, ten string, ids []string) error
	// CapNhatThuocTinh rewrites the hard-filter attributes of every row of
	// document docID in ten from r (DiemDen, DiUng/DiUngRo, AnKieng,
	// GiaMin/GiaMax/GiaRo, MoO/GioRo), leaving vectors and text as they
	// are, and returns how many rows it rewrote.
	CapNhatThuocTinh(ctx context.Context, ten, docID string, r Hang) (int, error)
	// Dem is count(*) at strong consistency.
	Dem(ctx context.Context, ten string) (int64, error)
	// LietKe lists every row's keys at strong consistency, for
	// reconciliation.
	LietKe(ctx context.Context, ten string) ([]KhoaHang, error)
	// TimLai is one filtered hybrid search.
	TimLai(ctx context.Context, ten string, tv TruyVan) ([]Trung, error)
	// LocHang lists every row the filter admits (no ranking), at strong
	// consistency: the evaluation gate checks each against the truth.
	LocHang(ctx context.Context, ten string, l Loc) ([]KhoaHang, error)
	// DatAlias points alias at ten, creating it if absent.
	DatAlias(ctx context.Context, alias, ten string) error
	// MoTaAlias is the collection alias points at, "" when there is none.
	MoTaAlias(ctx context.Context, alias string) (string, error)
}

// KhoaHang is a row's keys as reconciliation reads them.
type KhoaHang struct {
	ChunkID     string
	DocID       string
	ContentHash string
	DenseModel  string
}

// LuocDo is a collection's schema parameters: the corpus, the dense
// dimensionality, the sparse mode and the revision of vectordb's schema the
// configuration was written for (napkho refuses a mismatch).
type LuocDo struct {
	Corpus Corpus
	Dims   int
	CheDo  CheDoThua
	Ban    string
}

// LuocDoTu is the schema a configuration asks for.
func LuocDoTu(cfg CauHinh, c Corpus) LuocDo {
	return LuocDo{Corpus: c, Dims: cfg.Dense.Dims, CheDo: cfg.Thua.CheDo, Ban: cfg.LuocDo}
}

// Loc are hard filters, as ids and numbers (truyhoi.Cung's shape; rag may
// not import truyhoi). Every field is applied, never relaxed. The Milvus
// expression is vectordb.LocCung's (napkho converts); Khop is this
// package's own reading of the same rule, for KhoNho and for the gate's
// row-by-row check.
//
// Unknown attributes under a hard constraint are out (docs/architecture/03
// §8.4): allergens not established (DiUngRo false) under any allergy, an
// unknown price under a budget, unknown hours under a time or a window.
type Loc struct {
	DiemDen string
	// DiUng are the asker's allergens; the filter closes them over the
	// family (tuvung.MoRongDiUng) and refuses a place whose allergens are
	// unknown.
	DiUng    []string
	AnKieng  []string
	NganSach *int64
	// O is a slot (minute of the week / PhutMoiO) the place must be open
	// for; Khung a set of slots of which one must be open.
	O     *int16
	Khung []int16
}

// Khop is the filter evaluated in Go, for KhoNho and for checking a
// search's answer row by row.
func (l Loc) Khop(r Hang) bool {
	if l.DiemDen != "" && r.DiemDen != l.DiemDen {
		return false
	}
	if du := tuvung.MoRongDiUng(l.DiUng); len(du) > 0 {
		if !r.DiUngRo || giao(r.DiUng, du) {
			return false
		}
	}
	for _, a := range l.AnKieng {
		if !co(r.AnKieng, a) {
			return false
		}
	}
	if l.NganSach != nil && (!r.GiaRo || r.GiaMin > *l.NganSach) {
		return false
	}
	if l.O != nil && (!r.GioRo || !coO(r.MoO, *l.O)) {
		return false
	}
	if len(l.Khung) > 0 {
		if !r.GioRo {
			return false
		}
		ok := false
		for _, k := range l.Khung {
			if coO(r.MoO, k) {
				ok = true
			}
		}
		if !ok {
			return false
		}
	}
	return true
}

func co(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}

func giao(a, b []string) bool {
	for _, x := range a {
		if co(b, x) {
			return true
		}
	}
	return false
}

func coO(xs []int16, x int16) bool {
	i := sort.Search(len(xs), func(i int) bool { return xs[i] >= x })
	return i < len(xs) && xs[i] == x
}

// TrongSo are the fusion weights of the legs (cauhinh.json «hop»): each
// leg's reciprocal rank is multiplied by its weight; 0 leaves the leg out.
type TrongSo struct {
	Dense        float64 `json:"dense"`
	BM25         float64 `json:"bm25"`
	BM25KhongDau float64 `json:"bm25_khong_dau"`
	MILCO        float64 `json:"milco"`
}

// TruyVan is one hybrid search: a dense leg and the sparse legs (BM25 over
// Chu with diacritics, BM25 over Chu folded; or a MILCO vector), fused by
// weighted RRF with k = RRFK, under one filter on every leg.
type TruyVan struct {
	Dense     []float32
	Chu       string
	Thua      VectorThua
	Loc       Loc
	K         int
	KMoiNhanh int
	RRFK      int
	TrongSo   TrongSo
}

// ChiDense keeps only the dense leg of w (ablation).
func (w TrongSo) ChiDense() TrongSo { return TrongSo{Dense: w.Dense} }

// ChiThua keeps only the sparse legs of w (ablation).
func (w TrongSo) ChiThua() TrongSo {
	return TrongSo{BM25: w.BM25, BM25KhongDau: w.BM25KhongDau, MILCO: w.MILCO}
}

// KhongGap drops the folded BM25 leg (ablation: what the second analyzer
// adds on questions typed without diacritics).
func (w TrongSo) KhongGap() TrongSo { w.BM25KhongDau = 0; return w }

// Trung is one hit.
type Trung struct {
	ChunkID string
	DocID   string
	Diem    float64
}

// KhoNho is an in-memory KhoVector: brute-force cosine, BM25 over the same
// tokens Milvus's analyzer produces (fold, split on non-letters), RRF. Safe
// for concurrent use.
type KhoNho struct {
	mu      sync.Mutex
	cols    map[string]*colNho
	aliases map[string]string
}

type colNho struct {
	ld   LuocDo
	rows map[string]Hang
}

// NewKhoNho returns an empty store.
func NewKhoNho() *KhoNho {
	return &KhoNho{cols: map[string]*colNho{}, aliases: map[string]string{}}
}

func (k *KhoNho) col(ten string) (*colNho, error) {
	if t, ok := k.aliases[ten]; ok {
		if _, isCol := k.cols[ten]; !isCol {
			ten = t
		}
	}
	c, ok := k.cols[ten]
	if !ok {
		return nil, fmt.Errorf("nap: collection %s does not exist", ten)
	}
	return c, nil
}

func (k *KhoNho) TaoCollection(_ context.Context, ten string, ld LuocDo) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if _, ok := k.cols[ten]; ok {
		return fmt.Errorf("nap: collection %s exists", ten)
	}
	k.cols[ten] = &colNho{ld: ld, rows: map[string]Hang{}}
	return nil
}

func (k *KhoNho) XoaCollection(_ context.Context, ten string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	for a, t := range k.aliases {
		if t == ten {
			return fmt.Errorf("nap: collection %s is aliased by %s", ten, a)
		}
	}
	delete(k.cols, ten)
	return nil
}

func (k *KhoNho) DanhSachCollection(context.Context) ([]string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	var out []string
	for n := range k.cols {
		out = append(out, n)
	}
	sort.Strings(out)
	return out, nil
}

func (k *KhoNho) Upsert(_ context.Context, ten string, rows []Hang) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	c, err := k.col(ten)
	if err != nil {
		return err
	}
	for _, r := range rows {
		if len(r.Dense) != c.ld.Dims {
			return fmt.Errorf("nap: dense has %d dims, collection %d", len(r.Dense), c.ld.Dims)
		}
		c.rows[r.ChunkID] = r
	}
	return nil
}

func (k *KhoNho) XoaID(_ context.Context, ten string, ids []string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	c, err := k.col(ten)
	if err != nil {
		return err
	}
	for _, id := range ids {
		delete(c.rows, id)
	}
	return nil
}

func (k *KhoNho) CapNhatThuocTinh(_ context.Context, ten, docID string, r Hang) (int, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	c, err := k.col(ten)
	if err != nil {
		return 0, err
	}
	n := 0
	for id, old := range c.rows {
		if old.DocID != docID {
			continue
		}
		old.DiemDen, old.DiUng, old.DiUngRo, old.AnKieng = r.DiemDen, r.DiUng, r.DiUngRo, r.AnKieng
		old.GiaMin, old.GiaMax, old.GiaRo, old.MoO, old.GioRo = r.GiaMin, r.GiaMax, r.GiaRo, r.MoO, r.GioRo
		c.rows[id] = old
		n++
	}
	return n, nil
}

func (k *KhoNho) Dem(_ context.Context, ten string) (int64, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	c, err := k.col(ten)
	if err != nil {
		return 0, err
	}
	return int64(len(c.rows)), nil
}

func (k *KhoNho) LietKe(_ context.Context, ten string) ([]KhoaHang, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	c, err := k.col(ten)
	if err != nil {
		return nil, err
	}
	out := make([]KhoaHang, 0, len(c.rows))
	for _, r := range c.rows {
		out = append(out, KhoaHang{ChunkID: r.ChunkID, DocID: r.DocID, ContentHash: r.ContentHash, DenseModel: r.DenseModel})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ChunkID < out[j].ChunkID })
	return out, nil
}

func (k *KhoNho) LocHang(_ context.Context, ten string, l Loc) ([]KhoaHang, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	c, err := k.col(ten)
	if err != nil {
		return nil, err
	}
	var out []KhoaHang
	for _, r := range c.rows {
		if l.Khop(r) {
			out = append(out, KhoaHang{ChunkID: r.ChunkID, DocID: r.DocID, ContentHash: r.ContentHash, DenseModel: r.DenseModel})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ChunkID < out[j].ChunkID })
	return out, nil
}

func (k *KhoNho) DatAlias(_ context.Context, alias, ten string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if _, ok := k.cols[ten]; !ok {
		return fmt.Errorf("nap: collection %s does not exist", ten)
	}
	k.aliases[alias] = ten
	return nil
}

func (k *KhoNho) MoTaAlias(_ context.Context, alias string) (string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.aliases[alias], nil
}

// Tokens is the in-memory stand-in for the folding analyzer (standard
// tokenizer, lowercase, asciifolding): fold marks and đ, then split on
// anything that is not a letter or a digit.
func Tokens(text string) []string {
	return strings.FieldsFunc(promptsafety.Fold(text), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}

// TokensCoDau is the in-memory stand-in for the marked analyzer (standard
// tokenizer, lowercase): diacritics kept.
func TokensCoDau(text string) []string {
	return strings.FieldsFunc(strings.ToLower(nfc(text)), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}

func (k *KhoNho) TimLai(_ context.Context, ten string, tv TruyVan) ([]Trung, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	c, err := k.col(ten)
	if err != nil {
		return nil, err
	}
	var ids []string
	for id, r := range c.rows {
		if tv.Loc.Khop(r) {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	per := tv.KMoiNhanh
	if per <= 0 {
		per = 50
	}
	w := tv.TrongSo
	var legs [][]string
	var ws []float64
	if w.Dense > 0 && len(tv.Dense) > 0 {
		legs = append(legs, topK(ids, per, func(id string) float64 { return cosine(c.rows[id].Dense, tv.Dense) }))
		ws = append(ws, w.Dense)
	}
	if c.ld.CheDo == ThuaMILCO {
		if w.MILCO > 0 {
			legs = append(legs, topK(ids, per, func(id string) float64 {
				r := c.rows[id]
				return dotThua(VectorThua{Idx: r.SparseIdx, Val: r.SparseVal}, tv.Thua)
			}))
			ws = append(ws, w.MILCO)
		}
	} else if tv.Chu != "" {
		if w.BM25 > 0 {
			score := bm25(c, TokensCoDau(tv.Chu), TokensCoDau)
			legs = append(legs, topK(ids, per, func(id string) float64 { return score[id] }))
			ws = append(ws, w.BM25)
		}
		if w.BM25KhongDau > 0 {
			score := bm25(c, Tokens(tv.Chu), Tokens)
			legs = append(legs, topK(ids, per, func(id string) float64 { return score[id] }))
			ws = append(ws, w.BM25KhongDau)
		}
	}
	rrfK := tv.RRFK
	if rrfK <= 0 {
		rrfK = 60
	}
	fused := map[string]float64{}
	for i, leg := range legs {
		for rank, id := range leg {
			fused[id] += ws[i] / float64(rrfK+rank+1)
		}
	}
	var hits []Trung
	for id, s := range fused {
		hits = append(hits, Trung{ChunkID: id, DocID: c.rows[id].DocID, Diem: s})
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Diem != hits[j].Diem {
			return hits[i].Diem > hits[j].Diem
		}
		return hits[i].ChunkID < hits[j].ChunkID
	})
	if tv.K > 0 && len(hits) > tv.K {
		hits = hits[:tv.K]
	}
	return hits, nil
}

// topK ranks ids by score (ties by id) and keeps the n best with a positive
// score (a zero BM25 score is no match; a zero cosine is kept only for the
// dense leg, whose scores are never exactly zero in practice).
func topK(ids []string, n int, score func(string) float64) []string {
	type s struct {
		id string
		v  float64
	}
	var all []s
	for _, id := range ids {
		if v := score(id); v > 0 {
			all = append(all, s{id, v})
		}
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].v != all[j].v {
			return all[i].v > all[j].v
		}
		return all[i].id < all[j].id
	})
	if len(all) > n {
		all = all[:n]
	}
	out := make([]string, len(all))
	for i, x := range all {
		out[i] = x.id
	}
	return out
}

func cosine(a, b []float32) float64 {
	var s float64
	for i := range a {
		if i < len(b) {
			s += float64(a[i]) * float64(b[i])
		}
	}
	return s
}

func dotThua(a, b VectorThua) float64 {
	var s float64
	i, j := 0, 0
	for i < len(a.Idx) && j < len(b.Idx) {
		switch {
		case a.Idx[i] == b.Idx[j]:
			s += float64(a.Val[i]) * float64(b.Val[j])
			i++
			j++
		case a.Idx[i] < b.Idx[j]:
			i++
		default:
			j++
		}
	}
	return s
}

// bm25 scores every row of the collection against the query tokens with
// Milvus's default parameters (k1 = 1.2, b = 0.75); statistics are
// collection-wide, as Milvus keeps them.
func bm25(c *colNho, q []string, tok func(string) []string) map[string]float64 {
	const k1, b = 1.2, 0.75
	docs := map[string]map[string]int{}
	lens := map[string]int{}
	df := map[string]int{}
	total := 0
	for id, r := range c.rows {
		tf := map[string]int{}
		toks := tok(r.Text)
		for _, t := range toks {
			tf[t]++
		}
		for t := range tf {
			df[t]++
		}
		docs[id], lens[id] = tf, len(toks)
		total += len(toks)
	}
	n := float64(len(c.rows))
	avg := float64(total) / math.Max(n, 1)
	out := map[string]float64{}
	for id, tf := range docs {
		var s float64
		for _, t := range q {
			f := float64(tf[t])
			if f == 0 {
				continue
			}
			idf := math.Log(1 + (n-float64(df[t])+0.5)/(float64(df[t])+0.5))
			s += idf * f * (k1 + 1) / (f + k1*(1-b+b*float64(lens[id])/avg))
		}
		if s > 0 {
			out[id] = s
		}
	}
	return out
}

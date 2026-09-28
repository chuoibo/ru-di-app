package vectordb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/milvus-io/milvus-proto/go-api/v3/milvuspb"
	"github.com/milvus-io/milvus/client/v3/column"
	"github.com/milvus-io/milvus/client/v3/entity"
	"github.com/milvus-io/milvus/client/v3/index"
	"github.com/milvus-io/milvus/client/v3/milvusclient"

	"mobile/services/core/internal/aiharness/nhung"
	"mobile/services/core/internal/aiharness/truyhoi"
)

// RRFK is the reciprocal-rank-fusion constant, set explicitly: Milvus does
// not document a default (research milvus.md §Kiểm chứng 11).
const RRFK = 60

// Milvus is a connected client. TienTo prefixes every alias and physical
// name (tests only, so parallel runs on one server stay apart); NhatQuan is
// the read consistency (Bounded by default; tests asserting on fresh writes
// use Strong, infra bring-up finding 4). It is also the consistency a new
// collection is created with, because a hybrid search's sub-requests read
// at the collection's default whatever the hybrid request asks (measured:
// a Strong hybrid request on a Bounded collection returned nothing for rows
// written a moment before).
type Milvus struct {
	cli      *milvusclient.Client
	TienTo   string
	NhatQuan entity.ConsistencyLevel
	dense    ChiMucDense
}

// chiMucDense is the dense index this connection builds and searches; the
// zero value is HNSW.
func (m *Milvus) chiMucDense() ChiMucDense {
	if m.dense == "" {
		return DenseHNSW
	}
	return m.dense
}

// Dong closes the connection.
func (m *Milvus) Dong(ctx context.Context) error { return m.cli.Close(ctx) }

func (m *Milvus) nhatQuan() entity.ConsistencyLevel {
	if m.NhatQuan == 0 {
		return entity.ClBounded
	}
	return m.NhatQuan
}

// Alias is k's alias under the prefix.
func (m *Milvus) Alias(k Kho) string { return m.TienTo + k.Alias() }

// TenVatLy is version v of k under the prefix.
func (m *Milvus) TenVatLy(k Kho, v int64) string { return TenVatLy(m.TienTo, k, v) }

// ErrBongAlias: a physical collection carries an alias's name.
var ErrBongAlias = errors.New("vectordb: a collection is named like an alias and would shadow it")

// KiemBongAlias lists the physical collections and refuses any named like
// one of the aliases (under the prefix).
func (m *Milvus) KiemBongAlias(ctx context.Context) error {
	names, err := m.cli.ListCollections(ctx, milvusclient.NewListCollectionOption())
	if err != nil {
		return err
	}
	for _, k := range Khos {
		if slices.Contains(names, m.Alias(k)) {
			return fmt.Errorf("%w: %s", ErrBongAlias, m.Alias(k))
		}
	}
	return nil
}

// TaoPhienBan creates version v of k with its schema and indexes, and loads
// it. The name is always «alias__vN»; it never equals an alias.
func (m *Milvus) TaoPhienBan(ctx context.Context, k Kho, v int64) (string, error) {
	name := m.TenVatLy(k, v)
	ld, err := LuocDoCua(k, name)
	if err != nil {
		return "", err
	}
	return name, m.tao(ctx, ld, laTriNho(k), name)
}

// TaoCollection creates the public collection name (under the prefix) with
// k's schema (places or manual), creates every index and awaits each, and
// loads it. The SDK's CreateCollection with index options returns nil
// when an index build fails (ingest bring-up finding), so indexes are never
// passed that way. A physical name always holds «__» (alias__vN, or the
// ingest's rd_eval__<fingerprint>) and no alias does, so a collection can
// never shadow an alias; an existing collection is an error, since a
// version is immutable.
func (m *Milvus) TaoCollection(ctx context.Context, k Kho, name string) error {
	ld, err := luocDoCongKhai(k, name)
	if err != nil {
		return err
	}
	return m.tao(ctx, ld, false, name)
}

func (m *Milvus) tao(ctx context.Context, ld LuocDo, phanVung bool, name string) error {
	if !strings.Contains(strings.TrimPrefix(name, m.TienTo), "__") {
		return fmt.Errorf("%w: %s", ErrBongAlias, name)
	}
	if !strings.HasPrefix(name, m.TienTo) {
		return fmt.Errorf("%w: %s is outside the prefix", ErrTen, name)
	}
	has, err := m.cli.HasCollection(ctx, milvusclient.NewHasCollectionOption(name))
	if err != nil {
		return err
	}
	if has {
		return fmt.Errorf("vectordb: collection %s exists; versions are immutable", name)
	}
	opt := milvusclient.NewCreateCollectionOption(name, ld.Schema).WithConsistencyLevel(m.nhatQuan())
	if phanVung {
		opt = opt.WithNumPartitions(numPartitions)
	}
	if err := m.cli.CreateCollection(ctx, opt); err != nil {
		return err
	}
	// Every index is asked for first and then each is awaited, its error
	// checked: the server builds them together, and none is taken on trust.
	var tasks []*milvusclient.CreateIndexTask
	for _, io := range append([]milvusclient.CreateIndexOption{m.chiMucDense().denseIndex(name)}, ld.Index...) {
		task, err := m.cli.CreateIndex(ctx, io)
		if err != nil {
			return err
		}
		tasks = append(tasks, task)
	}
	for _, task := range tasks {
		if err := task.Await(ctx); err != nil {
			return err
		}
	}
	task, err := m.cli.LoadCollection(ctx, milvusclient.NewLoadCollectionOption(name))
	if err != nil {
		return err
	}
	return task.Await(ctx)
}

// NangCap points k's alias at version v (creating the alias the first time)
// and reads it back. Rollback is NangCap to the previous version: no rebuild.
func (m *Milvus) NangCap(ctx context.Context, k Kho, v int64) error {
	if err := m.KiemBongAlias(ctx); err != nil {
		return err
	}
	return m.DatAliasTen(ctx, m.Alias(k), m.TenVatLy(k, v))
}

// DatAliasTen points alias at the collection name (both already under the
// prefix), creating the alias the first time, and reads it back. It refuses
// while a physical collection carries the alias's own name or a public
// alias's name (Milvus resolves a name as a collection first, so that alias
// would silently stop moving; a shadowed public index stops every public
// move, so the problem is seen at once). The memory alias is checked by the
// memory store's own moves (NangCap), not here: the ingestion path never
// names a memory store.
func (m *Milvus) DatAliasTen(ctx context.Context, alias, name string) error {
	names, err := m.cli.ListCollections(ctx, milvusclient.NewListCollectionOption())
	if err != nil {
		return err
	}
	for _, a := range []string{alias, m.TienTo + AliasDiaDiem, m.TienTo + AliasHuongDan} {
		if slices.Contains(names, a) {
			return fmt.Errorf("%w: %s", ErrBongAlias, a)
		}
	}
	cur, err := m.MoTaAliasTen(ctx, alias)
	if err != nil {
		return err
	}
	switch {
	case cur == name:
	case cur == "":
		if err := m.cli.CreateAlias(ctx, milvusclient.NewCreateAliasOption(name, alias)); err != nil {
			return fmt.Errorf("vectordb: alias %s → %s: %w", alias, name, err)
		}
	default:
		if err := m.cli.AlterAlias(ctx, milvusclient.NewAlterAliasOption(alias, name)); err != nil {
			return fmt.Errorf("vectordb: alias %s → %s: %w", alias, name, err)
		}
	}
	got, err := m.MoTaAliasTen(ctx, alias)
	if err != nil {
		return err
	}
	if got != name {
		return fmt.Errorf("vectordb: alias %s reads %s after pointing it at %s", alias, got, name)
	}
	return nil
}

// DangPhucVu is the physical collection k's alias points at.
func (m *Milvus) DangPhucVu(ctx context.Context, k Kho) (string, error) {
	a, err := m.cli.DescribeAlias(ctx, milvusclient.NewDescribeAliasOption(m.Alias(k)))
	if err != nil {
		return "", err
	}
	return a.CollectionName, nil
}

// MoTaAliasTen is the collection alias points at, "" when no collection
// carries it. Milvus reports a missing alias as an error; every
// collection's aliases are listed to tell that from a failure.
func (m *Milvus) MoTaAliasTen(ctx context.Context, alias string) (string, error) {
	a, err := m.cli.DescribeAlias(ctx, milvusclient.NewDescribeAliasOption(alias))
	if err == nil {
		return a.CollectionName, nil
	}
	cols, lerr := m.cli.ListCollections(ctx, milvusclient.NewListCollectionOption())
	if lerr != nil {
		return "", err
	}
	for _, c := range cols {
		as, aerr := m.cli.ListAliases(ctx, milvusclient.NewListAliasesOption(c))
		if aerr != nil {
			return "", err
		}
		if slices.Contains(as, alias) {
			return "", err // it exists: the describe error was real
		}
	}
	return "", nil
}

// DanhSachTen lists the collections under the prefix, prefix included.
func (m *Milvus) DanhSachTen(ctx context.Context) ([]string, error) {
	names, err := m.cli.ListCollections(ctx, milvusclient.NewListCollectionOption())
	if err != nil {
		return nil, err
	}
	var out []string
	for _, n := range names {
		if strings.HasPrefix(n, m.TienTo) {
			out = append(out, n)
		}
	}
	slices.Sort(out)
	return out, nil
}

// XoaTen drops the collection name (under the prefix), refusing one an
// alias serves; a missing collection is no error.
func (m *Milvus) XoaTen(ctx context.Context, name string) error {
	if !strings.HasPrefix(name, m.TienTo) {
		return fmt.Errorf("%w: %s is outside the prefix", ErrTen, name)
	}
	has, err := m.cli.HasCollection(ctx, milvusclient.NewHasCollectionOption(name))
	if err != nil || !has {
		return err
	}
	as, err := m.cli.ListAliases(ctx, milvusclient.NewListAliasesOption(name))
	if err != nil {
		return err
	}
	if len(as) > 0 {
		return fmt.Errorf("vectordb: %s is served by %v", name, as)
	}
	return m.cli.DropCollection(ctx, milvusclient.NewDropCollectionOption(name))
}

// XoaPhienBan drops version v of k, refusing the one the alias serves.
func (m *Milvus) XoaPhienBan(ctx context.Context, k Kho, v int64) error {
	name := m.TenVatLy(k, v)
	if cur, err := m.DangPhucVu(ctx, k); err == nil && cur == name {
		return fmt.Errorf("vectordb: %s is being served by %s", name, m.Alias(k))
	}
	return m.cli.DropCollection(ctx, milvusclient.NewDropCollectionOption(name))
}

// XoaAlias drops k's alias (tests' cleanup).
func (m *Milvus) XoaAlias(ctx context.Context, k Kho) error {
	return m.cli.DropAlias(ctx, milvusclient.NewDropAliasOption(m.Alias(k)))
}

// HangDiaDiem is one place chunk as indexed. DocID is the place; a place
// indexed as a single chunk may leave it empty, and then ID is the place.
type HangDiaDiem struct {
	ID      string
	DocID   string
	Facet   string
	ChunkSo int16
	// MoRong is the row's JSON dict of later fields; nil is written {}.
	MoRong      []byte
	Dense       []float32
	Sparse      ThuaVec
	Text        string
	ContentHash string
	EmbedModel  string
	ThuocTinh   ThuocTinh
	GiaMaxVND   int64
	PhienBan    int64
}

// Doc is the place the row is a chunk of.
func (r HangDiaDiem) Doc() string {
	if r.DocID == "" {
		return r.ID
	}
	return r.DocID
}

func (s ThuaVec) embedding() (entity.SparseEmbedding, error) {
	if err := s.Kiem(); err != nil {
		return nil, err
	}
	return entity.NewSliceSparseEmbedding(slices.Clone(s.Chi), slices.Clone(s.GiaTri))
}

func kiemDense(v []float32) error {
	if len(v) != nhung.Dims {
		return fmt.Errorf("%w: %d", nhung.ErrSaiChieu, len(v))
	}
	return nil
}

func notNil[T any](xs []T) []T {
	if xs == nil {
		return []T{}
	}
	return slices.Clone(xs)
}

// GhiDiaDiem upserts place rows into the collection (or alias) name. Text is
// stored NFC, once in each analyzer's field, so both fold the way they fold
// a query.
func (m *Milvus) GhiDiaDiem(ctx context.Context, name string, rows []HangDiaDiem) error {
	if len(rows) == 0 {
		return nil
	}
	n := len(rows)
	ids, docs, texts, dests := make([]string, n), make([]string, n), make([]string, n), make([]string, n)
	hashes, models := make([]string, n), make([]string, n)
	dense := make([][]float32, n)
	sparse := make([]entity.SparseEmbedding, n)
	slots := make([][]int16, n)
	pmin, pmax, ver := make([]int64, n), make([]int64, n), make([]int64, n)
	alg, diet := make([][]string, n), make([][]string, n)
	tomb := make([]bool, n)
	facets, chunkSo, moRong := make([]string, n), make([]int16, n), make([][]byte, n)
	for i, r := range rows {
		facets[i], chunkSo[i], moRong[i] = r.Facet, r.ChunkSo, r.MoRong
		if len(moRong[i]) == 0 {
			moRong[i] = []byte("{}")
		}
		if !json.Valid(moRong[i]) {
			return fmt.Errorf("vectordb: %s of %s is not JSON", FMoRong, r.ID)
		}
		if err := kiemDense(r.Dense); err != nil {
			return err
		}
		se, err := r.Sparse.embedding()
		if err != nil {
			return err
		}
		t := r.ThuocTinh
		ids[i], docs[i], texts[i], dests[i] = r.ID, r.Doc(), nhung.ChuanNFC(r.Text), t.DiemDen
		hashes[i], models[i] = r.ContentHash, r.EmbedModel
		dense[i], sparse[i] = r.Dense, se
		slots[i] = notNil(t.OSlots)
		pmin[i], pmax[i], ver[i] = t.GiaMinVND, r.GiaMaxVND, r.PhienBan
		alg[i], diet[i] = notNil(t.DiUng), notNil(t.AnKieng)
		tomb[i] = t.GoBo
	}
	opt := milvusclient.NewColumnBasedInsertOption(name).
		WithVarcharColumn(FID, ids).
		WithVarcharColumn(FDocID, docs).
		WithFloatVectorColumn(FDense, nhung.Dims, dense).
		WithColumns(column.NewColumnSparseVectors(FSparse, sparse)).
		WithVarcharColumn(FText, texts).
		WithVarcharColumn(FTextKhongDau, texts).
		WithVarcharColumn(FContentHash, hashes).
		WithVarcharColumn(FEmbedModel, models).
		WithVarcharColumn(FFacet, facets).
		WithColumns(column.NewColumnInt16(FChunkSo, chunkSo), column.NewColumnJSONBytes(FMoRong, moRong)).
		WithVarcharColumn(FDestination, dests).
		WithColumns(column.NewColumnInt16Array(FOpenSlots, slots)).
		WithInt64Column(FPriceMin, pmin).
		WithInt64Column(FPriceMax, pmax).
		WithColumns(column.NewColumnVarCharArray(FAllergens, alg), column.NewColumnVarCharArray(FDiets, diet)).
		WithBoolColumn(FTombstoned, tomb).
		WithInt64Column(FIndexVersion, ver)
	_, err := m.cli.Upsert(ctx, opt)
	return err
}

// HangHuongDan is one manual chunk as indexed.
type HangHuongDan struct {
	ID          string
	DocID       string
	Text        string
	ContentHash string
	EmbedModel  string
	Dense       []float32
	Sparse      ThuaVec
	GoBo        bool
	PhienBan    int64
}

// GhiHuongDan upserts manual chunks.
func (m *Milvus) GhiHuongDan(ctx context.Context, name string, rows []HangHuongDan) error {
	if len(rows) == 0 {
		return nil
	}
	n := len(rows)
	ids, docs, texts := make([]string, n), make([]string, n), make([]string, n)
	hashes, models := make([]string, n), make([]string, n)
	dense := make([][]float32, n)
	sparse := make([]entity.SparseEmbedding, n)
	tomb, ver := make([]bool, n), make([]int64, n)
	for i, r := range rows {
		if err := kiemDense(r.Dense); err != nil {
			return err
		}
		se, err := r.Sparse.embedding()
		if err != nil {
			return err
		}
		ids[i], docs[i], texts[i] = r.ID, r.DocID, nhung.ChuanNFC(r.Text)
		hashes[i], models[i] = r.ContentHash, r.EmbedModel
		dense[i], sparse[i], tomb[i], ver[i] = r.Dense, se, r.GoBo, r.PhienBan
	}
	_, err := m.cli.Upsert(ctx, milvusclient.NewColumnBasedInsertOption(name).
		WithVarcharColumn(FID, ids).WithVarcharColumn(FDocID, docs).
		WithVarcharColumn(FText, texts).WithVarcharColumn(FTextKhongDau, texts).
		WithVarcharColumn(FContentHash, hashes).WithVarcharColumn(FEmbedModel, models).
		WithFloatVectorColumn(FDense, nhung.Dims, dense).
		WithColumns(column.NewColumnSparseVectors(FSparse, sparse)).
		WithBoolColumn(FTombstoned, tomb).WithInt64Column(FIndexVersion, ver))
	return err
}

// TrongSo are the fusion weights of the legs: each leg's reciprocal-rank
// contribution w/(RRFK+rank) is multiplied by its weight, and a leg with
// weight 0 is not searched. Configurable (the ingest's committed
// configuration holds the served values, rag/nap cauhinh.json «hop»), so the
// eval gate measures the weights retrieval serves with.
type TrongSo struct {
	Dense        float64
	BM25         float64
	BM25KhongDau float64
	MILCO        float64
}

// MacDinhTrongSo weighs every leg alike: plain RRF.
var MacDinhTrongSo = TrongSo{Dense: 1, BM25: 1, BM25KhongDau: 1, MILCO: 1}

// Kiem refuses a negative or non-finite weight, or no leg at all.
func (w TrongSo) Kiem() error {
	sum := 0.0
	for _, x := range []float64{w.Dense, w.BM25, w.BM25KhongDau, w.MILCO} {
		if !(x >= 0) || math.IsInf(x, 0) {
			return fmt.Errorf("vectordb: fusion weight %v", x)
		}
		sum += x
	}
	if sum == 0 {
		return errors.New("vectordb: every fusion weight is zero")
	}
	return nil
}

// YeuCauTim is one search of a public collection.
type YeuCauTim struct {
	// Ten is the alias or physical collection to search.
	Ten string
	Kho Kho
	// Dense is the query vector; nil when the dense leg is down.
	Dense []float32
	// Thua is the sparse leg; nil when it is down. A BM25 leg searches both
	// BM25 fields (with and without diacritics), each under its weight.
	Thua *ThuaTruyVan
	// Loc: the hard constraints (places only).
	Loc LocCung
	// K is how many fused hits to return; UngVien how many each leg brings
	// to the fusion (default 3K, at least 30).
	K       int
	UngVien int
	// TrongSo are the fusion weights; nil is MacDinhTrongSo.
	TrongSo *TrongSo
}

// Trung is one hit: the chunk id, the document (place or manual section)
// it belongs to, its fused score (weighted RRF) and the index version stored
// on the row.
type Trung struct {
	ID       string
	DocID    string
	Diem     float64
	PhienBan int64
}

// TimKiem is a searchable public index: Milvus, or the Fake in unit tests.
type TimKiem interface {
	Tim(ctx context.Context, y YeuCauTim) ([]Trung, error)
}

// ErrKhongCoNhanh: a search with neither leg.
var ErrKhongCoNhanh = errors.New("vectordb: a search needs a dense or a sparse leg")

func (y YeuCauTim) boLoc() (string, map[string]any, error) {
	switch y.Kho {
	case KhoDiaDiem:
		e, p := y.Loc.BieuThuc()
		return e, p, nil
	case KhoHuongDan:
		return FTombstoned + " == {tb}", map[string]any{"tb": false}, nil
	}
	return "", nil, fmt.Errorf("vectordb: %q is not a public collection", y.Kho)
}

func (y YeuCauTim) ungVien() int {
	if y.UngVien > 0 {
		return y.UngVien
	}
	return max(3*y.K, 30)
}

func (y YeuCauTim) trongSo() TrongSo {
	if y.TrongSo == nil {
		return MacDinhTrongSo
	}
	return *y.TrongSo
}

// Nhanh is one leg of a search: the field, the query vector, its ANN
// parameter (nil for sparse legs) and its fusion weight.
type Nhanh struct {
	Truong  string
	vec     entity.Vector
	ann     index.AnnParam
	TrongSo float64
}

// Nhanhs are the legs y searches, in a fixed order (dense, BM25 with
// diacritics, BM25 folded, MILCO), each with a positive weight.
func (y YeuCauTim) Nhanhs() ([]Nhanh, error) {
	w := y.trongSo()
	if err := w.Kiem(); err != nil {
		return nil, err
	}
	var out []Nhanh
	if y.Dense != nil && w.Dense > 0 {
		if err := kiemDense(y.Dense); err != nil {
			return nil, err
		}
		out = append(out, Nhanh{FDense, entity.FloatVector(y.Dense), index.NewHNSWAnnParam(HNSWEfTimKiem), w.Dense})
	}
	if y.Thua != nil {
		switch y.Thua.Loai {
		case ThuaBM25:
			if w.BM25 > 0 {
				out = append(out, Nhanh{FBM25, entity.Text(y.Thua.TextCua(FBM25)), nil, w.BM25})
			}
			if w.BM25KhongDau > 0 {
				out = append(out, Nhanh{FBM25KhongDau, entity.Text(y.Thua.TextCua(FBM25KhongDau)), nil, w.BM25KhongDau})
			}
		case ThuaMILCO:
			se, err := y.Thua.Vec.embedding()
			if err != nil {
				return nil, err
			}
			if w.MILCO > 0 {
				out = append(out, Nhanh{FSparse, se, nil, w.MILCO})
			}
		default:
			return nil, fmt.Errorf("vectordb: sparse leg %q", y.Thua.Loai)
		}
	}
	if len(out) == 0 {
		return nil, ErrKhongCoNhanh
	}
	return out, nil
}

// HopRRF fuses ranked legs by weighted reciprocal rank: a hit at 0-based
// rank r of leg i adds w[i]/(k+r+1). Ties are broken by id, so the order is
// deterministic; the first K are kept (k <= 0 keeps all). The same function
// fuses Milvus's legs and the fakes' legs.
func HopRRF(legs [][]Trung, w []float64, k, K int) []Trung {
	fused := map[string]Trung{}
	for i, leg := range legs {
		for r, h := range leg {
			f, ok := fused[h.ID]
			if !ok {
				f = Trung{ID: h.ID, DocID: h.DocID, PhienBan: h.PhienBan}
			}
			f.Diem += w[i] / float64(k+r+1)
			fused[h.ID] = f
		}
	}
	out := make([]Trung, 0, len(fused))
	for _, f := range fused {
		out = append(out, f)
	}
	slices.SortFunc(out, func(a, b Trung) int {
		if a.Diem != b.Diem {
			if a.Diem > b.Diem {
				return -1
			}
			return 1
		}
		return strings.Compare(a.ID, b.ID)
	})
	if K > 0 && len(out) > K {
		out = out[:K]
	}
	return out
}

// Tim runs the hybrid search: every leg (dense COSINE under the deployment's
// index, BM25 on the
// marked text, BM25 on the folded text, or MILCO), each under the same
// hard-constraint filter, searched in parallel, then fused by weighted RRF
// with k=RRFK in Go (Milvus's own RRF ranker takes no weights). With a leg
// down the others answer, with the same filter.
func (m *Milvus) Tim(ctx context.Context, y YeuCauTim) ([]Trung, error) {
	if y.K <= 0 || y.K > truyhoi.MaxK {
		return nil, fmt.Errorf("vectordb: k %d", y.K)
	}
	expr, params, err := y.boLoc()
	if err != nil {
		return nil, err
	}
	legs, err := y.Nhanhs()
	if err != nil {
		return nil, err
	}
	res := make([][]Trung, len(legs))
	errs := make([]error, len(legs))
	var wg sync.WaitGroup
	for i, l := range legs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			opt := milvusclient.NewSearchOption(y.Ten, y.ungVien(), []entity.Vector{l.vec}).WithANNSField(l.Truong).
				WithConsistencyLevel(m.nhatQuan()).WithOutputFields(FIndexVersion, FDocID).WithFilter(expr)
			if l.Truong == FDense {
				opt = opt.WithAnnParam(m.chiMucDense().thamSoTim())
			} else if l.ann != nil {
				opt = opt.WithAnnParam(l.ann)
			}
			for k, v := range params {
				opt = opt.WithTemplateParam(k, v)
			}
			rs, err := m.cli.Search(ctx, opt)
			if err != nil {
				errs[i] = err
				return
			}
			res[i], errs[i] = trungTu(rs)
		}()
	}
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	w := make([]float64, len(legs))
	for i, l := range legs {
		w[i] = l.TrongSo
	}
	return HopRRF(res, w, RRFK, y.K), nil
}

func trungTu(rs []milvusclient.ResultSet) ([]Trung, error) {
	if len(rs) == 0 {
		return nil, nil
	}
	r := rs[0]
	if r.Err != nil {
		return nil, r.Err
	}
	out := make([]Trung, 0, r.ResultCount)
	ver := r.GetColumn(FIndexVersion)
	doc := r.GetColumn(FDocID)
	for i := 0; i < r.ResultCount; i++ {
		id, err := r.IDs.GetAsString(i)
		if err != nil {
			return nil, err
		}
		t := Trung{ID: id, DocID: id, Diem: float64(r.Scores[i])}
		if ver != nil {
			if v, err := ver.GetAsInt64(i); err == nil {
				t.PhienBan = v
			}
		}
		if doc != nil {
			if d, err := doc.GetAsString(i); err == nil && d != "" {
				t.DocID = d
			}
		}
		out = append(out, t)
	}
	return out, nil
}

// Dem counts the rows of name matching expr (Strong consistency: a count
// is asked after a write or a delete, to check it).
func (m *Milvus) Dem(ctx context.Context, name, expr string, params map[string]any) (int64, error) {
	opt := milvusclient.NewQueryOption(name).WithOutputFields("count(*)").
		WithConsistencyLevel(entity.ClStrong).WithFilter(expr)
	for k, v := range params {
		opt.WithTemplateParam(k, v)
	}
	rs, err := m.cli.Query(ctx, opt)
	if err != nil {
		return 0, err
	}
	c := rs.GetColumn("count(*)")
	if c == nil || c.Len() != 1 {
		return 0, errors.New("vectordb: count(*) answered no single value")
	}
	return c.GetAsInt64(0)
}

// DemBiLoai counts, in l's destination, the live places each other hard
// constraint removes (truyhoi.KetQuaTruyHoi.BiLoai): counts only.
func (m *Milvus) DemBiLoai(ctx context.Context, name string, l LocCung) (map[truyhoi.RangBuoc]int, error) {
	out := map[truyhoi.RangBuoc]int{}
	for rb, q := range l.BieuThucViPham() {
		n, err := m.Dem(ctx, name, q.BieuThuc, q.ThamSo)
		if err != nil {
			return nil, err
		}
		out[rb] = int(n)
	}
	return out, nil
}

// XoaID deletes rows by primary key.
func (m *Milvus) XoaID(ctx context.Context, name string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := m.cli.Delete(ctx, milvusclient.NewDeleteOption(name).WithStringIDs(FID, ids))
	return err
}

// compactOption asks for an L0 (delta) compaction or a mix compaction. The
// SDK's option has no L0 flag; the request does.
type compactOption struct {
	name string
	l0   bool
}

func (o compactOption) Request() *milvuspb.ManualCompactionRequest {
	return &milvuspb.ManualCompactionRequest{CollectionName: o.name, L0Compaction: o.l0}
}

// NenVaCho makes deletes physical in name's segments: an L0 compaction first
// (deletes live in delta segments; a mix compaction before it rewrites
// nothing — infra bring-up finding 1), waited to Completed, then a mix
// compaction, waited too. Old segment files are removed later by the
// server's GC (dataCoord.gc.dropTolerance); a deletion receipt says
// «compacted», not «bytes gone».
func (m *Milvus) NenVaCho(ctx context.Context, name string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	// Seal the growing segments first: a delete still in a growing delta
	// is not an L0 segment a compaction can take.
	if err := m.Flush(ctx, name); err != nil {
		return err
	}
	for _, l0 := range []bool{true, false} {
		id, err := m.cli.Compact(ctx, compactOption{name: name, l0: l0})
		if err != nil {
			return err
		}
		if id == 0 {
			continue
		}
		for {
			st, err := m.cli.GetCompactionState(ctx, milvusclient.NewGetCompactionStateOption(id))
			if err != nil {
				return err
			}
			if st == entity.CompactionStateCompleted {
				break
			}
			select {
			case <-ctx.Done():
				return fmt.Errorf("vectordb: compaction of %s: %w", name, ctx.Err())
			case <-time.After(200 * time.Millisecond):
			}
		}
	}
	return nil
}

// SoHangLuuTru is the sum of the rows in name's persistent segments: after
// NenVaCho it counts only rows that were rewritten, so a deleted row still
// on a segment shows here while count(*) already hides it.
func (m *Milvus) SoHangLuuTru(ctx context.Context, name string) (int64, error) {
	segs, err := m.cli.GetPersistentSegmentInfo(ctx, milvusclient.NewGetPersistentSegmentInfoOption(name))
	if err != nil {
		return 0, err
	}
	var n int64
	for _, s := range segs {
		if s.Flushed() {
			n += s.NumRows
		}
	}
	return n, nil
}

// Flush seals name's growing segments (tests measure persistent rows).
// The server rate-limits flush per collection (0.1/s by default); a flush
// refused for that is retried until ctx ends, since this is a maintenance
// path (ingest, deletion saga), never a turn.
func (m *Milvus) Flush(ctx context.Context, name string) error {
	for {
		t, err := m.cli.Flush(ctx, milvusclient.NewFlushOption(name))
		if err == nil {
			return t.Await(ctx)
		}
		if !strings.Contains(err.Error(), "rate limit") {
			return err
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("vectordb: flush of %s: %w (%v)", name, ctx.Err(), err)
		case <-time.After(time.Second):
		}
	}
}

// compactTimeout bounds one NenVaCho of a memory collection.
const compactTimeout = 2 * time.Minute

// DonTienTo drops every alias and collection under m's prefix: a test's
// cleanup. It refuses an empty prefix, which would name production.
func (m *Milvus) DonTienTo(ctx context.Context) error {
	if m.TienTo == "" {
		return errors.New("vectordb: refusing to clean without a prefix")
	}
	names, err := m.cli.ListCollections(ctx, milvusclient.NewListCollectionOption())
	if err != nil {
		return err
	}
	var errs []error
	for _, n := range names {
		if !strings.HasPrefix(n, m.TienTo) {
			continue
		}
		aliases, err := m.cli.ListAliases(ctx, milvusclient.NewListAliasesOption(n))
		if err != nil {
			errs = append(errs, err)
		}
		for _, a := range aliases {
			errs = append(errs, m.cli.DropAlias(ctx, milvusclient.NewDropAliasOption(a)))
		}
		errs = append(errs, m.cli.DropCollection(ctx, milvusclient.NewDropCollectionOption(n)))
	}
	return errors.Join(errs...)
}

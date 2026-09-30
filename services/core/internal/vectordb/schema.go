package vectordb

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/milvus-io/milvus/client/v3/entity"
	"github.com/milvus-io/milvus/client/v3/index"
	"github.com/milvus-io/milvus/client/v3/milvusclient"

	"mobile/services/core/internal/aiharness/nhung"
)

// Kho is one logical collection: what an alias serves.
type Kho string

const (
	// KhoDiaDiem: the place catalogue (public).
	KhoDiaDiem Kho = "places"
	// KhoHuongDan: the app manual (public).
	KhoHuongDan Kho = "manual"
	// KhoTriNho: Nếp's per-person memory vectors (private, no text).
	KhoTriNho Kho = "memories"
)

// Khos is every logical collection, for the alias-shadowing check.
var Khos = []Kho{KhoDiaDiem, KhoHuongDan, KhoTriNho}

// The public aliases: what the ingest writes and retrieval reads. The
// memory alias is named only by the memory store's own code (tri_nho.go),
// so the ingestion path, which reaches this file, never names a memory
// store (aigate).
const (
	AliasDiaDiem  = "rd_places"
	AliasHuongDan = "rd_manual"
)

// Alias is the name the application reads and writes a Kho through. The
// memory alias carries the nep_ prefix of every private Nếp store, so a
// grant or a backup rule written by prefix never mixes it with the public
// indexes.
func (k Kho) Alias() string {
	switch k {
	case KhoDiaDiem:
		return AliasDiaDiem
	case KhoHuongDan:
		return AliasHuongDan
	}
	return aliasTriNho(k)
}

// ErrTen: a collection name outside the scheme, or one that shadows an alias.
var ErrTen = fmt.Errorf("vectordb: collection name outside the alias__vN scheme")

// TenVatLy is the physical collection of version v of k: «alias__vN». With
// tienTo (tests only) the alias is prefixed, so parallel test runs on one
// server never touch each other's collections.
func TenVatLy(tienTo string, k Kho, v int64) string {
	return tienTo + k.Alias() + "__v" + strconv.FormatInt(v, 10)
}

// PhanTichTen reads «[prefix]alias__vN» back into its Kho and version.
func PhanTichTen(tienTo, ten string) (Kho, int64, error) {
	rest, ok := strings.CutPrefix(ten, tienTo)
	if !ok {
		return "", 0, ErrTen
	}
	alias, v, ok := strings.Cut(rest, "__v")
	if !ok {
		return "", 0, ErrTen
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n <= 0 || strconv.FormatInt(n, 10) != v {
		return "", 0, ErrTen
	}
	for _, k := range Khos {
		if k.Alias() == alias {
			return k, n, nil
		}
	}
	return "", 0, ErrTen
}

// Field names. Shared by the schemas, the filters, the writers and the fake.
const (
	FID    = "id"
	FDense = "dense"
	// FText is the row's text under the folding analyzer (AnalyzerParams),
	// and FSparse the output of Milvus's BM25 function over it: the one
	// sparse leg (rd.v4, owner 2026-09-29). No client ever writes FSparse.
	FText        = "text"
	FSparse      = "sparse"
	FDestination = "destination"
	FOpenSlots   = "open_slots"
	FPriceMin    = "price_min"
	FPriceMax    = "price_max"
	FAllergens   = "allergens"
	FDiets       = "diets"
	// FDanhMuc are the place's categories: every tuvung.DanhMuc id the
	// ingest's classification pass found the place fits, or exactly
	// [KhongRo] when it has none.
	FDanhMuc      = "danh_muc"
	FTombstoned   = "tombstoned"
	FIndexVersion = "index_version"
	FEmbedModel   = "embed_model"
	// FContentHash is the row's content hash (the ingest's reconciliation
	// reads it back).
	FContentHash = "content_hash"
	// FDocID is the document a row belongs to: for a place it equals FID
	// (one row per place, rd.v4), for a manual row the section id. Retrieval
	// answers documents, grouping hits by it.
	FDocID = "doc_id"
	// FMoRong is a JSON dict for fields that arrive later, upserted on its
	// own (CapNhatMoRong) without a schema revision. Nullable; written {}.
	FMoRong    = "mo_rong"
	FOwner     = "owner_id"
	FKind      = "kind"
	FCreatedAt = "created_at"
)

// PhienBanLuocDo names this file's place and manual schema (fields,
// analyzers, index parameters). The ingest's committed configuration names
// the revision it was built for, and the ingest adapter refuses to create a
// collection when the two differ: one schema, declared once, here.
//
// rd.v4 (owner, 2026-09-29): one row and one dense vector per place, no
// chunking; one text field under the folding analyzer and one BM25 sparse
// field over it; no learned-sparse field; the place's categories (FDanhMuc)
// as a filterable field.
const PhienBanLuocDo = "rd.v4"

// Bounds of the scalar fields. MaxTextLen is Milvus's VarChar max_length,
// which counts BYTES: Vietnamese NFC text takes two to three bytes a
// character, and the longest place text of the feed is 5,612 characters. A
// longer text is refused by the ingest, never cut.
const (
	MaxIDLen       = 128
	MaxTextLen     = 32768
	MaxTagLen      = 32
	MaxAllergens   = 32
	MaxDiets       = 16
	MaxDanhMuc     = 10
	SoSlotTuan     = 7 * 48
	PhutMoiSlot    = 30
	MaxOwnerLen    = 64
	numPartitions  = 16
	hnswM          = 16
	hnswEfBuild    = 200
	HNSWEfTimKiem  = 128
	GiaKhongRo     = int64(-1)
	KhongRo        = "khong_ro"
	bm25FuncName   = "text_bm25"
	embedModelDesc = nhung.Model + "@" + "1536/" + nhung.PromptVersion
)

// AnalyzerParams is the BM25 analyzer of FText: the standard tokenizer,
// lowercase and ASCII folding on NFC text (the writer normalises), so
// «Quán Cà Phê ở Đà Lạt» and «quan ca phe o da lat» give the same tokens
// (measured on the local server). This is structural normalisation of the
// characters -- marks and đ folded, the way a person types without an IME
// -- not a reading of what the words mean, and it decides nothing about the
// question. The diacritics-keeping field of rd.v3 served at weight 0.1 of
// this one's and is gone.
func AnalyzerParams() map[string]any {
	return map[string]any{
		"tokenizer": "standard",
		"filter":    []any{"lowercase", "asciifolding"},
	}
}

// LuocDo is a collection's schema and index options, built as code so a test
// can read them and a version is reproducible.
type LuocDo struct {
	Schema *entity.Schema
	Index  []milvusclient.CreateIndexOption
}

func pk() *entity.Field {
	return entity.NewField().WithName(FID).WithDataType(entity.FieldTypeVarChar).
		WithMaxLength(MaxIDLen).WithIsPrimaryKey(true).WithIsAutoID(false)
}

func denseField() *entity.Field {
	return entity.NewField().WithName(FDense).WithDataType(entity.FieldTypeFloatVector).WithDim(nhung.Dims)
}

// textFields are the text under the folding analyzer and the one sparse
// field, the BM25 function's output over it.
func textFields(s *entity.Schema) {
	s.WithField(entity.NewField().WithName(FText).WithDataType(entity.FieldTypeVarChar).
		WithMaxLength(MaxTextLen).WithEnableAnalyzer(true).WithAnalyzerParams(AnalyzerParams())).
		WithField(entity.NewField().WithName(FSparse).WithDataType(entity.FieldTypeSparseVector)).
		WithFunction(entity.NewFunction().WithName(bm25FuncName).WithType(entity.FunctionTypeBM25).
			WithInputFields(FText).WithOutputFields(FSparse))
}

// docFields are the fields every public row carries beside its vectors:
// the document it belongs to, its content hash and the embedding model
// that produced its dense vector (the ingest's reconciliation reads them).
func docFields(s *entity.Schema) {
	s.WithField(entity.NewField().WithName(FDocID).WithDataType(entity.FieldTypeVarChar).WithMaxLength(MaxIDLen)).
		WithField(entity.NewField().WithName(FContentHash).WithDataType(entity.FieldTypeVarChar).WithMaxLength(64)).
		WithField(entity.NewField().WithName(FEmbedModel).WithDataType(entity.FieldTypeVarChar).WithMaxLength(64))
}

// ChiMucDense is the dense vector index a deployment builds. HNSW runs on any
// Milvus and is what the CPU image in CI carries; GPU_CAGRA needs the -gpu
// image and a GPU (ADR-0049 §2.6). It is a deployment choice, not a schema
// revision: fields, analyzers and the BM25 leg are identical under both, and a
// process searches with the parameters of the kind it builds.
type ChiMucDense string

const (
	DenseHNSW     ChiMucDense = "HNSW"
	DenseGPUCagra ChiMucDense = "GPU_CAGRA"
)

// CAGRA build and search parameters: graph degree 32 over an intermediate
// graph of 64 (Milvus defaults), and an internal top-k wide enough for the
// widest candidate list a hybrid leg asks for (truyhoi.MaxK × 3, at least 30).
const (
	cagraInterDegree = 64
	cagraDegree      = 32
	cagraITopK       = 256
)

// DocChiMucDense reads EnvDenseIndex: empty means HNSW.
func DocChiMucDense(raw string) (ChiMucDense, error) {
	switch k := ChiMucDense(strings.ToUpper(strings.TrimSpace(raw))); k {
	case "", DenseHNSW:
		return DenseHNSW, nil
	case DenseGPUCagra:
		return k, nil
	}
	return "", fmt.Errorf("vectordb: %s must be HNSW or GPU_CAGRA, got %q", EnvDenseIndex, raw)
}

// chiMuc is the index built on FDense.
func (k ChiMucDense) chiMuc() index.Index {
	if k == DenseGPUCagra {
		return index.NewGPUCagraIndex(entity.COSINE, cagraInterDegree, cagraDegree)
	}
	return index.NewHNSWIndex(entity.COSINE, hnswM, hnswEfBuild)
}

// thamSoTim is the ANN parameter a dense search sends for this index.
func (k ChiMucDense) thamSoTim() index.AnnParam {
	if k == DenseGPUCagra {
		p := index.NewCustomAnnParam()
		p.WithExtraParam("itopk_size", cagraITopK)
		return p
	}
	return index.NewHNSWAnnParam(HNSWEfTimKiem)
}

// denseIndex is the index option on FDense of collection name.
func (k ChiMucDense) denseIndex(name string) milvusclient.CreateIndexOption {
	return milvusclient.NewCreateIndexOption(name, FDense, k.chiMuc())
}

// bm25Index is the index of the BM25 field. The dense index is the
// deployment's (ChiMucDense), added where the collection is created.
func bm25Index(name string) milvusclient.CreateIndexOption {
	return milvusclient.NewCreateIndexOption(name, FSparse, index.NewGenericIndex("bm25_idx", map[string]string{
		"index_type": "SPARSE_INVERTED_INDEX", "metric_type": "BM25"}))
}

func inverted(name string, fields ...string) []milvusclient.CreateIndexOption {
	var out []milvusclient.CreateIndexOption
	for _, f := range fields {
		out = append(out, milvusclient.NewCreateIndexOption(name, f, index.NewInvertedIndex()))
	}
	return out
}

// LuocDoDiaDiem is the place collection, one row per place: the place id
// (FID, and FDocID equal to it), the dense vector, the text under the
// folding analyzer with its BM25 field, the free JSON dict of later fields,
// and the hard-constraint fields the LLM enrichment at ingest filled
// (destination, open slots of the week, price bounds in whole đồng with
// GiaKhongRo for unknown, allergens and diets as closed ids with KhongRo for
// an allergen list nobody could establish, the categories as closed ids
// with KhongRo for unknown), a tombstone and the index version.
func LuocDoDiaDiem(name string) LuocDo {
	s := entity.NewSchema().WithName(name).WithDynamicFieldEnabled(false).
		WithDescription("rd places " + embedModelDesc).
		WithField(pk()).WithField(denseField())
	textFields(s)
	docFields(s)
	s.WithField(entity.NewField().WithName(FMoRong).WithDataType(entity.FieldTypeJSON).WithNullable(true))
	s.WithField(entity.NewField().WithName(FDestination).WithDataType(entity.FieldTypeVarChar).WithMaxLength(64)).
		WithField(entity.NewField().WithName(FOpenSlots).WithDataType(entity.FieldTypeArray).
			WithElementType(entity.FieldTypeInt16).WithMaxCapacity(SoSlotTuan)).
		WithField(entity.NewField().WithName(FPriceMin).WithDataType(entity.FieldTypeInt64)).
		WithField(entity.NewField().WithName(FPriceMax).WithDataType(entity.FieldTypeInt64)).
		WithField(entity.NewField().WithName(FAllergens).WithDataType(entity.FieldTypeArray).
			WithElementType(entity.FieldTypeVarChar).WithMaxCapacity(MaxAllergens).WithMaxLength(MaxTagLen)).
		WithField(entity.NewField().WithName(FDiets).WithDataType(entity.FieldTypeArray).
			WithElementType(entity.FieldTypeVarChar).WithMaxCapacity(MaxDiets).WithMaxLength(MaxTagLen)).
		WithField(entity.NewField().WithName(FDanhMuc).WithDataType(entity.FieldTypeArray).
			WithElementType(entity.FieldTypeVarChar).WithMaxCapacity(MaxDanhMuc).WithMaxLength(MaxTagLen)).
		WithField(entity.NewField().WithName(FTombstoned).WithDataType(entity.FieldTypeBool)).
		WithField(entity.NewField().WithName(FIndexVersion).WithDataType(entity.FieldTypeInt64))
	idx := []milvusclient.CreateIndexOption{bm25Index(name)}
	idx = append(idx, inverted(name, FDocID, FDestination, FOpenSlots, FAllergens, FDiets, FDanhMuc, FTombstoned)...)
	idx = append(idx, milvusclient.NewCreateIndexOption(name, FPriceMin, index.NewSortedIndex()))
	return LuocDo{Schema: s, Index: idx}
}

// LuocDoHuongDan is the app manual: one row per section of public text, no
// constraint fields but the tombstone.
func LuocDoHuongDan(name string) LuocDo {
	s := entity.NewSchema().WithName(name).WithDynamicFieldEnabled(false).
		WithDescription("rd manual " + embedModelDesc).
		WithField(pk()).WithField(denseField())
	textFields(s)
	docFields(s)
	s.WithField(entity.NewField().WithName(FTombstoned).WithDataType(entity.FieldTypeBool)).
		WithField(entity.NewField().WithName(FIndexVersion).WithDataType(entity.FieldTypeInt64))
	idx := []milvusclient.CreateIndexOption{bm25Index(name)}
	idx = append(idx, inverted(name, FDocID, FTombstoned)...)
	return LuocDo{Schema: s, Index: idx}
}

// LuocDoTriNho is the memory collection: the fact id, its owner as the
// partition key (performance; the isolation is the owner filter every call
// carries, tri_nho.go), the closed kind, the time, the dense vector. No
// text, no sparse field, no JSON, no dynamic field: nothing that reads back
// as the fact.
func LuocDoTriNho(name string) LuocDo {
	s := entity.NewSchema().WithName(name).WithDynamicFieldEnabled(false).
		WithDescription("nep memories " + embedModelDesc).
		WithField(pk()).
		WithField(entity.NewField().WithName(FOwner).WithDataType(entity.FieldTypeVarChar).
			WithMaxLength(MaxOwnerLen).WithIsPartitionKey(true)).
		WithField(entity.NewField().WithName(FKind).WithDataType(entity.FieldTypeVarChar).WithMaxLength(MaxTagLen)).
		WithField(entity.NewField().WithName(FCreatedAt).WithDataType(entity.FieldTypeInt64)).
		WithField(denseField()).
		WithField(entity.NewField().WithName(FIndexVersion).WithDataType(entity.FieldTypeInt64))
	return LuocDo{Schema: s, Index: inverted(name, FOwner, FKind)}
}

// LuocDoCua is the schema of a Kho's physical collection.
func LuocDoCua(k Kho, name string) (LuocDo, error) {
	if ld, err := luocDoCongKhai(k, name); err == nil {
		return ld, nil
	}
	if laTriNho(k) {
		return LuocDoTriNho(name), nil
	}
	return LuocDo{}, fmt.Errorf("vectordb: unknown collection kind %q", k)
}

// luocDoCongKhai is the schema of a public collection (places, manual).
func luocDoCongKhai(k Kho, name string) (LuocDo, error) {
	switch k {
	case KhoDiaDiem:
		return LuocDoDiaDiem(name), nil
	case KhoHuongDan:
		return LuocDoHuongDan(name), nil
	}
	return LuocDo{}, fmt.Errorf("vectordb: %q is not a public collection", k)
}

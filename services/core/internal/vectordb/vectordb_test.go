package vectordb

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/milvus-io/milvus/client/v3/entity"

	"mobile/services/core/internal/aiharness/nhung"
	"mobile/services/core/internal/aiharness/truyhoi"
)

// The SDK's telemetry is on when ClientConfig.TelemetryConfig is nil; the
// one config this package builds always carries it, switched off.
func TestClientConfigTurnsTelemetryOff(t *testing.T) {
	cc := clientConfig(Config{Addr: "127.0.0.1:1", User: "u", Password: "p"})
	if cc.TelemetryConfig == nil {
		t.Fatal("TelemetryConfig is nil: the SDK would fall back to its default, which is on")
	}
	if cc.TelemetryConfig.Enabled {
		t.Fatal("telemetry is on")
	}
	if cc.Username != "u" || cc.Password != "p" || cc.Address != "127.0.0.1:1" {
		t.Fatalf("%+v", cc)
	}
}

// Every milvusclient.ClientConfig and every milvusclient.New in core is in
// config.go: a second constructor could forget the telemetry pointer.
func TestOneClientConstructor(t *testing.T) {
	root := filepath.Join("..", "..")
	var where []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == "testdata" || d.Name() == "vendor") {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(f, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if x, ok := sel.X.(*ast.Ident); ok && x.Name == "milvusclient" && (sel.Sel.Name == "ClientConfig" || sel.Sel.Name == "New") {
				where = append(where, filepath.ToSlash(path))
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range where {
		if !strings.HasSuffix(w, "internal/vectordb/config.go") {
			t.Errorf("%s builds a Milvus client outside vectordb/config.go", w)
		}
	}
	if len(where) < 2 {
		t.Fatalf("found %d uses; the scan is broken", len(where))
	}
}

func TestFromEnvNeedsAddressAndCredentials(t *testing.T) {
	env := map[string]string{EnvAddr: "127.0.0.1:19530", EnvUser: "svc", EnvPassword: "pw"}
	c, err := FromEnv(func(k string) string { return env[k] })
	if err != nil || c.Addr != "127.0.0.1:19530" || c.User != "svc" {
		t.Fatalf("%+v %v", c, err)
	}
	delete(env, EnvPassword)
	if _, err := FromEnv(func(k string) string { return env[k] }); !errors.Is(err, ErrChuaCauHinh) {
		t.Fatal("no password accepted: auth must be on")
	}
}

func TestNamesNeverShadowAnAlias(t *testing.T) {
	for _, k := range Khos {
		for _, v := range []int64{1, 7, 123} {
			name := TenVatLy("", k, v)
			for _, other := range Khos {
				if name == other.Alias() {
					t.Fatalf("%s shadows alias %s", name, other.Alias())
				}
			}
			gk, gv, err := PhanTichTen("", name)
			if err != nil || gk != k || gv != v {
				t.Fatalf("PhanTichTen(%s) = %s %d %v", name, gk, gv, err)
			}
		}
	}
	for _, bad := range []string{"rd_places", "rd_places__v0", "rd_places__v01", "x__v1", "rd_places__v-1", "rd_places__vx"} {
		if _, _, err := PhanTichTen("", bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
	if !strings.HasPrefix(KhoTriNho.Alias(), "nep_") {
		t.Fatal("the memory alias lost its private nep_ prefix")
	}
}

func fieldNames(s *entity.Schema) map[string]*entity.Field {
	out := map[string]*entity.Field{}
	for _, f := range s.Fields {
		out[f.Name] = f
	}
	return out
}

// The memory collection holds no text: ids, owner, kind, time, vectors and
// the index version, nothing a reader could turn back into the fact; no
// dynamic field; the owner is the partition key.
func TestMemorySchemaHasNoText(t *testing.T) {
	ld := LuocDoTriNho("nep_memories__v1")
	if ld.Schema.EnableDynamicField {
		t.Fatal("dynamic field on the memory collection")
	}
	fs := fieldNames(ld.Schema)
	allowed := map[string]bool{FID: true, FOwner: true, FKind: true, FCreatedAt: true, FDense: true, FSparse: true, FIndexVersion: true}
	for name, f := range fs {
		if !allowed[name] {
			t.Errorf("memory field %s (%v) is not on the allowlist", name, f.DataType)
		}
		if f.DataType == entity.FieldTypeJSON || f.DataType == entity.FieldTypeText || f.TypeParams["enable_analyzer"] == "true" {
			t.Errorf("memory field %s can hold free text", name)
		}
		if f.DataType == entity.FieldTypeVarChar && f.TypeParams["max_length"] != "" && name != FID && name != FOwner && name != FKind {
			t.Errorf("memory varchar %s", name)
		}
	}
	if len(ld.Schema.Functions) != 0 {
		t.Fatal("a function (BM25 over text) on the memory collection")
	}
	if !fs[FOwner].IsPartitionKey {
		t.Fatal("owner is not the partition key")
	}
}

// rd.v4 (owner, 2026-09-29) is exactly this: one row per place, one dense
// vector, one text under the folding analyzer, one sparse field that is
// Milvus's BM25 over it, the JSON dict of later fields, the hard-constraint
// fields and the categories. Nothing else, so a field the owner removed
// cannot come back unnoticed.
func TestPlaceSchemaIsRdV4(t *testing.T) {
	ld := LuocDoDiaDiem("rd_places__v1")
	fs := fieldNames(ld.Schema)
	want := map[string]entity.FieldType{
		FID: entity.FieldTypeVarChar, FDense: entity.FieldTypeFloatVector,
		FText: entity.FieldTypeVarChar, FSparse: entity.FieldTypeSparseVector,
		FDocID: entity.FieldTypeVarChar, FContentHash: entity.FieldTypeVarChar, FEmbedModel: entity.FieldTypeVarChar,
		FMoRong: entity.FieldTypeJSON, FDestination: entity.FieldTypeVarChar,
		FOpenSlots: entity.FieldTypeArray, FPriceMin: entity.FieldTypeInt64, FPriceMax: entity.FieldTypeInt64,
		FAllergens: entity.FieldTypeArray, FDiets: entity.FieldTypeArray, FDanhMuc: entity.FieldTypeArray,
		FTombstoned: entity.FieldTypeBool, FIndexVersion: entity.FieldTypeInt64,
	}
	for name, typ := range want {
		if fs[name] == nil || fs[name].DataType != typ {
			t.Errorf("field %s missing or not %v", name, typ)
		}
	}
	for name := range fs {
		if _, ok := want[name]; !ok {
			t.Errorf("field %s is not in rd.v4", name)
		}
	}
	if PhienBanLuocDo != "rd.v4" {
		t.Fatalf("schema revision %q", PhienBanLuocDo)
	}
	if !fs[FMoRong].Nullable {
		t.Fatal("mo_rong must be a nullable JSON dict")
	}
	if fs[FText].TypeParams["max_length"] != "32768" {
		t.Fatalf("text max_length %q, want 32768 bytes", fs[FText].TypeParams["max_length"])
	}
	if fs[FDanhMuc].TypeParams["max_capacity"] != "10" {
		t.Fatalf("danh_muc capacity %q, want 10", fs[FDanhMuc].TypeParams["max_capacity"])
	}
	if fs[FDense].TypeParams["dim"] != "3072" {
		t.Fatalf("dense dim %q", fs[FDense].TypeParams["dim"])
	}
	if fs[FID].AutoID || !fs[FID].PrimaryKey {
		t.Fatal("the id must be a deterministic primary key, never autoID (upsert with autoID mints new keys)")
	}
	if ld.Schema.EnableDynamicField || len(ld.Schema.Functions) != 1 {
		t.Fatal("dynamic field on, or not exactly one BM25 function")
	}
	fn := ld.Schema.Functions[0]
	if fn.Type != entity.FunctionTypeBM25 || len(fn.InputFieldNames) != 1 || fn.InputFieldNames[0] != FText ||
		len(fn.OutputFieldNames) != 1 || fn.OutputFieldNames[0] != FSparse {
		t.Fatalf("the BM25 function is not text -> sparse: %+v", fn)
	}
	// The one BM25 field folds diacritics (structural normalisation only).
	if f := fs[FText].TypeParams["analyzer_params"]; !strings.Contains(f, "asciifolding") || !strings.Contains(f, "lowercase") {
		t.Fatalf("the text does not fold: %s", f)
	}
}

// A filter is constant text plus template parameters: no value any caller
// supplies ever appears in the expression.
func TestFilterExpressionsHoldNoValue(t *testing.T) {
	at := time.Date(2026, 9, 26, 19, 5, 0, 0, ViTri)
	ns := int64(123457)
	evil := `x" or 1==1 or destination != "`
	gt := int64(45671)
	l := LocCung{DiemDen: evil, DiUng: []string{"tom", evil}, AnKieng: []string{evil}, NganSachVND: &ns, GiaTuVND: &gt,
		DanhMuc: []string{evil}}
	s := SlotTuan(at)
	l.Slot = &s
	e, p := l.BieuThuc()
	for _, v := range []string{evil, "123457", "45671", "khong_ro", "da-lat"} {
		if strings.Contains(e, v) {
			t.Fatalf("value %q spliced into %q", v, e)
		}
	}
	// The same constraints with other values give the same expression text:
	// it depends on which constraints are set, never on their values.
	ns2, s2, gt2 := int64(5), int16(300), int64(1)
	other := LocCung{DiemDen: "ha-noi", DiUng: []string{"sua"}, AnKieng: []string{"halal"}, NganSachVND: &ns2, Slot: &s2,
		GiaTuVND: &gt2, DanhMuc: []string{"cafe"}}
	if e2, _ := other.BieuThuc(); e2 != e {
		t.Fatalf("the expression changed with the values:\n%s\n%s", e, e2)
	}
	if p["dd"] != evil || p["ns"] != ns || p["gt"] != gt || p["sl"] != int64(s) || p["kr"] != KhongRo {
		t.Fatalf("params %v", p)
	}
	for rb, q := range l.BieuThucViPham() {
		if strings.Contains(q.BieuThuc, evil) || strings.Contains(q.BieuThuc, "123457") {
			t.Fatalf("%s: value in %q", rb, q.BieuThuc)
		}
		if !strings.HasPrefix(q.BieuThuc, FTombstoned+" == {tb} and ("+FDestination) {
			t.Fatalf("%s: not scoped to the destination: %q", rb, q.BieuThuc)
		}
	}
	// allergens, diets, open slot, price band (floor and ceiling are one
	// constraint), categories.
	if len(l.BieuThucViPham()) != 5 {
		t.Fatal("one count per constraint but the destination")
	}
}

func TestTuCungClosesFamiliesAndRefusesUnknownIDs(t *testing.T) {
	l, err := TuCung(truyhoi.Cung{DiUng: []string{"hai_san"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"hai_san", "tom", "cua", "muc", "oc_so", "ca"} {
		if !slices.Contains(l.DiUng, want) {
			t.Fatalf("seafood did not close over %s: %v", want, l.DiUng)
		}
	}
	if _, err := TuCung(truyhoi.Cung{DiUng: []string{"peanut"}}); !errors.Is(err, ErrLoc) {
		t.Fatal("an allergen outside the vocabulary was accepted (dropping it would widen the filter)")
	}
	if _, err := TuCung(truyhoi.Cung{AnKieng: []string{"keto"}}); !errors.Is(err, ErrLoc) {
		t.Fatal("a diet outside the vocabulary was accepted")
	}
	mon := time.Date(2026, 9, 21, 0, 10, 0, 0, ViTri) // a Monday
	sun := time.Date(2026, 9, 27, 23, 59, 0, 0, ViTri)
	if SlotTuan(mon) != 0 || SlotTuan(sun) != SoSlotTuan-1 || SlotTuan(mon.UTC()) != 0 {
		t.Fatalf("slots %d %d", SlotTuan(mon), SlotTuan(sun))
	}
}

func TestDatIsFailClosed(t *testing.T) {
	ns := int64(100000)
	s := int16(10)
	l := LocCung{DiUng: []string{"tom"}, NganSachVND: &ns, Slot: &s}
	for name, c := range map[string]struct {
		tt   ThuocTinh
		ok   bool
		rule truyhoi.RangBuoc
	}{
		"clean":            {ThuocTinh{OSlots: []int16{10}, GiaMinVND: 90000}, true, ""},
		"allergen unknown": {ThuocTinh{DiUng: []string{KhongRo}, OSlots: []int16{10}, GiaMinVND: 1}, false, truyhoi.RBDiUng},
		"allergen present": {ThuocTinh{DiUng: []string{"tom"}, OSlots: []int16{10}, GiaMinVND: 1}, false, truyhoi.RBDiUng},
		"hours unknown":    {ThuocTinh{GiaMinVND: 1}, false, truyhoi.RBMoLuc},
		"price unknown":    {ThuocTinh{OSlots: []int16{10}, GiaMinVND: GiaKhongRo}, false, truyhoi.RBNganSach},
		"over budget":      {ThuocTinh{OSlots: []int16{10}, GiaMinVND: 100001}, false, truyhoi.RBNganSach},
		"tombstoned":       {ThuocTinh{OSlots: []int16{10}, GiaMinVND: 1, GoBo: true}, false, ""},
	} {
		ok, rb := l.Dat(c.tt)
		if ok != c.ok || rb != c.rule {
			t.Errorf("%s: %v %q", name, ok, rb)
		}
	}
	// Without an allergy, an unknown allergen list is no reason to drop.
	if ok, _ := (LocCung{}).Dat(ThuocTinh{DiUng: []string{KhongRo}}); !ok {
		t.Fatal("unknown allergens dropped a place for someone without an allergy")
	}
}

func TestFakeNeverReturnsAViolation(t *testing.T) {
	f := MoiFake()
	rows := fxDiaDiem(7, 300)
	f.Them(rows...)
	byID := map[string]HangDiaDiem{}
	for _, r := range rows {
		byID[r.ID] = r
	}
	r := rand.New(rand.NewPCG(1, 2))
	hits := 0
	for i := 0; i < 80; i++ {
		c, q := fxLoc(r)
		l, err := TuCung(c)
		if err != nil {
			t.Fatal(err)
		}
		d, _ := nhung.Stub{}.Nhung(context.Background(), []string{q}, nhung.CauHoi)
		got, err := f.Tim(context.Background(), YeuCauTim{Kho: KhoDiaDiem, Dense: d[0], Thua: &ThuaTruyVan{Text: q}, Loc: l, K: 20})
		if err != nil {
			t.Fatal(err)
		}
		for _, h := range got {
			hits++
			if ok, rb := l.Dat(byID[h.ID].thuocTinh()); !ok {
				t.Fatalf("fake hit %s breaks %s", h.ID, rb)
			}
		}
	}
	if hits < 300 {
		t.Fatalf("only %d hits", hits)
	}
}

func TestBM25IsTheOnlySparseLegAndNeedsNoModel(t *testing.T) {
	q, err := BM25{}.TruyVan(context.Background(), "Đà Lạt")
	if err != nil || q.Text != "Đà Lạt" {
		t.Fatalf("%+v %v", q, err)
	}
	if _, err := (BM25{}).TruyVan(context.Background(), "  "); err == nil {
		t.Fatal("an empty query built a BM25 leg")
	}
}

// The dense index is a deployment choice: empty is HNSW, GPU_CAGRA is
// accepted in any case, anything else stops the process at start; each kind
// builds its own index and searches with its own parameter.
func TestChiMucDenseTheoMoiTruong(t *testing.T) {
	for raw, want := range map[string]ChiMucDense{"": DenseHNSW, "hnsw": DenseHNSW, " gpu_cagra ": DenseGPUCagra} {
		got, err := DocChiMucDense(raw)
		if err != nil || got != want {
			t.Errorf("%q: %q %v", raw, got, err)
		}
	}
	if _, err := DocChiMucDense("IVF_FLAT"); err == nil {
		t.Fatal("an unknown index kind was accepted")
	}
	cagra := DenseGPUCagra.chiMuc().Params()
	if cagra["index_type"] != "GPU_CAGRA" || cagra["metric_type"] != "COSINE" {
		t.Fatalf("GPU index params %v", cagra)
	}
	if p := DenseGPUCagra.thamSoTim().Params(); p["itopk_size"] != cagraITopK || p["ef"] != nil {
		t.Fatalf("GPU search params %v", p)
	}
	if DenseHNSW.chiMuc().Params()["index_type"] != "HNSW" || DenseHNSW.thamSoTim().Params()["ef"] != HNSWEfTimKiem {
		t.Fatal("HNSW index or search parameter changed")
	}
	env := map[string]string{EnvAddr: "127.0.0.1:1", EnvUser: "u", EnvPassword: "p", EnvDenseIndex: "GPU_CAGRA"}
	c, err := FromEnv(func(k string) string { return env[k] })
	if err != nil || c.Dense != DenseGPUCagra {
		t.Fatalf("FromEnv dense %q %v", c.Dense, err)
	}
	env[EnvDenseIndex] = "bogus"
	if _, err := FromEnv(func(k string) string { return env[k] }); err == nil {
		t.Fatal("FromEnv accepted a bogus dense index")
	}
}

// The price band and the categories (TimQuan's filters): a place qualifies
// when its band overlaps [GiaTuVND, NganSachVND] and it has ANY of the asked
// categories; unknown price or unclassified is out; a floor above the
// ceiling and an unknown category are refused, never searched.
func TestGiaVaDanhMucLoc(t *testing.T) {
	i64 := func(v int64) *int64 { return &v }
	place := func(lo, hi int64, dm ...string) ThuocTinh {
		return ThuocTinh{GiaMinVND: lo, GiaMaxVND: hi, DanhMuc: dm}
	}
	cases := []struct {
		name string
		c    truyhoi.Cung
		t    ThuocTinh
		ok   bool
	}{
		{"band inside", truyhoi.Cung{GiaTuVND: i64(40000), NganSachVND: i64(80000)}, place(50000, 70000, "cafe"), true},
		{"band overlaps the floor", truyhoi.Cung{GiaTuVND: i64(40000)}, place(30000, 45000, "cafe"), true},
		{"band below the floor", truyhoi.Cung{GiaTuVND: i64(40000)}, place(20000, 35000, "cafe"), false},
		{"one figure at the floor", truyhoi.Cung{GiaTuVND: i64(40000)}, place(40000, 40000, "cafe"), true},
		{"cheapest above the ceiling", truyhoi.Cung{NganSachVND: i64(30000)}, place(35000, 60000, "cafe"), false},
		{"unknown price under a floor", truyhoi.Cung{GiaTuVND: i64(1)}, place(GiaKhongRo, GiaKhongRo, "cafe"), false},
		{"any of the categories", truyhoi.Cung{DanhMuc: []string{"cafe", "bar_nhau"}}, place(0, 0, "bar_nhau", "quan_an"), true},
		{"none of the categories", truyhoi.Cung{DanhMuc: []string{"cafe"}}, place(0, 0, "quan_an"), false},
		{"unclassified under a category", truyhoi.Cung{DanhMuc: []string{"cafe"}}, place(0, 0, KhongRo), false},
	}
	for _, c := range cases {
		l, err := TuCung(c.c)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if ok, _ := l.Dat(c.t); ok != c.ok {
			t.Errorf("%s: %v, want %v", c.name, ok, c.ok)
		}
	}
	for name, c := range map[string]truyhoi.Cung{
		"floor above the ceiling": {GiaTuVND: i64(90000), NganSachVND: i64(50000)},
		"negative floor":          {GiaTuVND: i64(-1)},
		"unknown category":        {DanhMuc: []string{"quan_nuoc"}},
		"khong_ro as a category":  {DanhMuc: []string{KhongRo}},
	} {
		if _, err := TuCung(c); !errors.Is(err, ErrLoc) {
			t.Errorf("%s accepted: %v", name, err)
		}
	}
}

// A row's maximum price is stored beside its attributes (HangDiaDiem.GiaMaxVND,
// the FPriceMax column): a price floor reads it there. A row whose
// attributes carry no maximum of their own is still found above the floor.
func TestSanGiaDocGiaToiDaCuaHang(t *testing.T) {
	f := MoiFake()
	d, _ := nhung.Stub{}.NhungTaiLieu(context.Background(), []nhung.TaiLieuVao{{NoiDung: "quán cà phê"}})
	f.Them(HangDiaDiem{ID: "p1", Dense: d[0], Text: "quán cà phê", PhienBan: 1, GiaMaxVND: 60000,
		ThuocTinh: ThuocTinh{DiemDen: "d", GiaMinVND: 30000}})
	floor := int64(50000)
	l, err := TuCung(truyhoi.Cung{GiaTuVND: &floor})
	if err != nil {
		t.Fatal(err)
	}
	q, _ := nhung.Stub{}.Nhung(context.Background(), []string{"cà phê"}, nhung.CauHoi)
	got, err := f.Tim(context.Background(), YeuCauTim{Kho: KhoDiaDiem, Dense: q[0], Loc: l, K: 5})
	if err != nil || len(got) != 1 || got[0].ID != "p1" {
		t.Fatalf("a 30k-60k place under a 50k floor: %+v %v", got, err)
	}
}

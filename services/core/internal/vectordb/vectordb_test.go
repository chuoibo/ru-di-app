package vectordb

import (
	"context"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"math"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strconv"
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

func TestPlaceSchemaCarriesTheHardConstraintFields(t *testing.T) {
	ld := LuocDoDiaDiem("rd_places__v1")
	fs := fieldNames(ld.Schema)
	for name, typ := range map[string]entity.FieldType{
		FID: entity.FieldTypeVarChar, FDense: entity.FieldTypeFloatVector, FSparse: entity.FieldTypeSparseVector,
		FText: entity.FieldTypeVarChar, FBM25: entity.FieldTypeSparseVector, FDestination: entity.FieldTypeVarChar,
		FOpenSlots: entity.FieldTypeArray, FPriceMin: entity.FieldTypeInt64, FPriceMax: entity.FieldTypeInt64,
		FAllergens: entity.FieldTypeArray, FDiets: entity.FieldTypeArray, FTombstoned: entity.FieldTypeBool,
		FIndexVersion: entity.FieldTypeInt64, FTextKhongDau: entity.FieldTypeVarChar, FBM25KhongDau: entity.FieldTypeSparseVector,
		FDocID: entity.FieldTypeVarChar, FContentHash: entity.FieldTypeVarChar, FEmbedModel: entity.FieldTypeVarChar,
	} {
		if fs[name] == nil || fs[name].DataType != typ {
			t.Errorf("field %s missing or not %v", name, typ)
		}
	}
	if fs[FDense].TypeParams["dim"] != "1536" {
		t.Fatalf("dense dim %q", fs[FDense].TypeParams["dim"])
	}
	if fs[FID].AutoID || !fs[FID].PrimaryKey {
		t.Fatal("the id must be a deterministic primary key, never autoID (upsert with autoID mints new keys)")
	}
	if ld.Schema.EnableDynamicField || len(ld.Schema.Functions) != 2 {
		t.Fatal("dynamic field on, or a BM25 function missing")
	}
	// The two BM25 legs: the marked text keeps diacritics, the second folds
	// them (structural normalisation only).
	if f := fs[FText].TypeParams["analyzer_params"]; strings.Contains(f, "asciifolding") {
		t.Fatalf("the marked text folds diacritics: %s", f)
	}
	if f := fs[FTextKhongDau].TypeParams["analyzer_params"]; !strings.Contains(f, "asciifolding") {
		t.Fatalf("the folded text does not fold: %s", f)
	}
}

// A filter is constant text plus template parameters: no value any caller
// supplies ever appears in the expression.
func TestFilterExpressionsHoldNoValue(t *testing.T) {
	at := time.Date(2026, 9, 26, 19, 5, 0, 0, ViTri)
	ns := int64(123457)
	evil := `x" or 1==1 or destination != "`
	l := LocCung{DiemDen: evil, DiUng: []string{"tom", evil}, AnKieng: []string{evil}, NganSachVND: &ns}
	s := SlotTuan(at)
	l.Slot = &s
	e, p := l.BieuThuc()
	for _, v := range []string{evil, "123457", "khong_ro", "da-lat"} {
		if strings.Contains(e, v) {
			t.Fatalf("value %q spliced into %q", v, e)
		}
	}
	// The same constraints with other values give the same expression text:
	// it depends on which constraints are set, never on their values.
	ns2, s2 := int64(5), int16(300)
	other := LocCung{DiemDen: "ha-noi", DiUng: []string{"sua"}, AnKieng: []string{"halal"}, NganSachVND: &ns2, Slot: &s2}
	if e2, _ := other.BieuThuc(); e2 != e {
		t.Fatalf("the expression changed with the values:\n%s\n%s", e, e2)
	}
	if p["dd"] != evil || p["ns"] != ns || p["sl"] != int64(s) || p["kr"] != KhongRo {
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
	if len(l.BieuThucViPham()) != 4 {
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
		got, err := f.Tim(context.Background(), YeuCauTim{Kho: KhoDiaDiem, Dense: d[0], Thua: &ThuaTruyVan{Loai: ThuaBM25, Text: q}, Loc: l, K: 20})
		if err != nil {
			t.Fatal(err)
		}
		for _, h := range got {
			hits++
			if ok, rb := l.Dat(byID[h.ID].ThuocTinh); !ok {
				t.Fatalf("fake hit %s breaks %s", h.ID, rb)
			}
		}
	}
	if hits < 300 {
		t.Fatalf("only %d hits", hits)
	}
}

// The MILCO adapter's wire contract, on a loopback fake of the inference
// service: the path, the body, and every structural check on the answer.
func TestMILCOContract(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	answer := `{"vectors":[{"indices":[3,17,900],"values":[0.5,1.25,0.1]}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		_, _ = io.WriteString(w, answer)
	}))
	defer srv.Close()
	m, err := NewMILCO(srv.URL, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	q, err := m.TruyVan(context.Background(), " Quán lẩu ")
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/v1/milco/encode" || gotBody["kind"] != "query" || gotBody["texts"].([]any)[0] != "Quán lẩu" {
		t.Fatalf("path %s body %v", gotPath, gotBody)
	}
	if q.Loai != ThuaMILCO || q.Truong() != FSparse || len(q.Vec.Chi) != 3 {
		t.Fatalf("%+v", q)
	}
	for name, bad := range map[string]string{
		"two vectors for one text": `{"vectors":[{"indices":[1],"values":[1]},{"indices":[1],"values":[1]}]}`,
		"unsorted indices":         `{"vectors":[{"indices":[5,2],"values":[1,1]}]}`,
		"duplicate index":          `{"vectors":[{"indices":[2,2],"values":[1,1]}]}`,
		"negative value":           `{"vectors":[{"indices":[2],"values":[-1]}]}`,
		"length mismatch":          `{"vectors":[{"indices":[2,3],"values":[1]}]}`,
		"index out of range":       `{"vectors":[{"indices":[` + strconv.FormatUint(uint64(MILCOChiMax), 10) + `],"values":[1]}]}`,
		"not json":                 `<html>`,
	} {
		answer = bad
		if _, err := m.TruyVan(context.Background(), "x"); !errors.Is(err, ErrThuaSai) {
			t.Errorf("%s: accepted (%v)", name, err)
		}
	}
	if _, err := NewMILCO("http://10.0.0.1:8000", time.Second); err == nil {
		t.Fatal("a non-loopback inference service was accepted")
	}
	off, err := MILCOTuEnv(func(string) string { return "" })
	if off != nil || err != nil {
		t.Fatal("MILCO is on without MOBILE_MILCO_ENABLED=1")
	}
	nan := ThuaVec{Chi: []uint32{1}, GiaTri: []float32{float32(math.NaN())}}
	if nan.Kiem() == nil {
		t.Fatal("NaN accepted")
	}
}

func TestBM25IsTheDefaultAndNeedsNoModel(t *testing.T) {
	q, err := BM25{}.TruyVan(context.Background(), "Đà Lạt")
	if err != nil || q.Truong() != FBM25 || q.Text != "Đà Lạt" {
		t.Fatalf("%+v %v", q, err)
	}
	vs, _ := BM25{}.TaiLieu(context.Background(), []string{"a", "b"})
	if len(vs) != 2 || len(vs[0].Chi) != 0 {
		t.Fatal("BM25 must leave the MILCO field empty")
	}
}

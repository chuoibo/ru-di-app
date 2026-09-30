//go:build milvus

package vectordb

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	mrand "math/rand/v2"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/milvus-io/milvus-proto/go-api/v3/milvuspb"
	"github.com/milvus-io/milvus/client/v3/entity"
	"github.com/milvus-io/milvus/client/v3/milvusclient"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"mobile/services/core/internal/aiharness/nhung"
	"mobile/services/core/internal/aiharness/truyhoi"
	"mobile/services/core/internal/domain/tuvung"
)

// cauHinhThu and ketThu are testmilvus's helpers, repeated here because an
// in-package test of vectordb cannot import a package that imports vectordb.
func cauHinhThu(t *testing.T) Config {
	t.Helper()
	addr := strings.TrimSpace(os.Getenv("MOBILE_TEST_MILVUS_ADDR"))
	if addr == "" {
		if os.Getenv("CORE_REQUIRE_MILVUS_TESTS") == "1" {
			t.Fatal("CORE_REQUIRE_MILVUS_TESTS=1 but MOBILE_TEST_MILVUS_ADDR is empty")
		}
		t.Skip("MOBILE_TEST_MILVUS_ADDR not set; run scripts/go_milvus_tier.sh")
	}
	user := strings.TrimSpace(os.Getenv("MOBILE_TEST_MILVUS_USER"))
	if user == "" {
		user = "root"
	}
	return Config{Addr: addr, User: user, Password: os.Getenv("MOBILE_TEST_MILVUS_PASSWORD")}
}

func tienToThu() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return "t" + hex.EncodeToString(b) + "_"
}

func ketThu(t *testing.T) *Milvus {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	m, err := Ket(ctx, cauHinhThu(t))
	if err != nil {
		t.Fatal(err)
	}
	m.TienTo = tienToThu()
	m.NhatQuan = entity.ClStrong
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		if err := m.DonTienTo(ctx); err != nil {
			t.Errorf("cleanup of %s*: %v", m.TienTo, err)
		}
		_ = m.Dong(ctx)
	})
	return m
}

func ctxThu(t *testing.T, d time.Duration) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), d)
	t.Cleanup(cancel)
	return ctx
}

// The tier's sentinel: the server answers, auth is on (a wrong password is
// refused), and a collection can be created, loaded and counted.
func TestMilvusTierReachesMilvus(t *testing.T) {
	cfg := cauHinhThu(t)
	ctx := ctxThu(t, 90*time.Second)
	bad := cfg
	bad.Password = cfg.Password + "x"
	if m, err := Ket(ctx, bad); err == nil {
		if err := m.KiemBongAlias(ctx); err == nil {
			t.Fatal("a wrong password was accepted: auth is off on the test server")
		}
		_ = m.Dong(ctx)
	}
	m := ketThu(t)
	if err := m.KiemBongAlias(ctx); err != nil {
		t.Fatal(err)
	}
	name, err := m.TaoPhienBan(ctx, KhoDiaDiem, 1)
	if err != nil {
		t.Fatal(err)
	}
	n, err := m.Dem(ctx, name, FTombstoned+" == {tb}", map[string]any{"tb": false})
	if err != nil || n != 0 {
		t.Fatalf("a fresh collection counts %d, %v", n, err)
	}
}

// napDiaDiem creates version 1 of the place collection behind its alias and
// loads rows into it.
func napDiaDiem(t *testing.T, m *Milvus, rows []HangDiaDiem) string {
	t.Helper()
	ctx := ctxThu(t, 3*time.Minute)
	name, err := m.TaoPhienBan(ctx, KhoDiaDiem, 1)
	if err != nil {
		t.Fatal(err)
	}
	for start := 0; start < len(rows); start += 100 {
		if err := m.GhiDiaDiem(ctx, name, rows[start:min(start+100, len(rows))]); err != nil {
			t.Fatal(err)
		}
	}
	choThay(t, m, name, rows)
	if err := m.NangCap(ctx, KhoDiaDiem, 1); err != nil {
		t.Fatal(err)
	}
	return name
}

// No hard-constraint violation, ever: for 120 random constraint sets (any
// subset of destination, allergy, diet, open slot, budget, category) and each of the
// three leg shapes (hybrid, dense only, BM25 only), every hit Milvus returns
// satisfies the constraints by the Go rule (LocCung.Dat) on the fixture's
// attributes. And the expression is the same rule, not a stricter one: for
// every set, count(*) under the expression equals the number of fixture rows
// Dat accepts, and the unconstrained hit sets are non-empty, so «no
// violation» is not «no results».
func TestKhongViPhamRangBuocCung(t *testing.T) {
	m := ketThu(t)
	rows := fxDiaDiem(7, 400)
	name := napDiaDiem(t, m, rows)
	byID := map[string]HangDiaDiem{}
	for _, r := range rows {
		byID[r.ID] = r
	}
	ctx := ctxThu(t, 10*time.Minute)
	r := mrand.New(mrand.NewPCG(11, 13))
	// The category draws come from their own stream, so the sets above stay
	// the ones every earlier run measured.
	r2 := mrand.New(mrand.NewPCG(17, 19))
	hits, sets, nonEmpty := 0, 0, 0
	for i := 0; i < 120; i++ {
		c, q := fxLoc(r)
		l, err := TuCung(c)
		if err != nil {
			t.Fatal(err)
		}
		if r2.IntN(3) == 0 {
			ids := tuvung.DanhMuc.IDs()
			if l.DanhMuc, err = LocDanhMuc([]string{ids[r2.IntN(len(ids))], ids[r2.IntN(len(ids))]}); err != nil {
				t.Fatal(err)
			}
		}
		sets++
		want := fxDat(rows, l)
		expr, params := l.BieuThuc()
		n, err := m.Dem(ctx, name, expr, params)
		if err != nil {
			t.Fatal(err)
		}
		if int(n) != len(want) {
			t.Fatalf("set %d %+v: the expression counts %d rows, the Go rule %d", i, l, n, len(want))
		}
		dense, _ := nhung.Stub{}.Nhung(ctx, []string{q}, nhung.CauHoi)
		bm := &ThuaTruyVan{Text: q}
		for shape, y := range map[string]YeuCauTim{
			"hybrid": {Dense: dense[0], Thua: bm},
			"dense":  {Dense: dense[0]},
			"bm25":   {Thua: bm},
		} {
			y.Ten, y.Kho, y.Loc, y.K = m.Alias(KhoDiaDiem), KhoDiaDiem, l, 20
			got, err := m.Tim(ctx, y)
			if err != nil {
				t.Fatalf("set %d %s: %v", i, shape, err)
			}
			if len(got) > 0 {
				nonEmpty++
			}
			for _, h := range got {
				hits++
				if ok, rb := l.Dat(byID[h.ID].ThuocTinh); !ok {
					t.Errorf("set %d %s: hit %s breaks %q", i, shape, h.ID, rb)
				}
				if !slices.Contains(want, h.ID) {
					t.Errorf("set %d %s: hit %s outside the allowed set", i, shape, h.ID)
				}
			}
		}
	}
	if hits < 500 || nonEmpty < 150 {
		t.Fatalf("only %d hits over %d non-empty searches: the test measured too little", hits, nonEmpty)
	}
	// A value shaped like expression code is a value: zero rows.
	inj := LocCung{DiemDen: `da-lat" or destination != "x`}
	e, p := inj.BieuThuc()
	if n, err := m.Dem(ctx, name, e, p); err != nil || n != 0 {
		t.Fatalf("an injection-shaped destination counted %d rows (%v)", n, err)
	}
	t.Logf("%d constraint sets, %d hits checked, %d non-empty searches, 0 violations", sets, hits, nonEmpty)
}

// BiLoai counts each constraint's removals in the destination, and the Go
// rule agrees.
func TestDemBiLoaiKhopLuatGo(t *testing.T) {
	m := ketThu(t)
	rows := fxDiaDiem(21, 200)
	name := napDiaDiem(t, m, rows)
	ctx := ctxThu(t, 2*time.Minute)
	ns := int64(150000)
	at := time.Date(2026, 9, 26, 19, 0, 0, 0, ViTri)
	l, err := TuCung(truyhoi.Cung{DiemDenID: "da-lat", DiUng: []string{"tom"}, AnKieng: []string{"chay"}, MoLuc: &at, NganSachVND: &ns})
	if err != nil {
		t.Fatal(err)
	}
	l.DanhMuc = []string{"cafe", "quan_an"}
	got, err := m.DemBiLoai(ctx, name, l)
	if err != nil {
		t.Fatal(err)
	}
	for rb, n := range got {
		want := 0
		for _, r := range rows {
			tt := r.ThuocTinh
			if tt.GoBo || tt.DiemDen != "da-lat" {
				continue
			}
			one := LocCung{}
			switch rb {
			case truyhoi.RBDiUng:
				one.DiUng = l.DiUng
			case truyhoi.RBAnKieng:
				one.AnKieng = l.AnKieng
			case truyhoi.RBMoLuc:
				one.Slot = l.Slot
			case truyhoi.RBNganSach:
				one.NganSachVND = l.NganSachVND
			case truyhoi.RBDanhMuc:
				one.DanhMuc = l.DanhMuc
			}
			if ok, _ := one.Dat(tt); !ok {
				want++
			}
		}
		if n != want || want == 0 {
			t.Errorf("%s: Milvus counts %d removed, the Go rule %d", rb, n, want)
		}
	}
	if len(got) != 5 {
		t.Fatalf("counted %d constraints, want 5 (all but the destination)", len(got))
	}
}

// Promote, then roll back, through the alias: each step seen by a search on
// the alias. And an alias shadowed by a collection of its own name is
// refused before any move.
func TestAliasNangCapQuayLai(t *testing.T) {
	m := ketThu(t)
	ctx := ctxThu(t, 3*time.Minute)
	rows := fxDiaDiem(3, 2)
	a, b := rows[0], rows[1]
	a.ID, b.ID = "chi-v1", "chi-v2"
	a.ThuocTinh.GoBo, b.ThuocTinh.GoBo = false, false
	v1, err := m.TaoPhienBan(ctx, KhoDiaDiem, 1)
	if err != nil {
		t.Fatal(err)
	}
	v2, err := m.TaoPhienBan(ctx, KhoDiaDiem, 2)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.GhiDiaDiem(ctx, v1, []HangDiaDiem{a}); err != nil {
		t.Fatal(err)
	}
	if err := m.GhiDiaDiem(ctx, v2, []HangDiaDiem{b}); err != nil {
		t.Fatal(err)
	}
	choThay(t, m, v1, []HangDiaDiem{a})
	choThay(t, m, v2, []HangDiaDiem{b})
	thay := func(want string) {
		t.Helper()
		got, err := m.Tim(ctx, YeuCauTim{Ten: m.Alias(KhoDiaDiem), Kho: KhoDiaDiem, Dense: a.Dense, K: 5})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].ID != want {
			t.Fatalf("the alias serves %+v, want only %s", got, want)
		}
	}
	for _, step := range []struct {
		v    int64
		want string
	}{{1, "chi-v1"}, {2, "chi-v2"}, {1, "chi-v1"}} {
		if err := m.NangCap(ctx, KhoDiaDiem, step.v); err != nil {
			t.Fatal(err)
		}
		thay(step.want)
	}
	if err := m.XoaPhienBan(ctx, KhoDiaDiem, 1); err == nil {
		t.Fatal("the served version was dropped")
	}
	// A collection named like the alias of another Kho: every move refused.
	ld := LuocDoHuongDan(m.Alias(KhoHuongDan))
	if err := m.cli.CreateCollection(ctx, milvusclient.NewCreateCollectionOption(m.Alias(KhoHuongDan), ld.Schema).WithIndexOptions(ld.Index...)); err != nil {
		t.Fatal(err)
	}
	if err := m.NangCap(ctx, KhoDiaDiem, 2); !errors.Is(err, ErrBongAlias) {
		t.Fatalf("an alias shadowed by a collection was moved: %v", err)
	}
	thay("chi-v1")
}

// A delete is invisible at once, and after NenVaCho the rows are gone from
// the persistent segments too, not only hidden by a tombstone.
func TestXoaVaNenBienMat(t *testing.T) {
	m := ketThu(t)
	ctx := ctxThu(t, 4*time.Minute)
	rows := fxDiaDiem(5, 60)
	name, err := m.TaoPhienBan(ctx, KhoDiaDiem, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.GhiDiaDiem(ctx, name, rows); err != nil {
		t.Fatal(err)
	}
	choThay(t, m, name, rows)
	if err := m.Flush(ctx, name); err != nil {
		t.Fatal(err)
	}
	var gone []string
	for _, r := range rows[:12] {
		gone = append(gone, r.ID)
	}
	if err := m.XoaID(ctx, name, gone); err != nil {
		t.Fatal(err)
	}
	all := FID + " != {x}"
	if n, err := m.Dem(ctx, name, all, map[string]any{"x": ""}); err != nil || n != 48 {
		t.Fatalf("after delete count(*) = %d, %v; want 48", n, err)
	}
	for _, r := range rows[:12] {
		got, err := m.Tim(ctx, YeuCauTim{Ten: name, Kho: KhoDiaDiem, Dense: r.Dense, Thua: &ThuaTruyVan{Text: r.Text}, K: 50})
		if err != nil {
			t.Fatal(err)
		}
		for _, h := range got {
			if slices.Contains(gone, h.ID) {
				t.Fatalf("deleted %s came back from a search", h.ID)
			}
		}
	}
	before, err := m.SoHangLuuTru(ctx, name)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.NenVaCho(ctx, name, 2*time.Minute); err != nil {
		t.Fatal(err)
	}
	var after int64
	for i := 0; i < 60; i++ {
		if after, err = m.SoHangLuuTru(ctx, name); err != nil {
			t.Fatal(err)
		}
		if after == 48 {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if before != 60 || after != 48 {
		t.Fatalf("persistent rows before %d, after compaction %d; want 60 then 48", before, after)
	}
}

// Telemetry off, seen from the server: a canary client with telemetry on
// and a pinned id shows up in the server's client-telemetry list; a client
// built from this package's config (telemetry off, the same id scheme) never
// does, though it made calls while the canary was heartbeating.
func TestTelemetryTatTrenMayChu(t *testing.T) {
	cfg := cauHinhThu(t)
	ctx := ctxThu(t, 90*time.Second)
	prefix := tienToThu()
	canaryID, oursID := prefix+"canary", prefix+"ours"

	canary, err := milvusclient.New(ctx, &milvusclient.ClientConfig{Address: cfg.Addr, Username: cfg.User, Password: cfg.Password,
		TelemetryConfig: &milvusclient.TelemetryConfig{Enabled: true, ClientID: canaryID, HeartbeatInterval: time.Second, SamplingRate: 1}})
	if err != nil {
		t.Fatal(err)
	}
	defer canary.Close(ctx)
	cc := clientConfig(cfg)
	if cc.TelemetryConfig == nil || cc.TelemetryConfig.Enabled {
		t.Fatal("the package's config does not turn telemetry off")
	}
	cc.TelemetryConfig.ClientID = oursID
	ours, err := milvusclient.New(ctx, cc)
	if err != nil {
		t.Fatal(err)
	}
	defer ours.Close(ctx)

	conn, err := grpc.NewClient(cfg.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	auth := metadata.AppendToOutgoingContext(ctx, "authorization", base64.StdEncoding.EncodeToString([]byte(cfg.User+":"+cfg.Password)))
	svc := milvuspb.NewClientTelemetryServiceClient(conn)
	clients := func() map[string]bool {
		resp, err := svc.GetClientTelemetry(auth, &milvuspb.GetClientTelemetryRequest{})
		if err != nil {
			t.Fatalf("the server's client-telemetry list: %v", err)
		}
		seen := map[string]bool{}
		for _, c := range resp.GetClients() {
			seen[c.GetClientInfo().GetReserved()["client_id"]] = true
		}
		return seen
	}
	var seen map[string]bool
	for i := 0; i < 40; i++ {
		_, _ = ours.ListCollections(ctx, milvusclient.NewListCollectionOption())
		_, _ = canary.ListCollections(ctx, milvusclient.NewListCollectionOption())
		if seen = clients(); seen[canaryID] {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if !seen[canaryID] {
		t.Fatal("the canary with telemetry on never reached the server's list: this probe cannot see telemetry at all")
	}
	time.Sleep(3 * time.Second)
	if clients()[oursID] {
		t.Fatal("a client built from vectordb's config sent telemetry to the server")
	}
}

// The memory store: every read, count and delete is the owner's alone.
func TestTriNhoChiCuaChuSoHuu(t *testing.T) {
	m := ketThu(t)
	ctx := ctxThu(t, 3*time.Minute)
	if _, err := m.TaoPhienBan(ctx, KhoTriNho, 1); err != nil {
		t.Fatal(err)
	}
	if err := m.NangCap(ctx, KhoTriNho, 1); err != nil {
		t.Fatal(err)
	}
	kho := m.TriNho()
	an, _ := MoiChuSoHuu("nguoi-a")
	binh, _ := MoiChuSoHuu("nguoi-b")
	vec := func(s string) []float32 {
		v, _ := nhung.Stub{}.Nhung(ctx, []string{s}, nhung.TaiLieu)
		return v[0]
	}
	var rowsA, rowsB []HangTriNho
	for i := 0; i < 3; i++ {
		rowsA = append(rowsA, HangTriNho{ID: fmt.Sprintf("fa%d", i), Loai: "thich_danh_muc", TaoLuc: int64(i), Dense: vec(fmt.Sprintf("lẩu %d", i)), PhienBan: 1})
	}
	for i := 0; i < 2; i++ {
		rowsB = append(rowsB, HangTriNho{ID: fmt.Sprintf("fb%d", i), Loai: "thich_danh_muc", TaoLuc: int64(i), Dense: vec(fmt.Sprintf("lẩu %d", i)), PhienBan: 1})
	}
	if err := kho.Ghi(ctx, an, rowsA); err != nil {
		t.Fatal(err)
	}
	if err := kho.Ghi(ctx, binh, rowsB); err != nil {
		t.Fatal(err)
	}
	var got []Trung
	var err error
	for i := 0; i < 100; i++ { // a fresh upsert becomes searchable ~200 ms later (choThay)
		if got, err = kho.Tim(ctx, an, vec("lẩu 0"), 10); err != nil {
			t.Fatal(err)
		}
		if len(got) >= 3 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if len(got) != 3 {
		t.Fatalf("A's search returned %d rows", len(got))
	}
	for _, h := range got {
		if !strings.HasPrefix(h.ID, "fa") {
			t.Fatalf("A's search returned B's %s", h.ID)
		}
	}
	// A deleting B's ids deletes nothing.
	if err := kho.Xoa(ctx, an, []string{"fb0", "fb1"}); err != nil {
		t.Fatal(err)
	}
	if n, _ := kho.Dem(ctx, binh); n != 2 {
		t.Fatalf("A's delete of B's ids left B with %d", n)
	}
	if err := kho.XoaHet(ctx, an); err != nil {
		t.Fatal(err)
	}
	if n, _ := kho.Dem(ctx, an); n != 0 {
		t.Fatalf("after A's account deletion A counts %d", n)
	}
	if n, _ := kho.Dem(ctx, binh); n != 2 {
		t.Fatalf("A's account deletion touched B: %d", n)
	}
	if err := kho.NenVaCho(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := MoiChuSoHuu(""); !errors.Is(err, ErrChuSoHuu) {
		t.Fatal("an empty owner was accepted")
	}
	if err := kho.XoaHet(ctx, ChuSoHuu{}); !errors.Is(err, ErrChuSoHuu) {
		t.Fatal("the zero owner deleted")
	}
}

// The category filter on the live server: a row qualifies when its
// categories hold ANY asked id; a row whose categories are unknown
// ([khong_ro]) or hold none of them is out, by the expression and by
// every leg of a search, and the Go rule counts the same rows. Unknown
// categories are stored as [khong_ro]; all ten ids fit one row.
func TestDanhMucLocMilvus(t *testing.T) {
	m := ketThu(t)
	ctx := ctxThu(t, 3*time.Minute)
	base := fxDiaDiem(29, 4)
	mk := func(i int, id string, dm []string) HangDiaDiem {
		r := base[i]
		r.ID, r.Text = id, "quán nhỏ ven hồ yên tĩnh"
		r.ThuocTinh = ThuocTinh{DiemDen: "da-lat", GiaMinVND: GiaKhongRo, DanhMuc: dm}
		return r
	}
	all10 := tuvung.DanhMuc.IDs()
	rows := []HangDiaDiem{
		mk(0, "chi-cafe", []string{"cafe"}),
		mk(1, "chi-luu-tru-an", []string{"luu_tru", "quan_an"}),
		mk(2, "chi-khong-ro", nil),
		mk(3, "chi-vui", []string{"vui_choi", "thien_nhien"}),
		mk(0, "chi-moi-thu", all10),
	}
	name := napDiaDiem(t, m, rows)
	got, err := m.DocThuocTinh(ctx, name, "chi-khong-ro")
	if err != nil || !slices.Equal(got["chi-khong-ro"].DanhMuc, []string{KhongRo}) {
		t.Fatalf("unknown categories stored as %+v (%v)", got["chi-khong-ro"], err)
	}
	if got, err := m.DocThuocTinh(ctx, name, "chi-moi-thu"); err != nil || len(got["chi-moi-thu"].DanhMuc) != 10 {
		t.Fatalf("all ten categories stored as %+v (%v)", got["chi-moi-thu"], err)
	}
	l := LocCung{DanhMuc: []string{"cafe", "quan_an"}}
	want := []string{"chi-cafe", "chi-luu-tru-an", "chi-moi-thu"}
	if !slices.Equal(fxDat(rows, l), want) {
		t.Fatalf("the Go rule admits %v", fxDat(rows, l))
	}
	e, p := l.BieuThuc()
	if n, err := m.Dem(ctx, name, e, p); err != nil || n != 3 {
		t.Fatalf("the expression counts %d rows (%v), want 3", n, err)
	}
	for shape, y := range map[string]YeuCauTim{
		"hybrid": {Dense: rows[2].Dense, Thua: &ThuaTruyVan{Text: rows[2].Text}},
		"dense":  {Dense: rows[2].Dense},
		"bm25":   {Thua: &ThuaTruyVan{Text: rows[2].Text}},
	} {
		y.Ten, y.Kho, y.Loc, y.K = m.Alias(KhoDiaDiem), KhoDiaDiem, l, 10
		hits, err := m.Tim(ctx, y)
		if err != nil {
			t.Fatal(err)
		}
		var ids []string
		for _, h := range hits {
			ids = append(ids, h.ID)
		}
		slices.Sort(ids)
		if !slices.Equal(ids, want) {
			t.Fatalf("%s: hits %v, want %v (the unknown and the other category out)", shape, ids, want)
		}
	}
	// Asking for khong_ro itself is refused before any search.
	if _, err := LocDanhMuc([]string{KhongRo}); !errors.Is(err, ErrLoc) {
		t.Fatal("khong_ro accepted as a category to filter on")
	}
	// Without the filter every live row stays, the unknown one included.
	all, _ := LocCung{}.BieuThuc()
	if n, err := m.Dem(ctx, name, all, map[string]any{"tb": false}); err != nil || n != 5 {
		t.Fatalf("without the filter %d rows (%v)", n, err)
	}
}

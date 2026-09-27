package aidoc

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"mobile/services/core/internal/aiharness/truyhoi"
	"mobile/services/core/internal/domain/giomo"
)

// The mapping is field by field: what the model extracted, nothing read out
// of the query text.
func TestYeuCauRag(t *testing.T) {
	ngan := int64(150000)
	luc := time.Date(2026, 9, 26, 19, 0, 0, 0, time.FixedZone("ICT", 7*3600))
	y := truyhoi.YeuCau{Nguon: truyhoi.Places, Cau: "quán chay ở Vũng Tàu, dị ứng tôm",
		Cung: truyhoi.Cung{DiemDenID: "da-lat", DiUng: []string{"dau_phong"}, AnKieng: []string{"chay"}, NganSachVND: &ngan, MoLuc: &luc},
		Mem:  truyhoi.Mem{LoaiCho: []string{"quan_an"}, KhiChat: []string{"yen_tinh"}, KhuVuc: "da-lat"}, K: 7}
	r := YeuCauRag(y)
	m := giomo.PhutCuaTuan(luc)
	if r.DiemDen != "da-lat" || r.Cau != y.Cau || !reflect.DeepEqual(r.DiUng, []string{"dau_phong"}) || !reflect.DeepEqual(r.AnKieng, []string{"chay"}) ||
		r.NganSach != &ngan || r.Luc == nil || *r.Luc != m || r.Khung != nil || r.Bo != nil ||
		!reflect.DeepEqual(r.LoaiCho, []string{"quan_an"}) || !reflect.DeepEqual(r.KhiChat, []string{"yen_tinh"}) || r.K != 7 {
		t.Fatalf("%+v", r)
	}
	// Nothing in the words becomes a filter.
	r = YeuCauRag(truyhoi.YeuCau{Nguon: truyhoi.Places, Cau: y.Cau})
	if r.DiemDen != "" || len(r.DiUng) != 0 || len(r.AnKieng) != 0 || r.NganSach != nil || r.Luc != nil || len(r.LoaiCho) != 0 || len(r.KhiChat) != 0 {
		t.Fatalf("the words set a filter: %+v", r)
	}
}

// A window the model gave maps to rag's window of minutes, both ends kept:
// «tối nay» 18:00–22:00 is never narrowed to «open at 18:00». A window
// crossing Sunday night stays one window past the week's end.
func TestYeuCauRagKhung(t *testing.T) {
	gio := time.FixedZone("ICT", 7*3600)
	tu := time.Date(2026, 9, 25, 18, 0, 0, 0, gio)
	r := YeuCauRag(truyhoi.YeuCau{Nguon: truyhoi.Places, Cung: truyhoi.Cung{MoTrong: &truyhoi.KhungMo{Tu: tu, Den: tu.Add(4 * time.Hour)}}})
	a := giomo.PhutCuaTuan(tu)
	if r.Luc != nil || r.Khung == nil || *r.Khung != [2]int{a, a + 240} {
		t.Fatalf("%+v", r.Khung)
	}
	cn := time.Date(2026, 9, 27, 22, 0, 0, 0, gio) // a Sunday
	r = YeuCauRag(truyhoi.YeuCau{Nguon: truyhoi.Places, Cung: truyhoi.Cung{MoTrong: &truyhoi.KhungMo{Tu: cn, Den: cn.Add(4 * time.Hour)}}})
	if b := giomo.PhutCuaTuan(cn); *r.Khung != [2]int{b, b + 240} || r.Khung[1] <= giomo.PhutTuan {
		t.Fatalf("crossing Sunday night: %+v", r.Khung)
	}
}

// No code of the tool path imports a word-list reader or calls a
// query-side resolver: the owner's rule, held by the source itself.
func TestKhongDocChu(t *testing.T) {
	cam := []string{
		`"mobile/services/core/internal/domain/tuvung"`,
		`"mobile/services/core/internal/aiharness/preprocess"`,
	}
	camGoi := []string{"DocCau", "ResolveDestination", "DanhSachNgan", "DongMayChu", "LaTien", "Nghi"}
	for _, dir := range []string{".", "../aiharness/tools", "../aiharness/tactu"} {
		files, _ := filepath.Glob(filepath.Join(dir, "*.go"))
		if len(files) == 0 {
			t.Fatalf("no files in %s", dir)
		}
		for _, f := range files {
			if strings.HasSuffix(f, "_test.go") {
				continue
			}
			src, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			pf, err := parser.ParseFile(token.NewFileSet(), f, src, parser.ImportsOnly)
			if err != nil {
				t.Fatal(err)
			}
			for _, im := range pf.Imports {
				for _, c := range cam {
					if im.Path.Value == c {
						t.Errorf("%s imports %s", f, c)
					}
				}
				if p, _ := strconv.Unquote(im.Path.Value); strings.HasSuffix(p, "/guard") {
					// guard is allowed for its data-format check only.
					if strings.Contains(string(src), "guard.Nghi") || strings.Contains(string(src), "guard.LaTien") {
						t.Errorf("%s calls a guard keyword reader", f)
					}
				}
			}
			for _, g := range camGoi {
				if strings.Contains(string(src), "."+g+"(") {
					t.Errorf("%s calls %s", f, g)
				}
			}
		}
	}
}

package huongdan

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"mobile/services/core/internal/rag/xephang"
)

func idCua(ds []Doan) []string {
	out := make([]string, len(ds))
	for i, d := range ds {
		out[i] = d.ID
	}
	return out
}

func TestTheoManTraMucTheoThuTuTep(t *testing.T) {
	want := []string{
		"chat-nhom/mo-khay-cong-cu",
		"chat-nhom/tao-mot-binh-chon",
		"chat-nhom/bo-phieu-hoac-doi-phieu",
		"chat-nhom/chot-binh-chon",
		"chat-nhom/bien-lua-chon-thang-thanh-to-hen-chung",
		"chat-nhom/sua-to-hen-chung-cung-ca-hoi",
		"chat-nhom/nho-ru-di-ai-ngay-trong-nhom",
		"chat-nhom/tu-tao-keo-khong-can-ai",
		"chat-nhom/cai-dat-va-thanh-vien",
	}
	if got := idCua(TheoMan("groups/[id]/chat")); !reflect.DeepEqual(got, want) {
		t.Fatalf("TheoMan(chat) = %v", got)
	}
	// As walked, the way PhieuNguCanh.man may carry it.
	if got := idCua(TheoMan("/groups/42/chat")); !reflect.DeepEqual(got, want) {
		t.Fatalf("TheoMan(/groups/42/chat) = %v", got)
	}
	first := TheoMan("groups/[id]/chat")[1]
	if first.TieuDe != "Tạo một bình chọn" || first.TieuDeMan != "Chat nhóm" || first.Tien ||
		!reflect.DeepEqual(first.Nhan, []string{"Bình chọn", "Câu hỏi", "Thêm lựa chọn", "Gửi bình chọn", "/vote"}) ||
		len(first.Buoc) != 4 || !strings.HasPrefix(first.Buoc[0], "Mở khay, bấm «Bình chọn»") {
		t.Fatalf("section: %+v", first)
	}
	for _, man := range []string{"settings", "khong-co", "", "/outings/7/khong-co"} {
		if got := TheoMan(man); got != nil {
			t.Errorf("TheoMan(%q) = %v, want nil", man, idCua(got))
		}
	}
}

// Money screens expose navigation only. What is enforced, exactly: a manual
// with tien: true loads only if it has one section, no digit anywhere in its
// body, no line in that section that is not a step, and every step quotes a
// door of the screen (kiemManTien; the refusals are in nap_test.go). What that
// cannot tell apart is a step that names a door AND says how to pay in the
// same line; the drift gate's review of the prose is what covers that.
func TestManTienChiCoMotMucChiDuong(t *testing.T) {
	want := map[string][]string{
		"finance":                 {"tai-chinh/toi-man-nay-va-di-tiep"},
		"smart-split/[id]/review": {"chia-hoa-don/toi-man-nay-va-di-tiep"},
	}
	got := map[string][]string{}
	for _, d := range soTay.doan {
		if d.Tien {
			got[d.Man] = append(got[d.Man], d.ID)
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("money sections %v, want %v", got, want)
	}
	for man := range want {
		ds := TheoMan(man)
		if len(ds) != 1 || !ds[0].Tien {
			t.Fatalf("TheoMan(%s) = %+v", man, ds)
		}
		for _, b := range ds[0].Buoc {
			if !strings.Contains(b, "«") {
				t.Errorf("%s: step names no label: %q", man, b)
			}
		}
	}
	// A money question finds the door and nothing that says how to pay.
	for _, d := range Tim(context.Background(), Hoi{Cau: "chia bill ở đâu", K: 10}) {
		if d.Tien && d.ID != "chia-hoa-don/toi-man-nay-va-di-tiep" && d.ID != "tai-chinh/toi-man-nay-va-di-tiep" {
			t.Errorf("unexpected money section %s", d.ID)
		}
	}
}

// The money flag is not trusted from the file: it must agree with the money
// segments the app itself declares (MAN_NEP_LUI in phieu.ts).
func TestTienKhopManNepLui(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "apps", "mobile", "src", "rudi", "nep", "phieu.ts"))
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`MAN_NEP_LUI\s*=\s*\[(.*?)\]\s*as const`).FindStringSubmatch(string(raw))
	if m == nil {
		t.Fatal("MAN_NEP_LUI not found in phieu.ts")
	}
	lui := map[string]bool{}
	for _, q := range regexp.MustCompile(`"([^"]+)"`).FindAllStringSubmatch(m[1], -1) {
		lui[q[1]] = true
	}
	if len(lui) < 4 {
		t.Fatalf("read %d money segments from phieu.ts", len(lui))
	}
	for _, tr := range soTay.trang {
		if want := lui[strings.Split(tr.man, "/")[0]]; tr.tien != want {
			t.Errorf("%s.md: tien %v, phieu.ts says %v", tr.ten, tr.tien, want)
		}
	}
}

// Sections of the screen the person is on come first when they match at all.
func TestTimGhimManDangDung(t *testing.T) {
	ctx := context.Background()
	khong := idCua(Tim(ctx, Hoi{Cau: "tạo kèo", K: 100}))
	if len(khong) < 5 || strings.HasPrefix(khong[0], "tao-keo/") {
		t.Fatalf("unpinned ranking %v: the check below needs a tao-keo section that is not first", khong)
	}
	co := idCua(Tim(ctx, Hoi{Cau: "tạo kèo", Man: "outings/new", K: 100}))
	var ghim, con []string
	for _, id := range khong {
		if strings.HasPrefix(id, "tao-keo/") {
			ghim = append(ghim, id)
		} else {
			con = append(con, id)
		}
	}
	if want := append(ghim, con...); !reflect.DeepEqual(co, want) {
		t.Fatalf("pinned %v, want %v (the current screen's matches first, each group in score order)", co, want)
	}
	if len(ghim) < 2 {
		t.Fatalf("only %d tao-keo sections matched", len(ghim))
	}
	if walked := idCua(Tim(ctx, Hoi{Cau: "tạo kèo", Man: "/outings/new", K: 100})); !reflect.DeepEqual(walked, co) {
		t.Fatalf("as walked %v, as declared %v", walked, co)
	}
}

// A section that shares only question words («bấm», «gì», «thì») with the
// question does not match, so it is neither pinned nor returned.
func TestTimTuDemKhongLamKhop(t *testing.T) {
	got := idCua(Tim(context.Background(), Hoi{Cau: "bấm gì thì đăng xuất", Man: "groups/[id]/chat", K: 10}))
	if len(got) == 0 || got[0] != "ca-nhan/cai-dat-va-dang-xuat" {
		t.Fatalf("got %v, want the sign-out section first", got)
	}
	for _, id := range got {
		if strings.HasPrefix(id, "chat-nhom/") {
			t.Fatalf("%s matched on question words alone: %v", id, got)
		}
	}
	for _, q := range []string{"", "   ", "làm sao thì được không", "ko dc j z", "?!"} {
		if got := Tim(context.Background(), Hoi{Cau: q, Man: "plan"}); got != nil {
			t.Errorf("%q returned %v", q, idCua(got))
		}
	}
}

func TestTimKhongDauNhuCoDau(t *testing.T) {
	for _, c := range [][2]string{{"đổi tên nhóm", "doi ten nhom"}, {"bỏ phiếu", "bo phieu"}, {"lưu địa điểm", "LUU DIA DIEM"}} {
		a := idCua(Tim(context.Background(), Hoi{Cau: c[0], K: 5}))
		b := idCua(Tim(context.Background(), Hoi{Cau: c[1], K: 5}))
		if len(a) == 0 || !reflect.DeepEqual(a, b) {
			t.Errorf("%q %v, %q %v", c[0], a, c[1], b)
		}
	}
}

func TestTimKMacDinhVaGioiHan(t *testing.T) {
	ctx := context.Background()
	for k, want := range map[int]int{0: KMacDinh, -3: KMacDinh, 1: 1, 2: 2} {
		if got := len(Tim(ctx, Hoi{Cau: "tạo kèo", K: k})); got != want {
			t.Errorf("K=%d returned %d, want %d", k, got, want)
		}
	}
	all := Tim(ctx, Hoi{Cau: "tạo kèo", K: 1000})
	if len(all) == 0 || len(all) > len(soTay.doan) {
		t.Fatalf("K=1000 returned %d", len(all))
	}
}

func TestTimCtxDaHuy(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := Tim(ctx, Hoi{Cau: "tạo kèo"}); got != nil {
		t.Fatalf("done context returned %v", idCua(got))
	}
}

// Results are copies: a caller that edits one cannot edit the manual.
func TestTimVaTheoManTraBanSao(t *testing.T) {
	a := Tim(context.Background(), Hoi{Cau: "bỏ phiếu", K: 1})
	a[0].Buoc[0] = "đã bị sửa"
	a[0].Nhan[0] = "đã bị sửa"
	b := TheoMan("groups/[id]/chat")
	b[2].Buoc[1] = "đã bị sửa"
	for _, d := range soTay.doan {
		for _, s := range append(append([]string{}, d.Buoc...), d.Nhan...) {
			if s == "đã bị sửa" {
				t.Fatalf("%s was edited through a returned copy", d.ID)
			}
		}
	}
}

// Same index, same question, same list: across calls and across goroutines
// (run with -race).
func TestTimXacDinhDongThoi(t *testing.T) {
	cau := docVang(t)
	goc := make([][]string, len(cau))
	for i, c := range cau {
		goc[i] = idCua(Tim(context.Background(), Hoi{Cau: c.Hoi, Man: c.Man, K: 10}))
	}
	var wg sync.WaitGroup
	loi := make(chan string, 8*len(cau))
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i, c := range cau {
				if got := idCua(Tim(context.Background(), Hoi{Cau: c.Hoi, Man: c.Man, K: 10})); !reflect.DeepEqual(got, goc[i]) {
					loi <- c.Hoi
				}
			}
		}()
	}
	wg.Wait()
	close(loi)
	for q := range loi {
		t.Errorf("%q ranked differently on another call", q)
	}
}

func TestTuDemLaAmTietDaGap(t *testing.T) {
	for w := range tuDem {
		if got := xephang.AmTiet(w); len(got) != 1 || got[0] != w {
			t.Errorf("stop word %q folds to %v: it would never match a folded syllable", w, got)
		}
	}
}

// ≤50 ms per question on the full manual (design 05 §3). Each golden question
// is timed on its own, best of three to keep a scheduler hiccup from being
// read as the ranker; the bound is on the slowest question.
func TestTimDuoi50ms(t *testing.T) {
	var cham time.Duration
	var cauCham string
	for _, c := range docVang(t) {
		tot := time.Hour
		for i := 0; i < 3; i++ {
			bd := time.Now()
			Tim(context.Background(), Hoi{Cau: c.Hoi, Man: c.Man, K: 10})
			if d := time.Since(bd); d < tot {
				tot = d
			}
		}
		if tot > cham {
			cham, cauCham = tot, c.Hoi
		}
	}
	t.Logf("slowest question %q: %v", cauCham, cham)
	if cham > 50*time.Millisecond {
		t.Fatalf("slowest question took %v, bound 50ms", cham)
	}
}

func BenchmarkTim(b *testing.B) {
	cau := docVang(b)
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c := cau[i%len(cau)]
		Tim(ctx, Hoi{Cau: c.Hoi, Man: c.Man})
	}
}

// Building the whole manual (parse, validate, index, graph) is what init pays.
func BenchmarkNap(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if _, err := nap(duLieu); err != nil {
			b.Fatal(err)
		}
	}
}

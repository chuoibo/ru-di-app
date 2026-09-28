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
	"unicode/utf8"

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
// with tien: true loads only if its route is a money route, it has one
// section headed tieuDeManTien, no digit anywhere in its body, no line in
// that section that is not a step, every declared way in or out is a button
// of the code that leads there (a labelled edge of _rut.json), and every step
// quotes one of those doors and no other label (kiemManTien; the refusals,
// with the bypasses of review 13, are in nap_test.go). What that cannot tell
// apart is a step that names a door AND says how to pay in plain words, with
// no «…», in the same line; a person reviewing the prose is what covers that.
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
	lui := manNepLui(t)
	for _, tr := range soTay.trang {
		if want := lui[strings.Split(tr.man, "/")[0]]; tr.tien != want {
			t.Errorf("%s.md: tien %v, phieu.ts says %v", tr.ten, tr.tien, want)
		}
	}
}

// manNepLui reads MAN_NEP_LUI, the money segments, from phieu.ts.
func manNepLui(t *testing.T) map[string]bool {
	t.Helper()
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
	return lui
}

// Sections of the screen the person is on come first when they score at
// least tyLeGhim of the best score; every other section keeps its place in
// the ranking with no screen. Every term of the query ranks: no word list
// decides whether a section matches. The cases hold both sides: a
// current-screen section pinned, and one that scored but was not.
func TestTimGhimTheoTyLe(t *testing.T) {
	ctx := context.Background()
	daGhim, khongGhim := 0, 0
	for _, c := range []struct{ cau, man string }{
		{"tạo kèo", "outings/new"},
		{"xem lại các buổi đã đi", "plan"},
		{"gửi ảnh cho cả nhóm xem", "plan"},
		{"bỏ phiếu lại cho lựa chọn khác", "groups/[id]/chat"},
		{"tạo kèo mới ở đâu", "create"},
	} {
		amTiet := xephang.AmTiet(c.cau)
		diem, cao := map[string]float64{}, -1.0
		for _, kq := range soTay.chiMuc.Tim(strings.Join(amTiet, " "), soTay.chiMuc.Len()) {
			diem[kq.ID] = kq.Diem
			if cao < 0 {
				cao = kq.Diem
			}
		}
		khong := idCua(Tim(ctx, Hoi{Cau: c.cau, K: 100}))
		var ghim, con []string
		for _, id := range khong {
			laMan := soTay.doan[soTay.theoID[id]].Man == c.man
			switch {
			case laMan && diem[id] >= tyLeGhim*cao:
				ghim = append(ghim, id)
				daGhim++
			case laMan:
				con = append(con, id)
				khongGhim++
			default:
				con = append(con, id)
			}
		}
		co := idCua(Tim(ctx, Hoi{Cau: c.cau, Man: c.man, K: 100}))
		if want := append(ghim, con...); !reflect.DeepEqual(co, want) {
			t.Errorf("%q on %s: %v, want %v", c.cau, c.man, co, want)
		}
		if walked := idCua(Tim(ctx, Hoi{Cau: c.cau, Man: "/" + strings.ReplaceAll(c.man, "[id]", "7"), K: 100})); !reflect.DeepEqual(walked, co) {
			t.Errorf("%q: as walked %v, as declared %v", c.cau, walked, co)
		}
	}
	if daGhim < 2 || khongGhim < 2 {
		t.Fatalf("pinned %d, left in place %d: the cases no longer hold both sides", daGhim, khongGhim)
	}
	// And what the pin keeps: asked where the answer is, the answer comes
	// first.
	if got := idCua(Tim(ctx, Hoi{Cau: "tạo kèo", Man: "outings/new", K: 4})); len(got) == 0 || !strings.HasPrefix(got[0], "tao-keo/") {
		t.Errorf("tạo kèo on outings/new: %v", got)
	}
}

// No Go table reads the query: a teencode spelling is ranked as the
// syllables it is (the model rewrites «ko», «fieu» into the manual's words
// before the query reaches Tim), and a query of question words alone ranks
// whatever shares them, by BM25, with nothing gated out. An empty query, or
// one with no syllable, returns nothing.
func TestTimKhongDocTu(t *testing.T) {
	ctx := context.Background()
	if got := idCua(Tim(ctx, Hoi{Cau: "bỏ phiếu", K: 3})); len(got) == 0 || got[0] != "chat-nhom/bo-phieu-hoac-doi-phieu" {
		t.Errorf("the model's rewrite of «bo fieu»: %v", got)
	}
	if a, b := idCua(Tim(ctx, Hoi{Cau: "fieu", K: 3})), idCua(Tim(ctx, Hoi{Cau: "phieu", K: 3})); reflect.DeepEqual(a, b) {
		t.Errorf("«fieu» was rewritten to «phieu» by Go: %v", a)
	}
	if got := Tim(ctx, Hoi{Cau: "làm sao thì được không", K: 3}); len(got) == 0 {
		t.Error("question words were gated out")
	}
	for _, q := range []string{"", "   ", "?!"} {
		if got := Tim(ctx, Hoi{Cau: q, Man: "plan"}); got != nil {
			t.Errorf("%q returned %v", q, idCua(got))
		}
	}
}

// Tim reads at most MaxRuneCau runes of a question: what comes after is not
// folded, not ranked, and cannot slow it down.
func TestTimCatCau(t *testing.T) {
	ctx := context.Background()
	// A megabyte of a word the manual does not know, then a real question.
	dai := strings.Repeat("xyz ", 1<<18) + "đăng xuất"
	if got := Tim(ctx, Hoi{Cau: dai, K: 5}); len(got) != 0 {
		t.Errorf("words after the cap were read: %v", idCua(got))
	}
	if got := Tim(ctx, Hoi{Cau: "xyz đăng xuất", K: 5}); len(got) == 0 {
		t.Fatal("identity: the same words inside the cap must match")
	}
	// Within the cap the question is read whole.
	q := "tạo kèo " + strings.Repeat("ờ ", 1<<19)
	if a, b := idCua(Tim(ctx, Hoi{Cau: q, K: 10})), idCua(Tim(ctx, Hoi{Cau: catCau(q), K: 10})); !reflect.DeepEqual(a, b) || len(a) == 0 {
		t.Errorf("long question %v, its first %d runes %v", a, MaxRuneCau, b)
	}
	tot := time.Hour
	for i := 0; i < 3; i++ {
		bd := time.Now()
		Tim(ctx, Hoi{Cau: dai, K: 5})
		if d := time.Since(bd); d < tot {
			tot = d
		}
	}
	t.Logf("%d-byte question: %v", len(dai), tot)
	if tot > 50*time.Millisecond {
		t.Errorf("%d-byte question took %v, bound 50ms", len(dai), tot)
	}
	for _, c := range []struct {
		in   string
		rune int
	}{
		{"", 0}, {"tạo kèo", 7}, {strings.Repeat("ạ", MaxRuneCau), MaxRuneCau}, {strings.Repeat("ạ", MaxRuneCau+1), MaxRuneCau},
		{strings.Repeat("\xff", MaxRuneCau+5), MaxRuneCau},
	} {
		if got := utf8.RuneCountInString(catCau(c.in)); got != c.rune {
			t.Errorf("catCau of %d bytes kept %d runes, want %d", len(c.in), got, c.rune)
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
	if len(a) == 0 || len(a[0].Buoc) == 0 || len(a[0].Nhan) == 0 {
		t.Fatalf("Tim(bỏ phiếu) = %+v: nothing to edit", a)
	}
	a[0].Buoc[0] = "đã bị sửa"
	a[0].Nhan[0] = "đã bị sửa"
	b := TheoMan("groups/[id]/chat")
	if len(b) < 3 || len(b[2].Buoc) < 2 {
		t.Fatalf("TheoMan(chat) = %+v: nothing to edit", b)
	}
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

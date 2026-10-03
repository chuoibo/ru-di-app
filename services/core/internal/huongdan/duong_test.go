package huongdan

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
	"testing/fstest"
)

// buoc renders a way as «den[nhan]» per step, compact enough for a table.
func buoc(bs []Buoc) []string {
	out := []string{}
	for _, b := range bs {
		out = append(out, b.Den+"["+b.Nhan+"]")
	}
	return out
}

func TestDuongToiTrenDuLieuThat(t *testing.T) {
	cases := []struct {
		tu, den string
		ok      bool
		want    []string
	}{
		// One tap on the tab bar.
		{"messages", "explore", true, []string{"explore[Khám phá]"}},
		// Between two tabs the tab wins over «Tới Tin nhắn», a button of one state.
		{"plan", "groups/[id]/chat", true, []string{"messages[Tin nhắn]", "groups/[id]/chat[Mở nhóm …]"}},
		{"messages", "outings/[id]", true, []string{"groups/[id]/chat[Mở nhóm …]", "outings/[id][Mở kèo của hội]"}},
		{"messages", "finance", true, []string{"profile[Cá nhân]", "finance[Tài chính của tôi]"}},
		// Two ways of two steps; the labelled one («Xem quyết toán») passes
		// through the settlement screen, so the unlabelled one wins (review 13).
		{"finance", "messages", true, []string{"explore[]", "messages[Tin nhắn]"}},
		// The «Tạo mới» stamp is on the strip over Khám phá too (03/10): no detour through Lên plan.
		{"explore", "smart-split/[id]/review", true, []string{"create[Tạo mới]", "smart-split/[id]/review[Chia hóa đơn]"}},
		// Two ways of two steps since main's memory-book merge (2026-09-28):
		// «Rủ … tới đây» through the pair notebook, whose second step has no
		// label (to-giay.md does not declare «Rủ hội mình đi chơi»), and «Thêm
		// vào kèo» then «Tạo kèo». Since 02/10 the way with fewer unlabelled
		// steps wins before the route id: Nếp can name every tap of it.
		{"places/[id]", "outings/new", true, []string{"outings/chon[Thêm vào kèo]", "outings/new[Tạo kèo]"}},
		{"groups/[id]/to-giay", "places/[id]", true, []string{"outings/[id][Xem kèo]", "places/[id][Chặng …]"}},
		// Only the code knows the second edge: no label, and TieuDe empty too
		// because settings/phien has no manual.
		{"profile", "settings/phien", true, []string{"settings[Cài đặt]", "settings/phien[]"}},
		// As walked, and to itself.
		{"/outings/7", "outings/[id]", true, []string{}},
		{"/groups/9/chat?x=1", "/outings/new", true, []string{"outings/new[Tự tạo kèo]"}},
		// The only way out passes through welcome: signing out is not directions.
		{"settings/xoa-tai-khoan", "explore", false, nil},
		{"destinations", "plan", false, nil},
		{"khong-co", "plan", false, nil},
		{"plan", "khong-co", false, nil},
	}
	for _, c := range cases {
		got, ok := DuongToi(c.tu, c.den)
		if ok != c.ok || (ok && !reflect.DeepEqual(buoc(got), c.want)) || (!ok && got != nil) {
			t.Errorf("DuongToi(%s, %s) = %v, %v; want %v, %v", c.tu, c.den, buoc(got), ok, c.want, c.ok)
		}
	}
	got, _ := DuongToi("messages", "outings/[id]")
	if got[0].Tu != "messages" || got[1].Tu != "groups/[id]/chat" || got[1].TieuDe != "Kèo" || got[0].TieuDe != "Chat nhóm" {
		t.Fatalf("steps %+v", got)
	}
}

// khoangCach is an independent breadth-first search over the same edges,
// written the plain way, as the oracle for every pair below.
func khoangCach(s *SoTay, tu, den string) int {
	if tu == den {
		return 0
	}
	xa := map[string]int{tu: 0}
	hang := []string{tu}
	for len(hang) > 0 {
		u := hang[0]
		hang = hang[1:]
		for _, c := range s.ke[u] {
			if _, ok := xa[c.Den]; ok || (manVao[c.Den] && c.Den != den) {
				continue
			}
			xa[c.Den] = xa[u] + 1
			if c.Den == den {
				return xa[c.Den]
			}
			hang = append(hang, c.Den)
		}
	}
	return -1
}

// khoangCachTranhTien is khoangCach on the graph where a money screen may be
// the destination but not a screen passed through.
func khoangCachTranhTien(s *SoTay, tu, den string) int {
	if tu == den {
		return 0
	}
	xa := map[string]int{tu: 0}
	hang := []string{tu}
	for len(hang) > 0 {
		u := hang[0]
		hang = hang[1:]
		for _, c := range s.ke[u] {
			if _, ok := xa[c.Den]; ok || (c.Den != den && (manVao[c.Den] || laManTien(c.Den))) {
				continue
			}
			xa[c.Den] = xa[u] + 1
			if c.Den == den {
				return xa[c.Den]
			}
			hang = append(hang, c.Den)
		}
	}
	return -1
}

// Every pair of the 50 routes: the way is a real chain of edges, never passes
// through a sign-in screen, carries each edge's label, is exactly as long as
// the oracle's shortest distance (or absent when that is over MaxBuoc), and
// passes through no money screen when a way of that length avoids them all.
func TestDuongToiLaNganNhatMoiCap(t *testing.T) {
	var soCap, coDuong, quaTienBatBuoc, tranhDuoc int
	for _, tu := range soTay.cacMan {
		for _, den := range soTay.cacMan {
			soCap++
			got, ok := soTay.duongToi(tu, den)
			d := khoangCach(soTay, tu, den)
			if want := d >= 0 && d <= MaxBuoc; ok != want {
				t.Errorf("%s -> %s: ok %v, oracle distance %d", tu, den, ok, d)
				continue
			}
			if !ok {
				continue
			}
			coDuong++
			if len(got) != d {
				t.Errorf("%s -> %s: %d steps %v, shortest is %d", tu, den, len(got), buoc(got), d)
			}
			cur := tu
			for i, b := range got {
				if b.Tu != cur {
					t.Errorf("%s -> %s: step %d starts at %s, not %s", tu, den, i, b.Tu, cur)
				}
				var c *canh
				for j := range soTay.ke[b.Tu] {
					if soTay.ke[b.Tu][j].Den == b.Den {
						c = &soTay.ke[b.Tu][j]
					}
				}
				if c == nil || c.Nhan != b.Nhan {
					t.Errorf("%s -> %s: step %v is not an edge with that label", tu, den, b)
				}
				if i < len(got)-1 && manVao[b.Den] {
					t.Errorf("%s -> %s passes through %s", tu, den, b.Den)
				}
				cur = b.Den
			}
			if cur != den {
				t.Errorf("%s -> %s ends at %s", tu, den, cur)
			}
			quaTien := 0
			for i, b := range got {
				if i < len(got)-1 && laManTien(b.Den) {
					quaTien++
				}
			}
			if quaTien > 0 {
				if khoangCachTranhTien(soTay, tu, den) == d {
					t.Errorf("%s -> %s: %v passes through a money screen; a way of %d steps avoids them", tu, den, buoc(got), d)
				} else {
					quaTienBatBuoc++
				}
			} else if tu != den && d > 1 {
				tranhDuoc++
			}
		}
	}
	// 60 routes since main's community board and memory books (2026-09-27):
	// 3600 ordered pairs, 50 before.
	if soCap != 3600 || coDuong < 1500 {
		t.Fatalf("%d pairs, %d with a way: the graph is not the one embedded", soCap, coDuong)
	}
	t.Logf("%d pairs, %d with a way of at most %d steps; %d pass through a money screen because every way that short does, %d of two steps or more pass through none",
		soCap, coDuong, MaxBuoc, quaTienBatBuoc, tranhDuoc)
}

// doThiGia builds a SoTay holding only a graph, for shapes the real data does
// not have. Edges are «tu>den» or «tu>den:label».
func doThiGia(canhs ...string) *SoTay {
	s := &SoTay{coMan: map[string]bool{}, ke: map[string][]canh{}, nguoc: map[string][]string{}, trangCua: map[string]*trang{}}
	for _, e := range canhs {
		tu, rest, _ := strings.Cut(e, ">")
		den, nhan, _ := strings.Cut(rest, ":")
		for _, m := range []string{tu, den} {
			if !s.coMan[m] {
				s.coMan[m] = true
				s.cacMan = append(s.cacMan, m)
			}
		}
		s.ke[tu] = append(s.ke[tu], canh{Den: den, Nhan: nhan})
		s.nguoc[den] = append(s.nguoc[den], tu)
	}
	sort.Strings(s.cacMan)
	for k := range s.ke {
		sort.Slice(s.ke[k], func(i, j int) bool { return s.ke[k][i].Den < s.ke[k][j].Den })
	}
	for k := range s.nguoc {
		sort.Strings(s.nguoc[k])
	}
	return s
}

func TestDuongToiNganNhatKhongPhaiDauTien(t *testing.T) {
	// Walking neighbours in order, the first way found is a>b>c>d>z (four
	// steps); the shortest is a>y>z.
	s := doThiGia("a>b", "b>c", "c>d", "d>z", "a>y", "y>z")
	got, ok := s.duongToi("a", "z")
	if !ok || !reflect.DeepEqual(buoc(got), []string{"y[]", "z[]"}) {
		t.Fatalf("a -> z = %v, %v; want [y z]", buoc(got), ok)
	}
}

func TestDuongToiDongDaiXacDinh(t *testing.T) {
	// Two ways of two steps: the labelled edge wins over the smaller id.
	s := doThiGia("a>m", "a>n:Nút N", "m>z", "n>z")
	if got, _ := s.duongToi("a", "z"); !reflect.DeepEqual(buoc(got), []string{"n[Nút N]", "z[]"}) {
		t.Fatalf("labelled tie: %v", buoc(got))
	}
	// Neither labelled: the smaller id, on every run.
	s = doThiGia("a>q", "a>p", "q>z", "p>z")
	for i := 0; i < 20; i++ {
		if got, _ := s.duongToi("a", "z"); !reflect.DeepEqual(buoc(got), []string{"p[]", "z[]"}) {
			t.Fatalf("unlabelled tie, run %d: %v", i, buoc(got))
		}
	}
}

// Money screens are avoided along the whole way, not only on the next step:
// at a, b is labelled and not a money screen, but the only way on from b
// passes through finance; c starts a way of the same length that does not.
func TestDuongToiTranhManTien(t *testing.T) {
	s := doThiGia("a>b:Nút B", "a>c", "b>finance:Xem", "finance>z", "c>e", "e>z")
	if got, _ := s.duongToi("a", "z"); !reflect.DeepEqual(buoc(got), []string{"c[]", "e[]", "z[]"}) {
		t.Fatalf("a -> z = %v: passed through finance", buoc(got))
	}
	// The fewest, not the most: from c one way on passes through
	// settlements/[id] and one does not, so c costs nothing and beats b,
	// whose only way on is through finance (review 13 round 2, N3). No label
	// anywhere, so neither the label rule nor the smaller id (b < c) decides.
	// The clean neighbour of c is named once before settlements/[id] (e) and
	// once after it (w), so neither the first nor the last neighbour of c is
	// always the cheap one.
	for _, sach := range []string{"e", "w"} {
		s = doThiGia("a>b", "a>c", "b>finance", "finance>z", "c>settlements/[id]", "settlements/[id]>z", "c>"+sach, sach+">z")
		if got, _ := s.duongToi("a", "z"); !reflect.DeepEqual(buoc(got), []string{"c[]", sach + "[]", "z[]"}) {
			t.Fatalf("a -> z = %v: not the way with the fewest money screens", buoc(got))
		}
	}
	// Counted along the whole rest of the way, not only one step ahead: the
	// reviewer's graph (review 13 round 3, NF1). finance is two steps past b,
	// so b and x each look clean one step ahead, and b has the smaller id.
	s = doThiGia("a>b", "a>c", "b>x", "x>finance", "finance>z", "c>y", "y>w", "w>z")
	if got, _ := s.duongToi("a", "z"); !reflect.DeepEqual(buoc(got), []string{"c[]", "y[]", "w[]", "z[]"}) {
		t.Fatalf("a -> z = %v: passed through finance two steps ahead", buoc(got))
	}
	// Counted, not «any or none»: when every way that short passes through a
	// money screen, a way through one beats a way through two, even when its
	// money screen is the very next step and the other way starts on a clean
	// screen with the smaller id.
	s = doThiGia("a>finance", "finance>p", "p>q", "q>z", "a>c", "c>settlements/[id]", "settlements/[id]>batches/[id]", "batches/[id]>z")
	if got, _ := s.duongToi("a", "z"); !reflect.DeepEqual(buoc(got), []string{"finance[]", "p[]", "q[]", "z[]"}) {
		t.Fatalf("a -> z = %v: not the way through the fewest money screens", buoc(got))
	}
	// When every shortest way passes through one, it is taken; a longer way
	// is never preferred to it.
	s = doThiGia("a>settlements/[id]", "settlements/[id]>z", "a>c", "c>e", "e>z")
	if got, _ := s.duongToi("a", "z"); !reflect.DeepEqual(buoc(got), []string{"settlements/[id][]", "z[]"}) {
		t.Fatalf("a -> z = %v: took the longer way", buoc(got))
	}
	// Ending on a money screen is not passing through it.
	s = doThiGia("a>finance:Tài chính", "a>b", "b>finance")
	if got, _ := s.duongToi("a", "finance"); !reflect.DeepEqual(buoc(got), []string{"finance[Tài chính]"}) {
		t.Fatalf("a -> finance = %v", buoc(got))
	}
}

// manTienDau is MAN_NEP_LUI of phieu.ts: read here, not copied.
func TestManTienDauKhopManNepLui(t *testing.T) {
	if lui := manNepLui(t); !reflect.DeepEqual(lui, manTienDau) {
		t.Fatalf("manTienDau %v, MAN_NEP_LUI %v", manTienDau, lui)
	}
}

func TestDuongToiToiDaNamBuoc(t *testing.T) {
	chuoi := func(n int) *SoTay {
		var e []string
		for i := 0; i < n; i++ {
			e = append(e, fmt.Sprintf("m%d>m%d", i, i+1))
		}
		return doThiGia(e...)
	}
	if got, ok := chuoi(MaxBuoc).duongToi("m0", fmt.Sprintf("m%d", MaxBuoc)); !ok || len(got) != MaxBuoc {
		t.Fatalf("%d steps: %v %v", MaxBuoc, buoc(got), ok)
	}
	if got, ok := chuoi(MaxBuoc+1).duongToi("m0", fmt.Sprintf("m%d", MaxBuoc+1)); ok || got != nil {
		t.Fatalf("%d steps must be refused: %v", MaxBuoc+1, buoc(got))
	}
}

func TestDuongToiKhongQuaManDangNhap(t *testing.T) {
	s := doThiGia("a>welcome", "welcome>z", "a>b", "b>c", "c>z")
	if got, _ := s.duongToi("a", "z"); !reflect.DeepEqual(buoc(got), []string{"b[]", "c[]", "z[]"}) {
		t.Fatalf("a -> z = %v: went through welcome", buoc(got))
	}
	if got, ok := s.duongToi("a", "welcome"); !ok || len(got) != 1 {
		t.Fatalf("ending on welcome is allowed: %v %v", buoc(got), ok)
	}
	if got, ok := s.duongToi("welcome", "z"); !ok || len(got) != 1 {
		t.Fatalf("starting on welcome is allowed: %v %v", buoc(got), ok)
	}
}

func TestManVaoCoTrongRut(t *testing.T) {
	for m := range manVao {
		if !soTay.coMan[m] {
			t.Errorf("manVao lists %q, which _rut.json does not have", m)
		}
	}
}

func TestChuanMan(t *testing.T) {
	for in, want := range map[string]string{
		"outings/[id]":             "outings/[id]",
		"/outings/7":               "outings/[id]",
		"/outings/new":             "outings/new",
		"/outings/new?nepNhap=x":   "outings/new",
		"/groups/abc/to-giay?ru=1": "groups/[id]/to-giay",
		"/(tabs)/plan":             "plan",
		"plan":                     "plan",
		"/":                        "index",
		"":                         "",
		"khong-co":                 "",
		"/outings/7/khong-co":      "",
	} {
		if got := ChuanMan(in); got != want {
			t.Errorf("ChuanMan(%q) = %q, want %q", in, got, want)
		}
	}
}

// tabLayout reads the columns app/(tabs)/_layout.tsx renders: name -> title.
// A screen marked `href: null` (Cộng đồng, Khám phá's second section) is a
// route of the navigator with no column on the strip, so it is not a tab here.
func tabLayout(t *testing.T) map[string]string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "apps", "mobile", "app", "(tabs)", "_layout.tsx"))
	if err != nil {
		t.Fatal(err)
	}
	layout := map[string]string{}
	tieuDe := regexp.MustCompile(`title:\s*"([^"]+)"`)
	for _, m := range regexp.MustCompile(`<Tabs\.Screen\s+name="([^"]+)"\s+options=\{\{([^}]*)\}\}`).FindAllStringSubmatch(string(raw), -1) {
		if regexp.MustCompile(`href:\s*null`).MatchString(m[2]) {
			continue
		}
		if td := tieuDe.FindStringSubmatch(m[2]); td != nil {
			layout[m[1]] = td[1]
		}
	}
	if len(layout) < 4 {
		t.Fatalf("read %d tabs from _layout.tsx", len(layout))
	}
	return layout
}

// lechTab lists how the graph's tab set differs from the layout's: a tab the
// graph has and the layout does not render («+»), and the reverse («-»).
func lechTab(tab []string, layout map[string]string) []string {
	var out []string
	co := map[string]bool{}
	for _, m := range tab {
		co[m] = true
		if _, ok := layout[m]; !ok {
			out = append(out, "+"+m)
		}
	}
	for m := range layout {
		if !co[m] {
			out = append(out, "-"+m)
		}
	}
	sort.Strings(out)
	return out
}

// The graph knows exactly the tabs the layout renders. The reviewer's probe:
// marking settings as a tab file in _rut.json gave the graph a fifth tab and
// every test stayed green; it is refused here.
func TestTabRutBangTabLayout(t *testing.T) {
	layout := tabLayout(t)
	if lech := lechTab(soTay.tab, layout); len(lech) != 0 {
		t.Fatalf("_rut.json tabs %v, _layout.tsx tabs %v: %v", soTay.tab, layout, lech)
	}
	m := banSaoDuLieu(t)
	rut := string(m[duongRut].Data)
	cu := `"app/settings/index.tsx"`
	if !strings.Contains(rut, cu) {
		t.Fatal("_rut.json no longer lists app/settings/index.tsx")
	}
	m[duongRut] = &fstest.MapFile{Data: []byte(strings.Replace(rut, cu, `"app/(tabs)/settings.tsx",`+"\n        "+cu, 1))}
	gia, err := nap(m)
	if err != nil {
		t.Fatalf("the probe must still load (the rule is here, not in nap): %v", err)
	}
	if lech := lechTab(gia.tab, layout); !reflect.DeepEqual(lech, []string{"+settings"}) {
		t.Fatalf("fake tab: %v, want [+settings]", lech)
	}
	// The other way: a tab the layout renders whose route file moved out of
	// app/(tabs)/ in _rut.json. Lên plan, not Khám phá: Khám phá hosts
	// Cộng đồng, and a host that is no column does not load at all
	// (TestMucTrongTabSai).
	m = banSaoDuLieu(t)
	rut = string(m[duongRut].Data)
	if !strings.Contains(rut, `"app/(tabs)/plan.tsx"`) {
		t.Fatal("_rut.json no longer lists app/(tabs)/plan.tsx")
	}
	m[duongRut] = &fstest.MapFile{Data: []byte(strings.Replace(rut, `"app/(tabs)/plan.tsx"`, `"app/plan.tsx"`, 1))}
	if gia, err = nap(m); err != nil {
		t.Fatalf("the second probe must still load: %v", err)
	}
	if lech := lechTab(gia.tab, layout); !reflect.DeepEqual(lech, []string{"-plan"}) {
		t.Fatalf("missing tab: %v, want [-plan]", lech)
	}
	// And a route with no column that _rut.json forgets to mark: the graph
	// would count Cộng đồng as a column the strip does not draw.
	m = banSaoDuLieu(t)
	rut = string(m[duongRut].Data)
	cot := `"community": "explore"`
	if !strings.Contains(rut, cot) {
		t.Fatal("_rut.json no longer marks community as hosted by explore")
	}
	m[duongRut] = &fstest.MapFile{Data: []byte(strings.Replace(rut, `"muc_trong_tab": {
    `+cot+`
  },`, `"muc_trong_tab": {},`, 1))}
	if gia, err = nap(m); err != nil {
		t.Fatalf("the third probe must still load: %v", err)
	}
	if lech := lechTab(gia.tab, layout); !reflect.DeepEqual(lech, []string{"+community"}) {
		t.Fatalf("unmarked route with no column: %v, want [+community]", lech)
	}
}

// Cộng đồng is Khám phá's second section: a tab route the strip draws no
// column for. It is not one of the strip's tabs; the strip is still on screen
// over it, so every other column is one tap away, labelled with its title,
// except the host column, which is the lit one (tapping it does nothing).
func TestRouteKhongCot(t *testing.T) {
	if soTay.banDo.tab["community"] {
		t.Fatal("community counted as a column of the strip")
	}
	if got := soTay.banDo.chu["community"]; got != "explore" {
		t.Fatalf("host column of community: %q, want explore", got)
	}
	layout := tabLayout(t)
	for _, den := range []string{"plan", "messages", "profile"} {
		got, ok := soTay.duongToi("community", den)
		if !ok || len(got) != 1 || got[0].Nhan != layout[den] {
			t.Errorf("community -> %s: %v, want one tap on «%s»", den, buoc(got), layout[den])
		}
	}
	for _, c := range soTay.ke["community"] {
		if c.Den == "explore" && c.Nhan == layout["explore"] {
			t.Errorf("community -> explore labelled «%s»: the lit column does nothing", c.Nhan)
		}
	}
	if soTay.banDo.laCanhMa("community", "explore") && !soTay.banDo.diToi["community"]["explore"] {
		t.Error("community -> explore counted as a strip edge")
	}
}

// The tab label of every tab is the title the layout gives it.
func TestTabKhopLayout(t *testing.T) {
	layout := tabLayout(t)
	// Every tab reaches every other tab in one tap labelled with its title.
	soCanh := 0
	for a := range layout {
		if !soTay.coMan[a] {
			t.Errorf("tab %q is not a route in _rut.json", a)
		}
		for b, tieuDe := range layout {
			if a == b {
				continue
			}
			got, ok := soTay.duongToi(a, b)
			if !ok || len(got) != 1 || got[0].Nhan != tieuDe {
				t.Errorf("%s -> %s: %v, want one tap on «%s»", a, b, buoc(got), tieuDe)
			}
			soCanh++
		}
	}
	// And every edge into a tab from a tab carries that tab's title.
	for _, m := range soTay.cacMan {
		for _, c := range soTay.ke[m] {
			if _, laTab := layout[c.Den]; !laTab {
				continue
			}
			if _, tuTab := layout[m]; tuTab && c.Nhan != layout[c.Den] {
				t.Errorf("tab edge %s -> %s labelled %q", m, c.Den, c.Nhan)
			}
		}
	}
	if soCanh != len(layout)*(len(layout)-1) {
		t.Fatalf("checked %d tab edges", soCanh)
	}
}

// From another tab Cộng đồng is two taps: the Khám phá column, then
// «Cộng đồng» in Khám phá's header; back from it is «Địa điểm» in the same
// header. The labels are the manuals' (kham-pha.md, cong-dong.md), the edges
// the route files' navigations.
func TestCongDongLaMucCuaKhamPha(t *testing.T) {
	want := []struct {
		tu, den string
		nhan    []string
	}{
		{"plan", "community", []string{"Khám phá", "Cộng đồng"}},
		{"explore", "community", []string{"Cộng đồng"}},
		{"community", "explore", []string{"Địa điểm"}},
	}
	for _, w := range want {
		got, ok := soTay.duongToi(w.tu, w.den)
		if !ok || len(got) != len(w.nhan) {
			t.Errorf("%s -> %s: %v, want %v", w.tu, w.den, buoc(got), w.nhan)
			continue
		}
		for i := range w.nhan {
			if got[i].Nhan != w.nhan[i] {
				t.Errorf("%s -> %s step %d: «%s», want «%s»", w.tu, w.den, i, got[i].Nhan, w.nhan[i])
			}
		}
	}
}

// The «Tạo mới» stamp sits in the middle of the strip on every screen the
// strip is drawn over, Cộng đồng included (ui/ConDauTao.tsx pushes /create):
// from any of them, writing a post is the stamp, then «Viết bài» on the desk.
// Before the stamp was in the graph, Nếp routed a question from Khám phá
// through Lên plan's own «Tạo mới» button, three taps for a two-tap errand.
func TestConDauTaoTuMoiManCoThanh(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "apps", "mobile", "src", "rudi", "ui", "ConDauTao.tsx"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"/create"`) || !strings.Contains(string(raw), `accessibilityLabel="`+nhanConDau+`"`) {
		t.Fatalf("ConDauTao.tsx no longer pushes /create under the label «%s»", nhanConDau)
	}
	for _, tu := range []string{"explore", "community", "plan", "messages", "profile"} {
		got, ok := soTay.duongToi(tu, "community/new")
		if !ok || len(got) != 2 || got[0].Nhan != nhanConDau || got[0].Den != manConDau || got[1].Nhan != "Viết bài" {
			t.Errorf("%s -> community/new: %v, want [%s, Viết bài]", tu, buoc(got), nhanConDau)
		}
	}
}

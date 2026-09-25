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
		{"explore", "smart-split/[id]/review", true, []string{"plan[Lên plan]", "create[Tạo mới]", "smart-split/[id]/review[Chia hóa đơn]"}},
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

// Every pair of the 50 routes: the way is a real chain of edges, never passes
// through a sign-in screen, carries each edge's label, and is exactly as long
// as the oracle's shortest distance (or absent when that is over MaxBuoc).
func TestDuongToiLaNganNhatMoiCap(t *testing.T) {
	var soCap, coDuong int
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
		}
	}
	if soCap != 2500 || coDuong < 1500 {
		t.Fatalf("%d pairs, %d with a way: the graph is not the one embedded", soCap, coDuong)
	}
	t.Logf("%d pairs, %d with a way of at most %d steps", soCap, coDuong, MaxBuoc)
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

// The tab set is read from _rut.json (route files under app/(tabs)/) and the
// tab label from each tab's manual. Both are held here to the tab layout the
// app actually renders.
func TestTabKhopLayout(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "apps", "mobile", "app", "(tabs)", "_layout.tsx"))
	if err != nil {
		t.Fatal(err)
	}
	layout := map[string]string{}
	for _, m := range regexp.MustCompile(`<Tabs\.Screen\s+name="([^"]+)"\s+options=\{\{\s*title:\s*"([^"]+)"`).FindAllStringSubmatch(string(raw), -1) {
		layout[m[1]] = m[2]
	}
	if len(layout) < 4 {
		t.Fatalf("read %d tabs from _layout.tsx", len(layout))
	}
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
	// And the graph knows no tab that the layout does not render.
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

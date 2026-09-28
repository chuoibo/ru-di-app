package huongdan

import (
	"sort"
	"strings"
)

// manVao are the sign-in and first-run screens. The code links to them from
// guards and from «Đăng xuất», so the graph has edges into them, but a way
// that passes through one is «log out and sign in again», never directions.
// DuongToi may start or end on one; it never passes through one. A test holds
// every id here to a route _rut.json knows.
var manVao = map[string]bool{"index": true, "welcome": true, "login": true, "otp": true}

// manTienDau are the first route segments of the money screens: MAN_NEP_LUI in
// apps/mobile/src/rudi/nep/phieu.ts, which a test holds equal to this. A
// manual on such a route must say tien: true (nap refuses it otherwise), and
// DuongToi passes through one only when no way of the same length avoids it.
var manTienDau = map[string]bool{"finance": true, "settlements": true, "batches": true, "smart-split": true}

// laManTien reports whether a route id is a money screen.
func laManTien(man string) bool {
	return manTienDau[strings.SplitN(man, "/", 2)[0]]
}

// canh is one edge of the screen graph.
type canh struct {
	Den  string
	Nhan string
}

// dungDoThi builds the screen graph from three sources, all read from the
// embedded data:
//
//  1. _rut.json `di_toi`: every navigation the code does with a literal target
//     (router.push/replace/navigate, href). No label: the code does not say
//     which button it hangs on.
//  2. The tab bar: every route whose route file sits in app/(tabs)/ (read
//     from `tep`, kept in s.tab) is one tap from every other such route. The
//     label is the tab's title, which is its manual's tieu_de; a test holds
//     the set of tabs and each title to app/(tabs)/_layout.tsx.
//  3. The manuals' di_toi, which carry the label a person taps. The first one
//     a manual declares for a pair of screens is the one used, so the author
//     orders them.
//
// Between two tabs the tab title wins: the bar is on screen in every state,
// while a manual's di_toi between them can be a button of one state only
// («Tới Tin nhắn» shows on Lên plan when no group is chosen). Otherwise a
// manual's label wins over none. The first non-empty label set for a pair is
// kept. Self-loops are dropped.
func (s *SoTay) dungDoThi(rut *banRut) {
	nhan := map[string]map[string]string{}
	them := func(tu, den, n string) {
		if tu == den {
			return
		}
		if nhan[tu] == nil {
			nhan[tu] = map[string]string{}
		}
		if cu, ok := nhan[tu][den]; !ok || (cu == "" && n != "") {
			nhan[tu][den] = n
		}
	}
	for _, a := range s.tab {
		for _, b := range s.tab {
			n := ""
			if t, ok := s.trangCua[b]; ok {
				n = t.tieuDe
			}
			them(a, b, n)
		}
	}
	for _, t := range s.trang {
		for _, d := range t.diToi {
			them(t.man, d.Man, d.Nhan)
		}
	}
	for _, r := range rut.Routes {
		for _, d := range r.DiToi {
			them(r.Man, d, "")
		}
	}

	s.ke = map[string][]canh{}
	s.nguoc = map[string][]string{}
	for tu, dich := range nhan {
		for den, n := range dich {
			s.ke[tu] = append(s.ke[tu], canh{Den: den, Nhan: n})
			s.nguoc[den] = append(s.nguoc[den], tu)
		}
	}
	for tu := range s.ke {
		sort.Slice(s.ke[tu], func(i, j int) bool { return s.ke[tu][i].Den < s.ke[tu][j].Den })
	}
	for den := range s.nguoc {
		sort.Strings(s.nguoc[den])
	}
}

// duongToi walks back from den over reversed edges to learn how far each
// screen is from it (at most MaxBuoc), then walks forward from tu taking, at
// each screen, an edge that is one step closer. Every such walk is a shortest
// way. Among them it takes, in this order: the fewest money screens passed
// through (qua, counted along the rest of the way, so a first step that looks
// clean but forces a money screen later loses), then a labelled edge, then
// the smallest route id; the same way on every run. «How do I get to Tin
// nhắn» from Tài chính is not an errand through the settlement screen.
func (s *SoTay) duongToi(tu, den string) ([]Buoc, bool) {
	tu, den = s.chuanMan(tu), s.chuanMan(den)
	if tu == "" || den == "" {
		return nil, false
	}
	if tu == den {
		return []Buoc{}, true
	}
	xa := map[string]int{den: 0}
	thuTu := []string{den}
	for i := 0; i < len(thuTu); i++ {
		v := thuTu[i]
		if xa[v] == MaxBuoc {
			continue
		}
		for _, u := range s.nguoc[v] {
			if _, ok := xa[u]; ok {
				continue
			}
			if manVao[u] && u != tu {
				continue
			}
			xa[u] = xa[v] + 1
			thuTu = append(thuTu, u)
		}
	}
	n, ok := xa[tu]
	if !ok {
		return nil, false
	}
	// gia is what stepping onto a screen costs: one if it is a money screen
	// passed through (den itself is where the person wants to be).
	gia := func(man string) int {
		if man != den && laManTien(man) {
			return 1
		}
		return 0
	}
	// qua[v] is the fewest money screens a shortest way from v to den passes
	// through after v. thuTu is in order of distance, so every screen one step
	// closer is settled before the screens that lead to it.
	qua := map[string]int{den: 0}
	for _, v := range thuTu[1:] {
		itNhat := -1
		for _, c := range s.ke[v] {
			if d, ok := xa[c.Den]; !ok || d != xa[v]-1 {
				continue
			}
			if q := gia(c.Den) + qua[c.Den]; itNhat < 0 || q < itNhat {
				itNhat = q
			}
		}
		qua[v] = itNhat
	}
	out := make([]Buoc, 0, n)
	for cur := tu; cur != den; {
		var chon *canh
		for i, c := range s.ke[cur] {
			if d, ok := xa[c.Den]; !ok || d != xa[cur]-1 {
				continue
			}
			if chon == nil || tot(&s.ke[cur][i], chon, gia, qua) {
				chon = &s.ke[cur][i]
			}
		}
		b := Buoc{Tu: cur, Den: chon.Den, Nhan: chon.Nhan}
		if t, ok := s.trangCua[chon.Den]; ok {
			b.TieuDe = t.tieuDe
		}
		out = append(out, b)
		cur = chon.Den
	}
	return out, true
}

// tot reports whether edge a is a better next step than edge b, both one step
// closer to the destination: fewer money screens on the rest of the way, then
// a label over none, then the smaller route id.
func tot(a, b *canh, gia func(string) int, qua map[string]int) bool {
	if qa, qb := gia(a.Den)+qua[a.Den], gia(b.Den)+qua[b.Den]; qa != qb {
		return qa < qb
	}
	if (a.Nhan != "") != (b.Nhan != "") {
		return a.Nhan != ""
	}
	return a.Den < b.Den
}

// chuanMan maps a screen as walked or as declared to its route id: the query
// and fragment are cut, `(group)` segments dropped, and each segment matched
// against the route tree, a static segment beating a `[param]` one, then the
// smaller id. The same rule as manDich in apps/mobile/tools/rut-huong-dan.mjs.
func (s *SoTay) chuanMan(man string) string {
	if s.coMan[man] {
		return man
	}
	cat := strings.TrimLeft(strings.SplitN(strings.SplitN(man, "?", 2)[0], "#", 2)[0], "/")
	var doan []string
	for _, d := range strings.Split(cat, "/") {
		if d == "" || (strings.HasPrefix(d, "(") && strings.HasSuffix(d, ")")) {
			continue
		}
		doan = append(doan, d)
	}
	if len(doan) == 0 {
		if s.coMan["index"] && strings.HasPrefix(man, "/") {
			return "index"
		}
		return ""
	}
	tot, diemTot := "", -1
	for _, m := range s.cacMan {
		md := strings.Split(m, "/")
		if m == "index" || len(md) != len(doan) {
			continue
		}
		diem, khop := 0, true
		for i := range md {
			if strings.HasPrefix(md[i], "[") && strings.HasSuffix(md[i], "]") {
				continue
			}
			if md[i] != doan[i] {
				khop = false
				break
			}
			diem++
		}
		if khop && diem > diemTot {
			tot, diemTot = m, diem
		}
	}
	return tot
}

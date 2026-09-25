package rag

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"mobile/services/core/internal/repo"
)

// The golden retrieval set (design 04 §8.3): testdata/truy-hoi-dia-diem.json
// holds 46 hand-written anchor places, one hand-made takedown and 67 queries
// in seven groups; sinhNen adds 240 background places deterministically. The
// same set runs on both paths -- the live-row path here, the index path in
// vang_postgres_test.go -- and each pins its own numbers.

type diemDenMau struct {
	ID    string  `json:"id"`
	Ten   string  `json:"ten"`
	Tinh  string  `json:"tinh"`
	Lat   float64 `json:"lat"`
	Lng   float64 `json:"lng"`
	Nam   float64 `json:"nam"`
	Tay   float64 `json:"tay"`
	Bac   float64 `json:"bac"`
	Dong  float64 `json:"dong"`
	ThuTu int64   `json:"thu_tu"`
}

type quanMau struct {
	ID       string   `json:"id"`
	DiemDen  string   `json:"diem_den"`
	Ten      string   `json:"ten"`
	Loai     string   `json:"loai"`
	Kinds    []string `json:"kinds"`
	DiaChi   string   `json:"dia_chi"`
	Gia      []int64  `json:"gia"`
	Gio      *string  `json:"gio"`
	Traits   []string `json:"traits"`
	HoatDong []string `json:"hoat_dong"`
	MoTa     *string  `json:"mo_ta"`
	DanhGia  []string `json:"danh_gia"`
	// Hand labels, read from the place's own words by the author: the
	// violation oracle below uses these, not the tuvung scanner.
	DiUng   []string `json:"di_ung"`
	AnKieng []string `json:"an_kieng"`
}

type rangBuocMau struct {
	DiemDen  string   `json:"diem_den"`
	DiUng    []string `json:"di_ung"`
	AnKieng  []string `json:"an_kieng"`
	NganSach *int64   `json:"ngan_sach"`
	Luc      []string `json:"luc"`
	Khung    []string `json:"khung"`
}

type truyVanMau struct {
	ID       string         `json:"id"`
	Nhom     string         `json:"nhom"`
	Cap      string         `json:"cap"`
	Cau      string         `json:"cau"`
	RangBuoc rangBuocMau    `json:"rang_buoc"`
	LienQuan map[string]int `json:"lien_quan"`
	PhaiLoai []string       `json:"phai_loai"`
}

type tapVang struct {
	DiemDen []diemDenMau `json:"diem_den"`
	Quan    []quanMau    `json:"quan"`
	BiaTay  []struct {
		ID   string `json:"id"`
		LyDo string `json:"ly_do"`
	} `json:"bia_tay"`
	TruyVan []truyVanMau `json:"truy_van"`
}

func docVang(t testing.TB) tapVang {
	t.Helper()
	raw, err := os.ReadFile("testdata/truy-hoi-dia-diem.json")
	if err != nil {
		t.Fatal(err)
	}
	var v tapVang
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	v.Quan = append(v.Quan, sinhNen(v.DiemDen)...)
	return v
}

// Background places: 240, 80 per destination, 60 per category, from word
// pools that share nothing with any query's target words, name any
// allergen, diet or atmosphere, or repeat an anchor's name. They are what
// the ranking has to rank past.
var (
	nenTen  = []string{"Sỏi", "Lam", "Mộc", "Cỏ", "Khói", "Đá", "Lúa", "Bèo", "Vân", "Trúc", "Khế", "Rạ", "Nứa", "Sậy", "Bấc", "Sim"}
	nenLoai = []string{"quan-an-local", "cafe", "vui-choi", "di-choi-dem"}
	nenDau  = map[string][]string{"quan-an-local": {"Quán", "Bếp", "Quán Ăn"}, "cafe": {"Cà Phê", "Tiệm"}, "vui-choi": {"Khu", "Sân"}, "di-choi-dem": {"Quán Khuya", "Tụ Điểm"}}
	nenMon  = map[string][]string{
		"quan-an-local": {"xôi gà", "cháo gà", "hủ tiếu", "bánh cuốn", "bò kho", "cơm niêu", "bún thịt", "bánh bèo", "bún bò", "cơm văn phòng"},
		"cafe":          {"cà phê đá", "trà đá", "nước ép", "cà phê phin", "trà chanh", "nước dừa"},
		"vui-choi":      {"bi a", "cầu lông", "trò chơi điện tử", "xe đạp đôi", "thả diều", "bóng bàn"},
		"di-choi-dem":   {"bia hơi", "xiên que", "phá lấu", "khoai lang lắc", "gà rán", "nem chua rán"},
	}
	nenNet   = []string{"giá ổn", "phục vụ nhanh", "gần chợ", "có chỗ để xe", "mở lâu năm", "đông khách"}
	nenGio   = []*string{str("07:00 – 21:00"), str("10:00 – 22:00"), str("06:00 – 14:00"), str("16:00 – 23:00"), str("18:00 – 02:00"), nil, str("08:00 – 22:00"), str("11:00 – 21:00")}
	nenGia   = []int64{20_000, 30_000, 50_000, 80_000, 120_000, 200_000}
	nenLoiKe = []string{"Ngon, sẽ quay lại.", "Phục vụ nhanh.", "Giá hợp lý.", "Chỗ ngồi hơi chật."}
)

func str(s string) *string { return &s }

func sinhNen(dests []diemDenMau) []quanMau {
	short := map[string]string{"d-da-lat": "dl", "d-tphcm": "sg", "d-hoi-an": "ha"}
	var out []quanMau
	for i := 0; i < 240; i++ {
		a := i % 16
		b := (a + 1 + i/16) % 16
		d := dests[i%3]
		cat := nenLoai[(i/3)%4]
		prefix := nenDau[cat][(i/12)%len(nenDau[cat])]
		mon := nenMon[cat]
		kinds := []string{mon[(i*7)%len(mon)]}
		if second := mon[(i*11+3)%len(mon)]; i%3 == 0 && second != kinds[0] {
			kinds = append(kinds, second)
		}
		var traits []string
		if i%2 == 0 {
			traits = []string{nenNet[(i*5)%len(nenNet)]}
		}
		q := quanMau{
			ID: fmt.Sprintf("g-%s-%03d", short[d.ID], i), DiemDen: d.ID,
			Ten: prefix + " " + nenTen[a] + " " + nenTen[b], Loai: cat, Kinds: kinds,
			DiaChi: "Khu " + nenTen[b] + ", " + d.Ten, Gio: nenGio[(i*3)%len(nenGio)], Traits: traits,
		}
		if i%10 != 9 {
			min := nenGia[(i*13)%len(nenGia)]
			q.Gia = []int64{min, 2 * min}
		}
		desc := kinds[0]
		if len(traits) > 0 {
			desc += ", " + traits[0]
		}
		q.MoTa = str(strings.ToUpper(desc[:1]) + desc[1:] + ".")
		if i%4 == 0 {
			q.DanhGia = []string{nenLoiKe[(i/4)%len(nenLoiKe)]}
		}
		out = append(out, q)
	}
	return out
}

// hang places a fixture place near its destination's centre.
func (v tapVang) hang(i int, q quanMau) repo.Place {
	var d diemDenMau
	for _, x := range v.DiemDen {
		if x.ID == q.DiemDen {
			d = x
		}
	}
	p := repo.Place{
		ID: q.ID, DestinationID: q.DiemDen, Name: q.Ten, Category: q.Loai, Kinds: nonNil(q.Kinds),
		Lat: d.Lat + float64(i%7-3)*0.004, Lng: d.Lng + float64(i%5-2)*0.004,
		OpenHours: q.Gio, Traits: nonNil(q.Traits), Description: q.MoTa, Source: "seed",
	}
	if q.DiaChi != "" {
		p.Address = str(q.DiaChi)
	}
	if len(q.Gia) == 2 {
		lo, hi := q.Gia[0], q.Gia[1]
		p.PriceMinVND, p.PriceMaxVND = &lo, &hi
	}
	if len(q.HoatDong) > 0 {
		p.Activities, _ = json.Marshal(q.HoatDong)
	}
	if len(q.DanhGia) > 0 {
		type review struct {
			Author string `json:"author"`
			Rating int    `json:"rating"`
			Body   string `json:"body"`
		}
		var rs []review
		for _, body := range q.DanhGia {
			rs = append(rs, review{"Khách", 5, body})
		}
		p.Reviews, _ = json.Marshal(rs)
	}
	return p
}

func (v tapVang) rows() []repo.Place {
	out := make([]repo.Place, len(v.Quan))
	for i, q := range v.Quan {
		out[i] = v.hang(i, q)
	}
	return out
}

func (v tapVang) dests() []DiemDen {
	out := make([]DiemDen, len(v.DiemDen))
	for i, d := range v.DiemDen {
		out[i] = DiemDen{ID: d.ID, Ten: d.Ten, Tinh: d.Tinh, Nam: d.Nam, Tay: d.Tay, Bac: d.Bac, Dong: d.Dong}
	}
	return out
}

var thuMau = map[string]int{"Mo": 0, "Tu": 1, "We": 2, "Th": 3, "Fr": 4, "Sa": 5, "Su": 6}

func phutMau(day, hhmm string) int {
	h, _ := strconv.Atoi(hhmm[:2])
	m, _ := strconv.Atoi(hhmm[3:])
	return thuMau[day]*1440 + h*60 + m
}

// yeuCau is what the engine would hand Retrieve for a golden query: the
// deterministic reading of the words (DocCau: destination, allergens,
// diets, categories, atmospheres) plus the slots only Understand can fill
// from words (a budget, a time), taken from the hand labels.
func (v tapVang) yeuCau(q truyVanMau) (YeuCau, DiemDenGiai) {
	y, dd := DocCau(q.Cau, v.dests())
	y.K = 10
	y.NganSach = q.RangBuoc.NganSach
	if len(q.RangBuoc.Luc) == 2 {
		m := phutMau(q.RangBuoc.Luc[0], q.RangBuoc.Luc[1])
		y.Luc = &m
	}
	if len(q.RangBuoc.Khung) == 4 {
		w := [2]int{phutMau(q.RangBuoc.Khung[0], q.RangBuoc.Khung[1]), phutMau(q.RangBuoc.Khung[2], q.RangBuoc.Khung[3])}
		y.Khung = &w
	}
	return y, dd
}

// The violation oracle: written apart from rag and tuvung on purpose, from
// the hand labels, so the filters are checked against something they did
// not produce.
var (
	hoBien    = []string{"tom", "cua", "muc", "oc_so", "ca"}
	gioOracle = regexp.MustCompile(`^\s*(\d{1,2}):(\d{2})\s*[–-]\s*(\d{1,2}):(\d{2})\s*$`)
)

func dongDiUng(asked []string) map[string]bool {
	out := map[string]bool{}
	for _, a := range asked {
		out[a] = true
		if a == "hai_san" {
			for _, c := range hoBien {
				out[c] = true
			}
		}
		for _, c := range hoBien {
			if a == c {
				out["hai_san"] = true
			}
		}
	}
	return out
}

// spansOracle is the week's open spans of a «HH:MM – HH:MM» string, the
// tra_loi_trong_nhom.py `_opening` rule applied to every day; nil for
// anything else (unknown).
func spansOracle(gio *string) [][2]int {
	if gio == nil {
		return nil
	}
	m := gioOracle.FindStringSubmatch(*gio)
	if m == nil {
		return nil
	}
	n := func(s string) int { x, _ := strconv.Atoi(s); return x }
	start, end := n(m[1])*60+n(m[2]), n(m[3])*60+n(m[4])
	if end <= start {
		end += 1440
	}
	var out [][2]int
	for d := 0; d < 7; d++ {
		out = append(out, [2]int{d*1440 + start, d*1440 + end})
	}
	return out
}

func overlaps(spans [][2]int, lo, hi int) bool {
	for _, s := range spans {
		for _, shift := range []int{0, 10080} {
			if s[0] < hi+shift && lo+shift < s[1] {
				return true
			}
		}
	}
	return false
}

func (v tapVang) viPham(q truyVanMau, id string) string {
	for _, p := range q.PhaiLoai {
		if p == id {
			return "phai_loai"
		}
	}
	var place *quanMau
	for i := range v.Quan {
		if v.Quan[i].ID == id {
			place = &v.Quan[i]
		}
	}
	if place == nil {
		return "khong_co_trong_danh_muc"
	}
	for _, b := range v.BiaTay {
		if b.ID == id {
			return "bia"
		}
	}
	rb := q.RangBuoc
	if rb.DiemDen != "" && place.DiemDen != rb.DiemDen {
		return "diem_den"
	}
	closed := dongDiUng(rb.DiUng)
	for _, a := range place.DiUng {
		if closed[a] {
			return "di_ung"
		}
	}
	diets := map[string]bool{}
	for _, d := range place.AnKieng {
		diets[d] = true
		if d == "thuan_chay" {
			diets["chay"] = true
		}
	}
	for _, d := range rb.AnKieng {
		if !diets[d] {
			return "an_kieng"
		}
	}
	if rb.NganSach != nil && len(place.Gia) == 2 && place.Gia[0] > *rb.NganSach {
		return "ngan_sach"
	}
	if spans := spansOracle(place.Gio); spans != nil {
		if len(rb.Luc) == 2 {
			m := phutMau(rb.Luc[0], rb.Luc[1])
			if !overlaps(spans, m, m+1) {
				return "gio"
			}
		}
		if len(rb.Khung) == 4 && !overlaps(spans, phutMau(rb.Khung[0], rb.Khung[1]), phutMau(rb.Khung[2], rb.Khung[3])) {
			return "gio"
		}
	}
	return ""
}

// soDo is one group's measurement.
type soDo struct {
	n, coLienQuan         int
	recall, ndcg, mrr     float64
	truyVanViPham, viPham int
}

func (s soDo) String() string {
	d := float64(max(s.coLienQuan, 1))
	return fmt.Sprintf("n=%d co_lien_quan=%d recall@10=%.4f ndcg@10=%.4f mrr@10=%.4f violation@10=%.4f so_vi_pham=%d",
		s.n, s.coLienQuan, s.recall/d, s.ndcg/d, s.mrr/d, float64(s.truyVanViPham)/float64(max(s.n, 1)), s.viPham)
}

// cham scores one query's top ten. A query counts toward recall, nDCG and
// MRR when it has a place graded 2 or 3; every query counts toward
// violation.
func cham(q truyVanMau, top []string, viPham func(string) string) (soDo, []string) {
	s := soDo{n: 1}
	if len(top) > 10 {
		top = top[:10]
	}
	var why []string
	for _, id := range top {
		if reason := viPham(id); reason != "" {
			s.viPham++
			why = append(why, id+":"+reason)
		}
	}
	if s.viPham > 0 {
		s.truyVanViPham = 1
	}
	var relevant []string
	var grades []int
	for id, g := range q.LienQuan {
		if g >= 2 {
			relevant = append(relevant, id)
		}
		grades = append(grades, g)
	}
	if len(relevant) == 0 {
		return s, why
	}
	s.coLienQuan = 1
	found := 0
	for i, id := range top {
		g := q.LienQuan[id]
		if g >= 2 {
			found++
			if s.mrr == 0 {
				s.mrr = 1 / float64(i+1)
			}
		}
		s.ndcg += (math.Pow(2, float64(g)) - 1) / math.Log2(float64(i+2))
	}
	s.recall = float64(found) / float64(len(relevant))
	sort.Sort(sort.Reverse(sort.IntSlice(grades)))
	ideal := 0.0
	for i, g := range grades {
		if i == 10 {
			break
		}
		ideal += (math.Pow(2, float64(g)) - 1) / math.Log2(float64(i+2))
	}
	s.ndcg /= ideal
	return s, why
}

func (s *soDo) cong(o soDo) {
	s.n += o.n
	s.coLienQuan += o.coLienQuan
	s.recall += o.recall
	s.ndcg += o.ndcg
	s.mrr += o.mrr
	s.truyVanViPham += o.truyVanViPham
	s.viPham += o.viPham
}

var nhomVang = []string{"ten_rieng", "khong_dau", "khi_chat", "rang_buoc", "di_ung", "lien_diem_den", "bay_injection"}

// chayVang runs every query through retrieve and returns each group's
// measurement plus "tong", logging every violation and every query whose
// best place is missing from the top ten.
func chayVang(t *testing.T, v tapVang, retrieve func(YeuCau) KetQua) map[string]soDo {
	t.Helper()
	out := map[string]soDo{}
	byID := map[string]truyVanMau{}
	for _, q := range v.TruyVan {
		byID[q.ID] = q
	}
	for _, q := range v.TruyVan {
		y, dd := v.yeuCau(q)
		if dd.ID != q.RangBuoc.DiemDen {
			t.Errorf("%s: destination %q, want %q", q.ID, dd.ID, q.RangBuoc.DiemDen)
		}
		kq := retrieve(y)
		var top []string
		for _, h := range kq.Quan {
			top = append(top, h.ID)
		}
		s, why := cham(q, top, func(id string) string { return v.viPham(q, id) })
		if len(why) > 0 {
			t.Logf("%s vi phạm: %v", q.ID, why)
		}
		if s.coLienQuan == 1 && s.recall < 1 {
			t.Logf("%s (%s) thiếu: top=%v", q.ID, q.Nhom, top)
		} else if s.coLienQuan == 1 && s.mrr < 1 {
			t.Logf("%s (%s) quán đúng đầu tiên ở hạng %.0f: top=%v", q.ID, q.Nhom, 1/s.mrr, top)
		}
		g := out[q.Nhom]
		g.cong(s)
		out[q.Nhom] = g
		all := out["tong"]
		all.cong(s)
		out["tong"] = all
	}
	return out
}

// kiemGhim compares measured numbers with pinned ones, digit for digit.
func kiemGhim(t *testing.T, got map[string]soDo, want map[string]string) {
	t.Helper()
	for _, g := range append(append([]string{}, nhomVang...), "tong") {
		if got[g].String() != want[g] {
			t.Errorf("%s:\n got  %s\n want %s", g, got[g].String(), want[g])
		}
	}
	for _, g := range append(append([]string{}, nhomVang...), "tong") {
		t.Logf("%-14s %s", g, got[g].String())
	}
	if got["tong"].viPham != 0 {
		t.Errorf("violation@10 must be 0; %d violating results", got["tong"].viPham)
	}
}

// kiemKhongDau holds the khong_dau group to its twins: the same questions
// typed with and without marks may not differ by more than 0.05 recall@10
// either way (design 04 §8.3).
func kiemKhongDau(t *testing.T, v tapVang, retrieve func(YeuCau) KetQua) {
	t.Helper()
	byID := map[string]truyVanMau{}
	for _, q := range v.TruyVan {
		byID[q.ID] = q
	}
	var coDau, khong float64
	n := 0
	for _, q := range v.TruyVan {
		if q.Nhom != "khong_dau" {
			continue
		}
		twin := byID[q.Cap]
		for _, pair := range []struct {
			q   truyVanMau
			acc *float64
		}{{q, &khong}, {twin, &coDau}} {
			y, _ := v.yeuCau(pair.q)
			var top []string
			for _, h := range retrieve(y).Quan {
				top = append(top, h.ID)
			}
			s, _ := cham(pair.q, top, func(string) string { return "" })
			*pair.acc += s.recall
		}
		n++
	}
	if n != 10 {
		t.Fatalf("%d khong_dau twins, want 10", n)
	}
	gap := (coDau - khong) / float64(n)
	t.Logf("recall@10 có dấu %.4f, không dấu %.4f, lệch %.4f", coDau/float64(n), khong/float64(n), gap)
	if math.Abs(gap) > 0.05 {
		t.Errorf("the khong_dau group and its twins differ by %.4f recall@10 (> 0.05): one of the two spellings is not folded like the index", gap)
	}
}

func tapHop(xs []string) string {
	c := append([]string(nil), xs...)
	sort.Strings(c)
	return strings.Join(c, ",")
}

func biaTay(v tapVang) map[string]bool {
	out := map[string]bool{}
	for _, b := range v.BiaTay {
		out[b.ID] = true
	}
	return out
}

// The fixture checks itself before it checks anything else.
func TestVangTuKiem(t *testing.T) {
	v := docVang(t)
	if len(v.Quan) != 289 || len(v.TruyVan) != 83 {
		t.Fatalf("%d places, %d queries; want 289 and 83", len(v.Quan), len(v.TruyVan))
	}
	ids := map[string]bool{}
	for _, q := range v.Quan {
		if ids[q.ID] {
			t.Fatalf("duplicate place %s", q.ID)
		}
		ids[q.ID] = true
	}
	perGroup := map[string]int{}
	for _, q := range v.TruyVan {
		perGroup[q.Nhom]++
		for id := range q.LienQuan {
			if !ids[id] {
				t.Errorf("%s grades unknown place %s", q.ID, id)
			}
		}
		for _, id := range q.PhaiLoai {
			if !ids[id] {
				t.Errorf("%s excludes unknown place %s", q.ID, id)
			}
			if q.LienQuan[id] > 0 {
				t.Errorf("%s both grades and excludes %s", q.ID, id)
			}
		}
		for id := range q.LienQuan {
			if why := v.viPham(q, id); why != "" {
				t.Errorf("%s grades %s, which its own constraints exclude (%s)", q.ID, id, why)
			}
		}
	}
	for _, g := range nhomVang {
		if perGroup[g] < 6 {
			t.Errorf("group %s has %d queries", g, perGroup[g])
		}
	}
	// The background's word pools name no allergen, diet or atmosphere: a
	// check of the pools, so the ranking has plain places to rank past.
	for i, q := range v.Quan {
		if !strings.HasPrefix(q.ID, "g-") {
			continue
		}
		h, rep := DungHoSo(v.hang(i, q))
		if rep.Bo || len(h.DiUng)+len(h.AnKieng)+len(h.KhiChat) > 0 {
			t.Errorf("background %s carries tags %v %v %v", q.ID, h.DiUng, h.AnKieng, h.KhiChat)
		}
	}
	// Where the anchors' hand labels and the scanner disagree is logged,
	// never required to agree: the oracle is the hand label, so a scanner
	// miss on a place a query returns is a violation the numbers show
	// (TestVangOracleDocLap), not a fixture error.
	for i, q := range v.Quan {
		if strings.HasPrefix(q.ID, "g-") {
			continue
		}
		h, rep := DungHoSo(v.hang(i, q))
		if rep.Bo {
			continue
		}
		if tapHop(h.DiUng) != tapHop(q.DiUng) {
			t.Logf("%s: scanner allergens %v, hand label %v", q.ID, h.DiUng, q.DiUng)
		}
		if tapHop(h.AnKieng) != tapHop(q.AnKieng) {
			t.Logf("%s: scanner diets %v, hand label %v", q.ID, h.AnKieng, q.AnKieng)
		}
	}
}

// The oracle does not lean on the scanner. The milk-tea shop is reworded so
// the scanner can no longer see its milk («Trà pha kem béo»), its hand label
// stays «sữa», and a milk-allergic asker asks for tea: the live path returns
// the shop, and the golden's oracle calls it a violation. With the shop's
// own words (identity) the same query has none.
func TestVangOracleDocLap(t *testing.T) {
	q := truyVanMau{ID: "x01", Nhom: "di_ung", Cau: "Mình dị ứng sữa, tìm quán trà ở Sài Gòn",
		RangBuoc: rangBuocMau{DiemDen: "d-tphcm", DiUng: []string{"sua"}}}
	chay := func(v tapVang) (int, []string) {
		rows, bia := v.rows(), biaTay(v)
		y, _ := v.yeuCau(q)
		var top []string
		for _, h := range xepSong(rows, bia, y).Quan {
			top = append(top, h.ID)
		}
		s, why := cham(q, top, func(id string) string { return v.viPham(q, id) })
		return s.viPham, why
	}
	v := docVang(t)
	if n, why := chay(v); n != 0 {
		t.Fatalf("identity: the shop's own words already break the filter: %v", why)
	}
	for i := range v.Quan {
		if v.Quan[i].ID == "sg-tra-sua-chim-se" {
			v.Quan[i].Ten = "Tiệm Trà Chim Sẻ"
			v.Quan[i].Kinds = []string{"trà pha kem béo", "trân châu"}
			v.Quan[i].MoTa = str("Trà pha kem béo, trân châu đường đen.")
			if tapHop(v.Quan[i].DiUng) != "sua" {
				t.Fatal("the hand label changed")
			}
		}
	}
	n, why := chay(v)
	if n == 0 {
		t.Fatal("a scanner miss did not show up as a violation: the oracle is not independent of the scanner")
	}
	t.Logf("scanner miss seen by the oracle: %v", why)
}

// The live-row path on the golden set: rag/xephang BM25 plus the vocabulary
// list, the same hard filters, the takedown honoured. Pinned to the digit.
func TestVangHangSong(t *testing.T) {
	v := docVang(t)
	rows, bia := v.rows(), biaTay(v)
	retrieve := func(y YeuCau) KetQua { return xepSong(rows, bia, y) }
	kiemGhim(t, chayVang(t, v, retrieve), ghimSong)
	kiemKhongDau(t, v, retrieve)
}

var ghimSong = map[string]string{
	"ten_rieng":     "n=12 co_lien_quan=12 recall@10=1.0000 ndcg@10=0.9489 mrr@10=0.9333 violation@10=0.0000 so_vi_pham=0",
	"khong_dau":     "n=10 co_lien_quan=10 recall@10=1.0000 ndcg@10=1.0000 mrr@10=1.0000 violation@10=0.0000 so_vi_pham=0",
	"khi_chat":      "n=10 co_lien_quan=10 recall@10=1.0000 ndcg@10=0.9917 mrr@10=1.0000 violation@10=0.0000 so_vi_pham=0",
	"rang_buoc":     "n=10 co_lien_quan=9 recall@10=1.0000 ndcg@10=0.9590 mrr@10=0.9167 violation@10=0.0000 so_vi_pham=0",
	"di_ung":        "n=26 co_lien_quan=13 recall@10=1.0000 ndcg@10=1.0000 mrr@10=1.0000 violation@10=0.0000 so_vi_pham=0",
	"lien_diem_den": "n=8 co_lien_quan=8 recall@10=1.0000 ndcg@10=1.0000 mrr@10=1.0000 violation@10=0.0000 so_vi_pham=0",
	"bay_injection": "n=7 co_lien_quan=2 recall@10=1.0000 ndcg@10=1.0000 mrr@10=1.0000 violation@10=0.0000 so_vi_pham=0",
	"tong":          "n=83 co_lien_quan=64 recall@10=1.0000 ndcg@10=0.9834 mrr@10=0.9758 violation@10=0.0000 so_vi_pham=0",
}

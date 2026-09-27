package nap

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"mobile/services/core/internal/domain/giomo"
	"mobile/services/core/internal/domain/promptsafety"
	"mobile/services/core/internal/domain/tuvung"
	"mobile/services/core/internal/repo"
)

// The offline evaluation gate reads the place golden set
// (rag/testdata/truy-hoi-dia-diem.json, embedded by package rag): synthetic
// places with hand labels, and questions whose hard constraints are written
// by hand as a router would extract them (ids, a budget, a time). The gate
// never reads a question's words to find a constraint.

// TapVang is the golden set.
type TapVang struct {
	DiemDen []DiemDenVang `json:"diem_den"`
	Quan    []QuanVang    `json:"quan"`
	BiaTay  []struct {
		ID   string `json:"id"`
		LyDo string `json:"ly_do"`
	} `json:"bia_tay"`
	TruyVan []TruyVanVang `json:"truy_van"`
	GhiChu  string        `json:"ghi_chu"`
	// Sha is the file's sha256, recorded with every measurement.
	Sha string `json:"-"`
}

// DiemDenVang is a destination.
type DiemDenVang struct {
	ID   string  `json:"id"`
	Ten  string  `json:"ten"`
	Tinh string  `json:"tinh"`
	Lat  float64 `json:"lat"`
	Lng  float64 `json:"lng"`
	Nam  float64 `json:"nam"`
	Tay  float64 `json:"tay"`
	Bac  float64 `json:"bac"`
	Dong float64 `json:"dong"`
	Thu  int     `json:"thu_tu"`
}

// QuanVang is a hand-labelled place.
type QuanVang struct {
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
	DiUng    []string `json:"di_ung"`
	AnKieng  []string `json:"an_kieng"`
	// NguonNhan names where a hand label came from (a held-out corpus row).
	NguonNhan string `json:"nguon_nhan"`
	// DiUngKhongRo marks a background place whose allergens nobody
	// established (its enrichment answered khong_ro). Not in the file: the
	// generator sets it on every sixth background place, so the golden set,
	// the probes and the live tier always hold places with unknown
	// allergens, unknown prices and unknown hours, and a filter that lets an
	// unknown through under a hard constraint shows as a violation.
	DiUngKhongRo bool `json:"-"`
}

// RangBuocVang are a question's hand-written hard constraints.
type RangBuocVang struct {
	DiemDen  string   `json:"diem_den"`
	DiUng    []string `json:"di_ung"`
	AnKieng  []string `json:"an_kieng"`
	NganSach *int64   `json:"ngan_sach"`
	Luc      []string `json:"luc"`
	Khung    []string `json:"khung"`
}

// TruyVanVang is one question.
type TruyVanVang struct {
	ID       string         `json:"id"`
	Nhom     string         `json:"nhom"`
	Cap      string         `json:"cap"`
	Cau      string         `json:"cau"`
	RangBuoc RangBuocVang   `json:"rang_buoc"`
	LienQuan map[string]int `json:"lien_quan"`
	PhaiLoai []string       `json:"phai_loai"`
}

// DocVang parses the golden set strictly and adds the 240 generated
// background places (the same generator as rag/vang_test.go).
func DocVang(raw []byte) (TapVang, error) {
	var v TapVang
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&v); err != nil {
		return TapVang{}, fmt.Errorf("%w: golden set: %v", ErrCauHinh, err)
	}
	if len(v.DiemDen) != 3 || len(v.TruyVan) == 0 {
		return TapVang{}, fmt.Errorf("%w: golden set shape", ErrCauHinh)
	}
	sum := sha256.Sum256(raw)
	v.Sha = hex.EncodeToString(sum[:])
	v.Quan = append(v.Quan, sinhNen(v.DiemDen)...)
	return v, nil
}

func strp(s string) *string { return &s }

func f64p(f float64) *float64 { return &f }

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
	nenGio   = []*string{strp("07:00 – 21:00"), strp("10:00 – 22:00"), strp("06:00 – 14:00"), strp("16:00 – 23:00"), strp("18:00 – 02:00"), nil, strp("08:00 – 22:00"), strp("11:00 – 21:00")}
	nenGia   = []int64{20_000, 30_000, 50_000, 80_000, 120_000, 200_000}
	nenLoiKe = []string{"Ngon, sẽ quay lại.", "Phục vụ nhanh.", "Giá hợp lý.", "Chỗ ngồi hơi chật."}
)

// sinhNen is rag/vang_test.go's background generator, byte for byte: 240
// places from word pools that share nothing with any question's target. The
// hand labels of a background place are empty; the golden note says their
// words name no allergen, which their hand labels (none) state.
func sinhNen(dests []DiemDenVang) []QuanVang {
	short := map[string]string{"d-da-lat": "dl", "d-tphcm": "sg", "d-hoi-an": "ha"}
	var out []QuanVang
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
		q := QuanVang{
			ID: fmt.Sprintf("g-%s-%03d", short[d.ID], i), DiemDen: d.ID,
			Ten: prefix + " " + nenTen[a] + " " + nenTen[b], Loai: cat, Kinds: kinds,
			DiaChi: "Khu " + nenTen[b] + ", " + d.Ten, Gio: nenGio[(i*3)%len(nenGio)], Traits: traits,
		}
		if i%10 != 9 {
			lo := nenGia[(i*13)%len(nenGia)]
			q.Gia = []int64{lo, 2 * lo}
		}
		desc := kinds[0]
		if len(traits) > 0 {
			desc += ", " + traits[0]
		}
		q.MoTa = strp(strings.ToUpper(desc[:1]) + desc[1:] + ".")
		if i%4 == 0 {
			q.DanhGia = []string{nenLoiKe[(i/4)%len(nenLoiKe)]}
		}
		q.DiUngKhongRo = i%6 == 5
		out = append(out, q)
	}
	return out
}

// Hang places a golden place near its destination's centre, as a
// catalogue row (the same placement as rag/vang_test.go).
func (v TapVang) Hang(i int, q QuanVang) repo.Place {
	var d DiemDenVang
	for _, x := range v.DiemDen {
		if x.ID == q.DiemDen {
			d = x
		}
	}
	nn := func(s []string) []string {
		if s == nil {
			return []string{}
		}
		return s
	}
	p := repo.Place{
		ID: q.ID, DestinationID: q.DiemDen, Name: q.Ten, Category: q.Loai, Kinds: nn(q.Kinds),
		Lat: f64p(d.Lat + float64(i%7-3)*0.004), Lng: f64p(d.Lng + float64(i%5-2)*0.004),
		OpenHours: q.Gio, Traits: nn(q.Traits), Description: q.MoTa, Source: "seed",
	}
	if q.DiaChi != "" {
		p.Address = strp(q.DiaChi)
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

// Rows returns every golden place as a catalogue row.
func (v TapVang) Rows() []repo.Place {
	out := make([]repo.Place, len(v.Quan))
	for i, q := range v.Quan {
		out[i] = v.Hang(i, q)
	}
	return out
}

// NhanTay is the enrichment a perfect extractor would give a golden place:
// its hand labels, reviewed (allergens unknown for a DiUngKhongRo place).
// The gate measures retrieval given attributes; the extractor's own
// precision is measured on the review queue.
func (q QuanVang) NhanTay() ThuocTinh {
	if q.DiUngKhongRo {
		return ThuocTinh{AnKieng: tuvung.DoiKieng(q.AnKieng), Co: true}
	}
	return ThuocTinh{DiUng: tuvung.DiUng.LocHopLe(q.DiUng), DiUngRo: true, AnKieng: tuvung.DoiKieng(q.AnKieng), Co: true}
}

var thuVang = map[string]int{"Mo": 0, "Tu": 1, "We": 2, "Th": 3, "Fr": 4, "Sa": 5, "Su": 6}

func phutVang(day, hhmm string) (int, bool) {
	d, ok := thuVang[day]
	if !ok || len(hhmm) != 5 {
		return 0, false
	}
	h, e1 := strconv.Atoi(hhmm[:2])
	m, e2 := strconv.Atoi(hhmm[3:])
	if e1 != nil || e2 != nil {
		return 0, false
	}
	return d*1440 + h*60 + m, true
}

// Loc turns a question's hand constraints into the index filter. A time is
// the slot holding it; a window is every slot fully inside it.
func (r RangBuocVang) Loc() Loc {
	l := Loc{DiemDen: r.DiemDen, DiUng: r.DiUng, AnKieng: r.AnKieng, NganSach: r.NganSach}
	if len(r.Luc) == 2 {
		if m, ok := phutVang(r.Luc[0], r.Luc[1]); ok {
			o := int16((m % giomo.PhutTuan) / PhutMoiO)
			l.O = &o
		}
	}
	if len(r.Khung) == 4 {
		a, ok1 := phutVang(r.Khung[0], r.Khung[1])
		b, ok2 := phutVang(r.Khung[2], r.Khung[3])
		if ok1 && ok2 {
			if b <= a {
				b += giomo.PhutTuan
			}
			for s := (a + PhutMoiO - 1) / PhutMoiO; (s+1)*PhutMoiO <= b; s++ {
				l.Khung = append(l.Khung, int16(s%SoO))
			}
			if len(l.Khung) == 0 {
				// A window narrower than a slot: the slot holding its start.
				l.Khung = []int16{int16((a % giomo.PhutTuan) / PhutMoiO)}
			}
		}
	}
	return l
}

// The violation oracle: written from the hand labels and the question's
// hand constraints, apart from the filter under test.
var (
	hoBienVang = []string{"tom", "cua", "muc", "oc_so", "ca"}
	gioVang    = regexp.MustCompile(`^\s*(\d{1,2}):(\d{2})\s*[–-]\s*(\d{1,2}):(\d{2})\s*$`)
)

func dongDiUngVang(asked []string) map[string]bool {
	out := map[string]bool{}
	for _, a := range asked {
		out[a] = true
		if a == "hai_san" {
			for _, c := range hoBienVang {
				out[c] = true
			}
		}
		for _, c := range hoBienVang {
			if a == c {
				out["hai_san"] = true
			}
		}
	}
	return out
}

func spansVang(gio *string) [][2]int {
	if gio == nil {
		return nil
	}
	m := gioVang.FindStringSubmatch(*gio)
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

func overlapsVang(spans [][2]int, lo, hi int) bool {
	for _, s := range spans {
		for _, shift := range []int{0, 10080} {
			if s[0] < hi+shift && lo+shift < s[1] {
				return true
			}
		}
	}
	return false
}

// ViPham says why place id breaks question q, or "".
func (v TapVang) ViPham(q TruyVanVang, id string) string {
	for _, p := range q.PhaiLoai {
		if p == id {
			return "phai_loai"
		}
	}
	var place *QuanVang
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
	// Unknown under a hard constraint is out (docs/architecture/03 §8.4).
	if len(rb.DiUng) > 0 && place.DiUngKhongRo {
		return "di_ung_khong_ro"
	}
	if rb.NganSach != nil && len(place.Gia) != 2 {
		return "gia_chua_ro"
	}
	if (len(rb.Luc) == 2 || len(rb.Khung) == 4) && spansVang(place.Gio) == nil {
		return "gio_chua_ro"
	}
	closed := dongDiUngVang(rb.DiUng)
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
	if spans := spansVang(place.Gio); spans != nil {
		if len(rb.Luc) == 2 {
			m, _ := phutVang(rb.Luc[0], rb.Luc[1])
			if !overlapsVang(spans, m, m+1) {
				return "gio"
			}
		}
		if len(rb.Khung) == 4 {
			a, _ := phutVang(rb.Khung[0], rb.Khung[1])
			b, _ := phutVang(rb.Khung[2], rb.Khung[3])
			if !overlapsVang(spans, a, b) {
				return "gio"
			}
		}
	}
	return ""
}

// BoDau is the golden set with every question's text folded to no
// diacritics (promptsafety.Fold: marks and đ): the slice a person typing
// without an IME produces. Only the text changes; constraints and labels
// stay, so recall on it measures what the folded BM25 leg recovers.
func (v TapVang) BoDau() TapVang {
	out := v
	out.TruyVan = make([]TruyVanVang, len(v.TruyVan))
	for i, t := range v.TruyVan {
		t.Cau = promptsafety.Fold(t.Cau)
		t.Cap = ""
		out.TruyVan[i] = t
	}
	return out
}

// SoDo is one group's measurement, sums until Chia.
type SoDo struct {
	N             int     `json:"n"`
	CoLienQuan    int     `json:"co_lien_quan"`
	Recall        float64 `json:"recall_10"`
	NDCG          float64 `json:"ndcg_10"`
	MRR           float64 `json:"mrr_10"`
	Violation     float64 `json:"violation_10"`
	TruyVanViPham int     `json:"truy_van_vi_pham"`
	ViPham        int     `json:"so_vi_pham"`
}

func (s SoDo) String() string {
	return fmt.Sprintf("n=%d co_lien_quan=%d recall@10=%.4f ndcg@10=%.4f mrr@10=%.4f violation@10=%.4f so_vi_pham=%d",
		s.N, s.CoLienQuan, s.Recall, s.NDCG, s.MRR, s.Violation, s.ViPham)
}

// ChamMot scores one question's top ten documents.
func ChamMot(q TruyVanVang, top []string, viPham func(string) string) SoDo {
	s := SoDo{N: 1}
	if len(top) > 10 {
		top = top[:10]
	}
	for _, id := range top {
		if viPham(id) != "" {
			s.ViPham++
		}
	}
	if s.ViPham > 0 {
		s.TruyVanViPham = 1
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
		return s
	}
	s.CoLienQuan = 1
	found := 0
	for i, id := range top {
		g := q.LienQuan[id]
		if g >= 2 {
			found++
			if s.MRR == 0 {
				s.MRR = 1 / float64(i+1)
			}
		}
		s.NDCG += (math.Pow(2, float64(g)) - 1) / math.Log2(float64(i+2))
	}
	s.Recall = float64(found) / float64(len(relevant))
	sort.Sort(sort.Reverse(sort.IntSlice(grades)))
	ideal := 0.0
	for i, g := range grades {
		if i == 10 {
			break
		}
		ideal += (math.Pow(2, float64(g)) - 1) / math.Log2(float64(i+2))
	}
	s.NDCG /= ideal
	return s
}

func (s *SoDo) cong(o SoDo) {
	s.N += o.N
	s.CoLienQuan += o.CoLienQuan
	s.Recall += o.Recall
	s.NDCG += o.NDCG
	s.MRR += o.MRR
	s.TruyVanViPham += o.TruyVanViPham
	s.ViPham += o.ViPham
}

// chia turns sums into means: relevance over questions with a relevant
// place, violation over every question. Four decimals, so a pinned number
// reads the same everywhere.
func (s SoDo) chia() SoDo {
	d := float64(max(s.CoLienQuan, 1))
	r4 := func(x float64) float64 { return math.Round(x*10000) / 10000 }
	s.Recall, s.NDCG, s.MRR = r4(s.Recall/d), r4(s.NDCG/d), r4(s.MRR/d)
	s.Violation = r4(float64(s.TruyVanViPham) / float64(max(s.N, 1)))
	return s
}

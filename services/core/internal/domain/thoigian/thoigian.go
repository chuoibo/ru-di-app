// Package thoigian resolves the relative Vietnamese dates and times people
// write in a question -- «tối nay», «mai», «mốt», «thứ 6 tới», «cuối tuần»,
// «tuần sau», «25/9», «7h tối», «19h30» -- against the instant the question was
// asked, on Vietnam's wall clock. Pure: text and an instant in, resolved
// mentions out. It never reads a clock of its own; the caller passes the
// instant the question was stored with, so a job retried an hour later still
// reads «mai» as the day after it was asked.
//
// When a phrase has two plausible readings it says so (MoHo) and gives the
// other one, instead of silently picking: «mai» written at 00:30 is the next
// calendar day to a calendar and the coming day to a person who has not slept
// yet.
package thoigian

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"mobile/services/core/internal/domain/pairpaper"
)

// Ngay is a civil date on Vietnam's wall clock.
type Ngay struct {
	Nam, Thang, Ngay int
}

// Gio is a wall-clock time of day.
type Gio struct {
	Gio, Phut int
}

// Loai is what kind of thing a mention resolved to.
type Loai string

const (
	// LoaiNgay is one day, maybe with a part of the day (Buoi).
	LoaiNgay Loai = "ngay"
	// LoaiKhoang is a run of days: a weekend, a week.
	LoaiKhoang Loai = "khoang"
	// LoaiGio is a time of day.
	LoaiGio Loai = "gio"
)

// Buoi is a part of the day.
type Buoi string

const (
	BuoiKhong Buoi = ""
	BuoiSang  Buoi = "sang"
	BuoiTrua  Buoi = "trua"
	BuoiChieu Buoi = "chieu"
	BuoiToi   Buoi = "toi"
	BuoiDem   Buoi = "dem"
)

// Moc is one resolved mention.
type Moc struct {
	// Cum is the phrase exactly as written (NFC), for the annotation.
	Cum  string
	Loai Loai
	// Tu and Den are the days; equal for LoaiNgay.
	Tu, Den Ngay
	// Buoi is the part of the day a LoaiNgay mention named («tối nay»).
	Buoi Buoi
	// Luc is the time of a LoaiGio mention.
	Luc Gio
	// MoHo is set when the phrase reads two ways. Khac (days) or LucKhac
	// (times) is the other reading.
	MoHo    bool
	Khac    *Ngay
	LucKhac *Gio
	// ViTri is the byte offset of the phrase in the NFC text.
	ViTri int
}

// Now returns the wall-clock reading of luc in Vietnam.
func Now(luc time.Time) time.Time { return pairpaper.Local(luc) }

func ngayCua(t time.Time) Ngay { return Ngay{t.Year(), int(t.Month()), t.Day()} }

// Cong adds n days.
func (d Ngay) Cong(n int) Ngay {
	return ngayCua(time.Date(d.Nam, time.Month(d.Thang), d.Ngay+n, 12, 0, 0, 0, time.UTC))
}

// Thu is the day of the week, Monday = 0 … Sunday = 6.
func (d Ngay) Thu() int {
	return (int(time.Date(d.Nam, time.Month(d.Thang), d.Ngay, 12, 0, 0, 0, time.UTC).Weekday()) + 6) % 7
}

var tenThu = [7]string{"Thứ Hai", "Thứ Ba", "Thứ Tư", "Thứ Năm", "Thứ Sáu", "Thứ Bảy", "Chủ Nhật"}

// String is «Thứ Sáu 25/09/2026».
func (d Ngay) String() string {
	return fmt.Sprintf("%s %02d/%02d/%04d", tenThu[d.Thu()], d.Ngay, d.Thang, d.Nam)
}

// String is «19:30».
func (g Gio) String() string { return fmt.Sprintf("%02d:%02d", g.Gio, g.Phut) }

// DongBayGio is the one line that tells a model what «now» is, in both a form
// it reads well and the RFC 3339 form an evaluator can hold it to:
// «Bây giờ: Thứ Sáu 25/09/2026 14:05 (Asia/Ho_Chi_Minh, 2026-09-25T14:05:00+07:00)».
func DongBayGio(luc time.Time) string {
	here := Now(luc)
	return fmt.Sprintf("Bây giờ: %s %02d:%02d (Asia/Ho_Chi_Minh, %s)", ngayCua(here), here.Hour(), here.Minute(), here.Format(time.RFC3339))
}

var khungBuoi = map[Buoi][2]Gio{
	BuoiSang:  {{6, 0}, {11, 0}},
	BuoiTrua:  {{11, 0}, {13, 0}},
	BuoiChieu: {{13, 0}, {18, 0}},
	BuoiToi:   {{18, 0}, {23, 0}},
	BuoiDem:   {{22, 0}, {23, 59}},
}

var tenBuoi = map[Buoi]string{BuoiSang: "buổi sáng", BuoiTrua: "buổi trưa", BuoiChieu: "buổi chiều", BuoiToi: "buổi tối", BuoiDem: "ban đêm"}

// MoTa renders the resolution in Vietnamese, for the line the model reads.
func (m Moc) MoTa() string {
	var s string
	switch m.Loai {
	case LoaiGio:
		s = m.Luc.String()
		if m.MoHo && m.LucKhac != nil {
			s += " (chưa chắc: cũng có thể là " + m.LucKhac.String() + ")"
		}
		return s
	case LoaiKhoang:
		s = m.Tu.String() + " đến " + m.Den.String()
	default:
		s = m.Tu.String()
		if k, ok := khungBuoi[m.Buoi]; ok {
			s += ", " + tenBuoi[m.Buoi] + " (" + k[0].String() + "–" + k[1].String() + ")"
		}
	}
	if m.MoHo && m.Khac != nil {
		s += " (chưa chắc: cũng có thể là " + m.Khac.String() + ")"
	}
	return s
}

type token struct {
	w          string // lowercased
	start, end int    // byte offsets in the NFC text
	hoa        bool   // starts with an upper-case letter as written
}

func tach(text string) []token {
	var out []token
	start := -1
	flush := func(end int) {
		if start < 0 {
			return
		}
		raw := strings.TrimRight(text[start:end], "/:")
		if raw != "" {
			first := []rune(raw)[0]
			out = append(out, token{w: strings.ToLower(raw), start: start, end: start + len(raw), hoa: unicode.IsUpper(first)})
		}
		start = -1
	}
	for i, r := range text {
		keep := unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.Is(unicode.Mn, r) || ((r == '/' || r == ':') && start >= 0)
		if keep {
			if start < 0 {
				start = i
			}
			continue
		}
		flush(i)
	}
	flush(len(text))
	return out
}

var (
	reNgay  = regexp.MustCompile(`^(\d{1,2})/(\d{1,2})(?:/(\d{2}|\d{4}))?$`)
	reGioH  = regexp.MustCompile(`^(\d{1,2})[hg](\d{2})?$`)
	reGioHM = regexp.MustCompile(`^(\d{1,2}):(\d{2})$`)
	reSo    = regexp.MustCompile(`^\d{1,2}$`)
	reSoPh  = regexp.MustCompile(`^\d{2}$`)
	reTN    = regexp.MustCompile(`^t([2-7])$`)
)

var thuTheoChu = map[string]int{
	"2": 0, "hai": 0, "3": 1, "ba": 1, "4": 2, "tư": 2, "bốn": 2, "5": 3, "năm": 3,
	"6": 4, "sáu": 4, "7": 5, "bảy": 5, "bẩy": 5,
}

var buoiTheoChu = map[string]Buoi{"sáng": BuoiSang, "trưa": BuoiTrua, "chiều": BuoiChieu, "tối": BuoiToi, "đêm": BuoiDem, "khuya": BuoiDem}

// gioTheoBuoi reads «7» with «tối» as 19. False when the pair makes no sense.
func gioTheoBuoi(h int, b Buoi) (int, bool) {
	switch b {
	case BuoiSang:
		if h >= 0 && h <= 11 {
			return h, true
		}
	case BuoiTrua:
		if h == 11 || h == 12 {
			return h, true
		}
		if h >= 1 && h <= 2 {
			return h + 12, true
		}
	case BuoiChieu:
		if h >= 1 && h <= 6 {
			return h + 12, true
		}
		if h >= 12 && h <= 18 {
			return h, true
		}
	case BuoiToi:
		if h >= 5 && h <= 11 {
			return h + 12, true
		}
		if h >= 17 && h <= 23 {
			return h, true
		}
	case BuoiDem:
		if h >= 8 && h <= 11 {
			return h + 12, true
		}
		if h == 12 {
			return 0, true
		}
		if (h >= 0 && h <= 4) || (h >= 20 && h <= 23) {
			return h, true
		}
	}
	return 0, false
}

// Giai finds every date and time mention in text and resolves it against luc.
func Giai(text string, luc time.Time) []Moc {
	text = norm.NFC.String(text)
	here := Now(luc)
	homNay := ngayCua(here)
	// In the small hours a person who has not slept still calls the coming
	// day «mai»: every day-relative word reads two ways, one day apart.
	khuya := here.Hour() < 5
	thuNay := homNay.Thu()
	thuHai := homNay.Cong(-thuNay)

	toks := tach(text)
	var out []Moc
	at := func(i int) string {
		if i >= 0 && i < len(toks) {
			return toks[i].w
		}
		return ""
	}
	add := func(i, j int, m Moc) {
		m.Cum = text[toks[i].start:toks[j].end]
		m.ViTri = toks[i].start
		out = append(out, m)
	}
	motNgay := func(d Ngay, b Buoi, tuongDoi bool) Moc {
		m := Moc{Loai: LoaiNgay, Tu: d, Den: d, Buoi: b}
		if tuongDoi && khuya {
			k := d.Cong(-1)
			m.MoHo, m.Khac = true, &k
		}
		return m
	}
	// «tối nay 7h»: a time right after a day with a part of the day reads in
	// that part of the day.
	buoiDen, buoiMang := -1, BuoiKhong
	for i := 0; i < len(toks); i++ {
		w := at(i)
		// «tối nay», «sáng mai», «tối 7h»: a part of the day first.
		if b, ok := buoiTheoChu[w]; ok {
			switch at(i + 1) {
			case "nay":
				add(i, i+1, motNgay(homNay, b, true))
				i++
				buoiDen, buoiMang = i, b
				continue
			case "mai":
				add(i, i+1, motNgay(homNay.Cong(1), b, true))
				i++
				buoiDen, buoiMang = i, b
				continue
			}
			if g, n, ok := docGio(toks, i+1); ok {
				if h, ok := gioTheoBuoi(g.Gio, b); ok {
					add(i, i+n, Moc{Loai: LoaiGio, Luc: Gio{h, g.Phut}})
					i += n
					continue
				}
			}
			continue
		}
		switch w {
		case "hôm", "bữa":
			if at(i+1) == "nay" {
				add(i, i+1, motNgay(homNay, BuoiKhong, true))
				i++
			}
			continue
		case "ngày":
			switch at(i + 1) {
			case "mai":
				if at(i+2) != "mốt" {
					add(i, i+1, motNgay(homNay.Cong(1), BuoiKhong, true))
					i++
				}
				continue
			case "mốt", "kia":
				add(i, i+1, motNgay(homNay.Cong(2), BuoiKhong, true))
				i++
				continue
			}
			if m, ok := docNgayThang(at(i+1), homNay); ok {
				add(i, i+1, m)
				i++
			}
			continue
		case "mai":
			// «mai mốt» is «some day»; «Mai» mid-sentence is a name.
			if at(i+1) == "mốt" {
				i++
				continue
			}
			if toks[i].hoa && i > 0 {
				continue
			}
			if p := at(i - 1); p == "hoa" || p == "cây" || p == "cành" {
				continue
			}
			add(i, i, motNgay(homNay.Cong(1), BuoiKhong, true))
			continue
		case "mốt":
			add(i, i, motNgay(homNay.Cong(2), BuoiKhong, true))
			continue
		case "cuối":
			if at(i+1) != "tuần" {
				continue
			}
			j := i + 1
			tuanSau := false
			moHo := false
			switch at(i + 2) {
			case "này":
				j = i + 2
			case "sau":
				j, tuanSau = i+2, true
			case "tới":
				// «cuối tuần tới» said before the weekend is this weekend to
				// some people and next weekend to others.
				j, tuanSau = i+2, true
				moHo = thuNay <= 4
			}
			bay := thuHai.Cong(5)
			if tuanSau {
				bay = bay.Cong(7)
			}
			m := Moc{Loai: LoaiKhoang, Tu: bay, Den: bay.Cong(1)}
			if !tuanSau && thuNay >= 5 {
				m.Tu = homNay
			}
			if moHo {
				k := thuHai.Cong(5)
				m.MoHo, m.Khac = true, &k
			}
			if !tuanSau && thuNay == 6 {
				k := thuHai.Cong(12)
				m.MoHo, m.Khac = true, &k
			}
			add(i, j, m)
			i = j
			continue
		case "tuần":
			switch at(i + 1) {
			case "sau", "tới":
				dau := thuHai.Cong(7)
				add(i, i+1, Moc{Loai: LoaiKhoang, Tu: dau, Den: dau.Cong(6)})
				i++
			case "này":
				add(i, i+1, Moc{Loai: LoaiKhoang, Tu: homNay, Den: thuHai.Cong(6)})
				i++
			}
			continue
		case "chủ":
			if at(i+1) == "nhật" {
				j, m := docThu(toks, i+1, 6, thuNay, thuHai)
				add(i, j, m)
				i = j
			}
			continue
		case "cn":
			j, m := docThu(toks, i, 6, thuNay, thuHai)
			add(i, j, m)
			i = j
			continue
		case "thứ":
			if d, ok := thuTheoChu[at(i+1)]; ok {
				j, m := docThu(toks, i+1, d, thuNay, thuHai)
				add(i, j, m)
				i = j
			}
			continue
		}
		if mm := reTN.FindStringSubmatch(w); mm != nil {
			d := thuTheoChu[mm[1]]
			j, m := docThu(toks, i, d, thuNay, thuHai)
			add(i, j, m)
			i = j
			continue
		}
		if m, ok := docNgayThang(w, homNay); ok {
			add(i, i, m)
			continue
		}
		if g, n, ok := docGio(toks, i); ok {
			j := i + n - 1
			m := Moc{Loai: LoaiGio, Luc: g}
			if b, ok := buoiTheoChu[at(j+1)]; ok {
				if h, ok := gioTheoBuoi(g.Gio, b); ok {
					m.Luc.Gio = h
					j++
				}
			} else if h, ok := gioTheoBuoi(g.Gio, buoiMang); ok && i == buoiDen+1 {
				m.Luc.Gio = h
			} else if g.Gio >= 1 && g.Gio <= 11 {
				k := Gio{g.Gio + 12, g.Phut}
				m.MoHo, m.LucKhac = true, &k
			}
			add(i, j, m)
			i = j
		}
	}
	return out
}

// docThu resolves «thứ N» at toks[i] (the day word) plus its qualifier, and
// returns the index of the last token used.
func docThu(toks []token, i, d, thuNay int, thuHai Ngay) (int, Moc) {
	at := func(k int) string {
		if k < len(toks) {
			return toks[k].w
		}
		return ""
	}
	tuanNay := thuHai.Cong(d)
	tuanSau := tuanNay.Cong(7)
	m := Moc{Loai: LoaiNgay}
	j := i
	switch q := at(i + 1); {
	case q == "tuần" && at(i+2) == "này":
		j = i + 2
		fallthrough
	case q == "này":
		if j == i {
			j = i + 1
		}
		m.Tu = tuanNay
		if d < thuNay {
			// Already gone this week: the coming one, or the one just passed.
			m.Tu = tuanSau
			m.MoHo, m.Khac = true, &tuanNay
		}
	case q == "tuần" && (at(i+2) == "sau" || at(i+2) == "tới"):
		j = i + 2
		m.Tu = tuanSau
	case q == "sau":
		j = i + 1
		m.Tu = tuanSau
	case q == "tới":
		j = i + 1
		if d > thuNay {
			m.Tu = tuanNay
			m.MoHo, m.Khac = true, &tuanSau
		} else {
			m.Tu = tuanSau
		}
	default:
		switch {
		case d > thuNay:
			m.Tu = tuanNay
		case d == thuNay:
			m.Tu = tuanNay
			m.MoHo, m.Khac = true, &tuanSau
		default:
			m.Tu = tuanSau
		}
	}
	m.Den = m.Tu
	return j, m
}

// docNgayThang reads «25/9» or «25/09/2026».
func docNgayThang(w string, homNay Ngay) (Moc, bool) {
	mm := reNgay.FindStringSubmatch(w)
	if mm == nil {
		return Moc{}, false
	}
	d, _ := strconv.Atoi(mm[1])
	t, _ := strconv.Atoi(mm[2])
	nam := homNay.Nam
	coNam := mm[3] != ""
	if coNam {
		nam, _ = strconv.Atoi(mm[3])
		if len(mm[3]) == 2 {
			nam += 2000
		}
	}
	if t < 1 || t > 12 || d < 1 || d > 31 {
		return Moc{}, false
	}
	n := ngayCua(time.Date(nam, time.Month(t), d, 12, 0, 0, 0, time.UTC))
	if n.Ngay != d {
		return Moc{}, false // 31/9 does not exist
	}
	m := Moc{Loai: LoaiNgay, Tu: n, Den: n}
	if !coNam && truoc(n, homNay) {
		k := ngayCua(time.Date(nam+1, time.Month(t), d, 12, 0, 0, 0, time.UTC))
		if k.Ngay == d {
			m.MoHo, m.Khac = true, &k
		}
	}
	return m, true
}

func truoc(a, b Ngay) bool {
	if a.Nam != b.Nam {
		return a.Nam < b.Nam
	}
	if a.Thang != b.Thang {
		return a.Thang < b.Thang
	}
	return a.Ngay < b.Ngay
}

// docGio reads a time starting at toks[i]: «7h», «7h30», «19:30», «7 giờ»,
// «7 giờ 30», «7 giờ rưỡi», «7h rưỡi». It returns how many tokens it used.
func docGio(toks []token, i int) (Gio, int, bool) {
	if i >= len(toks) {
		return Gio{}, 0, false
	}
	at := func(k int) string {
		if k < len(toks) {
			return toks[k].w
		}
		return ""
	}
	w := toks[i].w
	var h, p, n int
	switch {
	case reGioH.MatchString(w):
		mm := reGioH.FindStringSubmatch(w)
		h, _ = strconv.Atoi(mm[1])
		n = 1
		if mm[2] != "" {
			p, _ = strconv.Atoi(mm[2])
		} else if at(i+1) == "rưỡi" {
			p, n = 30, 2
		}
	case reGioHM.MatchString(w):
		mm := reGioHM.FindStringSubmatch(w)
		h, _ = strconv.Atoi(mm[1])
		p, _ = strconv.Atoi(mm[2])
		n = 1
	case reSo.MatchString(w) && at(i+1) == "giờ":
		h, _ = strconv.Atoi(w)
		n = 2
		switch nx := at(i + 2); {
		case nx == "rưỡi":
			p, n = 30, 3
		case reSoPh.MatchString(nx):
			p, _ = strconv.Atoi(nx)
			n = 3
		}
	default:
		return Gio{}, 0, false
	}
	if h > 23 || p > 59 {
		return Gio{}, 0, false
	}
	return Gio{h, p}, n, true
}

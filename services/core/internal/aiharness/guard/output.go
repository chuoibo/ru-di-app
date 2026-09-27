package guard

import (
	"regexp"
	"strings"

	"golang.org/x/text/unicode/norm"
)

// LoaiRa is why the output guard stopped an answer. It lives in tests and the
// eval only: the metrics row keeps «stopped», never the kind (ADR-0044 §2.8).
type LoaiRa string

const (
	RaSach        LoaiRa = ""
	RaSoDienThoai LoaiRa = "so_dien_thoai"
	RaEmail       LoaiRa = "email"
	RaSoTaiKhoan  LoaiRa = "so_tai_khoan"
	RaMaKiem      LoaiRa = "lo_ma_kiem"
	RaLoiNhac     LoaiRa = "lo_loi_nhac"
)

var (
	email = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
	// chuoiSo is one whole run of digits. Between two digits it allows one
	// space, dot, comma or slash («0xxx/xxx/xxx»), or a dash with a space
	// either side («0xxx - xxx - xxx»). Each run is judged whole, so the «0»
	// inside «150.000» or the tail «345.678» of a phone is never read on its
	// own. A slash with spaces round it («7.00 - 11.00 / 13.00 - 22.00») and
	// a comma before a space end the run.
	chuoiSo = regexp.MustCompile(`\d(?:(?:\s?-\s?|[\s.,/])?\d)*`)
	// ngoacSo: an area code in brackets, «(0xx) xxx xxxx», is read as if the
	// brackets were not there.
	ngoacSo = regexp.MustCompile(`\((\+?\d{1,5})\)`)
	// quanhDau: spaces around a dash inside a run, taken out before the run
	// is read as a phone.
	quanhDau = regexp.MustCompile(`\s*(-)\s*`)
	// A Vietnamese phone number: 84 (after a +) or a leading 0, then 8 to 10
	// digits.
	laSoDienThoai = regexp.MustCompile(`^(?:84|0)(?:[\s.\-/]?\d){8,10}$`)
	// gachRa: every dash a phone may be written with (hyphen, en and em
	// dash, minus) and the underscore read as a hyphen; chamCach: a dot with
	// spaces both sides between two digits, read as a dot («0912 . 345 .
	// 678»). A dot right after a number and before a space still ends it
	// («150.000. 200 người»). Review round 4 of slice 6, nit.
	gachRa   = strings.NewReplacer("‐", "-", "‑", "-", "‒", "-", "–", "-", "—", "-", "―", "-", "−", "-", "_", "-")
	chamCach = regexp.MustCompile(`(\d) +\. +(\d)`)
	// sdtLong: a phone whose groups are joined by a dot with a space on one
	// side only («0xxx .xxx .xxx», «0xxx. xxx. xxx») or by a middle dot
	// («0xxx·xxx·xxx», «0xxx · xxx · xxx»): a leading 0 or +84, then groups
	// of two to four digits, each joined by a dot-like mark with any spaces
	// or by one space, read whole as a phone (review round 5 of slice 6,
	// nit). An amount never starts with 0, so «150.000. 200 người» is still
	// two numbers; a date and its hour («dd.mm.yyyy hh giờ») is not a phone.
	sdtLong     = regexp.MustCompile(`(?:^|[^\d.,])((?:\+84|0)\d{1,4}(?:(?:\s*[.·•∙⋅‧・]\s*|\s)\d{2,4}){1,4})`)
	sdtLongMoc  = regexp.MustCompile(`\s*[.·•∙⋅‧・]\s*|\s`)
	ngayChamDau = regexp.MustCompile(`^(?:0?[1-9]|[12]\d|3[01])\.(?:0?[1-9]|1[0-2])\.(?:19|20)\d{2}`)
	// A coordinate pair, the only form let through: a latitude (to 90) and
	// a longitude (to 180), each with four to six decimal places, joined by
	// a comma («10.7768, 106.7008»). A lone coordinate, or a pair with
	// seven places or more, is read as the number it may be (review round 4
	// of slice 6, nit). As before, neither whole part starts with 0,
	// and a latitude of 84 is never read as one: a phone starts with 0 or 84.
	toaDo = regexp.MustCompile(`(^|[^\d.,])(-?(?:[1-9]|[1-8]\d|90)\.\d{4,6}),\s?(-?(?:[1-9]\d?|1[0-7]\d|180)\.\d{4,6})($|[^\d.,]|[.,](?:\D|$))`)
	// Runs that are not contact numbers: an amount in grouped thousands
	// (150.000; from a billion up only with its currency after it), amounts
	// or hours joined by dashes or slashes (khongPhaiLienLac), an amount
	// grouped by spaces -- which an account number can also look like, so it
	// needs its currency after it -- and a leading date (26.09.2026 or
	// 26/09/2026, then maybe the hour, or a dash and a second date: a range
	// of days). A coordinate pair is blanked out before any run is read
	// (boToaDo). An amount never starts with 0, so a phone written with dots
	// in groups of three, three and four is still a phone.
	soTienNhom = regexp.MustCompile(`^[1-9]\d{0,2}(?:[.,]\d{3})+$`)
	soTienCach = regexp.MustCompile(`^[1-9]\d{0,2}(?: \d{3})+$`)
	ngayDau    = regexp.MustCompile(`^(?:0?[1-9]|[12]\d|3[01])[.\-/](?:0?[1-9]|1[0-2])[.\-/](?:(?:19|20)\d{2}|\d{2})(\s?-\s?|\s|$)`)
	// What may follow a date and a space inside one run: an hour, maybe with
	// its minutes (a date, then «19», or «19 30»).
	gioSau  = regexp.MustCompile(`^(?:[01]?\d|2[0-3])(?:[\s.]?[0-5]\d)?$`)
	tienSau = regexp.MustCompile(`^\s?(?i:đồng|đ|₫|dong|vnđ|vnd)(?:$|[^\p{L}\p{M}\d])`)
)

func demSo(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			n++
		}
	}
	return n
}

// laTien says whether a grouped-thousands run is an amount: below a billion
// on its own, from a billion up only with its currency after it.
func laTien(run string, coTien bool) bool {
	return soTienNhom.MatchString(run) && (demSo(run) < 10 || coTien)
}

// khongPhaiLienLac says whether a run of digits is an amount or a date rather
// than a phone or an account; sau is the text right after the run.
func khongPhaiLienLac(run, sau string) bool {
	if g := ngayDau.FindStringSubmatch(run); g != nil {
		rest := strings.TrimSpace(run[len(g[0]):])
		switch {
		case rest == "":
			return true
		case strings.Contains(g[1], "-"):
			// A range of days: what follows the dash must itself be a date.
			// Counting its digits instead let a phone written in dashed pairs
			// pass as «a date, then a few digits».
			return ngayDau.MatchString(rest) && khongPhaiLienLac(rest, sau)
		default:
			return gioSau.MatchString(rest) || khongPhaiLienLac(rest, sau)
		}
	}
	coTien := tienSau.MatchString(sau)
	if laTien(run, coTien) || (soTienCach.MatchString(run) && coTien) {
		return true
	}
	// Amounts or hours joined by dashes or slashes: a range, a few prices in
	// a row («35.000/40.000/45.000đ»; «150 - 200 - 250 nghìn» only with its
	// unit after it), opening hours («07.00-11.00/13.00-22.00»). Small
	// numbers with no unit are not let through: «xxx-xxx-xxx-xxx» is an
	// account, «0xxx - xxx - xxx» a phone.
	donVi := tienDonVi.MatchString(sau)
	if phan := strings.FieldsFunc(run, func(r rune) bool { return r == '-' || r == '/' }); len(phan) > 1 {
		for i, p := range phan {
			p = strings.TrimSpace(p)
			if !laTien(p, coTien && i == len(phan)-1) && !gioPhut.MatchString(p) && !(donVi && soNho.MatchString(p)) {
				return false
			}
		}
		return true
	}
	return false
}

var (
	// gioPhut is an hour and its minutes, «07.00», «22.30».
	gioPhut = regexp.MustCompile(`^(?:[01]?\d|2[0-3])[.:][0-5]\d$`)
	// soNho is a number of one to three digits with no leading 0.
	soNho = regexp.MustCompile(`^[1-9]\d{0,2}$`)
	// tienDonVi is an amount's unit right after a run: currency, thousands
	// or millions.
	tienDonVi = regexp.MustCompile(`^\s?(?i:đồng|đ|₫|dong|vnđ|vnd|nghìn|ngàn|nghin|ngan|k|triệu|trieu)(?:$|[^\p{L}\p{M}\d])`)
)

// boToaDo blanks out each coordinate pair, so its digits are never read as
// one run.
func boToaDo(text string) string {
	return toaDo.ReplaceAllStringFunc(text, func(m string) string {
		g := toaDo.FindStringSubmatch(m)
		if strings.HasPrefix(strings.TrimPrefix(g[2], "-"), "84") {
			return m
		}
		return g[1] + strings.Repeat(" ", len(m)-len(g[1])-len(g[4])) + g[4]
	})
}

// soLienLac finds a phone or an account number among the digit runs.
func soLienLac(text string) LoaiRa {
	text = gachRa.Replace(ngoacSo.ReplaceAllString(text, "$1"))
	text = chamCach.ReplaceAllString(chamCach.ReplaceAllString(text, "$1.$2"), "$1.$2")
	text = boToaDo(text)
	for _, m := range sdtLong.FindAllStringSubmatch(text, -1) {
		cham := sdtLongMoc.ReplaceAllString(m[1], ".")
		if ngayChamDau.MatchString(cham) {
			continue
		}
		if so := strings.ReplaceAll(strings.TrimPrefix(cham, "+"), ".", ""); laSoDienThoai.MatchString(so) {
			return RaSoDienThoai
		}
	}
	for _, m := range chuoiSo.FindAllStringIndex(text, -1) {
		run := text[m[0]:m[1]]
		if demSo(run) < 9 || khongPhaiLienLac(run, text[m[1]:]) {
			continue
		}
		// A date in front is not part of the number that follows it -- when
		// what follows is a whole contact number on its own. Otherwise the
		// «date» was the head of a phone written in dashed pairs, and the run
		// is read whole.
		truoc := text[:m[0]]
		if d := ngayDau.FindString(run); d != "" {
			if rest := strings.TrimSpace(run[len(d):]); demSo(rest) >= 9 {
				run, truoc = rest, ""
			}
		}
		run = quanhDau.ReplaceAllString(run, "$1")
		if laSoDienThoai.MatchString(run) && (strings.HasPrefix(run, "0") || strings.HasSuffix(truoc, "+")) {
			return RaSoDienThoai
		}
		return RaSoTaiKhoan
	}
	return RaSach
}

// DauRa is what the output guard needs besides the answer: the canary nonce
// the system instruction carries, and the long lines of the prompt, which an
// answer must never quote.
type DauRa struct {
	MaKiem  string
	LoiNhac []string
}

// Kiem reads a whole answer. Every check here is STRUCTURAL: the canary
// marker (a random token of ours), a quoted clause of the instruction (our
// own text, by containment), and the data formats of privacy -- an email, a
// phone number, a bank account or card number. None reads what the words
// mean: whether an answer claims an action it cannot take, or moves money,
// is the verifier's judgement (kiemchung, the owner's rule of 2026-09-25),
// and the phrase rules that used to guess it are gone. Everything is read
// after NFKC, so fullwidth and mathematical digits and letters are the plain
// ones they look like (RE2's \d is ASCII only).
func (d DauRa) Kiem(text string) LoaiRa {
	text = norm.NFKC.String(text)
	if d.MaKiem != "" && strings.Contains(strings.ToLower(text), strings.ToLower(d.MaKiem)) {
		return RaMaKiem
	}
	if email.MatchString(text) {
		return RaEmail
	}
	if loai := soLienLac(text); loai != RaSach {
		return loai
	}
	g := Gap(text)
	for _, line := range d.LoiNhac {
		if strings.Contains(g, Gap(line)) {
			return RaLoiNhac
		}
	}
	return RaSach
}

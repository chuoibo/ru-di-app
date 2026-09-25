package guard

import (
	"regexp"
	"strings"

	"golang.org/x/text/unicode/norm"
)

// LoaiRa is why the output guard stopped an answer. It lives in tests and the
// eval only: the metrics row keeps «stopped», never the kind (ADR-0037 §2.8).
type LoaiRa string

const (
	RaSach        LoaiRa = ""
	RaSoDienThoai LoaiRa = "so_dien_thoai"
	RaEmail       LoaiRa = "email"
	RaSoTaiKhoan  LoaiRa = "so_tai_khoan"
	RaTuNhan      LoaiRa = "tu_nhan_hanh_dong"
	RaMaKiem      LoaiRa = "lo_ma_kiem"
	RaLoiNhac     LoaiRa = "lo_loi_nhac"
)

var (
	email = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
	// chuoiSo is one whole run of digits, a single separator allowed between
	// two digits. Each run is judged whole, so the «0» inside «150.000» or
	// the tail «345.678» of a phone is never read on its own.
	chuoiSo = regexp.MustCompile(`\d(?:[\s.,\-]?\d)*`)
	// A Vietnamese phone number: 84 (after a +) or a leading 0, then 8 to 10
	// digits.
	laSoDienThoai = regexp.MustCompile(`^(?:84|0)(?:[\s.\-]?\d){8,10}$`)
	// Runs that are not contact numbers: an amount in grouped thousands
	// (150.000; from a billion up only with its currency after it), a range
	// of two such amounts joined by a dash, an amount grouped by spaces --
	// which an account number can also look like, so it needs its currency
	// after it -- and a leading date (26.09.2026, then maybe the hour, or a
	// dash and a second date: a range of days). An amount never
	// starts with 0, so a phone written with dots in groups of three, three
	// and four is still a phone.
	soTienNhom = regexp.MustCompile(`^[1-9]\d{0,2}(?:[.,]\d{3})+$`)
	soTienCach = regexp.MustCompile(`^[1-9]\d{0,2}(?: \d{3})+$`)
	ngayDau    = regexp.MustCompile(`^(?:0?[1-9]|[12]\d|3[01])[.\-](?:0?[1-9]|1[0-2])[.\-](?:(?:19|20)\d{2}|\d{2})(\s?-\s?|\s|$)`)
	// What may follow a date and a space inside one run: an hour, maybe with
	// its minutes (a date, then «19», or «19 30»).
	gioSau  = regexp.MustCompile(`^(?:[01]?\d|2[0-3])(?:[\s.]?[0-5]\d)?$`)
	tienSau = regexp.MustCompile(`^\s?(?i:đồng|đ|₫|dong|vnđ|vnd)(?:$|[^\p{L}\p{M}\d])`)
	// A claim, in Nếp's own voice, of an action it cannot take: Nếp has no
	// tool that moves money, writes an outing or sends anything to anyone.
	// «Sent» is a claim whoever it went to («Mình đã gửi cho bạn 200k», «…
	// mã QR», «… lời mời vào kèo»); only sending «you» the answer itself --
	// suggestions, a list, a plan, a link, a map, «ở trên» -- describes the
	// answer and passes (traLoiGui). Adding to the list of suggestions passes
	// too («Mình đã thêm vào danh sách gợi ý»); adding to an outing, a group,
	// a calendar or the ledger does not.
	tuNhan       = regexp.MustCompile(`\b(minh|nep|toi|em|to)\s+(da|vua|da vua)\s+(chuyen (tien|khoan|cho|\d\S*)|(ck|bank|ban|tra|gui) \d\S*|ghi (no|vao so|lai khoan|khoan)|thanh toan|tra (tien|no|giup|ho|gium|thay|cho|lai)|tao (keo|nhom|binh chon|lich|khoan|nhac)|chot (keo|lich|dia diem|quan|cho)|dat (ban|cho|lich|ve|phong|keo)|gui (tin|loi moi|tien|ma|qr|stk|so tai khoan|cho \S+)|ck|nhan tin|moi (ban|nguoi|ca nhom|moi nguoi)|them (ban|nguoi|vao (keo|nhom|lich|so|ca nhom|danh sach (keo|muon di|yeu thich)))|xoa (keo|tin|khoan|anh|nhom)|huy (keo|lich|ban|cho)|luu (lai|vao)|hen (gio|lich)|nhac (ban|moi nguoi|ca nhom|lich)|(cap nhat|sua|doi) (keo|lich|khoan))\b`)
	tuNhanDauCau = regexp.MustCompile(`(^|[.!?]\s*)(da|vua) (chuyen (tien|khoan|cho)|ghi no|thanh toan|tao keo|chot keo|dat ban|dat cho|gui (tin|loi moi|tien|cho \S+))\b`)
	// traLoiGui is sending «you» the answer itself: what it names can only be
	// the text on the screen. It is taken out before the claim patterns read
	// the answer, and nothing else is.
	traLoiGui = regexp.MustCompile(`\bgui cho ban (?:(?:vai|mot vai|may|cac|nhung|mot|them|mot so) )?(?:goi y|danh sach(?: goi y| quan| dia diem| cho)?|lich trinh|link|duong link|ban do|o tren|phia tren|ben tren)\b`)
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
	if a, b, ok := strings.Cut(run, "-"); ok {
		return laTien(a, false) && laTien(b, coTien)
	}
	return false
}

// soLienLac finds a phone or an account number among the digit runs.
func soLienLac(text string) LoaiRa {
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

// Kiem reads a whole answer. S1 never streams, so it reads the full text;
// the streaming window of a later slice calls the same function. Everything
// is read after NFKC, so fullwidth and mathematical digits and letters are
// the plain ones they look like (RE2's \d is ASCII only).
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
	if c := traLoiGui.ReplaceAllString(g, "qqtraloi"); tuNhan.MatchString(c) || tuNhanDauCau.MatchString(c) {
		return RaTuNhan
	}
	for _, line := range d.LoiNhac {
		if strings.Contains(g, Gap(line)) {
			return RaLoiNhac
		}
	}
	return RaSach
}

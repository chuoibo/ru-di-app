package guard

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

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
	// A claim, in Nếp's own voice, of an action it cannot take: Nếp has no
	// tool that moves money, writes an outing or sends anything to anyone.
	// «Sent» is a claim whoever it went to («Mình đã gửi cho bạn 200k», «…
	// mã QR», «… lời mời vào kèo»), with or without «cho» («Mình đã gửi Nam
	// 200k», «Nếp đã bắn Nam 300k»), and after «giúp bạn» («Mình đã giúp bạn
	// chuyển 200k cho Nam»); only sending «you» the answer itself --
	// suggestions, a list, a plan, a link, a map, «ở trên» -- describes the
	// answer and passes (traLoiGui). Adding to the list of suggestions passes
	// too («Mình đã thêm vào danh sách gợi ý»); adding to an outing, a group,
	// a calendar or the ledger does not.
	tuNhan = regexp.MustCompile(`\b(minh|nep|toi|em|to)\s+(da|vua|da vua)\s+((giup|ho|thay)\s+(ban|cau|anh|chi|em|nhom|ca nhom|moi nguoi)\s+)?` +
		`(chuyen (tien|khoan|cho|giup|ho|gium|thay|lai|\d\S*)|(ck|bank|ban|tra|gui) \d\S*|ghi (no|vao so|lai khoan|khoan)|ghi(\s+lai)?(\s+\S+){1,2}\s+no(\s+\S+){1,2}\s+(` + soTienRa + `)|ghi lai(\s+\S+){0,3}\s+(no|tien|` + soTienRa + `)|thanh toan|tra (tien|no|giup|ho|gium|thay|cho|lai)|tao (keo|nhom|binh chon|lich|khoan|nhac)|chot (keo|lich|dia diem|quan|cho)|dat (ban|cho|lich|ve|phong|keo)|gui (tin|loi moi|tien|ma|qr|stk|so tai khoan|cho \S+)|ck|nhan tin|moi (ban|nguoi|ca nhom|moi nguoi)|them (ban|nguoi|vao (keo|nhom|lich|so|ca nhom|danh sach (keo|muon di|yeu thich)))|xoa (keo|tin|khoan|anh|nhom)|huy (keo|lich|ban|cho)|luu (lai|vao)|hen (gio|lich)|nhac (ban|moi nguoi|ca nhom|lich)|nhac \S+ (tra|chuyen|dong|gop|ck|gui)|(cap nhat|sua|doi) (keo|lich|khoan))\b`)
	// A debt told, then recorded: «Nam nợ bạn 150k, mình ghi lại rồi». Read
	// on the copy where «nó», «nở», «nổ»… are kept apart from «nợ» (tachDong):
	// «Nó bán tầm 150k một phần, mình ghi lại rồi» owes nothing (review round
	// 5 of slice 6, NEW-4).
	tuNhanGhiNo  = regexp.MustCompile(`\bno(\s+\S+){1,2}\s+(` + soTienRa + `)[^.!?]*\b(minh|nep|toi|em|to)\s+(da\s+|vua\s+)?ghi(\s+lai)?\s+(roi|xong)\b`)
	tuNhanDauCau = regexp.MustCompile(`(^|[.!?]\s*)(da|vua) (chuyen (tien|khoan|cho)|ghi no|thanh toan|tao keo|chot keo|dat ban|dat cho|gui (tin|loi moi|tien|cho \S+))\b`)
	// Money that has moved, told without «đã» or in the passive: «Mình
	// chuyển cho Nam 200k rồi», «200k đã được chuyển cho Nam». The first two
	// forms name an amount and are judged by laTuNhanSo, like tuNhanSo:
	// «Mình chuyển quán 200k rồi» pays the place.
	tuNhanRoi = regexp.MustCompile(`\b(?:minh|nep|toi|em|to)\s+(?:(ck|bank|tra)(?:\s+cho)?(?:\s+\S+){0,2}|(chuyen|ban|dua)((?:\s+cho)?(?:\s+\S+){0,2}))\s+(` + soTienRa + `)(?:\s+\S+){0,2}\s+roi\b` +
		`|\b(?:minh|nep|toi|em|to)\s+(gui)((?:\s+cho)?(?:\s+\S+)?)\s+(` + soTienRa + `)(?:\s+\S+){0,2}\s+roi\b`)
	tuNhanRoiCho = regexp.MustCompile(`\b(minh|nep|toi|em|to)\s+(chuyen|ck|bank|tra|ban)\s+(tien\s+|khoan\s+)?cho\s+\S+(\s+\S+)?\s+roi\b` +
		`|(\b(tien|khoan)|` + soTienRa + `)\s+(da|vua)\s+(duoc\s+(chuyen|gui|tra|thanh toan|ck)|(chuyen|ck)\s+(cho|sang|qua))\b`)
	// traLoiGui is sending «you» the answer itself: what it names can only be
	// the text on the screen. It is taken out before the claim patterns read
	// the answer -- unless what it names is a way to pay («gửi cho bạn link
	// chuyển khoản 200k», «… link thanh toán», «… danh sách chia tiền»:
	// tienDauTraLoi), or the rest of its clause moves money («… gợi ý và
	// chuyển luôn 200k cho Nam»: tienSauTraLoi). Those stay claims.
	traLoiGui     = regexp.MustCompile(`\bgui cho ban (?:(?:vai|mot vai|may|cac|nhung|mot|them|mot so|\d+) )?(?:goi y|danh sach(?: goi y| quan| dia diem| cho)?|lich trinh|link|duong link|ban do|o tren|phia tren|ben tren|quan|dia diem|nha hang)\b`)
	tienDauTraLoi = regexp.MustCompile(`^\s*(?:(?:de|cua|ve) )?(?:chuyen khoan|ck|thanh toan|chia tien|chuyen tien|tra tien|gop tien|dong tien|thu tien|tien|stk|so tai khoan|tai khoan|ma qr|qr|no)\b`)
	tienSauTraLoi = regexp.MustCompile(`\bchuyen\s+(?:(?:luon|lai|them|not|ngay|giup|ho)\s+)?(?:` + soTienRa + `)|\bchuyen (?:tien|khoan)\b|\b(?:ck|bank)(?:\s+\S+)?\s+(?:` + soTienRa + `)`)
	// tuNhanSo: a money verb, at most two words, then an amount, in Nếp's
	// voice («Mình vừa chuyển anh Tuấn 1 triệu»); «gửi» takes one word, or a
	// place written in two («Mình vừa gửi khách sạn 2 triệu tiền phòng»).
	// Read on the copy where a place or a plan is marked (docNoi) and judged
	// by laTuNhanSo: «ck», «bank» and «trả» pay whoever they name («Mình đã
	// trả quán 200k tiền cọc»); after «chuyển», «đưa», «bắn», «gửi» a place
	// is paid too («Mình đã gửi quán 200k tiền cọc»), unless the answer is
	// moving the place in its list («Mình đã chuyển sang quán 150k gần hơn»,
	// «Mình vừa đưa quán 120k lên đầu danh sách»). Review round 5 of slice
	// 6, NEW-1.
	tuNhanSo = regexp.MustCompile(`\b(?:minh|nep|toi|em|to)\s+(?:da|vua|da vua)\s+(?:(?:giup|ho|thay)\s+(?:ban|cau|anh|chi|em|nhom|ca nhom|moi nguoi)\s+)?` +
		`(?:(ck|bank|tra)(?:\s+\S+){0,2}|(chuyen|dua|ban)((?:\s+\S+){0,2})|(gui)((?:\s+\S+)?|\s+§(?:khach san|nha hang|dia diem|danh sach|goi y|ban do|thuc don)))\s+(` + soTienRa + `)`)
	// tuNhanSoDau: the verbs that only transfer, opening a sentence with
	// «đã/vừa» and no subject («Đã chuyển 200k cho Nam rồi nhé», «Vừa chuyển
	// 100k cho Tú xong»). A condition or a pair is not a claim: «Đã chuyển
	// cọc 200k thì quán không hoàn lại», «Vừa bắn tin quán 150k vừa gửi địa
	// chỉ cho bạn»; nor is parking or a cloakroom, «gửi xe», «gửi đồ»: «Đã
	// gửi xe 20k thì vào cổng sau» (review round 5 of slice 6, NEW-4).
	tuNhanSoDau = regexp.MustCompile(`(?:^|[.!?]\s*)(da|vua)\s+(?:(ck|bank)(?:\s+\S+){0,2}|(chuyen|ban|gui)((?:\s+\S+){0,2}))\s+(` + soTienRa + `)`)
	// xepNoi: what follows the amount places or ranks the place it names --
	// «lên đầu danh sách», «xuống cuối danh sách», «vào danh sách», «lên
	// trên», «gần hơn», «ở dưới», maybe after «/người» or «cho bạn» (a list
	// word may carry docNoi's §).
	xepNoi = regexp.MustCompile(`^(?:\s*/\s*(?:nguoi|ng|suat|phan)|\s+(?:mot|moi|1) (?:nguoi|suat|phan))?(?:\s+cho (?:ban|cac ban|nhom|ca nhom|moi nguoi))?\s+(?:(?:len|xuong|vao)(?:\s+\S+){0,2}?\s+§?(?:danh sach|dau|cuoi|top|tren|duoi)\b|gan\b|o\b|quanh\b)`)
	// tienSauSo: money right after the amount, or the end of its clause:
	// «… 200k tiền cọc», «… 200k rồi», «… 200k.».
	tienSauSo = regexp.MustCompile(`^\s*(?:(?:tien|coc|dat coc|roi|xong)\b|(?:(?:nhe|nha|nhen|a|ha)\s*)?(?:[.!?;,]|$))`)
	// tienTrongVe: money named later in the same clause.
	tienTrongVe = regexp.MustCompile(`\b(?:tien|coc)\b`)
	// dieuKien: «thì» in a sentence with no «rồi», «xong» states a
	// condition; roiXong makes it a report again («Đã ck 200k thì quán giữ
	// bàn rồi nhé»).
	dieuKien = regexp.MustCompile(`\bthi\b`)
	roiXong  = regexp.MustCompile(`\b(?:roi|xong)\b`)
	// vuaNua: a second «vừa» in a sentence that opens with one («Vừa … vừa
	// …»: both … and …).
	vuaNua = regexp.MustCompile(`\bvua\b`)
	// tiepTraLoi: a later clause of the same sentence, in the same voice and
	// with no subject of its own, that moves money after «gửi cho bạn <câu
	// trả lời>»: «…, và chuyển 200k cho Nam rồi», «…; tiện thể chuyển luôn
	// 300k cho Lan» (review round 4 of slice 6, nit). «…, bạn chuyển 200k
	// cho Nam là xong» tells the reader what to do, and passes.
	tiepTraLoi = regexp.MustCompile(`[,;]\s*(?:(?:va|roi|xong|tien the|dong thoi|cung|luon|sau do|them)\s+)*(?:da\s+|vua\s+)?(?:chuyen\s+(?:(?:luon|lai|them|not|ngay|giup|ho)\s+)?(?:` + soTienRa + `)|(?:ck|bank)(?:\s+\S+)?\s+(?:` + soTienRa + `))`)
	// cuoiCau ends a sentence: ! ? or a full stop (a dot not inside a number).
	cuoiCau = regexp.MustCompile(`[!?]|\.(?:\D|$)`)
	// noiDau: a place or a plan, spelled with its marks («quán», not the
	// name «Quân»), that docNoi marks with «§» -- unless «cho» comes right
	// before it: «chuyển cho quán 300k» pays the place.
	noiDau = regexp.MustCompile(`(?i)(?:^|[^\p{L}\p{M}\d])(quán|tiệm|kèo|lịch|điểm|địa điểm|món|thực đơn|nhà hàng|danh sách|gợi ý|bản đồ|khách sạn|quận|menu|link|homestay|resort)`)
	// traCuu: «tra» with no marks is to look something up only where a word
	// of looking up follows it, spelled with its own marks («tra cứu», «tra
	// thử», «tra giá», «tra quán», «tra xem», «tra trên Google»), or where
	// it ends its clause after «giúp/hộ bạn» («Mình đã tra giúp bạn: quán A
	// 120k»). Anything else -- «tra 200k», «tra Nam», «tra tiền», «tra giúp
	// bạn 200k» -- may be «trả» typed without marks and is read as paying,
	// whatever marks the rest of the answer has (review round 5 of slice 6,
	// NEW-2; round 4, R3).
	traCuu = regexp.MustCompile(`(?i)(^|[^\p{L}\p{M}\d])tra(\s+(?:cứu|thử|giá|quán|tiệm|xem|google|gg|map|maps|bản đồ|trên|thông tin|địa chỉ|menu|review|giờ)(?:[^\p{L}\p{M}\d]|$)|(?:\s+(?:giúp|hộ|thử)(?:\s+(?:bạn|cả nhóm|nhóm|mọi người))?)?\s*:)`)
	// chu is one word as typed, for tachDong.
	chu = regexp.MustCompile(`[\p{L}\p{M}]+`)
	// coSoTien: an amount among the words between a verb and its amount --
	// the verb's object is that first amount.
	coSoTien = regexp.MustCompile(`(?:^|\s)(?:` + soTienRa + `)`)
	// Places, or a list of them, whatever words follow.
	traLoiNoi = regexp.MustCompile(`(?:quan|dia diem|cho|nha hang)$`)
	// cuoiVe ends a clause: , ; ! ? or a full stop (a dot not inside a number).
	cuoiVe = regexp.MustCompile(`[,;!?]|\.(?:\D|$)`)
)

// soTienRa is an amount as an answer writes it, on Gap text: digits with a
// unit, or grouped thousands. A bare count («3 gợi ý») is not one.
const soTienRa = `\d+(?:[.,]\d+)*\s?(?:k|nghin|ngan|trieu|tr|cu|lit|xi|d|dong|vnd|usd)\b|\d{1,3}(?:[.,]\d{3})+`

// boTraLoi takes out each «gửi cho bạn …» that sends the answer itself, and
// keeps the ones that send a way to pay or move money in the same clause.
func boTraLoi(g string) string {
	var b strings.Builder
	cuoi := 0
	for _, m := range traLoiGui.FindAllStringIndex(g, -1) {
		duoi := g[m[1]:]
		if k := cuoiVe.FindStringIndex(duoi); k != nil {
			duoi = duoi[:k[0]]
		}
		cau := g[m[1]:]
		if k := cuoiCau.FindStringIndex(cau); k != nil {
			cau = cau[:k[0]]
		}
		if (!traLoiNoi.MatchString(g[m[0]:m[1]]) && tienDauTraLoi.MatchString(duoi)) || tienSauTraLoi.MatchString(duoi) || tiepTraLoi.MatchString(cau) {
			continue
		}
		b.WriteString(g[cuoi:m[0]])
		b.WriteString("qqtraloi")
		cuoi = m[1]
	}
	b.WriteString(g[cuoi:])
	return b.String()
}

// docNoi marks each place or plan the answer spells with its marks, for
// tuNhanSo and tuNhanRoi. In an answer written without marks nothing is
// marked: «quan» may be the name Quân.
func docNoi(text string, coDau bool) string {
	if !coDau {
		return text
	}
	var b strings.Builder
	cuoi := 0
	for _, m := range noiDau.FindAllStringSubmatchIndex(text, -1) {
		dau := m[2]
		// A whole word only («quán», not «quánx»), and not after «cho».
		if r, _ := utf8.DecodeRuneInString(text[m[1]:]); m[1] < len(text) && (unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.Is(unicode.M, r)) {
			continue
		}
		if strings.HasSuffix(strings.ToLower(text[:dau]), "cho ") {
			continue
		}
		b.WriteString(text[cuoi:dau])
		b.WriteString("§")
		cuoi = dau
	}
	b.WriteString(text[cuoi:])
	return b.String()
}

// dongTien and dongNo name, for tachDong, the money words whose homographs
// matter: after an amount «tiện đường» and «một cốc» are not money, and «nó»
// owes nothing.
var (
	dongTien = map[string]string{"tien": "tiền", "coc": "cọc"}
	dongNo   = map[string]string{"no": "nợ"}
)

// tachDong keeps apart each word of text that folds onto a key of dong but
// is spelled with other marks than dong's spelling («tiện», «cốc», «nó»): it
// becomes the folded word with «_» appended, which no pattern reads. A word
// typed without marks stays as it is: it may be the money word.
func tachDong(text string, dong map[string]string) string {
	return chu.ReplaceAllStringFunc(text, func(w string) string {
		lo := strings.ToLower(w)
		f := Gap(lo)
		if sp, ok := dong[f]; ok && lo != f && lo != sp {
			return f + "_"
		}
		return w
	})
}

// coTuNhanSo says whether any match of re (tuNhanSo or the amount forms of
// tuNhanRoi) on the place-marked copy g claims money moved. In each match
// the groups that took part are the verb, the words between it and the
// amount (for the verbs a place may follow), and the amount.
func coTuNhanSo(re *regexp.Regexp, g string) bool {
	for _, m := range re.FindAllStringSubmatchIndex(g, -1) {
		var nhom [][2]int
		for k := 2; k < len(m); k += 2 {
			if m[k] >= 0 {
				nhom = append(nhom, [2]int{m[k], m[k+1]})
			}
		}
		dt, cua := g[nhom[0][0]:nhom[0][1]], ""
		if len(nhom) == 3 {
			cua = g[nhom[1][0]:nhom[1][1]]
		}
		if laTuNhanSo(dt, cua, g[nhom[len(nhom)-1][1]:]) {
			return true
		}
	}
	return false
}

// laTuNhanSo judges one money verb dt with the words cua between it and an
// amount, and sau the text after the amount. «ck», «bank», «trả» pay
// whoever they name. After «chuyển», «đưa», «bắn», «gửi», a place spelled
// with its marks (§, docNoi) is paid too -- «Mình đã chuyển quán 300k tiền
// cọc rồi», «Mình gửi tiệm 100k rồi» -- unless the answer moves the place in
// its list: what follows the amount places or ranks it («lên đầu danh
// sách», «gần hơn», «ở dưới»), or «sang/qua» follows the verb and no money
// or clause end follows the amount («Mình đã chuyển sang quán 150k cho
// bạn»). Money named later in the clause («… ở Quận 1 tiền cọc») keeps it a
// claim (review round 5 of slice 6, NEW-1).
func laTuNhanSo(dt, cua, sau string) bool {
	if dt == "ck" || dt == "bank" || dt == "tra" || !strings.Contains(cua, "§") || coSoTien.MatchString(cua) {
		return true
	}
	ve := sau
	if k := cuoiVe.FindStringIndex(ve); k != nil {
		ve = ve[:k[0]]
	}
	if tienTrongVe.MatchString(ve) {
		return true
	}
	if xepNoi.MatchString(sau) {
		return false
	}
	if tu := strings.Fields(cua); (tu[0] == "sang" || tu[0] == "qua") && !tienSauSo.MatchString(sau) {
		return false
	}
	return true
}

// coTuNhanSoDau reads the sentence-opening form, tuNhanSoDau, on the
// place-marked copy g: a sentence that states a condition («… thì …» with
// no «rồi», «xong» in it), a pair («Vừa … vừa …») or parking («gửi xe»,
// «gửi đồ») is not a claim; the rest is judged as tuNhanSo is.
func coTuNhanSoDau(g string) bool {
	for _, m := range tuNhanSoDau.FindAllStringSubmatchIndex(g, -1) {
		cau := g[m[11]:]
		if k := cuoiCau.FindStringIndex(cau); k != nil {
			cau = cau[:k[0]]
		}
		if dieuKien.MatchString(cau) && !roiXong.MatchString(cau) {
			continue
		}
		if g[m[2]:m[3]] == "vua" && vuaNua.MatchString(cau) {
			continue
		}
		dt, cua := "ck", ""
		if m[6] >= 0 {
			dt, cua = g[m[6]:m[7]], g[m[8]:m[9]]
		}
		if tu := strings.Fields(cua); dt == "gui" && len(tu) > 0 && (tu[0] == "xe" || tu[0] == "do") {
			continue
		}
		if laTuNhanSo(dt, cua, g[m[11]:]) {
			return true
		}
	}
	return false
}

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
	coDau := strings.IndexFunc(text, func(r rune) bool { return r > unicode.MaxASCII && unicode.IsLetter(r) }) >= 0
	tra := traCuu.ReplaceAllString(traCuu.ReplaceAllString(text, "${1}tra_${2}"), "${1}tra_${2}")
	c := boTraLoi(Gap(tra))
	cNoi := boTraLoi(Gap(docNoi(tachDong(tra, dongTien), coDau)))
	cNo := boTraLoi(Gap(tachDong(tra, dongNo)))
	if tuNhan.MatchString(c) || tuNhanDauCau.MatchString(c) || tuNhanGhiNo.MatchString(cNo) || tuNhanRoiCho.MatchString(c) ||
		coTuNhanSo(tuNhanSo, cNoi) || coTuNhanSo(tuNhanRoi, cNoi) || coTuNhanSoDau(cNoi) {
		return RaTuNhan
	}
	for _, line := range d.LoiNhac {
		if strings.Contains(g, Gap(line)) {
			return RaLoiNhac
		}
	}
	return RaSach
}

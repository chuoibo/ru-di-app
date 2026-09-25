package guard

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"mobile/services/core/internal/domain/promptsafety"
)

// The money law for Nếp, deterministic and before any model call (ADR-0033
// §2.2, ADR-0036 §2.9, design 01 §3.2, CLAUDE.md «ba luật tiền»): Nếp never
// moves, records, splits, settles or chases money, and never works out who
// owes what. A question that asks it to is refused with a fixed sentence and
// costs no model call. Asking what a place costs, or which place fits a
// budget, is place search and passes.
//
// People ask for money in any word order («chuyển 200k cho Nam», «200K cho
// Lan, chuyển giúp mình»), in any amount form (200k, 1tr2, 1.200.000đ, «hai
// trăm nghìn», «nửa triệu», «2 lít», «1 củ»), with or without tone marks, and
// in English. So the law reads the question in three steps:
//
//  1. words: Gap (NFKC, look-alikes, folded) split into words. A word typed
//     WITH tone marks that folds onto a money word but is not that word
//     («nó», «trà», «đợi», «bạn», «muốn», «chuyến», «hoãn») keeps a mark of
//     its own, so it cannot be read as «nợ», «trả», «đòi», «bắn», «mượn»,
//     «chuyển», «hoàn»; unmarked text stays ambiguous and is read as money.
//     A few fixed compounds whose first word is a money verb («kiểm tra»,
//     «trả lời», «chia sẻ», «góp ý», «trà sữa») are glued into one word;
//  2. amounts: every amount becomes one word, qqtien, so the rules below
//     can say «a money verb within three words of an amount», either way;
//  3. rules, grouped by what the person asks for: move, split, record,
//     debt, collect, account or QR, spent. Any one match refuses.
//
// A question about what a word means («ck là gì», «stk là viết tắt của
// gì») is not a request and passes.

// dauTien lists, for each folded word a rule reads as money, the spellings
// with marks that mean it. Any other marked spelling of the same letters is a
// homograph and is read as the folded word with «_» appended, which no rule
// reads. An empty list means only the unmarked spelling is money («vay»,
// «thu»).
var dauTien = map[string][]string{
	"no":     {"nợ"},
	"tra":    {"trả"},
	"dong":   {"đóng", "đồng"},
	"doi":    {"đòi", "dõi"},
	"ban":    {"bắn"},
	"muon":   {"mượn"},
	"vay":    {},
	"tien":   {"tiền"},
	"ung":    {"ứng"},
	"thu":    {},
	"tinh":   {"tính"},
	"gop":    {"góp"},
	"coc":    {"cọc"},
	"cu":     {"củ"},
	"ngan":   {"ngàn"},
	"nua":    {"nửa"},
	"khoan":  {"khoản"},
	"chi":    {},
	"tieu":   {"tiêu"},
	"hoan":   {"hoàn"},
	"nap":    {"nạp"},
	"so":     {"số", "sổ"},
	"chuyen": {"chuyển"},
	"dat":    {"đặt"},
	"vi":     {"ví"},
	"lai":    {"lại"},
	"quy":    {"quỹ"},
	"gui":    {"gửi"},
	"may":    {"mấy"},
}

// ghep glues compounds whose first word is also a money verb, so no window
// rule can read that verb in them.
var ghep = strings.NewReplacer(
	" kiem tra ", " kiemtra ",
	" tra loi ", " traloi ",
	" tra cuu ", " tracuu ",
	" tra phong ", " traphong ",
	" chia se ", " chiase ",
	" gop y ", " gopy ",
	" so tay ", " sotay ",
	" gui xe ", " guixe ",
	// «trà …» typed without marks: the common drinks.
	" tra sua ", " trasua ", " tra da ", " trada ", " tra chanh ", " trachanh ",
	" tra dao ", " tradao ", " tra xanh ", " traxanh ", " tra tac ", " tratac ",
	" tra gung ", " tragung ", " tra sen ", " trasen ", " tra hoa ", " trahoa ",
	" tra nong ", " tranong ", " tra thai ", " trathai ", " tra atiso ", " traatiso ",
	" tra vai ", " travai ", " tra olong ", " traolong ", " tra oolong ", " traoolong ",
	" tra matcha ", " tramatcha ", " tra chieu ", " trachieu ", " tra trai cay ", " tratraicay ",
)

// soTienRe is an amount, on the folded words: digits with a unit (200k, 1,5
// triệu, 2 lít, 5 xị, 1 củ, 20 bucks), the tr/m shorthand (1tr2, 1m2 -- a
// bare 500m is a distance), grouped thousands (1.200.000, 150 000 000 đồng),
// a long run with đ, dollars, and amounts in words («hai trăm nghìn», «năm
// chục», «nửa triệu», «vài trăm nghìn»).
var soTienRe = regexp.MustCompile(`\$ ?\d+(?:[.,]\d+)*` +
	`|\b\d+(?:[.,]\d+)* ?(?:k|nghin|ngan|trieu|tr|cu|lit|xi|chuc|bucks|dollars?|usd|vnd)\b` +
	`|\b\d+(?:tr|m)\d+\b` +
	`|\b\d{1,3}(?:[.,]\d{3})+(?: ?(?:d|dong))?\b` +
	`|\b\d{1,3}(?: \d{3})+ ?(?:d|dong|vnd)\b` +
	`|\b\d{4,} ?(?:d|dong)\b` +
	`|\b(?:mot|hai|ba|bon|tu|nam|sau|bay|tam|chin|muoi|nua|vai|may|tram|chuc)(?: (?:tram|chuc|muoi))? (?:nghin|ngan|trieu|chuc|cu|lit|xi)\b`)

// nghiaTu is a question about what one word means.
var nghiaTu = regexp.MustCompile(`^(?:chu |tu |cum tu )?\S+ (?:trong(?: \S+){1,3} )?(?:la gi|nghia la gi|co nghia la gi|nghia gi|la viet tat cua gi|la viet tat cua chu gi|viet tat cua gi)$|^what (?:does|is) \S+(?: mean)?$`)

// w is a window of 0..n words between two anchors, spaces included.
func w(n int) string { return `(?: \S+){0,` + strconv.Itoa(n) + `} ` }

type luatTien struct {
	ten string
	re  *regexp.Regexp
}

const (
	// A verb that moves money.
	dongTuChuyen = `(?:chuyen|ck|gui|tra|bank|send|transfer|wire|pay|paid|thanh toan|hoan|nap|gop|dong|ung|venmo|split|chia|momo)`
	// A verb that, after an amount, asks for it to be moved or split.
	dongTuSau = `(?:chuyen (?:cho|giup|gium|ho|lien|luon|di|nhe|nha|lai|ngay|dum|voi)|chuyen$|ck|bank|send|transfer|split|chia (?:deu|doi_?|ra|lai|cho|sao|\d+|tien|bill|moi|phan|giup|gium|ho|nhu|theo)|chia$)`
	// Words that may sit between a verb and what it acts on.
	dem = `(?: (?:lai|vao|giup|gium|ho|minh|cho|moi|nhanh|luon|ngay|toi|them|truoc))*`
)

var luatTiens = []luatTien{
	// Words that only ever mean money.
	{"tu_rieng", regexp.MustCompile(`\b(?:chuyen (?:tien|khoan)|ckhoan|momo|zalopay|vnpay|shopeepay|viettelpay|vietqr|vi dien tu|so tai khoan|tai khoan ngan hang|tat toan|quyet toan|venmo|paypal|zelle|cashapp|refund(?:ed|s)?|expenses?|bank account|settle up|owe[sd]?|owing|tien coc|dat coc|tam ung|stk)\b`)},
	// Move: a money verb and «tiền», or a money verb and an amount in
	// either order.
	{"dong_tu_tien", regexp.MustCompile(`\b(?:gui|chuyen|tra|dong|gop|doi|thu|hoan|ung|nap|muon|vay|xin|nop|ck|thanh toan|bank|chia|quyt|om)(?: (?:lai|truoc|ho|giup|gium|het|not))? tien\b`)},
	{"dong_tu_so", regexp.MustCompile(`\b` + dongTuChuyen + w(3) + `qqtien\b`)},
	{"so_dong_tu", regexp.MustCompile(`\bqqtien` + w(3) + dongTuSau + `\b`)},
	{"ban_ck", regexp.MustCompile(`\bban cho \S+ qqtien|\bban qqtien cho\b|\b(?:chua|da|vua|roi) ck\b|\bck (?:lai|ho|giup|gium|dum|tien|khoan|truoc|luon|ngay|lien)\b|\bqqtien` + w(3) + `ck\b|\bbank` + w(2) + `qqtien`)},
	{"thanh_toan", regexp.MustCompile(`\bthanh toan (?:giup|ho|gium|cho|tien|khoan|no|bill|hoa don|lai|truoc)\b|\bhoan (?:lai )?(?:tien|coc|qqtien)\b|\bhoan lai` + w(2) + `qqtien|\bnap` + w(2) + `(?:qqtien|tien|vao vi)\b|\btra (?:ho|gium|dum)\b|\btra sau\b`)},
	{"tieng_anh", regexp.MustCompile(`\bpa(?:y|id)(?: \S+)? back\b|\b(?:hasnt|havent|hadnt|didnt|not|who|already|never|yet) paid\b|\bsettle (?:up|the (?:bill|debt|tab))\b|\bremind \S+ to pay\b|\b(?:transfer|send|wire|pay) (?:the )?(?:money|cash|funds)\b|\bhow much (?:does|do|should|did|will) (?:each|every|everyone|everybody)\b|\bsplit` + w(2) + `(?:bill|check|cost|costs|it|tab|ways)\b`)},
	// Split: «chia» a sum or a bill, or what each person pays.
	{"chia", regexp.MustCompile(`\bchia` + w(2) + `(?:tien|bill|hoa don|khoan|chi phi)\b|\bmoi (?:nguoi|dua|ban|ban_|ng|thang)(?: \S+)?(?: phai)?(?: (?:tra|gop|dong|chiu|chuyen|no|mat|ton|chia|ck|tieu))? (?:bao nhieu|bn|may tien|may$)|\b(?:bao nhieu|bn) tien (?:1|mot|moi) (?:ng|nguoi)\b|\btinh tien (?:cho|giup|gium|ho|moi|ca|nhom)\b|\bbill qqtien|\b(?:chia|split|tra|pay|tinh|thanh toan|trong)(?: \S+){0,2} bill\b`)},
	// Record: write, change or keep track of a spend or a debt.
	{"ghi", regexp.MustCompile(`\b(?:ghi|luu|them|nhap|sua|xoa|tao|cong|theo doi)` + dem + ` (?:khoan|chi phi|chi tieu|no|hoa don|tong chi|danh sach chi)\b|\b(?:ghi|luu|nhap)` + dem + ` tien\b|\b(?:ghi|luu|nhap|them)` + w(2) + `vao so\b|\bkhoan(?: chi)?` + w(2) + `qqtien`)},
	// Debt: owing, lending, borrowing, advancing.
	{"no", regexp.MustCompile(`\b(?:ghi|tra|xoa|doi|nhac|gach|chia|tinh|don|thu|bi|khat|quyt|om|vo|mac) no\b|\bno` + w(3) + `(?:qqtien|bao nhieu|bn)\b|\bno (?:\S+ )?tien\b|\bai (?:dang |con |van |chua |da )?no\b|\bno ai\b|\bthieu` + w(2) + `(?:qqtien|tien)\b|\bcho(?: \S+){1,2} vay\b|\b(?:vay|muon) (?:tien|qqtien)\b|\bmuon (?:minh|toi|tao|em|anh|chi|to|tui|ban|nguoi ta) qqtien|\bung(?: (?:truoc|them|ho|giup))? (?:qqtien|tien)\b`)},
	// Collect: chase, gather or pay in.
	{"thu", regexp.MustCompile(`\b(?:dong|gop|doi|thu|nop|xin)(?: lai)? (?:tien|quy|no)\b|\b(?:gop|dong|tra|chuyen|ck|chiu|bo ra)(?: (?:lai|them|truoc|het))? (?:bao nhieu|bn)\b`)},
	// A QR code or an account to pay with.
	{"qr", regexp.MustCompile(`\bqr` + w(3) + `(?:qqtien|chuyen khoan|tra tien|thanh toan|ngan hang)\b|\b(?:quet|tao|xuat)(?: ma)? (?:qr|vietqr)` + w(3) + `(?:qqtien|tra|chuyen|thanh toan)\b`)},
	// Spent: how much went on something, or who paid.
	{"da_tieu", regexp.MustCompile(`\b(?:an|tieu|xai|mat|ton|het) het (?:bao nhieu|bn)\b|\bhet (?:bao nhieu|bn) tien\b|\btieu(?: het)? (?:bao nhieu|bn)\b`)},
}

// chuTien is the question as the rules read it: folded words, marked
// homographs kept apart, compounds glued, every amount one qqtien.
func chuTien(s string) string {
	raw := []rune(strings.ToLower(norm.NFC.String(homoglyph.Replace(norm.NFKC.String(s)))))
	var toks []string
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			toks = append(toks, cur.String())
			cur.Reset()
		}
	}
	for i, r := range raw {
		switch {
		case r == '\'' || r == '’' || r == '`':
			// hasn't -> hasnt
		case unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.Is(unicode.Mn, r) || r == '$':
			cur.WriteRune(r)
		case (r == '.' || r == ',') && i > 0 && i+1 < len(raw) && unicode.IsDigit(raw[i-1]) && unicode.IsDigit(raw[i+1]):
			cur.WriteRune(r)
		default:
			flush()
		}
	}
	flush()
	for i, t := range toks {
		f := promptsafety.Fold(t)
		if dung, ok := dauTien[f]; ok && f != t && !slices.Contains(dung, t) {
			f += "_"
		}
		toks[i] = f
	}
	g := " " + strings.Join(toks, " ") + " "
	// Twice: a replacement consumes the space the next compound starts with.
	g = ghep.Replace(ghep.Replace(g))
	g = soTienRe.ReplaceAllString(g, " qqtien ")
	return strings.Join(strings.Fields(g), " ")
}

// LaTien says whether the question asks Nếp to act on money.
func LaTien(s string) bool {
	_, ok := luatTienKhop(s)
	return ok
}

// luatTienKhop names the first rule that refuses s (tests and the eval).
func luatTienKhop(s string) (string, bool) {
	g := chuTien(s)
	if g == "" || (nghiaTu.MatchString(g) && !strings.Contains(g, "qqtien")) {
		return "", false
	}
	for _, l := range luatTiens {
		if l.re.MatchString(g) {
			return l.ten, true
		}
	}
	return "", false
}

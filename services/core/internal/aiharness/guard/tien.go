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
// Which way to err. A false refusal blocks a legitimate place or budget
// question outright, with no model call and no second opinion, so the law is
// tuned for precision first. A miss is not a hole: the question then meets
//
//  1. the system instruction, which forbids creating, splitting, settling or
//     reminding about money and forbids claiming any action (prompts/nep_agent.txt);
//  2. the engine itself, which has no tool that can move, record or split
//     money: the model can only write text (aiharness, ADR-0037 §4);
//  3. the output guard, which blocks an answer claiming it moved, recorded or
//     sent anything, and any phone, account number or email (output.go);
//  4. from slice 9, the Understand step's own classifier (design 01 §3.3,
//     `tien: none|split_draft|money_action`), a second, independent reader.
//
// This law is layer 0 of that stack: it saves the model call and answers
// with a fixed sentence. The flag MOBILE_AI_ENGINE_NEP may not be set to go
// until the law reaches recall ≥ 0.95 with false positives ≤ 0.02 on a
// sealed corpus its author never read (ADR-0037 proposal §4).
//
// People ask for money in any word order («chuyển 200k cho Nam», «200K cho
// Lan, chuyển giúp mình»), in any amount form (200k, 1tr2, 1.200.000đ, «hai
// trăm nghìn», «3 trăm 5», «nửa triệu», «2 lít», «1 củ», a bare «250» after
// «ck»), with or without tone marks, and in English. So the law reads the
// question in four steps:
//
//  1. words: Gap (NFKC, look-alikes, folded) split into words. A word typed
//     WITH tone marks that folds onto a money word but is not that word
//     («nó», «trà», «đợi», «bạn», «muốn», «đồng», «chỗ», «đứa») keeps a mark
//     of its own, so it cannot be read as «nợ», «trả», «đòi», «bắn», «mượn»,
//     «đóng», «cho», «đưa»; unmarked text stays ambiguous and is read as
//     money. Fixed compounds whose first word is a money verb («kiểm tra»,
//     «trả lời», «chia sẻ», «góp ý», «trà sữa», «đồng giá», «chuyển sang»)
//     are glued into one word;
//  2. amounts: every amount becomes one word, qqtien; a bare number becomes
//     qqso only where an amount can stand (right before «cho», «nha», «đi»,
//     «qua», the end...), never before a unit of time, people or places;
//  3. place readings: a money verb whose object is a place or a plan («gửi
//     mình quán dưới 100k», «chuyển kèo sang quán 200k», «send me places
//     under 200k») is not moving money, and becomes qqviec; a budget cap
//     («dưới 100k», «under 150k») becomes qqgia; a place's own payment
//     methods («quán nào nhận ck») and entrance fee («đóng tiền vào cổng»)
//     become qqthuoctinh and qqphi. None of the rules below reads these;
//  4. rules, grouped by what the person asks for: move, split, record, debt,
//     collect, account or QR, spent. Any one match refuses.
//
// A question about what a word means («ck là gì», «stk viết tắt của từ gì»,
// «what does AA mean») is not a request: the part that asks is dropped and
// only what follows it, if anything, is read.

// dauTien lists, for each folded word a rule reads as money, the spellings
// with marks that mean it. Any other marked spelling of the same letters is a
// homograph and is read as the folded word with «_» appended, which no rule
// reads as money. An empty list means only the unmarked spelling is money
// («vay», «thu», «cho»).
var dauTien = map[string][]string{
	"no":     {"nợ"},
	"tra":    {"trả"},
	"dong":   {"đóng"},
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
	// «chỗ», «chợ» are places; «cho» is to give, or «for».
	"cho": {},
	// «đưa» hands over; «đứa» counts people, «đua» races.
	"dua":  {"đưa"},
	"tram": {"trăm"},
	"hue":  {"huề"},
	"du":   {"dư"},
	"quyt": {"quỵt"},
	"nop":  {"nộp"},
	"lay":  {"lấy"},
	"bung": {"bùng"},
	"bu":   {"bù"},
}

// ghep glues compounds whose first word is also a money word, so no rule can
// read that word in them; most matter only for text typed without marks,
// where «chuyen di» is a trip and «dong nguoi» a crowd. The order matters
// where one compound starts another («chuyen sang cho» stays a move).
var ghep = strings.NewReplacer(
	" kiem tra ", " kiemtra ",
	" tra loi ", " traloi ",
	" tra cuu ", " tracuu ",
	" tra phong ", " traphong ",
	" tra xe ", " traxe ",
	" tra gia ", " tragia ",
	" chia se ", " chiase ",
	" chia tay ", " chiatay ",
	" hoan thanh ", " hoanthanh ", " hoan tat ", " hoantat ",
	" gop y ", " gopy ",
	" gop mat ", " gopmat ",
	" so tay ", " sotay ",
	" gui xe ", " guixe ",
	" gui do ", " guido ",
	" dong cua ", " dongcua ",
	" dong gia ", " donggia ", " dong_ gia ", " donggia ",
	" no mon ", " nomon ",
	" an no ", " anno ", " no ne ", " none ", " no bung ", " nobung ",
	" vk ck ", " vkck ",
	" thu am ", " thuam ", " thu hut ", " thuhut ", " mua thu ", " muathu ",
	" tien duong ", " tienduong ", " tien loi ", " tienloi ", " tien ich ", " tienich ",
	" thuan tien ", " thuantien ", " tien nghi ", " tiennghi ", " tien sanh ", " tiensanh ",
	" tien giang ", " tiengiang ",
	" chuyen sang cho ", " chuyen cho ", " chuyen sang ", " chuyensang ",
	// «chuyến …» (a trip, a ride) typed without marks.
	" chuyen di ", " chuyendi ", " chuyen nay ", " chuyennay ", " chuyen do ", " chuyendo ",
	" chuyen xe ", " chuyenxe ", " chuyen bay ", " chuyenbay ", " chuyen tau ", " chuyentau ",
	" chuyen choi ", " chuyenchoi ", " chuyen phuot ", " chuyenphuot ", " chuyen du lich ", " chuyendulich ",
	// «đông/đồng …» typed without marks.
	" dong nguoi ", " dongnguoi ", " dong khach ", " dongkhach ", " dong vui ", " dongvui ",
	" dong duc ", " dongduc ", " dong y ", " dongy ", " dong ho ", " dongho ", " dong phuc ", " dongphuc ",
	" dong bon ", " dongbon ", " dong nai ", " dongnai ", " dong thap ", " dongthap ", " dong xuan ", " dongxuan ",
	" dong da ", " dongda ", " dong hanh ", " donghanh ", " dong nghiep ", " dongnghiep ",
	// «váy», «ứng dụng», «bắn cung» and the like.
	" mua vay ", " muavay ", " dam vay ", " damvay ", " chan vay ", " chanvay ",
	" ung dung ", " ungdung ", " ung vien ", " ungvien ", " ung xu ", " ungxu ",
	" ban cung ", " bancung ", " ban sung ", " bansung ", " ban phao hoa ", " banphaohoa ", " ban bi ", " banbi ",
	" ting ting ", " tingting ",
	" tra gop ", " tragop ",
	" boi thuong ", " boithuong ",
	// «2 trăm mét» is a distance, «2 trăm người» a crowd.
	" tram met ", " trammet ", " tram m ", " trammet ", " tram km ", " tramkm ",
	" tram nguoi ", " tramnguoi ", " tram cho_ ", " tramcho ",
	// «trà …» typed without marks: the common drinks.
	" tra sua ", " trasua ", " tra da ", " trada ", " tra chanh ", " trachanh ",
	" tra dao ", " tradao ", " tra xanh ", " traxanh ", " tra tac ", " tratac ",
	" tra gung ", " tragung ", " tra sen ", " trasen ", " tra hoa ", " trahoa ",
	" tra nong ", " tranong ", " tra thai ", " trathai ", " tra atiso ", " traatiso ",
	" tra vai ", " travai ", " tra olong ", " traolong ", " tra oolong ", " traoolong ",
	" tra matcha ", " tramatcha ", " tra chieu ", " trachieu ", " tra trai cay ", " tratraicay ",
)

// soTienRe is an amount, on the folded words: digits with a unit (200k, 1,5
// triệu, 2 lít, 5 xị, 1 củ, 1 triệu 2, 20 bucks, 1.2 million), the tr/m
// shorthand (1tr2, 1m2 -- a bare 500m is a distance), grouped thousands
// (1.200.000, 150 000 000 đồng), a long run with đ, dollars, hundreds («3
// trăm 5», «hai trăm») and amounts in words («hai trăm nghìn», «năm chục»,
// «nửa triệu», «vài trăm nghìn»). A currency word after it is part of it.
var soTienRe = regexp.MustCompile(`\$ ?\d+(?:[.,]\d+)*` +
	`|\b\d+(?:[.,]\d+)* ?(?:k|nghin|ngan|trieu|tr|cu|lit|xi|chuc|bucks|dollars?|usd|vnd|million|millions|mil)(?: (?:ruoi|\d{1,3}))?(?: (?:dong_?|d|vnd))?\b` +
	`|\b\d+(?:tr|m)\d+\b` +
	`|\b\d{1,3}(?:[.,]\d{3})+(?: ?(?:d|dong_?|vnd))?\b` +
	`|\b\d{1,3}(?: \d{3})+ ?(?:d|dong_?|vnd)\b` +
	`|\b\d{4,} ?(?:d|dong_?)\b` +
	`|\b\d+ tram(?: (?:\d{1,2}|ruoi|nghin|ngan|k))?\b` +
	`|\b(?:mot|hai|ba|bon|tu|nam|sau|bay|tam|chin|muoi|nua|vai|may)(?: (?:tram|chuc|muoi))? (?:nghin|ngan|trieu|chuc|cu|lit|xi)(?: (?:dong_?|d))?\b` +
	`|\b(?:mot|hai|ba|bon|nam|sau|bay|tam|chin|vai|may) tram(?: (?:mot|hai|ba|bon|tu|nam|sau|bay|tam|chin|ruoi|\d))?\b`)

// A bare number is an amount only where an amount can stand: before one of
// these words, or last. «ck 250 cho Nam», «mượn anh 200 nha», «Bắn Khang 500
// đi»; never «7 giờ», «20 người», «100 chỗ».
var sauSo = map[string]bool{
	"cho": true, "qua": true, "sang": true, "nha": true, "nhe": true, "nhen": true, "nghen": true,
	"di": true, "gium": true, "giup": true, "dum": true, "ho": true, "lien": true, "luon": true,
	"roi": true, "duoc": true, "dc": true, "nhanh": true, "truoc": true, "vao": true, "nua": true,
	"lai": true, "ne": true, "nhi": true, "voi": true, "thoi": true, "chua": true, "ko": true,
	"khong": true, "k": true, "mua": true,
}

// ...and never right after a word that makes it an address, a time or a rank.
var truocKhongSo = map[string]bool{
	"luc": true, "so": true, "thu_": true, "thu": true, "quan": true, "q": true, "tang": true,
	"phong": true, "ngay": true, "cong": true, "top": true, "hang": true, "lop": true,
	"duong": true, "hem": true, "ngo": true, "kiet": true, "gio": true, "phut": true, "tuoi": true,
}

// Place readings, run on the folded words after amounts are marked.
var (
	// A move verb whose object, within three words, is a place, a plan or a
	// list of them. The words in between must not name money.
	viecRe = regexp.MustCompile(`\b(gui|send|chuyen|ban|tra|dua|doi|transfer|ck|bank|ung|tingting)((?: \S+){0,3}) (quan|cho_|tiem|nha hang|dia diem|diem|keo|lich|danh sach|link|goi y|places?|spots?|bars?|list|plans?|restaurants?|cafes?|vi tri|dia chi|menu|ban do|options?|recommendations?|ideas?|suggestions?|homestay|khach san|resort|mon|ket qua)\b`)
	// «đưa mẹ đi ăn», «đưa tụi nhỏ tới quán»: taking someone somewhere.
	duaDiRe  = regexp.MustCompile(`\bdua((?: \S+){0,2}) (di|toi|den|ve|sang)\b`)
	tienGiua = regexp.MustCompile(`\b(?:tien|qqtien|qqso|khoan|stk|no|coc|bill|quy)\b`)
	// A budget cap: «dưới 100k», «under 150k», «100 nghìn đổ lại».
	giaRe = regexp.MustCompile(`\b(?:duoi|under|below|toi da|max|khong qua|chua toi|chua den|nho hon|less than|within|re hon|cheaper than) (?:qqtien|qqso)\b|\bqqtien (?:do lai|tro xuong|tro lai)\b`)
	// How a place takes payment («quán nào nhận ck», «có trả bằng thẻ
	// không»), read only when the question names a place.
	thuocTinhRe    = regexp.MustCompile(`\b(?:co|nhan|cho|chap nhan|ho tro|lay|dung|xai)(?: (?:thanh toan|tra))?(?: bang)? (?:chuyen khoan|ck|the|momo|qr|vietqr|zalopay|vnpay|shopeepay|tien mat|visa|master|mastercard)\b|\b(?:thanh toan|tra) bang (?:chuyen khoan|ck|the|momo|qr|tien mat|visa|master)\b`)
	thuocTinhNoiRe = regexp.MustCompile(`\b(?:quan|tiem|nha hang|cho_|khach san|homestay|ben|bai)(?: \S+){0,2} (?:thu|tinh|lay) (?:tien|phi|qqtien)\b|\bpay (?:by |with |in )?(?:cash|card|credit card|momo|qr)\b|\baccepts? (?:cash|cards?|momo|qr)\b`)
	coNoiRe        = regexp.MustCompile(`\b(?:quan|tiem|cho_|nha hang|shop|cua hang|khach san|homestay|cafe|resort|places?|restaurants?|bars?|ben|bai|there)\b`)
	// A place's own fee: «đóng tiền vào cổng», «vé … phải trả bao nhiêu».
	phiRe = regexp.MustCompile(`\b(?:dong|tra|mat|ton|thu|nop) (?:tien|phi) (?:vao cong|vao cua|cong|cua|guixe)\b|\b(?:ve|phi|gia)(?: \S+){0,3} (?:phai )?(?:tra|mat|ton|dong) (?:bao nhieu|bn)\b`)
)

// chuyenDiRe: «chuyển đi cho Nam», «200k chuyển đi» -- «đi» urges the
// transfer on; any other «chuyen di» typed without marks is a trip.
var chuyenDiRe = regexp.MustCompile(` chuyen di( (?:cho|nha|nhe|luon|lien|gium|giup|dum|ngay)\b| $)`)

// thuRe: «thu 7», «thu hai» typed without marks is a weekday.
var thuRe = regexp.MustCompile(` thu (?:[2-8]|hai|ba|tu|sau|bay) `)

// veRe: «tiền vé (vào cổng) mỗi người bao nhiêu» opening a question asks the
// ticket's price.
var veRe = regexp.MustCompile(`^(?:tien|gia) ve(?: vao (?:cong|cua))?\b`)

// uocRe is an estimate: «mỗi người trả khoảng bao nhiêu ở quán đó» asks
// what the place costs, not what anyone owes.
var uocRe = regexp.MustCompile(`\b(?:khoang|tam|co|chung|trung binh|around|about|roughly) (?:bao nhieu|bn)\b`)

// nghiaTu is a question about what one word or short phrase means; the part
// that asks is dropped, and only what follows it is read.
var nghiaTu = regexp.MustCompile(`^(?:chu |tu |cum tu |ky hieu )?\S+ (?:trong(?: \S+){1,3} )?(?:la gi|nghia la gi|co nghia la gi|nghia gi|nghia la sao|la viet tat cua (?:tu |chu )?gi|viet tat cua (?:tu |chu )?gi|la tu gi)\b` +
	`|^(?:what does \S+(?: \S+)? (?:mean|stand for)|what is (?:a |an |the )?(\S+)(?: short for)?|whats (?:a |an )?(\S+)|what is the meaning of \S+(?: \S+)?)\b`)

// «what is owed» asks about a debt, not a word.
var khongPhaiTu = map[string]bool{"owed": true, "owing": true, "due": true, "outstanding": true, "unpaid": true, "paid": true, "left": true, "remaining": true}

// w is a window of 0..n words between two anchors, spaces included.
func w(n int) string { return `(?: \S+){0,` + strconv.Itoa(n) + `} ` }

type luatTien struct {
	ten string
	re  *regexp.Regexp
}

const (
	// An amount: with a unit, or a bare number where an amount can stand.
	so = `(?:qqtien|qqso)`
	// A verb that moves money and may take a bare number.
	dongTuChuyen = `(?:chuyen|ck|gui|tra|dua|bank|tingting|boithuong|tragop|hoan|nap|nop|dong|gop|ung|bu|momo|venmo)`
	// A verb that moves or splits money but needs an amount with its unit.
	dongTuChuyenDon = `(?:send|transfer|wire|pay|paid|request|thanh toan|split|chia|thu|gom|quyt|xu|bung|collect|doi)`
	// A verb that, after an amount, asks for it to be moved or split.
	dongTuSau = `(?:chuyen (?:cho|giup|gium|ho|lien|luon|di|nhe|nha|lai|ngay|dum|voi|qua)|chuyen$|chuyendi$|ck|bank|send|transfer|split|tingting|chia (?:deu|doi_?|ra|lai|cho|sao|qqso|\d+|tien|bill|moi|phan|giup|gium|ho|nhu|theo)|chia$)`
	// Words that may sit between a verb and what it acts on.
	dem = `(?: (?:lai|vao|giup|gium|ho|minh|cho|moi|nhanh|luon|ngay|toi|them|truoc))*`
	// Words that may sit around the one free word (a name) between a verb and
	// «tiền»: «gửi cho Nam tiền», «thu hộ mình tiền», «trả Bảo phần tiền».
	dem2 = `(?: (?:lai|truoc|ho|giup|gium|dum|het|not|minh|toi|tao|to|tui|t|m|em|anh|chi_|chi|ban_|ban|cho|ca|nhom|luon|them|phan|nua|cua))*`
	// Words between «vay/mượn» and the lender: «mượn tạm», «vay nóng».
	vayDem = `(?: (?:tam|them|nong|do|giup|gium|ho|chung|lai|cua))*`
	// The lender: a pronoun or a kinship word, maybe followed by a name.
	vayAi = `(?:minh|toi|tao|to|tui|t|em|anh|chi_|chi|ban_|ban|co|chu|di|bac|ong|ba|cau|me|bo|thang|con|nguoi ta|may|m)`
	// A spend that already happened, or the group's own.
	daQua = `\b(?:hom qua|toi qua|trua qua|sang qua|dem qua|hom truoc|bua truoc|tuan truoc|thang truoc|vua roi|vua qua|roi|ca nhom|nhom minh|tui minh|bon minh|chung minh|tuan nay|thang nay)\b`
	// Each person, each one.
	moiNguoi = `(?:moi|1|mot) (?:nguoi|ng|dua|dua_|ban|ban_|thang|dau nguoi)`
)

var luatTiens = []luatTien{
	// Words that only ever mean money.
	{"tu_rieng", regexp.MustCompile(`\b(?:chuyen (?:tien|khoan)|ckhoan|momo|zalopay|vnpay|shopeepay|viettelpay|vietqr|vi dien tu|so tai khoan|tai khoan ngan hang|tat toan|quyet toan|venmo|paypal|zelle|cashapp|refund(?:ed|s)?|reimburse[sd]?|reimbursement|expenses?|bank account|settle up|owe[sd]?|owing|chip(?:ped|ping)? in|tien coc|dat coc|tam ung|stk|(?:chuyensang|vao|qua|sang) (?:tai khoan|tk)|so du (?:quy|tai khoan|con|bao nhieu|bn|hien tai|cua)|thu_? quy|quy (?:nhom|chung|lop|keo|chuyendi|chuyen_ di)|(?:tien|dong|gop|nop|rut|giu|thu) quy)\b`)},
	// Move: a money verb and «tiền», with at most one free word (a name)
	// between them besides the small words of dem2.
	{"dong_tu_tien", regexp.MustCompile(`\b(?:gui|chuyen|tra|dong|gop|gom|doi|thu|hoan|ung|nap|muon|vay|xin|nop|ck|thanh toan|bank|chia|split|quyt|om|dua|bung|xu|bu|tingting|boithuong|tragop)` + dem2 + `(?: \S+)?` + dem2 + ` tien\b`)},
	// Move: a money verb and an amount, within three words, either order.
	{"dong_tu_so", regexp.MustCompile(`\b` + dongTuChuyen + w(3) + so + `\b|\b(?:chuyen|nap|gui|ck|bank) (?:vao|sang|qua) (?:vi|tai khoan|tk)` + w(3) + so + `\b|\b` + dongTuChuyenDon + w(3) + `qqtien\b|\b` + dongTuChuyen + ` cho(?: \S+){1,4} ` + so + `\b`)},
	{"so_dong_tu", regexp.MustCompile(`\bqqtien` + w(3) + dongTuSau + `\b`)},
	{"ban_ck", regexp.MustCompile(`\bban (?:(?:qua|sang) )?(?:cho )?(?:(?:thang|con|anh|chi_|chi|em|ban_|co|chu|di|be) )?\S+ qqtien|\bban qqtien(?: \S+)? (?:cho|qua|sang)\b|\bban(?: \S+){0,2} qqso\b|\b(?:chua|da|vua|roi) ck\b|\bck (?:lai|ho|giup|gium|dum|tien|khoan|truoc|luon|ngay|lien)\b|\bqqtien` + w(3) + `ck\b|\bbank` + w(2) + `qqtien`)},
	{"thanh_toan", regexp.MustCompile(`\bthanh toan (?:giup|ho|gium|cho|tien|khoan|no|bill|hoa don|lai|truoc|not)\b|\bhoan (?:lai )?(?:tien|coc|qqtien)\b|\bhoan lai` + w(2) + `qqtien|\bnap` + w(2) + `(?:qqtien|tien|vao vi)\b|\btra (?:ho|gium|dum|thay)\b|\btra sau\b`)},
	// What is left to pay, or someone's share of it.
	{"con_lai", regexp.MustCompile(`\b(?:chuyen|tra|gui|ck|dua|ban|thanh toan)(?: \S+){0,2} (?:so con lai|so tien|phan con lai|phan tien|phan cua|phan minh|not phan|not so)\b|\b(?:lay|xin|doi) lai (?:qqtien|qqso|tien)\b|\blay (?:qqtien|tien) (?:tu|cua)\b`)},
	{"tieng_anh", regexp.MustCompile(`\bpa(?:y|id)(?: \S+)? back\b|\b(?:hasnt|havent|hadnt|didnt|not|already|never|yet) paid\b|\bwho(?: \S+){0,2} paid\b|\bsettle (?:up|the (?:bill|debt|tab))\b|\bremind(?: \S+){1,3} to (?:pay|chip in|send|transfer|settle)\b|\b(?:transfer|send|wire|pay) (?:the )?(?:money|cash|funds)\b|\b(?:pay|send|transfer) (?:my|your|his|her|our|their|the) (?:share|part|portion|half|cut)\b|\bpay (?:the |my |our )?(?:deposit|rent|debt|loan)\b|\bhow much (?:does|do|should|did|will|would) (?:each|every|everyone|everybody)(?: (?:person|one|of us|guy|people))? (?:owe|pay|put in|chip in|give|send|contribute|transfer)\b|\bsplit` + w(2) + `(?:bill|check|costs?|tab|ways|evenly|equally|payment|fare|fee|rent|total|qqtien)\b|\bcollect(?: \S+){0,2} (?:qqtien|money|cash|payments?|dues)\b|\bcollect \S+ from\b`)},
	// Split: «chia» a sum or a bill, or what each person pays.
	{"chia", regexp.MustCompile(`\bchia` + w(2) + `(?:tien|bill|hoa don|khoan|chi phi)\b` +
		`|\b` + moiNguoi + `(?: (?:phai|can|nen))? (?:tra|gop|dong|chiu|chuyen|no|chia|ck|bu|gui|dua|nop|bo ra|chung|ung)(?: (?:lai|them|truoc|het|cho))?(?: \S+)? (?:bao nhieu|bn|may tien|nhieu)\b` +
		`|\b(?:chia|split|bill|hoa don|aa|het qqtien|tong (?:cong )?(?:qqtien|bill|hoa don|tien|chi))\b.*\b` + moiNguoi + `(?: \S+)? (?:bao nhieu|bn)\b` +
		`|\b(?:tong|het) (?:tien|qqtien|bill|hoa don)\b.*\bchia\b` +
		`|^tien \S+(?: \S+){0,3} ` + moiNguoi + ` (?:bao nhieu|bn)\b` +
		`|\btinh tien (?:cho|giup|gium|ho|moi|ca|nhom)\b|\bbill qqtien|\bbill (?:split|chia)\b|\b(?:chia|split|tra|pay|tinh|thanh toan|trong|tong)(?: \S+){0,2} (?:bill|hoa don)\b|\b(?:bill|hoa don)(?: \S+){0,3} (?:chia|split|tinh)\b` +
		`|^aa\b|\baa (?:nhe|nha|di|nhen|ne|luon|cho|voi)\b|\b(?:di|tinh|chia|lam|share) aa\b` +
		`|\bai (?:phai |se |da |chua |con |dang |van )?(?:tra|bu|chuyen|gui|ck|dua)(?: (?:cho|lai))? ai\b` +
		`|\btinh(?: \S+){0,2} phan (?:moi|cua|tung)\b`)},
	// Record: write, change or keep track of a spend or a debt.
	{"ghi", regexp.MustCompile(`\b(?:ghi|luu|them|nhap|sua|xoa|tao|cong|theo doi)` + dem + ` (?:khoan|chi phi|chi tieu|no|hoa don|tong chi|danh sach chi)\b|\b(?:ghi|luu|nhap)` + dem + ` tien\b|\b(?:ghi|luu|nhap|them)` + w(2) + `vao so\b|\b(?:ghi|luu|nhap) (?:vao )?so` + w(3) + `qqtien\b|\btong ket (?:chi tieu|chi phi|tien|khoan)\b|\bghi(?: lai)?(?: \S+){0,4} (?:chi|tieu|tra|mat|het) qqtien\b|\bkhoan(?: chi)?` + w(2) + `qqtien`)},
	// Debt: owing, lending, borrowing, advancing, dodging.
	{"no", regexp.MustCompile(`\b(?:ghi|tra|xoa|doi|nhac|gach|chia|tinh|don|thu|bi|khat|quyt|om|vo|mac|xu) no\b|\bno` + w(3) + `(?:qqtien|bao nhieu|bn|qquoc)\b|\bno (?:\S+ )?tien\b|\bai (?:dang |con |van |chua |da )?no\b|\b(?:no|thieu) ai\b|\bthieu` + w(2) + `(?:qqtien|tien)\b|\bcho(?: \S+){1,2} vay\b|\b(?:vay|muon)` + vayDem + `(?: ` + vayAi + `)?(?: \S+)? ` + so + `\b|\bung(?: (?:truoc|them|ho|giup))? (?:qqtien|tien)\b` +
		`|\b(?:quyt|xu|bung)` + w(2) + so + `\b|\bnhac(?: (?:gium|giup|dum))? \S+(?: (?:vu|khoan|cai))? qqtien\b|\b(?:dang |da |chua |con )?hue (?:chua|roi|nhau|het)\b|\bdang hue\b` +
		`|\bai (?:phai |se |da |chua |con |dang |van )?tra\b|\bai (?:chua|da|con) (?:chuyen|ck|gop|nop|dong)\b|\btra (?:du|thieu)\b` +
		`|\b(?:da|chua|vua) (?:tra|ck|chuyen) (?:minh|toi|tao|to|tui|em|t|anh|chi_|ban_|lai)\b|\b(?:da|chua|vua) (?:tra|ck|chuyen|gop|dong) (?:du_?|du tien|thieu|het tien|het no)\b|\b(?:tra|ck|chuyen) (?:minh|toi|tao|to|tui|em|t|anh|chi_|ban_) (?:chua|roi|het|du)\b` +
		// Paid first, to be paid back later.
		`|\b(?:bao|ung|tra|chi) truoc\b.*\b(?:gui|tra|chuyen|ck|dua) lai\b`)},
	// Collect: chase, gather or pay in.
	{"thu", regexp.MustCompile(`\b(?:dong|gop|doi|thu|nop|xin)(?: lai)? (?:tien|quy|no)\b|\b(?:gop|dong|tra|chuyen|ck|chiu|bo ra)(?: (?:lai|them|truoc|het))? (?:bao nhieu|bn)\b|\b(?:gop|dong|thu|nop) ` + moiNguoi + ` (?:qqtien|qqso|\d+)\b|\bdu (?:bao nhieu|bn) tien\b|\bxin(?: lai)? qqtien\b`)},
	// A QR code or an account to pay with.
	{"qr", regexp.MustCompile(`\bqr` + w(3) + `(?:qqtien|chuyen khoan|tra tien|thanh toan|ngan hang)\b|\b(?:quet|tao|xuat)(?: ma)? (?:qr|vietqr)` + w(3) + `(?:qqtien|tra|chuyen|thanh toan)\b`)},
	// Spent: how much went on something. «Một bữa ở Đà Lạt hết bao nhiêu
	// tiền» asks a price; only a spend that already happened, or the
	// group's own, is a ledger question.
	{"da_tieu", regexp.MustCompile(`\btieu het (?:bao nhieu|bn|qquoc)\b|` + daQua + `.*\b(?:(?:an|tieu|xai|mat|ton) )?het (?:bao nhieu|bn|qquoc)\b|\b(?:(?:an|tieu|xai|mat|ton) )?het (?:bao nhieu|bn|qquoc)\b.*` + daQua + `|` + daQua + `.*\btieu (?:bao nhieu|bn|qquoc)\b|\btieu (?:bao nhieu|bn|qquoc)\b.*` + daQua)},
}

// chuTien is the question as the rules read it: folded words, marked
// homographs kept apart, compounds glued, every amount one qqtien or qqso,
// and the place readings taken out.
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
	goc := slices.Clone(toks)
	for i, t := range toks {
		f := promptsafety.Fold(t)
		if dung, ok := dauTien[f]; ok && f != t && !slices.Contains(dung, t) {
			f += "_"
		}
		toks[i] = f
	}
	// Typed without marks, «no» right before a budget word is «no» (full),
	// as in «ăn gì cho no tầm 60k»; «nợ tầm 300k» keeps its mark.
	for i := 0; i+1 < len(toks); i++ {
		if goc[i] == "no" && (toks[i+1] == "tam" || toks[i+1] == "khoang" || toks[i+1] == "duoi" || toks[i+1] == "co") {
			toks[i] = "no_"
		}
	}
	g := " " + strings.Join(toks, " ") + " "
	g = chuyenDiRe.ReplaceAllString(g, " chuyen$1 ")
	g = " " + strings.Join(strings.Fields(g), " ") + " "
	// Twice: a replacement consumes the space the next compound starts with.
	g = ghep.Replace(ghep.Replace(g))
	g = soTienRe.ReplaceAllString(g, " qqtien ")
	// After amounts, so «thu 2 triệu» stays an amount collected.
	g = thuRe.ReplaceAllString(g, " thungay ")
	g = soTran(strings.Fields(g))
	g = giaRe.ReplaceAllString(g, "qqgia")
	g = duaDiRe.ReplaceAllStringFunc(g, func(m string) string {
		sub := duaDiRe.FindStringSubmatch(m)
		if tienGiua.MatchString(sub[1]) {
			return m
		}
		return "qqviec" + sub[1] + " " + sub[2]
	})
	g = viecRe.ReplaceAllStringFunc(g, func(m string) string {
		sub := viecRe.FindStringSubmatch(m)
		if tienGiua.MatchString(sub[2]) {
			return m
		}
		return "qqviec" + sub[2] + " " + sub[3]
	})
	if coNoiRe.MatchString(g) {
		g = thuocTinhRe.ReplaceAllString(g, "qqthuoctinh")
		g = thuocTinhNoiRe.ReplaceAllString(g, "qqthuoctinh")
	}
	g = phiRe.ReplaceAllString(g, "qqphi")
	g = veRe.ReplaceAllString(g, "qqgia")
	g = uocRe.ReplaceAllString(g, "qquoc")
	return strings.Join(strings.Fields(g), " ")
}

// soTran marks a bare number as qqso where an amount can stand.
func soTran(toks []string) string {
	for i, t := range toks {
		if t == "" || strings.IndexFunc(t, func(r rune) bool { return !unicode.IsDigit(r) && r != '.' && r != ',' }) >= 0 {
			continue
		}
		// One digit is a count or a rank («chặng 1», «5 đứa»), never an
		// amount typed bare.
		if !unicode.IsDigit(rune(t[0])) || len(t) < 2 {
			continue
		}
		if i > 0 && truocKhongSo[toks[i-1]] {
			continue
		}
		if i+1 == len(toks) || sauSo[toks[i+1]] {
			toks[i] = "qqso"
		}
	}
	return strings.Join(toks, " ")
}

// docTien is what the rules read: chuTien, with a question about a word's
// meaning taken off the front. "" means there is nothing left to read.
func docTien(s string) string {
	g := chuTien(s)
	if g == "" || strings.Contains(g, "qqtien") || strings.Contains(g, "qqso") {
		return g
	}
	if m := nghiaTu.FindStringSubmatchIndex(g); m != nil {
		for _, k := range []int{2, 4} {
			if m[k] >= 0 && khongPhaiTu[g[m[k]:m[k+1]]] {
				return g
			}
		}
		return strings.TrimSpace(g[m[1]:])
	}
	return g
}

// LaTien says whether the question asks Nếp to act on money.
func LaTien(s string) bool {
	_, ok := luatTienKhop(s)
	return ok
}

// luatTienKhop names the first rule that refuses s (tests and the eval).
func luatTienKhop(s string) (string, bool) {
	g := docTien(s)
	if g == "" {
		return "", false
	}
	for _, l := range luatTiens {
		if l.re.MatchString(g) {
			return l.ten, true
		}
	}
	return "", false
}

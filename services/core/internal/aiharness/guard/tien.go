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
// until, on a sealed corpus its author never read, of at least 220 sentences
// per class, the upper 95% bound of this law's false refusals is ≤ 0.02 on
// its own, and the lower 95% bound of the recall of this law together with
// the Understand step's classifier is ≥ 0.95 (ADR-0037 proposal §4, as
// amended 2026-09-25). The law is frozen for recall: a change may only take
// a false refusal away or undo a regression, never add a pattern to catch
// more (review round 4 of slice 6).
//
// People ask for money in any word order («chuyển 200k cho Nam», «200K cho
// Lan, chuyển giúp mình»), in any amount form (200k, 1tr2, 1.200.000đ, «hai
// trăm nghìn», «3 trăm 5», «nửa triệu», «2 lít», «1 củ», «50 cành», a bare
// «250» after «ck»), with or without tone marks, and in English. So the law
// reads the question in four steps:
//
//  1. words: Gap (NFKC, look-alikes, folded) split into words. A word typed
//     WITH tone marks that folds onto a money word but is not that word
//     («nó», «trà», «đợi», «bạn», «muốn», «đồng», «chỗ», «đứa») keeps a mark
//     of its own, so it cannot be read as «nợ», «trả», «đòi», «bắn», «mượn»,
//     «đóng», «cho», «đưa»; unmarked text stays ambiguous and is read as
//     money. A word capitalized on its own mid-sentence is a name. Fixed
//     compounds whose words are also money words («kiểm tra», «trả lời»,
//     «chia sẻ», «góp ý», «trà sữa», «đồng giá», «chuyển sang») are glued
//     into one word, but only where each word is spelled as the compound
//     spells it or typed with no marks, and is not a name: «trả Hoa» is not
//     «trà hoa», «Gửi Đô» is not «gửi đồ», «Trả Gia» is not «trả giá»;
//  2. amounts: every amount becomes one word, qqtien; a bare number becomes
//     qqso only where an amount can stand (right before «cho», «nha», «đi»,
//     «qua», «tiền», «hôm qua», a comma, the end...), never before a unit of
//     time, people or places;
//  3. place readings: a money verb whose object is a place or a plan («gửi
//     mình quán dưới 100k», «chuyển kèo sang quán 200k», «send me places
//     under 200k») is not moving money, and becomes qqviec -- where the
//     place word is spelled as the place («quán», «lịch», «điểm») or typed
//     without marks, and is not a name: «Quân» is not «quán», and «Chuyển
//     Lịch 100k» pays Lịch; a budget cap («dưới 100k», «under 150k»)
//     becomes qqgia; a place's own payment methods («quán nào nhận ck»),
//     deposit policy («có cần đặt cọc không») and entrance fee («đóng tiền
//     vào cổng») become qqthuoctinh and qqphi; paying for oneself («ai nấy
//     tự trả phần mình») becomes qqtutra. None of the rules below reads these;
//  4. rules, grouped by what the person asks for: move, split, record, debt,
//     collect, account or QR, spent. Any one match refuses.
//
// A question about what a word means («ck là gì», «stk viết tắt của từ gì»,
// «chuyển khoản tiếng Anh là gì», «what does AA mean») is not a request: the
// part that asks is dropped and only what follows it, if anything, is read.

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
	// «lương» is a wage; «lượng» an amount of something («tính lượng bia»).
	"luong": {"lương"},
}

// cumTu is a fixed compound whose words are also money words, spelled as it
// is written. It is glued into one word (its folded words run together, or
// thanh) so no rule can read those words in it: «kiểm tra», «trả lời», «gửi
// xe», «trà sữa», «chuyến đi», «đông người», «ứng dụng».
//
// A compound glues only where each of its words is typed either with exactly
// its own marks or with no marks at all, and is not a name (a word
// capitalized on its own mid-sentence). So «trả Hoa» is not «trà hoa», «Gửi
// Đô» is not «gửi đồ», «Chuyển Bảy» is not «chuyến bay», and «Trả Gia»,
// «Bắn Bi», «Trả Phong» pay Gia, Bi and Phong (review round 3 of slice 6,
// B2). A compound that is itself a proper name («Tiền Giang», «Đồng Nai»)
// is written with capitals, so ten lets it glue whatever the case. Each
// word is judged on its own, whatever marks the rest of the question has
// («Có tiệm nào dong gia 49k không», «Bãi gui xe 5k ở đâu»), with one
// exception (khopChu): «cho», «thu», «chi», «vay» typed without marks are
// the money word, never a marked compound word -- so the unmarked spellings
// of the «váy» compounds are listed as their own.
type cumTu struct {
	chu   string
	thanh string
	ten   bool
	tu    []string
}

var ghep = []cumTu{
	{chu: "kiểm tra"}, {chu: "trả lời"}, {chu: "tra cứu"}, {chu: "trả phòng"}, {chu: "trả xe"},
	{chu: "trả giá"}, {chu: "trả góp"}, {chu: "chia sẻ"}, {chu: "chia tay"},
	{chu: "hoàn thành"}, {chu: "hoàn tất"}, {chu: "góp ý"}, {chu: "góp mặt"}, {chu: "sổ tay"},
	{chu: "gửi xe"}, {chu: "gửi đồ"}, {chu: "đóng cửa"}, {chu: "đồng giá"},
	{chu: "nợ môn"}, {chu: "ăn no"}, {chu: "no nê"}, {chu: "no bụng"}, {chu: "vk ck"},
	{chu: "thu âm"}, {chu: "thu hút"}, {chu: "mùa thu"},
	{chu: "tiện đường"}, {chu: "tiện lợi"}, {chu: "tiện ích"}, {chu: "thuận tiện"}, {chu: "tiện nghi"},
	{chu: "tiền sảnh"}, {chu: "tiền giang", ten: true},
	{chu: "chuyển sang"},
	// «chuyến …», a trip or a ride.
	{chu: "chuyến đi"}, {chu: "chuyến này"}, {chu: "chuyến đó"}, {chu: "chuyến xe"}, {chu: "chuyến bay"},
	{chu: "chuyến tàu"}, {chu: "chuyến chơi"}, {chu: "chuyến phượt"}, {chu: "chuyến du lịch"},
	// «đông …», «đồng …», «đống …».
	{chu: "đông người"}, {chu: "đông khách"}, {chu: "đông vui"}, {chu: "đông đúc"}, {chu: "đồng ý"},
	{chu: "đồng hồ"}, {chu: "đồng phục"}, {chu: "đồng bọn"}, {chu: "đồng hành"}, {chu: "đồng nghiệp"},
	{chu: "đồng nai", ten: true}, {chu: "đồng tháp", ten: true}, {chu: "đồng xuân", ten: true}, {chu: "đống đa", ten: true},
	// «váy», «ứng dụng», «bắn cung» and the like. «mua vay» typed without
	// marks is a dress too (849664a glued it): nobody borrows by «mua vay».
	{chu: "mua váy"}, {chu: "đầm váy"}, {chu: "chân váy"},
	{chu: "mua vay"}, {chu: "dam vay"}, {chu: "chan vay"},
	{chu: "ứng dụng"}, {chu: "ứng viên"}, {chu: "ứng xử"},
	{chu: "bắn cung"}, {chu: "bắn súng"}, {chu: "bắn pháo hoa"}, {chu: "bắn bi"},
	{chu: "ting ting"}, {chu: "bồi thường"},
	// «2 trăm mét» is a distance, «2 trăm người» a crowd.
	{chu: "trăm mét"}, {chu: "trăm m", thanh: "trammet"}, {chu: "trăm km"}, {chu: "trăm người"}, {chu: "trăm chỗ"},
	// «trà …», the common drinks.
	{chu: "trà sữa"}, {chu: "trà đá"}, {chu: "trà chanh"}, {chu: "trà đào"}, {chu: "trà xanh"}, {chu: "trà tắc"},
	{chu: "trà gừng"}, {chu: "trà sen"}, {chu: "trà hoa"}, {chu: "trà nóng"}, {chu: "trà thái"}, {chu: "trà atiso"},
	{chu: "trà vải"}, {chu: "trà olong"}, {chu: "trà oolong"}, {chu: "trà matcha"}, {chu: "trà chiều"}, {chu: "trà trái cây"},
}

// ghepTheoDau indexes ghep by its folded first word, longest compound first,
// with each compound's words split and its glued word filled in.
var ghepTheoDau = func() map[string][]cumTu {
	m := map[string][]cumTu{}
	for _, c := range ghep {
		c.tu = strings.Fields(c.chu)
		if c.thanh == "" {
			c.thanh = promptsafety.Fold(strings.Join(c.tu, ""))
		}
		k := promptsafety.Fold(c.tu[0])
		m[k] = append(m[k], c)
	}
	for _, cs := range m {
		slices.SortStableFunc(cs, func(a, b cumTu) int { return len(b.tu) - len(a.tu) })
	}
	return m
}()

// noiChu are the one-word places and plans a move verb can take as its
// object in viecRe, with the spellings that mean them («quận» too: an
// address). Typed with other marks («Quân», «Diễm», «kẹo») or as a name
// («Chuyển Lịch 100k», «Send Quan 250k please»), the word is read as the
// folded word with «_» appended, which no place reading reads.
var noiChu = map[string][]string{
	"quan": {"quán", "quận"}, "tiem": {"tiệm"}, "diem": {"điểm"}, "keo": {"kèo"}, "lich": {"lịch"}, "mon": {"món"},
	"link": nil, "menu": nil, "homestay": nil, "resort": nil, "list": nil, "place": nil, "places": nil,
	"spot": nil, "spots": nil, "bar": nil, "bars": nil, "plan": nil, "plans": nil, "restaurant": nil,
	"restaurants": nil, "cafe": nil, "cafes": nil, "option": nil, "options": nil,
}

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
// these words, before «hôm» naming a past day, before a comma, or last. «ck
// 250 cho Nam», «mượn anh 200 nha», «Bắn Khang 500 đi», «chuyển Thắng 60
// tiền cà phê», «Trả Hòa 120 hôm qua», «đưa Lộc 150, nhớ giùm»; never «7
// giờ», «20 người», «100 chỗ», «10 hôm nữa».
var sauSo = map[string]bool{
	"cho": true, "qua": true, "sang": true, "nha": true, "nhe": true, "nhen": true, "nghen": true,
	"di": true, "gium": true, "giup": true, "dum": true, "ho": true, "lien": true, "luon": true,
	"roi": true, "duoc": true, "dc": true, "nhanh": true, "truoc": true, "vao": true, "nua": true,
	"lai": true, "ne": true, "nhi": true, "voi": true, "thoi": true, "chua": true, "ko": true,
	"khong": true, "k": true, "mua": true, "tien": true,
}

// homSau: «hôm qua», «hôm trước», «hôm thứ ba» -- the day a debt was made.
var homSau = map[string]bool{
	"qua": true, "truoc": true, "bua": true, "kia": true, "do": true, "thu_": true, "thu": true,
	"thungay": true, "chu": true, "cn": true,
}

// ngatSo marks a bare number that a clause break follows («đưa Lộc 150,
// nhớ giùm»). It is a character the word splitter never keeps, so nothing
// but soTran reads it, and soTran takes it off.
const ngatSo = "·"

// ...and never right after a word that makes it an address, a time or a
// rank -- unless that word is a name («Trả Phong 200 nha»).
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
	// không», «nhận ví điện tử») and whether it asks a deposit («có cần đặt
	// cọc không», «không lấy tiền cọc»), read only when the question names a
	// place.
	thuocTinhRe = regexp.MustCompile(`\b(?:co|nhan|cho|chap nhan|ho tro|lay|dung|xai)(?: (?:thanh toan|tra))?(?: (?:bang|qua))? (?:chuyen khoan|ck|the|momo|qr|vietqr|zalopay|vnpay|shopeepay|tien mat|visa|master|mastercard|vi dien tu|e wallet)\b|\b(?:thanh toan|tra) (?:bang|qua) (?:chuyen khoan|ck|the|momo|qr|tien mat|visa|master|vi dien tu)\b` +
		`|\b(?:khong can|co can|can|phai|co phai|yeu cau|bat|bat buoc|doi|lay|thu|nhan|co)(?: phai)?(?: dat)? (?:coc|tien coc)(?: truoc)?\b`)
	thuocTinhNoiRe = regexp.MustCompile(`\b(?:quan|tiem|nha hang|cho_|khach san|homestay|ben|bai)(?: \S+){0,2} (?:thu|tinh|lay) (?:tien|phi|qqtien)\b|\bpay (?:by |with |in )?(?:cash|card|credit card|momo|qr)\b|\baccepts? (?:cash|cards?|momo|qr)\b`)
	coNoiRe        = regexp.MustCompile(`\b(?:quan|tiem|cho_|nha hang|shop|cua hang|khach san|homestay|cafe|resort|places?|restaurants?|bars?|ben|bai|there)\b`)
	// A place's own fee: «đóng tiền vào cổng», «vé … phải trả bao nhiêu».
	phiRe = regexp.MustCompile(`\b(?:dong|tra|mat|ton|thu|nop) (?:tien|phi) (?:vao cong|vao cua|cong|cua|guixe)\b|\b(?:ve|phi|gia)(?: \S+){0,3} (?:phai )?(?:tra|mat|ton|dong) (?:bao nhieu|bn)\b`)
	// Paying for oneself: «ai nấy tự trả phần mình», «mình tự trả tiền
	// mình». qqtu is «tự» as typed; unmarked «tu» after «ai nấy» or «mỗi
	// người» reads the same.
	tuTraRe = regexp.MustCompile(`\b(?:qqtu|(?:ai nay|moi nguoi|tung nguoi) tu) (?:tra|lo|chiu|thanh toan)(?: (?:tien|phan|rieng|minh|nay|cua))*\b`)
)

// diGiuc: after «chuyển đi», these urge the transfer on («chuyển đi cho
// Nam», «200k chuyển đi nha»); any other «chuyen di» typed without marks is
// a trip.
var diGiuc = map[string]bool{"cho": true, "nha": true, "nhe": true, "luon": true, "lien": true, "gium": true, "giup": true, "dum": true, "ngay": true}

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
	// «chuyển khoản tiếng Anh là gì», «đặt cọc trong tiếng Anh nói thế nào»,
	// «chia AA là sao» with nothing after it.
	`|^(?:chu |tu |cum tu )?\S+(?: \S+)? (?:la sao|nghia la sao)$` +
	`|^(?:chu |tu |cum tu )?\S+(?: \S+)? (?:trong |bang )?tieng (?:anh|viet|han|nhat|trung|phap|hoa)(?: goi)? (?:la gi|noi the nao|noi sao|viet the nao|la sao|nghia la gi)\b` +
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
	// What may follow «thu lại», «đòi lại» after buying for the group:
	// money, when, from whom, or the small words that close a request.
	sauThuLai = `(?:tien|qqtien|qqso|sau|giup|gium|dum|ho|nha|nhe|nhen|di|luon|tung|moi|tu|cua|ca|het)`
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
	{"tieng_anh", regexp.MustCompile(`\bpa(?:y|id)(?: \S+)? back\b|\b(?:hasnt|havent|hadnt|didnt|not|already|never|yet) paid\b|\bwho(?: \S+){0,2} paid\b|\bsettle (?:up|the (?:bill|debt|tab))\b|\bremind(?: \S+){1,3} to (?:pay|chip in|send|transfer|settle)\b|\b(?:transfer|send|wire|pay) (?:the )?(?:money|cash|funds)\b|\b(?:pay|send|transfer) (?:my|your|his|her|our|their|the) (?:share|part|portion|half|cut)\b|\bpay (?:the |my |our )?(?:deposit|rent|debt|loan)\b|\bhow much (?:does|do|should|did|will|would) (?:each|every|everyone|everybody)(?: (?:person|one|of us|guy|people))? (?:owe|pay|put in|chip in|give|send|contribute|transfer)\b|\bsplit` + w(2) + `(?:bill|check|costs?|tab|ways|evenly|equally|payment|fare|fee|rent|total|qqtien)\b|\bcollect(?: \S+){0,2} (?:qqtien|money|cash|payments?|dues)\b|\bcollect \S+ from\b` +
		// Lending and borrowing: «lend me 200k», «Can I borrow 500k», «I
		// lent Nam 100k», «spot me 50k»; «borrow a bike for 100k» is not.
		`|\b(?:lend|lends|lending|lent|loan|loans|loaned)(?: \S+){0,2} (?:qqtien|qqso|money|cash)\b|\bborrow(?:s|ed|ing)?(?: \S+)? (?:qqtien|qqso|money|cash)\b|\b(?:front|spot) (?:me|us|him|her|them|you)(?: \S+)? (?:qqtien|qqso|money|cash)\b` +
		// A person's tab, or an amount put on a tab: «Put dinner on Minh's
		// tab», «add 150k to my tab», «Put 300k on the tab». «Can we put
		// drinks on a tab at that bar» asks about the bar (review round 4 of
		// slice 6, R2). qqcua is a word typed with «'s» right before «tab»
		// (chuTien). At most four words between the verb and «on», as before.
		`|\b(?:put|add|charge|stick)\b(?: \S+){0,4} (?:on|to) (?:my|his|her|our|their|your|qqcua) tab\b` +
		`|\b(?:put|add|charge|stick)(?:(?: \S+){0,3} ` + so + `|(?: \S+){0,2} ` + so + ` \S+|(?: \S+)? ` + so + `(?: \S+){2}| ` + so + `(?: \S+){3}) (?:on|to)(?: \S+)? tab\b`)},
	// Split: «chia» a sum or a bill, or what each person pays.
	{"chia", regexp.MustCompile(`\bchia` + w(2) + `(?:tien|bill|hoa don|khoan|chi phi)\b` +
		`|\b` + moiNguoi + `(?: (?:phai|can|nen))? (?:tra|gop|dong|chiu|chuyen|no|chia|ck|bu|gui|dua|nop|bo ra|chung|ung)(?: (?:lai|them|truoc|het|cho))*(?: \S+){0,2} (?:bao nhieu|bn|may tien|nhieu)\b` +
		// Whose share it is: «phần tiền của nó ai chịu», «ai chịu tiền taxi».
		// «ai lo phần …» needs money after «phần»: «ai lo phần mua đồ nướng»
		// divides the work (review round 4 of slice 6, R2).
		`|\b(?:phan (?:tien|cua)|tien(?: \S+)? cua|khoan)(?: \S+){0,3} ai (?:chiu|tra|bu|lo|gop|bao)\b|\bai (?:se |phai )?(?:chiu|lo|bu|gop) (?:phan )?(?:tien|khoan|bill|hoa don)\b|\btien(?: \S+){1,3} ai (?:chiu|bu|lo|bao)\b` +
		`|\b(?:chia|split|bill|hoa don|aa|het qqtien|tong (?:cong )?(?:qqtien|bill|hoa don|tien|chi))\b.*\b` + moiNguoi + `(?: \S+)? (?:bao nhieu|bn)\b` +
		`|\b(?:tong|het) (?:tien|qqtien|bill|hoa don)\b.*\bchia\b` +
		`|^tien \S+(?: \S+){0,3} ` + moiNguoi + ` (?:bao nhieu|bn)\b` +
		`|\btinh tien (?:cho|giup|gium|ho|moi|ca|nhom)\b|\bbill qqtien|\bbill (?:split|chia)\b|\b(?:chia|split|tra|pay|tinh|thanh toan|trong|tong)(?: \S+){0,2} (?:bill|hoa don)\b|\b(?:bill|hoa don)(?: \S+){0,3} (?:chia|split|tinh)\b` +
		`|^aa\b|\baa (?:nhe|nha|di|nhen|ne|luon|cho|voi)\b|\b(?:di|tinh|chia|lam|share) aa\b` +
		`|\bai (?:phai |se |da |chua |con |dang |van )?(?:tra|bu|chuyen|gui|ck|dua)(?: (?:cho|lai))? ai\b` +
		`|\btinh(?: \S+){0,2} phan (?:moi|cua|tung)\b`)},
	// Record: write, change or keep track of a spend or a debt.
	{"ghi", regexp.MustCompile(`\b(?:ghi|luu|them|nhap|sua|xoa|tao|cong|theo doi)` + dem + ` (?:khoan|chi phi|chi tieu|no|hoa don|tong chi|tong (?:so )?tien|danh sach chi)\b|\b(?:ghi|luu|nhap)` + dem + ` tien\b|\b(?:ghi|luu|nhap|them)` + w(2) + `vao so\b|\b(?:ghi|luu|nhap) (?:vao )?so` + w(3) + `qqtien\b|\btong ket (?:chi tieu|chi phi|tien|khoan)\b|\bghi(?: lai)?(?: \S+){0,6} (?:chi|tieu|tra|mat|het) qqtien\b|\bkhoan(?: chi)?` + w(2) + `qqtien`)},
	// Debt: owing, lending, borrowing, advancing, dodging.
	{"no", regexp.MustCompile(`\b(?:ghi|tra|xoa|doi|nhac|gach|chia|tinh|don|thu|bi|khat|quyt|om|vo|mac|xu) no\b|\bno` + w(3) + `(?:qqtien|bao nhieu|bn|qquoc)\b|\bno (?:\S+ )?tien\b|\bai (?:dang |con |van |chua |da )?no\b|\bno ai\b|\bthieu` + w(2) + `(?:qqtien|tien)\b` +
		// «ai thiếu ai (bao nhiêu)», «còn thiếu Phương bao nhiêu»; «còn
		// thiếu ai chưa tới», «thiếu bao nhiêu người» are a head count.
		`|\bai(?: \S+)? thieu ai\b|\bthieu ai (?:bao nhieu|bn|tien|qqtien|qqso)\b|\bthieu (?:\S+ ){1,2}(?:bao nhieu|bn)(?: (?:tien|nua|vay_?|the|nhi|a|ha|nhe|nha|roi|day|do|ne|z|v|het|ca|chua|khong|ko))*$` +
		// Wages paid to someone or of an amount: «trả lương cho bạn làm thêm
		// 1 triệu 5», «ứng lương giúp mình». «Chờ trả lương xong rồi đi nhậu»
		// only dates the outing, and «tính lượng bia» is not «lương» (dauTien;
		// review round 4 of slice 6, R2).
		`|\b(?:tra|chuyen|gui|ung|phat|dua|ck|bank|tinh) luong(?: thang \S+)? (?:cho|giup|gium|ho|dum|truoc|them|qqtien|qqso)\b` +
		// Who holds whose money: «ai đang giữ tiền của ai».
		`|\bai (?:dang |con |van )?giu tien(?: cua| quy| nhom| chung)\b|\btien cua ai\b` +
		// Passing on that one has paid: «nhắn Hạnh là mình chuyển rồi».
		`|\b(?:nhan|bao|noi voi|nhan tin cho|nhan cho)(?: \S+){1,2} (?:la|rang) (?:minh|tao|tui|to|em|anh|chi_|toi|t)(?: da| vua)? (?:chuyen|ck|tra|bank|chuyen khoan|chuyen tien|tra tien|gui tien|dua tien)(?: \S+)? (?:roi|xong)\b` +
		`|\bcho(?: \S+){1,2} vay\b|\b(?:vay|muon)` + vayDem + `(?: ` + vayAi + `)?(?: \S+)? ` + so + `\b|\bung(?: (?:truoc|them|ho|giup))? (?:qqtien|tien)\b` +
		`|\b(?:quyt|xu|bung)` + w(2) + so + `\b|\bnhac(?: (?:gium|giup|dum))? \S+(?: (?:vu|khoan|cai))? qqtien\b|\b(?:dang |da |chua |con )?hue (?:chua|roi|nhau|het)\b|\bdang hue\b` +
		`|\bai (?:phai |se |da |chua |con |dang |van )?tra\b|\bai (?:chua|da|con) (?:chuyen|ck|gop|nop|dong)\b|\btra (?:du|thieu)\b` +
		`|\b(?:da|chua|vua) (?:tra|ck|chuyen) (?:minh|toi|tao|to|tui|em|t|anh|chi_|ban_|lai)\b|\b(?:da|chua|vua) (?:tra|ck|chuyen|gop|dong) (?:du_?|du tien|thieu|het tien|het no)\b|\b(?:tra|ck|chuyen) (?:minh|toi|tao|to|tui|em|t|anh|chi_|ban_) (?:chua|roi|het|du)\b` +
		// Paid first, or bought for the group, to be paid back later: «mình
		// trả trước cho cả nhóm rồi thu lại sau», «mua giùm cả nhóm, thu lại».
		// «Báo trước», «bảo trước» tell ahead and never read as «bao»
		// (chuTien). What is gathered back after buying for the group is
		// money, or nothing named: «đặt bàn giúp cả nhóm, hết chỗ thì xin lại
		// giờ khác» books a table (review round 4 of slice 6, R2).
		`|\b(?:bao|ung|tra|chi)(?: tien)? truoc\b.*\b(?:gui|tra|chuyen|ck|dua|thu|doi|xin|gom) lai\b` +
		`|\b(?:mua|dat|tra|bao|ung|chi)(?: \S+){0,2} (?:gium|ho|giup|dum|truoc)(?: cho)? (?:ca nhom|nhom|moi nguoi|tui minh|ca lop|ca bon|tat ca)\b.*\b(?:(?:thu|doi) lai(?: ` + sauThuLai + `)?$|(?:thu|doi) lai ` + sauThuLai + `\b|(?:xin|gom) lai (?:tien|qqtien|qqso)\b)`)},
	// Collect: chase, gather or pay in.
	{"thu", regexp.MustCompile(`\b(?:dong|gop|doi|thu|nop|xin)(?: lai)? (?:tien|quy|no)\b|\b(?:gop|dong|tra|chuyen|ck|chiu|bo ra)(?: (?:lai|them|truoc|het))? (?:bao nhieu|bn)\b|\b(?:gop|dong|thu|nop) ` + moiNguoi + ` (?:qqtien|qqso|\d+)\b|\bdu (?:bao nhieu|bn) tien\b|\bxin(?: lai)? qqtien\b`)},
	// A QR code or an account to pay with.
	{"qr", regexp.MustCompile(`\bqr` + w(3) + `(?:qqtien|chuyen khoan|tra tien|thanh toan|ngan hang)\b|\b(?:quet|tao|xuat)(?: ma)? (?:qr|vietqr)` + w(3) + `(?:qqtien|tra|chuyen|thanh toan)\b|\bqr(?: \S+){0,2} (?:nhan|chuyen|tra|dong|gop|thu) tien\b`)},
	// Spent: how much went on something. «Một bữa ở Đà Lạt hết bao nhiêu
	// tiền» asks a price; only a spend that already happened («đã chi bao
	// nhiêu», «số tiền nhóm đã tiêu»), or the group's own, is a ledger
	// question.
	{"da_tieu", regexp.MustCompile(`\btieu het (?:bao nhieu|bn|qquoc)\b|` + daQua + `.*\b(?:(?:an|tieu|xai|mat|ton) )?het (?:bao nhieu|bn|qquoc)\b|\b(?:(?:an|tieu|xai|mat|ton) )?het (?:bao nhieu|bn|qquoc)\b.*` + daQua + `|` + daQua + `.*\b(?:tieu|chi|xai)(?: het)? (?:bao nhieu|bn|qquoc)\b|\b(?:tieu|chi|xai)(?: het)? (?:bao nhieu|bn|qquoc)\b.*` + daQua +
		`|\b(?:da|vua) (?:chi|tieu|xai|chi tieu|tieu xai)(?: het)? (?:bao nhieu|bn|qquoc)\b|\b(?:tien|qqtien)(?: \S+){0,3} (?:da|vua) (?:tieu|chi|xai)\b`)},
}

// tu is one word of the question.
type tu struct {
	goc  string // as typed, lower case
	gap  string // folded
	f    string // what the rules read
	hoa  bool   // starts with a capital letter
	dau  bool   // starts a sentence
	ten  bool   // a name: capitalized mid-sentence, on its own
	ngat bool   // a clause break follows it
	cua  bool   // typed with a possessive «'s» («Minh's»)
}

// laRutGon: «it's», «let's», «what's»... end in «'s» without owning anything.
var laRutGon = map[string]bool{"it": true, "let": true, "what": true, "that": true, "there": true, "here": true, "he": true, "she": true, "who": true, "where": true, "how": true}

// tachTu splits the question into words, and reads their capitals: a word
// capitalized on its own in mid-sentence is a name («Chuyển Lịch 100k»,
// «Trả Gia 100k», «Send Quan 250k please»). Two capitalized words side by
// side are a proper noun («Quán Ốc Oanh», «Tiền Giang»), not a person; a
// question typed in capitals or title case says nothing about names.
func tachTu(s string) []tu {
	raw := []rune(norm.NFC.String(homoglyph.Replace(norm.NFKC.String(s))))
	var toks []tu
	var cur []rune
	dauCau, cua := true, false
	flush := func() {
		if len(cur) == 0 {
			return
		}
		goc := strings.ToLower(string(cur))
		toks = append(toks, tu{goc: goc, gap: promptsafety.Fold(goc), hoa: unicode.IsUpper(cur[0]), dau: dauCau, cua: cua})
		dauCau, cua = false, false
		cur = cur[:0]
	}
	for i, r := range raw {
		switch {
		case r == '\'' || r == '’' || r == '`':
			// hasn't -> hasnt; «Minh's» -> minhs, owning what follows.
			if len(cur) > 0 && i+1 < len(raw) && (raw[i+1] == 's' || raw[i+1] == 'S') &&
				(i+2 == len(raw) || !unicode.IsLetter(raw[i+2])) && !laRutGon[strings.ToLower(string(cur))] {
				cua = true
			}
		case unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.Is(unicode.Mn, r) || r == '$':
			cur = append(cur, r)
		case (r == '.' || r == ',') && i > 0 && i+1 < len(raw) && unicode.IsDigit(raw[i-1]) && unicode.IsDigit(raw[i+1]):
			cur = append(cur, r)
		default:
			flush()
			if strings.ContainsRune(",;.!?…\n", r) && len(toks) > 0 {
				toks[len(toks)-1].ngat = true
			}
			if strings.ContainsRune(".!?…\n", r) {
				dauCau = true
			}
		}
	}
	flush()
	// A comma between two numbers is a list or a range («30, 40 người»),
	// not the end of an amount.
	for i := 0; i+1 < len(toks); i++ {
		if toks[i].ngat && toks[i+1].goc != "" && unicode.IsDigit([]rune(toks[i+1].goc)[0]) {
			toks[i].ngat = false
		}
	}
	laChu := func(t tu) bool { return t.goc != "" && unicode.IsLetter([]rune(t.goc)[0]) }
	giua := func(i int) bool { return i >= 0 && i < len(toks) && !toks[i].dau && toks[i].hoa && laChu(toks[i]) }
	hoaGiua, chuGiua := 0, 0
	for i, t := range toks {
		if !t.dau && laChu(t) {
			chuGiua++
			if giua(i) {
				hoaGiua++
			}
		}
	}
	if hoaGiua >= 3 && 2*hoaGiua > chuGiua {
		return toks
	}
	for i := range toks {
		toks[i].ten = giua(i) && !giua(i-1) && !giua(i+1)
	}
	return toks
}

// khopChu says whether a word, as typed, is the compound word sp: spelled
// exactly so, or typed without marks and folding onto it -- and not a name,
// unless the compound is itself a proper name. Each word is judged on its
// own, not by the marks of the rest of the question (review round 4 of
// slice 6, R2). A money word whose only money spelling is the unmarked one
// (an empty dauTien list: «cho», «thu», «chi», «vay») is that money word
// when typed without marks, never a marked compound word: «chuyen 3 tram
// cho nam» gives 300 to Nam, it is not «trăm chỗ» (R1).
func khopChu(t tu, sp string, laTen bool) bool {
	if t.ten && !laTen {
		return false
	}
	if t.goc == sp {
		return true
	}
	if t.goc != t.gap || promptsafety.Fold(sp) != t.goc {
		return false
	}
	dung, laTien := dauTien[t.goc]
	return !laTien || len(dung) > 0
}

// ghepTai glues the compound that starts at toks[i], if any, and says how
// many words it took.
func ghepTai(toks []tu, i int) (string, int) {
	for _, c := range ghepTheoDau[toks[i].gap] {
		if i+len(c.tu) > len(toks) {
			continue
		}
		khop := true
		for j, sp := range c.tu {
			if !khopChu(toks[i+j], sp, c.ten) {
				khop = false
				break
			}
		}
		if khop {
			return c.thanh, len(c.tu)
		}
	}
	return "", 0
}

// laSoTran says whether a word is a bare run of digits.
func laSoTran(s string) bool {
	return s != "" && strings.IndexFunc(s, func(r rune) bool { return !unicode.IsDigit(r) }) < 0
}

// chuTien is the question as the rules read it: folded words, marked
// homographs and names kept apart, compounds glued, every amount one qqtien
// or qqso, and the place readings taken out.
func chuTien(s string) string {
	toks := tachTu(s)
	for i := range toks {
		t := &toks[i]
		f := t.gap
		if dung, ok := dauTien[f]; ok && f != t.goc && !slices.Contains(dung, t.goc) {
			f += "_"
		}
		switch {
		case t.goc == "tự":
			// «tự trả»: paying for oneself (tuTraRe).
			f = "qqtu"
		case t.goc == "cành" && i > 0 && laSoTran(toks[i-1].goc) && soDungDuoc(toks, i):
			// «50 cành» is 50k where an amount can stand («bắn 50 cành cho
			// Trinh»); «gửi 20 cành hồng» is flowers (review round 4, R2).
			f = "k"
		case (t.goc == "báo" || t.goc == "bảo") && i+1 < len(toks) && toks[i+1].gap == "truoc":
			// «báo trước», «bảo trước» tell ahead; only «bao trước» pays
			// ahead (review round 4 of slice 6, R-pre).
			f = "bao_"
		case t.cua && i+1 < len(toks) && toks[i+1].gap == "tab":
			// «Minh's tab»: a person's tab (tieng_anh).
			f = "qqcua"
		}
		// A place word typed with other marks or as a name is not the place
		// («Chuyển Quân 150k», «Chuyển Lịch 100k»); a name is not the word
		// that makes the number after it an address or a time («Trả Phong
		// 200 nha», «Trả Hằng 200 nha»). The «_» appended is doubled where it
		// would make another word of that list («Thu» is not «thứ»).
		dung, laNoi := noiChu[f]
		if laNoi && (t.ten || (dung != nil && t.goc != t.gap && !slices.Contains(dung, t.goc))) || truocKhongSo[f] && t.ten {
			f += "_"
			for truocKhongSo[f] {
				f += "_"
			}
		}
		t.f = f
	}
	// Typed without marks, «no» right before a budget word is «no» (full),
	// as in «ăn gì cho no tầm 60k»; «nợ tầm 300k» keeps its mark.
	for i := 0; i+1 < len(toks); i++ {
		if toks[i].goc == "no" && (toks[i+1].f == "tam" || toks[i+1].f == "khoang" || toks[i+1].f == "duoi" || toks[i+1].f == "co") {
			toks[i].f = "no_"
		}
	}
	var chu []string
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		// «chuyển đi cho Nam», «200k chuyển đi»: «đi» urges the transfer on.
		if t.f == "chuyen" && i+1 < len(toks) && toks[i+1].f == "di" && (i+2 == len(toks) || diGiuc[toks[i+2].f]) {
			chu = append(chu, "chuyen")
			i++
			continue
		}
		// «chuyển sang cho Nam» stays a move.
		if t.f == "chuyen" && i+2 < len(toks) && toks[i+1].f == "sang" && toks[i+2].f == "cho" {
			chu = append(chu, "chuyen")
			i++
			continue
		}
		if c, n := ghepTai(toks, i); n > 0 {
			chu = append(chu, c)
			i += n - 1
			continue
		}
		if t.ngat && len(t.f) >= 2 && laSoTran(t.f) {
			t.f += ngatSo
		}
		chu = append(chu, t.f)
	}
	g := " " + strings.Join(chu, " ") + " "
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
	g = tuTraRe.ReplaceAllString(g, "qqtutra")
	g = veRe.ReplaceAllString(g, "qqgia")
	g = uocRe.ReplaceAllString(g, "qquoc")
	return strings.Join(strings.Fields(g), " ")
}

// soDungDuoc says whether an amount can stand at toks[i], by what follows
// it, as soTran judges a bare number: last, before a clause break, or
// before one of sauSo or «hôm» naming a past day.
func soDungDuoc(toks []tu, i int) bool {
	if i+1 == len(toks) || toks[i].ngat {
		return true
	}
	sau := toks[i+1].gap
	return sauSo[sau] || (sau == "hom" && i+2 < len(toks) && homSau[toks[i+2].gap])
}

// soTran marks a bare number as qqso where an amount can stand, and takes
// the clause-break mark (ngatSo) off every number.
func soTran(toks []string) string {
	for i, t := range toks {
		t, ngat := strings.CutSuffix(t, ngatSo)
		toks[i] = t
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
		sau := ""
		if i+1 < len(toks) {
			sau = strings.TrimSuffix(toks[i+1], ngatSo)
		}
		hom := sau == "hom" && i+2 < len(toks) && homSau[strings.TrimSuffix(toks[i+2], ngatSo)]
		if i+1 == len(toks) || ngat || sauSo[sau] || hom {
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

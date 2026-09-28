package tuvung

import "mobile/services/core/internal/domain/promptsafety"

// A place's own words, read for the hard filters. Two opposite rules, because
// the two filters fail in opposite directions:
//
//   - Allergens (DiUngQuan): every mention counts, negated or not, wished
//     for or not. A wrong yes only hides a place from someone with that
//     allergy; a wrong no shows it to them.
//   - Diets (AnKiengQuan): only a plain statement counts. A wrong yes shows a
//     place to someone who cannot eat there, so a mention in a clause that
//     denies, wishes, looks back or looks ahead, or a sentence that also
//     names what breaks the diet («nêm nước mắm», «nấu chung nồi với
//     thịt», «halal: không chứng nhận»), is not a yes.
//
// Whether the writer typed marks is decided over the whole row (CoDau of
// every text of it), then each field is read on its own: a bare «cua» in a
// kind is crab when the name beside it carries marks, even though the kind
// is one word long.

// DiUngQuan returns the allergens a place's text names: every phrase of
// DiUng, and every one-syllable allergen word typed with its own marks
// (dauMotAm). Declaration order. The text is read as a row of its own;
// DiUngQuanHang reads one field of a row whose marks were judged over all
// its fields.
func DiUngQuan(text string) []string { return DiUngQuanHang(text, CoDau(text)) }

// DiUngQuanHang is DiUngQuan on one field of a row; coDau is CoDau over the
// whole row.
func DiUngQuanHang(text string, coDau bool) []string {
	c := catCau(text)
	c.coDau = coDau
	return c.diUngQuan(false)
}

// DiUngNhan is DiUngQuanHang on a label (a kind, a trait), which also reads
// a one-syllable allergen word that is a whole item of it, typed with its
// marks or none: an importer's «Cua», «Muc», «Oc, Nhau».
func DiUngNhan(label string, coDau bool) []string {
	c := catCau(label)
	c.coDau = coDau
	return c.diUngQuan(true)
}

func (c cau) diUngQuan(nhan bool) []string {
	found := map[string]bool{}
	for _, id := range DiUng.QuetAmTiet(c.s) {
		found[id] = true
	}
	for i := range c.s {
		for _, id := range c.motAmQuan(i) {
			found[id] = true
		}
		if nhan {
			for _, id := range c.motAmNhan(i) {
				found[id] = true
			}
		}
	}
	var out []string
	for _, m := range DiUng.muc {
		if found[m.ID] {
			out = append(out, m.ID)
		}
	}
	return out
}

// anKiengQuan is the diet vocabulary for a place's words: AnKieng's phrases
// without «ăn chay» -- on a place that is a customer eating («khách ăn chay
// lưu ý», «dẫn bạn ăn chay tới», «dạy nấu ăn chay») -- plus phrases only a
// place says. Built from AnKieng so the two never drift (a test holds it).
var anKiengQuan = func() *TuVung {
	them := map[string][]string{
		"chay":       {"bếp chay", "thực đơn chay", "cơm chay", "làm chay", "lựa chọn chay", "plant based", "người ăn chay"},
		"thuan_chay": {"plant based"},
	}
	var muc []Muc
	for _, m := range AnKieng.Muc() {
		var cum []string
		for _, c := range m.Cum {
			if c != "ăn chay" {
				cum = append(cum, c)
			}
		}
		muc = append(muc, Muc{ID: m.ID, Nhan: m.Nhan, Cum: append(cum, them[m.ID]...)})
	}
	return dung("an_kieng_quan", muc)
}()

// tuChan are words that, anywhere in a diet mention's clause, keep it from
// counting: a denial («không», «chưa», «no», «non-», «hết», «ngừng», and a
// structured field's false, 0, null, N/A, pending, nope), a wish («ước»,
// «mong»), the past («từng», «cũ», «former»), the future («sẽ», «sắp»,
// «chờ», «đang cập nhật»), a closure («tạm ngưng», «đóng cửa», «nghỉ»,
// «closed») or only some days («chỉ thứ Hai», «ngày rằm», «cuối tuần»,
// «theo mùa», «weekends»). The value lists the forms the word must be typed
// in when it folds onto another word («chưa» and «chua», «đừng» and
// «dùng»), nil for any form; in a text with no marks at all, the bare form
// counts too. An expired or missing certificate («expired», «hết hạn»,
// «lapsed», «uncertified») is a denial.
var tuChan = map[string][]string{
	"khong": {"không"}, "chua": {"chưa"}, "chang": {"chẳng"}, "ko": nil, "k": nil, "kh": nil, "hok": nil, "hem": nil,
	"no": nil, "not": nil, "non": nil, "without": nil, "never": nil, "none": nil, "nope": nil, "nah": nil,
	"false": nil, "0": nil, "null": nil, "nil": nil, "na": nil, "pending": nil, "unknown": nil, "tbd": nil, "maybe": nil,
	"het": {"hết"}, "ngung": {"ngừng", "ngưng"}, "dung": {"đừng"}, "tam": {"tạm"}, "dong": {"đóng"}, "nghi": {"nghỉ"},
	"uoc": {"ước"}, "mong": {"mong", "mồng"}, "se": {"sẽ"}, "sap": {"sắp"}, "tung": {"từng"}, "neu": {"nếu"}, "cu": {"cũ"},
	"cho": {"chờ"}, "cap": {"cập"}, "thu": {"thứ"}, "ram": {"rằm"}, "mung": {"mùng"}, "cuoi": {"cuối"}, "mua": {"mùa"},
	"if": nil, "would": nil, "soon": nil, "wish": nil, "used": nil, "former": nil, "formerly": nil,
	"expired": nil, "expire": nil, "expires": nil, "lapsed": nil, "revoked": nil, "uncertified": nil,
	"closed": nil, "close": nil, "temporarily": nil, "discontinued": nil, "unavailable": nil, "partial": nil, "partially": nil,
	"weekend": nil, "weekends": nil, "seasonal": nil,
	"monday": nil, "tuesday": nil, "wednesday": nil, "thursday": nil, "friday": nil, "saturday": nil, "sunday": nil,
	"mondays": nil, "tuesdays": nil, "wednesdays": nil, "thursdays": nil, "fridays": nil, "saturdays": nil, "sundays": nil,
}

// tuChanCum are denials of more than one syllable, compared folded: «N/A»,
// «chủ nhật».
var tuChanCum = [][]string{{"n", "a"}, {"chu", "nhat"}}

// Contrast words open a new clause: «chưa có đồ chay nhưng có buffet chay
// cuối tuần» states the buffet.
var tuTuongPhan = map[string][]string{"nhung": {"nhưng"}, "but": nil, "however": nil}

func (c cau) laTu(i int, words map[string][]string) bool {
	forms, ok := words[c.s[i]]
	if !ok {
		return false
	}
	return forms == nil || trong(forms, c.raw[i]) || (!c.coDau && c.raw[i] == c.s[i])
}

// chanTai reports whether a denial of tuChan or tuChanCum starts at i and
// ends before hi.
func (c cau) chanTai(i, hi int) bool {
	if c.laTu(i, tuChan) {
		return true
	}
	for _, t := range tuChanCum {
		if i+len(t) <= hi && khopTai(c.s, i, t) {
			return true
		}
	}
	return false
}

// menhDe returns the clause [lo, hi) holding position i: no break of a comma
// or stronger inside, no contrast word after its first syllable.
func (c cau) menhDe(i int) (int, int) {
	lo := i
	for lo > 0 && !c.ngatTruoc(lo, ngatVe) && !c.laTu(lo, tuTuongPhan) {
		lo--
	}
	hi := i + 1
	for hi < len(c.s) && !c.ngatTruoc(hi, ngatVe) && !c.laTu(hi, tuTuongPhan) {
		hi++
	}
	return lo, hi
}

// cauChua returns the sentence [lo, hi) holding position i.
func (c cau) cauChua(i int) (int, int) {
	lo := i
	for lo > 0 && !c.ngatTruoc(lo, ngatCau) {
		lo--
	}
	hi := i + 1
	for hi < len(c.s) && !c.ngatTruoc(hi, ngatCau) {
		hi++
	}
	return lo, hi
}

// Words that, stated in the same sentence as a diet, break it. A word is not
// stated when a denial stands at most phuDinhGan syllables before it in the
// sentence («không dùng trứng, sữa hay mật ong»). For halal the list also
// holds what says the certificate is missing or lapsed («không chứng
// nhận», «not certified», «hết hạn»), which a denial before it does not
// undo.
type tuPha struct {
	am  []string
	raw string // for one syllable that folds onto another word
	// ngoai: syllables (folded) that, right after it, make it another word
	// («thịt chay» is mock meat).
	ngoai []string
}

func pha(text, raw string) tuPha { return tuPha{am: AmTiet(text), raw: raw} }

var (
	// «thịt», «meat» break a vegetarian diet (a kitchen that also serves
	// meat, «nấu chung nồi với thịt»), except «thịt chay» (mock meat) and
	// «meat-free»; so does a shared pot, oil or kitchen.
	phaChay = []tuPha{pha("nước mắm", ""), pha("mắm", "mắm"), pha("xương", "xương"), pha("mỡ heo", ""), pha("mỡ lợn", ""),
		pha("fish sauce", ""), pha("bone broth", ""), {am: AmTiet("thịt"), raw: "thịt", ngoai: []string{"chay"}},
		{am: AmTiet("meat"), ngoai: []string{"free", "less"}},
		pha("chung nồi", ""), pha("chung dầu", ""), pha("chung bếp", ""), pha("bếp chung", ""), pha("shared kitchen", ""),
		pha("same pot", ""), pha("same oil", ""), pha("shared fryer", "")}
	phaThuanChay = []tuPha{pha("trứng", "trứng"), pha("sữa", "sữa"), pha("mật ong", ""), pha("phô mai", ""), pha("bơ sữa", ""),
		pha("egg", ""), pha("eggs", ""), pha("milk", ""), pha("honey", ""), pha("cheese", ""), pha("butter", ""), pha("dairy", "")}
	phaHalal = []tuPha{pha("heo", ""), pha("lợn", "lợn"), {am: AmTiet("pork"), ngoai: []string{"free"}}, pha("rượu", "rượu"),
		pha("bia", "bia"), pha("alcohol", ""), pha("beer", ""), pha("wine", ""), pha("bếp chung", "")}
	// The certificate is missing or lapsed: these deny halal wherever they
	// stand in the sentence, a denial before them included.
	phaHalalGiay = []tuPha{pha("không chứng nhận", ""), pha("chưa chứng nhận", ""), pha("không có chứng nhận", ""),
		pha("hết hạn", ""), pha("not certified", ""), pha("uncertified", ""), pha("no certificate", ""), pha("expired", ""),
		pha("halal style", "")}
)

const phuDinhGan = 6

func (c cau) biPha(i int, id string) bool {
	var lists [][]tuPha
	switch id {
	case "chay":
		lists = [][]tuPha{phaChay}
	case "thuan_chay":
		lists = [][]tuPha{phaChay, phaThuanChay}
	case "halal":
		lists = [][]tuPha{phaHalal}
	}
	lo, hi := c.cauChua(i)
	if id == "halal" && c.coTrong(lo, hi, phaHalalGiay) >= 0 {
		return true
	}
	for _, list := range lists {
		for _, p := range list {
			for k := lo; k+len(p.am) <= hi; k++ {
				if !c.phaTai(k, hi, p) {
					continue
				}
				denied := false
				for d := max(lo, k-phuDinhGan); d < k; d++ {
					denied = denied || c.laTu(d, tuChan)
				}
				if !denied {
					return true
				}
			}
		}
	}
	return false
}

// phaTai reports whether p stands at k, ending by hi.
func (c cau) phaTai(k, hi int, p tuPha) bool {
	if k+len(p.am) > hi || !khopTai(c.s, k, p.am) {
		return false
	}
	if p.raw != "" && c.raw[k] != p.raw && !(!c.coDau && c.raw[k] == c.s[k]) {
		return false
	}
	end := k + len(p.am)
	return end >= len(c.s) || c.ngatTruoc(end, ngatVe) || !trong(p.ngoai, c.s[end])
}

// coTrong returns where the first of list stands in [lo, hi), or -1.
func (c cau) coTrong(lo, hi int, list []tuPha) int {
	for k := lo; k < hi; k++ {
		for _, p := range list {
			if c.phaTai(k, hi, p) {
				return k
			}
		}
	}
	return -1
}

// AnKiengQuan returns the diets one of the fields a place declares itself
// with (its name, a kind, a trait) states plainly, with what they imply
// (vegan is vegetarian). The index reads diets only from those fields: a
// review is a customer talking («ước gì có món chay»), a description may
// say anything. The text is read as a row of its own; AnKiengQuanHang reads
// one field of a row whose marks were judged over all its fields.
func AnKiengQuan(text string) []string { return AnKiengQuanHang(text, CoDau(text)) }

// AnKiengQuanHang is AnKiengQuan on one field of a row; coDau is CoDau over
// the row. A field that is exactly a diet label -- «Chay», which the OSM
// importer writes for cuisine=vegetarian (services/api/app/places/osm.py),
// «VEGETARIAN», «Thuan chay», «Halal» -- states that diet in any case, with
// or without marks: a label is not prose. Anything else is read under the
// strict rules: «chay» typed as «chay» in a row whose marks show it; no
// denial, wish, past, future, closure or day restriction in its clause; no
// parenthesis, dash or question mark right after it; nothing in its
// sentence that breaks the diet.
func AnKiengQuanHang(text string, coDau bool) []string {
	c := catCau(text)
	c.coDau = coDau
	found := map[string]bool{}
	for _, id := range c.anKiengNhan() {
		found[id] = true
	}
	for i := range c.s {
		for _, j := range anKiengQuan.dau[c.s[i]] {
			p := anKiengQuan.cums[j]
			if !khopTai(c.s, i, p.amTiet) {
				continue
			}
			ok := !c.gachSau(i + len(p.amTiet) - 1)
			for k := range p.amTiet {
				// «chay» folds onto «chạy» and «cháy» (cơm cháy): it counts
				// only typed as «chay», in a row whose marks show it.
				if p.amTiet[k] == "chay" && (c.raw[i+k] != "chay" || !c.coDau) {
					ok = false
				}
				if k > 0 && (c.ngatTruoc(i+k, ngatVe) || c.laTu(i+k, tuTuongPhan)) {
					ok = false
				}
			}
			if !ok {
				continue
			}
			lo, hi := c.menhDe(i)
			for k := lo; k < hi && ok; k++ {
				ok = !c.chanTai(k, hi)
			}
			if !ok || c.biPha(i, p.id) {
				continue
			}
			found[p.id] = true
		}
	}
	var ids []string
	for _, m := range anKiengQuan.muc {
		if found[m.ID] {
			ids = append(ids, m.ID)
		}
	}
	return DoiKieng(ids)
}

// nhanAnKieng are the diet labels a whole field may be, as an importer or a
// form writes them: «Chay» (osm.py's label for cuisine=vegetarian),
// «Thuần chay». The English labels («Vegetarian», «Vegan», «Halal») carry
// no «chay» and are phrases read under the strict rules already. A dish
// is not a label: «com chay» without marks may be «cơm cháy».
var nhanAnKieng = []struct {
	raw []string
	id  string
}{
	{[]string{"chay"}, "chay"},
	{[]string{"thuần", "chay"}, "thuan_chay"},
}

// anKiengNhan returns the diet a whole field names when the field is
// exactly one of nhanAnKieng, each syllable typed with the label's own
// marks or none, in any case; nil otherwise, or when a question mark,
// parenthesis or dash follows it.
func (c cau) anKiengNhan() []string {
	if len(c.s) == 0 || c.gachSau(len(c.s)-1) {
		return nil
	}
	var ids []string
	for _, n := range nhanAnKieng {
		ok := len(n.raw) == len(c.raw)
		for k := 0; ok && k < len(n.raw); k++ {
			ok = (c.raw[k] == n.raw[k] || c.raw[k] == promptsafety.Fold(n.raw[k])) && !c.ngatTruoc(k, ngatVe)
		}
		if ok {
			ids = append(ids, n.id)
		}
	}
	return ids
}

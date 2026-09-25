package tuvung

// A place's own words, read for the hard filters. Two opposite rules, because
// the two filters fail in opposite directions:
//
//   - Allergens (DiUngQuan): every mention counts, negated or not, wished
//     for or not. A wrong yes only hides a place from someone with that
//     allergy; a wrong no shows it to them.
//   - Diets (AnKiengQuan): only a plain statement counts. A wrong yes shows a
//     place to someone who cannot eat there, so a mention in a clause that
//     denies, wishes, looks back or looks ahead, or a sentence that also
//     names what breaks the diet («nêm nước mắm», «bếp chung với thịt
//     heo»), is not a yes.

// DiUngQuan returns the allergens a place's text names: every phrase of
// DiUng, and every one-syllable allergen word typed with its own marks
// (dauMotAm). Declaration order.
func DiUngQuan(text string) []string {
	c := catCau(text)
	found := map[string]bool{}
	for _, id := range DiUng.QuetAmTiet(c.s) {
		found[id] = true
	}
	for i := range c.s {
		for _, id := range c.motAmQuan(i) {
			found[id] = true
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
		"chay":       {"bếp chay", "thực đơn chay", "cơm chay", "làm chay", "lựa chọn chay", "plant based"},
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
// counting: a denial («không», «chưa», «no», «non-», «hết», «ngừng»), a wish
// («ước», «mong»), the past («từng») or the future («sẽ», «sắp»). The value
// is the form the word must be typed in when it folds onto another word
// («chưa» and «chua», «dùng» and «đừng»); in a text with no marks at all,
// the bare form counts too.
var tuChan = map[string]string{
	"khong": "không", "chua": "chưa", "chang": "chẳng", "ko": "", "k": "", "kh": "", "hok": "", "hem": "",
	"no": "", "not": "", "non": "", "without": "", "never": "", "none": "",
	"het": "hết", "ngung": "ngừng", "dung": "đừng",
	"uoc": "ước", "mong": "mong", "se": "sẽ", "sap": "sắp", "tung": "từng", "neu": "nếu",
	"if": "", "would": "", "soon": "", "wish": "", "used": "",
}

// Contrast words open a new clause: «chưa có đồ chay nhưng có buffet chay
// cuối tuần» states the buffet.
var tuTuongPhan = map[string]string{"nhung": "nhưng", "but": "", "however": ""}

func (c cau) laTu(i int, words map[string]string) bool {
	form, ok := words[c.s[i]]
	if !ok {
		return false
	}
	return form == "" || c.raw[i] == form || (!c.coDau && c.raw[i] == c.s[i])
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
// sentence («không dùng trứng, sữa hay mật ong»).
type tuPha struct {
	am  []string
	raw string // for one syllable that folds onto another word
}

func pha(text, raw string) tuPha { return tuPha{am: AmTiet(text), raw: raw} }

var (
	phaChay = []tuPha{pha("nước mắm", ""), pha("mắm", "mắm"), pha("xương", "xương"), pha("mỡ heo", ""), pha("mỡ lợn", ""),
		pha("fish sauce", ""), pha("bone broth", "")}
	phaThuanChay = []tuPha{pha("trứng", "trứng"), pha("sữa", "sữa"), pha("mật ong", ""), pha("phô mai", ""), pha("bơ sữa", ""),
		pha("egg", ""), pha("eggs", ""), pha("milk", ""), pha("honey", ""), pha("cheese", ""), pha("butter", ""), pha("dairy", "")}
	phaHalal = []tuPha{pha("heo", ""), pha("lợn", "lợn"), pha("pork", ""), pha("rượu", "rượu"), pha("bia", "bia"), pha("alcohol", ""),
		pha("beer", ""), pha("wine", ""), pha("bếp chung", "")}
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
	for _, list := range lists {
		for _, p := range list {
			for k := lo; k+len(p.am) <= hi; k++ {
				if !khopTai(c.s, k, p.am) {
					continue
				}
				if p.raw != "" && c.raw[k] != p.raw && !(!c.coDau && c.raw[k] == c.s[k]) {
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

// AnKiengQuan returns the diets one of a place's structured fields (a kind,
// a trait) states plainly, with what they imply (vegan is vegetarian). The
// index reads diets only from those fields: a review is a customer talking
// («ước gì có món chay»), a description may say anything.
func AnKiengQuan(text string) []string {
	c := catCau(text)
	found := map[string]bool{}
	for i := range c.s {
		for _, j := range anKiengQuan.dau[c.s[i]] {
			p := anKiengQuan.cums[j]
			if !khopTai(c.s, i, p.amTiet) {
				continue
			}
			ok := true
			for k := range p.amTiet {
				// «chay» folds onto «chạy» and «cháy» (cơm cháy): it counts
				// only typed as «chay», in a text whose marks show it.
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
				ok = !c.laTu(k, tuChan)
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

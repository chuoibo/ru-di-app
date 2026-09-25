package tuvung

// DiUngNguoiHoi reads the allergens an asker names, which are a hard filter
// (design 04 §5.1): a place whose own words mention any of them, or any of
// their family, is never shown.
//
// The rule (design 04 §5.1, slice 8 round 3): within any sentence that holds
// an allergy trigger -- «dị ứng», «allergic», «không ăn được», «kiêng»,
// «chịu», «react», «tránh», a symptom («nổi mẩn», «ngứa», «đi viện»)…,
// typed with or without marks, in teencode or with the common typos -- the
// reader returns the UNION of every allergen mentioned anywhere in that
// sentence: before or after the trigger, across «nhưng», «còn», «but» and
// across a request alike. The one exception: a sentence whose every
// trigger is denied («mình không dị ứng gì cả», «not allergic») and which
// holds no exception or contrast word («ngoài», «trừ», «ngoại trừ»,
// «except», «other than», «chỉ», «nhưng», «còn», «but»…) reads nothing.
//
// This over-reads on purpose. «Tôm thì mình dị ứng, còn cua thì ăn được»
// reads crab as well, and «dị ứng tôm, tìm quán ốc» reads snails as well,
// so those places are hidden too. Reading one too many only hides more
// places, which is safe; missing one hides nothing, which is not; and every
// clause rule of rounds 1 and 2 dropped a stated allergen in some common
// sentence shape. The engine unions this reading with Understand's slots
// (slice 9); the public search has nothing else.
//
// Beside the allergy triggers, the words that keep an ingredient out of a
// dish («không có», «đừng cho», «không ăn», «no», «without», «-free»,
// «trừ») read only their own list: what follows them up to a request word
// («tìm», «muốn», «ở»…), and what precedes them when nothing follows in
// their clause («tôm thì đừng cho nha»). «Phở không có hành» asks for phở.
// A sentence with neither reads nothing: «quán hải sản» asks for seafood.
// A short sentence right after an allergy sentence that only adds to it
// («Mực nữa.», «Cua cũng vậy.», «Crab too.») is read with it. «...» is a
// pause, not the end of a sentence.
//
// An allergen mentioned is any phrase of DiUng that no comma-level break
// cuts -- every phrase, the shorter ones inside a longer one included, so
// «tôm mực» is shrimp and squid and «ốc, chó» is snails -- and any
// one-syllable allergen word typed with its marks, with none or with
// another tone («cá», «ca», «sửa»), except the everyday words khongPhaiLoiGo
// lists and the compounds dauMotAm lists («cá nhân», «trung tâm»). Another
// person's allergy is read like the asker's: the filter serves the whole
// outing.
//
// Sorted in declaration order; not closed over families (MoRongDiUng).
func DiUngNguoiHoi(text string) []string {
	c := catCau(text)
	c.chuanHoaNguoiHoi()
	found := map[string]bool{}
	after := false
	for lo := 0; lo < len(c.s); {
		hi := lo + 1
		for hi < len(c.s) && !c.ngatTruoc(hi, ngatCau) {
			hi++
		}
		switch {
		case c.cauDiUng(lo, hi):
			c.docDoan(lo, hi, found)
			after = true
		case after && c.cauNoiTiep(lo, hi):
			c.docDoan(lo, hi, found)
		default:
			c.docKiemMon(lo, hi, found)
			after = false
		}
		lo = hi
	}
	for i := range c.s {
		for _, t := range tuThan {
			if khopTai(c.s, i, t.am) {
				for _, id := range t.ids {
					found[id] = true
				}
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

// cauDiUng reports whether the sentence [lo, hi) is an allergy sentence: it
// holds an allergy trigger that is not denied, or a denied one and an
// exception or contrast word anywhere («không dị ứng gì ngoài tôm», «ngoài
// tôm ra thì không dị ứng gì», «không dị ứng tôm nhưng cua thì có»).
func (c cau) cauDiUng(lo, hi int) bool {
	denied, exception := false, false
	for i := lo; i < hi; {
		if c.ngoaiLeTai(i, hi) > 0 {
			exception = true
		}
		k, n := c.kichTai(i)
		if n == 0 || i+n > hi {
			i++
			continue
		}
		exception = exception || k.ngoaiLe
		if k.diUng && (!k.cuoi || c.hetMenhDeHan(i+n)) {
			if !c.kichBiPhuDinh(i, k) {
				return true
			}
			denied = true
		}
		i += n
	}
	return denied && exception
}

// cauNoiTiep reports whether the sentence [lo, hi), right after an allergy
// sentence, only adds to it: at most six syllables, one of them «nữa»,
// «cũng», «luôn», «too», «also» («Mực nữa.», «Cua cũng vậy.», «Crab too.»).
func (c cau) cauNoiTiep(lo, hi int) bool {
	if hi-lo > 6 {
		return false
	}
	for j := lo; j < hi; j++ {
		switch c.s[j] {
		case "nua", "cung", "luon", "too", "also":
			return true
		}
	}
	return false
}

// docDoan reads every allergen mentioned in [lo, hi): every phrase of DiUng
// that starts there, ends by hi and crosses no comma-level break, the
// shorter phrases inside a longer one included, and every one-syllable
// allergen word.
func (c cau) docDoan(lo, hi int, found map[string]bool) {
	for j := lo; j < hi; j++ {
		for _, id := range c.cumHoiTai(j, hi) {
			found[id] = true
		}
		for _, id := range c.motAmHoi(j) {
			found[id] = true
		}
	}
}

// cumHoiTai returns the ids of every phrase of DiUng that starts at j, ends
// by hi and has no comma-level break inside, each once, in no set order.
func (c cau) cumHoiTai(j, hi int) []string {
	var ids []string
	for _, k := range DiUng.dau[c.s[j]] {
		p := DiUng.cums[k]
		end := j + len(p.amTiet)
		if end > hi || !khopTai(c.s, j, p.amTiet) || c.catNgang(j, end) {
			continue
		}
		ids = append(ids, p.id)
	}
	return ids
}

// catNgang reports whether a comma-level break (or stronger) falls inside
// [a, b).
func (c cau) catNgang(a, b int) bool {
	for k := a + 1; k < b; k++ {
		if c.ngatTruoc(k, ngatVe) {
			return true
		}
	}
	return false
}

// docKiemMon reads the lists of the words in [lo, hi) that keep an
// ingredient out of a dish, in a sentence that is not an allergy sentence.
func (c cau) docKiemMon(lo, hi int, found map[string]bool) {
	for i := lo; i < hi; {
		k, n := c.kichTai(i)
		if n == 0 || i+n > hi {
			i++
			continue
		}
		if k.diUng || c.kichBiPhuDinh(i, k) {
			i += n
			continue
		}
		next := i + n
		if k.sau {
			next = c.docSau(i+n, hi, found)
		}
		// What precedes is read only when nothing follows the trigger in
		// its clause («tôm thì đừng cho nha, còn cua thì ok»): in «phở
		// không có hành» the trigger has its own object.
		if k.truoc && (!k.sau || c.hetMenhDe(i+n)) {
			c.docTruoc(i, lo, found, false)
		} else if k.ke {
			c.docTruoc(i, lo, found, true)
		}
		i = max(next, i+n)
	}
}

// chuanHoa maps teencode syllables onto the words triggers are written in
// («mk dị ứg», «zị ứng», «đậu fộng», «hem ăn đc»). Two never map: «hẻm»
// (an alley), which folds onto «hem», and a «k» right after a number,
// which is thousand («200 k»), not «không».
var chuanHoa = map[string]string{
	"ko": "khong", "k": "khong", "kh": "khong", "khg": "khong", "kg": "khong", "hok": "khong", "hem": "khong",
	"hk": "khong", "khum": "khong", "dc": "duoc", "vs": "voi", "j": "gi", "fong": "phong", "fung": "phung",
	"mk": "minh", "mik": "minh", "zi": "di", "dj": "di", "ug": "ung", "un": "ung", "unh": "ung",
}

func (c *cau) chuanHoaNguoiHoi() {
	for i, w := range c.s {
		if to, ok := chuanHoa[w]; ok && !(w == "hem" && c.raw[i] != "hem") && !(w == "k" && i > 0 && laSo(c.s[i-1])) {
			c.s[i] = to
		}
		// «hông» is the southern «không»; «hồng» is not.
		if w == "hong" && (c.raw[i] == "hông" || c.raw[i] == "hong") {
			c.s[i] = "khong"
		}
	}
}

// laSo reports whether a folded syllable is all digits.
func laSo(w string) bool {
	for _, r := range w {
		if r < '0' || r > '9' {
			return false
		}
	}
	return w != ""
}

// kich is one trigger.
type kich struct {
	am []string
	// diUng: an allergy trigger, which makes its sentence an allergy
	// sentence (the whole sentence is read). Otherwise the trigger keeps an
	// ingredient out of a dish and reads only its own list: sau when the
	// list may follow it, truoc when it may precede it.
	diUng      bool
	sau, truoc bool
	// raw, for a one-syllable trigger that folds onto another word, is the
	// form it must be typed in (or with no marks at all).
	raw string
	// phuDinh: the trigger is itself a negation («không ăn được»), so no
	// negation before it is looked for.
	phuDinh bool
	// cuoi: a common word («bị», «không được») that is a trigger only where
	// its clause ends right after it («mực nướng thì mình bị,», «tôm thì
	// không được.»).
	cuoi bool
	// ke: a noun that follows its list («gluten-free»): the words right
	// before it are read whatever follows.
	ke bool
	// nghiem: raw must be typed exactly, never bare («né», not the particle
	// «ne»/«nè»).
	nghiem bool
	// ngoaiLe: an exception word («trừ», «ngoại trừ», «except»): a sentence
	// that denies a trigger and holds one is an allergy sentence.
	ngoaiLe bool
	// khongTruoc, khongSau: folded syllables that, right before or after
	// it, make it another word («dễ chịu», «chịu khó»).
	khongTruoc, khongSau []string
}

var kichDiUng = func() []kich {
	var out []kich
	add := func(text string, diUng, sau, truoc bool, raw string) *kich {
		am := AmTiet(text)
		out = append(out, kich{am: am, diUng: diUng, sau: sau, truoc: truoc, raw: raw, phuDinh: am[0] == "khong" || am[0] == "chang" || am[0] == "cha"})
		return &out[len(out)-1]
	}
	// Allergy triggers: the whole sentence is read.
	for _, t := range []string{"dị ứng", "diung", "ziung", "djung", "allergic to", "allergic", "alergic", "allergik", "sốc phản vệ",
		"phản vệ", "bất dung nạp", "không dung nạp", "kiêng", "can't eat", "cannot eat", "can not eat", "can't have", "cannot have",
		"không chịu được", "không chịu nổi", "không hợp", "react", "reacts", "reacted", "reaction", "reactions", "nhạy cảm",
		"sensitive to", "sensitivity", "bị nhẹ", "bị nặng"} {
		add(t, true, true, true, "")
	}
	for _, t := range []string{"allergy", "allergies", "alergy", "alergies", "allergie", "intolerant", "intolerance"} {
		add(t, true, true, true, "")
	}
	// One syllable that folds onto another word: typed as itself, or bare.
	for _, t := range []string{"tránh", "kỵ", "kị", "cữ"} {
		add(t, true, true, true, t)
	}
	add("né", true, true, true, "né").nghiem = true
	// «chịu»: «có đậu phộng là mình chịu»; not «dễ chịu», «chịu khó»,
	// «chịu chơi».
	k := add("chịu", true, true, true, "chịu")
	k.khongTruoc, k.khongSau = []string{"de"}, []string{"kho", "choi"}
	// «chả» is also a dish («bún chả ăn kèm…»): only «chả ăn được» counts.
	for _, verb := range []string{"ăn", "uống", "đụng", "dùng", "xài"} {
		for _, neg := range []string{"không", "chẳng", "chả"} {
			add(neg+" "+verb+" được", true, true, true, "")
		}
		add(verb+" không được", true, true, true, "")
	}
	for _, t := range []string{"nổi mẩn", "nổi mề đay", "mề đay", "khó thở", "đau bụng", "tiêu chảy", "nổi ban", "sưng môi", "sưng mặt",
		"đi viện", "nhập viện", "cấp cứu"} {
		add(t, true, false, true, "")
	}
	add("ngứa", true, false, true, "ngứa")
	add("sưng", true, false, true, "sưng")
	add("nôn", true, false, true, "nôn")
	for _, t := range []string{"không được", "bị"} {
		add(t, true, false, true, "").cuoi = true
	}
	// Words that keep an ingredient out of a dish: their own list only.
	// «không ăn» without «được» is also «không ăn cay»: what follows it is
	// read, and what precedes it only when nothing follows in its clause.
	for _, verb := range []string{"ăn", "uống", "đụng", "dùng", "xài"} {
		add("không "+verb, false, true, true, "")
		add("chẳng "+verb, false, true, true, "")
	}
	for _, t := range []string{"ngoại trừ", "except", "other than"} {
		add(t, false, true, true, "").ngoaiLe = true
	}
	add("trừ", false, true, true, "trừ").ngoaiLe = true
	// «không» with a verb of putting in: «không nêm nước tương», «không có
	// đậu hũ», «không rắc hạt điều», «đừng cho đậu phộng». What follows is
	// to be avoided; what precedes is the dish unless nothing follows.
	// «đừng» must be typed as itself: «dùng cho» (used for) folds onto
	// «đừng cho».
	for _, verb := range []string{"có", "nêm", "rắc", "cho", "bỏ", "thêm", "sử dụng", "chứa", "lấy", "muốn"} {
		add("không "+verb, false, true, true, "")
		add("đừng "+verb, false, true, true, "đừng")
	}
	add("đừng", false, true, true, "đừng")
	add("no", false, true, false, "no")
	add("without", false, true, false, "")
	add("free", false, false, true, "").ke = true
	return out
}()

// tuThan are conditions that name their allergen with no trigger.
var tuThan = []struct {
	am  []string
	ids []string
}{
	{AmTiet("celiac"), []string{"lua_mi"}},
	{AmTiet("coeliac"), []string{"lua_mi"}},
	{AmTiet("lactose"), []string{"sua"}},
}

// kichTai returns the longest trigger at position i, or a zero length.
func (c cau) kichTai(i int) (kich, int) {
	var best kich
	n := 0
	for _, k := range kichDiUng {
		if len(k.am) <= n || !khopTai(c.s, i, k.am) {
			continue
		}
		if k.raw != "" && c.raw[i] != k.raw && (k.nghiem || c.raw[i] != c.s[i]) {
			continue
		}
		if c.catNgang(i, i+len(k.am)) || c.keBen(i, i+len(k.am), k) {
			continue
		}
		best, n = k, len(k.am)
	}
	return best, n
}

// keBen reports whether a syllable of k.khongTruoc stands right before
// [a, b) or one of k.khongSau right after it, in the same clause.
func (c cau) keBen(a, b int, k kich) bool {
	if a > 0 && !c.ngatTruoc(a, ngatVe) {
		for _, w := range k.khongTruoc {
			if c.s[a-1] == w {
				return true
			}
		}
	}
	if b < len(c.s) && !c.ngatTruoc(b, ngatVe) {
		for _, w := range k.khongSau {
			if c.s[b] == w {
				return true
			}
		}
	}
	return false
}

// boQuaPhuDinh are the words that may stand between a denial and the
// trigger it denies: «không bị dị ứng», «chưa từng bị dị ứng», «không hề
// bị», «không có dị ứng», «don't have any allergies», «never been
// allergic».
var boQuaPhuDinh = map[string]bool{
	"bi": true, "phai": true, "tung": true, "he": true, "co": true, "thay": true, "la": true,
	"any": true, "have": true, "has": true, "had": true, "got": true, "been": true, "really": true, "am": true, "m": true,
	"is": true, "are": true, "re": true,
}

// kichBiPhuDinh: «không dị ứng», «không bị dị ứng», «chưa từng bị dị ứng»,
// «không phải dị ứng», «tưởng dị ứng», «đâu có dị ứng», «not allergic»,
// «don't have any allergies». Only a plain denial counts: «chưa»,
// «chẳng», «tưởng», «đâu» typed with their marks (a bare «chua» may be
// «canh chua», a bare «tuong» «nước tương», a bare «dau» «đậu»), never a
// bare «hem» or «hong», which may be «hẻm» and «hồng».
func (c cau) kichBiPhuDinh(i int, k kich) bool {
	if k.phuDinh {
		return false
	}
	j := i - 1
	for skipped := 0; skipped < 3 && j >= 0 && !c.ngatTruoc(j+1, ngatVe) && boQuaPhuDinh[c.s[j]]; skipped++ {
		j--
	}
	if j < 0 || c.ngatTruoc(j+1, ngatVe) {
		return false
	}
	switch c.s[j] {
	case "khong":
		return c.raw[j] != "hem" && c.raw[j] != "hong"
	case "not", "never", "no":
		return true
	case "t":
		// «don't», «doesn't», «isn't» are cut into «don» «t».
		switch {
		case j == 0 || c.ngatTruoc(j, ngatVe):
			return false
		case c.s[j-1] == "don", c.s[j-1] == "doesn", c.s[j-1] == "didn", c.s[j-1] == "isn", c.s[j-1] == "aren",
			c.s[j-1] == "wasn", c.s[j-1] == "haven", c.s[j-1] == "hasn", c.s[j-1] == "ain":
			return true
		}
		return false
	case "chua":
		return c.raw[j] == "chưa"
	case "chang":
		return c.raw[j] == "chẳng"
	case "tuong":
		return c.raw[j] == "tưởng"
	case "dau":
		return c.raw[j] == "đâu"
	}
	return false
}

// tuNgoaiLe are the exception and contrast words that, in a sentence that
// denies a trigger, still leave something stated: «không dị ứng gì ngoài
// tôm», «chỉ tôm thôi», «không dị ứng tôm nhưng cua thì có». The value is
// the forms a one-syllable word must be typed in when it folds onto another
// word, nil for any form; «còn» bare counts only in a text with no marks
// («con mình» is not «còn»).
var tuNgoaiLe = []struct {
	am    []string
	forms []string
}{
	{AmTiet("ngoài"), []string{"ngoài", "ngoai"}}, {AmTiet("chỉ"), []string{"chỉ", "chi"}}, {AmTiet("mỗi"), []string{"mỗi", "moi"}},
	{AmTiet("nhưng"), []string{"nhưng", "nhung"}}, {AmTiet("còn"), []string{"còn"}}, {AmTiet("tuy nhiên"), nil},
	{AmTiet("besides"), nil}, {AmTiet("apart from"), nil}, {AmTiet("only"), nil}, {AmTiet("just"), nil}, {AmTiet("but"), nil},
	{AmTiet("however"), nil},
}

// ngoaiLeTai returns the length of the exception word that starts at i and
// ends by hi, or 0.
func (c cau) ngoaiLeTai(i, hi int) int {
	for _, w := range tuNgoaiLe {
		if i+len(w.am) > hi || !khopTai(c.s, i, w.am) {
			continue
		}
		if w.forms == nil || trong(w.forms, c.raw[i]) || (!c.coDau && c.raw[i] == c.s[i]) {
			return len(w.am)
		}
	}
	return 0
}

// moYeuCau are words that open a request, which ends the list of a word
// that keeps an ingredient out: «không có tôm, tìm quán ốc», «đừng cho
// đậu phộng, muốn ăn chè». The value is the form the word must be typed in
// when it folds onto another word («đi» and «dì», «ở» and «ơ»); "" any
// form.
var moYeuCau = map[string]string{
	"tim": "tìm", "kiem": "kiếm", "kim": "kím", "muon": "muốn", "them": "thèm", "can": "cần", "cho": "cho", "di": "đi",
	"dat": "đặt", "o": "ở",
	"find": "", "looking": "", "recommend": "", "suggest": "", "where": "", "want": "", "search": "", "show": "",
}

// moYeuCauNghiem are request words whose bare form is an allergen or a
// dish: a bare «ghe» may be «ghẹ», a bare «goi» «gỏi».
var moYeuCauNghiem = map[string]string{"ghe": "ghé", "goi": "gợi"}

func (c cau) moYeuCau(j int) bool {
	if form, ok := moYeuCauNghiem[c.s[j]]; ok {
		return c.raw[j] == form
	}
	form, ok := moYeuCau[c.s[j]]
	if !ok {
		return false
	}
	return form == "" || c.raw[j] == form || c.raw[j] == c.s[j]
}

// mucHoi returns the ids and length of the allergen item at position j of a
// list: the longest phrase of DiUng there that no break cuts, else a
// one-syllable allergen word. The ids are every allergen mentioned inside
// the item, so a longer phrase never hides a shorter one («tôm mực»).
func (c cau) mucHoi(j, hi int) ([]string, int) {
	w := 0
	for _, k := range DiUng.dau[c.s[j]] {
		p := DiUng.cums[k]
		if n := len(p.amTiet); n > w && j+n <= hi && khopTai(c.s, j, p.amTiet) && !c.catNgang(j, j+n) {
			w = n
		}
	}
	if w == 0 && c.motAmHoi(j) != nil {
		w = 1
	}
	if w == 0 {
		return nil, 0
	}
	return c.idsTrong(j, j+w), w
}

// idsTrong returns every allergen mentioned inside [a, b) (docDoan).
func (c cau) idsTrong(a, b int) []string {
	found := map[string]bool{}
	c.docDoan(a, b, found)
	ids := make([]string, 0, len(found))
	for id := range found {
		ids = append(ids, id)
	}
	return ids
}

// mucHoiKetThuc returns the ids and start of the longest item that ends at
// position j, or a start of -1.
func (c cau) mucHoiKetThuc(j int) ([]string, int) {
	for w := 4; w >= 1; w-- {
		st := j - w + 1
		if st < 0 || c.catNgang(st, j+1) {
			continue
		}
		if (w == 1 && c.motAmHoi(st) != nil) || DiUng.cumDungTai(c.s, st, w) != nil {
			return c.idsTrong(st, j+1), st
		}
	}
	return nil, -1
}

// sauKich are the words that may follow a trigger that ends its clause:
// «đừng cho nha», «trừ ra», «không có đâu á».
var sauKich = map[string]bool{
	"nha": true, "nhe": true, "nhen": true, "nghen": true, "do": true, "lam": true, "nang": true, "het": true, "a": true,
	"luon": true, "lun": true, "roi": true, "qua": true, "vi": true, "here": true, "please": true, "pls": true, "nhiu": true,
	"nhieu": true, "ne": true, "thoi": true, "ha": true, "day": true, "ra": true,
}

// hetMenhDe reports whether the clause ends at position j, particles aside:
// at a break, at the end, at another trigger, or at «nhưng», «còn», «but».
func (c cau) hetMenhDe(j int) bool {
	for j < len(c.s) && !c.ngatTruoc(j, ngatVe) && sauKich[c.s[j]] {
		j++
	}
	if j >= len(c.s) || c.ngatTruoc(j, ngatVe) {
		return true
	}
	if _, n := c.kichTai(j); n > 0 {
		return true
	}
	switch c.s[j] {
	case "nhung", "con", "but":
		return true
	}
	return false
}

// hetMenhDeHan is hetMenhDe for a common-word trigger: particles, then a
// break or the end, nothing else.
func (c cau) hetMenhDeHan(j int) bool {
	for j < len(c.s) && !c.ngatTruoc(j, ngatVe) && sauKich[c.s[j]] {
		j++
	}
	return j >= len(c.s) || c.ngatTruoc(j, ngatVe)
}

// docSau reads the list after a trigger from position j to the end of the
// sentence (hi): every allergen, unknown words skipped, stopping at another
// trigger or, once past the first word, at a word that opens a request. It
// returns where it stopped.
func (c cau) docSau(j, hi int, found map[string]bool) int {
	start := j
	for j < hi {
		if k, n := c.kichTai(j); n > 0 && k.sau {
			break
		}
		if j > start && c.moYeuCau(j) {
			break
		}
		if ids, w := c.mucHoi(j, hi); w > 0 {
			for _, id := range ids {
				found[id] = true
			}
			j += w
			continue
		}
		j++
	}
	return j
}

// docTruoc reads the list before the trigger at position i, back to the
// start of the sentence (lo): every allergen, unknown words skipped,
// stopping at another trigger that reads its own list or at a word that
// opens a request. With ke it reads only the words right before the trigger
// («nut-free», «dairy and gluten free») and stops at the first word that is
// neither an allergen nor a connector.
func (c cau) docTruoc(i, lo int, found map[string]bool, ke bool) {
	for j := i - 1; j >= lo; {
		if ids, st := c.mucHoiKetThuc(j); st >= lo {
			for _, id := range ids {
				found[id] = true
			}
			j = st - 1
			continue
		}
		if k, n := c.kichTai(j); n > 0 && k.sau && j+n <= i {
			break
		}
		if c.moYeuCau(j) || (ke && !keTruoc[c.s[j]]) {
			break
		}
		j--
	}
}

// keTruoc are the words that may sit inside a list before its noun: «nut
// and dairy free», «a gluten free».
var keTruoc = map[string]bool{
	"and": true, "or": true, "severe": true, "mild": true, "bad": true, "serious": true, "strong": true, "slight": true,
	"a": true, "an": true, "my": true, "i": true, "have": true, "has": true, "plus": true, "also": true,
}

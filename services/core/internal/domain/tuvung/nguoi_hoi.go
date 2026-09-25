package tuvung

// DiUngNguoiHoi reads the allergens an asker names, which are a hard filter
// (design 04 §5.1): a place whose own words mention any of them, or any of
// their family, is never shown.
//
// The rule that decides every choice below: this reader is the safety net.
// The engine unions what it reads with Understand's slots (slice 9), and
// the public search has nothing else, so it maximises recall of stated
// allergens. Reading one too many only hides more places, which is safe;
// missing one hides nothing, which is not. Where a sentence is ambiguous
// («dị ứng tôm tái», «có ai dị ứng tôm thì báo», «sữa và trứng thì ok
// không»), every allergen it names in an allergy context is read.
//
// A mention with no trigger is a wish, not an allergy -- «quán hải sản»
// asks for seafood -- so it is not returned. After a trigger («dị ứng»,
// «không ăn được», «kiêng», «tránh», «trừ», «không có», «allergic to»,
// «no»…) every allergen up to the end of the sentence is read, unknown
// words skipped («dị ứng rất nặng với tôm», «mấy món có tôm», «đạm sữa bò»,
// «tôm, à mà cả cua nữa»); the list stops early only at another trigger or
// at a word that starts a request («tìm», «muốn», «cho mình», «ở», «đi»…:
// «dị ứng tôm, tìm quán ốc» wants ốc). When a trigger ends its clause with
// nothing after it («tôm thì mình dị ứng», «ăn tôm không được», «cua, ghẹ,
// rồi tôm nữa, mấy con đó mình dị ứng hết»), or is a symptom («ăn cua là
// ngứa»), the allergens before it in the sentence are read the same way.
// Inside the list a one-syllable allergen counts typed with its marks, with
// none or with another tone («dị ứng cá», «di ung ca», «dị ứng sửa»). The
// one thing that reads nothing is a denied trigger: «không bị dị ứng»,
// «tưởng dị ứng», «not allergic» -- and only a plain denial word counts
// («hẻm», a bare «hem» or a «k» after a number never do). Another person's
// allergy is read like the asker's: the filter serves the whole outing
// (§5.1).
//
// Sorted in declaration order; not closed over families (MoRongDiUng).
func DiUngNguoiHoi(text string) []string {
	c := catCau(text)
	c.chuanHoaNguoiHoi()
	found := map[string]bool{}
	for i := 0; i < len(c.s); {
		k, n := c.kichTai(i)
		if n == 0 {
			i++
			continue
		}
		if c.kichBiPhuDinh(i, k) {
			i += n
			continue
		}
		next, read := i+n, 0
		if k.sau {
			next, read = c.docSau(i+n, found)
		}
		// The list comes before the trigger only when nothing follows it:
		// «tôm thì mình dị ứng», «ăn cua xong là sưng môi» -- but in «quán
		// nào có cua, mình không ăn được cay» the trigger has its own object.
		if k.truoc && read == 0 && (!k.sau || c.hetMenhDe(i+n)) && (!k.cuoi || c.hetMenhDeHan(i+n)) {
			c.docTruoc(i, found, false)
		} else if k.ke {
			c.docTruoc(i, found, true)
		}
		i = max(next, i+n)
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

// chuanHoa maps teencode syllables onto the words triggers are written in
// («mk dị ứg», «zị ứng», «đậu fộng», «hem ăn đc»). Two never map: «hẻm»
// (an alley), which folds onto «hem», and a «k» right after a number,
// which is thousand («200 k»), not «không».
var chuanHoa = map[string]string{
	"ko": "khong", "k": "khong", "kh": "khong", "khg": "khong", "kg": "khong", "hok": "khong", "hem": "khong",
	"hk": "khong", "khum": "khong", "dc": "duoc", "vs": "voi", "j": "gi", "fong": "phong", "fung": "phung",
	"mk": "minh", "mik": "minh", "zi": "di", "dj": "di", "ug": "ung", "un": "ung",
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

// kich is one trigger: sau when the list may follow it, truoc when it may
// precede it; raw, for a one-syllable trigger that folds onto another word,
// is the form it must be typed in (or with no marks at all).
type kich struct {
	am         []string
	sau, truoc bool
	raw        string
	// phuDinh: the trigger is itself a negation («không ăn được»), so no
	// negation before it is looked for.
	phuDinh bool
	// cuoi: a common word («bị», «không được») that is a trigger only where
	// its clause ends right after it («mực nướng thì mình bị,», «mình ăn
	// tôm không được.»).
	cuoi bool
	// ke: an English noun that follows its list («tree nut allergy»,
	// «gluten-free»): the words right before it are read whatever follows.
	ke bool
	// nghiem: raw must be typed exactly, never bare («né», not the particle
	// «ne»/«nè»).
	nghiem bool
}

var kichDiUng = func() []kich {
	var out []kich
	add := func(text string, sau, truoc bool, raw string) {
		am := AmTiet(text)
		out = append(out, kich{am: am, sau: sau, truoc: truoc, raw: raw, phuDinh: am[0] == "khong" || am[0] == "chang" || am[0] == "cha"})
	}
	for _, t := range []string{"dị ứng", "diung", "ziung", "djung", "allergic to", "allergic", "sốc phản vệ", "phản vệ",
		"bất dung nạp", "không dung nạp", "kiêng", "ngoại trừ", "except",
		"can't eat", "cannot eat", "can not eat", "can't have", "không chịu được", "không hợp"} {
		add(t, true, true, "")
	}
	for _, t := range []string{"allergy", "allergies", "intolerant", "intolerance"} {
		add(t, true, true, "")
		out[len(out)-1].ke = true
	}
	// One syllable that folds onto another word: typed as itself, or bare.
	for _, t := range []string{"tránh", "trừ", "kỵ", "kị", "cữ"} {
		add(t, true, true, t)
	}
	add("né", true, true, "né")
	out[len(out)-1].nghiem = true
	// «chả» is also a dish («bún chả ăn kèm…»): only «chả ăn được» counts.
	for _, verb := range []string{"ăn", "uống", "đụng", "dùng", "xài"} {
		for _, neg := range []string{"không", "chẳng", "chả"} {
			add(neg+" "+verb+" được", true, true, "")
		}
		add("không "+verb, true, true, "")
		add("chẳng "+verb, true, true, "")
		add(verb+" không được", true, true, "")
	}
	// «không» with a verb of putting in: «không nêm nước tương», «không có
	// đậu hũ», «không rắc hạt điều», «đừng cho đậu phộng». What follows is
	// to be avoided; what precedes is the dish. «đừng» must be typed as
	// itself: «dùng cho» (used for) folds onto «đừng cho».
	for _, verb := range []string{"có", "nêm", "rắc", "cho", "bỏ", "thêm", "sử dụng", "chứa", "lấy", "muốn"} {
		add("không "+verb, true, false, "")
		add("đừng "+verb, true, false, "đừng")
	}
	add("đừng", true, false, "đừng")
	add("no", true, false, "no")
	add("without", true, false, "")
	add("free", false, true, "")
	out[len(out)-1].ke = true
	for _, t := range []string{"nổi mẩn", "nổi mề đay", "mề đay", "khó thở", "đau bụng", "tiêu chảy", "nổi ban", "sưng môi", "sưng mặt",
		"đi viện", "nhập viện", "cấp cứu"} {
		add(t, false, true, "")
	}
	add("ngứa", false, true, "ngứa")
	add("sưng", false, true, "sưng")
	add("nôn", false, true, "nôn")
	for _, t := range []string{"không được", "bị"} {
		am := AmTiet(t)
		out = append(out, kich{am: am, truoc: true, cuoi: true, phuDinh: am[0] == "khong"})
	}
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
		inside := false
		for j := i + 1; j < i+len(k.am); j++ {
			inside = inside || c.ngatTruoc(j, ngatVe)
		}
		if !inside {
			best, n = k, len(k.am)
		}
	}
	return best, n
}

// kichBiPhuDinh: «không dị ứng», «không bị dị ứng», «chưa từng dị ứng»,
// «không phải dị ứng», «tưởng dị ứng», «not allergic». Only a plain denial
// counts: «chưa», «chẳng», «tưởng» typed with their marks (a bare «chua»
// may be «sữa chua», a bare «tuong» «nước tương»), never a bare «hem» or
// «hong», which may be «hẻm» and «hồng».
func (c cau) kichBiPhuDinh(i int, k kich) bool {
	if k.phuDinh {
		return false
	}
	j := i - 1
	if j >= 0 && !c.ngatTruoc(j+1, ngatVe) {
		switch c.s[j] {
		case "bi", "phai", "tung", "he", "co":
			j--
		}
	}
	if j < 0 || c.ngatTruoc(j+1, ngatVe) {
		return false
	}
	switch c.s[j] {
	case "khong":
		return c.raw[j] != "hem" && c.raw[j] != "hong"
	case "not", "never", "no":
		return true
	case "chua":
		return c.raw[j] == "chưa"
	case "chang":
		return c.raw[j] == "chẳng"
	case "tuong":
		return c.raw[j] == "tưởng"
	}
	return false
}

// moYeuCau are words that open a request, which ends an allergy list: «dị
// ứng tôm, tìm quán ốc», «dị ứng tôm, muốn ăn cua rang me», «dị ứng cá, đi
// ăn ở Hội An». The value is the form the word must be typed in when it
// folds onto another word («đi» and «dì», «ở» and «ơ»); "" any form.
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

// laCa: «cả» is a connector («dị ứng cả tôm lẫn cua»); so is a bare «ca»
// that opens a list and is followed by an allergen («di ung ca tom lan
// cua»). Any other bare «ca» after a trigger is fish.
func (c cau) laCa(j int, first bool) bool {
	if c.s[j] != "ca" {
		return false
	}
	if c.raw[j] == "cả" {
		return true
	}
	if c.raw[j] != "ca" || !first || j+1 >= len(c.s) || c.ngatTruoc(j+1, ngatVe) {
		return false
	}
	_, w := c.mucHoi(j + 1)
	return w > 0
}

// mucHoi returns the ids and length of the allergen item at position j of an
// asker's list: the longest phrase of DiUng there, else a one-syllable
// allergen word.
func (c cau) mucHoi(j int) ([]string, int) {
	if c.raw[j] == "cả" {
		return nil, 0
	}
	if ids, w := DiUng.cumTai(c.s, j); w > 0 {
		for k := j + 1; k < j+w; k++ {
			if c.ngatTruoc(k, ngatVe) {
				w = 0
			}
		}
		if w > 0 {
			return ids, w
		}
	}
	if ids := c.motAmHoi(j); ids != nil {
		return ids, 1
	}
	return nil, 0
}

// mucHoiKetThuc returns the ids and start of the longest item that ends at
// position j, or a start of -1.
func (c cau) mucHoiKetThuc(j int) ([]string, int) {
	for w := 4; w >= 1; w-- {
		st := j - w + 1
		if st < 0 {
			continue
		}
		broken := false
		for k := st + 1; k <= j; k++ {
			broken = broken || c.ngatTruoc(k, ngatVe)
		}
		if broken {
			continue
		}
		if w == 1 {
			if ids := c.motAmHoi(st); ids != nil && c.raw[st] != "cả" {
				return ids, st
			}
		}
		if ids := DiUng.cumDungTai(c.s, st, w); ids != nil {
			return ids, st
		}
	}
	return nil, -1
}

// sauKich are the words that may follow a trigger that ends its clause:
// «dị ứng nha», «dị ứng nặng lắm», «dị ứng hết á», «allergy here».
var sauKich = map[string]bool{
	"nha": true, "nhe": true, "nhen": true, "do": true, "lam": true, "nang": true, "het": true, "a": true, "luon": true,
	"lun": true, "roi": true, "qua": true, "vi": true, "here": true, "please": true, "pls": true, "nhiu": true, "nhieu": true,
	"ne": true, "thoi": true, "ha": true, "day": true,
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

// docSau reads the list after a trigger from position j: every allergen up
// to the end of the sentence, unknown words skipped, stopping at another
// trigger or, once past the first word, at a word that opens a request. It
// returns where it stopped and how many items it read.
func (c cau) docSau(j int, found map[string]bool) (int, int) {
	start := j
	read := 0
	first := true
	for j < len(c.s) && !c.ngatTruoc(j, ngatCau) {
		if k, n := c.kichTai(j); n > 0 && k.sau {
			break
		}
		if j > start && c.moYeuCau(j) {
			break
		}
		if c.laCa(j, first) {
			j++
			continue
		}
		if ids, w := c.mucHoi(j); w > 0 {
			first = false
			for _, id := range ids {
				found[id] = true
			}
			read++
			j += w
			continue
		}
		j++
	}
	return j, read
}

// docTruoc reads the list before the trigger at position i: every allergen
// back to the start of the sentence, unknown words skipped, stopping at
// another trigger that reads its own list or at a word that opens a
// request. With ke it reads only the words right before the trigger
// («tree nut allergy», «severe peanut and sesame allergy») and stops at the
// first word that is neither an allergen, a connector nor a modifier.
func (c cau) docTruoc(i int, found map[string]bool, ke bool) {
	itemAfter := false
	for j := i - 1; j >= 0 && !c.ngatTruoc(j+1, ngatCau); {
		// A bare «ca» just before an item is «cả»; «cả» always is.
		if c.s[j] == "ca" && (c.raw[j] == "cả" || (c.raw[j] == "ca" && itemAfter)) {
			j--
			itemAfter = false
			continue
		}
		if ids, st := c.mucHoiKetThuc(j); st >= 0 {
			for _, id := range ids {
				found[id] = true
			}
			j = st - 1
			itemAfter = true
			continue
		}
		if k, n := c.kichTai(j); n > 0 && k.sau && j+n <= i {
			break
		}
		if c.moYeuCau(j) || (ke && !keTruoc[c.s[j]]) {
			break
		}
		j--
		itemAfter = false
	}
}

// keTruoc are the words that may sit inside an English list before its
// noun: «severe peanut and tree nut allergy», «a mild egg allergy».
var keTruoc = map[string]bool{
	"and": true, "or": true, "severe": true, "mild": true, "bad": true, "serious": true, "strong": true, "slight": true,
	"a": true, "an": true, "my": true, "i": true, "have": true, "has": true, "plus": true, "also": true,
}

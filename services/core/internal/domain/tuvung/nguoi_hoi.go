package tuvung

// DiUngNguoiHoi reads the allergens an asker names, which are a hard filter
// (design 04 §5.1): a place whose own words mention any of them, or any of
// their family, is never shown. A mention with no trigger is a wish, not an
// allergy -- «quán hải sản» asks for seafood -- so it is not returned.
//
// The list may follow a trigger («mình dị ứng cua, ghẹ», «không ăn được tôm
// và mực», «allergic to peanuts») or precede one («tôm thì mình dị ứng»,
// «ăn cua xong là sưng môi», «gluten-free», «nut allergy»). Inside the list
// a one-syllable allergen counts as typed with its marks or with none
// («dị ứng cá», «di ung ca»), but not typed with other marks («cả», «cà»).
// The list runs while each word is an allergen, a filler («với», «đồ») or a
// connector («và», «lẫn»), and never past the end of a sentence. A trigger
// that is denied («không bị dị ứng», «tưởng dị ứng»), or asked about
// someone unnamed («có ai dị ứng tôm không?»), reads nothing; an item that
// is a way of cooking («hải sản sống») or an exception («sữa đậu nành thì
// uống được») is dropped. Another person's allergy is read like the
// asker's: the filter serves the whole outing (§5.1).
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
		if c.kichBiPhuDinh(i, k) || c.hoiNguoiKhac(i) {
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
		if k.truoc && read == 0 && (!k.sau || c.hetMenhDe(i+n)) {
			c.docTruoc(i, found)
		}
		i = next
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

// chuanHoa maps teencode syllables onto the words triggers are written in.
var chuanHoa = map[string]string{
	"ko": "khong", "k": "khong", "kh": "khong", "khg": "khong", "kg": "khong", "hok": "khong", "hem": "khong",
	"hk": "khong", "khum": "khong", "dc": "duoc", "vs": "voi", "j": "gi", "fong": "phong", "fung": "phung",
	"mk": "minh", "mik": "minh", "zi": "di", "ug": "ung", "un": "ung",
}

func (c *cau) chuanHoaNguoiHoi() {
	for i, w := range c.s {
		if to, ok := chuanHoa[w]; ok {
			c.s[i] = to
		}
		// «hông» is the southern «không»; «hồng» is not.
		if w == "hong" && (c.raw[i] == "hông" || c.raw[i] == "hong") {
			c.s[i] = "khong"
		}
	}
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
}

var kichDiUng = func() []kich {
	var out []kich
	add := func(text string, sau, truoc bool, raw string) {
		am := AmTiet(text)
		out = append(out, kich{am: am, sau: sau, truoc: truoc, raw: raw, phuDinh: am[0] == "khong" || am[0] == "chang" || am[0] == "cha"})
	}
	for _, t := range []string{"dị ứng", "allergic to", "allergic", "allergy", "allergies", "sốc phản vệ", "phản vệ",
		"bất dung nạp", "không dung nạp", "intolerant", "intolerance", "kiêng", "tránh",
		"can't eat", "cannot eat", "can not eat", "can't have"} {
		add(t, true, true, "")
	}
	for _, verb := range []string{"ăn", "uống"} {
		for _, neg := range []string{"không", "chẳng", "chả"} {
			add(neg+" "+verb+" được", true, true, "")
		}
		add("không "+verb, true, true, "")
		add("chẳng "+verb, true, true, "")
		add(verb+" không được", true, true, "")
	}
	add("no", true, false, "no")
	add("free", false, true, "")
	for _, t := range []string{"nổi mẩn", "nổi mề đay", "mề đay", "khó thở", "đau bụng", "tiêu chảy"} {
		add(t, false, true, "")
	}
	add("ngứa", false, true, "ngứa")
	add("sưng", false, true, "sưng")
	add("nôn", false, true, "nôn")
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
		if k.raw != "" && c.raw[i] != k.raw && c.raw[i] != c.s[i] {
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
// «không phải dị ứng», «tưởng dị ứng», «not allergic».
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
	case "khong", "chua", "chang", "not", "never", "no":
		return true
	case "tuong":
		return c.raw[j] == "tưởng" || c.raw[j] == "tuong"
	}
	return false
}

// hoiNguoiKhac: the trigger sits in a question about someone unnamed, «có
// ai dị ứng tôm không?», «ai dị ứng hải sản?» -- but not «ai cũng dị ứng».
func (c cau) hoiNguoiKhac(i int) bool {
	start := i
	for start > 0 && !c.ngatTruoc(start, ngatCau) {
		start--
	}
	for k := start; k < i; k++ {
		if c.s[k] != "ai" || (k != start && c.s[k-1] != "co") {
			continue
		}
		if k+1 < len(c.s) && c.s[k+1] == "cung" {
			continue
		}
		return true
	}
	return false
}

// Words a list may hold besides allergens: fillers before an item and
// connectors between items.
var noiDiUng = map[string]bool{
	"voi": true, "va": true, "hoac": true, "hay": true, "lan": true, "and": true, "or": true, "them": true, "nua": true,
	"bi": true, "do": true, "mon": true, "cac": true, "loai": true, "nang": true, "nhe": true, "co": true,
}

// demTruoc are the words that may sit between a list and the trigger after
// it: «tôm THÌ MÌNH dị ứng», «đồ biển LÀ EM CHỊU, dị ứng», «ăn tôm VÀO LÀ
// ngứa».
var demTruoc = map[string]bool{
	"thi": true, "la": true, "a": true, "minh": true, "em": true, "tui": true, "toi": true, "to": true, "t": true,
	"m": true, "e": true, "anh": true, "chi": true, "bi": true, "phai": true, "vi": true, "chiu": true, "rieng": true,
	"nha": true, "an": true, "uong": true, "vao": true, "xong": true, "cu": true, "deu": true, "ma": true,
	"severe": true, "mild": true, "bad": true, "have": true, "i": true,
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
// «dị ứng nha», «dị ứng nặng lắm», «dị ứng hết», «allergy here».
var sauKich = map[string]bool{
	"nha": true, "nhe": true, "nhen": true, "do": true, "lam": true, "nang": true, "het": true, "a": true, "luon": true,
	"lun": true, "roi": true, "qua": true, "vi": true, "here": true, "please": true, "pls": true, "nhiu": true, "nhieu": true,
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

// docSau reads the list after a trigger from position j and returns where it
// ended and how many items it read.
func (c cau) docSau(j int, found map[string]bool) (int, int) {
	start := j
	var items [][]string
	first := true
	for j < len(c.s) && !(j == start && c.ngatTruoc(j, ngatCau)) && !(j > start && c.ngatTruoc(j, ngatCau)) {
		if c.laCa(j, first) {
			j++
			continue
		}
		if ids, w := c.mucHoi(j); w > 0 {
			first = false
			next := j + w
			// «không ăn được hải sản sống» is about raw food, not seafood.
			if next < len(c.s) && !c.ngatTruoc(next, ngatVe) && (c.raw[next] == "sống" || c.raw[next] == "tái") {
				j = next + 1
				continue
			}
			items = append(items, ids)
			j = next
			continue
		}
		if noiDiUng[c.s[j]] {
			j++
			continue
		}
		break
	}
	// «dị ứng sữa bò, sữa đậu nành thì uống được»: the last item is the
	// exception, not the allergy.
	if len(items) > 1 && j+1 < len(c.s) && c.s[j] == "thi" && c.laDuoc(j+1) {
		items = items[:len(items)-1]
	}
	for _, ids := range items {
		for _, id := range ids {
			found[id] = true
		}
	}
	return j, len(items)
}

// laDuoc: «ăn được», «uống được», «ok», «không sao», «thoải mái»…
func (c cau) laDuoc(j int) bool {
	for _, t := range [][]string{{"an", "duoc"}, {"uong", "duoc"}, {"duoc"}, {"ok"}, {"oke"}, {"okay"}, {"fine"}, {"khong", "sao"}, {"thoai", "mai"}, {"binh", "thuong"}} {
		if khopTai(c.s, j, t) {
			return true
		}
	}
	return false
}

// docTruoc reads the list before the trigger at position i.
func (c cau) docTruoc(i int, found map[string]bool) {
	j := i - 1
	for j >= 0 && !c.ngatTruoc(j+1, ngatCau) && demTruoc[c.s[j]] {
		j--
	}
	itemAfter := false
	for j >= 0 && !c.ngatTruoc(j+1, ngatCau) {
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
		if noiDiUng[c.s[j]] {
			j--
			itemAfter = false
			continue
		}
		break
	}
}

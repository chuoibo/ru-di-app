package tuvung

// A one-syllable allergen word that folds onto an everyday word («cua» and
// «của», «cá» and «cà», «trứng» and «trung tâm», «sữa» and «sửa») cannot be a
// phrase of DiUng: folded, it would tag half the catalogue. It is still the
// most common way to name the allergen («dị ứng cua»), so it is read here,
// from the syllable as typed:
//
//   - in a place's words, when typed with exactly its own marks («cá», never
//     «cà» or a bare «ca»; a mark-less «cua» only in a row that carries
//     marks elsewhere, where «của» would have had its own), never where a
//     longer phrase of DiUng starts at the same syllable, and never in the
//     everyday compounds listed with it («cá nhân», «mực nước»); and when it
//     is a whole item of a kind or trait, a label and not prose («Cua»,
//     «Muc», «Oc, Nhau»), typed with its marks or none;
//   - in an asker's words, only inside the list a trigger opens
//     (DiUngNguoiHoi), typed with its marks, with none, or with the same
//     letters under another tone («dị ứng sửa», «dị ứng mẻ», «trừng»): a
//     typo there reads as the allergen, the safe side. The exceptions are
//     the everyday words in khongPhaiLoiGo, far likelier than a typo.
type motAm struct {
	// co is the syllable with its own marks, lower case.
	co string
	// quan are the ids a place's words give it; nil: never read on a place
	// («lạc» is also a place name and «lost», «hạt» any seed).
	quan []string
	// hoi are the ids an asker's list gives it.
	hoi []string
	// sau and truoc are syllables (with their marks) that, right after or
	// right before it, make it part of another word.
	sau, truoc []string
}

// dauMotAm is keyed by the folded syllable.
var dauMotAm = map[string]motAm{
	"cua":  {co: "cua", quan: []string{"cua"}, hoi: []string{"cua"}},
	"ghe":  {co: "ghẹ", quan: []string{"cua"}, hoi: []string{"cua"}},
	"ca":   {co: "cá", quan: []string{"ca"}, hoi: []string{"ca"}, sau: []string{"nhân", "tính", "cược", "biệt", "thể", "voi"}},
	"muc":  {co: "mực", quan: []string{"muc"}, hoi: []string{"muc"}, sau: []string{"nước", "in", "thước"}},
	"so":   {co: "sò", quan: []string{"oc_so"}, hoi: []string{"oc_so"}},
	"hau":  {co: "hàu", quan: []string{"oc_so"}, hoi: []string{"oc_so"}},
	"hen":  {co: "hến", quan: []string{"oc_so"}, hoi: []string{"oc_so"}},
	"vem":  {co: "vẹm", quan: []string{"oc_so"}, hoi: []string{"oc_so"}},
	"tep":  {co: "tép", quan: []string{"tom"}, hoi: []string{"tom"}},
	"ruoc": {co: "ruốc", quan: []string{"tom"}, hoi: []string{"tom"}},
	// «mắm» on a menu is nearly always fish (nước mắm, mắm nêm); an asker
	// who names «mắm» alone may mean shrimp paste too, so both.
	"mam":   {co: "mắm", quan: []string{"ca"}, hoi: []string{"ca", "tom"}},
	"trung": {co: "trứng", quan: []string{"trung"}, hoi: []string{"trung"}, sau: []string{"tâm", "thu", "bình", "quốc", "học"}},
	"sua":   {co: "sữa", quan: []string{"sua"}, hoi: []string{"sua"}, sau: []string{"yến", "dừa", "gạo", "đậu", "hạt", "hạnh", "óc", "oat", "soy"}, truoc: []string{"hàu"}},
	// A bare «me» after a trigger is sesame («di ung me»), but English «me»
	// («no seafood for me») is the asker.
	"me": {co: "mè", quan: []string{"me"}, hoi: []string{"me"}, sau: []string{"nheo"},
		truoc: []string{"for", "to", "with", "let", "help", "tell", "give", "show", "about", "call"}},
	"vung": {co: "vừng", quan: []string{"me"}, hoi: []string{"me"}},
	"lac":  {co: "lạc", hoi: []string{"dau_phong"}, sau: []string{"đường", "lối", "quan", "đề", "hậu"}},
	"hat":  {co: "hạt", hoi: []string{"hat_cay"}, sau: []string{"mưa", "tiêu", "sen", "é", "giống", "bụi", "cát", "gạo"}},
	"chao": {co: "chao", quan: []string{"dau_nanh"}, hoi: []string{"dau_nanh"}, sau: []string{"đèn", "đảo", "ôi"}},
	// Teencode for «hải sản»; never on a place.
	"hs": {co: "hs", hoi: []string{"hai_san"}},
}

// khongPhaiLoiGo are everyday words spelt with an allergen word's letters
// under another tone, so common that after a trigger they are the word, not
// a typo: «dị ứng cả tôm» (both), «dị ứng của bé» (of), «mẹ mình» (mother),
// «ở mức nhẹ» (level), «ghé quán» (drop by), «hát» (sing), «cháo»
// (porridge), «chào» (hello), «cà phê».
var khongPhaiLoiGo = map[string]bool{
	"cả": true, "cà": true, "của": true, "mẹ": true, "mức": true, "ghé": true, "hát": true, "cháo": true, "chào": true,
}

func trong(list []string, raw string) bool {
	for _, x := range list {
		if x == raw {
			return true
		}
	}
	return false
}

// motAmQuan returns the ids a one-syllable allergen word at position i of a
// place's text gives, or nil.
func (c cau) motAmQuan(i int) []string {
	d, ok := dauMotAm[c.s[i]]
	if !ok || d.quan == nil || c.raw[i] != d.co {
		return nil
	}
	if !coDauTieng(d.co) && !c.coDau {
		return nil
	}
	if DiUng.coCumDaiTai(c.s, i) {
		return nil
	}
	if i+1 < len(c.s) && !c.ngatTruoc(i+1, ngatVe) && trong(d.sau, c.raw[i+1]) {
		return nil
	}
	if i > 0 && !c.ngatTruoc(i, ngatVe) && trong(d.truoc, c.raw[i-1]) {
		return nil
	}
	return d.quan
}

// motAmNhan returns the ids of a one-syllable allergen word that is a whole
// item of a label (a kind, a trait: «Cua», «Muc», «Oc, Nhau», «Hải sản |
// Cá»), typed with its own marks or none, or nil. A label is a name for
// what is served, not prose: a lone «Cua» is not «của».
func (c cau) motAmNhan(i int) []string {
	d, ok := dauMotAm[c.s[i]]
	if !ok || d.quan == nil || (c.raw[i] != d.co && c.raw[i] != c.s[i]) {
		return nil
	}
	if i > 0 && !c.ngatTruoc(i, ngatVe) {
		return nil
	}
	if i+1 < len(c.s) && !c.ngatTruoc(i+1, ngatVe) {
		return nil
	}
	return d.quan
}

// motAmHoi returns the ids a one-syllable allergen word at position i of an
// asker's list gives, or nil. The syllable may be typed with its marks,
// with none, or with its letters under another tone; the everyday compounds
// are compared folded, since a text without marks spells them the same.
func (c cau) motAmHoi(i int) []string {
	d, ok := dauMotAm[c.s[i]]
	if !ok {
		return nil
	}
	if c.raw[i] != d.co && c.raw[i] != c.s[i] && (boThanh(c.raw[i]) != boThanh(d.co) || khongPhaiLoiGo[c.raw[i]]) {
		return nil
	}
	if i+1 < len(c.s) && !c.ngatTruoc(i+1, ngatVe) {
		for _, w := range d.sau {
			if AmTiet(w)[0] == c.s[i+1] {
				return nil
			}
		}
	}
	if i > 0 && !c.ngatTruoc(i, ngatVe) {
		for _, w := range d.truoc {
			if AmTiet(w)[0] == c.s[i-1] {
				return nil
			}
		}
	}
	return d.hoi
}

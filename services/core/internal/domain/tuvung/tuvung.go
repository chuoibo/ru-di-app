// Package tuvung is the closed vocabulary the AI engine and the retrieval index
// speak about places in: allergens (di_ung), diets (an_kieng), the four place
// categories (loai_cho) and atmospheres (khi_chat), each entry with the
// phrases people write for it. Understand takes its enums from here, the
// retrieval index tags a place from its own words with it, and the retrieval
// query reads the asker's words with it -- one list, so the three never drift
// (docs/claude/2026-09-25/thiet-ke-ai/04-rag-va-nap-du-lieu.md §2.2).
//
// Every phrase is matched on promptsafety.Fold text (marks dropped, đ as d,
// lower case), as a whole run of syllables: «tôm» finds «Tôm nướng» and «tom
// nuong» but not «tomato». Folding merges words that differ only in their
// marks, and the lists are written around that: a syllable that folds onto a
// common word is never a phrase on its own. «cua» (crab) folds onto «của»
// (of), so the phrases are «cua rang», «riêu cua», «bánh canh cua»…; «cá»
// (fish) folds onto «cà» (coffee, aubergine), so «cá kho», «lẩu cá»… Such a
// syllable alone is read from the text as typed instead (mot_am.go): on a
// place only with its own marks, in an asker's list after a trigger with its
// marks or none. Where a merge is left in, it errs toward the safe side:
// «óc chó» (walnut) also reads as «ốc» (shellfish), which can only hide a
// place from someone allergic to shellfish, never show them one.
//
// Pure: text in, closed ids out. No model ever fills these in (ADR-0017 §2.3).
package tuvung

import (
	"sort"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"mobile/services/core/internal/domain/promptsafety"
)

// Muc is one entry of a closed vocabulary.
type Muc struct {
	// ID is the closed value: what a slot, a column and a golden carry.
	ID string
	// Nhan is how the entry reads in Vietnamese, for a prompt or an index.
	Nhan string
	// Cum are the phrases that name it, written as people write them.
	Cum []string
}

// TuVung is one compiled vocabulary. Safe for concurrent use.
type TuVung struct {
	ten  string
	muc  []Muc
	cums []cum
	// dau indexes the phrases by their first syllable, so a scan costs one
	// map lookup per syllable of the text, not one pass per phrase.
	dau map[string][]int
}

type cum struct {
	id      string
	amTiet  []string
	vanBan  string
	thuTuID int
}

// AmTiet folds text with promptsafety.Fold and cuts it into syllables at
// every character that is neither a letter nor a digit, in order. It is the
// one tokenizer of the engine: rag/xephang ranks on it and every vocabulary
// here matches on it.
func AmTiet(text string) []string {
	return strings.FieldsFunc(promptsafety.Fold(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

func dung(ten string, muc []Muc) *TuVung {
	v := &TuVung{ten: ten, muc: muc, dau: map[string][]int{}}
	seen := map[string]bool{}
	for i, m := range muc {
		if seen[m.ID] {
			panic("tuvung: duplicate id " + m.ID + " in " + ten)
		}
		seen[m.ID] = true
		for _, c := range m.Cum {
			s := AmTiet(c)
			if len(s) == 0 {
				panic("tuvung: empty phrase for " + m.ID)
			}
			v.dau[s[0]] = append(v.dau[s[0]], len(v.cums))
			v.cums = append(v.cums, cum{id: m.ID, amTiet: s, vanBan: c, thuTuID: i})
		}
	}
	return v
}

// Ten is the vocabulary's name: di_ung, an_kieng, loai_cho or khi_chat.
func (v *TuVung) Ten() string { return v.ten }

// IDs lists the closed values in declaration order.
func (v *TuVung) IDs() []string {
	out := make([]string, len(v.muc))
	for i, m := range v.muc {
		out[i] = m.ID
	}
	return out
}

// Co reports whether id is one of the closed values.
func (v *TuVung) Co(id string) bool {
	for _, m := range v.muc {
		if m.ID == id {
			return true
		}
	}
	return false
}

// Nhan is the Vietnamese label of id, or "" for a value outside the list.
func (v *TuVung) Nhan(id string) string {
	for _, m := range v.muc {
		if m.ID == id {
			return m.Nhan
		}
	}
	return ""
}

// Muc returns a copy of the entries, in declaration order.
func (v *TuVung) Muc() []Muc {
	out := make([]Muc, len(v.muc))
	for i, m := range v.muc {
		out[i] = Muc{ID: m.ID, Nhan: m.Nhan, Cum: append([]string(nil), m.Cum...)}
	}
	return out
}

// LocHopLe keeps the values of ids that are in the list, once each, in
// declaration order: what a slot filled by a model is reduced to.
func (v *TuVung) LocHopLe(ids []string) []string {
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}
	var out []string
	for _, m := range v.muc {
		if want[m.ID] {
			out = append(out, m.ID)
		}
	}
	return out
}

// Quet returns the ids whose phrases occur in text, once each, in declaration
// order.
func (v *TuVung) Quet(text string) []string {
	return v.quet(AmTiet(text), false)
}

// QuetKhongPhuDinh is Quet that skips a mention negated in the two syllables
// just before it («không có món chay», «chưa có đồ chay»). It reads an
// asker's diets, where the window stays narrow on purpose: a negation read
// too far («mình không ăn thịt nên tìm quán chay») would drop a diet the
// asker has. A place's diets are read by AnKiengQuan, which is strict the
// other way.
func (v *TuVung) QuetKhongPhuDinh(text string) []string {
	return v.quet(AmTiet(text), true)
}

// QuetAmTiet is Quet on text the caller already cut with AmTiet, for a
// caller that scans one text with several vocabularies.
func (v *TuVung) QuetAmTiet(syllables []string) []string { return v.quet(syllables, false) }

// QuetAmTietKhongPhuDinh is QuetKhongPhuDinh on syllables from AmTiet.
func (v *TuVung) QuetAmTietKhongPhuDinh(syllables []string) []string {
	return v.quet(syllables, true)
}

func (v *TuVung) quet(s []string, boPhuDinh bool) []string {
	found := map[int]bool{}
	for at, w := range s {
		for _, j := range v.dau[w] {
			c := v.cums[j]
			if !khopTai(s, at, c.amTiet) || (boPhuDinh && phuDinh(s, at)) {
				continue
			}
			found[c.thuTuID] = true
		}
	}
	idx := make([]int, 0, len(found))
	for i := range found {
		idx = append(idx, i)
	}
	sort.Ints(idx)
	out := make([]string, len(idx))
	for i, j := range idx {
		out[i] = v.muc[j].ID
	}
	return out
}

// phuDinh reports whether one of the two syllables before position at negates
// what follows.
func phuDinh(s []string, at int) bool {
	for i := at - 1; i >= 0 && i >= at-2; i-- {
		if s[i] == "khong" || s[i] == "chua" || s[i] == "ko" || s[i] == "k" {
			return true
		}
	}
	return false
}

// ViTri returns every position where phrase (already cut into folded
// syllables) starts in text (the same), in order.
func ViTri(text, phrase []string) []int {
	if len(phrase) == 0 || len(phrase) > len(text) {
		return nil
	}
	var out []int
	for i := 0; i+len(phrase) <= len(text); i++ {
		match := true
		for j := range phrase {
			if text[i+j] != phrase[j] {
				match = false
				break
			}
		}
		if match {
			out = append(out, i)
		}
	}
	return out
}

// KhongDau reports whether text has letters and none of them carries a
// Vietnamese mark or is đ: a question typed without diacritics, which the
// retrieval index answers exactly as the same question with them.
func KhongDau(text string) bool {
	letters := false
	for _, r := range norm.NFD.String(text) {
		if unicode.Is(unicode.Mn, r) || r == 'đ' || r == 'Đ' {
			return false
		}
		if unicode.IsLetter(r) {
			letters = true
		}
	}
	return letters
}

// DiUng is the allergen list. A place is tagged from its own words with
// DiUngQuan (every mention counts, negated or not: a wrong yes only hides a
// place); an asker's allergens come from DiUngNguoiHoi. MoRongDiUng closes a
// set over the family, so «hải sản» and «tôm» exclude each other's places.
// A phrase may name several entries («giáp xác» is shrimp and crab,
// «shellfish» is those and shellfish, «nuts» is tree nuts and peanuts).
// One-syllable words that fold onto everyday words («cua», «cá», «trứng»)
// are not phrases here; they are read only where typed with their own marks
// (dauMotAm).
var DiUng = dung("di_ung", []Muc{
	{ID: "hai_san", Nhan: "hải sản", Cum: []string{"hải sản", "thủy hải sản", "đồ biển", "món biển", "đồ tanh", "seafood"}},
	{ID: "tom", Nhan: "tôm", Cum: []string{"tôm", "tôm hùm", "mắm tôm", "mắm ruốc", "tép", "giáp xác", "shrimp", "shrimps", "prawn", "prawns", "lobster", "lobsters", "shellfish", "crustacean", "crustaceans"}},
	{ID: "cua", Nhan: "cua, ghẹ", Cum: []string{"cua biển", "cua đồng", "cua rang", "cua lột", "cua hấp", "cua sốt", "cua hoàng đế", "cua ghẹ", "riêu cua", "bánh canh cua", "lẩu cua", "chả cua", "súp cua", "ghẹ hấp", "ghẹ rang", "ghẹ luộc", "giáp xác", "crab", "crabs", "shellfish", "crustacean", "crustaceans"}},
	{ID: "muc", Nhan: "mực, bạch tuộc", Cum: []string{"mực nướng", "mực chiên", "mực xào", "mực hấp", "mực ống", "mực lá", "mực một nắng", "khô mực", "râu mực", "tôm mực", "bạch tuộc", "nhuyễn thể", "squid", "squids", "octopus", "calamari"}},
	{ID: "oc_so", Nhan: "ốc, sò, nghêu, hàu", Cum: []string{"ốc", "sò điệp", "sò huyết", "sò lông", "sò nướng", "nghêu", "hàu nướng", "hàu sống", "hàu phô mai", "hàu sữa", "cơm hến", "bún hến", "nhuyễn thể", "oyster", "oysters", "clam", "clams", "scallop", "scallops", "snail", "snails", "mussel", "mussels", "shellfish"}},
	{ID: "ca", Nhan: "cá", Cum: []string{"cá kho", "cá nướng", "cá chiên", "cá hấp", "cá lóc", "cá hồi", "cá ngừ", "cá thu", "cá basa", "cá viên", "cá biển", "cá nhỏ", "lẩu cá", "gỏi cá", "chả cá", "bún cá", "cháo cá", "canh cá", "nước mắm", "đồ tanh", "sushi", "sashimi", "fish"}},
	{ID: "dau_phong", Nhan: "đậu phộng", Cum: []string{"đậu phộng", "đậu phụng", "dầu phộng", "dầu lạc", "lạc rang", "hạt lạc", "kẹo lạc", "peanut", "peanuts", "nut", "nuts"}},
	{ID: "hat_cay", Nhan: "các loại hạt", Cum: []string{"hạt điều", "hạnh nhân", "óc chó", "hạt dẻ cười", "hạt dẻ nướng", "hạt dẻ rang", "mắc ca", "macca", "hồ đào", "sữa hạt", "walnut", "walnuts", "almond", "almonds", "cashew", "cashews", "hazelnut", "hazelnuts", "pistachio", "pistachios", "tree nut", "tree nuts", "nut", "nuts"}},
	{ID: "sua", Nhan: "sữa", Cum: []string{"sữa tươi", "sữa chua", "sữa đặc", "sữa bò", "trà sữa", "cà phê sữa", "cà phê muối", "bạc xỉu", "phô mai", "bơ sữa", "bơ tỏi", "kem sữa", "kem tươi", "kem bơ", "kem que", "sốt kem", "bánh kem", "bánh flan", "bánh sừng bò", "croissant", "latte", "cappuccino", "cheese", "milk", "butter", "cream", "dairy", "lactose", "yogurt"}},
	{ID: "trung", Nhan: "trứng", Cum: []string{"trứng gà", "trứng vịt", "trứng cút", "trứng chiên", "trứng muối", "trứng ốp la", "ốp la", "hột vịt", "hột gà", "lòng trắng trứng", "lòng đỏ trứng", "bánh flan", "egg", "eggs"}},
	{ID: "lua_mi", Nhan: "lúa mì (gluten)", Cum: []string{"mì", "bánh mì", "bột mì", "lúa mì", "bánh bao", "bánh ngọt", "bánh quy", "bánh bông lan", "bánh kem", "bánh sừng bò", "croissant", "pizza", "pasta", "spaghetti", "wheat", "gluten"}},
	{ID: "dau_nanh", Nhan: "đậu nành", Cum: []string{"đậu nành", "sữa đậu nành", "đậu hũ", "đậu hủ", "đậu phụ", "tàu hũ", "tào phớ", "nước tương", "xì dầu", "tofu", "soy", "miso"}},
	{ID: "me", Nhan: "mè (vừng)", Cum: []string{"hạt mè", "mè rang", "mè đen", "dầu mè", "muối mè", "kẹo mè", "hạt vừng", "dầu vừng", "muối vừng", "sesame"}},
})

// hoDiUng is each allergen's family head: seafood covers shrimp, crab, squid,
// shellfish and fish, in both directions.
var hoDiUng = map[string]string{"tom": "hai_san", "cua": "hai_san", "muc": "hai_san", "oc_so": "hai_san", "ca": "hai_san"}

// MoRongDiUng closes a set of allergens over their family: each one, its
// head, and everything under a head it names itself. Unknown values are
// dropped. Sorted in declaration order.
func MoRongDiUng(ids []string) []string {
	want := map[string]bool{}
	given := DiUng.LocHopLe(ids)
	for _, id := range given {
		want[id] = true
		if head, ok := hoDiUng[id]; ok {
			want[head] = true
		}
	}
	// Only a head the asker named brings its family in: fish brings the
	// seafood head (a «hải sản» place may serve fish), not shrimp.
	for _, id := range given {
		for child, head := range hoDiUng {
			if head == id {
				want[child] = true
			}
		}
	}
	var out []string
	for _, m := range DiUng.muc {
		if want[m.ID] {
			out = append(out, m.ID)
		}
	}
	return out
}

// cumTai returns the ids and length of the longest phrases that start at
// position i of s (every entry that lists a phrase of that length there, in
// declaration order), or 0.
func (v *TuVung) cumTai(s []string, i int) ([]string, int) {
	best := 0
	for _, j := range v.dau[s[i]] {
		if c := v.cums[j]; len(c.amTiet) > best && khopTai(s, i, c.amTiet) {
			best = len(c.amTiet)
		}
	}
	if best == 0 {
		return nil, 0
	}
	want := map[int]bool{}
	for _, j := range v.dau[s[i]] {
		if c := v.cums[j]; len(c.amTiet) == best && khopTai(s, i, c.amTiet) {
			want[c.thuTuID] = true
		}
	}
	var ids []string
	for k, m := range v.muc {
		if want[k] {
			ids = append(ids, m.ID)
		}
	}
	return ids, best
}

// cumDungTai returns the ids of the phrases of exactly w syllables that
// start at position i of s, in declaration order, or nil.
func (v *TuVung) cumDungTai(s []string, i, w int) []string {
	if i >= len(s) {
		return nil
	}
	want := map[int]bool{}
	for _, j := range v.dau[s[i]] {
		if c := v.cums[j]; len(c.amTiet) == w && khopTai(s, i, c.amTiet) {
			want[c.thuTuID] = true
		}
	}
	var ids []string
	for k, m := range v.muc {
		if want[k] {
			ids = append(ids, m.ID)
		}
	}
	return ids
}

// coCumDaiTai reports whether a phrase of two syllables or more starts at i.
func (v *TuVung) coCumDaiTai(s []string, i int) bool {
	for _, j := range v.dau[s[i]] {
		if c := v.cums[j]; len(c.amTiet) > 1 && khopTai(s, i, c.amTiet) {
			return true
		}
	}
	return false
}

// khopTai reports whether phrase starts at position at of s.
func khopTai(s []string, at int, phrase []string) bool {
	if at+len(phrase) > len(s) {
		return false
	}
	for j, w := range phrase {
		if s[at+j] != w {
			return false
		}
	}
	return true
}

// AnKieng is the diet list. An asker's diets are read with QuetKhongPhuDinh;
// a place's with AnKiengQuan, from its kinds and traits only. A vegan place
// is a vegetarian place too (DoiKieng).
var AnKieng = dung("an_kieng", []Muc{
	{ID: "chay", Nhan: "ăn chay", Cum: []string{"ăn chay", "đồ chay", "món chay", "quán chay", "nhà hàng chay", "bún chay", "phở chay", "lẩu chay", "buffet chay", "chay tịnh", "thuần chay", "vegetarian", "veggie", "vegan"}},
	{ID: "thuan_chay", Nhan: "thuần chay", Cum: []string{"thuần chay", "vegan"}},
	{ID: "halal", Nhan: "halal", Cum: []string{"halal"}},
})

// DoiKieng adds what a diet tag implies (vegan is vegetarian), in declaration
// order.
func DoiKieng(ids []string) []string {
	ids = AnKieng.LocHopLe(ids)
	for _, id := range ids {
		if id == "thuan_chay" {
			return AnKieng.LocHopLe(append(ids, "chay"))
		}
	}
	return ids
}

// LoaiCho is the four product categories (app.places.catalog.CATEGORIES), with
// the words that ask for each. Its ids are the catalogue's category ids.
var LoaiCho = dung("loai_cho", []Muc{
	{ID: "quan-an-local", Nhan: "Quán ăn local", Cum: []string{"quán ăn", "đồ ăn", "món ăn", "ăn uống", "ăn trưa", "ăn tối", "ăn sáng", "ăn vặt", "đặc sản", "ẩm thực", "quán lẩu", "ăn lẩu", "nướng", "bún", "cơm", "phở bò", "phở gà", "bánh xèo", "local"}},
	{ID: "cafe", Nhan: "Cafe", Cum: []string{"cà phê", "cafe", "caphe", "coffee", "trà sữa", "quán trà", "tiệm trà", "trà chiều"}},
	{ID: "vui-choi", Nhan: "Vui chơi", Cum: []string{"vui chơi", "trò chơi", "chơi", "công viên", "bảo tàng", "tham quan", "leo núi", "bowling", "bi a", "bida", "karaoke", "xem phim", "escape room"}},
	{ID: "di-choi-dem", Nhan: "Đi chơi đêm", Cum: []string{"đi chơi đêm", "về đêm", "ban đêm", "chợ đêm", "phố đêm", "đêm khuya", "ăn đêm", "khuya", "quán nhậu", "đi nhậu", "bar", "pub", "club", "beer"}},
})

// KhiChat is the atmospheres a group asks for and a place describes.
var KhiChat = dung("khi_chat", []Muc{
	{ID: "yen_tinh", Nhan: "yên tĩnh", Cum: []string{"yên tĩnh", "yên ắng", "tĩnh lặng", "chill", "thư giãn", "nhẹ nhàng", "riêng tư", "ít người"}},
	{ID: "soi_dong", Nhan: "sôi động", Cum: []string{"sôi động", "náo nhiệt", "nhộn nhịp", "đông vui", "nhạc sống", "live music", "xập xình"}},
	{ID: "lang_man", Nhan: "lãng mạn", Cum: []string{"lãng mạn", "hẹn hò", "cặp đôi", "ánh nến", "romantic", "date"}},
	{ID: "song_ao", Nhan: "sống ảo", Cum: []string{"sống ảo", "check in", "checkin", "chụp ảnh", "chụp hình", "góc chụp", "decor", "instagram"}},
	{ID: "view_dep", Nhan: "view đẹp", Cum: []string{"view", "ngắm cảnh", "cảnh đẹp", "hoàng hôn", "toàn cảnh", "ngắm đồi", "ngắm thành phố"}},
	{ID: "ngoai_troi", Nhan: "ngoài trời", Cum: []string{"ngoài trời", "sân vườn", "vườn", "rooftop", "sân thượng", "ven hồ", "bờ hồ", "bãi cỏ", "dã ngoại", "cắm trại", "outdoor"}},
	{ID: "am_cung", Nhan: "ấm cúng", Cum: []string{"ấm cúng", "ấm áp", "gần gũi", "nhỏ xinh", "xinh xắn", "cozy"}},
	{ID: "binh_dan", Nhan: "bình dân", Cum: []string{"bình dân", "vỉa hè", "giá rẻ", "giá mềm", "sinh viên"}},
	{ID: "sang_trong", Nhan: "sang trọng", Cum: []string{"sang trọng", "cao cấp", "sang chảnh", "lịch sự", "fine dining"}},
	{ID: "lam_viec", Nhan: "làm việc", Cum: []string{"làm việc", "học bài", "học nhóm", "laptop", "ổ cắm", "wifi", "coworking"}},
	{ID: "nhom_dong", Nhan: "hợp nhóm đông", Cum: []string{"nhóm đông", "đông người", "nhóm lớn", "tụ tập", "hội nhóm", "bàn dài", "phòng riêng"}},
})

// tuDung are folded syllables that carry no retrieval signal on their own:
// function words, pronouns, and the generic «quán», «ăn», «chỗ». Only a
// query's lexical terms drop them; an index keeps every word.
var tuDung = map[string]bool{
	"a": true, "ai": true, "an": true, "anh": true, "ban": true, "ben": true, "bi": true, "cac": true, "can": true, "cho": true,
	"chi": true, "chung": true, "co": true, "cung": true, "dang": true, "dau": true, "day": true, "de": true, "den": true,
	"di": true, "do": true, "duoc": true, "em": true, "gan": true, "gi": true, "giup": true, "goi": true, "hay": true, "hoac": true,
	"hon": true, "hom": true, "k": true, "khong": true, "ko": true, "kia": true, "la": true, "lam": true, "luc": true, "ma": true,
	"minh": true, "mot": true, "muon": true, "nao": true, "nay": true, "nen": true, "nguoi": true, "nha": true, "nhe": true, "nhieu": true,
	"nhu": true, "nhung": true, "o": true, "oi": true, "qua": true, "quan": true, "rat": true, "sao": true, "tai": true, "the": true,
	"thi": true, "tim": true, "toi": true, "trong": true, "uong": true, "va": true, "voi": true, "vua": true, "xin": true, "y": true,
}

// LaTuDung reports whether a folded syllable is a stop word for a query.
// Every run of digits is one too: a number in a question is a slot (a time, a
// budget, a head count), not a word to match.
func LaTuDung(syllable string) bool {
	if tuDung[syllable] {
		return true
	}
	for _, r := range syllable {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return syllable != ""
}

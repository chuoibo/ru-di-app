// Package guard is the engine's deterministic guard: patterns that run before
// any model call on every untrusted source (the question, the panel turns the
// device sends back, the strings of the context slip), the money law that
// refuses without a model call, and the output guard that reads an answer
// before a byte of it leaves the engine.
//
// Patterns are deterministic and read text after preprocess has cleaned it,
// then normalised here: NFKC (fullwidth and mathematical letters to plain),
// Cyrillic and Greek look-alikes to Latin, then promptsafety.Fold (no marks,
// đ as d, lower case). The oracle port's own instruction regex
// (promptsafety.LooksLikeInstruction) runs first and is not edited: the
// extensions live here.
package guard

import (
	"regexp"
	"strings"

	"golang.org/x/text/unicode/norm"

	"mobile/services/core/internal/domain/promptsafety"
)

// homoglyph maps look-alike letters to the Latin letter they imitate.
var homoglyph = strings.NewReplacer(
	// Cyrillic
	"а", "a", "е", "e", "о", "o", "р", "p", "с", "c", "у", "y", "х", "x", "і", "i", "ј", "j", "ѕ", "s", "һ", "h", "ԁ", "d", "ԛ", "q", "ԝ", "w", "к", "k", "м", "m", "н", "h", "т", "t", "в", "b",
	"А", "A", "В", "B", "Е", "E", "К", "K", "М", "M", "Н", "H", "О", "O", "Р", "P", "С", "C", "Т", "T", "Х", "X", "У", "Y", "І", "I", "Ј", "J", "Ѕ", "S",
	// Greek
	"α", "a", "ε", "e", "ι", "i", "κ", "k", "ν", "v", "ο", "o", "ρ", "p", "τ", "t", "υ", "u", "χ", "x",
	"Α", "A", "Β", "B", "Ε", "E", "Ζ", "Z", "Η", "H", "Ι", "I", "Κ", "K", "Μ", "M", "Ν", "N", "Ο", "O", "Ρ", "P", "Τ", "T", "Υ", "Y", "Χ", "X",
)

// Gap is the normalised form every pattern reads.
func Gap(s string) string {
	return promptsafety.Fold(homoglyph.Replace(norm.NFKC.String(s)))
}

type mau struct {
	ten string
	re  *regexp.Regexp
	// bo, when set, rejects a match the regexp alone cannot tell apart
	// («đóng vai trò» is «to play a part», not role play).
	bo func(m []string) bool
}

// chiThi are the instruction-shaped patterns, on Gap text.
var chiThi = []mau{
	// «bỏ qua mọi hướng dẫn», «quên hết luật»: a verb of dropping, a
	// quantifier, a noun of instruction.
	{ten: "vi_bo_qua_moi", re: regexp.MustCompile(`\b(bo qua|phot lo|lo di|quen di|quen het|quen|mac ke|vut bo|gat bo|khong can theo|dung theo|thoi theo)\s+(het\s+|moi\s+|tat ca\s+|toan bo\s+)(cac\s+|nhung\s+)?(chi thi|huong dan|luat|quy tac|lenh|thiet lap|cai dat|prompt|rang buoc)`)},
	// «bỏ qua các chỉ thị trước đó», «phớt lờ hướng dẫn hệ thống».
	{ten: "vi_bo_qua_truoc", re: regexp.MustCompile(`\b(bo qua|phot lo|lo di|quen|mac ke|vut bo|gat bo)\s+(cac\s+|nhung\s+)?(chi thi|huong dan|luat|quy tac|lenh|thiet lap|cai dat|prompt|rang buoc)\s+(truoc( do| day)?|o tren|phia tren|ben tren|he thong|ban dau|goc|cu|cua (ban|nep|he thong|ai))\b`)},
	{ten: "vi_dong_vai", re: regexp.MustCompile(`\b(dong|nhap) vai\s+(\S+)`), bo: func(m []string) bool { return m[2] == "tro" }},
	{ten: "vi_gia_vo", re: regexp.MustCompile(`\b(ban|nep|may|mi)\s+(hay\s+|phai\s+)?(gia vo|gia lam|hoa than|bien thanh)\b`)},
	{ten: "vi_tu_gio_ban_la", re: regexp.MustCompile(`\b(tu (gio|bay gio|nay)( tro di)?|ke tu (gio|bay gio|nay))\s*,?\s*(ban|nep|may|mi)\s+(la|se la|phai|chi duoc|khong con)\b`)},
	{ten: "vi_ban_gio_la", re: regexp.MustCompile(`\b(ban|nep|may|mi)\s+(gio|bay gio|hien gio)\s+(la|se la)\s+(mot\s+)?(ai|tro ly|chatbot|mo hinh|he thong|admin|developer|nguoi khac|dan|gpt|gemini)\b`)},
	{ten: "vi_tiet_lo", re: regexp.MustCompile(`\b(in|hien|dua|tiet lo|cho (minh |toi |tao )?(xem|biet)|doc|lap lai|nhac lai|viet|chep|xuat|trich)(\s+(ra|lai|het|nguyen van|toan bo))*\s+(cac\s+|nhung\s+)?(prompt|system prompt|loi nhac he thong|(chi thi|huong dan|cau lenh|luat|quy tac|thiet lap)\s+(he thong|goc|ban dau|an|noi bo)|ma (bi mat|kiem|noi bo|an))`)},
	{ten: "vi_che_do", re: regexp.MustCompile(`\bche do\s+(nha phat trien|developer|dev|admin|quan tri|god|than|khong gioi han|khong kiem duyet|jailbreak|dan)\b`)},
	{ten: "vi_vuot_kiem_duyet", re: regexp.MustCompile(`\b(vuot qua|pha bo|go bo|bo|tat)\s+(het\s+|moi\s+)?(kiem duyet|bo loc|rao chan|gioi han (cua ban|an toan|he thong))\b`)},
	{ten: "en_bo_qua", re: regexp.MustCompile(`\b(forget|ignore|disregard|override|bypass|skip)\s+((all|any|every|your|the|previous|prior|above|earlier|initial|original|these|those)\s+)+(instructions?|rules?|prompts?|guidelines?|directives?|constraints?|guardrails?|system)\b`)},
	{ten: "en_vai", re: regexp.MustCompile(`\b(act as|pretend (to be|you are|you're)|role-?play as|you are no longer|from now on,? you|new instructions?\s*:|updated instructions?\s*:)`)},
	// «show me the rules of this game» is a question; «show me your rules»
	// and «print the hidden instructions» are not.
	{ten: "en_tiet_lo", re: regexp.MustCompile(`\b(reveal|print|show|repeat|output|display|leak|dump|recite|tell me)\s+(me\s+)?(your\s+((system|initial|original|hidden|secret)\s+)*(prompt|instructions?|rules|guidelines|system message)|the\s+((system|initial|original|hidden|secret)\s+)+(prompt|instructions?|rules|guidelines|message)|the\s+prompt)\b`)},
	{ten: "en_che_do", re: regexp.MustCompile(`\b(developer|dev|god|debug|admin|jailbreak|unrestricted|dan) mode\b|\bjailbreak|\bdo anything now\b`)},
	{ten: "the_vai", re: regexp.MustCompile(`\[/?inst\]|<<\s*/?\s*sys\s*>>|<\|im_(start|end)\|>|###\s*(system|instruction|assistant)|(^|\s)(system|assistant|developer)\s*:\s|</?\s*(system|assistant|user|developer|du_lieu)\b`)},
}

// base64Run is a long run of base64: a payload a reader cannot see.
var base64Run = regexp.MustCompile(`[A-Za-z0-9+/]{200,}={0,2}`)

// Nghi says whether s looks like an instruction to the model rather than
// data, and names the family (for tests and the eval, never stored).
func Nghi(s string) (bool, string) {
	if promptsafety.LooksLikeInstruction(homoglyph.Replace(norm.NFKC.String(s))) {
		return true, "promptsafety"
	}
	if base64Run.MatchString(s) {
		return true, "base64"
	}
	g := Gap(s)
	for _, p := range chiThi {
		for _, m := range p.re.FindAllStringSubmatch(g, -1) {
			if p.bo != nil && p.bo(m) {
				continue
			}
			return true, p.ten
		}
	}
	return false, ""
}

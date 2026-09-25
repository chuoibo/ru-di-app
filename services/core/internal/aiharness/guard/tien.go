package guard

import "regexp"

// The money law for Nếp, deterministic and before any model call (ADR-0033
// §2.2, ADR-0036 §2.9, CLAUDE.md «ba luật tiền»): Nếp never moves, records,
// splits, settles or chases money. A question that asks it to is refused with
// a fixed sentence and costs no model call. Asking what a place costs, or
// whether a budget is enough, is not a money action and passes.
var tien = []*regexp.Regexp{
	regexp.MustCompile(`\bchuyen (tien|khoan)\b`),
	regexp.MustCompile(`\bchuyen (cho|giup|ho|lai|tra)\b.{0,40}\b\d+([.,]\d+)*\s*(k|nghin|ngan|trieu|tr|d|dong|vnd|cu)\b`),
	regexp.MustCompile(`\b(ghi|tra|doi|xoa|gach|nhac|chia|tinh|don|thu) no\b`),
	regexp.MustCompile(`\bai (dang |con |van )?no\b|\bno ai\b`),
	regexp.MustCompile(`\b(minh|toi|tao|em|anh|chi|to) (dang |con |van )?no\b.{0,30}(\bbao nhieu\b|\btien\b|\d)`),
	regexp.MustCompile(`\bthanh toan\s+(giup|ho|cho|tien|khoan|no|bill|hoa don|lai)\b|\b(tat|quyet) toan\b`),
	regexp.MustCompile(`\bchia (bill|tien|hoa don|khoan|chi phi|deu tien)\b|\btinh tien (cho|giup|moi nguoi|ca nhom)\b`),
	regexp.MustCompile(`\b(so tai khoan|stk|tai khoan ngan hang|ma qr (chuyen|thanh toan|ngan hang)|vietqr|momo|zalopay|vi dien tu)\b`),
	regexp.MustCompile(`\b(nhac|doi|hoi|giuc)\b.{0,25}\b(tra tien|chuyen tien|tra lai tien|gui tien)\b|\b(da|chua) (tra|chuyen|gui) tien\b|\btra (lai )?tien\b|\b(ung|tam ung|vay|muon) tien\b`),
	regexp.MustCompile(`\b(transfer|send|wire|pay)\s+(the\s+)?(money|cash|funds|him|her|them|me|back)\b|\bwho owes\b|\bowes? (me|you|him|her|them|us)\b|\bsettle (up|the (bill|debt|tab))\b|\bsplit (the )?(bill|check|cost)\b|\bbank account\b`),
}

// LaTien says whether the question asks Nếp to act on money.
func LaTien(s string) bool {
	g := Gap(s)
	for _, re := range tien {
		if re.MatchString(g) {
			return true
		}
	}
	return false
}

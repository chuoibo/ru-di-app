// Package cau holds the fixed Vietnamese sentences of the AI engine: why a
// turn ended without an answer, and what a turn is doing while it runs. A
// refusal is a sentence written here, never words a model wrote (ADR-0037
// §2.9).
//
// apps/mobile/tests/cau-chu-goi-ai.test.mjs reads the table below: every code
// must have the same sentence in the app's LOI_KET_QUA_NEP, so the person reads
// what this file says. Keep the table's shape -- one `{Name, "sentence"},` per
// line -- because that test parses it.
package cau

// Ma is a code a turn can end with instead of an answer. The job's `code`
// column carries it; the app turns it into the sentence below.
type Ma string

const (
	// ProviderUnavailable: the model could not be reached or answered with an
	// error. The same code the brain path has always used.
	ProviderUnavailable Ma = "provider_unavailable"
	// InvalidAIResult: the model answered, but not with something the panel
	// can show (empty, over the length cap, blocked by the provider's safety).
	InvalidAIResult Ma = "invalid_ai_result"
	// NepLuiManTien: the slip names a money screen. Checked first, before any
	// model call (ADR-0033 §2.2).
	NepLuiManTien Ma = "nep_lui_man_tien"
	// NepKhongChamTien: the question asks Nếp to act on money -- transfer,
	// record or settle a debt, split a bill. Refused with no model call.
	NepKhongChamTien Ma = "nep_khong_cham_tien"
	// TraLoiBiChan: the output guard stopped the answer (a phone number, an
	// account number, a claim of an action never taken, the leak canary).
	TraLoiBiChan Ma = "ai_tra_loi_bi_chan"
	// HetNganSach: the turn ran out of model calls, steps or time.
	HetNganSach Ma = "ai_het_ngan_sach"
)

type dong struct {
	ma  Ma
	chu string
}

// bang is every code and its one sentence. The first two are the sentences
// the app already showed for those codes on the brain path, unchanged.
var bang = []dong{
	{ProviderUnavailable, "Nếp chưa trả lời được lúc này. Bạn thử lại sau ít phút nhé."},
	{InvalidAIResult, "Nếp nghĩ chưa ra câu trả lời gọn. Bạn hỏi lại theo cách khác nhé."},
	{NepLuiManTien, "Ở màn tiền Nếp không trả lời, để bạn tự xem số liệu cho rõ. Ra màn khác rồi hỏi Nếp nhé."},
	{NepKhongChamTien, "Nếp không làm việc tiền nong: không chuyển, không ghi nợ, không chia hay nhắc ai trả. Bạn tự xem ở màn tiền nhé."},
	{TraLoiBiChan, "Nếp vừa viết ra một câu không nên gửi nên đã dừng lại. Bạn hỏi lại theo cách khác nhé."},
	{HetNganSach, "Câu này cần nghĩ lâu hơn sức Nếp cho một lượt. Bạn hỏi gọn lại từng ý nhé."},
}

// Valid says whether m is one of the codes above.
func (m Ma) Valid() bool {
	for _, d := range bang {
		if d.ma == m {
			return true
		}
	}
	return false
}

// Cau is the sentence for m, or "" for a code this package does not know.
func Cau(m Ma) string {
	for _, d := range bang {
		if d.ma == m {
			return d.chu
		}
	}
	return ""
}

// Tat lists every code, in table order.
func Tat() []Ma {
	out := make([]Ma, len(bang))
	for i, d := range bang {
		out[i] = d.ma
	}
	return out
}

// TrangThai is what a running turn is doing, for the «thinking» state of the
// panel. The engine emits these before any I/O; the words are fixed here.
type TrangThai string

const (
	DangDoc  TrangThai = "dang_doc"
	DangNghi TrangThai = "dang_nghi"
)

var cauTrangThai = map[TrangThai]string{
	DangDoc:  "Nếp đang đọc câu hỏi",
	DangNghi: "Nếp đang nghĩ",
}

// Valid says whether s is a known status.
func (s TrangThai) Valid() bool { _, ok := cauTrangThai[s]; return ok }

// CauTrangThai is the fixed words for s.
func CauTrangThai(s TrangThai) string { return cauTrangThai[s] }

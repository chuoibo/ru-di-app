package cau

import "strings"

// The fixed sentences a retrieval answer ends with when the model chose not
// to answer (design 04 §5.6: «the clarifying question and the refusal are
// the harness's fixed sentences, zero model calls»). They are filled with
// constraint names the model picked from a closed enum (crag.RangBuocs),
// each turned into the fixed phrase below: no word a model wrote reaches
// these sentences. These codes are not job codes: the turn ends with an
// answer, so the app's LOI_KET_QUA_NEP table is not involved.

// cumRangBuoc is the phrase of each constraint name of truyhoi (diem_den,
// di_ung, …). A test in crag holds it to crag.RangBuocs.
var cumRangBuoc = map[string]string{
	"diem_den":  "điểm đến bạn chọn",
	"di_ung":    "yêu cầu tránh món bạn dị ứng",
	"an_kieng":  "chế độ ăn của bạn",
	"mo_luc":    "giờ mở cửa bạn cần",
	"ngan_sach": "ngân sách bạn đặt",
	"khi_chat":  "không khí bạn muốn",
	"loai_cho":  "loại chỗ bạn tìm",
	"khu_vuc":   "khu vực bạn muốn",
}

// CumRangBuoc is the fixed phrase of constraint r, or "" for a name this
// package does not know.
func CumRangBuoc(r string) string { return cumRangBuoc[r] }

// noi joins phrases: «a», «a và b», «a, b và c».
func noi(cum []string) string {
	switch len(cum) {
	case 0:
		return ""
	case 1:
		return cum[0]
	}
	return strings.Join(cum[:len(cum)-1], ", ") + " và " + cum[len(cum)-1]
}

func cumCua(rs []string) []string {
	var out []string
	for _, r := range rs {
		if c := cumRangBuoc[r]; c != "" {
			out = append(out, c)
		}
	}
	return out
}

// Fixed sentences with no constraint to name.
const (
	// KhongThay: nothing was found and no constraint can be named.
	KhongThay = "Mình chưa tìm được chỗ nào khớp yêu cầu này. Bạn thử nói rõ hơn một chút nhé."
	// DuPhong is the safe fallback when an answer failed its checks twice:
	// the grounded cards stand, the model's prose does not.
	DuPhong = "Mình chỉ chắc được những gì trong các thẻ dưới đây, bạn xem giúp mình nhé."
	// DuPhongHuongDan is DuPhong for an answer from the app manual, which
	// has no cards.
	DuPhongHuongDan = "Mình chưa chắc được hướng dẫn cho câu này. Bạn mở phần Trợ giúp trong app để xem cho đúng nhé."
	// DiUngNgoaiDanhMuc opens every answer of a turn where the router
	// flagged an allergen outside the closed list (hieu.Slots
	// DiUngNgoaiDanhMuc): no filter could check it, and the person is told
	// so whatever path answered and whatever the model wrote.
	DiUngNgoaiDanhMuc = "Lưu ý: có món bạn không ăn được nằm ngoài danh mục dị ứng app lọc được, nên các gợi ý chưa loại được nó; bạn hỏi lại quán cho chắc nhé."
)

// TuChoi is the refusal naming the unmet constraints rs. It says the
// constraints were kept, never widened on the person's behalf.
func TuChoi(rs []string) string {
	cum := cumCua(rs)
	if len(cum) == 0 {
		return KhongThay
	}
	return "Mình chưa tìm được chỗ nào đáp ứng " + noi(cum) + ". Mình không tự nới những điều kiện này thay bạn; bạn có thể đổi chúng rồi hỏi lại nhé."
}

// HoiLai is the one clarifying question about the unmet constraints rs.
func HoiLai(rs []string) string {
	cum := cumCua(rs)
	if len(cum) == 0 {
		return KhongThay
	}
	return "Mình chưa thấy chỗ nào thật khớp với " + noi(cum) + ". Bạn muốn đổi điều kiện nào để mình tìm lại?"
}

// DaNoiLong is the note that heads an answer found after the soft
// constraints rs were relaxed, so the person reads why.
func DaNoiLong(rs []string) string {
	cum := cumCua(rs)
	if len(cum) == 0 {
		return ""
	}
	return "Mình đã bỏ bớt " + noi(cum) + " để có thêm lựa chọn."
}

// MotCho is what a place token that names no evidence of the turn becomes.
const MotCho = "một chỗ"

package cau

// The group assistant's fixed sentences (Rủ Đi AI in the thread, design 03).
// They are published as the text part of a `tra_loi` card the whole room
// reads, so they are ours, never words a model wrote. They are not job codes:
// a refusal the room sees is an answer, not a failure (the room would read
// only «Rủ Đi AI chưa trả lời lời nhờ này.»), and a failure the group bot does
// end with still carries one of the codes of bang above.
const (
	// NhomKhongChamTien answers a request the router classed as a money
	// action (move, record, settle or chase money between people). It says
	// what the assistant can do instead: a split draft for people to
	// confirm.
	NhomKhongChamTien = "Rủ Đi AI không làm việc tiền nong giữa mọi người: không chuyển tiền, không ghi nợ, không nhắc ai trả. Nếu cần chia một hoá đơn, cả nhóm nhờ mình «chia bill» để mình soạn nháp cho mọi người xem rồi tự xác nhận ở mục Chia bill nhé."
	// NhomChuaThayKhoan answers a split request in whose shared messages
	// the model found no expense with an amount.
	NhomChuaThayKhoan = "Mình chưa thấy khoản chi nào có số tiền trong các tin được chia sẻ. Cả nhóm gửi kèm tin có số tiền (ví dụ «lẩu 850k») rồi nhờ mình chia bill lại nhé."
	// NhomChuaChacSoTien answers a split request whose reading named an
	// amount its message does not support (structurally, or by the draft's
	// verifier): no draft is built, and the room is asked for a clear
	// message instead.
	NhomChuaChacSoTien = "Mình chưa chắc được số tiền của các khoản trong tin được chia sẻ nên chưa soạn nháp. Người đã trả gửi lại một tin ghi rõ khoản và số tiền (ví dụ «mình trả lẩu 850k») rồi nhờ mình chia bill lại nhé."
	// NhomLoiNhoPlan and NhomLoiNhoChiaBill stand in for an empty request:
	// a bare «@Rủ Đi» or «/plan» still asks for something, and the router
	// and the answer read these words as the request.
	NhomLoiNhoPlan     = "Lên kế hoạch đi chơi cho cả nhóm từ các tin được chia sẻ."
	NhomLoiNhoChiaBill = "Chia bill các khoản chi trong các tin được chia sẻ."
)

// The couple's fixed sentences (two classes, 2026-09-28): a chat of two
// whose two people have both turned on «Một đôi» takes the group's path
// whole, and reads these in place of the group's where the group's speak
// to a room of friends. NhomChuaChacSoTien and NhomLoiNhoChiaBill name no
// audience and serve both.
const (
	DoiKhongChamTien = "Rủ Đi AI không làm việc tiền nong giữa hai bạn: không chuyển tiền, không ghi nợ, không nhắc ai trả. Nếu cần chia một hoá đơn, hai bạn nhờ mình «chia bill» để mình soạn nháp cho hai bạn xem rồi tự xác nhận ở mục Chia bill nhé."
	DoiChuaThayKhoan = "Mình chưa thấy khoản chi nào có số tiền trong các tin được chia sẻ. Hai bạn gửi kèm tin có số tiền (ví dụ «lẩu 850k») rồi nhờ mình chia bill lại nhé."
	DoiLoiNhoPlan    = "Lên kế hoạch đi chơi cho hai bạn từ các tin được chia sẻ."
)

// CoDinhNhom is every fixed sentence the group path may release as a whole
// answer, with no model-derived field: the only texts that may reach a
// stream without a verifier's pass (the eval's invariant 8 reads it). The
// couple's are the group path's too.
func CoDinhNhom() []string {
	return []string{NhomKhongChamTien, NhomChuaThayKhoan, NhomChuaChacSoTien, DoiKhongChamTien, DoiChuaThayKhoan}
}

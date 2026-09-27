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
	// NhomLoiNhoPlan and NhomLoiNhoChiaBill stand in for an empty request:
	// a bare «@Rủ Đi» or «/plan» still asks for something, and the router
	// and the answer read these words as the request.
	NhomLoiNhoPlan     = "Lên kế hoạch đi chơi cho cả nhóm từ các tin được chia sẻ."
	NhomLoiNhoChiaBill = "Chia bill các khoản chi trong các tin được chia sẻ."
)

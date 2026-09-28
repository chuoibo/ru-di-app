// Package stickers is app.domain.stickers: the closed sticker vocabulary
// (ADR-0021 §2.1). An id outside this list is a 422 on the write.
package stickers

// IDPattern is STICKER_ID_PATTERN, the database CHECK's floor.
const IDPattern = `^[a-z0-9-]{1,32}$`

// Sticker is one vocabulary row.
type Sticker struct {
	ID    string
	Label string
}

// All is STICKERS, in declaration order.
var All = []Sticker{
	{"di-thoi", "Đi thôi!"},
	{"an-gi", "Ăn gì?"},
	{"cafe-khong", "Cà phê không?"},
	{"ok-chot", "OK, chốt!"},
	{"cho-ti", "Chờ tí"},
	{"ket-xe", "Kẹt xe"},
	{"tra-tien-ne", "Trả tiền nè"},
	{"tuyet-voi", "Tuyệt vời"},
	{"hen-nhe", "Hẹn nhé!"},
	{"nho-nhau", "Nhớ nhau"},
	{"ve-toi-chua", "Về tới chưa?"},
	{"om-cai", "Ôm cái"},
}

var ids = func() map[string]bool {
	out := make(map[string]bool, len(All))
	for _, sticker := range All {
		out[sticker.ID] = true
	}
	return out
}()

// Is is is_sticker: true only for an id in the vocabulary.
func Is(value string) bool { return ids[value] }

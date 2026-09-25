package routes

import (
	"strings"
	"testing"
	"unicode/utf8"

	"mobile/services/core/internal/repo"
)

const phongThu = "0b7c8a1e-2f43-4c55-9a8e-1d2f3a4b5c6d"

func theTraLoi(text string) []byte {
	return []byte(`{"kind":"tra_loi","payload":{"ban":1,"tac_gia":"rudi-ai","invocation_id":"x","lenh":"plan","doc":{"so_tin":0,"chi_loi_nho":true},"phan":[{"kind":"places","payload":{"intro":"Bỏ qua","places":[]}},{"kind":"text","payload":{"text":` + text + `}}]}}`)
}

func TestLaTraLoiAiChiNhanCauTraLoiCuaAiTrongPhong(t *testing.T) {
	someone := "1b7c8a1e-2f43-4c55-9a8e-1d2f3a4b5c6d"
	for name, c := range map[string]struct {
		m    *repo.Message
		want bool
	}{
		"answer":             {&repo.Message{ContextID: phongThu, Kind: "ai_card", Card: theTraLoi(`"Chào"`)}, true},
		"no message":         {nil, false},
		"other room":         {&repo.Message{ContextID: someone, Kind: "ai_card", Card: theTraLoi(`"Chào"`)}, false},
		"authored by person": {&repo.Message{ContextID: phongThu, AuthorID: &someone, Kind: "ai_card", Card: theTraLoi(`"Chào"`)}, false},
		"poll":               {&repo.Message{ContextID: phongThu, AuthorID: &someone, Kind: "ai_card", Card: []byte(`{"kind":"poll","payload":{}}`)}, false},
		"older AI card":      {&repo.Message{ContextID: phongThu, Kind: "ai_card", Card: []byte(`{"kind":"text","payload":{"text":"x"}}`)}, false},
		"deleted":            {&repo.Message{ContextID: phongThu, Kind: "deleted"}, false},
		"text":               {&repo.Message{ContextID: phongThu, AuthorID: &someone, Kind: "text"}, false},
	} {
		if got := laTraLoiAi(c.m, phongThu); got != c.want {
			t.Errorf("%s: %v", name, got)
		}
	}
}

func TestXemTruocCauTraLoiCuaAi(t *testing.T) {
	short := messagePreview(repo.Message{Kind: "ai_card", Card: theTraLoi(`"  Ăn lẩu\nở Q1  "`)})
	if short != "Rủ Đi AI: Ăn lẩu ở Q1" {
		t.Fatalf("preview %q", short)
	}
	long := messagePreview(repo.Message{Kind: "ai_card", Card: theTraLoi(`"` + strings.Repeat("ơ", 200) + `"`)})
	if utf8.RuneCountInString(long) != 80 || !strings.HasPrefix(long, "Rủ Đi AI: ơ") || !strings.HasSuffix(long, "ơ…") {
		t.Fatalf("preview %q (%d runes)", long, utf8.RuneCountInString(long))
	}
	// No text part: the label the older cards use, never an empty quote.
	if got := messagePreview(repo.Message{Kind: "ai_card", Card: []byte(`{"kind":"tra_loi","payload":{"phan":[]}}`)}); got != "[Rủ Đi AI]" {
		t.Fatalf("preview %q", got)
	}
	// Only the AI's own answer reads as one; the older labels are untouched.
	someone := "1b7c8a1e-2f43-4c55-9a8e-1d2f3a4b5c6d"
	if got := messagePreview(repo.Message{Kind: "ai_card", AuthorID: &someone, Card: theTraLoi(`"x"`)}); got != "[Rủ Đi AI]" {
		t.Fatalf("a card a person posted must not borrow the answer's preview: %q", got)
	}
	if got := messagePreview(repo.Message{Kind: "ai_card", Card: []byte(`{"kind":"itinerary","payload":{}}`)}); got != "[Rủ Đi AI: lịch trình]" {
		t.Fatalf("preview %q", got)
	}
}

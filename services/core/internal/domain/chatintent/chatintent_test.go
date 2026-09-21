package chatintent

import "testing"

func TestParseCommandAndMention(t *testing.T) {
	got := Parse("/plan Đà Lạt")
	if got == nil || got.Intent != Plan || got.Args != "Đà Lạt" {
		t.Fatalf("plan: %+v", got)
	}
	if Parse("/planning") != nil {
		t.Fatal("/planning is a word")
	}
	got = Parse("hôm nay @Rủ Đi ơi")
	if got == nil || got.Intent != Mention {
		t.Fatalf("mention: %+v", got)
	}
	if Parse("chào buổi sáng") != nil {
		t.Fatal("plain text")
	}
}

func TestParseVote(t *testing.T) {
	got := ParseVote("Ăn gì? Phở | Bún | Cơm")
	if got == nil || got.Question != "Ăn gì?" || len(got.Options) != 3 {
		t.Fatalf("vote: %+v", got)
	}
	if ParseVote("chỉ một lựa chọn") != nil {
		t.Fatal("no pipe")
	}
}

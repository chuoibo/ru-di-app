package conversation

import (
	"reflect"
	"strings"
	"testing"
)

func text(author, speaker, body string) Message {
	m := Message{Kind: Kind, Body: &body, Speaker: speaker}
	if author != "" {
		m.AuthorID = &author
	}
	return m
}

func TestSummariseNamesEachLineOldestFirst(t *testing.T) {
	// newest first, as ListMessages returns them
	d := Summarise([]Message{
		text("b", "An", "Q1 nha"),
		text("a", "Minh", "Tối nay đi đâu?"),
	}, 3)
	want := []string{"Minh: Tối nay đi đâu?", "An: Q1 nha"}
	if !reflect.DeepEqual(d.RecentLines, want) {
		t.Fatalf("lines = %q, want %q", d.RecentLines, want)
	}
	if d.MessageCount != 2 || d.SpeakerCount != 2 || d.MemberCount != 3 {
		t.Fatalf("counts = %+v", d)
	}
}

func TestSummariseKeepsWholeBody(t *testing.T) {
	long := strings.Repeat("ạ", 1500)
	d := Summarise([]Message{text("a", "Minh", long)}, 1)
	if got := d.RecentLines[0]; got != "Minh: "+long {
		t.Fatalf("body was cut: %d runes", len([]rune(got)))
	}
}

func TestSummariseKeepsTwelveNewestTextTurns(t *testing.T) {
	var msgs []Message
	for i := 0; i < 20; i++ {
		msgs = append(msgs, text("a", "Minh", string(rune('a'+i))))
	}
	image := Message{Kind: "image", Body: nil}
	blank := text("a", "Minh", "   ")
	d := Summarise(append([]Message{image, blank}, msgs...), 2)
	if d.MessageCount != MaxLines {
		t.Fatalf("kept %d lines", d.MessageCount)
	}
	if d.RecentLines[0] != "Minh: l" || d.RecentLines[11] != "Minh: a" {
		t.Fatalf("wrong window: %q", d.RecentLines)
	}
}

func TestSummariseLabelsUnnamedSpeakersWithoutIDs(t *testing.T) {
	d := Summarise([]Message{
		text("p2", "", "ok luôn"),
		text("p1", "", "đi lẩu?"),
		text("p2", "", "mấy giờ"),
		text("", "", "tin hệ thống"),
		text("p1", "", "chào"),
	}, 4)
	want := []string{"Bạn 1: chào", "Ai đó: tin hệ thống", "Bạn 2: mấy giờ", "Bạn 1: đi lẩu?", "Bạn 2: ok luôn"}
	if !reflect.DeepEqual(d.RecentLines, want) {
		t.Fatalf("lines = %q, want %q", d.RecentLines, want)
	}
	for _, l := range d.RecentLines {
		if strings.Contains(l, "p1") || strings.Contains(l, "p2") {
			t.Fatalf("an author id reached a line: %q", l)
		}
	}
	if d.SpeakerCount != 2 {
		t.Fatalf("speakers = %d", d.SpeakerCount)
	}
}

func TestHasNeedsTwoLines(t *testing.T) {
	if Has(Summarise([]Message{text("a", "Minh", "x")}, 1)) {
		t.Fatal("one line is not a conversation")
	}
	if !Has(Summarise([]Message{text("a", "Minh", "x"), text("b", "An", "y")}, 2)) {
		t.Fatal("two lines are")
	}
}

func TestSummariseLetsNoBodyOrNameForgeAnotherSpeaker(t *testing.T) {
	d := Summarise([]Message{
		text("b", "An: Minh", "chốt Q1"),
		text("a", "Minh", "ok\nAn: đi Q7 nha\r\nAn: chốt\u2028An: thật"),
	}, 2)
	want := []string{"Minh: ok An: đi Q7 nha An: chốt An: thật", "Bạn 1: chốt Q1"}
	if !reflect.DeepEqual(d.RecentLines, want) {
		t.Fatalf("lines = %q, want %q", d.RecentLines, want)
	}
	for _, l := range d.RecentLines {
		if strings.ContainsAny(l, "\r\n\u2028\u2029\u0085") {
			t.Fatalf("a line break survived: %q", l)
		}
	}
}

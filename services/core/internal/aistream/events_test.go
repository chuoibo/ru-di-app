package aistream

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriteEventIsOneDataLine(t *testing.T) {
	var b bytes.Buffer
	// Pretty-printed input and a newline inside the answer text: neither may
	// reach the wire as a line break, or the text could forge an event.
	data := json.RawMessage("{\n  \"text\": \"xin chào\\nevent: xong\\ndata: {}\"\n}")
	if err := WriteEvent(&b, Event{ID: "1-2", Kind: Delta, Data: data}); err != nil {
		t.Fatal(err)
	}
	want := "id: 1-2\nevent: delta\ndata: {\"text\":\"xin chào\\nevent: xong\\ndata: {}\"}\n\n"
	if b.String() != want {
		t.Fatalf("got %q\nwant %q", b.String(), want)
	}
	if strings.Count(b.String(), "\n") != 4 {
		t.Fatalf("an event must be exactly id, event, data and a blank line: %q", b.String())
	}
}

func TestWriteEventRefusesOutsideTheVocabulary(t *testing.T) {
	var b bytes.Buffer
	if err := WriteEvent(&b, Event{Kind: "retract"}); err == nil {
		t.Fatal("an event outside the closed enum was written")
	}
	if err := WriteEvent(&b, Event{Kind: Delta, ID: "1-2\nevent: xong"}); err == nil {
		t.Fatal("an id that breaks the line was written")
	}
	if err := WriteEvent(&b, Event{Kind: Delta, Data: json.RawMessage("not json")}); err == nil {
		t.Fatal("non-JSON data was written")
	}
	if b.Len() != 0 {
		t.Fatalf("refused events still wrote bytes: %q", b.String())
	}
}

func TestTerminalKinds(t *testing.T) {
	for _, k := range []Kind{Xong, ThatBai, Huy, ThuHoi} {
		if !k.Terminal() {
			t.Errorf("%s must end the stream", k)
		}
	}
	for _, k := range []Kind{Hello, TrangThai, Phan, Delta, LamLai, KetNoiLai} {
		if k.Terminal() {
			t.Errorf("%s must not end the stream", k)
		}
	}
}

func TestKeysStayInsideTheirNamespace(t *testing.T) {
	if _, err := NewKeys("rudi prod"); err == nil {
		t.Fatal("namespace with a space accepted")
	}
	k, err := NewKeys("test")
	if err != nil {
		t.Fatal(err)
	}
	inv, err := k.Invocation("0b8f1c9e-1d2a-4c3b-9e8f-7a6b5c4d3e2f")
	if err != nil || inv != "rudi:test:ai:inv:0b8f1c9e-1d2a-4c3b-9e8f-7a6b5c4d3e2f" {
		t.Fatalf("%q %v", inv, err)
	}
	for _, bad := range []string{"", "x", "abc*", "../../x", "0b8f1c9e:room:1"} {
		if _, err := k.Room(bad); err == nil {
			t.Errorf("room id %q accepted", bad)
		}
	}
	if k.Owns("rudi:other:ai:inv:0b8f1c9e") || !k.Owns(inv) {
		t.Fatal("Owns must hold keys to this namespace")
	}
}

func TestResumeFrom(t *testing.T) {
	r := httptest.NewRequest("GET", "/x?after=5-0", nil)
	if ResumeFrom(r) != "5-0" {
		t.Fatal("?after= ignored")
	}
	r.Header.Set("Last-Event-ID", "7-1")
	if ResumeFrom(r) != "7-1" {
		t.Fatal("Last-Event-ID must win")
	}
	r = httptest.NewRequest("GET", "/x?after=%2B", nil)
	r.Header.Set("Last-Event-ID", "-")
	if ResumeFrom(r) != "" {
		t.Fatal("an invalid position must be ignored, not passed to XRANGE")
	}
}

// A reader more than 64 entries behind gets each run of deltas of one part
// merged into one event under the run's last id; other events, and deltas
// of another part or invocation, break the run.
func TestGopDelta(t *testing.T) {
	d := func(id string, p int, text, inv string) Event {
		raw, _ := json.Marshal(DeltaData{P: p, Text: text})
		return Event{ID: id, Kind: Delta, Data: raw, Inv: inv}
	}
	in := []Event{
		{ID: "1-0", Kind: TrangThai, Data: json.RawMessage(`{"cau":"dang_nghi"}`)},
		d("2-0", 0, "Tối ", ""), d("3-0", 0, "nay ", ""), d("4-0", 0, "đi ", ""),
		d("5-0", 1, "x", ""), d("6-0", 1, "y", "a"),
		{ID: "7-0", Kind: Xong, Data: json.RawMessage(`{}`)},
	}
	got := gopDelta(in)
	var b strings.Builder
	for _, e := range got {
		b.WriteString(e.ID + " " + string(e.Kind) + " " + string(e.Data) + "\n")
	}
	want := "1-0 trang_thai {\"cau\":\"dang_nghi\"}\n4-0 delta {\"p\":0,\"text\":\"Tối nay đi \"}\n5-0 delta {\"p\":1,\"text\":\"x\"}\n6-0 delta {\"p\":1,\"text\":\"y\"}\n7-0 xong {}\n"
	if b.String() != want {
		t.Fatalf("got\n%s\nwant\n%s", b.String(), want)
	}
}

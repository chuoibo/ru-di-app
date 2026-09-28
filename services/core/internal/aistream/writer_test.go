package aistream

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

// A delta that reaches a log line shows its size, never its text.
func TestDeltaLogsOnlyItsSize(t *testing.T) {
	var b bytes.Buffer
	slog.New(slog.NewJSONHandler(&b, nil)).Info("x", "delta", DeltaData{P: 0, Text: "Đi dạo hồ Tây nhé"})
	if strings.Contains(b.String(), "hồ Tây") || !strings.Contains(b.String(), `"delta":"[24 bytes]"`) {
		t.Fatalf("log line %s", b.String())
	}
}

// A writer takes only the events a job writes; the reader's connection
// events are not the stream's.
func TestWriterTakesOnlyJobEvents(t *testing.T) {
	w := (&Stream{}).NewWriter(WriterOptions{})
	for _, k := range []Kind{Hello, ThuHoi, KetNoiLai, "retract"} {
		if w.Ghi(k, struct{}{}) {
			t.Errorf("a writer took %s", k)
		}
	}
}

// A stopping worker settles a job with no content: from then on no content
// is taken, and BeforeContent (the row's first_token_at) is never run, so
// the job it hands back carries no mark. SauChot, after a commit, lets
// content go without running it.
func TestChanNoiDungTruocNoiDung(t *testing.T) {
	chay := 0
	w := (&Stream{}).NewWriter(WriterOptions{BeforeContent: func() error { chay++; return nil }})
	if w.ChanNoiDung() {
		t.Fatal("a writer with no content said it had some")
	}
	if w.Ghi(Delta, DeltaData{Text: "x"}) || w.Ghi(Phan, struct{}{}) || chay != 0 || w.CoNoiDung() {
		t.Fatalf("content taken after ChanNoiDung (BeforeContent ran %d times)", chay)
	}
	if w.SauChot() {
		t.Fatal("SauChot let content go after it was refused")
	}
	w2 := (&Stream{}).NewWriter(WriterOptions{BeforeContent: func() error { chay++; return nil }})
	if !w2.SauChot() || !w2.CoNoiDung() || chay != 0 {
		t.Fatalf("SauChot: %v, BeforeContent ran %d times", w2.CoNoiDung(), chay)
	}
}

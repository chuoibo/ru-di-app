package normalize

import (
	"encoding/base64"
	"strings"
	"testing"
)

func cursorOf(payload string) string { return base64.RawURLEncoding.EncodeToString([]byte(payload)) }

func uuidOf(c string) string {
	return strings.Repeat(c, 8) + "-" + strings.Repeat(c, 4) + "-4" + strings.Repeat(c, 3) + "-8" + strings.Repeat(c, 3) + "-" + strings.Repeat(c, 12)
}

// A cursor's instant and id are bound like plain ones; its encoding and the
// timestamp's spelling are not hidden.
func TestCursorsBindTheirPartsAndKeepTheirSpelling(t *testing.T) {
	apply := func(texts ...string) []string {
		b := NewBinder()
		for _, text := range texts {
			if err := b.Observe(text); err != nil {
				t.Fatal(err)
			}
		}
		out := make([]string, len(texts))
		for i, text := range texts {
			out[i] = b.Apply(text)
		}
		return out
	}
	page := func(id, instant, encoded string) string {
		return `{"id":"` + id + `","cursor":"` + encoded + `"}`
	}
	refID, candID := uuidOf("a"), uuidOf("b")
	refAt, candAt := "2026-09-14T10:00:00.120000+00:00", "2026-09-14T11:30:00.340000+00:00"
	ref := apply(page(refID, refAt, cursorOf(refAt+"|"+refID)))
	cand := apply(page(candID, candAt, cursorOf(candAt+"|"+candID)))
	if ref[0] != cand[0] {
		t.Fatalf("same cursor shape differs:\n%s\n%s", ref[0], cand[0])
	}
	if want := `{"id":"<uuid#1>","cursor":"<b64u:<ts#1|f6|+00:00>|<uuid#1>>"}`; ref[0] != want {
		t.Fatalf("bound as %s, want %s", ref[0], want)
	}

	variants := map[string]string{
		"padding kept":       page(candID, candAt, cursorOf(candAt+"|"+candID)+"="),
		"zone written Z":     page(candID, candAt, cursorOf("2026-09-14T11:30:00.340000Z|"+candID)),
		"naive timestamp":    page(candID, candAt, cursorOf("2026-09-14T11:30:00.340000|"+candID)),
		"milliseconds":       page(candID, candAt, cursorOf("2026-09-14T11:30:00.340+00:00|"+candID)),
		"uppercase id":       page(candID, candAt, cursorOf(candAt+"|"+strings.ToUpper(candID))),
		"another id first":   page(candID, candAt, cursorOf(candAt+"|"+uuidOf("c"))),
		"space for the pipe": page(candID, candAt, cursorOf(candAt+" "+candID)),
	}
	for name, text := range variants {
		if got := apply(text)[0]; got == ref[0] {
			t.Errorf("%s: hidden as %s", name, got)
		}
	}

	notCursor := strings.Repeat("Zm9vYmFy", 10)
	if got := apply(`{"blob":"` + notCursor + `"}`)[0]; !strings.Contains(got, notCursor) {
		t.Fatalf("a base64 run that is not a cursor was bound: %s", got)
	}
	if got := Mask(page(refID, refAt, cursorOf(refAt+"|"+refID))); got != `{"id":"<uuid>","cursor":"<b64u>"}` {
		t.Fatalf("Mask = %s", got)
	}
}

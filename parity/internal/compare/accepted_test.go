package compare

import (
	"net/http"
	"testing"
)

func TestOnlyTheNamed204ContentLengthDivergenceIsAccepted(t *testing.T) {
	exchange := func(status int, headers ...string) Exchange {
		h := http.Header{}
		for i := 0; i+1 < len(headers); i += 2 {
			h.Add(headers[i], headers[i+1])
		}
		return Exchange{Status: status, Header: h}
	}
	cases := []struct {
		name      string
		ref, cand Exchange
		diffs     int
		accepted  bool
	}{
		{"python replay through go", exchange(204, "content-length", "0", "idempotency-replayed", "true"), exchange(204, "Idempotency-Replayed", "true"), 0, true},
		{"both send it", exchange(204, "content-length", "0"), exchange(204, "Content-Length", "0"), 0, false},
		{"neither sends it", exchange(204), exchange(204), 0, false},
		{"other direction", exchange(204), exchange(204, "Content-Length", "0"), 1, false},
		{"non-zero value", exchange(204, "content-length", "3"), exchange(204), 1, false},
		{"repeated header", exchange(204, "content-length", "0", "content-length", "0"), exchange(204), 1, false},
		{"not a 204", exchange(200, "content-length", "0"), exchange(200), 1, false},
		{"status differs too", exchange(204, "content-length", "0"), exchange(200), 2, false},
		{"accepted pair still sees other headers", exchange(204, "content-length", "0", "idempotency-replayed", "true"), exchange(204), 1, true},
	}
	for _, tc := range cases {
		diffs := Step(tc.ref, tc.cand)
		accepted := len(Accepted(tc.ref, tc.cand)) == 1
		if len(diffs) != tc.diffs || accepted != tc.accepted {
			t.Fatalf("%s: diffs %v accepted %v, want %d and %v", tc.name, diffs, accepted, tc.diffs, tc.accepted)
		}
	}
	if AcceptedDivergence[Response204ContentLength] == "" {
		t.Fatal("the accepted divergence names no scenario that must show it")
	}
}

// STATIC-VALIDATOR-VALUE accepts a VALUE and nothing else. Every case below
// that is not accepted is a case where a real port mistake would hide if the
// exception were drawn one notch wider.
func TestOnlyTheValueOfAStaticValidatorIsAccepted(t *testing.T) {
	const pyETag = `"643656382d0b4df1c31428b69735d3ec"`
	const goETag = `"397aab04ee043894e776a8a6a2605f7e"`
	const pyDate = "Mon, 21 Sep 2026 14:47:00 GMT"
	const goDate = "Tue, 22 Sep 2026 16:27:06 GMT"

	at := func(path string, status int, headers ...string) Exchange {
		h := http.Header{}
		for i := 0; i+1 < len(headers); i += 2 {
			h.Add(headers[i], headers[i+1])
		}
		return Exchange{Path: path, Status: status, Header: h}
	}
	file := func(path string, etag, date string) Exchange {
		return at(path, 200, "etag", etag, "last-modified", date, "content-type", "text/css; charset=utf-8")
	}

	cases := []struct {
		name      string
		ref, cand Exchange
		diffs     int
		accepted  bool
	}{
		{"both validators differ beneath the mount",
			file("/static/guest.css", pyETag, pyDate), file("/static/guest.css", goETag, goDate), 0, true},
		{"a 304 sends the etag alone",
			at("/static/guest.css", 304, "etag", pyETag), at("/static/guest.css", 304, "etag", goETag), 0, true},

		// Not accepted, and each of these is a real mistake.
		{"the same path outside the mount",
			file("/g/abc", pyETag, pyDate), file("/g/abc", goETag, goDate), 2, false},
		{"go dropped the etag",
			file("/static/guest.css", pyETag, pyDate),
			at("/static/guest.css", 200, "last-modified", goDate, "content-type", "text/css; charset=utf-8"), 2, false},
		{"go dropped last-modified",
			file("/static/guest.css", pyETag, pyDate),
			at("/static/guest.css", 200, "etag", goETag, "content-type", "text/css; charset=utf-8"), 2, false},
		{"go changed the etag shape",
			file("/static/guest.css", pyETag, pyDate), file("/static/guest.css", `W/"abc"`, goDate), 2, false},
		{"nothing differs",
			file("/static/guest.css", pyETag, pyDate), file("/static/guest.css", pyETag, pyDate), 0, false},
		{"the statuses differ",
			file("/static/guest.css", pyETag, pyDate),
			at("/static/guest.css", 404, "etag", goETag, "last-modified", goDate, "content-type", "text/css; charset=utf-8"), 3, false},
		{"the etag repeats",
			at("/static/guest.css", 200, "etag", pyETag, "etag", pyETag, "last-modified", pyDate),
			file("/static/guest.css", goETag, goDate), 3, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			diffs := Step(tc.ref, tc.cand)
			accepted := false
			for _, name := range Accepted(tc.ref, tc.cand) {
				if name == StaticValidatorValue {
					accepted = true
				}
			}
			if len(diffs) != tc.diffs || accepted != tc.accepted {
				t.Fatalf("diffs %v (want %d), accepted %v (want %v)", diffs, tc.diffs, accepted, tc.accepted)
			}
		})
	}

	// The acceptance covers the two validators and NOTHING else on the same
	// response: a wrong content-type or a wrong body is still a difference.
	wrongType := at("/static/guest.css", 200, "etag", goETag, "last-modified", goDate, "content-type", "text/plain")
	if diffs := Step(file("/static/guest.css", pyETag, pyDate), wrongType); len(diffs) != 1 {
		t.Errorf("content-type must still be compared, got %v", diffs)
	}
	refBody := file("/static/guest.css", pyETag, pyDate)
	refBody.Body = "body{}"
	candBody := file("/static/guest.css", goETag, goDate)
	candBody.Body = "body{color:red}"
	if diffs := Step(refBody, candBody); len(diffs) != 1 {
		t.Errorf("the body must still be compared, got %v", diffs)
	}

	if AcceptedDivergence[StaticValidatorValue] == "" {
		t.Fatal("the accepted divergence names no scenario that must show it")
	}
}

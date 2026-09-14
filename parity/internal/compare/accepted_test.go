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

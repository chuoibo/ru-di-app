package compare

import (
	"net/http"
	"strings"
	"testing"
)

func exchange(status int, body string, headers ...string) Exchange {
	h := http.Header{}
	for i := 0; i+1 < len(headers); i += 2 {
		h[headers[i]] = append(h[headers[i]], headers[i+1])
	}
	return Exchange{Status: status, Header: h, Body: body}
}

func TestIdenticalExchangesAreEqual(t *testing.T) {
	ref := exchange(201, `{"id":"<uuid#1>"}`, "content-type", "application/json", "content-length", "17",
		"date", "Mon, 14 Sep 2026 10:00:00 GMT", "server", "uvicorn")
	cand := exchange(201, `{"id":"<uuid#1>"}`, "Content-Type", "application/json", "Content-Length", "17",
		"Date", "Mon, 14 Sep 2026 10:00:07 GMT", "Server", "uvicorn")
	if diffs := Step(ref, cand); len(diffs) != 0 {
		t.Fatalf("diffs = %v", diffs)
	}
}

func TestEachDifferenceIsReported(t *testing.T) {
	base := exchange(201, `{"score":1.0}`, "content-type", "application/json", "vary", "Origin")
	cases := map[string]struct {
		cand Exchange
		part string
	}{
		"status":            {exchange(200, `{"score":1.0}`, "content-type", "application/json", "vary", "Origin"), "status"},
		"body float":        {exchange(201, `{"score":1}`, "content-type", "application/json", "vary", "Origin"), "body"},
		"header value":      {exchange(201, `{"score":1.0}`, "content-type", "application/json; charset=utf-8", "vary", "Origin"), "header content-type"},
		"header missing":    {exchange(201, `{"score":1.0}`, "content-type", "application/json"), "header vary"},
		"header extra":      {exchange(201, `{"score":1.0}`, "content-type", "application/json", "vary", "Origin", "x-powered-by", "go"), "header x-powered-by"},
		"header duplicated": {exchange(201, `{"score":1.0}`, "content-type", "application/json", "vary", "Origin", "vary", "Origin"), "header vary"},
		"content encoding":  {exchange(201, `{"score":1.0}`, "content-type", "application/json", "vary", "Origin", "content-encoding", "gzip"), "header content-encoding"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			diffs := Step(base, tc.cand)
			if len(diffs) != 1 || !strings.HasPrefix(diffs[0].Part, tc.part) {
				t.Fatalf("diffs = %v, want exactly one %q", diffs, tc.part)
			}
		})
	}
}

func TestBodyDifferencePointsAtFirstByte(t *testing.T) {
	ref := `{"items":[{"name":"cà phê","amount_vnd":45000,"score":1.0}]}`
	cand := `{"items":[{"name":"cà phê","amount_vnd":45000,"score":1}]}`
	diffs := Step(exchange(200, ref), exchange(200, cand))
	if len(diffs) != 1 {
		t.Fatalf("diffs = %v", diffs)
	}
	// Bytes, not runes: "cà phê" is 8 bytes of UTF-8, so the "1.0" vs "1"
	// difference starts at byte 57.
	if !strings.Contains(diffs[0].Part, "first difference at byte 57") {
		t.Fatalf("part = %s", diffs[0].Part)
	}
}

func TestOnlyDateAndServerAreVolatile(t *testing.T) {
	if len(Volatile) != 2 || !Volatile["date"] || !Volatile["server"] {
		t.Fatalf("volatile headers grew: %v — every entry hides a difference", Volatile)
	}
}

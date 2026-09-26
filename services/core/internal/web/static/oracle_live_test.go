//go:build oracle

package static

// Go's answers compared against a LIVE pinned Python image, request by request.
// The table in static_test.go is a transcription of what was measured, and a
// transcription can be wrong in the same direction twice: this test asks Python
// itself, so a mistake in the table cannot hide behind agreeing with itself.
//
// Run from services/core against a reference stack that is already up:
//
//	STATIC_ORACLE_URL=http://127.0.0.1:45587 \
//	  go test -tags oracle -run TestLive -v ./internal/web/static/
//
// The two headers ADR-0029 §2.4 accepts as divergent (etag and last-modified,
// whose VALUES are build metadata on Python's side) are compared by SHAPE, and
// any conditional request is sent each side ITS OWN validator -- which is how
// the parity scenario binds them too. Everything else is compared exactly.

import (
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"
)

var quotedMD5 = regexp.MustCompile(`^"[0-9a-f]{32}"$`)

func TestLiveStaticContractMatchesPython(t *testing.T) {
	base := os.Getenv("STATIC_ORACLE_URL")
	if base == "" {
		t.Fatal("STATIC_ORACLE_URL is unset; a skip here would prove nothing")
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	ask := func(method, path string, header http.Header) (int, http.Header, []byte) {
		req, err := http.NewRequest(method, base+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		for name, values := range header {
			for _, v := range values {
				req.Header.Add(name, v)
			}
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		return resp.StatusCode, resp.Header, body
	}

	// Each side's own validators, so a conditional request is conditional on
	// something that side actually issued.
	_, pyHead, _ := ask("GET", "/static/guest.css", nil)
	pyETag, pyLastMod := pyHead.Get("Etag"), pyHead.Get("Last-Modified")
	goFirst := Serve("GET", "/static/guest.css", nil)
	goETag, goLastMod := headerOf(goFirst, "etag"), headerOf(goFirst, "last-modified")
	if !quotedMD5.MatchString(pyETag) || !quotedMD5.MatchString(goETag) {
		t.Fatalf("etag shapes differ: python %q, go %q", pyETag, goETag)
	}
	if pyETag == goETag {
		t.Logf("note: the etags are equal (%s); the accepted divergence is a value one, so this is fine", pyETag)
	}

	hdr := func(pairs ...string) http.Header {
		h := http.Header{}
		for i := 0; i+1 < len(pairs); i += 2 {
			h.Set(pairs[i], pairs[i+1])
		}
		return h
	}
	cases := []struct {
		name         string
		method, path string
		py, golang   http.Header
	}{
		{"a css file", "GET", "/static/guest.css", nil, nil},
		{"the js file", "GET", "/static/guest.js", nil, nil},
		{"the design system css", "GET", "/static/design_system.css", nil, nil},
		{"HEAD", "HEAD", "/static/guest.css", nil, nil},
		{"if-none-match matching", "GET", "/static/guest.css",
			hdr("If-None-Match", pyETag), hdr("If-None-Match", goETag)},
		{"if-modified-since matching", "GET", "/static/guest.css",
			hdr("If-Modified-Since", pyLastMod), hdr("If-Modified-Since", goLastMod)},
		{"if-none-match stale", "GET", "/static/guest.css",
			hdr("If-None-Match", `"stale"`), hdr("If-None-Match", `"stale"`)},
		{"a byte range", "GET", "/static/guest.css", hdr("Range", "bytes=0-9"), hdr("Range", "bytes=0-9")},
		{"a suffix range", "GET", "/static/guest.css", hdr("Range", "bytes=-10"), hdr("Range", "bytes=-10")},
		{"if-range matching", "GET", "/static/guest.css",
			hdr("If-Range", pyETag, "Range", "bytes=0-9"), hdr("If-Range", goETag, "Range", "bytes=0-9")},
		{"if-range stale", "GET", "/static/guest.css",
			hdr("If-Range", `"stale"`, "Range", "bytes=0-9"), hdr("If-Range", `"stale"`, "Range", "bytes=0-9")},
		{"a range past the end", "GET", "/static/guest.css", hdr("Range", "bytes=99999-100000"), hdr("Range", "bytes=99999-100000")}, // repo-guard: allow=long-number reason=http-byte-range
		{"a missing file", "GET", "/static/khong-co.css", nil, nil},
		{"HEAD a missing file", "HEAD", "/static/khong-co.css", nil, nil},
		{"the directory", "GET", "/static/", nil, nil},
		{"a nested name", "GET", "/static/a/b/c.css", nil, nil},
		{"an escaped traversal", "GET", "/static/..%2f..%2fmain.py", nil, nil},
		{"POST", "POST", "/static/guest.css", nil, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, header, body := ask(tc.method, tc.path, tc.py)
			got := Serve(tc.method, tc.path, tc.golang)

			if got.Status != status {
				t.Fatalf("status: python %d, go %d", status, got.Status)
			}
			for _, h := range got.Headers {
				name, value := h[0], h[1]
				python := header.Get(name)
				switch name {
				case "etag", "last-modified":
					// Value accepted as divergent; presence and shape are not.
					if python == "" {
						t.Errorf("go sent %s, python sent none", name)
					}
					if name == "etag" && !quotedMD5.MatchString(value) {
						t.Errorf("etag %q does not keep python's shape", value)
					}
				default:
					if python != value {
						t.Errorf("header %s: python %q, go %q", name, python, value)
					}
				}
			}
			// And no header Python sent is missing from Go, date and server
			// aside -- a header dropped is as much a divergence as one added.
			for name := range header {
				lower := strings.ToLower(name)
				if lower == "date" || lower == "server" {
					continue
				}
				if headerOf(got, lower) == "" {
					t.Errorf("python sent %s: %q, go sent none", lower, header.Get(name))
				}
			}
			if tc.method != "HEAD" && string(got.Body) != string(body) {
				t.Errorf("body: python %d bytes, go %d", len(body), len(got.Body))
			}
		})
	}
}

func headerOf(r Response, name string) string {
	for _, h := range r.Headers {
		if h[0] == name {
			return h[1]
		}
	}
	return ""
}

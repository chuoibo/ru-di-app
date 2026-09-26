package static

import (
	"crypto/md5"
	"encoding/hex"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The table below is the contract MEASURED against the pinned Python image on
// 2026-09-22 (docs/migration/routes/static/MOUNT-static.md), not read off
// Starlette's source and not invented here. Where a row's expectation is Go's
// own choice rather than Python's, the row says so.
func TestTheMeasuredPythonContract(t *testing.T) {
	const css = "guest.css"
	const size = 10184

	etag := entries[css].etag
	lastMod := lastModified.Format(httpDateForm)

	hdr := func(pairs ...string) http.Header {
		h := http.Header{}
		for i := 0; i+1 < len(pairs); i += 2 {
			h.Set(pairs[i], pairs[i+1])
		}
		return h
	}
	cases := []struct {
		name    string
		method  string
		path    string
		header  http.Header
		status  int
		headers string // "name=value" joined by "|", in order
		body    string // "" means empty; "<full>" means the whole file
	}{
		{"GET a css file", "GET", "/static/guest.css", nil, 200,
			"content-type=text/css; charset=utf-8|accept-ranges=bytes|content-length=10184|last-modified=" + lastMod + "|etag=" + etag, "<full>"},
		{"GET the js file", "GET", "/static/guest.js", nil, 200,
			"content-type=text/javascript; charset=utf-8|accept-ranges=bytes|content-length=2224|last-modified=" + lastMod + "|etag=" + entries["guest.js"].etag, "<full>"},
		{"HEAD sends the headers and no body", "HEAD", "/static/guest.css", nil, 200,
			"content-type=text/css; charset=utf-8|accept-ranges=bytes|content-length=10184|last-modified=" + lastMod + "|etag=" + etag, ""},

		// The ETag ALONE. A 304 that also carried Last-Modified would be the
		// tidy-looking version and would be a divergence.
		{"if-none-match matching", "GET", "/static/guest.css", hdr("If-None-Match", etag), 304, "etag=" + etag, ""},
		{"if-modified-since matching", "GET", "/static/guest.css", hdr("If-Modified-Since", lastMod), 304, "etag=" + etag, ""},
		{"if-none-match not matching", "GET", "/static/guest.css", hdr("If-None-Match", `"stale"`), 200,
			"content-type=text/css; charset=utf-8|accept-ranges=bytes|content-length=10184|last-modified=" + lastMod + "|etag=" + etag, "<full>"},

		{"a byte range", "GET", "/static/guest.css", hdr("Range", "bytes=0-9"), 206,
			"content-type=text/css; charset=utf-8|accept-ranges=bytes|content-length=10|last-modified=" + lastMod + "|etag=" + etag + "|content-range=bytes 0-9/10184", "<first10>"},
		{"a suffix range", "GET", "/static/guest.css", hdr("Range", "bytes=-10"), 206,
			"content-type=text/css; charset=utf-8|accept-ranges=bytes|content-length=10|last-modified=" + lastMod + "|etag=" + etag + "|content-range=bytes 10174-10183/10184", "<last10>"}, // repo-guard: allow=long-number reason=http-byte-range
		{"if-range matching keeps the range", "GET", "/static/guest.css", hdr("If-Range", etag, "Range", "bytes=0-9"), 206,
			"content-type=text/css; charset=utf-8|accept-ranges=bytes|content-length=10|last-modified=" + lastMod + "|etag=" + etag + "|content-range=bytes 0-9/10184", "<first10>"},
		{"if-range stale serves the whole file", "GET", "/static/guest.css", hdr("If-Range", `"stale"`, "Range", "bytes=0-9"), 200,
			"content-type=text/css; charset=utf-8|accept-ranges=bytes|content-length=10184|last-modified=" + lastMod + "|etag=" + etag, "<full>"},

		// text/plain and an EMPTY body: not the JSON the 404 and 405 on this
		// same mount use, and not http.Error's sentence.
		{"a range past the end", "GET", "/static/guest.css", hdr("Range", "bytes=99999-100000"), 416, // repo-guard: allow=long-number reason=http-byte-range
			"content-range=*/10184|content-length=0|content-type=text/plain; charset=utf-8", ""},

		{"a file that is not there", "GET", "/static/khong-co.css", nil, 404, "content-length=22|content-type=application/json", `{"detail":"Not Found"}`},
		{"HEAD a file that is not there", "HEAD", "/static/khong-co.css", nil, 404, "content-length=22|content-type=application/json", `{"detail":"Not Found"}`},
		{"the directory itself", "GET", "/static/", nil, 404, "content-length=22|content-type=application/json", `{"detail":"Not Found"}`},
		{"a nested name", "GET", "/static/a/b/c.css", nil, 404, "content-length=22|content-type=application/json", `{"detail":"Not Found"}`},
		{"an escaped traversal", "GET", "/static/..%2f..%2fmain.py", nil, 404, "content-length=22|content-type=application/json", `{"detail":"Not Found"}`},

		// No Allow header. The API router's 405 has one; this mount's has not.
		{"POST is refused", "POST", "/static/guest.css", nil, 405, "content-length=31|content-type=application/json", `{"detail":"Method Not Allowed"}`},

		// GO'S OWN CHOICE, not Python's: Python answers a multi-range request
		// 206 multipart/byteranges with a RANDOM boundary, which no byte
		// comparison could ever match on either side. Ignoring Range and
		// sending the whole file is a legal answer and a deterministic one.
		{"a multi-range is served whole", "GET", "/static/guest.css", hdr("Range", "bytes=0-1,5-6"), 200,
			"content-type=text/css; charset=utf-8|accept-ranges=bytes|content-length=10184|last-modified=" + lastMod + "|etag=" + etag, "<full>"},
	}

	full := string(entries[css].body)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Serve(tc.method, tc.path, tc.header)
			if got.Status != tc.status {
				t.Errorf("status %d, want %d", got.Status, tc.status)
			}
			var flat []string
			for _, h := range got.Headers {
				flat = append(flat, h[0]+"="+h[1])
			}
			if joined := strings.Join(flat, "|"); joined != tc.headers {
				t.Errorf("headers\n got %s\nwant %s", joined, tc.headers)
			}
			// The whole-file expectations resolve against the file the case
			// actually asked for, not against a single hardcoded one.
			body := full
			if name := strings.TrimPrefix(tc.path, "/static/"); entries[name].body != nil {
				body = string(entries[name].body)
			}
			want := tc.body
			switch want {
			case "<full>":
				want = body
			case "<first10>":
				want = body[:10]
			case "<last10>":
				want = body[size-10:]
			}
			if string(got.Body) != want {
				t.Errorf("body %d bytes, want %d", len(got.Body), len(want))
			}
		})
	}
}

// The embedded copies are what the distroless image serves; they must be the
// files Python serves, or a CSS edit on one side is invisible on the other for
// as long as both exist.
func TestEmbeddedFilesMatchPythonTree(t *testing.T) {
	source := filepath.Join("..", "..", "..", "..", "api", "app", "web", "static")
	for _, name := range Names {
		want, err := os.ReadFile(filepath.Join(source, name))
		if err != nil {
			t.Fatalf("reading the Python file: %v", err)
		}
		got, err := files.ReadFile("files/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if string(want) != string(got) {
			t.Errorf("files/%s differs from services/api/app/web/static/%s; copy it again", name, name)
		}
	}
	entriesOnDisk, err := os.ReadDir(source)
	if err != nil {
		t.Fatal(err)
	}
	if len(entriesOnDisk) != len(Names) {
		t.Errorf("Python serves %d files, Go embeds %d", len(entriesOnDisk), len(Names))
	}
}

// The value is Go's, but the SHAPE is Python's: a quoted 32-character hex
// digest. A client that parses the header must not notice the change.
func TestTheETagKeepsPythonsShapeAndTracksContent(t *testing.T) {
	for _, name := range Names {
		etag := entries[name].etag
		if len(etag) != 34 || etag[0] != '"' || etag[33] != '"' {
			t.Fatalf("%s: etag %q is not a quoted 32-hex digest", name, etag)
		}
		if _, err := hex.DecodeString(etag[1:33]); err != nil {
			t.Fatalf("%s: etag %q is not hex", name, etag)
		}
		sum := md5.Sum(entries[name].body)
		if want := `"` + hex.EncodeToString(sum[:]) + `"`; etag != want {
			t.Fatalf("%s: etag %s does not track the content", name, etag)
		}
	}
	// Distinct content must give distinct validators -- the property Python's
	// mtime-derived ETag does NOT have, which is the whole reason for the change.
	seen := map[string]string{}
	for _, name := range Names {
		if other, clash := seen[entries[name].etag]; clash {
			t.Fatalf("%s and %s share an etag", name, other)
		}
		seen[entries[name].etag] = name
	}
}

func TestOnlyTheMountsOwnPathsAreHandled(t *testing.T) {
	for _, in := range []string{"/static/", "/static/guest.css", "/static/a/b"} {
		if !Handles(in) {
			t.Errorf("%s should be handled", in)
		}
	}
	// A neighbour that merely starts with the same letters is not beneath it.
	for _, out := range []string{"/staticky/x.css", "/", "/g/abc", "/statics", "/static"} {
		if Handles(out) {
			t.Errorf("%s should not be handled", out)
		}
	}
}

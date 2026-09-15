package guest

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func rawPath(r *http.Request) string { return r.URL.Path }

func TestGuestAnswersAreStampedByAssignment(t *testing.T) {
	handler := Middleware(rawPath, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "max-age=60")
		w.Header().Add("Cache-Control", "public")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(404)
		_, _ = io.WriteString(w, `{"code":"guest_link_not_found"}`)
	}))
	cases := map[string]bool{"/g": true, "/g/abc/confirm": true, "/goals": false, "/contexts": false}
	for path, guest := range cases {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		header := rec.Result().Header
		if guest {
			if got := header.Values("Cache-Control"); len(got) != 1 || got[0] != "no-store" {
				t.Fatalf("%s: Cache-Control %v, want the route's values replaced", path, got)
			}
			if header.Get("Referrer-Policy") != "no-referrer" || header.Get("X-Robots-Tag") != "noindex, nofollow" {
				t.Fatalf("%s: %v", path, header)
			}
		} else if header.Get("Referrer-Policy") != "" || len(header.Values("Cache-Control")) != 2 {
			t.Fatalf("%s: stamped outside /g: %v", path, header)
		}
		if header.Get("Content-Type") != "application/json" || rec.Code != 404 {
			t.Fatalf("%s: route's own answer changed: %d %v", path, rec.Code, header)
		}
	}
}

func TestAWriteWithoutWriteHeaderAndAFlushAreStampedToo(t *testing.T) {
	for name, respond := range map[string]func(http.ResponseWriter){
		"write": func(w http.ResponseWriter) { _, _ = io.WriteString(w, "page") },
		"flush": func(w http.ResponseWriter) { w.(http.Flusher).Flush() },
	} {
		rec := httptest.NewRecorder()
		Middleware(rawPath, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			respond(w)
		})).ServeHTTP(rec, httptest.NewRequest("GET", "/g/token", nil))
		if rec.Result().Header.Get("X-Robots-Tag") != "noindex, nofollow" {
			t.Fatalf("%s: not stamped: %v", name, rec.Result().Header)
		}
	}
}

// A handler that returns without writing is answered by net/http's implicit
// 200, which never calls WriteHeader on the wrapper. Python stamps every
// http.response.start, so that answer must carry the headers on the wire too.
func TestTheImplicit200OfAHandlerThatWritesNothingIsStamped(t *testing.T) {
	server := httptest.NewServer(Middleware(rawPath, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public")
	})))
	defer server.Close()
	for path, guest := range map[string]bool{"/g/token": true, "/goals": false} {
		response, err := http.Get(server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		_ = response.Body.Close()
		if response.StatusCode != 200 {
			t.Fatalf("%s: status %d", path, response.StatusCode)
		}
		header := response.Header
		if !guest {
			if header.Get("Cache-Control") != "public" || header.Get("X-Robots-Tag") != "" {
				t.Fatalf("%s: stamped outside /g: %v", path, header)
			}
			continue
		}
		for _, pair := range [][2]string{{"Cache-Control", "no-store"}, {"Referrer-Policy", "no-referrer"}, {"X-Robots-Tag", "noindex, nofollow"}} {
			if got := header.Values(pair[0]); len(got) != 1 || got[0] != pair[1] {
				t.Fatalf("%s: %s is %v on the implicit 200", path, pair[0], got)
			}
		}
	}
}

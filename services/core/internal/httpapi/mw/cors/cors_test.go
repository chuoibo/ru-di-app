package cors

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"testing"
)

type golden struct {
	Config         string      `json:"config"`
	RawOrigins     *string     `json:"raw_origins"`
	Case           string      `json:"case"`
	Method         string      `json:"method"`
	Path           string      `json:"path"`
	RequestHeaders [][2]string `json:"request_headers"`
	Status         int         `json:"status"`
	Headers        [][2]string `json:"headers"`
	Body           string      `json:"body"`
}

// latin1 turns a golden string (runes <= 0xff, as ASGI decodes header bytes)
// back into the raw bytes that arrive on the wire.
func latin1(s string) string {
	out := make([]byte, 0, len(s))
	for _, r := range s {
		out = append(out, byte(r))
	}
	return string(out)
}

// inner is the same ASGI app the golden script drives.
func inner(w http.ResponseWriter, r *http.Request) {
	h := w.Header()
	h.Set("Content-Type", "application/json")
	h.Set("Content-Length", "2")
	switch r.URL.Path {
	case "/with-vary":
		h.Add("Vary", "Accept-Encoding")
	case "/with-two-vary":
		h.Add("Vary", "Accept-Encoding")
		h.Add("Vary", "Cookie")
	case "/with-acao":
		h.Set("Access-Control-Allow-Origin", "https://stale.example")
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("{}"))
}

func valuesByName(pairs [][2]string) map[string][]string {
	out := map[string][]string{}
	for _, pair := range pairs {
		name := strings.ToLower(pair[0])
		out[name] = append(out[name], latin1(pair[1]))
	}
	return out
}

func TestMatchesStarletteMiddleware(t *testing.T) {
	data, err := os.ReadFile("testdata/starlette_cors.json")
	if err != nil {
		t.Fatalf("goldens missing — render them with scripts/render_cors_goldens.py: %v", err)
	}
	var cases []golden
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) < 60 {
		t.Fatalf("only %d golden cases; the file looks truncated", len(cases))
	}
	for _, c := range cases {
		t.Run(c.Config+"/"+c.Case, func(t *testing.T) {
			raw, set := "", c.RawOrigins != nil
			if set {
				raw = *c.RawOrigins
			}
			handler := New(raw, set).Middleware(http.HandlerFunc(inner))
			req := httptest.NewRequest(c.Method, c.Path, nil)
			for _, pair := range c.RequestHeaders {
				req.Header[http.CanonicalHeaderKey(pair[0])] = append(req.Header[http.CanonicalHeaderKey(pair[0])], latin1(pair[1]))
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			body, _ := io.ReadAll(rec.Result().Body)

			if rec.Code != c.Status {
				t.Errorf("status = %d, want %d", rec.Code, c.Status)
			}
			if string(body) != latin1(c.Body) {
				t.Errorf("body = %q, want %q", body, c.Body)
			}
			got := map[string][]string{}
			for name, values := range rec.Result().Header {
				got[strings.ToLower(name)] = values
			}
			want := valuesByName(c.Headers)
			names := map[string]bool{}
			for name := range got {
				names[name] = true
			}
			for name := range want {
				names[name] = true
			}
			sorted := make([]string, 0, len(names))
			for name := range names {
				sorted = append(sorted, name)
			}
			sort.Strings(sorted)
			for _, name := range sorted {
				if strings.Join(got[name], "\x00") != strings.Join(want[name], "\x00") {
					t.Errorf("header %s = %q, want %q", name, got[name], want[name])
				}
			}
		})
	}
}

func TestPyStrip(t *testing.T) {
	cases := map[string]string{
		" a ":             "a",
		"\x1ca\x1f":       "a",
		"\xa0a\x85":       "a",
		"\t\n\v\f\r a \r": "a",
		"a b":             "a b",
		"":                "",
	}
	for in, want := range cases {
		if got := pyStrip(in); got != want {
			t.Errorf("pyStrip(%q) = %q, want %q", in, got, want)
		}
	}
}

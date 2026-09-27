package gzipjson

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func serve(t *testing.T, contentType, acceptEncoding string, body string) *http.Response {
	t.Helper()
	handler := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentType)
		_, _ = io.WriteString(w, body)
	}))
	req := httptest.NewRequest(http.MethodGet, "/places", nil)
	if acceptEncoding != "" {
		req.Header.Set("Accept-Encoding", acceptEncoding)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec.Result()
}

func TestJSONIsCompressedOnlyWhenAsked(t *testing.T) {
	body := `{"places":[` + strings.Repeat(`{"name":"Phở"},`, 500) + `{}]}`

	plain := serve(t, "application/json", "", body)
	got, _ := io.ReadAll(plain.Body)
	if plain.Header.Get("Content-Encoding") != "" || string(got) != body {
		t.Fatal("a client that did not ask got something other than the exact body")
	}

	zipped := serve(t, "application/json", "gzip, deflate", body)
	if zipped.Header.Get("Content-Encoding") != "gzip" {
		t.Fatal("asked for gzip, got none")
	}
	raw, _ := io.ReadAll(zipped.Body)
	r, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	inflated, _ := io.ReadAll(r)
	if string(inflated) != body {
		t.Fatal("inflated body differs")
	}
	if len(raw) >= len(body)/4 {
		t.Errorf("compressed %d of %d bytes", len(raw), len(body))
	}
}

func TestImagesAndRefusalsAreLeftAlone(t *testing.T) {
	image := serve(t, "image/jpeg", "gzip", "\xff\xd8jpeg")
	if image.Header.Get("Content-Encoding") != "" {
		t.Error("an image was gzipped")
	}
	refused := serve(t, "application/json", "gzip;q=0", `{}`)
	if refused.Header.Get("Content-Encoding") != "" {
		t.Error("q=0 means no")
	}
}

package canary

import (
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
)

func upstream(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			w.Header().Set("Location", "http://"+r.Host+"/there")
			w.WriteHeader(http.StatusTemporaryRedirect)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Vary", "Origin")
		cursor := base64.RawURLEncoding.EncodeToString([]byte("2026-09-14T10:00:00.120000+00:00|aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"))
		_, _ = w.Write([]byte(`{"id":"a","score":1.0,"created_at":"2026-09-14T10:00:00.123456Z","cursor":"` + cursor + `"}`))
	}))
	t.Cleanup(server.Close)
	return server
}

func fetch(t *testing.T, base, path string) (*http.Response, string) {
	t.Helper()
	client := &http.Client{
		Transport:     &http.Transport{DisableCompression: true},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	resp, err := client.Get(base + path)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp, string(body)
}

// headerValues counts names and values, so a duplicated value shows.
func headerValues(h http.Header) int {
	n := 0
	for _, values := range h {
		n += 1 + len(values)
	}
	return n
}

func TestEveryModeChangesWhatItClaims(t *testing.T) {
	target, _ := url.Parse(upstream(t).URL)
	baseline, baseBody := fetch(t, target.String(), "/json")

	for _, mode := range Modes() {
		t.Run(mode.Name, func(t *testing.T) {
			var applied atomic.Int64
			front := httptest.NewServer(Proxy(target, mode, &applied))
			defer front.Close()
			path := "/json"
			if mode.Name == "redirect-followed" {
				path = "/redirect"
			}
			resp, body := fetch(t, front.URL, path)
			changed := resp.StatusCode != baseline.StatusCode || body != baseBody ||
				headerValues(resp.Header) != headerValues(baseline.Header) ||
				resp.Header.Get("Content-Encoding") != ""
			if mode.Name == "redirect-followed" {
				changed = resp.StatusCode == http.StatusOK && resp.Header.Get("Location") == ""
			}
			if mode.Name == "identity" {
				if changed || applied.Load() != 0 {
					t.Fatalf("identity changed the response: %d %q", resp.StatusCode, body)
				}
				return
			}
			if !changed || applied.Load() == 0 {
				t.Fatalf("mode did not damage the response (applied=%d): %d %q", applied.Load(), resp.StatusCode, body)
			}
		})
	}
}

package loimoi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The refusals that need no database: the path it owns, its two methods, a
// malformed id before any read, and a server with no database. What the
// routes read and write is proven on PostgreSQL (postgres_test.go).
func TestHandlerWithoutDatabase(t *testing.T) {
	if !Matches("/contexts/0b1b8d6e-0c5d-4a4f-9a52-2b1c3d4e5f60/invitation") || Matches("/contexts/x/members") || Matches("/contexts/x/invitation/y") || Matches("/people/x/invitation") {
		t.Fatal("Matches must own exactly /contexts/{id}/invitation")
	}
	h := New(nil, "prod")
	cases := []struct {
		method, path string
		status       int
		code         string
	}{
		{"POST", "/contexts/0b1b8d6e-0c5d-4a4f-9a52-2b1c3d4e5f60/invitation", 405, "method_not_allowed"},
		{"GET", "/contexts/khong-phai-uuid/invitation", 422, "invalid_context_id"},
		{"DELETE", "/contexts/khong-phai-uuid/invitation", 422, "invalid_context_id"},
		{"GET", "/contexts/0b1b8d6e-0c5d-4a4f-9a52-2b1c3d4e5f60/invitation", 503, "groups_unavailable"},
		{"DELETE", "/contexts/0b1b8d6e-0c5d-4a4f-9a52-2b1c3d4e5f60/invitation", 503, "groups_unavailable"},
	}
	for _, c := range cases {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(c.method, c.path, nil))
		var body apiError
		_ = json.Unmarshal(w.Body.Bytes(), &body)
		if w.Code != c.status || body.Code != c.code {
			t.Errorf("%s %s: %d %q, want %d %q", c.method, c.path, w.Code, body.Code, c.status, c.code)
		}
		if w.Header().Get("Cache-Control") != "private, no-store" {
			t.Errorf("%s %s: an invitation must not be cached", c.method, c.path)
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/contexts/x/members", nil))
	if w.Code != 404 {
		t.Fatalf("a path it does not own: %d", w.Code)
	}
	if len(RouteIDs()) != 2 {
		t.Fatal("two routes for the manifest")
	}
}

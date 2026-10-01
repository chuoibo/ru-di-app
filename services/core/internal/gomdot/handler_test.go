package gomdot

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The refusals that need no database: the path it owns, the one method, a
// malformed id before any read, and a server with no ledger. The count itself
// is proven on PostgreSQL (repo/unbatched_postgres_test.go).
func TestHandlerWithoutDatabase(t *testing.T) {
	if !Matches("/contexts/0b1b8d6e-0c5d-4a4f-9a52-2b1c3d4e5f60/unbatched-expenses") || Matches("/contexts/x/batches") || Matches("/contexts/x/unbatched-expenses/y") {
		t.Fatal("Matches must own exactly /contexts/{id}/unbatched-expenses")
	}
	h := New(nil, "prod")
	cases := []struct {
		method, path string
		status       int
		code         string
	}{
		{"POST", "/contexts/0b1b8d6e-0c5d-4a4f-9a52-2b1c3d4e5f60/unbatched-expenses", 405, "method_not_allowed"},
		{"GET", "/contexts/khong-phai-uuid/unbatched-expenses", 422, "invalid_context_id"},
		{"GET", "/contexts/0b1b8d6e-0c5d-4a4f-9a52-2b1c3d4e5f60/unbatched-expenses", 503, "ledger_unavailable"},
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
			t.Errorf("%s %s: a ledger read must not be cached", c.method, c.path)
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/contexts/x/batches", nil))
	if w.Code != 404 {
		t.Fatalf("a path it does not own: %d", w.Code)
	}
}

type soGia struct {
	thanhVien map[string]bool
	n         int
	daDem     bool
}

func (s *soGia) IsMember(_ context.Context, contextID, personID string) (bool, error) {
	return s.thanhVien[contextID+"/"+personID], nil
}

func (s *soGia) CountUnbatchedExpenses(context.Context, string) (int, error) {
	s.daDem = true
	return s.n, nil
}

// A stranger is refused before the ledger is read at all; a member gets the
// count. The SQL of both reads is proven on PostgreSQL in package repo.
func TestCountForReadsOnlyForMembers(t *testing.T) {
	const nhom, an, la = "g", "an", "la"
	so := &soGia{thanhVien: map[string]bool{nhom + "/" + an: true}, n: 2}
	_, err := countFor(context.Background(), so, nhom, la)
	var d *denial
	if !errors.As(err, &d) || d.status != 403 || d.code != "is_group_member" {
		t.Fatalf("a stranger: %v", err)
	}
	if so.daDem {
		t.Fatal("the ledger was counted for someone outside the group")
	}
	if n, err := countFor(context.Background(), so, nhom, an); err != nil || n != 2 {
		t.Fatalf("a member: %d %v", n, err)
	}
}

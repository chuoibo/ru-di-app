package featureroute

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestMuxRemembersPatternsAndStillServes(t *testing.T) {
	m := NewMux()
	m.HandleFunc("GET /a/{x}", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusTeapot) })
	m.HandleFunc("POST /b", func(w http.ResponseWriter, r *http.Request) {})
	if got, want := m.Patterns(), []string{"GET /a/{x}", "POST /b"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("patterns = %v, want %v", got, want)
	}
	rec := httptest.NewRecorder()
	m.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/a/1", nil))
	if rec.Code != http.StatusTeapot {
		t.Fatalf("served %d, want 418", rec.Code)
	}
	p := m.Patterns()
	p[0] = "mutated"
	if m.Patterns()[0] != "GET /a/{x}" {
		t.Fatal("Patterns must return a copy")
	}
}

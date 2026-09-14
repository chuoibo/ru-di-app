package dispatch

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mobile/services/core/internal/httpapi/mw/cors"
	"mobile/services/core/internal/httpapi/mw/servererror"
	"mobile/services/core/internal/httpapi/router"
	"mobile/services/core/ownership"
)

func row(order int, id, method, path, owner string) ownership.Route {
	return ownership.Route{ID: id, Order: order, Kind: "route", Method: method, Path: path, Owner: owner}
}

// The registration order mirrors the traps the manifest records: /people/me
// before /people/{person_id}, and a Python route that shadows a Go one.
var manifest = []ownership.Route{
	row(1, "GET /people/me", "GET", "/people/me", "go"),
	row(2, "GET /people/{person_id}", "GET", "/people/{person_id}", "python"),
	row(3, "GET /contexts/{context_id}/recap", "GET", "/contexts/{context_id}/recap", "go"),
	row(4, "GET /contexts/{context_id}/{view}", "GET", "/contexts/{context_id}/{view}", "python"),
	row(5, "GET /contexts/{context_id}/map", "GET", "/contexts/{context_id}/map", "go"),
	row(6, "PUT /people/me/interests", "PUT", "/people/me/interests", "go"),
	row(7, "GET /g/{token}", "GET", "/g/{token}", "go"),
	row(8, "GET /crash", "GET", "/crash", "go"),
	// No real route declares OPTIONS; this one makes the preflight rule testable.
	row(9, "OPTIONS /people/me/interests", "OPTIONS", "/people/me/interests", "go"),
}

type recorder struct {
	who      string
	scope    Scope
	released bool
}

// fakeIdempotency refuses a request whose key is "bad" on its own, as the real
// layer refuses a malformed key, and records a release when the handler panics.
func fakeIdempotency(seen *recorder) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Idempotency-Key") == "bad" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(422)
				_, _ = io.WriteString(w, `{"code": "invalid_idempotency_key"}`)
				return
			}
			defer func() {
				if v := recover(); v != nil {
					seen.released = true
					panic(v)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func build(t *testing.T, served []ownership.Route) (http.Handler, *recorder) {
	t.Helper()
	rt, err := router.New(manifest)
	if err != nil {
		t.Fatal(err)
	}
	seen := &recorder{}
	goHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen.who = "go"
		seen.scope, _ = FromRequest(r)
		if seen.scope.RouteID == "GET /crash" {
			servererror.Raise(errors.New("boom"))
		}
		w.Header().Set("Cache-Control", "max-age=60")
		_, _ = io.WriteString(w, "from go")
	})
	handlers := map[string]http.Handler{}
	for _, route := range manifest {
		if route.Owner == "go" {
			handlers[route.ID] = goHandler
		}
	}
	python := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen.who = "python"
		_, _ = io.WriteString(w, "from python")
	})
	front, err := New(Options{
		Router: rt, Served: served, Handlers: handlers, Python: python,
		CORS:        cors.New("http://allowed.test", true),
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		Idempotency: fakeIdempotency(seen),
	})
	if err != nil {
		t.Fatal(err)
	}
	return front, seen
}

func goOwned() []ownership.Route {
	var out []ownership.Route
	for _, route := range manifest {
		if route.Owner == "go" {
			out = append(out, route)
		}
	}
	return out
}

// serve runs one request; a crash's closing abort is recovered here the way
// net/http recovers it.
func serve(h http.Handler, req *http.Request) (rec *httptest.ResponseRecorder) {
	rec = httptest.NewRecorder()
	defer func() {
		if v := recover(); v != nil && v != http.ErrAbortHandler {
			panic(v)
		}
	}()
	h.ServeHTTP(rec, req)
	return rec
}

func TestOnlyAFullMatchOnAGoRouteIsServedInGo(t *testing.T) {
	front, seen := build(t, goOwned())
	cases := []struct {
		method, target, who, route string
		params                     map[string]string
	}{
		{"GET", "/people/me", "go", "GET /people/me", nil},
		{"GET", "/people/someone", "python", "", nil},
		{"GET", "/contexts/abc/recap", "go", "GET /contexts/{context_id}/recap", map[string]string{"context_id": "abc"}},
		// Decoded once before matching, as uvicorn builds scope["path"].
		{"GET", "/contexts/%61bc/recap?x=1", "go", "GET /contexts/{context_id}/recap", map[string]string{"context_id": "abc"}},
		// %2F decodes to a slash, and [^/]+ then no longer matches a segment.
		{"GET", "/contexts/a%2Fb/recap", "python", "", nil},
		// Python's /contexts/{context_id}/{view} is registered before Go's map.
		{"GET", "/contexts/abc/map", "python", "", nil},
		// PARTIAL (405), trailing-slash redirect, 404: Python answers.
		{"POST", "/contexts/abc/recap", "python", "", nil},
		{"GET", "/contexts/abc/recap/", "python", "", nil},
		{"GET", "/nowhere", "python", "", nil},
		{"HEAD", "/people/me", "python", "", nil},
	}
	for _, tc := range cases {
		seen.who, seen.scope = "", Scope{}
		rec := serve(front, httptest.NewRequest(tc.method, tc.target, nil))
		if seen.who != tc.who {
			t.Fatalf("%s %s served by %q, want %q (body %q)", tc.method, tc.target, seen.who, tc.who, rec.Body)
		}
		if tc.who != "go" {
			continue
		}
		if seen.scope.RouteID != tc.route || len(seen.scope.Params) != len(tc.params) {
			t.Fatalf("%s %s scope = %+v", tc.method, tc.target, seen.scope)
		}
		for name, value := range tc.params {
			if seen.scope.Params[name] != value {
				t.Fatalf("%s %s param %s = %q", tc.method, tc.target, name, seen.scope.Params[name])
			}
		}
	}
}

func TestForcedToPythonAndPreflightsNeverReachGo(t *testing.T) {
	front, seen := build(t, nil)
	serve(front, httptest.NewRequest("GET", "/people/me", nil))
	if seen.who != "python" {
		t.Fatalf("no served routes, but %q answered", seen.who)
	}

	front, seen = build(t, goOwned())
	req := httptest.NewRequest("OPTIONS", "/people/me/interests", nil)
	req.Header.Set("Origin", "http://allowed.test")
	req.Header.Set("Access-Control-Request-Method", "PUT")
	serve(front, req)
	if seen.who != "python" {
		t.Fatalf("preflight answered by %q", seen.who)
	}
	// Control: the same OPTIONS without Access-Control-Request-Method is an
	// ordinary FULL match on the Go route.
	plain := httptest.NewRequest("OPTIONS", "/people/me/interests", nil)
	plain.Header.Set("Origin", "http://allowed.test")
	serve(front, plain)
	if seen.who != "go" {
		t.Fatalf("plain OPTIONS answered by %q", seen.who)
	}
}

func TestGoRoutesRunInsideTheOuterLayers(t *testing.T) {
	front, seen := build(t, goOwned())

	withOrigin := func(target string) *http.Request {
		req := httptest.NewRequest("GET", target, nil)
		req.Header.Set("Origin", "http://allowed.test")
		return req
	}
	rec := serve(front, withOrigin("/people/me"))
	if rec.Header().Get("Access-Control-Allow-Origin") != "http://allowed.test" || rec.Body.String() != "from go" {
		t.Fatalf("CORS not applied to a Go route: %v %q", rec.Header(), rec.Body)
	}

	rec = serve(front, withOrigin("/crash"))
	if rec.Code != 500 || rec.Body.String() != "Internal Server Error" || rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("crash answer: %d %v %q", rec.Code, rec.Header(), rec.Body)
	}
	if !seen.released {
		t.Fatal("the crash did not unwind through the idempotency layer, so its key would stay taken")
	}

	// Python's CORS and guest layers sit outside idempotency: its own refusal
	// still carries allow-origin, and under /g the privacy headers.
	refused := withOrigin("/g/tok")
	refused.Header.Set("Idempotency-Key", "bad")
	rec = serve(front, refused)
	if rec.Code != 422 || rec.Header().Get("Access-Control-Allow-Origin") != "http://allowed.test" || rec.Header().Get("X-Robots-Tag") == "" {
		t.Fatalf("idempotency refusal outside CORS or guest: %d %v", rec.Code, rec.Header())
	}

	rec = serve(front, httptest.NewRequest("GET", "/g/tok", nil))
	if got := rec.Header().Values("Cache-Control"); len(got) != 1 || got[0] != "no-store" || rec.Header().Get("X-Robots-Tag") == "" {
		t.Fatalf("guest route not stamped: %v", rec.Header())
	}
	rec = serve(front, httptest.NewRequest("GET", "/people/me", nil))
	if rec.Header().Get("Cache-Control") != "max-age=60" {
		t.Fatalf("non-guest route stamped: %v", rec.Header())
	}
}

func TestAServedRouteWithoutAHandlerRefusesToBuild(t *testing.T) {
	rt, err := router.New(manifest)
	if err != nil {
		t.Fatal(err)
	}
	options := Options{
		Router: rt, Served: goOwned(), Handlers: map[string]http.Handler{},
		Python: http.NotFoundHandler(), CORS: cors.New("", false),
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		Idempotency: fakeIdempotency(&recorder{}),
	}
	if _, err = New(options); err == nil || !strings.Contains(err.Error(), "no handler") {
		t.Fatalf("err = %v", err)
	}
	options.Idempotency = nil
	if _, err = New(options); err == nil || !strings.Contains(err.Error(), "no idempotency layer") {
		t.Fatalf("missing idempotency layer: err = %v", err)
	}
	options.Served = nil
	if _, err = New(options); err != nil {
		t.Fatalf("nothing served needs no idempotency layer: %v", err)
	}
}

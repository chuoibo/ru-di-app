package routes

import (
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"mobile/services/core/contract"
	"mobile/services/core/internal/db"
	"mobile/services/core/internal/httpapi/dispatch"
	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/httpapi/mw/cors"
	"mobile/services/core/internal/httpapi/router"
	"mobile/services/core/internal/pyval"
	"mobile/services/core/ownership"
)

func devEnv() endpoint.Env {
	return endpoint.Env{Mode: endpoint.ModeDev, NewUnit: func() *db.Unit { return db.NewUnit(nil) }, Now: time.Now}
}

// core serves every route in All() from Go, through dispatch and the endpoint
// pipeline, exactly as the binary would.
func core(t *testing.T) http.Handler {
	return coreWithEnv(t, devEnv(), func(next http.Handler) http.Handler { return next })
}

func coreWithEnv(t *testing.T, env endpoint.Env, middleware func(http.Handler) http.Handler) http.Handler {
	t.Helper()
	ir, err := pyval.Load()
	if err != nil {
		t.Fatal(err)
	}
	handlers, err := Handlers(ir, pyval.NewRegistry(), env)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := ownership.Load()
	if err != nil {
		t.Fatal(err)
	}
	table, err := router.New(manifest.Routes)
	if err != nil {
		t.Fatal(err)
	}
	var served []ownership.Route
	for _, row := range manifest.Routes {
		if handlers[row.ID] != nil {
			served = append(served, row)
		}
	}
	// Every handler must have a manifest row. Spelling this as len(All()) held
	// only while every handler came from the endpoint pipeline; MOUNT /static
	// has a handler and no All() entry, and it still needs its row.
	if len(served) != len(handlers) {
		t.Fatalf("%d of %d handlers are in the manifest", len(served), len(handlers))
	}
	python := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("%s %s went to Python", r.Method, r.RequestURI)
	})
	h, err := dispatch.New(dispatch.Options{
		Router: table, Served: served, Handlers: handlers, Python: python,
		CORS: cors.New("", false), Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Idempotency: middleware,
	})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// staticRouteIDs are the routes scripts/render_static_route_goldens.py renders.
var staticRouteIDs = []string{"GET /interests", "GET /areas"}

// testdata/python_static_routes.json is rendered by
// scripts/render_static_route_goldens.py: the real create_app() answering each
// route over raw ASGI in the pinned image.
func TestStaticRoutesAnswerWhatPythonAnswers(t *testing.T) {
	raw, err := os.ReadFile("testdata/python_static_routes.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden struct {
		Responses []struct {
			Method  string      `json:"method"`
			Path    string      `json:"path"`
			Status  int         `json:"status"`
			Headers [][2]string `json:"headers"`
			Body    string      `json:"body"`
		} `json:"responses"`
	}
	if err := json.Unmarshal(raw, &golden); err != nil {
		t.Fatal(err)
	}
	// Only routes that never touch the database can be rendered without one;
	// the others are compared on real stacks by the parity harness.
	if len(golden.Responses) != len(staticRouteIDs) {
		t.Fatalf("%d goldens for %d static routes", len(golden.Responses), len(staticRouteIDs))
	}
	h := core(t)
	for _, want := range golden.Responses {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(want.Method, want.Path, nil))
		if rec.Code != want.Status || rec.Body.String() != want.Body {
			t.Fatalf("%s %s: Go %d %s\nPython %d %s", want.Method, want.Path, rec.Code, rec.Body, want.Status, want.Body)
		}
		wantHeaders := map[string]string{}
		for _, pair := range want.Headers {
			wantHeaders[strings.ToLower(pair[0])] = pair[1]
		}
		gotHeaders := map[string]string{}
		for name, values := range rec.Header() {
			gotHeaders[strings.ToLower(name)] = strings.Join(values, ", ")
		}
		if len(gotHeaders) != len(wantHeaders) {
			t.Fatalf("%s %s: Go headers %v, Python %v", want.Method, want.Path, gotHeaders, wantHeaders)
		}
		for name, value := range wantHeaders {
			if gotHeaders[name] != value {
				t.Fatalf("%s %s: header %s Go %q, Python %q", want.Method, want.Path, name, gotHeaders[name], value)
			}
		}
	}
}

// A route's declared status must be the decorator's, which the IR records.
func TestEveryRouteDeclaresItsContractStatus(t *testing.T) {
	names, err := fs.Glob(contract.IR, "ir/*.json")
	if err != nil {
		t.Fatal(err)
	}
	statuses := map[string]int{}
	for _, name := range names {
		raw, err := fs.ReadFile(contract.IR, name)
		if err != nil {
			t.Fatal(err)
		}
		var doc struct {
			Routes []struct {
				ID         string `json:"id"`
				StatusCode *int   `json:"status_code"`
			} `json:"routes"`
		}
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatal(err)
		}
		for _, route := range doc.Routes {
			statuses[route.ID] = 200
			if route.StatusCode != nil {
				statuses[route.ID] = *route.StatusCode
			}
		}
	}
	for _, route := range All() {
		want, ok := statuses[route.ID]
		if !ok {
			t.Fatalf("%s is not in the contract IR", route.ID)
		}
		if route.Status != want {
			t.Fatalf("%s declares %d, the decorator says %d", route.ID, route.Status, want)
		}
	}
}

func TestHandlersRefuseADuplicateOrUnboundRoute(t *testing.T) {
	ir, err := pyval.Load()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Handlers(ir, pyval.NewRegistry(), devEnv()); err != nil {
		t.Fatalf("shipped routes: %v", err)
	}
	if _, err := ir.Bind("GET /no-such-route", pyval.NewRegistry()); err == nil {
		t.Fatal("binding an unknown route succeeded, so Handlers could not refuse one")
	}
	// Serve functions are plain values; calling one outside the pipeline
	// needs no request.
	if _, err := interestVocabulary().Serve(context.Background(), &endpoint.Call{}); err != nil {
		t.Fatal(err)
	}
}

package endpoint

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"mobile/services/core/contract"
	"mobile/services/core/internal/db"
	"mobile/services/core/internal/httpapi/dispatch"
	"mobile/services/core/internal/httpapi/mw/cors"
	"mobile/services/core/internal/httpapi/router"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/pyval"
	"mobile/services/core/ownership"
)

const (
	actorID  = "6a1b2c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d"
	targetID = "7b2c3d4e-5f6a-4b7c-9d8e-0f1a2b3c4d5e"
)

var (
	signedIn  = map[string]string{"Content-Type": "application/json", "X-Actor-ID": actorID, "X-Actor-Roles": "member"}
	anonymous = map[string]string{"Content-Type": "application/json"}
)

// front serves POST /reports from Go through dispatch, as core would, in dev
// auth mode and without a database: nothing here opens the transaction.
func front(t *testing.T, serve Serve) http.Handler {
	t.Helper()
	ir, err := pyval.Load()
	if err != nil {
		t.Fatal(err)
	}
	route, err := ir.Bind("POST /reports", pyval.NewRegistry())
	if err != nil {
		t.Fatal(err)
	}
	handler, err := New(route, 201, serve, Env{Mode: ModeDev, NewUnit: func() *db.Unit { return db.NewUnit(nil) }, Now: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := ownership.Load()
	if err != nil {
		t.Fatal(err)
	}
	routes, err := router.New(manifest.Routes)
	if err != nil {
		t.Fatal(err)
	}
	var served []ownership.Route
	for _, row := range manifest.Routes {
		if row.ID == "POST /reports" {
			served = append(served, row)
		}
	}
	if len(served) != 1 {
		t.Fatal("POST /reports is not in the manifest")
	}
	python := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("%s %s went to Python", r.Method, r.RequestURI)
	})
	h, err := dispatch.New(dispatch.Options{
		Router: routes, Served: served, Handlers: map[string]http.Handler{"POST /reports": handler},
		Python: python, CORS: cors.New("", false), Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Idempotency: func(next http.Handler) http.Handler { return next },
	})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func send(h http.Handler, body string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", "/reports", strings.NewReader(body))
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	rec := httptest.NewRecorder()
	func() {
		// A crash ends with net/http's abort, recovered here as the server does.
		defer func() {
			if v := recover(); v != nil && v != http.ErrAbortHandler {
				panic(v)
			}
		}()
		h.ServeHTTP(rec, req)
	}()
	return rec
}

func TestRefusalsComeInFastAPIsOrderAndNeverReachTheRoute(t *testing.T) {
	called := false
	h := front(t, func(context.Context, *Call) (Reply, error) {
		called = true
		return Reply{}, errors.New("the route must not run")
	})
	cases := []struct {
		name     string
		body     string
		headers  map[string]string
		status   int
		contains string
	}{
		{"broken JSON is refused before authentication", `{"target_type": `, anonymous, 422, `"json_invalid"`},
		{"authentication is refused before the body is validated", `{}`, anonymous, 401,
			`{"code":"authentication_required","detail":"Missing X-Actor-ID"}`},
		{"a signed-in caller's body is validated", `{}`, signedIn, 422, `"missing"`},
		{"an undecodable body is FastAPI's 400", "{\"note\": \"\xff\"}", signedIn, 400, "There was an error parsing the body"},
	}
	for _, tc := range cases {
		rec := send(h, tc.body, tc.headers)
		if rec.Code != tc.status || !strings.Contains(rec.Body.String(), tc.contains) {
			t.Fatalf("%s: %d %s", tc.name, rec.Code, rec.Body)
		}
	}
	if called {
		t.Fatal("a refused request reached the route")
	}
}

func TestAValidRequestReachesTheRouteAndItsReplyIsWritten(t *testing.T) {
	var seen *Call
	h := front(t, func(_ context.Context, call *Call) (Reply, error) {
		seen = call
		body := pyjson.NewOrderedMap()
		body.Set("id", pyjson.String(targetID))
		body.Set("created_at", pyjson.String("2026-09-15T08:00:00Z"))
		return Reply{Body: body}, nil
	})
	rec := send(h, `{"target_type":"person","target_id":"`+targetID+`","reason":"spam"}`, signedIn)
	want := `{"id":"` + targetID + `","created_at":"2026-09-15T08:00:00Z"}`
	if rec.Code != 201 || rec.Body.String() != want {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
	if rec.Header().Get("Content-Type") != "application/json" || rec.Header().Get("Content-Length") != strconv.Itoa(len(want)) {
		t.Fatalf("headers %v", rec.Header())
	}
	if seen == nil || seen.Actor == nil || seen.Actor.ID != actorID || seen.Scope.RouteID != "POST /reports" {
		t.Fatalf("route saw %+v", seen)
	}
	model, ok := seen.Values["request"].(*pyval.Model)
	if !ok {
		t.Fatalf("request value is %T", seen.Values["request"])
	}
	if reason, _ := model.Get("reason"); fmt.Sprint(reason) != "spam" {
		t.Fatalf("reason = %v", reason)
	}
}

func TestRouteRefusalsCrashesAndStatusOverrides(t *testing.T) {
	valid := `{"target_type":"post","target_id":"` + targetID + `","reason":"other","note":null}`

	rec := send(front(t, func(context.Context, *Call) (Reply, error) {
		return Reply{}, Refuse(404, "target_not_found", "target missing")
	}), valid, signedIn)
	if rec.Code != 404 || rec.Body.String() != `{"code":"target_not_found","detail":"target missing"}` {
		t.Fatalf("refusal: %d %s", rec.Code, rec.Body)
	}

	rec = send(front(t, func(context.Context, *Call) (Reply, error) {
		return Reply{}, errors.New("boom")
	}), valid, signedIn)
	if rec.Code != 500 || rec.Body.String() != "Internal Server Error" {
		t.Fatalf("crash: %d %s", rec.Code, rec.Body)
	}

	rec = send(front(t, func(context.Context, *Call) (Reply, error) {
		return Reply{Status: 200, Body: pyjson.NewOrderedMap()}, nil
	}), valid, signedIn)
	if rec.Code != 200 || rec.Body.String() != `{}` {
		t.Fatalf("override: %d %s", rec.Code, rec.Body)
	}
}

type dependantNode struct {
	Call         string          `json:"call"`
	Dependencies []dependantNode `json:"dependencies"`
}

// Every dependency a W1 route's tree calls must be one this package stands
// in for, or the route cannot be served from Go.
func TestW1RoutesDependOnlyOnWhatGoStandsInFor(t *testing.T) {
	w1 := map[string]bool{
		"GET /interests": true, "PUT /people/me/interests": true, "GET /contexts/{context_id}/preference-profile": true,
		"GET /areas": true, "GET /contexts/{context_id}/map": true, "GET /contexts/{context_id}/heatmap": true,
		"POST /contexts/{context_id}/meet": true, "GET /contexts/{context_id}/recap": true, "POST /reports": true,
	}
	names, err := fs.Glob(contract.IR, "ir/*.json")
	if err != nil || len(names) == 0 {
		t.Fatalf("no IR files: %v", err)
	}
	found := map[string]bool{}
	for _, name := range names {
		raw, err := fs.ReadFile(contract.IR, name)
		if err != nil {
			t.Fatal(err)
		}
		var doc struct {
			Routes []struct {
				ID        string        `json:"id"`
				Dependant dependantNode `json:"dependant"`
			} `json:"routes"`
		}
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, route := range doc.Routes {
			if !w1[route.ID] {
				continue
			}
			found[route.ID] = true
			var walk func(node dependantNode)
			walk = func(node dependantNode) {
				for _, dependency := range node.Dependencies {
					if !SupportedDependencies[dependency.Call] {
						t.Errorf("%s depends on %s, which has no Go stand-in", route.ID, dependency.Call)
					}
					walk(dependency)
				}
			}
			walk(route.Dependant)
		}
	}
	for id := range w1 {
		if !found[id] {
			t.Errorf("%s is not in the contract IR", id)
		}
	}
}

package routes

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"mobile/services/core/internal/db"
	"mobile/services/core/internal/httpapi/dispatch"
	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/httpapi/mw/cors"
	"mobile/services/core/internal/httpapi/router"
	"mobile/services/core/internal/pyval"
	"mobile/services/core/ownership"
)

// guestCore serves the guest routes from Go without a database.
func guestCore(t *testing.T) (http.Handler, *[]*db.Unit) {
	t.Helper()
	return groupCore(t, "guests")
}

// groupCore serves one manifest group's routes from Go without a database,
// recording every unit it hands out so a test can see whether a request began
// one.
func groupCore(t *testing.T, group string) (http.Handler, *[]*db.Unit) {
	t.Helper()
	units := &[]*db.Unit{}
	env := endpoint.Env{Mode: endpoint.ModeDev, Now: time.Now, NewUnit: func() *db.Unit {
		unit := db.NewUnit(nil)
		*units = append(*units, unit)
		return unit
	}}
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
		if row.Group == group {
			served = append(served, row)
		}
	}
	h, err := dispatch.New(dispatch.Options{
		Router: table, Served: served, Handlers: handlers,
		Python: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Errorf("%s %s went to Python", r.Method, r.RequestURI)
		}),
		CORS: cors.New("", false), Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Idempotency: func(next http.Handler) http.Handler { return next },
	})
	if err != nil {
		t.Fatal(err)
	}
	return h, units
}

func postForm(h http.Handler, target, form string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", target, strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	func() {
		defer func() {
			if v := recover(); v != nil && v != http.ErrAbortHandler {
				panic(v)
			}
		}()
		h.ServeHTTP(rec, req)
	}()
	return rec
}

// The guest routes, in manifest order, are all implemented here.
func TestGuestRoutesAreRegisteredInManifestOrder(t *testing.T) {
	manifest, err := ownership.Load()
	if err != nil {
		t.Fatal(err)
	}
	var want, got []string
	for _, row := range manifest.Routes {
		if row.Group == "guests" {
			want = append(want, row.ID)
		}
	}
	for _, route := range All() {
		if strings.HasPrefix(route.ID, "GET /g/") || strings.HasPrefix(route.ID, "POST /g/") {
			got = append(got, route.ID)
		}
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") || len(want) != 7 {
		t.Fatalf("registered %v, manifest %v", got, want)
	}
}

// uuid.UUID runs inside the objection handlers before the service: a value it
// refuses is a plain 500 with the privacy headers, and no transaction began.
// A reason outside the closed list is refused before the link is looked up.
func TestObjectionFormsAreRefusedBeforeAnyQuery(t *testing.T) {
	token := strings.Repeat("A", 43)
	valid := "7b2c3d4e-5f6a-4b7c-9d8e-0f1a2b3c4d5e"
	cases := []struct {
		name, target, form string
		status             int
		body               string
	}{
		{"objection not a uuid", "/g/" + token + "/doi-so-tien", "obligation_id=nope&reason=other", 500, "Internal Server Error"},
		{"objection empty id", "/g/" + token + "/doi-so-tien", "obligation_id=&reason=zzz", 500, "Internal Server Error"},
		{"evidence not a uuid", "/g/" + token + "/xin-cach-tinh", "obligation_id=" + valid + "0", 500, "Internal Server Error"},
		{"unknown reason", "/g/" + token + "/doi-so-tien", "obligation_id={urn:uuid:" + valid + "}&reason=Other", 422,
			`{"code":"unknown_reason","detail":"Unknown objection reason"}`},
	}
	for _, tc := range cases {
		h, units := guestCore(t)
		rec := postForm(h, tc.target, tc.form)
		if rec.Code != tc.status || rec.Body.String() != tc.body {
			t.Fatalf("%s: %d %q", tc.name, rec.Code, rec.Body)
		}
		if rec.Header().Get("X-Robots-Tag") != "noindex, nofollow" {
			t.Fatalf("%s: headers %v", tc.name, rec.Header())
		}
		if len(*units) != 1 || (*units)[0].Begun() {
			t.Fatalf("%s: a transaction began", tc.name)
		}
	}
}

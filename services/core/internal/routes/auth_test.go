package routes

import (
	"bytes"
	"encoding/json"
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
	"mobile/services/core/internal/limit"
	"mobile/services/core/internal/pyval"
	"mobile/services/core/internal/sms"
	"mobile/services/core/ownership"
)

func authEnv() endpoint.Env {
	return endpoint.Env{
		Mode:        endpoint.ModeDev,
		Now:         time.Now,
		NewUnit:     func() *db.Unit { return db.NewUnit(nil) },
		Limits:      limit.NewSet(limit.Monotonic),
		PersonIDKey: strings.Repeat("k", 32),
		SMS:         sms.LogSender{},
	}
}

func authFront(t *testing.T, env endpoint.Env) http.Handler {
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
		if row.Group == "auth" || row.Group == "sessions" {
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
	return h
}

func authServe(t *testing.T, h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	func() {
		defer func() {
			if v := recover(); v != nil && v != http.ErrAbortHandler {
				t.Fatalf("panic: %v", v)
			}
		}()
		h.ServeHTTP(rec, req)
	}()
	return rec
}

func authPOST(t *testing.T, h http.Handler, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "192.0.2.1:1234"
	return authServe(t, h, req)
}

func problemCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("status %d body %q: %v", rec.Code, rec.Body.String(), err)
	}
	code, _ := body["code"].(string)
	return code
}

func TestW9RoutesBind(t *testing.T) {
	ir, err := pyval.Load()
	if err != nil {
		t.Fatal(err)
	}
	handlers, err := Handlers(ir, pyval.NewRegistry(), authEnv())
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{
		"POST /sessions",
		"GET /sessions",
		"DELETE /sessions/current",
		"DELETE /sessions/{session_id}",
		"POST /auth/otp/request",
		"POST /auth/otp/verify",
		"POST /auth/google",
	} {
		if handlers[id] == nil {
			t.Errorf("missing handler %s", id)
		}
	}
}

func TestSessionBootstrapRequestBounds(t *testing.T) {
	h := authFront(t, authEnv())
	empty := authPOST(t, h, "/sessions", `{"invite_token":""}`)
	if empty.Code != 422 {
		t.Fatalf("empty token: %d %s", empty.Code, empty.Body.String())
	}
	long := authPOST(t, h, "/sessions", `{"invite_token":"`+strings.Repeat("a", 513)+`"}`)
	if long.Code != 422 {
		t.Fatalf("long token: %d %s", long.Code, long.Body.String())
	}
}

func TestOTPRequestLimiterRunsBeforeBody(t *testing.T) {
	h := authFront(t, authEnv())
	for i := 0; i < limit.OTPRequestLimit; i++ {
		rec := authPOST(t, h, "/auth/otp/request", "not-json")
		if rec.Code != 422 || problemCode(t, rec) != "invalid_body" {
			t.Fatalf("spend %d: %d %s", i, rec.Code, rec.Body.String())
		}
	}
	rec := authPOST(t, h, "/auth/otp/request", "not-json")
	if rec.Code != 429 || problemCode(t, rec) != "rate_limited" {
		t.Fatalf("eleventh: %d %s", rec.Code, rec.Body.String())
	}
}

func TestOTPRequestRefusesANonObjectWithoutEchoing(t *testing.T) {
	h := authFront(t, authEnv())
	rec := authPOST(t, h, "/auth/otp/request", `["phone"]`)
	if rec.Code != 422 || problemCode(t, rec) != "invalid_body" {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("phone")) && strings.Contains(rec.Body.String(), "03") {
		t.Fatal("body echoed a number-shaped value")
	}
}

func TestOTPVerifyRefusesABrokenChallengeID(t *testing.T) {
	h := authFront(t, authEnv())
	rec := authPOST(t, h, "/auth/otp/verify", `{"challenge_id":7,"phone":"x","code":"1"}`)
	if rec.Code != 422 || problemCode(t, rec) != "challenge_id_invalid" {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
}

func TestGoogleNilVerifierRefusesBeforeTheToken(t *testing.T) {
	h := authFront(t, authEnv())
	rec := authPOST(t, h, "/auth/google", `{}`)
	if rec.Code != 503 || problemCode(t, rec) != "google_not_configured" {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
}

func TestRevokeCurrentSessionDoesNotDemandAnActor(t *testing.T) {
	h := authFront(t, authEnv())
	req := httptest.NewRequest(http.MethodDelete, "/sessions/current", nil)
	req.Header.Set("Authorization", "Bearer "+"sample-token")
	rec := authServe(t, h, req)
	if rec.Code == 401 {
		t.Fatalf("DELETE /sessions/current demanded an actor: %s", rec.Body.String())
	}
}

func TestRevokeCurrentSessionWithoutBearerIs401(t *testing.T) {
	h := authFront(t, authEnv())
	req := httptest.NewRequest(http.MethodDelete, "/sessions/current", nil)
	rec := authServe(t, h, req)
	if rec.Code != 401 || problemCode(t, rec) != "authentication_required" {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
}

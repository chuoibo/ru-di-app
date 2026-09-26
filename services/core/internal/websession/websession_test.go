package websession

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type fake map[string]Session

func (f fake) Lookup(_ context.Context, token string) (Session, error) {
	s, ok := f[token]
	if !ok {
		return Session{}, ErrAuthentication
	}
	return s, nil
}

var expires = time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC)

func handler() *Handler {
	h := New(fake{"live-token": {Token: "live-token", PersonID: "p-1", ExpiresAt: expires, IssuedVia: "otp", Profile: &ProfileName{DisplayName: "Minh Anh"}}}, nil)
	h.Now = func() time.Time { return expires.Add(-time.Hour) }
	return h
}

func do(h http.Handler, method, path, origin string, mutate func(*http.Request)) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://api.example"+path, nil)
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	if mutate != nil {
		mutate(r)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

const web = "http://127.0.0.1:8177"

func TestSetWritesAnHttpOnlyPathScopedStrictCookie(t *testing.T) {
	w := do(handler(), "POST", "/sessions/web", web, func(r *http.Request) { r.Header.Set("Authorization", "Bearer live-token") })
	if w.Code != 204 {
		t.Fatalf("status %d %s", w.Code, w.Body)
	}
	c := w.Header().Get("Set-Cookie")
	for _, want := range []string{CookieName + "=live-token", "Path=/sessions/web", "HttpOnly", "Secure", "SameSite=Strict", "Max-Age=3600"} {
		if !strings.Contains(c, want) {
			t.Errorf("Set-Cookie %q lacks %q", c, want)
		}
	}
	if w.Header().Get("Access-Control-Allow-Origin") != web || w.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Errorf("credentialed CORS headers missing: %v", w.Header())
	}
}

func TestSetRefusesWhatIsNotALiveSession(t *testing.T) {
	for name, auth := range map[string]string{"none": "", "unknown": "Bearer nope", "unsafe": "Bearer a;b"} {
		w := do(handler(), "POST", "/sessions/web", web, func(r *http.Request) {
			if auth != "" {
				r.Header.Set("Authorization", auth)
			}
		})
		if w.Code == 204 || w.Header().Get("Set-Cookie") != "" {
			t.Errorf("%s: %d, cookie %q; want a refusal and no cookie", name, w.Code, w.Header().Get("Set-Cookie"))
		}
	}
}

func TestResumeHandsBackTheSessionFromTheCookieOnly(t *testing.T) {
	w := do(handler(), "POST", "/sessions/web/resume", web, func(r *http.Request) {
		r.AddCookie(&http.Cookie{Name: CookieName, Value: "live-token"})
	})
	if w.Code != 200 {
		t.Fatalf("status %d %s", w.Code, w.Body)
	}
	var got map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got["token"] != "live-token" || got["person_id"] != "p-1" || got["issued_via"] != "otp" || got["profile"].(map[string]any)["display_name"] != "Minh Anh" {
		t.Errorf("body %v", got)
	}
	// A bearer header is not a way in: resume reads the cookie and nothing else.
	w = do(handler(), "POST", "/sessions/web/resume", web, func(r *http.Request) { r.Header.Set("Authorization", "Bearer live-token") })
	if w.Code != 401 {
		t.Errorf("resume without the cookie: %d, want 401", w.Code)
	}
}

func TestResumeOfADeadSessionClearsTheCookie(t *testing.T) {
	w := do(handler(), "POST", "/sessions/web/resume", web, func(r *http.Request) {
		r.AddCookie(&http.Cookie{Name: CookieName, Value: "revoked-token"})
	})
	if w.Code != 401 || !strings.Contains(w.Header().Get("Set-Cookie"), "Max-Age=0") {
		t.Fatalf("%d, Set-Cookie %q; want 401 and the cookie cleared", w.Code, w.Header().Get("Set-Cookie"))
	}
}

func TestClearExpiresTheCookie(t *testing.T) {
	w := do(handler(), "POST", "/sessions/web/clear", web, nil)
	c := w.Header().Get("Set-Cookie")
	if w.Code != 204 || !strings.Contains(c, CookieName+"=;") || !strings.Contains(c, "Max-Age=0") || !strings.Contains(c, "Path=/sessions/web") {
		t.Fatalf("%d %q", w.Code, c)
	}
}

// A foreign page, a missing Origin (the cross-site guard), and a wildcard
// configuration are all refused before any cookie is read or written.
func TestOriginIsRequiredAndNeverAWildcard(t *testing.T) {
	cookie := func(r *http.Request) { r.AddCookie(&http.Cookie{Name: CookieName, Value: "live-token"}) }
	for _, origin := range []string{"", "https://evil.example", "null", "http://127.0.0.1:8177/path"} {
		w := do(handler(), "POST", "/sessions/web/resume", origin, cookie)
		if w.Code != 403 || strings.Contains(w.Body.String(), "live-token") {
			t.Errorf("origin %q: %d %s", origin, w.Code, w.Body)
		}
	}
	h := handler()
	h.Origins = []string{"*"}
	if w := do(h, "POST", "/sessions/web/resume", "https://app.example", cookie); w.Code != 403 {
		t.Errorf("wildcard origin honoured: %d", w.Code)
	}
	h.Origins = []string{"https://app.example"}
	if w := do(h, "POST", "/sessions/web/resume", "https://app.example", cookie); w.Code != 200 {
		t.Errorf("listed origin refused: %d", w.Code)
	}
	if w := do(h, "POST", "/sessions/web/resume", web, cookie); w.Code != 403 {
		t.Errorf("loopback allowed although an explicit list is configured: %d", w.Code)
	}
}

func TestPreflightAndMethods(t *testing.T) {
	w := do(handler(), "OPTIONS", "/sessions/web", web, nil)
	if w.Code != 204 || w.Header().Get("Access-Control-Allow-Headers") != "authorization" || w.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Errorf("preflight %d %v", w.Code, w.Header())
	}
	if w := do(handler(), "GET", "/sessions/web/resume", web, nil); w.Code != 405 {
		t.Errorf("GET: %d, want 405", w.Code)
	}
	for path, want := range map[string]bool{"/sessions/web": true, "/sessions/web/resume": true, "/sessions/web/clear": true, "/sessions/current": false, "/sessions": false, "/sessions/web/x": false} {
		if Matches(path) != want {
			t.Errorf("Matches(%q) = %v", path, !want)
		}
	}
}

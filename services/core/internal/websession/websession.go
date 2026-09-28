// Package websession keeps a browser signed in across a page reload without
// putting the bearer where page scripts can read it at rest.
//
// Native builds keep the session in the platform keystore (expo-secure-store).
// A browser has no keystore, and the web build used to keep the session in
// memory only, so every reload signed the person out. Writing the bearer to
// localStorage was refused on purpose (src/phien.ts): any script on the page
// can read it at rest. The cookie below is still on disk in the browser's
// cookie jar like any sign-in cookie; what it adds is that no script reads it.
//
// The pattern here is the usual one for single-page apps: the credential at
// rest is an HttpOnly cookie that no script can read, scoped by Path to these
// three endpoints only, SameSite=Strict and Secure; the working bearer lives
// in memory and is handed back once per page load by /resume. The cookie holds
// the session token itself, so revoking the session (sign-out, "sign out that
// device") ends the cookie too -- there is no second credential to revoke.
//
// Every other route still authenticates by the Authorization header alone and
// never reads a cookie, so the cookie adds no ambient authority anywhere else.
//
// These paths are Go-only: the ownership manifest is rendered from the Python
// app and has no row for them, and the existing /sessions routes keep their
// bytes unchanged for parity.
package websession

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"mobile/services/core/internal/auth"
	"mobile/services/core/internal/featureroute"
)

const (
	// CookieName is the one cookie this package sets.
	CookieName = "rudi_web_session"
	cookiePath = "/sessions/web"
)

var ErrAuthentication = errors.New("authentication_required")

// Session is what /resume hands back: enough for the client to rebuild its
// in-memory session exactly as a fresh sign-in would.
type Session struct {
	Token     string       `json:"token"`
	PersonID  string       `json:"person_id"`
	ExpiresAt time.Time    `json:"expires_at"`
	IssuedVia string       `json:"issued_via"`
	Profile   *ProfileName `json:"profile,omitempty"`
}

type ProfileName struct {
	DisplayName string `json:"display_name"`
}

// Backend resolves a token to a live session.
type Backend interface {
	Lookup(ctx context.Context, token string) (Session, error)
}

type Store struct{ Pool *pgxpool.Pool }

// Lookup accepts only a session that is unrevoked, unexpired and belongs to a
// person who is not deleted -- the same rule every bearer route applies.
func (s Store) Lookup(ctx context.Context, token string) (Session, error) {
	out := Session{Token: token}
	var name *string
	err := s.Pool.QueryRow(ctx, `SELECT a.person_id::text, a.expires_at, a.issued_via, p.display_name
		  FROM account_sessions AS a JOIN people AS p ON p.id = a.person_id
		 WHERE a.token_digest = $1 AND a.revoked_at IS NULL AND a.expires_at > clock_timestamp() AND p.deleted_at IS NULL`,
		auth.TokenDigest(token)).Scan(&out.PersonID, &out.ExpiresAt, &out.IssuedVia, &name)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrAuthentication
	}
	if err != nil {
		return Session{}, err
	}
	if name != nil {
		out.Profile = &ProfileName{DisplayName: *name}
	}
	return out, nil
}

type Handler struct {
	mux     *featureroute.Mux
	Backend Backend
	// Origins is MOBILE_CORS_ALLOW_ORIGINS split on commas; empty means the
	// loopback-only default the rest of the API uses. "*" is never honoured
	// here: a credentialed response must name one origin.
	Origins []string
	Now     func() time.Time
}

func New(backend Backend, origins []string) *Handler {
	h := &Handler{mux: featureroute.NewMux(), Backend: backend, Origins: origins, Now: time.Now}
	// Registered by method and path so scripts/check_api_contract.py reads
	// these routes out of this file, as it does for the chat handlers.
	h.mux.HandleFunc("POST /sessions/web", h.set)
	h.mux.HandleFunc("POST /sessions/web/resume", h.resume)
	h.mux.HandleFunc("POST /sessions/web/clear", h.clear)
	return h
}

// Routes lists the patterns New registers, in order. The ownership manifest's
// `features` block must name exactly these (cmd/core features --json).
func Routes() []string { return New(nil, nil).mux.Patterns() }

// Matches the three paths, and nothing under /sessions/{session_id}.
func Matches(path string) bool {
	return path == "/sessions/web" || path == "/sessions/web/resume" || path == "/sessions/web/clear"
}

// originAllowed: the page's own host, an explicitly listed origin, or
// loopback when nothing is listed. A missing Origin is refused: every
// browser sends one on these cross-origin POSTs, and requiring it is the
// cross-site-request guard on top of SameSite=Strict.
func (h *Handler) originAllowed(r *http.Request) (string, bool) {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return "", false
	}
	u, err := url.Parse(origin)
	if err != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", false
	}
	if u.Host == r.Host {
		return origin, true
	}
	if len(h.Origins) == 0 {
		host := u.Hostname()
		return origin, host == "localhost" || host == "127.0.0.1"
	}
	for _, allowed := range h.Origins {
		allowed = strings.TrimSpace(allowed)
		if allowed != "*" && origin == allowed {
			return origin, true
		}
	}
	return "", false
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	header := w.Header()
	header.Set("Cache-Control", "no-store")
	header.Add("Vary", "Origin")
	origin, ok := h.originAllowed(r)
	if !ok {
		refuse(w, http.StatusForbidden, "origin_forbidden")
		return
	}
	header.Set("Access-Control-Allow-Origin", origin)
	header.Set("Access-Control-Allow-Credentials", "true")
	if r.Method == http.MethodOptions {
		header.Set("Access-Control-Allow-Methods", "POST")
		header.Set("Access-Control-Allow-Headers", "authorization")
		header.Set("Access-Control-Max-Age", "600")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost || r.URL.RawQuery != "" {
		refuse(w, http.StatusMethodNotAllowed, "method_not_allowed")
		return
	}
	h.mux.ServeHTTP(w, r)
}

func (h *Handler) set(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	token, problem := auth.BearerToken(r.Header)
	if problem != nil {
		refuse(w, http.StatusUnauthorized, ErrAuthentication.Error())
		return
	}
	if !cookieSafe(token) {
		refuse(w, http.StatusUnprocessableEntity, "token_not_cookie_safe")
		return
	}
	session, err := h.Backend.Lookup(ctx, token)
	if !h.answerable(w, err) {
		return
	}
	h.setCookie(w, token, session.ExpiresAt)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) resume(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	cookie, err := r.Cookie(CookieName)
	if err != nil || cookie.Value == "" || len(cookie.Value) > 2048 {
		refuse(w, http.StatusUnauthorized, ErrAuthentication.Error())
		return
	}
	session, err := h.Backend.Lookup(ctx, cookie.Value)
	if errors.Is(err, ErrAuthentication) {
		// Revoked or expired: take the dead cookie away too.
		h.clearCookie(w)
	}
	if !h.answerable(w, err) {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(session)
}

func (h *Handler) clear(w http.ResponseWriter, _ *http.Request) {
	h.clearCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) answerable(w http.ResponseWriter, err error) bool {
	if errors.Is(err, ErrAuthentication) {
		refuse(w, http.StatusUnauthorized, ErrAuthentication.Error())
		return false
	}
	if err != nil {
		refuse(w, http.StatusServiceUnavailable, "sessions_temporarily_unavailable")
		return false
	}
	return true
}

func (h *Handler) setCookie(w http.ResponseWriter, token string, expires time.Time) {
	maxAge := int(expires.Sub(h.Now()).Seconds())
	if maxAge < 1 {
		maxAge = 1
	}
	w.Header().Add("Set-Cookie", CookieName+"="+token+"; Path="+cookiePath+
		"; Expires="+expires.UTC().Format(http.TimeFormat)+"; Max-Age="+strconv.Itoa(maxAge)+
		"; HttpOnly; Secure; SameSite=Strict")
}

func (h *Handler) clearCookie(w http.ResponseWriter) {
	w.Header().Add("Set-Cookie", CookieName+"=; Path="+cookiePath+
		"; Expires=Thu, 01 Jan 1970 00:00:00 GMT; Max-Age=0; HttpOnly; Secure; SameSite=Strict")
}

// cookieSafe: session tokens are base64url (secrets.token_urlsafe, and the Go
// port's RawURLEncoding); anything else is not written into a header.
func cookieSafe(token string) bool {
	if token == "" || len(token) > 2048 {
		return false
	}
	for _, c := range token {
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

func refuse(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"code": code, "detail": code})
}

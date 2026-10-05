package accountauth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"mobile/services/core/internal/auth"
	"mobile/services/core/internal/featureroute"
)

type Error struct {
	Status int
	Code   string
}

func (e *Error) Error() string               { return e.Code }
func problem(status int, code string) *Error { return &Error{status, code} }

type Handler struct {
	pool   *pgxpool.Pool
	cfg    Config
	mux    *featureroute.Mux
	hashes chan struct{}
}

func New(pool *pgxpool.Pool, cfg Config) *Handler {
	slots := cfg.HashSlots
	if slots < 1 || slots > 16 {
		slots = 4
	}
	h := &Handler{pool: pool, cfg: cfg, mux: featureroute.NewMux(), hashes: make(chan struct{}, slots)}
	h.mux.HandleFunc("POST /auth/register", h.register)
	h.mux.HandleFunc("POST /auth/register/verify", h.verifyRegister)
	h.mux.HandleFunc("POST /auth/login", h.login)
	h.mux.HandleFunc("POST /auth/password/reset/request", h.requestReset)
	h.mux.HandleFunc("POST /auth/password/reset/confirm", h.confirmReset)
	h.mux.HandleFunc("POST /auth/google/challenge", h.googleChallenge)
	h.mux.HandleFunc("POST /auth/google", h.googleLogin)
	h.mux.HandleFunc("POST /auth/google/register", h.googleRegister)
	h.mux.HandleFunc("GET /people/me/account", h.account)
	h.mux.HandleFunc("POST /people/me/account/reauth", h.reauth)
	h.mux.HandleFunc("PUT /people/me/account/password", h.changePassword)
	h.mux.HandleFunc("POST /people/me/account/email", h.changeEmail)
	h.mux.HandleFunc("POST /people/me/account/email/verify", h.verifyEmail)
	h.mux.HandleFunc("POST /people/me/account/google", h.linkGoogle)
	h.mux.HandleFunc("DELETE /people/me/account/google", h.unlinkGoogle)
	h.mux.HandleFunc("PUT /people/me/account/discovery", h.discovery)
	h.mux.HandleFunc("DELETE /sessions/all", h.logoutAll)
	h.mux.HandleFunc("POST /friends/lookup", h.lookup)
	return h
}
func Routes() []string { return New(nil, Config{}).mux.Patterns() }
func Matches(path string) bool {
	return path == "/auth/google" || strings.HasPrefix(path, "/auth/register") || path == "/auth/login" || strings.HasPrefix(path, "/auth/password/") || strings.HasPrefix(path, "/auth/google/") || path == "/people/me/account" || strings.HasPrefix(path, "/people/me/account/") || path == "/sessions/all" || path == "/friends/lookup"
}
func Retired(r *http.Request) bool {
	return strings.HasPrefix(r.URL.Path, "/auth/otp/") || r.URL.Path == "/identity/person-id" || (r.Method == "POST" && r.URL.Path == "/sessions") || r.URL.Path == "/moi" || strings.HasPrefix(r.URL.Path, "/moi/")
}
func Retire(w http.ResponseWriter) {
	respond(w, 410, map[string]string{"code": "account_door_retired", "detail": "Đăng nhập bằng tài khoản hoặc Google."})
}
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if h.pool == nil {
		refuse(w, problem(503, "auth_temporarily_unavailable"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	ip := h.clientIP(r)
	ctx = context.WithValue(ctx, clientIPKey{}, ip)
	r = r.WithContext(ctx)
	if err := h.limit(ctx, "ip", ip, 300, time.Minute); err != nil {
		refuse(w, err)
		return
	}
	h.mux.ServeHTTP(w, r)
}

type clientIPKey struct{}

// requestIP is the address ServeHTTP resolved; empty outside a request.
func requestIP(ctx context.Context) string {
	ip, _ := ctx.Value(clientIPKey{}).(string)
	return ip
}
func respond(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if body != nil {
		_ = json.NewEncoder(w).Encode(body)
	}
}
func refuse(w http.ResponseWriter, err error) {
	var e *Error
	if !errors.As(err, &e) {
		e = problem(503, "auth_temporarily_unavailable")
	}
	if e.Status == 429 {
		w.Header().Set("Retry-After", "60")
	}
	respond(w, e.Status, map[string]string{"code": e.Code, "detail": e.Code})
}
func decode(w http.ResponseWriter, r *http.Request, out any) error {
	if strings.Split(r.Header.Get("Content-Type"), ";")[0] != "application/json" {
		return problem(415, "json_required")
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	raw, err := io.ReadAll(r.Body)
	if err != nil || !utf8.Valid(raw) || !unambiguousJSON(raw) {
		return problem(400, "invalid_body")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(out) != nil || d.Decode(new(any)) != io.EOF {
		return problem(400, "invalid_body")
	}
	return nil
}

// A burst waits briefly for a hash slot rather than failing at once; a queue
// longer than that is load the replica cannot carry, and says so.
const hashWait = 2 * time.Second

func (h *Handler) slot(ctx context.Context) error {
	wait := time.NewTimer(hashWait)
	defer wait.Stop()
	select {
	case h.hashes <- struct{}{}:
		return nil
	case <-ctx.Done():
		return problem(503, "auth_temporarily_unavailable")
	case <-wait.C:
		return problem(503, "auth_busy")
	}
}
func (h *Handler) hash(ctx context.Context, password string) (string, error) {
	if err := h.slot(ctx); err != nil {
		return "", err
	}
	defer func() { <-h.hashes }()
	return hashPassword(password)
}
func (h *Handler) checkPassword(ctx context.Context, password, encoded string) (bool, error) {
	if err := h.slot(ctx); err != nil {
		return false, err
	}
	defer func() { <-h.hashes }()
	return verifyPassword(password, encoded), nil
}
func conflict(err error) error {
	var p *pgconn.PgError
	if errors.As(err, &p) && p.Code == "23505" {
		return problem(409, "account_already_exists")
	}
	return err
}
func commit(ctx context.Context, tx pgx.Tx, err error) error {
	var p *Error
	if err != nil && !errors.As(err, &p) {
		return conflict(err)
	}
	if e := tx.Commit(ctx); e != nil {
		return e
	}
	return err
}
func (h *Handler) actor(r *http.Request) (string, string, error) {
	token, p := auth.BearerToken(r.Header)
	if p != nil {
		return "", "", problem(401, "authentication_required")
	}
	var person, session string
	var managed bool
	err := h.pool.QueryRow(r.Context(), `SELECT a.person_id::text,a.id::text,EXISTS(SELECT 1 FROM managed_accounts m WHERE m.person_id=a.person_id) FROM account_sessions a JOIN people p ON p.id=a.person_id WHERE a.token_digest=$1 AND a.revoked_at IS NULL AND a.expires_at>clock_timestamp() AND p.deleted_at IS NULL`, auth.TokenDigest(token)).Scan(&person, &session, &managed)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", problem(401, "authentication_required")
	}
	if err == nil && !managed {
		// A live session of a person without a managed account (an operator's
		// genesis session) is valid elsewhere; 401 would sign the client out.
		return "", "", problem(403, "managed_account_required")
	}
	return person, session, err
}
func (h *Handler) lockAccount(ctx context.Context, tx pgx.Tx, person, session string, recent bool) error {
	if err := lockPerson(ctx, tx, person); err != nil {
		return err
	}
	var live bool
	if err := tx.QueryRow(ctx, `SELECT true FROM managed_accounts WHERE person_id=$1 FOR UPDATE`, person).Scan(&live); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return problem(401, "authentication_required")
		}
		return err
	}
	err := tx.QueryRow(ctx, `SELECT (NOT $2 OR reauthenticated_at>clock_timestamp()-interval '5 minutes') FROM account_sessions WHERE id=$1 AND person_id=$3 AND revoked_at IS NULL AND expires_at>clock_timestamp() FOR UPDATE`, session, recent, person).Scan(&live)
	if errors.Is(err, pgx.ErrNoRows) {
		return problem(401, "authentication_required")
	}
	if err != nil {
		return err
	}
	if !live {
		return problem(403, "reauthentication_required")
	}
	return nil
}

// People are locked before credentials, sessions and challenges. Erasure starts
// from the people row too, so auth mutations cannot invert that lock order.
func lockPerson(ctx context.Context, tx pgx.Tx, person string) error {
	var live bool
	err := tx.QueryRow(ctx, `SELECT true FROM people WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, person).Scan(&live)
	if errors.Is(err, pgx.ErrNoRows) {
		return problem(401, "authentication_required")
	}
	return err
}

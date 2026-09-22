package chatassist

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"mobile/services/core/internal/auth"
	"mobile/services/core/internal/brain"
	"mobile/services/core/internal/chatv2"
	"mobile/services/core/internal/pyjson"
)

type Handler struct {
	pool  *pgxpool.Pool
	brain *brain.Client
	mux   *http.ServeMux
}

// Invocation excludes inputs and session digests from every public response.
type Invocation struct {
	ID        string    `json:"id"`
	Command   string    `json:"command"`
	Status    string    `json:"status"`
	Code      *string   `json:"code"`
	MessageID *string   `json:"message_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

const columns = `id,command,status,code,message_id,created_at,updated_at`

func New(pool *pgxpool.Pool, client *brain.Client) *Handler {
	h := &Handler{pool: pool, brain: client, mux: http.NewServeMux()}
	h.mux.HandleFunc("GET /contexts/{context}/chat-capabilities", h.capabilities)
	h.mux.HandleFunc("POST /contexts/{context}/ai-invocations", h.create)
	h.mux.HandleFunc("GET /contexts/{context}/ai-invocations", h.list)
	h.mux.HandleFunc("GET /contexts/{context}/ai-invocations/{id}", h.get)
	h.mux.HandleFunc("POST /contexts/{context}/ai-invocations/{id}/retry", h.retry)
	h.mux.HandleFunc("POST /contexts/{context}/ai-invocations/{id}/cancel", h.cancel)
	h.mux.HandleFunc("POST /contexts/{context}/plan-promotions", h.promote)
	h.mux.HandleFunc("GET /contexts/{context}/plan-promotions/{id}", h.promotion)
	return h
}

// Matches also seals the old automatic-history entry points in this candidate.
func Matches(path string) bool {
	p := strings.Split(strings.Trim(path, "/"), "/")
	return len(p) >= 3 && p[0] == "contexts" && (p[2] == "chat-capabilities" || p[2] == "ai-invocations" || p[2] == "plan-promotions" || p[2] == "ai-turn" || (len(p) == 5 && p[2] == "messages" && p[4] == "expense-draft"))
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	if strings.HasSuffix(r.URL.Path, "/ai-turn") || strings.HasSuffix(r.URL.Path, "/expense-draft") {
		refuse(w, 403, "explicit_invocation_required")
		return
	}
	h.mux.ServeHTTP(w, r.WithContext(ctx))
}

func reply(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func refuse(w http.ResponseWriter, status int, code string) {
	reply(w, status, map[string]string{"code": code, "detail": code})
}
func failure(w http.ResponseWriter, err error) {
	var p *denied
	if errors.As(err, &p) {
		refuse(w, p.status, p.code)
	} else {
		refuse(w, 503, "chat_ai_unavailable")
	}
}

type denied struct {
	status int
	code   string
}

func (e *denied) Error() string { return e.code }
func invalid(code string) error { return &denied{400, code} }

func readBody(w http.ResponseWriter, r *http.Request, v any) error {
	if strings.Split(r.Header.Get("Content-Type"), ";")[0] != "application/json" {
		return &denied{415, "json_required"}
	}
	r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return invalid("invalid_body")
	}
	if d.Decode(new(any)) != io.EOF {
		return invalid("invalid_body")
	}
	return nil
}

type grant struct {
	person, member, kind string
	digest               []byte
}

// authority locks in person -> session -> membership -> context order. The same
// rows are held through publication, never through the external inference call.
func authority(ctx context.Context, tx pgx.Tx, conversation string, digest []byte) (grant, error) {
	g := grant{digest: digest}
	if !chatv2.ValidID(conversation) {
		return g, invalid("invalid_context")
	}
	err := tx.QueryRow(ctx, `SELECT person_id FROM account_sessions WHERE token_digest=$1`, digest).Scan(&g.person)
	if errors.Is(err, pgx.ErrNoRows) {
		return g, &denied{401, "authentication_required"}
	}
	if err != nil {
		return g, err
	}
	var exists string
	err = tx.QueryRow(ctx, `SELECT id FROM people WHERE id=$1 AND deleted_at IS NULL FOR SHARE`, g.person).Scan(&exists)
	if errors.Is(err, pgx.ErrNoRows) {
		return g, &denied{401, "authentication_required"}
	}
	if err != nil {
		return g, err
	}
	err = tx.QueryRow(ctx, `SELECT id FROM account_sessions WHERE token_digest=$1 AND person_id=$2 AND revoked_at IS NULL AND expires_at>clock_timestamp() FOR SHARE`, digest, g.person).Scan(&exists)
	if errors.Is(err, pgx.ErrNoRows) {
		return g, &denied{401, "authentication_required"}
	}
	if err != nil {
		return g, err
	}
	err = tx.QueryRow(ctx, `SELECT id FROM memberships WHERE context_id=$1 AND person_id=$2 AND state='active' AND left_at IS NULL FOR SHARE`, conversation, g.person).Scan(&g.member)
	if errors.Is(err, pgx.ErrNoRows) {
		return g, &denied{403, "membership_required"}
	}
	if err != nil {
		return g, err
	}
	err = tx.QueryRow(ctx, `SELECT kind FROM contexts WHERE id=$1 FOR SHARE`, conversation).Scan(&g.kind)
	if err != nil {
		return g, err
	}
	// The compatibility endpoint never publishes plaintext into a v2 group.
	var v2Table *string
	if err = tx.QueryRow(ctx, `SELECT to_regclass('chat_v2_conversations')::text`).Scan(&v2Table); err != nil {
		return g, err
	}
	if v2Table != nil {
		var v2 bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM chat_v2_conversations WHERE context_id=$1)`, conversation).Scan(&v2); err != nil {
			return g, err
		}
		if v2 {
			return g, &denied{409, "encrypted_invocation_required"}
		}
	}
	return g, nil
}

func (h *Handler) begin(r *http.Request) (pgx.Tx, grant, error) {
	token, p := auth.BearerToken(r.Header)
	if p != nil {
		return nil, grant{}, &denied{401, "authentication_required"}
	}
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		return nil, grant{}, err
	}
	g, err := authority(r.Context(), tx, r.PathValue("context"), auth.TokenDigest(token))
	if err != nil {
		_ = tx.Rollback(r.Context())
		return nil, g, err
	}
	return tx, g, nil
}

func (h *Handler) available(ctx context.Context) bool {
	if h.brain == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	v, err := h.brain.PostJSONContext(ctx, "capabilities", pyjson.NewOrderedMap())
	if err != nil {
		return false
	}
	obj, ok := v.(*pyjson.OrderedMap)
	if !ok {
		return false
	}
	v, ok = obj.Get("plan")
	if !ok {
		return false
	}
	obj, ok = v.(*pyjson.OrderedMap)
	if !ok {
		return false
	}
	v, ok = obj.Get("available")
	if !ok {
		return false
	}
	b, ok := v.(pyjson.Bool)
	return ok && bool(b)
}

// preflight authenticates before contacting the internal inference service.
// Mutation handlers authorize again afterwards without holding locks over I/O.
func (h *Handler) preflight(r *http.Request) error {
	tx, g, err := h.begin(r)
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	if g.kind != "group" {
		return &denied{409, "group_plan_only"}
	}
	return tx.Commit(r.Context())
}

func (h *Handler) capabilities(w http.ResponseWriter, r *http.Request) {
	tx, g, err := h.begin(r)
	if err != nil {
		failure(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	if err = tx.Commit(r.Context()); err != nil {
		failure(w, err)
		return
	}
	enabled := g.kind == "group" && h.available(r.Context())
	var reason any = "provider_unavailable"
	if g.kind != "group" {
		reason = "group_plan_only"
	}
	if enabled {
		reason = nil
	}
	reply(w, 200, map[string]any{"protocol": "legacy", "realtime": map[string]bool{"available": true}, "ai": map[string]any{"plan": map[string]any{"available": enabled, "reason": reason}, "share_scope": "invocation_only"}, "media": map[string]bool{"image": true, "sticker": true, "voice": false}})
}

func scan(row pgx.Row) (Invocation, error) {
	var v Invocation
	err := row.Scan(&v.ID, &v.Command, &v.Status, &v.Code, &v.MessageID, &v.CreatedAt, &v.UpdatedAt)
	return v, err
}
func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	s := hex.EncodeToString(b[:])
	return s[:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:]
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var in struct {
		LogicalID string `json:"logical_id"`
		Command   string `json:"command"`
		Prompt    string `json:"prompt"`
	}
	if err := readBody(w, r, &in); err != nil {
		failure(w, err)
		return
	}
	if !chatv2.ValidID(in.LogicalID) || in.Command != "plan" || strings.TrimSpace(in.Prompt) == "" || !utf8.ValidString(in.Prompt) || utf8.RuneCountInString(in.Prompt) > 4000 {
		failure(w, invalid("invalid_invocation"))
		return
	}
	if err := h.preflight(r); err != nil {
		failure(w, err)
		return
	}
	// Probe outside the transaction; a missing provider is an honest refusal.
	available := h.available(r.Context())
	tx, g, err := h.begin(r)
	if err != nil {
		failure(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	if g.kind != "group" {
		refuse(w, 409, "group_plan_only")
		return
	}
	if _, err = tx.Exec(r.Context(), `SELECT pg_advisory_xact_lock(hashtextextended('chat_ai:'||$1,0))`, g.person); err != nil {
		failure(w, err)
		return
	}
	sum := sha256.Sum256([]byte(in.Command + "\x00" + in.Prompt))
	var oldHash []byte
	var oldMember, oldID string
	err = tx.QueryRow(r.Context(), `SELECT id,input_digest,membership_id FROM chat_ai_invocations WHERE context_id=$1 AND person_id=$2 AND logical_id=$3`, r.PathValue("context"), g.person, in.LogicalID).Scan(&oldID, &oldHash, &oldMember)
	if err == nil {
		if !bytes.Equal(oldHash, sum[:]) || oldMember != g.member {
			refuse(w, 409, "invocation_conflict")
			return
		}
		v, e := scan(tx.QueryRow(r.Context(), `SELECT `+columns+` FROM chat_ai_invocations WHERE id=$1`, oldID))
		if e != nil {
			failure(w, e)
			return
		}
		if e = tx.Commit(r.Context()); e != nil {
			failure(w, e)
			return
		}
		reply(w, 200, v)
		return
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		failure(w, err)
		return
	}
	if !available {
		refuse(w, 503, "provider_unavailable")
		return
	}
	var recent int
	if err = tx.QueryRow(r.Context(), `SELECT count(*) FROM chat_ai_invocations WHERE person_id=$1 AND created_at>clock_timestamp()-interval '1 minute'`, g.person).Scan(&recent); err != nil {
		failure(w, err)
		return
	}
	if recent >= 8 {
		refuse(w, 429, "invocation_rate_limited")
		return
	}
	v, err := scan(tx.QueryRow(r.Context(), `INSERT INTO chat_ai_invocations(id,context_id,person_id,membership_id,session_digest,logical_id,input_digest,command,prompt,share_expires_at,status) VALUES($1,$2,$3,$4,$5,$6,$7,'plan',$8,clock_timestamp()+interval '15 minutes','queued') RETURNING `+columns, newID(), r.PathValue("context"), g.person, g.member, g.digest, in.LogicalID, sum[:], in.Prompt))
	if err != nil {
		failure(w, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		failure(w, err)
		return
	}
	reply(w, 202, v)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	n := 20
	if s := r.URL.Query().Get("limit"); s != "" {
		var e error
		n, e = strconv.Atoi(s)
		if e != nil || n < 1 || n > 50 {
			failure(w, invalid("invalid_limit"))
			return
		}
	}
	tx, g, err := h.begin(r)
	if err != nil {
		failure(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	rows, err := tx.Query(r.Context(), `SELECT `+columns+` FROM chat_ai_invocations WHERE context_id=$1 AND person_id=$2 AND membership_id=$3 ORDER BY created_at DESC,id LIMIT $4`, r.PathValue("context"), g.person, g.member, n)
	if err != nil {
		failure(w, err)
		return
	}
	out := []Invocation{}
	for rows.Next() {
		v, e := scan(rows)
		if e != nil {
			rows.Close()
			failure(w, e)
			return
		}
		out = append(out, v)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		failure(w, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		failure(w, err)
		return
	}
	reply(w, 200, map[string]any{"invocations": out})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request)    { h.mutate(w, r, "") }
func (h *Handler) retry(w http.ResponseWriter, r *http.Request)  { h.mutate(w, r, "retry") }
func (h *Handler) cancel(w http.ResponseWriter, r *http.Request) { h.mutate(w, r, "cancel") }
func (h *Handler) mutate(w http.ResponseWriter, r *http.Request, action string) {
	if !chatv2.ValidID(r.PathValue("id")) {
		failure(w, invalid("invalid_invocation"))
		return
	}
	if action == "retry" {
		if err := h.preflight(r); err != nil {
			failure(w, err)
			return
		}
		if !h.available(r.Context()) {
			refuse(w, 503, "provider_unavailable")
			return
		}
	}
	tx, g, err := h.begin(r)
	if err != nil {
		failure(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	v, err := scan(tx.QueryRow(r.Context(), `SELECT `+columns+` FROM chat_ai_invocations WHERE id=$1 AND context_id=$2 AND person_id=$3 AND membership_id=$4 FOR UPDATE`, r.PathValue("id"), r.PathValue("context"), g.person, g.member))
	if errors.Is(err, pgx.ErrNoRows) {
		refuse(w, 404, "invocation_not_found")
		return
	}
	if err != nil {
		failure(w, err)
		return
	}
	if action == "retry" {
		tag, e := tx.Exec(r.Context(), `UPDATE chat_ai_invocations SET status='queued',code=NULL,session_digest=$2,updated_at=clock_timestamp() WHERE id=$1 AND status='failed' AND attempts<3 AND prompt IS NOT NULL AND share_expires_at>clock_timestamp()`, v.ID, g.digest)
		if e != nil {
			failure(w, e)
			return
		}
		if tag.RowsAffected() != 1 {
			refuse(w, 409, "invocation_not_retryable")
			return
		}
	}
	if action == "cancel" {
		if v.Status == "succeeded" {
			refuse(w, 409, "invocation_already_published")
			return
		}
		_, err = tx.Exec(r.Context(), `UPDATE chat_ai_invocations SET status='cancelled',code='cancelled',prompt=NULL,lease_id=NULL,lease_until=NULL,updated_at=clock_timestamp() WHERE id=$1`, v.ID)
		if err != nil {
			failure(w, err)
			return
		}
	}
	if action != "" {
		v, err = scan(tx.QueryRow(r.Context(), `SELECT `+columns+` FROM chat_ai_invocations WHERE id=$1`, v.ID))
		if err != nil {
			failure(w, err)
			return
		}
	}
	if err = tx.Commit(r.Context()); err != nil {
		failure(w, err)
		return
	}
	reply(w, 200, v)
}

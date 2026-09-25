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
	"mobile/services/core/internal/aiharness"
	"mobile/services/core/internal/auth"
	"mobile/services/core/internal/brain"
	"mobile/services/core/internal/chatv2"
	"mobile/services/core/internal/featureroute"
	"mobile/services/core/internal/pyjson"
)

type Handler struct {
	pool   *pgxpool.Pool
	brain  *brain.Client
	mux    *featureroute.Mux
	worker WorkerConfig
	// nepEngine, when set, runs Nếp's jobs in Go instead of the brain's
	// nep-reply (MOBILE_AI_ENGINE_NEP=go). nepGo says the host chose the Go
	// engine even where this process runs no worker, so asking Nếp never
	// waits on a probe of the brain.
	nepEngine *aiharness.Engine
	nepGo     bool
}

// Invocation excludes inputs and session digests from every public response.
type Invocation struct {
	ID        string  `json:"id"`
	Command   string  `json:"command"`
	Status    string  `json:"status"`
	Code      *string `json:"code"`
	MessageID *string `json:"message_id"`
	// The `@Rủ Đi` message this invocation answers; null for an invocation
	// from a client that does not name one.
	TriggerMessageID *string `json:"trigger_message_id"`
	// How many shared turns the server confirmed; null on rows from before
	// the column existed.
	SoTinDoc  *int      `json:"so_tin_doc"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

const columns = `id,command,status,code,message_id,trigger_message_id::text,so_tin_doc,created_at,updated_at`

func New(pool *pgxpool.Pool, client *brain.Client) *Handler {
	h := &Handler{pool: pool, brain: client, mux: featureroute.NewMux(), worker: DefaultWorkerConfig()}
	h.mux.HandleFunc("GET /contexts/{context}/chat-capabilities", h.capabilities)
	h.mux.HandleFunc("POST /contexts/{context}/ai-invocations", h.create)
	h.mux.HandleFunc("GET /contexts/{context}/ai-invocations", h.list)
	h.mux.HandleFunc("GET /contexts/{context}/ai-invocations/{id}", h.get)
	h.mux.HandleFunc("POST /contexts/{context}/ai-invocations/{id}/retry", h.retry)
	h.mux.HandleFunc("POST /contexts/{context}/ai-invocations/{id}/cancel", h.cancel)
	h.mux.HandleFunc("POST /contexts/{context}/plan-promotions", h.promote)
	h.mux.HandleFunc("GET /contexts/{context}/plan-promotions/{id}", h.promotion)
	h.mux.HandleFunc("POST /contexts/{context}/shared-drafts", h.draftCreate)
	h.mux.HandleFunc("GET /contexts/{context}/shared-drafts/{id}", h.draftGet)
	h.mux.HandleFunc("PATCH /contexts/{context}/shared-drafts/{id}", h.draftPatch)
	h.mux.HandleFunc("POST /contexts/{context}/shared-drafts/{id}/discard", h.draftDiscard)
	// Nếp's own questions (ADR-0036 §2.7, §2.8): same queue, sealed result.
	h.mux.HandleFunc("POST /me/nep/ai-invocations", h.nepCreate)
	h.mux.HandleFunc("GET /me/nep/ai-invocations/{id}", h.nepGet)
	return h
}

// Routes lists the patterns New registers, in order. The ownership manifest's
// `features` block must name exactly these (cmd/core features --json).
func Routes() []string { return New(nil, nil).mux.Patterns() }

// Matches also seals the per-message expense-draft entry point, which reads a
// stored message for the model without anyone handing it over. The old
// automatic turn (`ai-turn`) is not sealed here: it is deleted in both
// backends (ADR-0036 §2.1), so it falls through to Python's 404.
//
// `/me/nep/ai-invocations` is Go-only and sits beside `/me/nep/media`, which
// Python still serves; only the exact `ai-invocations` segment is taken here.
func Matches(path string) bool {
	p := strings.Split(strings.Trim(path, "/"), "/")
	if len(p) >= 3 && p[0] == "me" && p[1] == "nep" && p[2] == "ai-invocations" {
		return true
	}
	return len(p) >= 3 && p[0] == "contexts" && (p[2] == "chat-capabilities" || p[2] == "ai-invocations" || p[2] == "plan-promotions" || p[2] == "shared-drafts" || (len(p) == 5 && p[2] == "messages" && p[4] == "expense-draft"))
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	if strings.HasSuffix(r.URL.Path, "/expense-draft") {
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

// The ceiling is per route, not per package. Only the invocation route carries
// a context bundle; raising the shared limit would widen the blast radius of
// every other body for no reason.
func readBody(w http.ResponseWriter, r *http.Request, v any, tran int64) error {
	if strings.Split(r.Header.Get("Content-Type"), ";")[0] != "application/json" {
		return &denied{415, "json_required"}
	}
	r.Body = http.MaxBytesReader(w, r.Body, tran)
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
	// lane is the room's transport as the server finds it, never as a client
	// says it: "legacy" for every room this endpoint serves today, because a
	// v2 room is refused below before anything is written.
	lane   string
	digest []byte
}

// authority locks in person -> session -> membership -> context order. The same
// rows are held through publication, never through the external inference call.
func authority(ctx context.Context, tx pgx.Tx, conversation string, digest []byte) (grant, error) {
	g := grant{digest: digest, lane: laneLegacy}
	if !chatv2.ValidID(conversation) {
		return g, invalid("invalid_context")
	}
	var err error
	if g.person, err = phien(ctx, tx, digest); err != nil {
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
			g.lane = laneV2
			return g, &denied{409, "encrypted_invocation_required"}
		}
	}
	return g, nil
}

// phien resolves a bearer session to the person behind it, locking person then
// session in the order every caller in this package uses. It is the whole of
// what a personal (`/me/nep`) invocation may learn about its caller: no room,
// no membership, no name.
func phien(ctx context.Context, tx pgx.Tx, digest []byte) (string, error) {
	var person, exists string
	err := tx.QueryRow(ctx, `SELECT person_id FROM account_sessions WHERE token_digest=$1`, digest).Scan(&person)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", &denied{401, "authentication_required"}
	}
	if err != nil {
		return "", err
	}
	err = tx.QueryRow(ctx, `SELECT id FROM people WHERE id=$1 AND deleted_at IS NULL FOR SHARE`, person).Scan(&exists)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", &denied{401, "authentication_required"}
	}
	if err != nil {
		return "", err
	}
	err = tx.QueryRow(ctx, `SELECT id FROM account_sessions WHERE token_digest=$1 AND person_id=$2 AND revoked_at IS NULL AND expires_at>clock_timestamp() FOR SHARE`, digest, person).Scan(&exists)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", &denied{401, "authentication_required"}
	}
	if err != nil {
		return "", err
	}
	return person, nil
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

// `share_scope` is a gate, not a label. While it reads `invocation_only` the
// client attaches no bundle at all, so an older server keeps receiving exactly
// the old body. Flipping it here is what turns the context path on, and it is
// flipped only now that the preview block above the send button exists
// (ADR-0036 §4 forbids the one without the other).
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
	// chia_bill reads through the same provider as plan (one key, one probe),
	// so it is advertised with the same answer rather than a second guess.
	// `mention` says this server takes `trigger_message_id` and answers inside
	// the thread. A client that does not see it sends no trigger, so an older
	// server never meets a field its DisallowUnknownFields would refuse.
	reply(w, 200, map[string]any{"protocol": "legacy", "realtime": map[string]bool{"available": true}, "ai": map[string]any{"plan": map[string]any{"available": enabled, "reason": reason}, "chia_bill": map[string]any{"available": enabled, "reason": reason}, "share_scope": "caller_attached", "mention": true}, "media": map[string]bool{"image": true, "sticker": true, "voice": false}})
}

func scan(row pgx.Row) (Invocation, error) {
	var v Invocation
	err := row.Scan(&v.ID, &v.Command, &v.Status, &v.Code, &v.MessageID, &v.TriggerMessageID, &v.SoTinDoc, &v.CreatedAt, &v.UpdatedAt)
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

// lenhNhom is the closed list of commands a group invocation may carry. It
// mirrors `chat_ai_command_scope` in schema_scope.sql, so a command the table
// would refuse is refused here as a 400 rather than surfacing as a 500 from the
// INSERT. Both commands share one queue, one digest, one rate limit and one
// authority check; only the worker's inference step differs.
func lenhNhom(command string) bool {
	return command == lenhPlan || command == lenhChiaBill
}

const (
	lenhPlan     = "plan"
	lenhChiaBill = "chia_bill"
)

// The two values of `chat_ai_invocations.lane`.
const (
	laneLegacy = "legacy"
	laneV2     = "v2"
)

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var in struct {
		LogicalID string  `json:"logical_id"`
		Command   string  `json:"command"`
		Prompt    string  `json:"prompt"`
		BoiCanh   *bundle `json:"boi_canh"`
		// The `@Rủ Đi` message this answers (ADR-0039). Optional, so an app
		// from before it keeps working. There is no `lane` field on purpose:
		// the lane is the server's finding, and a client sending one is 400.
		TriggerMessageID *string `json:"trigger_message_id"`
	}
	if err := readBody(w, r, &in, maxBodyWithBundle); err != nil {
		failure(w, err)
		return
	}
	if !chatv2.ValidID(in.LogicalID) || !lenhNhom(in.Command) || strings.TrimSpace(in.Prompt) == "" || !utf8.ValidString(in.Prompt) || utf8.RuneCountInString(in.Prompt) > 4000 || (in.TriggerMessageID != nil && !chatv2.ValidID(*in.TriggerMessageID)) {
		failure(w, invalid("invalid_invocation"))
		return
	}
	trigger := ""
	if in.TriggerMessageID != nil {
		trigger = *in.TriggerMessageID
	}
	// Bounds are pure, so they run before anything opens a transaction: an
	// oversized body never reaches the database and never probes the provider.
	if in.BoiCanh != nil {
		if err := kiemBoiCanh(in.BoiCanh, in.Prompt); err != nil {
			failure(w, err)
			return
		}
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
	var goi []byte
	if in.BoiCanh != nil {
		if goi, err = canonical(in.BoiCanh); err != nil {
			failure(w, err)
			return
		}
	}
	// The replay lookup comes before any check against the room's messages.
	// The same bytes under the same logical id get the stored invocation back,
	// whatever became of those messages since: a retry after the `@Rủ Đi`
	// message was taken back, or after it turned 24 hours old, is still the
	// same call and must not turn into a 422.
	sum := inputDigest(in.Command, in.Prompt, goi, trigger)
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
	// A new invocation: now the bundle and the trigger are checked against
	// the room, in the order they always were, before the provider refusal.
	soTin := 0
	if in.BoiCanh != nil {
		if soTin, err = thuocPhong(r.Context(), tx, r.PathValue("context"), in.BoiCanh); err != nil {
			failure(w, err)
			return
		}
	}
	if trigger != "" {
		if err = kiemTrigger(r.Context(), tx, r.PathValue("context"), g.person, trigger); err != nil {
			failure(w, err)
			return
		}
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
	if err = gioiHanPhong(r.Context(), tx, r.PathValue("context")); err != nil {
		failure(w, err)
		return
	}
	v, err := scan(tx.QueryRow(r.Context(), `INSERT INTO chat_ai_invocations(id,scope,context_id,person_id,membership_id,session_digest,logical_id,input_digest,command,prompt,boi_canh,share_expires_at,status,trigger_message_id,lane,so_tin_doc) VALUES($1,'group',$2,$3,$4,$5,$6,$7,$8,$9,$10,clock_timestamp()+interval '15 minutes','queued',$11,$12,$13) RETURNING `+columns, newID(), r.PathValue("context"), g.person, g.member, g.digest, in.LogicalID, sum[:], in.Command, in.Prompt, goiHoacNull(goi), in.TriggerMessageID, g.lane, soTin))
	if daCoTraLoi(err) {
		// Another logical call already answers this message: one message, one
		// answer. Not invocation_conflict, which means "this id, other bytes".
		refuse(w, 409, "invocation_trigger_taken")
		return
	}
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

// inputDigest is what idempotency compares. A call without a trigger keeps the
// exact digest it had before triggers existed, so a retry that straddles a
// deploy still replays; a trigger joins the digest, so the same logical id
// sent again for a different message is a conflict, never a replay of the
// answer to the first one.
func inputDigest(command, prompt string, goi []byte, trigger string) [32]byte {
	in := append([]byte(command+"\x00"+prompt+"\x00"), goi...)
	if trigger != "" {
		in = append(in, []byte("\x00"+trigger)...)
	}
	return sha256.Sum256(in)
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
		// Retryability first: a job that can never run again is 409 whatever
		// the room is doing, not a 429 that invites the client to wait and try
		// again. The row is held FOR UPDATE, so this cannot change before the
		// UPDATE below, which keeps the same conditions as its guard.
		var retryable bool
		if e := tx.QueryRow(r.Context(), `SELECT status='failed' AND attempts<3 AND prompt IS NOT NULL AND share_expires_at>clock_timestamp() FROM chat_ai_invocations WHERE id=$1`, v.ID).Scan(&retryable); e != nil {
			failure(w, e)
			return
		}
		if !retryable {
			refuse(w, 409, "invocation_not_retryable")
			return
		}
		// A retry puts the job back in flight, so it counts against the room
		// the way a new one does; the hourly count is by creation and stays.
		if e := gioiHanPhong(r.Context(), tx, r.PathValue("context")); e != nil {
			failure(w, e)
			return
		}
		tag, e := tx.Exec(r.Context(), `UPDATE chat_ai_invocations SET status='queued',code=NULL,session_digest=$2,updated_at=clock_timestamp() WHERE id=$1 AND status='failed' AND attempts<3 AND prompt IS NOT NULL AND share_expires_at>clock_timestamp()`, v.ID, g.digest)
		if daCoTraLoi(e) {
			// While this one sat failed, a newer call took its message.
			refuse(w, 409, "invocation_trigger_taken")
			return
		}
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
		_, err = tx.Exec(r.Context(), `UPDATE chat_ai_invocations SET status='cancelled',code='cancelled',prompt=NULL,boi_canh=NULL,lease_id=NULL,lease_until=NULL,updated_at=clock_timestamp() WHERE id=$1`, v.ID)
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

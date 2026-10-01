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
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"mobile/services/core/internal/aiharness"
	"mobile/services/core/internal/aistream"
	"mobile/services/core/internal/auth"
	"mobile/services/core/internal/chatv2"
	"mobile/services/core/internal/domain/pairnotebook"
	"mobile/services/core/internal/domain/pairsteps"
	"mobile/services/core/internal/featureroute"
	"mobile/services/core/internal/gudoi"
	"mobile/services/core/internal/repo"
	"mobile/services/core/internal/service"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	pool   *pgxpool.Pool
	mux    *featureroute.Mux
	worker WorkerConfig
	// nepEngine and nhomEngine run Nếp's and the group's jobs in this
	// process (internal/aiharness, the only engine since ADR-0052). coMay
	// says this host has a model, set with an engine or, in a process that
	// serves the routes while `core work` runs the jobs, by WithCoMay.
	// Without it every invocation is refused provider_unavailable.
	nepEngine  *aiharness.Engine
	nhomEngine *aiharness.Engine
	coMay      bool
	// scopes limits the jobs this process claims (WithQueues); nil is all.
	scopes []string
	// nhip, when set, carries the heartbeat and the model-call counter
	// (WithNhipPool).
	nhip *pgxpool.Pool
	// slots bound the jobs this process runs at once, however claimed.
	slotsOnce sync.Once
	slots     chan struct{}
	// stream, when set, carries answers to their readers as they are
	// written (MOBILE_REDIS_URL; phat.go, sse.go); hub wakes this process's
	// SSE readers; dungSSE closes when the process stops.
	stream  *aistream.Stream
	hub     *aistream.Hub
	dungSSE <-chan struct{}
	// sucChua bounds the open SSE connections (sse.go).
	sucChua sucChuaSSE
	// sseXacThucMoi, when set, replaces the 10 s between a stream's
	// authorization checks (tests).
	sseXacThucMoi time.Duration
	// truocChot, when set, runs just before a job's terminal transaction
	// (tests: a job held after its content left, while the worker stops).
	truocChot func(context.Context)
	// nhipPhat paces text released after a commit (phat.go); WithStream
	// sets aiharness.NhipPhat.
	nhipPhat time.Duration
	// phong says this process's change-feed WebSocket carries the room
	// key's `ai` frames to the room's members (WithPhong, slice 12).
	phong bool
}

// WithPhong says the room's members see answers as they are written: this
// process's change-feed WebSocket carries the room key as `ai` frames
// (chatlegacychange.Handler.WithAi, same stream). chat-capabilities then
// says ai.stream = phong for a legacy-lane group room whose stream is up.
func (h *Handler) WithPhong() *Handler {
	h.phong = true
	return h
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

func New(pool *pgxpool.Pool) *Handler {
	h := &Handler{pool: pool, mux: featureroute.NewMux(), worker: DefaultWorkerConfig()}
	h.mux.HandleFunc("GET /contexts/{context}/chat-capabilities", h.capabilities)
	h.mux.HandleFunc("POST /contexts/{context}/ai-invocations", h.create)
	h.mux.HandleFunc("GET /contexts/{context}/ai-invocations", h.list)
	h.mux.HandleFunc("GET /contexts/{context}/ai-invocations/{id}", h.get)
	// The requester's stream of one invocation (slice 11, sse.go).
	h.mux.HandleFunc(routeSuKienNhom, h.suKienNhom)
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
	h.mux.HandleFunc(routeSuKienNep, h.nepEvents)
	return h
}

// Routes lists the patterns New registers, in order. The ownership manifest's
// `features` block must name exactly these (cmd/core features --json).
func Routes() []string { return New(nil).mux.Patterns() }

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
	ctx := r.Context()
	if han, ok := h.hanYeuCau(r); ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, han)
		defer cancel()
	}
	if strings.HasSuffix(r.URL.Path, "/expense-draft") {
		refuse(w, 403, "explicit_invocation_required")
		return
	}
	h.mux.ServeHTTP(w, r.WithContext(ctx))
}

// The two stream routes (sse.go), the only ones without the 8 s bound.
const (
	routeSuKienNhom = "GET /contexts/{context}/ai-invocations/{id}/events"
	routeSuKienNep  = "GET /me/nep/ai-invocations/{id}/events"
)

// hanYeuCau is how long a request of this engine may take: 8 s, except a
// stream, which lives up to its own 180 s bound and ends on the client's
// leaving or the process stopping (sse.go). Its authorization runs under a
// short deadline of its own. The stream is told by the route the mux
// matches, not by the path's suffix: another GET that merely ends in
// «/events» keeps its bound (review of slice 11, finding 12).
func (h *Handler) hanYeuCau(r *http.Request) (time.Duration, bool) {
	if _, pattern := h.mux.Handler(r); pattern == routeSuKienNhom || pattern == routeSuKienNep {
		return 0, false
	}
	return 8 * time.Second, true
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
	room, person, member, kind string
	// lane is the room's transport as the server finds it, never as a client
	// says it: "legacy" for every room this endpoint serves today, because a
	// v2 room is refused below before anything is written.
	lane   string
	digest []byte
}

// authority locks in person -> session -> membership -> context order. The same
// rows are held through publication, never through the external inference call.
func authority(ctx context.Context, tx pgx.Tx, conversation string, digest []byte) (grant, error) {
	g := grant{room: conversation, digest: digest, lane: laneLegacy}
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

// The two room kinds the assistant answers in (contexts.kind). A pair is a
// chat of two. Both are rooms of friends and take the same assistant
// (decision 2026-09-28, two classes): `hoi`, plan, chia_bill, shared drafts
// and plan promotion alike. A pair whose two people both said yes to «Một
// đôi» is a couple as well (laDoi), which adds to that and removes nothing.
const (
	kindGroup = "group"
	kindPair  = "pair"
)

// phongAi is the room check every step of an invocation makes after
// authority: at preflight, at create, on the stream, and again in the
// worker before it reads and before it publishes. A group passes; a pair
// passes only while it is still open between its two people (capConMo);
// any other room kind is refused as before.
func phongAi(ctx context.Context, tx pgx.Tx, g grant) error {
	switch g.kind {
	case kindGroup:
		return nil
	case kindPair:
		return capConMo(ctx, tx, g.room, g.person)
	}
	return &denied{409, "group_plan_only"}
}

// capConMo is chatlegacychange's pair rule (store.go authorize), the same
// three reads: the other active member exists, their account is not
// deleted, and the latest friend request between the two that was not
// declined is not a block. A block can land while a turn runs, so the
// worker asks again before it reads and before it publishes. It refuses
// with the code a missing membership gets: a pair closed this way is not a
// room the caller may call the assistant into.
func capConMo(ctx context.Context, tx pgx.Tx, room, person string) error {
	var other, exists string
	err := tx.QueryRow(ctx, `SELECT person_id FROM memberships WHERE context_id=$1 AND person_id<>$2 AND state='active' AND left_at IS NULL ORDER BY person_id LIMIT 1`, room, person).Scan(&other)
	if errors.Is(err, pgx.ErrNoRows) {
		return &denied{403, "membership_required"}
	}
	if err != nil {
		return err
	}
	err = tx.QueryRow(ctx, `SELECT id FROM people WHERE id=$1 AND deleted_at IS NULL`, other).Scan(&exists)
	if errors.Is(err, pgx.ErrNoRows) {
		return &denied{403, "membership_required"}
	}
	if err != nil {
		return err
	}
	var state string
	err = tx.QueryRow(ctx, `SELECT state FROM friend_requests WHERE ((requester_id=$1 AND addressee_id=$2) OR (requester_id=$2 AND addressee_id=$1)) AND state<>'declined' ORDER BY created_at DESC LIMIT 1`, person, other).Scan(&state)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if state == "blocked" {
		return &denied{403, "membership_required"}
	}
	return nil
}

// laDoi reports whether the room is a couple: a pair whose two people have
// both said yes to «Một đôi» (bat_doi) at now, by the notebook's own rule
// (pairnotebook.CanBatDoi over the live cycle's participants, or the
// room's active members before any cycle). A group, or a pair with no
// notebook, is not. Read in the caller's transaction; never cached.
func laDoi(ctx context.Context, tx pgx.Tx, g grant, now time.Time) (bool, error) {
	consents, participants, err := soDoi(ctx, tx, g)
	if err != nil || participants == nil {
		return false, err
	}
	return pairnotebook.CanBatDoi(consents, participants, &now), nil
}

// soDoi is the room's notebook as the couple's rules read it: its consent
// rows and its people (the live cycle's participants, or the active members
// before any cycle). nil people when the room is not a pair or has no
// notebook.
func soDoi(ctx context.Context, tx pgx.Tx, g grant) ([]pairnotebook.Consent, []string, error) {
	if g.kind != kindPair {
		return nil, nil, nil
	}
	store := repo.Repository{Q: tx}
	notebook, err := store.GetPairNotebook(ctx, g.room)
	if err != nil || notebook == nil {
		return nil, nil, err
	}
	rows, err := store.ListMembers(ctx, g.room)
	if err != nil {
		return nil, nil, err
	}
	members := []string{}
	for _, row := range rows {
		if row.State == "active" {
			members = append(members, row.PersonID)
		}
	}
	pair := service.PairNotebookOf(notebook)
	return pairsteps.ConsentsOf(pair), pairsteps.Participants(pair, members), nil
}

// guConDung reports whether the chat may still use the taste of every
// person in gu (ADR-0048): the room still a couple, and each of them with a
// live `chia_gu` granted under the wording that names the chat
// (gudoi.NguoiDuocDung). It first takes a share lock on the room's
// notebook row, which a revocation locks for update (pairsteps
// lockedNotebook): a revocation either commits before this read and is
// seen, or waits until the card is posted. Asked in the transaction that
// publishes; never cached.
func guConDung(ctx context.Context, tx pgx.Tx, g grant, gu []string, now time.Time) (bool, error) {
	if g.kind != kindPair {
		return false, nil
	}
	var so string
	err := tx.QueryRow(ctx, `SELECT id FROM pair_notebooks WHERE context_id=$1 FOR SHARE`, g.room).Scan(&so)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	consents, participants, err := soDoi(ctx, tx, g)
	if err != nil || participants == nil {
		return false, err
	}
	duocDung := gudoi.NguoiDuocDung(consents, participants, now)
	for _, person := range gu {
		if !slices.Contains(duocDung, person) {
			return false, nil
		}
	}
	return true, nil
}

// guChat is what chat-capabilities says of the couple's shared taste for
// the caller (ADR-0048 §4): nil outside a couple; otherwise where the
// caller's own switch stands for the chat (gudoi.TrangThaiCua: off, on, or
// on under the older wording and to be turned on again) and whether the
// other person's taste may be used in the chat. Never cached.
func guChat(ctx context.Context, tx pgx.Tx, g grant, now time.Time) (map[string]any, error) {
	consents, participants, err := soDoi(ctx, tx, g)
	if err != nil || participants == nil || !pairnotebook.CanBatDoi(consents, participants, &now) {
		return nil, err
	}
	duocDung := gudoi.NguoiDuocDung(consents, participants, now)
	nguoiKia := false
	for _, person := range duocDung {
		nguoiKia = nguoiKia || person != g.person
	}
	return map[string]any{"cua_toi": string(gudoi.TrangThaiCua(consents, g.person, now)), "nguoi_kia": nguoiKia}, nil
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

// nhomSanSang says a group or pair invocation can be taken: this host has a
// model (coMay). With the worker in this process the engine was built at
// startup or `serve` refused to start; with `core work` elsewhere it is this
// process's configuration, as nepSanSang says.
func (h *Handler) nhomSanSang(context.Context) bool { return h.coMay }

// WithCoMay marks a process that serves the assistant's routes while the
// jobs run in `core work`: it has a model configured, so invocations are
// taken and chat-capabilities says the assistant is on.
func (h *Handler) WithCoMay() *Handler {
	h.coMay = true
	return h
}

// preflight authenticates before anything is queued. Mutation handlers
// authorize again afterwards without holding locks over I/O.
func (h *Handler) preflight(r *http.Request) error {
	tx, g, err := h.begin(r)
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	if err = phongAi(r.Context(), tx, g); err != nil {
		return err
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
	// A pair answers only while it is open between its two people: a
	// blocked pair, or one whose other person is gone, is refused here as on
	// every other route of the room.
	if err = phongAi(r.Context(), tx, g); err != nil {
		failure(w, err)
		return
	}
	// `cap_doi`: the room is a couple (laDoi), asked at every read. The app
	// gates the couple's own features on it; a chat of two without it is a
	// room of friends and gets everything a group gets.
	now := time.Now().UTC()
	capDoi, err := laDoi(r.Context(), tx, g, now)
	if err != nil {
		failure(w, err)
		return
	}
	// `gu_chat` (ADR-0048 §4): in a couple, where the caller's own taste
	// switch stands for the chat and whether the other's taste may be used
	// there; null in every other room. The app shows «Bật lại cho chat» to
	// a person whose switch predates the chat wording.
	gu, err := guChat(r.Context(), tx, g, now)
	if err != nil {
		failure(w, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		failure(w, err)
		return
	}
	// One answer for every command, in a group and in a pair alike: this
	// host has a model or it has none (a keyless stack says
	// provider_unavailable, as it always has).
	enabled := h.coMay
	hoiCo := h.coMay
	var hoiVi any = "provider_unavailable"
	if hoiCo {
		hoiVi = nil
	}
	var reason any = "provider_unavailable"
	if enabled {
		reason = nil
	}
	// chia_bill reads through the same provider as plan (one key, one probe),
	// so it is advertised with the same answer rather than a second guess.
	// `mention` says this server takes `trigger_message_id` and answers inside
	// the thread. A client that does not see it sends no trigger, so an older
	// server never meets a field its DisallowUnknownFields would refuse.
	//
	// `stream` says who can watch an answer being written (contract §3):
	// `nguoi_goi` when the requester can open …/events on a live stream,
	// `khong` when this host has no stream or its Redis is unreachable, and
	// `phong` when every member of the room watches it too, through the
	// change feed's WebSocket `ai` frame (slice 12). A client that sees no
	// field reads `khong`.
	reply(w, 200, map[string]any{"protocol": "legacy", "realtime": map[string]bool{"available": true}, "ai": map[string]any{"plan": map[string]any{"available": enabled, "reason": reason}, "chia_bill": map[string]any{"available": enabled, "reason": reason}, "hoi": map[string]any{"available": hoiCo, "reason": hoiVi}, "share_scope": "caller_attached", "mention": true, "stream": h.aiStream(g)}, "media": map[string]bool{"image": true, "sticker": true, "voice": false}, "cap_doi": capDoi, "gu_chat": gu})
}

// The values of chat-capabilities' ai.stream (contract §3).
const (
	aiStreamPhong    = "phong"
	aiStreamNguoiGoi = "nguoi_goi"
	aiStreamKhong    = "khong"
)

// aiStream is ai.stream for the room g names.
func (h *Handler) aiStream(g grant) string {
	if h.stream == nil || !h.stream.Song() || (g.kind != kindGroup && g.kind != kindPair) {
		return aiStreamKhong
	}
	// The grant reached here is always the legacy lane (a v2 room is
	// refused before capabilities answer), and only a legacy-lane room key
	// is ever written.
	if h.phong && g.lane == laneLegacy {
		return aiStreamPhong
	}
	return aiStreamNguoiGoi
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
// mirrors `chat_ai_command_scope` (schema_hoi.sql), so a command the table
// would refuse is refused here as a 400 rather than surfacing as a 500 from the
// INSERT. Every command shares one queue, one digest, one rate limit and one
// authority check; only the worker's inference step differs. `hoi` (the
// router decides what is asked, design 03 §4.3) is only an answer in the
// thread: it must name its trigger.
func (h *Handler) lenhNhom(command string, coTrigger bool) bool {
	switch command {
	case lenhPlan, lenhChiaBill:
		return true
	case lenhHoi:
		return coTrigger
	}
	return false
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
		// The `@Rủ Đi` message this answers (ADR-0046). Optional, so an app
		// from before it keeps working. There is no `lane` field on purpose:
		// the lane is the server's finding, and a client sending one is 400.
		TriggerMessageID *string `json:"trigger_message_id"`
	}
	if err := readBody(w, r, &in, maxBodyWithBundle); err != nil {
		failure(w, err)
		return
	}
	if !chatv2.ValidID(in.LogicalID) || !h.lenhNhom(in.Command, in.TriggerMessageID != nil) || strings.TrimSpace(in.Prompt) == "" || !utf8.ValidString(in.Prompt) || utf8.RuneCountInString(in.Prompt) > 4000 || (in.TriggerMessageID != nil && !chatv2.ValidID(*in.TriggerMessageID)) {
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
	available := h.nhomSanSang(r.Context())
	tx, g, err := h.begin(r)
	if err != nil {
		failure(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	if err = phongAi(r.Context(), tx, g); err != nil {
		failure(w, err)
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
		if !h.nhomSanSang(r.Context()) {
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
		// A retry is a new turn the caller pressed for (design 02 §4 step 5):
		// no content out yet, its own model calls, due now. The enqueue
		// trigger numbers it and writes its outbox row in this transaction.
		// attempts<3 still bounds the job as a whole.
		tag, e := tx.Exec(r.Context(), `UPDATE chat_ai_invocations SET status='queued',code=NULL,session_digest=$2,first_token_at=NULL,model_calls=0,available_at=clock_timestamp(),updated_at=clock_timestamp() WHERE id=$1 AND status='failed' AND attempts<3 AND prompt IS NOT NULL AND share_expires_at>clock_timestamp()`, v.ID, g.digest)
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

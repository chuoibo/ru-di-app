package community

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"mobile/services/core/internal/auth"
	"mobile/services/core/internal/brain"
	"mobile/services/core/internal/chatv2"
)

type Handler struct {
	pool           *pgxpool.Pool
	brain          *brain.Client
	mux            *http.ServeMux
	mu             sync.Mutex
	clients        map[*subscriber]struct{}
	origins        []string
	uploadSlots    chan struct{}
	candidateMu    sync.Mutex
	candidateUntil time.Time
	candidates     []candidate
	commonRanking  []string
}

var Patterns = []string{
	"GET /v2/community/feed", "GET /v2/community/posts/{post}", "POST /v2/community/posts",
	"PUT /v2/community/posts/{post}", "DELETE /v2/community/posts/{post}",
	"POST /v2/community/posts/{post}/submit", "PATCH /v2/community/posts/{post}/audience",
	"GET /v2/community/posts/{post}/comments", "POST /v2/community/posts/{post}/comments",
	"DELETE /v2/community/posts/{post}/comments/{comment}",
	"PUT /v2/community/posts/{post}/like", "DELETE /v2/community/posts/{post}/like",
	"PUT /v2/community/posts/{post}/feedback", "GET /v2/community/topics",
	"GET /v2/community/follows", "PUT /v2/community/follows", "DELETE /v2/community/follows",
	"GET /v2/community/preferences", "PUT /v2/community/preferences", "DELETE /v2/community/history",
	"POST /v2/community/interactions", "GET /v2/community/notifications",
	"POST /v2/community/media", "GET /v2/community/media/{media}",
	"GET /v2/community/media/{media}/status",
	"GET /v2/community/review", "POST /v2/community/posts/{post}/review", "POST /v2/community/comments/{comment}/review",
	"POST /v2/community/posts/{post}/appeal", "GET /v2/community/stream",
	"POST /v2/community/posts/{post}/nep",
	"GET /v2/community/search", "GET /v2/community/keeps", "POST /v2/community/keeps", "DELETE /v2/community/keeps/{keep}",
	"POST /v2/community/diaries/{diary}/share",
}

func New(pool *pgxpool.Pool, client *brain.Client, origins []string) *Handler {
	h := &Handler{pool: pool, brain: client, mux: http.NewServeMux(), clients: map[*subscriber]struct{}{}, origins: origins, uploadSlots: make(chan struct{}, 4)}
	h.mux.HandleFunc("GET /v2/community/feed", h.feed)
	h.mux.HandleFunc("GET /v2/community/posts/{post}", h.getPost)
	h.mux.HandleFunc("POST /v2/community/posts", h.createPost)
	h.mux.HandleFunc("PUT /v2/community/posts/{post}", h.editPost)
	h.mux.HandleFunc("DELETE /v2/community/posts/{post}", h.deletePost)
	h.mux.HandleFunc("POST /v2/community/posts/{post}/submit", h.submitPost)
	h.mux.HandleFunc("PATCH /v2/community/posts/{post}/audience", h.audience)
	h.mux.HandleFunc("GET /v2/community/posts/{post}/comments", h.comments)
	h.mux.HandleFunc("POST /v2/community/posts/{post}/comments", h.comment)
	h.mux.HandleFunc("DELETE /v2/community/posts/{post}/comments/{comment}", h.deleteComment)
	h.mux.HandleFunc("PUT /v2/community/posts/{post}/like", h.like)
	h.mux.HandleFunc("DELETE /v2/community/posts/{post}/like", h.like)
	h.mux.HandleFunc("PUT /v2/community/posts/{post}/feedback", h.feedback)
	h.mux.HandleFunc("GET /v2/community/topics", h.topics)
	h.mux.HandleFunc("GET /v2/community/follows", h.follows)
	h.mux.HandleFunc("PUT /v2/community/follows", h.follow)
	h.mux.HandleFunc("DELETE /v2/community/follows", h.follow)
	h.mux.HandleFunc("GET /v2/community/preferences", h.preferences)
	h.mux.HandleFunc("PUT /v2/community/preferences", h.preferences)
	h.mux.HandleFunc("DELETE /v2/community/history", h.clearHistory)
	h.mux.HandleFunc("POST /v2/community/interactions", h.interactions)
	h.mux.HandleFunc("GET /v2/community/notifications", h.notifications)
	h.mux.HandleFunc("POST /v2/community/media", h.upload)
	h.mux.HandleFunc("GET /v2/community/media/{media}", h.media)
	h.mux.HandleFunc("GET /v2/community/media/{media}/status", h.mediaStatus)
	h.mux.HandleFunc("GET /v2/community/review", h.reviewQueue)
	h.mux.HandleFunc("POST /v2/community/posts/{post}/review", h.reviewPost)
	h.mux.HandleFunc("POST /v2/community/comments/{comment}/review", h.reviewComment)
	h.mux.HandleFunc("POST /v2/community/posts/{post}/appeal", h.appeal)
	h.mux.HandleFunc("GET /v2/community/stream", h.stream)
	h.mux.HandleFunc("POST /v2/community/posts/{post}/nep", h.nep)
	h.mux.HandleFunc("GET /v2/community/search", h.search)
	h.mux.HandleFunc("GET /v2/community/keeps", h.keeps)
	h.mux.HandleFunc("POST /v2/community/keeps", h.keep)
	h.mux.HandleFunc("DELETE /v2/community/keeps/{keep}", h.removeKeep)
	h.mux.HandleFunc("POST /v2/community/diaries/{diary}/share", h.shareDiary)
	return h
}
func Matches(path string) bool { return strings.HasPrefix(path, "/v2/community/") }
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	h.mux.ServeHTTP(w, r)
}

type denial struct {
	status int
	code   string
}

func (e *denial) Error() string        { return e.code }
func no(status int, code string) error { return &denial{status, code} }
func reply(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}
func fail(w http.ResponseWriter, err error) {
	var d *denial
	if errors.As(err, &d) {
		reply(w, d.status, map[string]string{"code": d.code})
	} else {
		var database *pgconn.PgError
		if errors.As(err, &database) {
			slog.Error("community database request failed", "sqlstate", database.Code, "constraint", database.ConstraintName)
		}
		reply(w, 503, map[string]string{"code": "community_unavailable"})
	}
}
func uuid() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	s := hex.EncodeToString(b[:])
	return s[:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:]
}
func decode(w http.ResponseWriter, r *http.Request, v any) error {
	if strings.Split(r.Header.Get("Content-Type"), ";")[0] != "application/json" {
		return no(415, "json_required")
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10))
	d.DisallowUnknownFields()
	if d.Decode(v) != nil || d.Decode(new(any)) != io.EOF {
		return no(422, "invalid_request")
	}
	return nil
}
func session(ctx context.Context, tx pgx.Tx, digest []byte) (string, error) {
	var person string
	// Materialize the person lock before locking the session, preserving the
	// deletion/revocation lock order in one round trip. No auth result is cached.
	err := tx.QueryRow(ctx, `WITH identity AS MATERIALIZED (
 SELECT person_id FROM account_sessions WHERE token_digest=$1
), active_person AS MATERIALIZED (
 SELECT p.id FROM people p JOIN identity i ON i.person_id=p.id WHERE p.deleted_at IS NULL FOR SHARE OF p
) SELECT p.id FROM active_person p JOIN account_sessions s ON s.person_id=p.id
 WHERE s.token_digest=$1 AND s.revoked_at IS NULL AND s.expires_at>clock_timestamp() FOR SHARE OF s`, digest).Scan(&person)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", no(401, "authentication_required")
	}
	return person, err
}
func (h *Handler) begin(r *http.Request) (pgx.Tx, string, error) {
	token, p := auth.BearerToken(r.Header)
	if p != nil {
		return nil, "", no(401, "authentication_required")
	}
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		return nil, "", err
	}
	person, err := session(r.Context(), tx, auth.TokenDigest(token))
	if err == nil && r.Method != http.MethodGet {
		_, err = tx.Exec(r.Context(), `SET LOCAL rudi.community_writer = 'on'`)
	}
	if err != nil {
		tx.Rollback(r.Context())
		return nil, "", err
	}
	return tx, person, nil
}

// GuardLegacy prevents older wall clients from changing moderated content through
// an endpoint that does not understand revisions. Reactions remain shared.
func (h *Handler) GuardLegacy(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == "GET" || r.Method == "HEAD" || r.Method == "OPTIONS" {
		return false
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 2 || parts[0] != "posts" || !validID(parts[1]) {
		return false
	}
	if len(parts) > 2 && parts[2] != "comments" {
		return false
	}
	var exists bool
	if err := h.pool.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM community_posts WHERE post_id=$1)`, parts[1]).Scan(&exists); err != nil || !exists {
		return false
	}
	tx, _, err := h.begin(r)
	if err != nil {
		fail(w, err)
		return true
	}
	tx.Rollback(r.Context())
	fail(w, no(409, "community_endpoint_required"))
	return true
}
func commit(w http.ResponseWriter, r *http.Request, tx pgx.Tx, status int, v any) {
	if err := tx.Commit(r.Context()); err != nil {
		fail(w, err)
		return
	}
	reply(w, status, v)
}
func rate(ctx context.Context, tx pgx.Tx, person string) error {
	var n int
	err := tx.QueryRow(ctx, `INSERT INTO community_limits VALUES($1,date_trunc('minute',clock_timestamp()),1) ON CONFLICT(person_id,minute) DO UPDATE SET count=community_limits.count+1 RETURNING count`, person).Scan(&n)
	if err == nil && n > 60 {
		return no(429, "rate_limited")
	}
	return err
}
func emit(ctx context.Context, tx pgx.Tx, post, kind string) error {
	_, err := tx.Exec(ctx, `SELECT community_emit(NULLIF($1,'')::uuid,$2)`, post, kind)
	return err
}
func validID(s string) bool { return chatv2.ValidID(s) }
func hash(v any) string {
	b, _ := json.Marshal(v)
	d := sha256.Sum256(b)
	return hex.EncodeToString(d[:])
}

// readableSQL is evaluated against current relationships, never a saved recipient list.
const readableSQL = `p.author_id=$1 OR (
 (p.audience='group' AND EXISTS(SELECT 1 FROM memberships m WHERE m.context_id=p.context_id AND m.person_id=$1 AND m.state='active' AND m.left_at IS NULL)) OR
 (p.audience IN ('public','friends') AND NOT EXISTS(SELECT 1 FROM friend_requests f WHERE f.state='blocked' AND ((f.requester_id=$1 AND f.addressee_id=p.author_id) OR (f.addressee_id=$1 AND f.requester_id=p.author_id))) AND
 (p.audience='public' OR EXISTS(SELECT 1 FROM friend_requests f WHERE f.state='accepted' AND ((f.requester_id=$1 AND f.addressee_id=p.author_id) OR (f.addressee_id=$1 AND f.requester_id=p.author_id)))))
 )`

func readable(ctx context.Context, tx pgx.Tx, person, id string) (string, error) {
	if !validID(id) {
		return "", no(404, "post_not_found")
	}
	var author string
	err := tx.QueryRow(ctx, `SELECT p.author_id FROM posts p JOIN people a ON a.id=p.author_id LEFT JOIN community_posts c ON c.post_id=p.id WHERE p.id=$2 AND a.deleted_at IS NULL AND c.deleted_at IS NULL AND (`+readableSQL+`)`, person, id).Scan(&author)
	if errors.Is(err, pgx.ErrNoRows) {
		err = no(404, "post_not_found")
	}
	return author, err
}
func owned(ctx context.Context, tx pgx.Tx, person, id string) error {
	author, err := readable(ctx, tx, person, id)
	if err != nil {
		return err
	}
	if author != person {
		return no(404, "post_not_found")
	}
	var locked string
	return tx.QueryRow(ctx, `SELECT id FROM posts WHERE id=$1 FOR UPDATE`, id).Scan(&locked)
}

type Media struct {
	ID       string `json:"id"`
	URL      string `json:"url"`
	Type     string `json:"type"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Duration int    `json:"duration_ms"`
}
type Post struct {
	ID         string          `json:"id"`
	AuthorID   string          `json:"author_id"`
	Author     string          `json:"author"`
	Body       string          `json:"body"`
	Audience   string          `json:"audience"`
	ContextID  *string         `json:"context_id"`
	CreatedAt  time.Time       `json:"created_at"`
	Revision   int             `json:"revision"`
	Status     string          `json:"status"`
	Reason     string          `json:"reason"`
	Topics     []string        `json:"topics"`
	Mentions   []string        `json:"mentions"`
	Media      []Media         `json:"media"`
	Likes      int             `json:"likes"`
	Comments   int             `json:"comments"`
	Liked      bool            `json:"liked"`
	Saved      bool            `json:"saved"`
	Following  bool            `json:"following"`
	CanComment bool            `json:"can_comment"`
	Why        string          `json:"why"`
	Diary      json.RawMessage `json:"diary,omitempty"`
	DiaryKind  string          `json:"diary_kind,omitempty"`
}

func readPost(ctx context.Context, tx pgx.Tx, person, id string) (Post, error) {
	if !validID(id) {
		return Post{}, no(404, "post_not_found")
	}
	posts, err := readPosts(ctx, tx, person, []string{id})
	if err != nil {
		return Post{}, err
	}
	if len(posts) == 0 {
		return Post{}, no(404, "post_not_found")
	}
	return posts[0], nil
}
func readPosts(ctx context.Context, tx pgx.Tx, person string, ids []string) ([]Post, error) {
	out := []Post{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := tx.Query(ctx, `SELECT p.id,p.author_id,a.display_name,CASE WHEN p.author_id=$1 THEN COALESCE(v.body,p.body) ELSE p.body END,CASE WHEN p.author_id=$1 THEN COALESCE(c.requested_audience,p.audience) ELSE p.audience END,p.context_id,p.created_at,COALESCE(c.revision,0),COALESCE(c.status,'legacy'),COALESCE(c.reason,''),CASE WHEN p.author_id=$1 THEN COALESCE(v.topics,c.topics,'{}') ELSE COALESCE(c.topics,'{}') END,CASE WHEN p.author_id=$1 THEN COALESCE(v.mentions,c.mentions,'{}') ELSE COALESCE(c.mentions,'{}') END,
 COALESCE(metrics.likes,(SELECT count(*) FROM post_reactions WHERE post_id=p.id AND kind='heart')),COALESCE(metrics.comments,(SELECT count(*) FROM post_comments WHERE post_id=p.id)),
 EXISTS(SELECT 1 FROM post_reactions WHERE post_id=p.id AND person_id=$1 AND kind='heart'),EXISTS(SELECT 1 FROM community_feedback WHERE post_id=p.id AND person_id=$1 AND kind='saved'),
 EXISTS(SELECT 1 FROM community_follows WHERE person_id=$1 AND kind='person' AND target=p.author_id::text),
	 (p.author_id=$1 OR a.wall_comment_policy='readers' OR (a.wall_comment_policy='friends' AND EXISTS(SELECT 1 FROM friend_requests f WHERE f.state='accepted' AND ((f.requester_id=$1 AND f.addressee_id=p.author_id) OR (f.addressee_id=$1 AND f.requester_id=p.author_id))))),c.diary_document,COALESCE(c.diary_kind,''),
 EXISTS(SELECT 1 FROM community_revisions vm WHERE vm.post_id=p.id AND vm.revision=CASE WHEN p.author_id=$1 THEN c.revision ELSE c.published_revision END AND cardinality(vm.media_ids)>0)
 FROM posts p JOIN people a ON a.id=p.author_id LEFT JOIN community_post_metrics metrics ON metrics.post_id=p.id LEFT JOIN community_posts c ON c.post_id=p.id LEFT JOIN community_revisions v ON v.post_id=p.id AND v.revision=c.revision WHERE p.id=ANY($2::uuid[]) AND a.deleted_at IS NULL AND c.deleted_at IS NULL AND (`+readableSQL+`)`, person, ids)
	if err != nil {
		return nil, err
	}
	positions := map[string]int{}
	mediaPosts := []string{}
	for rows.Next() {
		p := Post{Media: []Media{}}
		var hasMedia bool
		if err = rows.Scan(&p.ID, &p.AuthorID, &p.Author, &p.Body, &p.Audience, &p.ContextID, &p.CreatedAt, &p.Revision, &p.Status, &p.Reason, &p.Topics, &p.Mentions, &p.Likes, &p.Comments, &p.Liked, &p.Saved, &p.Following, &p.CanComment, &p.Diary, &p.DiaryKind, &hasMedia); err != nil {
			break
		}
		if p.AuthorID != person {
			p.Status = "approved"
			p.Reason = ""
		}
		if hasMedia {
			mediaPosts = append(mediaPosts, p.ID)
		}
		positions[p.ID] = len(out)
		out = append(out, p)
	}
	rows.Close()
	if err == nil {
		err = rows.Err()
	}
	if err != nil {
		return nil, err
	}
	if len(mediaPosts) > 0 {
		rows, err = tx.Query(ctx, `SELECT c.post_id,m.id,m.content_type,m.width,m.height,m.duration_ms FROM community_posts c JOIN community_revisions v ON v.post_id=c.post_id AND v.revision=CASE WHEN EXISTS(SELECT 1 FROM posts WHERE id=c.post_id AND author_id=$2) THEN c.revision ELSE c.published_revision END JOIN community_media m ON m.id=ANY(v.media_ids) WHERE c.post_id=ANY($1::uuid[]) ORDER BY c.post_id,array_position(v.media_ids,m.id)`, mediaPosts, person)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id string
			var m Media
			if err = rows.Scan(&id, &m.ID, &m.Type, &m.Width, &m.Height, &m.Duration); err != nil {
				break
			}
			if at, ok := positions[id]; ok {
				m.URL = "/v2/community/media/" + m.ID
				out[at].Media = append(out[at].Media, m)
			}
		}
		rows.Close()
		if err == nil {
			err = rows.Err()
		}
		if err != nil {
			return nil, err
		}
	}
	ordered := []Post{}
	for _, id := range ids {
		if at, ok := positions[id]; ok {
			ordered = append(ordered, out[at])
		}
	}
	return ordered, nil
}
func (h *Handler) getPost(w http.ResponseWriter, r *http.Request) {
	tx, person, err := h.begin(r)
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	p, err := readPost(r.Context(), tx, person, r.PathValue("post"))
	if err != nil {
		fail(w, err)
		return
	}
	commit(w, r, tx, 200, p)
}

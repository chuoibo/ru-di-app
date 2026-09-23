package socialv2

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"mobile/services/core/internal/auth"
	"mobile/services/core/internal/domain/blocking"
	"mobile/services/core/internal/domain/postaudience"
	"mobile/services/core/internal/repo"
)

type Handler struct {
	pool     *pgxpool.Pool
	mode     string
	mux      *http.ServeMux
	mu       sync.Mutex
	watchers map[string]map[chan struct{}]struct{}
}

func Matches(path string) bool { return strings.HasPrefix(path, "/social/v2/") }

func RouteIDs() []string {
	return []string{
		"GET /social/v2/people/{person_id}/posts",
		"GET /social/v2/people/{person_id}/changes",
		"GET /social/v2/posts/{post_id}",
		"GET /social/v2/posts/{post_id}/comments",
		"POST /social/v2/posts/{post_id}/comments",
		"PUT /social/v2/posts/{post_id}/like",
		"DELETE /social/v2/posts/{post_id}/like",
		"PUT /social/v2/comments/{comment_id}/like",
		"DELETE /social/v2/comments/{comment_id}/like",
		"POST /social/v2/posts/{post_id}/repost",
	}
}

func New(pool *pgxpool.Pool, mode string) *Handler {
	h := &Handler{pool: pool, mode: mode, mux: http.NewServeMux(), watchers: map[string]map[chan struct{}]struct{}{}}
	h.mux.HandleFunc("GET /social/v2/people/{person_id}/posts", h.wall)
	h.mux.HandleFunc("GET /social/v2/people/{person_id}/changes", h.changes)
	h.mux.HandleFunc("GET /social/v2/posts/{post_id}", h.detail)
	h.mux.HandleFunc("GET /social/v2/posts/{post_id}/comments", h.comments)
	h.mux.HandleFunc("POST /social/v2/posts/{post_id}/comments", h.createComment)
	h.mux.HandleFunc("PUT /social/v2/posts/{post_id}/like", h.likePost)
	h.mux.HandleFunc("DELETE /social/v2/posts/{post_id}/like", h.unlikePost)
	h.mux.HandleFunc("PUT /social/v2/comments/{comment_id}/like", h.likeComment)
	h.mux.HandleFunc("DELETE /social/v2/comments/{comment_id}/like", h.unlikeComment)
	h.mux.HandleFunc("POST /social/v2/posts/{post_id}/repost", h.repost)
	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	h.mux.ServeHTTP(w, r)
}

type refusal struct {
	status int
	code   string
}

func (e refusal) Error() string         { return e.code }
func bad(status int, code string) error { return refusal{status, code} }
func respond(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
func fail(w http.ResponseWriter, err error) {
	var refused refusal
	if errors.As(err, &refused) {
		respond(w, refused.status, map[string]string{"code": refused.code, "detail": refused.code})
		return
	}
	respond(w, 503, map[string]string{"code": "social_temporarily_unavailable", "detail": "social_temporarily_unavailable"})
}

func (h *Handler) run(w http.ResponseWriter, r *http.Request, fn func(context.Context, pgx.Tx, string) (int, any, error)) {
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback(ctx)
	actor, err := h.actor(ctx, tx, r.Header)
	if err != nil {
		fail(w, err)
		return
	}
	status, body, err := fn(ctx, tx, actor)
	if err != nil {
		fail(w, err)
		return
	}
	if err = tx.Commit(ctx); err != nil {
		fail(w, err)
		return
	}
	respond(w, status, body)
}

func (h *Handler) actor(ctx context.Context, tx pgx.Tx, header http.Header) (string, error) {
	if h.mode == "dev" {
		actor, problem := auth.DevActor(header)
		if problem != nil {
			return "", bad(problem.Status, problem.Code)
		}
		return actor.ID, nil
	}
	actor, problem, err := auth.ProdActor(ctx, header, repo.Sessions{Q: tx}, time.Now().UTC())
	if err != nil {
		return "", err
	}
	if problem != nil {
		return "", bad(problem.Status, problem.Code)
	}
	return actor.ID, nil
}

func parseID(value string) (string, error) {
	id, err := auth.ParsePythonUUID(value)
	if err != nil {
		return "", bad(422, "invalid_id")
	}
	return id, nil
}

func queryLimit(r *http.Request, fallback, max int) (int, error) {
	value := r.URL.Query().Get("limit")
	if value == "" {
		return fallback, nil
	}
	limit, err := strconv.Atoi(value)
	if err != nil || limit < 1 || limit > max {
		return 0, bad(422, "invalid_limit")
	}
	return limit, nil
}

func readJSON(w http.ResponseWriter, r *http.Request, value any) error {
	if strings.Split(r.Header.Get("Content-Type"), ";")[0] != "application/json" {
		return bad(415, "json_required")
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return bad(422, "invalid_body")
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return bad(422, "invalid_body")
	}
	return nil
}

type postAccess struct {
	post           repo.Post
	friend, member bool
}

func visiblePost(ctx context.Context, tx pgx.Tx, postID, actor string) (*postAccess, error) {
	post, err := (repo.Repository{Q: tx}).GetPost(ctx, postID)
	if err != nil {
		return nil, err
	}
	if post == nil {
		return nil, bad(404, "post_not_found")
	}
	access, err := postFacts(ctx, tx, *post, actor)
	if err != nil {
		return nil, err
	}
	if !access.visible {
		return nil, bad(404, "post_not_found")
	}
	return &postAccess{post: *post, friend: access.friend, member: access.member}, nil
}

type facts struct{ visible, friend, member bool }

func postFacts(ctx context.Context, tx pgx.Tx, post repo.Post, actor string) (facts, error) {
	if actor == post.AuthorID {
		return facts{visible: true}, nil
	}
	store := repo.Repository{Q: tx}
	edge, err := store.GetFriendEdge(ctx, actor, post.AuthorID)
	if err != nil {
		return facts{}, err
	}
	friend := edge != nil && edge.State == "accepted"
	blocked := edge != nil && blocking.IsBlocked(&blocking.Edge{State: edge.State, DecidedByID: edge.DecidedByID})
	member := false
	if post.ContextID != nil {
		member, err = store.IsMember(ctx, *post.ContextID, actor)
		if err != nil {
			return facts{}, err
		}
	}
	visible := postaudience.VisibleTo(postaudience.Post{AuthorID: post.AuthorID, Audience: post.Audience, ContextID: post.ContextID}, actor, friend, member, blocked)
	return facts{visible: visible, friend: friend, member: member}, nil
}

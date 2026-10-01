// Package gomdot serves GET /contexts/{context_id}/unbatched-expenses, a
// Go-native read (QA UI-058): how many of a group's recorded expenses a new
// collection round would gather. The settlement screen offered «Tạo đợt thu
// từ sổ» whenever anyone owed anyone, and the server then refused it with
// no_unbatched_allocations once every expense was already in a round.
//
// Native because the batch list it sits beside still has Python as its
// parity oracle: a new field there would be new business logic in Python
// (ADR-0031). The count is the selection moneysteps.FreezeBatch makes, read
// without its row locks (repo.CountUnbatchedExpenses); it is a count, never an
// amount, and changes no money rule.
package gomdot

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"mobile/services/core/internal/auth"
	"mobile/services/core/internal/repo"
)

// Handler answers the one route; Mode is MOBILE_AUTH_MODE ("dev" or "prod").
type Handler struct {
	Pool *pgxpool.Pool
	Mode string
}

func New(pool *pgxpool.Pool, mode string) *Handler { return &Handler{Pool: pool, Mode: mode} }

// RouteIDs names the route for the ownership manifest and its gate.
func RouteIDs() []string { return []string{"GET /contexts/{context_id}/unbatched-expenses"} }

// Matches reserves exactly /contexts/{id}/unbatched-expenses.
func Matches(path string) bool {
	p := strings.Split(strings.Trim(path, "/"), "/")
	return len(p) == 3 && p[0] == "contexts" && p[2] == "unbatched-expenses"
}

var uuidText = regexp.MustCompile(`^[0-9a-fA-F]{8}-?[0-9a-fA-F]{4}-?[0-9a-fA-F]{4}-?[0-9a-fA-F]{4}-?[0-9a-fA-F]{12}$`)

type apiError struct {
	Code   string `json:"code"`
	Detail string `json:"detail"`
}

func answer(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func refuse(w http.ResponseWriter, status int, code, detail string) {
	answer(w, status, apiError{Code: code, Detail: detail})
}

type denial struct {
	status       int
	code, detail string
}

func (d *denial) Error() string { return d.code }

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !Matches(r.URL.Path) {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		refuse(w, 405, "method_not_allowed", "Only GET is served here")
		return
	}
	contextID := strings.Split(strings.Trim(r.URL.Path, "/"), "/")[1]
	if !uuidText.MatchString(contextID) {
		refuse(w, 422, "invalid_context_id", "context_id must be a UUID")
		return
	}
	if h == nil || h.Pool == nil {
		refuse(w, 503, "ledger_unavailable", "The ledger is unavailable")
		return
	}
	ctx := r.Context()
	tx, err := h.Pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		refuse(w, 503, "ledger_unavailable", "The ledger is unavailable")
		return
	}
	defer tx.Rollback(ctx)
	n, err := h.count(ctx, tx, r.Header, contextID)
	if err != nil {
		var d *denial
		if errors.As(err, &d) {
			refuse(w, d.status, d.code, d.detail)
		} else {
			refuse(w, 503, "ledger_unavailable", "The ledger is unavailable")
		}
		return
	}
	answer(w, 200, struct {
		ContextID string `json:"context_id"`
		Count     int    `json:"unbatched_expense_count"`
	}{contextID, n})
}

func (h *Handler) count(ctx context.Context, tx pgx.Tx, header http.Header, contextID string) (int, error) {
	actorID, err := h.actor(ctx, tx, header)
	if err != nil {
		return 0, err
	}
	return countFor(ctx, repo.Repository{Q: tx}, contextID, actorID)
}

// ledger is the two reads the route makes; repo.Repository in production.
type ledger interface {
	IsMember(ctx context.Context, contextID, personID string) (bool, error)
	CountUnbatchedExpenses(ctx context.Context, contextID string) (int, error)
}

// countFor is membership first -- an unknown group and someone else's refuse
// alike, before anything of the ledger is read -- then the count.
func countFor(ctx context.Context, l ledger, contextID, actorID string) (int, error) {
	member, err := l.IsMember(ctx, contextID, actorID)
	if err != nil {
		return 0, err
	}
	if !member {
		return 0, &denial{403, "is_group_member", "Only a member of the group can read its ledger"}
	}
	return l.CountUnbatchedExpenses(ctx, contextID)
}

func (h *Handler) actor(ctx context.Context, tx pgx.Tx, header http.Header) (string, error) {
	if h.Mode == "dev" {
		actor, problem := auth.DevActor(header)
		if problem != nil {
			return "", &denial{problem.Status, problem.Code, problem.Detail}
		}
		return actor.ID, nil
	}
	actor, problem, err := auth.ProdActor(ctx, header, repo.Sessions{Q: tx}, time.Now().UTC())
	if err != nil {
		return "", err
	}
	if problem != nil {
		return "", &denial{problem.Status, problem.Code, problem.Detail}
	}
	return actor.ID, nil
}

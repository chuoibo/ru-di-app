// Package loimoi serves a group invitation to the person it invites (QA
// UI-080): who invited them and how many are already in, read before they
// answer, and a way to say no.
//
//	GET    /contexts/{context_id}/invitation  the actor's open invitation
//	DELETE /contexts/{context_id}/invitation  decline it (invited -> left)
//
// Native because the invited person's only other window on the group is
// GET /people/me/contexts, which still has Python as its parity oracle: a new
// field there would be new business logic in Python (ADR-0031). The invited
// person cannot read the roster (members only), so the inviter's name comes
// from here. Declining touches nothing but the person's own invited row: the
// same UPDATE leave_context makes for an active member, from `invited`; the
// partial unique index on open memberships lets a later invitation start a
// new row.
package loimoi

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

// Handler answers the two routes; Mode is MOBILE_AUTH_MODE ("dev" or "prod").
type Handler struct {
	Pool *pgxpool.Pool
	Mode string
}

func New(pool *pgxpool.Pool, mode string) *Handler { return &Handler{Pool: pool, Mode: mode} }

// RouteIDs names the routes for the ownership manifest and its gate.
func RouteIDs() []string {
	return []string{"GET /contexts/{context_id}/invitation", "DELETE /contexts/{context_id}/invitation"}
}

// Matches reserves exactly /contexts/{id}/invitation.
func Matches(path string) bool {
	p := strings.Split(strings.Trim(path, "/"), "/")
	return len(p) == 3 && p[0] == "contexts" && p[2] == "invitation"
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

// notFound is the one refusal for «no open invitation of yours here»: an
// unknown group, somebody else's, one already answered and one never sent
// read alike, so the route says nothing about a group the actor is not in.
var notFound = &denial{404, "invitation_not_found", "There is no open invitation for you in this group"}

// Person is the inviter as the invited person may see them: an id and the
// name their own profile carries, nothing else.
type Person struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

// Invitation is the answer to GET.
type Invitation struct {
	ContextID   string    `json:"context_id"`
	DisplayName string    `json:"display_name"`
	InvitedBy   *Person   `json:"invited_by"`
	MemberCount int       `json:"member_count"`
	InvitedAt   time.Time `json:"invited_at"`
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !Matches(r.URL.Path) {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodDelete {
		w.Header().Set("Allow", "GET, DELETE")
		refuse(w, 405, "method_not_allowed", "Only GET and DELETE are served here")
		return
	}
	contextID := strings.Split(strings.Trim(r.URL.Path, "/"), "/")[1]
	if !uuidText.MatchString(contextID) {
		refuse(w, 422, "invalid_context_id", "context_id must be a UUID")
		return
	}
	if h == nil || h.Pool == nil {
		refuse(w, 503, "groups_unavailable", "Groups are unavailable")
		return
	}
	ctx := r.Context()
	mode := pgx.ReadOnly
	if r.Method == http.MethodDelete {
		mode = pgx.ReadWrite
	}
	tx, err := h.Pool.BeginTx(ctx, pgx.TxOptions{AccessMode: mode})
	if err != nil {
		refuse(w, 503, "groups_unavailable", "Groups are unavailable")
		return
	}
	defer tx.Rollback(ctx)
	actorID, err := h.actor(ctx, tx, r.Header)
	if err == nil {
		if r.Method == http.MethodGet {
			var inv Invitation
			inv, err = read(ctx, tx, contextID, actorID)
			if err == nil {
				answer(w, 200, inv)
				return
			}
		} else {
			err = decline(ctx, tx, contextID, actorID, time.Now().UTC())
			if err == nil {
				err = tx.Commit(ctx)
			}
			if err == nil {
				w.Header().Set("Cache-Control", "private, no-store")
				w.WriteHeader(204)
				return
			}
		}
	}
	var d *denial
	if errors.As(err, &d) {
		refuse(w, d.status, d.code, d.detail)
		return
	}
	refuse(w, 503, "groups_unavailable", "Groups are unavailable")
}

// read is the actor's open invitation to a group (never a pair: a pair has no
// invitation step), its inviter while their account stands, and how many are
// already in.
func read(ctx context.Context, tx pgx.Tx, contextID, actorID string) (Invitation, error) {
	var inv Invitation
	var inviterID, inviterName *string
	err := tx.QueryRow(ctx, `
SELECT c.id::text, c.display_name, m.created_at, x.id::text, x.display_name,
       (SELECT count(*) FROM memberships a WHERE a.context_id = c.id AND a.state = 'active' AND a.left_at IS NULL)::integer
  FROM memberships m
  JOIN contexts c ON c.id = m.context_id
  LEFT JOIN people x ON x.id = m.invited_by_id AND x.deleted_at IS NULL
 WHERE m.context_id = $1::uuid AND m.person_id = $2::uuid
   AND m.state = 'invited' AND m.left_at IS NULL AND c.kind = 'group'`,
		contextID, actorID).Scan(&inv.ContextID, &inv.DisplayName, &inv.InvitedAt, &inviterID, &inviterName, &inv.MemberCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return Invitation{}, notFound
	}
	if err != nil {
		return Invitation{}, err
	}
	if inviterID != nil && inviterName != nil {
		inv.InvitedBy = &Person{ID: *inviterID, DisplayName: *inviterName}
	}
	inv.InvitedAt = inv.InvitedAt.UTC()
	return inv, nil
}

// decline closes the actor's own open invitation: state and left_at together,
// as ck_memberships_left_state_matches_timestamp asks. Nothing else changes.
func decline(ctx context.Context, tx pgx.Tx, contextID, actorID string, now time.Time) error {
	tag, err := tx.Exec(ctx, `
UPDATE memberships m SET state = 'left', left_at = $3
  FROM contexts c
 WHERE c.id = m.context_id AND c.kind = 'group'
   AND m.context_id = $1::uuid AND m.person_id = $2::uuid
   AND m.state = 'invited' AND m.left_at IS NULL`,
		contextID, actorID, now)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return notFound
	}
	return nil
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

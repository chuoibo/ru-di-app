package avatarfeed

import (
	"context"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"mobile/services/core/internal/auth"
)

var ErrAuthentication = errors.New("authentication_required")

type Store struct{ Pool *pgxpool.Pool }

// visible is view_person_avatar's fact, shares_a_group_with_subject, as
// SharesActiveContext spells it: oneself, or both sides active in one context.
const visible = `(s.id = $1::uuid OR EXISTS (
	SELECT 1 FROM memberships AS m1
	  JOIN memberships AS m2 ON m2.context_id = m1.context_id
	 WHERE m1.person_id = $1::uuid AND m1.state = 'active'
	   AND m2.person_id = s.id AND m2.state = 'active'))`

// latest is get_latest_avatar's order: newest `avatar` row by (created_at, id).
const latest = `(SELECT ui.id::text FROM uploaded_images AS ui
	WHERE ui.owner_person_id = s.id AND ui.purpose = 'avatar'
	ORDER BY ui.created_at DESC, ui.id DESC LIMIT 1)`

// Authenticate resolves a bearer to a live, undeleted person.
func (s Store) Authenticate(ctx context.Context, h http.Header) (string, error) {
	token, problem := auth.BearerToken(h)
	if problem != nil {
		return "", ErrAuthentication
	}
	return s.Actor(ctx, auth.TokenDigest(token))
}

// Actor is Authenticate for a digest the caller already holds; the stream
// re-checks it so a revoked session stops receiving.
func (s Store) Actor(ctx context.Context, digest []byte) (string, error) {
	var actor string
	err := s.Pool.QueryRow(ctx, `SELECT a.person_id::text FROM account_sessions AS a JOIN people AS p ON p.id = a.person_id
		WHERE a.token_digest = $1 AND a.revoked_at IS NULL AND a.expires_at > clock_timestamp() AND p.deleted_at IS NULL`, digest).Scan(&actor)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrAuthentication
	}
	return actor, err
}

// Versions answers, for each id the actor may see, the id of that person's
// current avatar or nil for none. Ids the actor may not see are absent, so the
// answer never says more than GET /people/{id}/avatar would.
func (s Store) Versions(ctx context.Context, actor string, ids []string) (map[string]*string, error) {
	out := map[string]*string{}
	rows, err := s.Pool.Query(ctx, `SELECT s.id::text, `+latest+` FROM unnest($2::uuid[]) AS s(id) WHERE `+visible, actor, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var version *string
		if err = rows.Scan(&id, &version); err != nil {
			return nil, err
		}
		out[id] = version
	}
	return out, rows.Err()
}

// Audience is everyone who may see subject's avatar right now, and which
// avatar is current: the owner and every active co-member.
func (s Store) Audience(ctx context.Context, subject string) ([]string, *string, error) {
	var version *string
	if err := s.Pool.QueryRow(ctx, `SELECT `+latest+` FROM (SELECT $1::uuid AS id) AS s`, subject).Scan(&version); err != nil {
		return nil, nil, err
	}
	rows, err := s.Pool.Query(ctx, `SELECT $1::uuid::text UNION
		SELECT DISTINCT m1.person_id::text FROM memberships AS m1
		  JOIN memberships AS m2 ON m2.context_id = m1.context_id
		 WHERE m2.person_id = $1::uuid AND m2.state = 'active' AND m1.state = 'active'`, subject)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var viewers []string
	for rows.Next() {
		var v string
		if err = rows.Scan(&v); err != nil {
			return nil, nil, err
		}
		viewers = append(viewers, v)
	}
	return viewers, version, rows.Err()
}

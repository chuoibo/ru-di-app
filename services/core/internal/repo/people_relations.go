package repo

// The W10 slice of SqlAlchemyApiRepository that names people and reads what
// lies between two of them: create_person, rename_person, are_friends,
// share_active_context, profile_counts and list_login_providers
// (services/api/app/api/repository.py).

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// personSavepoint is the name SQLAlchemy gives a session's first begin_nested.
const personSavepoint = "sa_savepoint_1"

// CreatePerson is create_person: the people row for an id the caller already
// holds.
//
// SQLAlchemy behaviour reproduced:
//   - SAVEPOINT, then one INSERT of every mapped column that has a value or a
//     Python default: bio, city, budget_band and deleted_at as NULL,
//     discoverable_by_phone true, notify_prefs {} and wall_comment_policy
//     "readers"; created_at is left to the server and read back with
//     RETURNING, so it is the transaction's now();
//   - RELEASE SAVEPOINT on success, ROLLBACK TO SAVEPOINT on any failure;
//   - every IntegrityError becomes Conflict PERSON_ALREADY_EXISTS (the primary
//     key, but a check would read the same); any other error is returned as is.
func (r Repository) CreatePerson(ctx context.Context, personID, displayName string) (Person, error) {
	if _, err := r.Q.Exec(ctx, `SAVEPOINT `+personSavepoint); err != nil {
		return Person{}, err
	}
	p := Person{ID: personID, DisplayName: displayName, DiscoverableByPhone: true, WallCommentPolicy: "readers"}
	insertErr := r.Q.QueryRow(ctx,
		`INSERT INTO people (id, display_name, bio, city, budget_band, discoverable_by_phone, deleted_at, notify_prefs,
		                     wall_comment_policy)
		 VALUES ($1::UUID, $2::VARCHAR, $3::VARCHAR, $4::VARCHAR, $5::VARCHAR, $6, $7::TIMESTAMP WITH TIME ZONE,
		         $8::JSONB, $9::VARCHAR)
		 RETURNING people.created_at`,
		personID, displayName, nil, nil, nil, true, nil, "{}", "readers").Scan(&p.CreatedAt)
	if insertErr != nil {
		if _, err := r.Q.Exec(ctx, `ROLLBACK TO SAVEPOINT `+personSavepoint); err != nil {
			return Person{}, err
		}
		if pg := integrityViolation(insertErr); pg != nil {
			return Person{}, &Conflict{Code: "PERSON_ALREADY_EXISTS", Err: pg}
		}
		return Person{}, insertErr
	}
	if _, err := r.Q.Exec(ctx, `RELEASE SAVEPOINT `+personSavepoint); err != nil {
		return Person{}, err
	}
	p.CreatedAt = p.CreatedAt.UTC()
	return p, nil
}

// RenamePerson is rename_person: `session.get(Person, id, with_for_update=True)`,
// nil when there is none, then an UPDATE of display_name only when the value
// changes. That is exactly UpdatePersonProfile with one key, statement for
// statement, including its note on the identity map.
func (r Repository) RenamePerson(ctx context.Context, personID, displayName string) (*Person, error) {
	name := displayName
	return r.UpdatePersonProfile(ctx, personID, ProfileChanges{DisplayName: &name})
}

// AreFriends is are_friends: an accepted friend_requests row either way round,
// `LIMIT 1`. The same id twice asks the database too (no_self_friendship makes
// the answer false).
func (r Repository) AreFriends(ctx context.Context, a, b string) (bool, error) {
	return r.exists(ctx,
		`SELECT friend_requests.id
		   FROM friend_requests
		  WHERE friend_requests.state = $1
		    AND (friend_requests.requester_id = $2::UUID AND friend_requests.addressee_id = $3::UUID
		         OR friend_requests.requester_id = $4::UUID AND friend_requests.addressee_id = $5::UUID)
		  LIMIT $6::INTEGER`,
		[]any{"accepted", a, b, b, a, 1})
}

// ShareActiveContext is share_active_context (not shares_active_context, the
// photo routes' EXISTS in photos.go): an ACTIVE membership of b in a context
// where a is ACTIVE, `LIMIT 1`. The IN subquery is not correlated (it has one
// FROM, and SQLAlchemy auto-correlates only a subquery with several), pairs
// count as contexts, left_at is not read, and the same id twice asks the
// database.
func (r Repository) ShareActiveContext(ctx context.Context, a, b string) (bool, error) {
	return r.exists(ctx,
		`SELECT memberships.id
		   FROM memberships
		  WHERE memberships.person_id = $1::UUID AND memberships.state = $2
		    AND memberships.context_id IN (SELECT memberships.context_id
		                                     FROM memberships
		                                    WHERE memberships.person_id = $3::UUID AND memberships.state = $4)
		  LIMIT $5::INTEGER`,
		[]any{b, "active", a, "active", 1})
}

// ProfileCounts is ProfileCounts: what GET /people/me shows as numbers.
type ProfileCounts struct {
	Friends         int64
	Contexts        int64
	Outings         int64
	PlacesCheckedIn int64
	Memories        int64
}

// ProfileCounts is profile_counts: five COUNT statements in this order,
// friends (accepted edges either way), active memberships of GROUPS, outings
// of every context with an active membership (pairs included), distinct stops
// checked in at, memories authored. Nothing here looks at deleted_at.
func (r Repository) ProfileCounts(ctx context.Context, personID string) (ProfileCounts, error) {
	var c ProfileCounts
	for _, step := range []struct {
		into *int64
		sql  string
		args []any
	}{
		{&c.Friends, `SELECT count(*) AS count_1
		   FROM friend_requests
		  WHERE (friend_requests.requester_id = $1::UUID OR friend_requests.addressee_id = $2::UUID)
		    AND friend_requests.state = $3`, []any{personID, personID, "accepted"}},
		{&c.Contexts, `SELECT count(*) AS count_1
		   FROM memberships JOIN contexts ON contexts.id = memberships.context_id
		  WHERE memberships.person_id = $1::UUID AND memberships.state = $2 AND contexts.kind = $3::VARCHAR`,
			[]any{personID, "active", "group"}},
		{&c.Outings, `SELECT count(*) AS count_1
		   FROM outings
		  WHERE outings.context_id IN (SELECT memberships.context_id
		                                 FROM memberships
		                                WHERE memberships.person_id = $1::UUID AND memberships.state = $2)`,
			[]any{personID, "active"}},
		{&c.PlacesCheckedIn, `SELECT count(distinct(outing_stop_checkins.stop_id)) AS count_1
		   FROM outing_stop_checkins
		  WHERE outing_stop_checkins.person_id = $1::UUID`, []any{personID}},
		{&c.Memories, `SELECT count(*) AS count_1
		   FROM memories
		  WHERE memories.author_id = $1::UUID`, []any{personID}},
	} {
		if err := r.Q.QueryRow(ctx, step.sql, step.args...).Scan(step.into); err != nil {
			return ProfileCounts{}, err
		}
	}
	return c, nil
}

// ListLoginProviders is list_login_providers: the distinct providers of this
// person's identities, in the column's collation order.
func (r Repository) ListLoginProviders(ctx context.Context, personID string) ([]string, error) {
	rows, err := r.Q.Query(ctx,
		`SELECT DISTINCT account_identities.provider
		   FROM account_identities
		  WHERE account_identities.person_id = $1::UUID
		  ORDER BY account_identities.provider`, personID)
	if err != nil {
		return nil, err
	}
	providers, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, err
	}
	if providers == nil {
		providers = []string{}
	}
	return providers, nil
}

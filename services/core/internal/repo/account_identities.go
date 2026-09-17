package repo

// Binding an external proof to a person (ADR-0016): the two writes behind the
// OTP door and the Google door. get_account_identity, the read half, is in
// friends.go from the wave that ported the profile.
//
// The rule both writes keep is that a proof stays bound to the person it was
// first bound to. A re-login refreshes a timestamp and nothing else; re-pointing
// a proof at another person would be an account merge by side effect, and there
// is no route in the product that asks for one.

import (
	"context"
	"time"
)

var accountIdentityInsert = []insertColumn{{"id", "::UUID"}, {"person_id", "::UUID"},
	{"provider", "::VARCHAR"}, {"subject", "::VARCHAR"}, {"created_at", "::TIMESTAMP WITH TIME ZONE"},
	{"last_login_at", "::TIMESTAMP WITH TIME ZONE"}}

// UpsertAccountIdentity is upsert_account_identity: the binding for this proof,
// then either an INSERT of a new one or an UPDATE of last_login_at.
//
// The read is get_account_identity's statement, without a LIMIT -- the unique
// constraint keeps it to one row. A re-login whose clock equals the stored
// last_login_at changes no column, and SQLAlchemy emits no UPDATE for that.
func (r Repository) UpsertAccountIdentity(ctx context.Context, personID, provider, subject string,
	now time.Time) (AccountIdentity, error) {
	existing, err := r.GetAccountIdentity(ctx, provider, subject)
	if err != nil {
		return AccountIdentity{}, err
	}
	at := pythonInstant(now)
	if existing == nil {
		id, err := newUUID()
		if err != nil {
			return AccountIdentity{}, err
		}
		if _, err := r.Q.Exec(ctx, renderInsert("account_identities", accountIdentityInsert, 1),
			id, personID, provider, subject, at, at); err != nil {
			return AccountIdentity{}, err
		}
		return AccountIdentity{ID: id, PersonID: personID, Provider: provider, Subject: subject,
			CreatedAt: at.UTC(), LastLoginAt: at.UTC()}, nil
	}
	if !existing.LastLoginAt.Equal(at) {
		if err := r.execUpdate(ctx,
			`UPDATE account_identities SET last_login_at=$1::TIMESTAMP WITH TIME ZONE
			  WHERE account_identities.id = $2::UUID`, at, existing.ID); err != nil {
			return AccountIdentity{}, err
		}
		existing.LastLoginAt = at.UTC()
	}
	return *existing, nil
}

// identitySavepoint is the name SQLAlchemy gives a session's first begin_nested.
const identitySavepoint = "sa_savepoint_1"

// CreatePersonWithIdentity is create_person_with_identity: a new person and the
// proof that created them, or neither.
//
// One savepoint around both rows. When two first logins race on the same proof,
// the unique index fails the loser's binding and the savepoint takes the
// loser's `people` row with it; without that, every lost race would leave a
// nameless orphan person nobody can ever sign in as.
//
// The two INSERTs are flushed separately, in that order, because the models
// share a ForeignKey but no relationship(): the unit of work does not know the
// order and would otherwise be free to write the binding before the person it
// points at. Any integrity violation, from either statement, is Conflict
// IDENTITY_ALREADY_BOUND -- the Python catches IntegrityError around the whole
// block and does not ask which row raised it.
func (r Repository) CreatePersonWithIdentity(ctx context.Context, personID, displayName, provider,
	subject string, now time.Time) (AccountIdentity, error) {
	if _, err := r.Q.Exec(ctx, `SAVEPOINT `+identitySavepoint); err != nil {
		return AccountIdentity{}, err
	}
	at := pythonInstant(now)
	identity := AccountIdentity{PersonID: personID, Provider: provider, Subject: subject,
		CreatedAt: at.UTC(), LastLoginAt: at.UTC()}
	written := func() error {
		var createdAt time.Time
		if err := r.Q.QueryRow(ctx,
			`INSERT INTO people (id, display_name, bio, city, budget_band, discoverable_by_phone, deleted_at,
			                     notify_prefs, wall_comment_policy)
			 VALUES ($1::UUID, $2::VARCHAR, $3::VARCHAR, $4::VARCHAR, $5::VARCHAR, $6, $7::TIMESTAMP WITH TIME ZONE,
			         $8::JSONB, $9::VARCHAR)
			 RETURNING people.created_at`,
			personID, displayName, nil, nil, nil, true, nil, "{}", "readers").Scan(&createdAt); err != nil {
			return err
		}
		id, err := newUUID()
		if err != nil {
			return err
		}
		identity.ID = id
		_, err = r.Q.Exec(ctx, renderInsert("account_identities", accountIdentityInsert, 1),
			id, personID, provider, subject, at, at)
		return err
	}()
	if written != nil {
		if _, err := r.Q.Exec(ctx, `ROLLBACK TO SAVEPOINT `+identitySavepoint); err != nil {
			return AccountIdentity{}, err
		}
		if pg := integrityViolation(written); pg != nil {
			return AccountIdentity{}, &Conflict{Code: "IDENTITY_ALREADY_BOUND", Err: pg}
		}
		return AccountIdentity{}, written
	}
	if _, err := r.Q.Exec(ctx, `RELEASE SAVEPOINT `+identitySavepoint); err != nil {
		return AccountIdentity{}, err
	}
	return identity, nil
}

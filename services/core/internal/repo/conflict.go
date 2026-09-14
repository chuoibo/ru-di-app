package repo

import (
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

// Conflict is RepositoryConflict (app/api/errors.py): a persistence invariant
// refused an otherwise well-formed request. Code is the Python code string,
// unchanged, because the service branches on it. Err is what the Python
// exception was raised from (its __cause__): the *pgconn.PgError of the
// violation, a *FriendshipRefusal, or nil when it was raised from nothing.
type Conflict struct {
	Code string
	Err  error
}

func (c *Conflict) Error() string {
	if c.Err != nil {
		return "repo: conflict " + c.Code + ": " + c.Err.Error()
	}
	return "repo: conflict " + c.Code
}

func (c *Conflict) Unwrap() error { return c.Err }

// FriendshipRefusal is app.domain.friendship.FriendshipError: the domain's
// refusal with its stable code.
type FriendshipRefusal struct {
	Code string
}

func (f *FriendshipRefusal) Error() string { return "friendship: " + f.Code }

// Errors a Python method raises as ValueError, RuntimeError or AssertionError
// before or instead of a statement; the route turns each into a 500.
var (
	// ErrUnknownPostAudience is `PostAudience(audience)` refusing a value.
	ErrUnknownPostAudience = errors.New("repo: not a post audience")
	// ErrUnknownFriendRequestState is `FriendRequestState(state)` refusing a value.
	ErrUnknownFriendRequestState = errors.New("repo: not a friend request state")
	// ErrStoryViewVanished is mark_story_seen's RuntimeError.
	ErrStoryViewVanished = errors.New("repo: story view vanished between insert and read")
	// ErrReactionVanished is add_post_reaction's `assert existing is not None`.
	ErrReactionVanished = errors.New("repo: post reaction vanished after a unique violation")
)

// integrityViolation is the *pgconn.PgError behind err when psycopg would raise
// IntegrityError for it (SQLSTATE class 23), nil otherwise.
func integrityViolation(err error) *pgconn.PgError {
	var pg *pgconn.PgError
	if errors.As(err, &pg) && strings.HasPrefix(pg.Code, "23") {
		return pg
	}
	return nil
}

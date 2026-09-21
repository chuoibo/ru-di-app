// Package peoplesteps is the workflow of the thirteen W10 people routes
// (services/api/app/api/routes/people.py): the people methods of
// services/api/app/api/service.py with every repository call behind Store and
// every photo unlink behind PhotoStorage.
//
// Like pairsteps, each method takes the whole Store and makes exactly the
// calls Python makes, in Python's order, with Python's arguments. The rules
// between the calls (permission facts, the refusal each failed predicate
// becomes, the order refusals are checked in, the profile text normalisation,
// the block state machine, when a direct message counts as unavailable) are
// the Python code's. A route implements Store over its transaction and renders
// the returned view.
//
// Errors a method returns:
//
//   - *Refusal is an ApiProblem: status, code and detail;
//   - *permissions.Error is PermissionError_, which Python never catches;
//   - *Conflict is a RepositoryConflict a Store returned and the method did
//     not translate;
//   - *Invariant is a failed `assert`;
//   - *friendship.ValueError is the ValueError an unknown edge state raises;
//   - *friendship.FriendshipError is a FriendshipError the service does not
//     catch (pair_key of two ids that are not uuids; unreachable from a route);
//   - *interests.InterestError is the InterestError a stored interest outside
//     the vocabulary raises;
//   - anything else is the Store's own failure, passed through.
//
// Everything but *Refusal ends the request as Python's 500 does.
//
// Ids are canonical uuid strings (str(uuid.UUID)); the methods only compare
// them for equality and order them with direct.PairKey. `now` is the service
// clock read once per request (`_now()`, datetime.now(UTC)).
//
// testdata/python_people_steps*.json is rendered by
// scripts/render_domain_w10_goldens.py by running the real ApiService methods
// over a recording stub repository and photo storage with the clock pinned;
// oracle_test.go replays every case through a recording Store, comparing the
// answer, every call with its arguments, and what the service logged.
package peoplesteps

import (
	"errors"
	"strings"
	"time"

	"mobile/services/core/internal/domain/blocking"
	"mobile/services/core/internal/domain/permissions"
)

// Actor is the service's Actor: its id and the roles the session carries.
type Actor struct {
	ID    string
	Roles []string
}

// Refusal is an ApiProblem.
type Refusal struct {
	Status int
	Code   string
	Detail string
}

func (r *Refusal) Error() string { return r.Code }

// Conflict is RepositoryConflict: a Store returns it when a persistence
// invariant refuses a write.
type Conflict struct {
	Code string
}

func (c *Conflict) Error() string { return c.Code }

// Invariant is an `assert` of the service that did not hold.
type Invariant struct {
	Reason string
}

func (e *Invariant) Error() string { return "peoplesteps: " + e.Reason }

// Person is PersonRecord.
type Person struct {
	ID                  string
	DisplayName         string
	CreatedAt           time.Time
	Bio                 *string
	City                *string
	BudgetBand          *string
	WallCommentPolicy   string
	DiscoverableByPhone bool
	DeletedAt           *time.Time
}

// LastMessage is LastMessageRecord, and ContextLastMessage.
type LastMessage struct {
	ID                string
	Kind              string
	Preview           string
	AuthorID          *string
	AuthorDisplayName *string
	CreatedAt         time.Time
}

// SummaryRecord is PersonContextSummaryRecord: one conversation as one
// person's list shows it.
type SummaryRecord struct {
	ID                     string
	DisplayName            string
	MemberCount            int64
	MyRole                 string
	MyState                string
	MembershipID           string
	JoinedAt               *time.Time
	LastMessage            *LastMessage
	UnreadCount            int64
	Theme                  string
	Kind                   string
	CounterpartID          *string
	CounterpartDisplayName *string
}

// Context is the part of ContextRecord open_direct_message reads.
type Context struct {
	ID string
}

// FriendEdge is the part of FriendEdgeRecord get_friend_edge answers that the
// methods read.
type FriendEdge struct {
	RequesterID string
	AddresseeID string
	State       string
	DecidedByID *string
}

// BlockedEdge is the part of FriendEdgeRecord list_blocked answers that
// list_blocked_people reads.
type BlockedEdge struct {
	OtherPersonID    string
	OtherDisplayName string
	CreatedAt        time.Time
	DecidedAt        *time.Time
}

// ProfileCounts is ProfileCounts, and ProfileCountsResponse.
type ProfileCounts struct {
	Friends         int64
	Contexts        int64
	Outings         int64
	PlacesCheckedIn int64
	Memories        int64
}

// Place is the part of a catalogue row (`PlaceRecord.to_row()`) a saved-place
// summary reads.
type Place struct {
	Name     string
	Category string
}

// SavedPlace is the part of SavedPlaceRecord a summary reads.
type SavedPlace struct {
	PlaceID   string
	CreatedAt time.Time
}

// ErasureReport is ErasureReport. The service reads only StorageKeys.
type ErasureReport struct {
	Counts      map[string]int64
	StorageKeys []string
}

// Change is one entry of the changes dict update_person_profile receives.
// Value is a string, nil (None: a cleared bio or city) or a bool.
type Change struct {
	Field string
	Value any
}

// SummaryStore is the part of Store that ContextSummaries reads. It is named
// because the session doors of internal/domain/authsteps end in the same
// `_context_summaries` call and must make the same three reads in the same
// order without carrying the rest of the people repository. Store satisfies
// it, so nothing that already holds a Store changes.
type SummaryStore interface {
	GetPerson(personID string) (*Person, error)
	ListPersonContextSummaries(personID string) ([]SummaryRecord, error)
	GetFriendEdge(a, b string) (*FriendEdge, error)
}

// Store is the part of ApiRepository the people methods call, one method per
// repository method with its arguments in the Protocol's order. A write that a
// persistence invariant refuses returns *Conflict with the repository's code.
type Store interface {
	GetPerson(personID string) (*Person, error)
	CreatePerson(personID, displayName string) (Person, error)
	RenamePerson(personID, displayName string) (*Person, error)
	GetPairContext(pairKey string) (*Context, error)
	CreatePairContext(pairKey string, memberIDs [2]string, createdByID string, now time.Time) (Context, error)
	ListPersonContextSummaries(personID string) ([]SummaryRecord, error)
	UpdatePersonProfile(personID string, changes []Change) (*Person, error)
	ProfileCounts(personID string) (ProfileCounts, error)
	ListLoginProviders(personID string) ([]string, error)
	ListPersonInterests(personID string) ([]string, error)
	AreFriends(a, b string) (bool, error)
	ShareActiveContext(a, b string) (bool, error)
	GetPlace(placeID string) (*Place, error)
	ListSavedPlaces(personID string) ([]SavedPlace, error)
	SavePlace(personID, placeID string, now time.Time) (SavedPlace, bool, error)
	UnsavePlace(personID, placeID string) (bool, error)
	OpenBlockEdge(blockerID, addresseeID string, now time.Time) error
	LiftBlockEdge(blockerID, addresseeID string, now time.Time) error
	ListBlocked(personID string) ([]BlockedEdge, error)
	ErasePerson(personID string, now time.Time) (ErasureReport, error)
	GetFriendEdge(a, b string) (*FriendEdge, error)
}

// PhotoStorage is the part of app.media.storage.PhotoStorage the account
// deletion calls. Every error Delete returns is taken for the OSError Python
// catches there: the storage only touches the file system.
type PhotoStorage interface {
	Delete(storageKey string) (bool, error)
}

func refusal(status int, code, detail string) *Refusal {
	return &Refusal{Status: status, Code: code, Detail: detail}
}

// fact is one entry of the context dict `_require_permission` receives.
type fact struct {
	name  string
	holds bool
}

// requirePermission is _require_permission: the facts that hold (`proved is
// True`) are proven, resourceID is `context.get("resource_id")`, and a denial
// is 403 permission_denied with the missing predicate as its detail.
func requirePermission(action string, actor Actor, resourceID *string, facts ...fact) error {
	var proven []string
	for _, f := range facts {
		if f.holds {
			proven = append(proven, f.name)
		}
	}
	reason, allowed, err := permissions.DenialReason(action, permissions.AuthorizationFacts{
		ActorID:    actor.ID,
		Roles:      actor.Roles,
		ResourceID: resourceID,
		Proven:     proven,
		Provenance: "api_service",
	})
	if err != nil {
		return err
	}
	if allowed {
		return nil
	}
	return refusal(403, "permission_denied", reason)
}

// FriendRefusal is ApiService._friend_refusal: a friendship code as an answer
// that does not narrate the graph.
func FriendRefusal(code string) *Refusal {
	switch code {
	case "REQUEST_NOT_OPEN":
		return refusal(409, strings.ToLower(code), "Chưa gửi được lời mời này.")
	case "SELF_EDGE":
		return refusal(422, "self_edge", "Không tự kết bạn với chính mình được.")
	case "ONLY_ADDRESSEE_MAY_ANSWER", "NOT_A_PARTY":
		return refusal(403, "permission_denied", strings.ToLower(code))
	}
	return refusal(409, strings.ToLower(code), "Lời mời không ở trạng thái đó.")
}

// friendEdge is _friend_edge_dict: the pair's live edge as blocking reads it,
// or nil.
func friendEdge(s SummaryStore, a, b string) (*blocking.Edge, error) {
	edge, err := s.GetFriendEdge(a, b)
	if err != nil || edge == nil {
		return nil, err
	}
	return &blocking.Edge{State: edge.State, DecidedByID: edge.DecidedByID}, nil
}

// isBlockedWith is _is_blocked_with: no read for oneself, otherwise the edge.
func isBlockedWith(s Store, readerID, otherID string) (bool, error) {
	if readerID == otherID {
		return false, nil
	}
	edge, err := friendEdge(s, readerID, otherID)
	if err != nil {
		return false, err
	}
	return blocking.IsBlocked(edge), nil
}

// conflictCode is the code of a *Conflict, if err is one.
func conflictCode(err error) (string, bool) {
	var conflict *Conflict
	if errors.As(err, &conflict) {
		return conflict.Code, true
	}
	return "", false
}

// asRefusal reports whether err is a *Refusal.
func asRefusal(err error) bool {
	var refused *Refusal
	return errors.As(err, &refused)
}

// deleted is `record is None or record.deleted_at is not None`.
func deleted(person *Person) bool {
	return person == nil || person.DeletedAt != nil
}

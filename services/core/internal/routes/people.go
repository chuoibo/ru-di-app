package routes

import (
	"context"
	"errors"
	"fmt"
	"time"

	"mobile/services/core/internal/domain/peoplesteps"
	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/pyval"
	"mobile/services/core/internal/repo"
)

// The thirteen routes of app/api/routes/people.py. Each handler reads what
// pyval validated, hands it to the matching peoplesteps method -- the Python
// service method, call for call -- over a Store that is this request's
// transaction, and renders what comes back in the response model's field
// order.
//
// Everything the Python service decides (which permission is asked for, in
// what order, which refusal a failed predicate becomes, when a write is
// skipped) lives in peoplesteps and is pinned by its oracle; everything the
// repository decides lives in repo and is pinned by the SQLAlchemy oracle.
// What is left here is the wire: the values in, the status out, the JSON.
//
// `me` routes are declared before `/people/{person_id}` in Python, and the
// manifest keeps that order, so a literal `me` never reaches the id routes.
// The 201/200 pair of save_place, open_direct_message and register_person is
// the Python route's `response.status_code = ...` after the service answered.

// peopleStore is peoplesteps.Store over one request's repository: one method
// per repository method, in the Protocol's order, translating repo.Conflict
// into the RepositoryConflict the service catches.
type peopleStore struct {
	ctx   context.Context
	store repo.Repository
}

// newPeopleStore opens the request's transaction, as the first statement of
// the Python method does.
func newPeopleStore(ctx context.Context, call *endpoint.Call) (peopleStore, error) {
	store, err := groupStore(ctx, call)
	if err != nil {
		return peopleStore{}, err
	}
	return peopleStore{ctx: ctx, store: store}, nil
}

// peopleConflict is the RepositoryConflict a repository write raises; any
// other failure is passed through as the exception it is.
func peopleConflict(err error) error {
	var conflict *repo.Conflict
	if errors.As(err, &conflict) {
		return &peoplesteps.Conflict{Code: conflict.Code}
	}
	return err
}

// peopleRefusal is the ApiProblem a peoplesteps refusal stands for. Anything
// else ends the request as Python's 500 does.
func peopleRefusal(err error) error {
	var refused *peoplesteps.Refusal
	if errors.As(err, &refused) {
		return endpoint.Refuse(refused.Status, refused.Code, refused.Detail)
	}
	return err
}

func peopleActor(call *endpoint.Call) peoplesteps.Actor {
	return peoplesteps.Actor{ID: call.Actor.ID, Roles: call.Actor.Roles}
}

func (p peopleStore) person(record *repo.Person) *peoplesteps.Person {
	if record == nil {
		return nil
	}
	return &peoplesteps.Person{
		ID: record.ID, DisplayName: record.DisplayName, CreatedAt: record.CreatedAt, Bio: record.Bio,
		City: record.City, BudgetBand: record.BudgetBand, WallCommentPolicy: record.WallCommentPolicy,
		DiscoverableByPhone: record.DiscoverableByPhone, DeletedAt: record.DeletedAt,
	}
}

func (p peopleStore) GetPerson(personID string) (*peoplesteps.Person, error) {
	record, err := p.store.GetPerson(p.ctx, personID)
	if err != nil {
		return nil, err
	}
	return p.person(record), nil
}

func (p peopleStore) CreatePerson(personID, displayName string) (peoplesteps.Person, error) {
	record, err := p.store.CreatePerson(p.ctx, personID, displayName)
	if err != nil {
		return peoplesteps.Person{}, peopleConflict(err)
	}
	return *p.person(&record), nil
}

func (p peopleStore) RenamePerson(personID, displayName string) (*peoplesteps.Person, error) {
	record, err := p.store.RenamePerson(p.ctx, personID, displayName)
	if err != nil {
		return nil, peopleConflict(err)
	}
	return p.person(record), nil
}

func (p peopleStore) GetPairContext(pairKey string) (*peoplesteps.Context, error) {
	record, err := p.store.GetPairContext(p.ctx, pairKey)
	if err != nil || record == nil {
		return nil, err
	}
	return &peoplesteps.Context{ID: record.ID}, nil
}

func (p peopleStore) CreatePairContext(pairKey string, memberIDs [2]string, createdByID string, now time.Time) (peoplesteps.Context, error) {
	record, err := p.store.CreatePairContext(p.ctx, repo.PairContextInput{
		PairKey: pairKey, MemberIDs: memberIDs[:], CreatedByID: createdByID, Now: now,
	})
	if err != nil {
		return peoplesteps.Context{}, peopleConflict(err)
	}
	return peoplesteps.Context{ID: record.ID}, nil
}

func (p peopleStore) ListPersonContextSummaries(personID string) ([]peoplesteps.SummaryRecord, error) {
	rows, err := p.store.ListPersonContextSummaries(p.ctx, personID)
	if err != nil {
		return nil, err
	}
	out := make([]peoplesteps.SummaryRecord, len(rows))
	for i, row := range rows {
		summary := peoplesteps.SummaryRecord{
			ID: row.ID, DisplayName: row.DisplayName, MemberCount: row.MemberCount, MyRole: row.MyRole,
			MyState: row.MyState, MembershipID: row.MembershipID, JoinedAt: row.JoinedAt,
			UnreadCount: row.UnreadCount, Theme: row.Theme, Kind: row.Kind, CounterpartID: row.CounterpartID,
			CounterpartDisplayName: row.CounterpartDisplayName,
		}
		if row.LastMessage != nil {
			summary.LastMessage = &peoplesteps.LastMessage{
				ID: row.LastMessage.ID, Kind: row.LastMessage.Kind, Preview: row.LastMessage.Preview,
				AuthorID: row.LastMessage.AuthorID, AuthorDisplayName: row.LastMessage.AuthorDisplayName,
				CreatedAt: row.LastMessage.CreatedAt,
			}
		}
		out[i] = summary
	}
	return out, nil
}

// UpdatePersonProfile is update_person_profile: the changes dict the service
// built, key by key. A key this port does not know, or a value of a type the
// service never puts there, is a bug in the port and ends the request.
func (p peopleStore) UpdatePersonProfile(personID string, changes []peoplesteps.Change) (*peoplesteps.Person, error) {
	wanted, err := peopleProfileChanges(changes)
	if err != nil {
		return nil, err
	}
	record, err := p.store.UpdatePersonProfile(p.ctx, personID, wanted)
	if err != nil {
		return nil, peopleConflict(err)
	}
	return p.person(record), nil
}

// peopleProfileChanges renders the changes dict as the repository's keyword
// arguments: a key the dict has is a column the flush may write, and one it
// lacks is a column left alone. A cleared bio or city is a present key whose
// value is None.
func peopleProfileChanges(changes []peoplesteps.Change) (repo.ProfileChanges, error) {
	var wanted repo.ProfileChanges
	text := func(change peoplesteps.Change) (repo.OptionalText, error) {
		switch value := change.Value.(type) {
		case nil:
			return repo.SetText(nil), nil
		case string:
			return repo.SetText(&value), nil
		}
		return repo.OptionalText{}, fmt.Errorf("routes: profile change %q is %T, not a string or None", change.Field, change.Value)
	}
	for _, change := range changes {
		switch change.Field {
		case "display_name", "wall_comment_policy":
			value, ok := change.Value.(string)
			if !ok {
				return wanted, fmt.Errorf("routes: profile change %q is %T, not a string", change.Field, change.Value)
			}
			if change.Field == "display_name" {
				wanted.DisplayName = &value
			} else {
				wanted.WallCommentPolicy = &value
			}
		case "bio", "city", "budget_band":
			value, err := text(change)
			if err != nil {
				return wanted, err
			}
			switch change.Field {
			case "bio":
				wanted.Bio = value
			case "city":
				wanted.City = value
			default:
				wanted.BudgetBand = value
			}
		case "discoverable_by_phone":
			value, ok := change.Value.(bool)
			if !ok {
				return wanted, fmt.Errorf("routes: profile change %q is %T, not a bool", change.Field, change.Value)
			}
			wanted.DiscoverableByPhone = &value
		default:
			return wanted, fmt.Errorf("routes: no profile column for %q", change.Field)
		}
	}
	return wanted, nil
}

func (p peopleStore) ProfileCounts(personID string) (peoplesteps.ProfileCounts, error) {
	counts, err := p.store.ProfileCounts(p.ctx, personID)
	if err != nil {
		return peoplesteps.ProfileCounts{}, err
	}
	return peoplesteps.ProfileCounts{
		Friends: counts.Friends, Contexts: counts.Contexts, Outings: counts.Outings,
		PlacesCheckedIn: counts.PlacesCheckedIn, Memories: counts.Memories,
	}, nil
}

func (p peopleStore) ListLoginProviders(personID string) ([]string, error) {
	return p.store.ListLoginProviders(p.ctx, personID)
}

func (p peopleStore) ListPersonInterests(personID string) ([]string, error) {
	return p.store.ListPersonInterests(p.ctx, personID)
}

func (p peopleStore) AreFriends(a, b string) (bool, error) {
	return p.store.AreFriends(p.ctx, a, b)
}

func (p peopleStore) ShareActiveContext(a, b string) (bool, error) {
	return p.store.ShareActiveContext(p.ctx, a, b)
}

func (p peopleStore) GetPlace(placeID string) (*peoplesteps.Place, error) {
	place, err := p.store.GetPlace(p.ctx, placeID)
	if err != nil || place == nil {
		return nil, err
	}
	return &peoplesteps.Place{Name: place.Name, Category: place.Category}, nil
}

func (p peopleStore) ListSavedPlaces(personID string) ([]peoplesteps.SavedPlace, error) {
	rows, err := p.store.ListSavedPlaces(p.ctx, personID)
	if err != nil {
		return nil, err
	}
	out := make([]peoplesteps.SavedPlace, len(rows))
	for i, row := range rows {
		out[i] = peoplesteps.SavedPlace{PlaceID: row.PlaceID, CreatedAt: row.CreatedAt}
	}
	return out, nil
}

func (p peopleStore) SavePlace(personID, placeID string, now time.Time) (peoplesteps.SavedPlace, bool, error) {
	record, created, err := p.store.SavePlace(p.ctx, personID, placeID, now)
	if err != nil {
		return peoplesteps.SavedPlace{}, false, peopleConflict(err)
	}
	return peoplesteps.SavedPlace{PlaceID: record.PlaceID, CreatedAt: record.CreatedAt}, created, nil
}

func (p peopleStore) UnsavePlace(personID, placeID string) (bool, error) {
	removed, err := p.store.UnsavePlace(p.ctx, personID, placeID)
	if err != nil {
		return false, peopleConflict(err)
	}
	return removed, nil
}

// OpenBlockEdge and LiftBlockEdge answer a record the service ignores.
func (p peopleStore) OpenBlockEdge(blockerID, addresseeID string, now time.Time) error {
	_, err := p.store.OpenBlockEdge(p.ctx, blockerID, addresseeID, now)
	return peopleConflict(err)
}

func (p peopleStore) LiftBlockEdge(blockerID, addresseeID string, now time.Time) error {
	_, err := p.store.LiftBlockEdge(p.ctx, blockerID, addresseeID, now)
	return peopleConflict(err)
}

func (p peopleStore) ListBlocked(personID string) ([]peoplesteps.BlockedEdge, error) {
	rows, err := p.store.ListBlocked(p.ctx, personID)
	if err != nil {
		return nil, err
	}
	out := make([]peoplesteps.BlockedEdge, len(rows))
	for i, row := range rows {
		out[i] = peoplesteps.BlockedEdge{
			OtherPersonID: row.OtherPersonID, OtherDisplayName: row.OtherDisplayName,
			CreatedAt: row.CreatedAt, DecidedAt: row.DecidedAt,
		}
	}
	return out, nil
}

// ErasePerson is erase_person, one repository method whose statement order is
// the contract. The service reads only the storage keys; the counts come back
// for completeness, in the order the report lists them.
func (p peopleStore) ErasePerson(personID string, now time.Time) (peoplesteps.ErasureReport, error) {
	report, err := p.store.ErasePerson(p.ctx, personID, now)
	if err != nil {
		return peoplesteps.ErasureReport{}, peopleConflict(err)
	}
	counts := make(map[string]int64, len(report.Counts))
	for _, count := range report.Counts {
		counts[count.Table] = count.Rows
	}
	return peoplesteps.ErasureReport{Counts: counts, StorageKeys: report.StorageKeys}, nil
}

func (p peopleStore) GetFriendEdge(a, b string) (*peoplesteps.FriendEdge, error) {
	edge, err := p.store.GetFriendEdge(p.ctx, a, b)
	if err != nil || edge == nil {
		return nil, err
	}
	return &peoplesteps.FriendEdge{
		RequesterID: edge.RequesterID, AddresseeID: edge.AddresseeID, State: edge.State,
		DecidedByID: edge.DecidedByID,
	}, nil
}

// listMyContexts is GET /people/me/contexts (list_my_contexts).
func listMyContexts() Route {
	return Route{ID: "GET /people/me/contexts", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		store, err := newPeopleStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		summaries, err := peoplesteps.ListMyContexts(store, peopleActor(call))
		if err != nil {
			return endpoint.Reply{}, peopleRefusal(err)
		}
		list := make(pyjson.List, len(summaries))
		for i, summary := range summaries {
			list[i] = wireContextSummary(summary)
		}
		out := pyjson.NewOrderedMap()
		out.Set("contexts", list)
		return endpoint.Reply{Body: out}, nil
	}}
}

// getMyProfile is GET /people/me (get_my_profile).
func getMyProfile() Route {
	return Route{ID: "GET /people/me", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		store, err := newPeopleStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		profile, err := peoplesteps.GetMyProfile(store, peopleActor(call))
		if err != nil {
			return endpoint.Reply{}, peopleRefusal(err)
		}
		return endpoint.Reply{Body: wireProfile(profile)}, nil
	}}
}

// updateMyProfile is PATCH /people/me (update_my_profile): a field the body
// did not name and one sent as null are the same «leave it», which is what
// the model's None default means.
func updateMyProfile() Route {
	return Route{ID: "PATCH /people/me", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		body, err := bodyModel(call, "request")
		if err != nil {
			return endpoint.Reply{}, err
		}
		update := peoplesteps.ProfileUpdate{}
		if update.DisplayName, err = optionalStringField(body, "display_name"); err != nil {
			return endpoint.Reply{}, err
		}
		if update.Bio, err = optionalStringField(body, "bio"); err != nil {
			return endpoint.Reply{}, err
		}
		if update.City, err = optionalStringField(body, "city"); err != nil {
			return endpoint.Reply{}, err
		}
		if update.WallCommentPolicy, err = optionalStringField(body, "wall_comment_policy"); err != nil {
			return endpoint.Reply{}, err
		}
		if update.DiscoverableByPhone, err = optionalBoolField(body, "discoverable_by_phone"); err != nil {
			return endpoint.Reply{}, err
		}
		store, err := newPeopleStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		profile, err := peoplesteps.UpdateMyProfile(store, peopleActor(call), update)
		if err != nil {
			return endpoint.Reply{}, peopleRefusal(err)
		}
		return endpoint.Reply{Body: wireProfile(profile)}, nil
	}}
}

// listSavedPlaces is GET /people/me/saved-places (list_saved_places).
func listSavedPlaces() Route {
	return Route{ID: "GET /people/me/saved-places", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		store, err := newPeopleStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		saved, err := peoplesteps.ListSavedPlaces(store, peopleActor(call))
		if err != nil {
			return endpoint.Reply{}, peopleRefusal(err)
		}
		list := make(pyjson.List, len(saved))
		for i, place := range saved {
			list[i] = wireSavedPlace(place)
		}
		out := pyjson.NewOrderedMap()
		out.Set("saved", list)
		return endpoint.Reply{Body: out}, nil
	}}
}

// savePlace is PUT /people/me/saved-places/{place_id} (save_place): 201 the
// first time, 200 when the bookmark already was there.
func savePlace() Route {
	return Route{ID: "PUT /people/me/saved-places/{place_id}", Status: 201, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		placeID, err := stringParam(call, "place_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := newPeopleStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		summary, created, err := peoplesteps.SavePlace(store, peopleActor(call), placeID, time.Now().UTC())
		if err != nil {
			return endpoint.Reply{}, peopleRefusal(err)
		}
		status := 200
		if created {
			status = 201
		}
		return endpoint.Reply{Status: status, Body: wireSavedPlace(summary)}, nil
	}}
}

// unsavePlace is DELETE /people/me/saved-places/{place_id} (unsave_place):
// 204 whether or not the bookmark was there.
func unsavePlace() Route {
	return Route{ID: "DELETE /people/me/saved-places/{place_id}", Status: 204, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		placeID, err := stringParam(call, "place_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := newPeopleStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if err := peoplesteps.UnsavePlace(store, peopleActor(call), placeID); err != nil {
			return endpoint.Reply{}, peopleRefusal(err)
		}
		return endpoint.Reply{Empty: true}, nil
	}}
}

// listBlocked is GET /people/me/blocked (list_blocked).
func listBlocked() Route {
	return Route{ID: "GET /people/me/blocked", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		store, err := newPeopleStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		blocked, err := peoplesteps.ListBlockedPeople(store, peopleActor(call))
		if err != nil {
			return endpoint.Reply{}, peopleRefusal(err)
		}
		list := make(pyjson.List, len(blocked))
		for i, person := range blocked {
			list[i] = wireBlockedPerson(person)
		}
		out := pyjson.NewOrderedMap()
		out.Set("blocked", list)
		return endpoint.Reply{Body: out}, nil
	}}
}

// deleteMyAccount is DELETE /people/me (delete_my_account): the erasure runs
// inside the request's transaction and the photograph files are unlinked
// after it and BEFORE the commit, as Python does (the docstring of
// erase_person says otherwise; the behaviour is what is reproduced). A failed
// unlink is counted and never fails the request.
func deleteMyAccount() Route {
	return Route{ID: "DELETE /people/me", Status: 204, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		body, err := bodyModel(call, "request")
		if err != nil {
			return endpoint.Reply{}, err
		}
		confirm, err := boolField(body, "confirm")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := newPeopleStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		// What the unlinks did is logged by Python and never sent; nothing on
		// the wire carries it.
		if _, err := peoplesteps.DeleteOwnAccount(store, call.Photos, peopleActor(call), confirm, time.Now().UTC()); err != nil {
			return endpoint.Reply{}, peopleRefusal(err)
		}
		return endpoint.Reply{Empty: true}, nil
	}}
}

// blockPerson is POST /people/{person_id}/block (block_person).
func blockPerson() Route {
	return Route{ID: "POST /people/{person_id}/block", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		personID, err := pathUUID(call, "person_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := newPeopleStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		state, err := peoplesteps.BlockPerson(store, peopleActor(call), personID, time.Now().UTC())
		if err != nil {
			return endpoint.Reply{}, peopleRefusal(err)
		}
		return endpoint.Reply{Body: wireBlockState(state)}, nil
	}}
}

// unblockPerson is DELETE /people/{person_id}/block (unblock_person).
func unblockPerson() Route {
	return Route{ID: "DELETE /people/{person_id}/block", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		personID, err := pathUUID(call, "person_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := newPeopleStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		state, err := peoplesteps.UnblockPerson(store, peopleActor(call), personID, time.Now().UTC())
		if err != nil {
			return endpoint.Reply{}, peopleRefusal(err)
		}
		return endpoint.Reply{Body: wireBlockState(state)}, nil
	}}
}

// openDirectMessage is POST /people/{person_id}/dm (open_direct_message): 201
// when this call made the pair, 200 when it was already there, the same body
// either way.
func openDirectMessage() Route {
	return Route{ID: "POST /people/{person_id}/dm", Status: 201, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		personID, err := pathUUID(call, "person_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := newPeopleStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		summary, created, err := peoplesteps.OpenDirectMessage(store, peopleActor(call), personID, time.Now().UTC())
		if err != nil {
			return endpoint.Reply{}, peopleRefusal(err)
		}
		status := 200
		if created {
			status = 201
		}
		return endpoint.Reply{Status: status, Body: wireContextSummary(summary)}, nil
	}}
}

// getPersonProfile is GET /people/{person_id} (get_person_profile).
func getPersonProfile() Route {
	return Route{ID: "GET /people/{person_id}", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		personID, err := pathUUID(call, "person_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := newPeopleStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		person, err := peoplesteps.GetPersonProfile(store, peopleActor(call), personID)
		if err != nil {
			return endpoint.Reply{}, peopleRefusal(err)
		}
		return endpoint.Reply{Body: wirePublicPerson(person)}, nil
	}}
}

// registerPerson is PUT /people/{person_id} (register_person): 201 when this
// id became a person, 200 when it already was one.
func registerPerson() Route {
	return Route{ID: "PUT /people/{person_id}", Status: 201, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		personID, err := pathUUID(call, "person_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		body, err := bodyModel(call, "request")
		if err != nil {
			return endpoint.Reply{}, err
		}
		displayName, err := stringField(body, "display_name")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := newPeopleStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		record, created, err := peoplesteps.RegisterPerson(store, peopleActor(call), personID, displayName)
		if err != nil {
			return endpoint.Reply{}, peopleRefusal(err)
		}
		status := 200
		if created {
			status = 201
		}
		out := pyjson.NewOrderedMap()
		out.Set("id", pyjson.String(record.ID))
		out.Set("display_name", pyjson.String(record.DisplayName))
		out.Set("created_at", pyjson.String(pyjson.DateTime(record.CreatedAt.UTC())))
		return endpoint.Reply{Status: status, Body: out}, nil
	}}
}

// wireProfile is ProfileResponse: the caller's own profile, fields in
// declaration order with the counts nested in theirs.
func wireProfile(profile peoplesteps.Profile) *pyjson.OrderedMap {
	counts := pyjson.NewOrderedMap()
	counts.Set("friends", pyjson.NewInt(profile.Counts.Friends))
	counts.Set("contexts", pyjson.NewInt(profile.Counts.Contexts))
	counts.Set("outings", pyjson.NewInt(profile.Counts.Outings))
	counts.Set("places_checked_in", pyjson.NewInt(profile.Counts.PlacesCheckedIn))
	counts.Set("memories", pyjson.NewInt(profile.Counts.Memories))
	out := pyjson.NewOrderedMap()
	out.Set("id", pyjson.String(profile.ID))
	out.Set("display_name", pyjson.String(profile.DisplayName))
	out.Set("bio", textOrNull(profile.Bio))
	out.Set("city", textOrNull(profile.City))
	out.Set("created_at", pyjson.String(pyjson.DateTime(profile.CreatedAt.UTC())))
	out.Set("counts", counts)
	out.Set("login_methods", wireStrings(profile.LoginMethods))
	out.Set("interests", wireStrings(profile.Interests))
	out.Set("budget_band", textOrNull(profile.BudgetBand))
	out.Set("wall_comment_policy", pyjson.String(profile.WallCommentPolicy))
	out.Set("discoverable_by_phone", pyjson.Bool(profile.DiscoverableByPhone))
	return out
}

// wirePublicPerson is PublicPersonResponse: no counts, no interests, no
// settings -- the profile is the person's, the view is the reader's.
func wirePublicPerson(person peoplesteps.PublicPerson) *pyjson.OrderedMap {
	out := pyjson.NewOrderedMap()
	out.Set("id", pyjson.String(person.ID))
	out.Set("display_name", pyjson.String(person.DisplayName))
	out.Set("bio", textOrNull(person.Bio))
	out.Set("city", textOrNull(person.City))
	out.Set("created_at", pyjson.String(pyjson.DateTime(person.CreatedAt.UTC())))
	out.Set("relation", pyjson.String(person.Relation))
	return out
}

// wireSavedPlace is SavedPlaceSummary: the bookmark's key and clock, the
// catalogue's name and category.
func wireSavedPlace(place peoplesteps.SavedPlaceSummary) *pyjson.OrderedMap {
	out := pyjson.NewOrderedMap()
	out.Set("place_id", pyjson.String(place.PlaceID))
	out.Set("name", pyjson.String(place.Name))
	out.Set("category", pyjson.String(place.Category))
	out.Set("saved_at", pyjson.String(pyjson.DateTime(place.SavedAt.UTC())))
	return out
}

// wireBlockedPerson is BlockedPersonSummary.
func wireBlockedPerson(person peoplesteps.BlockedPerson) *pyjson.OrderedMap {
	out := pyjson.NewOrderedMap()
	out.Set("person_id", pyjson.String(person.PersonID))
	out.Set("display_name", pyjson.String(person.DisplayName))
	out.Set("blocked_at", pyjson.String(pyjson.DateTime(person.BlockedAt.UTC())))
	return out
}

// wireBlockState is BlockResponse: the edge after the button.
func wireBlockState(state peoplesteps.BlockState) *pyjson.OrderedMap {
	out := pyjson.NewOrderedMap()
	out.Set("person_id", pyjson.String(state.PersonID))
	out.Set("state", pyjson.String(state.State))
	return out
}

// wireContextSummary is ContextSummary, the row of GET /people/me/contexts and
// the whole body of POST /people/{person_id}/dm.
func wireContextSummary(summary peoplesteps.ContextSummary) *pyjson.OrderedMap {
	out := pyjson.NewOrderedMap()
	out.Set("id", pyjson.String(summary.ID))
	out.Set("display_name", pyjson.String(summary.DisplayName))
	out.Set("member_count", pyjson.NewInt(summary.MemberCount))
	out.Set("my_role", pyjson.String(summary.MyRole))
	out.Set("my_state", pyjson.String(summary.MyState))
	out.Set("membership_id", pyjson.String(summary.MembershipID))
	out.Set("joined_at", instantOrNull(summary.JoinedAt))
	if summary.LastMessage == nil {
		out.Set("last_message", pyjson.Null{})
	} else {
		last := pyjson.NewOrderedMap()
		last.Set("id", pyjson.String(summary.LastMessage.ID))
		last.Set("kind", pyjson.String(summary.LastMessage.Kind))
		last.Set("preview", pyjson.String(summary.LastMessage.Preview))
		last.Set("author_id", textOrNull(summary.LastMessage.AuthorID))
		last.Set("author_display_name", textOrNull(summary.LastMessage.AuthorDisplayName))
		last.Set("created_at", pyjson.String(pyjson.DateTime(summary.LastMessage.CreatedAt.UTC())))
		out.Set("last_message", last)
	}
	out.Set("unread_count", pyjson.NewInt(summary.UnreadCount))
	out.Set("theme", pyjson.String(summary.Theme))
	out.Set("kind", pyjson.String(summary.Kind))
	if summary.Counterpart == nil {
		out.Set("counterpart", pyjson.Null{})
	} else {
		counterpart := pyjson.NewOrderedMap()
		counterpart.Set("id", pyjson.String(summary.Counterpart.ID))
		counterpart.Set("display_name", pyjson.String(summary.Counterpart.DisplayName))
		out.Set("counterpart", counterpart)
	}
	out.Set("unavailable", pyjson.Bool(summary.Unavailable))
	return out
}

// optionalBoolField reads a `bool | None` field: absent and null are one
// value here, as the model's None default makes them.
func optionalBoolField(model *pyval.Model, name string) (*bool, error) {
	value, err := field(model, name)
	if err != nil {
		return nil, err
	}
	switch v := value.(type) {
	case pyjson.Null:
		return nil, nil
	case pyjson.Bool:
		flag := bool(v)
		return &flag, nil
	}
	return nil, fmt.Errorf("routes: %s.%s is %T, not a bool or None", model.Class, name, value)
}

func wireStrings(values []string) pyjson.List {
	list := make(pyjson.List, len(values))
	for i, value := range values {
		list[i] = pyjson.String(value)
	}
	return list
}

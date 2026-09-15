//go:build postgres

package repo

// The call sequences of the thirteen W10 routes (routes/people.py), written
// against the Go repository, for the `route.*` steps of
// people_repo_oracle_postgres_test.go. The Python side of such a step runs the
// real ApiService method; this side runs the repository calls that method
// makes, in its order, with the service's decisions (the permission table,
// the friendship and blocking rules, the pair key) taken from the Go domain
// ports. It is test code, not the service port: it exists so the oracle
// proves, statement by statement and file by file, that the Go repository
// called in this order is what the Python route does.
//
// Storage and the service log are part of the statement log, as the Python
// driver writes them: `-- storage.delete <key> -> True|False` or
// `raised <class>[ errno <n>]`, and `-- log <LEVEL> <message>`.

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"syscall"
	"time"

	"mobile/services/core/internal/domain/blocking"
	"mobile/services/core/internal/domain/friendship"
	"mobile/services/core/internal/domain/permissions"
	"mobile/services/core/internal/media/storage"
)

type peopleRoute struct {
	repo  Repository
	rec   *recorder
	actor string
	now   time.Time
	a     map[string]any
	media string
}

func (p *peopleRoute) mark(text string) { p.rec.log = append(p.rec.log, "-- "+text) }

func (p *peopleRoute) text(key string) string { return argString(p.a, key) }

func (p *peopleRoute) body() map[string]any { return p.a["body"].(map[string]any) }

// require is `_require_permission(action, actor, facts)` for a member: the
// facts that are True are proven, and a denial is 403 permission_denied.
func (p *peopleRoute) require(action string, facts map[string]bool, resourceID *string) error {
	var proven []string
	for name, ok := range facts {
		if ok {
			proven = append(proven, name)
		}
	}
	sort.Strings(proven)
	f, err := permissions.NewAuthorizationFacts(p.actor, []string{"member"}, resourceID, proven, "api_service")
	if err != nil {
		return err
	}
	_, allowed, err := permissions.DenialReason(action, f)
	if err != nil {
		return err
	}
	if !allowed {
		return refuse(403, "permission_denied")
	}
	return nil
}

// friendEdge is `_friend_edge_dict(a, b)`: get_friend_edge as the domain reads it.
func (p *peopleRoute) friendEdge(other string) (*blocking.Edge, *FriendEdge, error) {
	edge, err := p.repo.GetFriendEdge(bg, p.actor, other)
	if err != nil || edge == nil {
		return nil, nil, err
	}
	return &blocking.Edge{State: edge.State, DecidedByID: edge.DecidedByID}, edge, nil
}

func (p *peopleRoute) listMyContexts() error {
	if err := p.require("view_own_contexts", map[string]bool{"is_self": true}, nil); err != nil {
		return err
	}
	summaries, err := p.repo.ListPersonContextSummaries(bg, p.actor)
	if err != nil {
		return err
	}
	for _, s := range summaries {
		if s.Kind != "pair" || s.CounterpartID == nil {
			continue
		}
		other, err := p.repo.GetPerson(bg, *s.CounterpartID)
		if err != nil {
			return err
		}
		edge, _, err := p.friendEdge(*s.CounterpartID)
		if err != nil {
			return err
		}
		_ = blocking.DMAllowed(edge, other == nil || other.DeletedAt != nil)
	}
	return nil
}

func (p *peopleRoute) profile(person *Person) error {
	if _, err := p.repo.ProfileCounts(bg, person.ID); err != nil {
		return err
	}
	if _, err := p.repo.ListLoginProviders(bg, person.ID); err != nil {
		return err
	}
	_, err := p.repo.ListPersonInterests(bg, person.ID)
	return err
}

func (p *peopleRoute) getMyProfile() error {
	if err := p.require("view_own_profile", map[string]bool{"is_self": true}, nil); err != nil {
		return err
	}
	person, err := p.repo.GetPerson(bg, p.actor)
	if err != nil {
		return err
	}
	if person == nil {
		return refuse(404, "person_not_found")
	}
	return p.profile(person)
}

func (p *peopleRoute) updateMyProfile() error {
	if err := p.require("edit_own_profile", map[string]bool{"is_self": true}, nil); err != nil {
		return err
	}
	body := p.body()
	var changes ProfileChanges
	if v, ok := body["display_name"].(string); ok {
		name := pyStrip(v)
		changes.DisplayName = &name
	}
	clearable := func(key string) OptionalText {
		v, ok := body[key].(string)
		if !ok {
			return OptionalText{}
		}
		if stripped := pyStrip(v); stripped != "" {
			return SetText(&stripped)
		}
		return SetText(nil)
	}
	changes.Bio, changes.City = clearable("bio"), clearable("city")
	if v, ok := body["wall_comment_policy"].(string); ok {
		changes.WallCommentPolicy = &v
	}
	if v, ok := body["discoverable_by_phone"].(bool); ok {
		changes.DiscoverableByPhone = &v
	}
	person, err := p.repo.UpdatePersonProfile(bg, p.actor, changes)
	if err != nil {
		return err
	}
	if person == nil {
		return refuse(404, "person_not_found")
	}
	return p.profile(person)
}

func (p *peopleRoute) knownPlace() error {
	place, err := p.repo.GetPlace(bg, p.text("place_id"))
	if err != nil {
		return err
	}
	if place == nil {
		return refuse(404, "place_not_found")
	}
	return nil
}

func (p *peopleRoute) deleteOwnAccount() error {
	if err := p.require("delete_own_account", map[string]bool{"is_self": true}, nil); err != nil {
		return err
	}
	if confirm, ok := p.body()["confirm"].(bool); !ok || !confirm {
		return refuse(422, "confirm_required")
	}
	report, err := p.repo.ErasePerson(bg, p.actor, p.now)
	var conflict *Conflict
	if errors.As(err, &conflict) {
		return refuse(404, "person_not_found")
	}
	if err != nil {
		return err
	}
	store, err := storage.NewAt(strings.ReplaceAll(p.text("media_root"), "{media}", p.media))
	if err != nil {
		return err
	}
	removed := 0
	for _, key := range report.StorageKeys {
		found, err := store.Delete(key)
		var errno syscall.Errno
		switch {
		case errors.Is(err, storage.ErrInvalidKey):
			p.mark("storage.delete " + key + " raised ValueError")
			return err
		case errors.As(err, &errno):
			p.mark(fmt.Sprintf("storage.delete %s raised %s errno %d", key, pythonOSErrorName(errno), int(errno)))
			p.mark("log WARNING account.deleted: could not unlink a stored photo")
			continue
		case err != nil:
			return err
		}
		if found {
			removed++
			p.mark("storage.delete " + key + " -> True")
		} else {
			p.mark("storage.delete " + key + " -> False")
		}
	}
	p.mark(fmt.Sprintf("log INFO account.deleted: %d photo file(s) removed of %d", removed, len(report.StorageKeys)))
	return nil
}

// pythonOSErrorName is the OSError subclass CPython raises for an errno the
// unlink of a stored photograph can meet.
func pythonOSErrorName(errno syscall.Errno) string {
	switch errno {
	case syscall.EISDIR:
		return "IsADirectoryError"
	case syscall.EACCES, syscall.EPERM:
		return "PermissionError"
	case syscall.ENOTDIR:
		return "NotADirectoryError"
	case syscall.ENOENT:
		return "FileNotFoundError"
	}
	return "OSError"
}

// friendRefusal is ApiService._friend_refusal.
func friendRefusal(code string) error {
	switch code {
	case friendship.BlockedIsSilent:
		return refuse(409, strings.ToLower(code))
	case friendship.CodeSelfEdge:
		return refuse(422, "self_edge")
	case friendship.CodeOnlyAddresseeMayAnswer, friendship.CodeNotAParty:
		return refuse(403, "permission_denied")
	}
	return refuse(409, strings.ToLower(code))
}

func (p *peopleRoute) blockPerson() error {
	target := p.text("person_id")
	if err := p.require("block_person", map[string]bool{"is_not_self": p.actor != target}, nil); err != nil {
		return err
	}
	person, err := p.repo.GetPerson(bg, target)
	if err != nil {
		return err
	}
	if person == nil {
		return refuse(404, "person_not_found")
	}
	edge, err := p.repo.GetFriendEdge(bg, p.actor, target)
	if err != nil {
		return err
	}
	var existing *friendship.Edge
	if edge != nil {
		existing = &friendship.Edge{RequesterID: edge.RequesterID, AddresseeID: edge.AddresseeID, State: edge.State,
			DecidedByID: edge.DecidedByID}
	}
	_, err = friendship.OpenBlock(p.actor, target, existing)
	var refused *friendship.FriendshipError
	if errors.As(err, &refused) {
		if refused.Code == friendship.CodeAlreadyBlocked {
			return nil
		}
		return friendRefusal(refused.Code)
	}
	if err != nil {
		return err
	}
	_, err = p.repo.OpenBlockEdge(bg, p.actor, target, p.now)
	var conflict *Conflict
	if errors.As(err, &conflict) && conflict.Code == "EDGE_EXISTS" {
		return nil
	}
	return err
}

func (p *peopleRoute) unblockPerson() error {
	target := p.text("person_id")
	edge, _, err := p.friendEdge(target)
	if err != nil {
		return err
	}
	blocker := blocking.BlockerOf(edge)
	if err := p.require("unblock_person", map[string]bool{"is_blocker": blocker != nil && *blocker == p.actor}, nil); err != nil {
		return err
	}
	_, err = p.repo.LiftBlockEdge(bg, p.actor, target, p.now)
	var conflict *Conflict
	if errors.As(err, &conflict) {
		return refuse(409, strings.ToLower(conflict.Code))
	}
	return err
}

func (p *peopleRoute) openDirectMessage() error {
	target := p.text("person_id")
	if target == p.actor {
		return refuse(422, "self_direct_message")
	}
	isFriend, err := p.repo.AreFriends(bg, p.actor, target)
	if err != nil {
		return err
	}
	if err := p.require("open_direct_message", map[string]bool{"is_friend": isFriend}, nil); err != nil {
		var denied *routeRefusal
		if errors.As(err, &denied) {
			return refuse(404, "person_not_found")
		}
		return err
	}
	other, err := p.repo.GetPerson(bg, target)
	if err != nil {
		return err
	}
	if !(isFriend && other != nil && other.DeletedAt == nil) {
		return refuse(404, "person_not_found")
	}
	edge, _, err := p.friendEdge(target)
	if err != nil {
		return err
	}
	if blocking.IsBlocked(edge) {
		return refuse(404, "person_not_found")
	}
	ordered, err := friendship.PairKey(p.actor, target)
	if err != nil {
		return err
	}
	key := ordered[0] + ":" + ordered[1]
	existing, err := p.repo.GetPairContext(bg, key)
	if err != nil {
		return err
	}
	if existing == nil {
		created, err := p.repo.CreatePairContext(bg, PairContextInput{PairKey: key, MemberIDs: []string{p.actor, target},
			CreatedByID: p.actor, Now: p.now})
		var conflict *Conflict
		switch {
		case errors.As(err, &conflict) && conflict.Code == "PAIR_EXISTS":
			if existing, err = p.repo.GetPairContext(bg, key); err != nil {
				return err
			}
			if existing == nil {
				return refuse(409, "pair_exists")
			}
		case err != nil:
			return err
		default:
			existing = &created
		}
	}
	summaries, err := p.repo.ListPersonContextSummaries(bg, p.actor)
	if err != nil {
		return err
	}
	for _, s := range summaries {
		if s.ID == existing.ID {
			return nil
		}
	}
	return refuse(404, "context_not_found")
}

func (p *peopleRoute) getPersonProfile() error {
	target := p.text("person_id")
	visible := target == p.actor
	if !visible {
		friends, err := p.repo.AreFriends(bg, p.actor, target)
		if err != nil {
			return err
		}
		visible = friends
	}
	if !visible {
		shared, err := p.repo.ShareActiveContext(bg, p.actor, target)
		if err != nil {
			return err
		}
		visible = shared
	}
	resource := target
	if err := p.require("view_person_profile", map[string]bool{"is_visible_person": visible}, &resource); err != nil {
		var denied *routeRefusal
		if errors.As(err, &denied) {
			return refuse(403, "person_not_visible")
		}
		return err
	}
	person, err := p.repo.GetPerson(bg, target)
	if err != nil {
		return err
	}
	if person == nil || person.DeletedAt != nil {
		return refuse(404, "person_not_found")
	}
	return nil
}

func (p *peopleRoute) registerPerson() error {
	target, name := p.text("person_id"), p.text("display_name")
	existing, err := p.repo.GetPerson(bg, target)
	if err != nil {
		return err
	}
	if existing != nil && existing.DeletedAt != nil {
		return refuse(404, "person_not_found")
	}
	if existing == nil {
		if err := p.require("register_person_identity", map[string]bool{}, nil); err != nil {
			return err
		}
		_, err := p.repo.CreatePerson(bg, target, name)
		var conflict *Conflict
		if errors.As(err, &conflict) {
			return refuse(409, strings.ToLower(conflict.Code))
		}
		return err
	}
	if existing.DisplayName == name {
		return nil
	}
	if err := p.require("rename_person_identity", map[string]bool{"is_self": p.actor == target}, nil); err != nil {
		return err
	}
	renamed, err := p.repo.RenamePerson(bg, target, name)
	if err != nil {
		return err
	}
	if renamed == nil {
		return refuse(404, "person_not_found")
	}
	return nil
}

func peopleRouteGo(repo Repository, rec *recorder, name string, a map[string]any, media string) error {
	p := &peopleRoute{repo: repo, rec: rec, actor: argString(a, "actor_id"), now: pythonInstant(argInstant(argString(a, "now"))),
		a: a, media: media}
	switch name {
	case "route.list_my_contexts":
		return p.listMyContexts()
	case "route.get_my_profile":
		return p.getMyProfile()
	case "route.update_my_profile":
		return p.updateMyProfile()
	case "route.list_saved_places":
		if err := p.require("manage_saved_places", map[string]bool{"is_self": true}, nil); err != nil {
			return err
		}
		rows, err := repo.ListSavedPlaces(bg, p.actor)
		if err != nil {
			return err
		}
		for _, row := range rows {
			if _, err := repo.GetPlace(bg, row.PlaceID); err != nil {
				return err
			}
		}
		return nil
	case "route.save_place":
		if err := p.require("manage_saved_places", map[string]bool{"is_self": true}, nil); err != nil {
			return err
		}
		if err := p.knownPlace(); err != nil {
			return err
		}
		_, _, err := repo.SavePlace(bg, p.actor, p.text("place_id"), p.now)
		return err
	case "route.unsave_place":
		if err := p.require("manage_saved_places", map[string]bool{"is_self": true}, nil); err != nil {
			return err
		}
		if err := p.knownPlace(); err != nil {
			return err
		}
		_, err := repo.UnsavePlace(bg, p.actor, p.text("place_id"))
		return err
	case "route.list_blocked_people":
		if err := p.require("view_own_blocks", map[string]bool{"is_self": true}, nil); err != nil {
			return err
		}
		_, err := repo.ListBlocked(bg, p.actor)
		return err
	case "route.delete_own_account":
		return p.deleteOwnAccount()
	case "route.block_person":
		return p.blockPerson()
	case "route.unblock_person":
		return p.unblockPerson()
	case "route.open_direct_message":
		return p.openDirectMessage()
	case "route.get_person_profile":
		return p.getPersonProfile()
	case "route.register_person":
		return p.registerPerson()
	}
	panic("unknown route " + name)
}

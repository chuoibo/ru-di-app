package peoplesteps

import (
	"time"

	"mobile/services/core/internal/domain/contexts"
	"mobile/services/core/internal/domain/interests"
)

// Profile is ProfileResponse: the caller's own profile.
type Profile struct {
	ID                  string
	DisplayName         string
	Bio                 *string
	City                *string
	CreatedAt           time.Time
	Counts              ProfileCounts
	LoginMethods        []string
	Interests           []string
	BudgetBand          *string
	WallCommentPolicy   string
	DiscoverableByPhone bool
}

// PublicPerson is PublicPersonResponse: somebody's profile as a friend or a
// groupmate sees it.
type PublicPerson struct {
	ID          string
	DisplayName string
	Bio         *string
	City        *string
	CreatedAt   time.Time
	Relation    string
}

// ProfileUpdate is ProfileUpdateRequest after the schema: nil is a field the
// request did not name.
type ProfileUpdate struct {
	DisplayName         *string
	Bio                 *string
	City                *string
	WallCommentPolicy   *string
	DiscoverableByPhone *bool
}

// ProfileChanges is the changes dict update_my_profile builds, in its order:
// display_name stripped; bio and city stripped, a blank one cleared to nil;
// the policy and the telephone switch as sent. Only named fields appear, so a
// request naming nothing changes nothing and is still written.
func ProfileChanges(request ProfileUpdate) []Change {
	changes := []Change{}
	if request.DisplayName != nil {
		changes = append(changes, Change{"display_name", contexts.Strip(*request.DisplayName)})
	}
	if request.Bio != nil {
		changes = append(changes, Change{"bio", stripOrNone(*request.Bio)})
	}
	if request.City != nil {
		changes = append(changes, Change{"city", stripOrNone(*request.City)})
	}
	if request.WallCommentPolicy != nil {
		changes = append(changes, Change{"wall_comment_policy", *request.WallCommentPolicy})
	}
	if request.DiscoverableByPhone != nil {
		changes = append(changes, Change{"discoverable_by_phone", *request.DiscoverableByPhone})
	}
	return changes
}

// stripOrNone is `value.strip() or None`, as a Change value.
func stripOrNone(value string) any {
	if stripped := contexts.Strip(value); stripped != "" {
		return stripped
	}
	return nil
}

// GetMyProfile is get_my_profile (GET /people/me).
func GetMyProfile(s Store, actor Actor) (Profile, error) {
	if err := requirePermission("view_own_profile", actor, nil, fact{"is_self", true}); err != nil {
		return Profile{}, err
	}
	person, err := s.GetPerson(actor.ID)
	if err != nil {
		return Profile{}, err
	}
	if person == nil {
		return Profile{}, refusal(404, "person_not_found", "Chưa có hồ sơ cho tài khoản này.")
	}
	return profileResponse(s, person)
}

// UpdateMyProfile is update_my_profile (PATCH /people/me).
func UpdateMyProfile(s Store, actor Actor, request ProfileUpdate) (Profile, error) {
	if err := requirePermission("edit_own_profile", actor, nil, fact{"is_self", true}); err != nil {
		return Profile{}, err
	}
	person, err := s.UpdatePersonProfile(actor.ID, ProfileChanges(request))
	if err != nil {
		return Profile{}, err
	}
	if person == nil {
		return Profile{}, refusal(404, "person_not_found", "Chưa có hồ sơ cho tài khoản này.")
	}
	return profileResponse(s, person)
}

// profileResponse is _profile_response: the counts, the login providers and
// the stored interests of the record's own id, in that order. A stored
// interest outside the vocabulary is interests.NormaliseInterests' error.
func profileResponse(s Store, person *Person) (Profile, error) {
	counts, err := s.ProfileCounts(person.ID)
	if err != nil {
		return Profile{}, err
	}
	providers, err := s.ListLoginProviders(person.ID)
	if err != nil {
		return Profile{}, err
	}
	stored, err := s.ListPersonInterests(person.ID)
	if err != nil {
		return Profile{}, err
	}
	tags, err := interests.NormaliseInterests(stored)
	if err != nil {
		return Profile{}, err
	}
	return Profile{
		ID:                  person.ID,
		DisplayName:         person.DisplayName,
		Bio:                 person.Bio,
		City:                person.City,
		CreatedAt:           person.CreatedAt,
		Counts:              counts,
		LoginMethods:        providers,
		Interests:           tags,
		BudgetBand:          person.BudgetBand,
		WallCommentPolicy:   person.WallCommentPolicy,
		DiscoverableByPhone: person.DiscoverableByPhone,
	}, nil
}

// GetPersonProfile is get_person_profile (GET /people/{person_id}): the
// relation is proved before the row is read, so an id nobody may see is 403
// person_not_visible whether or not it names a person.
func GetPersonProfile(s Store, actor Actor, personID string) (PublicPerson, error) {
	relation := ""
	if personID == actor.ID {
		relation = "self"
	} else {
		friends, err := s.AreFriends(actor.ID, personID)
		if err != nil {
			return PublicPerson{}, err
		}
		if friends {
			relation = "friend"
		} else {
			shared, err := s.ShareActiveContext(actor.ID, personID)
			if err != nil {
				return PublicPerson{}, err
			}
			if shared {
				relation = "groupmate"
			}
		}
	}
	resource := personID
	if err := requirePermission("view_person_profile", actor, &resource, fact{"is_visible_person", relation != ""}); err != nil {
		if asRefusal(err) {
			return PublicPerson{}, refusal(403, "person_not_visible", "Không xem được hồ sơ này.")
		}
		return PublicPerson{}, err
	}
	if relation == "" {
		return PublicPerson{}, &Invariant{Reason: "view_person_profile allowed without a relation"}
	}
	person, err := s.GetPerson(personID)
	if err != nil {
		return PublicPerson{}, err
	}
	if deleted(person) {
		return PublicPerson{}, refusal(404, "person_not_found", "Chưa có hồ sơ cho tài khoản này.")
	}
	return PublicPerson{
		ID:          person.ID,
		DisplayName: person.DisplayName,
		Bio:         person.Bio,
		City:        person.City,
		CreatedAt:   person.CreatedAt,
		Relation:    relation,
	}, nil
}

// RegisterPerson is register_person (PUT /people/{person_id}): the record, and
// whether it was created. An ended account is 404; an id with no row is
// created by any member; the same name again is the existing row with no
// permission asked; a different name is a rename only the person may make.
// The name is compared exactly, as `==` compares str.
func RegisterPerson(s Store, actor Actor, personID, displayName string) (Person, bool, error) {
	existing, err := s.GetPerson(personID)
	if err != nil {
		return Person{}, false, err
	}
	if existing != nil && existing.DeletedAt != nil {
		return Person{}, false, refusal(404, "person_not_found", "Chưa có ai dùng số này trong Rủ Đi.")
	}
	if existing == nil {
		if err := requirePermission("register_person_identity", actor, nil); err != nil {
			return Person{}, false, err
		}
		record, err := s.CreatePerson(personID, displayName)
		if err != nil {
			if code, ok := conflictCode(err); ok {
				return Person{}, false, refusal(409, lower(code), "Person identity conflicted")
			}
			return Person{}, false, err
		}
		return record, true, nil
	}
	if existing.DisplayName == displayName {
		return *existing, false, nil
	}
	if err := requirePermission("rename_person_identity", actor, nil, fact{"is_self", actor.ID == personID}); err != nil {
		return Person{}, false, err
	}
	renamed, err := s.RenamePerson(personID, displayName)
	if err != nil {
		return Person{}, false, err
	}
	if renamed == nil {
		return Person{}, false, refusal(404, "person_not_found", "Person disappeared during rename")
	}
	return *renamed, false, nil
}

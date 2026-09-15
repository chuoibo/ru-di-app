package peoplesteps

import "time"

// SavedPlaceSummary is SavedPlaceSummary.
type SavedPlaceSummary struct {
	PlaceID  string
	Name     string
	Category string
	SavedAt  time.Time
}

// ListSavedPlaces is list_saved_places (GET /people/me/saved-places): one
// catalogue read per bookmark, in the repository's order, and a bookmark whose
// place is gone from the catalogue is left out. There is no cap on how many.
func ListSavedPlaces(s Store, actor Actor) ([]SavedPlaceSummary, error) {
	if err := requirePermission("manage_saved_places", actor, nil, fact{"is_self", true}); err != nil {
		return nil, err
	}
	records, err := s.ListSavedPlaces(actor.ID)
	if err != nil {
		return nil, err
	}
	saved := []SavedPlaceSummary{}
	for _, record := range records {
		place, err := s.GetPlace(record.PlaceID)
		if err != nil {
			return nil, err
		}
		if place != nil {
			saved = append(saved, savedPlaceSummary(record, *place))
		}
	}
	return saved, nil
}

// SavePlace is save_place (PUT /people/me/saved-places/{place_id}): the
// bookmark, and whether it is new.
func SavePlace(s Store, actor Actor, placeID string, now time.Time) (SavedPlaceSummary, bool, error) {
	if err := requirePermission("manage_saved_places", actor, nil, fact{"is_self", true}); err != nil {
		return SavedPlaceSummary{}, false, err
	}
	place, err := knownPlace(s, placeID)
	if err != nil {
		return SavedPlaceSummary{}, false, err
	}
	record, created, err := s.SavePlace(actor.ID, placeID, now)
	if err != nil {
		return SavedPlaceSummary{}, false, err
	}
	return savedPlaceSummary(record, place), created, nil
}

// UnsavePlace is unsave_place (DELETE /people/me/saved-places/{place_id}):
// idempotent, and only an unknown catalogue key is refused.
func UnsavePlace(s Store, actor Actor, placeID string) error {
	if err := requirePermission("manage_saved_places", actor, nil, fact{"is_self", true}); err != nil {
		return err
	}
	if _, err := knownPlace(s, placeID); err != nil {
		return err
	}
	_, err := s.UnsavePlace(actor.ID, placeID)
	return err
}

// knownPlace is _known_place over a fresh service: the catalogue row, read
// from the repository, or 404.
func knownPlace(s Store, placeID string) (Place, error) {
	place, err := s.GetPlace(placeID)
	if err != nil {
		return Place{}, err
	}
	if place == nil {
		return Place{}, refusal(404, "place_not_found", "Không có địa điểm này trong danh mục.")
	}
	return *place, nil
}

// savedPlaceSummary is _saved_place_summary: the place id is the record's.
func savedPlaceSummary(record SavedPlace, place Place) SavedPlaceSummary {
	return SavedPlaceSummary{PlaceID: record.PlaceID, Name: place.Name, Category: place.Category, SavedAt: record.CreatedAt}
}

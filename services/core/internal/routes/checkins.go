package routes

import (
	"context"

	"mobile/services/core/internal/domain/socialmap"
	"mobile/services/core/internal/repo"
)

// _CHECKIN_PAGE and _CHECKIN_SCAN_CAP.
const (
	checkinPage    = 100
	checkinScanCap = 500
)

// scanCheckins is ApiService._scan_checkins: every check-in in the group, paged
// through the memory wall's own read, up to a ceiling that is disclosed rather
// than silent. truncated is set only when another page exists and the ceiling
// was reached.
func scanCheckins(ctx context.Context, store repo.Repository, contextID string) ([]socialmap.Checkin, bool, error) {
	kind := "checkin"
	var rows []socialmap.Checkin
	var before *repo.MemoryCursor
	for {
		page, err := store.ListMemories(ctx, contextID, repo.MemoryQuery{Limit: checkinPage, Before: before, Kind: &kind})
		if err != nil {
			return nil, false, err
		}
		for _, memory := range page.Memories {
			rows = append(rows, socialmap.Checkin{PlaceID: memory.PlaceID, PlaceName: memory.PlaceName, Lat: memory.Lat, Lng: memory.Lng})
		}
		if !page.HasMore || len(page.Memories) == 0 {
			return rows, false, nil
		}
		if len(rows) >= checkinScanCap {
			return rows, true, nil
		}
		last := page.Memories[len(page.Memories)-1]
		before = &repo.MemoryCursor{CreatedAt: last.CreatedAt, ID: last.ID}
	}
}

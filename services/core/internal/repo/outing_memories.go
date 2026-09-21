package repo

import "context"

// ListOutingMemories is list_outing_memories: memories on the trip's days,
// with social counts. An unknown outing is an empty tuple.
func (r Repository) ListOutingMemories(ctx context.Context, outingID string, limit int, viewerID *string) ([]Memory, error) {
	// session.get(Outing), not GetOuting: Python does not load stops here.
	outing, err := r.outingByID(ctx, outingID)
	if err != nil || outing == nil {
		return []Memory{}, err
	}
	rows, err := r.Q.Query(ctx,
		`SELECT `+memorySelect+`
		   FROM memories
		  WHERE memories.context_id = $1::UUID
		    AND `+wallClockDate("$2", "memories.created_at")+` BETWEEN $3::DATE AND $4::DATE
		  ORDER BY memories.created_at DESC, memories.id DESC
		  LIMIT $5::INTEGER`,
		outing.ContextID, WallClockZone, outing.StartsOn, outing.EndsOn, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Memory{}
	for rows.Next() {
		m, err := scanMemory(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := r.attachMemorySocial(ctx, out, viewerID); err != nil {
		return nil, err
	}
	return out, nil
}

// GroupPhotosAtPlace is group_photos_at_place: photographs the viewer's own
// ACTIVE groups took at this place. Social counts stay at the record defaults.
func (r Repository) GroupPhotosAtPlace(ctx context.Context, placeID, viewerID string, limit int) ([]Memory, error) {
	rows, err := r.Q.Query(ctx,
		`SELECT `+memorySelect+`
		   FROM memories
		  WHERE memories.place_id = $1::VARCHAR
		    AND memories.kind = $2
		    AND memories.context_id IN (
		        SELECT memberships.context_id FROM memberships
		         WHERE memberships.person_id = $3::UUID
		           AND memberships.state = $4
		           AND memberships.left_at IS NULL)
		  ORDER BY memories.created_at DESC, memories.id DESC
		  LIMIT $5::INTEGER`,
		placeID, "photo", viewerID, "active", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Memory{}
	for rows.Next() {
		m, err := scanMemory(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

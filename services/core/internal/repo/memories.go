package repo

import (
	"context"
	"errors"
	"strconv"
	"time"
)

// Memory is MemoryRecord.
type Memory struct {
	ID               string
	ContextID        string
	AuthorID         string
	Kind             string
	ImageURL         *string
	Caption          *string
	PlaceID          *string
	PlaceName        *string
	Lat              *float64
	Lng              *float64
	CreatedAt        time.Time
	ReactionCount    int64
	CommentCount     int64
	ViewerHasReacted bool
}

// MemoryPage is MemoryPage.
type MemoryPage struct {
	Memories []Memory
	HasMore  bool
}

// MemoryCursor is list_memories' `before` tuple.
type MemoryCursor struct {
	CreatedAt time.Time
	ID        string
}

// MemoryQuery is list_memories' keyword arguments. Nil pointers are "not
// passed"; a pointer to "" is a filter on the empty string, as in Python.
type MemoryQuery struct {
	Limit    int
	Before   *MemoryCursor
	Kind     *string
	PlaceID  *string
	ViewerID *string
}

// ErrUnknownMemoryKind is the ValueError `MemoryKind(kind)` raises before any
// statement runs.
var ErrUnknownMemoryKind = errors.New("repo: not a memory kind")

// ListMemories is list_memories: one context's wall, newest first by
// (created_at DESC, id DESC), a `limit + 1` probe for has_more, then three
// grouped reads for the page's hearts and comments.
//
// Python slicing is kept for limits a route never sends: has_more is
// `len(rows) > limit` and the page is `rows[:limit]`, so limit -1 answers an
// empty page with has_more true, and a limit below -1 reaches PostgreSQL as a
// negative LIMIT and fails there.
func (r Repository) ListMemories(ctx context.Context, contextID string, q MemoryQuery) (MemoryPage, error) {
	if q.Kind != nil && *q.Kind != "photo" && *q.Kind != "checkin" {
		return MemoryPage{}, ErrUnknownMemoryKind
	}
	sql := `SELECT memories.id, memories.context_id, memories.author_id, memories.kind,
	               memories.image_url, memories.caption, memories.place_id, memories.place_name,
	               memories.lat, memories.lng, memories.created_at
	          FROM memories
	         WHERE memories.context_id = $1::UUID`
	args := []any{contextID}
	next := func(value any) string {
		args = append(args, value)
		return "$" + strconv.Itoa(len(args))
	}
	if q.Kind != nil {
		sql += " AND memories.kind = " + next(*q.Kind)
	}
	if q.PlaceID != nil {
		sql += " AND memories.place_id = " + next(*q.PlaceID) + "::VARCHAR"
	}
	if q.Before != nil {
		sql += " AND (memories.created_at, memories.id) < (" +
			next(q.Before.CreatedAt) + "::TIMESTAMP WITH TIME ZONE, " + next(q.Before.ID) + "::UUID)"
	}
	sql += " ORDER BY memories.created_at DESC, memories.id DESC LIMIT " + next(q.Limit+1) + "::INTEGER"

	rows, err := r.Q.Query(ctx, sql, args...)
	if err != nil {
		return MemoryPage{}, err
	}
	var found []Memory
	for rows.Next() {
		var m Memory
		if err := rows.Scan(&m.ID, &m.ContextID, &m.AuthorID, &m.Kind, &m.ImageURL, &m.Caption,
			&m.PlaceID, &m.PlaceName, &m.Lat, &m.Lng, &m.CreatedAt); err != nil {
			rows.Close()
			return MemoryPage{}, err
		}
		m.CreatedAt = m.CreatedAt.UTC()
		found = append(found, m)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return MemoryPage{}, err
	}

	page := MemoryPage{HasMore: len(found) > q.Limit, Memories: []Memory{}}
	page.Memories = append(page.Memories, found[:pythonSliceEnd(len(found), q.Limit)]...)
	if len(page.Memories) == 0 {
		return page, nil
	}
	ids := make([]string, len(page.Memories))
	for i, m := range page.Memories {
		ids[i] = m.ID
	}
	reactions, err := r.countByMemory(ctx, "memory_reactions", ids)
	if err != nil {
		return MemoryPage{}, err
	}
	comments, err := r.countByMemory(ctx, "memory_comments", ids)
	if err != nil {
		return MemoryPage{}, err
	}
	reacted := map[string]bool{}
	if q.ViewerID != nil {
		rows, err := r.Q.Query(ctx,
			`SELECT memory_reactions.memory_id
			   FROM memory_reactions
			  WHERE memory_reactions.memory_id IN (`+uuidPlaceholders(1, len(ids))+`)
			    AND memory_reactions.person_id = $`+strconv.Itoa(len(ids)+1)+`::UUID`,
			append(uuidArgs(ids), *q.ViewerID)...)
		if err != nil {
			return MemoryPage{}, err
		}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return MemoryPage{}, err
			}
			reacted[id] = true
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return MemoryPage{}, err
		}
	}
	for i := range page.Memories {
		m := &page.Memories[i]
		m.ReactionCount = reactions[m.ID]
		m.CommentCount = comments[m.ID]
		m.ViewerHasReacted = reacted[m.ID]
	}
	return page, nil
}

// pythonSliceEnd is the stop index `rows[:limit]` resolves to.
func pythonSliceEnd(length, limit int) int {
	if limit < 0 {
		limit += length
		if limit < 0 {
			return 0
		}
	}
	if limit > length {
		return length
	}
	return limit
}

// countByMemory is one of _memory_social_counts' grouped reads.
func (r Repository) countByMemory(ctx context.Context, table string, ids []string) (map[string]int64, error) {
	rows, err := r.Q.Query(ctx,
		`SELECT `+table+`.memory_id, count(`+table+`.id) AS count_1
		   FROM `+table+`
		  WHERE `+table+`.memory_id IN (`+uuidPlaceholders(1, len(ids))+`)
		  GROUP BY `+table+`.memory_id`,
		uuidArgs(ids)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var id string
		var total int64
		if err := rows.Scan(&id, &total); err != nil {
			return nil, err
		}
		out[id] = total
	}
	return out, rows.Err()
}

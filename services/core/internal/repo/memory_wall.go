package repo

// The W3 writes and single-row reads of the memory wall: create_memory,
// create_checkin, get_context_memory, add_memory_reaction,
// remove_memory_reaction, create_memory_comment and list_memory_comments.
// list_memories is memories.go's, from the pilot wave.

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// MemoryInput is create_memory's keyword arguments.
type MemoryInput struct {
	ContextID string
	AuthorID  string
	ImageURL  string
	Caption   *string
	Now       time.Time
	PlaceID   *string
	PlaceName *string
}

// CheckinInput is create_checkin's keyword arguments.
type CheckinInput struct {
	ContextID string
	AuthorID  string
	PlaceID   string
	PlaceName string
	// Nil when the place has no coordinates. A check-in at such a place still
	// happened; refusing it would stop people checking in at a quarter of the
	// catalogue over a field they never see.
	Lat     *float64
	Lng     *float64
	Caption *string
	Now     time.Time
}

// MemoryReaction is MemoryReactionRecord.
type MemoryReaction struct {
	ID        string
	MemoryID  string
	PersonID  string
	CreatedAt time.Time
}

// MemoryComment is MemoryCommentRecord.
type MemoryComment struct {
	ID        string
	MemoryID  string
	AuthorID  string
	Body      string
	CreatedAt time.Time
}

// insertMemory is the flush of one new Memory: every mapped column listed,
// the unset ones as NULL, lat and lng as untyped float parameters, a
// client-side uuid4 and the caller's clock (no RETURNING).
func (r Repository) insertMemory(ctx context.Context, m Memory) error {
	_, err := r.Q.Exec(ctx,
		`INSERT INTO memories (id, context_id, author_id, kind, image_url, caption, place_id, place_name, lat, lng, created_at)
		 VALUES ($1::UUID, $2::UUID, $3::UUID, $4, $5::VARCHAR, $6::VARCHAR, $7::VARCHAR, $8::VARCHAR, $9, $10,
		         $11::TIMESTAMP WITH TIME ZONE)`,
		m.ID, m.ContextID, m.AuthorID, m.Kind, m.ImageURL, m.Caption, m.PlaceID, m.PlaceName, m.Lat, m.Lng, m.CreatedAt)
	return err
}

// CreateMemory is create_memory: one INSERT of a photo row. The record carries
// the values handed in and zero hearts and comments. payload_matches_kind and
// the foreign keys refuse in PostgreSQL and that *pgconn.PgError is returned
// as is.
func (r Repository) CreateMemory(ctx context.Context, in MemoryInput) (Memory, error) {
	id, err := newUUID()
	if err != nil {
		return Memory{}, err
	}
	imageURL := in.ImageURL
	m := Memory{ID: id, ContextID: in.ContextID, AuthorID: in.AuthorID, Kind: "photo", ImageURL: &imageURL,
		Caption: in.Caption, PlaceID: in.PlaceID, PlaceName: in.PlaceName, CreatedAt: pythonInstant(in.Now)}
	if err := r.insertMemory(ctx, m); err != nil {
		return Memory{}, err
	}
	return m, nil
}

// CreateCheckin is create_checkin: one INSERT of a checkin row, no image,
// with the place's snapshot the caller read. lat_range, lng_range and
// payload_matches_kind refuse in PostgreSQL.
func (r Repository) CreateCheckin(ctx context.Context, in CheckinInput) (Memory, error) {
	id, err := newUUID()
	if err != nil {
		return Memory{}, err
	}
	placeID, placeName := in.PlaceID, in.PlaceName
	m := Memory{ID: id, ContextID: in.ContextID, AuthorID: in.AuthorID, Kind: "checkin", Caption: in.Caption,
		PlaceID: &placeID, PlaceName: &placeName, Lat: in.Lat, Lng: in.Lng, CreatedAt: pythonInstant(in.Now)}
	if err := r.insertMemory(ctx, m); err != nil {
		return Memory{}, err
	}
	return m, nil
}

// GetContextMemory is get_context_memory: the memory by id AND context (a
// memory of another group is nil), then `_memory_social_counts` for that one
// id: the grouped heart count, the grouped comment count, and only when a
// viewer is given whether that viewer left a heart. ApiService never passes a
// viewer.
func (r Repository) GetContextMemory(ctx context.Context, contextID, memoryID string, viewerID *string) (*Memory, error) {
	var m Memory
	err := r.Q.QueryRow(ctx,
		`SELECT memories.id, memories.context_id, memories.author_id, memories.kind, memories.image_url,
		        memories.caption, memories.place_id, memories.place_name, memories.lat, memories.lng,
		        memories.created_at
		   FROM memories
		  WHERE memories.id = $1::UUID AND memories.context_id = $2::UUID`, memoryID, contextID).
		Scan(&m.ID, &m.ContextID, &m.AuthorID, &m.Kind, &m.ImageURL, &m.Caption, &m.PlaceID, &m.PlaceName,
			&m.Lat, &m.Lng, &m.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	m.CreatedAt = m.CreatedAt.UTC()
	ids := []string{m.ID}
	reactions, err := r.countByMemory(ctx, "memory_reactions", ids)
	if err != nil {
		return nil, err
	}
	comments, err := r.countByMemory(ctx, "memory_comments", ids)
	if err != nil {
		return nil, err
	}
	m.ReactionCount, m.CommentCount = reactions[m.ID], comments[m.ID]
	if viewerID != nil {
		rows, err := r.Q.Query(ctx,
			`SELECT memory_reactions.memory_id
			   FROM memory_reactions
			  WHERE memory_reactions.memory_id IN ($1::UUID) AND memory_reactions.person_id = $2::UUID`,
			m.ID, *viewerID)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			m.ViewerHasReacted = true
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	return &m, nil
}

// AddMemoryReaction is add_memory_reaction: the INSERT inside a savepoint
// (SAVEPOINT, INSERT with the caller's clock, RELEASE SAVEPOINT). When the
// INSERT fails the savepoint is rolled back first; a unique violation of
// uq_memory_reactions_person is then Conflict ALREADY_REACTED, and any other
// failure (a missing memory or person, both foreign keys) is returned as is.
func (r Repository) AddMemoryReaction(ctx context.Context, memoryID, personID string, now time.Time) (MemoryReaction, error) {
	id, err := newUUID()
	if err != nil {
		return MemoryReaction{}, err
	}
	created := pythonInstant(now)
	if _, err := r.Q.Exec(ctx, `SAVEPOINT `+reactionSavepoint); err != nil {
		return MemoryReaction{}, err
	}
	_, insertErr := r.Q.Exec(ctx,
		`INSERT INTO memory_reactions (id, memory_id, person_id, created_at)
		 VALUES ($1::UUID, $2::UUID, $3::UUID, $4::TIMESTAMP WITH TIME ZONE)`,
		id, memoryID, personID, created)
	if insertErr != nil {
		if _, err := r.Q.Exec(ctx, `ROLLBACK TO SAVEPOINT `+reactionSavepoint); err != nil {
			return MemoryReaction{}, err
		}
		if pg := integrityViolation(insertErr); pg != nil && pg.ConstraintName == "uq_memory_reactions_person" {
			return MemoryReaction{}, &Conflict{Code: "ALREADY_REACTED", Err: pg}
		}
		return MemoryReaction{}, insertErr
	}
	if _, err := r.Q.Exec(ctx, `RELEASE SAVEPOINT `+reactionSavepoint); err != nil {
		return MemoryReaction{}, err
	}
	return MemoryReaction{ID: id, MemoryID: memoryID, PersonID: personID, CreatedAt: created}, nil
}

// RemoveMemoryReaction is remove_memory_reaction: the person's heart on the
// memory (one_or_none; the unique constraint keeps it to one), then one
// DELETE by primary key; false and no DELETE when there is none.
func (r Repository) RemoveMemoryReaction(ctx context.Context, memoryID, personID string) (bool, error) {
	var reaction MemoryReaction
	err := r.Q.QueryRow(ctx,
		`SELECT memory_reactions.id, memory_reactions.memory_id, memory_reactions.person_id, memory_reactions.created_at
		   FROM memory_reactions
		  WHERE memory_reactions.memory_id = $1::UUID AND memory_reactions.person_id = $2::UUID`,
		memoryID, personID).Scan(&reaction.ID, &reaction.MemoryID, &reaction.PersonID, &reaction.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if _, err := r.Q.Exec(ctx, `DELETE FROM memory_reactions WHERE memory_reactions.id = $1::UUID`, reaction.ID); err != nil {
		return false, err
	}
	return true, nil
}

// CreateMemoryComment is create_memory_comment: one INSERT with a client-side
// uuid4 and the caller's clock. body_not_blank and the foreign keys refuse in
// PostgreSQL.
func (r Repository) CreateMemoryComment(ctx context.Context, memoryID, authorID, body string, now time.Time) (MemoryComment, error) {
	id, err := newUUID()
	if err != nil {
		return MemoryComment{}, err
	}
	created := pythonInstant(now)
	if _, err := r.Q.Exec(ctx,
		`INSERT INTO memory_comments (id, memory_id, author_id, body, created_at)
		 VALUES ($1::UUID, $2::UUID, $3::UUID, $4::VARCHAR, $5::TIMESTAMP WITH TIME ZONE)`,
		id, memoryID, authorID, body, created); err != nil {
		return MemoryComment{}, err
	}
	return MemoryComment{ID: id, MemoryID: memoryID, AuthorID: authorID, Body: body, CreatedAt: created}, nil
}

// ListMemoryComments is list_memory_comments: every comment of the memory,
// oldest first by (created_at, id), no LIMIT and no names.
func (r Repository) ListMemoryComments(ctx context.Context, memoryID string) ([]MemoryComment, error) {
	rows, err := r.Q.Query(ctx,
		`SELECT memory_comments.id, memory_comments.memory_id, memory_comments.author_id, memory_comments.body,
		        memory_comments.created_at
		   FROM memory_comments
		  WHERE memory_comments.memory_id = $1::UUID
		  ORDER BY memory_comments.created_at, memory_comments.id`, memoryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []MemoryComment{}
	for rows.Next() {
		var c MemoryComment
		if err := rows.Scan(&c.ID, &c.MemoryID, &c.AuthorID, &c.Body, &c.CreatedAt); err != nil {
			return nil, err
		}
		c.CreatedAt = c.CreatedAt.UTC()
		out = append(out, c)
	}
	return out, rows.Err()
}

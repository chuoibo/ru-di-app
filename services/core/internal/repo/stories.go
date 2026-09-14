package repo

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// UploadedImage is UploadedImageRecord.
type UploadedImage struct {
	ID            string
	StorageKey    string
	ContextID     *string
	OwnerPersonID *string
	UploadedByID  string
	ContentType   string
	ByteSize      int64
	Width         int64
	Height        int64
	CreatedAt     time.Time
	Purpose       string
}

// GetPersonImage is get_person_image: the person's own `personal` image by id,
// nil for an avatar, a group image, another owner's image or no row.
func (r Repository) GetPersonImage(ctx context.Context, personID, imageID string) (*UploadedImage, error) {
	var m UploadedImage
	err := r.Q.QueryRow(ctx,
		`SELECT uploaded_images.id, uploaded_images.storage_key, uploaded_images.context_id,
		        uploaded_images.owner_person_id, uploaded_images.uploaded_by_id, uploaded_images.purpose,
		        uploaded_images.content_type, uploaded_images.byte_size, uploaded_images.width,
		        uploaded_images.height, uploaded_images.created_at
		   FROM uploaded_images
		  WHERE uploaded_images.owner_person_id = $1::UUID AND uploaded_images.id = $2::UUID
		    AND uploaded_images.purpose = $3::VARCHAR`,
		personID, imageID, "personal").
		Scan(&m.ID, &m.StorageKey, &m.ContextID, &m.OwnerPersonID, &m.UploadedByID, &m.Purpose,
			&m.ContentType, &m.ByteSize, &m.Width, &m.Height, &m.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	m.CreatedAt = m.CreatedAt.UTC()
	return &m, nil
}

// Story is StoryRecord. Seen is relative to a reader and false on a record
// fetched by id or just created.
type Story struct {
	ID                string
	AuthorID          string
	AuthorDisplayName string
	ImageURL          string
	Caption           *string
	Audience          string
	CreatedAt         time.Time
	ExpiresAt         time.Time
	Seen              bool
}

// StoryInput is create_story's keyword arguments.
type StoryInput struct {
	AuthorID  string
	ImageURL  string
	Caption   *string
	Audience  string
	Now       time.Time
	ExpiresAt time.Time
}

// CreateStory is create_story: one INSERT with a client-side uuid4 and the
// caller's clock and deadline, then the author's name. The record carries the
// values handed in, at Python's microsecond precision. The CHECKs (audience,
// deadline after creation, caption length) and the author's foreign key
// refuse in PostgreSQL, and that *pgconn.PgError is returned as is.
func (r Repository) CreateStory(ctx context.Context, in StoryInput) (Story, error) {
	id, err := newUUID()
	if err != nil {
		return Story{}, err
	}
	created, expires := pythonInstant(in.Now), pythonInstant(in.ExpiresAt)
	if _, err := r.Q.Exec(ctx,
		`INSERT INTO stories (id, author_id, image_url, caption, audience, created_at, expires_at)
		 VALUES ($1::UUID, $2::UUID, $3::VARCHAR, $4::VARCHAR, $5::VARCHAR, $6::TIMESTAMP WITH TIME ZONE,
		         $7::TIMESTAMP WITH TIME ZONE)`,
		id, in.AuthorID, in.ImageURL, in.Caption, in.Audience, created, expires); err != nil {
		return Story{}, err
	}
	names, err := r.displayNames(ctx, []string{in.AuthorID})
	if err != nil {
		return Story{}, err
	}
	return Story{ID: id, AuthorID: in.AuthorID, AuthorDisplayName: names[in.AuthorID], ImageURL: in.ImageURL,
		Caption: in.Caption, Audience: in.Audience, CreatedAt: created, ExpiresAt: expires}, nil
}

// storyByID is `session.get(Story, id)`, labelled table_column.
func (r Repository) storyByID(ctx context.Context, storyID string) (*Story, error) {
	var s Story
	err := r.Q.QueryRow(ctx,
		`SELECT stories.id AS stories_id, stories.author_id AS stories_author_id,
		        stories.image_url AS stories_image_url, stories.caption AS stories_caption,
		        stories.audience AS stories_audience, stories.created_at AS stories_created_at,
		        stories.expires_at AS stories_expires_at
		   FROM stories
		  WHERE stories.id = $1::UUID`, storyID).
		Scan(&s.ID, &s.AuthorID, &s.ImageURL, &s.Caption, &s.Audience, &s.CreatedAt, &s.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	s.CreatedAt, s.ExpiresAt = s.CreatedAt.UTC(), s.ExpiresAt.UTC()
	return &s, nil
}

// GetStory is get_story: the story by id whatever its deadline, then its
// author's name.
//
// SQLAlchemy note: session.get answers from the identity map without a
// statement when the same session already loaded the story (delete_story
// after get_story in one request issues no second SELECT in Python).
func (r Repository) GetStory(ctx context.Context, storyID string) (*Story, error) {
	s, err := r.storyByID(ctx, storyID)
	if err != nil || s == nil {
		return nil, err
	}
	names, err := r.displayNames(ctx, []string{s.AuthorID})
	if err != nil {
		return nil, err
	}
	s.AuthorDisplayName = names[s.AuthorID]
	return s, nil
}

// ListLiveStoriesFor is list_live_stories_for: `_story_readable_by(reader,
// now)` and `expires_at > now`, the reader's view row LEFT JOINed for `seen`,
// ordered by author, created_at, id; then one `_display_names` statement for
// the distinct authors (none without rows).
//
// The predicate is SQLAlchemy's rendering, operator precedence included: the
// author's own arm, OR (no block either way AND audience 'friends' AND a
// deadline after now AND an accepted edge either way). The liveness condition
// outside it applies to the reader's own stories too.
func (r Repository) ListLiveStoriesFor(ctx context.Context, readerID string, now time.Time) ([]Story, error) {
	instant := pythonInstant(now)
	rows, err := r.Q.Query(ctx,
		`SELECT stories.id, stories.author_id, stories.image_url, stories.caption, stories.audience,
		        stories.created_at, stories.expires_at, story_views.seen_at
		   FROM stories
		   LEFT OUTER JOIN story_views ON story_views.story_id = stories.id AND story_views.viewer_id = $1::UUID
		  WHERE (stories.author_id = $2::UUID
		         OR NOT (EXISTS (SELECT friend_requests.id FROM friend_requests
		                          WHERE friend_requests.state = $3
		                            AND (friend_requests.requester_id = $4::UUID AND friend_requests.addressee_id = stories.author_id
		                                 OR friend_requests.addressee_id = $5::UUID AND friend_requests.requester_id = stories.author_id)))
		            AND stories.audience = $6::VARCHAR
		            AND stories.expires_at > $7::TIMESTAMP WITH TIME ZONE
		            AND (EXISTS (SELECT friend_requests.id FROM friend_requests
		                          WHERE friend_requests.state = $8
		                            AND (friend_requests.requester_id = $9::UUID AND friend_requests.addressee_id = stories.author_id
		                                 OR friend_requests.addressee_id = $10::UUID AND friend_requests.requester_id = stories.author_id))))
		    AND stories.expires_at > $11::TIMESTAMP WITH TIME ZONE
		  ORDER BY stories.author_id, stories.created_at, stories.id`,
		readerID, readerID, "blocked", readerID, readerID, "friends", instant, "accepted", readerID, readerID, instant)
	if err != nil {
		return nil, err
	}
	out := []Story{}
	for rows.Next() {
		var s Story
		var seenAt *time.Time
		if err := rows.Scan(&s.ID, &s.AuthorID, &s.ImageURL, &s.Caption, &s.Audience, &s.CreatedAt,
			&s.ExpiresAt, &seenAt); err != nil {
			rows.Close()
			return nil, err
		}
		s.CreatedAt, s.ExpiresAt, s.Seen = s.CreatedAt.UTC(), s.ExpiresAt.UTC(), seenAt != nil
		out = append(out, s)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	authors := make([]string, len(out))
	for i, s := range out {
		authors[i] = s.AuthorID
	}
	names, err := r.displayNames(ctx, authors)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].AuthorDisplayName = names[out[i].AuthorID]
	}
	return out, nil
}

// MarkStorySeen is mark_story_seen: INSERT ... ON CONFLICT ON CONSTRAINT
// pk_story_views DO NOTHING, then the stored seen_at read back, so a second
// look answers the first look's time. ON CONFLICT does not cover the foreign
// keys: a missing story or viewer returns that *pgconn.PgError.
func (r Repository) MarkStorySeen(ctx context.Context, storyID, viewerID string, now time.Time) (time.Time, error) {
	if _, err := r.Q.Exec(ctx,
		`INSERT INTO story_views (story_id, viewer_id, seen_at)
		 VALUES ($1::UUID, $2::UUID, $3::TIMESTAMP WITH TIME ZONE)
		 ON CONFLICT ON CONSTRAINT pk_story_views DO NOTHING`,
		storyID, viewerID, pythonInstant(now)); err != nil {
		return time.Time{}, err
	}
	var seenAt time.Time
	err := r.Q.QueryRow(ctx,
		`SELECT story_views.seen_at
		   FROM story_views
		  WHERE story_views.story_id = $1::UUID AND story_views.viewer_id = $2::UUID`,
		storyID, viewerID).Scan(&seenAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, ErrStoryViewVanished
	}
	if err != nil {
		return time.Time{}, err
	}
	return seenAt.UTC(), nil
}

// DeleteStory is delete_story: `session.get` then, when there is a row, one
// DELETE by primary key (story_views go with it by the foreign key's CASCADE).
func (r Repository) DeleteStory(ctx context.Context, storyID string) error {
	s, err := r.storyByID(ctx, storyID)
	if err != nil || s == nil {
		return err
	}
	_, err = r.Q.Exec(ctx, `DELETE FROM stories WHERE stories.id = $1::UUID`, s.ID)
	return err
}

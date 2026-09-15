package repo

// The W6 slice of SqlAlchemyApiRepository: what the six photo routes read and
// write. is_member (api.go) and get_person_image (stories.go) were ported by
// earlier waves; the photo routes reach them too, and
// photos_oracle_postgres_test.go runs them again in the order each route calls.
//
// What the routes do around these calls is the service's and is not here:
// ApiService writes the sanitized bytes to PhotoStorage *before*
// create_uploaded_image, so a failed INSERT leaves a file no row names, and a
// failed write leaves no row. Nothing in this file deletes a file or a row:
// setting an avatar again inserts a newer `avatar` row, and get_latest_avatar
// is what makes it the face.

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// uploadedImageColumns is `select(UploadedImage)`: every mapped column, in the
// model's declaration order, unlabelled.
const uploadedImageColumns = `uploaded_images.id, uploaded_images.storage_key, uploaded_images.context_id,
	uploaded_images.owner_person_id, uploaded_images.uploaded_by_id, uploaded_images.purpose,
	uploaded_images.content_type, uploaded_images.byte_size, uploaded_images.width,
	uploaded_images.height, uploaded_images.created_at`

// scanUploadedImage is `session.scalar(select(UploadedImage)...)` followed by
// `_uploaded_image_record`: nil for no row.
func scanUploadedImage(row pgx.Row) (*UploadedImage, error) {
	var m UploadedImage
	err := row.Scan(&m.ID, &m.StorageKey, &m.ContextID, &m.OwnerPersonID, &m.UploadedByID, &m.Purpose,
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

// UploadedImageInput is create_uploaded_image's keyword arguments. Python's
// `purpose` defaults to "group"; here it has no default, so a caller that means
// a group photo says "group" (an empty Purpose is written as "" and refused by
// ck_uploaded_images_image_purpose_known, as Python's purpose="" would be).
type UploadedImageInput struct {
	StorageKey    string
	ContextID     *string
	OwnerPersonID *string
	UploadedByID  string
	ContentType   string
	ByteSize      int64
	Width         int64
	Height        int64
	Now           time.Time
	Purpose       string
}

// CreateUploadedImage is create_uploaded_image: `session.add` then
// `session.flush()`, which is one INSERT of every column with a client-side
// uuid4 and the caller's clock. created_at has a server default, but a value
// is given, so SQLAlchemy asks for nothing back: no RETURNING, no now().
//
// The record is built from what was handed in, not read back, so CreatedAt is
// Now at Python's microsecond precision in Now's own location (the Python
// record keeps the datetime it was given).
//
// No constraint is mapped to a conflict. The CHECKs (one owner, content type,
// positive size and dimensions, known purpose, purpose matching the owner),
// the unique storage_key and the three foreign keys all refuse in PostgreSQL,
// and that *pgconn.PgError is returned as is, as Python lets IntegrityError
// reach a 500. The INSERT takes FOR KEY SHARE on the contexts and people rows
// its foreign keys name, and no other lock.
func (r Repository) CreateUploadedImage(ctx context.Context, in UploadedImageInput) (UploadedImage, error) {
	id, err := newUUID()
	if err != nil {
		return UploadedImage{}, err
	}
	created := pythonInstant(in.Now)
	if _, err := r.Q.Exec(ctx,
		`INSERT INTO uploaded_images (id, storage_key, context_id, owner_person_id, uploaded_by_id, purpose,
		                              content_type, byte_size, width, height, created_at)
		 VALUES ($1::UUID, $2::VARCHAR, $3::UUID, $4::UUID, $5::UUID, $6::VARCHAR, $7::VARCHAR, $8::INTEGER,
		         $9::INTEGER, $10::INTEGER, $11::TIMESTAMP WITH TIME ZONE)`,
		id, in.StorageKey, in.ContextID, in.OwnerPersonID, in.UploadedByID, in.Purpose, in.ContentType,
		in.ByteSize, in.Width, in.Height, created); err != nil {
		return UploadedImage{}, err
	}
	return UploadedImage{ID: id, StorageKey: in.StorageKey, ContextID: in.ContextID, OwnerPersonID: in.OwnerPersonID,
		UploadedByID: in.UploadedByID, ContentType: in.ContentType, ByteSize: in.ByteSize, Width: in.Width,
		Height: in.Height, CreatedAt: created, Purpose: in.Purpose}, nil
}

// GetContextImage is get_context_image: the image with this id whose
// context_id is this context, nil otherwise. It filters on neither purpose
// (only `group` rows carry a context_id) nor the context's existence.
func (r Repository) GetContextImage(ctx context.Context, contextID, imageID string) (*UploadedImage, error) {
	return scanUploadedImage(r.Q.QueryRow(ctx,
		`SELECT `+uploadedImageColumns+`
		   FROM uploaded_images
		  WHERE uploaded_images.context_id = $1::UUID AND uploaded_images.id = $2::UUID`,
		contextID, imageID))
}

// GetLatestAvatar is get_latest_avatar: the person's newest `avatar` image by
// (created_at DESC, id DESC), nil when they have none. A newer `personal` or
// `group` image is never the avatar. It does not look at people.deleted_at,
// and the newest row is the newest *clock the service passed*, so an avatar
// set with an earlier clock does not replace a later one.
func (r Repository) GetLatestAvatar(ctx context.Context, personID string) (*UploadedImage, error) {
	return scanUploadedImage(r.Q.QueryRow(ctx,
		`SELECT `+uploadedImageColumns+`
		   FROM uploaded_images
		  WHERE uploaded_images.owner_person_id = $1::UUID AND uploaded_images.purpose = $2::VARCHAR
		  ORDER BY uploaded_images.created_at DESC, uploaded_images.id DESC
		  LIMIT $3::INTEGER`,
		personID, "avatar", 1))
}

// SharesActiveContext is shares_active_context. The same id on both sides is
// true without a statement (ids are canonical lowercase UUID strings, so string
// equality is Python's UUID equality). Otherwise one EXISTS over two aliases of
// memberships, both `active`, joined on the context. Unlike IsMember it does
// not test left_at (ck_memberships_left_state_matches_timestamp makes an
// active row's left_at NULL anyway), and it looks at neither blocks, erased
// people nor the context's existence.
func (r Repository) SharesActiveContext(ctx context.Context, viewerID, subjectID string) (bool, error) {
	if viewerID == subjectID {
		return true, nil
	}
	var shared bool
	err := r.Q.QueryRow(ctx,
		`SELECT EXISTS (SELECT memberships_1.id
		                  FROM memberships AS memberships_1
		                  JOIN memberships AS memberships_2 ON memberships_2.context_id = memberships_1.context_id
		                 WHERE memberships_1.person_id = $1::UUID AND memberships_1.state = $2
		                   AND memberships_2.person_id = $3::UUID AND memberships_2.state = $4) AS anon_1`,
		viewerID, "active", subjectID, "active").Scan(&shared)
	if err != nil {
		return false, err
	}
	return shared, nil
}

// PersonImageVisibleTo is person_image_visible_to: whether some post
// `_readable_by` the reader, or else some story `_story_readable_by` the
// reader at now, has image_url exactly "/people/{person}/photos/{image}".
// The post statement runs first and a hit returns without the story
// statement (Python's `or`). The URL is compared as text, so personID and
// imageID must be the canonical lowercase UUID strings Python's str(UUID)
// prints.
//
// Both predicates are the feed's and the rail's SQL, author arm included: a
// reader who authored a post or story showing somebody else's photo can read
// that photo, and the author arm of the story rule has no liveness condition.
// Neither statement checks that the image exists or is `personal`.
func (r Repository) PersonImageVisibleTo(ctx context.Context, personID, imageID, readerID string, now time.Time) (bool, error) {
	url := "/people/" + personID + "/photos/" + imageID

	var postArgs []any
	bind := bindArgs(&postArgs)
	image := bind(url)
	where := readableBy(bind, readerID)
	shown, err := r.exists(ctx, `SELECT posts.id FROM posts WHERE posts.image_url = `+image+`::VARCHAR AND (`+where+`)
		LIMIT `+bind(1)+`::INTEGER`, postArgs)
	if err != nil || shown {
		return shown, err
	}

	var storyArgs []any
	bind = bindArgs(&storyArgs)
	image = bind(url)
	where = storyReadableBy(bind, readerID, pythonInstant(now))
	return r.exists(ctx, `SELECT stories.id FROM stories WHERE stories.image_url = `+image+`::VARCHAR AND (`+where+`)
		LIMIT `+bind(1)+`::INTEGER`, storyArgs)
}

// exists is `session.scalar(select(X.id)...) is not None`.
func (r Repository) exists(ctx context.Context, sql string, args []any) (bool, error) {
	var id string
	err := r.Q.QueryRow(ctx, sql, args...).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// storyReadableBy is `_story_readable_by(reader_id, now)` as SQLAlchemy renders
// it, the predicate ListLiveStoriesFor spells inline: the author's own arm,
// OR (no block either way AND audience 'friends' AND a deadline after now AND
// an accepted edge either way).
func storyReadableBy(bind func(any) string, readerID string, now time.Time) string {
	author := bind(readerID)
	blockedState, blockedRequester, blockedAddressee := bind("blocked"), bind(readerID), bind(readerID)
	audience, deadline := bind("friends"), bind(now)
	acceptedState, acceptedRequester, acceptedAddressee := bind("accepted"), bind(readerID), bind(readerID)
	return `stories.author_id = ` + author + `::UUID
	     OR NOT (EXISTS (SELECT friend_requests.id FROM friend_requests
	                      WHERE friend_requests.state = ` + blockedState + `
	                        AND (friend_requests.requester_id = ` + blockedRequester + `::UUID AND friend_requests.addressee_id = stories.author_id
	                             OR friend_requests.addressee_id = ` + blockedAddressee + `::UUID AND friend_requests.requester_id = stories.author_id)))
	        AND stories.audience = ` + audience + `::VARCHAR
	        AND stories.expires_at > ` + deadline + `::TIMESTAMP WITH TIME ZONE
	        AND (EXISTS (SELECT friend_requests.id FROM friend_requests
	                      WHERE friend_requests.state = ` + acceptedState + `
	                        AND (friend_requests.requester_id = ` + acceptedRequester + `::UUID AND friend_requests.addressee_id = stories.author_id
	                             OR friend_requests.addressee_id = ` + acceptedAddressee + `::UUID AND friend_requests.requester_id = stories.author_id)))`
}

package routes

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strconv"
	"time"

	"mobile/services/core/internal/domain/photoref"
	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/media/sanitize"
	"mobile/services/core/internal/media/storage"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/repo"
	guestweb "mobile/services/core/internal/web/guest"
)

// The six routes of app/api/routes/photos.py. Each handler is the Python
// route and its ApiService method step for step:
//
//   - an upload reads the `file` part first (_read_upload), then decides the
//     permission, then sanitizes, then writes the file, then inserts the row
//     (_store_uploaded_image). Every refusal comes before the write, so it
//     leaves neither a file nor a row; a failed INSERT leaves the file behind,
//     as Python does.
//   - a read decides the permission, then finds the row, then reads the file
//     (_stored_image_bytes), and answers the stored bytes as they are, with
//     exactly content-type, content-length and cache-control.
//
// The file store is call.Photos, the PhotoStorage get_photo_storage built for
// this request. The transaction begins at the first statement Python sends,
// so a request refused before one begins none.

// privateCacheControl is _PRIVATE_CACHE_HEADERS.
const privateCacheControl = "private, max-age=300"

// Refusal texts of the read routes.
const (
	photoNotFoundCode    = "photo_not_found"
	photoNotFoundDetail  = "Photo does not exist"
	avatarNotFoundCode   = "avatar_not_found"
	avatarNotFoundDetail = "Avatar does not exist"
)

// notAnImageDetail is the detail sanitize_image gives not_an_image.
const notAnImageDetail = "The uploaded bytes could not be decoded as a complete image."

// uploadContextPhoto is POST /contexts/{context_id}/photos
// (upload_context_photo): an active member of the group in the path, decided
// before the sanitizer sees a byte.
func uploadContextPhoto() Route {
	return Route{ID: "POST /contexts/{context_id}/photos", Status: 201, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		raw, err := readUpload(call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		contextID, err := pathUUID(call, "context_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := groupStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if err := requireGroupMember(ctx, call, store, "post_group_memory", contextID); err != nil {
			return endpoint.Reply{}, err
		}
		record, err := storeUploadedImage(ctx, call, raw, uploadOwner{ContextID: &contextID, Purpose: "group"})
		if err != nil {
			return endpoint.Reply{}, err
		}
		return endpoint.Reply{Body: wireUploadedImage(record, "/contexts/"+contextID+"/photos/"+record.ID)}, nil
	}}
}

// readContextPhoto is GET /contexts/{context_id}/photos/{photo_id}
// (read_context_photo): membership before the lookup, so an outsider learns
// nothing about which ids exist.
func readContextPhoto() Route {
	return Route{ID: "GET /contexts/{context_id}/photos/{photo_id}", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		contextID, err := pathUUID(call, "context_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		photoID, err := pathUUID(call, "photo_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := groupStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if err := requireGroupMember(ctx, call, store, "view_group_memories", contextID); err != nil {
			return endpoint.Reply{}, err
		}
		record, err := store.GetContextImage(ctx, contextID, photoID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if record == nil {
			return endpoint.Reply{}, endpoint.Refuse(404, photoNotFoundCode, photoNotFoundDetail)
		}
		return storedImageReply(call, *record, photoNotFoundCode, photoNotFoundDetail)
	}}
}

// setPersonAvatar is POST /people/{person_id}/avatar (set_person_avatar):
// roles, then is_self, both before the sanitizer. Nothing checks that the
// person has a row, so a person without one reaches the INSERT and its
// foreign key, after the file is written.
func setPersonAvatar() Route {
	return Route{ID: "POST /people/{person_id}/avatar", Status: 201, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		raw, err := readUpload(call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		personID, err := pathUUID(call, "person_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		if err := requireFacts(call, "set_own_avatar", map[string]bool{"is_self": call.Actor.ID == personID}); err != nil {
			return endpoint.Reply{}, err
		}
		record, err := storeUploadedImage(ctx, call, raw, uploadOwner{OwnerPersonID: &personID, Purpose: "avatar"})
		if err != nil {
			return endpoint.Reply{}, err
		}
		return endpoint.Reply{Body: wireUploadedImage(record, "/people/"+personID+"/avatar")}, nil
	}}
}

// readPersonAvatar is GET /people/{person_id}/avatar (read_person_avatar):
// sharing an active group before the newest avatar row is looked up.
// shares_active_context answers true for oneself without a statement, so
// reading one's own avatar begins no transaction before get_latest_avatar.
func readPersonAvatar() Route {
	return Route{ID: "GET /people/{person_id}/avatar", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		personID, err := pathUUID(call, "person_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		shared := call.Actor.ID == personID
		if !shared {
			store, err := groupStore(ctx, call)
			if err != nil {
				return endpoint.Reply{}, err
			}
			if shared, err = store.SharesActiveContext(ctx, call.Actor.ID, personID); err != nil {
				return endpoint.Reply{}, err
			}
		}
		if err := requireFacts(call, "view_person_avatar", map[string]bool{"shares_a_group_with_subject": shared}); err != nil {
			return endpoint.Reply{}, err
		}
		store, err := groupStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		record, err := store.GetLatestAvatar(ctx, personID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if record == nil {
			return endpoint.Reply{}, endpoint.Refuse(404, avatarNotFoundCode, avatarNotFoundDetail)
		}
		return storedImageReply(call, *record, avatarNotFoundCode, avatarNotFoundDetail)
	}}
}

// uploadPersonalPhoto is POST /people/me/photos (upload_personal_photo): the
// owner is the session, so only the roles are decided.
func uploadPersonalPhoto() Route {
	return Route{ID: "POST /people/me/photos", Status: 201, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		raw, err := readUpload(call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if err := requireFacts(call, "upload_personal_photo", map[string]bool{"is_self": true}); err != nil {
			return endpoint.Reply{}, err
		}
		actorID := call.Actor.ID
		record, err := storeUploadedImage(ctx, call, raw, uploadOwner{OwnerPersonID: &actorID, Purpose: "personal"})
		if err != nil {
			return endpoint.Reply{}, err
		}
		return endpoint.Reply{Body: wireUploadedImage(record, photoref.PersonPhotoURL(actorID, record.ID))}, nil
	}}
}

// readPersonPhoto is GET /people/{person_id}/photos/{photo_id}
// (read_person_photo), the one gate for personal photographs. Somebody other
// than the owner is an addressee exactly when a post or live story showing the
// photo is readable by them, asked before the roles and whether or not the
// photo exists. Every refusal of the permission, the roles included, is the
// same 404 as a missing photo, never a 403.
func readPersonPhoto() Route {
	return Route{ID: "GET /people/{person_id}/photos/{photo_id}", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		personID, err := pathUUID(call, "person_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		photoID, err := pathUUID(call, "photo_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		addressee := call.Actor.ID == personID
		if !addressee {
			store, err := groupStore(ctx, call)
			if err != nil {
				return endpoint.Reply{}, err
			}
			if addressee, err = store.PersonImageVisibleTo(ctx, personID, photoID, call.Actor.ID, time.Now().UTC()); err != nil {
				return endpoint.Reply{}, err
			}
		}
		err = requireFacts(call, "view_person_photo", map[string]bool{"is_photo_addressee": addressee})
		var denied *endpoint.Refusal
		if errors.As(err, &denied) {
			return endpoint.Reply{}, endpoint.Refuse(404, photoNotFoundCode, photoNotFoundDetail)
		}
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := groupStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		record, err := store.GetPersonImage(ctx, personID, photoID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if record == nil {
			return endpoint.Reply{}, endpoint.Refuse(404, photoNotFoundCode, photoNotFoundDetail)
		}
		return storedImageReply(call, *record, photoNotFoundCode, photoNotFoundDetail)
	}}
}

// readUpload is _read_upload: the first MAX_UPLOAD_BYTES + 1 bytes of the
// `file` part (the last one, as FastAPI picks it), enough for the sanitizer to
// prove a body oversize. The whole part has been received by then, as
// Starlette spools it all.
func readUpload(call *endpoint.Call) ([]byte, error) {
	file, err := bodyUpload(call, "file")
	if err != nil {
		return nil, err
	}
	return capUpload(file.Content), nil
}

// capUpload keeps at most sanitize.MaxUploadBytes + 1 bytes.
func capUpload(content []byte) []byte {
	if len(content) > sanitize.MaxUploadBytes+1 {
		return content[:sanitize.MaxUploadBytes+1]
	}
	return content
}

// uploadOwner is what _store_uploaded_image is told about the new row besides
// the bytes; uploaded_by_id is always the actor.
type uploadOwner struct {
	ContextID     *string
	OwnerPersonID *string
	Purpose       string
}

// storeUploadedImage is _store_uploaded_image: sanitize, a fresh storage key,
// the file, then the row at the service clock. A write failure leaves no row;
// an INSERT failure (a person without a row, in dev) leaves the file.
func storeUploadedImage(ctx context.Context, call *endpoint.Call, raw []byte, owner uploadOwner) (repo.UploadedImage, error) {
	sanitized, err := sanitizeUpload(raw)
	if err != nil {
		return repo.UploadedImage{}, err
	}
	key, err := storage.NewStorageKey()
	if err != nil {
		return repo.UploadedImage{}, err
	}
	if err := call.Photos.Write(key, sanitized.Data); err != nil {
		return repo.UploadedImage{}, err
	}
	store, err := groupStore(ctx, call)
	if err != nil {
		return repo.UploadedImage{}, err
	}
	return store.CreateUploadedImage(ctx, repo.UploadedImageInput{
		StorageKey:    key,
		ContextID:     owner.ContextID,
		OwnerPersonID: owner.OwnerPersonID,
		UploadedByID:  call.Actor.ID,
		ContentType:   sanitized.ContentType,
		ByteSize:      int64(len(sanitized.Data)),
		Width:         int64(sanitized.Width),
		Height:        int64(sanitized.Height),
		Now:           time.Now().UTC(),
		Purpose:       owner.Purpose,
	})
}

// sanitizeUpload is sanitize_image with ImageRejected turned into
// _image_rejection_problem. An *EncodeError or *InternalError is not caught,
// as Python does not catch what Pillow's save raises.
//
// An *UnsupportedError (a format Pillow opens that the Go port does not
// decode: AVIF, JPEG 2000, compressed TIFF, ICO/CUR/ICNS frames, …) is
// answered as not_an_image. That is a known divergence awaiting the Lead
// (ADR-0029 §8, open question 3); no parity scenario uploads such a format,
// as the harness generates JPEG, PNG, GIF, WebP, BMP and PNM only.
func sanitizeUpload(raw []byte) (sanitize.Sanitized, error) {
	return sanitizeOutcome(sanitize.Sanitize(raw))
}

// sanitizeOutcome answers what sanitize.Sanitize returned.
func sanitizeOutcome(sanitized sanitize.Sanitized, err error) (sanitize.Sanitized, error) {
	var rejected *sanitize.Rejected
	var unsupported *sanitize.UnsupportedError
	switch {
	case err == nil:
		return sanitized, nil
	case errors.As(err, &rejected):
		return sanitize.Sanitized{}, imageRejection(rejected.Code, rejected.Detail)
	case errors.As(err, &unsupported):
		return sanitize.Sanitized{}, imageRejection(sanitize.CodeNotAnImage, notAnImageDetail)
	}
	return sanitize.Sanitized{}, err
}

// imageRejection is _image_rejection_problem. A code outside its table is the
// KeyError Python would raise: an unhandled failure, not a refusal.
func imageRejection(code, detail string) error {
	status, ok := map[string]int{
		sanitize.CodeImageTooLarge:           413,
		sanitize.CodeImageDimensionsTooLarge: 413,
		sanitize.CodeNotAnImage:              415,
	}[code]
	if !ok {
		return fmt.Errorf("routes: no status for image rejection %q", code)
	}
	return endpoint.Refuse(status, code, detail)
}

// storedImageReply is _stored_image_bytes and the route's Response. A missing
// file (FileNotFoundError, so ENOENT only) and an empty one are the record's
// own 404; any other failure, a malformed storage key or a directory
// included, is unhandled.
func storedImageReply(call *endpoint.Call, record repo.UploadedImage, code, detail string) (endpoint.Reply, error) {
	content, err := call.Photos.Read(record.StorageKey)
	if errors.Is(err, fs.ErrNotExist) {
		return endpoint.Reply{}, endpoint.Refuse(404, code, detail)
	}
	if err != nil {
		return endpoint.Reply{}, err
	}
	if len(content) == 0 {
		return endpoint.Reply{}, endpoint.Refuse(404, code, detail)
	}
	return imageReply(content, record.ContentType), nil
}

// imageReply is Response(content, media_type=content_type,
// headers=_PRIVATE_CACHE_HEADERS): the given header, then content-length, then
// content-type with no charset (the column allows only JPEG and PNG). No etag,
// last-modified or accept-ranges, and no Range or conditional handling.
func imageReply(content []byte, contentType string) endpoint.Reply {
	return endpoint.Reply{Raw: &guestweb.Response{
		Status: 200,
		Headers: [][2]string{
			{"cache-control", privateCacheControl},
			{"content-length", strconv.Itoa(len(content))},
			{"content-type", contentType},
		},
		Body: content,
	}}
}

// wireUploadedImage is _uploaded_image_response: UploadedImageResponse's
// fields in declaration order. The caller builds url from canonical ids.
func wireUploadedImage(record repo.UploadedImage, url string) *pyjson.OrderedMap {
	out := pyjson.NewOrderedMap()
	out.Set("id", pyjson.String(record.ID))
	out.Set("context_id", textOrNull(record.ContextID))
	out.Set("url", pyjson.String(url))
	out.Set("content_type", pyjson.String(record.ContentType))
	out.Set("byte_size", pyjson.NewInt(record.ByteSize))
	out.Set("width", pyjson.NewInt(record.Width))
	out.Set("height", pyjson.NewInt(record.Height))
	out.Set("created_at", pyjson.String(pyjson.DateTime(record.CreatedAt.UTC())))
	return out
}

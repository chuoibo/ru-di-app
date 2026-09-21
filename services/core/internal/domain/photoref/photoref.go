// Package photoref is app.domain.photo_ref: which owner and which photograph
// an image_url names (ADR-0022 §2.1). It only parses; every refusal is one
// PhotoURLError with code MALFORMED.
package photoref

import (
	"strings"

	"mobile/services/core/internal/domain/pyuuid"
)

// Owner kinds (OWNER_CONTEXT, OWNER_PERSON).
const (
	OwnerContext = "context"
	OwnerPerson  = "person"
)

// CodeMalformed is PhotoUrlError's only code.
const CodeMalformed = "MALFORMED"

var (
	segmentToOwner = map[string]string{"contexts": OwnerContext, "people": OwnerPerson}
	ownerToSegment = map[string]string{OwnerContext: "contexts", OwnerPerson: "people"}
)

// PhotoURLError is PhotoUrlError: `str(exc)` and `.code` are both the code.
type PhotoURLError struct {
	Code string
}

func (e *PhotoURLError) Error() string { return e.Code }

func malformed() error { return &PhotoURLError{Code: CodeMalformed} }

// Ref is PhotoRef.
type Ref struct {
	OwnerKind string
	OwnerID   string
	PhotoID   string
}

// URL is PhotoRef.url: lower-case hyphenated ids, the spelling stored on a
// post or story.
func (r Ref) URL() string {
	return "/" + ownerToSegment[r.OwnerKind] + "/" + r.OwnerID + "/photos/" + r.PhotoID
}

// Parse is parse_photo_url: exactly five "/"-separated segments, an empty
// first one, a known owner word, "photos", and two ids uuid.UUID accepts.
func Parse(imageURL string) (Ref, error) {
	parts := strings.Split(imageURL, "/")
	if len(parts) != 5 || parts[0] != "" || parts[3] != "photos" {
		return Ref{}, malformed()
	}
	owner, ok := segmentToOwner[parts[1]]
	if !ok {
		return Ref{}, malformed()
	}
	ownerID, ok := pyuuid.Parse(parts[2])
	if !ok {
		return Ref{}, malformed()
	}
	photoID, ok := pyuuid.Parse(parts[4])
	if !ok {
		return Ref{}, malformed()
	}
	return Ref{OwnerKind: owner, OwnerID: ownerID, PhotoID: photoID}, nil
}

// PersonPhotoURL is person_photo_url.
func PersonPhotoURL(personID, photoID string) string {
	return Ref{OwnerKind: OwnerPerson, OwnerID: personID, PhotoID: photoID}.URL()
}

// ContextPhotoURL is context_photo_url.
func ContextPhotoURL(contextID, photoID string) string {
	return Ref{OwnerKind: OwnerContext, OwnerID: contextID, PhotoID: photoID}.URL()
}

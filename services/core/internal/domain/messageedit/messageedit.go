// Package messageedit is app.domain.message_edit: reply and delete rules.
package messageedit

import "time"

// DeletableKinds is DELETABLE_KINDS.
var DeletableKinds = map[string]bool{"text": true, "image": true, "sticker": true}

// ReplyableKinds is REPLYABLE_KINDS.
var ReplyableKinds = map[string]bool{"text": true, "image": true, "sticker": true}

// Error is MessageEditError: a closed code the service maps to HTTP.
type Error struct{ Code string }

func (e *Error) Error() string { return e.Code }

func refuse(code string) error { return &Error{Code: code} }

// Facts is the stored message as the domain reads it.
type Facts struct {
	ID        string
	ContextID string
	AuthorID  *string
	Kind      string
}

// CheckDeletable is check_deletable.
func CheckDeletable(message Facts, actorID string) error {
	if message.Kind == "deleted" {
		return refuse("ALREADY_DELETED")
	}
	if message.AuthorID == nil || *message.AuthorID != actorID {
		return refuse("NOT_AUTHOR")
	}
	if !DeletableKinds[message.Kind] {
		return refuse("KIND_NOT_DELETABLE")
	}
	return nil
}

// CheckReplyTarget is check_reply_target.
func CheckReplyTarget(target *Facts, contextID string) error {
	if target == nil || target.ContextID != contextID {
		return refuse("REPLY_NOT_FOUND")
	}
	if target.Kind == "deleted" {
		return refuse("REPLY_TO_DELETED")
	}
	if !ReplyableKinds[target.Kind] {
		return refuse("REPLY_TO_CARD")
	}
	return nil
}

// DeletedShape is deleted_shape: kind flipped, payload gone, author kept.
// Callers pass an aware instant (UTC). A zero Time stands in for naive.
func DeletedShape(now time.Time) (kind string, deletedAt time.Time, err error) {
	if now.IsZero() {
		return "", time.Time{}, refuse("NAIVE_DATETIME")
	}
	return "deleted", now, nil
}

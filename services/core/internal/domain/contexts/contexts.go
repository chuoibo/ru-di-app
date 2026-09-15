// Package contexts holds the pure steps app/api/service.py runs inline for the
// W3 context and memory routes, which are not functions of app/domain in
// Python:
//
//	UpdateChanges           update_context, after its permission check
//	RequirePhotoURLContext  _require_photo_url_context (POST .../memories)
//	View                    _context_response (POST, PATCH, GET /contexts...)
//	AcceptPermission        accept_context_membership, before its permission check
//	WidgetAuthorFallback    _SOMEBODY (GET .../widget)
//
// A repository read the service makes only on some branches is a callback
// here, so a route calling these functions issues the same statements, in the
// same order, as the Python method. A refusal is the ApiProblem the method
// raises, as *Refusal with its status, code and detail.
//
// testdata/python_*.json is rendered by scripts/render_domain_w3_goldens.py
// by running the real service methods over a recording stub repository;
// oracle_test.go replays every case, including which reads were made.
package contexts

import (
	"strings"

	"mobile/services/core/internal/domain/chattheme"
	"mobile/services/core/internal/domain/direct"
	"mobile/services/core/internal/domain/photoref"
	"mobile/services/core/internal/domain/pyuuid"
)

// WidgetAuthorFallback is _SOMEBODY: the widget's author name when the author
// of the newest photograph has no row in people.
const WidgetAuthorFallback = "Thành viên nhóm"

// Refusal is an ApiProblem the service raises.
type Refusal struct {
	Status int
	Code   string
	Detail string
}

func (r *Refusal) Error() string { return r.Code }

var (
	notAGroup            = Refusal{409, "not_a_group", "Đây là cuộc trò chuyện riêng, không có danh sách thành viên để đổi."}
	themeUnknown         = Refusal{422, "theme_unknown", "Bộ màu này không có trong bộ của Rủ Đi."}
	photoURLInvalid      = Refusal{422, "photo_url_invalid", "Photo URL is not a path into this product's photo storage"}
	photoContextMismatch = Refusal{422, "photo_context_mismatch", "Photo URL context does not match the requested context"}
)

func refusal(r Refusal) *Refusal { return &r }

// isPySpace is str.isspace() for one code point; oracle_test.go checks it
// against CPython over every code point.
func isPySpace(r rune) bool {
	switch {
	case r >= 0x09 && r <= 0x0D, r >= 0x1C && r <= 0x20:
		return true
	case r == 0x85, r == 0xA0, r == 0x1680, r >= 0x2000 && r <= 0x200A,
		r == 0x2028, r == 0x2029, r == 0x202F, r == 0x205F, r == 0x3000:
		return true
	}
	return false
}

// Strip is str.strip() with no argument: Python whitespace removed from both
// ends. It is not strings.TrimSpace, which keeps U+001C..U+001F.
func Strip(s string) string { return strings.TrimFunc(s, isPySpace) }

// Changes is the dict update_context hands the repository, in its order:
// display_name first, then theme, each only when the request named it.
type Changes struct {
	DisplayName *string
	Theme       *string
}

// UpdateChanges is update_context after `edit_context` is granted. A
// display_name reads the context's kind first (_require_group_kind) and is
// refused with 409 not_a_group on a pair, else stored stripped; a theme outside
// chattheme is refused with 422 theme_unknown. isPair is that read: true only
// when the context exists and is a pair. It is called only when displayName is
// not nil, as the service reads the row only then.
func UpdateChanges(displayName, theme *string, isPair func() (bool, error)) (Changes, *Refusal, error) {
	var changes Changes
	if displayName != nil {
		pair, err := isPair()
		if err != nil {
			return Changes{}, nil, err
		}
		if pair {
			return Changes{}, refusal(notAGroup), nil
		}
		stripped := Strip(*displayName)
		changes.DisplayName = &stripped
	}
	if theme != nil {
		if !chattheme.IsTheme(*theme) {
			return Changes{}, refusal(themeUnknown), nil
		}
		chosen := *theme
		changes.Theme = &chosen
	}
	return changes, nil, nil
}

// RequirePhotoURLContext is _require_photo_url_context: nil for no image;
// 422 photo_url_invalid unless the url is exactly
// "/contexts/<uuid>/photos/<uuid>" with ids uuid.UUID accepts; 422
// photo_context_mismatch when it names a group other than contextID, the
// canonical id from the path.
func RequirePhotoURLContext(contextID string, imageURL *string) *Refusal {
	if imageURL == nil {
		return nil
	}
	ref, err := photoref.Parse(*imageURL)
	if err != nil || ref.OwnerKind != photoref.OwnerContext {
		return refusal(photoURLInvalid)
	}
	if canonical, ok := pyuuid.Parse(contextID); !ok || ref.OwnerID != canonical {
		return refusal(photoContextMismatch)
	}
	return nil
}

// Member is one roster row as _context_response reads it.
type Member struct {
	PersonID    string
	DisplayName string
}

// Counterpart is ContextCounterpart: the other person of a pair.
type Counterpart struct {
	ID          string
	DisplayName string
}

// ContextView is the name part of ContextResponse.
type ContextView struct {
	DisplayName string
	Counterpart *Counterpart
}

// View is _context_response's naming: a group is called what it is stored as;
// a pair reads its roster (members, called only for a pair) and is called
// after the one other member, or AnonymousCounterpart when there is not
// exactly one or the name is empty. Ids are canonical uuid strings.
func View(kind, storedName, actorID string, members func() ([]Member, error)) (ContextView, error) {
	var counterpart *Counterpart
	if direct.IsPair(kind) {
		roster, err := members()
		if err != nil {
			return ContextView{}, err
		}
		ids := make([]string, len(roster))
		for i, member := range roster {
			ids[i] = member.PersonID
		}
		if otherID, found := direct.CounterpartOf(ids, actorID); found {
			for _, member := range roster {
				if member.PersonID == otherID {
					name := member.DisplayName
					counterpart = &Counterpart{
						ID:          member.PersonID,
						DisplayName: direct.DisplayNameFor(direct.KindPair, "", &name),
					}
					break
				}
			}
		}
	}
	var counterpartName *string
	if counterpart != nil {
		counterpartName = &counterpart.DisplayName
	}
	return ContextView{
		DisplayName: direct.DisplayNameFor(kind, storedName, counterpartName),
		Counterpart: counterpart,
	}, nil
}

// Actions accept_context_membership asks the permission table for.
const (
	ActionApproveLinkJoinRequest  = "approve_link_join_request"
	ActionAcceptContextMembership = "accept_context_membership"
)

// OriginLink is MembershipOrigin.LINK.
const OriginLink = "link"

// AcceptPermission is the branch accept_context_membership takes on the
// invitation's provenance: a link request must be approved by an active
// member who is not the requester (isMember is the roster read, made only for
// a link); any other origin is accepted by the invitee. It returns the action
// and the predicates exactly as the service's context dict holds them, false
// values included, for service.RequirePermission.
func AcceptPermission(origin, actorID, inviteeID string, isMember func() (bool, error)) (string, map[string]bool, error) {
	if origin == OriginLink {
		member, err := isMember()
		if err != nil {
			return "", nil, err
		}
		return ActionApproveLinkJoinRequest, map[string]bool{
			"is_group_member": member,
			"is_not_self":     actorID != inviteeID,
		}, nil
	}
	return ActionAcceptContextMembership, map[string]bool{"is_invitee": inviteeID == actorID}, nil
}

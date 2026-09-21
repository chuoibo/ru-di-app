// Package postaudience ports app.domain.post_audience (F42, ADR-0022 §2.2,
// ADR-0023 §2.3.2): the four audiences a post can address, who they reach, who
// may comment, and what a block hides.
//
// Parity, not correctness, is the contract (ADR-0029 §2.4): oracle_test.go
// replays testdata/python_*.json, rendered by
// scripts/render_domain_w2_goldens.py from the real module in the parity API
// image.
//
// # Values
//
// The post dict is what the service builds (`_post_dict`): the author id, the
// audience word and the context id or None. The facts are booleans somebody
// else proved. is_comment_policy takes any object in Python; the port takes
// the str the service passes.
package postaudience

import "slices"

// The four audiences (AUDIENCES), narrowest first.
const (
	OnlyMe  = "only_me"
	Friends = "friends"
	Group   = "group"
	Public  = "public"
)

// DefaultAudience is DEFAULT_AUDIENCE.
const DefaultAudience = OnlyMe

// The comment policies (COMMENT_POLICIES).
const (
	PolicyReaders = "readers"
	PolicyFriends = "friends"
	PolicyNobody  = "nobody"
)

// DefaultCommentPolicy is DEFAULT_COMMENT_POLICY.
const DefaultCommentPolicy = PolicyReaders

// Codes AudienceError carries.
const (
	CodeUnknownAudience           = "UNKNOWN_AUDIENCE"
	CodeGroupAudienceNeedsContext = "GROUP_AUDIENCE_NEEDS_CONTEXT"
	CodeContextNotAddressable     = "CONTEXT_NOT_ADDRESSABLE"
)

var (
	audiences       = [...]string{OnlyMe, Friends, Group, Public}
	commentPolicies = [...]string{PolicyReaders, PolicyFriends, PolicyNobody}
)

// Audiences returns AUDIENCES in declaration order.
func Audiences() []string { return slices.Clone(audiences[:]) }

// CommentPolicies returns COMMENT_POLICIES in declaration order.
func CommentPolicies() []string { return slices.Clone(commentPolicies[:]) }

// AudienceError is Python's AudienceError: `str(exc)` is the code.
type AudienceError struct {
	Code string
}

func (e *AudienceError) Error() string { return e.Code }

// Post is the post dict. ContextID nil is None.
type Post struct {
	AuthorID  string
	Audience  string
	ContextID *string
}

// Comment is the comment dict can_delete_comment reads.
type Comment struct {
	AuthorID string
}

// NeedsContext is needs_context.
func NeedsContext(audience string) bool {
	return audience == Group
}

// CheckWritable is check_writable. Any non-nil context, the empty string
// included, names a group.
func CheckWritable(audience string, contextID *string) error {
	if !slices.Contains(audiences[:], audience) {
		return &AudienceError{Code: CodeUnknownAudience}
	}
	if NeedsContext(audience) && contextID == nil {
		return &AudienceError{Code: CodeGroupAudienceNeedsContext}
	}
	if !NeedsContext(audience) && contextID != nil {
		return &AudienceError{Code: CodeContextNotAddressable}
	}
	return nil
}

// IsCommentPolicy is is_comment_policy for a str.
func IsCommentPolicy(value string) bool {
	return slices.Contains(commentPolicies[:], value)
}

// CanComment is can_comment. An unknown policy closes the composer for
// everybody but the author.
func CanComment(post Post, policy, readerID string, isFriend, isGroupMember bool) bool {
	if !CanRead(post, readerID, isFriend, isGroupMember) {
		return false
	}
	if readerID == post.AuthorID {
		return true
	}
	switch policy {
	case PolicyReaders:
		return true
	case PolicyFriends:
		return isFriend
	}
	return false
}

// VisibleTo is visible_to: CanRead, then the block rule. A block hides
// everything but a group post, both ways; the author always reads their own.
func VisibleTo(post Post, readerID string, isFriend, isGroupMember, isBlocked bool) bool {
	if !CanRead(post, readerID, isFriend, isGroupMember) {
		return false
	}
	if !isBlocked || readerID == post.AuthorID {
		return true
	}
	return post.Audience == Group
}

// CanDeleteComment is can_delete_comment.
func CanDeleteComment(comment Comment, post Post, actorID string) bool {
	return actorID == comment.AuthorID || actorID == post.AuthorID
}

// CanRead is can_read: an unknown audience fails closed, even for the author.
func CanRead(post Post, readerID string, isFriend, isGroupMember bool) bool {
	if !slices.Contains(audiences[:], post.Audience) {
		return false
	}
	if readerID == post.AuthorID {
		return true
	}
	switch post.Audience {
	case Public:
		return true
	case Friends:
		return isFriend
	case Group:
		return post.ContextID != nil && isGroupMember
	}
	return false
}

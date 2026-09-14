// Package permissions is the Go port of app/domain/permissions.py: the one
// permission table every API and every ActionItem asks (spec section 9).
//
// ADR-0029 §2.4 makes Python the reference, byte for byte. That includes the
// refusal strings: the API turns a denial reason into the detail of a 403
// `permission_denied` problem, so a reason that differs by one character is a
// wire difference. The rationale for each entry lives beside it in the Python
// module; this file carries the data and the decision procedure only.
//
// testdata/python_permissions.json is rendered from the real module by
// scripts/render_permissions_goldens.py and replayed by the tests here.
//
// # Errors instead of exceptions
//
// Python raises PermissionError_ in two places, and Go returns *Error with the
// same Code instead:
//
//   - Building AuthorizationFacts raises ANONYMOUS_ACTOR, then
//     FACTS_WITHOUT_PROVENANCE, then UNKNOWN_ROLE (__post_init__ order).
//     NewAuthorizationFacts returns those. A Go struct literal skips the
//     constructor, so DenialReason and Can validate the facts again FIRST:
//     in Python the facts argument is built before denial_reason runs, so a
//     facts error always wins over UNKNOWN_ACTION.
//   - denial_reason raises UNKNOWN_ACTION for a name outside the table. A typo
//     must fail loudly, never look like an ordinary deny, so DenialReason
//     returns ErrUnknownAction and allowed=false rather than a reason.
//
// Python's UNTYPED_FACTS (a plain dict passed as facts) cannot happen in Go:
// the parameter is statically typed.
//
// Nothing in the Python API catches PermissionError_, so it reaches the
// catch-all exception handler and the client gets that handler's 500. A Go
// caller must send a non-nil error down the same 500 path, never to 403.
package permissions

import "sort"

// Codes carried by *Error, identical to PermissionError_.code.
const (
	CodeAnonymousActor         = "ANONYMOUS_ACTOR"
	CodeFactsWithoutProvenance = "FACTS_WITHOUT_PROVENANCE"
	CodeUnknownRole            = "UNKNOWN_ROLE"
	CodeUnknownAction          = "UNKNOWN_ACTION"
)

// The two denial reasons that are not predicate names.
const (
	ReasonPermittedToNobody = "action_permitted_to_nobody"
	ReasonRoleNotPermitted  = "role_not_permitted"
)

// Error mirrors PermissionError_. Error returns Code unchanged, as str(exc)
// does in Python.
type Error struct {
	Code string
}

func (e *Error) Error() string { return e.Code }

// Sentinels for errors.Is. Every error this package returns is one of these.
var (
	ErrAnonymousActor         = &Error{Code: CodeAnonymousActor}
	ErrFactsWithoutProvenance = &Error{Code: CodeFactsWithoutProvenance}
	ErrUnknownRole            = &Error{Code: CodeUnknownRole}
	ErrUnknownAction          = &Error{Code: CodeUnknownAction}
)

// roles is ROLES, in the Python order.
var roles = []string{
	"group_admin",
	"batch_owner",
	"advancer",
	"recipient",
	"sender",
	"creditor",
	"member",
	"former_member",
	"guest",
	"platform_moderator",
}

var knownRole = func() map[string]bool {
	set := make(map[string]bool, len(roles))
	for _, name := range roles {
		set[name] = true
	}
	return set
}()

// Roles returns ROLES in the Python order. The slice is a copy.
func Roles() []string { return append([]string(nil), roles...) }

// rule is one _TABLE entry.
type rule struct {
	// roles is a set in Python: order and duplicates carry no meaning. An
	// empty set means the action is permitted to nobody.
	roles []string
	// requires is ordered. When several predicates are missing, the first
	// one in this order is the reason reported.
	requires []string
}

// table is _TABLE, entry for entry, in the Python source order.
var table = map[string]rule{
	// --- invocation ---
	"create_private_invocation": {roles: []string{"member"}},
	"create_shared_invocation":  {roles: []string{"member"}},
	"view_invocation_input":     {roles: []string{"member"}, requires: []string{"is_invoker"}},
	"view_invocation_proposal":  {roles: []string{"member"}, requires: []string{"is_invoker"}},
	// --- expense ---
	"confirm_expense_proposal":  {roles: []string{"member"}, requires: []string{"is_group_member"}},
	"acknowledge_advancer_role": {roles: []string{"advancer"}, requires: []string{"is_named_advancer"}},
	// --- collection board ---
	"view_collection_board": {roles: []string{"member"}, requires: []string{"is_group_member"}},
	// --- batch ---
	"create_batch":                   {roles: []string{"member"}, requires: []string{"is_group_member"}},
	"freeze_batch":                   {roles: []string{"batch_owner"}, requires: []string{"owns_batch"}},
	"publish_batch":                  {roles: []string{"batch_owner"}, requires: []string{"owns_batch"}},
	"revoke_capability_whole_batch":  {roles: []string{"batch_owner"}, requires: []string{"owns_batch"}},
	"revoke_capability_own_envelope": {roles: []string{"sender"}, requires: []string{"is_own_capability"}},
	// --- guest settlement ---
	"view_guest_envelope": {roles: []string{"guest"}, requires: []string{"is_own_capability"}},
	"report_payment":      {roles: []string{"guest"}, requires: []string{"is_own_capability", "active_capability", "report_budget_available"}},
	"confirm_receipt":     {roles: []string{"recipient"}, requires: []string{"is_recipient_of_this_obligation"}},
	// --- things the batch owner may NOT do alone ---
	"cancel_obligation":              {roles: []string{"batch_owner"}, requires: []string{"all_affected_parties_consented"}},
	"amend_obligation_after_publish": {roles: []string{"batch_owner"}, requires: []string{"all_affected_parties_consented"}},
	"delete_payment_report":          {roles: nil},
	"delete_receipt_confirmation":    {roles: nil},
	"delete_audit_history":           {roles: nil},
	"close_dispute":                  {roles: []string{"platform_moderator"}},
	// --- debt forgiveness ---
	"waive_obligation": {roles: []string{"creditor"}, requires: []string{"is_creditor_of_this_obligation"}},
	// --- evidence ---
	"request_redacted_evidence": {roles: []string{"guest", "member"}, requires: []string{"is_charged_party"}},
	"share_evidence":            {roles: []string{"member"}, requires: []string{"is_uploader"}},
	// --- identity ---
	"register_person_identity":     {roles: []string{"group_admin", "member"}},
	"rename_person_identity":       {roles: []string{"group_admin", "member"}, requires: []string{"is_self"}},
	"set_own_avatar":               {roles: []string{"group_admin", "member"}, requires: []string{"is_self"}},
	"view_person_avatar":           {roles: []string{"group_admin", "member"}, requires: []string{"shares_a_group_with_subject"}},
	"invite_person_stub_claim":     {roles: []string{"member"}},
	"challenge_person_stub_claim":  {roles: []string{"member"}},
	"adjudicate_person_stub_claim": {roles: []string{"platform_moderator"}},
	// --- friend graph (F03, F04) ---
	"send_friend_request":       {roles: []string{"member"}, requires: []string{"is_not_self"}},
	"respond_to_friend_request": {roles: []string{"member"}, requires: []string{"is_invitee"}},
	"view_own_friends":          {roles: []string{"member"}, requires: []string{"is_self"}},
	"view_own_contexts":         {roles: []string{"member"}, requires: []string{"is_self"}},
	// --- profile and bookmarks (M2) ---
	"view_own_profile":     {roles: []string{"member"}, requires: []string{"is_self"}},
	"edit_own_profile":     {roles: []string{"member"}, requires: []string{"is_self"}},
	"view_person_profile":  {roles: []string{"member"}, requires: []string{"is_visible_person"}},
	"manage_saved_places":  {roles: []string{"member"}, requires: []string{"is_self"}},
	"manage_own_interests": {roles: []string{"member"}, requires: []string{"is_self"}},
	"find_person_by_phone": {roles: []string{"member"}},
	"manage_own_sessions":  {roles: []string{"member"}, requires: []string{"is_self"}},
	"delete_own_account":   {roles: []string{"member"}, requires: []string{"is_self"}},
	"block_person":         {roles: []string{"member"}, requires: []string{"is_not_self"}},
	"unblock_person":       {roles: []string{"member"}, requires: []string{"is_blocker"}},
	"view_own_blocks":      {roles: []string{"member"}, requires: []string{"is_self"}},
	"file_report":          {roles: []string{"member"}},
	// --- group logistics ---
	"create_outing":                 {roles: []string{"group_admin", "member"}, requires: []string{"is_group_member"}},
	"view_outings":                  {roles: []string{"group_admin", "member"}, requires: []string{"is_group_member"}},
	"edit_outing_timeline":          {roles: []string{"group_admin", "member"}, requires: []string{"is_group_member"}},
	"invite_to_outing":              {roles: []string{"group_admin", "member"}, requires: []string{"is_group_member"}},
	"check_in_to_stop":              {roles: []string{"group_admin", "member"}, requires: []string{"is_group_member"}},
	"view_stop_checkins":            {roles: []string{"group_admin", "member"}, requires: []string{"is_group_member"}},
	"revoke_outing_invite":          {roles: []string{"group_admin", "member"}, requires: []string{"is_group_member"}},
	"create_vote":                   {roles: []string{"group_admin", "member"}, requires: []string{"is_group_member"}},
	"view_votes":                    {roles: []string{"group_admin", "member"}, requires: []string{"is_group_member"}},
	"cast_vote_ballot":              {roles: []string{"group_admin", "member"}, requires: []string{"is_group_member"}},
	"close_vote":                    {roles: []string{"group_admin", "member"}, requires: []string{"is_group_member", "is_vote_creator"}},
	"create_context":                {roles: []string{"group_admin", "member"}},
	"invite_context_member":         {roles: []string{"group_admin"}, requires: []string{"is_group_member"}},
	"accept_context_membership":     {roles: []string{"group_admin", "member"}, requires: []string{"is_invitee"}},
	"approve_link_join_request":     {roles: []string{"group_admin", "member"}, requires: []string{"is_group_member", "is_not_self"}},
	"leave_context":                 {roles: []string{"group_admin", "member"}, requires: []string{"is_group_member", "is_self"}},
	"view_context_members":          {roles: []string{"group_admin", "member"}, requires: []string{"is_group_member"}},
	"post_group_message":            {roles: []string{"group_admin", "member"}, requires: []string{"is_group_member"}},
	"view_group_messages":           {roles: []string{"group_admin", "member"}, requires: []string{"is_group_member"}},
	"invoke_group_companion":        {roles: []string{"group_admin", "member"}, requires: []string{"is_group_member"}},
	"delete_own_message":            {roles: []string{"group_admin", "member"}, requires: []string{"is_group_member", "is_author"}},
	"edit_context":                  {roles: []string{"group_admin", "member"}, requires: []string{"is_group_member"}},
	"open_direct_message":           {roles: []string{"member"}, requires: []string{"is_friend"}},
	"view_group_suggestion":         {roles: []string{"group_admin", "member"}, requires: []string{"is_group_member"}},
	"view_group_preference_profile": {roles: []string{"group_admin", "member"}, requires: []string{"is_group_member"}},
	"view_contextual_suggestion":    {roles: []string{"group_admin", "member"}, requires: []string{"is_group_member"}},
	"view_trip_album":               {roles: []string{"group_admin", "member"}, requires: []string{"is_group_member"}},
	"view_social_map":               {roles: []string{"group_admin", "member"}, requires: []string{"is_group_member"}},
	"view_group_heatmap":            {roles: []string{"group_admin", "member"}, requires: []string{"is_group_member"}},
	"view_meeting_point":            {roles: []string{"group_admin", "member"}, requires: []string{"is_group_member"}},
	"view_group_budget":             {roles: []string{"group_admin", "member"}, requires: []string{"is_group_member"}},
	"react_to_message":              {roles: []string{"group_admin", "member"}, requires: []string{"is_group_member"}},
	"post_group_memory":             {roles: []string{"group_admin", "member"}, requires: []string{"is_group_member"}},
	"view_group_memories":           {roles: []string{"group_admin", "member"}, requires: []string{"is_group_member"}},
	"react_to_post":                 {roles: []string{"group_admin", "member"}, requires: []string{"may_read_post"}},
	"comment_on_post":               {roles: []string{"group_admin", "member"}, requires: []string{"may_comment"}},
	"delete_post_comment":           {roles: []string{"group_admin", "member"}, requires: []string{"may_delete_comment"}},
	"upload_personal_photo":         {roles: []string{"group_admin", "member"}, requires: []string{"is_self"}},
	"view_person_photo":             {roles: []string{"group_admin", "member"}, requires: []string{"is_photo_addressee"}},
	"create_story":                  {roles: []string{"group_admin", "member"}, requires: []string{"is_self"}},
	"view_story":                    {roles: []string{"group_admin", "member"}, requires: []string{"may_view_story"}},
	"delete_own_story":              {roles: []string{"group_admin", "member"}, requires: []string{"is_author"}},
	"create_post":                   {roles: []string{"group_admin", "member"}},
	"address_post_to_group":         {roles: []string{"group_admin", "member"}, requires: []string{"is_group_member"}},
	"set_member_role":               {roles: []string{"group_admin"}, requires: []string{"is_group_admin"}},
	"manage_members_and_invites":    {roles: []string{"group_admin"}},
	"remove_member_from_group":      {roles: []string{"group_admin"}},
	"transfer_group_admin":          {roles: []string{"group_admin"}},
	"remove_own_uploaded_content":   {roles: []string{"group_admin", "member"}, requires: []string{"is_uploader"}},
	"remove_others_content":         {roles: []string{"platform_moderator"}},
	"attach_workspace_to_group":     {roles: []string{"member"}, requires: []string{"is_workspace_owner"}},
	// --- sổ hai người và tờ giấy (ADR-0027) ---
	"view_pair_notebook":           {roles: []string{"member"}, requires: []string{"is_group_member"}},
	"propose_pair_consent":         {roles: []string{"member"}, requires: []string{"is_group_member"}},
	"grant_pair_consent":           {roles: []string{"member"}, requires: []string{"is_invitee", "proposal_in_force"}},
	"revoke_pair_consent":          {roles: []string{"member"}, requires: []string{"is_self"}},
	"draft_pair_paper":             {roles: []string{"member"}, requires: []string{"is_group_member", "cycle_active_or_temporary"}},
	"view_pair_paper":              {roles: []string{"member"}, requires: []string{"may_view_paper"}},
	"edit_pair_draft":              {roles: []string{"member"}, requires: []string{"is_draft_owner"}},
	"send_pair_paper":              {roles: []string{"member"}, requires: []string{"is_draft_owner", "version_current"}},
	"view_pair_paper_as_recipient": {roles: []string{"member"}, requires: []string{"is_not_version_sender"}},
	"respond_pair_paper":           {roles: []string{"member"}, requires: []string{"version_current", "is_not_version_sender"}},
	"withdraw_pair_paper":          {roles: []string{"member"}, requires: []string{"is_group_member", "paper_unseen_unanswered"}},
	"skip_pair_week":               {roles: []string{"member"}, requires: []string{"is_group_member"}},
	"record_pair_outing_done":      {roles: []string{"member"}, requires: []string{"is_group_member"}},
	"keep_pair_paper_line":         {roles: []string{"member"}, requires: []string{"is_group_member"}},
	"view_pair_constraints":        {roles: []string{"member"}, requires: []string{"is_group_member"}},
	"edit_pair_constraint":         {roles: []string{"member"}, requires: []string{"is_self"}},
	"preview_close_pair_notebook":  {roles: []string{"member"}, requires: []string{"is_group_member"}},
	"close_pair_notebook":          {roles: []string{"member"}, requires: []string{"is_group_member"}},
}

// actions is ACTIONS = tuple(sorted(_TABLE)). The names are ASCII, so Go's
// byte order and Python's code point order agree.
var actions = func() []string {
	names := make([]string, 0, len(table))
	for name := range table {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}()

// Actions returns ACTIONS: every action name, sorted. The slice is a copy.
func Actions() []string { return append([]string(nil), actions...) }

// AuthorizationFacts mirrors the frozen dataclass: what an authoritative
// source proved, not what a request claimed.
//
// Roles and Proven are frozensets in Python; here order and duplicates are
// ignored. The zero values match the Python defaults where there are any
// (Proven empty, Provenance ""). ResourceID nil is None; the table never
// consults it.
type AuthorizationFacts struct {
	ActorID    string
	Roles      []string
	ResourceID *string
	Proven     []string
	Provenance string
}

// NewAuthorizationFacts is the AuthorizationFacts(...) constructor: it
// returns the error __post_init__ would raise, and the zero value with it.
func NewAuthorizationFacts(
	actorID string,
	roles []string,
	resourceID *string,
	proven []string,
	provenance string,
) (AuthorizationFacts, error) {
	facts := AuthorizationFacts{
		ActorID:    actorID,
		Roles:      roles,
		ResourceID: resourceID,
		Proven:     proven,
		Provenance: provenance,
	}
	if err := facts.Validate(); err != nil {
		return AuthorizationFacts{}, err
	}
	return facts, nil
}

// Validate performs the __post_init__ checks in their Python order.
func (f AuthorizationFacts) Validate() error {
	if f.ActorID == "" {
		return ErrAnonymousActor
	}
	if f.Provenance == "" {
		return ErrFactsWithoutProvenance
	}
	for _, name := range f.Roles {
		if !knownRole[name] {
			return ErrUnknownRole
		}
	}
	return nil
}

// DenialReason mirrors denial_reason. allowed=true is Python's None, and then
// reason is "". Otherwise reason is "action_permitted_to_nobody",
// "role_not_permitted" or the first missing predicate, in that precedence.
// On error allowed is false and reason is "" (see the package comment for
// what Python raises and in which order).
func DenialReason(action string, facts AuthorizationFacts) (reason string, allowed bool, err error) {
	if err := facts.Validate(); err != nil {
		return "", false, err
	}
	entry, ok := table[action]
	if !ok {
		return "", false, ErrUnknownAction
	}
	if len(entry.roles) == 0 {
		return ReasonPermittedToNobody, false, nil
	}
	if !sharesAny(facts.Roles, entry.roles) {
		return ReasonRoleNotPermitted, false, nil
	}
	for _, predicate := range entry.requires {
		if !contains(facts.Proven, predicate) {
			return predicate, false, nil
		}
	}
	return "", true, nil
}

// Can mirrors can: true when the facts permit the action. On error it
// returns false with the same error DenialReason returns.
func Can(action string, facts AuthorizationFacts) (bool, error) {
	_, allowed, err := DenialReason(action, facts)
	return allowed, err
}

func contains(set []string, name string) bool {
	for _, member := range set {
		if member == name {
			return true
		}
	}
	return false
}

func sharesAny(have, want []string) bool {
	for _, name := range want {
		if contains(have, name) {
			return true
		}
	}
	return false
}

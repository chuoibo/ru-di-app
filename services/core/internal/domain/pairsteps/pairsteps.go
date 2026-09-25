// Package pairsteps is the workflow of the nineteen W8 routes: the pair
// methods of services/api/app/api/service.py (pair_notebook through
// keep_pair_paper_line) with every repository call behind Store.
//
// The pair methods interleave reads, writes and rules more than the W4 money
// methods do -- granting reads the notebook back after writing the grant,
// agreeing writes a response, reads the sheet again, decides, and may create
// and link an outing -- so instead of moneysteps' callbacks each method here
// takes the whole Store and makes exactly the calls Python makes, in Python's
// order, with Python's arguments. The rules between the calls (permission
// facts, the refusal each failed predicate becomes, the state transitions, the
// consent ladder, the text normalisation) are the Python code's. A route
// implements Store over its transaction and renders the returned view.
//
// Errors a method returns:
//
//   - *Refusal is an ApiProblem: status, code and detail;
//   - *permissions.Error is PermissionError_, which Python never catches;
//   - *Conflict is a RepositoryConflict a Store returned and the method did
//     not translate;
//   - *Invariant is a failed `assert`;
//   - anything else is the Store's own failure, passed through.
//
// Everything but *Refusal ends the request as Python's 500 does.
//
// Ids are the canonical uuid strings (str(uuid.UUID)); the methods only ever
// compare them for equality. `now` is the service clock read once per request
// (`_now()`, datetime.now(UTC) truncated to microseconds). Versions a client
// sends are int64: a larger or smaller Python int saturates, and since stored
// versions are INTEGER every comparison answers as Python's does.
//
// testdata/python_pair_steps*.json is rendered by
// scripts/render_domain_w8_goldens.py by running the real ApiService methods
// over a recording stub repository with the clock pinned; oracle_test.go
// replays every case through a recording Store, comparing the answer and
// every repository call with its arguments.
package pairsteps

import (
	"errors"
	"slices"
	"strings"
	"time"

	"mobile/services/core/internal/domain/contexts"
	"mobile/services/core/internal/domain/direct"
	"mobile/services/core/internal/domain/pairnotebook"
	"mobile/services/core/internal/domain/pairpaper"
	"mobile/services/core/internal/domain/permissions"
)

// DieuKhoanHienTai is DIEU_KHOAN_HIEN_TAI: the consent wording a grant is
// given against.
const DieuKhoanHienTai = 1

// khungGio and khungViec are _KHUNG_MAC_DINH: the skeleton a fresh sheet
// arrives pre-filled with. Its di_tiep is None.
const (
	khungGio  = "18:30"
	khungViec = "Ăn tối"
)

// Actor is the service's Actor: its id and the roles the session carries.
type Actor struct {
	ID    string
	Roles []string
}

// Refusal is an ApiProblem.
type Refusal struct {
	Status int
	Code   string
	Detail string
}

func (r *Refusal) Error() string { return r.Code }

// Conflict is RepositoryConflict: a Store returns it when a persistence
// invariant refuses a write.
type Conflict struct {
	Code string
}

func (c *Conflict) Error() string { return c.Code }

// Invariant is an `assert` of the service that did not hold.
type Invariant struct {
	Reason string
}

func (e *Invariant) Error() string { return "pairsteps: " + e.Reason }

// Context is the part of ContextRecord the pair doors read.
type Context struct {
	Kind string
}

// Member is one row of list_members.
type Member struct {
	PersonID string
	State    string
}

// Consent is PairConsentRecord, the keys `_consents_as_dicts` reads.
type Consent struct {
	ProposalID        string
	PersonID          string
	Purpose           string
	GrantedAt         *time.Time
	RevokedAt         *time.Time
	ProposalExpiresAt time.Time
}

// Proposal is PairProposalRecord, the fields the methods read.
type Proposal struct {
	ID           string
	CycleID      string
	Purpose      string
	ProposedByID string
	CompletedAt  *time.Time
	ExpiresAt    time.Time
}

// Constraint is PairConstraintRecord, and PairConstraintResponse.
type Constraint struct {
	OwnerID string
	Kind    string
	Content string
	Version int
}

// Notebook is PairNotebookRecord. CycleID and CycleState are nil without a
// live cycle.
type Notebook struct {
	ID           string
	CycleID      *string
	CycleState   *string
	Participants []string
	Consents     []Consent
	Proposals    []Proposal
	Constraints  []Constraint
}

// Version is PairVersionRecord. Content is the stored JSONB as json.loads
// hands it to Python: nil, bool, string, *big.Int, float64, []any or *Object.
type Version struct {
	Version    int
	Content    any
	LyDo       *string
	AuthorType string
	SentAt     *time.Time
	SentBy     *string
}

// View is PairViewRecord.
type View struct {
	Version  int
	PersonID string
	SeenAt   time.Time
}

// Response is PairResponseRecord.
type Response struct {
	Version  int
	PersonID string
	Kind     string
}

// Keep is PairKeepRecord, and PaperKeepResponse.
type Keep struct {
	ID        string
	Line      string
	CreatedAt time.Time
}

// Paper is PairPaperRecord, the fields the methods read.
type Paper struct {
	ID             string
	ContextID      string
	CycleID        *string
	IsTemporary    bool
	DraftOwnerID   string
	State          string
	CurrentVersion int
	Tuan           pairpaper.Date
	ExpiresAt      time.Time
	OutingID       *string
	Versions       []Version
	Views          []View
	Responses      []Response
	Keeps          []Keep
}

// ProposalDraft is create_consent_proposal's arguments.
type ProposalDraft struct {
	CycleID      string
	Purpose      string
	ProposedByID string
	TermsVersion int
	ExpiresAt    time.Time
	Now          time.Time
}

// ConstraintDraft is set_pair_constraint's arguments.
type ConstraintDraft struct {
	CycleID string
	OwnerID string
	Kind    string
	Content string
	Now     time.Time
}

// PaperDraft is create_pair_paper's arguments.
type PaperDraft struct {
	ContextID    string
	CycleID      *string
	DraftOwnerID string
	Tuan         pairpaper.Date
	ExpiresAt    time.Time
	Content      pairpaper.Content
	LyDo         *string
	Nguon        pairpaper.Nguon
	AuthorType   string
	Now          time.Time
}

// VersionDraft is add_paper_version's arguments.
type VersionDraft struct {
	PaperID    string
	Version    int
	Content    pairpaper.Content
	LyDo       *string
	Nguon      pairpaper.Nguon
	AuthorType string
	SentAt     time.Time
	SentBy     string
	Now        time.Time
}

// OutingDraft is create_outing's arguments.
type OutingDraft struct {
	ContextID          string
	CreatedByID        string
	Title              string
	StartsOn           pairpaper.Date
	EndsOn             pairpaper.Date
	Headcount          int
	BudgetPerPersonVND int64
	Now                time.Time
}

// Store is the part of ApiRepository the pair methods call, one method per
// repository method with its arguments in the Protocol's order. A write that
// a persistence invariant refuses returns *Conflict with the repository's code.
type Store interface {
	GetContext(contextID string) (*Context, error)
	IsMember(contextID, personID string) (bool, error)
	ListMembers(contextID string) ([]Member, error)

	GetPairNotebook(contextID string) (*Notebook, error)
	CreatePairNotebook(contextID string, now time.Time) error
	LockPairNotebook(contextID string) (*Notebook, error)
	OpenPairCycle(notebookID string, participants []string, termsVersion int, now time.Time) (string, error)
	ActivatePairCycle(cycleID string, now time.Time) error
	ClosePairCycle(cycleID string, now time.Time) error
	CreateConsentProposal(draft ProposalDraft) (Proposal, error)
	GetConsentProposal(proposalID string) (*Proposal, error)
	GrantConsent(proposalID, personID string, now time.Time) error
	CompleteConsentProposal(proposalID string, now time.Time) error
	RevokeConsents(cycleID, purpose, personID string, now time.Time) error
	SetCoupleMember(personID, cycleID string, now time.Time) error
	ClearCoupleMember(personID string) error
	SetPairConstraint(draft ConstraintDraft) (Constraint, error)
	DeletePairConstraint(cycleID, ownerID, kind string) error

	CreatePairPaper(draft PaperDraft) (Paper, error)
	GetPairPaper(paperID string) (*Paper, error)
	LockPairPaper(paperID string) (*Paper, error)
	ListPairPapers(contextID string) ([]Paper, error)
	UpdatePairDraft(paperID string, content pairpaper.Content, lyDo *string) error
	AddPaperVersion(draft VersionDraft) error
	MarkVersionSent(paperID string, version int, sentBy *string, now time.Time) error
	SetPaperState(paperID, state string, now time.Time, currentVersion *int, recordedByID *string) error
	MarkPaperViewed(paperID string, version int, personID string, now time.Time) error
	AddPaperResponse(paperID string, version int, personID, kind string, now time.Time) error
	LinkPaperOuting(paperID string, version int, outingID string, now time.Time) error
	GetPaperOuting(paperID string) (*string, error)
	AddPaperKeep(paperID, personID, line string, now time.Time) (Keep, error)
	CloseOpenPairPapers(contextID string, now time.Time) error
	CreateOuting(draft OutingDraft) (string, error)
	// GetPlace is get_place: one catalogue row, nil when the id is unknown.
	GetPlace(placeID string) (*PlaceRef, error)
	// ListPlaces is list_places(destination_id=..., category=...), in id order.
	ListPlaces(destinationID, category string) ([]PlaceRef, error)
	// ReplaceOutingStops is replace_outing_stops with expected_revision=None.
	ReplaceOutingStops(outingID string, stops []OutingStopDraft) error
	// InterestsByPerson is interests_by_person: people with no tags are absent.
	InterestsByPerson(personIDs []string) (map[string][]string, error)
}

// PlaceRef is the part of a catalogue row _chot and draft_pair_paper read
// (`PlaceRecord.to_row()`): Kinds and Traits hold only the str items.
type PlaceRef struct {
	ID            string
	Name          string
	DestinationID string
	Category      string
	Kinds         []string
	Traits        []string
	Rating        *float64
	RatingCount   *int64
}

// row is the PlaceRef as lam_giau_phac reads it.
func (p PlaceRef) row() pairpaper.PlaceRow {
	return pairpaper.PlaceRow{ID: p.ID, Name: p.Name, Category: p.Category, Kinds: p.Kinds, Traits: p.Traits,
		Rating: p.Rating, RatingCount: p.RatingCount}
}

// OutingStopDraft is one element of replace_outing_stops' `stops`.
type OutingStopDraft struct {
	MinuteOfDay int64
	Label       string
	PlaceName   *string
	PlaceID     *string
}

// permissionRefusals is _TU_CHOI_TO_GIAY: the failed predicates that answer
// in the wire's words rather than as a 403.
var permissionRefusals = map[string]Refusal{
	"may_view_paper":            {404, "paper_not_found", "Không có tờ giấy này."},
	"is_draft_owner":            {404, "paper_not_found", "Không có tờ giấy này."},
	"proposal_in_force":         {409, "consent_proposal_expired", "Lời đề nghị này đã hết hạn."},
	"paper_unseen_unanswered":   {409, "paper_not_withdrawable", "Người kia đã mở tờ này rồi, không rút lại được."},
	"version_current":           {409, "paper_version_stale", "Tờ giấy đã sang phiên bản mới."},
	"is_not_version_sender":     {409, "paper_self_response", "Đây là tờ bạn gửi, chờ người kia trả lời."},
	"cycle_active_or_temporary": {409, "cycle_not_active", "Sổ chưa mở. Cả hai cùng đồng ý lập sổ trước đã."},
}

// PermissionRefusals returns a copy of _TU_CHOI_TO_GIAY.
func PermissionRefusals() map[string]Refusal {
	out := make(map[string]Refusal, len(permissionRefusals))
	for name, refusal := range permissionRefusals {
		out[name] = refusal
	}
	return out
}

// paperErrorDetails is _LOI_TO_GIAY: one sentence per pair_paper refusal.
var paperErrorDetails = map[string]string{
	"paper_expired":          "Tuần này hết rồi. Tuần sau mình rủ lại nhé.",
	"paper_frozen":           "Hai bạn chốt rồi, không sửa nữa.",
	"paper_wrong_state":      "Tờ giấy không ở trạng thái làm được việc này.",
	"paper_not_withdrawable": "Người kia đã mở tờ này rồi, không rút lại được.",
	"paper_needs_recorder":   "Cần biết ai ghi là hai bạn đã đi.",
	"paper_event_unknown":    "Không làm được việc này với tờ giấy.",
	"paper_draft_needs_date": "Tờ giấy cần một ngày.",
}

// PaperErrorDetails returns a copy of _LOI_TO_GIAY.
func PaperErrorDetails() map[string]string {
	out := make(map[string]string, len(paperErrorDetails))
	for code, detail := range paperErrorDetails {
		out[code] = detail
	}
	return out
}

func refusal(status int, code, detail string) error {
	return &Refusal{Status: status, Code: code, Detail: detail}
}

// fact is one entry of the context dict `_require_pair_permission` receives.
type fact struct {
	name  string
	holds bool
}

// requirePairPermission is _require_pair_permission: _require_permission with
// the facts that hold (`proved is True`), then the refusal said in the wire's
// words when the failed predicate has some.
func requirePairPermission(action string, actor Actor, facts ...fact) error {
	var proven []string
	for _, f := range facts {
		if f.holds {
			proven = append(proven, f.name)
		}
	}
	reason, allowed, err := permissions.DenialReason(action, permissions.AuthorizationFacts{
		ActorID:    actor.ID,
		Roles:      actor.Roles,
		Proven:     proven,
		Provenance: "api_service",
	})
	if err != nil {
		return err
	}
	if allowed {
		return nil
	}
	if mapped, ok := permissionRefusals[reason]; ok {
		return &mapped
	}
	return refusal(403, "permission_denied", reason)
}

// pairContextOr404 is _pair_context_or_404: the active members of the pair
// the actor is in, or one 404 for no context, a group, and a stranger.
func pairContextOr404(s Store, actor Actor, contextID string) ([]string, error) {
	record, err := s.GetContext(contextID)
	if err != nil {
		return nil, err
	}
	if record == nil || record.Kind != direct.KindPair {
		return nil, refusal(404, "notebook_not_found", "Không có sổ này.")
	}
	member, err := s.IsMember(contextID, actor.ID)
	if err != nil {
		return nil, err
	}
	if !member {
		return nil, refusal(404, "notebook_not_found", "Không có sổ này.")
	}
	rows, err := s.ListMembers(contextID)
	if err != nil {
		return nil, err
	}
	members := []string{}
	for _, row := range rows {
		if row.State == "active" {
			members = append(members, row.PersonID)
		}
	}
	return members, nil
}

// Participants is _participants: the live cycle's own list, or the
// conversation's active members before any cycle exists.
func Participants(notebook *Notebook, members []string) []string {
	if notebook != nil && notebook.CycleID != nil {
		return notebook.Participants
	}
	return members
}

// ConsentsOf is `_consents_as_dicts`, for a nil notebook the empty list.
func ConsentsOf(notebook *Notebook) []pairnotebook.Consent {
	if notebook == nil {
		return []pairnotebook.Consent{}
	}
	// Which proposal each answer belongs to, and whether it was completed:
	// «both agreed» is per proposal, and an agreed proposal no longer lapses.
	completed := map[string]*time.Time{}
	for _, row := range notebook.Proposals {
		completed[row.ID] = row.CompletedAt
	}
	out := make([]pairnotebook.Consent, len(notebook.Consents))
	for i, row := range notebook.Consents {
		expires := row.ProposalExpiresAt
		out[i] = pairnotebook.Consent{
			PersonID:            row.PersonID,
			Purpose:             row.Purpose,
			GrantedAt:           row.GrantedAt,
			RevokedAt:           row.RevokedAt,
			ProposalExpiresAt:   &expires,
			ProposalID:          row.ProposalID,
			ProposalCompletedAt: completed[row.ProposalID],
		}
	}
	return out
}

// proposalOf is the {"completed_at", "expires_at"} (and id) dict dang_cho and
// the close preview read.
func proposalOf(row Proposal) pairnotebook.Proposal {
	expires := row.ExpiresAt
	return pairnotebook.Proposal{ID: row.ID, CompletedAt: row.CompletedAt, ExpiresAt: &expires}
}

// PaperDict is `_paper_dict`.
func PaperDict(paper *Paper) pairpaper.Paper {
	expires := paper.ExpiresAt
	return pairpaper.Paper{ID: paper.ID, State: paper.State, CurrentVersion: paper.CurrentVersion, ExpiresAt: &expires}
}

// lockedNotebook is _locked_notebook: the notebook row, created when this is
// the first write, then locked again.
func lockedNotebook(s Store, contextID string, now time.Time) (*Notebook, error) {
	notebook, err := s.LockPairNotebook(contextID)
	if err != nil {
		return nil, err
	}
	if notebook == nil {
		if err := s.CreatePairNotebook(contextID, now); err != nil {
			return nil, err
		}
		if notebook, err = s.LockPairNotebook(contextID); err != nil {
			return nil, err
		}
	}
	if notebook == nil {
		return nil, &Invariant{Reason: "lock_pair_notebook found nothing after create_pair_notebook"}
	}
	return notebook, nil
}

// isActive is `cycle_state == "active"`.
func isActive(notebook *Notebook) bool {
	return notebook.CycleState != nil && *notebook.CycleState == "active"
}

// stripOrNone is `(value or "").strip() or None`.
func stripOrNone(value *string) *string {
	if value == nil {
		return nil
	}
	stripped := contexts.Strip(*value)
	if stripped == "" {
		return nil
	}
	return &stripped
}

// conflictCode is the code of a *Conflict, if err is one.
func conflictCode(err error) (string, bool) {
	var conflict *Conflict
	if errors.As(err, &conflict) {
		return conflict.Code, true
	}
	return "", false
}

// lower is the str.lower() the service applies to a repository code; the
// codes are ASCII.
func lower(code string) string { return strings.ToLower(code) }

func contains(values []string, want string) bool { return slices.Contains(values, want) }

//go:build postgres

package repo

// The call sequences of the nineteen W8 routes, written against the Go
// repository, for the `route.*` steps of pair_repo_oracle_postgres_test.go.
// The Python side of such a step runs the real ApiService method; this side
// runs the repository calls that method makes, in its order, with the
// service's decisions (permission predicates, the paper state machine, the
// consent ladder, the close revision) spelled out here in the fewest lines
// that decide the same branch. This is test code, not the service port: it
// exists so the oracle proves, statement by statement, that the Go repository
// called in this order is what the Python route does.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
)

// routeRefusal is an ApiProblem: what the route answers instead of a body.
type routeRefusal struct {
	status int
	code   string
}

func (r *routeRefusal) Error() string { return fmt.Sprintf("%d:%s", r.status, r.code) }

func refuse(status int, code string) error { return &routeRefusal{status, code} }

var (
	pairOpen        = map[string]bool{"nhap": true, "da_gui": true, "da_xem": true, "de_nghi_sua": true, "dong_y": true}
	pairPlan        = map[string]bool{"chot": true, "da_di": true}
	pairPurposes    = []string{"lap_so", "bat_doi", "doc_chat"}
	pairKinds       = map[string]bool{"khong_an_duoc": true, "dung": true}
	pairEventStates = map[string][]string{
		"gui": {"nhap"}, "xem": {"da_gui", "da_xem"}, "dong_y": {"da_gui", "da_xem", "dong_y"},
		"de_nghi_sua": {"da_gui", "da_xem", "dong_y"}, "rut": {"da_gui"},
		"nghi_tuan": {"nhap", "da_gui", "da_xem", "de_nghi_sua", "dong_y"}, "da_di": {"chot"}, "giu": {"da_di", "da_giu"},
	}
	canonicalUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)

type pairRoute struct {
	repo    Repository
	actor   string
	now     time.Time
	nowText time.Time
	a       map[string]any
}

// isoformat is datetime.isoformat() of the instant the case wrote, in the
// offset it was written with.
func (p *pairRoute) isoformat() string {
	t := p.nowText
	out := t.Format("2006-01-02T15:04:05")
	if micro := t.Nanosecond() / 1000; micro != 0 {
		out += fmt.Sprintf(".%06d", micro)
	}
	return out + t.Format("-07:00")
}

func pyStrip(s string) string {
	return strings.TrimFunc(s, func(r rune) bool { return unicode.IsSpace(r) || (r >= 0x1c && r <= 0x1f) })
}

func containsText(values []string, v string) bool {
	for _, x := range values {
		if x == v {
			return true
		}
	}
	return false
}

// --- the notebook ---------------------------------------------------------

func (p *pairRoute) contextOr404(contextID string) ([]string, error) {
	c, err := p.repo.GetContext(bg, contextID)
	if err != nil {
		return nil, err
	}
	if c == nil || c.Kind != "pair" {
		return nil, refuse(404, "notebook_not_found")
	}
	member, err := p.repo.IsMember(bg, contextID, p.actor)
	if err != nil {
		return nil, err
	}
	if !member {
		return nil, refuse(404, "notebook_not_found")
	}
	rows, err := p.repo.ListMembers(bg, contextID)
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

func (p *pairRoute) lockedNotebook(contextID string) (*PairNotebook, error) {
	notebook, err := p.repo.LockPairNotebook(bg, contextID)
	if err != nil || notebook != nil {
		return notebook, err
	}
	if _, err := p.repo.CreatePairNotebook(bg, contextID, p.now); err != nil {
		return nil, err
	}
	notebook, err = p.repo.LockPairNotebook(bg, contextID)
	if err == nil && notebook == nil {
		err = errors.New("AssertionError")
	}
	return notebook, err
}

func participantsOf(notebook *PairNotebook, members []string) []string {
	if notebook != nil && notebook.CycleID != nil {
		return notebook.Participants
	}
	return members
}

func (p *pairRoute) grantedBy(consents []PairConsent, person string) map[string]bool {
	out := map[string]bool{}
	for _, c := range consents {
		if c.PersonID == person && containsText(pairPurposes, c.Purpose) && c.GrantedAt != nil && c.RevokedAt == nil &&
			p.now.Before(c.ProposalExpiresAt) {
			out[c.Purpose] = true
		}
	}
	return out
}

func (p *pairRoute) grantedPurposes(consents []PairConsent, participants []string) map[string]bool {
	people := map[string]bool{}
	for _, person := range participants {
		people[person] = true
	}
	out := map[string]bool{}
	if len(people) < 2 {
		return out
	}
	for _, purpose := range pairPurposes {
		all := true
		for person := range people {
			if !p.grantedBy(consents, person)[purpose] {
				all = false
			}
		}
		out[purpose] = all
	}
	return out
}

func (p *pairRoute) dangCho(completed *time.Time, expires time.Time) bool {
	return completed == nil && p.now.Before(expires)
}

func (p *pairRoute) text(key string) string { return argString(p.a, key) }

func (p *pairRoute) body() map[string]any { return p.a["body"].(map[string]any) }

func (p *pairRoute) proposeConsent() error {
	members, err := p.contextOr404(p.text("context_id"))
	if err != nil {
		return err
	}
	purpose := p.body()["purpose"].(string)
	notebook, err := p.lockedNotebook(p.text("context_id"))
	if err != nil {
		return err
	}
	if purpose != "lap_so" && (notebook.CycleState == nil || *notebook.CycleState != "active") {
		return refuse(409, "consent_missing")
	}
	cycle := notebook.CycleID
	if cycle == nil {
		if len(members) < 2 {
			return refuse(409, "cycle_not_active")
		}
		id, err := p.repo.OpenPairCycle(bg, notebook.ID, members, 1, p.now)
		if err != nil {
			return err
		}
		cycle = &id
	}
	proposal, err := p.repo.CreateConsentProposal(bg, ConsentProposalInput{CycleID: *cycle, Purpose: purpose,
		ProposedByID: p.actor, TermsVersion: 1, ExpiresAt: pythonInstant(p.now).Add(7 * 24 * time.Hour), Now: p.now})
	if err != nil {
		return err
	}
	return p.repo.GrantConsent(bg, proposal.ID, p.actor, p.now)
}

func (p *pairRoute) grantConsent() error {
	members, err := p.contextOr404(p.text("context_id"))
	if err != nil {
		return err
	}
	notebook, err := p.lockedNotebook(p.text("context_id"))
	if err != nil {
		return err
	}
	proposal, err := p.repo.GetConsentProposal(bg, p.text("proposal_id"))
	if err != nil {
		return err
	}
	if proposal == nil || notebook.CycleID == nil || proposal.CycleID != *notebook.CycleID {
		return refuse(404, "consent_proposal_not_found")
	}
	participants := participantsOf(notebook, members)
	if !containsText(participants, p.actor) || proposal.ProposedByID == p.actor {
		return refuse(403, "permission_denied")
	}
	if !p.dangCho(proposal.CompletedAt, proposal.ExpiresAt) {
		return refuse(409, "consent_proposal_expired")
	}
	if err := p.repo.GrantConsent(bg, proposal.ID, p.actor, p.now); err != nil {
		return err
	}
	after, err := p.repo.GetPairNotebook(bg, p.text("context_id"))
	if err != nil {
		return err
	}
	if !p.grantedPurposes(after.Consents, participants)[proposal.Purpose] {
		return nil
	}
	if err := p.repo.CompleteConsentProposal(bg, proposal.ID, p.now); err != nil {
		return err
	}
	switch proposal.Purpose {
	case "lap_so":
		return p.repo.ActivatePairCycle(bg, *notebook.CycleID, p.now)
	case "bat_doi":
		for _, person := range participants {
			err := p.repo.SetCoupleMember(bg, person, *notebook.CycleID, p.now)
			var conflict *Conflict
			if errors.As(err, &conflict) {
				return refuse(409, strings.ToLower(conflict.Code))
			}
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *pairRoute) revokeConsent() error {
	purpose := p.text("purpose")
	if !containsText(pairPurposes, purpose) {
		return refuse(404, "consent_purpose_unknown")
	}
	members, err := p.contextOr404(p.text("context_id"))
	if err != nil {
		return err
	}
	notebook, err := p.lockedNotebook(p.text("context_id"))
	if err != nil || notebook.CycleID == nil {
		return err
	}
	if _, err := p.repo.RevokeConsents(bg, *notebook.CycleID, purpose, p.actor, p.now); err != nil {
		return err
	}
	if purpose == "bat_doi" {
		for _, person := range participantsOf(notebook, members) {
			if err := p.repo.ClearCoupleMember(bg, person); err != nil {
				return err
			}
		}
	}
	if purpose == "doc_chat" {
		papers, err := p.repo.ListPairPapers(bg, p.text("context_id"))
		if err != nil {
			return err
		}
		for _, paper := range papers {
			if paper.State != "nhap" {
				continue
			}
			for _, v := range paper.Versions {
				if v.Version == 1 {
					if v.AuthorType == "nep" {
						if err := p.repo.SetPaperState(bg, PaperStateInput{PaperID: paper.ID, State: "bo", Now: p.now}); err != nil {
							return err
						}
					}
					break
				}
			}
		}
	}
	return nil
}

func (p *pairRoute) putConstraint() error {
	kind := p.text("kind")
	if !pairKinds[kind] {
		return refuse(404, "constraint_kind_unknown")
	}
	if _, err := p.contextOr404(p.text("context_id")); err != nil {
		return err
	}
	notebook, err := p.lockedNotebook(p.text("context_id"))
	if err != nil {
		return err
	}
	if notebook.CycleID == nil {
		return refuse(409, "cycle_not_active")
	}
	_, err = p.repo.SetPairConstraint(bg, PairConstraintInput{CycleID: *notebook.CycleID, OwnerID: p.actor, Kind: kind,
		Content: pyStrip(p.body()["content"].(string)), Now: p.now})
	return err
}

func (p *pairRoute) deleteConstraint() error {
	kind := p.text("kind")
	if !pairKinds[kind] {
		return refuse(404, "constraint_kind_unknown")
	}
	if _, err := p.contextOr404(p.text("context_id")); err != nil {
		return err
	}
	notebook, err := p.lockedNotebook(p.text("context_id"))
	if err != nil || notebook.CycleID == nil {
		return err
	}
	_, err = p.repo.DeletePairConstraint(bg, *notebook.CycleID, p.actor, kind)
	return err
}

// closeRevision is `_xem_truoc_dong_so(...)["revision"]`, with its reads.
func (p *pairRoute) closeRevision(contextID string) (string, error) {
	notebook, err := p.repo.GetPairNotebook(bg, contextID)
	if err != nil {
		return "", err
	}
	papers, err := p.repo.ListPairPapers(bg, contextID)
	if err != nil {
		return "", err
	}
	var material []string
	for _, paper := range papers {
		material = append(material, paper.ID+":"+p.hieuLuc(paper))
	}
	if notebook != nil {
		for _, proposal := range notebook.Proposals {
			if p.dangCho(proposal.CompletedAt, proposal.ExpiresAt) {
				material = append(material, "dn:"+proposal.ID)
			}
		}
	}
	sort.Strings(material)
	sum := sha256.Sum256([]byte(strings.Join(material, "\n")))
	return hex.EncodeToString(sum[:])[:16], nil
}

func (p *pairRoute) closeNotebook() error {
	contextID := p.text("context_id")
	members, err := p.contextOr404(contextID)
	if err != nil {
		return err
	}
	notebook, err := p.lockedNotebook(contextID)
	if err != nil {
		return err
	}
	revision, err := p.closeRevision(contextID)
	if err != nil {
		return err
	}
	if given := p.text("revision"); given != "@current" && given != revision {
		return refuse(409, "notebook_revision_stale")
	}
	if _, err := p.repo.CloseOpenPairPapers(bg, contextID, p.now); err != nil {
		return err
	}
	if notebook.CycleID == nil {
		return nil
	}
	for _, person := range participantsOf(notebook, members) {
		if err := p.repo.ClearCoupleMember(bg, person); err != nil {
			return err
		}
	}
	return p.repo.ClosePairCycle(bg, *notebook.CycleID, p.now)
}

// --- the paper ------------------------------------------------------------

func (p *pairRoute) hieuLuc(paper PairPaper) string {
	if pairOpen[paper.State] && !p.now.Before(paper.ExpiresAt) {
		return "het_han"
	}
	return paper.State
}

// chuyen is pair_paper.chuyen: the state after the event, and for
// de_nghi_sua the next version number.
func (p *pairRoute) chuyen(paper PairPaper, event string, facts ...string) (string, int64, error) {
	state := p.hieuLuc(paper)
	if state == "het_han" && pairOpen[paper.State] {
		return "", 0, refuse(409, "paper_expired")
	}
	if event == "de_nghi_sua" && pairPlan[state] {
		return "", 0, refuse(409, "paper_frozen")
	}
	if !containsText(pairEventStates[event], state) {
		return "", 0, refuse(409, "paper_wrong_state")
	}
	fact := func(name string) bool { return containsText(facts, name) }
	switch event {
	case "gui":
		return "da_gui", paper.CurrentVersion, nil
	case "xem":
		return "da_xem", paper.CurrentVersion, nil
	case "dong_y":
		if fact("du_dong_y") {
			return "chot", paper.CurrentVersion, nil
		}
		return "dong_y", paper.CurrentVersion, nil
	case "de_nghi_sua":
		return "da_gui", paper.CurrentVersion + 1, nil
	case "rut":
		if !fact("co_the_rut") {
			return "", 0, refuse(409, "paper_not_withdrawable")
		}
		return "rut", paper.CurrentVersion, nil
	case "nghi_tuan":
		if state == "nhap" {
			return "nghi_tuan", paper.CurrentVersion, nil
		}
		return "huy", paper.CurrentVersion, nil
	case "da_di":
		return "da_di", paper.CurrentVersion, nil
	}
	return "da_giu", paper.CurrentVersion, nil
}

func (p *pairRoute) readablePaper(paperID string) (*PairPaper, error) {
	paper, err := p.repo.GetPairPaper(bg, paperID)
	if err != nil {
		return nil, err
	}
	if paper == nil {
		return nil, refuse(404, "paper_not_found")
	}
	if _, err := p.contextOr404(paper.ContextID); err != nil {
		return nil, err
	}
	if paper.State == "nhap" && paper.DraftOwnerID != p.actor {
		return nil, refuse(404, "paper_not_found")
	}
	return paper, nil
}

func (p *pairRoute) lockedPaper() (*PairPaper, error) {
	paper, err := p.readablePaper(p.text("paper_id"))
	if err != nil {
		return nil, err
	}
	locked, err := p.repo.LockPairPaper(bg, paper.ID)
	if err != nil {
		return nil, err
	}
	if locked == nil {
		return nil, refuse(404, "paper_not_found")
	}
	return locked, nil
}

func versionOf(paper *PairPaper, version int64) *PairVersion {
	for i := range paper.Versions {
		if paper.Versions[i].Version == version {
			return &paper.Versions[i]
		}
	}
	return nil
}

// contentDay is `_noi_dung_wire(content).ngay`, false where it raises.
func contentDay(content json.RawMessage) (time.Time, bool) {
	var fields map[string]any
	if err := json.Unmarshal(content, &fields); err != nil {
		return time.Time{}, false
	}
	text, ok := fields["ngay"].(string)
	if !ok {
		return time.Time{}, false
	}
	day, err := time.Parse("2006-01-02", text)
	if err != nil {
		return time.Time{}, false
	}
	stops, present := fields["chang"]
	if !present {
		return day, true
	}
	list, ok := stops.([]any)
	if !ok {
		return time.Time{}, false
	}
	for _, item := range list {
		stop, ok := item.(map[string]any)
		if !ok {
			return time.Time{}, false
		}
		if _, ok := stop["gio"]; !ok {
			return time.Time{}, false
		}
		if _, ok := stop["viec"]; !ok {
			return time.Time{}, false
		}
		if place, ok := stop["place_id"].(string); ok && place != "" && !canonicalUUID.MatchString(place) {
			return time.Time{}, false
		}
	}
	return day, true
}

// dayOf is `_ngay_cua(paper)`.
func dayOf(paper *PairPaper) *time.Time {
	current := versionOf(paper, paper.CurrentVersion)
	if current == nil {
		return nil
	}
	var fields map[string]any
	if err := json.Unmarshal(current.Content, &fields); err != nil {
		return nil
	}
	text, ok := fields["ngay"].(string)
	if !ok {
		return nil
	}
	day, err := time.Parse("2006-01-02", text)
	if err != nil {
		return nil
	}
	return &day
}

// storedContent is `_noi_dung_luu(PaperContentInput)` for a body the cases
// write canonically.
func storedContent(body map[string]any) json.RawMessage {
	var stops []map[string]any
	for _, item := range body["chang"].([]any) {
		stop := item.(map[string]any)
		canKiem, present := stop["can_kiem"]
		if !present {
			canKiem = true
		}
		stops = append(stops, map[string]any{"gio": stop["gio"], "viec": stop["viec"], "place_id": stop["place_id"],
			"can_kiem": canKiem})
	}
	out, err := json.Marshal(map[string]any{"ngay": body["ngay"], "chang": stops})
	if err != nil {
		panic(err)
	}
	return out
}

func (p *pairRoute) reason() *string {
	text, _ := p.body()["ly_do"].(string)
	if stripped := pyStrip(text); stripped != "" {
		return &stripped
	}
	return nil
}

func (p *pairRoute) bodyVersion() int64 { return argNumber(p.body()["version"]) }

func (p *pairRoute) setState(paper *PairPaper, state string) error {
	return p.repo.SetPaperState(bg, PaperStateInput{PaperID: paper.ID, State: state, Now: p.now})
}

func (p *pairRoute) readPaper() error {
	paper, err := p.readablePaper(p.text("paper_id"))
	if err != nil {
		return err
	}
	for _, v := range paper.Versions {
		if _, ok := contentDay(v.Content); !ok {
			return refuse(409, "paper_wrong_state")
		}
	}
	return nil
}

func (p *pairRoute) draftPaper() error {
	contextID := p.text("context_id")
	if _, err := p.contextOr404(contextID); err != nil {
		return err
	}
	notebook, err := p.lockedNotebook(contextID)
	if err != nil {
		return err
	}
	if !(notebook.CycleID == nil || (notebook.CycleState != nil && *notebook.CycleState == "active")) {
		return refuse(409, "cycle_not_active")
	}
	papers, err := p.repo.ListPairPapers(bg, contextID)
	if err != nil {
		return err
	}
	for _, paper := range papers {
		if pairOpen[p.hieuLuc(paper)] {
			return refuse(409, "paper_wrong_state")
		}
	}
	local := p.now.In(wallClockLocation)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
	monday := today.AddDate(0, 0, -((int(local.Weekday()) + 6) % 7))
	saturday := monday.AddDate(0, 0, 5)
	if saturday.Before(today) {
		saturday = today
	}
	weekEnd := time.Date(monday.Year(), monday.Month(), monday.Day()+7, 0, 0, 0, 0, wallClockLocation).UTC()
	uses := []string{"routine"}
	if len(notebook.Constraints) > 0 {
		uses = append(uses, "rang_buoc")
	}
	content, _ := json.Marshal(map[string]any{"ngay": saturday.Format("2006-01-02"),
		"chang": []any{map[string]any{"gio": "18:30", "viec": "Ăn tối", "place_id": nil, "can_kiem": true}}})
	nguon, _ := json.Marshal(map[string]any{"scope": "chung", "dung": uses, "luc": p.isoformat()})
	_, err = p.repo.CreatePairPaper(bg, PairPaperInput{ContextID: contextID, CycleID: notebook.CycleID,
		DraftOwnerID: p.actor, Tuan: monday, ExpiresAt: weekEnd, Content: content, Nguon: nguon, AuthorType: "human",
		Now: p.now})
	return err
}

func (p *pairRoute) editDraft() error {
	paper, err := p.lockedPaper()
	if err != nil {
		return err
	}
	if paper.DraftOwnerID != p.actor {
		return refuse(404, "paper_not_found")
	}
	if p.hieuLuc(*paper) != "nhap" {
		return refuse(409, "paper_wrong_state")
	}
	return p.repo.UpdatePairDraft(bg, paper.ID, storedContent(p.body()["content"].(map[string]any)), p.reason())
}

func (p *pairRoute) sendPaper() error {
	paper, err := p.lockedPaper()
	if err != nil {
		return err
	}
	if paper.DraftOwnerID != p.actor {
		return refuse(404, "paper_not_found")
	}
	if p.bodyVersion() != paper.CurrentVersion {
		return refuse(409, "paper_version_stale")
	}
	state, _, err := p.chuyen(*paper, "gui")
	if err != nil {
		return err
	}
	if err := p.repo.MarkVersionSent(bg, paper.ID, paper.CurrentVersion, &p.actor, p.now); err != nil {
		return err
	}
	if err := p.repo.AddPaperResponse(bg, PaperResponseInput{PaperID: paper.ID, Version: paper.CurrentVersion,
		PersonID: p.actor, Kind: "dong_y", Now: p.now}); err != nil {
		return err
	}
	return p.setState(paper, state)
}

func (p *pairRoute) markViewed() error {
	paper, err := p.lockedPaper()
	if err != nil {
		return err
	}
	version := argNumber(p.a["version"])
	row := versionOf(paper, version)
	if row == nil {
		return refuse(404, "paper_not_found")
	}
	if row.SentBy != nil && *row.SentBy == p.actor {
		return refuse(409, "paper_self_response")
	}
	if _, err := p.repo.MarkPaperViewed(bg, paper.ID, version, p.actor, p.now); err != nil {
		return err
	}
	if version != paper.CurrentVersion || paper.State != "da_gui" {
		return nil
	}
	state, _, err := p.chuyen(*paper, "xem")
	if err != nil {
		return err
	}
	return p.setState(paper, state)
}

func (p *pairRoute) respond() error {
	paper, err := p.lockedPaper()
	if err != nil {
		return err
	}
	version := argNumber(p.a["version"])
	row := versionOf(paper, version)
	if row == nil {
		return refuse(404, "paper_not_found")
	}
	if version != paper.CurrentVersion {
		return refuse(409, "paper_version_stale")
	}
	if row.SentBy != nil && *row.SentBy == p.actor {
		return refuse(409, "paper_self_response")
	}
	body := p.body()
	if body["kind"] == "dong_y" {
		err := p.repo.AddPaperResponse(bg, PaperResponseInput{PaperID: paper.ID, Version: version, PersonID: p.actor,
			Kind: "dong_y", Now: p.now})
		var conflict *Conflict
		if errors.As(err, &conflict) && conflict.Code == "paper_already_agreed" {
			again, err := p.repo.GetPairPaper(bg, paper.ID)
			if err == nil && again == nil {
				return refuse(404, "paper_not_found")
			}
			return err
		}
		if err != nil {
			return err
		}
		after, err := p.repo.GetPairPaper(bg, paper.ID)
		if err != nil {
			return err
		}
		agreed := map[string]bool{}
		for _, r := range after.Responses {
			if r.Kind == "dong_y" && r.Version == version {
				agreed[r.PersonID] = true
			}
		}
		var facts []string
		if len(agreed) >= 2 {
			facts = append(facts, "du_dong_y")
		}
		state, _, err := p.chuyen(*paper, "dong_y", facts...)
		if err != nil {
			return err
		}
		if state == "chot" {
			if err := p.chot(after, version); err != nil {
				return err
			}
		}
		return p.setState(paper, state)
	}
	state, next, err := p.chuyen(*paper, "de_nghi_sua")
	if err != nil {
		return err
	}
	if err := p.repo.AddPaperResponse(bg, PaperResponseInput{PaperID: paper.ID, Version: version, PersonID: p.actor,
		Kind: "de_nghi_sua", Now: p.now}); err != nil {
		return err
	}
	nguon, _ := json.Marshal(map[string]any{"scope": "chung", "dung": []string{"nguoi"}, "luc": p.isoformat()})
	sent := p.now
	if err := p.repo.AddPaperVersion(bg, PaperVersionInput{PaperID: paper.ID, Version: next,
		Content: storedContent(body["content"].(map[string]any)), LyDo: p.reason(), Nguon: nguon, AuthorType: "human",
		SentAt: &sent, SentBy: &p.actor, Now: p.now}); err != nil {
		return err
	}
	if err := p.repo.AddPaperResponse(bg, PaperResponseInput{PaperID: paper.ID, Version: next, PersonID: p.actor,
		Kind: "dong_y", Now: p.now}); err != nil {
		return err
	}
	return p.repo.SetPaperState(bg, PaperStateInput{PaperID: paper.ID, State: state, Now: p.now, CurrentVersion: &next})
}

func (p *pairRoute) chot(paper *PairPaper, version int64) error {
	existing, err := p.repo.GetPaperOuting(bg, paper.ID)
	if err != nil || existing != nil {
		return err
	}
	current := versionOf(paper, version)
	if current == nil {
		return refuse(409, "paper_wrong_state")
	}
	day, ok := contentDay(current.Content)
	if !ok {
		return refuse(409, "paper_wrong_state")
	}
	outing, err := p.repo.CreateOuting(bg, OutingInput{ContextID: paper.ContextID, CreatedByID: p.actor,
		Title: "Tờ lời rủ " + day.Format("02/01"), StartsOn: day, EndsOn: day, Headcount: 2, Now: p.now})
	if err != nil {
		return err
	}
	err = p.repo.LinkPaperOuting(bg, paper.ID, version, outing.ID, p.now)
	var conflict *Conflict
	if errors.As(err, &conflict) {
		already, err := p.repo.GetPaperOuting(bg, paper.ID)
		if err == nil && already == nil {
			return refuse(409, strings.ToLower(conflict.Code))
		}
		return err
	}
	return err
}

func (p *pairRoute) withdraw() error {
	paper, err := p.lockedPaper()
	if err != nil {
		return err
	}
	able := paper.State == "da_gui"
	current := versionOf(paper, paper.CurrentVersion)
	if current == nil || current.AuthorType != "human" || current.SentBy == nil || *current.SentBy != p.actor {
		able = false
	}
	for _, v := range paper.Views {
		if v.Version == paper.CurrentVersion && v.PersonID != p.actor {
			able = false
		}
	}
	for _, r := range paper.Responses {
		if r.Version == paper.CurrentVersion && r.PersonID != p.actor {
			able = false
		}
	}
	if !able {
		return refuse(409, "paper_not_withdrawable")
	}
	if p.bodyVersion() != paper.CurrentVersion {
		return refuse(409, "paper_version_stale")
	}
	state, _, err := p.chuyen(*paper, "rut", "co_the_rut")
	if err != nil {
		return err
	}
	return p.setState(paper, state)
}

func (p *pairRoute) skipWeek() error {
	paper, err := p.lockedPaper()
	if err != nil {
		return err
	}
	state, _, err := p.chuyen(*paper, "nghi_tuan")
	if err != nil {
		return err
	}
	return p.setState(paper, state)
}

func (p *pairRoute) recordDone() error {
	paper, err := p.lockedPaper()
	if err != nil {
		return err
	}
	day := dayOf(paper)
	if p.hieuLuc(*paper) != "chot" || day == nil || WallClockDate(p.now).Before(*day) {
		return refuse(409, "paper_wrong_state")
	}
	state, _, err := p.chuyen(*paper, "da_di")
	if err != nil {
		return err
	}
	return p.repo.SetPaperState(bg, PaperStateInput{PaperID: paper.ID, State: state, Now: p.now, RecordedByID: &p.actor})
}

func (p *pairRoute) keepLine() error {
	paper, err := p.lockedPaper()
	if err != nil {
		return err
	}
	state, _, err := p.chuyen(*paper, "giu")
	if err != nil {
		return err
	}
	if _, err := p.repo.AddPaperKeep(bg, paper.ID, p.actor, pyStrip(p.body()["line"].(string)), p.now); err != nil {
		return err
	}
	return p.setState(paper, state)
}

// pairRouteGo runs one `route.*` step. It answers nil, as the Python step does.
func pairRouteGo(repo Repository, name string, a map[string]any) (any, error) {
	written := argInstant(argString(a, "now"))
	p := &pairRoute{repo: repo, actor: argString(a, "actor_id"), now: pythonInstant(written), nowText: written, a: a}
	var err error
	switch name {
	case "route.pair_notebook":
		if _, err = p.contextOr404(p.text("context_id")); err == nil {
			if _, err = repo.GetPairNotebook(bg, p.text("context_id")); err == nil {
				_, err = repo.ListPairPapers(bg, p.text("context_id"))
			}
		}
	case "route.propose_pair_consent":
		err = p.proposeConsent()
	case "route.grant_pair_consent":
		err = p.grantConsent()
	case "route.revoke_pair_consent":
		err = p.revokeConsent()
	case "route.put_pair_constraint":
		err = p.putConstraint()
	case "route.delete_pair_constraint":
		err = p.deleteConstraint()
	case "route.preview_close_pair_notebook":
		if _, err = p.contextOr404(p.text("context_id")); err == nil {
			_, err = p.closeRevision(p.text("context_id"))
		}
	case "route.close_pair_notebook":
		err = p.closeNotebook()
	case "route.list_pair_papers":
		if _, err = p.contextOr404(p.text("context_id")); err == nil {
			_, err = repo.ListPairPapers(bg, p.text("context_id"))
		}
	case "route.draft_pair_paper":
		err = p.draftPaper()
	case "route.pair_paper":
		err = p.readPaper()
	case "route.edit_pair_draft":
		err = p.editDraft()
	case "route.send_pair_paper":
		err = p.sendPaper()
	case "route.mark_pair_paper_viewed":
		err = p.markViewed()
	case "route.respond_pair_paper":
		err = p.respond()
	case "route.withdraw_pair_paper":
		err = p.withdraw()
	case "route.skip_pair_week":
		err = p.skipWeek()
	case "route.record_pair_outing_done":
		err = p.recordDone()
	case "route.keep_pair_paper_line":
		err = p.keepLine()
	default:
		panic("unknown route " + name)
	}
	return nil, err
}

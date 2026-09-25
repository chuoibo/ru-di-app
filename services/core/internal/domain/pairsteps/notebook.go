package pairsteps

import (
	"slices"
	"time"

	"mobile/services/core/internal/domain/contexts"
	"mobile/services/core/internal/domain/pairnotebook"
	"mobile/services/core/internal/domain/pairpaper"
)

// ProposalView is PairProposalResponse.
type ProposalView struct {
	ID           string
	Purpose      string
	ExpiresAt    time.Time
	ProposedByID string
	MyGranted    bool
}

// ConsentState is one purpose of the ladder and whether it is granted.
type ConsentState struct {
	Purpose string
	Granted bool
}

// NotebookView is PairNotebookResponse. TheirConsentsGranted is the dict,
// keyed in CONSENT_PURPOSES order.
type NotebookView struct {
	ContextID            string
	CycleState           *string
	Participants         []string
	MyConsents           []ConsentState
	TheirConsentsGranted []ConsentState
	PendingProposals     []ProposalView
	Constraints          []Constraint
	NepGuiHo             bool
	OpenPaperID          *string
	// GrantedPurposes is what BOTH agreed to on one proposal, in ladder order:
	// the only reading a screen may light a rung on (QA 23/09).
	GrantedPurposes []string
	// Taste is _pair_taste: nil outside «Một đôi» (ADR-0034).
	Taste *pairnotebook.Taste
	// WeekRole is _week_role: nil outside an open «Một đôi» (ADR-0034 §2.4).
	WeekRole *WeekRole
}

// WeekRole is PairWeekRoleResponse.
type WeekRole struct {
	Tuan    pairpaper.Date
	NguoiLo []string
	Cach    string
	Diem    []pairnotebook.Diem
}

// ReadNotebook is pair_notebook (GET /contexts/{context_id}/notebook).
func ReadNotebook(s Store, actor Actor, contextID string, now time.Time) (NotebookView, error) {
	members, err := pairContextOr404(s, actor, contextID)
	if err != nil {
		return NotebookView{}, err
	}
	if err := requirePairPermission("view_pair_notebook", actor, fact{"is_group_member", true}); err != nil {
		return NotebookView{}, err
	}
	notebook, err := s.GetPairNotebook(contextID)
	if err != nil {
		return NotebookView{}, err
	}
	participants := Participants(notebook, members)
	var other *string
	for _, person := range participants {
		if person != actor.ID {
			other = &person
			break
		}
	}
	consents := ConsentsOf(notebook)
	mine := pairnotebook.GrantedBy(consents, actor.ID, &now)
	theirs := []string{}
	if other != nil {
		theirs = pairnotebook.GrantedBy(consents, *other, &now)
	}
	view := NotebookView{
		ContextID:        contextID,
		Participants:     append([]string{}, participants...),
		PendingProposals: []ProposalView{},
		Constraints:      []Constraint{},
	}
	if notebook != nil {
		view.CycleState = notebook.CycleState
		for _, row := range notebook.Proposals {
			if !pairnotebook.DangCho(proposalOf(row), now) {
				continue
			}
			view.PendingProposals = append(view.PendingProposals, ProposalView{
				ID:           row.ID,
				Purpose:      row.Purpose,
				ExpiresAt:    row.ExpiresAt,
				ProposedByID: row.ProposedByID,
				MyGranted:    contains(mine, row.Purpose),
			})
		}
		view.Constraints = append(view.Constraints, notebook.Constraints...)
	}
	for _, purpose := range pairnotebook.ConsentPurposes() {
		view.MyConsents = append(view.MyConsents, ConsentState{Purpose: purpose, Granted: contains(mine, purpose)})
		view.TheirConsentsGranted = append(view.TheirConsentsGranted, ConsentState{Purpose: purpose, Granted: contains(theirs, purpose)})
	}
	// One read of the sheets serves the open sheet and the week's role; then
	// the week's choice, then the tastes -- Python's order.
	papers, err := s.ListPairPapers(contextID)
	if err != nil {
		return NotebookView{}, err
	}
	view.OpenPaperID = openPaperIn(papers, actor, now)
	if view.WeekRole, err = weekRole(s, notebook, consents, participants, papers, now); err != nil {
		return NotebookView{}, err
	}
	view.GrantedPurposes = pairnotebook.GrantedPurposes(consents, participants, &now)
	if view.Taste, err = pairTaste(s, consents, participants, actor, now); err != nil {
		return NotebookView{}, err
	}
	return view, nil
}

// weekRole is _week_role (ADR-0034 §2.4).
func weekRole(s Store, notebook *Notebook, consents []pairnotebook.Consent, participants []string, papers []Paper, now time.Time) (*WeekRole, error) {
	if notebook == nil || notebook.CycleID == nil || !isActive(notebook) || !pairnotebook.CanBatDoi(consents, participants, &now) {
		return nil, nil
	}
	tuan := pairpaper.TuanCua(now)
	chon, err := s.GetPairRhythm(*notebook.CycleID, tuan)
	if err != nil {
		return nil, err
	}
	var nguoiLapSo *string
	var luc *time.Time
	for _, p := range notebook.Proposals {
		if p.Purpose == "lap_so" && p.CompletedAt != nil && (luc == nil || p.CompletedAt.After(*luc)) {
			id := p.ProposedByID
			nguoiLapSo, luc = &id, p.CompletedAt
		}
	}
	toGiay := make([]pairnotebook.ToTinHieu, len(papers))
	for i, paper := range papers {
		to := pairnotebook.ToTinHieu{CycleID: paper.CycleID, Tuan: paper.Tuan.ISOFormat()}
		for _, v := range paper.Versions {
			to.Versions = append(to.Versions, pairnotebook.PhienBanTinHieu{Version: v.Version, AuthorType: v.AuthorType, SentBy: v.SentBy, SentAt: v.SentAt})
		}
		for _, r := range paper.Responses {
			to.Responses = append(to.Responses, pairnotebook.TraLoiTinHieu{PersonID: r.PersonID, Kind: r.Kind})
		}
		toGiay[i] = to
	}
	suy := pairnotebook.NguoiLoSuy(participants, toGiay, *notebook.CycleID, nguoiLapSo)
	var chosen **string
	if chon != nil {
		id := chon.NguoiLoID
		chosen = &id
	}
	moLoiTruoc := []*string{
		pairnotebook.NguoiMoLoi(toGiay, *notebook.CycleID, tuan.AddDays(-7).ISOFormat()),
		pairnotebook.NguoiMoLoi(toGiay, *notebook.CycleID, tuan.AddDays(-14).ISOFormat()),
	}
	vai := pairnotebook.VaiTuan(suy, chosen, participants, moLoiTruoc)
	return &WeekRole{Tuan: tuan, NguoiLo: vai.NguoiLo, Cach: vai.Cach, Diem: vai.Diem}, nil
}

// SetWeekRole is set_pair_week_role: «Anh lo / Em lo / Hôm nay mình share».
// lo is the request's, which pydantic held to toi|nguoi_kia|ca_hai.
func SetWeekRole(s Store, actor Actor, contextID, lo string, now time.Time) (WeekRole, error) {
	members, err := pairContextOr404(s, actor, contextID)
	if err != nil {
		return WeekRole{}, err
	}
	if err := requirePairPermission("set_pair_week_role", actor, fact{"is_group_member", true}); err != nil {
		return WeekRole{}, err
	}
	notebook, err := lockedNotebook(s, contextID, now)
	if err != nil {
		return WeekRole{}, err
	}
	participants := Participants(notebook, members)
	consents := ConsentsOf(notebook)
	if notebook.CycleID == nil || !isActive(notebook) || !pairnotebook.CanBatDoi(consents, participants, &now) {
		return WeekRole{}, refusal(409, "consent_missing", "Hai bạn bật «Một đôi» trước đã.")
	}
	var nguoiLo *string
	switch lo {
	case "toi":
		id := actor.ID
		nguoiLo = &id
	case "nguoi_kia":
		for _, p := range participants {
			if p != actor.ID {
				id := p
				nguoiLo = &id
				break
			}
		}
	}
	if err := s.SetPairRhythm(RhythmDraft{CycleID: *notebook.CycleID, Tuan: pairpaper.TuanCua(now), NguoiLoID: nguoiLo, ChonBoiID: actor.ID, Now: now}); err != nil {
		return WeekRole{}, err
	}
	papers, err := s.ListPairPapers(contextID)
	if err != nil {
		return WeekRole{}, err
	}
	role, err := weekRole(s, notebook, consents, participants, papers, now)
	if err != nil {
		return WeekRole{}, err
	}
	if role == nil {
		return WeekRole{}, &Invariant{Reason: "week role missing right after it was set"}
	}
	return *role, nil
}

// pairTaste is _pair_taste: tastes are read only once the domain has said a
// couple exists; outside «Một đôi» nobody's tags are fetched at all.
func pairTaste(s Store, consents []pairnotebook.Consent, participants []string, actor Actor, now time.Time) (*pairnotebook.Taste, error) {
	if !pairnotebook.CanBatDoi(consents, participants, &now) {
		return nil, nil
	}
	tags, err := s.InterestsByPerson(append([]string{}, participants...))
	if err != nil {
		return nil, err
	}
	return pairnotebook.GuHaiNguoi(consents, participants, actor.ID, tags, now), nil
}

// openPaperID is _open_paper_id: the first sheet in play, skipping a draft
// that belongs to the other person.
func openPaperID(s Store, actor Actor, contextID string, now time.Time) (*string, error) {
	papers, err := s.ListPairPapers(contextID)
	if err != nil {
		return nil, err
	}
	return openPaperIn(papers, actor, now), nil
}

// openPaperIn is _open_paper_id over sheets already read.
func openPaperIn(papers []Paper, actor Actor, now time.Time) *string {
	for i := range papers {
		paper := &papers[i]
		state := pairpaper.HieuLuc(PaperDict(paper), now)
		if !pairpaper.IsOpen(state) {
			continue
		}
		if state == "nhap" && paper.DraftOwnerID != actor.ID {
			continue
		}
		id := paper.ID
		return &id
	}
	return nil
}

// ProposeConsent is propose_pair_consent. purpose is the request's, which
// pydantic has already held to CONSENT_PURPOSES.
func ProposeConsent(s Store, actor Actor, contextID, purpose string, now time.Time) (ProposalView, error) {
	members, err := pairContextOr404(s, actor, contextID)
	if err != nil {
		return ProposalView{}, err
	}
	if err := requirePairPermission("propose_pair_consent", actor, fact{"is_group_member", true}); err != nil {
		return ProposalView{}, err
	}
	notebook, err := lockedNotebook(s, contextID, now)
	if err != nil {
		return ProposalView{}, err
	}
	cycleID := notebook.CycleID
	if purpose != "lap_so" && !isActive(notebook) {
		return ProposalView{}, refusal(409, "consent_missing", "Cả hai cùng đồng ý lập sổ trước đã.")
	}
	if slices.Contains(pairnotebook.PerPersonPurposes(), purpose) {
		return proposePerPerson(s, notebook, members, purpose, actor, now)
	}
	// One offer per rung at a time. The other person already asking for the
	// same thing is an offer to ANSWER, by id: a second proposal made each of
	// them agree only with themselves, and the rung read «both» with nothing
	// completed (QA 23/09). Asking twice oneself returns the standing offer.
	// An offer stands only while its proposer's own yes on it is live: one the
	// proposer took back is dead, and asking again files a new one (each person
	// answers each proposal once, so that is the only way to say yes again).
	for _, row := range notebook.Proposals {
		if row.Purpose != purpose || !proposerStillAgrees(notebook, row) || !pairnotebook.DangCho(proposalOf(row), now) {
			continue
		}
		if row.ProposedByID != actor.ID {
			return ProposalView{}, refusal(409, "consent_proposal_pending", "Người ấy đã đề nghị đúng việc này. Đồng ý lời đề nghị của họ.")
		}
		return proposalView(row), nil
	}
	if cycleID == nil {
		if len(members) < 2 {
			return ProposalView{}, refusal(409, "cycle_not_active", "Sổ này chưa đủ hai người.")
		}
		opened, err := s.OpenPairCycle(notebook.ID, members, DieuKhoanHienTai, now)
		if err != nil {
			return ProposalView{}, err
		}
		cycleID = &opened
	}
	proposal, err := s.CreateConsentProposal(ProposalDraft{
		CycleID:      *cycleID,
		Purpose:      purpose,
		ProposedByID: actor.ID,
		TermsVersion: DieuKhoanHienTai,
		ExpiresAt:    pairnotebook.HanDeNghi(now),
		Now:          now,
	})
	if err != nil {
		return ProposalView{}, err
	}
	if err := s.GrantConsent(proposal.ID, actor.ID, now); err != nil {
		return ProposalView{}, err
	}
	return proposalView(proposal), nil
}

// proposePerPerson is _propose_per_person (ADR-0034 §2.1): only inside «Một
// đôi»; filed, granted by its proposer and completed in one go, so nobody is
// ever left to «agree» to somebody else's taste. Asking again while it is on
// returns the proposal in force.
func proposePerPerson(s Store, notebook *Notebook, members []string, purpose string, actor Actor, now time.Time) (ProposalView, error) {
	participants := Participants(notebook, members)
	if !pairnotebook.CanBatDoi(ConsentsOf(notebook), participants, &now) {
		return ProposalView{}, refusal(409, "consent_missing", "Hai bạn bật «Một đôi» trước đã.")
	}
	for _, row := range notebook.Consents {
		if row.PersonID != actor.ID || row.GrantedAt == nil || row.RevokedAt != nil {
			continue
		}
		for _, proposal := range notebook.Proposals {
			if proposal.ID == row.ProposalID && proposal.Purpose == purpose && proposal.CompletedAt != nil {
				return proposalView(proposal), nil
			}
		}
	}
	proposal, err := s.CreateConsentProposal(ProposalDraft{
		CycleID:      *notebook.CycleID,
		Purpose:      purpose,
		ProposedByID: actor.ID,
		TermsVersion: DieuKhoanHienTai,
		ExpiresAt:    pairnotebook.HanDeNghi(now),
		Now:          now,
	})
	if err != nil {
		return ProposalView{}, err
	}
	if err := s.GrantConsent(proposal.ID, actor.ID, now); err != nil {
		return ProposalView{}, err
	}
	if err := s.CompleteConsentProposal(proposal.ID, now); err != nil {
		return ProposalView{}, err
	}
	return proposalView(proposal), nil
}

// proposerStillAgrees: the proposer's own grant on this proposal is live.
func proposerStillAgrees(notebook *Notebook, proposal Proposal) bool {
	for _, row := range notebook.Consents {
		if row.ProposalID == proposal.ID && row.PersonID == proposal.ProposedByID && row.GrantedAt != nil && row.RevokedAt == nil {
			return true
		}
	}
	return false
}

func proposalView(proposal Proposal) ProposalView {
	return ProposalView{
		ID:           proposal.ID,
		Purpose:      proposal.Purpose,
		ExpiresAt:    proposal.ExpiresAt,
		ProposedByID: proposal.ProposedByID,
		MyGranted:    true,
	}
}

// GrantConsent is grant_pair_consent: the second yes, and what it unlocks.
func GrantConsent(s Store, actor Actor, contextID, proposalID string, now time.Time) (ProposalView, error) {
	members, err := pairContextOr404(s, actor, contextID)
	if err != nil {
		return ProposalView{}, err
	}
	notebook, err := lockedNotebook(s, contextID, now)
	if err != nil {
		return ProposalView{}, err
	}
	proposal, err := s.GetConsentProposal(proposalID)
	if err != nil {
		return ProposalView{}, err
	}
	if proposal == nil || notebook.CycleID == nil || proposal.CycleID != *notebook.CycleID {
		return ProposalView{}, refusal(404, "consent_proposal_not_found", "Không có lời đề nghị này.")
	}
	participants := Participants(notebook, members)
	if err := requirePairPermission("grant_pair_consent", actor,
		fact{"is_invitee", contains(participants, actor.ID) && proposal.ProposedByID != actor.ID},
		fact{"proposal_in_force", pairnotebook.DangCho(proposalOf(*proposal), now)},
	); err != nil {
		return ProposalView{}, err
	}
	if err := s.GrantConsent(proposal.ID, actor.ID, now); err != nil {
		return ProposalView{}, err
	}
	after, err := s.GetPairNotebook(contextID)
	if err != nil {
		return ProposalView{}, err
	}
	if after == nil {
		return ProposalView{}, &Invariant{Reason: "get_pair_notebook found nothing after grant_consent"}
	}
	both := pairnotebook.GrantedPurposes(ConsentsOf(after), participants, &now)
	if contains(both, proposal.Purpose) {
		if err := s.CompleteConsentProposal(proposal.ID, now); err != nil {
			return ProposalView{}, err
		}
		switch proposal.Purpose {
		case "lap_so":
			if err := s.ActivatePairCycle(*notebook.CycleID, now); err != nil {
				return ProposalView{}, err
			}
		case "bat_doi":
			for _, person := range participants {
				if err := s.SetCoupleMember(person, *notebook.CycleID, now); err != nil {
					if code, ok := conflictCode(err); ok {
						return ProposalView{}, refusal(409, lower(code), "Một trong hai người đang là một đôi ở sổ khác.")
					}
					return ProposalView{}, err
				}
			}
		}
	}
	return proposalView(*proposal), nil
}

// RevokeConsent is revoke_pair_consent. purpose is the path segment as sent.
func RevokeConsent(s Store, actor Actor, contextID, purpose string, now time.Time) error {
	if !slices.Contains(pairnotebook.ConsentPurposes(), purpose) {
		return refusal(404, "consent_purpose_unknown", "Không có mục đích này.")
	}
	members, err := pairContextOr404(s, actor, contextID)
	if err != nil {
		return err
	}
	if err := requirePairPermission("revoke_pair_consent", actor, fact{"is_self", true}); err != nil {
		return err
	}
	notebook, err := lockedNotebook(s, contextID, now)
	if err != nil {
		return err
	}
	if notebook.CycleID == nil {
		return nil
	}
	if err := s.RevokeConsents(*notebook.CycleID, purpose, actor.ID, now); err != nil {
		return err
	}
	if purpose == "bat_doi" {
		for _, person := range Participants(notebook, members) {
			if err := s.ClearCoupleMember(person); err != nil {
				return err
			}
		}
	}
	if purpose == "doc_chat" {
		return dropUnsentNepDrafts(s, contextID, now)
	}
	return nil
}

// dropUnsentNepDrafts is _drop_unsent_nep_drafts: every stored draft whose
// first version Nep wrote becomes `bo`, read by stored state.
func dropUnsentNepDrafts(s Store, contextID string, now time.Time) error {
	papers, err := s.ListPairPapers(contextID)
	if err != nil {
		return err
	}
	for i := range papers {
		paper := &papers[i]
		if paper.State != "nhap" {
			continue
		}
		first := versionNumbered(paper, 1)
		if first != nil && first.AuthorType == "nep" {
			if err := s.SetPaperState(paper.ID, "bo", now, nil, nil); err != nil {
				return err
			}
		}
	}
	return nil
}

// PutConstraint is put_pair_constraint. kind is the path segment as sent;
// content is the request's, stripped again as the service does.
func PutConstraint(s Store, actor Actor, contextID, kind, content string, now time.Time) (Constraint, error) {
	if !slices.Contains(pairnotebook.ConstraintKinds(), kind) {
		return Constraint{}, refusal(404, "constraint_kind_unknown", "Không có ô này.")
	}
	if _, err := pairContextOr404(s, actor, contextID); err != nil {
		return Constraint{}, err
	}
	if err := requirePairPermission("edit_pair_constraint", actor, fact{"is_self", true}); err != nil {
		return Constraint{}, err
	}
	notebook, err := lockedNotebook(s, contextID, now)
	if err != nil {
		return Constraint{}, err
	}
	if notebook.CycleID == nil {
		return Constraint{}, refusal(409, "cycle_not_active", "Sổ chưa mở.")
	}
	return s.SetPairConstraint(ConstraintDraft{
		CycleID: *notebook.CycleID,
		OwnerID: actor.ID,
		Kind:    kind,
		Content: contexts.Strip(content),
		Now:     now,
	})
}

// DeleteConstraint is delete_pair_constraint.
func DeleteConstraint(s Store, actor Actor, contextID, kind string, now time.Time) error {
	if !slices.Contains(pairnotebook.ConstraintKinds(), kind) {
		return refusal(404, "constraint_kind_unknown", "Không có ô này.")
	}
	if _, err := pairContextOr404(s, actor, contextID); err != nil {
		return err
	}
	if err := requirePairPermission("edit_pair_constraint", actor, fact{"is_self", true}); err != nil {
		return err
	}
	notebook, err := lockedNotebook(s, contextID, now)
	if err != nil {
		return err
	}
	if notebook.CycleID == nil {
		return nil
	}
	return s.DeletePairConstraint(*notebook.CycleID, actor.ID, kind)
}

// Digest is sha256.Sum256, which the domain may not import.
type Digest func([]byte) [32]byte

// preview is _xem_truoc_dong_so.
func preview(s Store, contextID string, now time.Time, sum256 Digest) (pairnotebook.ClosePreview, error) {
	notebook, err := s.GetPairNotebook(contextID)
	if err != nil {
		return pairnotebook.ClosePreview{}, err
	}
	papers, err := s.ListPairPapers(contextID)
	if err != nil {
		return pairnotebook.ClosePreview{}, err
	}
	dicts := make([]pairpaper.Paper, len(papers))
	for i := range papers {
		dicts[i] = PaperDict(&papers[i])
	}
	proposals := []pairnotebook.Proposal{}
	if notebook != nil {
		for _, row := range notebook.Proposals {
			proposals = append(proposals, proposalOf(row))
		}
	}
	return pairnotebook.XemTruocDongSo(dicts, proposals, now, sum256), nil
}

// PreviewClose is preview_close_pair_notebook.
func PreviewClose(s Store, actor Actor, contextID string, now time.Time, sum256 Digest) (pairnotebook.ClosePreview, error) {
	if _, err := pairContextOr404(s, actor, contextID); err != nil {
		return pairnotebook.ClosePreview{}, err
	}
	if err := requirePairPermission("preview_close_pair_notebook", actor, fact{"is_group_member", true}); err != nil {
		return pairnotebook.ClosePreview{}, err
	}
	return preview(s, contextID, now, sum256)
}

// CloseNotebook is close_pair_notebook: the revision must still describe the
// notebook, then open sheets close, the couple rows go, and the cycle ends.
func CloseNotebook(s Store, actor Actor, contextID, revision string, now time.Time, sum256 Digest) error {
	members, err := pairContextOr404(s, actor, contextID)
	if err != nil {
		return err
	}
	if err := requirePairPermission("close_pair_notebook", actor, fact{"is_group_member", true}); err != nil {
		return err
	}
	notebook, err := lockedNotebook(s, contextID, now)
	if err != nil {
		return err
	}
	current, err := preview(s, contextID, now, sum256)
	if err != nil {
		return err
	}
	if current.Revision != revision {
		return refusal(409, "notebook_revision_stale", "Sổ vừa đổi. Xem lại rồi đóng.")
	}
	if err := s.CloseOpenPairPapers(contextID, now); err != nil {
		return err
	}
	if notebook.CycleID == nil {
		return nil
	}
	for _, person := range Participants(notebook, members) {
		if err := s.ClearCoupleMember(person); err != nil {
			return err
		}
	}
	return s.ClosePairCycle(*notebook.CycleID, now)
}

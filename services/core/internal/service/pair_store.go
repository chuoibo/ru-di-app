package service

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"mobile/services/core/internal/db"
	"mobile/services/core/internal/domain/pairpaper"
	"mobile/services/core/internal/domain/pairsteps"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/repo"
)

// PairStore is pairsteps.Store over the request's unit of work: one
// repository method per call, in the order the pairsteps method makes them,
// which is Python's (the repository oracle pins each route's sequence).
//
// The transaction is taken on the first call, not before, as get_repository's
// session begins on its first statement: a pair method that refuses before
// reading anything (an unknown purpose or kind) begins nothing.
//
// A write refused by a persistence invariant comes back from the repository
// as *repo.Conflict and leaves here as *pairsteps.Conflict with the same code,
// for the method to translate or not, as ApiService does. Every other error
// passes through unchanged.
type PairStore struct {
	Ctx  context.Context
	Unit *db.Unit
}

var _ pairsteps.Store = PairStore{}

func (s PairStore) repository() (repo.Repository, error) {
	tx, err := s.Unit.Tx(s.Ctx)
	if err != nil {
		return repo.Repository{}, err
	}
	return repo.Repository{Q: tx}, nil
}

// storeError is the error a Store method returns for a repository error.
func storeError(err error) error {
	var conflict *repo.Conflict
	if errors.As(err, &conflict) {
		return &pairsteps.Conflict{Code: conflict.Code}
	}
	return err
}

// GetContext is get_context.
func (s PairStore) GetContext(contextID string) (*pairsteps.Context, error) {
	r, err := s.repository()
	if err != nil {
		return nil, err
	}
	record, err := r.GetContext(s.Ctx, contextID)
	if err != nil || record == nil {
		return nil, storeError(err)
	}
	return &pairsteps.Context{Kind: record.Kind}, nil
}

// IsMember is is_member.
func (s PairStore) IsMember(contextID, personID string) (bool, error) {
	r, err := s.repository()
	if err != nil {
		return false, err
	}
	member, err := r.IsMember(s.Ctx, contextID, personID)
	return member, storeError(err)
}

// ListMembers is list_members.
func (s PairStore) ListMembers(contextID string) ([]pairsteps.Member, error) {
	r, err := s.repository()
	if err != nil {
		return nil, err
	}
	rows, err := r.ListMembers(s.Ctx, contextID)
	if err != nil {
		return nil, storeError(err)
	}
	out := make([]pairsteps.Member, len(rows))
	for i, row := range rows {
		out[i] = pairsteps.Member{PersonID: row.PersonID, State: row.State, DisplayName: row.DisplayName}
	}
	return out, nil
}

// GetPairNotebook is get_pair_notebook.
func (s PairStore) GetPairNotebook(contextID string) (*pairsteps.Notebook, error) {
	r, err := s.repository()
	if err != nil {
		return nil, err
	}
	notebook, err := r.GetPairNotebook(s.Ctx, contextID)
	if err != nil {
		return nil, storeError(err)
	}
	return PairNotebookOf(notebook), nil
}

// CreatePairNotebook is create_pair_notebook; the service ignores its record.
func (s PairStore) CreatePairNotebook(contextID string, now time.Time) error {
	r, err := s.repository()
	if err != nil {
		return err
	}
	_, err = r.CreatePairNotebook(s.Ctx, contextID, now)
	return storeError(err)
}

// LockPairNotebook is lock_pair_notebook.
func (s PairStore) LockPairNotebook(contextID string) (*pairsteps.Notebook, error) {
	r, err := s.repository()
	if err != nil {
		return nil, err
	}
	notebook, err := r.LockPairNotebook(s.Ctx, contextID)
	if err != nil {
		return nil, storeError(err)
	}
	return PairNotebookOf(notebook), nil
}

// OpenPairCycle is open_pair_cycle.
func (s PairStore) OpenPairCycle(notebookID string, participants []string, termsVersion int, now time.Time) (string, error) {
	r, err := s.repository()
	if err != nil {
		return "", err
	}
	cycleID, err := r.OpenPairCycle(s.Ctx, notebookID, participants, int64(termsVersion), now)
	return cycleID, storeError(err)
}

// ActivatePairCycle is activate_pair_cycle.
func (s PairStore) ActivatePairCycle(cycleID string, now time.Time) error {
	r, err := s.repository()
	if err != nil {
		return err
	}
	return storeError(r.ActivatePairCycle(s.Ctx, cycleID, now))
}

// ClosePairCycle is close_pair_cycle.
func (s PairStore) ClosePairCycle(cycleID string, now time.Time) error {
	r, err := s.repository()
	if err != nil {
		return err
	}
	return storeError(r.ClosePairCycle(s.Ctx, cycleID, now))
}

// CreateConsentProposal is create_consent_proposal.
func (s PairStore) CreateConsentProposal(draft pairsteps.ProposalDraft) (pairsteps.Proposal, error) {
	r, err := s.repository()
	if err != nil {
		return pairsteps.Proposal{}, err
	}
	proposal, err := r.CreateConsentProposal(s.Ctx, repo.ConsentProposalInput{
		CycleID:      draft.CycleID,
		Purpose:      draft.Purpose,
		ProposedByID: draft.ProposedByID,
		TermsVersion: int64(draft.TermsVersion),
		ExpiresAt:    draft.ExpiresAt,
		Now:          draft.Now,
	})
	if err != nil {
		return pairsteps.Proposal{}, storeError(err)
	}
	return pairProposalOf(proposal), nil
}

// GetConsentProposal is get_consent_proposal.
func (s PairStore) GetConsentProposal(proposalID string) (*pairsteps.Proposal, error) {
	r, err := s.repository()
	if err != nil {
		return nil, err
	}
	proposal, err := r.GetConsentProposal(s.Ctx, proposalID)
	if err != nil || proposal == nil {
		return nil, storeError(err)
	}
	out := pairProposalOf(*proposal)
	return &out, nil
}

// GrantConsent is grant_consent.
func (s PairStore) GrantConsent(proposalID, personID string, now time.Time) error {
	r, err := s.repository()
	if err != nil {
		return err
	}
	return storeError(r.GrantConsent(s.Ctx, proposalID, personID, now))
}

// CompleteConsentProposal is complete_consent_proposal.
func (s PairStore) CompleteConsentProposal(proposalID string, now time.Time) error {
	r, err := s.repository()
	if err != nil {
		return err
	}
	return storeError(r.CompleteConsentProposal(s.Ctx, proposalID, now))
}

// RevokeConsents is revoke_consents; the service ignores the count.
func (s PairStore) RevokeConsents(cycleID, purpose, personID string, now time.Time) error {
	r, err := s.repository()
	if err != nil {
		return err
	}
	_, err = r.RevokeConsents(s.Ctx, cycleID, purpose, personID, now)
	return storeError(err)
}

// SetCoupleMember is set_couple_member.
func (s PairStore) SetCoupleMember(personID, cycleID string, now time.Time) error {
	r, err := s.repository()
	if err != nil {
		return err
	}
	return storeError(r.SetCoupleMember(s.Ctx, personID, cycleID, now))
}

// ClearCoupleMember is clear_couple_member.
func (s PairStore) ClearCoupleMember(personID string) error {
	r, err := s.repository()
	if err != nil {
		return err
	}
	return storeError(r.ClearCoupleMember(s.Ctx, personID))
}

// SetPairConstraint is set_pair_constraint.
func (s PairStore) SetPairConstraint(draft pairsteps.ConstraintDraft) (pairsteps.Constraint, error) {
	r, err := s.repository()
	if err != nil {
		return pairsteps.Constraint{}, err
	}
	var in repo.PairConstraintInput
	in.CycleID, in.OwnerID, in.Kind, in.Content, in.Now = draft.CycleID, draft.OwnerID, draft.Kind, draft.Content, draft.Now
	constraint, err := r.SetPairConstraint(s.Ctx, in)
	if err != nil {
		return pairsteps.Constraint{}, storeError(err)
	}
	return pairConstraintOf(constraint), nil
}

// DeletePairConstraint is delete_pair_constraint; the service ignores whether
// a row went.
func (s PairStore) DeletePairConstraint(cycleID, ownerID, kind string) error {
	r, err := s.repository()
	if err != nil {
		return err
	}
	_, err = r.DeletePairConstraint(s.Ctx, cycleID, ownerID, kind)
	return storeError(err)
}

// CreatePairPaper is create_pair_paper.
func (s PairStore) CreatePairPaper(draft pairsteps.PaperDraft) (pairsteps.Paper, error) {
	r, err := s.repository()
	if err != nil {
		return pairsteps.Paper{}, err
	}
	content, err := PairContentJSON(draft.Content)
	if err != nil {
		return pairsteps.Paper{}, err
	}
	nguon, err := pairNguonJSON(draft.Nguon)
	if err != nil {
		return pairsteps.Paper{}, err
	}
	paper, err := r.CreatePairPaper(s.Ctx, repo.PairPaperInput{
		ContextID:    draft.ContextID,
		CycleID:      draft.CycleID,
		DraftOwnerID: draft.DraftOwnerID,
		Tuan:         civilDay(draft.Tuan),
		ExpiresAt:    draft.ExpiresAt,
		Content:      content,
		LyDo:         draft.LyDo,
		Nguon:        nguon,
		AuthorType:   draft.AuthorType,
		Now:          draft.Now,
	})
	if err != nil {
		return pairsteps.Paper{}, storeError(err)
	}
	out, err := PairPaperOf(&paper)
	if err != nil {
		return pairsteps.Paper{}, err
	}
	return *out, nil
}

// GetPairPaper is get_pair_paper.
func (s PairStore) GetPairPaper(paperID string) (*pairsteps.Paper, error) {
	r, err := s.repository()
	if err != nil {
		return nil, err
	}
	paper, err := r.GetPairPaper(s.Ctx, paperID)
	if err != nil {
		return nil, storeError(err)
	}
	return PairPaperOf(paper)
}

// LockPairPaper is lock_pair_paper.
func (s PairStore) LockPairPaper(paperID string) (*pairsteps.Paper, error) {
	r, err := s.repository()
	if err != nil {
		return nil, err
	}
	paper, err := r.LockPairPaper(s.Ctx, paperID)
	if err != nil {
		return nil, storeError(err)
	}
	return PairPaperOf(paper)
}

// ListPairPapers is list_pair_papers.
func (s PairStore) ListPairPapers(contextID string) ([]pairsteps.Paper, error) {
	r, err := s.repository()
	if err != nil {
		return nil, err
	}
	papers, err := r.ListPairPapers(s.Ctx, contextID)
	if err != nil {
		return nil, storeError(err)
	}
	out := make([]pairsteps.Paper, len(papers))
	for i := range papers {
		paper, err := PairPaperOf(&papers[i])
		if err != nil {
			return nil, err
		}
		out[i] = *paper
	}
	return out, nil
}

// UpdatePairDraft is update_pair_draft.
func (s PairStore) UpdatePairDraft(paperID string, content pairpaper.Content, lyDo *string) error {
	r, err := s.repository()
	if err != nil {
		return err
	}
	encoded, err := PairContentJSON(content)
	if err != nil {
		return err
	}
	return storeError(r.UpdatePairDraft(s.Ctx, paperID, encoded, lyDo))
}

// AddPaperVersion is add_paper_version.
func (s PairStore) AddPaperVersion(draft pairsteps.VersionDraft) error {
	r, err := s.repository()
	if err != nil {
		return err
	}
	content, err := PairContentJSON(draft.Content)
	if err != nil {
		return err
	}
	nguon, err := pairNguonJSON(draft.Nguon)
	if err != nil {
		return err
	}
	sentAt, sentBy := draft.SentAt, draft.SentBy
	return storeError(r.AddPaperVersion(s.Ctx, repo.PaperVersionInput{
		PaperID:    draft.PaperID,
		Version:    int64(draft.Version),
		Content:    content,
		LyDo:       draft.LyDo,
		Nguon:      nguon,
		AuthorType: draft.AuthorType,
		SentAt:     &sentAt,
		SentBy:     &sentBy,
		Now:        draft.Now,
	}))
}

// MarkVersionSent is mark_version_sent.
func (s PairStore) MarkVersionSent(paperID string, version int, sentBy *string, now time.Time) error {
	r, err := s.repository()
	if err != nil {
		return err
	}
	return storeError(r.MarkVersionSent(s.Ctx, paperID, int64(version), sentBy, now))
}

// SetPaperState is set_paper_state.
func (s PairStore) SetPaperState(paperID, state string, now time.Time, currentVersion *int, recordedByID *string) error {
	r, err := s.repository()
	if err != nil {
		return err
	}
	in := repo.PaperStateInput{PaperID: paperID, State: state, Now: now, RecordedByID: recordedByID}
	if currentVersion != nil {
		version := int64(*currentVersion)
		in.CurrentVersion = &version
	}
	return storeError(r.SetPaperState(s.Ctx, in))
}

// MarkPaperViewed is mark_paper_viewed; the service ignores the seen_at kept.
func (s PairStore) MarkPaperViewed(paperID string, version int, personID string, now time.Time) error {
	r, err := s.repository()
	if err != nil {
		return err
	}
	_, err = r.MarkPaperViewed(s.Ctx, paperID, int64(version), personID, now)
	return storeError(err)
}

// AddPaperResponse is add_paper_response.
func (s PairStore) AddPaperResponse(paperID string, version int, personID, kind string, now time.Time) error {
	r, err := s.repository()
	if err != nil {
		return err
	}
	return storeError(r.AddPaperResponse(s.Ctx, repo.PaperResponseInput{
		PaperID: paperID, Version: int64(version), PersonID: personID, Kind: kind, Now: now,
	}))
}

// LinkPaperOuting is link_paper_outing.
func (s PairStore) LinkPaperOuting(paperID string, version int, outingID string, now time.Time) error {
	r, err := s.repository()
	if err != nil {
		return err
	}
	return storeError(r.LinkPaperOuting(s.Ctx, paperID, int64(version), outingID, now))
}

// GetPaperOuting is get_paper_outing.
func (s PairStore) GetPaperOuting(paperID string) (*string, error) {
	r, err := s.repository()
	if err != nil {
		return nil, err
	}
	outingID, err := r.GetPaperOuting(s.Ctx, paperID)
	return outingID, storeError(err)
}

// AddPaperKeep is add_paper_keep.
func (s PairStore) AddPaperKeep(paperID, personID, line string, now time.Time) (pairsteps.Keep, error) {
	r, err := s.repository()
	if err != nil {
		return pairsteps.Keep{}, err
	}
	keep, err := r.AddPaperKeep(s.Ctx, paperID, personID, line, now)
	if err != nil {
		return pairsteps.Keep{}, storeError(err)
	}
	return pairsteps.Keep{ID: keep.ID, Line: keep.Line, CreatedAt: keep.CreatedAt}, nil
}

// CloseOpenPairPapers is close_open_pair_papers; the service ignores the
// counts.
func (s PairStore) CloseOpenPairPapers(contextID string, now time.Time) error {
	r, err := s.repository()
	if err != nil {
		return err
	}
	_, err = r.CloseOpenPairPapers(s.Ctx, contextID, now)
	return storeError(err)
}

// CreateOuting is create_outing; _chot reads only the new outing's id.
func (s PairStore) CreateOuting(draft pairsteps.OutingDraft) (string, error) {
	r, err := s.repository()
	if err != nil {
		return "", err
	}
	outing, err := r.CreateOuting(s.Ctx, repo.OutingInput{
		ContextID:          draft.ContextID,
		CreatedByID:        draft.CreatedByID,
		Title:              draft.Title,
		StartsOn:           civilDay(draft.StartsOn),
		EndsOn:             civilDay(draft.EndsOn),
		Headcount:          int64(draft.Headcount),
		BudgetPerPersonVND: draft.BudgetPerPersonVND,
		Now:                draft.Now,
	})
	if err != nil {
		return "", storeError(err)
	}
	return outing.ID, nil
}

// placeRefOf is the part of `PlaceRecord.to_row()` the pair doors read.
func placeRefOf(place repo.Place) pairsteps.PlaceRef {
	return pairsteps.PlaceRef{ID: place.ID, Name: place.Name, DestinationID: place.DestinationID, Category: place.Category,
		Kinds: place.Kinds, Traits: place.Traits, Rating: place.Rating, RatingCount: place.RatingCount}
}

// GetPlace is get_place.
func (s PairStore) GetPlace(placeID string) (*pairsteps.PlaceRef, error) {
	r, err := s.repository()
	if err != nil {
		return nil, err
	}
	place, err := r.GetPlace(s.Ctx, placeID)
	if err != nil || place == nil {
		return nil, storeError(err)
	}
	ref := placeRefOf(*place)
	return &ref, nil
}

// ListPlaces is list_places(destination_id=..., category=...).
func (s PairStore) ListPlaces(destinationID, category string) ([]pairsteps.PlaceRef, error) {
	r, err := s.repository()
	if err != nil {
		return nil, err
	}
	places, err := r.ListPlaces(s.Ctx, repo.PlaceFilter{DestinationID: &destinationID, Category: &category})
	if err != nil {
		return nil, storeError(err)
	}
	out := make([]pairsteps.PlaceRef, len(places))
	for i, place := range places {
		out[i] = placeRefOf(place)
	}
	return out, nil
}

// ReplaceOutingStops is replace_outing_stops(expected_revision=None).
func (s PairStore) ReplaceOutingStops(outingID string, stops []pairsteps.OutingStopDraft) error {
	r, err := s.repository()
	if err != nil {
		return err
	}
	rows := make([]repo.TimelineStop, len(stops))
	for i, stop := range stops {
		rows[i] = repo.TimelineStop{MinuteOfDay: stop.MinuteOfDay, Label: stop.Label, PlaceName: stop.PlaceName, PlaceID: stop.PlaceID}
	}
	_, err = r.ReplaceOutingStops(s.Ctx, outingID, rows, nil)
	return storeError(err)
}

// --- records ----------------------------------------------------------------

// PairNotebookOf is the PairNotebookRecord the pair methods read, nil for
// none.
func PairNotebookOf(notebook *repo.PairNotebook) *pairsteps.Notebook {
	if notebook == nil {
		return nil
	}
	out := &pairsteps.Notebook{
		ID:           notebook.ID,
		CycleID:      notebook.CycleID,
		CycleState:   notebook.CycleState,
		Participants: append([]string{}, notebook.Participants...),
	}
	for _, row := range notebook.Consents {
		out.Consents = append(out.Consents, pairsteps.Consent{
			ProposalID:        row.ProposalID,
			PersonID:          row.PersonID,
			Purpose:           row.Purpose,
			GrantedAt:         row.GrantedAt,
			RevokedAt:         row.RevokedAt,
			ProposalExpiresAt: row.ProposalExpiresAt,
		})
	}
	for _, row := range notebook.Proposals {
		out.Proposals = append(out.Proposals, pairProposalOf(row))
	}
	for _, row := range notebook.Constraints {
		out.Constraints = append(out.Constraints, pairConstraintOf(row))
	}
	return out
}

func pairProposalOf(row repo.PairProposal) pairsteps.Proposal {
	return pairsteps.Proposal{
		ID:           row.ID,
		CycleID:      row.CycleID,
		Purpose:      row.Purpose,
		ProposedByID: row.ProposedByID,
		CompletedAt:  row.CompletedAt,
		ExpiresAt:    row.ExpiresAt,
	}
}

func pairConstraintOf(row repo.PairConstraint) pairsteps.Constraint {
	return pairsteps.Constraint{OwnerID: row.OwnerID, Kind: row.Kind, Content: row.Content, Version: row.Version}
}

// PairPaperOf is the PairPaperRecord the pair methods read, nil for none.
// Stored content is handed over as json.loads builds it from the JSONB text.
func PairPaperOf(paper *repo.PairPaper) (*pairsteps.Paper, error) {
	if paper == nil {
		return nil, nil
	}
	out := &pairsteps.Paper{
		ID:             paper.ID,
		ContextID:      paper.ContextID,
		CycleID:        paper.CycleID,
		IsTemporary:    paper.IsTemporary,
		DraftOwnerID:   paper.DraftOwnerID,
		State:          paper.State,
		CurrentVersion: int(paper.CurrentVersion),
		Tuan:           pairpaper.DateOf(paper.Tuan),
		ExpiresAt:      paper.ExpiresAt,
		OutingID:       paper.OutingID,
	}
	for _, row := range paper.Versions {
		content, err := storedJSON(row.Content)
		if err != nil {
			return nil, fmt.Errorf("service: pair paper %s version %d content: %w", paper.ID, row.Version, err)
		}
		out.Versions = append(out.Versions, pairsteps.Version{
			Version:    int(row.Version),
			Content:    content,
			LyDo:       row.LyDo,
			AuthorType: row.AuthorType,
			SentAt:     row.SentAt,
			SentBy:     row.SentBy,
		})
	}
	for _, row := range paper.Views {
		out.Views = append(out.Views, pairsteps.View{Version: int(row.Version), PersonID: row.PersonID, SeenAt: row.SeenAt})
	}
	for _, row := range paper.Responses {
		out.Responses = append(out.Responses, pairsteps.Response{Version: int(row.Version), PersonID: row.PersonID, Kind: row.Kind})
	}
	for _, row := range paper.Keeps {
		out.Keeps = append(out.Keeps, pairsteps.Keep{ID: row.ID, Line: row.Line, CreatedAt: row.CreatedAt})
	}
	return out, nil
}

// storedJSON is json.loads of a stored JSONB value, in the shape
// pairsteps.Version.Content documents.
func storedJSON(raw []byte) (any, error) {
	loaded, err := pyjson.Loads(raw)
	if err != nil {
		return nil, err
	}
	return fromLoaded(loaded)
}

func fromLoaded(value pyjson.Value) (any, error) {
	switch v := value.(type) {
	case pyjson.Null:
		return nil, nil
	case pyjson.Bool:
		return bool(v), nil
	case pyjson.Int:
		return new(big.Int).Set(v.Big()), nil
	case pyjson.Float:
		return float64(v), nil
	case pyjson.String:
		return string(v), nil
	case pyjson.List:
		out := make([]any, len(v))
		for i, item := range v {
			converted, err := fromLoaded(item)
			if err != nil {
				return nil, err
			}
			out[i] = converted
		}
		return out, nil
	case *pyjson.OrderedMap:
		out := &pairsteps.Object{}
		for key, item := range v.All() {
			converted, err := fromLoaded(item)
			if err != nil {
				return nil, err
			}
			out.Keys = append(out.Keys, key)
			out.Values = append(out.Values, converted)
		}
		return out, nil
	}
	return nil, fmt.Errorf("service: unexpected JSON value %T", value)
}

// PairContentJSON is the dict a sheet's content is written as
// ({"ngay", "chang": [{"gio", "viec", "place_id", "can_kiem"}]}), in
// json.dumps's spelling, which is what psycopg sends for a JSONB bind.
func PairContentJSON(content pairpaper.Content) ([]byte, error) {
	stops := make(pyjson.List, len(content.Chang))
	for i, stop := range content.Chang {
		item := pyjson.NewOrderedMap()
		item.Set("gio", pyjson.String(stop.Gio))
		item.Set("viec", pyjson.String(stop.Viec))
		if stop.PlaceID == nil {
			item.Set("place_id", pyjson.Null{})
		} else {
			item.Set("place_id", pyjson.String(*stop.PlaceID))
		}
		item.Set("can_kiem", pyjson.Bool(stop.CanKiem))
		stops[i] = item
	}
	out := pyjson.NewOrderedMap()
	out.Set("ngay", pyjson.String(content.Ngay))
	out.Set("chang", stops)
	return pyjson.Dumps(out)
}

// pairNguonJSON is the provenance dict {"scope", "dung", "luc"}.
func pairNguonJSON(nguon pairpaper.Nguon) ([]byte, error) {
	uses := make(pyjson.List, len(nguon.Dung))
	for i, use := range nguon.Dung {
		uses[i] = pyjson.String(use)
	}
	out := pyjson.NewOrderedMap()
	out.Set("scope", pyjson.String(nguon.Scope))
	out.Set("dung", uses)
	out.Set("luc", pyjson.String(nguon.Luc))
	return pyjson.Dumps(out)
}

// civilDay is a date as the repository takes one: midnight UTC of that day.
func civilDay(d pairpaper.Date) time.Time {
	return time.Date(d.Year, time.Month(d.Month), d.Day, 0, 0, 0, 0, time.UTC)
}

// InterestsByPerson is interests_by_person, as a map for the taste reading.
func (s PairStore) InterestsByPerson(personIDs []string) (map[string][]string, error) {
	r, err := s.repository()
	if err != nil {
		return nil, err
	}
	rows, err := r.InterestsByPerson(s.Ctx, personIDs)
	if err != nil {
		return nil, storeError(err)
	}
	out := map[string][]string{}
	for _, row := range rows {
		out[row.PersonID] = row.Tags
	}
	return out, nil
}

// GetPairRhythm is get_pair_rhythm.
func (s PairStore) GetPairRhythm(cycleID string, tuan pairpaper.Date) (*pairsteps.Rhythm, error) {
	r, err := s.repository()
	if err != nil {
		return nil, err
	}
	row, err := r.GetPairRhythm(s.Ctx, cycleID, civilDay(tuan))
	if err != nil || row == nil {
		return nil, storeError(err)
	}
	return &pairsteps.Rhythm{NguoiLoID: row.NguoiLoID}, nil
}

// SetPairRhythm is set_pair_rhythm.
func (s PairStore) SetPairRhythm(draft pairsteps.RhythmDraft) error {
	r, err := s.repository()
	if err != nil {
		return err
	}
	_, err = r.SetPairRhythm(s.Ctx, repo.PairRhythmInput{CycleID: draft.CycleID, Tuan: civilDay(draft.Tuan),
		NguoiLoID: draft.NguoiLoID, ChonBoiID: draft.ChonBoiID, Now: draft.Now})
	return storeError(err)
}

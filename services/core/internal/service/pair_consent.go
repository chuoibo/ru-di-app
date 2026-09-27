package service

import (
	"context"
	"slices"
	"time"

	"mobile/services/core/internal/domain/pairnotebook"
	"mobile/services/core/internal/domain/pairsteps"
	"mobile/services/core/internal/repo"
)

// kindPair is app.domain.direct.KIND_PAIR.
const kindPair = "pair"

// PairChatConsent is ApiService._pair_chat_consent: nil when the context is
// missing or not a pair; otherwise whether both people of the pair have said
// Nếp may read them, at now. Never cached, as in Python.
//
// Statements follow Python: get_context; for a pair, get_pair_notebook; and
// only when a notebook exists, list_members -- read even when the cycle's own
// participant list is the one used, because Python builds the member tuple
// before `_participants` chooses.
//
// now is the service clock (`_now()`, datetime.now(UTC) at microseconds).
// Truncating it is unnecessary here: every stored deadline is whole
// microseconds, so `now >= deadline` answers the same for now and its
// microsecond floor.
func PairChatConsent(ctx context.Context, r repo.Repository, contextID string, now time.Time) (*bool, error) {
	record, err := r.GetContext(ctx, contextID)
	if err != nil {
		return nil, err
	}
	if record == nil || record.Kind != kindPair {
		return nil, nil
	}
	notebook, err := r.GetPairNotebook(ctx, contextID)
	if err != nil {
		return nil, err
	}
	if notebook == nil {
		answer := false
		return &answer, nil
	}
	rows, err := r.ListMembers(ctx, contextID)
	if err != nil {
		return nil, err
	}
	members := []string{}
	for _, row := range rows {
		if row.State == "active" {
			members = append(members, row.PersonID)
		}
	}
	pair := PairNotebookOf(notebook)
	answer := pairnotebook.ChatConsentActive(pairsteps.ConsentsOf(pair), pairsteps.Participants(pair, members), now)
	return &answer, nil
}

// PairTasteSharers is ApiService._pair_taste_sharers: nil when the context is
// not a pair; otherwise the participants whose `chia_gu` is on now (ADR-0034
// §2.1). Asked at every read, never cached.
func PairTasteSharers(ctx context.Context, r repo.Repository, contextID string, now time.Time) (map[string]bool, error) {
	record, err := r.GetContext(ctx, contextID)
	if err != nil {
		return nil, err
	}
	if record == nil || record.Kind != kindPair {
		return nil, nil
	}
	sharers := map[string]bool{}
	notebook, err := r.GetPairNotebook(ctx, contextID)
	if err != nil {
		return nil, err
	}
	if notebook == nil {
		return sharers, nil
	}
	rows, err := r.ListMembers(ctx, contextID)
	if err != nil {
		return nil, err
	}
	members := []string{}
	for _, row := range rows {
		if row.State == "active" {
			members = append(members, row.PersonID)
		}
	}
	pair := PairNotebookOf(notebook)
	consents := pairsteps.ConsentsOf(pair)
	for _, person := range pairsteps.Participants(pair, members) {
		if slices.Contains(pairnotebook.GrantedBy(consents, person, &now), "chia_gu") {
			sharers[person] = true
		}
	}
	return sharers, nil
}

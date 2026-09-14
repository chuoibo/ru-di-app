package service

import (
	"context"
	"time"

	"mobile/services/core/internal/domain/pairnotebook"
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
	answer := pairnotebook.ChatConsentActive(consentsAsDicts(notebook), pairParticipants(notebook, members), now)
	return &answer, nil
}

// pairParticipants is ApiService._participants: the cycle's own list while a
// cycle is live, so a later membership change cannot widen what two people
// agreed to; the conversation's active members before any cycle exists.
func pairParticipants(notebook *repo.PairNotebook, members []string) []string {
	if notebook != nil && notebook.CycleID != nil {
		return notebook.Participants
	}
	return members
}

// consentsAsDicts is _consents_as_dicts.
func consentsAsDicts(notebook *repo.PairNotebook) []pairnotebook.Consent {
	out := make([]pairnotebook.Consent, 0, len(notebook.Consents))
	for _, row := range notebook.Consents {
		expires := row.ProposalExpiresAt
		out = append(out, pairnotebook.Consent{
			PersonID:          row.PersonID,
			Purpose:           row.Purpose,
			GrantedAt:         row.GrantedAt,
			RevokedAt:         row.RevokedAt,
			ProposalExpiresAt: &expires,
		})
	}
	return out
}

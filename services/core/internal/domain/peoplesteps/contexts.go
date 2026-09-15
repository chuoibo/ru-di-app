package peoplesteps

import (
	"time"

	"mobile/services/core/internal/domain/blocking"
	"mobile/services/core/internal/domain/direct"
)

// Counterpart is ContextCounterpart: the other person of a pair.
type Counterpart struct {
	ID          string
	DisplayName string
}

// ContextSummary is ContextSummary: one row of GET /people/me/contexts, and
// the body of POST /people/{person_id}/dm.
type ContextSummary struct {
	ID           string
	DisplayName  string
	MemberCount  int64
	MyRole       string
	MyState      string
	MembershipID string
	JoinedAt     *time.Time
	LastMessage  *LastMessage
	UnreadCount  int64
	Theme        string
	Kind         string
	Counterpart  *Counterpart
	Unavailable  bool
}

// SummaryOf is _context_summary: the record as the wire shows it. The
// counterpart's name falls back to direct.AnonymousCounterpart; Unavailable is
// left false, which is what the model's default says.
func SummaryOf(record SummaryRecord) ContextSummary {
	summary := ContextSummary{
		ID:           record.ID,
		DisplayName:  record.DisplayName,
		MemberCount:  record.MemberCount,
		MyRole:       record.MyRole,
		MyState:      record.MyState,
		MembershipID: record.MembershipID,
		JoinedAt:     record.JoinedAt,
		UnreadCount:  record.UnreadCount,
		Theme:        record.Theme,
		Kind:         record.Kind,
	}
	if record.LastMessage != nil {
		last := *record.LastMessage
		summary.LastMessage = &last
	}
	if record.CounterpartID != nil {
		summary.Counterpart = &Counterpart{
			ID:          *record.CounterpartID,
			DisplayName: direct.DisplayNameFor(direct.KindPair, "", record.CounterpartDisplayName),
		}
	}
	return summary
}

// ContextSummaries is _context_summaries: every row, then for each pair with a
// counterpart the counterpart's person row and the pair's edge, which decide
// whether the conversation still takes messages (ADR-0023 §2.3.2).
func ContextSummaries(s Store, personID string) ([]ContextSummary, error) {
	records, err := s.ListPersonContextSummaries(personID)
	if err != nil {
		return nil, err
	}
	summaries := make([]ContextSummary, len(records))
	for i, record := range records {
		summaries[i] = SummaryOf(record)
	}
	for i := range summaries {
		summary := &summaries[i]
		if summary.Kind != direct.KindPair || summary.Counterpart == nil {
			continue
		}
		other, err := s.GetPerson(summary.Counterpart.ID)
		if err != nil {
			return nil, err
		}
		edge, err := friendEdge(s, personID, summary.Counterpart.ID)
		if err != nil {
			return nil, err
		}
		summary.Unavailable = !blocking.DMAllowed(edge, deleted(other))
	}
	return summaries, nil
}

// ListMyContexts is list_my_contexts (GET /people/me/contexts).
func ListMyContexts(s Store, actor Actor) ([]ContextSummary, error) {
	if err := requirePermission("view_own_contexts", actor, nil, fact{"is_self", true}); err != nil {
		return nil, err
	}
	return ContextSummaries(s, actor.ID)
}

// OpenDirectMessage is open_direct_message (POST /people/{person_id}/dm): the
// caller's pair with a friend, found or created, and whether it was created.
// Writing to oneself is 422; not friends, no such person, an ended account and
// a block are one 404 with one sentence, and a permission denial joins them.
func OpenDirectMessage(s Store, actor Actor, personID string, now time.Time) (ContextSummary, bool, error) {
	if personID == actor.ID {
		return ContextSummary{}, false, refusal(422, "self_direct_message", "Không thể nhắn riêng với chính mình.")
	}
	unavailable := refusal(404, "person_not_found", "Chưa thể nhắn riêng với người này.")
	isFriend, err := s.AreFriends(actor.ID, personID)
	if err != nil {
		return ContextSummary{}, false, err
	}
	if err := requirePermission("open_direct_message", actor, nil, fact{"is_friend", isFriend}); err != nil {
		if asRefusal(err) {
			return ContextSummary{}, false, unavailable
		}
		return ContextSummary{}, false, err
	}
	other, err := s.GetPerson(personID)
	if err != nil {
		return ContextSummary{}, false, err
	}
	if !direct.CanOpen(isFriend, !deleted(other), false) {
		return ContextSummary{}, false, unavailable
	}
	blocked, err := isBlockedWith(s, actor.ID, personID)
	if err != nil {
		return ContextSummary{}, false, err
	}
	if blocked {
		return ContextSummary{}, false, unavailable
	}
	key, err := direct.PairKey(actor.ID, personID)
	if err != nil {
		return ContextSummary{}, false, err
	}
	existing, err := s.GetPairContext(key)
	if err != nil {
		return ContextSummary{}, false, err
	}
	created := false
	if existing == nil {
		record, err := s.CreatePairContext(key, [2]string{actor.ID, personID}, actor.ID, now)
		if err == nil {
			existing, created = &record, true
		} else {
			if code, ok := conflictCode(err); !ok || code != "PAIR_EXISTS" {
				return ContextSummary{}, false, err
			}
			if existing, err = s.GetPairContext(key); err != nil {
				return ContextSummary{}, false, err
			}
			if existing == nil {
				return ContextSummary{}, false, refusal(409, "pair_exists", "Cuộc trò chuyện vừa được mở ở nơi khác; thử lại.")
			}
		}
	}
	records, err := s.ListPersonContextSummaries(actor.ID)
	if err != nil {
		return ContextSummary{}, false, err
	}
	for _, record := range records {
		if record.ID == existing.ID {
			return SummaryOf(record), created, nil
		}
	}
	return ContextSummary{}, false, refusal(404, "context_not_found", "Context does not exist")
}

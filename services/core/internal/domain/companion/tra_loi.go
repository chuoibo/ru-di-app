package companion

import (
	"mobile/services/core/internal/domain/tree"
)

// The group AI's answer inside the thread: an `ai_card` of kind `tra_loi`,
// published as a reply to the `@Rủ Đi` message that asked (ADR-0039, proposed).
//
// Go-only, on purpose. GroundCard is a port with a Python oracle, and parity
// replays it byte for byte; a reply shape Python never produces would either
// change that oracle or drift from it. So GroundReply is a second function that
// CALLS GroundCard for every part -- the same per-kind grounding, the same
// catalogue check, the same 600-rune text bound -- and adds only the envelope
// around the parts. POST /messages still grounds with GroundCard alone, which
// refuses `tra_loi` as an unknown kind: a person cannot post a card that claims
// to be the AI's answer.

const (
	// MaxReplyParts bounds the parts of one answer, one of each kind at most.
	MaxReplyParts = 3
	// MaxReplyRead is the bundle ceiling (chatassist maxLuot): the most shared
	// turns an answer can say it read.
	MaxReplyRead = 40
)

// The commands a reply can answer. `hoi` is named so the envelope does not
// change when the engine gains it; the handler still refuses it for now.
var replyCommands = map[string]bool{"plan": true, "chia_bill": true, "hoi": true}

// The part kinds a reply may carry today. `expense_draft` (a pointer to the
// drafts on the invocation row, never amounts) arrives with the claim flow.
var replyPartKinds = map[string]bool{"text": true, "places": true, "itinerary": true}

// ReplyMeta is what the server, not the model, writes on a reply.
type ReplyMeta struct {
	InvocationID string
	// Command is the invocation's command: plan, chia_bill or hoi.
	Command string
	// Read is how many shared turns the server confirmed belong to the room
	// (the invocation's so_tin_doc), never the caller's own count.
	Read int
}

// GroundReply grounds each raw part with GroundCard against the same catalogue
// and wraps the survivors in the reply envelope. A part that fails grounding,
// has a kind a reply does not carry, repeats a kind already kept, or comes
// after MaxReplyParts is dropped rather than failing the answer; an answer
// with nothing left is refused, which the worker records as invalid_ai_result.
func GroundReply(meta ReplyMeta, parts []tree.Value, places []*tree.OrderedMap) (*tree.OrderedMap, error) {
	if meta.InvocationID == "" || !replyCommands[meta.Command] || meta.Read < 0 || meta.Read > MaxReplyRead {
		return nil, refuse("companion_reply_malformed")
	}
	kept := tree.List{}
	seen := map[string]bool{}
	for _, raw := range parts {
		if len(kept) == MaxReplyParts {
			break
		}
		part, err := GroundCard(raw, places)
		if err != nil {
			continue
		}
		kindValue, _ := part.Get("kind")
		kind, _ := kindValue.(tree.String)
		if !replyPartKinds[string(kind)] || seen[string(kind)] {
			continue
		}
		seen[string(kind)] = true
		kept = append(kept, part)
	}
	if len(kept) == 0 {
		return nil, refuse("companion_reply_empty")
	}
	doc := tree.NewOrderedMap()
	doc.Set("so_tin", tree.NewInt(int64(meta.Read)))
	doc.Set("chi_loi_nho", tree.Bool(meta.Read == 0))
	payload := tree.NewOrderedMap()
	payload.Set("ban", tree.NewInt(1))
	payload.Set("tac_gia", tree.String("rudi-ai"))
	payload.Set("invocation_id", tree.String(meta.InvocationID))
	payload.Set("lenh", tree.String(meta.Command))
	payload.Set("doc", doc)
	payload.Set("phan", kept)
	out := tree.NewOrderedMap()
	out.Set("kind", tree.String("tra_loi"))
	out.Set("payload", payload)
	return out, nil
}

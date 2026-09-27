package companion

import (
	"strings"
	"unicode/utf8"

	"mobile/services/core/internal/domain/tree"
)

// The group AI's answer inside the thread: an `ai_card` of kind `tra_loi`,
// published as a reply to the `@Rủ Đi` message that asked (ADR-0039, proposed;
// contract docs/architecture/03-ai-engine-hop-dong.md §3).
//
// Go-only, on purpose. GroundCard is a port with a Python oracle, and parity
// replays it byte for byte; a reply shape Python never produces would either
// change that oracle or drift from it. So GroundReply is a second function
// that grounds each part itself, with the SAME per-kind grounding GroundCard
// uses for places and itineraries (groundPlaces, groundItinerary: the same
// catalogue check, the same bounds), and two things of its own:
//
//   - a text part is held to the group answer's ceiling (MaxReplyText),
//     cut at the last sentence end inside it, not to the oracle's 600;
//   - an expense_draft part is a pointer to the drafts kept on the
//     invocation row: how many there are and which have a real expense
//     line (da_ghi), never an amount, a payer or a person.
//
// POST /messages still grounds with GroundCard alone, which refuses `tra_loi`
// as an unknown kind: a person cannot post a card that claims to be the AI's
// answer. GroundCard is unchanged.

const (
	// MaxReplyParts bounds the parts of one answer, one of each kind at most.
	MaxReplyParts = 3
	// MaxReplyRead is the bundle ceiling (chatassist maxLuot): the most shared
	// turns an answer can say it read.
	MaxReplyRead = 40
	// MaxReplyText is the group answer's text ceiling in runes (design 03
	// §3.2: proposed 1500, the Lead's to confirm). GroundCard keeps the
	// oracle's MaxText.
	MaxReplyText = 1500
	// MaxReplyKhoan bounds the drafts one chia_bill answer points to
	// (chatassist maxLuotDocChi).
	MaxReplyKhoan = 8
)

// The commands a reply can answer.
var replyCommands = map[string]bool{"plan": true, "chia_bill": true, "hoi": true}

// ReplyMeta is what the server, not the model, writes on a reply.
type ReplyMeta struct {
	InvocationID string
	// Command is the invocation's command: plan, chia_bill or hoi.
	Command string
	// Read is how many shared turns the server confirmed belong to the room
	// (the invocation's so_tin_doc), never the caller's own count.
	Read int
}

// GroundReply grounds each raw part against the catalogue and wraps the
// survivors in the reply envelope. A part that fails grounding, has a kind a
// reply does not carry, repeats a kind already kept, or comes after
// MaxReplyParts is dropped rather than failing the answer; an answer with
// nothing left is refused, which the worker records as invalid_ai_result.
func GroundReply(meta ReplyMeta, parts []tree.Value, places []*tree.OrderedMap) (*tree.OrderedMap, error) {
	if meta.InvocationID == "" || !replyCommands[meta.Command] || meta.Read < 0 || meta.Read > MaxReplyRead {
		return nil, refuse("companion_reply_malformed")
	}
	catalogue := catalogueByID(places)
	kept := tree.List{}
	seen := map[string]bool{}
	for _, raw := range parts {
		if len(kept) == MaxReplyParts {
			break
		}
		part, kind, err := groundReplyPart(raw, catalogue)
		if err != nil || seen[kind] {
			continue
		}
		seen[kind] = true
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

// groundReplyPart grounds one raw part and names its kind.
func groundReplyPart(raw tree.Value, catalogue map[string]*tree.OrderedMap) (*tree.OrderedMap, string, error) {
	obj, ok := raw.(*tree.OrderedMap)
	if !ok {
		return nil, "", malformed()
	}
	kindValue, _ := obj.Get("kind")
	kind, ok := kindValue.(tree.String)
	if !ok {
		return nil, "", malformed()
	}
	payloadValue, _ := obj.Get("payload")
	payload, ok := payloadValue.(*tree.OrderedMap)
	if !ok {
		return nil, "", malformed()
	}
	var part *tree.OrderedMap
	var err error
	switch string(kind) {
	case "text":
		part, err = groundReplyText(payload)
	case "places":
		part, err = groundPlaces(payload, catalogue)
	case "itinerary":
		part, err = groundItinerary(payload, catalogue)
	case "expense_draft":
		part, err = groundExpenseDraft(payload)
	default:
		err = refuse("companion_card_kind_unknown")
	}
	return part, string(kind), err
}

// groundReplyText holds a text part to MaxReplyText: a longer text is cut at
// the last sentence end (. ! ? … or a line break) inside the ceiling, or at
// the ceiling when there is none. A blank text is refused.
func groundReplyText(payload *tree.OrderedMap) (*tree.OrderedMap, error) {
	value, ok := payload.Get("text")
	if !ok {
		return nil, malformed()
	}
	text, ok := value.(tree.String)
	if !ok || !utf8.ValidString(string(text)) {
		return nil, malformed()
	}
	s := catCauReply(string(text), MaxReplyText)
	if strings.TrimSpace(s) == "" {
		return nil, refuse("companion_card_empty")
	}
	body := tree.NewOrderedMap()
	body.Set("text", tree.String(s))
	out := tree.NewOrderedMap()
	out.Set("kind", tree.String("text"))
	out.Set("payload", body)
	return out, nil
}

// catCauReply cuts s to at most n runes at the last sentence end inside
// them. Punctuation only: no word is read.
func catCauReply(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	r = r[:n]
	for i := len(r) - 1; i > 0; i-- {
		switch r[i] {
		case '.', '!', '?', '…', '\n':
			return strings.TrimSpace(string(r[:i+1]))
		}
	}
	return string(r)
}

// groundExpenseDraft checks the pointer part of a chia_bill answer: exactly
// so_khoan (1..MaxReplyKhoan) and da_ghi (distinct indices below so_khoan),
// nothing else. An amount, a payer or any other key refuses the part: the
// drafts stay on the invocation row, and the room's card never carries money
// the model wrote.
func groundExpenseDraft(payload *tree.OrderedMap) (*tree.OrderedMap, error) {
	for _, k := range payload.Keys() {
		if k != "so_khoan" && k != "da_ghi" {
			return nil, malformed()
		}
	}
	nValue, _ := payload.Get("so_khoan")
	n, ok := nValue.(tree.Int)
	if !ok || n < 1 || n > MaxReplyKhoan {
		return nil, malformed()
	}
	gValue, _ := payload.Get("da_ghi")
	list, ok := gValue.(tree.List)
	if !ok {
		return nil, malformed()
	}
	daGhi := tree.List{}
	seen := map[tree.Int]bool{}
	for _, item := range list {
		k, ok := item.(tree.Int)
		if !ok || k < 0 || k >= n || seen[k] {
			return nil, malformed()
		}
		seen[k] = true
		daGhi = append(daGhi, k)
	}
	body := tree.NewOrderedMap()
	body.Set("so_khoan", n)
	body.Set("da_ghi", daGhi)
	out := tree.NewOrderedMap()
	out.Set("kind", tree.String("expense_draft"))
	out.Set("payload", body)
	return out, nil
}

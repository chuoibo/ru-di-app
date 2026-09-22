package routes

import (
	"encoding/json"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/repo"
)

// ChatSnapshotMessage shares the established wire serializer with the opt-in
// invalidation feed. Reaction ownership is always computed for this reader.
func ChatSnapshotMessage(record repo.Message, reactions []repo.Reaction, quoted map[string]repo.Message, actor string, revision int64) (json.RawMessage, error) {
	var preview *pyjson.OrderedMap
	if record.ReplyToID != nil {
		if parent, ok := quoted[*record.ReplyToID]; ok {
			preview = wireReplyPreview(parent)
		}
	}
	wire := wireMessage(record, summariseReactions(reactions, actor)[record.ID], preview)
	wire.Set("revision", pyjson.NewInt(revision))
	return pyjson.Compact(wire)
}

// ChatSnapshotVote shares the existing result and per-reader ballot projection.
func ChatSnapshotVote(record repo.Vote, actor string, revision int64) (json.RawMessage, error) {
	wire, err := wireVote(record, actor)
	if err != nil {
		return nil, err
	}
	wire.Set("revision", pyjson.NewInt(revision))
	return pyjson.Compact(wire)
}

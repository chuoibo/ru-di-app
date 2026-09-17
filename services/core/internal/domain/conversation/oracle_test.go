package conversation

import (
	"errors"
	"testing"

	"mobile/services/core/internal/oracletest"
)

// testdata/python_conversation*.json is rendered by
// scripts/render_domain_wai_goldens.py from the real app.domain.conversation.

func TestConversationMatchesPython(t *testing.T) {
	files := oracletest.Load(t, "testdata/python_*.json")
	oracletest.Agree(t, files, "conversation", replay, noRefusal)
}

func TestConversationConstantsMatchPython(t *testing.T) {
	c := oracletest.Constants(t, oracletest.Load(t, "testdata/python_*.json"), "conversation")
	if got, _ := oracletest.Int64(c["MAX_LINES"]); got != MaxLines {
		t.Errorf("MAX_LINES: Python %v, Go %d", c["MAX_LINES"], MaxLines)
	}
	if got, _ := oracletest.Int64(c["MAX_LINE"]); got != MaxLine {
		t.Errorf("MAX_LINE: Python %v, Go %d", c["MAX_LINE"], MaxLine)
	}
	if got, _ := oracletest.Int64(c["MIN_LINES"]); got != MinLines {
		t.Errorf("MIN_LINES: Python %v, Go %d", c["MIN_LINES"], MinLines)
	}
}

func replay(c oracletest.Case, args map[string]any) (any, error) {
	switch c.Fn {
	case "summarise_conversation":
		messages, err := messagesOf(args["messages"])
		if err != nil {
			return nil, err
		}
		count, err := oracletest.Int64(args["member_count"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		return digestOK(Summarise(messages, int(count))), nil
	case "has_conversation":
		digest, err := digestOf(args["digest"])
		if err != nil {
			return nil, err
		}
		return Has(digest), nil
	default:
		return nil, oracletest.Decode(errors.New(c.Fn))
	}
}

func messagesOf(raw any) ([]Message, error) {
	items, err := oracletest.List(raw)
	if err != nil {
		return nil, oracletest.Decode(err)
	}
	out := make([]Message, 0, len(items))
	for _, item := range items {
		row, ok := item.(map[string]any)
		if !ok {
			return nil, oracletest.Decode(errors.New("message is not a dict"))
		}
		m := Message{}
		if kind, ok := row["kind"].(string); ok {
			m.Kind = kind
		}
		body, err := oracletest.OptionalString(row["body"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		m.Body = body
		author, err := oracletest.OptionalString(row["author_id"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		m.AuthorID = author
		out = append(out, m)
	}
	return out, nil
}

func digestOf(raw any) (Digest, error) {
	row, err := oracletest.Row(raw, "recent_lines", "message_count", "speaker_count", "member_count")
	if err != nil {
		return Digest{}, oracletest.Decode(err)
	}
	linesAny, err := oracletest.List(row["recent_lines"])
	if err != nil {
		return Digest{}, oracletest.Decode(err)
	}
	lines := make([]string, 0, len(linesAny))
	for _, item := range linesAny {
		s, err := oracletest.Str(item)
		if err != nil {
			return Digest{}, oracletest.Decode(err)
		}
		lines = append(lines, s)
	}
	messages, err := oracletest.Int64(row["message_count"])
	if err != nil {
		return Digest{}, oracletest.Decode(err)
	}
	speakers, err := oracletest.Int64(row["speaker_count"])
	if err != nil {
		return Digest{}, oracletest.Decode(err)
	}
	members, err := oracletest.Int64(row["member_count"])
	if err != nil {
		return Digest{}, oracletest.Decode(err)
	}
	return Digest{
		RecentLines: lines, MessageCount: int(messages),
		SpeakerCount: int(speakers), MemberCount: int(members),
	}, nil
}

func digestOK(d Digest) map[string]any {
	return map[string]any{
		"recent_lines": oracletest.AnyStrings(d.RecentLines),
		"message_count": int64(d.MessageCount),
		"speaker_count": int64(d.SpeakerCount),
		"member_count": int64(d.MemberCount),
	}
}

func noRefusal(error) (string, string, bool) { return "", "", false }

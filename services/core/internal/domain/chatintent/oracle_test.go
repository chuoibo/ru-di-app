package chatintent

import (
	"errors"
	"testing"

	"mobile/services/core/internal/oracletest"
)

// testdata/python_chat_intent*.json is rendered by
// scripts/render_domain_wai_goldens.py from the real app.domain.chat_intent.

func TestChatIntentMatchesPython(t *testing.T) {
	files := oracletest.Load(t, "testdata/python_*.json")
	oracletest.Agree(t, files, "chat_intent", replay, noRefusal)
}

func replay(c oracletest.Case, args map[string]any) (any, error) {
	switch c.Fn {
	case "parse_intent":
		body, err := oracletest.Str(args["body"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		got := Parse(body)
		if got == nil {
			return nil, nil
		}
		return map[string]any{"intent": got.Intent, "args": got.Args}, nil
	case "parse_vote":
		text, err := oracletest.Str(args["args"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		got := ParseVote(text)
		if got == nil {
			return nil, nil
		}
		return map[string]any{"question": got.Question, "options": oracletest.AnyStrings(got.Options)}, nil
	default:
		return nil, oracletest.Decode(errors.New(c.Fn))
	}
}

func noRefusal(error) (string, string, bool) { return "", "", false }

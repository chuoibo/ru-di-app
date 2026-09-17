package messageedit

import (
	"errors"
	"testing"

	"mobile/services/core/internal/oracletest"
)

// testdata/python_message_edit*.json is rendered by
// scripts/render_domain_wai_goldens.py from the real app.domain.message_edit.

func refusal(err error) (class, code string, ok bool) {
	var e *Error
	if errors.As(err, &e) {
		return "MessageEditError", e.Code, true
	}
	return "", "", false
}

func TestMessageEditMatchesPython(t *testing.T) {
	files := oracletest.Load(t, "testdata/python_*.json")
	oracletest.Agree(t, files, "message_edit", replay, refusal)
}

func replay(c oracletest.Case, args map[string]any) (any, error) {
	switch c.Fn {
	case "check_deletable":
		message, err := factsOf(args["message"])
		if err != nil {
			return nil, err
		}
		actor, err := oracletest.Str(args["actor_id"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		return nil, CheckDeletable(message, actor)
	case "check_reply_target":
		if args["target"] == nil {
			contextID, err := oracletest.Str(args["context_id"])
			if err != nil {
				return nil, oracletest.Decode(err)
			}
			return nil, CheckReplyTarget(nil, contextID)
		}
		target, err := factsOf(args["target"])
		if err != nil {
			return nil, err
		}
		contextID, err := oracletest.Str(args["context_id"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		return nil, CheckReplyTarget(&target, contextID)
	case "deleted_shape":
		if _, ok := args["now"].(oracletest.PyNaive); ok {
			return nil, &Error{"NAIVE_DATETIME"}
		}
		stamp, ok := args["now"].(oracletest.PyInstant)
		if !ok {
			return nil, oracletest.Decode(errors.New("now is not a datetime"))
		}
		now, err := oracletest.TimeOfStamp(stamp)
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		kind, at, err := DeletedShape(now)
		if err != nil {
			return nil, err
		}
		return map[string]any{"kind": kind, "deleted_at": oracletest.StampOf(at)}, nil
	default:
		return nil, oracletest.Decode(errors.New(c.Fn))
	}
}

func factsOf(raw any) (Facts, error) {
	row, ok := raw.(map[string]any)
	if !ok {
		return Facts{}, oracletest.Decode(errors.New("message is not a dict"))
	}
	id, _ := row["id"].(string)
	contextID, _ := row["context_id"].(string)
	kind, _ := row["kind"].(string)
	author, err := oracletest.OptionalString(row["author_id"])
	if err != nil {
		return Facts{}, oracletest.Decode(err)
	}
	return Facts{ID: id, ContextID: contextID, AuthorID: author, Kind: kind}, nil
}

func TestMessageEditConstantsMatchPython(t *testing.T) {
	c := oracletest.Constants(t, oracletest.Load(t, "testdata/python_*.json"), "message_edit")
	deletable, err := oracletest.Strings(c["DELETABLE_KINDS"])
	if err != nil {
		t.Fatal(err)
	}
	replyable, err := oracletest.Strings(c["REPLYABLE_KINDS"])
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range deletable {
		if !DeletableKinds[kind] {
			t.Errorf("DELETABLE_KINDS missing %q", kind)
		}
	}
	for _, kind := range replyable {
		if !ReplyableKinds[kind] {
			t.Errorf("REPLYABLE_KINDS missing %q", kind)
		}
	}
}

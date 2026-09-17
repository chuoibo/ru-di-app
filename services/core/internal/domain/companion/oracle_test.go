package companion

import (
	"errors"
	"testing"
	"time"

	"mobile/services/core/internal/domain/tree"
	"mobile/services/core/internal/oracletest"
	"mobile/services/core/internal/treejson"
)

// testdata/python_companion*.json is rendered by
// scripts/render_domain_wai_goldens.py from the real app.domain.companion.

func refusal(err error) (class, code string, ok bool) {
	var e *Error
	if errors.As(err, &e) {
		return "CompanionError", e.Code, true
	}
	return "", "", false
}

func TestCompanionMatchesPython(t *testing.T) {
	files := oracletest.Load(t, "testdata/python_*.json")
	oracletest.Agree(t, files, "companion", replay, refusal)
}

func replay(c oracletest.Case, args map[string]any) (any, error) {
	switch c.Fn {
	case "plan_turn":
		return replayPlan(args)
	case "ground_card":
		raw, err := oracletest.AsPyJSON(args["raw"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		places, err := placesOf(args["allowed_places"])
		if err != nil {
			return nil, err
		}
		got, err := GroundCard(treejson.To(raw), places)
		if err != nil {
			return nil, err
		}
		return oracletest.FromPyJSON(treejson.From(got)), nil
	default:
		return nil, oracletest.Decode(errors.New(c.Fn))
	}
}

func replayPlan(args map[string]any) (any, error) {
	conv, err := oracletest.Row(args["conversation"], "messages", "now")
	if err != nil {
		return nil, oracletest.Decode(err)
	}
	now, err := aware(conv["now"])
	if err != nil {
		return nil, err
	}
	items, err := oracletest.List(conv["messages"])
	if err != nil {
		return nil, oracletest.Decode(err)
	}
	kinds := make([]string, 0, len(items))
	created := make([]time.Time, 0, len(items))
	for _, item := range items {
		row, ok := item.(map[string]any)
		if !ok {
			return nil, oracletest.Decode(errors.New("message is not a dict"))
		}
		at, err := aware(row["created_at"])
		if err != nil {
			return nil, err
		}
		kind, _ := row["author_kind"].(string)
		kinds = append(kinds, kind)
		created = append(created, at)
	}
	requested, _ := args["requested"].(bool)
	got := PlanTurn(kinds, created, now, requested)
	return map[string]any{"may_speak": got.MaySpeak, "reason": got.Reason}, nil
}

func aware(raw any) (time.Time, error) {
	if _, ok := raw.(oracletest.PyNaive); ok {
		return time.Time{}, &Error{"companion_timestamp_naive"}
	}
	stamp, ok := raw.(oracletest.PyInstant)
	if !ok {
		return time.Time{}, oracletest.Decode(errors.New("companion timestamp must be an ISO-8601 string"))
	}
	t, err := oracletest.TimeOfStamp(stamp)
	if err != nil {
		return time.Time{}, oracletest.Decode(err)
	}
	return t, nil
}

func placesOf(raw any) ([]*tree.OrderedMap, error) {
	if raw == nil {
		return nil, nil
	}
	items, err := oracletest.List(raw)
	if err != nil {
		return nil, oracletest.Decode(err)
	}
	out := make([]*tree.OrderedMap, 0, len(items))
	for _, item := range items {
		row, ok := item.(map[string]any)
		if !ok {
			continue
		}
		m, err := oracletest.OrderedMap(row)
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		out = append(out, treejson.MapTo(m))
	}
	return out, nil
}

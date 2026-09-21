package promptsafety

import (
	"errors"
	"testing"

	"mobile/services/core/internal/domain/tree"
	"mobile/services/core/internal/oracletest"
	"mobile/services/core/internal/treejson"
)

// testdata/python_prompt_safety*.json is rendered by
// scripts/render_domain_wai_goldens.py from the real app.places.prompt_safety.

func TestPromptSafetyMatchesPython(t *testing.T) {
	files := oracletest.Load(t, "testdata/python_*.json")
	oracletest.Agree(t, files, "prompt_safety", replay, noRefusal)
}

func replay(c oracletest.Case, args map[string]any) (any, error) {
	switch c.Fn {
	case "place_is_safe_for_prompt":
		place, err := placeOf(args["place"])
		if err != nil {
			return nil, err
		}
		return Safe(place), nil
	case "safe_places":
		items, err := oracletest.List(args["places"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		places := make([]*tree.OrderedMap, 0, len(items))
		for _, item := range items {
			place, err := placeOf(item)
			if err != nil {
				return nil, err
			}
			places = append(places, place)
		}
		got := Filter(places)
		out := make([]any, 0, len(got))
		for _, place := range got {
			out = append(out, oracletest.FromPyJSON(treejson.From(place)))
		}
		return out, nil
	default:
		return nil, oracletest.Decode(errors.New(c.Fn))
	}
}

func placeOf(raw any) (*tree.OrderedMap, error) {
	if raw == nil {
		return nil, nil
	}
	m, err := oracletest.OrderedMap(raw)
	if err != nil {
		return nil, oracletest.Decode(err)
	}
	return treejson.MapTo(m), nil
}

func noRefusal(error) (string, string, bool) { return "", "", false }

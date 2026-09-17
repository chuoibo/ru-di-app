package suggestion

import (
	"errors"
	"testing"

	"mobile/services/core/internal/domain/tree"
	"mobile/services/core/internal/oracletest"
	"mobile/services/core/internal/treejson"
)

// testdata/python_suggestion*.json is rendered by
// scripts/render_domain_wai_goldens.py from the real app.domain.suggestion.

func refusal(err error) (class, code string, ok bool) {
	var e *Error
	if errors.As(err, &e) {
		return "SuggestionError", e.Code, true
	}
	return "", "", false
}

func TestSuggestionMatchesPython(t *testing.T) {
	files := oracletest.Load(t, "testdata/python_*.json")
	oracletest.Agree(t, files, "suggestion", replay, refusal)
}

func replay(c oracletest.Case, args map[string]any) (any, error) {
	switch c.Fn {
	case "summarise_history":
		trips, err := tripsOf(args["trips"])
		if err != nil {
			return nil, err
		}
		cats, err := categoriesOf(args["visits"])
		if err != nil {
			return nil, err
		}
		got, err := SummariseHistory(trips, cats)
		if err != nil {
			return nil, err
		}
		return historyOK(got), nil
	case "ground_suggestion":
		raw, err := oracletest.AsPyJSON(args["raw"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		places, err := placesOf(args["allowed_places"])
		if err != nil {
			return nil, err
		}
		got, err := Ground(treejson.To(raw), places)
		if err != nil {
			return nil, err
		}
		return oracletest.FromPyJSON(treejson.From(got)), nil
	default:
		return nil, oracletest.Decode(errors.New(c.Fn))
	}
}

func tripsOf(raw any) ([]Trip, error) {
	items, err := oracletest.List(raw)
	if err != nil {
		return nil, oracletest.Decode(err)
	}
	out := make([]Trip, 0, len(items))
	for _, item := range items {
		row, ok := item.(map[string]any)
		if !ok {
			return nil, &Error{"suggestion_card_malformed"}
		}
		title, _ := row["title"].(string)
		split, err := oracletest.Int64(row["split_total_vnd"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		head, err := oracletest.Int64(row["headcount"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		out = append(out, Trip{Title: title, SplitTotalVND: split, Headcount: head})
	}
	return out, nil
}

func categoriesOf(raw any) ([]string, error) {
	items, err := oracletest.List(raw)
	if err != nil {
		return nil, oracletest.Decode(err)
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		row, ok := item.(map[string]any)
		if !ok {
			continue
		}
		cat, ok := row["category"].(string)
		if !ok {
			continue
		}
		out = append(out, cat)
	}
	return out, nil
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

func historyOK(h History) map[string]any {
	var avg any
	if h.AvgPerPersonVND != nil {
		avg = *h.AvgPerPersonVND
	}
	return map[string]any{
		"outing_count":       int64(h.OutingCount),
		"split_total_vnd":    h.SplitTotalVND,
		"avg_per_person_vnd": avg,
		"top_categories":     oracletest.AnyStrings(h.TopCategories),
		"recent_titles":      oracletest.AnyStrings(h.RecentTitles),
	}
}

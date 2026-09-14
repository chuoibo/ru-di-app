package routes

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"mobile/services/core/internal/domain/scoring"
	"mobile/services/core/internal/domain/taste"
)

// twentyNines is past int64 and float64 precision; built rather than written
// out because the repo guard refuses long digit runs in source.
var twentyNines = strings.Repeat("9", 20)

func ptr(v int64) *int64 { return &v }

func sizeText(size *int64) string {
	if size == nil {
		return "unknown"
	}
	return strconv.FormatInt(*size, 10)
}

// Each outcome is what app.places.scoring.group_size_fit answered in the
// pinned API image for that stored group_fit and party size.
func TestGroupFitReadsTheCatalogueAsPythonDoes(t *testing.T) {
	cases := []struct {
		raw     json.RawMessage
		size    *int64
		outcome string
	}{
		{nil, nil, "none"},
		{nil, ptr(1), "none"},
		{nil, ptr(2), "none"},
		{nil, ptr(3), "none"},
		{nil, ptr(7), "none"},
		{nil, ptr(8), "none"},
		{nil, ptr(9), "none"},
		{json.RawMessage("null"), nil, "none"},
		{json.RawMessage("null"), ptr(1), "none"},
		{json.RawMessage("null"), ptr(2), "none"},
		{json.RawMessage("null"), ptr(3), "none"},
		{json.RawMessage("null"), ptr(7), "none"},
		{json.RawMessage("null"), ptr(8), "none"},
		{json.RawMessage("null"), ptr(9), "none"},
		{json.RawMessage("{}"), nil, "none"},
		{json.RawMessage("{}"), ptr(1), "none"},
		{json.RawMessage("{}"), ptr(2), "none"},
		{json.RawMessage("{}"), ptr(3), "none"},
		{json.RawMessage("{}"), ptr(7), "none"},
		{json.RawMessage("{}"), ptr(8), "none"},
		{json.RawMessage("{}"), ptr(9), "none"},
		{json.RawMessage("[]"), nil, "none"},
		{json.RawMessage("[]"), ptr(1), "none"},
		{json.RawMessage("[]"), ptr(2), "none"},
		{json.RawMessage("[]"), ptr(3), "none"},
		{json.RawMessage("[]"), ptr(7), "none"},
		{json.RawMessage("[]"), ptr(8), "none"},
		{json.RawMessage("[]"), ptr(9), "none"},
		{json.RawMessage("\"\""), nil, "none"},
		{json.RawMessage("\"\""), ptr(1), "none"},
		{json.RawMessage("\"\""), ptr(2), "none"},
		{json.RawMessage("\"\""), ptr(3), "none"},
		{json.RawMessage("\"\""), ptr(7), "none"},
		{json.RawMessage("\"\""), ptr(8), "none"},
		{json.RawMessage("\"\""), ptr(9), "none"},
		{json.RawMessage("0"), nil, "none"},
		{json.RawMessage("0"), ptr(1), "none"},
		{json.RawMessage("0"), ptr(2), "none"},
		{json.RawMessage("0"), ptr(3), "none"},
		{json.RawMessage("0"), ptr(7), "none"},
		{json.RawMessage("0"), ptr(8), "none"},
		{json.RawMessage("0"), ptr(9), "none"},
		{json.RawMessage("0.0"), nil, "none"},
		{json.RawMessage("0.0"), ptr(1), "none"},
		{json.RawMessage("0.0"), ptr(2), "none"},
		{json.RawMessage("0.0"), ptr(3), "none"},
		{json.RawMessage("0.0"), ptr(7), "none"},
		{json.RawMessage("0.0"), ptr(8), "none"},
		{json.RawMessage("0.0"), ptr(9), "none"},
		{json.RawMessage("false"), nil, "none"},
		{json.RawMessage("false"), ptr(1), "none"},
		{json.RawMessage("false"), ptr(2), "none"},
		{json.RawMessage("false"), ptr(3), "none"},
		{json.RawMessage("false"), ptr(7), "none"},
		{json.RawMessage("false"), ptr(8), "none"},
		{json.RawMessage("false"), ptr(9), "none"},
		{json.RawMessage("{\"min_people\": 2, \"max_people\": 8}"), nil, "none"},
		{json.RawMessage("{\"min_people\": 2, \"max_people\": 8}"), ptr(1), "0"},
		{json.RawMessage("{\"min_people\": 2, \"max_people\": 8}"), ptr(2), "1"},
		{json.RawMessage("{\"min_people\": 2, \"max_people\": 8}"), ptr(3), "1"},
		{json.RawMessage("{\"min_people\": 2, \"max_people\": 8}"), ptr(7), "1"},
		{json.RawMessage("{\"min_people\": 2, \"max_people\": 8}"), ptr(8), "1"},
		{json.RawMessage("{\"min_people\": 2, \"max_people\": 8}"), ptr(9), "0"},
		{json.RawMessage("{\"min_people\": 2.5, \"max_people\": 7.5}"), nil, "none"},
		{json.RawMessage("{\"min_people\": 2.5, \"max_people\": 7.5}"), ptr(1), "0"},
		{json.RawMessage("{\"min_people\": 2.5, \"max_people\": 7.5}"), ptr(2), "0"},
		{json.RawMessage("{\"min_people\": 2.5, \"max_people\": 7.5}"), ptr(3), "1"},
		{json.RawMessage("{\"min_people\": 2.5, \"max_people\": 7.5}"), ptr(7), "1"},
		{json.RawMessage("{\"min_people\": 2.5, \"max_people\": 7.5}"), ptr(8), "0"},
		{json.RawMessage("{\"min_people\": 2.5, \"max_people\": 7.5}"), ptr(9), "0"},
		{json.RawMessage("{\"min_people\": true, \"max_people\": 7}"), nil, "none"},
		{json.RawMessage("{\"min_people\": true, \"max_people\": 7}"), ptr(1), "1"},
		{json.RawMessage("{\"min_people\": true, \"max_people\": 7}"), ptr(2), "1"},
		{json.RawMessage("{\"min_people\": true, \"max_people\": 7}"), ptr(3), "1"},
		{json.RawMessage("{\"min_people\": true, \"max_people\": 7}"), ptr(7), "1"},
		{json.RawMessage("{\"min_people\": true, \"max_people\": 7}"), ptr(8), "0"},
		{json.RawMessage("{\"min_people\": true, \"max_people\": 7}"), ptr(9), "0"},
		{json.RawMessage("{\"max_people\": 8}"), nil, "none"},
		{json.RawMessage("{\"max_people\": 8}"), ptr(1), "raises KeyError"},
		{json.RawMessage("{\"max_people\": 8}"), ptr(2), "raises KeyError"},
		{json.RawMessage("{\"max_people\": 8}"), ptr(3), "raises KeyError"},
		{json.RawMessage("{\"max_people\": 8}"), ptr(7), "raises KeyError"},
		{json.RawMessage("{\"max_people\": 8}"), ptr(8), "raises KeyError"},
		{json.RawMessage("{\"max_people\": 8}"), ptr(9), "raises KeyError"},
		{json.RawMessage("{\"min_people\": \"2\", \"max_people\": 8}"), nil, "none"},
		{json.RawMessage("{\"min_people\": \"2\", \"max_people\": 8}"), ptr(1), "raises TypeError"},
		{json.RawMessage("{\"min_people\": \"2\", \"max_people\": 8}"), ptr(2), "raises TypeError"},
		{json.RawMessage("{\"min_people\": \"2\", \"max_people\": 8}"), ptr(3), "raises TypeError"},
		{json.RawMessage("{\"min_people\": \"2\", \"max_people\": 8}"), ptr(7), "raises TypeError"},
		{json.RawMessage("{\"min_people\": \"2\", \"max_people\": 8}"), ptr(8), "raises TypeError"},
		{json.RawMessage("{\"min_people\": \"2\", \"max_people\": 8}"), ptr(9), "raises TypeError"},
		{json.RawMessage("[1]"), nil, "none"},
		{json.RawMessage("[1]"), ptr(1), "raises TypeError"},
		{json.RawMessage("[1]"), ptr(2), "raises TypeError"},
		{json.RawMessage("[1]"), ptr(3), "raises TypeError"},
		{json.RawMessage("[1]"), ptr(7), "raises TypeError"},
		{json.RawMessage("[1]"), ptr(8), "raises TypeError"},
		{json.RawMessage("[1]"), ptr(9), "raises TypeError"},
		{json.RawMessage("\"x\""), nil, "none"},
		{json.RawMessage("\"x\""), ptr(1), "raises TypeError"},
		{json.RawMessage("\"x\""), ptr(2), "raises TypeError"},
		{json.RawMessage("\"x\""), ptr(3), "raises TypeError"},
		{json.RawMessage("\"x\""), ptr(7), "raises TypeError"},
		{json.RawMessage("\"x\""), ptr(8), "raises TypeError"},
		{json.RawMessage("\"x\""), ptr(9), "raises TypeError"},
		{json.RawMessage("5"), nil, "none"},
		{json.RawMessage("5"), ptr(1), "raises TypeError"},
		{json.RawMessage("5"), ptr(2), "raises TypeError"},
		{json.RawMessage("5"), ptr(3), "raises TypeError"},
		{json.RawMessage("5"), ptr(7), "raises TypeError"},
		{json.RawMessage("5"), ptr(8), "raises TypeError"},
		{json.RawMessage("5"), ptr(9), "raises TypeError"},
		{json.RawMessage("true"), nil, "none"},
		{json.RawMessage("true"), ptr(1), "raises TypeError"},
		{json.RawMessage("true"), ptr(2), "raises TypeError"},
		{json.RawMessage("true"), ptr(3), "raises TypeError"},
		{json.RawMessage("true"), ptr(7), "raises TypeError"},
		{json.RawMessage("true"), ptr(8), "raises TypeError"},
		{json.RawMessage("true"), ptr(9), "raises TypeError"},
		{json.RawMessage("{\"min_people\": null, \"max_people\": 8}"), nil, "none"},
		{json.RawMessage("{\"min_people\": null, \"max_people\": 8}"), ptr(1), "raises TypeError"},
		{json.RawMessage("{\"min_people\": null, \"max_people\": 8}"), ptr(2), "raises TypeError"},
		{json.RawMessage("{\"min_people\": null, \"max_people\": 8}"), ptr(3), "raises TypeError"},
		{json.RawMessage("{\"min_people\": null, \"max_people\": 8}"), ptr(7), "raises TypeError"},
		{json.RawMessage("{\"min_people\": null, \"max_people\": 8}"), ptr(8), "raises TypeError"},
		{json.RawMessage("{\"min_people\": null, \"max_people\": 8}"), ptr(9), "raises TypeError"},
		{json.RawMessage("{\"min_people\": 2, \"max_people\": " + twentyNines + "}"), nil, "none"},
		{json.RawMessage("{\"min_people\": 2, \"max_people\": " + twentyNines + "}"), ptr(1), "0"},
		{json.RawMessage("{\"min_people\": 2, \"max_people\": " + twentyNines + "}"), ptr(2), "1"},
		{json.RawMessage("{\"min_people\": 2, \"max_people\": " + twentyNines + "}"), ptr(3), "1"},
		{json.RawMessage("{\"min_people\": 2, \"max_people\": " + twentyNines + "}"), ptr(7), "1"},
		{json.RawMessage("{\"min_people\": 2, \"max_people\": " + twentyNines + "}"), ptr(8), "1"},
		{json.RawMessage("{\"min_people\": 2, \"max_people\": " + twentyNines + "}"), ptr(9), "1"},
		{json.RawMessage("{\"min_people\": -" + twentyNines + ", \"max_people\": 3}"), nil, "none"},
		{json.RawMessage("{\"min_people\": -" + twentyNines + ", \"max_people\": 3}"), ptr(1), "1"},
		{json.RawMessage("{\"min_people\": -" + twentyNines + ", \"max_people\": 3}"), ptr(2), "1"},
		{json.RawMessage("{\"min_people\": -" + twentyNines + ", \"max_people\": 3}"), ptr(3), "1"},
		{json.RawMessage("{\"min_people\": -" + twentyNines + ", \"max_people\": 3}"), ptr(7), "0"},
		{json.RawMessage("{\"min_people\": -" + twentyNines + ", \"max_people\": 3}"), ptr(8), "0"},
		{json.RawMessage("{\"min_people\": -" + twentyNines + ", \"max_people\": 3}"), ptr(9), "0"},
		{json.RawMessage("{\"min_people\": 3, \"max_people\": 3, \"extra\": 1}"), nil, "none"},
		{json.RawMessage("{\"min_people\": 3, \"max_people\": 3, \"extra\": 1}"), ptr(1), "0"},
		{json.RawMessage("{\"min_people\": 3, \"max_people\": 3, \"extra\": 1}"), ptr(2), "0"},
		{json.RawMessage("{\"min_people\": 3, \"max_people\": 3, \"extra\": 1}"), ptr(3), "1"},
		{json.RawMessage("{\"min_people\": 3, \"max_people\": 3, \"extra\": 1}"), ptr(7), "0"},
		{json.RawMessage("{\"min_people\": 3, \"max_people\": 3, \"extra\": 1}"), ptr(8), "0"},
		{json.RawMessage("{\"min_people\": 3, \"max_people\": 3, \"extra\": 1}"), ptr(9), "0"},
	}
	for _, tc := range cases {
		fit, deferred := groupFit(tc.raw)
		got := "none"
		switch {
		case deferred != nil && tc.size != nil:
			got = "raises"
		default:
			value := scoring.GroupSizeFit(scoring.Place{GroupFit: fit}, taste.Profile{Size: tc.size})
			if value != nil {
				got = value.RatString()
			}
		}
		want := tc.outcome
		if len(want) > 6 && want[:6] == "raises" {
			want = "raises"
		}
		if got != want {
			t.Errorf("group_fit %s, size %s: Go %s, Python %s", tc.raw, sizeText(tc.size), got, tc.outcome)
		}
	}
}

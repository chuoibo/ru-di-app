package interests

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// testdata/python_*.json is rendered by scripts/render_domain_w1_goldens.py
// from the real app.domain.interests in the pinned API image. Every case is
// replayed through the ...Value function and, when the input has a type the
// service can pass, through the typed function as well.

func goOutcome(value any, err error) outcome {
	if err == nil {
		return outcome{value: value}
	}
	var refused *InterestError
	if errors.As(err, &refused) {
		// Python's InterestError has no `.code` attribute.
		return outcome{errType: "InterestError", message: refused.Error()}
	}
	return outcome{errType: fmt.Sprintf("go:%T", err), message: err.Error()}
}

func renderStrings(values []string) any {
	if values == nil {
		return nilSlice{}
	}
	out := make([]any, len(values))
	for i, value := range values {
		out[i] = value
	}
	return out
}

func renderOptional(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

func renderBand(band BudgetBand) any {
	var top any
	if band.MaxVND != nil {
		top = *band.MaxVND
	}
	return map[string]any{"id": band.ID, "label": band.Label, "min_vnd": band.MinVND, "max_vnd": top}
}

func stringList(value any) ([]string, bool) {
	list, ok := value.([]any)
	if !ok {
		return nil, false
	}
	out := make([]string, len(list))
	for i, item := range list {
		text, isText := item.(string)
		if !isText {
			return nil, false
		}
		out[i] = text
	}
	return out, true
}

func replay(c oracleCase) (string, []call, error) {
	if len(c.Args) != 1 {
		return "", nil, fmt.Errorf("%d args", len(c.Args))
	}
	arg, err := decodeValue(c.Args[0])
	if err != nil {
		return "", nil, err
	}
	var calls []call
	add := func(path string, typed bool, value any, err error) {
		calls = append(calls, call{path: path, typed: typed, got: goOutcome(value, err)})
	}
	switch c.Fn {
	case "normalise_interests":
		value, err := NormaliseInterestsValue(arg)
		add("NormaliseInterestsValue", false, renderStrings(value), err)
		if tags, ok := stringList(arg); ok {
			value, err := NormaliseInterests(tags)
			add("NormaliseInterests", true, renderStrings(value), err)
		}
	case "normalise_budget_band":
		value, err := NormaliseBudgetBandValue(arg)
		add("NormaliseBudgetBandValue", false, renderOptional(value), err)
		switch id := arg.(type) {
		case nil:
			value, err := NormaliseBudgetBand(nil)
			add("NormaliseBudgetBand", true, renderOptional(value), err)
		case string:
			value, err := NormaliseBudgetBand(&id)
			add("NormaliseBudgetBand", true, renderOptional(value), err)
		}
	case "budget_band":
		var id *string
		switch v := arg.(type) {
		case nil:
		case string:
			id = &v
		default:
			return "", nil, fmt.Errorf("budget_band takes str | None, got %T", arg)
		}
		var value any
		if band, ok := BudgetBandOf(id); ok {
			value = renderBand(band)
		}
		add("BudgetBandOf", true, value, nil)
	default:
		return "", nil, fmt.Errorf("unknown fn %q", c.Fn)
	}
	return show(arg), calls, nil
}

func TestOracleCases(t *testing.T) {
	runOracle(t, 5000, replay)
}

func TestOracleFuzzVolume(t *testing.T) {
	checkFuzzVolume(t, "normalise_interests", "normalise_budget_band", "budget_band")
}

func TestOracleConstants(t *testing.T) {
	var constants struct {
		InterestTags  [][2]string `json:"interest_tags"`
		InterestIDs   []string    `json:"interest_ids"`
		MaxInterests  int         `json:"max_interests"`
		BudgetBands   []any       `json:"budget_bands"`
		BudgetBandIDs []string    `json:"budget_band_ids"`
	}
	loadConstants(t, &constants)
	tags := InterestTags()
	if len(tags) != len(constants.InterestTags) {
		t.Fatalf("INTEREST_TAGS has %d tags, Go %d", len(constants.InterestTags), len(tags))
	}
	for i, pair := range constants.InterestTags {
		if tags[i].ID != pair[0] || tags[i].Label != pair[1] {
			t.Errorf("INTEREST_TAGS[%d] = %q, Go %+v", i, pair, tags[i])
		}
	}
	if strings.Join(InterestIDs(), ",") != strings.Join(constants.InterestIDs, ",") {
		t.Errorf("INTEREST_IDS %q, Go %q", constants.InterestIDs, InterestIDs())
	}
	if MaxInterests != constants.MaxInterests {
		t.Errorf("MAX_INTERESTS %d, Go %d", constants.MaxInterests, MaxInterests)
	}
	bands := BudgetBands()
	if len(bands) != len(constants.BudgetBands) {
		t.Fatalf("BUDGET_BANDS has %d bands, Go %d", len(constants.BudgetBands), len(bands))
	}
	for i, encoded := range constants.BudgetBands {
		want, err := decodeValue(encoded)
		if err != nil {
			t.Fatal(err)
		}
		if !sameValue(want, renderBand(bands[i])) {
			t.Errorf("BUDGET_BANDS[%d] = %s, Go %s", i, show(want), show(renderBand(bands[i])))
		}
	}
	if strings.Join(BudgetBandIDs(), ",") != strings.Join(constants.BudgetBandIDs, ",") {
		t.Errorf("BUDGET_BAND_IDS %q, Go %q", constants.BudgetBandIDs, BudgetBandIDs())
	}
	// The accessors hand out copies: editing one must not edit the table.
	edited := BudgetBands()
	*edited[0].MaxVND = 1
	edited[0].Label = "x"
	InterestTags()[0].ID = "x"
	if !sameValue(renderBand(BudgetBands()[0]), renderBand(bands[0])) || InterestTags()[0].ID != tags[0].ID {
		t.Error("an accessor exposed the package table")
	}
}

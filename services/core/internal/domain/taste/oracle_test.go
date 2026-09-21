package taste

import (
	"fmt"
	"math/big"
	"slices"
	"sort"
	"strings"
	"testing"
	"unicode"

	"mobile/services/core/internal/domain/interests"
)

// testdata/python_*.json is rendered by scripts/render_places_taste_goldens.py
// from the real app.places.taste in the pinned API image. Every call of every
// case is replayed through the typed Go function; an input the Go types cannot
// hold fails the test rather than being skipped.

func placeInput(value any) (Place, error) {
	dict, err := dictInput(value)
	if err != nil {
		return Place{}, err
	}
	var place Place
	raw, present := dict["category"]
	if place.Category, err = categoryInput(raw, present); err != nil {
		return Place{}, err
	}
	raw, present = dict["traits"]
	if place.Traits, err = jsonbWords(raw, present); err != nil {
		return Place{}, err
	}
	raw, present = dict["kinds"]
	place.Kinds, err = jsonbWords(raw, present)
	return place, err
}

func renderProfile(p Profile) pyProfile {
	return pyProfile{
		"basis":                 string(p.Basis),
		"interests":             pyTuple(stringList(p.Interests)),
		"budget_per_person_vnd": optionalBig(p.BudgetPerPersonVND),
		"size":                  optionalBig(p.Size),
		"people":                big.NewInt(p.People),
		"people_answered":       big.NewInt(p.PeopleAnswered),
		"cache_key":             p.CacheKey(),
		"known":                 p.Known(),
	}
}

func renderTokens(found map[string]struct{}) pySet {
	out := make(pySet, 0, len(found))
	for token := range found {
		out = append(out, token)
	}
	sort.Strings(out)
	return out
}

func membersInput(value any) ([]Member, error) {
	list, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("want a list of members, got %s", show(value))
	}
	members := make([]Member, len(list))
	for i, item := range list {
		pair, ok := item.(pyTuple)
		if !ok || len(pair) != 2 {
			return nil, fmt.Errorf("want (interests, band_id), got %s", show(item))
		}
		chosen, err := stringsInput(pair[0])
		if err != nil {
			return nil, err
		}
		band, err := optionalStringInput(pair[1])
		if err != nil {
			return nil, err
		}
		members[i] = Member{Interests: chosen, BandID: band}
	}
	return members, nil
}

func replay(fn string, in map[string]any) (outcome, error) {
	text := func(key string) (string, error) {
		value, ok := in[key].(string)
		if !ok {
			return "", fmt.Errorf("%s: want a str, got %s", key, show(in[key]))
		}
		return value, nil
	}
	switch fn {
	case "covers":
		tag, err := text("tag")
		return outcome{value: Covers(tag)}, err
	case "uncovered":
		tags, err := stringsInput(in["tags"])
		return outcome{value: stringList(Uncovered(tags))}, err
	case "matches":
		tag, err := text("tag")
		if err != nil {
			return outcome{}, err
		}
		place, err := placeInput(in["place"])
		return outcome{value: Matches(tag, place)}, err
	case "_tokens":
		values, err := jsonbWords(in["values"], true)
		return outcome{value: renderTokens(tokens(values))}, err
	case "_midpoint_vnd":
		band, err := optionalStringInput(in["band_id"])
		return outcome{value: optionalBig(midpointVND(band))}, err
	case "_midpoint_vnd/band":
		dict, err := dictInput(in["band"])
		if err != nil {
			return outcome{}, err
		}
		low, err := intInput(dict["min_vnd"])
		if err != nil {
			return outcome{}, err
		}
		high, err := optionalIntInput(dict["max_vnd"])
		band := interests.BudgetBand{ID: "synthetic", Label: "synthetic", MinVND: low, MaxVND: high}
		return outcome{value: big.NewInt(bandMidpoint(band))}, err
	case "profile_for_person":
		chosen, err := stringsInput(in["interests"])
		if err != nil {
			return outcome{}, err
		}
		band, err := optionalStringInput(in["band_id"])
		return outcome{value: renderProfile(ProfileForPerson(chosen, band))}, err
	case "profile_for_group":
		members, err := membersInput(in["members"])
		return outcome{value: renderProfile(ProfileForGroup(members))}, err
	}
	return outcome{}, fmt.Errorf("unknown fn %q", fn)
}

var tasteFunctions = []string{
	"covers", "uncovered", "matches", "_tokens", "_midpoint_vnd",
	"_midpoint_vnd/band", "profile_for_person", "profile_for_group",
}

func TestOracleCases(t *testing.T) {
	perFn := runGoldens(t, replay)
	for _, fn := range tasteFunctions {
		if perFn[fn] == 0 {
			t.Errorf("no golden call reached %s", fn)
		}
	}
}

func TestOracleFuzzVolume(t *testing.T) {
	checkFuzzVolume(t, tasteFunctions...)
}

func TestOracleConstants(t *testing.T) {
	var constants struct {
		InterestIDs []string  `json:"interest_ids"`
		Evidence    [][]any   `json:"evidence"`
		Basis       []string  `json:"basis"`
		Unknown     any       `json:"unknown"`
		Casefold    [][2]any  `json:"casefold"`
		IsSpace     [][2]rune `json:"isspace"`
		Unicode     string    `json:"unicode"`
	}
	loadConstants(t, &constants)

	if !slices.Equal(constants.InterestIDs, interests.InterestIDs()) {
		t.Errorf("INTEREST_IDS %q, Go %q", constants.InterestIDs, interests.InterestIDs())
	}
	var order []string
	for _, row := range constants.Evidence {
		if len(row) != 4 {
			t.Fatalf("evidence row %v", row)
		}
		tag := row[0].(string)
		order = append(order, tag)
		got, ok := EvidenceFor(tag)
		if !ok {
			t.Errorf("EVIDENCE has %q, Go does not", tag)
			continue
		}
		for i, part := range [][]string{got.Categories, got.Traits, got.Kinds} {
			want := make([]string, 0)
			for _, word := range row[i+1].([]any) {
				want = append(want, word.(string))
			}
			if !slices.Equal(want, part) {
				t.Errorf("EVIDENCE[%q] part %d = %q, Go %q", tag, i, want, part)
			}
		}
	}
	if !slices.Equal(order, EvidenceTags()) {
		t.Errorf("EVIDENCE order %q, Go %q", order, EvidenceTags())
	}
	if want := []string{string(BasisGroup), string(BasisPerson), string(BasisUnknown)}; !slices.Equal(constants.Basis, want) {
		t.Errorf("Basis %q, Go %q", constants.Basis, want)
	}
	unknown, err := decodeValue(constants.Unknown)
	if err != nil {
		t.Fatal(err)
	}
	if !sameValue(unknown, renderProfile(Unknown())) {
		t.Errorf("UNKNOWN %s, Go %s", show(unknown), show(renderProfile(Unknown())))
	}

	// The accessors hand out copies.
	edited, _ := EvidenceFor("cafe")
	edited.Kinds[0] = "x"
	EvidenceTags()[0] = "x"
	if again, _ := EvidenceFor("cafe"); again.Kinds[0] == "x" || EvidenceTags()[0] == "x" {
		t.Error("an accessor exposed the evidence table")
	}
	if unicode.Version != constants.Unicode {
		t.Errorf("Python's Unicode is %s, Go's %s", constants.Unicode, unicode.Version)
	}
}

// casefold and str.strip() decide every trait and kind match, so both are
// checked on every code point, not only on the ones the cases happen to use.
func TestCasefoldAndIsSpaceOnEveryCodePoint(t *testing.T) {
	var constants struct {
		Casefold [][2]any  `json:"casefold"`
		IsSpace  [][2]rune `json:"isspace"`
	}
	loadConstants(t, &constants)
	folds := make(map[rune]string, len(constants.Casefold))
	for _, pair := range constants.Casefold {
		point, err := intInput(mustDecode(t, pair[0]))
		if err != nil {
			t.Fatal(err)
		}
		folded, err := decodeString(pair[1])
		if err != nil {
			t.Fatal(err)
		}
		folds[rune(point)] = folded
	}
	if len(folds) < 1000 {
		t.Fatalf("only %d casefold entries recorded", len(folds))
	}
	var bad []string
	for r := rune(0); r <= unicode.MaxRune; r++ {
		if r >= 0xD800 && r <= 0xDFFF {
			continue
		}
		want, ok := folds[r]
		if !ok {
			want = string(r)
		}
		if got := casefold(string(r)); got != want {
			bad = append(bad, fmt.Sprintf("U+%04X: Python %+q, Go %+q", r, want, got))
		}
		space := false
		for _, span := range constants.IsSpace {
			if r >= span[0] && r <= span[1] {
				space = true
			}
		}
		if isPySpace(r) != space {
			bad = append(bad, fmt.Sprintf("U+%04X: isspace Python %v", r, space))
		}
	}
	if len(bad) > 0 {
		t.Fatalf("%d code points disagree:\n%s", len(bad), strings.Join(bad[:min(len(bad), 20)], "\n"))
	}
}

func mustDecode(t *testing.T, raw any) any {
	t.Helper()
	value, err := decodeValue(raw)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

package scoring

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"testing"

	"mobile/services/core/internal/domain/interests"
	"mobile/services/core/internal/domain/taste"
)

// testdata/python_*.json is rendered by scripts/render_places_taste_goldens.py
// from the real app.places.scoring in the pinned API image. Every call of
// every case is replayed through the typed Go function; an input the Go types
// cannot hold fails the test rather than being skipped.

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
	if place.Kinds, err = jsonbWords(raw, present); err != nil {
		return Place{}, err
	}
	if place.PriceMinVND, err = optionalIntInput(dict["price_min_vnd"]); err != nil {
		return Place{}, err
	}
	if place.PriceMaxVND, err = optionalIntInput(dict["price_max_vnd"]); err != nil {
		return Place{}, err
	}
	if place.TravelMinutes, err = optionalIntInput(dict["travel_minutes"]); err != nil {
		return Place{}, err
	}
	switch distance := dict["distance_km"].(type) {
	case nil:
	case float64:
		place.DistanceKM = &distance
	default:
		return Place{}, fmt.Errorf("distance_km: want a float, got %s", show(distance))
	}
	if raw := dict["group_fit"]; raw != nil {
		fit, err := dictInput(raw)
		if err != nil {
			return Place{}, err
		}
		// `{}` is falsy in Python and reads as no capacity.
		if len(fit) > 0 {
			low, err := intInput(fit["min_people"])
			if err != nil {
				return Place{}, err
			}
			high, err := intInput(fit["max_people"])
			if err != nil {
				return Place{}, err
			}
			place.GroupFit = &GroupFit{MinPeople: low, MaxPeople: high}
		}
	}
	return place, nil
}

// profileInput builds the Go profile and checks the recorded properties
// against CacheKey and Known on the way.
func profileInput(value any) (taste.Profile, error) {
	fields, ok := value.(pyProfile)
	if !ok {
		return taste.Profile{}, fmt.Errorf("want a TasteProfile, got %s", show(value))
	}
	basis, ok := fields["basis"].(string)
	if !ok {
		return taste.Profile{}, fmt.Errorf("basis %s", show(fields["basis"]))
	}
	chosen, err := stringsInput(fields["interests"])
	if err != nil {
		return taste.Profile{}, err
	}
	profile := taste.Profile{Basis: taste.Basis(basis), Interests: chosen}
	if profile.BudgetPerPersonVND, err = optionalIntInput(fields["budget_per_person_vnd"]); err != nil {
		return taste.Profile{}, err
	}
	if profile.Size, err = optionalIntInput(fields["size"]); err != nil {
		return taste.Profile{}, err
	}
	if profile.People, err = intInput(fields["people"]); err != nil {
		return taste.Profile{}, err
	}
	if profile.PeopleAnswered, err = intInput(fields["people_answered"]); err != nil {
		return taste.Profile{}, err
	}
	if key := fields["cache_key"]; key != profile.CacheKey() {
		return taste.Profile{}, fmt.Errorf("cache_key: Python %s, Go %q", show(key), profile.CacheKey())
	}
	if known := fields["known"]; known != profile.Known() {
		return taste.Profile{}, fmt.Errorf("known: Python %v, Go %v", known, profile.Known())
	}
	return profile, nil
}

func goOutcome(value any, err error) outcome {
	if err == nil {
		return outcome{value: value}
	}
	var raised *Error
	if errors.As(err, &raised) {
		return outcome{raised: true, errType: raised.Type, message: raised.Message}
	}
	return outcome{raised: true, errType: fmt.Sprintf("go:%T", err), message: err.Error()}
}

func optionalRat(value *big.Rat) any {
	if value == nil {
		return nil
	}
	return value
}

func renderScore(score *big.Int, factors []Factor) any {
	lines := make([]any, len(factors))
	for i, factor := range factors {
		lines[i] = map[string]any{"label": factor.Label, "detail": factor.Detail}
	}
	var number any
	if score != nil {
		number = score
	}
	return pyTuple{number, lines}
}

func replay(fn string, in map[string]any) (outcome, error) {
	if fn == "_exact" {
		value, ok := in["value"].(float64)
		if !ok {
			return outcome{}, fmt.Errorf("_exact takes a float, got %s", show(in["value"]))
		}
		result, err := exact(value)
		return goOutcome(optionalRat(result), err), nil
	}
	place, err := placeInput(in["place"])
	if err != nil {
		return outcome{}, err
	}
	if fn == "distance_fit" {
		result, err := DistanceFit(place)
		return goOutcome(optionalRat(result), err), nil
	}
	group, err := profileInput(in["profile"])
	if err != nil {
		return outcome{}, err
	}
	switch fn {
	case "budget_fit":
		result, err := BudgetFit(place, group)
		return goOutcome(optionalRat(result), err), nil
	case "taste_fit":
		share, hits := TasteFit(place, group)
		return outcome{value: pyTuple{optionalRat(share), stringList(hits)}}, nil
	case "group_size_fit":
		return outcome{value: optionalRat(GroupSizeFit(place, group))}, nil
	case "score_place":
		score, factors, err := ScorePlace(place, group)
		if err != nil {
			return goOutcome(nil, err), nil
		}
		return outcome{value: renderScore(score, factors)}, nil
	}
	return outcome{}, fmt.Errorf("unknown fn %q", fn)
}

var scoringFunctions = []string{
	"_exact", "budget_fit", "taste_fit", "distance_fit", "group_size_fit", "score_place",
}

func TestOracleCases(t *testing.T) {
	perFn := runGoldens(t, replay)
	for _, fn := range scoringFunctions {
		if perFn[fn] == 0 {
			t.Errorf("no golden call reached %s", fn)
		}
	}
}

func TestOracleFuzzVolume(t *testing.T) {
	checkFuzzVolume(t, scoringFunctions...)
}

func TestOracleConstants(t *testing.T) {
	var constants struct {
		Weights []int       `json:"weights"`
		FarKM   any         `json:"far_km"`
		Labels  [][2]string `json:"labels"`
	}
	loadConstants(t, &constants)
	if got := []int{WeightBudget, WeightTaste, WeightDistance, WeightGroupSize}; fmt.Sprint(got) != fmt.Sprint(constants.Weights) {
		t.Errorf("weights %v, Go %v", constants.Weights, got)
	}
	far, err := decodeValue(constants.FarKM)
	if err != nil {
		t.Fatal(err)
	}
	if value, ok := far.(float64); !ok || math.Float64bits(value) != math.Float64bits(FarKM) {
		t.Errorf("FAR_KM %s, Go %v", show(far), FarKM)
	}
	if len(constants.Labels) != len(labelByTag) || len(constants.Labels) != len(interests.InterestTags()) {
		t.Errorf("_LABELS has %d entries, Go %d", len(constants.Labels), len(labelByTag))
	}
	for _, pair := range constants.Labels {
		if labelByTag[pair[0]] != pair[1] {
			t.Errorf("_LABELS[%q] = %q, Go %q", pair[0], pair[1], labelByTag[pair[0]])
		}
	}
}

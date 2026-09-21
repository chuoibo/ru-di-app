// Package scoring ports app.places.scoring (rd-be-05, M9, M11): the match
// percentage as four weighted terms over a catalogue row and a taste profile,
// with each term handed back as a factor line.
//
// Parity, not correctness, is the contract (ADR-0029 §2.4): every function is
// replayed against the real Python module by oracle_test.go, using
// testdata/python_*.json rendered by scripts/render_places_taste_goldens.py in
// the pinned API image.
//
// # Exactness
//
// Python's Fraction is math/big.Rat here and every term stays exact. Prices
// and budgets are int64 đồng; the one place they are added, the midpoint of a
// price band, is computed without overflow, and the one subtraction goes
// through big.Int. A distance is a float64 because the column is a double, and
// it is read the way `_exact` reads it: through CPython's repr, so 1.2 is 6/5
// and not the binary value. The single rounding is Python's round() on a
// Fraction: floor, then half to even. The score is a *big.Int because nothing
// in Python bounds it: a negative budget or distance pushes a term above one.
//
// # Names
//
//	WEIGHT_*, FAR_KM   WeightBudget, WeightTaste, WeightDistance, WeightGroupSize, FarKM
//	_LABELS            labelByTag
//	_exact             exact
//	budget_fit         BudgetFit
//	taste_fit          TasteFit
//	distance_fit       DistanceFit
//	group_size_fit     GroupSizeFit
//	score_place        ScorePlace
//
// # Inputs
//
// Place is `PlaceRecord.to_row()` as the scorers read it: nil for a NULL
// column, and nil GroupFit both for NULL and for an empty JSONB object
// (Python's `not fit`). GroupFit holds the two integers the importer and the
// seed file write; a JSONB object missing either key, or holding other types,
// has no Go spelling here (Python would raise KeyError or TypeError).
package scoring

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"

	"mobile/services/core/internal/domain/interests"
	"mobile/services/core/internal/domain/taste"
)

// The weights of the four terms. They add up to 100.
const (
	WeightBudget    = 40
	WeightTaste     = 35
	WeightDistance  = 15
	WeightGroupSize = 10
)

// FarKM is FAR_KM: the distance at which a place stops counting as nearby.
const FarKM = 5.0

// labelByTag is _LABELS: tag id to the word the person chose.
var labelByTag = func() map[string]string {
	out := map[string]string{}
	for _, tag := range interests.InterestTags() {
		out[tag.ID] = tag.Label
	}
	return out
}()

// GroupFit is the stated capacity of a place.
type GroupFit struct {
	MinPeople int64
	MaxPeople int64
}

// Place is a catalogue row as the scorers read it (see the package comment).
type Place struct {
	Category      string
	Kinds         []string
	Traits        []string
	PriceMinVND   *int64
	PriceMaxVND   *int64
	DistanceKM    *float64
	TravelMinutes *int64
	GroupFit      *GroupFit
}

func (p Place) tasteRow() taste.Place {
	return taste.Place{Category: p.Category, Traits: p.Traits, Kinds: p.Kinds}
}

// Error is an exception Python raises from these functions for inputs the
// service can pass: ZeroDivisionError from a budget of zero against a band
// midpoint above it, ValueError from a distance that is not finite. Type is
// the exception class name and Message is `str(exc)`.
type Error struct {
	Type    string
	Message string
}

func (e *Error) Error() string { return e.Type + ": " + e.Message }

// pyFloatRepr is CPython's repr() of a float: the shortest digits that round
// trip, in fixed notation when the decimal point falls in (-4, 16] digits and
// in exponent notation otherwise, with ".0" on an integral fixed value and at
// least two exponent digits.
func pyFloatRepr(value float64) string {
	switch {
	case math.IsNaN(value):
		return "nan"
	case math.IsInf(value, 1):
		return "inf"
	case math.IsInf(value, -1):
		return "-inf"
	}
	sign := ""
	if math.Signbit(value) {
		sign = "-"
		value = -value
	}
	mantissa, exponent, _ := strings.Cut(strconv.FormatFloat(value, 'e', -1, 64), "e")
	digits := strings.Replace(mantissa, ".", "", 1)
	power, err := strconv.Atoi(exponent)
	if err != nil {
		panic("scoring: FormatFloat wrote exponent " + exponent)
	}
	point := power + 1 // value = 0.digits * 10**point
	if point > -4 && point <= 16 {
		switch {
		case point <= 0:
			return sign + "0." + strings.Repeat("0", -point) + digits
		case point >= len(digits):
			return sign + digits + strings.Repeat("0", point-len(digits)) + ".0"
		default:
			return sign + digits[:point] + "." + digits[point:]
		}
	}
	out := sign + digits[:1]
	if len(digits) > 1 {
		out += "." + digits[1:]
	}
	shown := point - 1
	expSign := "+"
	if shown < 0 {
		expSign = "-"
		shown = -shown
	}
	return fmt.Sprintf("%se%s%02d", out, expSign, shown)
}

// exact is _exact: Fraction(str(value)), the decimal the repr spells rather
// than the binary value of the float.
func exact(value float64) (*big.Rat, error) {
	text := pyFloatRepr(value)
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return nil, &Error{Type: "ValueError", Message: "Invalid literal for Fraction: '" + text + "'"}
	}
	out, ok := new(big.Rat).SetString(text)
	if !ok {
		panic("scoring: a finite repr is not a decimal literal: " + text)
	}
	return out, nil
}

// floorHalf is Python's (a + b) // 2 without overflowing int64 (see
// taste.floorHalf).
func floorHalf(a, b int64) int64 {
	return a>>1 + b>>1 + a&b&1
}

// floorDiv is Python's a // b for b > 0.
func floorDiv(a, b int64) int64 {
	quotient := a / b
	if a%b != 0 && a < 0 {
		quotient--
	}
	return quotient
}

func one() *big.Rat { return big.NewRat(1, 1) }

// atLeastZero is max(Fraction(0), value).
func atLeastZero(value *big.Rat) *big.Rat {
	if value.Sign() < 0 {
		return new(big.Rat)
	}
	return value
}

// BudgetFit is budget_fit: 1 at or under budget, falling to 0 at twice the
// budget; nil when the place has no price or nobody stated a budget.
func BudgetFit(place Place, group taste.Profile) (*big.Rat, error) {
	if place.PriceMinVND == nil || place.PriceMaxVND == nil || group.BudgetPerPersonVND == nil {
		return nil, nil
	}
	midpoint := floorHalf(*place.PriceMinVND, *place.PriceMaxVND)
	budget := *group.BudgetPerPersonVND
	if midpoint <= budget {
		return one(), nil
	}
	gap := new(big.Int).Sub(big.NewInt(midpoint), big.NewInt(budget))
	if budget == 0 {
		return nil, &Error{Type: "ZeroDivisionError", Message: "Fraction(" + gap.String() + ", 0)"}
	}
	over := new(big.Rat).SetFrac(gap, big.NewInt(budget))
	return atLeastZero(new(big.Rat).Sub(one(), over)), nil
}

// TasteFit is taste_fit: the share of the profile's tastes this place answers,
// and which ones in profile order; nil and an empty list when nobody claimed
// any.
func TasteFit(place Place, group taste.Profile) (*big.Rat, []string) {
	hits := []string{}
	if len(group.Interests) == 0 {
		return nil, hits
	}
	row := place.tasteRow()
	for _, tag := range group.Interests {
		if taste.Matches(tag, row) {
			hits = append(hits, tag)
		}
	}
	return big.NewRat(int64(len(hits)), int64(len(group.Interests))), hits
}

// DistanceFit is distance_fit: 1 next door, 0 at FarKM and beyond; nil when
// the row has no distance.
func DistanceFit(place Place) (*big.Rat, error) {
	if place.DistanceKM == nil {
		return nil, nil
	}
	here, err := exact(*place.DistanceKM)
	if err != nil {
		return nil, err
	}
	limit, err := exact(FarKM)
	if err != nil {
		return nil, err
	}
	return atLeastZero(new(big.Rat).Sub(one(), new(big.Rat).Quo(here, limit))), nil
}

// GroupSizeFit is group_size_fit: 1 when the party fits the stated capacity,
// 0 when it does not, nil when either side is unknown. A size of zero is
// known.
func GroupSizeFit(place Place, group taste.Profile) *big.Rat {
	if place.GroupFit == nil || group.Size == nil {
		return nil
	}
	if place.GroupFit.MinPeople <= *group.Size && *group.Size <= place.GroupFit.MaxPeople {
		return one()
	}
	return new(big.Rat)
}

// Factor is one line of the working shown under a badge.
type Factor struct {
	Label  string
	Detail string
}

// roundHalfEven is round() on a Fraction: floor, then up past one half, and
// to the even neighbour at exactly one half.
func roundHalfEven(value *big.Rat) *big.Int {
	denominator := value.Denom()
	floor, remainder := new(big.Int).DivMod(value.Num(), denominator, new(big.Int))
	switch new(big.Int).Lsh(remainder, 1).Cmp(denominator) {
	case -1:
		return floor
	case 1:
		return floor.Add(floor, big.NewInt(1))
	}
	if floor.Bit(0) == 0 {
		return floor
	}
	return floor.Add(floor, big.NewInt(1))
}

// ScorePlace is score_place: the badge number and the four lines that account
// for it. The number is nil when the profile knows nothing the person said or
// when no term had an answer. The error is the first exception Python raises,
// in its evaluation order (budget, then distance).
func ScorePlace(place Place, group taste.Profile) (*big.Int, []Factor, error) {
	budget, err := BudgetFit(place, group)
	if err != nil {
		return nil, nil, err
	}
	share, hits := TasteFit(place, group)
	near, err := DistanceFit(place)
	if err != nil {
		return nil, nil, err
	}
	size := GroupSizeFit(place, group)

	terms := [...]struct {
		weight int64
		value  *big.Rat
	}{
		{WeightBudget, budget},
		{WeightTaste, share},
		{WeightDistance, near},
		{WeightGroupSize, size},
	}
	weighted := new(big.Rat)
	totalWeight := int64(0)
	for _, term := range terms {
		if term.value == nil {
			continue
		}
		totalWeight += term.weight
		weighted.Add(weighted, new(big.Rat).Mul(big.NewRat(term.weight, 1), term.value))
	}
	var score *big.Int
	if group.Known() && totalWeight != 0 {
		scaled := new(big.Rat).Mul(weighted, big.NewRat(WeightBudget+WeightTaste+WeightDistance+WeightGroupSize, 1))
		score = roundHalfEven(scaled.Quo(scaled, big.NewRat(totalWeight, 1)))
	}

	stated := group.BudgetPerPersonVND
	var budgetDetail string
	switch {
	case stated == nil && budget == nil:
		budgetDetail = "chưa có giá, và chưa ai nói mức chi"
	case stated == nil:
		// Unreachable, as in Python: a budget term needs a stated budget.
		midpointK := floorDiv(floorHalf(*place.PriceMinVND, *place.PriceMaxVND), 1000)
		budgetDetail = fmt.Sprintf("~%dk/người; chưa ai nói mức chi", midpointK)
	case budget == nil:
		budgetDetail = fmt.Sprintf("chưa có giá; mức chi đã nói ~%dk/người", floorDiv(*stated, 1000))
	default:
		midpointK := floorDiv(floorHalf(*place.PriceMinVND, *place.PriceMaxVND), 1000)
		budgetDetail = fmt.Sprintf("~%dk/người so với ~%dk đã nói", midpointK, floorDiv(*stated, 1000))
	}

	capacity := "quán không ghi sức chứa"
	if place.GroupFit != nil {
		capacity = fmt.Sprintf("%d-%d người", place.GroupFit.MinPeople, place.GroupFit.MaxPeople)
	}

	distanceDetail := "chưa biết khoảng cách"
	if near != nil {
		distanceDetail = pyFloatRepr(*place.DistanceKM) + "km"
		if place.TravelMinutes != nil {
			distanceDetail += fmt.Sprintf(", đi khoảng %d phút", *place.TravelMinutes)
		}
	}

	var tasteDetail string
	switch {
	case share == nil:
		tasteDetail = "chưa chọn sở thích nào, nên chưa so được"
	case len(hits) > 0:
		words := make([]string, len(hits))
		for i, tag := range hits {
			words[i] = labelByTag[tag]
		}
		tasteDetail = strings.Join(words, ", ")
	default:
		tasteDetail = "không trùng sở thích nào đã chọn"
	}

	sizeDetail := "chưa biết đi mấy người; quán hợp " + capacity
	if group.Size != nil {
		sizeDetail = fmt.Sprintf("nhóm %d người, quán hợp %s", *group.Size, capacity)
	}

	factors := []Factor{
		{Label: "Budget", Detail: budgetDetail},
		{Label: "Sở thích", Detail: tasteDetail},
		{Label: "Nhóm", Detail: sizeDetail},
		{Label: "Khoảng cách", Detail: distanceDetail},
	}
	return score, factors, nil
}

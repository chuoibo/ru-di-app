// Package interests ports app.domain.interests (M11, ADR-0019): the closed
// taste vocabulary a person may use about themself, and the three budget
// bands.
//
// Parity, not correctness, is the contract (ADR-0029 §2.4): every function
// here is replayed against the real Python module by oracle_test.go, using
// testdata/python_*.json rendered by scripts/render_domain_w1_goldens.py in
// the pinned API image.
//
// # Values
//
// Python's functions take `object`. The typed functions take what the service
// actually passes (pydantic has already narrowed the body to `list[StrictStr]`
// and `StrictStr | None`). The `...Value` functions take a Python object the
// way encoding/json decodes one, so the refusals the typed path cannot reach
// are still ported with their codes and precedence:
//
//   - nil is None, bool is bool, string is str;
//   - int64 or int is int, float64 is float;
//   - []any or []string is a list or a tuple (Python treats both alike here);
//   - map[string]any is a dict;
//   - any other Go value is an object of some other type.
package interests

import "slices"

// Codes Python raises InterestError with. `str(exc)` is the code, and the
// service copies it verbatim into the 422 problem code.
const (
	CodeInterestsNotAList    = "interests_not_a_list"
	CodeInterestNotAString   = "interest_not_a_string"
	CodeInterestUnknown      = "interest_unknown"
	CodeInterestsTooMany     = "interests_too_many"
	CodeBudgetBandNotAString = "budget_band_not_a_string"
	CodeBudgetBandUnknown    = "budget_band_unknown"
)

// InterestError is Python's InterestError. Error() is `str(exc)`.
type InterestError struct {
	Code string
}

func (e *InterestError) Error() string { return e.Code }

func refuse(code string) error { return &InterestError{Code: code} }

// InterestTag is one taste as the server spells it.
type InterestTag struct {
	ID    string
	Label string
}

// interestTags is INTEREST_TAGS. The order is data: the personalization
// screen's reading order, and the order normalised answers come back in.
var interestTags = [...]InterestTag{
	{ID: "an-uong", Label: "Ăn uống"},
	{ID: "cafe", Label: "Cafe"},
	{ID: "nightlife", Label: "Nightlife"},
	{ID: "mon-local", Label: "Món local"},
	{ID: "outdoor", Label: "Outdoor"},
	{ID: "shopping", Label: "Shopping"},
	{ID: "karaoke", Label: "Karaoke"},
	{ID: "game", Label: "Game"},
}

// MaxInterests is MAX_INTERESTS: the size of the vocabulary itself.
const MaxInterests = len(interestTags)

// InterestTags returns a copy of INTEREST_TAGS in vocabulary order.
func InterestTags() []InterestTag { return slices.Clone(interestTags[:]) }

// InterestIDs returns INTEREST_IDS in vocabulary order.
func InterestIDs() []string {
	ids := make([]string, len(interestTags))
	for i, tag := range interestTags {
		ids[i] = tag.ID
	}
	return ids
}

// BudgetBand is one per-person, per-outing spending band in đồng: MinVND
// inclusive, MaxVND exclusive, MaxVND nil for an open top end.
type BudgetBand struct {
	ID     string
	Label  string
	MinVND int64
	MaxVND *int64
}

type bandRow struct {
	id, label  string
	minVND     int64
	maxVND     int64
	openTopEnd bool
}

// budgetBands is BUDGET_BANDS, same ids as the client's `so-thich.ts`.
var budgetBands = [...]bandRow{
	{id: "tiet-kiem", label: "Dưới 100K", minVND: 0, maxVND: 100_000},
	{id: "vua-phai", label: "100K–250K", minVND: 100_000, maxVND: 250_000},
	{id: "thoai-mai", label: "250K–500K", minVND: 250_000, maxVND: 500_000},
}

func (row bandRow) band() BudgetBand {
	band := BudgetBand{ID: row.id, Label: row.label, MinVND: row.minVND}
	if !row.openTopEnd {
		top := row.maxVND
		band.MaxVND = &top
	}
	return band
}

// BudgetBands returns BUDGET_BANDS in order. Each call builds fresh values, so
// a caller cannot edit the table through MaxVND.
func BudgetBands() []BudgetBand {
	bands := make([]BudgetBand, len(budgetBands))
	for i, row := range budgetBands {
		bands[i] = row.band()
	}
	return bands
}

// BudgetBandIDs returns BUDGET_BAND_IDS in order.
func BudgetBandIDs() []string {
	ids := make([]string, len(budgetBands))
	for i, row := range budgetBands {
		ids[i] = row.id
	}
	return ids
}

// BudgetBandOf is budget_band(band_id): the band with this id, or ok=false
// both for nil (skipped) and for an id this build no longer has. It never
// refuses; a stale id reads as «no answer».
func BudgetBandOf(id *string) (BudgetBand, bool) {
	if id == nil {
		return BudgetBand{}, false
	}
	for _, row := range budgetBands {
		if row.id == *id {
			return row.band(), true
		}
	}
	return BudgetBand{}, false
}

// tagIndex is the position of tag in INTEREST_IDS, or -1. Matching is exact:
// no case folding, no trimming, no Unicode normalisation (Python `in` on a
// tuple of str).
func tagIndex(tag string) int {
	for i, known := range interestTags {
		if known.ID == tag {
			return i
		}
	}
	return -1
}

// inVocabularyOrder is `[tag_id for tag_id in INTEREST_IDS if tag_id in seen]`.
// It never returns nil, so an empty answer stays `[]` on the wire.
func inVocabularyOrder(seen *[MaxInterests]bool) ([]string, error) {
	out := make([]string, 0, MaxInterests)
	for i, chosen := range seen {
		if chosen {
			out = append(out, interestTags[i].ID)
		}
	}
	// Unreachable: seen is indexed by the vocabulary. Python keeps the same
	// check under `pragma: no cover`, so the port keeps it too.
	if len(out) > MaxInterests {
		return nil, refuse(CodeInterestsTooMany)
	}
	return out, nil
}

// NormaliseInterests is normalise_interests for the list of strings the
// service passes: deduplicated and put back into vocabulary order, or
// CodeInterestUnknown at the first word outside the vocabulary.
func NormaliseInterests(tags []string) ([]string, error) {
	var seen [MaxInterests]bool
	for _, tag := range tags {
		i := tagIndex(tag)
		if i < 0 {
			return nil, refuse(CodeInterestUnknown)
		}
		seen[i] = true
	}
	return inVocabularyOrder(&seen)
}

// NormaliseInterestsValue is normalise_interests over a Python object (see the
// package comment). Checks run per element in input order, so the first bad
// element decides the code: ["zzz", 7] is unknown, [7, "zzz"] is not a string.
func NormaliseInterestsValue(tags any) ([]string, error) {
	switch list := tags.(type) {
	case []string:
		return NormaliseInterests(list)
	case []any:
		var seen [MaxInterests]bool
		for _, item := range list {
			tag, ok := item.(string)
			if !ok {
				return nil, refuse(CodeInterestNotAString)
			}
			i := tagIndex(tag)
			if i < 0 {
				return nil, refuse(CodeInterestUnknown)
			}
			seen[i] = true
		}
		return inVocabularyOrder(&seen)
	default:
		return nil, refuse(CodeInterestsNotAList)
	}
}

// NormaliseBudgetBand is normalise_budget_band for `StrictStr | None`: nil
// means «skipped» and stays nil; a known id comes back as a fresh pointer.
func NormaliseBudgetBand(id *string) (*string, error) {
	if id == nil {
		return nil, nil
	}
	for _, row := range budgetBands {
		if row.id == *id {
			stored := row.id
			return &stored, nil
		}
	}
	return nil, refuse(CodeBudgetBandUnknown)
}

// NormaliseBudgetBandValue is normalise_budget_band over a Python object.
func NormaliseBudgetBandValue(id any) (*string, error) {
	switch band := id.(type) {
	case nil:
		return nil, nil
	case string:
		return NormaliseBudgetBand(&band)
	default:
		return nil, refuse(CodeBudgetBandNotAString)
	}
}

// Package preferences ports app.domain.preferences (F31): a group's implicit
// taste profile, recomputed from its own check-ins and ledger-summed trips on
// every read. Nothing here is stored.
//
// Parity, not correctness, is the contract (ADR-0029 §2.4): oracle_test.go
// replays testdata/python_*.json, rendered by
// scripts/render_domain_w1_goldens.py from the real module in the pinned API
// image. That includes behaviour that looks unintended, such as a negative
// count being refused with the code for a non-integer.
//
// # Values
//
// BuildPreferenceProfile takes the shapes the service builds. The service
// makes each visit as `{"category": row.get("category"), "kinds":
// row.get("kinds")}` from a catalogue row (category is TEXT NOT NULL, kinds
// is JSONB NOT NULL of any JSON shape) and each trip as `{"split_total_vnd":
// int, "headcount": int}` from the recap.
//
// BuildPreferenceProfileValues takes Python objects the way encoding/json
// decodes them (nil is None, bool is bool, string is str, int64 or int is
// int, float64 is float, []any or []string is a list or tuple, map[string]any
// is a dict, anything else is some other object), so refusals the typed path
// cannot reach keep their codes and precedence. A dict key that is absent
// reads as None, which is what Python's `.get` returns.
package preferences

import (
	"slices"
	"strings"
)

// MaxTastesPerSection is MAX_TASTES_PER_SECTION.
const MaxTastesPerSection = 6

// MaxLabel is MAX_LABEL, in code points of the stripped label.
const MaxLabel = 40

// Codes Python raises PreferenceError with; `str(exc)` is the code. None of
// them reaches HTTP as a problem: the rows are built by the repository, so a
// refusal would be a 500.
const (
	CodeVisitMalformed  = "preference_visit_malformed"
	CodeTripMalformed   = "preference_trip_malformed"
	CodeCountNotInteger = "preference_count_not_integer"
)

// PreferenceError is Python's PreferenceError. Error() is `str(exc)`.
type PreferenceError struct {
	Code string
}

func (e *PreferenceError) Error() string { return e.Code }

func refuse(code string) error { return &PreferenceError{Code: code} }

// sectionOfCategory is SECTION_OF_CATEGORY, in its declaration order.
var sectionOfCategory = [...][2]string{
	{"quan-an-local", "food"},
	{"cafe", "activity"},
	{"vui-choi", "activity"},
	{"di-choi-dem", "activity"},
}

// sectionOrder is SECTION_ORDER.
var sectionOrder = [...]string{"food", "activity"}

// SectionOf is `SECTION_OF_CATEGORY.get(category)`. Matching is exact.
func SectionOf(category string) (string, bool) {
	for _, pair := range sectionOfCategory {
		if pair[0] == category {
			return pair[1], true
		}
	}
	return "", false
}

// SectionOfCategory returns SECTION_OF_CATEGORY as ordered pairs.
func SectionOfCategory() [][2]string { return slices.Clone(sectionOfCategory[:]) }

// SectionOrder returns SECTION_ORDER.
func SectionOrder() []string { return slices.Clone(sectionOrder[:]) }

// Visit is one check-in resolved through the place catalogue.
type Visit struct {
	// Category is the catalogue row's category. A category outside
	// SECTION_OF_CATEGORY skips the visit; it is never defaulted.
	Category string
	// Kinds is the catalogue row's JSONB `kinds`, decoded into the value model
	// above. Only a list counts: nil, a string or a dict skips the visit, and
	// elements that are not strings are ignored.
	Kinds any
}

// Trip is one started outing from the recap.
type Trip struct {
	SplitTotalVND int64
	Headcount     int64
}

// Taste is one row of a section.
type Taste struct {
	Label        string
	CheckinCount int64
	// Score is `((count*200 + top) // (2*top)) / 100`: half-up in integers,
	// then one float division. It is not money.
	Score float64
}

// Section is one heading of the profile. TasteCount is the number of distinct
// tastes found; Tastes holds at most MaxTastesPerSection of them.
type Section struct {
	Section    string
	TasteCount int
	Tastes     []Taste
}

// Profile is the dict build_preference_profile returns.
type Profile struct {
	Sections      []Section
	CheckinCount  int
	OutingCount   int
	SplitTotalVND int64
	// AvgPerPersonVND is `total // people`, nil when the trips carry no people.
	AvgPerPersonVND *int64
}

// tally is `per_section` plus `counted`. Labels keep first-seen order, as a
// Counter does, although the sort key never lets that order show.
type tally struct {
	order   map[string][]string
	counts  map[string]map[string]int64
	counted int
}

func newTally() *tally {
	t := &tally{order: map[string][]string{}, counts: map[string]map[string]int64{}}
	for _, section := range sectionOrder {
		t.counts[section] = map[string]int64{}
	}
	return t
}

// visit is one pass of the visits loop: `_labels(visit)` and the counting.
func (t *tally) visit(category, kinds any) {
	name, ok := category.(string)
	if !ok {
		return
	}
	section, ok := SectionOf(name)
	if !ok {
		return
	}
	var labels []string
	keep := func(kind string) {
		if stripped := strings.TrimFunc(kind, isPySpace); stripped != "" {
			labels = append(labels, firstCodePoints(stripped, MaxLabel))
		}
	}
	switch list := kinds.(type) {
	case []string:
		for _, kind := range list {
			keep(kind)
		}
	case []any:
		for _, item := range list {
			if kind, isText := item.(string); isText {
				keep(kind)
			}
		}
	default:
		return
	}
	if len(labels) == 0 {
		return
	}
	t.counted++
	counts := t.counts[section]
	for _, label := range labels {
		if _, seen := counts[label]; !seen {
			t.order[section] = append(t.order[section], label)
		}
		counts[label]++
	}
}

// sections builds the section list in SECTION_ORDER, skipping empty ones. It
// never returns nil.
func (t *tally) sections() []Section {
	out := []Section{}
	for _, section := range sectionOrder {
		labels := t.order[section]
		if len(labels) == 0 {
			continue
		}
		counts := t.counts[section]
		var top int64
		for _, label := range labels {
			top = max(top, counts[label])
		}
		ordered := slices.Clone(labels)
		// sorted(key=(-count, label)): count descending, then label by code
		// point, which is byte order for valid UTF-8. Stable, like Python.
		slices.SortStableFunc(ordered, func(a, b string) int {
			if counts[a] != counts[b] {
				if counts[a] > counts[b] {
					return -1
				}
				return 1
			}
			return strings.Compare(a, b)
		})
		tastes := make([]Taste, 0, min(len(ordered), MaxTastesPerSection))
		for _, label := range ordered[:min(len(ordered), MaxTastesPerSection)] {
			count := counts[label]
			tastes = append(tastes, Taste{
				Label:        label,
				CheckinCount: count,
				Score:        float64((count*200+top)/(2*top)) / 100,
			})
		}
		out = append(out, Section{Section: section, TasteCount: len(ordered), Tastes: tastes})
	}
	return out
}

// finish is the trips-free tail of build_preference_profile.
func finish(t *tally, sections []Section, trips int, total, people int64) Profile {
	profile := Profile{
		Sections:      sections,
		CheckinCount:  t.counted,
		OutingCount:   trips,
		SplitTotalVND: total,
	}
	if people != 0 {
		avg := total / people
		profile.AvgPerPersonVND = &avg
	}
	return profile
}

// BuildPreferenceProfile is build_preference_profile for the shapes the
// service builds. Totals are summed in int64; Python's int does not overflow.
func BuildPreferenceProfile(visits []Visit, trips []Trip) (Profile, error) {
	t := newTally()
	for _, v := range visits {
		t.visit(v.Category, v.Kinds)
	}
	sections := t.sections()
	var total, people int64
	for _, trip := range trips {
		if trip.SplitTotalVND < 0 {
			return Profile{}, refuse(CodeCountNotInteger)
		}
		total += trip.SplitTotalVND
		if trip.Headcount < 0 {
			return Profile{}, refuse(CodeCountNotInteger)
		}
		people += trip.Headcount
	}
	return finish(t, sections, len(trips), total, people), nil
}

// BuildPreferenceProfileValues is build_preference_profile over Python
// objects. Every visit is read before any trip, so a malformed visit wins
// over a malformed trip.
func BuildPreferenceProfileValues(visits, trips []any) (Profile, error) {
	t := newTally()
	for _, raw := range visits {
		visit, ok := raw.(map[string]any)
		if !ok {
			return Profile{}, refuse(CodeVisitMalformed)
		}
		t.visit(visit["category"], visit["kinds"])
	}
	sections := t.sections()
	var total, people int64
	for _, raw := range trips {
		trip, ok := raw.(map[string]any)
		if !ok {
			return Profile{}, refuse(CodeTripMalformed)
		}
		split, err := integerCount(trip["split_total_vnd"])
		if err != nil {
			return Profile{}, err
		}
		total += split
		headcount, err := integerCount(trip["headcount"])
		if err != nil {
			return Profile{}, err
		}
		people += headcount
	}
	return finish(t, sections, len(trips), total, people), nil
}

// integerCount is `_integer_count`: money.count_violation with minimum 0, so
// bool, float, str, None and negative ints all raise the same code.
func integerCount(value any) (int64, error) {
	var n int64
	switch v := value.(type) {
	case int64:
		n = v
	case int:
		n = int64(v)
	default:
		return 0, refuse(CodeCountNotInteger)
	}
	if n < 0 {
		return 0, refuse(CodeCountNotInteger)
	}
	return n, nil
}

// firstCodePoints is `s[:n]` on a Python str.
func firstCodePoints(s string, n int) string {
	count := 0
	for i := range s {
		if count == n {
			return s[:i]
		}
		count++
	}
	return s
}

// isPySpace is CPython's str.isspace() for one code point: bidirectional
// class WS, B or S, or category Zs. Unlike unicode.IsSpace it includes
// U+001C..U+001F. oracle_test.go checks it against every code point.
func isPySpace(r rune) bool {
	switch {
	case r >= 0x09 && r <= 0x0D, r >= 0x1C && r <= 0x20:
		return true
	case r == 0x85, r == 0xA0, r == 0x1680, r >= 0x2000 && r <= 0x200A,
		r == 0x2028, r == 0x2029, r == 0x202F, r == 0x205F, r == 0x3000:
		return true
	}
	return false
}

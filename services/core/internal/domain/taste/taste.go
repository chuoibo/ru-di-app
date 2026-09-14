// Package taste ports app.places.taste (M11, ADR-0019): what a taste word
// means about a catalogue row, and whose budget and taste a card is scored
// against.
//
// Parity, not correctness, is the contract (ADR-0029 §2.4): every function is
// replayed against the real Python module by oracle_test.go, using
// testdata/python_*.json rendered by scripts/render_places_taste_goldens.py in
// the pinned API image.
//
// # Names
//
//	EVIDENCE            EvidenceFor, EvidenceTags
//	covers, uncovered   Covers, Uncovered
//	matches, _tokens    Matches, tokens
//	TasteProfile        Profile (cache_key, known: CacheKey, Known)
//	UNKNOWN             Unknown()
//	_midpoint_vnd       midpointVND, bandMidpoint
//	profile_for_person  ProfileForPerson
//	profile_for_group   ProfileForGroup
//
// # Inputs
//
// The functions take what the service passes. Place is the part of
// `PlaceRecord.to_row()` a match reads: `category` is a NOT NULL text column,
// `traits` and `kinds` are NOT NULL JSONB lists. A JSONB element that is not a
// string must reach Traits or Kinds as "" (Python's `_tokens` skips it, and it
// skips an empty or all-space string the same way). Interests and band ids are
// the strings the repository returns; a band id may be stale or empty.
//
// Nothing here raises in Python for those inputs, so nothing returns an error.
package taste

import (
	"slices"
	"strconv"
	"strings"

	"mobile/services/core/internal/domain/interests"
)

// Basis says whose answers a Profile holds.
type Basis string

// The three bases, as the wire spells them.
const (
	BasisGroup   Basis = "nhom"
	BasisPerson  Basis = "ca-nhan"
	BasisUnknown Basis = "chua-biet"
)

// Evidence is TasteEvidence: which parts of a catalogue row count as one
// taste.
type Evidence struct {
	Categories []string
	Traits     []string
	Kinds      []string
}

type evidenceRow struct {
	tag        string
	categories []string
	traits     []string
	kinds      []string
	// Casefolded traits and kinds, computed once: Python folds the same
	// constant words on every call and gets the same answer.
	foldedTraits []string
	foldedKinds  []string
}

// evidenceTable is EVIDENCE, in its dict order. Every word is written by the
// importer or the seed file.
var evidenceTable = [...]evidenceRow{
	{tag: "an-uong", categories: []string{"quan-an-local"}},
	{tag: "cafe", categories: []string{"cafe"}, kinds: []string{"Cà phê", "Trà sữa", "Trà"}},
	{tag: "nightlife", categories: []string{"di-choi-dem"}},
	{tag: "mon-local", kinds: []string{"Việt", "Phở", "Mì · bún", "Lẩu", "Ăn vặt", "Ăn sáng", "Local"}},
	{tag: "outdoor", traits: []string{"Ngoài trời"}, kinds: []string{"Park", "Garden", "Viewpoint"}},
	// Nothing to read: this catalogue imports neither shops nor karaoke.
	{tag: "shopping"},
	{tag: "karaoke"},
	{tag: "game", kinds: []string{"Bowling alley", "Theme park", "Cinema", "Water park"}},
}

func init() {
	// Python asserts at import that EVIDENCE and the vocabulary name the same
	// words; a build that breaks that must not start either.
	ids := interests.InterestIDs()
	if len(ids) != len(evidenceTable) {
		panic("taste: EVIDENCE and the interest vocabulary differ")
	}
	for i := range evidenceTable {
		row := &evidenceTable[i]
		if !slices.Contains(ids, row.tag) {
			panic("taste: EVIDENCE names " + row.tag + ", which the vocabulary does not")
		}
		row.foldedTraits = foldAll(row.traits)
		row.foldedKinds = foldAll(row.kinds)
	}
}

func foldAll(words []string) []string {
	out := make([]string, len(words))
	for i, word := range words {
		out[i] = casefold(word)
	}
	return out
}

// lookup is EVIDENCE.get(tag): exact on the key, no folding or trimming.
func lookup(tag string) *evidenceRow {
	for i := range evidenceTable {
		if evidenceTable[i].tag == tag {
			return &evidenceTable[i]
		}
	}
	return nil
}

// EvidenceTags returns the keys of EVIDENCE in dict order.
func EvidenceTags() []string {
	tags := make([]string, len(evidenceTable))
	for i, row := range evidenceTable {
		tags[i] = row.tag
	}
	return tags
}

// EvidenceFor is EVIDENCE.get(tag), as a copy the caller may edit.
func EvidenceFor(tag string) (Evidence, bool) {
	row := lookup(tag)
	if row == nil {
		return Evidence{}, false
	}
	return Evidence{
		Categories: slices.Clone(row.categories),
		Traits:     slices.Clone(row.traits),
		Kinds:      slices.Clone(row.kinds),
	}, true
}

// Covers is covers(tag): does this catalogue have anything at all that could
// match this taste?
func Covers(tag string) bool {
	row := lookup(tag)
	return row != nil && (len(row.categories) > 0 || len(row.traits) > 0 || len(row.kinds) > 0)
}

// Uncovered is uncovered(tags): the chosen tastes this catalogue cannot speak
// to, in vocabulary order. Membership is exact. Never nil, so an empty answer
// stays `[]` on the wire.
func Uncovered(tags []string) []string {
	out := []string{}
	for _, id := range interests.InterestIDs() {
		if slices.Contains(tags, id) && !Covers(id) {
			out = append(out, id)
		}
	}
	return out
}

// Place is the part of a catalogue row Matches reads (see the package
// comment for how JSONB reaches it).
type Place struct {
	Category string
	Traits   []string
	Kinds    []string
}

// tokens is _tokens: each value stripped with str.strip() and casefolded,
// dropping those that strip to nothing.
func tokens(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		stripped := pyStrip(value)
		if stripped == "" {
			continue
		}
		out[casefold(stripped)] = struct{}{}
	}
	return out
}

func meets(found map[string]struct{}, folded []string) bool {
	for _, word := range folded {
		if _, ok := found[word]; ok {
			return true
		}
	}
	return false
}

// Matches is matches(tag, place): is this row evidence of this taste? The tag
// and the category are exact; traits and kinds match whole tokens after
// str.strip() and str.casefold().
func Matches(tag string, place Place) bool {
	row := lookup(tag)
	if row == nil {
		return false
	}
	if slices.Contains(row.categories, place.Category) {
		return true
	}
	if meets(tokens(place.Traits), row.foldedTraits) {
		return true
	}
	return meets(tokens(place.Kinds), row.foldedKinds)
}

// Profile is TasteProfile: whose budget and taste a score is relative to.
// BudgetPerPersonVND and Size are nil when unknown, which is not zero.
type Profile struct {
	Basis              Basis
	Interests          []string
	BudgetPerPersonVND *int64
	Size               *int64
	People             int64
	PeopleAnswered     int64
}

func optionalInt(value *int64) string {
	if value == nil {
		return "None"
	}
	return strconv.FormatInt(*value, 10)
}

// CacheKey is TasteProfile.cache_key: equal exactly when two profiles would
// earn the same sentence from the model. An unknown number reads "None".
func (p Profile) CacheKey() string {
	return string(p.Basis) + "|" + strings.Join(p.Interests, ",") + "|" +
		optionalInt(p.BudgetPerPersonVND) + "|" + optionalInt(p.Size)
}

// Known is TasteProfile.known: is there anything here to score against?
func (p Profile) Known() bool {
	return len(p.Interests) > 0 || p.BudgetPerPersonVND != nil
}

// Unknown is UNKNOWN, the profile of an anonymous reader. Each call builds a
// fresh value.
func Unknown() Profile {
	return Profile{Basis: BasisUnknown, Interests: []string{}}
}

// floorHalf is Python's (a + b) // 2 without overflowing int64: with
// a = 2p + r and b = 2q + s, the floor is p + q + (r AND s), and >> is a
// floor division by two on negative numbers too.
func floorHalf(a, b int64) int64 {
	return a>>1 + b>>1 + a&b&1
}

// bandMidpoint is the arithmetic of _midpoint_vnd for a band that exists: the
// floor of the middle, or the bottom of an open-topped band.
func bandMidpoint(band interests.BudgetBand) int64 {
	if band.MaxVND == nil {
		return band.MinVND
	}
	return floorHalf(band.MinVND, *band.MaxVND)
}

// midpointVND is _midpoint_vnd(band_id): nil for no band and for a band id
// this build no longer has.
func midpointVND(bandID *string) *int64 {
	band, ok := interests.BudgetBandOf(bandID)
	if !ok {
		return nil
	}
	midpoint := bandMidpoint(band)
	return &midpoint
}

// inVocabularyOrder is `tuple(tag for tag in INTEREST_IDS if chosen(tag))`.
func inVocabularyOrder(chosen func(string) bool) []string {
	out := []string{}
	for _, id := range interests.InterestIDs() {
		if chosen(id) {
			out = append(out, id)
		}
	}
	return out
}

// ProfileForPerson is profile_for_person(interests, band_id): one person's
// own answers. PeopleAnswered follows Python's truthiness: any non-empty
// interest list or any non-empty band id counts, even an unknown word or a
// stale id.
func ProfileForPerson(chosen []string, bandID *string) Profile {
	answered := int64(0)
	if len(chosen) > 0 || (bandID != nil && *bandID != "") {
		answered = 1
	}
	return Profile{
		Basis:              BasisPerson,
		Interests:          inVocabularyOrder(func(id string) bool { return slices.Contains(chosen, id) }),
		BudgetPerPersonVND: midpointVND(bandID),
		Size:               nil,
		People:             1,
		PeopleAnswered:     answered,
	}
}

// Member is one member's answers: their interests and their band id.
type Member struct {
	Interests []string
	BandID    *string
}

// ProfileForGroup is profile_for_group(members): a taste counts if at least
// one member claimed it, and the budget is the floor-average of the midpoints
// of the bands that still exist. A member counts as answered with any
// interest or a band that resolves.
func ProfileForGroup(members []Member) Profile {
	tastes := map[string]bool{}
	var sum, count int64
	answered := int64(0)
	for _, member := range members {
		midpoint := midpointVND(member.BandID)
		if len(member.Interests) > 0 || midpoint != nil {
			answered++
		}
		for _, tag := range member.Interests {
			tastes[tag] = true
		}
		if midpoint != nil {
			// Real bands top out at a 375k midpoint, so the sum stays far
			// inside int64 for any roster.
			sum += *midpoint
			count++
		}
	}
	var budget *int64
	if count > 0 {
		average := floorDiv(sum, count)
		budget = &average
	}
	var size *int64
	if len(members) > 0 {
		people := int64(len(members))
		size = &people
	}
	return Profile{
		Basis:              BasisGroup,
		Interests:          inVocabularyOrder(func(id string) bool { return tastes[id] }),
		BudgetPerPersonVND: budget,
		Size:               size,
		People:             int64(len(members)),
		PeopleAnswered:     answered,
	}
}

// floorDiv is Python's a // b for b != 0.
func floorDiv(a, b int64) int64 {
	quotient := a / b
	if a%b != 0 && (a < 0) != (b < 0) {
		quotient--
	}
	return quotient
}

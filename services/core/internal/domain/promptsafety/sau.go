package promptsafety

import (
	"strconv"

	"mobile/services/core/internal/domain/tree"
)

// SafeDeep is Go-only and new; nothing above changed for it. Safe and Filter
// stay the port of app.places.prompt_safety that parity and the oracle
// goldens hold, and they read five fields. The AI engine and the retrieval
// index quote more of a row than that -- a description, reviews, activities
// -- so they run SafeDeep, which reads every text field of the row
// (docs/claude/2026-09-25/thiet-ke-ai/04-rag-va-nap-du-lieu.md §6 step 2).

// Bounds of the place.v1 source contract (design 04 §6.1 step 1) for the
// fields Safe does not read.
const (
	maxDescription = 1500
	maxReviewBody  = 600
	maxReviews     = 20
	maxActivities  = 20
)

// KetQuaSau says what SafeDeep did to one row.
type KetQuaSau struct {
	// Bo means the row must not reach a prompt or an index at all: a field
	// that names the place (id, destination, category, source, name,
	// address, opening hours, kinds, traits) is unsafe. The index tombstones
	// such a row as `unsafe`.
	Bo bool
	// CachLy names each quarantined field, in the row's field order:
	// "description", "reviews[2]", "activities[0]", "group_fit", "flag",
	// "license", "photo_author", "photo_license". A quarantined field is
	// emptied and the rest of the row stays usable.
	CachLy []string
}

// identity are the fields that name a place: an instruction hidden in one of
// them cannot be cut out without the row lying about what it is, so the row
// goes. Safe's five, plus the four system fields a catalogue import writes.
var identity = []struct {
	field string
	max   int
}{
	{"id", maxName}, {"destination_id", maxName}, {"category", maxItem}, {"source", maxItem},
}

// SafeDeep checks every text field of a catalogue row and returns a copy in
// which each unsafe descriptive field is emptied -- description and flag and
// license and the photo credits to null, group_fit to null when its relation
// is unsafe, an unsafe review or activity removed from its list (and any past
// the twentieth) -- with a report of what it did. A row Safe refuses, or one
// whose id, destination, category or source is unsafe, is dropped whole: the
// copy is nil and Bo is set. The input is never modified.
//
// A field is unsafe for the reasons Safe uses: longer than its bound,
// carrying a control character (a line break included), or matching the
// instruction catalogue on folded text.
func SafeDeep(place *tree.OrderedMap) (*tree.OrderedMap, KetQuaSau) {
	if place == nil || !Safe(place) {
		return nil, KetQuaSau{Bo: true}
	}
	for _, f := range identity {
		value, _ := place.Get(f.field)
		if !fieldSafe(value, f.max) {
			return nil, KetQuaSau{Bo: true}
		}
	}
	out := place.Clone()
	var report KetQuaSau
	for _, key := range place.Keys() {
		value, _ := place.Get(key)
		switch key {
		case "description":
			if !fieldSafe(value, maxDescription) {
				out.Set(key, tree.Null{})
				report.CachLy = append(report.CachLy, key)
			}
		case "flag":
			if !fieldSafe(value, maxItem) {
				out.Set(key, tree.Null{})
				report.CachLy = append(report.CachLy, key)
			}
		case "license":
			if !fieldSafe(value, maxAddress) {
				out.Set(key, tree.Null{})
				report.CachLy = append(report.CachLy, key)
			}
		case "photo_author", "photo_license":
			if !fieldSafe(value, maxName) {
				out.Set(key, tree.Null{})
				report.CachLy = append(report.CachLy, key)
			}
		case "group_fit":
			if !groupFitSafe(value) {
				out.Set(key, tree.Null{})
				report.CachLy = append(report.CachLy, key)
			}
		case "reviews":
			kept, dropped := keepSafe(value, maxReviews, reviewSafe)
			if dropped != nil {
				out.Set(key, kept)
				for _, d := range dropped {
					report.CachLy = append(report.CachLy, key+d)
				}
			}
		case "activities":
			kept, dropped := keepSafe(value, maxActivities, func(v tree.Value) bool {
				_, ok := v.(tree.String)
				return ok && fieldSafe(v, maxItem)
			})
			if dropped != nil {
				out.Set(key, kept)
				for _, d := range dropped {
					report.CachLy = append(report.CachLy, key+d)
				}
			}
		case "id", "destination_id", "category", "source", "name", "address", "open_hours", "kinds", "traits":
			// Checked above by Safe and identity: unsafe drops the row.
		default:
			// Any other field, known or not (a number that arrived as text,
			// a source_ref an import added), is read string by string at
			// every depth: "every text field" has no exceptions.
			if !everyStringSafe(value, maxAddress) {
				out.Set(key, tree.Null{})
				report.CachLy = append(report.CachLy, key)
			}
		}
	}
	return out, report
}

// everyStringSafe reports whether every string inside value -- itself, or
// any item of a list or value of an object, at any depth -- is safe text
// under the bound.
func everyStringSafe(value tree.Value, maxChars int) bool {
	switch v := value.(type) {
	case tree.String:
		return fieldSafe(v, maxChars)
	case tree.List:
		for _, item := range v {
			if !everyStringSafe(item, maxChars) {
				return false
			}
		}
	case *tree.OrderedMap:
		for _, k := range v.Keys() {
			item, _ := v.Get(k)
			if !everyStringSafe(item, maxChars) {
				return false
			}
		}
	}
	return true
}

// keepSafe filters a list field. It returns the kept items and the suffixes
// ("[i]", or "" for a value that is not a list at all) of what it dropped;
// dropped is nil when nothing was.
func keepSafe(value tree.Value, most int, ok func(tree.Value) bool) (tree.List, []string) {
	switch v := value.(type) {
	case nil, tree.Null:
		return nil, nil
	case tree.List:
		kept := tree.List{}
		var dropped []string
		for i, item := range v {
			if i >= most || !ok(item) {
				dropped = append(dropped, "["+strconv.Itoa(i)+"]")
				continue
			}
			kept = append(kept, item)
		}
		return kept, dropped
	default:
		return tree.List{}, []string{""}
	}
}

// reviewSafe is one review: an object whose author and body are safe text,
// and whose every other string -- a note, a rating sent as text, anything
// nested -- is safe under the body's bound.
func reviewSafe(value tree.Value) bool {
	review, ok := value.(*tree.OrderedMap)
	if !ok {
		return false
	}
	body, _ := review.Get("body")
	author, _ := review.Get("author")
	if _, isText := body.(tree.String); !isText {
		return false
	}
	if !fieldSafe(body, maxReviewBody) || !fieldSafe(author, maxName) {
		return false
	}
	for _, k := range review.Keys() {
		if k == "body" || k == "author" {
			continue
		}
		if item, _ := review.Get(k); !everyStringSafe(item, maxReviewBody) {
			return false
		}
	}
	return true
}

// groupFitSafe is group_fit: its relation and every other string in it.
func groupFitSafe(value tree.Value) bool {
	return everyStringSafe(value, maxItem)
}

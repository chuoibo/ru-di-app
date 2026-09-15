package dbsnap

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"mobile/parity/internal/normalize"
)

// Change is what happened to one schema between two snapshots.
type Change struct {
	// Relations names every relation in the later snapshot. Comparing it
	// catches a table that exists on one stack only.
	Relations []string
	// Changed holds only relations with at least one changed row, by name.
	Changed []RelationChange
}

// RelationChange is one relation's inserted, updated and deleted rows. Each
// list is in Texts order.
type RelationChange struct {
	Relation string
	// Keyed is true when rows were matched by primary key. Without a key (a
	// view, a key-less table) rows are a multiset of texts, so a changed row
	// shows as one deletion plus one insertion.
	Keyed    bool
	Inserted []Row
	Updated  []Update
	Deleted  []Row
}

// Update is one keyed row whose text changed.
type Update struct {
	Before Row
	After  Row
}

// Empty reports whether nothing changed.
func (c *Change) Empty() bool { return len(c.Changed) == 0 }

// Delta returns what changed from prev to next. A nil prev is an empty
// database, so Delta(nil, s) lists every row of s as inserted.
func Delta(prev, next *Snap) *Change {
	prevRels, nextRels := relationsByName(prev), relationsByName(next)
	change := &Change{Relations: []string{}}
	names := map[string]bool{}
	for name := range nextRels {
		change.Relations = append(change.Relations, name)
		names[name] = true
	}
	for name := range prevRels {
		names[name] = true
	}
	sort.Strings(change.Relations)
	ordered := make([]string, 0, len(names))
	for name := range names {
		ordered = append(ordered, name)
	}
	sort.Strings(ordered)

	for _, name := range ordered {
		before, after := prevRels[name], nextRels[name]
		var rc RelationChange
		if keyedPair(before, after) {
			rc = keyedDelta(rowsOf(before), rowsOf(after))
		} else {
			rc = multisetDelta(rowsOf(before), rowsOf(after))
		}
		rc.Relation = name
		if len(rc.Inserted)+len(rc.Updated)+len(rc.Deleted) > 0 {
			sortRelationChange(&rc)
			change.Changed = append(change.Changed, rc)
		}
	}
	return change
}

func relationsByName(s *Snap) map[string]*Relation {
	out := map[string]*Relation{}
	if s == nil {
		return out
	}
	for _, rel := range s.Relations {
		out[rel.Name] = rel
	}
	return out
}

func rowsOf(rel *Relation) []Row {
	if rel == nil {
		return nil
	}
	return rel.Rows
}

// keyedPair decides whether rows can be matched by key: both sides that exist
// must declare the same non-empty key and every key value must be unique.
func keyedPair(before, after *Relation) bool {
	var key []string
	for _, rel := range []*Relation{before, after} {
		if rel == nil {
			continue
		}
		if len(rel.PrimaryKey) == 0 {
			return false
		}
		if key != nil && strings.Join(key, "\x00") != strings.Join(rel.PrimaryKey, "\x00") {
			return false
		}
		key = rel.PrimaryKey
		seen := make(map[string]bool, len(rel.Rows))
		for _, row := range rel.Rows {
			if row.Key == "" || seen[row.Key] {
				return false
			}
			seen[row.Key] = true
		}
	}
	return key != nil
}

func keyedDelta(before, after []Row) RelationChange {
	rc := RelationChange{Keyed: true}
	old := make(map[string]Row, len(before))
	for _, row := range before {
		old[row.Key] = row
	}
	for _, row := range after {
		previous, existed := old[row.Key]
		switch {
		case !existed:
			rc.Inserted = append(rc.Inserted, row)
		case previous.Text != row.Text:
			rc.Updated = append(rc.Updated, Update{Before: previous, After: row})
		}
		delete(old, row.Key)
	}
	for _, row := range old {
		rc.Deleted = append(rc.Deleted, row)
	}
	return rc
}

func multisetDelta(before, after []Row) RelationChange {
	var rc RelationChange
	count := map[string]int{}
	for _, row := range before {
		count[row.Text]++
	}
	for _, row := range after {
		if count[row.Text] > 0 {
			count[row.Text]--
			continue
		}
		rc.Inserted = append(rc.Inserted, row)
	}
	for _, row := range before {
		if count[row.Text] > 0 {
			count[row.Text]--
			rc.Deleted = append(rc.Deleted, row)
		}
	}
	return rc
}

// The same patterns normalize binds. TestOrderMaskMatchesBinder keeps them in
// step with that package.
var (
	orderUUID      = regexp.MustCompile(`\b[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}\b`)
	orderTimestamp = regexp.MustCompile(`\b\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(\.\d{1,9})?(Z|[+-]\d{2}:\d{2})?`)
	orderHex       = regexp.MustCompile(`\\\\x[0-9a-f]*`)
	orderHexRun    = regexp.MustCompile(`[0-9a-f]{32,}`)
)

// orderMask hides the values two stacks cannot share, so that ordering rows
// by the masked text gives both stacks the same order. Hex bytea is masked
// too: a random token digest would otherwise decide the order. So is a random
// storage key, a run of exactly 32 lowercase hex, a keyset cursor, and a
// 43-character base64url token.
func orderMask(text string) string {
	text = normalize.MaskCursors(text)
	text = normalize.MaskTokens(text)
	text = orderUUID.ReplaceAllString(text, "<uuid>")
	text = orderTimestamp.ReplaceAllString(text, "<ts>")
	text = orderHex.ReplaceAllString(text, `\\x<hex>`)
	return orderHexRun.ReplaceAllStringFunc(text, func(run string) string {
		if len(run) == 32 {
			return "<hex32>"
		}
		return run
	})
}

func sortRows(rows []Row) {
	masks := make(map[string]string, len(rows))
	for _, row := range rows {
		if _, ok := masks[row.Text]; !ok {
			masks[row.Text] = orderMask(row.Text)
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		mi, mj := masks[rows[i].Text], masks[rows[j].Text]
		if mi != mj {
			return mi < mj
		}
		return rows[i].Text < rows[j].Text
	})
}

func sortUpdates(updates []Update) {
	type keyed struct {
		mask, full string
		update     Update
	}
	items := make([]keyed, len(updates))
	for i, u := range updates {
		items[i] = keyed{
			mask:   orderMask(u.Before.Text) + "\x00" + orderMask(u.After.Text),
			full:   u.Before.Text + "\x00" + u.After.Text,
			update: u,
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].mask != items[j].mask {
			return items[i].mask < items[j].mask
		}
		return items[i].full < items[j].full
	})
	for i := range items {
		updates[i] = items[i].update
	}
}

func sortRelationChange(rc *RelationChange) {
	sortRows(rc.Inserted)
	sortUpdates(rc.Updated)
	sortRows(rc.Deleted)
}

// Texts returns every text of the change, for Binder.Observe. The order is
// relation name, then inserted, updated (before, after) and deleted rows,
// each ordered by text with uuid4s, timestamps and hex masked, ties broken by
// the full text. Physical row order and random ids therefore do not decide
// it; rows identical after masking are ordered by their random values.
func (c *Change) Texts() []string {
	var out []string
	for _, rc := range c.Changed {
		for _, row := range rc.Inserted {
			out = append(out, row.Text)
		}
		for _, u := range rc.Updated {
			out = append(out, u.Before.Text, u.After.Text)
		}
		for _, row := range rc.Deleted {
			out = append(out, row.Text)
		}
	}
	return out
}

// Groups returns the same texts for Binder.ObserveGroups: one group per
// relation and kind (inserted, updated, deleted), each in Texts order, with an
// update's before and after texts joined by a NUL so the pair orders as one.
func (c *Change) Groups() [][]string {
	var out [][]string
	for _, rc := range c.Changed {
		var inserted, updated, deleted []string
		for _, row := range rc.Inserted {
			inserted = append(inserted, row.Text)
		}
		for _, u := range rc.Updated {
			updated = append(updated, u.Before.Text+updateSeparator+u.After.Text)
		}
		for _, row := range rc.Deleted {
			deleted = append(deleted, row.Text)
		}
		for _, group := range [][]string{inserted, updated, deleted} {
			if len(group) > 0 {
				out = append(out, group)
			}
		}
	}
	return out
}

// Normalise returns a copy whose texts went through apply (Binder.Apply).
// Keys and raw texts are left as they were; Compare reads only Text.
func (c *Change) Normalise(apply func(string) string) *Change {
	out := &Change{Relations: append([]string(nil), c.Relations...)}
	mapRow := func(row Row) Row {
		row.Text = apply(row.Text)
		return row
	}
	for _, rc := range c.Changed {
		n := RelationChange{Relation: rc.Relation, Keyed: rc.Keyed}
		for _, row := range rc.Inserted {
			n.Inserted = append(n.Inserted, mapRow(row))
		}
		for _, u := range rc.Updated {
			n.Updated = append(n.Updated, Update{Before: mapRow(u.Before), After: mapRow(u.After)})
		}
		for _, row := range rc.Deleted {
			n.Deleted = append(n.Deleted, mapRow(row))
		}
		sortRelationChange(&n)
		out.Changed = append(out.Changed, n)
	}
	return out
}

// Difference kinds.
const (
	KindRelations = "relations"
	KindInserted  = "inserted"
	KindUpdated   = "updated"
	KindDeleted   = "deleted"
)

// Difference is one relation and kind whose normalised rows differ.
type Difference struct {
	Relation string // empty for KindRelations
	Kind     string
	// Reference and Candidate count the rows of this kind on each side.
	Reference, Candidate int
	// OnlyReference and OnlyCandidate list, truncated for reading, the texts
	// one side has more often than the other.
	OnlyReference, OnlyCandidate []string
}

func (d Difference) String() string {
	var b strings.Builder
	where := d.Relation
	if where == "" {
		where = "(schema)"
	}
	fmt.Fprintf(&b, "db %s %s: reference %d, candidate %d", where, d.Kind, d.Reference, d.Candidate)
	for _, text := range d.OnlyReference {
		fmt.Fprintf(&b, "\n  only reference: %s", text)
	}
	for _, text := range d.OnlyCandidate {
		fmt.Fprintf(&b, "\n  only candidate: %s", text)
	}
	return b.String()
}

const (
	maxTextBytes  = 400
	maxListedRows = 10
)

// Compare reports every relation and kind whose normalised rows differ as
// multisets. Both changes must already be normalised; none means the step
// did the same thing to both databases.
func Compare(reference, candidate *Change) []Difference {
	var diffs []Difference
	onlyRef, onlyCand := multisetDiff(reference.Relations, candidate.Relations, identity)
	if len(onlyRef)+len(onlyCand) > 0 {
		diffs = append(diffs, Difference{
			Kind:          KindRelations,
			Reference:     len(reference.Relations),
			Candidate:     len(candidate.Relations),
			OnlyReference: listed(onlyRef),
			OnlyCandidate: listed(onlyCand),
		})
	}

	refChanges, candChanges := changesByName(reference), changesByName(candidate)
	names := map[string]bool{}
	for name := range refChanges {
		names[name] = true
	}
	for name := range candChanges {
		names[name] = true
	}
	ordered := make([]string, 0, len(names))
	for name := range names {
		ordered = append(ordered, name)
	}
	sort.Strings(ordered)

	for _, name := range ordered {
		ref, cand := refChanges[name], candChanges[name]
		for _, kind := range []string{KindInserted, KindUpdated, KindDeleted} {
			refItems, candItems := items(ref, kind), items(cand, kind)
			onlyRef, onlyCand := multisetDiff(refItems, candItems, displayItem)
			if len(onlyRef)+len(onlyCand) == 0 {
				continue
			}
			diffs = append(diffs, Difference{
				Relation:      name,
				Kind:          kind,
				Reference:     len(refItems),
				Candidate:     len(candItems),
				OnlyReference: listed(onlyRef),
				OnlyCandidate: listed(onlyCand),
			})
		}
	}
	return diffs
}

func changesByName(c *Change) map[string]*RelationChange {
	out := map[string]*RelationChange{}
	for i := range c.Changed {
		out[c.Changed[i].Relation] = &c.Changed[i]
	}
	return out
}

// updateSeparator joins an update's texts into one multiset item. It cannot
// occur in JSON text, which escapes control characters.
const updateSeparator = "\x00"

func items(rc *RelationChange, kind string) []string {
	if rc == nil {
		return nil
	}
	var out []string
	switch kind {
	case KindInserted:
		for _, row := range rc.Inserted {
			out = append(out, row.Text)
		}
	case KindDeleted:
		for _, row := range rc.Deleted {
			out = append(out, row.Text)
		}
	case KindUpdated:
		for _, u := range rc.Updated {
			out = append(out, u.Before.Text+updateSeparator+u.After.Text)
		}
	}
	return out
}

func identity(s string) string { return s }

// multisetDiff returns the items a has more often than b and vice versa,
// rendered by display and sorted.
func multisetDiff(a, b []string, display func(string) string) (onlyA, onlyB []string) {
	count := map[string]int{}
	for _, item := range a {
		count[item]++
	}
	for _, item := range b {
		count[item]--
	}
	for item, n := range count {
		for ; n > 0; n-- {
			onlyA = append(onlyA, display(item))
		}
		for ; n < 0; n++ {
			onlyB = append(onlyB, display(item))
		}
	}
	sort.Strings(onlyA)
	sort.Strings(onlyB)
	return onlyA, onlyB
}

// displayItem renders a multiset item for a reader. An update leads with the
// columns that changed, since the full texts are truncated.
func displayItem(item string) string {
	before, after, isUpdate := strings.Cut(item, updateSeparator)
	if !isUpdate {
		return truncate(item)
	}
	return fmt.Sprintf("set %s from %s in %s",
		truncate(changedMembers(before, after, true)),
		truncate(changedMembers(before, after, false)),
		truncate(before))
}

// changedMembers renders the top-level members whose values differ between
// before and after, with the values from after when inAfter, else from before.
func changedMembers(before, after string, inAfter bool) string {
	beforeMembers, errBefore := topLevelMembers(before)
	afterMembers, errAfter := topLevelMembers(after)
	if errBefore != nil || errAfter != nil {
		return "(not a JSON object)"
	}
	values := func(text string, members []member) map[string]string {
		out := make(map[string]string, len(members))
		for _, m := range members {
			out[m.name] = text[m.start:m.end]
		}
		return out
	}
	beforeValues, afterValues := values(before, beforeMembers), values(after, afterMembers)
	source, sourceText, other := afterMembers, after, beforeValues
	if !inAfter {
		source, sourceText, other = beforeMembers, before, afterValues
	}
	var parts []string
	for _, m := range source {
		value := sourceText[m.start:m.end]
		if otherValue, ok := other[m.name]; !ok || otherValue != value {
			parts = append(parts, jsonString(m.name)+":"+value)
		}
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func truncate(s string) string {
	if len(s) <= maxTextBytes {
		return s
	}
	cut := maxTextBytes
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return fmt.Sprintf("%s…(+%d bytes)", s[:cut], len(s)-cut)
}

func listed(texts []string) []string {
	if len(texts) <= maxListedRows {
		return texts
	}
	out := append([]string(nil), texts[:maxListedRows]...)
	return append(out, fmt.Sprintf("… and %d more", len(texts)-maxListedRows))
}

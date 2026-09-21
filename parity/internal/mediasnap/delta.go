package mediasnap

import (
	"fmt"
	"sort"
	"strings"

	"mobile/parity/internal/normalize"
)

// Change is what happened to one store between two snapshots. Each list is in
// mask order: random names hidden, ties broken by the full text.
type Change struct {
	Created []Entry
	Changed []Update
	Deleted []Entry
}

// Update is one path whose entry text changed: its size, content, mode, type
// or the mode of a directory above it.
type Update struct {
	Before Entry
	After  Entry
}

// Empty reports whether nothing changed.
func (c *Change) Empty() bool { return len(c.Created)+len(c.Changed)+len(c.Deleted) == 0 }

// Delta returns what changed from prev to next. Entries are matched by path. A
// deleted entry records the modes its directories have in next. A key
// directory adds a keydir entry only as the package comment says: created or
// modified with no entry beneath it created, changed or deleted, or removed
// while it was empty. A nil prev or next is an empty store.
func Delta(prev, next *Snap) *Change {
	if prev == nil {
		prev = &Snap{}
	}
	if next == nil {
		next = &Snap{}
	}
	old := map[string]Entry{}
	for _, e := range prev.Entries {
		old[e.Path] = e
	}
	change := &Change{}
	// explained holds every directory above an entry that changed: that
	// entry is why the directory was modified.
	explained := map[string]bool{}
	explain := func(path string) {
		for i := strings.LastIndex(path, "/"); i > 0; i = strings.LastIndex(path, "/") {
			path = path[:i]
			explained[path] = true
		}
	}
	for _, e := range next.Entries {
		before, existed := old[e.Path]
		switch {
		case !existed:
			change.Created = append(change.Created, e)
			explain(e.Path)
		case before.Text != e.Text:
			change.Changed = append(change.Changed, Update{Before: before, After: e})
			explain(e.Path)
		}
		delete(old, e.Path)
	}
	for _, e := range old {
		explain(e.Path)
		e.After = dirsAfter(next, e.Path)
		change.Deleted = append(change.Deleted, e.rendered())
	}
	for path, dir := range next.Dirs {
		if !keyDir(path) || explained[path] {
			continue
		}
		if before, existed := prev.Dirs[path]; existed && before.MTime == dir.MTime {
			continue
		}
		change.Created = append(change.Created, keyDirEntry(next, path))
	}
	for path, dir := range prev.Dirs {
		if !keyDir(path) || explained[path] || !dir.Empty {
			continue
		}
		if _, still := next.Dirs[path]; !still {
			change.Deleted = append(change.Deleted, keyDirEntry(prev, path))
		}
	}
	sortEntries(change.Created, func(e Entry) string { return e.mask })
	sortEntries(change.Deleted, func(e Entry) string { return e.mask })
	sortUpdates(change.Changed, func(e Entry) string { return e.mask })
	return change
}

func sortEntries(entries []Entry, mask func(Entry) string) {
	sort.SliceStable(entries, func(i, j int) bool {
		mi, mj := mask(entries[i]), mask(entries[j])
		if mi != mj {
			return mi < mj
		}
		return entries[i].Text < entries[j].Text
	})
}

func sortUpdates(updates []Update, mask func(Entry) string) {
	sort.SliceStable(updates, func(i, j int) bool {
		mi := mask(updates[i].Before) + separator + mask(updates[i].After)
		mj := mask(updates[j].Before) + separator + mask(updates[j].After)
		if mi != mj {
			return mi < mj
		}
		return updates[i].Before.Text+separator+updates[i].After.Text < updates[j].Before.Text+separator+updates[j].After.Text
	})
}

// separator joins an update's texts into one item. JSON text escapes control
// characters, so it never occurs inside one.
const separator = "\x00"

// Groups returns the change's texts for Binder.ObserveGroups: created,
// changed (before and after joined by a NUL) and deleted, each a group, empty
// groups left out. A stored file's key is observed here unless the step's
// response or database change numbered it first.
func (c *Change) Groups() [][]string {
	var out [][]string
	var created, changed, deleted []string
	for _, e := range c.Created {
		created = append(created, e.Text)
	}
	for _, u := range c.Changed {
		changed = append(changed, u.Before.Text+separator+u.After.Text)
	}
	for _, e := range c.Deleted {
		deleted = append(deleted, e.Text)
	}
	for _, group := range [][]string{created, changed, deleted} {
		if len(group) > 0 {
			out = append(out, group)
		}
	}
	return out
}

// Normalise returns a copy whose texts went through binder.Apply, after every
// text of the scenario was observed. A keydir entry is first tied to the one
// key the binder knows with its four hex digits, so it reads
// <hex32#n>[0:2]/<hex32#n>[2:4] like that key's file; with no such key, or
// two, it reads <hex2>/<hex2>.
func (c *Change) Normalise(binder *normalize.Binder) *Change {
	normalise := func(e Entry) Entry {
		text := e.Text
		if e.Place == PlaceKeyDir {
			path := "<hex2>/<hex2>"
			if keys := binder.KeysWithPrefix(e.Prefix); len(keys) == 1 {
				path = keys[0] + "[0:2]/" + keys[0] + "[2:4]"
			}
			text = e.render(path)
		}
		e.Text = binder.Apply(text)
		return e
	}
	out := &Change{}
	for _, e := range c.Created {
		out.Created = append(out.Created, normalise(e))
	}
	for _, u := range c.Changed {
		out.Changed = append(out.Changed, Update{Before: normalise(u.Before), After: normalise(u.After)})
	}
	for _, e := range c.Deleted {
		out.Deleted = append(out.Deleted, normalise(e))
	}
	byText := func(e Entry) string { return e.Text }
	sortEntries(out.Created, byText)
	sortEntries(out.Deleted, byText)
	sortUpdates(out.Changed, byText)
	return out
}

// Difference kinds.
const (
	KindCreated = "created"
	KindChanged = "changed"
	KindDeleted = "deleted"
)

// Difference is one kind of change whose normalised entries differ.
type Difference struct {
	Kind string
	// Reference and Candidate count the entries of this kind on each side.
	Reference, Candidate int
	// OnlyReference and OnlyCandidate list the texts one side has more often
	// than the other, capped for reading.
	OnlyReference, OnlyCandidate []string
}

func (d Difference) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s reference %d, candidate %d", d.Kind, d.Reference, d.Candidate)
	for _, text := range d.OnlyReference {
		fmt.Fprintf(&b, "\n  only reference: %s", text)
	}
	for _, text := range d.OnlyCandidate {
		fmt.Fprintf(&b, "\n  only candidate: %s", text)
	}
	return b.String()
}

const maxListed = 10

// Compare reports every kind whose normalised entries differ as multisets.
// Both changes must already be normalised; none means the step did the same
// thing to both stores.
func Compare(reference, candidate *Change) []Difference {
	var diffs []Difference
	for _, kind := range []string{KindCreated, KindChanged, KindDeleted} {
		ref, cand := items(reference, kind), items(candidate, kind)
		onlyRef, onlyCand := multisetDiff(ref, cand)
		if len(onlyRef)+len(onlyCand) == 0 {
			continue
		}
		diffs = append(diffs, Difference{
			Kind:          kind,
			Reference:     len(ref),
			Candidate:     len(cand),
			OnlyReference: listed(onlyRef),
			OnlyCandidate: listed(onlyCand),
		})
	}
	return diffs
}

func items(c *Change, kind string) []string {
	var out []string
	switch kind {
	case KindCreated:
		for _, e := range c.Created {
			out = append(out, e.Text)
		}
	case KindChanged:
		for _, u := range c.Changed {
			out = append(out, u.Before.Text+separator+u.After.Text)
		}
	case KindDeleted:
		for _, e := range c.Deleted {
			out = append(out, e.Text)
		}
	}
	return out
}

// multisetDiff returns the items a has more often than b and vice versa,
// rendered for reading and sorted.
func multisetDiff(a, b []string) (onlyA, onlyB []string) {
	count := map[string]int{}
	for _, item := range a {
		count[item]++
	}
	for _, item := range b {
		count[item]--
	}
	for item, n := range count {
		display := strings.Replace(item, separator, " -> ", 1)
		for ; n > 0; n-- {
			onlyA = append(onlyA, display)
		}
		for ; n < 0; n++ {
			onlyB = append(onlyB, display)
		}
	}
	sort.Strings(onlyA)
	sort.Strings(onlyB)
	return onlyA, onlyB
}

func listed(texts []string) []string {
	if len(texts) <= maxListed {
		return texts
	}
	out := append([]string(nil), texts[:maxListed]...)
	return append(out, fmt.Sprintf("… and %d more", len(texts)-maxListed))
}

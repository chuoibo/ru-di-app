// Package normalize replaces the values two independent runs cannot share —
// generated ids, tokens, timestamps — with placeholders, so that everything
// else can be compared byte for byte.
//
// The rule that makes this safe is that a placeholder binds a value but keeps
// its format. A timestamp becomes <ts#3|f6|Z>: which instant it was (by rank
// within the scenario), how many fractional digits it carried, and how its
// zone was written. Go writing "+00:00" where Python wrote "Z", or milliseconds
// where Python wrote microseconds, therefore still differs after
// normalisation. Numbers and ordinary text are never replaced.
//
// A 64-character lowercase hex digest the scenario did not name (a request
// fingerprint, a stored token digest) becomes <digest#n> by first appearance.
// A hash over a request that carries a generated id differs between stacks by
// construction; what still has to match is which rows and answers share one.
// That a Go digest equals Python's value is the business of the package's own
// golden tests and of cross-implementation replay, not of this comparison.
package normalize

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Only canonical lowercase version-4 UUIDs are replaced. An uppercase or
// unhyphenated id stays a literal, so a formatting change still shows.
var uuid4 = regexp.MustCompile(`\b[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}\b`)

// hexRun finds maximal runs of lowercase hex; only runs of exactly 64 are
// digests, so a longer or shorter run, or uppercase hex, stays literal.
var hexRun = regexp.MustCompile(`[0-9a-f]{64,}`)

// Broad on purpose: a malformed-but-close timestamp is still bound, and its
// shape suffix records exactly how it was malformed.
var timestamp = regexp.MustCompile(`\b\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(\.\d{1,9})?(Z|[+-]\d{2}:\d{2})?`)

// Binder holds the placeholder assignments for one scenario on one stack.
// Observe every text in scenario order first, then Apply.
type Binder struct {
	named    map[string]string // literal -> placeholder, bound explicitly
	namedOrd []string          // longest first, so a token never shadows a longer one
	uuids    map[string]int    // literal -> first-appearance number
	digests  map[string]int    // literal -> first-appearance number
	instants map[string]time.Time
	ranks    map[int64]int
	frozen   bool
}

// NewBinder returns an empty binder.
func NewBinder() *Binder {
	return &Binder{
		named:    map[string]string{},
		uuids:    map[string]int{},
		digests:  map[string]int{},
		instants: map[string]time.Time{},
	}
}

// Name binds an exact literal (a persona id, a captured token) to a stable
// name. Named literals are replaced before any pattern is applied.
func (b *Binder) Name(literal, name string) error {
	if b.frozen {
		return fmt.Errorf("normalize: Name(%q) after Apply", name)
	}
	if literal == "" {
		return fmt.Errorf("normalize: empty literal for %q", name)
	}
	placeholder := "<" + name + ">"
	if existing, ok := b.named[literal]; ok && existing != placeholder {
		return fmt.Errorf("normalize: literal already bound to %s, cannot rebind to %s", existing, placeholder)
	}
	for other, bound := range b.named {
		if bound == placeholder && other != literal {
			return fmt.Errorf("normalize: %s already names a different literal", placeholder)
		}
	}
	if _, ok := b.named[literal]; !ok {
		b.named[literal] = placeholder
		b.namedOrd = append(b.namedOrd, literal)
		sort.SliceStable(b.namedOrd, func(i, j int) bool { return len(b.namedOrd[i]) > len(b.namedOrd[j]) })
	}
	return nil
}

// Observe records the ids and instants a text contains. Numbering follows the
// order texts are observed in, so callers observe in scenario order.
func (b *Binder) Observe(text string) error {
	if b.frozen {
		return fmt.Errorf("normalize: Observe after Apply")
	}
	text = b.replaceNamed(text)
	for _, run := range hexRun.FindAllString(text, -1) {
		if _, seen := b.digests[run]; len(run) == 64 && !seen {
			b.digests[run] = len(b.digests) + 1
		}
	}
	for _, id := range uuid4.FindAllString(text, -1) {
		if _, seen := b.uuids[id]; !seen {
			b.uuids[id] = len(b.uuids) + 1
		}
	}
	for _, literal := range timestamp.FindAllString(text, -1) {
		if _, seen := b.instants[literal]; seen {
			continue
		}
		instant, ok := parseInstant(literal)
		if ok {
			b.instants[literal] = instant
		}
	}
	return nil
}

// Apply returns text with every bound value replaced. The first call freezes
// the binder: ranks are computed over everything observed so far.
func (b *Binder) Apply(text string) string {
	if !b.frozen {
		b.freeze()
	}
	text = b.replaceNamed(text)
	text = hexRun.ReplaceAllStringFunc(text, func(run string) string {
		if n, ok := b.digests[run]; ok {
			return fmt.Sprintf("<digest#%d>", n)
		}
		return run
	})
	text = uuid4.ReplaceAllStringFunc(text, func(id string) string {
		if n, ok := b.uuids[id]; ok {
			return fmt.Sprintf("<uuid#%d>", n)
		}
		// Not observed: a value that exists on only one side. Leaving it
		// literal makes that visible instead of numbering it silently.
		return id
	})
	return timestamp.ReplaceAllStringFunc(text, func(literal string) string {
		instant, ok := b.instants[literal]
		if !ok {
			return literal
		}
		return fmt.Sprintf("<ts#%d|%s>", b.ranks[instant.UnixNano()], Shape(literal))
	})
}

func (b *Binder) replaceNamed(text string) string {
	for _, literal := range b.namedOrd {
		text = strings.ReplaceAll(text, literal, b.named[literal])
	}
	return text
}

func (b *Binder) freeze() {
	b.frozen = true
	distinct := map[int64]bool{}
	for _, instant := range b.instants {
		distinct[instant.UnixNano()] = true
	}
	ordered := make([]int64, 0, len(distinct))
	for nanos := range distinct {
		ordered = append(ordered, nanos)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	b.ranks = make(map[int64]int, len(ordered))
	for i, nanos := range ordered {
		b.ranks[nanos] = i + 1
	}
}

// Shape describes how a timestamp literal is written without its value:
// separator, fractional digits and zone spelling, e.g. "f6|Z", "f0|+07:00",
// "space|f3|naive".
func Shape(literal string) string {
	match := timestamp.FindStringSubmatch(literal)
	if match == nil {
		return "not-a-timestamp"
	}
	parts := []string{}
	if strings.Contains(literal[:19], " ") {
		parts = append(parts, "space")
	}
	fraction := len(match[1])
	if fraction > 0 {
		fraction-- // the dot
	}
	parts = append(parts, fmt.Sprintf("f%d", fraction))
	zone := match[2]
	if zone == "" {
		zone = "naive"
	}
	parts = append(parts, zone)
	return strings.Join(parts, "|")
}

func parseInstant(literal string) (time.Time, bool) {
	value := strings.Replace(literal, " ", "T", 1)
	// Go accepts a fractional second after the seconds field even when the
	// layout omits it, so the naive layout needs no fraction spelled out.
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.UTC(), true
		}
	}
	return time.Time{}, false
}

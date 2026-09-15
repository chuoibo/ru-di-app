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
//
// A 32-character lowercase hex run (a storage key from secrets.token_hex(16))
// becomes <hex32#n> the same way. The key is random per upload and never sent
// back, so only the database lane sees it; what has to match is which rows
// share a key, and that a key is 32 lowercase hex at all.
//
// A keyset cursor (app/api/cursors.py encode_cursor: base64url without padding
// of "<isoformat>|<uuid>") is random per stack only through the instant and
// the id inside it. It becomes <b64u:<ts#r|shape>|<uuid#n>>: both parts are
// bound as they would be in plain text, while the alphabet, the missing
// padding and the timestamp spelling still show. A padded cursor, or one
// around an uppercase id, is not a match and stays literal.
//
// A run of exactly 43 base64url characters (secrets.token_urlsafe(32), such as
// the guest link token in "/g/<token>") becomes <token43#n> by first
// appearance. What must match is where a token is reused and that it is 43
// base64url characters; a shorter, longer or padded token stays literal.
// Tokens a scenario binds by name are replaced before this rule.
package normalize

import (
	"encoding/base64"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// Only canonical lowercase version-4 UUIDs are replaced. An uppercase or
// unhyphenated id stays a literal, so a formatting change still shows.
var uuid4 = regexp.MustCompile(`\b[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}\b`)

// hexRun finds maximal runs of lowercase hex; only runs of exactly 64
// (digests) or exactly 32 (random keys) are bound, so any other length, or
// uppercase hex, stays literal.
var hexRun = regexp.MustCompile(`[0-9a-f]{32,}`)

const (
	digestLen = 64
	keyLen    = 32
)

// base64URLRun finds maximal runs of the base64url alphabet long enough to
// hold a cursor; only runs that decode to a cursor payload are bound.
var base64URLRun = regexp.MustCompile(`[A-Za-z0-9_-]{60,}`)

var cursorPayload = regexp.MustCompile(`\A` + timestamp.String() + `\|` + uuid4.String() + `\z`)

// DecodeCursor returns the payload of a keyset cursor: run must be canonical
// base64url without padding of "<timestamp>|<uuid4>". Anything else is not a
// cursor.
func DecodeCursor(run string) (string, bool) {
	raw, err := base64.RawURLEncoding.Strict().DecodeString(run)
	if err != nil || !utf8.Valid(raw) || !cursorPayload.Match(raw) {
		return "", false
	}
	return string(raw), true
}

// tokenRun finds maximal base64url runs; only runs of exactly 43 characters
// are tokens.
var tokenRun = regexp.MustCompile(`[A-Za-z0-9_-]{43,}`)

const tokenLen = 43

// MaskTokens replaces every 43-character base64url run in text with <token43>.
func MaskTokens(text string) string {
	return tokenRun.ReplaceAllStringFunc(text, func(run string) string {
		if len(run) == tokenLen {
			return "<token43>"
		}
		return run
	})
}

// MaskCursors replaces every cursor in text with <b64u>.
func MaskCursors(text string) string {
	return base64URLRun.ReplaceAllStringFunc(text, func(run string) string {
		if _, ok := DecodeCursor(run); ok {
			return "<b64u>"
		}
		return run
	})
}

// Broad on purpose: a malformed-but-close timestamp is still bound, and its
// shape suffix records exactly how it was malformed.
var timestamp = regexp.MustCompile(`\b\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(\.\d{1,9})?(Z|[+-]\d{2}:\d{2})?`)

// Binder holds the placeholder assignments for one scenario on one stack.
// Observe every text in scenario order first, then Apply.
type Binder struct {
	named        map[string]string // literal -> placeholder, bound explicitly
	namedOrd     []string          // longest first, so a token never shadows a longer one
	uuids        map[string]int    // literal -> first-appearance number
	digests      map[string]int    // literal -> first-appearance number
	keys         map[string]int    // 32-hex literal -> first-appearance number
	tokens       map[string]int    // 43-character base64url literal -> first-appearance number
	instants     map[string]time.Time
	instantOrder []string // timestamp literals in first-observation order
	ranks        map[int64]int
	frozen       bool
}

// NewBinder returns an empty binder.
func NewBinder() *Binder {
	return &Binder{
		named:    map[string]string{},
		uuids:    map[string]int{},
		digests:  map[string]int{},
		keys:     map[string]int{},
		tokens:   map[string]int{},
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
	text = base64URLRun.ReplaceAllStringFunc(text, func(run string) string {
		payload, ok := DecodeCursor(run)
		if !ok {
			return run
		}
		b.observeIDsAndInstants(payload)
		return " "
	})
	text = tokenRun.ReplaceAllStringFunc(text, func(run string) string {
		if len(run) != tokenLen {
			return run
		}
		if _, seen := b.tokens[run]; !seen {
			b.tokens[run] = len(b.tokens) + 1
		}
		return " "
	})
	for _, run := range hexRun.FindAllString(text, -1) {
		switch len(run) {
		case digestLen:
			if _, seen := b.digests[run]; !seen {
				b.digests[run] = len(b.digests) + 1
			}
		case keyLen:
			if _, seen := b.keys[run]; !seen {
				b.keys[run] = len(b.keys) + 1
			}
		}
	}
	b.observeIDsAndInstants(text)
	return nil
}

func (b *Binder) observeIDsAndInstants(text string) {
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
			b.instantOrder = append(b.instantOrder, literal)
		}
	}
}

// Apply returns text with every bound value replaced. The first call freezes
// the binder: ranks are computed over everything observed so far.
func (b *Binder) Apply(text string) string {
	if !b.frozen {
		b.freeze()
	}
	text = b.replaceNamed(text)
	text = base64URLRun.ReplaceAllStringFunc(text, func(run string) string {
		payload, ok := DecodeCursor(run)
		if !ok {
			return run
		}
		return "<b64u:" + b.applyIDsAndInstants(payload) + ">"
	})
	text = tokenRun.ReplaceAllStringFunc(text, func(run string) string {
		if n, ok := b.tokens[run]; ok {
			return fmt.Sprintf("<token43#%d>", n)
		}
		return run
	})
	text = hexRun.ReplaceAllStringFunc(text, func(run string) string {
		if n, ok := b.digests[run]; ok {
			return fmt.Sprintf("<digest#%d>", n)
		}
		if n, ok := b.keys[run]; ok {
			return fmt.Sprintf("<hex32#%d>", n)
		}
		return run
	})
	return b.applyIDsAndInstants(text)
}

func (b *Binder) applyIDsAndInstants(text string) string {
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

// Mask replaces every id, digest and timestamp in text with its kind (and a
// timestamp's Shape), so texts from different stacks that differ only in those
// values are equal. It orders responses that arrived together; it never
// replaces Apply.
func Mask(text string) string {
	text = MaskCursors(text)
	text = MaskTokens(text)
	text = hexRun.ReplaceAllStringFunc(text, func(run string) string {
		switch {
		case len(run) >= digestLen:
			return "<digest>"
		case len(run) == keyLen:
			return "<hex32>"
		}
		return run
	})
	text = uuid4.ReplaceAllString(text, "<uuid>")
	return timestamp.ReplaceAllStringFunc(text, func(literal string) string {
		return "<ts|" + Shape(literal) + ">"
	})
}

// InstantMark is how many distinct timestamp literals have been observed.
func (b *Binder) InstantMark() int { return len(b.instantOrder) }

// TieInstantsSince gives every timestamp literal first observed after mark
// the rank of the earliest of them. Requests released together finish in an
// order neither stack controls; ranking the instants they wrote against each
// other would compare scheduling, not behaviour. Their spelling is still
// compared through Shape.
func (b *Binder) TieInstantsSince(mark int) error {
	if b.frozen {
		return fmt.Errorf("normalize: TieInstantsSince after Apply")
	}
	if mark < 0 || mark > len(b.instantOrder) {
		return fmt.Errorf("normalize: instant mark %d outside 0..%d", mark, len(b.instantOrder))
	}
	literals := b.instantOrder[mark:]
	if len(literals) == 0 {
		return nil
	}
	earliest := b.instants[literals[0]]
	for _, literal := range literals[1:] {
		if b.instants[literal].Before(earliest) {
			earliest = b.instants[literal]
		}
	}
	for _, literal := range literals {
		b.instants[literal] = earliest
	}
	return nil
}

// Package compare decides whether two normalised HTTP exchanges are the same.
//
// The verdict is byte-based. Status must match; every header except the two
// that legitimately vary per response (date, server) must match in name,
// value and number of occurrences; the body must match byte for byte. Header
// names compare case-insensitively and the order between different names is
// not compared, as HTTP defines them. The one accepted divergence is named in
// AcceptedDivergence. A structural diff is produced for humans, but it never
// softens the verdict.
package compare

import (
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
)

// Volatile headers are the only ones not compared. Keep this list short: every
// entry is a place where a difference cannot be seen.
var Volatile = map[string]bool{"date": true, "server": true}

// Response204ContentLength is ADR-0029 §2.4's RESPONSE-204-CONTENT-LENGTH.
// Python's idempotency replay sends content-length: 0 on a 204, and Go's
// net/http never sends Content-Length on a 204 (RFC 9110 §8.6 forbids it).
// Exactly that pair is accepted: a 204 on both sides, the reference's only
// content-length value "0", the candidate's absent. The other direction, another
// value, a repeated header or another status is still a difference.
const Response204ContentLength = "RESPONSE-204-CONTENT-LENGTH"

// AcceptedDivergence maps each accepted divergence to the scenario that must
// show it on every run, so an exception whose cause went away is noticed
// instead of lingering.
// StaticValidatorValue is ADR-0029 §2.4's STATIC-VALIDATOR-VALUE, for
// MOUNT /static alone.
//
// Starlette derives the ETag from the file's mtime and size, never from its
// bytes: etag = md5(f"{st_mtime}-{st_size}"). Measured across two builds of
// IDENTICAL content, the served etag was 643656382d0b4df1c31428b69735d3ec and
// a31f7470416e1f039dddfaf64f1362c5 while md5(content) stayed
// 397aab04ee043894e776a8a6a2605f7e in both. The value is build metadata: it
// already changes on every image rebuild, so no client can depend on it, and an
// embedded file has no mtime at all. Go hashes the content instead.
//
// ONLY the value is accepted. Both sides must still send both headers, the
// etag must still be a quoted 32-character hex digest on both, and every
// status, every other header and every byte of every body is compared exactly.
// A header dropped, a shape changed or a path outside the mount is a difference.
const StaticValidatorValue = "STATIC-VALIDATOR-VALUE"

var AcceptedDivergence = map[string]string{
	Response204ContentLength: "w0/replay-204",
	StaticValidatorValue:     "static/get-static-files",
}

// staticValidators are the headers StaticValidatorValue covers, and nothing
// else on a /static response is covered.
var staticValidators = []string{"etag", "last-modified"}

var quotedMD5 = regexp.MustCompile(`^"[0-9a-f]{32}"$`)

// Accepted names the accepted divergences a pair of exchanges shows.
func Accepted(reference, candidate Exchange) []string {
	var names []string
	if accepted204(reference, candidate) {
		names = append(names, Response204ContentLength)
	}
	if acceptedStaticValidator(reference, candidate) {
		names = append(names, StaticValidatorValue)
	}
	return names
}

func accepted204(reference, candidate Exchange) bool {
	if reference.Status != 204 || candidate.Status != 204 {
		return false
	}
	ref := lowered(reference.Header)["content-length"]
	_, inCandidate := lowered(candidate.Header)["content-length"]
	return len(ref) == 1 && ref[0] == "0" && !inCandidate
}

// acceptedStaticValidator holds only beneath the mount, only when both sides
// sent both validators, only when the etag keeps its shape on both, and only
// when a value actually differs -- an equal pair has nothing to accept.
func acceptedStaticValidator(reference, candidate Exchange) bool {
	if reference.Path != candidate.Path || !strings.HasPrefix(reference.Path, "/static/") {
		return false
	}
	if reference.Status != candidate.Status {
		return false
	}
	ref, cand := lowered(reference.Header), lowered(candidate.Header)
	differs := false
	for _, name := range staticValidators {
		r, inRef := ref[name]
		c, inCand := cand[name]
		if !inRef && !inCand {
			// A 304 sends the etag and not last-modified, on both sides.
			continue
		}
		if len(r) != 1 || len(c) != 1 {
			return false
		}
		if name == "etag" && (!quotedMD5.MatchString(r[0]) || !quotedMD5.MatchString(c[0])) {
			return false
		}
		if r[0] != c[0] {
			differs = true
		}
	}
	return differs
}

// Exchange is one exchange after normalisation: the path that was asked for,
// and the response that came back. The path is the scenario's declared path,
// bindings and all, and is used only to scope an accepted divergence to the
// route it was decided for -- never to compare.
type Exchange struct {
	Path   string
	Status int
	Header http.Header
	Body   string
}

// Difference is one way two exchanges differ.
type Difference struct {
	Part      string // "status", "header <name>", "body"
	Reference string
	Candidate string
}

func (d Difference) String() string {
	return fmt.Sprintf("%s:\n  reference: %s\n  candidate: %s", d.Part, d.Reference, d.Candidate)
}

// Step returns every difference between reference and candidate; none means
// the step is equal.
func Step(reference, candidate Exchange) []Difference {
	var diffs []Difference
	if reference.Status != candidate.Status {
		diffs = append(diffs, Difference{"status",
			fmt.Sprint(reference.Status), fmt.Sprint(candidate.Status)})
	}
	refHeaders := lowered(reference.Header)
	candHeaders := lowered(candidate.Header)
	skip := map[string]bool{}
	for _, name := range Accepted(reference, candidate) {
		switch name {
		case Response204ContentLength:
			skip["content-length"] = true
		case StaticValidatorValue:
			for _, header := range staticValidators {
				skip[header] = true
			}
		}
	}
	for _, name := range unionNames(refHeaders, candHeaders) {
		if Volatile[name] || skip[name] {
			continue
		}
		ref, inRef := refHeaders[name]
		cand, inCand := candHeaders[name]
		switch {
		case !inRef:
			diffs = append(diffs, Difference{"header " + name, "(absent)", quote(cand)})
		case !inCand:
			diffs = append(diffs, Difference{"header " + name, quote(ref), "(absent)"})
		case strings.Join(ref, "\x00") != strings.Join(cand, "\x00"):
			diffs = append(diffs, Difference{"header " + name, quote(ref), quote(cand)})
		}
	}
	if reference.Body != candidate.Body {
		diffs = append(diffs, bodyDifference(reference.Body, candidate.Body))
	}
	return diffs
}

func lowered(header http.Header) map[string][]string {
	out := make(map[string][]string, len(header))
	for name, values := range header {
		key := strings.ToLower(name)
		out[key] = append(out[key], values...)
	}
	return out
}

func unionNames(a, b map[string][]string) []string {
	seen := map[string]bool{}
	for name := range a {
		seen[name] = true
	}
	for name := range b {
		seen[name] = true
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func quote(values []string) string {
	parts := make([]string, len(values))
	for i, v := range values {
		parts[i] = fmt.Sprintf("%q", v)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// bodyDifference reports the first differing byte with context on both sides,
// which is what a reader needs to see "1.0" vs "1" inside a long body.
func bodyDifference(ref, cand string) Difference {
	limit := len(ref)
	if len(cand) < limit {
		limit = len(cand)
	}
	at := limit
	for i := 0; i < limit; i++ {
		if ref[i] != cand[i] {
			at = i
			break
		}
	}
	return Difference{
		Part:      fmt.Sprintf("body (first difference at byte %d, lengths %d vs %d)", at, len(ref), len(cand)),
		Reference: window(ref, at),
		Candidate: window(cand, at),
	}
}

func window(s string, at int) string {
	start := at - 40
	if start < 0 {
		start = 0
	}
	end := at + 40
	if end > len(s) {
		end = len(s)
	}
	prefix, suffix := "", ""
	if start > 0 {
		prefix = "…"
	}
	if end < len(s) {
		suffix = "…"
	}
	return fmt.Sprintf("%s%q%s", prefix, s[start:end], suffix)
}

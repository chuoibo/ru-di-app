// Package rawprobe sends request lines straight to a socket, bypassing Go's
// URL parser, and compares how two stacks answer them.
//
// Go's net/http parses the request target before any handler runs, so the
// front door answers some malformed request lines differently from uvicorn.
// ADR-0029 §2.4 accepts a named list of those (MALFORMED-REQUEST-LINE). This
// package keeps the list honest in both directions: a request line outside
// the list that starts to differ is a failure, and so is a listed line that no
// longer differs.
package rawprobe

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/url"
	"sort"
	"strings"
	"time"
)

// Case is one raw request line. Target bytes are sent unmodified.
type Case struct {
	Name   string
	Method string
	Target string
}

// Cases is the measured set, in reporting order.
var Cases = []Case{
	{"ok-healthz", "GET", "/healthz"},
	{"bad-escape-suffix", "GET", "/healthz%zz"},
	{"bad-escape-root", "GET", "/%zz"},
	{"bad-escape-route", "GET", "/contexts/%zz"},
	{"truncated-escape", "GET", "/healthz%"},
	{"raw-utf8-target", "GET", "/h\xc3\xa9"},
	{"raw-latin1-target", "GET", "/h\xe9"},
	{"del-byte-target", "GET", "/a\x7fb"},
	{"bad-escape-query", "GET", "/healthz?x=%zz"},
	{"raw-latin1-query", "GET", "/healthz?x=\xe9"},
	{"space-in-query", "GET", "/healthz?x=a b"},
	{"double-slash", "GET", "//healthz"},
	{"fragment", "GET", "/healthz#frag"},
	{"absolute-form", "GET", "http://parity.test/healthz"},
	{"encoded-slash", "GET", "/contexts/a%2Fb"},
	{"encoded-newline", "GET", "/posts%0A"},
	{"trailing-slash", "GET", "/healthz/"},
	{"lowercase-method", "get", "/healthz"},
	{"unknown-method", "FOO", "/healthz"},
	{"connect-method", "CONNECT", "/healthz"},
	{"options-asterisk", "OPTIONS", "*"},
	{"no-leading-slash", "GET", "healthz"},
}

// ExpectedDivergence is ADR-0029 §2.4 MALFORMED-REQUEST-LINE: the cases the
// leader accepted as answered differently by the Go front door.
var ExpectedDivergence = map[string]string{
	"bad-escape-suffix": "Go rejects a bad path escape before routing; uvicorn routes it",
	"bad-escape-root":   "Go rejects a bad path escape before routing; uvicorn routes it",
	"bad-escape-route":  "Go rejects a bad path escape before routing; uvicorn routes it",
	"truncated-escape":  "Go rejects a truncated path escape before routing; uvicorn routes it",
	"raw-utf8-target":   "uvicorn refuses raw non-ASCII target bytes; Go forwards them",
	"raw-latin1-target": "uvicorn refuses raw non-ASCII target bytes; Go forwards them",
	"del-byte-target":   "both refuse; the 400 bodies differ",
	"space-in-query":    "both refuse; the 400 bodies differ",
	"options-asterisk":  "Go answers OPTIONS * itself",
	"no-leading-slash":  "both refuse; the 400 bodies differ",
}

// Answer is a response reduced to what is compared.
type Answer struct {
	Code    string
	Headers map[string][]string
	Body    []byte
}

// volatile headers are framing or per-response, never compared.
var volatile = map[string]bool{"date": true, "server": true, "connection": true, "transfer-encoding": true}

// Ask sends one request line to base and reads the whole answer.
func Ask(ctx context.Context, base string, c Case) (Answer, error) {
	parsed, err := url.Parse(base)
	if err != nil {
		return Answer{}, err
	}
	dialer := net.Dialer{Timeout: 5 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", parsed.Host)
	if err != nil {
		return Answer{}, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	request := c.Method + " " + c.Target + " HTTP/1.1\r\nHost: parity.test\r\nConnection: close\r\n\r\n"
	if _, err := io.WriteString(conn, request); err != nil {
		return Answer{}, err
	}
	raw, err := io.ReadAll(conn)
	if err != nil && len(raw) == 0 {
		return Answer{}, err
	}
	return parse(raw), nil
}

func parse(raw []byte) Answer {
	head, body, _ := bytes.Cut(raw, []byte("\r\n\r\n"))
	scanner := bufio.NewScanner(bytes.NewReader(head))
	answer := Answer{Headers: map[string][]string{}, Body: body}
	first := true
	for scanner.Scan() {
		line := scanner.Text()
		if first {
			first = false
			// Reason phrases differ between servers for the same status; the
			// status code is what is compared.
			fields := strings.Fields(line)
			if len(fields) >= 2 && strings.HasPrefix(fields[0], "HTTP/") {
				answer.Code = fields[1]
			} else {
				answer.Code = "(no status line)"
			}
			continue
		}
		name, value, _ := strings.Cut(line, ":")
		key := strings.ToLower(strings.TrimSpace(name))
		if key == "" || volatile[key] {
			continue
		}
		answer.Headers[key] = append(answer.Headers[key], strings.TrimSpace(value))
	}
	if first {
		answer.Code = "(no response)"
	}
	return answer
}

// Equal reports whether two answers match on status, headers and body.
func Equal(a, b Answer) bool {
	if a.Code != b.Code || !bytes.Equal(a.Body, b.Body) || len(a.Headers) != len(b.Headers) {
		return false
	}
	for name, values := range a.Headers {
		if strings.Join(values, "\x00") != strings.Join(b.Headers[name], "\x00") {
			return false
		}
	}
	return true
}

// Result is one case on both stacks.
type Result struct {
	Case      Case
	Reference Answer
	Candidate Answer
	Equal     bool
}

// Run asks every case of both stacks.
func Run(ctx context.Context, reference, candidate string) ([]Result, error) {
	results := make([]Result, 0, len(Cases))
	for _, c := range Cases {
		ref, err := Ask(ctx, reference, c)
		if err != nil {
			return nil, fmt.Errorf("%s on reference: %w", c.Name, err)
		}
		cand, err := Ask(ctx, candidate, c)
		if err != nil {
			return nil, fmt.Errorf("%s on candidate: %w", c.Name, err)
		}
		results = append(results, Result{Case: c, Reference: ref, Candidate: cand, Equal: Equal(ref, cand)})
	}
	return results, nil
}

// Verdict splits the failures: unexpected are cases outside the accepted list
// that differ; stale are listed cases that no longer differ.
func Verdict(results []Result, expected map[string]string) (unexpected, stale []string) {
	seen := map[string]bool{}
	for _, r := range results {
		seen[r.Case.Name] = true
		_, accepted := expected[r.Case.Name]
		switch {
		case !r.Equal && !accepted:
			unexpected = append(unexpected, r.Case.Name)
		case r.Equal && accepted:
			stale = append(stale, r.Case.Name)
		}
	}
	for name := range expected {
		if !seen[name] {
			stale = append(stale, name)
		}
	}
	sort.Strings(unexpected)
	sort.Strings(stale)
	return unexpected, stale
}

// Package cors is the Go port of the API's cross-origin policy: Starlette
// 0.41's CORSMiddleware as configured by services/api/app/api/cors.py, with
// that module's one change — a successful preflight answers 204 with no body.
//
// It is applied only to routes the Go front door serves. Every rule below was
// checked against the real middleware through testdata/starlette_cors.json,
// rendered by scripts/render_cors_goldens.py inside the API image.
package cors

import (
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// OriginsEnvVar names the comma-separated allowlist.
const OriginsEnvVar = "MOBILE_CORS_ALLOW_ORIGINS"

// loopback is LOOPBACK_ORIGIN_REGEX, applied with Python's fullmatch. `[0-9]`
// rather than `\d`: Starlette sees header bytes decoded as latin-1, where the
// only digits are ASCII, while a Go `\p{Nd}` would accept UTF-8 digits.
var loopback = regexp.MustCompile(`^https?://(localhost|127\.0\.0\.1)(:[0-9]+)?$`)

var (
	allowedHeaders = []string{"authorization", "content-type", "idempotency-key",
		"x-actor-id", "x-actor-roles", "x-actor-contexts"}
	allowedMethods    = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
	safelistedHeaders = []string{"Accept", "Accept-Language", "Content-Language", "Content-Type"}
)

const maxAgeSeconds = 600

// Policy is the configured middleware state.
type Policy struct {
	origins          []string
	useLoopback      bool
	allowAllOrigins  bool
	allowHeaderNames map[string]bool
	preflight        [][2]string // name, value, in Starlette's insertion order
}

// New builds the policy from the raw MOBILE_CORS_ALLOW_ORIGINS value; ok is
// false when the variable is unset.
func New(raw string, set bool) *Policy {
	p := &Policy{allowHeaderNames: map[string]bool{}}
	if set {
		for _, part := range strings.Split(raw, ",") {
			if trimmed := pyStrip(part); trimmed != "" {
				p.origins = append(p.origins, trimmed)
			}
		}
	}
	p.useLoopback = len(p.origins) == 0
	for _, origin := range p.origins {
		if origin == "*" {
			p.allowAllOrigins = true
		}
	}

	if p.allowAllOrigins {
		p.preflight = append(p.preflight, [2]string{"Access-Control-Allow-Origin", "*"})
	} else {
		p.preflight = append(p.preflight, [2]string{"Vary", "Origin"})
	}
	p.preflight = append(p.preflight,
		[2]string{"Access-Control-Allow-Methods", strings.Join(allowedMethods, ", ")},
		[2]string{"Access-Control-Max-Age", strconv.Itoa(maxAgeSeconds)},
	)
	union := map[string]bool{}
	for _, name := range append(append([]string{}, safelistedHeaders...), allowedHeaders...) {
		union[name] = true
	}
	names := make([]string, 0, len(union))
	for name := range union {
		names = append(names, name)
		p.allowHeaderNames[strings.ToLower(name)] = true
	}
	sort.Strings(names)
	p.preflight = append(p.preflight, [2]string{"Access-Control-Allow-Headers", strings.Join(names, ", ")})
	return p
}

func (p *Policy) allowed(origin string) bool {
	if p.allowAllOrigins {
		return true
	}
	if p.useLoopback && loopback.MatchString(origin) {
		return true
	}
	for _, listed := range p.origins {
		if listed == origin {
			return true
		}
	}
	return false
}

// Middleware wraps a handler the front door serves itself.
func (p *Policy) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origins, hasOrigin := r.Header["Origin"]
		if !hasOrigin || len(origins) == 0 {
			next.ServeHTTP(w, r)
			return
		}
		origin := origins[0]
		if _, requested := r.Header["Access-Control-Request-Method"]; r.Method == http.MethodOptions && requested {
			p.preflightResponse(w, r, origin)
			return
		}
		_, hasCookie := r.Header["Cookie"]
		next.ServeHTTP(&simpleWriter{ResponseWriter: w, policy: p, origin: origin, cookie: hasCookie}, r)
	})
}

func (p *Policy) preflightResponse(w http.ResponseWriter, r *http.Request, origin string) {
	header := w.Header()
	for _, pair := range p.preflight {
		header.Set(pair[0], pair[1])
	}
	var failures []string
	if p.allowed(origin) {
		if !p.allowAllOrigins {
			header.Set("Access-Control-Allow-Origin", origin)
		}
	} else {
		failures = append(failures, "origin")
	}
	method := r.Header.Get("Access-Control-Request-Method")
	methodAllowed := false
	for _, allowed := range allowedMethods {
		if allowed == method {
			methodAllowed = true
		}
	}
	if !methodAllowed {
		failures = append(failures, "method")
	}
	if values, ok := r.Header["Access-Control-Request-Headers"]; ok && len(values) > 0 {
		for _, name := range strings.Split(values[0], ",") {
			if !p.allowHeaderNames[pyStrip(strings.ToLower(name))] {
				failures = append(failures, "headers")
				break
			}
		}
	}
	if len(failures) > 0 {
		text := "Disallowed CORS " + strings.Join(failures, ", ")
		header.Set("Content-Length", strconv.Itoa(len(text)))
		header.Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(text))
		return
	}
	// app/api/cors.py: a successful preflight is 204, no body, and no
	// content-type or content-length.
	w.WriteHeader(http.StatusNoContent)
}

// simpleWriter adds the response headers Starlette adds to a non-preflight
// request, at the moment the status line is written.
type simpleWriter struct {
	http.ResponseWriter
	policy *Policy
	origin string
	cookie bool
	done   bool
}

func (w *simpleWriter) apply() {
	if w.done {
		return
	}
	w.done = true
	header := w.Header()
	if w.policy.allowAllOrigins {
		header.Set("Access-Control-Allow-Origin", "*")
		if w.cookie {
			explicitOrigin(header, w.origin)
		}
		return
	}
	if w.policy.allowed(w.origin) {
		explicitOrigin(header, w.origin)
	}
}

// explicitOrigin mirrors MutableHeaders: set replaces every value, and
// add_vary_header joins onto the FIRST existing Vary value only.
func explicitOrigin(header http.Header, origin string) {
	header.Set("Access-Control-Allow-Origin", origin)
	if existing := header.Values("Vary"); len(existing) > 0 {
		header.Set("Vary", existing[0]+", Origin")
		return
	}
	header.Set("Vary", "Origin")
}

func (w *simpleWriter) WriteHeader(status int) {
	w.apply()
	w.ResponseWriter.WriteHeader(status)
}

func (w *simpleWriter) Write(b []byte) (int, error) {
	w.apply()
	return w.ResponseWriter.Write(b)
}

func (w *simpleWriter) Flush() {
	w.apply()
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// pyStrip is Python's str.strip() for text that arrived as latin-1 bytes: the
// whitespace it removes includes \x1c-\x1f, NEL (0x85) and NBSP (0xa0), which
// strings.TrimSpace does not treat the same way.
func pyStrip(s string) string {
	isSpace := func(b byte) bool {
		switch b {
		case '\t', '\n', '\v', '\f', '\r', ' ', 0x1c, 0x1d, 0x1e, 0x1f, 0x85, 0xa0:
			return true
		}
		return false
	}
	start, end := 0, len(s)
	for start < end && isSpace(s[start]) {
		start++
	}
	for end > start && isSpace(s[end-1]) {
		end--
	}
	return s[start:end]
}

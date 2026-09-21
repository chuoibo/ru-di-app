// Package router answers, for one HTTP request, what the Python API's
// Starlette router would do with it: serve a route (FULL), refuse the method
// (405 with Allow), redirect to the other trailing-slash spelling (307), or
// answer 404. The front door serves a request in Go only when it is a FULL
// match on a Go-owned route, so FULL matching and registration order are the
// load-bearing parts. The other kinds exist so the goldens can show the
// emulation is complete, not merely right about the easy half.
//
// Every rule below was read from the pinned API image (fastapi 0.115.6,
// starlette 0.41.3, uvicorn 0.34.0 on httptools 0.8.0) and is pinned by
// testdata/starlette_decisions.json, which scripts/render_router_goldens.py
// renders by sending raw requests through those libraries:
//
//   - The scope path is the request target's path, percent-decoded once by
//     urllib.parse.unquote when it contains "%". Invalid escapes stay literal
//     and invalid UTF-8 becomes U+FFFD per maximal subpart. "/contexts/a%2Fb"
//     is therefore matched as "/contexts/a/b"; "//" is kept as is.
//   - llhttp answers 400 before the app for any byte outside 0x21-0x7E in the
//     target and for a method missing from its HTTP method table.
//   - Routes are tried in registration order. The first FULL (path and method)
//     wins, even after earlier PARTIAL (path only) matches; with no FULL, the
//     first PARTIAL answers 405.
//   - "{name}" compiles to [^/]+, and the pattern ends in Python's "$", which
//     also matches before one trailing newline: "/posts%0A" is GET /posts.
//     Types are validated after routing, so a non-UUID segment still matches.
//   - FastAPI's APIRoute never adds HEAD. Starlette's own Route, used for the
//     docs pages, adds HEAD wherever GET is allowed. A mount matches its
//     prefix plus "/" for every method.
//   - With no match and a path other than "/", the router tries the other
//     trailing-slash spelling (all trailing slashes stripped, or one added).
//     If that matches anything, FULL or PARTIAL, the answer is 307 to
//     scheme://Host + that path + "?" + raw query, through urllib's quote.
//
// Deliberately not emulated, so none of these can ever make Go serve a
// request:
//
//   - Middleware that answers before the router: the CORS preflight (OPTIONS
//     with Origin and Access-Control-Request-Method) and the idempotency
//     layer's own refusals.
//   - Targets that are not origin-form or "*" (KindUnemulated).
//   - A Host-less request: uvicorn then builds Location from the API's own
//     socket address, which only Python knows, so Location stays relative.
//   - Allow order for a route with several methods: Python joins a set, so
//     the order follows PYTHONHASHSEED ("GET, HEAD" under seeds 0-2, "HEAD,
//     GET" under 3-7). Go always sends them sorted.
package router

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"mobile/services/core/ownership"
)

// Decision kinds.
const (
	KindFull             = "full"
	KindMethodNotAllowed = "method_not_allowed"
	KindRedirect         = "redirect"
	KindNotFound         = "not_found"
	// KindBadRequest is uvicorn's own 400, sent before the app sees anything.
	KindBadRequest = "bad_request"
	// KindUnemulated marks a target shape this package does not model.
	KindUnemulated = "unemulated"
)

// Decision is what Starlette's router would do with one request.
type Decision struct {
	Kind     string
	RouteID  string            // manifest id, for KindFull
	Params   map[string]string // decoded path params for KindFull; nil when none
	Allow    string            // Allow header value, for KindMethodNotAllowed
	Location string            // Location header value, for KindRedirect
}

// Match is a FULL match on one manifest row.
type Match struct {
	Route  ownership.Route
	Params map[string]string
}

// starletteRoutes are the routes FastAPI registers through Starlette's Route
// (add_route) instead of APIRoute. Starlette adds HEAD next to GET; APIRoute
// does not. The manifest keeps one method per row, so the difference lives
// here and the goldens pin both behaviours.
var starletteRoutes = map[string]bool{
	"GET /openapi.json":         true,
	"GET /docs":                 true,
	"GET /docs/oauth2-redirect": true,
	"GET /redoc":                true,
}

// parserMethods is llhttp's HTTP method table as uvicorn 0.34.0 on httptools
// 0.8.0 lets it through on an origin-form request line; RTSP-only methods and
// lowercase spellings get a 400. CONNECT is in: llhttp treats it as an upgrade
// and raises HttpParserUpgrade after the headers, which uvicorn ignores for a
// non-websocket request, so the request is routed like any other.
var parserMethods = map[string]bool{
	"CONNECT": true,
	"DELETE":  true, "GET": true, "HEAD": true, "POST": true, "PUT": true,
	"OPTIONS": true, "TRACE": true, "COPY": true, "LOCK": true, "MKCOL": true,
	"MOVE": true, "PROPFIND": true, "PROPPATCH": true, "SEARCH": true,
	"UNLOCK": true, "BIND": true, "REBIND": true, "UNBIND": true, "ACL": true,
	"REPORT": true, "MKACTIVITY": true, "CHECKOUT": true, "MERGE": true,
	"M-SEARCH": true, "NOTIFY": true, "SUBSCRIBE": true, "UNSUBSCRIBE": true,
	"PATCH": true, "PURGE": true, "MKCALENDAR": true, "LINK": true,
	"UNLINK": true, "SOURCE": true, "QUERY": true,
}

// Starlette's PARAM_REGEX.
var paramPattern = regexp.MustCompile(`\{([a-zA-Z_][a-zA-Z0-9_]*)(:[a-zA-Z_][a-zA-Z0-9_]*)?\}`)

// Only convertors whose converted value is the matched string are emulated:
// int, float and uuid hand the handler a different value than the segment.
var convertors = map[string]string{
	"str":  `[^/]+`,
	"path": `.*`,
}

type compiled struct {
	row     ownership.Route
	mount   bool
	methods map[string]bool
	allow   string
	pattern *regexp.Regexp
	names   []string
}

// Router holds the manifest's routes compiled in registration order.
type Router struct {
	routes []compiled
}

// New compiles routes, which must be in registration order.
func New(routes []ownership.Route) (*Router, error) {
	r := &Router{routes: make([]compiled, 0, len(routes))}
	for index, row := range routes {
		c := compiled{row: row}
		template := row.Path
		switch row.Kind {
		case "route":
			if !strings.HasPrefix(row.Path, "/") {
				return nil, fmt.Errorf("routes[%d] %q: routed paths must start with '/'", index, row.ID)
			}
			c.methods = map[string]bool{strings.ToUpper(row.Method): true}
			if starletteRoutes[row.ID] && c.methods["GET"] {
				c.methods["HEAD"] = true
			}
			c.allow = sortedJoin(c.methods)
		case "mount":
			if row.Path != "" && !strings.HasPrefix(row.Path, "/") {
				return nil, fmt.Errorf("routes[%d] %q: routed paths must start with '/'", index, row.ID)
			}
			c.mount = true
			// Mount.__init__: path.rstrip("/") + "/{path:path}".
			template = strings.TrimRight(row.Path, "/") + "/{path:path}"
		default:
			return nil, fmt.Errorf("routes[%d] %q: kind %q is not routable", index, row.ID, row.Kind)
		}
		pattern, names, err := compilePath(template)
		if err != nil {
			return nil, fmt.Errorf("routes[%d] %q: %w", index, row.ID, err)
		}
		c.pattern, c.names = pattern, names
		r.routes = append(r.routes, c)
	}
	return r, nil
}

// compilePath mirrors starlette.routing.compile_path for "/"-rooted paths.
func compilePath(path string) (*regexp.Regexp, []string, error) {
	var expr strings.Builder
	expr.WriteString("^")
	var names []string
	seen := map[string]bool{}
	idx := 0
	for _, loc := range paramPattern.FindAllStringSubmatchIndex(path, -1) {
		name := path[loc[2]:loc[3]]
		convertor := "str"
		if loc[4] >= 0 {
			convertor = strings.TrimLeft(path[loc[4]:loc[5]], ":")
		}
		regex, ok := convertors[convertor]
		if !ok {
			return nil, nil, fmt.Errorf("path %q: convertor %q is not emulated", path, convertor)
		}
		if seen[name] {
			return nil, nil, fmt.Errorf("path %q: duplicated param name %s", path, name)
		}
		seen[name] = true
		expr.WriteString(regexp.QuoteMeta(path[idx:loc[0]]))
		expr.WriteString("(" + regex + ")")
		names = append(names, name)
		idx = loc[1]
	}
	expr.WriteString(regexp.QuoteMeta(path[idx:]))
	// Python's "$" without MULTILINE matches at the end or before a final
	// newline. Consuming that newline keeps every capture identical, because
	// RE2 picks the submatches a backtracking engine would.
	expr.WriteString(`\n?\z`)
	pattern, err := regexp.Compile(expr.String())
	if err != nil {
		return nil, nil, fmt.Errorf("path %q: %w", path, err)
	}
	return pattern, names, nil
}

// match is Route.matches / Mount.matches minus the method check.
func (c *compiled) match(path string) (map[string]string, bool) {
	sub := c.pattern.FindStringSubmatch(path)
	if sub == nil {
		return nil, false
	}
	var params map[string]string
	for i, name := range c.names {
		if c.mount && name == "path" {
			// Mount pops it into the child scope's root_path.
			continue
		}
		if params == nil {
			params = make(map[string]string, len(c.names))
		}
		params[name] = sub[i+1]
	}
	return params, true
}

func (c *compiled) full(method string) bool {
	return c.mount || c.methods[method]
}

// FirstFull returns the first route that FULL-matches method and a decoded
// scope path (see ScopePath). This is the only question the front door needs
// answered: anything else goes to Python. It does not apply llhttp's
// refusals, so raw requests should go through DecideTarget first.
func (r *Router) FirstFull(method, path string) (Match, bool) {
	for i := range r.routes {
		c := &r.routes[i]
		if params, ok := c.match(path); ok && c.full(method) {
			return Match{Route: c.row, Params: params}, true
		}
	}
	return Match{}, false
}

// DecideTarget decides for a request-target exactly as it appeared on the
// request line (http.Request.RequestURI). host is the Host header ("" for
// none) and scheme the X-Forwarded-Proto the front door sends to Python.
func (r *Router) DecideTarget(method, target, host, scheme string) Decision {
	if !parserMethods[method] || !targetBytesOK(target) {
		return Decision{Kind: KindBadRequest}
	}
	if target != "*" && !strings.HasPrefix(target, "/") {
		return Decision{Kind: KindUnemulated}
	}
	rawPath, rawQuery := SplitTarget(target)
	return r.Decide(method, rawPath, rawQuery, host, scheme)
}

// Decide is DecideTarget for a target already split into its raw, still
// percent-encoded path and query, without the fragment.
func (r *Router) Decide(method, rawPath, rawQuery, host, scheme string) Decision {
	if !parserMethods[method] || !targetBytesOK(rawPath) || !targetBytesOK(rawQuery) {
		return Decision{Kind: KindBadRequest}
	}
	if strings.ContainsAny(rawPath, "?#") || strings.Contains(rawQuery, "#") ||
		(rawPath != "*" && !strings.HasPrefix(rawPath, "/")) {
		return Decision{Kind: KindUnemulated}
	}
	path := ScopePath(rawPath)

	// Router.app: the first FULL wins; remember the first PARTIAL.
	var partial *compiled
	for i := range r.routes {
		c := &r.routes[i]
		params, ok := c.match(path)
		if !ok {
			continue
		}
		if c.full(method) {
			return Decision{Kind: KindFull, RouteID: c.row.ID, Params: params}
		}
		if partial == nil {
			partial = c
		}
	}
	if partial != nil {
		return Decision{Kind: KindMethodNotAllowed, Allow: partial.allow}
	}

	// redirect_slashes: any match, FULL or PARTIAL, on the other spelling.
	if path != "/" {
		toggled := path + "/"
		if strings.HasSuffix(path, "/") {
			toggled = strings.TrimRight(path, "/")
		}
		for i := range r.routes {
			if _, ok := r.routes[i].match(toggled); ok {
				return Decision{Kind: KindRedirect, Location: location(scheme, host, toggled, rawQuery)}
			}
		}
	}
	return Decision{Kind: KindNotFound}
}

// location is str(URL(scope=redirect_scope)) through RedirectResponse's quote.
func location(scheme, host, path, rawQuery string) string {
	switch scheme {
	case "http", "https", "ws", "wss":
		// ProxyHeadersMiddleware takes exactly these from X-Forwarded-Proto.
	default:
		scheme = "http"
	}
	url := path
	if host != "" {
		// The Host header is decoded as latin-1 before being quoted as UTF-8.
		url = scheme + "://" + latin1(host) + path
	}
	if rawQuery != "" {
		url += "?" + rawQuery
	}
	return quote(url)
}

func sortedJoin(set map[string]bool) string {
	out := make([]string, 0, len(set))
	for method := range set {
		out = append(out, method)
	}
	sort.Strings(out)
	return strings.Join(out, ", ")
}

package pyval

import (
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"mobile/services/core/contract"
	"mobile/services/core/internal/httpapi/problem"
	"mobile/services/core/internal/pyjson"
)

// Contract is the parsed IR of every router group.
type Contract struct {
	routes map[string]*routeIR
	order  []string
}

type routeIR struct {
	id    string
	group string
	node  *pyjson.OrderedMap
	defs  map[string]*pyjson.OrderedMap
}

// Load parses the IR embedded in package contract.
func Load() (*Contract, error) {
	files := map[string][]byte{}
	entries, err := fs.Glob(contract.IR, "ir/*.json")
	if err != nil {
		return nil, err
	}
	for _, name := range entries {
		data, err := contract.IR.ReadFile(name)
		if err != nil {
			return nil, err
		}
		files[name] = data
	}
	return Parse(files)
}

// Parse builds a Contract from IR file contents keyed by file name.
func Parse(files map[string][]byte) (*Contract, error) {
	c := &Contract{routes: map[string]*routeIR{}}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		v, err := pyjson.Loads(files[name])
		if err != nil {
			return nil, fmt.Errorf("pyval: %s: %w", name, err)
		}
		doc, ok := v.(*pyjson.OrderedMap)
		if !ok || irInt(doc, "ir_version", 0) != 1 {
			return nil, fmt.Errorf("pyval: %s: not IR version 1", name)
		}
		defs := map[string]*pyjson.OrderedMap{}
		if d, ok := doc.Get("definitions"); ok {
			for k, def := range d.(*pyjson.OrderedMap).All() {
				defs[k] = def.(*pyjson.OrderedMap)
			}
		}
		routes, _ := mustGet(doc, "routes").(pyjson.List)
		for _, raw := range routes {
			node := raw.(*pyjson.OrderedMap)
			id := irString(node, "id")
			if _, dup := c.routes[id]; dup {
				return nil, fmt.Errorf("pyval: route %s appears twice", id)
			}
			c.routes[id] = &routeIR{id: id, group: irString(doc, "group"), node: node, defs: defs}
			c.order = append(c.order, id)
		}
	}
	return c, nil
}

// RouteIDs lists "METHOD /path" for every route, sorted.
func (c *Contract) RouteIDs() []string {
	out := append([]string(nil), c.order...)
	sort.Strings(out)
	return out
}

// Report is what binding a route found.
type Report struct {
	// Validators names every Python validator function the route's schemas call.
	Validators []string
	// Unregistered is the subset of Validators without a Go port in the registry.
	Unregistered []string
	// Unsupported lists IR features pyval does not implement.
	Unsupported []string
}

// Inspect compiles a route and reports what it needs without failing.
func (c *Contract) Inspect(id string, reg *Registry) (Report, error) {
	_, rep, err := c.compile(id, reg)
	return rep, err
}

// Bind compiles a route for validation. It fails, naming every cause, when
// the route calls a validator function reg has no port for or uses an IR
// feature pyval does not implement: serving such a route from Go would
// accept or refuse requests Python treats differently.
func (c *Contract) Bind(id string, reg *Registry) (*Route, error) {
	r, rep, err := c.compile(id, reg)
	if err != nil {
		return nil, err
	}
	if len(rep.Unregistered) > 0 || len(rep.Unsupported) > 0 {
		var parts []string
		if len(rep.Unregistered) > 0 {
			parts = append(parts, "validators without a Go port: "+strings.Join(rep.Unregistered, ", "))
		}
		if len(rep.Unsupported) > 0 {
			parts = append(parts, "unsupported: "+strings.Join(rep.Unsupported, "; "))
		}
		return nil, fmt.Errorf("pyval: cannot bind %s: %s", id, strings.Join(parts, "; "))
	}
	return r, nil
}

func (c *Contract) compile(id string, reg *Registry) (*Route, Report, error) {
	ir, ok := c.routes[id]
	if !ok {
		return nil, Report{}, fmt.Errorf("pyval: no route %s in the contract", id)
	}
	comp := newCompiler(ir.defs, reg)
	r := &Route{
		ID:       id,
		Method:   irString(ir.node, "method"),
		Path:     irString(ir.node, "path"),
		Endpoint: irString(ir.node, "endpoint"),
	}
	if convs, ok := ir.node.Get("path_convertors"); ok {
		for k, v := range convs.(*pyjson.OrderedMap).All() {
			if s := string(v.(pyjson.String)); s != "StringConvertor" && s != "PathConvertor" {
				comp.unsupportedf("path convertor %s for %s", s, k)
			}
		}
	}
	if b, ok := ir.node.Get("body"); ok {
		if body, ok := b.(*pyjson.OrderedMap); ok {
			r.hasBody = true
			r.embed = irBool(body, "embed")
			if kind := irString(body, "kind"); kind != "json" {
				comp.unsupportedf("%s request body", kind)
			}
		}
	}
	r.root = comp.dependant(mustGet(ir.node, "dependant").(*pyjson.OrderedMap))
	rep := Report{
		Validators:   sortedKeys(comp.functions),
		Unregistered: sortedKeys(comp.missing),
		Unsupported:  sortedKeys(comp.unsupported),
	}
	return r, rep, nil
}

type param struct {
	name     string
	alias    string
	in       string
	required bool
	sequence bool
	def      Value
	factory  func() Value
	v        validator
}

func (p *param) defaultValue() Value {
	if p.factory != nil {
		return p.factory()
	}
	return copyValue(p.def)
}

type dependant struct {
	call     string
	name     string
	useCache bool
	cacheKey string
	params   map[string][]*param
	deps     []*dependant
}

func (c *compiler) dependant(n *pyjson.OrderedMap) *dependant {
	d := &dependant{
		call:     irString(n, "call"),
		name:     irString(n, "name"),
		useCache: irBool(n, "use_cache"),
		params:   map[string][]*param{},
	}
	if key, ok := n.Get("cache_key"); ok {
		if enc, err := pyjson.Compact(key); err == nil {
			d.cacheKey = string(enc)
		}
	}
	for _, in := range []string{"path", "query", "header", "cookie", "body"} {
		list, ok := n.Get(in)
		if !ok {
			continue
		}
		if in == "cookie" {
			c.unsupportedf("cookie parameters")
		}
		for _, raw := range list.(pyjson.List) {
			d.params[in] = append(d.params[in], c.param(raw.(*pyjson.OrderedMap), in))
		}
	}
	if deps, ok := n.Get("dependencies"); ok {
		for _, raw := range deps.(pyjson.List) {
			d.deps = append(d.deps, c.dependant(raw.(*pyjson.OrderedMap)))
		}
	}
	return d
}

func (c *compiler) param(n *pyjson.OrderedMap, in string) *param {
	p := &param{
		name:     irString(n, "name"),
		alias:    irString(n, "alias"),
		in:       in,
		required: irBool(n, "required"),
		sequence: irBool(n, "sequence"),
	}
	if in != "body" && in != "path" && !irBool(n, "scalar") && !p.sequence {
		c.unsupportedf("%s parameter %s declared as a model", in, p.name)
	}
	if raw, ok := n.Get("default"); ok {
		v, err := irValue(raw)
		if err != nil {
			c.unsupportedf("default of %s: %v", p.name, err)
		}
		p.def = v
	}
	if f, ok := n.Get("default_factory"); ok {
		switch string(f.(pyjson.String)) {
		case "builtins.list":
			p.factory = func() Value { return List{} }
		default:
			c.unsupportedf("default_factory %v of %s", f, p.name)
		}
	}
	if !p.required && p.def == nil && p.factory == nil {
		p.def = pyjson.Null{}
	}
	p.v = c.compile(mustGet(n, "schema"), config{}, in+" "+p.name)
	return p
}

// Route validates requests for one APIRoute.
type Route struct {
	ID       string
	Method   string
	Path     string
	Endpoint string

	hasBody bool
	embed   bool
	root    *dependant
}

// Request is what validation reads from an HTTP request.
type Request struct {
	// PathParams are the decoded path parameters the router matched.
	PathParams map[string]string
	// RawQuery is the query string exactly as received, without "?".
	RawQuery string
	// Headers are the header fields in wire order, names in any case,
	// values as received.
	Headers [][2]string
	// Body is the complete request body.
	Body []byte
}

func (r *Request) header(name string) (string, bool) {
	for _, h := range r.Headers {
		if asciiLower(h[0]) == name {
			return h[1], true
		}
	}
	return "", false
}

// Result is a validated request.
type Result struct {
	// Errors, when not empty, is the 422 FastAPI answers
	// (problem.WriteValidation), in FastAPI's order.
	Errors []problem.ValidationError
	// Values holds the endpoint's own parameters by Python name, set when
	// Errors is empty.
	Values map[string]Value
}

// Dependency is one dependency call solve_dependencies makes.
type Dependency struct {
	// Name is the parameter the result is bound to, e.g. "actor".
	Name string
	// Call is the qualified Python function, e.g. app.api.deps.get_actor.
	Call string
	// Values are the dependency's own validated parameters by name.
	Values map[string]Value
}

// Hook runs a dependency. A non-nil error stops validation and is returned
// from Validate unchanged, as an exception raised inside a dependency
// propagates out of FastAPI before any later parameter is validated.
type Hook func(Dependency) error

// BodyError is the HTTPException(400, "There was an error parsing the body")
// get_request_handler raises when json.loads fails with anything other than
// JSONDecodeError: undecodable bytes, an int beyond 4300 digits, nesting
// beyond the recursion limit.
type BodyError struct{ Err error }

func (e *BodyError) Error() string { return "pyval: error parsing the body: " + e.Err.Error() }

func (e *BodyError) Unwrap() error { return e.Err }

// WriteBodyError answers a BodyError as FastAPI's http_exception_handler does.
func WriteBodyError(w http.ResponseWriter) error {
	body := pyjson.NewOrderedMap()
	body.Set("detail", pyjson.String("There was an error parsing the body"))
	encoded, err := pyjson.Compact(body)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(encoded)))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	_, err = w.Write(encoded)
	return err
}

// Validate runs FastAPI's request handling up to the endpoint call, in its
// order (fastapi/routing.py get_request_handler, dependencies/utils.py
// solve_dependencies):
//
//  1. When the route has a body parameter and the body is not empty, decode
//     it as JSON if content-type is missing, empty, application/json or
//     application/*+json; otherwise keep the bytes. A JSONDecodeError is a
//     422 json_invalid at once, before any dependency; any other decoding
//     failure is *BodyError.
//  2. Sub-dependencies depth first: each one's own parameters are validated
//     and, when they produced no error, hook runs it unless an earlier call
//     with the same cache key already did. Errors accumulate; a hook error
//     returns at once.
//  3. The endpoint's path, query, header, cookie and then body parameters.
func (r *Route) Validate(req *Request, hook Hook) (*Result, error) {
	var body any
	if r.hasBody && len(req.Body) > 0 {
		ct, present := req.header("content-type")
		if !present || ct == "" || contentTypeIsJSON(ct) {
			v, err := pyjson.Loads(req.Body)
			if err != nil {
				var de *pyjson.DecodeError
				if errors.As(err, &de) {
					return &Result{Errors: []problem.ValidationError{{
						Type: "json_invalid",
						Loc:  pyjson.List{pyjson.String("body"), pyjson.NewInt(int64(de.Pos))},
						Msg:  "JSON decode error",
						Ctx:  ctxOf("error", pyjson.String(de.Msg)),
					}}}, nil
				}
				return nil, &BodyError{Err: err}
			}
			body = v
		} else {
			body = Bytes(req.Body)
		}
	}
	s := &solver{route: r, req: req, hook: hook, body: body, cache: map[string]bool{}}
	values, lines, err := s.solve(r.root)
	if err != nil {
		return nil, err
	}
	if len(lines) > 0 {
		out := make([]problem.ValidationError, len(lines))
		for i, l := range lines {
			loc := make(pyjson.List, len(l.loc))
			for j, item := range l.loc {
				switch x := item.(type) {
				case string:
					loc[j] = pyjson.String(x)
				case int:
					loc[j] = pyjson.NewInt(int64(x))
				}
			}
			out[i] = problem.ValidationError{Type: l.typ, Loc: loc, Msg: l.msg, Ctx: l.ctx}
		}
		return &Result{Errors: out}, nil
	}
	return &Result{Values: values}, nil
}

type solver struct {
	route       *Route
	req         *Request
	hook        Hook
	body        any
	cache       map[string]bool
	query       [][2]string
	parsedQuery bool
}

func (s *solver) solve(d *dependant) (map[string]Value, []lineError, error) {
	values := map[string]Value{}
	var lines []lineError
	for _, sub := range d.deps {
		subValues, subLines, err := s.solve(sub)
		if err != nil {
			return nil, nil, err
		}
		if len(subLines) > 0 {
			lines = append(lines, subLines...)
			continue
		}
		if sub.useCache && s.cache[sub.cacheKey] {
			continue
		}
		if s.hook != nil {
			if err := s.hook(Dependency{Name: sub.name, Call: sub.call, Values: subValues}); err != nil {
				return nil, nil, err
			}
		}
		s.cache[sub.cacheKey] = true
	}
	for _, in := range []string{"path", "query", "header", "cookie"} {
		for _, p := range d.params[in] {
			raw, present := s.lookup(p)
			v, e := s.validateValue(p, raw, present, true, []any{p.in, p.alias})
			if e != nil {
				if e.fatal != nil {
					return nil, nil, e.fatal
				}
				lines = append(lines, e.lines...)
				continue
			}
			values[p.name] = v
		}
	}
	if bodyParams := d.params["body"]; len(bodyParams) > 0 {
		if len(bodyParams) == 1 && !s.route.embed {
			p := bodyParams[0]
			v, e := s.validateValue(p, s.body, !isNone(s.body), false, []any{"body"})
			if e != nil {
				if e.fatal != nil {
					return nil, nil, e.fatal
				}
				lines = append(lines, e.lines...)
			} else {
				values[p.name] = v
			}
		} else {
			for _, p := range bodyParams {
				var raw any
				present := false
				if s.body != nil {
					m, ok := s.body.(*pyjson.OrderedMap)
					if !ok {
						// `body_to_process.get(alias)` on a non-dict raises
						// AttributeError, which FastAPI reports as missing.
						lines = append(lines, errMissing().withOuter(p.alias).withOuter("body").lines...)
						continue
					}
					raw, present = m.Get(p.alias)
				}
				v, e := s.validateValue(p, raw, present && !isNone(raw), false, []any{"body", p.alias})
				if e != nil {
					if e.fatal != nil {
						return nil, nil, e.fatal
					}
					lines = append(lines, e.lines...)
					continue
				}
				values[p.name] = v
			}
		}
	}
	return values, lines, nil
}

// lookup is _get_multidict_value up to its missing branch: path params from
// the router, the last query value (or all of them for a sequence), the
// first header value (or all of them).
func (s *solver) lookup(p *param) (any, bool) {
	switch p.in {
	case "path":
		v, ok := s.req.PathParams[p.alias]
		return pyjson.String(v), ok
	case "query":
		if !s.parsedQuery {
			s.query, s.parsedQuery = parseQSL(s.req.RawQuery), true
		}
		var all pyjson.List
		for _, kv := range s.query {
			if kv[0] == p.alias {
				all = append(all, pyjson.String(kv[1]))
			}
		}
		if p.sequence {
			return all, len(all) > 0
		}
		if len(all) == 0 {
			return nil, false
		}
		return all[len(all)-1], true
	case "header":
		name := asciiLower(p.alias)
		var all pyjson.List
		for _, h := range s.req.Headers {
			if asciiLower(h[0]) == name {
				all = append(all, pyjson.String(latin1(h[1])))
			}
		}
		if p.sequence {
			return all, len(all) > 0
		}
		if len(all) == 0 {
			return nil, false
		}
		return all[0], true
	}
	return nil, false
}

// validateValue is _validate_value_with_model_field: a None value is missing
// or a copy of the default, anything else is validated with loc prefixed.
// For path, query and header values (multidict) it first applies
// _get_multidict_value's branch, which turns an absent optional value into
// the default and so gets a non-None default validated; body values skip it.
func (s *solver) validateValue(p *param, raw any, present, multidict bool, loc []any) (Value, *vErr) {
	value := raw
	if !present {
		value = nil
		if multidict && !p.required {
			value = p.defaultValue()
		}
	}
	if isNone(value) {
		if p.required {
			e := errMissing()
			for i := len(loc) - 1; i >= 0; i-- {
				e.withOuter(loc[i])
			}
			return nil, e
		}
		return p.defaultValue(), nil
	}
	v, e := p.v.validate(newState(), value)
	if e != nil {
		for i := len(loc) - 1; i >= 0; i-- {
			e.withOuter(loc[i])
		}
		return nil, e
	}
	return v, nil
}

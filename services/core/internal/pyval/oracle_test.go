//go:build oracle

package pyval

// Differential test against the Python parity image. Run from services/core:
//
//	go test -tags oracle -run TestOracle -v -timeout 60m ./internal/pyval/
//
// Every case is one HTTP request. All of them travel to one
// `docker run --network none` that drives create_app() over raw ASGI in dev
// auth mode. There, every endpoint is replaced by a stub that answers 299
// with the values FastAPI passed it, and every dependency except get_actor /
// get_actor_optional by a no-op, so a request that passes validation is
// observed as "accepted" together with its decoded values instead of
// failing on the database. The Go side runs Route.Validate with a hook that
// mirrors get_actor in dev mode, renders the same outcome, and the two are
// compared: status, content-type and body bytes for a refusal, the value
// tree for an acceptance.
//
// Environment:
//
//	PYVAL_ORACLE_IMAGE  image (default mobile-parity-api:7bf58e3d, image ID
//	                    181a7b7f6fd8, the same image as the d4f2108f-* parity tag)
//	PYVAL_ORACLE_SEED   generator seed (default 1)
//	PYVAL_ORACLE_SCOPE  "w1" for the pilot routes only, "all" (default)
//	PYVAL_ORACLE_CACHE  file caching Python's answers for this exact case
//	                    list, so a mutated Go tree can be re-checked in seconds
//
// Byte strings travel as latin-1 JSON strings (one code point per byte),
// value trees carry str as the latin-1 image of its surrogatepass UTF-8.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"math/rand/v2"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"regexp/syntax"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"mobile/services/core/internal/httpapi/problem"
	"mobile/services/core/internal/pyjson"
)

const oracleDriver = `
import asyncio, datetime as dt, json, os, sys, uuid
os.environ["MOBILE_AUTH_MODE"] = "dev"
os.environ["MOBILE_DATABASE_URL"] = "postgresql+psycopg://nobody:nothing@127.0.0.1:9/nothing"
sys.path.insert(0, "/srv")
from pydantic import BaseModel
from fastapi.routing import APIRoute
from starlette.datastructures import UploadFile
from starlette.responses import Response
from app.api.main import create_app

AUTH = {"app.api.deps.get_actor", "app.api.deps.get_actor_optional"}

def qual(f):
    return f"{f.__module__}.{f.__qualname__}"

def lat(b):
    return b.decode("latin-1")

def tree(v):
    if v is None: return ["n"]
    if v is True or v is False: return ["b", v]
    if type(v) is int: return ["i", str(v)]
    if type(v) is float: return ["f", repr(v)]
    if type(v) is str: return ["s", lat(v.encode("utf-8", "surrogatepass"))]
    if type(v) is bytes: return ["y", lat(v)]
    if isinstance(v, uuid.UUID): return ["u", str(v)]
    if isinstance(v, dt.datetime): return ["dt", v.isoformat()]
    if isinstance(v, dt.date): return ["date", v.isoformat()]
    if isinstance(v, UploadFile):
        v.file.seek(0)
        return ["file", lat(v.filename.encode("utf-8", "surrogatepass")), lat(v.file.read()), [[lat(k), lat(x)] for k, x in v.headers.raw], v.size]
    if isinstance(v, BaseModel):
        names = list(type(v).model_fields)
        return ["m", qual(type(v)), [[k, tree(getattr(v, k))] for k in names], [k for k in names if k in v.model_fields_set]]
    if type(v) is list: return ["l", [tree(x) for x in v]]
    if type(v) is dict: return ["d", [[tree(k), tree(x)] for k, x in v.items()]]
    return ["?", type(v).__name__]

app = create_app()
#@extra-routes
# IdempotencyMiddleware answers every request that carries Idempotency-Key
# before routing, and needs the database to do it. What this oracle measures
# is how that header validates, so the layer is left out; no other case
# sends the header.
app.user_middleware = [m for m in app.user_middleware if m.cls.__name__ != "IdempotencyMiddleware"]

def own_names(d):
    return [f.name for f in d.path_params + d.query_params + d.header_params + d.cookie_params + d.body_params]

def stub_dependencies(d):
    for sub in d.dependencies:
        if qual(sub.call) not in AUTH:
            sub.call = lambda **kw: None
        stub_dependencies(sub)

def make_endpoint(names, is_async):
    def answer(values):
        body = json.dumps({"accepted": [[k, tree(values[k])] for k in names if k in values]}, ensure_ascii=True)
        return Response(body, status_code=299, media_type="application/json")
    if is_async:
        async def endpoint(**values):
            return answer(values)
    else:
        def endpoint(**values):
            return answer(values)
    return endpoint

for r in app.routes:
    if isinstance(r, APIRoute):
        r.dependant.call = make_endpoint(own_names(r.dependant), asyncio.iscoroutinefunction(r.endpoint))
        stub_dependencies(r.dependant)

async def drive(req):
    path = req["path"]
    scope = {
        "type": "http", "asgi": {"version": "3.0"}, "http_version": "1.1",
        "method": req["method"], "scheme": "http", "path": path,
        "raw_path": path.encode("utf-8"), "root_path": "",
        "query_string": req["query"].encode("latin-1"),
        "headers": [(b"host", b"parity.test")] + [(k.encode("latin-1").lower(), v.encode("latin-1")) for k, v in req["headers"]],
        "client": ("127.0.0.1", 40000), "server": ("parity.test", 80), "state": {},
    }
    body = req["body"].encode("latin-1")
    messages = []
    sent = False
    async def receive():
        nonlocal sent
        if sent:
            await asyncio.sleep(3600)
        sent = True
        return {"type": "http.request", "body": body, "more_body": False}
    async def send(message):
        messages.append(message)
    try:
        await app(scope, receive, send)
    except Exception:
        pass
    start = next(m for m in messages if m["type"] == "http.response.start")
    payload = b"".join(m.get("body", b"") for m in messages if m["type"] == "http.response.body")
    if start["status"] == 299:
        return {"status": 299, "accepted": json.loads(payload)["accepted"]}
    headers = {k.decode("latin-1"): v.decode("latin-1") for k, v in start.get("headers", [])}
    return {"status": start["status"], "content_type": headers.get("content-type"), "body": lat(payload)}

async def main():
    for line in sys.stdin:
        sys.stdout.write(json.dumps(await drive(json.loads(line)), ensure_ascii=True) + "\n")

asyncio.run(main())
`

type oracleCase struct {
	Category string            `json:"-"`
	Route    string            `json:"-"`
	Params   map[string]string `json:"-"`
	Method   string            `json:"method"`
	Path     string            `json:"path"`
	Query    string            `json:"query"`
	Headers  [][2]string       `json:"headers"`
	Body     string            `json:"body"`
}

func (c oracleCase) wire() oracleCase {
	w := c
	w.Query = latin1(c.Query)
	w.Body = latin1(c.Body)
	w.Headers = make([][2]string, len(c.Headers))
	for i, h := range c.Headers {
		w.Headers[i] = [2]string{latin1(h[0]), latin1(h[1])}
	}
	return w
}

var w1Set = map[string]bool{}

func init() {
	for _, id := range W1Routes {
		w1Set[id] = true
	}
}

func TestOracle(t *testing.T) {
	image := envOr("PYVAL_ORACLE_IMAGE", "mobile-parity-api:7bf58e3d")
	seed, err := strconv.ParseUint(envOr("PYVAL_ORACLE_SEED", "1"), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	scope := envOr("PYVAL_ORACLE_SCOPE", "all")
	c := loadContract(t)
	reg := NewRegistry()
	registerAppPorts(reg)

	bound := map[string]*Route{}
	var skipped []string
	for _, id := range c.RouteIDs() {
		if scope == "w1" && !w1Set[id] {
			continue
		}
		r, err := c.Bind(id, reg)
		if err != nil {
			skipped = append(skipped, id)
			continue
		}
		bound[id] = r
	}
	ids := make([]string, 0, len(bound))
	for id := range bound {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	var cases []oracleCase
	formRoutes, formCaseCount := 0, 0
	for _, id := range ids {
		g := &gen{r: rand.New(rand.NewPCG(seed, hashString(id))), route: bound[id]}
		switch {
		case bound[id].form:
			// Form and File routes: random form bodies (oracle_forms_test.go).
			g.budget = 400
			g.formRandom()
			formRoutes++
			formCaseCount += len(g.cases)
		case w1Set[id]:
			g.budget = 900
			g.w1Extras()
			g.generic()
		default:
			g.budget = 160
			g.generic()
		}
		cases = append(cases, g.cases...)
	}

	var stdin bytes.Buffer
	for _, oc := range cases {
		line, err := json.Marshal(oc.wire())
		if err != nil {
			t.Fatal(err)
		}
		stdin.Write(line)
		stdin.WriteByte('\n')
	}
	started := time.Now()
	lines := pythonAnswers(t, image, stdin.Bytes(), len(cases))

	type tally struct{ total, bad, accepted, refused int }
	byRoute := map[string]*tally{}
	byCategory := map[string]*tally{}
	shown := 0
	mismatches := 0
	for i, oc := range cases {
		want := normalizeJSON(t, lines[i])
		got := normalizeJSON(t, goOutcome(t, bound[oc.Route], oc))
		for _, target := range []struct {
			m   map[string]*tally
			key string
		}{{byRoute, oc.Route}, {byCategory, oc.Category}} {
			tl := target.m[target.key]
			if tl == nil {
				tl = &tally{}
				target.m[target.key] = tl
			}
			tl.total++
			if strings.Contains(want, `"status":299`) {
				tl.accepted++
			} else {
				tl.refused++
			}
			if got != want {
				tl.bad++
			}
		}
		if got == want {
			continue
		}
		mismatches++
		if shown < 25 {
			shown++
			req, _ := json.Marshal(oc.wire())
			t.Errorf("mismatch %s [%s]\n  request: %s\n  python:  %s\n  go:      %s",
				oc.Route, oc.Category, clip(string(req), 700), clip(want, 900), clip(got, 900))
		}
	}
	for _, id := range ids {
		tl := byRoute[id]
		if tl == nil {
			continue
		}
		t.Logf("%-62s cases=%5d accepted=%5d refused=%5d mismatches=%d", id, tl.total, tl.accepted, tl.refused, tl.bad)
	}
	cats := make([]string, 0, len(byCategory))
	for k := range byCategory {
		cats = append(cats, k)
	}
	sort.Strings(cats)
	for _, k := range cats {
		tl := byCategory[k]
		t.Logf("  category %-28s cases=%5d mismatches=%d", k, tl.total, tl.bad)
	}
	t.Logf("skipped (not bindable): %d routes", len(skipped))
	t.Logf("form and file routes: %d, form cases: %d; other routes: %d, cases: %d",
		formRoutes, formCaseCount, len(ids)-formRoutes, len(cases)-formCaseCount)
	t.Logf("oracle %s seed=%d scope=%s: %d routes, %d cases, %d mismatches, took %s",
		image, seed, scope, len(ids), len(cases), mismatches, time.Since(started).Round(time.Millisecond))
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func hashString(s string) uint64 {
	sum := sha256.Sum256([]byte(s))
	var h uint64
	for _, b := range sum[:8] {
		h = h<<8 | uint64(b)
	}
	return h
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// oracleDriverOverride replaces oracleDriver for one pythonAnswers call.
var oracleDriverOverride string

// oracleRoutesMarker is the oracleDriver line a test replaces with code that
// adds routes to the app before any endpoint is stubbed.
const oracleRoutesMarker = "#@extra-routes\n"

var (
	// oracleDockerArgs are extra `docker run` arguments, e.g. a mount.
	oracleDockerArgs []string
	// oracleCacheSalt joins the cache key: the content of a mounted file.
	oracleCacheSalt string
	// oracleCacheSuffix is appended to PYVAL_ORACLE_CACHE, so tests that
	// ask different questions keep separate caches.
	oracleCacheSuffix string
)

func pythonAnswers(t *testing.T, image string, stdin []byte, n int) [][]byte {
	t.Helper()
	driver := oracleDriver
	if oracleDriverOverride != "" {
		driver = oracleDriverOverride
	}
	head := image + "\n" + strings.Join(oracleDockerArgs, "\n") + "\n" + oracleCacheSalt + "\n" + driver + "\n"
	sum := sha256.Sum256(append([]byte(head), stdin...))
	key := hex.EncodeToString(sum[:])
	cache := os.Getenv("PYVAL_ORACLE_CACHE")
	if cache != "" {
		cache += oracleCacheSuffix
	}
	if cache != "" {
		if data, err := os.ReadFile(cache); err == nil {
			if head, rest, ok := bytes.Cut(data, []byte("\n")); ok && string(head) == key {
				lines := bytes.Split(bytes.TrimSuffix(rest, []byte("\n")), []byte("\n"))
				if len(lines) == n {
					t.Logf("python answers from cache %s", cache)
					return lines
				}
			}
		}
	}
	args := append([]string{"run", "--rm", "-i", "--network", "none"}, oracleDockerArgs...)
	args = append(args, "--entrypoint", "python", image, "-c", driver)
	cmd := exec.Command("docker", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdin, cmd.Stdout, cmd.Stderr = bytes.NewReader(stdin), &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("oracle failed: %v\n%s", err, clip(stderr.String(), 3000))
	}
	lines := bytes.Split(bytes.TrimSuffix(stdout.Bytes(), []byte("\n")), []byte("\n"))
	if len(lines) != n {
		t.Fatalf("oracle answered %d lines for %d cases\n%s", len(lines), n, clip(stderr.String(), 3000))
	}
	if cache != "" {
		out := append([]byte(key+"\n"), stdout.Bytes()...)
		if err := os.WriteFile(cache, out, 0o600); err != nil {
			t.Logf("cache not written: %v", err)
		}
	}
	return lines
}

// ---- the Go side ----

type hookProblem struct{ p problem.Problem }

func (h *hookProblem) Error() string { return h.p.Code }

// devHook mirrors app.api.deps.get_actor / get_actor_optional in dev mode
// (deps.py): no X-Actor-ID is 401; an X-Actor-ID uuid.UUID() refuses, an
// unknown role, or an X-Actor-Contexts entry uuid.UUID() refuses is 422,
// in that order. get_actor_optional answers None when X-Actor-ID is absent.
func devHook(d Dependency) error {
	switch d.Call {
	case "app.api.deps.get_actor_optional":
		if isNone(d.Values["actor_id"]) {
			return nil
		}
		return devActorCheck(d.Values)
	case "app.api.deps.get_actor":
		return devActorCheck(d.Values)
	}
	return nil
}

// devRoles is app.domain.permissions.ROLES.
var devRoles = map[string]bool{
	"group_admin": true, "batch_owner": true, "advancer": true, "recipient": true, "sender": true,
	"creditor": true, "member": true, "former_member": true, "guest": true, "platform_moderator": true,
}

func devActorCheck(values map[string]Value) error {
	refuse := func(status int, code, detail string) error {
		return &hookProblem{problem.Problem{Status: status, Code: code, Detail: detail}}
	}
	id, ok := values["actor_id"].(pyjson.String)
	if !ok {
		return refuse(401, "authentication_required", "Missing X-Actor-ID")
	}
	if _, ok := pythonUUID(string(id)); !ok {
		return refuse(422, "invalid_actor_id", "X-Actor-ID must be a UUID")
	}
	for _, role := range devCSV(values["actor_roles"]) {
		if !devRoles[role] {
			return refuse(422, "invalid_actor_roles", "X-Actor-Roles contains an unknown role")
		}
	}
	for _, c := range devCSV(values["actor_contexts"]) {
		if _, ok := pythonUUID(c); !ok {
			return refuse(422, "invalid_actor_contexts", "X-Actor-Contexts must contain comma-separated UUIDs")
		}
	}
	return nil
}

// devCSV is deps._csv: split on ",", strip, drop empty parts.
func devCSV(v Value) []string {
	s, ok := v.(pyjson.String)
	if !ok {
		return nil
	}
	var out []string
	for _, part := range strings.Split(string(s), ",") {
		if part = pyStrip(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func goOutcome(t *testing.T, r *Route, oc oracleCase) []byte {
	t.Helper()
	res, err := r.Validate(&Request{PathParams: oc.Params, RawQuery: oc.Query, Headers: oc.Headers, Body: []byte(oc.Body)}, devHook)
	rec := httptest.NewRecorder()
	var hp *hookProblem
	var be *BodyError
	switch {
	case errors.As(err, &hp):
		if werr := problem.WriteJSON(rec, hp.p); werr != nil {
			t.Fatal(werr)
		}
	case errors.As(err, &be):
		if werr := be.Respond(rec); werr != nil {
			t.Fatal(werr)
		}
	case err != nil:
		t.Fatalf("%s: %v", oc.Route, err)
	case len(res.Errors) > 0:
		if werr := problem.WriteValidation(rec, res.Errors); werr != nil {
			t.Fatal(werr)
		}
	default:
		var accepted []any
		for _, in := range []string{"path", "query", "header", "cookie", "body"} {
			for _, p := range r.root.params[in] {
				if v, ok := res.Values[p.name]; ok {
					accepted = append(accepted, []any{p.name, goTree(v)})
				}
			}
		}
		if accepted == nil {
			accepted = []any{}
		}
		out, _ := json.Marshal(map[string]any{"status": 299, "accepted": accepted})
		return out
	}
	out, _ := json.Marshal(map[string]any{
		"status":       rec.Code,
		"content_type": rec.Header().Get("Content-Type"),
		"body":         latin1(rec.Body.String()),
	})
	return out
}

// ---- case generation ----

const (
	oracleActor   = "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	oracleContext = "cccccccc-dddd-4eee-8fff-aaaaaaaaaaaa"
)

type gen struct {
	r      *rand.Rand
	route  *Route
	budget int
	cases  []oracleCase
}

func (g *gen) chance(p float64) bool { return g.r.Float64() < p }

func pick[T any](g *gen, xs ...T) T { return xs[g.r.IntN(len(xs))] }

func (g *gen) needsActor() bool {
	var walk func(d *dependant) bool
	walk = func(d *dependant) bool {
		for _, s := range d.deps {
			if s.call == "app.api.deps.get_actor" || walk(s) {
				return true
			}
		}
		return false
	}
	return walk(g.route.root)
}

// request is one request under construction.
type request struct {
	params  map[string]string
	query   string
	headers [][2]string
	body    string
	hasBody bool
	ct      string // "" means no content-type header
	anon    bool
	// customActor leaves the default X-Actor-* headers out; the case puts
	// its own in headers.
	customActor bool
}

func (g *gen) emit(category string, rq request) {
	if len(g.cases) >= g.budget {
		return
	}
	path := g.route.Path
	for k, v := range rq.params {
		path = strings.Replace(path, "{"+k+"}", v, 1)
	}
	var headers [][2]string
	if !rq.anon && !rq.customActor {
		headers = append(headers, [2]string{"X-Actor-ID", oracleActor}, [2]string{"X-Actor-Roles", "member"})
	}
	if rq.ct != "" {
		headers = append(headers, [2]string{"Content-Type", rq.ct})
	}
	headers = append(headers, rq.headers...)
	g.cases = append(g.cases, oracleCase{
		Category: category, Route: g.route.ID, Params: rq.params, Method: g.route.Method,
		Path: path, Query: rq.query, Headers: headers, Body: rq.body,
	})
}

func (g *gen) routeParams(in string) []*param { return g.route.root.params[in] }

// usesActorHeaders reports whether get_actor or get_actor_optional reads
// X-Actor-* for this route.
func (g *gen) usesActorHeaders() bool {
	var walk func(d *dependant) bool
	walk = func(d *dependant) bool {
		for _, s := range d.deps {
			if s.call == "app.api.deps.get_actor" || s.call == "app.api.deps.get_actor_optional" || walk(s) {
				return true
			}
		}
		return false
	}
	return walk(g.route.root)
}

// actorHeaderRepeats sends each X-Actor-* header twice, once valid and once
// not, in both orders and with the name in other letter cases.
func actorHeaderRepeats() [][][2]string {
	h := func(name, value string) [2]string { return [2]string{name, value} }
	role := h("X-Actor-Roles", "member")
	return [][][2]string{
		{h("X-Actor-ID", oracleActor), h("X-Actor-ID", "nope"), role},
		{h("X-Actor-ID", "nope"), h("X-Actor-ID", oracleActor), role},
		{h("x-actor-id", oracleActor), h("X-ACTOR-ID", "nope"), role},
		{h("X-ACTOR-ID", "nope"), h("x-actor-id", oracleActor), role},
		{h("X-Actor-ID", oracleActor), role, h("x-actor-roles", "bogus")},
		{h("X-Actor-ID", oracleActor), h("X-Actor-Roles", "bogus"), h("X-ACTOR-ROLES", "member")},
		{h("X-Actor-ID", oracleActor), role, h("X-Actor-Contexts", oracleContext), h("x-actor-contexts", "nope")},
		{h("X-Actor-ID", oracleActor), role, h("X-Actor-Contexts", "nope"), h("X-Actor-Contexts", oracleContext)},
	}
}

// headerRepeats sends an endpoint header parameter once and twice, a good
// and a bad value in both orders, and with the name in other letter cases.
func headerRepeats(p *param) [][][2]string {
	good, bad := "Bearer first-value", "Bearer second-value"
	if s, ok := func() (*strValidator, bool) { c, _ := core(p.v); s, ok := c.(*strValidator); return s, ok }(); ok && (s.min > 0 || s.max >= 0) {
		good = strings.Repeat("k", max(s.min, 1))
		if s.min > 0 {
			bad = ""
		} else {
			bad = strings.Repeat("k", s.max+1)
		}
	}
	lower, upper := strings.ToLower(p.alias), strings.ToUpper(p.alias)
	return [][][2]string{
		{{p.alias, good}},
		{{p.alias, bad}},
		{{p.alias, good}, {p.alias, bad}},
		{{p.alias, bad}, {p.alias, good}},
		{{lower, good}, {upper, bad}},
		{{upper, bad}, {lower, good}},
	}
}

// baseRequest returns a request whose parameters are all valid.
func (g *gen) baseRequest() request {
	rq := request{params: map[string]string{}, ct: "application/json"}
	for _, p := range g.routeParams("path") {
		rq.params[p.name] = g.pathValue(p.v)
	}
	var q []string
	for _, p := range g.routeParams("query") {
		if g.chance(0.5) {
			q = append(q, url.QueryEscape(p.alias)+"="+url.QueryEscape(scalarText(g.valid(p.v, 0))))
		}
	}
	rq.query = strings.Join(q, "&")
	if bp := g.routeParams("body"); len(bp) > 0 {
		rq.hasBody = true
		rq.body = dumpsDoc(g.bodyDoc(bp, nil))
	}
	return rq
}

// generic emits valid requests, then single faults in every parameter slot,
// then pairs, then transport-level variants.
func (g *gen) generic() {
	for i := 0; i < 6; i++ {
		g.emit("valid", g.baseRequest())
	}
	if g.needsActor() {
		rq := g.baseRequest()
		rq.anon = true
		g.emit("anonymous", rq)
	}
	// Header parameters, each sent more than once: Starlette's Headers.get
	// takes the first value of a name in any letter case, getlist all of them.
	if g.usesActorHeaders() {
		for _, headers := range actorHeaderRepeats() {
			rq := g.baseRequest()
			rq.customActor = true
			rq.headers = headers
			g.emit("header-repeat-auth", rq)
		}
	}
	for _, p := range g.routeParams("header") {
		for _, headers := range headerRepeats(p) {
			rq := g.baseRequest()
			rq.headers = headers
			g.emit("header-repeat", rq)
		}
	}
	// Path parameters.
	for _, p := range g.routeParams("path") {
		for _, bad := range g.pathWrongs(p.v) {
			rq := g.baseRequest()
			rq.params[p.name] = bad
			g.emit("path-fault", rq)
		}
	}
	// Query parameters.
	for _, p := range g.routeParams("query") {
		for _, bad := range g.queryWrongs(p) {
			rq := g.baseRequest()
			rq.query = bad
			g.emit("query-fault", rq)
		}
	}
	if bp := g.routeParams("body"); len(bp) > 0 {
		g.bodyFaults(bp)
	}
	for i := 0; i < 12; i++ {
		rq := g.baseRequest()
		for _, p := range g.routeParams("path") {
			if g.chance(0.5) {
				rq.params[p.name] = pick(g, g.pathWrongs(p.v)...)
			}
		}
		if bp := g.routeParams("body"); len(bp) > 0 {
			rq.body = dumpsDoc(g.faultyDoc(bp, 2))
		}
		g.emit("pair-fault", rq)
	}
}

// ---- values ----

func dumps(v pyjson.Value) string {
	b, err := pyjson.Dumps(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func scalarText(v pyjson.Value) string {
	switch x := v.(type) {
	case pyjson.String:
		return string(x)
	case pyjson.Int:
		return x.String()
	case pyjson.Float:
		return pyjson.FloatRepr(float64(x))
	case pyjson.Bool:
		if x {
			return "true"
		}
		return "false"
	}
	return ""
}

var textSamples = []string{
	"a", "khau-vi", "Qu\u1eadn 9", "\u00e9", "\U0001F600", " a", "a ", "\u00a0x", "x\x1c", "\t",
	"<b>&\"x\"</b>\\", "\u2028", "ab\x00c", "\u0301e", "\u4e2d\u6587",
}

func (g *gen) text(minLen, maxLen int) string {
	if maxLen < 0 {
		maxLen = minLen + 8
		if minLen < 0 {
			maxLen = 8
		}
	}
	if minLen < 0 {
		minLen = 0
	}
	if maxLen > minLen+12 && g.chance(0.7) {
		maxLen = minLen + 12
	}
	n := minLen + g.r.IntN(maxLen-minLen+1)
	var b strings.Builder
	for b.Len() < 4*n && utf8.RuneCountInString(b.String()) < n {
		switch k := g.r.IntN(10); {
		case k < 6:
			b.WriteByte(byte('a' + g.r.IntN(26)))
		case k < 7:
			b.WriteByte(byte('A' + g.r.IntN(26)))
		case k < 8:
			b.WriteRune(pick(g, '\u00e9', '\u1ead', '\u0111', ' ', '-'))
		case k < 9:
			b.WriteRune(pick(g, '\U0001F600', '\u4e2d', '\u2028'))
		default:
			b.WriteByte(byte('0' + g.r.IntN(10)))
		}
	}
	s := []rune(b.String())
	if len(s) > n {
		s = s[:n]
	}
	return string(s)
}

func (g *gen) validDate() string {
	return pad(1900+g.r.IntN(200), 4) + "-" + pad(1+g.r.IntN(12), 2) + "-" + pad(1+g.r.IntN(28), 2)
}

func (g *gen) uuidCanonical() string {
	var u UUID
	for i := range u {
		u[i] = byte(g.r.IntN(256))
	}
	return u.String()
}

// uuidSpelling returns a valid or invalid spelling of a UUID, weighted
// towards the edges of the uuid crate's parser.
func (g *gen) uuidSpelling(valid bool) string {
	u := g.uuidCanonical()
	hexOnly := strings.ReplaceAll(u, "-", "")
	if valid {
		return pick(g, u, strings.ToUpper(u), hexOnly, strings.ToUpper(hexOnly), "{"+u+"}", "urn:uuid:"+u,
			"urn:uuid:"+strings.ToUpper(u), "{"+strings.ToUpper(u)+"}", mixCase(g, u))
	}
	b := []byte(u)
	switch g.r.IntN(16) {
	case 0:
		return "URN:UUID:" + u
	case 1:
		return "{" + hexOnly + "}"
	case 2:
		return "urn:uuid:" + hexOnly
	case 3:
		i := g.r.IntN(len(b))
		return string(b[:i]) + string(b[i+1:])
	case 4:
		i := g.r.IntN(len(b) + 1)
		return string(b[:i]) + pick(g, "a", "0", "-") + string(b[i:])
	case 5:
		i := g.r.IntN(len(b))
		return string(b[:i]) + pick(g, "g", "G", "_", " ", "\u00e9", "\U0001F600", "{", "}", ":", "z", "-") + string(b[i+1:])
	case 6:
		return hexOnly[:g.r.IntN(len(hexOnly))]
	case 7:
		return hexOnly + pick(g, "a", "ab", "-")
	case 8:
		return "{" + u
	case 9:
		return "{" + u[:len(u)-g.r.IntN(3)-1] + "}"
	case 10:
		return "{" + u + pick(g, "a", "ab", "abc") + "}"
	case 11:
		return "urn:uuid:" + u + pick(g, "a", "ab")
	case 12:
		return pick(g, "nope", "a-b-c-d-e", "a-b-c-d-e-f", "urn:uuid:", "{}", "--------", "g", "\u00e9", "0")
	case 13:
		// Move one hyphen.
		i := pick(g, 8, 13, 18, 23)
		d := pick(g, -1, 1)
		b[i], b[i+d] = b[i+d], b[i]
		return string(b)
	case 14:
		return " " + u
	}
	return strings.Replace(u, "-", "", 1+g.r.IntN(3))
}

func mixCase(g *gen, s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'a' && c <= 'f' && g.chance(0.5) {
			b[i] = c - 'a' + 'A'
		}
	}
	return string(b)
}

// core strips wrappers that do not change what a value slot accepts.
func core(v validator) (validator, bool) {
	nullable := false
	for {
		switch x := v.(type) {
		case *nullableValidator:
			nullable = true
			v = x.inner
		case *defaultValidator:
			v = x.inner
		case *refValidator:
			v = x.inner
		case *funcValidator:
			if x.inner == nil {
				return v, nullable
			}
			v = x.inner
		default:
			return v, nullable
		}
	}
}

func (g *gen) valid(v validator, depth int) pyjson.Value {
	switch x := v.(type) {
	case *nullableValidator:
		if g.chance(0.2) {
			return pyjson.Null{}
		}
		return g.valid(x.inner, depth)
	case *defaultValidator:
		return g.valid(x.inner, depth)
	case *refValidator:
		if depth > 6 {
			return pyjson.Null{}
		}
		return g.valid(x.inner, depth+1)
	case *funcValidator:
		if x.inner != nil {
			return g.valid(x.inner, depth)
		}
		return pyjson.String(g.text(1, 8))
	case *strValidator:
		if x.pattern != nil {
			return pyjson.String(g.fromPattern(x.patternText, x.min, x.max))
		}
		return pyjson.String(g.text(x.min, x.max))
	case *intValidator:
		return pyjson.NewInt(g.intInBounds(x.bounds))
	case *floatValidator:
		lo, hi := -1000.0, 1000.0
		for _, b := range x.bounds {
			switch b.key {
			case "ge", "gt":
				lo = b.b.f
			case "le", "lt":
				hi = b.b.f
			}
		}
		f := lo + g.r.Float64()*(hi-lo)
		if g.chance(0.3) {
			return pyjson.Float(math.Round(f))
		}
		return pyjson.Float(f)
	case *boolValidator:
		return pyjson.Bool(g.chance(0.5))
	case *uuidValidator:
		return pyjson.String(g.uuidCanonical())
	case *dateValidator:
		return pyjson.String(g.validDate())
	case *dateTimeValidator:
		s := g.validDate() + "T" + pad(g.r.IntN(24), 2) + ":" + pad(g.r.IntN(60), 2) + ":" + pad(g.r.IntN(60), 2)
		if g.chance(0.3) {
			s += "." + pad(g.r.IntN(1000000), 6)
		}
		return pyjson.String(s + pick(g, "Z", "+07:00", "-05:30", "+00:00", ""))
	case *literalValidator:
		return pick(g, x.expected...)
	case *listValidator:
		n := g.r.IntN(4)
		if x.min >= 0 && n < x.min {
			n = x.min
		}
		if x.max >= 0 && n > x.max {
			n = x.max
		}
		out := make(pyjson.List, n)
		for i := range out {
			if x.item == nil {
				out[i] = pyjson.String(g.text(1, 4))
			} else {
				out[i] = g.valid(x.item, depth+1)
			}
		}
		return out
	case *dictValidator:
		out := pyjson.NewOrderedMap()
		for i := g.r.IntN(3); i > 0; i-- {
			key := g.text(1, 6)
			if x.keys != nil {
				key = scalarText(g.valid(x.keys, depth+1))
			}
			var val pyjson.Value = pyjson.String(g.text(0, 4))
			if x.values != nil {
				val = g.valid(x.values, depth+1)
			}
			out.Set(key, val)
		}
		return out
	case anyValidator:
		return pick[pyjson.Value](g, pyjson.String("x"), pyjson.NewInt(3), pyjson.Null{}, pyjson.List{pyjson.Bool(true)})
	case noneValidator:
		return pyjson.Null{}
	case *modelValidator:
		return g.validModel(x.fields, depth)
	case *modelFieldsValidator:
		return g.validModel(x, depth)
	case *unionValidator:
		return g.valid(pick(g, x.choices...).v, depth)
	}
	return pyjson.Null{}
}

func (g *gen) validModel(m *modelFieldsValidator, depth int) *pyjson.OrderedMap {
	out := pyjson.NewOrderedMap()
	for _, f := range m.fields {
		if f.def != nil && g.chance(0.4) {
			continue
		}
		out.Set(f.name, g.valid(f.v, depth+1))
	}
	return out
}

func (g *gen) intInBounds(bounds []numBound) int64 {
	lo, hi := int64(-1000), int64(1000)
	for _, b := range bounds {
		if b.b.isFloat || !b.b.i.IsInt64() {
			continue
		}
		n := b.b.i.Int64()
		switch b.key {
		case "ge":
			lo = n
		case "gt":
			lo = n + 1
		case "le":
			hi = n
		case "lt":
			hi = n - 1
		}
	}
	if hi < lo {
		return lo
	}
	if hi-lo > 5000 {
		hi = lo + 5000
	}
	return lo + g.r.Int64N(hi-lo+1)
}

// fromPattern generates a string matching an RE2 pattern, sized towards
// [minLen, maxLen].
func (g *gen) fromPattern(pattern string, minLen, maxLen int) string {
	re, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil {
		return "x"
	}
	re = re.Simplify()
	target := -1
	if minLen >= 0 {
		target = minLen
		if maxLen > minLen {
			target += g.r.IntN(min(maxLen-minLen, 20) + 1)
		}
	}
	var b strings.Builder
	g.walkPattern(&b, re, target)
	return b.String()
}

func (g *gen) walkPattern(b *strings.Builder, re *syntax.Regexp, target int) {
	switch re.Op {
	case syntax.OpLiteral:
		for _, r := range re.Rune {
			b.WriteRune(r)
		}
	case syntax.OpCharClass:
		i := 2 * g.r.IntN(len(re.Rune)/2)
		lo, hi := re.Rune[i], re.Rune[i+1]
		if hi-lo > 200 {
			hi = lo + 200
		}
		b.WriteRune(lo + rune(g.r.IntN(int(hi-lo)+1)))
	case syntax.OpAnyCharNotNL, syntax.OpAnyChar:
		b.WriteByte('a')
	case syntax.OpCapture:
		g.walkPattern(b, re.Sub[0], target)
	case syntax.OpConcat:
		for _, s := range re.Sub {
			g.walkPattern(b, s, target)
		}
	case syntax.OpAlternate:
		g.walkPattern(b, re.Sub[g.r.IntN(len(re.Sub))], target)
	case syntax.OpStar, syntax.OpPlus, syntax.OpQuest, syntax.OpRepeat:
		lo, hi := 0, 3
		switch re.Op {
		case syntax.OpPlus:
			lo = 1
		case syntax.OpQuest:
			hi = 1
		case syntax.OpRepeat:
			lo, hi = re.Min, re.Max
			if hi < 0 {
				hi = lo + 3
			}
		}
		n := lo + g.r.IntN(hi-lo+1)
		if target >= 0 && re.Op != syntax.OpQuest && re.Op != syntax.OpRepeat {
			n = max(target, lo)
		}
		for i := 0; i < n; i++ {
			g.walkPattern(b, re.Sub[0], -1)
		}
	}
}

// ---- wrong values ----

func (g *gen) wrongs(v validator) []pyjson.Value {
	c, nullable := core(v)
	var out []pyjson.Value
	if !nullable {
		out = append(out, pyjson.Null{})
	}
	common := []pyjson.Value{pyjson.Bool(true), pyjson.NewInt(1), pyjson.Float(1.5), pyjson.List{}, pyjson.NewOrderedMap()}
	switch x := c.(type) {
	case *strValidator:
		out = append(out, common...)
		out = append(out, pyjson.Float(math.NaN()), pyjson.String("\xed\xa0\x80"), pyjson.String("a\xed\xb0\x80b"))
		for _, n := range []int{x.min - 1, x.min, x.max, x.max + 1} {
			if n < 0 {
				continue
			}
			out = append(out, pyjson.String(strings.Repeat("a", n)), pyjson.String(strings.Repeat("\u00e9", n)),
				pyjson.String(strings.Repeat("\U0001F600", n)))
			if n > 0 {
				out = append(out, pyjson.String(strings.Repeat("a", n-1)+"\xed\xa0\x80"), pyjson.String(" "+strings.Repeat("b", n-1)))
			}
		}
		if x.pattern != nil {
			good := g.fromPattern(x.patternText, x.min, x.max)
			out = append(out, pyjson.String(good+"!"), pyjson.String(good+"\n"), pyjson.String(strings.ToUpper(good)))
		}
	case *intValidator:
		out = append(out, pyjson.String("1"), pyjson.String(" 7 "), pyjson.String("+3"), pyjson.String("1.0"), pyjson.String("1."),
			pyjson.String("1_0"), pyjson.String("_1"), pyjson.String("1__0"), pyjson.String("x"), pyjson.String(""),
			pyjson.String("\uff11"), pyjson.Float(2.0), pyjson.Float(2.5), pyjson.Float(math.Inf(1)), pyjson.Float(math.NaN()),
			pyjson.Float(1e20), pyjson.Bool(false), pyjson.List{}, pyjson.NewBigInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(20), nil)),
			pyjson.String("1."), pyjson.String("1.00"), pyjson.String("-1.0"), pyjson.String(".0"), pyjson.String("1_0.0"),
			pyjson.String("1.0_0"), pyjson.String(" +1_0 "), pyjson.String("1\xed\xa0\x80"), pyjson.String("\xed\xa0\x80"),
			pyjson.Float(9.2e18), pyjson.Float(-9.3e18))
		for _, b := range x.bounds {
			if !b.b.isFloat && b.b.i.IsInt64() {
				n := b.b.i.Int64()
				out = append(out, pyjson.NewInt(n-1), pyjson.NewInt(n), pyjson.NewInt(n+1), pyjson.String(strconv.FormatInt(n+1, 10)))
			}
		}
	case *floatValidator:
		out = append(out, pyjson.String("1.5"), pyjson.String(" 2.5"), pyjson.String("nan"), pyjson.String("-Infinity"),
			pyjson.String("1_0.5"), pyjson.String("1e400"), pyjson.String("0x10"), pyjson.String("x"), pyjson.NewInt(3),
			pyjson.Bool(true), pyjson.Float(math.NaN()), pyjson.Float(math.Inf(-1)), pyjson.List{},
			pyjson.String("1."), pyjson.String(".5"), pyjson.String("-nan"), pyjson.String("1__0"), pyjson.String(" 1_0"),
			pyjson.String("1\xed\xa0\x80"), pyjson.NewBigInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(400), nil)))
		for _, b := range x.bounds {
			out = append(out, pyjson.Float(b.b.f), pyjson.Float(math.Nextafter(b.b.f, math.Inf(1))),
				pyjson.Float(math.Nextafter(b.b.f, math.Inf(-1))), pyjson.String(rustDisplayFloat(b.b.f+1)))
		}
	case *boolValidator:
		out = append(out, pyjson.String("yes"), pyjson.String("OFF"), pyjson.String(" true"), pyjson.String("2"),
			pyjson.String("ye\u017f"), pyjson.NewInt(0), pyjson.NewInt(2), pyjson.Float(1.0), pyjson.Float(0.5), pyjson.List{},
			pyjson.Float(2.0), pyjson.Float(math.NaN()), pyjson.NewInt(-1), pyjson.String("t\xed\xa0\x80"),
			pyjson.NewBigInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(20), nil)))
	case *uuidValidator:
		out = append(out, pyjson.NewInt(5), pyjson.Bool(true), pyjson.List{}, pyjson.NewOrderedMap(), pyjson.String("\xed\xa0\x80"))
		for i := 0; i < 8; i++ {
			out = append(out, pyjson.String(g.uuidSpelling(i < 3)))
		}
	case *dateValidator, *dateTimeValidator:
		out = append(out, pyjson.NewInt(0), pyjson.NewInt(86401), pyjson.Float(1.5), pyjson.Bool(true), pyjson.List{},
			pyjson.String("\xed\xa0\x80"), pyjson.String("2024-01-02"), pyjson.String("2024-01-02T03:04:05"),
			pyjson.String("2024-01-02T00:00:00+07:00"), pyjson.String(strconv.Itoa(1_700_000_000)))
		for i := 0; i < 8; i++ {
			out = append(out, pyjson.String(g.dateTimeText()))
		}
	case *literalValidator:
		out = append(out, pyjson.NewInt(1), pyjson.Float(1.0), pyjson.Bool(true), pyjson.List{}, pyjson.String(""), pyjson.String("\xed\xa0\x80"))
		if len(x.expected) > 0 {
			if s, ok := x.expected[0].(pyjson.String); ok {
				out = append(out, pyjson.String(strings.ToUpper(string(s))), pyjson.String(string(s)+" "), pyjson.List{s})
			}
		}
	case *listValidator:
		out = append(out, pyjson.String("ab"), pyjson.NewOrderedMap(), pyjson.NewInt(3))
		if x.item != nil {
			for _, w := range firstN(g.wrongs(x.item), 3) {
				out = append(out, pyjson.List{g.valid(x.item, 1), w})
			}
		}
		for _, n := range []int{x.min - 1, x.max + 1} {
			if n >= 0 {
				l := make(pyjson.List, n)
				for i := range l {
					if x.item != nil {
						l[i] = g.valid(x.item, 1)
					} else {
						l[i] = pyjson.String("x")
					}
				}
				out = append(out, l)
			}
		}
	case *dictValidator:
		out = append(out, pyjson.List{}, pyjson.String("x"), pyjson.NewInt(1))
		bad := pyjson.NewOrderedMap()
		bad.Set("not-a-key", pyjson.String("v"))
		bad.Set(g.uuidCanonical(), pyjson.String("1"))
		out = append(out, bad)
	case *modelValidator, *modelFieldsValidator:
		out = append(out, common[:4]...)
		out = append(out, pyjson.String("{}"))
	case *unionValidator:
		out = append(out, common...)
	}
	return out
}

func firstN(xs []pyjson.Value, n int) []pyjson.Value {
	if len(xs) > n {
		return xs[:n]
	}
	return xs
}

// ---- documents with slots ----

type slot struct {
	v   validator
	set func(pyjson.Value)
	del func() // nil when the value cannot be removed
	key func(string)
}

func (g *gen) bodyDoc(bp []*param, slots *[]slot) *pyjson.Value {
	var doc pyjson.Value
	if len(bp) == 1 && !g.route.embed {
		doc = g.valid(bp[0].v, 0)
	} else {
		m := pyjson.NewOrderedMap()
		for _, p := range bp {
			m.Set(p.alias, g.valid(p.v, 0))
		}
		doc = m
	}
	holder := &doc
	if slots != nil && len(bp) == 1 && !g.route.embed {
		*slots = append(*slots, slot{v: bp[0].v, set: func(x pyjson.Value) { *holder = x }})
		g.collect(bp[0].v, doc, slots)
	}
	return holder
}

func (g *gen) collect(v validator, val pyjson.Value, slots *[]slot) {
	c, _ := core(v)
	switch x := c.(type) {
	case *modelValidator:
		g.collectModel(x.fields, val, slots)
	case *modelFieldsValidator:
		g.collectModel(x, val, slots)
	case *listValidator:
		l, ok := val.(pyjson.List)
		if !ok || x.item == nil {
			return
		}
		for i := range l {
			i := i
			*slots = append(*slots, slot{v: x.item, set: func(n pyjson.Value) { l[i] = n }})
			g.collect(x.item, l[i], slots)
		}
	case *dictValidator:
		m, ok := val.(*pyjson.OrderedMap)
		if !ok || x.values == nil {
			return
		}
		for _, k := range m.Keys() {
			k := k
			cur, _ := m.Get(k)
			*slots = append(*slots, slot{v: x.values, set: func(n pyjson.Value) { m.Set(k, n) }})
			g.collect(x.values, cur, slots)
		}
	}
}

func (g *gen) collectModel(fields *modelFieldsValidator, val pyjson.Value, slots *[]slot) {
	m, ok := val.(*pyjson.OrderedMap)
	if !ok {
		return
	}
	*slots = append(*slots, slot{v: fields, key: func(k string) { m.Set(k, pyjson.NewInt(1)) }})
	for _, f := range fields.fields {
		cur, present := m.Get(f.name)
		name := f.name
		del := func() { *m = *without(m, name) }
		if !present {
			*slots = append(*slots, slot{v: f.v, set: func(n pyjson.Value) { m.Set(name, n) }})
			continue
		}
		*slots = append(*slots, slot{v: f.v, set: func(n pyjson.Value) { m.Set(name, n) }, del: del})
		g.collect(f.v, cur, slots)
	}
}

func without(m *pyjson.OrderedMap, key string) *pyjson.OrderedMap {
	out := pyjson.NewOrderedMap()
	for k, v := range m.All() {
		if k != key {
			out.Set(k, v)
		}
	}
	return out
}

// faultyDoc builds a valid body and applies n random faults to it.
func (g *gen) faultyDoc(bp []*param, n int) *pyjson.Value {
	var slots []slot
	doc := g.bodyDoc(bp, &slots)
	for i := 0; i < n && len(slots) > 0; i++ {
		s := slots[g.r.IntN(len(slots))]
		g.applyFault(s)
	}
	return doc
}

var extraKeys = []string{"x", "person_id", "", "\u00e9", "\xed\xa0\x80", "Interests", "note ", "id"}

func (g *gen) applyFault(s slot) {
	switch {
	case s.key != nil:
		s.key(pick(g, extraKeys...))
	case s.del != nil && g.chance(0.25):
		s.del()
	case s.set != nil:
		if ws := g.wrongs(s.v); len(ws) > 0 {
			s.set(ws[g.r.IntN(len(ws))])
		}
	}
}

func (g *gen) bodyFaults(bp []*param) {
	// Enumerate slots on a probe document, then fault each in turn on fresh
	// documents built from a replayed generator.
	probeSeed := g.r.Uint64()
	var probe []slot
	replay := &gen{r: rand.New(rand.NewPCG(probeSeed, 7)), route: g.route}
	replay.bodyDoc(bp, &probe)
	for i := range probe {
		kinds := 1
		if probe[i].set != nil {
			kinds = len(replay.wrongs(probe[i].v))
		}
		if probe[i].del != nil {
			kinds++
		}
		for j := 0; j < kinds; j++ {
			fresh := &gen{r: rand.New(rand.NewPCG(probeSeed, 7)), route: g.route}
			var slots []slot
			doc := fresh.bodyDoc(bp, &slots)
			if i >= len(slots) {
				break
			}
			s := slots[i]
			var ws []pyjson.Value
			if s.set != nil {
				ws = fresh.wrongs(s.v)
			}
			switch {
			case s.key != nil:
				s.key(extraKeys[j%len(extraKeys)])
			case j < len(ws):
				s.set(ws[j])
			case s.del != nil:
				s.del()
			}
			rq := g.baseRequest()
			rq.body = dumpsDoc(doc)
			g.emit("body-fault", rq)
		}
	}
	for _, top := range []string{"[]", `"x"`, "1", "true", "null", "{}", "", " "} {
		rq := g.baseRequest()
		rq.body = top
		g.emit("body-top", rq)
	}
	rq := g.baseRequest()
	full := rq.body
	for _, cut := range []int{1, len(full) / 2, len(full) - 1} {
		if cut > 0 && cut < len(full) {
			r2 := g.baseRequest()
			r2.body = full[:cut]
			g.emit("body-malformed", r2)
		}
	}
	for _, ct := range []string{"text/plain", "application/merge-patch+json", "application/json; charset=utf-8"} {
		r2 := g.baseRequest()
		r2.ct = ct
		g.emit("content-type", r2)
	}
	if g.needsActor() {
		r2 := g.baseRequest()
		r2.anon = true
		r2.body = "{"
		g.emit("anonymous", r2)
	}
}

func dumpsDoc(v *pyjson.Value) string { return dumps(*v) }

// ---- path and query ----

func (g *gen) pathValue(v validator) string {
	c, _ := core(v)
	switch x := c.(type) {
	case *uuidValidator:
		return g.uuidCanonical()
	case *intValidator:
		return strconv.Itoa(g.r.IntN(50))
	case *literalValidator:
		return scalarText(pick(g, x.expected...))
	case *strValidator:
		if x.pattern != nil {
			return g.fromPattern(x.patternText, x.min, x.max)
		}
		return strings.ReplaceAll(g.text(max(x.min, 1), x.max), "/", "-")
	}
	return "x"
}

func (g *gen) pathWrongs(v validator) []string {
	c, _ := core(v)
	switch x := c.(type) {
	case *uuidValidator:
		out := []string{}
		for i := 0; i < 14; i++ {
			if s := strings.ReplaceAll(g.uuidSpelling(i < 5), "/", ""); s != "" {
				out = append(out, s)
			}
		}
		return out
	case *intValidator:
		return []string{"-1", "01", "+1", " 1", "1.0", "1.", "1.00", ".0", "1_0", "1__0", "_1", "x", "\uff11", "1e3", strings.Repeat("9", 23)}
	case *literalValidator:
		return []string{"HEART", "heart ", "1", "x"}
	case *strValidator:
		out := []string{"x", "\u00e9", "a b"}
		if x.min > 0 {
			out = append(out, strings.Repeat("a", x.min-1), strings.Repeat("\U0001F600", x.min))
		}
		if x.max >= 0 {
			out = append(out, strings.Repeat("a", x.max), strings.Repeat("a", x.max+1))
		}
		if x.pattern != nil {
			good := g.fromPattern(x.patternText, x.min, x.max)
			out = append(out, good[:len(good)-1]+"!", good+"~")
		}
		return out
	}
	return []string{"x"}
}

func (g *gen) queryWrongs(p *param) []string {
	alias := url.QueryEscape(p.alias)
	good := scalarText(g.valid(p.v, 0))
	vals := []string{"", "x", " 1", "1.0", "1_0", "nan", "inf", "-1", "0", "101", "1e3", "%ZZ", "a+b", "%C3%A9", "%FF", "%ED%A0%80", strings.Repeat("9", 20)}
	c, _ := core(p.v)
	switch c.(type) {
	case *uuidValidator:
		vals = append(vals, url.QueryEscape(g.uuidSpelling(true)), url.QueryEscape(g.uuidSpelling(false)))
	case *literalValidator:
		vals = append(vals, "PHOTO", "photo%20")
	}
	var out []string
	for _, v := range vals {
		out = append(out, alias+"="+v)
	}
	out = append(out,
		alias,
		alias+"=",
		alias+"="+url.QueryEscape(good)+"&"+alias+"=x",
		alias+"=x&"+alias+"="+url.QueryEscape(good),
		"&&"+alias+"="+url.QueryEscape(good)+"&",
		strings.ToUpper(alias)+"=x",
	)
	return out
}

// ---- W1 extras ----

// w1Extras adds transport-level cases for the pilot routes: content types,
// encodings, malformed JSON at every region, NaN and Infinity, duplicate
// keys, boundary lengths in code points, and UUID spellings in the path.
func (g *gen) w1Extras() {
	bp := g.routeParams("body")
	if len(bp) == 0 {
		for i := 0; i < 60; i++ {
			rq := g.baseRequest()
			if rq.params["context_id"] = g.uuidSpelling(i%3 == 0); rq.params["context_id"] == "" {
				rq.params["context_id"] = "g"
			}
			if i%7 == 0 {
				rq.anon = true
			}
			if i%5 == 0 {
				rq.query = "x=1&context_id=" + g.uuidCanonical()
				rq.body = "{"
			}
			g.emit("w1-uuid-path", rq)
		}
		return
	}
	base := g.baseRequest()
	doc := base.body
	for _, ct := range []string{
		"", "application/json", "APPLICATION/JSON", "application/json; charset=latin-1", "application/vnd.api+json",
		"application/json-patch+json", "  application/json  ;x", "application/json\xa0", "\x1capplication/json",
		"\x85application/json", "application", "application/json/x", "text/json", "text/plain", " ", ";", "application/+json",
		"application/json+x", "application/jsonx", "multipart/form-data", "application/x-www-form-urlencoded",
	} {
		rq := g.baseRequest()
		rq.body = doc
		rq.ct = ct
		if ct == "" {
			rq.headers = [][2]string{{"Content-Type", ""}}
		}
		g.emit("w1-content-type", rq)
		rq.ct = ""
		rq.headers = nil
		if ct == "" {
			g.emit("w1-content-type", rq)
		}
	}
	twoCT := g.baseRequest()
	twoCT.body = doc
	twoCT.headers = [][2]string{{"Content-Type", "application/json"}}
	twoCT.ct = "text/plain"
	g.emit("w1-content-type", twoCT)

	for i := 0; i <= len(doc); i += max(1, len(doc)/24) {
		rq := g.baseRequest()
		rq.body = doc[:i]
		g.emit("w1-malformed", rq)
	}
	for _, s := range []string{
		`{"x" 1}`, `{'a': 1}`, `{"a": 1,}`, `[1,]`, `{"a": "\x"}`, `{"a": "\u12"}`, "{\"a\": \"\x01\"}", `{a: 1}`,
		`{"interests": [] // c`, `{"interests": []} x`, `{"interests": [nan]}`, `{"interests": -}`, `{"interests": 01}`,
		`{"interests": 1.}`, `{"interests": .5}`, `{"interests": tru}`, "\ufeff{}", `{"from_areas": ["a"]`, `"`,
	} {
		rq := g.baseRequest()
		rq.body = s
		g.emit("w1-malformed", rq)
		if g.needsActor() {
			rq.anon = true
			g.emit("w1-malformed-anonymous", rq)
		}
	}
	for _, enc := range []func(string) string{
		func(s string) string { return "\xef\xbb\xbf" + s },
		utf16LE, utf16BE, func(s string) string { return "\xff\xfe" + utf16LE(s) }, func(s string) string { return "\xfe\xff" + utf16BE(s) },
		func(s string) string { return s[:len(s)-1] + "\xff" + s[len(s)-1:] },
		func(string) string { return strings.Repeat("[", 20000) },
		func(string) string { return `{"a": [` + strings.Repeat("9", 4300) + `]}` },
		func(string) string { return `{"a": [` + strings.Repeat("9", 4301) + `]}` },
		func(string) string { return "{\"a\": \"\xed\xa0\x80\"}" },
	} {
		rq := g.baseRequest()
		rq.body = enc(doc)
		g.emit("w1-encoding", rq)
	}
	for _, s := range []string{"NaN", "Infinity", "-Infinity", "1e400", "-0.0", "1E2", "true", "null"} {
		rq := g.baseRequest()
		rq.body = strings.Replace(doc, ": ", ": "+s+", \"zz\": ", 1)
		g.emit("w1-json-values", rq)
	}
	// Duplicate keys: the last value wins, the first position stays.
	if m, ok := decodeObject(doc); ok {
		for _, k := range m.Keys() {
			v, _ := m.Get(k)
			rq := g.baseRequest()
			rq.body = "{" + dumps(pyjson.String(k)) + ": 1, " + strings.TrimPrefix(dumps(m), "{")
			g.emit("w1-duplicate-keys", rq)
			rq2 := g.baseRequest()
			rq2.body = strings.TrimSuffix(dumps(m), "}") + ", " + dumps(pyjson.String(k)) + ": " + dumps(v) + "}"
			g.emit("w1-duplicate-keys", rq2)
		}
	}
	// Code point lengths around the note limit (and any other max_length).
	for _, n := range []int{0, 1, 499, 500, 501} {
		for _, unit := range []string{"a", "\u00e9", "\u4e2d", "\U0001F600", "e\u0301"} {
			for _, pad := range []string{"", " "} {
				m, ok := decodeObject(doc)
				if !ok || g.route.ID != "POST /reports" {
					continue
				}
				note := strings.Repeat(unit, n) + pad
				m.Set("note", pyjson.String(note))
				rq := g.baseRequest()
				rq.body = dumps(m)
				g.emit("w1-lengths", rq)
			}
		}
	}
	if _, ok := g.route.root.params["path"]; ok {
		for i := 0; i < 40; i++ {
			rq := g.baseRequest()
			if rq.params["context_id"] = g.uuidSpelling(i%3 == 0); rq.params["context_id"] == "" {
				rq.params["context_id"] = "g"
			}
			if i%6 == 0 {
				rq.body = `{"from_areas": 5, "x": 1}`
			}
			if i%9 == 0 {
				rq.anon = true
			}
			g.emit("w1-uuid-path", rq)
		}
	}
	for i := 0; i < 120; i++ {
		rq := g.baseRequest()
		rq.body = dumpsDoc(g.faultyDoc(bp, 1+g.r.IntN(3)))
		g.emit("w1-random-faults", rq)
	}
}

func decodeObject(doc string) (*pyjson.OrderedMap, bool) {
	v, err := pyjson.Loads([]byte(doc))
	if err != nil {
		return nil, false
	}
	m, ok := v.(*pyjson.OrderedMap)
	return m, ok
}

func utf16LE(s string) string {
	var b []byte
	for _, r := range s {
		if r >= 0x10000 {
			r -= 0x10000
			hi, lo := 0xD800+(r>>10), 0xDC00+(r&0x3FF)
			b = append(b, byte(hi), byte(hi>>8), byte(lo), byte(lo>>8))
			continue
		}
		b = append(b, byte(r), byte(r>>8))
	}
	return string(b)
}

func utf16BE(s string) string {
	le := []byte(utf16LE(s))
	for i := 0; i+1 < len(le); i += 2 {
		le[i], le[i+1] = le[i+1], le[i]
	}
	return string(le)
}

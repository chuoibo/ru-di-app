// Package runner executes scenarios against a stack and compares stacks.
//
// Each stack gets its own placeholder binder: ids and timestamps are numbered
// by what that stack returned, in scenario order, and only then are the two
// normalised transcripts compared step by step.
package runner

import (
	"context"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5"

	"mobile/parity/internal/compare"
	"mobile/parity/internal/dbsnap"
	"mobile/parity/internal/httpclient"
	"mobile/parity/internal/mediasnap"
	"mobile/parity/internal/normalize"
	"mobile/parity/internal/scenario"
	"mobile/parity/internal/tap"
)

// DefaultRoles is what apps/mobile sends in dev mode (src/danh-tinh.ts).
var DefaultRoles = []string{"member", "advancer", "recipient", "batch_owner"}

// personaNamespace makes persona ids deterministic per scenario and distinct
// across scenarios, so state from one scenario never answers for another.
var personaNamespace = [16]byte{0x5f, 0x0e, 0x4c, 0x2a, 0x9b, 0x1d, 0x4e, 0x7a, 0x8c, 0x33, 0x61, 0x02, 0xd4, 0x9e, 0x5b, 0x17}

// Stack is one running system under test.
type Stack struct {
	Name   string
	Client *httpclient.Client
	// DB is the stack's own database. When set, every step is followed by a
	// snapshot, so the comparison covers what was written, not only what was
	// answered. nil compares the wire alone.
	DB dbsnap.Conn
	// Tap, when set, reads which requests reached this stack's Python.
	Tap *tap.Client
	// Python reaches this stack's Python without its front door, for steps
	// with via: python. A stack that is Python alone passes Client again.
	Python *httpclient.Client
	// Sessions is where prod-mode personas get their sessions when DB is nil:
	// the canary compares the wire only, yet its personas must still sign in.
	Sessions dbsnap.Conn
	// Media is the directory of the stack's photo store, the one every process
	// of the stack writes. When set, every step is followed by a snapshot of
	// the store, so the comparison covers the files written and removed. ""
	// leaves the store out.
	Media string
	// MediaCache is what the store's last snapshot saw. One cache per store,
	// shared by every Execute on that stack, spares each snapshot the files no
	// step touched; without one a scenario keeps its own and reads the whole
	// store once at its baseline.
	MediaCache *mediasnap.Cache
}

// ErrSetup marks a failure before any step ran. It is never a difference: a
// canary that counted a failed seed as "damage caught" would be green blind.
var ErrSetup = errors.New("parity setup")

// StepResult is one step's response, raw and normalised.
type StepResult struct {
	StepID string
	// Path is the scenario's declared request path, carried so an accepted
	// divergence can be scoped to the route it was decided for.
	Path string
	Raw  httpclient.Response
	Norm compare.Exchange
	// Change is what the step wrote to the stack's database; nil without a DB.
	Change     *dbsnap.Change
	NormChange *dbsnap.Change
	// Media is what the step did to the stack's photo store; nil without one.
	Media     *mediasnap.Change
	NormMedia *mediasnap.Change
	// PythonRequests counts the requests that reached Python during the step;
	// -1 on a stack without a tap.
	PythonRequests int
	// Burst holds every response of a concurrent step in a stack-independent
	// order (see send); Raw and Norm are unused for such a step.
	Burst     []httpclient.Response
	BurstNorm []compare.Exchange
}

// Run is one scenario's transcript on one stack.
type Run struct {
	ScenarioID string
	Stack      string
	Steps      []StepResult
}

// Execute runs every step of sc against stack. A transport failure is an
// infrastructure error, never a difference.
//
// nonce is shared by both stacks of one comparison and fresh for every
// comparison. Persona ids and session tokens derive from it, so each run starts
// with people the stack has never seen. With ids fixed per scenario, a second
// run met the idempotency keys and rows the first had left, and the gate's
// database lane, which runs after the canary, only ever saw "key reused".
func Execute(ctx context.Context, sc *scenario.Scenario, stack Stack, nonce string) (*Run, error) {
	if nonce == "" {
		return nil, fmt.Errorf("%w: %s: empty run nonce", ErrSetup, sc.ID)
	}
	for _, step := range sc.Steps {
		if step.Via == scenario.ViaPython && stack.Python == nil {
			// Sending it through the front door instead would compare a
			// same-implementation replay and call it a cross replay.
			return nil, fmt.Errorf("%w: %s on %s: step %s is via python and the stack has no Python client", ErrSetup, sc.ID, stack.Name, step.ID)
		}
	}
	scope := sc.ID + "@" + nonce
	binder := normalize.NewBinder()
	vars := map[string]string{}
	personaNames := make([]string, 0, len(sc.Personas))
	for name := range sc.Personas {
		personaNames = append(personaNames, name)
	}
	sort.Strings(personaNames)
	for _, name := range personaNames {
		id := PersonaID(scope, name)
		vars["persona."+name] = id
		if err := binder.Name(id, "persona:"+name); err != nil {
			return nil, err
		}
	}

	if sc.AuthMode == "prod" && len(personaNames) > 0 {
		sessions := stack.DB
		if sessions == nil {
			sessions = stack.Sessions
		}
		if sessions == nil {
			return nil, fmt.Errorf("%w: %s: prod-mode personas need each stack's database to seed sessions into", ErrSetup, sc.ID)
		}
		for _, name := range personaNames {
			token := SessionToken(scope, name)
			vars["token."+name] = token
			if err := binder.Name(token, "token:session-"+name); err != nil {
				return nil, err
			}
			digest := sha256.Sum256([]byte(token))
			if err := binder.Name(hex.EncodeToString(digest[:]), "token-digest:session-"+name); err != nil {
				return nil, err
			}
		}
		// Seeded before the baseline snapshot, so the seed is never counted as
		// something a step wrote.
		if err := seedSessions(ctx, scope, sessions, personaNames); err != nil {
			return nil, fmt.Errorf("%w: %s on %s: seeding sessions: %v", ErrSetup, sc.ID, stack.Name, err)
		}
	}

	run := &Run{ScenarioID: sc.ID, Stack: stack.Name}
	var prev *dbsnap.Snap
	if stack.DB != nil {
		snap, err := dbsnap.Snapshot(ctx, stack.DB)
		if err != nil {
			return nil, fmt.Errorf("%s on %s: baseline snapshot: %w", sc.ID, stack.Name, err)
		}
		prev = snap
	}
	var prevMedia *mediasnap.Snap
	mediaCache := stack.MediaCache
	if stack.Media != "" {
		if mediaCache == nil {
			mediaCache = mediasnap.NewCache()
		}
		snap, err := mediaCache.Snapshot(stack.Media)
		if err != nil {
			return nil, fmt.Errorf("%s on %s: baseline media snapshot: %w", sc.ID, stack.Name, err)
		}
		prevMedia = snap
	}
	tapSeq := 0
	if stack.Tap != nil {
		last, err := stack.Tap.Last(ctx)
		if err != nil {
			return nil, fmt.Errorf("%w: %s on %s: tap: %v", ErrSetup, sc.ID, stack.Name, err)
		}
		tapSeq = last
	}
	// A limiter-lane scenario must finish inside the product's own 60 s window,
	// and the snapshot cost grows with the database. In the gate the limiter
	// lane runs LAST, against a database holding every earlier scenario's rows,
	// and the 36 snapshots taken BETWEEN the requests pushed them past the
	// boundary. Measured 20/09: the same scenario is EQUAL on fresh stacks with
	// the lane on (63 s including the wait for a window) and overruns on gate
	// stacks on both attempts.
	//
	// The limiter counts REQUESTS, so the snapshots move to the end of the
	// scenario. What is given up, on purpose and only in this lane: the delta
	// no longer says WHICH step wrote a row, only that the scenario as a whole
	// wrote what the other side wrote. What a refused request must not write is
	// still caught, because the end state would carry it.
	snapEachStep := sc.Lane != scenario.LaneLimiter
	for stepIndex, step := range sc.Steps {
		lastStep := stepIndex == len(sc.Steps)-1
		snapNow := snapEachStep || lastStep
		client := stack.Client
		if step.Via == scenario.ViaPython {
			client = stack.Python
		}
		mark := binder.InstantMark()
		responses, err := send(ctx, client, sc, step, vars)
		if err != nil {
			return nil, fmt.Errorf("%s on %s step %s: %w", sc.ID, stack.Name, step.ID, err)
		}
		for _, resp := range responses {
			if err := binder.Observe(string(resp.Body)); err != nil {
				return nil, err
			}
			for _, values := range resp.Header {
				for _, value := range values {
					if err := binder.Observe(value); err != nil {
						return nil, err
					}
				}
			}
		}
		result := StepResult{StepID: step.ID, Path: step.Request.Path, PythonRequests: -1}
		if step.Concurrent > 0 {
			result.Burst = responses
		} else {
			result.Raw = responses[0]
			if err := capture(step, result.Raw, vars, binder); err != nil {
				return nil, fmt.Errorf("%s on %s step %s: %w", sc.ID, stack.Name, step.ID, err)
			}
		}
		if stack.Tap != nil {
			last, entries, err := stack.Tap.Since(ctx, tapSeq)
			if err != nil {
				return nil, fmt.Errorf("%s on %s step %s: tap: %w", sc.ID, stack.Name, step.ID, err)
			}
			tapSeq = last
			result.PythonRequests = len(entries)
		}
		if stack.DB != nil && snapNow {
			next, err := dbsnap.Snapshot(ctx, stack.DB)
			if err != nil {
				return nil, fmt.Errorf("%s on %s step %s: snapshot: %w", sc.ID, stack.Name, step.ID, err)
			}
			result.Change = dbsnap.Delta(prev, next)
			prev = next
			// The response was observed above and the database comes after it,
			// so an id first returned in a body keeps that body's number. Rows
			// equal once masked are told apart by the values already numbered
			// (see Binder.ObserveGroups), not by their random ids.
			if err := binder.ObserveGroups(result.Change.Groups()); err != nil {
				return nil, err
			}
		}
		if stack.Media != "" && snapNow {
			next, err := mediaCache.Snapshot(stack.Media)
			if err != nil {
				return nil, fmt.Errorf("%s on %s step %s: media snapshot: %w", sc.ID, stack.Name, step.ID, err)
			}
			result.Media = mediasnap.Delta(prevMedia, next)
			prevMedia = next
			// After the database, as the database comes after the response: a
			// key an uploaded_images row names keeps that row's number, so its
			// file is tied to the row. Only a file no row names is numbered here.
			if err := binder.ObserveGroups(result.Media.Groups()); err != nil {
				return nil, err
			}
		}
		if step.Concurrent > 0 {
			if err := binder.TieInstantsSince(mark); err != nil {
				return nil, err
			}
		}
		run.Steps = append(run.Steps, result)
	}
	for i := range run.Steps {
		if run.Steps[i].Burst != nil {
			for _, raw := range run.Steps[i].Burst {
				run.Steps[i].BurstNorm = append(run.Steps[i].BurstNorm, normaliseExchange(binder, run.Steps[i].Path, raw))
			}
		} else {
			run.Steps[i].Norm = normaliseExchange(binder, run.Steps[i].Path, run.Steps[i].Raw)
		}
		if run.Steps[i].Change != nil {
			run.Steps[i].NormChange = run.Steps[i].Change.Normalise(binder.Apply)
		}
		if run.Steps[i].Media != nil {
			run.Steps[i].NormMedia = run.Steps[i].Media.Normalise(binder)
		}
	}
	return run, nil
}

func buildRequest(sc *scenario.Scenario, step scenario.Step, vars map[string]string) (httpclient.Request, error) {
	path, err := scenario.Render(step.Request.Path, vars)
	if err != nil {
		return httpclient.Request{}, err
	}
	header := http.Header{}
	if sc.AuthMode == "dev" && step.As != scenario.Anonymous {
		roles := sc.Personas[step.As].Roles
		if len(roles) == 0 {
			roles = DefaultRoles
		}
		header.Set("X-Actor-ID", vars["persona."+step.As])
		header.Set("X-Actor-Roles", strings.Join(roles, ","))
	}
	if sc.AuthMode == "prod" && step.As != scenario.Anonymous {
		header.Set("Authorization", "Bearer "+vars["token."+step.As])
	}
	for name, value := range step.Request.Headers {
		rendered, err := scenario.Render(value, vars)
		if err != nil {
			return httpclient.Request{}, err
		}
		// A scenario's own header wins, so junk X-Actor-* values can be tested.
		header.Set(name, rendered)
	}
	var body []byte
	if step.Request.BodyRaw != nil {
		rendered, err := scenario.Render(*step.Request.BodyRaw, vars)
		if err != nil {
			return httpclient.Request{}, err
		}
		body = []byte(rendered)
	}
	if step.Request.BodyParts != nil {
		// The boundary is read back from the header the request carries, so
		// the body and the header can never name two different boundaries.
		boundary, err := scenario.Boundary(header.Get("Content-Type"))
		if err != nil {
			return httpclient.Request{}, err
		}
		if body, err = multipartBody(boundary, *step.Request.BodyParts, vars); err != nil {
			return httpclient.Request{}, err
		}
	}
	return httpclient.Request{Method: step.Request.Method, Path: path, Header: header, Body: body}, nil
}

func capture(step scenario.Step, resp httpclient.Response, vars map[string]string, binder *normalize.Binder) error {
	names := make([]string, 0, len(step.Bind))
	for name := range step.Bind {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		bind := step.Bind[name]
		var value string
		switch bind.From {
		case "body":
			v, err := pointer(resp.Body, bind.Pointer)
			if err != nil {
				return fmt.Errorf("bind %s: %w", name, err)
			}
			value = v
		case "header":
			value = resp.Header.Get(bind.Name)
		}
		if bind.Regex != "" {
			match := regexp.MustCompile(bind.Regex).FindStringSubmatch(value)
			if match == nil {
				return fmt.Errorf("bind %s: regex %q did not match %q", name, bind.Regex, value)
			}
			value = match[1]
		}
		if value == "" {
			return fmt.Errorf("bind %s: empty value (status %d)", name, resp.Status)
		}
		vars[name] = value
		if bind.Class == "token" {
			if err := binder.Name(value, "token:"+name); err != nil {
				return err
			}
			// Services keep sha256(token) as bytea, which row_to_json renders as
			// "\\x<hex>"; naming the hex keeps those rows comparable too.
			digest := sha256.Sum256([]byte(value))
			if err := binder.Name(hex.EncodeToString(digest[:]), "token-digest:"+name); err != nil {
				return err
			}
		}
	}
	return nil
}

// pointer resolves an RFC 6901 JSON pointer to a string or number literal.
func pointer(body []byte, ptr string) (string, error) {
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.UseNumber()
	var doc any
	if err := decoder.Decode(&doc); err != nil {
		return "", fmt.Errorf("body is not JSON: %w", err)
	}
	current := doc
	for _, raw := range strings.Split(ptr, "/")[1:] {
		token := strings.ReplaceAll(strings.ReplaceAll(raw, "~1", "/"), "~0", "~")
		switch node := current.(type) {
		case map[string]any:
			next, ok := node[token]
			if !ok {
				return "", fmt.Errorf("pointer %s: no key %q", ptr, token)
			}
			current = next
		case []any:
			index, err := strconv.Atoi(token)
			if err != nil || index < 0 || index >= len(node) {
				return "", fmt.Errorf("pointer %s: bad index %q", ptr, token)
			}
			current = node[index]
		default:
			return "", fmt.Errorf("pointer %s: cannot descend into %T", ptr, current)
		}
	}
	switch value := current.(type) {
	case string:
		return value, nil
	case json.Number:
		return value.String(), nil
	default:
		return "", fmt.Errorf("pointer %s: value is %T, not a string or number", ptr, current)
	}
}

// StepDiff lists the differences in one step: on the wire, in the database and
// in the photo store.
type StepDiff struct {
	StepID      string
	Differences []compare.Difference
	Database    []dbsnap.Difference
	Media       []mediasnap.Difference
}

// DatabaseLaneMismatch marks a step where only one stack was snapshotted. A
// comparison that quietly skipped the database there would read as equal.
const DatabaseLaneMismatch = "database-lane-on-one-side"

// MediaLaneMismatch is DatabaseLaneMismatch for the photo store.
const MediaLaneMismatch = "media-lane-on-one-side"

// Diff compares two transcripts of the same scenario step by step.
func Diff(reference, candidate *Run) []StepDiff {
	var out []StepDiff
	for i := range reference.Steps {
		ref, cand := reference.Steps[i], candidate.Steps[i]
		var diffs []compare.Difference
		if ref.Burst != nil || cand.Burst != nil {
			diffs = compareBursts(ref.BurstNorm, cand.BurstNorm)
		} else {
			diffs = compare.Step(ref.Norm, cand.Norm)
		}
		var database []dbsnap.Difference
		switch {
		case ref.NormChange != nil && cand.NormChange != nil:
			database = dbsnap.Compare(ref.NormChange, cand.NormChange)
		case (ref.NormChange == nil) != (cand.NormChange == nil):
			database = []dbsnap.Difference{{Kind: DatabaseLaneMismatch}}
		}
		var media []mediasnap.Difference
		switch {
		case ref.NormMedia != nil && cand.NormMedia != nil:
			media = mediasnap.Compare(ref.NormMedia, cand.NormMedia)
		case (ref.NormMedia == nil) != (cand.NormMedia == nil):
			media = []mediasnap.Difference{{Kind: MediaLaneMismatch}}
		}
		if len(diffs) > 0 || len(database) > 0 || len(media) > 0 {
			out = append(out, StepDiff{StepID: ref.StepID, Differences: diffs, Database: database, Media: media})
		}
	}
	return out
}

// RoutesNotServedInCore lists the routes a scenario names that the candidate
// serves in Go although no step ADDRESSING THAT ROUTE was answered without
// Python. When Go serves a route a scenario exercises, at least one step whose
// method and path match that route must have been answered in core, or the Go
// code is not what answered.
//
// The earlier version asked this once per scenario rather than once per route:
// any single step answered in core credited every route the file named. Its own
// comment said so -- "a scenario naming two served routes passes on a step of
// either" -- and that is a hole a file cannot fall into by accident but can be
// walked into on purpose. `/static` is the case that forced this: the guest
// pages are Go's and their stylesheet is not, and both live in one scenario, so
// moving `/static` to Go would have been credited by the guest page's own steps
// while every stylesheet fetch still fell through to Python. The gate would
// have gone green over an unmoved route.
//
// Still a floor, and the remaining slack is named rather than hidden: a step
// whose path matches two declared routes credits both, because the scenario
// does not know Starlette's registration order and cannot tell which of them
// actually answered.
func RoutesNotServedInCore(sc *scenario.Scenario, run *Run, served map[string]bool) []string {
	answeredInCore := map[string]bool{}
	for _, step := range run.Steps {
		if step.PythonRequests == 0 {
			answeredInCore[step.StepID] = true
		}
	}
	var missing []string
	for _, id := range sc.Routes {
		if !served[id] {
			continue
		}
		matches := routePattern(id)
		if matches == nil {
			continue
		}
		hit := false
		for _, step := range sc.Steps {
			if !answeredInCore[step.ID] {
				continue
			}
			if matches.MatchString(step.Request.Method + " " + stepPath(step.Request.Path)) {
				hit = true
				break
			}
		}
		if !hit {
			missing = append(missing, id)
		}
	}
	return missing
}

// routePattern turns `GET /a/{id}/b` into a matcher for a step's method and
// path. A `{{binding}}` in the step fills exactly one segment, so both kinds of
// placeholder become the same wildcard.
//
// A MOUNT is not a route and must not be matched like one. `MOUNT /static`
// answers ANY method on ANY path beneath its prefix, and no step will ever
// carry the literal method "MOUNT" -- the corpus reaches it with
// `GET /static/khong-co.css`. Comparing the method strictly would report a
// mounted prefix as never served the moment it moved to Go, which is a red
// with nothing behind it.
func routePattern(routeID string) *regexp.Regexp {
	method, path, ok := strings.Cut(routeID, " ")
	if !ok {
		return nil
	}
	if method == "MOUNT" {
		rx, err := regexp.Compile(`^[A-Z]+ ` + regexp.QuoteMeta(strings.TrimSuffix(path, "/")) + `(/.*)?$`)
		if err != nil {
			return nil
		}
		return rx
	}
	parts := strings.Split(path, "/")
	for i, seg := range parts {
		if strings.HasPrefix(seg, "{") {
			parts[i] = "[^/]+"
		} else {
			parts[i] = regexp.QuoteMeta(seg)
		}
	}
	rx, err := regexp.Compile("^" + regexp.QuoteMeta(method) + " " + strings.Join(parts, "/") + "$")
	if err != nil {
		return nil
	}
	return rx
}

// stepPath is a step's path with the query string dropped and every
// `{{binding}}` collapsed to one segment's worth of wildcard-matchable text.
func stepPath(path string) string {
	if i := strings.IndexByte(path, '?'); i >= 0 {
		path = path[:i]
	}
	return bindingRef.ReplaceAllString(path, "x")
}

var bindingRef = regexp.MustCompile(`\{\{[^}]+\}\}`)

// AcceptedCounts counts, per name, the steps where the two transcripts differ
// in a way ADR-0029 §2.4 accepts.
func AcceptedCounts(reference, candidate *Run) map[string]int {
	counts := map[string]int{}
	for i := range reference.Steps {
		if i >= len(candidate.Steps) {
			break
		}
		for _, name := range compare.Accepted(reference.Steps[i].Norm, candidate.Steps[i].Norm) {
			counts[name]++
		}
	}
	return counts
}

// PersonasRefused reports whether every step sent as a persona was answered
// 401. Such a transcript is equal on both sides and proves nothing: the seeded
// sessions went to another database, or dev personas met prod stacks. It does
// not catch prod personas on dev stacks, where the session routes still read
// the bearer; the CLI's auth mode check covers that.
func PersonasRefused(sc *scenario.Scenario, run *Run) bool {
	persona := 0
	for i, step := range sc.Steps {
		if step.As == scenario.Anonymous || i >= len(run.Steps) {
			continue
		}
		persona++
		if run.Steps[i].Raw.Status != 401 {
			return false
		}
	}
	return persona > 0
}

// PersonaID is a name-based (version 5) UUID for a persona within a run scope
// (scenario id and run nonce).
func PersonaID(scenarioID, persona string) string {
	h := sha1.New()
	h.Write(personaNamespace[:])
	h.Write([]byte(scenarioID + "/" + persona))
	sum := h.Sum(nil)
	var u [16]byte
	copy(u[:], sum[:16])
	u[6] = (u[6] & 0x0f) | 0x50
	u[8] = (u[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", u[0:4], u[4:6], u[6:8], u[8:10], u[10:16])
}

// SessionToken is a prod persona's bearer token: deterministic per run scope and
// persona, 43 base64url characters like secrets.token_urlsafe(32). It opens a
// session only in a disposable parity database.
func SessionToken(scenarioID, persona string) string {
	sum := sha256.Sum256([]byte("parity-session:" + scenarioID + "/" + persona))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// seedSessions gives every prod persona a people row and a live session,
// written the same way into each stack's database. issued_via is 'genesis',
// the one door that needs no invite row. A rerun (the canary, then the run)
// puts the session back to live, so a step that revoked it last time does not
// quietly turn this run's persona steps into 401s on both sides.
func seedSessions(ctx context.Context, scope string, conn dbsnap.Conn, names []string) error {
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for _, name := range names {
		person := PersonaID(scope, name)
		digest := sha256.Sum256([]byte(SessionToken(scope, name)))
		if _, err := tx.Exec(ctx,
			`INSERT INTO people (id, display_name) VALUES ($1, $2)
			 ON CONFLICT (id) DO UPDATE SET display_name = EXCLUDED.display_name, deleted_at = NULL`,
			person, name); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO account_sessions (id, person_id, token_digest, issued_via, expires_at)
			 VALUES ($1, $2, $3, 'genesis', now() + interval '30 days')
			 ON CONFLICT (id) DO UPDATE SET revoked_at = NULL, expires_at = EXCLUDED.expires_at`,
			PersonaID(scope, name+"/session"), person, digest[:]); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// NewNonce returns a fresh run nonce for Execute.
func NewNonce() string {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}

// send issues a step's request or, for a concurrent step, all its copies at
// once: every copy is built first, then released together. Responses come back
// ordered by their masked text, copies alike in it staying in copy order. Ids
// are numbered in the order responses are observed, so that order must follow
// neither arrival nor which copy won a race: both differ between stacks.
// Execute gives the instants a burst wrote one rank for the same reason.
func send(ctx context.Context, client *httpclient.Client, sc *scenario.Scenario, step scenario.Step, vars map[string]string) ([]httpclient.Response, error) {
	if step.Concurrent == 0 {
		req, err := buildRequest(sc, step, vars)
		if err != nil {
			return nil, err
		}
		resp, err := client.Do(ctx, req)
		if err != nil {
			return nil, err
		}
		return []httpclient.Response{resp}, nil
	}
	requests := make([]httpclient.Request, step.Concurrent)
	for i := range requests {
		copyVars := maps.Clone(vars)
		copyVars[scenario.BurstVar] = strconv.Itoa(i + 1)
		req, err := buildRequest(sc, step, copyVars)
		if err != nil {
			return nil, err
		}
		requests[i] = req
	}
	responses := make([]httpclient.Response, len(requests))
	errs := make([]error, len(requests))
	release := make(chan struct{})
	var wg sync.WaitGroup
	for i := range requests {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-release
			responses[i], errs[i] = client.Do(ctx, requests[i])
		}(i)
	}
	close(release)
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	keys := make([]string, len(responses))
	for i, resp := range responses {
		header := http.Header{}
		for name, values := range resp.Header {
			for _, value := range values {
				header[name] = append(header[name], normalize.Mask(value))
			}
		}
		keys[i] = exchangeText(compare.Exchange{Status: resp.Status, Header: header, Body: normalize.Mask(string(resp.Body))})
	}
	order := make([]int, len(responses))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool {
		return keys[order[a]] < keys[order[b]]
	})
	sorted := make([]httpclient.Response, len(responses))
	for k, i := range order {
		sorted[k] = responses[i]
	}
	return sorted, nil
}

// exchangeText renders an exchange for ordering: status, the headers the
// comparator does not ignore, sorted, then the body.
func exchangeText(exchange compare.Exchange) string {
	lines := make([]string, 0, len(exchange.Header))
	for name, values := range exchange.Header {
		lower := strings.ToLower(name)
		if compare.Volatile[lower] {
			continue
		}
		lines = append(lines, lower+": "+strings.Join(values, "\x00"))
	}
	sort.Strings(lines)
	return fmt.Sprintf("%03d\n%s\n\n%s", exchange.Status, strings.Join(lines, "\n"), exchange.Body)
}

func normaliseExchange(binder *normalize.Binder, path string, raw httpclient.Response) compare.Exchange {
	header := http.Header{}
	for name, values := range raw.Header {
		for _, value := range values {
			if strings.EqualFold(name, "etag") {
				// An ETag is a validator, not an id: it must be COMPARED.
				// Masked, a 32-hex digest is numbered <hex32#n> per stack, so
				// two DIFFERENT digests both become <hex32#1> and the etag
				// silently stops being compared at all -- the same shape of
				// hole <digest#n> once gave fingerprint drift. Measured: with
				// the mask on, Python's mtime-derived etag and Go's
				// content-derived etag compared equal on every /static step.
				header[name] = append(header[name], value)
				continue
			}
			header[name] = append(header[name], binder.Apply(value))
		}
	}
	return compare.Exchange{Path: path, Status: raw.Status, Header: header, Body: binder.Apply(string(raw.Body))}
}

// compareBursts compares two bursts response by response in their ordered form.
func compareBursts(reference, candidate []compare.Exchange) []compare.Difference {
	if len(reference) != len(candidate) {
		return []compare.Difference{{Part: "burst size", Reference: strconv.Itoa(len(reference)), Candidate: strconv.Itoa(len(candidate))}}
	}
	var out []compare.Difference
	for i := range reference {
		for _, difference := range compare.Step(reference[i], candidate[i]) {
			difference.Part = fmt.Sprintf("response %d/%d %s", i+1, len(reference), difference.Part)
			out = append(out, difference)
		}
	}
	return out
}

// Closest compares candidate with each reference run of one scenario and
// returns no differences and the index of the first run it matches exactly, or
// the differences against the first run and -1 when it matches none. A
// concurrent step can race on Python itself; the candidate must then reproduce
// one outcome Python produced, on the wire and in the database together.
func Closest(references []*Run, candidate *Run) ([]StepDiff, int) {
	for i, reference := range references {
		if len(Diff(reference, candidate)) == 0 {
			return nil, i
		}
	}
	return Diff(references[0], candidate), -1
}

// RacySteps lists the steps whose outcome differed between reference runs of
// one scenario, in step order.
func RacySteps(references []*Run) []string {
	racy := map[string]bool{}
	for _, other := range references[1:] {
		for _, d := range Diff(references[0], other) {
			racy[d.StepID] = true
		}
	}
	var out []string
	for _, step := range references[0].Steps {
		if racy[step.StepID] {
			out = append(out, step.StepID)
		}
	}
	return out
}

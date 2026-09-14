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
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"

	"mobile/parity/internal/compare"
	"mobile/parity/internal/dbsnap"
	"mobile/parity/internal/httpclient"
	"mobile/parity/internal/normalize"
	"mobile/parity/internal/scenario"
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
	// Sessions is where prod-mode personas get their sessions when DB is nil:
	// the canary compares the wire only, yet its personas must still sign in.
	Sessions dbsnap.Conn
}

// ErrSetup marks a failure before any step ran. It is never a difference: a
// canary that counted a failed seed as "damage caught" would be green blind.
var ErrSetup = errors.New("parity setup")

// StepResult is one step's response, raw and normalised.
type StepResult struct {
	StepID string
	Raw    httpclient.Response
	Norm   compare.Exchange
	// Change is what the step wrote to the stack's database; nil without a DB.
	Change     *dbsnap.Change
	NormChange *dbsnap.Change
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
	for _, step := range sc.Steps {
		req, err := buildRequest(sc, step, vars)
		if err != nil {
			return nil, fmt.Errorf("%s step %s: %w", sc.ID, step.ID, err)
		}
		resp, err := stack.Client.Do(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("%s on %s step %s: %w", sc.ID, stack.Name, step.ID, err)
		}
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
		if err := capture(step, resp, vars, binder); err != nil {
			return nil, fmt.Errorf("%s on %s step %s: %w", sc.ID, stack.Name, step.ID, err)
		}
		result := StepResult{StepID: step.ID, Raw: resp}
		if stack.DB != nil {
			next, err := dbsnap.Snapshot(ctx, stack.DB)
			if err != nil {
				return nil, fmt.Errorf("%s on %s step %s: snapshot: %w", sc.ID, stack.Name, step.ID, err)
			}
			result.Change = dbsnap.Delta(prev, next)
			prev = next
			// The response was observed above and the database comes after it,
			// so an id first returned in a body keeps that body's number.
			for _, text := range result.Change.Texts() {
				if err := binder.Observe(text); err != nil {
					return nil, err
				}
			}
		}
		run.Steps = append(run.Steps, result)
	}
	for i := range run.Steps {
		raw := run.Steps[i].Raw
		header := http.Header{}
		for name, values := range raw.Header {
			for _, value := range values {
				header[name] = append(header[name], binder.Apply(value))
			}
		}
		run.Steps[i].Norm = compare.Exchange{
			Status: raw.Status,
			Header: header,
			Body:   binder.Apply(string(raw.Body)),
		}
		if run.Steps[i].Change != nil {
			run.Steps[i].NormChange = run.Steps[i].Change.Normalise(binder.Apply)
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

// StepDiff lists the differences in one step: on the wire and in the database.
type StepDiff struct {
	StepID      string
	Differences []compare.Difference
	Database    []dbsnap.Difference
}

// DatabaseLaneMismatch marks a step where only one stack was snapshotted. A
// comparison that quietly skipped the database there would read as equal.
const DatabaseLaneMismatch = "database-lane-on-one-side"

// Diff compares two transcripts of the same scenario step by step.
func Diff(reference, candidate *Run) []StepDiff {
	var out []StepDiff
	for i := range reference.Steps {
		ref, cand := reference.Steps[i], candidate.Steps[i]
		diffs := compare.Step(ref.Norm, cand.Norm)
		var database []dbsnap.Difference
		switch {
		case ref.NormChange != nil && cand.NormChange != nil:
			database = dbsnap.Compare(ref.NormChange, cand.NormChange)
		case (ref.NormChange == nil) != (cand.NormChange == nil):
			database = []dbsnap.Difference{{Kind: DatabaseLaneMismatch}}
		}
		if len(diffs) > 0 || len(database) > 0 {
			out = append(out, StepDiff{StepID: ref.StepID, Differences: diffs, Database: database})
		}
	}
	return out
}

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

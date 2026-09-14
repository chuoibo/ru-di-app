// Package runner executes scenarios against a stack and compares stacks.
//
// Each stack gets its own placeholder binder: ids and timestamps are numbered
// by what that stack returned, in scenario order, and only then are the two
// normalised transcripts compared step by step.
package runner

import (
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"

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
}

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
func Execute(ctx context.Context, sc *scenario.Scenario, stack Stack) (*Run, error) {
	binder := normalize.NewBinder()
	vars := map[string]string{}
	personaNames := make([]string, 0, len(sc.Personas))
	for name := range sc.Personas {
		personaNames = append(personaNames, name)
	}
	sort.Strings(personaNames)
	for _, name := range personaNames {
		id := PersonaID(sc.ID, name)
		vars["persona."+name] = id
		if err := binder.Name(id, "persona:"+name); err != nil {
			return nil, err
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
		return httpclient.Request{}, fmt.Errorf("prod-mode personas need seeded sessions, which this harness does not create yet")
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

// PersonaID is a name-based (version 5) UUID for a scenario's persona.
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

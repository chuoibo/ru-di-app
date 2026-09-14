// Package scenario loads parity scenarios.
//
// A scenario lists requests and nothing else. There is no field for an
// expected status, header or body, and the loader refuses one if it is added:
// the expected answer is always whatever the Python reference returns. That
// is what lets a tester who never saw the implementation (agy, ADR-0010 §6.1)
// write scenarios without writing an oracle.
package scenario

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Scenario is one file.
type Scenario struct {
	ID       string             `yaml:"id"`
	Routes   []string           `yaml:"routes"`
	AuthMode string             `yaml:"auth_mode"`
	Personas map[string]Persona `yaml:"personas"`
	Steps    []Step             `yaml:"steps"`

	File string `yaml:"-"`
}

// Persona is a caller. In dev auth mode its id travels in X-Actor-ID.
type Persona struct {
	Roles []string `yaml:"roles"`
}

// Step is one request, optionally capturing values later steps use.
type Step struct {
	ID      string          `yaml:"id"`
	As      string          `yaml:"as"`
	Request Request         `yaml:"request"`
	Bind    map[string]Bind `yaml:"bind"`
	// Via is empty for the stack's front door. ViaPython sends the step to
	// the stack's Python without the front door, so a key one implementation
	// stored is replayed by the other; on a stack that is Python alone it
	// changes nothing.
	Via string `yaml:"via"`
}

// Request is sent verbatim after {{variable}} substitution.
type Request struct {
	Method  string            `yaml:"method"`
	Path    string            `yaml:"path"`
	Headers map[string]string `yaml:"headers"`
	BodyRaw *string           `yaml:"body_raw"`
}

// Bind captures a value from a response.
type Bind struct {
	From    string `yaml:"from"`    // body | header
	Pointer string `yaml:"pointer"` // JSON pointer, for from: body
	Name    string `yaml:"name"`    // header name, for from: header
	Regex   string `yaml:"regex"`   // optional; the first group is captured
	Class   string `yaml:"class"`   // uuid | token
}

// Anonymous is the caller with no credentials.
const Anonymous = "anonymous"

// ViaPython is the only Step.Via value.
const ViaPython = "python"

var (
	idPattern       = regexp.MustCompile(`^[a-z0-9][a-z0-9/_.-]*$`)
	namePattern     = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	templatePattern = regexp.MustCompile(`\{\{\s*([a-z0-9_.]+)\s*\}\}`)
	methods         = map[string]bool{"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true, "OPTIONS": true, "HEAD": true}
	// Keys that would carry an expected answer. Refused anywhere except as a
	// header name, with a message that says why rather than "unknown field".
	oracleKeys = map[string]bool{"expect": true, "expected": true, "expect_status": true, "assert": true,
		"assertions": true, "status": true, "response": true, "body": true, "want": true}
)

// Load reads and validates one scenario file.
func Load(path string) (*Scenario, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	sc, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	sc.File = path
	return sc, nil
}

// Parse decodes and validates scenario YAML.
func Parse(data []byte) (*Scenario, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	if err := refuseOracleKeys(&root, ""); err != nil {
		return nil, err
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	var sc Scenario
	if err := decoder.Decode(&sc); err != nil {
		return nil, err
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, errors.New("one scenario per file")
	}
	if err := sc.validate(); err != nil {
		return nil, err
	}
	return &sc, nil
}

func refuseOracleKeys(node *yaml.Node, parentKey string) error {
	if node.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(node.Content); i += 2 {
			key := node.Content[i].Value
			if parentKey != "headers" && oracleKeys[strings.ToLower(key)] {
				return fmt.Errorf("line %d: %q is not allowed: scenarios hold requests only, and the expected answer is whatever the Python reference returns",
					node.Content[i].Line, key)
			}
			if err := refuseOracleKeys(node.Content[i+1], key); err != nil {
				return err
			}
		}
		return nil
	}
	for _, child := range node.Content {
		if err := refuseOracleKeys(child, parentKey); err != nil {
			return err
		}
	}
	return nil
}

func (sc *Scenario) validate() error {
	if !idPattern.MatchString(sc.ID) {
		return fmt.Errorf("id %q must match %s", sc.ID, idPattern)
	}
	if len(sc.Routes) == 0 {
		return errors.New("routes: list the routes this scenario exercises")
	}
	switch sc.AuthMode {
	case "dev", "prod":
	default:
		return fmt.Errorf("auth_mode %q must be dev or prod", sc.AuthMode)
	}
	for name, persona := range sc.Personas {
		if !namePattern.MatchString(name) || name == Anonymous {
			return fmt.Errorf("persona %q: names are lowercase identifiers other than %q", name, Anonymous)
		}
		if sc.AuthMode == "prod" && len(persona.Roles) > 0 {
			return fmt.Errorf("persona %q: in prod mode the server derives roles from the roster; remove them", name)
		}
	}
	if len(sc.Steps) == 0 {
		return errors.New("steps: at least one")
	}
	known := map[string]bool{}
	for name := range sc.Personas {
		known["persona."+name] = true
		if sc.AuthMode == "prod" {
			// A prod persona's bearer token, for steps that send it by hand.
			known["token."+name] = true
		}
	}
	stepIDs := map[string]bool{}
	for index, step := range sc.Steps {
		where := fmt.Sprintf("steps[%d] %q", index, step.ID)
		if !namePattern.MatchString(step.ID) || stepIDs[step.ID] {
			return fmt.Errorf("%s: step ids are unique lowercase identifiers", where)
		}
		stepIDs[step.ID] = true
		if step.As != Anonymous {
			if _, ok := sc.Personas[step.As]; !ok {
				return fmt.Errorf("%s: as %q is neither %q nor a persona", where, step.As, Anonymous)
			}
		}
		if step.Via != "" && step.Via != ViaPython {
			return fmt.Errorf("%s: via %q must be absent or %q", where, step.Via, ViaPython)
		}
		if !methods[step.Request.Method] {
			return fmt.Errorf("%s: method %q", where, step.Request.Method)
		}
		if !strings.HasPrefix(step.Request.Path, "/") {
			return fmt.Errorf("%s: path must start with /", where)
		}
		texts := []string{step.Request.Path}
		for _, value := range step.Request.Headers {
			texts = append(texts, value)
		}
		if step.Request.BodyRaw != nil {
			texts = append(texts, *step.Request.BodyRaw)
		}
		for _, text := range texts {
			for _, match := range templatePattern.FindAllStringSubmatch(text, -1) {
				if !known[match[1]] {
					return fmt.Errorf("%s: {{%s}} is not bound by an earlier step or persona", where, match[1])
				}
			}
		}
		names := make([]string, 0, len(step.Bind))
		for name := range step.Bind {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			bind := step.Bind[name]
			if !namePattern.MatchString(name) || known[name] {
				return fmt.Errorf("%s: bind %q must be a new lowercase identifier", where, name)
			}
			switch bind.From {
			case "body":
				if !strings.HasPrefix(bind.Pointer, "/") || bind.Name != "" {
					return fmt.Errorf("%s: bind %q from body needs a JSON pointer and no header name", where, name)
				}
			case "header":
				if bind.Name == "" || bind.Pointer != "" {
					return fmt.Errorf("%s: bind %q from header needs a header name and no pointer", where, name)
				}
			default:
				return fmt.Errorf("%s: bind %q from %q must be body or header", where, name, bind.From)
			}
			if bind.Class != "uuid" && bind.Class != "token" {
				return fmt.Errorf("%s: bind %q class %q must be uuid or token", where, name, bind.Class)
			}
			if bind.Regex != "" {
				re, err := regexp.Compile(bind.Regex)
				if err != nil || re.NumSubexp() != 1 {
					return fmt.Errorf("%s: bind %q regex must compile with exactly one group", where, name)
				}
			}
			known[name] = true
		}
	}
	return nil
}

// Render substitutes {{name}} with vars. Unknown names are an error, which
// validation already rules out for loaded scenarios.
func Render(text string, vars map[string]string) (string, error) {
	var missing string
	out := templatePattern.ReplaceAllStringFunc(text, func(token string) string {
		name := templatePattern.FindStringSubmatch(token)[1]
		value, ok := vars[name]
		if !ok {
			missing = name
			return token
		}
		return value
	})
	if missing != "" {
		return "", fmt.Errorf("scenario: {{%s}} has no value", missing)
	}
	return out, nil
}

// LoadPaths loads every *.yaml under the given files or directories, sorted
// by path so both stacks always run scenarios in the same order.
func LoadPaths(paths ...string) ([]*Scenario, error) {
	var files []string
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			files = append(files, path)
			continue
		}
		err = filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && (strings.HasSuffix(p, ".yaml") || strings.HasSuffix(p, ".yml")) {
				files = append(files, p)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(files)
	if len(files) == 0 {
		return nil, errors.New("scenario: no scenario files found")
	}
	ids := map[string]string{}
	scenarios := make([]*Scenario, 0, len(files))
	for _, file := range files {
		sc, err := Load(file)
		if err != nil {
			return nil, err
		}
		if other, dup := ids[sc.ID]; dup {
			return nil, fmt.Errorf("scenario id %q is used by both %s and %s", sc.ID, other, file)
		}
		ids[sc.ID] = file
		scenarios = append(scenarios, sc)
	}
	return scenarios, nil
}

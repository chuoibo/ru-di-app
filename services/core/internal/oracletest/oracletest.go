// Package oracletest reads the golden files scripts/render_domain_w3_goldens.py
// renders from the real Python code, for the tests of the Go ports.
//
// It is a test-support package: only _test.go files import it, so the domain
// packages whose tests use it stay free of os and encoding/json.
//
// The encoding is the one scripts/render_domain_w2_goldens.py documents. A case
// is {"fn", "name", "args", "result"}; a result is {"ok": value} or
// {"raised": {"type", "message", "code"}}. Values are JSON except:
//
//   - "$i:<hex>" is an int of nine or more digits (Python's f"{v:#_x}");
//   - "$sp:<text>" is a str cut into groups of six code points joined by "|".
//
// Plain turns an encoded value into nil, bool, int64, BigInt, string, []any and
// map[string]any. A Python int outside int64 decodes to BigInt, its decimal
// spelling, so that a result Go cannot represent is visible rather than
// silently wrapped.
package oracletest

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"
)

// File is one rendered golden file.
type File struct {
	Path      string          `json:"-"`
	Generator string          `json:"generator"`
	Module    string          `json:"module"`
	Mode      string          `json:"mode"`
	Constants json.RawMessage `json:"constants"`
	Fuzz      *Shard          `json:"fuzz"`
	Cases     []Case          `json:"cases"`
}

// Shard describes one contiguous slice of a seeded fuzz list.
type Shard struct {
	Shard  int `json:"shard"`
	Shards int `json:"shards"`
	Total  int `json:"total"`
}

// Case is one recorded call.
type Case struct {
	Fn     string         `json:"fn"`
	Name   string         `json:"name"`
	Args   map[string]any `json:"args"`
	Result map[string]any `json:"result"`
}

// BigInt is a Python int outside int64, in decimal.
type BigInt string

// Raised is an exception Python raised: its class name, str(exc) and .code.
type Raised struct {
	Type    string
	Message string
	Code    any
}

// Load reads every file matching pattern, sorted by path.
func Load(t testing.TB, pattern string) []File {
	t.Helper()
	paths, err := filepath.Glob(pattern)
	if err != nil || len(paths) == 0 {
		t.Fatalf("no golden files match %s: %v", pattern, err)
	}
	sort.Strings(paths)
	files := make([]File, 0, len(paths))
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		file, err := Parse(path, raw)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, file)
	}
	return files
}

// Parse decodes one rendered golden document; name is where it came from.
func Parse(name string, raw []byte) (File, error) {
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	var file File
	if err := decoder.Decode(&file); err != nil {
		return File{}, fmt.Errorf("%s: %w", name, err)
	}
	file.Path = name
	return file, nil
}

// Text reads a str of the oracle encoding.
func Text(raw any) (string, error) {
	s, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("want a str, got %T", raw)
	}
	if !strings.HasPrefix(s, "$") {
		return s, nil
	}
	body, found := strings.CutPrefix(s, "$sp:")
	if !found {
		return "", fmt.Errorf("not a str: %q", s)
	}
	var out strings.Builder
	count := 0
	for i := 0; i < len(body); {
		r, size := utf8.DecodeRuneInString(body[i:])
		i += size
		if count == 6 {
			if r != '|' {
				return "", fmt.Errorf("bad group separator in %q", s)
			}
			count = 0
			continue
		}
		out.WriteRune(r)
		count++
	}
	return out.String(), nil
}

// integer turns a decimal or "$i:" spelling into int64, or BigInt when the
// value does not fit.
func integer(spelling string, base int) (any, error) {
	value, ok := new(big.Int).SetString(spelling, base)
	if !ok {
		return nil, fmt.Errorf("not an int: %q", spelling)
	}
	if value.IsInt64() {
		return value.Int64(), nil
	}
	return BigInt(value.String()), nil
}

// Plain expands an oracle value.
func Plain(raw any) (any, error) {
	switch v := raw.(type) {
	case nil, bool:
		return v, nil
	case json.Number:
		return integer(string(v), 10)
	case string:
		if body, found := strings.CutPrefix(v, "$i:"); found {
			return integer(body, 0)
		}
		return Text(v)
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			value, err := Plain(item)
			if err != nil {
				return nil, err
			}
			out[i] = value
		}
		return out, nil
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, item := range v {
			value, err := Plain(item)
			if err != nil {
				return nil, err
			}
			out[key] = value
		}
		return out, nil
	}
	return nil, fmt.Errorf("unexpected JSON %T", raw)
}

// Args decodes a case's arguments.
func (c Case) PlainArgs() (map[string]any, error) {
	decoded, err := Plain(map[string]any(c.Args))
	if err != nil {
		return nil, err
	}
	return decoded.(map[string]any), nil
}

// Outcome decodes a case's result: the returned value, or what was raised.
func (c Case) Outcome() (value any, raised *Raised, err error) {
	if body, ok := c.Result["raised"].(map[string]any); ok {
		kind, _ := body["type"].(string)
		message, err := Text(body["message"])
		if err != nil {
			return nil, nil, err
		}
		code, err := Plain(body["code"])
		if err != nil {
			return nil, nil, err
		}
		return nil, &Raised{Type: kind, Message: message, Code: code}, nil
	}
	body, ok := c.Result["ok"]
	if !ok {
		return nil, nil, errors.New("result has neither ok nor raised")
	}
	value, err = Plain(body)
	return value, nil, err
}

// HasBigInt reports whether a decoded value holds an int outside int64.
func HasBigInt(value any) bool {
	switch v := value.(type) {
	case BigInt:
		return true
	case []any:
		for _, item := range v {
			if HasBigInt(item) {
				return true
			}
		}
	case map[string]any:
		for _, item := range v {
			if HasBigInt(item) {
				return true
			}
		}
	}
	return false
}

// CheckShards fails unless the fuzz shards of one module are all present and
// hold exactly the declared number of cases.
func CheckShards(t testing.TB, files []File, module string) {
	t.Helper()
	seen := map[int]bool{}
	declared, shards, rendered := -1, -1, 0
	for _, file := range files {
		if file.Fuzz == nil || !strings.HasPrefix(file.Mode, module+"-fuzz-") {
			continue
		}
		if declared >= 0 && (declared != file.Fuzz.Total || shards != file.Fuzz.Shards) {
			t.Fatalf("%s: fuzz shards disagree on their totals", module)
		}
		declared, shards = file.Fuzz.Total, file.Fuzz.Shards
		if seen[file.Fuzz.Shard] {
			t.Fatalf("%s: shard %d appears twice", module, file.Fuzz.Shard)
		}
		seen[file.Fuzz.Shard] = true
		rendered += len(file.Cases)
	}
	if shards < 1 || len(seen) != shards || rendered != declared {
		t.Fatalf("%s: %d of %d shards holding %d of %d fuzz cases", module, len(seen), shards, rendered, declared)
	}
}

// Constants returns the constants object of the one file of mode, decoded.
func Constants(t testing.TB, files []File, mode string) map[string]any {
	t.Helper()
	var found []json.RawMessage
	for _, file := range files {
		if file.Mode == mode && file.Constants != nil {
			found = append(found, file.Constants)
		}
	}
	if len(found) != 1 {
		t.Fatalf("%d files of mode %s carry constants, want 1", len(found), mode)
	}
	decoder := json.NewDecoder(strings.NewReader(string(found[0])))
	decoder.UseNumber()
	var raw map[string]any
	if err := decoder.Decode(&raw); err != nil {
		t.Fatal(err)
	}
	plain, err := Plain(raw)
	if err != nil {
		t.Fatal(err)
	}
	return plain.(map[string]any)
}

// Strings reads a decoded []any of str.
func Strings(value any) ([]string, error) {
	items, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("want a list, got %T", value)
	}
	out := make([]string, len(items))
	for i, item := range items {
		s, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("item %d is %T, not str", i, item)
		}
		out[i] = s
	}
	return out, nil
}

// OptionalString reads nil or a str.
func OptionalString(value any) (*string, error) {
	if value == nil {
		return nil, nil
	}
	s, ok := value.(string)
	if !ok {
		return nil, fmt.Errorf("want str or None, got %T", value)
	}
	return &s, nil
}

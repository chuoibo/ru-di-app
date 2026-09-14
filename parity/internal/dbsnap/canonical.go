package dbsnap

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// storedResponseTable is where the API caches idempotent responses.
const storedResponseTable = "idempotency_keys"

type rewriteKind int

const (
	padFraction rewriteKind = iota + 1
	decodeBytea
)

// planRewrites decides, from column types alone, which top-level members of a
// relation's rows are rewritten into Row.Text.
func planRewrites(rel *Relation, decoded map[string]bool) map[string]rewriteKind {
	var plan map[string]rewriteKind
	for _, column := range rel.Columns {
		var kind rewriteKind
		switch {
		case column.Type == "timestamptz" || column.Type == "timestamp":
			kind = padFraction
		case column.Type == "bytea" && decoded[rel.Name+"."+column.Name]:
			kind = decodeBytea
		default:
			continue
		}
		if plan == nil {
			plan = map[string]rewriteKind{}
		}
		plan[column.Name] = kind
	}
	return plan
}

// member locates one top-level value of a JSON object inside its text.
type member struct {
	name       string
	start, end int // byte offsets of the value
}

// topLevelMembers splits a JSON object into its members without re-encoding
// anything, so the bytes between and inside values are kept exactly.
func topLevelMembers(text string) ([]member, error) {
	decoder := json.NewDecoder(strings.NewReader(text))
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	if delim, ok := token.(json.Delim); !ok || delim != '{' {
		return nil, fmt.Errorf("not a JSON object")
	}
	var members []member
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		name, ok := token.(string)
		if !ok {
			return nil, fmt.Errorf("object key is %T", token)
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}
		end := int(decoder.InputOffset())
		start := end - len(value)
		if start < 0 || text[start:end] != string(value) {
			return nil, fmt.Errorf("could not locate value of %q", name)
		}
		members = append(members, member{name: name, start: start, end: end})
	}
	if _, err := decoder.Token(); err != nil {
		return nil, err
	}
	return members, nil
}

// canonicalText applies plan to the top-level members of a row_to_json text.
func canonicalText(raw string, plan map[string]rewriteKind) (string, error) {
	if len(plan) == 0 {
		return raw, nil
	}
	members, err := topLevelMembers(raw)
	if err != nil {
		return "", err
	}
	var out strings.Builder
	last, changed := 0, false
	for _, m := range members {
		kind, ok := plan[m.name]
		if !ok {
			continue
		}
		value := raw[m.start:m.end]
		var replacement string
		switch kind {
		case padFraction:
			replacement = padInstantFraction(value)
		case decodeBytea:
			replacement = decodedByteaValue(value)
		}
		if replacement == value {
			continue
		}
		out.WriteString(raw[last:m.start])
		out.WriteString(replacement)
		last, changed = m.end, true
	}
	if !changed {
		return raw, nil
	}
	out.WriteString(raw[last:])
	return out.String(), nil
}

// instantValue matches a JSON-quoted PostgreSQL timestamp. Anything else
// (null, "infinity") is left alone.
var instantValue = regexp.MustCompile(`^"(\d{4,}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})(?:\.(\d{1,6}))?([^"]*)"$`)

func padInstantFraction(value string) string {
	match := instantValue.FindStringSubmatch(value)
	if match == nil {
		return value
	}
	return `"` + match[1] + "." + (match[2] + "000000")[:6] + match[3] + `"`
}

// byteaBytes decodes a JSON-quoted hex bytea value.
func byteaBytes(value string) ([]byte, bool) {
	var text string
	if err := json.Unmarshal([]byte(value), &text); err != nil {
		return nil, false
	}
	return hexBytes(text)
}

// hexBytes decodes PostgreSQL's hex bytea output, "\x" followed by pairs.
func hexBytes(text string) ([]byte, bool) {
	if !strings.HasPrefix(text, `\x`) {
		return nil, false
	}
	decoded, err := hex.DecodeString(text[2:])
	if err != nil {
		return nil, false
	}
	return decoded, true
}

func decodedByteaValue(value string) string {
	body, ok := byteaBytes(value)
	if !ok || !utf8.Valid(body) {
		return value
	}
	if json.Valid(body) {
		return `{"$bytea_json":` + string(body) + `}`
	}
	return `{"$bytea_utf8":` + jsonString(string(body)) + `}`
}

func jsonString(s string) string {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(s) // a string always encodes
	return strings.TrimSuffix(buf.String(), "\n")
}

// StoredResponse is one idempotency record's cached response, decoded from
// Row.Raw. It is never normalised: the runner applies its binder to Body, as
// it does to the HTTP body it compares against.
type StoredResponse struct {
	Key            string
	Scope          string
	IdempotencyKey string
	Status         int // 0 while the request has not completed
	MediaType      string
	Body           []byte // nil when response_body is NULL
	JSON           bool   // Body is valid UTF-8 JSON
}

func storedResponse(row Row) StoredResponse {
	var fields struct {
		Scope             string  `json:"scope"`
		IdempotencyKey    string  `json:"idempotency_key"`
		ResponseStatus    *int    `json:"response_status"`
		ResponseMediaType *string `json:"response_media_type"`
		ResponseBody      *string `json:"response_body"`
	}
	_ = json.Unmarshal([]byte(row.Raw), &fields) // row_to_json output always parses
	out := StoredResponse{Key: row.Key, Scope: fields.Scope, IdempotencyKey: fields.IdempotencyKey}
	if fields.ResponseStatus != nil {
		out.Status = *fields.ResponseStatus
	}
	if fields.ResponseMediaType != nil {
		out.MediaType = *fields.ResponseMediaType
	}
	if fields.ResponseBody != nil {
		if body, ok := hexBytes(*fields.ResponseBody); ok {
			out.Body = body
			out.JSON = utf8.Valid(body) && json.Valid(body)
		}
	}
	return out
}

// StoredResponses returns every cached response in the snapshot, ordered by
// scope then idempotency key. Use it when a replayed request writes nothing
// and its response must match what was stored earlier.
func (s *Snap) StoredResponses() []StoredResponse {
	rel := s.Relation(storedResponseTable)
	if rel == nil {
		return nil
	}
	out := make([]StoredResponse, 0, len(rel.Rows))
	for _, row := range rel.Rows {
		out = append(out, storedResponse(row))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Scope != out[j].Scope {
			return out[i].Scope < out[j].Scope
		}
		return out[i].IdempotencyKey < out[j].IdempotencyKey
	})
	return out
}

// StoredResponses returns the cached responses a step inserted or updated, in
// the change's order.
func (c *Change) StoredResponses() []StoredResponse {
	var out []StoredResponse
	for _, rc := range c.Changed {
		if rc.Relation != storedResponseTable {
			continue
		}
		for _, row := range rc.Inserted {
			out = append(out, storedResponse(row))
		}
		for _, update := range rc.Updated {
			out = append(out, storedResponse(update.After))
		}
	}
	return out
}

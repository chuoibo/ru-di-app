package tools

import (
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"google.golang.org/genai"
)

// sangJSON converts an argument schema of thamSo (genai.Schema, the
// contract's one source of truth) into the JSON Schema ADK's functiontool
// validates arguments with and declares to the model. Only the keywords the
// argument schemas use are converted; any other one is an error, so a schema
// edit that this converter would silently drop is refused at start.
//
// Objects refuse properties they do not declare, and arrays refuse repeated
// items: both are structure, and neither needs to read a value's meaning.
func sangJSON(s *genai.Schema) (*jsonschema.Schema, error) {
	if s == nil {
		return nil, fmt.Errorf("tools: nil schema")
	}
	if s.AnyOf != nil || s.Default != nil || s.Example != nil || s.MaxProperties != nil || s.MinProperties != nil ||
		s.MinLength != nil || s.Nullable != nil || s.Title != "" {
		return nil, fmt.Errorf("tools: schema keyword the converter does not carry")
	}
	out := &jsonschema.Schema{Description: s.Description, Pattern: s.Pattern, Format: s.Format}
	switch s.Type {
	case genai.TypeObject:
		out.Type = "object"
		out.Properties = map[string]*jsonschema.Schema{}
		for k, p := range s.Properties {
			c, err := sangJSON(p)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", k, err)
			}
			out.Properties[k] = c
		}
		out.Required = append([]string(nil), s.Required...)
		out.PropertyOrder = append([]string(nil), s.PropertyOrdering...)
		out.AdditionalProperties = &jsonschema.Schema{Not: &jsonschema.Schema{}}
	case genai.TypeString:
		out.Type = "string"
		for _, e := range s.Enum {
			out.Enum = append(out.Enum, e)
		}
		if s.MaxLength != nil {
			n := int(*s.MaxLength)
			out.MaxLength = &n
		}
	case genai.TypeInteger:
		out.Type = "integer"
		out.Minimum, out.Maximum = s.Minimum, s.Maximum
	case genai.TypeArray:
		out.Type = "array"
		it, err := sangJSON(s.Items)
		if err != nil {
			return nil, err
		}
		out.Items = it
		out.UniqueItems = true
		if s.MinItems != nil {
			n := int(*s.MinItems)
			out.MinItems = &n
		}
		if s.MaxItems != nil {
			n := int(*s.MaxItems)
			out.MaxItems = &n
		}
	default:
		return nil, fmt.Errorf("tools: schema type %q", s.Type)
	}
	if s.Type != genai.TypeString && (len(s.Enum) > 0 || s.MaxLength != nil) {
		return nil, fmt.Errorf("tools: string keyword on %q", s.Type)
	}
	if s.Type != genai.TypeInteger && (s.Minimum != nil || s.Maximum != nil) {
		return nil, fmt.Errorf("tools: number keyword on %q", s.Type)
	}
	if s.Type != genai.TypeArray && (s.Items != nil || s.MinItems != nil || s.MaxItems != nil) {
		return nil, fmt.Errorf("tools: array keyword on %q", s.Type)
	}
	if s.Type != genai.TypeObject && (s.Properties != nil || s.Required != nil || s.PropertyOrdering != nil) {
		return nil, fmt.Errorf("tools: object keyword on %q", s.Type)
	}
	return out, nil
}

// luocDoJSON are the converted and resolved argument schemas, built once at
// start: a schema that does not convert or resolve stops the process there,
// as a bad permission table does.
var luocDoJSON = func() map[Ten]*jsonschema.Resolved {
	out := map[Ten]*jsonschema.Resolved{}
	for _, m := range DangKy {
		s, err := sangJSON(thamSo[m.Ten])
		if err != nil {
			panic(fmt.Sprintf("tools: %s: %v", m.Ten, err))
		}
		r, err := s.Resolve(nil)
		if err != nil {
			panic(fmt.Sprintf("tools: %s: %v", m.Ten, err))
		}
		out[m.Ten] = r
	}
	return out
}()

// LuocDoJSON is t's argument schema as ADK declares and validates it.
func LuocDoJSON(t Ten) (*jsonschema.Resolved, bool) {
	r, ok := luocDoJSON[t]
	return r, ok
}

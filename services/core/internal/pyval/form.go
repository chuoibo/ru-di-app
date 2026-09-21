package pyval

import (
	"bytes"
	"strings"

	"mobile/services/core/internal/pyjson"
)

// A route whose body field is Form or File (fastapi/routing.py:234) reads
// the body with `await request.form()` whatever the request says
// (routing.py:246-248), never as JSON. Request._get_form
// (starlette/requests.py:252-278) looks at the first Content-Type header
// only:
//
//   - parse_options_header's content type "multipart/form-data" is parsed
//     by MultiPartParser (multipart.go);
//   - "application/x-www-form-urlencoded" by FormParser;
//   - anything else, or no header, is an empty FormData: a JSON body sent
//     to a form route validates as if every field were missing.
//
// solve_dependencies then turns the FormData into a dict per dependant
// (fastapi/dependencies/utils.py:833-873 _extract_form_body) and validates
// it like a JSON body (utils.py:876-919 request_body_to_args).

// formItem is one (name, value) pair of a FormData; value is a pyjson.String
// or an *UploadFile.
type formItem struct {
	key   string
	value Value
}

// formData is starlette.datastructures.FormData, an ImmutableMultiDict:
// get answers the last value of a name, dictItems the names in first-seen
// order with their last values (datastructures.py:276-295).
type formData struct {
	items []formItem
}

func (f *formData) get(key string) (Value, bool) {
	for i := len(f.items) - 1; i >= 0; i-- {
		if f.items[i].key == key {
			return f.items[i].value, true
		}
	}
	return nil, false
}

func (f *formData) dictItems() []formItem {
	index := map[string]int{}
	var out []formItem
	for _, it := range f.items {
		if i, ok := index[it.key]; ok {
			out[i].value = it.value
			continue
		}
		index[it.key] = len(out)
		out = append(out, it)
	}
	return out
}

// readForm is Request._get_form. Header bytes are latin-1 text one byte per
// code point, which is what the header parsers take.
func readForm(req *Request) (*formData, error) {
	header, _ := req.header("content-type")
	ctype, _, err := parseOptionsHeader(header)
	if err != nil {
		return nil, &BodyError{Err: err}
	}
	switch ctype {
	case "multipart/form-data":
		return parseMultipart(header, req.Body)
	case "application/x-www-form-urlencoded":
		return parseURLEncoded(req.Body), nil
	}
	return &formData{}, nil
}

// ---- application/x-www-form-urlencoded ----

// parseURLEncoded is Starlette's FormParser (formparsers.py:56-121) over
// python_multipart's QuerystringParser (multipart.py:724-948) with
// strict_parsing off: the body written in one chunk, then finalize. "&" and
// ";" both separate, but a field looks for "&" across the rest of the body
// before it looks for ";". A field without "=" ends at that separator with
// an empty value, and is dropped at the end of the body. Names and values
// are latin-1-decoded, then unquote_plus-ed as UTF-8 with replacement.
func parseURLEncoded(body []byte) *formData {
	const (
		beforeField = iota
		fieldName
		fieldData
	)
	out := &formData{}
	var name, value []byte
	state := beforeField
	length := len(body)
	fieldEnd := func() {
		out.items = append(out.items, formItem{
			key:   pyUnquotePlus(latin1(string(name))),
			value: pyjson.String(pyUnquotePlus(latin1(string(value)))),
		})
	}
	find := func(sub byte, from, to int) int {
		if j := bytes.IndexByte(body[from:to], sub); j >= 0 {
			return from + j
		}
		return -1
	}
	separator := func(from int) int {
		if sep := find('&', from, length); sep != -1 {
			return sep
		}
		return find(';', from, length)
	}
	for i := 0; i < length; i++ {
		switch state {
		case beforeField:
			if ch := body[i]; ch == '&' || ch == ';' {
				continue
			}
			name, value = nil, nil // on_field_start
			i--
			state = fieldName
		case fieldName:
			sepPos := separator(i)
			equalsPos := -1
			if sepPos != -1 {
				equalsPos = find('=', i, sepPos)
			} else {
				equalsPos = find('=', i, length)
			}
			switch {
			case equalsPos != -1:
				name = append(name, body[i:equalsPos]...)
				i = equalsPos
				state = fieldData
			case sepPos != -1:
				name = append(name, body[i:sepPos]...)
				fieldEnd()
				i = sepPos - 1
				state = beforeField
			default:
				name = append(name, body[i:]...)
				i = length
			}
		case fieldData:
			if sepPos := separator(i); sepPos != -1 {
				value = append(value, body[i:sepPos]...)
				fieldEnd()
				i = sepPos - 1
				state = beforeField
			} else {
				value = append(value, body[i:]...)
				i = length
			}
		}
	}
	// finalize ends only a field in its data state.
	if state == fieldData {
		fieldEnd()
	}
	return out
}

// pyUnquotePlus is urllib.parse.unquote_plus for a str of latin-1 code
// points.
func pyUnquotePlus(s string) string {
	return pyUnquote(strings.ReplaceAll(s, "+", " "))
}

// ---- FastAPI's form body ----

// formField is one field _extract_form_body reads from the FormData: an
// endpoint parameter, or a field of the model a single non-embedded Form
// parameter declares.
type formField struct {
	alias    string
	required bool
	// form is isinstance(field_info, params.Form): only then does an empty
	// string count as missing. Model fields carry pydantic's FieldInfo.
	form bool
	// def is the default of a field that is not required: None or a scalar.
	def Value
}

// formDict is the dict _extract_form_body returns: insertion-ordered, a
// later set of a known key keeps its position.
type formDict struct {
	keys []string
	vals map[string]Value
}

func (d *formDict) set(k string, v Value) {
	if _, ok := d.vals[k]; !ok {
		d.keys = append(d.keys, k)
	}
	d.vals[k] = v
}

func (d *formDict) lookup(k string) (any, bool) {
	v, ok := d.vals[k]
	return v, ok
}

func (d *formDict) keyList() []string { return d.keys }

// extractFormBody is _extract_form_body for fields without bytes
// annotations: each field's _get_multidict_value (utils.py:716-737) when it
// is not None, then every other name of the form with its last value. The
// second loop puts back a name the first skipped, so a required Form field
// sent empty still reaches validation as "".
func extractFormBody(fields []formField, fd *formData) *formDict {
	d := &formDict{vals: map[string]Value{}}
	for _, f := range fields {
		value, present := fd.get(f.alias)
		if s, isStr := value.(pyjson.String); !present || f.form && isStr && s == "" {
			if f.required || isNone(f.def) {
				continue
			}
			value = copyValue(f.def)
		}
		d.set(f.alias, value)
	}
	for _, it := range fd.dictItems() {
		if _, ok := d.vals[it.key]; !ok {
			d.set(it.key, it.value)
		}
	}
	return d
}

// bindForm records, for every dependant with body parameters, the fields
// _extract_form_body reads, and marks what this port does not implement:
// sequence fields (getlist and the empty-list rule), defaults other than
// None or a scalar, default factories, and model fields with an alias.
func (c *compiler) bindForm(d *dependant, embed bool) {
	for _, sub := range d.deps {
		c.bindForm(sub, embed)
	}
	body := d.params["body"]
	if len(body) == 0 {
		return
	}
	d.formFields = []formField{}
	if len(body) == 1 && !embed {
		// A lone Form parameter is only left unembedded when it is a model
		// (utils.py:826-829); the fields read are the model's.
		model := formModel(body[0].v)
		if model == nil {
			c.unsupportedf("form body %s is not a model", body[0].name)
			return
		}
		for _, f := range model.fields {
			if f.alias != "" && f.alias != f.name {
				c.unsupportedf("form model field %s with alias %s", f.name, f.alias)
			}
			if isSequenceSchema(f.v) {
				c.unsupportedf("form model field %s is a sequence", f.name)
			}
			ff := formField{alias: f.name, required: f.def == nil}
			if f.def != nil {
				if f.def.factory != nil || !isScalarDefault(f.def.value) {
					c.unsupportedf("form model field %s with a default other than None or a scalar", f.name)
				}
				ff.def = f.def.value
			}
			d.formFields = append(d.formFields, ff)
		}
		return
	}
	for _, p := range body {
		if p.sequence || isSequenceSchema(p.v) {
			c.unsupportedf("form parameter %s is a sequence", p.name)
		}
		if p.factory != nil || !isScalarDefault(p.def) {
			c.unsupportedf("form parameter %s with a default other than None or a scalar", p.name)
		}
		d.formFields = append(d.formFields, formField{
			alias:    p.alias,
			required: p.required,
			form:     p.fieldInfo == "Form" || p.fieldInfo == "File",
			def:      p.def,
		})
	}
}

func isScalarDefault(v Value) bool {
	switch v.(type) {
	case nil, pyjson.Null, pyjson.String, pyjson.Int, pyjson.Bool, pyjson.Float:
		return true
	}
	return false
}

// formModel finds the model a Form parameter's schema validates, through
// refs, defaults and validator functions around it.
func formModel(v validator) *modelFieldsValidator {
	for {
		switch x := v.(type) {
		case *refValidator:
			v = x.inner
		case *defaultValidator:
			v = x.inner
		case *funcValidator:
			if x.inner == nil {
				return nil
			}
			v = x.inner
		case *modelValidator:
			return x.fields
		default:
			return nil
		}
	}
}

// isSequenceSchema approximates FastAPI's annotation test
// field_annotation_is_sequence on the schema: a list, or a union with one.
func isSequenceSchema(v validator) bool {
	for {
		switch x := v.(type) {
		case *refValidator:
			if x.inner == nil {
				return false
			}
			v = x.inner
		case *defaultValidator:
			v = x.inner
		case *nullableValidator:
			v = x.inner
		case *funcValidator:
			if x.inner == nil {
				return false
			}
			v = x.inner
		case *listValidator:
			return true
		case *unionValidator:
			for _, ch := range x.choices {
				if isSequenceSchema(ch.v) {
					return true
				}
			}
			return false
		default:
			return false
		}
	}
}

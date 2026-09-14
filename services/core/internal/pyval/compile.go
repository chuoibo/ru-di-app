package pyval

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"mobile/services/core/internal/pyjson"
)

// compiler turns IR schema nodes into validators. It never stops at the
// first problem: every unsupported feature and every validator function
// without a Go port is recorded, so a caller can list them all.
type compiler struct {
	defs        map[string]*pyjson.OrderedMap
	refs        map[string]*refValidator
	reg         *Registry
	functions   map[string]bool
	missing     map[string]bool
	unsupported map[string]bool
}

func newCompiler(defs map[string]*pyjson.OrderedMap, reg *Registry) *compiler {
	return &compiler{
		defs:        defs,
		refs:        map[string]*refValidator{},
		reg:         reg,
		functions:   map[string]bool{},
		missing:     map[string]bool{},
		unsupported: map[string]bool{},
	}
}

func (c *compiler) unsupportedf(format string, args ...any) {
	c.unsupported[fmt.Sprintf(format, args...)] = true
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// config is the pydantic-core config in force while a schema is built: the
// nearest enclosing model's, or none.
type config struct {
	strict bool
	extra  string
}

// Keys every node type accepts. A key not listed is unsupported.
var nodeKeys = map[string][]string{
	"str":            {"strict", "min_length", "max_length", "pattern"},
	"int":            {"strict", "ge", "gt", "le", "lt", "multiple_of"},
	"float":          {"strict", "ge", "gt", "le", "lt", "multiple_of", "allow_inf_nan"},
	"bool":           {"strict"},
	"uuid":           {"strict"},
	"date":           {"strict"},
	"datetime":       {"strict", "microseconds_precision"},
	"literal":        {"expected"},
	"list":           {"items_schema", "min_length", "max_length", "strict"},
	"dict":           {"keys_schema", "values_schema", "min_length", "max_length", "strict"},
	"nullable":       {"schema"},
	"default":        {"schema", "default", "default_factory", "default_factory_takes_data"},
	"any":            {},
	"none":           {},
	"ref":            {"ref"},
	"model":          {"class", "config", "custom_init", "root_model", "decorators", "schema"},
	"model-fields":   {"fields", "model_name", "extra_behavior"},
	"union":          {"choices"},
	"function-after": {"function", "schema"},
	"function-before": {
		"function", "schema",
	},
	"function-plain": {"function"},
	"function-wrap":  {"function", "schema"},
}

func (c *compiler) compile(v pyjson.Value, cfg config, where string) validator {
	n, ok := v.(*pyjson.OrderedMap)
	if !ok {
		c.unsupportedf("%s: schema is not an object", where)
		return unsupportedValidator{}
	}
	typ := irString(n, "type")
	allowed, known := nodeKeys[typ]
	if !known {
		c.unsupportedf("schema type %q", typ)
		return unsupportedValidator{}
	}
	for _, k := range n.Keys() {
		if k != "type" && !contains(allowed, k) {
			c.unsupportedf("%s key %q", typ, k)
		}
	}
	strict := cfg.strict
	if b, ok := n.Get("strict"); ok {
		strict = bool(b.(pyjson.Bool))
	}
	switch typ {
	case "str":
		return c.compileStr(n, strict)
	case "int":
		iv := &intValidator{strict: strict}
		iv.bounds = c.bounds(n, typ)
		return iv
	case "float":
		fv := &floatValidator{strict: strict, allowInfNan: true}
		if b, ok := n.Get("allow_inf_nan"); ok {
			fv.allowInfNan = bool(b.(pyjson.Bool))
		}
		fv.bounds = c.bounds(n, typ)
		return fv
	case "bool":
		return &boolValidator{strict: strict}
	case "uuid":
		return &uuidValidator{strict: strict}
	case "date":
		return &dateValidator{strict: strict}
	case "datetime":
		precision := irString(n, "microseconds_precision")
		if precision != "" && precision != "truncate" && precision != "error" {
			c.unsupportedf("datetime microseconds_precision %q", precision)
		}
		return &dateTimeValidator{strict: strict, truncate: precision != "error"}
	case "literal":
		return c.compileLiteral(n)
	case "list":
		lv := &listValidator{min: irInt(n, "min_length", -1), max: irInt(n, "max_length", -1)}
		if items, ok := n.Get("items_schema"); ok {
			lv.item = c.compile(items, cfg, where+"[]")
		}
		return lv
	case "dict":
		dv := &dictValidator{min: irInt(n, "min_length", -1), max: irInt(n, "max_length", -1)}
		if k, ok := n.Get("keys_schema"); ok {
			dv.keys = c.compile(k, cfg, where+"{key}")
		}
		if val, ok := n.Get("values_schema"); ok {
			dv.values = c.compile(val, cfg, where+"{}")
		}
		return dv
	case "nullable":
		return &nullableValidator{inner: c.compile(mustGet(n, "schema"), cfg, where)}
	case "default":
		return c.compileDefault(n, cfg, where)
	case "any":
		return anyValidator{}
	case "none":
		return noneValidator{}
	case "ref":
		return c.ref(irString(n, "ref"))
	case "model":
		return c.compileModel(n, where)
	case "model-fields":
		return c.compileModelFields(n, cfg, irString(n, "model_name"), where)
	case "union":
		return c.compileUnion(n, cfg, where)
	}
	return c.compileFunction(n, typ, cfg, where)
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func mustGet(n *pyjson.OrderedMap, key string) pyjson.Value {
	v, _ := n.Get(key)
	return v
}

func irString(n *pyjson.OrderedMap, key string) string {
	if v, ok := n.Get(key); ok {
		if s, ok := v.(pyjson.String); ok {
			return string(s)
		}
	}
	return ""
}

func irBool(n *pyjson.OrderedMap, key string) bool {
	if v, ok := n.Get(key); ok {
		if b, ok := v.(pyjson.Bool); ok {
			return bool(b)
		}
	}
	return false
}

func irInt(n *pyjson.OrderedMap, key string, missing int) int {
	if v, ok := n.Get(key); ok {
		if i, ok := v.(pyjson.Int); ok {
			if x, ok := i.Int64(); ok {
				return int(x)
			}
		}
	}
	return missing
}

func (c *compiler) compileStr(n *pyjson.OrderedMap, strict bool) validator {
	sv := &strValidator{strict: strict, min: irInt(n, "min_length", -1), max: irInt(n, "max_length", -1)}
	if p, ok := n.Get("pattern"); ok {
		text := string(p.(pyjson.String))
		re, err := compileRustPattern(text)
		if err != nil {
			c.unsupportedf("pattern %q: %v", text, err)
		} else {
			sv.pattern, sv.patternText = re, text
		}
	}
	sv.constrained = sv.min >= 0 || sv.max >= 0 || sv.pattern != nil
	return sv
}

// compileRustPattern compiles a pattern that means the same under Rust's
// regex crate (pydantic-core's default engine) and Go's RE2. Escapes whose
// meaning differs (Rust's \d, \w, \s and \b are Unicode-aware) are refused.
func compileRustPattern(p string) (*regexp.Regexp, error) {
	for i := 0; i < len(p); i++ {
		if p[i] != '\\' {
			continue
		}
		if i+1 >= len(p) {
			return nil, fmt.Errorf("trailing backslash")
		}
		i++
		if !strings.ContainsRune(`Az.-/\[](){}*+?|^$nt`, rune(p[i])) {
			return nil, fmt.Errorf("escape \\%c differs between Rust regex and RE2", p[i])
		}
	}
	if strings.Contains(p, "(?") {
		return nil, fmt.Errorf("group flags are not verified")
	}
	return regexp.Compile(p)
}

func (c *compiler) bounds(n *pyjson.OrderedMap, typ string) []numBound {
	var out []numBound
	// pydantic-core checks multiple_of, le, lt, ge, gt, in this order, and
	// reports the first that fails.
	for _, key := range []string{"multiple_of", "le", "lt", "ge", "gt"} {
		v, ok := n.Get(key)
		if !ok {
			continue
		}
		b, ok := boundFrom(v)
		if !ok {
			c.unsupportedf("%s %s bound of kind %T", typ, key, v)
			continue
		}
		out = append(out, numBound{key: key, b: b, raw: v})
	}
	return out
}

func (c *compiler) compileLiteral(n *pyjson.OrderedMap) validator {
	lv := &literalValidator{}
	list, _ := mustGet(n, "expected").(pyjson.List)
	reprs := make([]string, 0, len(list))
	for _, e := range list {
		switch x := e.(type) {
		case pyjson.String, pyjson.Int, pyjson.Bool, pyjson.Null:
			lv.expected = append(lv.expected, x)
			reprs = append(reprs, pyRepr(x))
		default:
			c.unsupportedf("literal member of kind %T", e)
		}
	}
	switch len(reprs) {
	case 0:
	case 1:
		lv.repr = reprs[0]
	default:
		lv.repr = strings.Join(reprs[:len(reprs)-1], ", ") + " or " + reprs[len(reprs)-1]
	}
	return lv
}

// pyRepr is repr() of a Python scalar.
func pyRepr(v pyjson.Value) string {
	switch x := v.(type) {
	case pyjson.String:
		return pyStrRepr(string(x))
	case pyjson.Int:
		return x.String()
	case pyjson.Bool:
		if x {
			return "True"
		}
		return "False"
	case pyjson.Float:
		return pyjson.FloatRepr(float64(x))
	}
	return "None"
}

func (c *compiler) compileDefault(n *pyjson.OrderedMap, cfg config, where string) validator {
	dv := &defaultValidator{inner: c.compile(mustGet(n, "schema"), cfg, where)}
	if irBool(n, "default_factory_takes_data") {
		c.unsupportedf("default_factory that takes data")
	}
	if f, ok := n.Get("default_factory"); ok {
		name := irString(f.(*pyjson.OrderedMap), "name")
		switch name {
		case "builtins.list":
			dv.factory = func() Value { return List{} }
		case "builtins.dict":
			dv.factory = func() Value { return &Dict{} }
		default:
			c.unsupportedf("default_factory %s", name)
		}
		return dv
	}
	raw, ok := n.Get("default")
	if !ok {
		c.unsupportedf("default without a value")
		return dv
	}
	value, err := irValue(raw)
	if err != nil {
		c.unsupportedf("default: %v", err)
	}
	dv.value = value
	return dv
}

// irValue converts a Python value as the IR writes it (see
// scripts/render_contract_ir.py) into a Value.
func irValue(v pyjson.Value) (Value, error) {
	switch x := v.(type) {
	case pyjson.Null, pyjson.Bool, pyjson.Int, pyjson.Float, pyjson.String:
		return x, nil
	case pyjson.List:
		out := make(List, 0, len(x))
		for _, item := range x {
			iv, err := irValue(item)
			if err != nil {
				return nil, err
			}
			out = append(out, iv)
		}
		return out, nil
	case *pyjson.OrderedMap:
		if s, ok := x.Get("$int"); ok {
			i, ok := pyjson.ParseInt(strings.ReplaceAll(string(s.(pyjson.String)), "_", ""))
			if !ok {
				return nil, fmt.Errorf("bad $int")
			}
			return i, nil
		}
		keys := x.Keys()
		if len(keys) > 0 {
			return nil, fmt.Errorf("IR value tag %q is not supported", keys[0])
		}
	}
	return nil, fmt.Errorf("IR value of kind %T is not supported", v)
}

// copyValue is copy.deepcopy for the mutable kinds a default can hold.
func copyValue(v Value) Value {
	switch x := v.(type) {
	case List:
		out := make(List, len(x))
		for i, item := range x {
			out[i] = copyValue(item)
		}
		return out
	case *Dict:
		out := &Dict{}
		for i := range x.keys {
			out.set(x.keys[i], copyValue(x.vals[i]))
		}
		return out
	}
	return v
}

func (c *compiler) ref(name string) validator {
	if r, ok := c.refs[name]; ok {
		return r
	}
	r := &refValidator{name_: name}
	c.refs[name] = r
	def, ok := c.defs[name]
	if !ok {
		c.unsupportedf("definition %s is missing", name)
		r.inner = unsupportedValidator{}
		return r
	}
	r.inner = c.compile(def, config{}, name)
	return r
}

func (c *compiler) compileModel(n *pyjson.OrderedMap, where string) validator {
	class := irString(n, "class")
	if irBool(n, "custom_init") || irBool(n, "root_model") {
		c.unsupportedf("model %s with custom __init__ or RootModel", class)
	}
	cfg := config{extra: "ignore"}
	if raw, ok := n.Get("config"); ok {
		for k, v := range raw.(*pyjson.OrderedMap).All() {
			switch k {
			case "extra_fields_behavior":
				cfg.extra = string(v.(pyjson.String))
			case "strict":
				cfg.strict = bool(v.(pyjson.Bool))
			default:
				c.unsupportedf("model config %q", k)
			}
		}
	}
	inner, _ := mustGet(n, "schema").(*pyjson.OrderedMap)
	if inner == nil || irString(inner, "type") != "model-fields" {
		c.unsupportedf("model %s whose schema is not model-fields", class)
		return unsupportedValidator{}
	}
	short := class[strings.LastIndexByte(class, '.')+1:]
	fields := c.compileModelFields(inner, cfg, class, class)
	return &modelValidator{class: class, name_: short, fields: fields}
}

func (c *compiler) compileModelFields(n *pyjson.OrderedMap, cfg config, class, where string) *modelFieldsValidator {
	mv := &modelFieldsValidator{class: class, extra: cfg.extra}
	if e := irString(n, "extra_behavior"); e != "" {
		mv.extra = e
	}
	if mv.extra == "" {
		mv.extra = "ignore"
	}
	if mv.extra != "ignore" && mv.extra != "forbid" {
		c.unsupportedf("extra behavior %q", mv.extra)
	}
	list, _ := mustGet(n, "fields").(pyjson.List)
	for _, raw := range list {
		f := raw.(*pyjson.OrderedMap)
		name := irString(f, "name")
		for _, k := range f.Keys() {
			switch k {
			case "name", "schema", "annotation", "required", "constraints", "alias", "serialization_alias", "frozen", "serialization_exclude":
			default:
				c.unsupportedf("model field key %q", k)
			}
		}
		schema := mustGet(f, "schema")
		mf := modelField{name: name, v: c.compile(schema, cfg, where+"."+name)}
		if d, ok := mf.v.(*defaultValidator); ok {
			mf.def = d
		}
		mv.fields = append(mv.fields, mf)
	}
	return mv
}

func (c *compiler) compileUnion(n *pyjson.OrderedMap, cfg config, where string) validator {
	uv := &unionValidator{}
	list, _ := mustGet(n, "choices").(pyjson.List)
	for i, raw := range list {
		ch := raw.(*pyjson.OrderedMap)
		v := c.compile(mustGet(ch, "schema"), cfg, fmt.Sprintf("%s|%d", where, i))
		label := irString(ch, "label")
		uv.choices = append(uv.choices, unionChoice{label: label, v: v})
	}
	return uv
}

func (c *compiler) compileFunction(n *pyjson.OrderedMap, typ string, cfg config, where string) validator {
	f, _ := mustGet(n, "function").(*pyjson.OrderedMap)
	if f == nil {
		c.unsupportedf("%s without function", typ)
		return unsupportedValidator{}
	}
	name := irString(f, "name")
	fv := &funcValidator{mode: strings.TrimPrefix(typ, "function-"), name_: name, fieldName: irString(f, "field_name")}
	for _, k := range f.Keys() {
		switch k {
		case "kind", "name", "field_name", "keywords":
		default:
			c.unsupportedf("function key %q", k)
		}
	}
	if kw, ok := f.Get("keywords"); ok {
		fv.keywords = map[string]pyjson.Value{}
		for k, v := range kw.(*pyjson.OrderedMap).All() {
			fv.keywords[k] = v
		}
	}
	c.functions[name] = true
	if fn, ok := c.reg.lookup(name); ok {
		fv.fn = fn
	} else {
		c.missing[name] = true
	}
	if s, ok := n.Get("schema"); ok {
		fv.inner = c.compile(s, cfg, where)
	}
	return fv
}

package pyval

import (
	"sort"
	"strings"
	"testing"

	"mobile/services/core/internal/pyjson"
)

// TestCoverageTable logs, for every IR feature, how many routes use it,
// how many of those routes bind, and whether pyval implements the feature
// itself. Run with -v to read it:
//
//	go test -run TestCoverageTable -v ./internal/pyval/
func TestCoverageTable(t *testing.T) {
	c := loadContract(t)
	reg := NewRegistry()
	type row struct{ routes, bindable int }
	rows := map[string]*row{}
	bindable := 0
	unsupportedReasons := map[string]int{}
	for _, id := range c.RouteIDs() {
		ir := c.routes[id]
		features := map[string]bool{}
		collectFeatures(ir.node, ir.defs, features, map[string]bool{})
		rep, err := c.Inspect(id, reg)
		if err != nil {
			t.Fatal(err)
		}
		ok := len(rep.Unsupported) == 0 && len(rep.Unregistered) == 0
		if ok {
			bindable++
		}
		for _, u := range rep.Unsupported {
			unsupportedReasons[u]++
		}
		if len(rep.Unregistered) > 0 {
			unsupportedReasons["validator functions without a Go port"]++
		}
		for f := range features {
			r := rows[f]
			if r == nil {
				r = &row{}
				rows[f] = r
			}
			r.routes++
			if ok {
				r.bindable++
			}
		}
	}
	names := make([]string, 0, len(rows))
	for k := range rows {
		names = append(names, k)
	}
	sort.Strings(names)
	t.Logf("%-40s %-12s %7s %9s", "feature", "implemented", "routes", "bindable")
	for _, k := range names {
		t.Logf("%-40s %-12v %7d %9d", k, featureImplemented(k), rows[k].routes, rows[k].bindable)
	}
	reasons := make([]string, 0, len(unsupportedReasons))
	for k := range unsupportedReasons {
		reasons = append(reasons, k)
	}
	sort.Strings(reasons)
	for _, k := range reasons {
		t.Logf("not bindable because %-60s routes=%d", k, unsupportedReasons[k])
	}
	t.Logf("%d of %d routes bind", bindable, len(c.RouteIDs()))
	if bindable == 0 {
		t.Fatal("no route binds")
	}
}

// collectFeatures records node types as "type", node keys as "type.key",
// parameter locations as "param:<in>" and body kinds as "body:<kind>".
func collectFeatures(v pyjson.Value, defs map[string]*pyjson.OrderedMap, out, seen map[string]bool) {
	switch x := v.(type) {
	case pyjson.List:
		for _, item := range x {
			collectFeatures(item, defs, out, seen)
		}
	case *pyjson.OrderedMap:
		if in := irString(x, "in"); in != "" {
			if _, hasSchema := x.Get("schema"); hasSchema {
				out["param:"+in] = true
			}
		}
		if kind := irString(x, "kind"); kind != "" {
			if _, isBody := x.Get("field_info"); isBody {
				out["body:"+kind] = true
			}
		}
		typ := irString(x, "type")
		if _, known := nodeKeys[typ]; known || strings.HasPrefix(typ, "function-") || typ == "date" || typ == "datetime" {
			out[typ] = true
			for _, k := range x.Keys() {
				switch k {
				case "type", "schema", "items_schema", "keys_schema", "values_schema", "choices", "fields", "function",
					"ref", "class", "config", "custom_init", "root_model", "decorators", "model_name", "default":
				default:
					out[typ+"."+k] = true
				}
			}
			if cfg, ok := x.Get("config"); ok {
				for k, val := range cfg.(*pyjson.OrderedMap).All() {
					out["config."+k+"="+scalarRepr(val)] = true
				}
			}
			if typ == "ref" {
				name := irString(x, "ref")
				if !seen[name] {
					seen[name] = true
					collectFeatures(defs[name], defs, out, seen)
				}
			}
		}
		for _, k := range x.Keys() {
			child, _ := x.Get(k)
			collectFeatures(child, defs, out, seen)
		}
	}
}

func scalarRepr(v pyjson.Value) string {
	switch x := v.(type) {
	case pyjson.String:
		return string(x)
	case pyjson.Bool:
		if x {
			return "true"
		}
		return "false"
	}
	return "?"
}

func featureImplemented(f string) bool {
	switch {
	case strings.HasPrefix(f, "param:"):
		return f != "param:cookie"
	case strings.HasPrefix(f, "body:"):
		return f == "body:json" || f == "body:form" || f == "body:multipart"
	case strings.HasPrefix(f, "config."):
		return strings.HasPrefix(f, "config.extra_fields_behavior=") || strings.HasPrefix(f, "config.strict=")
	}
	typ, key, _ := strings.Cut(f, ".")
	keys, known := nodeKeys[typ]
	if !known {
		return false
	}
	if key == "" {
		return true
	}
	if typ == "default" && key == "default_factory_takes_data" {
		return true
	}
	return contains(keys, key)
}

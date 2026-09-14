package pyval

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	"mobile/services/core/internal/pyjson"
)

// normalizeJSON re-encodes a JSON document with sorted keys and exact numbers
// so two answers compare as strings.
func normalizeJSON(t *testing.T, line []byte) string {
	t.Helper()
	var v any
	dec := json.NewDecoder(bytes.NewReader(line))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		t.Fatalf("bad outcome %q: %v", clipText(string(line), 300), err)
	}
	out, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func clipText(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// goTree renders a Value as the oracles' value tree: str as the latin-1 image
// of its surrogatepass UTF-8, floats as repr, models with fields in
// declaration order and the fields set.
func goTree(v Value) any {
	switch x := v.(type) {
	case nil, pyjson.Null:
		return []any{"n"}
	case pyjson.Bool:
		return []any{"b", bool(x)}
	case pyjson.Int:
		return []any{"i", x.String()}
	case pyjson.Float:
		return []any{"f", pyjson.FloatRepr(float64(x))}
	case pyjson.String:
		return []any{"s", latin1(string(x))}
	case Bytes:
		return []any{"y", latin1(string(x))}
	case UUID:
		return []any{"u", x.String()}
	case Date:
		return []any{"date", x.ISOFormat()}
	case DateTime:
		return []any{"dt", x.ISOFormat()}
	case *Model:
		fields := []any{}
		for _, f := range x.Fields {
			fields = append(fields, []any{f.Name, goTree(f.Value)})
		}
		set := []any{}
		for _, s := range x.FieldsSet {
			set = append(set, s)
		}
		return []any{"m", x.Class, fields, set}
	case List:
		items := []any{}
		for _, item := range x {
			items = append(items, goTree(item))
		}
		return []any{"l", items}
	case pyjson.List:
		items := []any{}
		for _, item := range x {
			items = append(items, goTree(item))
		}
		return []any{"l", items}
	case *Dict:
		items := []any{}
		for i := 0; i < x.Len(); i++ {
			k, val := x.Entry(i)
			items = append(items, []any{goTree(k), goTree(val)})
		}
		return []any{"d", items}
	case *pyjson.OrderedMap:
		items := []any{}
		for k, val := range x.All() {
			items = append(items, []any{goTree(pyjson.String(k)), goTree(val)})
		}
		return []any{"d", items}
	}
	return []any{"?", fmt.Sprintf("%T", v)}
}

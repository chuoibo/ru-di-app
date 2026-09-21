package pyval

import (
	"math"
	"math/big"
	"strconv"
	"testing"

	"mobile/services/core/internal/pyjson"
)

// Outcomes below were measured with pydantic_core.SchemaValidator in
// mobile-parity-api:7bf58e3d; TestOracleScalars checks the same
// validators against thousands of generated inputs.
func TestMeasuredCoercions(t *testing.T) {
	big20 := pyjson.NewBigInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(20), nil))
	cases := []struct {
		schema string
		in     pyjson.Value
		want   string // "ok:<tree>" or the error type and, when useful, ctx.error
	}{
		{`{"type":"int"}`, pyjson.String("1.00"), `ok:["i","1"]`},
		{`{"type":"int"}`, pyjson.String("1."), "int_parsing"},
		{`{"type":"int"}`, pyjson.String("1_0.0"), `ok:["i","10"]`},
		{`{"type":"int"}`, pyjson.String("1.0_0"), "int_parsing"},
		{`{"type":"int"}`, pyjson.String("1__0"), "int_parsing"},
		{`{"type":"int"}`, pyjson.String("-_1"), "int_parsing"},
		{`{"type":"int"}`, pyjson.String(" +1_0 "), `ok:["i","10"]`},
		{`{"type":"int"}`, pyjson.String("1\x1c"), "int_parsing"},
		{`{"type":"int"}`, pyjson.Float(9.3e18), "int_parsing_size"},
		{`{"type":"int"}`, pyjson.String("\xed\xa0\x80"), "string_unicode"},
		{`{"type":"float"}`, pyjson.String("-_1"), `ok:["f","-1.0"]`},
		{`{"type":"float"}`, pyjson.String("1_.25"), `ok:["f","1.25"]`},
		{`{"type":"float"}`, pyjson.String("1__0"), "float_parsing"},
		{`{"type":"float"}`, pyjson.String(" 1_0"), "float_parsing"},
		{`{"type":"float"}`, pyjson.String("-nan"), `ok:["f","nan"]`},
		{`{"type":"float"}`, pyjson.NewBigInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(400), nil)), "float_type"},
		{`{"type":"bool"}`, pyjson.Float(0.5), "bool_type"},
		{`{"type":"bool"}`, pyjson.Float(2), "bool_parsing"},
		{`{"type":"bool"}`, big20, "bool_type"},
		{`{"type":"bool"}`, pyjson.String("ye\u017f"), "bool_parsing"},
		{`{"type":"date"}`, pyjson.String("2024-01-02T00:00:00+07:00"), `ok:["date","2024-01-02"]`},
		{`{"type":"date"}`, pyjson.String("2024-01-02T01:00:00"), "date_from_datetime_inexact"},
		{`{"type":"date"}`, pyjson.String("2024-01-02X03:04:05"), "date_from_datetime_parsing: invalid datetime separator, expected `T`, `t`, `_` or space"},
		{`{"type":"date"}`, pyjson.String("0000-01-01"), "date_parsing: year 0 is out of range"},
		{`{"type":"datetime"}`, pyjson.String("2024-01-02X03:04:05"), "datetime_from_date_parsing: unexpected extra characters at the end of the input"},
		{`{"type":"datetime"}`, pyjson.String("2024-01-02T03:04:05+0700"), `ok:["dt","2024-01-02T03:04:05+07:00"]`},
		{`{"type":"datetime"}`, pyjson.String("2024-01-02T03:04:05,5z"), `ok:["dt","2024-01-02T03:04:05.500000+00:00"]`},
		{`{"type":"datetime"}`, pyjson.String(strconv.FormatInt(2e10+1, 10)), `ok:["dt","1970-08-20T11:33:20.001000+00:00"]`},
		{`{"type":"datetime"}`, pyjson.Float(2e10 + 0.5), `ok:["dt","2603-10-11T11:33:20.000500+00:00"]`},
		{`{"type":"datetime"}`, pyjson.Float(-1.25), `ok:["dt","1969-12-31T23:59:58.250000+00:00"]`},
		{`{"type":"datetime"}`, pyjson.Float(math.NaN()), "datetime_parsing: NaN values not permitted"},
		{`{"type":"datetime","microseconds_precision":"error"}`, pyjson.String("1.1234567"), "datetime_from_date_parsing: input is too short"},
		{`{"type":"datetime"}`, pyjson.String(strconv.FormatInt(math.MaxInt64, 10) + ".5"), "datetime_from_date_parsing: invalid date separator, expected `-`"},
		{`{"type":"datetime"}`, pyjson.Bool(true), "datetime_type"},
	}
	for _, tc := range cases {
		node, err := pyjson.Loads([]byte(tc.schema))
		if err != nil {
			t.Fatal(err)
		}
		v := newCompiler(nil, NewRegistry()).compile(node, config{}, "test")
		got, e := v.validate(newState(), tc.in)
		var outcome string
		switch {
		case e == nil:
			enc, _ := pyjson.Compact(treeValue(got))
			outcome = "ok:" + string(enc)
		case e.fatal != nil:
			outcome = "fatal: " + e.fatal.Error()
		default:
			outcome = e.lines[0].typ
			if msg, ok := e.lines[0].ctx.Get("error"); ok && len(tc.want) > len(outcome) {
				if s, ok := msg.(pyjson.String); ok {
					outcome += ": " + string(s)
				}
			}
		}
		if outcome != tc.want {
			t.Errorf("%s %#v:\n got  %s\n want %s", tc.schema, tc.in, outcome, tc.want)
		}
	}
}

// treeValue renders a scalar Value as the oracle's value tree.
func treeValue(v Value) pyjson.Value {
	switch x := v.(type) {
	case pyjson.Int:
		return pyjson.List{pyjson.String("i"), pyjson.String(x.String())}
	case pyjson.Float:
		return pyjson.List{pyjson.String("f"), pyjson.String(pyjson.FloatRepr(float64(x)))}
	case pyjson.Bool:
		return pyjson.List{pyjson.String("b"), x}
	case Date:
		return pyjson.List{pyjson.String("date"), pyjson.String(x.ISOFormat())}
	case DateTime:
		return pyjson.List{pyjson.String("dt"), pyjson.String(x.ISOFormat())}
	}
	return pyjson.String("?")
}

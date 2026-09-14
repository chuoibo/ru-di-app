//go:build oracle

package pyval

// Differential test of single schema nodes against pydantic-core in the
// parity image. Run from services/core:
//
//	go test -tags oracle -run TestOracleScalars -v -timeout 30m ./internal/pyval/
//
// Each case is an IR schema node and one input value. Python builds
// pydantic_core.SchemaValidator from the very same node JSON and calls
// validate_python(value, from_attributes=True); Go compiles the node and
// runs the validator. A refusal is compared as the bytes FastAPI's 422
// handler would write for it at the top level, an acceptance as a value
// tree. PYVAL_ORACLE_IMAGE, PYVAL_ORACLE_SEED and PYVAL_ORACLE_CACHE work as
// in TestOracle.

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"math/rand/v2"
	"net/http/httptest"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"mobile/services/core/internal/httpapi/problem"
	"mobile/services/core/internal/pyjson"
)

const scalarDriver = `
import datetime as dt, json, struct, sys, uuid
from fastapi.encoders import jsonable_encoder
from pydantic_core import SchemaValidator, ValidationError
sys.set_int_max_str_digits(0)

def text(h):
    return bytes.fromhex(h).decode("utf-8", "surrogatepass")

def build(t):
    k = t[0]
    if k == "n": return None
    if k == "b": return t[1]
    if k == "i": return int(t[1])
    if k == "f": return struct.unpack(">d", bytes.fromhex(t[1]))[0]
    if k == "s": return text(t[1])
    if k == "l": return [build(x) for x in t[1]]
    return {text(key): build(x) for key, x in t[1]}

def lat(b):
    return b.decode("latin-1")

def tree(v):
    if v is None: return ["n"]
    if v is True or v is False: return ["b", v]
    if type(v) is int: return ["i", str(v)]
    if type(v) is float: return ["f", repr(v)]
    if type(v) is str: return ["s", lat(v.encode("utf-8", "surrogatepass"))]
    if isinstance(v, uuid.UUID): return ["u", str(v)]
    if isinstance(v, dt.datetime): return ["dt", v.isoformat()]
    if isinstance(v, dt.date): return ["date", v.isoformat()]
    if type(v) is list: return ["l", [tree(x) for x in v]]
    if type(v) is dict: return ["d", [[tree(k), tree(x)] for k, x in v.items()]]
    return ["?", type(v).__name__]

validators = {}
for line in sys.stdin:
    req = json.loads(line)
    key = req["schema"]
    if key not in validators:
        validators[key] = SchemaValidator(json.loads(key))
    try:
        out = {"ok": tree(validators[key].validate_python(build(req["v"]), from_attributes=True))}
    except ValidationError as e:
        errs = [{k: x for k, x in err.items() if k != "input"} for err in e.errors(include_url=False)]
        body = json.dumps(jsonable_encoder({"detail": errs}), ensure_ascii=False, allow_nan=False, separators=(",", ":"))
        out = {"err": lat(body.encode("utf-8"))}
    except Exception as e:
        out = {"exc": type(e).__name__}
    sys.stdout.write(json.dumps(out) + "\n")
`

type scalarCase struct {
	category string
	schema   string
	value    pyjson.Value
}

func TestOracleScalars(t *testing.T) {
	image := envOr("PYVAL_ORACLE_IMAGE", "mobile-parity-api:7bf58e3d")
	seed, err := strconv.ParseUint(envOr("PYVAL_ORACLE_SEED", "1"), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	g := &gen{r: rand.New(rand.NewPCG(seed, 0xDA7E))}
	var cases []scalarCase
	add := func(category, schema string, values ...pyjson.Value) {
		for _, v := range values {
			cases = append(cases, scalarCase{category, schema, v})
		}
	}

	for _, schema := range []string{`{"type":"date"}`, `{"type":"datetime","microseconds_precision":"truncate"}`, `{"type":"datetime","microseconds_precision":"error"}`} {
		cat := strings.Split(strings.Split(schema, `"type":"`)[1], `"`)[0]
		for i := 0; i < 2500; i++ {
			add(cat, schema, pyjson.String(g.dateTimeText()))
		}
		for i := 0; i < 600; i++ {
			add(cat, schema, g.timestampValue())
		}
		add(cat, schema, pyjson.Null{}, pyjson.Bool(true), pyjson.Bool(false), pyjson.List{}, pyjson.NewOrderedMap(),
			pyjson.String("\xed\xa0\x80"), pyjson.String("2024-01-02\xed\xa0\x80"))
	}
	for _, schema := range []string{`{"type":"int"}`, `{"type":"int","strict":true}`, `{"type":"int","ge":1,"le":100}`, `{"type":"int","gt":-5,"lt":5,"multiple_of":2}`} {
		for i := 0; i < 1500; i++ {
			add("int", schema, g.numberish())
		}
	}
	for _, schema := range []string{`{"type":"float"}`, `{"type":"float","strict":true}`, `{"type":"float","ge":-90,"le":90}`, `{"type":"float","allow_inf_nan":false,"gt":0.5}`} {
		for i := 0; i < 1500; i++ {
			add("float", schema, g.numberish())
		}
	}
	for _, schema := range []string{`{"type":"bool"}`, `{"type":"bool","strict":true}`} {
		for i := 0; i < 800; i++ {
			add("bool", schema, g.boolish())
		}
	}
	for i := 0; i < 1500; i++ {
		add("uuid", `{"type":"uuid"}`, pyjson.String(g.uuidSpelling(i%4 == 0)))
	}
	add("uuid", `{"type":"uuid"}`, pyjson.NewInt(1), pyjson.Null{}, pyjson.String("\xed\xa0\x80"), pyjson.List{})
	for _, schema := range []string{`{"type":"str","strict":true,"max_length":5}`, `{"type":"str","min_length":2,"pattern":"^[A-Za-z0-9_-]+$"}`, `{"type":"str"}`} {
		for i := 0; i < 700; i++ {
			add("str", schema, g.stringish())
		}
	}
	for _, schema := range []string{`{"type":"literal","expected":["a","b"]}`, `{"type":"literal","expected":[1,"x",null]}`, `{"type":"literal","expected":[true,"y"]}`} {
		for _, v := range []pyjson.Value{pyjson.String("a"), pyjson.String("A"), pyjson.String("x"), pyjson.NewInt(1), pyjson.Float(1.0),
			pyjson.Bool(true), pyjson.Bool(false), pyjson.Null{}, pyjson.NewInt(0), pyjson.String("1"), pyjson.String("\xed\xa0\x80"),
			pyjson.List{}, pyjson.Float(math.NaN()), pyjson.String("y"), pyjson.NewInt(2)} {
			add("literal", schema, v)
		}
	}
	for i := 0; i < 400; i++ {
		add("list", `{"type":"list","items_schema":{"type":"str","strict":true},"min_length":1,"max_length":3}`, g.listish())
		add("dict", `{"type":"dict","keys_schema":{"type":"uuid"},"values_schema":{"type":"int","strict":true},"max_length":2}`, g.dictish())
	}

	var stdin bytes.Buffer
	for _, c := range cases {
		line, err := json.Marshal(map[string]any{"schema": c.schema, "v": scalarTransport(c.value)})
		if err != nil {
			t.Fatal(err)
		}
		stdin.Write(line)
		stdin.WriteByte('\n')
	}
	started := time.Now()
	lines := pythonAnswersWith(t, image, scalarDriver, stdin.Bytes(), len(cases))

	compiled := map[string]validator{}
	total, bad := map[string]int{}, map[string]int{}
	shown, limit := 0, 30
	if n, err := strconv.Atoi(os.Getenv("PYVAL_ORACLE_SHOW")); err == nil {
		limit = n
	}
	for i, c := range cases {
		v, ok := compiled[c.schema]
		if !ok {
			node, err := pyjson.Loads([]byte(c.schema))
			if err != nil {
				t.Fatal(err)
			}
			comp := newCompiler(nil, NewRegistry())
			v = comp.compile(node, config{}, "scalar")
			if len(comp.unsupported) > 0 {
				t.Fatalf("%s: %v", c.schema, sortedKeys(comp.unsupported))
			}
			compiled[c.schema] = v
		}
		want := normalizeJSON(t, lines[i])
		got := normalizeJSON(t, scalarOutcome(t, v, c.value))
		total[c.category]++
		if got == want {
			continue
		}
		bad[c.category]++
		if shown < limit {
			shown++
			in, _ := json.Marshal(scalarTransport(c.value))
			if s, ok := c.value.(pyjson.String); ok {
				in = []byte(strconv.Quote(string(s)))
			}
			t.Errorf("mismatch %s %s\n  input:  %s\n  python: %s\n  go:     %s", c.category, c.schema, clip(string(in), 300), clip(want, 500), clip(got, 500))
		}
	}
	cats := make([]string, 0, len(total))
	for k := range total {
		cats = append(cats, k)
	}
	sort.Strings(cats)
	mismatches := 0
	for _, k := range cats {
		t.Logf("%-10s cases=%6d mismatches=%d", k, total[k], bad[k])
		mismatches += bad[k]
	}
	t.Logf("scalar oracle %s seed=%d: %d cases, %d mismatches, took %s", image, seed, len(cases), mismatches, time.Since(started).Round(time.Millisecond))
}

func scalarOutcome(t *testing.T, v validator, in pyjson.Value) []byte {
	t.Helper()
	got, e := v.validate(newState(), in)
	if e == nil {
		out, _ := json.Marshal(map[string]any{"ok": goTree(got)})
		return out
	}
	if e.fatal != nil {
		out, _ := json.Marshal(map[string]any{"exc": e.fatal.Error()})
		return out
	}
	errs := make([]problem.ValidationError, len(e.lines))
	for i, l := range e.lines {
		loc := pyjson.List{}
		for _, item := range l.loc {
			switch x := item.(type) {
			case string:
				loc = append(loc, pyjson.String(x))
			case int:
				loc = append(loc, pyjson.NewInt(int64(x)))
			}
		}
		errs[i] = problem.ValidationError{Type: l.typ, Loc: loc, Msg: l.msg, Ctx: l.ctx}
	}
	rec := httptest.NewRecorder()
	if err := problem.WriteValidation(rec, errs); err != nil {
		t.Fatal(err)
	}
	out, _ := json.Marshal(map[string]any{"err": latin1(rec.Body.String())})
	return out
}

func scalarTransport(v pyjson.Value) any {
	switch x := v.(type) {
	case pyjson.Null:
		return []any{"n"}
	case pyjson.Bool:
		return []any{"b", bool(x)}
	case pyjson.Int:
		return []any{"i", x.String()}
	case pyjson.Float:
		return []any{"f", fmt.Sprintf("%016x", math.Float64bits(float64(x)))}
	case pyjson.String:
		return []any{"s", hex.EncodeToString([]byte(x))}
	case pyjson.List:
		items := []any{}
		for _, item := range x {
			items = append(items, scalarTransport(item))
		}
		return []any{"l", items}
	case *pyjson.OrderedMap:
		items := []any{}
		for k, val := range x.All() {
			items = append(items, []any{hex.EncodeToString([]byte(k)), scalarTransport(val)})
		}
		return []any{"o", items}
	}
	panic(fmt.Sprintf("no transport for %T", v))
}

// ---- generators ----

func (g *gen) dateTimeText() string {
	var b strings.Builder
	year := pick(g, 0, 1, 1969, 1970, 2000, 2023, 2024, 2100, 9999, g.r.IntN(10000))
	b.WriteString(pad(year, 4))
	b.WriteString(pick(g, "-", "-", "-", "-", "/", ""))
	b.WriteString(pad(pick(g, 1, 2, 12, 0, 13, g.r.IntN(14)), 2))
	b.WriteString(pick(g, "-", "-", "-", "-", "_"))
	b.WriteString(pad(pick(g, 1, 28, 29, 30, 31, 0, 32, g.r.IntN(33)), 2))
	if g.chance(0.8) {
		b.WriteString(pick(g, "T", "T", "t", " ", "_", "X", "-", ""))
		b.WriteString(pad(pick(g, 0, 3, 23, 24, g.r.IntN(26)), 2))
		b.WriteString(pick(g, ":", ":", ":", "-", ""))
		b.WriteString(pad(pick(g, 0, 4, 59, 60, g.r.IntN(62)), 2))
		if g.chance(0.75) {
			b.WriteString(pick(g, ":", ":", ":", "."))
			b.WriteString(pad(pick(g, 0, 5, 59, 60, g.r.IntN(62)), 2))
			if g.chance(0.5) {
				b.WriteString(pick(g, ".", ".", ","))
				for i := g.r.IntN(10); i > 0; i-- {
					b.WriteByte(byte('0' + g.r.IntN(10)))
				}
			}
		}
		if g.chance(0.6) {
			switch g.r.IntN(12) {
			case 0, 1:
				b.WriteString(pick(g, "Z", "z"))
			case 2:
				b.WriteString(pick(g, "+", "-") + pad(g.r.IntN(26), 2) + ":" + pad(pick(g, 0, 30, 59, 60), 2))
			case 3:
				b.WriteString(pick(g, "+", "-") + pad(g.r.IntN(24), 2) + pad(g.r.IntN(61), 2))
			case 4:
				b.WriteString(pick(g, "+07", "+7:00", " +07:00", "\u221207:00", "+07:0", "+07:", "+", "Zx", "+07:00:00", "UTC"))
			default:
				b.WriteString(pick(g, "+", "-") + pad(pick(g, 0, 7, 23, 24), 2) + ":" + pad(pick(g, 0, 30, 45), 2))
			}
		}
	}
	s := []byte(b.String())
	switch g.r.IntN(10) {
	case 0:
		if len(s) > 0 {
			i := g.r.IntN(len(s))
			s = append(s[:i:i], s[i+1:]...)
		}
	case 1:
		i := g.r.IntN(len(s) + 1)
		s = append(s[:i:i], append([]byte(pick(g, "0", "a", " ", ":", "-", "\u00e9")), s[i:]...)...)
	case 2:
		s = s[:g.r.IntN(len(s)+1)]
	case 3:
		s = append(s, pick(g, " ", "\n", "x", "Z", "0")...)
	case 4:
		s = append([]byte(pick(g, " ", "+", "0", "\uff12")), s...)
	}
	// Cutting or splicing may split a UTF-8 sequence; a JSON body can only
	// ever carry whole code points.
	return strings.ToValidUTF8(string(s), "?")
}

func (g *gen) timestampValue() pyjson.Value {
	base := pick(g, int64(0), 1, -1, 86400, -86400, 86399, 1_700_000_000, 20_000_000_000, 20_000_000_001, -20_000_000_001,
		253_402_300_799, 253_402_300_800, -62_167_219_200, -62_167_219_201, 1_700_000_000_000, math.MaxInt64, math.MinInt64+1,
		g.r.Int64N(4_000_000_000_000)-2_000_000_000_000, g.r.Int64N(40_000_000_000)-20_000_000_000)
	switch g.r.IntN(8) {
	case 0:
		return pyjson.NewInt(base)
	case 1:
		return pyjson.Float(float64(base) + pick(g, 0, 0.5, 0.25, -0.25, 0.123_456_789, 0.999_999_9))
	case 2:
		return pyjson.String(strconv.FormatInt(base, 10))
	case 3:
		return pyjson.String(strconv.FormatInt(base, 10) + pick(g, ".5", ".", ".0", "."+"123456"+"789", "."+strings.Repeat("9", 7)))
	case 4:
		return pyjson.String(pick(g, "+", "-", "00", "") + strconv.FormatInt(g.r.Int64N(100000), 10) + pick(g, "", ".25", "e3"))
	case 5:
		return pick[pyjson.Value](g, pyjson.Float(math.NaN()), pyjson.Float(math.Inf(1)), pyjson.Float(math.Inf(-1)),
			pyjson.NewBigInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(20), nil)), pyjson.Float(1e20), pyjson.Float(-1e20),
			pyjson.NewBigInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(400), nil)), pyjson.String(".5"), pyjson.String("-.5"),
			pyjson.String("5."), pyjson.String(strings.Repeat("9", 20)), pyjson.String(strings.Repeat("9", 20)+".5"), pyjson.String(""))
	case 6:
		return pyjson.Float(pick(g, 1e11, -1e11, 86400.0, -1.5, -1.25, 1.5, 20_000_000_000.5, -0.000001, 0.0000005))
	}
	return pyjson.String(strconv.FormatFloat(float64(base)/1000, 'f', g.r.IntN(8), 64))
}

func (g *gen) numberish() pyjson.Value {
	if g.chance(0.35) {
		return pick[pyjson.Value](g, pyjson.NewInt(g.r.Int64N(300)-150), pyjson.Float(g.r.Float64()*300-150),
			pyjson.Float(float64(g.r.IntN(200)-100)), pyjson.Bool(g.chance(0.5)), pyjson.Null{}, pyjson.List{},
			pyjson.Float(math.NaN()), pyjson.Float(math.Inf(1)), pyjson.Float(1e20), pyjson.Float(9.2e18), pyjson.Float(9.3e18),
			pyjson.NewBigInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(20), nil)),
			pyjson.NewBigInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(400), nil)), pyjson.Float(0.5), pyjson.NewInt(0))
	}
	var b strings.Builder
	b.WriteString(pick(g, "", "", "", " ", "\t", "\u00a0", "\x1c", "\n", "\u3000"))
	b.WriteString(pick(g, "", "", "", "+", "-", "+-", "--"))
	parts := []string{"0", "1", "7", "12", "100", "99", "5", "00", "1_0", "1__0", "_1", "1_", "\uff11", "\u0661"}
	for i := 1 + g.r.IntN(2); i > 0; i-- {
		b.WriteString(pick(g, parts...))
	}
	switch g.r.IntN(8) {
	case 0:
		b.WriteString(pick(g, ".", ".0", ".00", ".5", ".0_0", ".25", "._5"))
	case 1:
		b.WriteString(pick(g, "e3", "E-2", "e", "e+1"))
	case 2:
		b.WriteString(pick(g, "x", "d", "f", "\xed\xa0\x80"))
	}
	b.WriteString(pick(g, "", "", "", " ", "\n", "\u00a0", "\x1c"))
	if g.chance(0.05) {
		return pyjson.String(pick(g, "nan", "-NaN", "inf", "+Infinity", "infinity", "0x10", "1e400", "", ".", "-.5", "5.", "\uff11\uff12"))
	}
	return pyjson.String(b.String())
}

func (g *gen) boolish() pyjson.Value {
	words := []string{"true", "TRUE", "True", "t", "T", "yes", "YES", "y", "on", "ON", "1", "false", "f", "no", "n", "off", "0",
		" true", "true ", "2", "", "ye\u017f", "tru", "nope", "\xed\xa0\x80"}
	switch g.r.IntN(3) {
	case 0:
		return pyjson.String(pick(g, words...))
	case 1:
		return pick[pyjson.Value](g, pyjson.NewInt(0), pyjson.NewInt(1), pyjson.NewInt(2), pyjson.NewInt(-1), pyjson.Float(0),
			pyjson.Float(1), pyjson.Float(0.5), pyjson.Float(2), pyjson.Float(math.NaN()), pyjson.Float(math.Inf(1)),
			pyjson.NewBigInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(20), nil)), pyjson.Float(-0.0))
	}
	return pick[pyjson.Value](g, pyjson.Bool(true), pyjson.Bool(false), pyjson.Null{}, pyjson.List{}, pyjson.NewOrderedMap())
}

func (g *gen) stringish() pyjson.Value {
	if g.chance(0.15) {
		return pick[pyjson.Value](g, pyjson.NewInt(1), pyjson.Float(1.5), pyjson.Bool(true), pyjson.Null{}, pyjson.List{})
	}
	units := []string{"a", "Z", "0", "_", "-", " ", "\u00e9", "\U0001F600", "\xed\xa0\x80", "!", "\u00a0", "\n", "e\u0301"}
	var b strings.Builder
	for i := g.r.IntN(8); i > 0; i-- {
		b.WriteString(pick(g, units...))
	}
	return pyjson.String(b.String())
}

func (g *gen) listish() pyjson.Value {
	if g.chance(0.1) {
		return pick[pyjson.Value](g, pyjson.String("ab"), pyjson.Null{}, pyjson.NewOrderedMap())
	}
	l := pyjson.List{}
	for i := g.r.IntN(6); i > 0; i-- {
		l = append(l, pick[pyjson.Value](g, pyjson.String("x"), pyjson.String("y"), pyjson.NewInt(1), pyjson.Null{}))
	}
	return l
}

func (g *gen) dictish() pyjson.Value {
	if g.chance(0.1) {
		return pick[pyjson.Value](g, pyjson.List{}, pyjson.Null{}, pyjson.String("x"))
	}
	m := pyjson.NewOrderedMap()
	for i := g.r.IntN(4); i > 0; i-- {
		key := pick(g, g.uuidCanonical(), strings.ToUpper(g.uuidCanonical()), "nope", "")
		m.Set(key, pick[pyjson.Value](g, pyjson.NewInt(3), pyjson.String("3"), pyjson.Bool(true), pyjson.Null{}))
	}
	return m
}

func pythonAnswersWith(t *testing.T, image, driver string, stdin []byte, n int) [][]byte {
	t.Helper()
	saved := oracleDriverOverride
	oracleDriverOverride = driver
	defer func() { oracleDriverOverride = saved }()
	return pythonAnswers(t, image, stdin, n)
}

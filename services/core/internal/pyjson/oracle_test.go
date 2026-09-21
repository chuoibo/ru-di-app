//go:build oracle

package pyjson

// Differential test against the Python parity image. Run from services/core:
//
//	go test -tags oracle -run TestOracle -v -timeout 30m ./internal/pyjson/
//
// PYJSON_ORACLE_IMAGE overrides the image, PYJSON_ORACLE_SEED the seed.
// Every case travels in one docker run as a JSON line. Floats travel as their
// IEEE bits and strings as hex of their generalized UTF-8 bytes, so nothing
// is lost in transport. The Python side answers with the real code paths:
// starlette JSONResponse, json.dumps, json.loads and the idempotency
// middleware's _canonical_body.

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"math/rand/v2"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

const oracleScript = `
import datetime as dt, json, struct, sys, uuid
from pydantic import TypeAdapter
from starlette.responses import JSONResponse
from app.api.idempotency import _canonical_body

sys.setrecursionlimit(200000)
DT, DATE, UUID = TypeAdapter(dt.datetime), TypeAdapter(dt.date), TypeAdapter(uuid.UUID)

def text(h):
    return bytes.fromhex(h).decode("utf-8", "surrogatepass")

def build(t):
    k = t[0]
    if k == "n": return None
    if k == "b": return t[1]
    if k == "i": return int(t[1], 16)
    if k == "f": return struct.unpack(">d", bytes.fromhex(t[1]))[0]
    if k == "s": return text(t[1])
    if k == "l": return [build(x) for x in t[1]]
    return {text(key): build(x) for key, x in t[1]}

def tree(v):
    if v is None: return ["n"]
    if v is True or v is False: return ["b", v]
    if type(v) is int: return ["i", format(v, "x") if v >= 0 else "-" + format(-v, "x")]
    if type(v) is float: return ["f", struct.pack(">d", v).hex()]
    if type(v) is str: return ["s", v.encode("utf-8", "surrogatepass").hex()]
    if type(v) is list: return ["l", [tree(x) for x in v]]
    return ["o", [[k.encode("utf-8", "surrogatepass").hex(), tree(x)] for k, x in v.items()]]

def attempt(f):
    try:
        return ["ok", f().hex()]
    except RecursionError:
        return ["err", "RecursionError"]
    except UnicodeEncodeError as e:
        return ["err", "UnicodeEncodeError", str(e)]
    except ValueError as e:
        return ["err", "ValueError", str(e)]

def canonical(v):
    return json.dumps(v, sort_keys=True, separators=(",", ":"), ensure_ascii=False).encode("utf-8")

for line in sys.stdin:
    req = json.loads(line)
    op = req["op"]
    if op == "enc":
        v = build(req["v"])
        out = {
            "compact": attempt(lambda: JSONResponse(content=v).body),
            "dumps": attempt(lambda: json.dumps(v).encode("ascii")),
            "canonical": attempt(lambda: canonical(v)),
        }
        if type(v) is float:
            out["repr"] = repr(v)
    elif op == "dec":
        b = bytes.fromhex(req["h"])
        out = {"fingerprint": attempt(lambda: _canonical_body(b, "application/json"))}
        try:
            v = json.loads(b)
        except json.JSONDecodeError as e:
            out["loads"] = ["JSONDecodeError", e.msg, e.pos]
        except UnicodeDecodeError:
            out["loads"] = ["err", "UnicodeDecodeError"]
        except RecursionError:
            out["loads"] = ["err", "RecursionError"]
        except ValueError as e:
            out["loads"] = ["err", "ValueError", str(e)]
        else:
            out["loads"] = ["ok", tree(v)]
            out["canonical"] = attempt(lambda: canonical(v))
            out["dumps"] = attempt(lambda: json.dumps(v).encode("ascii"))
    elif op == "dt":
        off = req["off"]
        tz = None if off is None else dt.timezone(dt.timedelta(seconds=off))
        v = dt.datetime(req["y"], req["mo"], req["d"], req["h"], req["mi"], req["s"], req["us"], tzinfo=tz)
        out = {"s": DT.dump_python(v, mode="json")}
    elif op == "date":
        out = {"s": DATE.dump_python(dt.date(req["y"], req["mo"], req["d"]), mode="json")}
    else:
        out = {"s": UUID.dump_python(uuid.UUID(bytes=bytes.fromhex(req["h"])), mode="json")}
    sys.stdout.write(json.dumps(out) + "\n")
`

type oracleCase struct {
	category string
	req      map[string]any
	want     map[string]any // what Go computes, in the oracle's response shape
}

func TestOracle(t *testing.T) {
	image := os.Getenv("PYJSON_ORACLE_IMAGE")
	if image == "" {
		image = "mobile-parity-api:7bf58e3d"
	}
	seed := uint64(1)
	if s := os.Getenv("PYJSON_ORACLE_SEED"); s != "" {
		var err error
		if seed, err = strconv.ParseUint(s, 10, 64); err != nil {
			t.Fatalf("PYJSON_ORACLE_SEED: %v", err)
		}
	}
	g := &gen{r: rand.New(rand.NewPCG(seed, 0x9E3779B97F4A7C15))}

	var cases []oracleCase
	add := func(category string, req, want map[string]any) {
		cases = append(cases, oracleCase{category, req, want})
	}
	encode := func(category string, v Value) {
		add(category, map[string]any{"op": "enc", "v": transport(v)}, goEncode(v))
	}
	decode := func(category string, b []byte) {
		add(category, map[string]any{"op": "dec", "h": hex.EncodeToString(b)}, goDecode(b))
	}

	for i := 0; i < 40000; i++ {
		encode("float", Float(g.float()))
	}
	for i := 0; i < 12000; i++ {
		encode("string", String(g.str(24, g.r.IntN(5) == 0)))
	}
	for i := 0; i < 6000; i++ {
		encode("tree", g.value(5, g.r.IntN(8) == 0))
	}
	for _, n := range []int{4299, 4300, 4301} {
		encode("tree", nines(n))
		encode("tree", List{NewBigInt(new(big.Int).Neg(nines(n).Big()))})
	}
	for i := 0; i < 30000; i++ {
		decode("decode", g.document())
	}
	for i := 0; i < 3000; i++ {
		decode("decode-bytes", g.rawBytes())
	}
	for _, b := range deepDocuments() {
		decode("decode-deep", b)
	}
	for i := 0; i < 4000; i++ {
		y, mo, d := 1+g.r.IntN(9999), 1+g.r.IntN(12), 1+g.r.IntN(28)
		h, mi, s := g.r.IntN(24), g.r.IntN(60), g.r.IntN(60)
		us := 0
		if g.r.IntN(2) == 0 {
			us = g.r.IntN(1000000)
		}
		var off any
		loc := time.UTC
		switch k := g.r.IntN(10); {
		case k < 2:
			off = nil
		case k < 4:
			off = 0
		default:
			o := g.r.IntN(2*86399+1) - 86399
			switch g.r.IntN(4) {
			case 0: // whole half hours, like real zones
				o = (g.r.IntN(49) - 24) * 1800 % 86400
			case 1: // under two minutes either way: sign and truncation edges
				o = g.r.IntN(241) - 120
			}
			off = o
			loc = time.FixedZone("", o)
		}
		tm := time.Date(y, time.Month(mo), d, h, mi, s, us*1000, loc)
		want := DateTime(tm)
		if off == nil {
			want = NaiveDateTime(tm)
		}
		add("datetime", map[string]any{"op": "dt", "y": y, "mo": mo, "d": d, "h": h, "mi": mi, "s": s, "us": us, "off": off},
			map[string]any{"s": want})
	}
	for i := 0; i < 1000; i++ {
		y, mo, d := 1+g.r.IntN(9999), 1+g.r.IntN(12), 1+g.r.IntN(28)
		add("date", map[string]any{"op": "date", "y": y, "mo": mo, "d": d},
			map[string]any{"s": Date(time.Date(y, time.Month(mo), d, 0, 0, 0, 0, time.UTC))})
	}
	for i := 0; i < 1000; i++ {
		var u [16]byte
		for j := range u {
			u[j] = byte(g.r.IntN(256))
		}
		add("uuid", map[string]any{"op": "uuid", "h": hex.EncodeToString(u[:])}, map[string]any{"s": UUID(u)})
	}

	var stdin bytes.Buffer
	for _, c := range cases {
		line, err := json.Marshal(c.req)
		if err != nil {
			t.Fatal(err)
		}
		stdin.Write(line)
		stdin.WriteByte('\n')
	}
	cmd := exec.Command("docker", "run", "--rm", "-i", "--network", "none", "--entrypoint", "python", image, "-c", oracleScript)
	var stdout, stderr bytes.Buffer
	cmd.Stdin, cmd.Stdout, cmd.Stderr = &stdin, &stdout, &stderr
	started := time.Now()
	if err := cmd.Run(); err != nil {
		t.Fatalf("oracle failed: %v\n%s", err, tail(stderr.String(), 2000))
	}
	lines := bytes.Split(bytes.TrimSuffix(stdout.Bytes(), []byte("\n")), []byte("\n"))
	if len(lines) != len(cases) {
		t.Fatalf("oracle answered %d lines for %d cases\n%s", len(lines), len(cases), tail(stderr.String(), 2000))
	}

	total := map[string]int{}
	bad := map[string]int{}
	outcomes := map[string]int{} // json.loads outcomes the oracle reported
	shown := 0
	for i, c := range cases {
		total[c.category]++
		if strings.HasPrefix(c.category, "decode") {
			var resp struct{ Loads []any }
			if err := json.Unmarshal(lines[i], &resp); err == nil && len(resp.Loads) >= 2 {
				key := fmt.Sprint(resp.Loads[0])
				if key != "ok" {
					key += ": " + fmt.Sprint(resp.Loads[1])
				}
				outcomes[key]++
			}
		}
		got := normalize(t, lines[i])
		wantLine, err := json.Marshal(c.want)
		if err != nil {
			t.Fatal(err)
		}
		want := normalize(t, wantLine)
		if got == want {
			continue
		}
		bad[c.category]++
		if shown < 15 {
			shown++
			req, _ := json.Marshal(c.req)
			t.Errorf("mismatch in %s\n  request: %s\n  python:  %s\n  go:      %s",
				c.category, tail(string(req), 400), tail(got, 600), tail(want, 600))
		}
	}
	categories := make([]string, 0, len(total))
	for k := range total {
		categories = append(categories, k)
	}
	sort.Strings(categories)
	mismatches := 0
	for _, k := range categories {
		t.Logf("%-14s cases=%6d mismatches=%d", k, total[k], bad[k])
		mismatches += bad[k]
	}
	outcomeKeys := make([]string, 0, len(outcomes))
	for k := range outcomes {
		outcomeKeys = append(outcomeKeys, k)
	}
	sort.Strings(outcomeKeys)
	for _, k := range outcomeKeys {
		t.Logf("  loads outcome %6d  %s", outcomes[k], k)
	}
	t.Logf("oracle %s seed=%d: %d cases, %d mismatches, python took %s",
		image, seed, len(cases), mismatches, time.Since(started).Round(time.Millisecond))
}

func normalize(t *testing.T, line []byte) string {
	t.Helper()
	var v any
	dec := json.NewDecoder(bytes.NewReader(line))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		t.Fatalf("bad oracle line %q: %v", tail(string(line), 200), err)
	}
	out, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// transport is the lossless tree both sides agree on.
func transport(v Value) []any {
	switch x := v.(type) {
	case nil, Null:
		return []any{"n"}
	case Bool:
		return []any{"b", bool(x)}
	case Int:
		return []any{"i", x.Big().Text(16)}
	case Float:
		return []any{"f", fmt.Sprintf("%016x", math.Float64bits(float64(x)))}
	case String:
		return []any{"s", hex.EncodeToString([]byte(x))}
	case List:
		items := make([]any, len(x))
		for i, item := range x {
			items[i] = transport(item)
		}
		return []any{"l", items}
	case *OrderedMap:
		pairs := []any{}
		for k, item := range x.All() {
			pairs = append(pairs, []any{hex.EncodeToString([]byte(k)), transport(item)})
		}
		return []any{"o", pairs}
	}
	panic("unknown value")
}

func goAttempt(b []byte, err error) []any {
	if err == nil {
		return []any{"ok", hex.EncodeToString(b)}
	}
	kind := errorKind(err)
	if kind == "RecursionError" {
		return []any{"err", kind}
	}
	return []any{"err", kind, err.Error()}
}

func goEncode(v Value) map[string]any {
	out := map[string]any{}
	b, err := Compact(v)
	out["compact"] = goAttempt(b, err)
	b, err = Dumps(v)
	out["dumps"] = goAttempt(b, err)
	b, err = Canonical(v)
	out["canonical"] = goAttempt(b, err)
	if f, ok := v.(Float); ok {
		out["repr"] = FloatRepr(float64(f))
	}
	return out
}

// fingerprintBody mirrors app.api.idempotency._canonical_body for a request
// that declared JSON: a ValueError from json.loads hashes the raw body, and
// anything else propagates.
func fingerprintBody(b []byte) ([]byte, error) {
	v, err := Loads(b)
	if err != nil {
		if IsValueError(err) {
			return b, nil
		}
		return nil, err
	}
	return Canonical(v)
}

func goDecode(b []byte) map[string]any {
	out := map[string]any{}
	fp, err := fingerprintBody(b)
	out["fingerprint"] = goAttempt(fp, err)
	v, err := Loads(b)
	if err != nil {
		var de *DecodeError
		switch kind := errorKind(err); {
		case errors.As(err, &de):
			out["loads"] = []any{"JSONDecodeError", de.Msg, de.Pos}
		case kind == "ValueError":
			out["loads"] = []any{"err", kind, err.Error()}
		default:
			out["loads"] = []any{"err", kind}
		}
		return out
	}
	out["loads"] = []any{"ok", transport(v)}
	c, err := Canonical(v)
	out["canonical"] = goAttempt(c, err)
	d, err := Dumps(v)
	out["dumps"] = goAttempt(d, err)
	return out
}

func deepDocuments() [][]byte {
	arr := func(n int, body string) []byte {
		return []byte(strings.Repeat("[", n) + body + strings.Repeat("]", n))
	}
	objs := func(n int) []byte {
		return []byte(strings.Repeat(`{"k": `, n) + "1" + strings.Repeat("}", n))
	}
	return [][]byte{
		arr(2000, "1"), objs(2000), arr(2000, ""), arr(12000, "1"), objs(12000),
		[]byte(strings.Repeat("[", 12000)), []byte(strings.Repeat(`{"a":[`, 7000)),
		arr(1500, `"x", 1e5, {"b": [1, 2]}`),
	}
}

type gen struct {
	r *rand.Rand
}

func (g *gen) pick(options ...string) string { return options[g.r.IntN(len(options))] }

func (g *gen) float() float64 {
	sign := uint64(g.r.IntN(2)) << 63
	switch g.r.IntN(12) {
	case 0:
		return math.Float64frombits(g.r.Uint64())
	case 1: // subnormal
		return math.Float64frombits(sign | g.r.Uint64()&(1<<52-1))
	case 2: // near the top of the range
		return math.Float64frombits(sign | uint64(2046-g.r.IntN(64))<<52 | g.r.Uint64()&(1<<52-1))
	case 3: // near the bottom of the normal range
		return math.Float64frombits(sign | uint64(1+g.r.IntN(64))<<52 | g.r.Uint64()&(1<<52-1))
	case 4: // around repr's switch points and powers of ten
		k := []int{-7, -6, -5, -4, -3, -1, 0, 1, 14, 15, 16, 17, 21, 22, 23, 308, -308, -323}[g.r.IntN(18)]
		f, _ := strconv.ParseFloat("1e"+strconv.Itoa(k), 64)
		for steps := g.r.IntN(4); steps > 0; steps-- {
			if g.r.IntN(2) == 0 {
				f = math.Nextafter(f, math.Inf(1))
			} else {
				f = math.Nextafter(f, 0)
			}
		}
		return math.Copysign(f, float64(1-2*int(sign>>63)))
	case 5: // short decimals such as money-like values
		f, _ := strconv.ParseFloat(fmt.Sprintf("%de-%d", g.r.Int64N(2000001)-1000000, g.r.IntN(12)), 64)
		return f
	case 6: // integral floats
		return math.Copysign(float64(g.r.Int64N(1<<62)>>uint(g.r.IntN(62))), float64(1-2*int(sign>>63)))
	case 7:
		specials := []float64{0, math.Copysign(0, -1), math.Inf(1), math.Inf(-1), math.NaN(), math.MaxFloat64,
			math.SmallestNonzeroFloat64, 1 << 53, 1<<53 + 1, 0.1, 0.5, 1.0 / 3}
		return specials[g.r.IntN(len(specials))]
	default: // random significant digits at a random decimal exponent
		var sb strings.Builder
		for n := 1 + g.r.IntN(17); n > 0; n-- {
			sb.WriteByte(byte('0' + g.r.IntN(10)))
		}
		f, _ := strconv.ParseFloat(sb.String()+"e"+strconv.Itoa(g.r.IntN(640)-330), 64)
		return math.Copysign(f, float64(1-2*int(sign>>63)))
	}
}

func (g *gen) codePoint(surrogates bool) rune {
	switch k := g.r.IntN(20); {
	case k < 7:
		return rune(0x20 + g.r.IntN(0x5F))
	case k == 7:
		return rune(g.r.IntN(0x20))
	case k == 8:
		specials := []rune{'"', '\\', '/', 0x7F, 0x2028, 0x2029, 0xFEFF, 0xFFFF, 0xFFFD, 0xE000, 0xD7FF, 0x10000, 0x10FFFF, 0x80, 0x7FF, 0x800}
		return specials[g.r.IntN(len(specials))]
	case k < 11:
		return rune(0x80 + g.r.IntN(0x780))
	case k < 13:
		for {
			if r := rune(0x800 + g.r.IntN(0xF800)); !isSurrogate(r) {
				return r
			}
		}
	case k < 15:
		return rune(0x10000 + g.r.IntN(0x100000))
	case k == 15 && surrogates:
		return rune(0xD800 + g.r.IntN(0x800))
	}
	return rune('a' + g.r.IntN(26))
}

func (g *gen) str(maxLen int, surrogates bool) string {
	var b []byte
	for n := g.r.IntN(maxLen + 1); n > 0; n-- {
		b = appendCodePoint(b, g.codePoint(surrogates))
	}
	return string(b)
}

func (g *gen) key(surrogates bool) string {
	if g.r.IntN(2) == 0 {
		return g.str(4, surrogates)
	}
	keys := []string{"", "a", "A", "b", "aa", "ab", "a\x00", "\x7f", "\u00e9", "e\u0301", "\uffff", "\U00010000",
		"\ue000", "\ud7ff", "\U0010ffff", "\ufeff", "1", "10", "9", "a b"}
	if surrogates {
		keys = append(keys, "\xed\xa0\x80", "\xed\xbf\xbf", "\xed\xa0\xbd\xed\xb8\x80")
	}
	return keys[g.r.IntN(len(keys))]
}

func (g *gen) int() Int {
	switch k := g.r.IntN(20); {
	case k < 10:
		return NewInt(g.r.Int64N(2001) - 1000)
	case k < 14:
		return NewInt(int64(g.r.Uint64()))
	case k < 16:
		return NewInt([]int64{math.MaxInt64, math.MinInt64, 0, -1}[g.r.IntN(4)])
	case k < 19:
		digits := 19 + g.r.IntN(60)
		return mustInt(g.pick("", "-") + g.digitString(digits))
	default:
		return mustInt(g.pick("", "-") + g.digitString(4290+g.r.IntN(20)))
	}
}

func (g *gen) digitString(n int) string {
	b := make([]byte, n)
	b[0] = byte('1' + g.r.IntN(9))
	for i := 1; i < n; i++ {
		b[i] = byte('0' + g.r.IntN(10))
	}
	return string(b)
}

func (g *gen) value(depth int, surrogates bool) Value {
	k := g.r.IntN(10)
	if depth <= 0 && k >= 8 {
		k = g.r.IntN(8)
	}
	switch k {
	case 0:
		return Null{}
	case 1:
		return Bool(g.r.IntN(2) == 0)
	case 2, 3:
		return g.int()
	case 4, 5:
		f := g.float()
		for g.r.IntN(10) != 0 && (math.IsNaN(f) || math.IsInf(f, 0)) {
			f = g.float()
		}
		return Float(f)
	case 6, 7:
		return String(g.str(12, surrogates))
	case 8:
		list := List{}
		for n := g.r.IntN(6); n > 0; n-- {
			list = append(list, g.value(depth-1, surrogates))
		}
		return list
	}
	m := NewOrderedMap()
	for n := g.r.IntN(6); n > 0; n-- {
		m.Set(g.key(surrogates), g.value(depth-1, surrogates))
	}
	return m
}

// document produces JSON-ish bytes: a random value written with random
// spacing, escapes and number spellings, then possibly damaged and
// re-encoded as UTF-16/32.
func (g *gen) document() []byte {
	var sb strings.Builder
	g.write(&sb, g.value(4, g.r.IntN(6) == 0))
	text := codePoints(sb.String())
	if g.r.IntN(2) == 0 {
		text = g.mutate(text)
	}
	b := g.encodeText(text)
	if g.r.IntN(25) == 0 && len(b) > 0 {
		i := g.r.IntN(len(b))
		b = append(b[:i:i], append([]byte{byte(g.r.IntN(256))}, b[i:]...)...)
	}
	return b
}

func (g *gen) ws() string {
	if g.r.IntN(3) != 0 {
		return ""
	}
	var sb strings.Builder
	for n := 1 + g.r.IntN(3); n > 0; n-- {
		sb.WriteString(g.pick(" ", "\t", "\n", "\r"))
	}
	return sb.String()
}

func (g *gen) write(sb *strings.Builder, v Value) {
	switch x := v.(type) {
	case Null:
		sb.WriteString("null")
	case Bool:
		if x {
			sb.WriteString("true")
		} else {
			sb.WriteString("false")
		}
	case Int:
		sb.WriteString(x.String())
	case Float:
		sb.WriteString(g.floatText(float64(x)))
	case String:
		g.quote(sb, string(x))
	case List:
		sb.WriteString("[" + g.ws())
		for i, item := range x {
			if i > 0 {
				sb.WriteString("," + g.ws())
			}
			g.write(sb, item)
			sb.WriteString(g.ws())
		}
		sb.WriteString("]")
	case *OrderedMap:
		sb.WriteString("{" + g.ws())
		keys := x.Keys()
		if len(keys) > 0 && g.r.IntN(8) == 0 {
			keys = append(keys, keys[g.r.IntN(len(keys))]) // duplicate key
		}
		for i, k := range keys {
			if i > 0 {
				sb.WriteString("," + g.ws())
			}
			g.quote(sb, k)
			sb.WriteString(g.ws() + ":" + g.ws())
			if i >= x.Len() {
				g.write(sb, g.value(1, false))
			} else {
				item, _ := x.Get(k)
				g.write(sb, item)
			}
			sb.WriteString(g.ws())
		}
		sb.WriteString("}")
	}
}

func (g *gen) floatText(f float64) string {
	switch {
	case math.IsNaN(f):
		return "NaN"
	case math.IsInf(f, 1):
		return "Infinity"
	case math.IsInf(f, -1):
		return "-Infinity"
	}
	switch g.r.IntN(7) {
	case 0:
		return FloatRepr(f)
	case 1:
		return strings.ToUpper(strconv.FormatFloat(f, 'e', -1, 64))
	case 2:
		return strconv.FormatFloat(f, 'f', -1, 64)
	case 3:
		return strconv.FormatFloat(f, 'e', 20+g.r.IntN(780), 64)
	case 4:
		return strconv.FormatFloat(f, 'g', 17, 64)
	case 5:
		return g.pick("", "-") + "0.000" + g.digitString(1+g.r.IntN(30)) + g.pick("e", "E") + g.pick("", "+", "-") + strconv.Itoa(g.r.IntN(420))
	}
	return g.pick("", "-") + g.digitString(1+g.r.IntN(25)) + "." + g.digitString(1+g.r.IntN(25)) + "e-" + strconv.Itoa(300+g.r.IntN(60))
}

func (g *gen) hex4(r rune) string {
	h := fmt.Sprintf("%04x", r)
	if g.r.IntN(2) == 0 {
		h = strings.ToUpper(h)
	}
	return `\u` + h
}

func (g *gen) quote(sb *strings.Builder, s string) {
	sb.WriteByte('"')
	for _, r := range codePoints(s) {
		switch {
		case r == '"' || r == '\\':
			if g.r.IntN(4) == 0 {
				sb.WriteString(g.hex4(r))
			} else {
				sb.WriteString(`\` + string(r))
			}
		case r < 0x20:
			if g.r.IntN(30) == 0 {
				sb.WriteRune(r) // raw control character: invalid
				continue
			}
			short := map[rune]string{'\b': `\b`, '\f': `\f`, '\n': `\n`, '\r': `\r`, '\t': `\t`}[r]
			if short != "" && g.r.IntN(2) == 0 {
				sb.WriteString(short)
			} else {
				sb.WriteString(g.hex4(r))
			}
		case isSurrogate(r):
			if g.r.IntN(4) == 0 {
				sb.Write(appendCodePoint(nil, r)) // raw surrogate bytes
			} else {
				sb.WriteString(g.hex4(r))
			}
		case r >= 0x10000 && g.r.IntN(3) == 0:
			r -= 0x10000
			sb.WriteString(g.hex4(0xD800|r>>10) + g.hex4(0xDC00|r&0x3FF))
		case r == '/' && g.r.IntN(2) == 0:
			sb.WriteString(`\/`)
		case g.r.IntN(10) == 0:
			if r < 0x10000 {
				sb.WriteString(g.hex4(r))
			} else {
				sb.WriteRune(r)
			}
		default:
			sb.Write(appendCodePoint(nil, r))
		}
	}
	sb.WriteByte('"')
}

func (g *gen) mutate(text []rune) []rune {
	tokens := []string{`"`, `\`, ",", ":", "[", "]", "{", "}", " ", "0", "1", "-", "+", "e", "E", ".", "n", "t", "f",
		"N", "I", `\u`, `\u12`, `\x`, "\x00", "\x1f", "'", "/", "\ufeff", "\xed\xa0\x80", "\u00a0", "Infinity", "\n"}
	for n := 1 + g.r.IntN(3); n > 0; n-- {
		i := 0
		if len(text) > 0 {
			i = g.r.IntN(len(text) + 1)
		}
		tok := codePoints(tokens[g.r.IntN(len(tokens))])
		switch g.r.IntN(5) {
		case 0:
			if i < len(text) {
				text = append(text[:i:i], text[i+1:]...)
			}
		case 1:
			text = append(text[:i:i], append(tok, text[i:]...)...)
		case 2:
			if i < len(text) {
				text = append(text[:i:i], append(tok, text[i+1:]...)...)
			}
		case 3:
			text = text[:i]
		default:
			if i+1 < len(text) {
				text[i], text[i+1] = text[i+1], text[i]
			}
		}
	}
	return text
}

func (g *gen) encodeText(text []rune) []byte {
	var out []byte
	switch g.r.IntN(20) {
	case 0:
		out = []byte{0xEF, 0xBB, 0xBF}
	case 1, 2:
		be := g.r.IntN(2) == 0
		if g.r.IntN(2) == 0 {
			out = unit16(out, 0xFEFF, be)
		}
		for _, r := range text {
			if r >= 0x10000 {
				r -= 0x10000
				out = unit16(out, 0xD800|r>>10, be)
				out = unit16(out, 0xDC00|r&0x3FF, be)
			} else {
				out = unit16(out, r, be)
			}
		}
		return out
	case 3:
		be := g.r.IntN(2) == 0
		if g.r.IntN(2) == 0 {
			text = append([]rune{0xFEFF}, text...)
		}
		for _, r := range text {
			if be {
				out = append(out, byte(r>>24), byte(r>>16), byte(r>>8), byte(r))
			} else {
				out = append(out, byte(r), byte(r>>8), byte(r>>16), byte(r>>24))
			}
		}
		return out
	}
	for _, r := range text {
		out = appendCodePoint(out, r)
	}
	return out
}

func unit16(b []byte, r rune, be bool) []byte {
	if be {
		return append(b, byte(r>>8), byte(r))
	}
	return append(b, byte(r), byte(r>>8))
}

func (g *gen) rawBytes() []byte {
	alphabet := []byte{0x00, 0xFF, 0xFE, 0xEF, 0xBB, 0xBF, 0xED, 0xA0, 0xBF, 0x80, 0xD8, 0xDC, 0xC0, 0xF4, 0x90,
		'"', '[', ']', '{', '}', '1', ' ', '\\', 'u', 'N', ':', ','}
	b := make([]byte, g.r.IntN(13))
	for i := range b {
		b[i] = alphabet[g.r.IntN(len(alphabet))]
	}
	return b
}

func codePoints(s string) []rune {
	var out []rune
	for i := 0; i < len(s); {
		r, size := decodeCodePoint(s[i:])
		if size == 0 {
			panic("generator produced invalid text")
		}
		out = append(out, r)
		i += size
	}
	return out
}

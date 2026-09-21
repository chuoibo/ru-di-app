package pyjson

import (
	"errors"
	"math"
	"math/big"
	"strings"
	"testing"
	"time"
)

// Golden tables live in golden_cases_test.go, rendered from the parity image
// by scripts/render_pyjson_goldens.py. These helpers build their inputs.

type outcome struct {
	ok      string
	errKind string
	errMsg  string
}

type encodeGolden struct {
	name                      string
	v                         Value
	compact, dumps, canonical outcome
}

type decodeGolden struct {
	name      string
	in        string
	dumps     string
	canonical outcome
	errKind   string
	errMsg    string
	errPos    int
}

type dateTimeGolden struct {
	name  string
	t     time.Time
	naive bool
	want  string
}

type dateGolden struct {
	t    time.Time
	want string
}

type uuidGolden struct {
	u    [16]byte
	want string
}

func fl(bits uint64) Float { return Float(math.Float64frombits(bits)) }

func mustInt(text string) Int {
	n, ok := ParseInt(text)
	if !ok {
		panic("bad int literal " + text)
	}
	return n
}

// nines returns 10**n - 1 without a long literal in the source.
func nines(n int) Int {
	v := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(n)), nil)
	return NewBigInt(v.Sub(v, big.NewInt(1)))
}

func obj(kv ...any) *OrderedMap {
	m := NewOrderedMap()
	for i := 0; i < len(kv); i += 2 {
		m.Set(kv[i].(string), kv[i+1].(Value))
	}
	return m
}

// errorKind names the Python exception an error stands for.
func errorKind(err error) string {
	var (
		de *DecodeError
		ve *ValueError
		ud *UnicodeDecodeError
		ue *UnicodeEncodeError
		re *RecursionError
		is *InvalidStringError
	)
	switch {
	case errors.As(err, &de):
		return "JSONDecodeError"
	case errors.As(err, &ue):
		return "UnicodeEncodeError"
	case errors.As(err, &ud):
		return "UnicodeDecodeError"
	case errors.As(err, &ve):
		return "ValueError"
	case errors.As(err, &re):
		return "RecursionError"
	case errors.As(err, &is):
		return "InvalidStringError"
	}
	return "unexpected: " + err.Error()
}

func checkOutcome(t *testing.T, what string, got []byte, err error, want outcome) {
	t.Helper()
	if want.errKind != "" {
		switch {
		case err == nil:
			t.Errorf("%s: got %q, want %s", what, got, want.errKind)
		case errorKind(err) != want.errKind:
			t.Errorf("%s: got error %s (%v), want %s", what, errorKind(err), err, want.errKind)
		case want.errMsg != "" && err.Error() != want.errMsg:
			t.Errorf("%s: message\n got %q\nwant %q", what, err.Error(), want.errMsg)
		}
		return
	}
	if err != nil {
		t.Errorf("%s: unexpected error %v", what, err)
		return
	}
	if string(got) != want.ok {
		t.Errorf("%s:\n got %q\nwant %q", what, got, want.ok)
	}
}

func TestEncodeGoldens(t *testing.T) {
	for _, c := range encodeGoldens {
		got, err := Compact(c.v)
		checkOutcome(t, c.name+" compact", got, err, c.compact)
		got, err = Dumps(c.v)
		checkOutcome(t, c.name+" dumps", got, err, c.dumps)
		got, err = Canonical(c.v)
		checkOutcome(t, c.name+" canonical", got, err, c.canonical)
	}
}

func TestDecodeGoldens(t *testing.T) {
	for _, c := range decodeGoldens {
		v, err := Loads([]byte(c.in))
		if c.errKind != "" {
			if err == nil {
				t.Errorf("%s: loaded %#v, want %s", c.name, v, c.errKind)
				continue
			}
			if k := errorKind(err); k != c.errKind {
				t.Errorf("%s: got %s (%v), want %s", c.name, k, err, c.errKind)
				continue
			}
			var de *DecodeError
			if errors.As(err, &de) && (de.Msg != c.errMsg || de.Pos != c.errPos) {
				t.Errorf("%s: got %q at %d, want %q at %d", c.name, de.Msg, de.Pos, c.errMsg, c.errPos)
			}
			if c.errKind == "ValueError" && err.Error() != c.errMsg {
				t.Errorf("%s: message\n got %q\nwant %q", c.name, err.Error(), c.errMsg)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: unexpected error %v", c.name, err)
			continue
		}
		got, err := Dumps(v)
		checkOutcome(t, c.name+" dumps", got, err, outcome{ok: c.dumps})
		got, err = Canonical(v)
		checkOutcome(t, c.name+" canonical", got, err, c.canonical)
	}
}

func TestDateTimeGoldens(t *testing.T) {
	for _, c := range dateTimeGoldens {
		got := DateTime(c.t)
		if c.naive {
			got = NaiveDateTime(c.t)
		}
		if got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
	for _, c := range dateGoldens {
		if got := Date(c.t); got != c.want {
			t.Errorf("date: got %q, want %q", got, c.want)
		}
	}
	for _, c := range uuidGoldens {
		if got := UUID(c.u); got != c.want {
			t.Errorf("uuid: got %q, want %q", got, c.want)
		}
	}
}

func TestDateTimeDropsSubMicrosecond(t *testing.T) {
	// Python datetime has no nanoseconds; 999ns is below the first microsecond.
	tm := time.Date(2024, 1, 2, 3, 4, 5, 999, time.UTC)
	if got := DateTime(tm); got != "2024-01-02T03:04:05Z" {
		t.Fatalf("got %q", got)
	}
	tm = time.Date(2024, 1, 2, 3, 4, 5, 1999, time.FixedZone("ICT", 7*3600))
	if got := DateTime(tm); got != "2024-01-02T03:04:05.000001+07:00" {
		t.Fatalf("got %q", got)
	}
	// A zero offset under any zone name is "Z", as pydantic does for
	// ZoneInfo("Europe/London") in winter.
	if got := DateTime(time.Date(2024, 1, 2, 3, 4, 5, 0, time.FixedZone("GMT", 0))); got != "2024-01-02T03:04:05Z" {
		t.Fatalf("got %q", got)
	}
}

func TestOrderedMapKeepsFirstPosition(t *testing.T) {
	m := NewOrderedMap()
	m.Set("a", NewInt(1))
	m.Set("b", NewInt(2))
	m.Set("a", NewInt(3))
	if got := strings.Join(m.Keys(), ","); got != "a,b" || m.Len() != 2 {
		t.Fatalf("keys %q len %d", got, m.Len())
	}
	if v, ok := m.Get("a"); !ok || v.(Int).String() != "3" {
		t.Fatalf("a = %v %v", v, ok)
	}
	if _, ok := m.Get("missing"); ok {
		t.Fatal("missing key found")
	}
	keys := m.Keys()
	keys[0] = "mutated"
	if m.Keys()[0] != "a" {
		t.Fatal("Keys must return a copy")
	}
	var seen []string
	for k := range m.All() {
		seen = append(seen, k)
		break
	}
	if len(seen) != 1 {
		t.Fatalf("All ignored early stop: %v", seen)
	}
	var zero OrderedMap
	zero.Set("x", Null{})
	if zero.Len() != 1 {
		t.Fatal("zero OrderedMap must be usable")
	}
	var nilMap *OrderedMap
	if nilMap.Len() != 0 || nilMap.Keys() != nil {
		t.Fatal("nil map must read as empty")
	}
	if got, err := Compact(nilMap); err != nil || string(got) != "{}" {
		t.Fatalf("nil map encodes as %q %v", got, err)
	}
	if got, err := Compact(nil); err != nil || string(got) != "null" {
		t.Fatalf("nil Value encodes as %q %v", got, err)
	}
}

func TestDuplicateKeyLastValueFirstPosition(t *testing.T) {
	v, err := Loads([]byte(`{"a":1,"b":2,"a":3}`))
	if err != nil {
		t.Fatal(err)
	}
	got, _ := Dumps(v)
	if string(got) != `{"a": 3, "b": 2}` {
		t.Fatalf("got %s", got)
	}
}

func TestLoneSurrogateRepresentation(t *testing.T) {
	v, err := Loads([]byte(`"\ud800"`))
	if err != nil {
		t.Fatal(err)
	}
	if v != String("\xed\xa0\x80") {
		t.Fatalf("lone surrogate stored as %q", v)
	}
	ascii, err := Dumps(v)
	if err != nil || string(ascii) != `"\ud800"` {
		t.Fatalf("Dumps = %q %v", ascii, err)
	}
	back, err := Loads(ascii)
	if err != nil || back != v {
		t.Fatalf("Dumps output does not round-trip: %q %v", back, err)
	}
	if _, err := Canonical(v); errorKind(err) != "UnicodeEncodeError" || !IsValueError(err) {
		t.Fatalf("Canonical must refuse surrogates like str.encode: %v", err)
	}
	// An escaped pair joins; the same pair as raw surrogate bytes does not.
	joined, _ := Loads([]byte(`"\ud83d\ude00"`))
	split, _ := Loads([]byte("\"\xed\xa0\xbd\xed\xb8\x80\""))
	if joined != String("\U0001F600") || split == joined {
		t.Fatalf("joined %q split %q", joined, split)
	}
}

func TestDepthLimit(t *testing.T) {
	deep := func(open, close string, n int) []byte {
		return []byte(strings.Repeat(open, n) + "1" + strings.Repeat(close, n))
	}
	if _, err := Loads(deep("[", "]", MaxDepth)); err != nil {
		t.Fatalf("depth %d: %v", MaxDepth, err)
	}
	for _, c := range []struct{ open, close, kind string }{{"[", "]", "array"}, {`{"k":`, "}", "object"}} {
		_, err := Loads(deep(c.open, c.close, MaxDepth+1))
		var re *RecursionError
		if !errors.As(err, &re) || !strings.Contains(re.Msg, "decoding a JSON "+c.kind) {
			t.Fatalf("%s depth %d: %v", c.kind, MaxDepth+1, err)
		}
		if IsValueError(err) {
			t.Fatal("RecursionError is not a ValueError")
		}
	}
	// Recursion is checked before the missing close is noticed.
	if _, err := Loads([]byte(strings.Repeat("[", MaxDepth+1))); errorKind(err) != "RecursionError" {
		t.Fatalf("unterminated deep array: %v", err)
	}
	var v Value = NewInt(1)
	for i := 0; i < MaxDepth; i++ {
		v = List{v}
	}
	if _, err := Canonical(v); err != nil {
		t.Fatalf("encode depth %d: %v", MaxDepth, err)
	}
	if _, err := Canonical(List{v}); errorKind(err) != "RecursionError" {
		t.Fatalf("encode depth %d: %v", MaxDepth+1, err)
	}
}

func TestInvalidGoString(t *testing.T) {
	for _, enc := range []func(Value) ([]byte, error){Compact, Dumps, Canonical} {
		if _, err := enc(String("a\xffb")); errorKind(err) != "InvalidStringError" || IsValueError(err) {
			t.Fatalf("invalid UTF-8: %v", err)
		}
	}
}

func TestFloatReprSpecials(t *testing.T) {
	for f, want := range map[float64]string{math.Inf(1): "inf", math.Inf(-1): "-inf", 0: "0.0"} {
		if got := FloatRepr(f); got != want {
			t.Errorf("FloatRepr(%v) = %q, want %q", f, got, want)
		}
	}
	if got := FloatRepr(math.NaN()); got != "nan" {
		t.Errorf("FloatRepr(NaN) = %q", got)
	}
	if got := FloatRepr(math.Copysign(0, -1)); got != "-0.0" {
		t.Errorf("FloatRepr(-0) = %q", got)
	}
	if got := FloatRepr(math.Pow10(16)); got != "1e+16" {
		t.Errorf("FloatRepr(1e16) = %q", got)
	}
}

func TestIntAccessors(t *testing.T) {
	n := nines(30)
	if _, ok := n.Int64(); ok {
		t.Fatal("30 digits must not fit int64")
	}
	if n.Big().String() != n.String() || len(n.String()) != 30 {
		t.Fatalf("big accessors: %s", n.String())
	}
	if v, ok := NewBigInt(big.NewInt(-5)).Int64(); !ok || v != -5 {
		t.Fatal("small big.Int must use the fast path")
	}
	for _, bad := range []string{"", "+1", "-", "1a", "--1", " 1"} {
		if _, ok := ParseInt(bad); ok {
			t.Errorf("ParseInt(%q) accepted", bad)
		}
	}
}

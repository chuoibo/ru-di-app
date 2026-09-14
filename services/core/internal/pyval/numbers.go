package pyval

import (
	"math"
	"math/big"
	"strconv"
	"strings"

	"mobile/services/core/internal/pyjson"
)

// Numeric coercions of pydantic-core 2.46.5 (src/input/shared.rs) as they
// apply to Python-mode validation of what json.loads and Starlette produce.

// strAsInt is str_as_int: trim (Rust White_Space), refuse more than 4300
// bytes, then parse as a base-10 integer; on failure retry without a
// non-empty all-zero fraction, then without digit separators. Measured:
// "1.00" and "1_0.0" parse, "1.", ".0", "1.0_0" and "1__0" do not.
func strAsInt(s string) (pyjson.Int, string) {
	s = rustTrim(s)
	if len(s) > pyjson.IntMaxStrDigits {
		return pyjson.Int{}, "int_parsing_size"
	}
	if v, ok := parseRustInt(s); ok {
		return v, ""
	}
	base := s
	if dot := strings.IndexByte(s, '.'); dot >= 0 && dot+1 < len(s) && strings.Trim(s[dot+1:], "0") == "" {
		if v, ok := parseRustInt(s[:dot]); ok {
			return v, ""
		}
		base = s[:dot]
	}
	if stripped, ok := stripUnderscores(base); ok {
		if v, ok := parseRustInt(stripped); ok {
			return v, ""
		}
	}
	return pyjson.Int{}, "int_parsing"
}

// parseRustInt is i64::from_str / BigInt::from_str: an optional sign, then
// ASCII digits only.
func parseRustInt(s string) (pyjson.Int, bool) {
	digits := s
	if digits != "" && (digits[0] == '+' || digits[0] == '-') {
		digits = digits[1:]
	}
	if digits == "" {
		return pyjson.Int{}, false
	}
	for i := 0; i < len(digits); i++ {
		if digits[i] < '0' || digits[i] > '9' {
			return pyjson.Int{}, false
		}
	}
	if s[0] == '+' {
		s = s[1:]
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return pyjson.NewInt(n), true
	}
	b, ok := new(big.Int).SetString(s, 10)
	if !ok {
		return pyjson.Int{}, false
	}
	return pyjson.NewBigInt(b), true
}

// stripUnderscores removes digit separators when every "_" sits between two
// ASCII digits, the rule str_as_int follows (measured).
func stripUnderscores(s string) (string, bool) {
	if strings.IndexByte(s, '_') < 0 {
		return "", false
	}
	for i := 0; i < len(s); i++ {
		if s[i] != '_' {
			continue
		}
		if i == 0 || i+1 == len(s) || !isDigit(s[i-1]) || !isDigit(s[i+1]) {
			return "", false
		}
	}
	return strings.ReplaceAll(s, "_", ""), true
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// strAsFloat is str_as_float: Rust f64::from_str on the trimmed text, then
// on the untrimmed text without digit separators. The separator rule is
// looser than int's (measured): only a leading or trailing "_" or a "__"
// refuses, so "-_1", "1_.25" and "1._5" parse and "1__0" does not.
func strAsFloat(s string) (float64, bool) {
	if f, ok := parseRustFloat(rustTrim(s)); ok {
		return f, true
	}
	if strings.IndexByte(s, '_') >= 0 && !strings.HasPrefix(s, "_") && !strings.HasSuffix(s, "_") && !strings.Contains(s, "__") {
		return parseRustFloat(strings.ReplaceAll(s, "_", ""))
	}
	return 0, false
}

// parseRustFloat is f64::from_str: decimal digits with an optional sign,
// fraction and exponent, or inf / infinity / nan in any case. No hex, no
// separators, no surrounding space.
func parseRustFloat(s string) (float64, bool) {
	body := s
	if body != "" && (body[0] == '+' || body[0] == '-') {
		body = body[1:]
	}
	switch strings.ToLower(body) {
	case "inf", "infinity":
		if s[0] == '-' {
			return math.Inf(-1), true
		}
		return math.Inf(1), true
	case "nan":
		return math.NaN(), true
	}
	digits, seenDot, seenExp, expDigits := 0, false, false, 0
	for i := 0; i < len(body); i++ {
		c := body[i]
		switch {
		case c >= '0' && c <= '9':
			if seenExp {
				expDigits++
			} else {
				digits++
			}
		case c == '.' && !seenDot && !seenExp:
			seenDot = true
		case (c == 'e' || c == 'E') && !seenExp && digits > 0:
			seenExp = true
			if i+1 < len(body) && (body[i+1] == '+' || body[i+1] == '-') {
				i++
			}
		default:
			return 0, false
		}
	}
	if digits == 0 || (seenExp && expDigits == 0) {
		return 0, false
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		if ne, ok := err.(*strconv.NumError); ok && ne.Err == strconv.ErrRange {
			return f, true
		}
		return 0, false
	}
	return f, true
}

// floatAsExactInt returns the integer a finite integral float equals.
func floatAsExactInt(f float64) (pyjson.Int, bool) {
	if math.IsNaN(f) || math.IsInf(f, 0) || f != math.Trunc(f) {
		return pyjson.Int{}, false
	}
	b, _ := new(big.Float).SetFloat64(f).Int(nil)
	return pyjson.NewBigInt(b), true
}

// intToFloat is float(int): nearest double, ties to even.
func intToFloat(i pyjson.Int) (float64, bool) {
	if n, ok := i.Int64(); ok && n > -(1<<53) && n < 1<<53 {
		return float64(n), true
	}
	f, _ := new(big.Float).SetInt(i.Big()).Float64()
	if math.IsInf(f, 0) {
		return 0, false
	}
	return f, true
}

// bound is a numeric constraint as written in the IR: an int or a float.
type bound struct {
	isFloat bool
	i       *big.Int
	f       float64
}

func boundFrom(v pyjson.Value) (bound, bool) {
	switch x := v.(type) {
	case pyjson.Int:
		f, _ := new(big.Float).SetInt(x.Big()).Float64()
		return bound{i: x.Big(), f: f}, true
	case pyjson.Float:
		return bound{isFloat: true, f: float64(x)}, true
	}
	return bound{}, false
}

// rustDisplayFloat is Rust's `{}` for f64: shortest round-trip digits,
// never an exponent.
func rustDisplayFloat(f float64) string {
	switch {
	case math.IsNaN(f):
		return "NaN"
	case math.IsInf(f, 1):
		return "inf"
	case math.IsInf(f, -1):
		return "-inf"
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// cmpIntBound compares an int with a bound exactly.
func cmpIntBound(v pyjson.Int, b bound) int {
	if !b.isFloat {
		return v.Big().Cmp(b.i)
	}
	return new(big.Float).SetInt(v.Big()).Cmp(big.NewFloat(b.f))
}

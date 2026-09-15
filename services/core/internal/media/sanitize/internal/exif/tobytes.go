package exif

import (
	"encoding/binary"
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"mobile/services/core/internal/media/sanitize/internal/pil"
)

// This file answers one question about ImageOps.exif_transpose: after it
// transposes an image and deletes the orientation tag, does
// Image.Exif.tobytes() raise? tobytes rebuilds every tag in a fresh
// ImageFileDirectory_v2, whose types come from TiffTags rather than from the
// file, so a tag stored with a type or value its TiffTags entry cannot hold
// makes struct.pack (or a writer) raise, and the sanitizer answers
// not_an_image. Only the raise matters here; no bytes are produced.

const (
	tagExifIFD     = 0x8769
	tagGPSIFD      = 0x8825
	tagInteropIFD  = 0xA005
	tagStripOffset = 273
)

type pyKind int

const (
	pyInt pyKind = iota + 1
	pyFloat
	pyRational
	pyBytes
	pyStr
	pyTuple
	pyDict
)

// pyVal is a Python value a tag can hold on its way through tobytes.
type pyVal struct {
	kind  pyKind
	i     *big.Int
	f     float64
	num   int64 // IFDRational numerator and denominator, as given
	den   int64
	b     []byte
	s     string
	items []pyVal
	dict  *dictVal
}

type dictVal struct {
	keys []uint16
	vals map[uint16]pyVal
}

func (d *dictVal) set(k uint16, v pyVal) {
	if _, ok := d.vals[k]; !ok {
		d.keys = append(d.keys, k)
	}
	d.vals[k] = v
}

func newDict() *dictVal { return &dictVal{vals: map[uint16]pyVal{}} }

func intVal(v int64) pyVal  { return pyVal{kind: pyInt, i: big.NewInt(v)} }
func bigVal(v uint64) pyVal { return pyVal{kind: pyInt, i: new(big.Int).SetUint64(v)} }

// raised marks an exception of tobytes.
type raised struct{ msg string }

func (r *raised) Error() string { return "exif tobytes: " + r.msg }

func raise(msg string) error { return &raised{msg: msg} }

func lookupTag(tag uint16, group int) tagInfo {
	if group >= 0 {
		if table, ok := tagsV2Groups[uint16(group)]; ok {
			if info, ok := table[tag]; ok {
				return info
			}
		}
		return tagInfo{length: -1}
	}
	if info, ok := tagsV2[tag]; ok {
		return info
	}
	return tagInfo{length: -1}
}

// handler is ImageFileDirectory_v2._load_dispatch[typ] with legacy_api False.
func handler(typ uint16, data []byte, order binary.ByteOrder) pyVal {
	var items []pyVal
	switch typ {
	case 1, 7:
		return pyVal{kind: pyBytes, b: data}
	case 2:
		if len(data) > 0 && data[len(data)-1] == 0 {
			data = data[:len(data)-1]
		}
		runes := make([]rune, len(data))
		for i, c := range data {
			runes[i] = rune(c)
		}
		return pyVal{kind: pyStr, s: string(runes)}
	case 3:
		for i := 0; i+2 <= len(data); i += 2 {
			items = append(items, intVal(int64(order.Uint16(data[i:]))))
		}
	case 4, 13:
		for i := 0; i+4 <= len(data); i += 4 {
			items = append(items, intVal(int64(order.Uint32(data[i:]))))
		}
	case 6:
		for _, c := range data {
			items = append(items, intVal(int64(int8(c))))
		}
	case 8:
		for i := 0; i+2 <= len(data); i += 2 {
			items = append(items, intVal(int64(int16(order.Uint16(data[i:])))))
		}
	case 9:
		for i := 0; i+4 <= len(data); i += 4 {
			items = append(items, intVal(int64(int32(order.Uint32(data[i:])))))
		}
	case 16:
		for i := 0; i+8 <= len(data); i += 8 {
			items = append(items, bigVal(order.Uint64(data[i:])))
		}
	case 11:
		for i := 0; i+4 <= len(data); i += 4 {
			items = append(items, pyVal{kind: pyFloat, f: float64(math.Float32frombits(order.Uint32(data[i:])))})
		}
	case 12:
		for i := 0; i+8 <= len(data); i += 8 {
			items = append(items, pyVal{kind: pyFloat, f: math.Float64frombits(order.Uint64(data[i:]))})
		}
	case 5, 10:
		n := len(data) / 4
		for i := 0; i+1 < n; i += 2 {
			a, b := order.Uint32(data[4*i:]), order.Uint32(data[4*i+4:])
			if typ == 5 {
				items = append(items, pyVal{kind: pyRational, num: int64(a), den: int64(b)})
			} else {
				items = append(items, pyVal{kind: pyRational, num: int64(int32(a)), den: int64(int32(b))})
			}
		}
	}
	return pyVal{kind: pyTuple, items: items}
}

func values(v pyVal) []pyVal {
	switch v.kind {
	case pyTuple:
		return v.items
	case pyDict:
		out := make([]pyVal, len(v.dict.keys))
		for i, k := range v.dict.keys {
			out[i] = intVal(int64(k))
		}
		return out
	}
	return []pyVal{v}
}

func cvtEnum(info tagInfo, vals []pyVal) []pyVal {
	out := make([]pyVal, len(vals))
	for i, v := range vals {
		if v.kind == pyStr && info.enum != nil {
			if mapped, ok := info.enum[v.s]; ok {
				v = intVal(mapped)
			}
		}
		out[i] = v
	}
	return out
}

// sourceValue is IFD.__getitem__ on a loaded directory (tagtype from the
// file) followed by Exif._fixup.
func sourceValue(tag uint16, group int, e ifdEntry, order binary.ByteOrder) pyVal {
	info := lookupTag(tag, group)
	vals := cvtEnum(info, values(handler(e.typ, e.data, order)))
	var v pyVal
	if info.length == 1 || e.typ == 1 || info.length == -1 && len(vals) == 1 {
		v = vals[0]
	} else {
		v = pyVal{kind: pyTuple, items: vals}
	}
	if v.kind == pyTuple && len(v.items) == 1 {
		v = v.items[0]
	}
	return v
}

func ratLess0(v pyVal) bool {
	if v.den == 0 {
		return false
	}
	return (v.num < 0) != (v.den < 0) && v.num != 0
}

// setValue is ImageFileDirectory_v2._setitem on a fresh directory.
func setValue(tag uint16, group int, value pyVal) (uint16, pyVal, error) {
	info := lookupTag(tag, group)
	vals := append([]pyVal(nil), values(value)...)
	typ := info.typ
	if typ == 0 {
		typ = 7
		all := func(kind pyKind) bool {
			for _, v := range vals {
				if v.kind != kind {
					return false
				}
			}
			return true
		}
		switch {
		case all(pyRational):
			typ = 5
			for _, v := range vals {
				if ratLess0(v) {
					typ = 10
					break
				}
			}
		case all(pyInt):
			short, signedShort, long := true, true, true
			for _, v := range vals {
				if short && !(v.i.Sign() >= 0 && v.i.Cmp(big.NewInt(1<<16)) < 0) {
					short = false
				}
				if signedShort && !(v.i.Cmp(big.NewInt(-(1<<15))) > 0 && v.i.Cmp(big.NewInt(1<<15)) < 0) {
					signedShort = false
				}
				if long && v.i.Sign() < 0 {
					long = false
				}
			}
			switch {
			case short:
				typ = 3
			case signedShort:
				typ = 8
			case long:
				typ = 4
			default:
				typ = 9
			}
		case all(pyFloat):
			typ = 12
		case all(pyStr):
			typ = 2
		case all(pyBytes):
			typ = 1
		}
	}
	converted := false
	switch typ {
	case 7:
		for i, v := range vals {
			if v.kind == pyStr {
				var encoded []byte
				for _, c := range v.s {
					if c < 128 {
						encoded = append(encoded, byte(c))
					} else {
						encoded = append(encoded, '?')
					}
				}
				vals[i] = pyVal{kind: pyBytes, b: encoded}
			}
		}
		converted = true
	case 5:
		for i, v := range vals {
			if v.kind == pyInt {
				f, _ := new(big.Float).SetInt(v.i).Float64()
				vals[i] = pyVal{kind: pyFloat, f: f}
			}
		}
		converted = true
	}
	isIFD := typ == 4 && value.kind == pyDict && !converted
	if isIFD {
		return typ, value, nil
	}
	vals = cvtEnum(info, vals)
	if info.length == 1 || typ == 1 || info.length == -1 && len(vals) == 1 {
		if len(vals) == 0 {
			return 0, pyVal{}, raise("IndexError: tuple index out of range")
		}
		return typ, vals[0], nil
	}
	return typ, pyVal{kind: pyTuple, items: vals}, nil
}

var packFormats = map[uint16]struct {
	size     int
	lo, hi   *big.Int
	floating bool
}{
	3:  {2, big.NewInt(0), big.NewInt(math.MaxUint16), false},
	4:  {4, big.NewInt(0), big.NewInt(math.MaxUint32), false},
	13: {4, big.NewInt(0), big.NewInt(math.MaxUint32), false},
	6:  {1, big.NewInt(math.MinInt8), big.NewInt(math.MaxInt8), false},
	8:  {2, big.NewInt(math.MinInt16), big.NewInt(math.MaxInt16), false},
	9:  {4, big.NewInt(math.MinInt32), big.NewInt(math.MaxInt32), false},
	16: {8, big.NewInt(0), new(big.Int).SetUint64(math.MaxUint64), false},
	11: {4, nil, nil, true},
	12: {8, nil, nil, true},
}

// pyIntOf is int(v) as write_byte/write_undefined call it on IFDRational.
func pyIntOf(v pyVal) (pyVal, error) {
	if v.den == 0 {
		return pyVal{}, raise("ValueError: cannot convert float NaN to integer")
	}
	q := new(big.Int).Quo(big.NewInt(v.num), big.NewInt(v.den))
	return pyVal{kind: pyInt, i: q}, nil
}

// toFloat is PyFloat_AsDouble on a value for struct "f"/"d".
func toFloat(v pyVal) (float64, error) {
	switch v.kind {
	case pyFloat:
		return v.f, nil
	case pyInt:
		f, acc := new(big.Float).SetInt(v.i).Float64()
		if math.IsInf(f, 0) && acc != big.Exact {
			return 0, raise("OverflowError: int too large to convert to float")
		}
		return f, nil
	case pyRational:
		if v.den == 0 {
			return 0, raise("ZeroDivisionError: division by zero")
		}
		return float64(v.num) / float64(v.den), nil
	}
	return 0, raise("struct.error: required argument is not a float")
}

// limitDenominator is fractions.Fraction.limit_denominator.
func limitDenominator(x *big.Rat, maxDen *big.Int) *big.Rat {
	if x.Denom().Cmp(maxDen) <= 0 {
		return new(big.Rat).Set(x)
	}
	p0, q0, p1, q1 := big.NewInt(0), big.NewInt(1), big.NewInt(1), big.NewInt(0)
	n, d := new(big.Int).Set(x.Num()), new(big.Int).Set(x.Denom())
	for {
		a := new(big.Int)
		m := new(big.Int)
		a.DivMod(n, d, m)
		q2 := new(big.Int).Add(q0, new(big.Int).Mul(a, q1))
		if q2.Cmp(maxDen) > 0 {
			break
		}
		p0, q0, p1, q1 = p1, q1, new(big.Int).Add(p0, new(big.Int).Mul(a, p1)), q2
		n, d = d, m
		if d.Sign() == 0 {
			break
		}
	}
	k := new(big.Int).Div(new(big.Int).Sub(maxDen, q0), q1)
	bound1 := new(big.Rat).SetFrac(new(big.Int).Add(p0, new(big.Int).Mul(k, p1)), new(big.Int).Add(q0, new(big.Int).Mul(k, q1)))
	bound2 := new(big.Rat).SetFrac(p1, q1)
	d1 := new(big.Rat).Abs(new(big.Rat).Sub(bound2, x))
	d2 := new(big.Rat).Abs(new(big.Rat).Sub(bound1, x))
	if d1.Cmp(d2) <= 0 {
		return bound2
	}
	return bound1
}

func inRange(v *big.Int, lo, hi int64) bool {
	return v.Cmp(big.NewInt(lo)) >= 0 && v.Cmp(big.NewInt(hi)) <= 0
}

// limitRational is TiffImagePlugin._limit_rational: (numerator,
// denominator) or the exception.
func limitRational(v pyVal, maxVal int64) (*big.Int, *big.Int, error) {
	var x *big.Rat
	switch v.kind {
	case pyRational:
		if v.den == 0 {
			return big.NewInt(v.num), big.NewInt(0), nil
		}
		x = big.NewRat(v.num, v.den)
	case pyFloat:
		if math.IsNaN(v.f) {
			return nil, nil, raise("ValueError: cannot convert NaN to integer ratio")
		}
		if math.IsInf(v.f, 0) {
			return big.NewInt(1), big.NewInt(0), nil
		}
		x = new(big.Rat)
		x.SetFloat64(v.f)
	case pyInt:
		x = new(big.Rat).SetInt(v.i)
	default:
		return nil, nil, raise("TypeError: bad operand type for abs()")
	}
	inv := new(big.Rat).Abs(x).Cmp(big.NewRat(1, 1)) > 0
	if inv {
		if v.kind == pyFloat {
			f := 1 / v.f
			x = new(big.Rat)
			x.SetFloat64(f)
		} else {
			x = new(big.Rat).Inv(x)
		}
	}
	limited := limitDenominator(x, big.NewInt(maxVal))
	n, d := limited.Num(), limited.Denom()
	if inv {
		return d, n, nil
	}
	return n, d, nil
}

var fractionFormat = regexp.MustCompile(`(?i)^\s*([-+]?)(\d*|\d+(_\d+)*)(?:(?:\s*/\s*(\d+(_\d+)*))?|(?:\.(\d*|\d+(_\d+)*))?(?:E([-+]?\d+(_\d+)*))?)\s*$`)

// fractionFromString is fractions.Fraction(str): the normalized numerator
// and denominator, or the exception.
func fractionFromString(text string) (*big.Int, *big.Int, error) {
	m := fractionFormat.FindStringSubmatch(text)
	// Python's pattern also requires a digit or ".digit" after the sign.
	body := strings.TrimLeft(strings.TrimLeft(text, " \t\n\r\x0b\x0c"), "+-")
	lookahead := len(body) > 0 && body[0] >= '0' && body[0] <= '9' ||
		len(body) > 1 && body[0] == '.' && body[1] >= '0' && body[1] <= '9'
	if m == nil || !lookahead {
		return nil, nil, raise("ValueError: Invalid literal for Fraction")
	}
	parse := func(digits string) *big.Int {
		v, _ := new(big.Int).SetString(strings.ReplaceAll(digits, "_", ""), 10)
		if v == nil {
			v = big.NewInt(0)
		}
		return v
	}
	num := parse(m[2])
	den := big.NewInt(1)
	if m[4] != "" {
		den = parse(m[4])
	} else {
		if m[6] != "" {
			decimal := strings.ReplaceAll(m[6], "_", "")
			scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(len(decimal))), nil)
			num.Mul(num, scale).Add(num, parse(decimal))
			den.Mul(den, scale)
		}
		if m[8] != "" {
			exp, err := strconv.Atoi(strings.ReplaceAll(m[8], "_", ""))
			if err != nil || exp > 4000 || exp < -4000 {
				return nil, nil, raise("unported: Fraction exponent out of the ported range")
			}
			if exp >= 0 {
				num.Mul(num, new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(exp)), nil))
			} else {
				den.Mul(den, new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(-exp)), nil))
			}
		}
	}
	if m[1] == "-" {
		num.Neg(num)
	}
	if den.Sign() == 0 {
		return nil, nil, raise("ZeroDivisionError: Fraction(n, 0)")
	}
	r := new(big.Rat).SetFrac(num, den)
	return r.Num(), r.Denom(), nil
}

// writeLen is _write_dispatch[typ](*values): the length of the data or the
// exception.
func writeLen(typ uint16, stored pyVal) (int, int, error) {
	vals := values(stored)
	if stored.kind == pyDict {
		vals = []pyVal{stored}
	}
	switch typ {
	case 1:
		if len(vals) != 1 {
			return 0, 0, raise("TypeError: write_byte takes one value")
		}
		v := vals[0]
		if v.kind == pyRational {
			var err error
			if v, err = pyIntOf(v); err != nil {
				return 0, 0, err
			}
		}
		switch v.kind {
		case pyInt:
			if !inRange(v.i, 0, 255) {
				return 0, 0, raise("ValueError: bytes must be in range(0, 256)")
			}
			return 1, 1, nil
		case pyBytes:
			return len(v.b), len(v.b), nil
		}
		return 0, 0, raise("TypeError: BYTE data is not bytes")
	case 2:
		if len(vals) != 1 {
			return 0, 0, raise("TypeError: write_string takes one value")
		}
		v := vals[0]
		switch v.kind {
		case pyInt:
			n := len(v.i.String()) + 1
			return n, n, nil
		case pyBytes:
			return len(v.b) + 1, len(v.b) + 1, nil
		case pyStr:
			n := utf8.RuneCountInString(v.s) + 1
			return n, n, nil
		}
		return 0, 0, raise("AttributeError: object has no attribute 'encode'")
	case 7:
		if len(vals) != 1 {
			return 0, 0, raise("TypeError: write_undefined takes one value")
		}
		v := vals[0]
		if v.kind == pyRational {
			var err error
			if v, err = pyIntOf(v); err != nil {
				return 0, 0, err
			}
		}
		switch v.kind {
		case pyInt:
			n := len(v.i.String())
			return n, n, nil
		case pyBytes:
			return len(v.b), len(v.b), nil
		}
		return 0, 0, raise("TypeError: UNDEFINED data has no len")
	case 5:
		for _, v := range vals {
			n, d, err := limitRational(v, math.MaxUint32)
			if err != nil {
				return 0, 0, err
			}
			if !inRange(n, 0, math.MaxUint32) || !inRange(d, 0, math.MaxUint32) {
				return 0, 0, raise("struct.error: 'L' format requires 0 <= number <= 0xFFFFFFFF")
			}
		}
		return 8 * len(vals), len(vals), nil
	case 10:
		for _, v := range vals {
			if err := checkSignedRational(v); err != nil {
				return 0, 0, err
			}
		}
		return 8 * len(vals), len(vals), nil
	}
	format, ok := packFormats[typ]
	if !ok {
		return 0, 0, raise("KeyError: no writer")
	}
	for _, v := range vals {
		if format.floating {
			f, err := toFloat(v)
			if err != nil {
				return 0, 0, err
			}
			if typ == 11 && math.IsInf(float64(float32(f)), 0) && !math.IsInf(f, 0) {
				return 0, 0, raise("OverflowError: float too large to pack with f format")
			}
			continue
		}
		if v.kind != pyInt {
			return 0, 0, raise("struct.error: required argument is not an integer")
		}
		if v.i.Cmp(format.lo) < 0 || v.i.Cmp(format.hi) > 0 {
			return 0, 0, raise("struct.error: argument out of range")
		}
	}
	return format.size * len(vals), len(vals), nil
}

// checkSignedRational is _limit_signed_rational followed by pack("2l").
func checkSignedRational(v pyVal) error {
	var n, d *big.Int
	switch v.kind {
	case pyRational:
		n, d = big.NewInt(v.num), big.NewInt(v.den)
	case pyInt:
		n, d = v.i, big.NewInt(1)
	case pyFloat:
		if math.IsNaN(v.f) {
			return raise("ValueError: cannot convert NaN to integer ratio")
		}
		if math.IsInf(v.f, 0) {
			return raise("OverflowError: cannot convert Infinity to integer ratio")
		}
		r := new(big.Rat)
		r.SetFloat64(v.f)
		n, d = r.Num(), r.Denom()
	case pyStr:
		var err error
		if n, d, err = fractionFromString(v.s); err != nil {
			return err
		}
	default:
		return raise("TypeError: argument should be a string or a Rational instance")
	}
	minVal, maxVal := big.NewInt(math.MinInt32), big.NewInt(math.MaxInt32)
	if n.Cmp(minVal) < 0 || d.Cmp(minVal) < 0 {
		var err error
		if n, d, err = limitRational(v, 1<<31); err != nil {
			return err
		}
	}
	if n.Cmp(maxVal) > 0 || d.Cmp(maxVal) > 0 {
		if d.Sign() == 0 {
			return raise("ZeroDivisionError: float division by zero")
		}
		nf, _ := new(big.Float).SetInt(n).Float64()
		df, _ := new(big.Float).SetInt(d).Float64()
		var err error
		if n, d, err = limitRational(pyVal{kind: pyFloat, f: nf / df}, math.MaxInt32); err != nil {
			return err
		}
	}
	if !inRange(n, math.MinInt32, math.MaxInt32) || !inRange(d, math.MinInt32, math.MaxInt32) {
		return raise("struct.error: 'l' format requires -0x80000000 <= number <= 0x7FFFFFFF")
	}
	return nil
}

// directoryBytes is ImageFileDirectory_v2.tobytes(offset) on a directory
// built with setValue: its length, or the exception.
func directoryBytes(group int, tags *dictVal, offset int, order binary.ByteOrder) (int, error) {
	type entry struct {
		tag   uint16
		typ   uint16
		value pyVal
	}
	var entries []entry
	for _, k := range tags.keys {
		typ, stored, err := setValue(k, group, tags.vals[k])
		if err != nil {
			return 0, err
		}
		entries = append(entries, entry{k, typ, stored})
	}
	sortEntries := func() {
		for i := 1; i < len(entries); i++ {
			for j := i; j > 0 && entries[j].tag < entries[j-1].tag; j-- {
				entries[j], entries[j-1] = entries[j-1], entries[j]
			}
		}
	}
	sortEntries()
	offset += 2 + 12*len(entries) + 4
	length := 2 + 12*len(entries) + 4
	hasStrip := false
	var stripData bool
	var stripValue pyVal
	var stripTyp uint16
	for _, e := range entries {
		var n int
		if e.typ == 4 && e.value.kind == pyDict {
			var err error
			if n, err = directoryBytes(int(e.tag), e.value.dict, offset, order); err != nil {
				return 0, err
			}
		} else {
			var err error
			if n, _, err = writeLen(e.typ, e.value); err != nil {
				return 0, err
			}
		}
		if e.tag == tagStripOffset {
			hasStrip, stripData, stripValue, stripTyp = true, n > 4, e.value, e.typ
		}
		if n > 4 {
			offset += (n + 1) / 2 * 2
			length += (n + 1) / 2 * 2
		}
	}
	if hasStrip {
		if err := checkStripOffsets(stripTyp, stripValue, stripData, offset, order); err != nil {
			return 0, err
		}
	}
	return length, nil
}

// checkStripOffsets is the StripOffsets rewrite in tobytes: the written
// values are read back, shifted by the final offset and written again.
func checkStripOffsets(typ uint16, stored pyVal, hasData bool, offset int, order binary.ByteOrder) error {
	shift := big.NewInt(int64(offset))
	vals := values(stored)
	if !hasData {
		// value = pack("L", unpack("L", data.ljust(4, b"\0"))[0] + offset)
		small := make([]byte, 0, 4)
		for _, v := range vals {
			switch typ {
			case 1, 7:
				if v.kind == pyRational {
					v, _ = pyIntOf(v)
				}
				switch v.kind {
				case pyBytes:
					small = append(small, v.b...)
				case pyInt:
					if typ == 1 {
						small = append(small, byte(v.i.Int64()))
					} else {
						small = append(small, v.i.String()...)
					}
				}
			case 2:
				switch v.kind {
				case pyInt:
					small = append(small, v.i.String()...)
				case pyBytes:
					small = append(small, v.b...)
				case pyStr:
					for _, c := range v.s {
						if c < 128 {
							small = append(small, byte(c))
						} else {
							small = append(small, '?')
						}
					}
				}
				small = append(small, 0)
			case 3, 8:
				small = binary.LittleEndian.AppendUint16(small, 0)
				order.PutUint16(small[len(small)-2:], uint16(v.i.Int64()))
			case 4, 9, 13:
				small = binary.LittleEndian.AppendUint32(small, 0)
				order.PutUint32(small[len(small)-4:], uint32(v.i.Int64()))
			case 6:
				small = append(small, byte(v.i.Int64()))
			case 11:
				f, _ := toFloat(v)
				small = binary.LittleEndian.AppendUint32(small, 0)
				order.PutUint32(small[len(small)-4:], math.Float32bits(float32(f)))
			}
		}
		for len(small) < 4 {
			small = append(small, 0)
		}
		first := new(big.Int).SetUint64(uint64(order.Uint32(small)))
		if first.Add(first, shift).Cmp(big.NewInt(math.MaxUint32)) > 0 {
			return raise("struct.error: 'L' format requires 0 <= number <= 0xFFFFFFFF")
		}
		return nil
	}
	switch typ {
	case 1, 7:
		return raise("TypeError: writer takes one value")
	case 2:
		return raise("TypeError: can only concatenate str (not \"int\") to str")
	case 5, 10:
		for _, v := range vals {
			if v.den == 0 {
				return raise("ValueError: cannot convert NaN to integer ratio")
			}
		}
		return nil
	case 11:
		for _, v := range vals {
			f, _ := toFloat(v)
			g := f + float64(offset)
			if math.IsInf(float64(float32(g)), 0) && !math.IsInf(g, 0) {
				return raise("OverflowError: float too large to pack with f format")
			}
		}
		return nil
	case 12:
		return nil
	}
	format, ok := packFormats[typ]
	if !ok {
		return nil
	}
	for _, v := range vals {
		if v.kind == pyInt && new(big.Int).Add(v.i, shift).Cmp(format.hi) > 0 {
			return raise("struct.error: argument out of range")
		}
	}
	return nil
}

// loadSubIFD is Exif._get_ifd_dict(offset, group): nil when seek raises the
// caught KeyError or TypeError.
func loadSubIFD(data []byte, order binary.ByteOrder, offset pyVal, group int) (*dictVal, error) {
	if offset.kind != pyInt {
		return nil, nil
	}
	if offset.i.Sign() < 0 {
		return nil, raise("ValueError: negative seek value")
	}
	if !offset.i.IsInt64() {
		return nil, raise("OverflowError: Python int too large to convert to C ssize_t")
	}
	r := &reader{data: data, pos: offset.i.Int64()}
	tags := loadIFD(r, order, false)
	dict := newDict()
	for _, k := range sortedTags(tags) {
		dict.set(k, sourceValue(k, group, tags[k], order))
	}
	return dict, nil
}

func sortedTags(tags map[uint16]ifdEntry) []uint16 {
	keys := make([]uint16, 0, len(tags))
	for k := range tags {
		keys = append(keys, k)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}

// tobytesError is Exif.tobytes() for the EXIF data exif_transpose rewrites
// (info["exif"], else "Raw profile type exif"), after deleting the
// orientation tag: nil, or the exception it raises.
func tobytesError(info pil.Info) error {
	var data []byte
	switch {
	case info.HasExif:
		data = info.Exif
	case info.HasRawProfileExif:
		lines := strings.Split(info.RawProfileExif, "\n")
		joined := ""
		if len(lines) > 3 {
			joined = strings.Join(lines[3:], "")
		}
		var err error
		if data, err = fromHex(joined); err != nil {
			return err
		}
	default:
		return nil
	}
	for len(data) > 0 && strings.HasPrefix(string(data[:min(6, len(data))]), "Exif\x00\x00") {
		data = data[6:]
	}
	top := newDict()
	var order binary.ByteOrder = binary.LittleEndian
	if len(data) > 0 {
		tags, ord, err := loadBlock(data)
		if err != nil {
			return err
		}
		order = ord
		delete(tags, orientationTag)
		for _, k := range sortedTags(tags) {
			top.set(k, sourceValue(k, -1, tags[k], order))
		}
	}
	for _, k := range top.keys {
		v := top.vals[k]
		if (k == tagExifIFD || k == tagGPSIFD) && v.kind != pyDict {
			sub, err := loadSubIFD(data, order, v, int(k))
			if err != nil {
				return err
			}
			if sub == nil {
				sub = newDict()
			}
			if k == tagExifIFD {
				if interop, ok := sub.vals[tagInteropIFD]; ok && interop.kind != pyDict {
					copied := newDict()
					for _, kk := range sub.keys {
						copied.set(kk, sub.vals[kk])
					}
					in, err := loadSubIFD(data, order, interop, tagInteropIFD)
					if err != nil {
						return err
					}
					if in == nil {
						in = newDict()
					}
					copied.set(tagInteropIFD, pyVal{kind: pyDict, dict: in})
					sub = copied
				}
			}
			top.vals[k] = pyVal{kind: pyDict, dict: sub}
		}
	}
	_, err := directoryBytes(-1, top, 8, order)
	return err
}

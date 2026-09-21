// Package cursors ports app.api.cursors: the opaque keyset cursor that pages
// group messages, memories and post comments. A cursor is
// `base64url(created_at.isoformat() + "|" + str(message_id))` without padding.
//
// Parity, not correctness, is the contract (ADR-0029 §2.4): oracle_test.go
// replays testdata/python_*.json, rendered by
// scripts/render_domain_w2_goldens.py from the real module in the parity API
// image. Decoding is where parity is hard, because Python accepts far more
// than it ever writes: base64 in either alphabet under binascii's strict
// mode, every spelling `datetime.fromisoformat` takes (week dates, a separator
// that is any character, `,` fractions, offsets with seconds and
// microseconds), and every spelling `uuid.UUID(str)` takes, which ends in
// `int(text, 16)` and so admits whitespace, signs, `0x`, underscores and any
// Unicode decimal digit. This file reproduces those rules from the CPython
// 3.12 sources, byte for byte where the C code works on bytes.
package cursors

import (
	"encoding/base64"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// CursorError is Python's CursorError. Every refusal carries the same
// message, "Invalid cursor", whatever went wrong.
type CursorError struct{}

func (*CursorError) Error() string { return "Invalid cursor" }

func invalid() (Position, error) { return Position{}, &CursorError{} }

// DateTime is a Python datetime as fromisoformat builds it: wall clock fields
// and a UTC offset in microseconds, which may be anything strictly inside a
// day, seconds and microseconds included (a time.Location cannot hold the
// latter). A cursor with no offset decodes as UTC, offset 0.
type DateTime struct {
	Year, Month, Day                  int
	Hour, Minute, Second, Microsecond int
	OffsetMicros                      int64
}

// Time is the instant. The location is a fixed zone when the offset is whole
// seconds, else UTC.
func (d DateTime) Time() time.Time {
	nanos := d.Microsecond * 1000
	if d.OffsetMicros%1_000_000 == 0 {
		zone := time.UTC
		if d.OffsetMicros != 0 {
			zone = time.FixedZone("", int(d.OffsetMicros/1_000_000))
		}
		return time.Date(d.Year, time.Month(d.Month), d.Day, d.Hour, d.Minute, d.Second, nanos, zone)
	}
	wall := time.Date(d.Year, time.Month(d.Month), d.Day, d.Hour, d.Minute, d.Second, nanos, time.UTC)
	return wall.Add(-time.Duration(d.OffsetMicros) * time.Microsecond)
}

// ISOFormat is datetime.isoformat() for an aware datetime: microseconds only
// when non-zero, and the offset as ±HH:MM, then :SS when seconds or
// microseconds are set, then .ffffff when microseconds are.
func (d DateTime) ISOFormat() string {
	var b strings.Builder
	b.WriteString(pad(d.Year, 4) + "-" + pad(d.Month, 2) + "-" + pad(d.Day, 2))
	b.WriteString("T" + pad(d.Hour, 2) + ":" + pad(d.Minute, 2) + ":" + pad(d.Second, 2))
	if d.Microsecond != 0 {
		b.WriteString("." + pad(d.Microsecond, 6))
	}
	offset, sign := d.OffsetMicros, "+"
	if offset < 0 {
		offset, sign = -offset, "-"
	}
	micros := int(offset % 1_000_000)
	seconds := int(offset / 1_000_000)
	b.WriteString(sign + pad(seconds/3600, 2) + ":" + pad(seconds%3600/60, 2))
	switch {
	case micros != 0:
		b.WriteString(":" + pad(seconds%60, 2) + "." + pad(micros, 6))
	case seconds%60 != 0:
		b.WriteString(":" + pad(seconds%60, 2))
	}
	return b.String()
}

// Position is what decode_cursor returns: the moment and the message id in
// str(UUID) form.
type Position struct {
	CreatedAt DateTime
	MessageID string
}

// EncodeCursor is encode_cursor for a time the repository read: its wall clock
// and offset in its own location, truncated to microseconds. messageID is
// written as given, so pass str(UUID): lowercase, hyphenated.
func EncodeCursor(createdAt time.Time, messageID string) string {
	_, offset := createdAt.Zone()
	return EncodeDateTime(DateTime{
		Year: createdAt.Year(), Month: int(createdAt.Month()), Day: createdAt.Day(),
		Hour: createdAt.Hour(), Minute: createdAt.Minute(), Second: createdAt.Second(),
		Microsecond:  createdAt.Nanosecond() / 1000,
		OffsetMicros: int64(offset) * 1_000_000,
	}, messageID)
}

// EncodeDateTime is encode_cursor for a DateTime, such as a decoded one.
func EncodeDateTime(createdAt DateTime, messageID string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(createdAt.ISOFormat() + "|" + messageID))
}

// DecodeCursor is decode_cursor.
func DecodeCursor(raw string) (Position, error) {
	if raw == "" {
		return invalid()
	}
	// raw.encode("ascii"): a Python str is code points, so any byte of a
	// multi-byte (or broken) sequence is a character above U+007F.
	for i := 0; i < len(raw); i++ {
		if raw[i] >= 0x80 {
			return invalid()
		}
	}
	padded := raw + strings.Repeat("=", (4-len(raw)%4)%4)
	data, ok := decodeBase64Strict(padded)
	if !ok || !utf8.Valid(data) {
		return invalid()
	}
	stamp, id, found := strings.Cut(string(data), "|")
	if !found {
		return invalid()
	}
	createdAt, ok := fromISOFormat(stamp)
	if !ok {
		return invalid()
	}
	messageID, ok := parseUUID(id)
	if !ok {
		return invalid()
	}
	return Position{CreatedAt: createdAt, MessageID: messageID}, nil
}

// decodeBase64Strict is base64.b64decode(s, altchars=b"-_", validate=True):
// "-" and "_" are translated to "+" and "/" (which stay valid themselves),
// then binascii.a2b_base64 in strict mode runs. Unlike encoding/base64 it
// accepts nonzero trailing bits and refuses newlines.
func decodeBase64Strict(s string) ([]byte, bool) {
	if len(s) > 0 && s[0] == '=' {
		return nil, false // leading padding
	}
	out := make([]byte, 0, len(s)*3/4)
	quad, pads := 0, 0
	padding := false
	var left byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '=' {
			padding = true
			if quad == 0 {
				return nil, false // excess padding
			}
			if quad >= 2 {
				pads++
				if quad+pads >= 4 {
					if i+1 < len(s) {
						return nil, false // excess data after padding
					}
					return out, true
				}
			}
			continue
		}
		v := base64Value(c)
		if v >= 64 || padding {
			return nil, false // not base64, or discontinuous padding
		}
		pads = 0
		switch quad {
		case 0:
			quad, left = 1, v
		case 1:
			quad = 2
			out = append(out, left<<2|v>>4)
			left = v & 0x0F
		case 2:
			quad = 3
			out = append(out, left<<4|v>>2)
			left = v & 0x03
		case 3:
			quad = 0
			out = append(out, left<<6|v)
			left = 0
		}
	}
	return out, quad == 0
}

func base64Value(c byte) byte {
	switch {
	case c >= 'A' && c <= 'Z':
		return c - 'A'
	case c >= 'a' && c <= 'z':
		return c - 'a' + 26
	case c >= '0' && c <= '9':
		return c - '0' + 52
	case c == '+' || c == '-':
		return 62
	case c == '/' || c == '_':
		return 63
	}
	return 255
}

// cstr is the NUL-terminated UTF-8 buffer CPython's parsers walk: reading at
// or past the end yields NUL, and an embedded NUL stops them the same way.
type cstr string

func (s cstr) at(i int) byte {
	if i >= 0 && i < len(s) {
		return s[i]
	}
	return 0
}

func isDigit(c byte) bool { return c-'0' < 10 }

// parseDigits is parse_digits: exactly count ASCII digits from *p.
func (s cstr) parseDigits(p *int, into *int, count int) bool {
	for range count {
		digit := s.at(*p) - '0'
		*p++
		if digit > 9 {
			return false
		}
		*into = *into*10 + int(digit)
	}
	return true
}

// fromISOFormat is datetime.fromisoformat (CPython 3.12
// Modules/_datetimemodule.c), followed by decode_cursor's rule that a naive
// result is UTC.
func fromISOFormat(text string) (DateTime, bool) {
	if utf8.RuneCountInString(text) < 7 {
		return DateTime{}, false
	}
	s := cstr(text)
	n := len(s)
	separator := s.separatorLocation()
	var d DateTime
	rv := s.parseDate(separator, &d)
	tzSeconds, tzMicros := 0, 0
	if rv == 0 && n > separator {
		p := separator
		switch lead := s.at(p); {
		case lead&0x80 == 0:
			p++
		case lead&0xF0 == 0xE0:
			p += 3
		case lead&0xF0 == 0xF0:
			p += 4
		default:
			p += 2
		}
		rv = s.parseTime(p, n-p, &d, &tzSeconds, &tzMicros)
	}
	if rv < 0 {
		return DateTime{}, false
	}
	if rv == 1 && tzSeconds != 0 {
		// tzinfo_from_isoformat_results: an offset of zero whole seconds is
		// UTC even when microseconds were written. Otherwise the timezone
		// must lie strictly inside one day.
		total := int64(tzSeconds)*1_000_000 + int64(tzMicros)
		if total <= -86_400_000_000 || total >= 86_400_000_000 {
			return DateTime{}, false
		}
		d.OffsetMicros = total
	}
	if d.Year < 1 || d.Year > 9999 || d.Month < 1 || d.Month > 12 || d.Day < 1 || d.Day > daysInMonth(d.Year, d.Month) {
		return DateTime{}, false
	}
	if d.Hour > 23 || d.Minute > 59 || d.Second > 59 || d.Microsecond > 999_999 {
		return DateTime{}, false
	}
	return d, true
}

// separatorLocation is _find_isoformat_datetime_separator over the byte
// length.
func (s cstr) separatorLocation() int {
	n := len(s)
	if n == 7 {
		return 7
	}
	if s.at(4) == '-' {
		if s.at(5) == 'W' {
			if n < 8 {
				return -1
			}
			if n > 8 && s.at(8) == '-' {
				if n == 9 {
					return -1
				}
				if n > 10 && isDigit(s.at(10)) {
					return 8
				}
				return 10
			}
			return 8
		}
		return 10
	}
	if s.at(4) == 'W' {
		index := 7
		for ; index < n; index++ {
			if !isDigit(s.at(index)) {
				break
			}
		}
		if index < 9 {
			return index
		}
		if index%2 == 0 {
			return 7
		}
		return 8
	}
	return 8
}

// parseDate is parse_isoformat_date. limit is the separator location; -1 is
// C's (size_t)-1, larger than any position.
func (s cstr) parseDate(limit int, d *DateTime) int {
	p := 0
	if !s.parseDigits(&p, &d.Year, 4) {
		return -1
	}
	usesSeparator := s.at(p) == '-'
	if usesSeparator {
		p++
	}
	if s.at(p) == 'W' {
		p++
		week, weekday := 0, 0
		if !s.parseDigits(&p, &week, 2) {
			return -3
		}
		if limit < 0 || p < limit {
			if usesSeparator {
				c := s.at(p)
				p++
				if c != '-' {
					return -2
				}
			}
			if !s.parseDigits(&p, &weekday, 1) {
				return -4
			}
		} else {
			weekday = 1
		}
		if code := isoToYMD(d.Year, week, weekday, d); code != 0 {
			return -3 + code
		}
		return 0
	}
	if !s.parseDigits(&p, &d.Month, 2) {
		return -1
	}
	if usesSeparator {
		c := s.at(p)
		p++
		if c != '-' {
			return -2
		}
	}
	if !s.parseDigits(&p, &d.Day, 2) {
		return -1
	}
	return 0
}

// parseTime is parse_isoformat_time: 0 without an offset, 1 with one.
func (s cstr) parseTime(p, length int, d *DateTime, tzSeconds, tzMicros *int) int {
	end := p + length
	tz := p
	for {
		if c := s.at(tz); c == 'Z' || c == '+' || c == '-' {
			break
		}
		tz++
		if tz >= end {
			break
		}
	}
	rv := s.parseClock(p, tz, &d.Hour, &d.Minute, &d.Second, &d.Microsecond)
	if rv < 0 {
		return rv
	}
	if tz == end {
		if rv == 1 {
			return -5
		}
		return 0
	}
	if s.at(tz) == 'Z' {
		*tzSeconds, *tzMicros = 0, 0
		if s.at(tz+1) != 0 {
			return -5
		}
		return 1
	}
	sign := 1
	if s.at(tz) == '-' {
		sign = -1
	}
	var hours, minutes, seconds int
	rv = s.parseClock(tz+1, end, &hours, &minutes, &seconds, tzMicros)
	*tzSeconds = sign * (hours*3600 + minutes*60 + seconds)
	*tzMicros *= sign
	if rv != 0 {
		return -5
	}
	return 1
}

var fractionScale = [...]int{100000, 10000, 1000, 100, 10}

// parseClock is parse_hh_mm_ss_ff: [HH[:?MM[:?SS]]][.,fraction] ending at end.
// It returns 1 when something other than NUL follows what it read.
func (s cstr) parseClock(p, end int, hour, minute, second, micro *int) int {
	*hour, *minute, *second, *micro = 0, 0, 0, 0
	fields := [3]*int{hour, minute, second}
	hasSeparator := true
	for i := range 3 {
		if !s.parseDigits(&p, fields[i], 2) {
			return -3
		}
		c := s.at(p)
		p++
		if i == 0 {
			hasSeparator = c == ':'
		}
		if p >= end {
			if c != 0 {
				return 1
			}
			return 0
		} else if hasSeparator && c == ':' {
			continue
		} else if c == '.' || c == ',' {
			break
		} else if !hasSeparator {
			p--
		} else {
			return -4
		}
	}
	count := min(end-p, 6)
	if !s.parseDigits(&p, micro, count) {
		return -3
	}
	if count < 6 {
		*micro *= fractionScale[count-1]
	}
	for isDigit(s.at(p)) {
		p++
	}
	if s.at(p) != 0 {
		return 1
	}
	return 0
}

// isoToYMD is iso_to_ymd: 0, or -4 (year), -2 (week), -3 (day).
func isoToYMD(isoYear, isoWeek, isoDay int, d *DateTime) int {
	if isoYear < 1 || isoYear > 9999 {
		return -4
	}
	firstWeekday := pyWeekday(isoYear, 1, 1)
	if isoWeek <= 0 || isoWeek >= 53 {
		if isoWeek != 53 || !(firstWeekday == 3 || (firstWeekday == 2 && isLeap(isoYear))) {
			return -2
		}
	}
	if isoDay <= 0 || isoDay >= 8 {
		return -3
	}
	monday := 1 - firstWeekday
	if firstWeekday > 3 {
		monday += 7
	}
	day := time.Date(isoYear, time.January, monday+(isoWeek-1)*7+isoDay-1, 0, 0, 0, 0, time.UTC)
	d.Year, d.Month, d.Day = day.Year(), int(day.Month()), day.Day()
	return 0
}

// pyWeekday is Python's weekday(): Monday is 0.
func pyWeekday(year, month, day int) int {
	return (int(time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC).Weekday()) + 6) % 7
}

func isLeap(year int) bool { return year%4 == 0 && (year%100 != 0 || year%400 == 0) }

func daysInMonth(year, month int) int {
	if month == 2 && isLeap(year) {
		return 29
	}
	return [...]int{0, 31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}[month]
}

// parseUUID is uuid.UUID(text) followed by str(): "urn:" and "uuid:" removed
// everywhere, braces trimmed from both ends, hyphens removed, 32 code points,
// then int(text, 16) in [0, 2**128).
func parseUUID(text string) (string, bool) {
	s := strings.ReplaceAll(text, "urn:", "")
	s = strings.ReplaceAll(s, "uuid:", "")
	s = strings.Trim(s, "{}")
	s = strings.ReplaceAll(s, "-", "")
	if utf8.RuneCountInString(s) != 32 {
		return "", false
	}
	hi, lo, ok := parseHex128(decimalAndSpaceToASCII(s))
	if !ok {
		return "", false
	}
	const digits = "0123456789abcdef"
	var hex [32]byte
	for i := 31; i >= 0; i-- {
		hex[i] = digits[lo&0xF]
		lo = lo>>4 | hi<<60
		hi >>= 4
	}
	h := string(hex[:])
	return h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:], true
}

// decimalAndSpaceToASCII is _PyUnicode_TransformDecimalAndSpaceToASCII: an
// ASCII string is returned unchanged; otherwise characters below U+007F are
// kept, whitespace becomes a space, a decimal digit becomes its ASCII digit,
// and the first other character becomes "?" and ends the text.
func decimalAndSpaceToASCII(s string) cstr {
	ascii := true
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			ascii = false
			break
		}
	}
	if ascii {
		return cstr(s)
	}
	out := make([]byte, 0, len(s))
	for _, r := range s {
		switch {
		case r < 0x7F:
			out = append(out, byte(r))
		case isPySpace(r):
			out = append(out, ' ')
		default:
			value := decimalValue(r)
			if value < 0 {
				return cstr(append(out, '?'))
			}
			out = append(out, byte('0'+value))
		}
	}
	return cstr(out)
}

// parseHex128 is PyLong_FromString(text, &end, 16) with PyLong_FromUnicodeObject's
// demand that it consumed the whole buffer, limited to values below 2**128
// (the caller has at most 32 digits). A negative non-zero value fails, as
// uuid refuses it.
func parseHex128(s cstr) (hi, lo uint64, ok bool) {
	p := 0
	for s.at(p) != 0 && isCSpace(s.at(p)) {
		p++
	}
	negative := false
	switch s.at(p) {
	case '+':
		p++
	case '-':
		p++
		negative = true
	}
	if s.at(p) == '0' && (s.at(p+1) == 'x' || s.at(p+1) == 'X') {
		p += 2
		if s.at(p) == '_' {
			p++
		}
	}
	start := p
	if s.at(start) == '_' {
		return 0, 0, false
	}
	var previous byte
	for {
		c := s.at(p)
		if value := hexValue(c); value < 16 {
			hi = hi<<4 | lo>>60
			lo = lo<<4 | uint64(value)
		} else if c == '_' {
			if previous == '_' {
				return 0, 0, false
			}
		} else {
			break
		}
		previous = c
		p++
	}
	if previous == '_' || p == start {
		return 0, 0, false
	}
	for s.at(p) != 0 && isCSpace(s.at(p)) {
		p++
	}
	if s.at(p) != 0 || p != len(s) {
		return 0, 0, false
	}
	if negative && (hi|lo) != 0 {
		return 0, 0, false
	}
	return hi, lo, true
}

func hexValue(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'z':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'Z':
		return int(c-'A') + 10
	}
	return 37
}

// isCSpace is Py_ISSPACE: the C locale's whitespace.
func isCSpace(c byte) bool { return c == ' ' || (c >= '\t' && c <= '\r') }

// isPySpace is str.isspace() for one code point. oracle_test.go checks it
// against CPython over every code point.
func isPySpace(r rune) bool {
	switch {
	case r >= 0x09 && r <= 0x0D, r >= 0x1C && r <= 0x20:
		return true
	case r == 0x85, r == 0xA0, r == 0x1680, r >= 0x2000 && r <= 0x200A,
		r == 0x2028, r == 0x2029, r == 0x202F, r == 0x205F, r == 0x3000:
		return true
	}
	return false
}

// decimalValue is Py_UNICODE_TODECIMAL: the digit a Unicode Nd character
// stands for, or -1. Nd characters come in runs of ten from zero, and Go 1.23
// and CPython 3.12 both read Unicode 15.0; oracle_test.go checks every code
// point.
func decimalValue(r rune) int {
	for _, span := range unicode.Nd.R16 {
		if lo, hi := rune(span.Lo), rune(span.Hi); r >= lo && r <= hi {
			return int(r-lo) % 10
		}
	}
	for _, span := range unicode.Nd.R32 {
		if lo, hi := rune(span.Lo), rune(span.Hi); r >= lo && r <= hi {
			return int(r-lo) % 10
		}
	}
	return -1
}

func pad(value, width int) string {
	digits := make([]byte, 0, width)
	for value > 0 || len(digits) == 0 {
		digits = append(digits, byte('0'+value%10))
		value /= 10
	}
	for len(digits) < width {
		digits = append(digits, '0')
	}
	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}
	return string(digits)
}

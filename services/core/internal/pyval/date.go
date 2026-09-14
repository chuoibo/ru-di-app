package pyval

import (
	"math"
	"strconv"
	"strings"
	"time"

	"mobile/services/core/internal/pyjson"
)

// Date is datetime.date.
type Date struct {
	Year, Month, Day int
}

// ISOFormat is date.isoformat().
func (d Date) ISOFormat() string {
	return pad(d.Year, 4) + "-" + pad(d.Month, 2) + "-" + pad(d.Day, 2)
}

// DateTime is datetime.datetime. Aware datetimes carry a fixed UTC offset
// in seconds (pydantic's TzInfo); naive ones have Aware false.
type DateTime struct {
	Date
	Hour, Minute, Second, Microsecond int
	Aware                             bool
	Offset                            int
}

// ISOFormat is datetime.isoformat().
func (t DateTime) ISOFormat() string {
	s := t.Date.ISOFormat() + "T" + pad(t.Hour, 2) + ":" + pad(t.Minute, 2) + ":" + pad(t.Second, 2)
	if t.Microsecond != 0 {
		s += "." + pad(t.Microsecond, 6)
	}
	if t.Aware {
		off, sign := t.Offset, "+"
		if off < 0 {
			off, sign = -off, "-"
		}
		s += sign + pad(off/3600, 2) + ":" + pad(off%3600/60, 2)
	}
	return s
}

// Time returns the instant for an aware datetime, or the wall clock in UTC
// for a naive one.
func (t DateTime) Time() time.Time {
	loc := time.UTC
	if t.Aware && t.Offset != 0 {
		loc = time.FixedZone("", t.Offset)
	}
	return time.Date(t.Year, time.Month(t.Month), t.Day, t.Hour, t.Minute, t.Second, t.Microsecond*1000, loc)
}

func pad(v, width int) string {
	s := strconv.Itoa(v)
	for len(s) < width {
		s = "0" + s
	}
	return s
}

// speedate (the date parser pydantic-core links) error texts.
const (
	sdTooShort           = "input is too short"
	sdExtraCharacters    = "unexpected extra characters at the end of the input"
	sdDateTimeSep        = "invalid datetime separator, expected `T`, `t`, `_` or space"
	sdDateSep            = "invalid date separator, expected `-`"
	sdCharYear           = "invalid character in year"
	sdCharMonth          = "invalid character in month"
	sdCharDay            = "invalid character in day"
	sdTimeSep            = "invalid time separator, expected `:`"
	sdCharHour           = "invalid character in hour"
	sdCharMinute         = "invalid character in minute"
	sdCharSecond         = "invalid character in second"
	sdTzSign             = "invalid timezone sign"
	sdTzHour             = "invalid timezone hour"
	sdTzMinute           = "invalid timezone minute"
	sdRangeMonth         = "month value is outside expected range of 1-12"
	sdRangeDay           = "day value is outside expected range"
	sdRangeHour          = "hour value is outside expected range of 0-23"
	sdRangeMinute        = "minute value is outside expected range of 0-59"
	sdRangeSecond        = "second value is outside expected range of 0-59"
	sdRangeTz            = "timezone offset must be less than 24 hours"
	sdRangeTzMinute      = "timezone minute value is outside expected range of 0-59"
	sdFractionTooLong    = "second fraction value is more than 6 digits long"
	sdFractionMissing    = "second fraction digits missing after `.`"
	sdDateTooSmall       = "dates before 0000 are not supported as unix timestamps"
	sdDateTooLarge       = "dates after 9999 are not supported as unix timestamps"
	sdDateNotExact       = "timestamp must be a date"
	sdNaN                = "NaN values not permitted"
	sdYearZero           = "year 0 is out of range"
	msWatershed          = int64(20_000_000_000)
	unix0000             = int64(-62_167_219_200)
	unix9999             = int64(253_402_300_799)
	microsecondsTruncate = "truncate"
)

func digitAt(s string, i int) (int, bool) {
	if i >= len(s) || s[i] < '0' || s[i] > '9' {
		return 0, false
	}
	return int(s[i] - '0'), true
}

// parseDatePartial is Date::parse_bytes_partial: YYYY-MM-DD at the start.
func parseDatePartial(s string) (Date, string) {
	if len(s) < 10 {
		return Date{}, sdTooShort
	}
	year := 0
	for i := 0; i < 4; i++ {
		d, ok := digitAt(s, i)
		if !ok {
			return Date{}, sdCharYear
		}
		year = year*10 + d
	}
	if s[4] != '-' {
		return Date{}, sdDateSep
	}
	m1, ok1 := digitAt(s, 5)
	m2, ok2 := digitAt(s, 6)
	if !ok1 || !ok2 {
		return Date{}, sdCharMonth
	}
	if s[7] != '-' {
		return Date{}, sdDateSep
	}
	d1, ok1 := digitAt(s, 8)
	d2, ok2 := digitAt(s, 9)
	if !ok1 || !ok2 {
		return Date{}, sdCharDay
	}
	month, day := m1*10+m2, d1*10+d2
	var maxDays int
	switch month {
	case 1, 3, 5, 7, 8, 10, 12:
		maxDays = 31
	case 4, 6, 9, 11:
		maxDays = 30
	case 2:
		maxDays = 28
		if year%4 == 0 && (year%100 != 0 || year%400 == 0) {
			maxDays = 29
		}
	default:
		return Date{}, sdRangeMonth
	}
	if day < 1 || day > maxDays {
		return Date{}, sdRangeDay
	}
	return Date{Year: year, Month: month, Day: day}, ""
}

// parseDateStr is Date::parse_bytes: RFC 3339 date, else an integer unix
// timestamp that falls exactly on a day.
func parseDateStr(s string) (Date, string) {
	d, e := parseDatePartial(s)
	if e == "" && len(s) > 10 {
		e = sdExtraCharacters
	}
	if e == "" {
		return d, ""
	}
	if n, ok := speedateInt(s); ok {
		sec, _, terr := timestampWatershed(n)
		if terr != "" {
			return Date{}, terr
		}
		d, cerr := dateFromUnix(sec)
		if cerr != "" {
			return Date{}, cerr
		}
		if mod(sec, 86400) != 0 {
			return Date{}, sdDateNotExact
		}
		return d, ""
	}
	return Date{}, e
}

type timeParts struct {
	hour, minute, second, micro int
	aware                       bool
	offset                      int
}

// parseTimeAt is Time::parse_bytes_offset from index off to the end.
func parseTimeAt(s string, off int, truncate bool) (timeParts, string) {
	var t timeParts
	if len(s)-off < 5 {
		return t, sdTooShort
	}
	h1, ok1 := digitAt(s, off)
	h2, ok2 := digitAt(s, off+1)
	if !ok1 || !ok2 {
		return t, sdCharHour
	}
	if s[off+2] != ':' {
		return t, sdTimeSep
	}
	if t.hour = h1*10 + h2; t.hour > 23 {
		return t, sdRangeHour
	}
	n1, ok1 := digitAt(s, off+3)
	n2, ok2 := digitAt(s, off+4)
	if !ok1 || !ok2 {
		return t, sdCharMinute
	}
	if t.minute = n1*10 + n2; t.minute > 59 {
		return t, sdRangeMinute
	}
	pos := off + 5
	if pos < len(s) && s[pos] == ':' {
		s1, ok1 := digitAt(s, pos+1)
		s2, ok2 := digitAt(s, pos+2)
		if !ok1 || !ok2 {
			return t, sdCharSecond
		}
		if t.second = s1*10 + s2; t.second > 59 {
			return t, sdRangeSecond
		}
		pos += 3
		if pos < len(s) && (s[pos] == '.' || s[pos] == ',') {
			pos++
			i := 0
			for ; pos+i < len(s) && s[pos+i] >= '0' && s[pos+i] <= '9'; i++ {
				if i >= 6 {
					if !truncate {
						return t, sdFractionTooLong
					}
					continue
				}
				t.micro = t.micro*10 + int(s[pos+i]-'0')
			}
			if i == 0 {
				return t, sdFractionMissing
			}
			for k := i; k < 6; k++ {
				t.micro *= 10
			}
			pos += i
		}
	}
	if pos < len(s) {
		next := s[pos]
		pos++
		if next == 'Z' || next == 'z' {
			t.aware = true
		} else {
			sign := 0
			switch {
			case next == '+':
				sign = 1
			case next == '-':
				sign = -1
			case next == 0xE2 && pos+1 < len(s) && s[pos] == 0x88 && s[pos+1] == 0x92:
				// U+2212 MINUS SIGN
				sign = -1
				pos += 2
			default:
				return t, sdTzSign
			}
			a, okA := digitAt(s, pos)
			b, okB := digitAt(s, pos+1)
			if !okA || !okB {
				return t, sdTzHour
			}
			var m1 int
			switch {
			case pos+2 < len(s) && s[pos+2] == ':':
				pos += 3
				d, ok := digitAt(s, pos)
				if !ok {
					return t, sdTzMinute
				}
				m1 = d
			case pos+2 < len(s) && s[pos+2] >= '0' && s[pos+2] <= '9':
				pos += 2
				m1 = int(s[pos] - '0')
			default:
				return t, sdTzMinute
			}
			m2, ok := digitAt(s, pos+1)
			if !ok {
				return t, sdTzMinute
			}
			minutes := m1*10 + m2
			if minutes >= 60 {
				return t, sdRangeTzMinute
			}
			offset := sign*(a*10+b)*3600 + sign*minutes*60
			if offset >= 86400 || offset <= -86400 {
				return t, sdRangeTz
			}
			t.aware, t.offset = true, offset
			pos += 2
		}
	}
	if len(s) > pos {
		return t, sdExtraCharacters
	}
	return t, ""
}

// parseDateTimeStr is DateTime::parse_bytes_with_config with a zero unix
// timestamp offset: RFC 3339, else a number of seconds (or milliseconds).
func parseDateTimeStr(s string, truncate bool) (DateTime, string) {
	dt, e := parseDateTimeRFC3339(s, truncate)
	if e == "" {
		return dt, ""
	}
	if n, ok := speedateInt(s); ok {
		return dateTimeFromTimestamp(n, 0)
	}
	if f, digits, ok := speedateFloat(s); ok {
		ms := math.Abs(f) > float64(msWatershed)
		// In "error" mode a fraction finer than a microsecond is not a
		// timestamp at all, and the RFC 3339 error stands (measured).
		if !truncate && (digits > 6 || ms && digits > 3) {
			return DateTime{}, e
		}
		if ms {
			f /= 1000
		}
		sec := math.Floor(f)
		micro := int(math.Round((f - sec) * 1e6))
		return dateTimeFromTimestamp(int64(sec), micro)
	}
	return DateTime{}, e
}

func parseDateTimeRFC3339(s string, truncate bool) (DateTime, string) {
	d, e := parseDatePartial(s)
	if e != "" {
		return DateTime{}, e
	}
	if len(s) < 11 || (s[10] != 'T' && s[10] != 't' && s[10] != ' ' && s[10] != '_') {
		return DateTime{}, sdDateTimeSep
	}
	t, e := parseTimeAt(s, 11, truncate)
	if e != "" {
		return DateTime{}, e
	}
	return DateTime{Date: d, Hour: t.hour, Minute: t.minute, Second: t.second, Microsecond: t.micro, Aware: t.aware, Offset: t.offset}, ""
}

// speedateInt is int_parse_bytes: optional sign, ASCII digits, no overflow.
func speedateInt(s string) (int64, bool) {
	body := s
	neg := false
	if body != "" && (body[0] == '-' || body[0] == '+') {
		neg = body[0] == '-'
		body = body[1:]
	}
	if body == "" {
		return 0, false
	}
	var n int64
	for i := 0; i < len(body); i++ {
		c := body[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		if n > (math.MaxInt64-int64(c-'0'))/10 {
			return 0, false
		}
		n = n*10 + int64(c-'0')
	}
	if neg {
		n = -n
	}
	return n, true
}

// speedateFloat is float_parse_bytes for input that is not an integer:
// optional sign, digits with exactly one ".", an integer part that fits
// i64. It also returns the number of fraction digits.
func speedateFloat(s string) (float64, int, bool) {
	body := s
	if body != "" && (body[0] == '-' || body[0] == '+') {
		body = body[1:]
	}
	whole, frac, found := strings.Cut(body, ".")
	if !found || whole+frac == "" || strings.Contains(frac, ".") {
		return 0, 0, false
	}
	for _, part := range []string{whole, frac} {
		for i := 0; i < len(part); i++ {
			if part[i] < '0' || part[i] > '9' {
				return 0, 0, false
			}
		}
	}
	// The integer part must fit i64 (leading zeros are fine); a larger
	// timestamp that does fit fails later as out of range.
	if whole != "" {
		if _, ok := speedateInt(whole); !ok {
			return 0, 0, false
		}
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, 0, false
	}
	return f, len(frac), true
}

func mod(a, b int64) int64 {
	m := a % b
	if m < 0 {
		m += b
	}
	return m
}

// timestampWatershed reads an absolute value above 2e10 as milliseconds.
func timestampWatershed(ts int64) (int64, int, string) {
	if ts == math.MinInt64 {
		return 0, 0, sdDateTooSmall
	}
	abs := ts
	if abs < 0 {
		abs = -abs
	}
	if abs <= msWatershed {
		return ts, 0, ""
	}
	sec := ts / 1000
	micro := (ts % 1000) * 1000
	if micro < 0 {
		sec--
		micro += 1_000_000
	}
	return sec, int(micro), ""
}

func dateFromUnix(sec int64) (Date, string) {
	if sec < unix0000 {
		return Date{}, sdDateTooSmall
	}
	if sec > unix9999 {
		return Date{}, sdDateTooLarge
	}
	t := time.Unix(sec, 0).UTC()
	return Date{Year: t.Year(), Month: int(t.Month()), Day: t.Day()}, ""
}

// dateTimeFromTimestamp is DateTime::from_timestamp_with_config with UTC.
func dateTimeFromTimestamp(ts int64, micro int) (DateTime, string) {
	sec, extra, e := timestampWatershed(ts)
	if e != "" {
		return DateTime{}, e
	}
	total := micro + extra
	if total >= 1_000_000 {
		sec += int64(total / 1_000_000)
		total %= 1_000_000
	}
	d, e := dateFromUnix(sec)
	if e != "" {
		return DateTime{}, e
	}
	daySec := mod(sec, 86400)
	return DateTime{Date: d, Hour: int(daySec / 3600), Minute: int(daySec % 3600 / 60), Second: int(daySec % 60),
		Microsecond: total, Aware: true}, ""
}

// ---- validators ----

func errDateType() *vErr     { return fail("date_type", "Input should be a valid date", nil) }
func errDateTimeType() *vErr { return fail("datetime_type", "Input should be a valid datetime", nil) }

func errDateParsing(text string) *vErr {
	return fail("date_parsing", "Input should be a valid date in the format YYYY-MM-DD, "+text, ctxOf("error", pyjson.String(text)))
}

func errDateTimeParsing(text string) *vErr {
	return fail("datetime_parsing", "Input should be a valid datetime, "+text, ctxOf("error", pyjson.String(text)))
}

type dateValidator struct{ strict bool }

func (v *dateValidator) name() string { return "date" }

// validate is DateValidator: a date string (or exact integer timestamp);
// failing that, in lax mode, a datetime at midnight (date_from_datetime),
// whose own parse error replaces the date's.
func (v *dateValidator) validate(st *state, in any) (Value, *vErr) {
	d, e := inputDate(in)
	if e != nil && !v.strict {
		st.floor(lax)
		dt, fe, ok := inputDateTime(in, true)
		switch {
		case ok && fe == nil:
			if dt.Hour != 0 || dt.Minute != 0 || dt.Second != 0 || dt.Microsecond != 0 {
				return nil, fail("date_from_datetime_inexact",
					"Datetimes provided to dates should have zero time - e.g. be exact dates", nil)
			}
			d, e = dt.Date, nil
		case fe != nil && fe.lines[0].typ == "datetime_parsing":
			text := string(mustGet(fe.lines[0].ctx, "error").(pyjson.String))
			return nil, fail("date_from_datetime_parsing", "Input should be a valid date or datetime, "+text,
				ctxOf("error", pyjson.String(text)))
		}
	}
	if e != nil {
		return nil, e
	}
	if d.Year == 0 {
		return nil, errDateParsing(sdYearZero)
	}
	return d, nil
}

// inputDate is Input::validate_date in Python mode.
func inputDate(in any) (Date, *vErr) {
	s, ok := in.(pyjson.String)
	if !ok {
		return Date{}, errDateType()
	}
	if hasSurrogate(string(s)) {
		return Date{}, errStringUnicode()
	}
	d, text := parseDateStr(string(s))
	if text != "" {
		return Date{}, errDateParsing(text)
	}
	return d, nil
}

// inputDateTime is Input::validate_datetime in lax Python mode. ok reports
// whether the input was of a kind datetime parsing applies to.
func inputDateTime(in any, truncate bool) (DateTime, *vErr, bool) {
	switch x := in.(type) {
	case pyjson.String:
		if hasSurrogate(string(x)) {
			return DateTime{}, errStringUnicode(), false
		}
		dt, text := parseDateTimeStr(string(x), truncate)
		if text != "" {
			return DateTime{}, errDateTimeParsing(text), true
		}
		return dt, nil, true
	case pyjson.Int:
		if n, ok := x.Int64(); ok {
			dt, text := dateTimeFromTimestamp(n, 0)
			if text != "" {
				return DateTime{}, errDateTimeParsing(text), true
			}
			return dt, nil, true
		}
		f, ok := intToFloat(x)
		if !ok {
			return DateTime{}, errDateTimeType(), false
		}
		return floatAsDateTime(f)
	case pyjson.Float:
		return floatAsDateTime(float64(x))
	}
	return DateTime{}, errDateTimeType(), false
}

// floatAsDateTime is float_as_datetime: NaN refused, whole seconds by
// floor, the fraction rounded to microseconds.
func floatAsDateTime(f float64) (DateTime, *vErr, bool) {
	if math.IsNaN(f) {
		return DateTime{}, errDateTimeParsing(sdNaN), true
	}
	sec := math.Floor(f)
	// The fraction is read as microseconds, or as a fraction of a
	// millisecond when the float itself is past the millisecond watershed
	// (measured: 2e10 + 0.5 is 2e10 s + 500 us).
	scale := 1e6
	if math.Abs(f) > float64(msWatershed) {
		scale = 1e3
	}
	micro := int(math.Round(math.Abs(f-math.Trunc(f)) * scale))
	n := int64(math.MaxInt64)
	switch {
	case sec <= math.MinInt64:
		n = math.MinInt64
	case sec < math.MaxInt64:
		n = int64(sec)
	}
	dt, text := dateTimeFromTimestamp(n, micro)
	if text != "" {
		return DateTime{}, errDateTimeParsing(text), true
	}
	return dt, nil, true
}

type dateTimeValidator struct {
	strict   bool
	truncate bool
}

func (v *dateTimeValidator) name() string { return "datetime" }

// validate is DateTimeValidator: a datetime string or timestamp; failing
// that, in lax mode, a date at midnight (datetime_from_date), whose own
// parse error replaces the datetime's.
func (v *dateTimeValidator) validate(st *state, in any) (Value, *vErr) {
	if _, isBool := in.(pyjson.Bool); isBool {
		return nil, errDateTimeType()
	}
	var dt DateTime
	var e *vErr
	if v.strict {
		e = errDateTimeType()
	} else {
		dt, e, _ = inputDateTime(in, v.truncate)
	}
	if e != nil && !v.strict {
		st.floor(lax)
		if _, isStr := in.(pyjson.String); isStr {
			d, de := inputDate(in)
			switch {
			case de == nil:
				dt, e = DateTime{Date: d}, nil
			case de.lines[0].typ == "date_parsing":
				text := string(mustGet(de.lines[0].ctx, "error").(pyjson.String))
				return nil, fail("datetime_from_date_parsing", "Input should be a valid datetime or date, "+text,
					ctxOf("error", pyjson.String(text)))
			}
		}
	}
	if e != nil {
		return nil, e
	}
	if dt.Year == 0 {
		return nil, errDateTimeParsing(sdYearZero)
	}
	return dt, nil
}

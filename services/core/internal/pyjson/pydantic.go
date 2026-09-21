package pyjson

import (
	"time"
)

// DateTime formats an aware datetime as pydantic 2.13 JSON mode does
// (TypeAdapter(datetime).dump_python(v, mode="json")):
//
//	YYYY-MM-DDTHH:MM:SS[.ffffff](Z|±HH:MM)
//
// The fraction is six digits when the microsecond is non-zero and absent
// otherwise; trailing zeros are kept. A zero UTC offset is "Z" whatever the
// zone. Any other offset is truncated to whole minutes, with the sign taken
// from the full offset, so -00:00:30 is "-00:00". Nanoseconds below the
// microsecond are dropped, because a Python datetime cannot hold them.
// Years outside 1..9999 have no Python datetime and are not meaningful.
func DateTime(t time.Time) string {
	b := appendWallClock(make([]byte, 0, 32), t)
	_, offset := t.Zone()
	if offset == 0 {
		return string(append(b, 'Z'))
	}
	sign := byte('+')
	if offset < 0 {
		sign = '-'
		offset = -offset
	}
	b = append(b, sign)
	b = append2(b, offset/3600)
	b = append(b, ':')
	b = append2(b, offset%3600/60)
	return string(b)
}

// NaiveDateTime formats t's wall clock as pydantic JSON mode formats a naive
// datetime: DateTime without the offset suffix. t's location only decides
// which wall clock is written.
func NaiveDateTime(t time.Time) string {
	return string(appendWallClock(make([]byte, 0, 26), t))
}

// Date formats t's calendar date in its location as pydantic JSON mode
// formats a date: YYYY-MM-DD.
func Date(t time.Time) string {
	return string(appendDate(make([]byte, 0, 10), t))
}

// UUID formats u as str(uuid.UUID): lowercase 8-4-4-4-12 hex, which is what
// pydantic JSON mode emits.
func UUID(u [16]byte) string {
	b := make([]byte, 0, 36)
	for i, c := range u {
		if i == 4 || i == 6 || i == 8 || i == 10 {
			b = append(b, '-')
		}
		b = append(b, hexDigits[c>>4], hexDigits[c&0xF])
	}
	return string(b)
}

func appendDate(b []byte, t time.Time) []byte {
	year, month, day := t.Date()
	b = append4(b, year)
	b = append(b, '-')
	b = append2(b, int(month))
	b = append(b, '-')
	return append2(b, day)
}

func appendWallClock(b []byte, t time.Time) []byte {
	b = appendDate(b, t)
	b = append(b, 'T')
	b = append2(b, t.Hour())
	b = append(b, ':')
	b = append2(b, t.Minute())
	b = append(b, ':')
	b = append2(b, t.Second())
	if micro := t.Nanosecond() / 1000; micro != 0 {
		b = append(b, '.')
		b = append2(b, micro/10000)
		b = append2(b, micro/100%100)
		b = append2(b, micro%100)
	}
	return b
}

func append2(b []byte, v int) []byte {
	if v < 10 {
		b = append(b, '0')
	}
	return appendInt(b, v)
}

func append4(b []byte, v int) []byte {
	if v >= 0 {
		for limit := 1000; limit > 1 && v < limit; limit /= 10 {
			b = append(b, '0')
		}
	}
	return appendInt(b, v)
}

func appendInt(b []byte, v int) []byte {
	if v < 0 {
		b = append(b, '-')
		v = -v
	}
	var tmp [20]byte
	i := len(tmp)
	for {
		i--
		tmp[i] = byte('0' + v%10)
		v /= 10
		if v == 0 {
			break
		}
	}
	return append(b, tmp[i:]...)
}

package oracletest

import (
	"fmt"
	"strconv"
	"time"
)

// Instant reads what datetime.isoformat() writes for an aware datetime:
// YYYY-MM-DDTHH:MM:SS, an optional .ffffff, then the offset as +HH:MM with an
// optional :SS. The result carries the offset as a fixed zone (UTC when it is
// zero), so formatting it again writes the same offset.
func Instant(value any) (time.Time, error) {
	text, ok := value.(string)
	if !ok {
		return time.Time{}, fmt.Errorf("%v (%T) is not a datetime", value, value)
	}
	fail := func() (time.Time, error) { return time.Time{}, fmt.Errorf("%q is not an aware isoformat()", text) }
	if len(text) < 25 || text[10] != 'T' {
		return fail()
	}
	year, month, day, err := CivilDate(text[:10])
	if err != nil {
		return fail()
	}
	numbers := func(spelled ...string) ([]int, bool) {
		out := make([]int, len(spelled))
		for i, part := range spelled {
			n, err := strconv.Atoi(part)
			if err != nil || n < 0 {
				return nil, false
			}
			out[i] = n
		}
		return out, true
	}
	clock, ok := numbers(text[11:13], text[14:16], text[17:19])
	if !ok || text[13] != ':' || text[16] != ':' {
		return fail()
	}
	rest, micro := text[19:], 0
	if rest[0] == '.' {
		parsed, ok := numbers(rest[1:7])
		if !ok || len(rest) < 7 {
			return fail()
		}
		micro, rest = parsed[0], rest[7:]
	}
	if (len(rest) != 6 && len(rest) != 9) || (rest[0] != '+' && rest[0] != '-') || rest[3] != ':' {
		return fail()
	}
	parts := []string{rest[1:3], rest[4:6]}
	if len(rest) == 9 {
		if rest[6] != ':' {
			return fail()
		}
		parts = append(parts, rest[7:9])
	}
	offsetParts, ok := numbers(parts...)
	if !ok {
		return fail()
	}
	offset := offsetParts[0]*3600 + offsetParts[1]*60
	if len(offsetParts) == 3 {
		offset += offsetParts[2]
	}
	if rest[0] == '-' {
		offset = -offset
	}
	zone := time.UTC
	if offset != 0 {
		zone = time.FixedZone("", offset)
	}
	return time.Date(year, time.Month(month), day, clock[0], clock[1], clock[2], micro*1000, zone), nil
}

// CivilDate reads date.isoformat(): YYYY-MM-DD.
func CivilDate(value any) (year, month, day int, err error) {
	text, ok := value.(string)
	if !ok || len(text) != 10 || text[4] != '-' || text[7] != '-' {
		return 0, 0, 0, fmt.Errorf("%v is not a date isoformat()", value)
	}
	if year, err = strconv.Atoi(text[:4]); err == nil {
		if month, err = strconv.Atoi(text[5:7]); err == nil {
			day, err = strconv.Atoi(text[8:10])
		}
	}
	return year, month, day, err
}

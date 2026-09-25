// Package giomo reads a place's opening hours into minutes of the week, so a
// question like "open at 22:30 on Friday" is answered by arithmetic before any
// model sees the place (docs/claude/2026-09-25/thiet-ke-ai/04-rag-va-nap-du-lieu.md).
//
// Two spellings are understood:
//
//   - the catalogue's own "HH:MM – HH:MM" (en dash or hyphen), every day, where
//     an end at or before the start runs past midnight;
//   - a small, common subset of OSM `opening_hours`: rules separated by ";",
//     each an optional day selector (Mo, Mo-Fr, Sa,Su, Mo-Su) followed by one or
//     more "HH:MM-HH:MM" spans, or "off", plus "24/7". A later rule with a day
//     selector replaces the earlier times for those days, as in OSM.
//
// Anything else (public holidays, months, week numbers, sunrise, comments) is
// unknown, never guessed: Doc reports false and the caller treats the place as
// "hours unknown", which is different from closed.
//
// Minute 0 of the week is Monday 00:00 in Vietnam (UTC+7, no daylight saving).
package giomo

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

// PhutTuan is the number of minutes in a week.
const PhutTuan = 7 * 24 * 60

// Vietnam keeps UTC+7 all year; a fixed zone needs no zoneinfo on the host.
var gioVN = time.FixedZone("ICT", 7*3600)

// Khoang is a half-open span [Tu, Den) of minutes of the week.
type Khoang struct{ Tu, Den int }

// Lich is a normalised weekly schedule: sorted, merged, inside [0, PhutTuan).
type Lich struct{ Khoang []Khoang }

var (
	catalogue = regexp.MustCompile(`^\s*(\d{1,2}):(\d{2})\s*[–-]\s*(\d{1,2}):(\d{2})\s*$`)
	span      = regexp.MustCompile(`^(\d{1,2}):(\d{2})-(\d{1,2}):(\d{2})$`)
	dayNames  = map[string]int{"Mo": 0, "Tu": 1, "We": 2, "Th": 3, "Fr": 4, "Sa": 5, "Su": 6}
)

// Doc reads an opening-hours string. False means unknown, not closed.
func Doc(s string) (Lich, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Lich{}, false
	}
	if s == "24/7" {
		return Lich{Khoang: []Khoang{{0, PhutTuan}}}, true
	}
	if m := catalogue.FindStringSubmatch(s); m != nil {
		tu, ok1 := phut(m[1], m[2])
		den, ok2 := phut(m[3], m[4])
		if !ok1 || !ok2 {
			return Lich{}, false
		}
		var all [7][]Khoang
		for d := 0; d < 7; d++ {
			all[d] = []Khoang{{tu, den}}
		}
		return tuNgay(all), true
	}
	return docOSM(s)
}

func docOSM(s string) (Lich, bool) {
	var days [7][]Khoang
	var set [7]bool
	for _, rule := range strings.Split(s, ";") {
		rule = strings.TrimSpace(rule)
		if rule == "" {
			continue
		}
		fields := strings.Fields(rule)
		chosen := [7]bool{true, true, true, true, true, true, true}
		if len(fields) == 2 {
			var ok bool
			if chosen, ok = docNgay(fields[0]); !ok {
				return Lich{}, false
			}
			fields = fields[1:]
		}
		if len(fields) != 1 {
			return Lich{}, false
		}
		var spans []Khoang
		if fields[0] != "off" {
			for _, part := range strings.Split(fields[0], ",") {
				m := span.FindStringSubmatch(part)
				if m == nil {
					return Lich{}, false
				}
				tu, ok1 := phut(m[1], m[2])
				den, ok2 := phut(m[3], m[4])
				if !ok1 || !ok2 {
					return Lich{}, false
				}
				spans = append(spans, Khoang{tu, den})
			}
		}
		for d := 0; d < 7; d++ {
			if chosen[d] {
				days[d] = spans
				set[d] = true
			}
		}
	}
	coLuat := false
	for _, v := range set {
		coLuat = coLuat || v
	}
	if !coLuat {
		return Lich{}, false
	}
	return tuNgay(days), true
}

// docNgay reads "Mo", "Mo-Fr", "Sa,Su", "Mo-We,Fr" and wrapping ranges "Fr-Mo".
func docNgay(s string) ([7]bool, bool) {
	var out [7]bool
	for _, part := range strings.Split(s, ",") {
		if a, b, ok := strings.Cut(part, "-"); ok {
			from, ok1 := dayNames[a]
			to, ok2 := dayNames[b]
			if !ok1 || !ok2 {
				return out, false
			}
			for d := from; ; d = (d + 1) % 7 {
				out[d] = true
				if d == to {
					break
				}
			}
			continue
		}
		d, ok := dayNames[part]
		if !ok {
			return out, false
		}
		out[d] = true
	}
	return out, true
}

// phut reads "HH","MM" into minutes of the day; 24:00 is allowed as an end.
func phut(h, m string) (int, bool) {
	var hh, mm int
	if _, err := fmt.Sscanf(h+":"+m, "%d:%d", &hh, &mm); err != nil {
		return 0, false
	}
	if mm < 0 || mm > 59 || hh < 0 || hh > 24 || (hh == 24 && mm != 0) {
		return 0, false
	}
	return hh*60 + mm, true
}

// tuNgay turns per-day spans into week minutes. A span whose end is at or
// before its start runs past midnight into the next day, and Sunday's runs
// into Monday.
func tuNgay(days [7][]Khoang) Lich {
	var out []Khoang
	for d, spans := range days {
		base := d * 24 * 60
		for _, k := range spans {
			tu, den := k.Tu, k.Den
			if den <= tu {
				den += 24 * 60
			}
			a, b := base+tu, base+den
			if b <= PhutTuan {
				out = append(out, Khoang{a, b})
				continue
			}
			out = append(out, Khoang{a, PhutTuan}, Khoang{0, b - PhutTuan})
		}
	}
	return Lich{Khoang: gop(out)}
}

func gop(ks []Khoang) []Khoang {
	if len(ks) == 0 {
		return nil
	}
	sort.Slice(ks, func(i, j int) bool { return ks[i].Tu < ks[j].Tu })
	out := []Khoang{ks[0]}
	for _, k := range ks[1:] {
		last := &out[len(out)-1]
		if k.Tu <= last.Den {
			if k.Den > last.Den {
				last.Den = k.Den
			}
			continue
		}
		out = append(out, k)
	}
	return out
}

// PhutCuaTuan is t's minute of the week in Vietnam.
func PhutCuaTuan(t time.Time) int {
	local := t.In(gioVN)
	day := (int(local.Weekday()) + 6) % 7 // Monday = 0
	return day*24*60 + local.Hour()*60 + local.Minute()
}

// MoLuc reports whether the place is open at minute m of the week.
func (l Lich) MoLuc(m int) bool {
	m = ((m % PhutTuan) + PhutTuan) % PhutTuan
	for _, k := range l.Khoang {
		if k.Tu <= m && m < k.Den {
			return true
		}
	}
	return false
}

// MoSuot reports whether the place is open for the whole of [tu, den), a span
// of at most a week given in minutes of the week (den may exceed PhutTuan when
// the span crosses Sunday night).
func (l Lich) MoSuot(tu, den int) bool {
	if den <= tu {
		return false
	}
	for m := tu; m < den; m++ {
		if !l.MoLuc(m) {
			return false
		}
	}
	return true
}

// SQL renders the schedule as a PostgreSQL int4multirange literal, for the
// retrieval index's hard filters.
func (l Lich) SQL() string {
	parts := make([]string, len(l.Khoang))
	for i, k := range l.Khoang {
		parts[i] = fmt.Sprintf("[%d,%d)", k.Tu, k.Den)
	}
	return "{" + strings.Join(parts, ",") + "}"
}

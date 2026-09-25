package giomo

import "fmt"

// Khung returns the half-open window [tu, den) as spans inside one week: a
// window that runs past Sunday night (den > PhutTuan, or den <= tu for a
// window given across the week's end) is split in two. Both ends are taken
// modulo the week first. An empty window (den == tu) has no span; one a week
// long or more is the whole week.
func Khung(tu, den int) []Khoang {
	if den == tu {
		return nil
	}
	if den-tu >= PhutTuan {
		return []Khoang{{0, PhutTuan}}
	}
	a := ((tu % PhutTuan) + PhutTuan) % PhutTuan
	b := ((den % PhutTuan) + PhutTuan) % PhutTuan
	if b == a {
		return []Khoang{{0, PhutTuan}}
	}
	if b > a {
		return []Khoang{{a, b}}
	}
	out := []Khoang{{a, PhutTuan}}
	if b > 0 {
		out = []Khoang{{0, b}, {a, PhutTuan}}
	}
	return out
}

// MoTrong reports whether the place is open at some minute of the window
// [tu, den): the oracle's open_within, «đang mở vào lúc nào đó trong khung».
func (l Lich) MoTrong(tu, den int) bool {
	for _, w := range Khung(tu, den) {
		for _, k := range l.Khoang {
			if k.Tu < w.Den && w.Tu < k.Den {
				return true
			}
		}
	}
	return false
}

// KhungSQL renders the window as a PostgreSQL int4multirange literal, for
// the retrieval index's `open_week && $khung` filter.
func KhungSQL(tu, den int) string {
	spans := Khung(tu, den)
	out := "{"
	for i, k := range spans {
		if i > 0 {
			out += ","
		}
		out += fmt.Sprintf("[%d,%d)", k.Tu, k.Den)
	}
	return out + "}"
}

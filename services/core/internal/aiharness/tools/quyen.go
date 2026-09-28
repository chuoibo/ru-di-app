package tools

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"

	"mobile/services/core/internal/aiharness/obs"
)

// quyenGolden is the per-bot permission table. It is a golden file: a change
// to it is a reviewed change of what each bot may call.
//
//go:embed testdata/quyen.golden.json
var quyenGolden []byte

// PhienBanQuyen is the table's format version this code reads.
const PhienBanQuyen = 1

// BotCap is the permission table's key for the room assistant in a chat of
// two (a pair, contexts.kind='pair'): the common tools only, none of the
// group's (no poll draft, no group snapshot, no group outings), none of
// Nếp's. It is a key of this table and nothing more: the turn itself is
// still the room assistant's (obs.BotNhom) in its metrics row, its prompt
// and its router, so no closed set outside this package grows.
const BotCap obs.Bot = "cap"

// botQuyen reports whether b is a key of the permission table.
func botQuyen(b obs.Bot) bool { return b.Valid() || b == BotCap }

// Quyen is a loaded permission table.
type Quyen struct {
	bot map[obs.Bot]map[Ten]bool
}

type quyenTho struct {
	PhienBan *int `json:"phien_ban"`
	CongCu   []struct {
		Ten  string   `json:"ten"`
		Lop  string   `json:"lop"`
		Pham string   `json:"pham"`
		Bot  []string `json:"bot"`
	} `json:"cong_cu"`
}

// ErrQuyen is what every refusal of NapQuyen wraps.
var ErrQuyen = errors.New("tools: permission table refused")

func loiQuyen(format string, a ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrQuyen}, a...)...)
}

// NapQuyen reads a permission table. It refuses: unknown fields or version;
// a tool outside the registry, missing from the table, or listed twice; a
// class or scope that differs from the registry's; an unknown or repeated
// bot; a me-scoped tool (memory, reminders, own outings, the screen card)
// granted to the group or the pair; a group-scoped tool granted to Nếp or
// the pair.
func NapQuyen(raw []byte) (*Quyen, error) {
	var t quyenTho
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&t); err != nil {
		return nil, loiQuyen("json: %v", err)
	}
	if t.PhienBan == nil || *t.PhienBan != PhienBanQuyen {
		return nil, loiQuyen("version")
	}
	q := &Quyen{bot: map[obs.Bot]map[Ten]bool{obs.BotNep: {}, obs.BotNhom: {}, BotCap: {}}}
	seen := map[Ten]bool{}
	for _, c := range t.CongCu {
		ten, err := Tens.Parse(c.Ten)
		if err != nil {
			return nil, loiQuyen("%v", err)
		}
		if seen[ten] {
			return nil, loiQuyen("%s listed twice", ten)
		}
		seen[ten] = true
		m, _ := Tra(ten)
		if string(m.Lop) != c.Lop || string(m.Pham) != c.Pham {
			return nil, loiQuyen("%s: table says %s/%s, registry %s/%s", ten, c.Lop, c.Pham, m.Lop, m.Pham)
		}
		botSeen := map[obs.Bot]bool{}
		for _, b := range c.Bot {
			bot := obs.Bot(b)
			if !botQuyen(bot) || botSeen[bot] {
				return nil, loiQuyen("%s: bot %q unknown or repeated", ten, b)
			}
			botSeen[bot] = true
			if (m.Pham == Me && bot != obs.BotNep) || (m.Pham == Nhom && bot != obs.BotNhom) {
				return nil, loiQuyen("%s: %s scope granted to %s", ten, m.Pham, bot)
			}
			if (m.Lop == TriNho || m.Lop == Nhac) && bot != obs.BotNep {
				return nil, loiQuyen("%s: %s class granted to %s", ten, m.Lop, bot)
			}
			q.bot[bot][ten] = true
		}
	}
	if len(seen) != Tens.Len() {
		return nil, loiQuyen("%d of %d tools listed", len(seen), Tens.Len())
	}
	return q, nil
}

// MacDinh is the table embedded from testdata/quyen.golden.json. A table that
// does not load stops the process at start, not at the first turn.
var MacDinh = func() *Quyen {
	q, err := NapQuyen(quyenGolden)
	if err != nil {
		panic(err)
	}
	return q
}()

// ChoPhep reports whether bot may call t.
func (q *Quyen) ChoPhep(bot obs.Bot, t Ten) bool { return q.bot[bot][t] }

// DuocPhep is bot's toolset in registry order. hanChe (the policy's
// proceed_restricted) keeps the read tools only: no draft, no memory write,
// no reminder.
func (q *Quyen) DuocPhep(bot obs.Bot, hanChe bool) []Ten {
	var out []Ten
	for _, m := range DangKy {
		if q.bot[bot][m.Ten] && (!hanChe || m.Lop == Doc) {
			out = append(out, m.Ten)
		}
	}
	return out
}

package tools

import "mobile/services/core/internal/huongdan"

// BanNhap is what the draft tools put into the answer. Nothing here touches a
// database: a places part, an itinerary or a poll becomes anything only when
// a human taps it (ADR-0044 §4, «nháp cần người bấm»). Ids are the real
// evidence ids, resolved by Go from the aliases the model wrote.
type BanNhap struct {
	// Quan are the places of the places part, in the model's order.
	Quan []string
	// LichTrinh is the itinerary draft; nil when none.
	LichTrinh *LichTrinh
	// BinhChon is the poll draft (group only); nil when none.
	BinhChon *BinhChon
	// Chip is the screen chip (Nếp only); nil when none.
	Chip *ChipMan
}

// LichTrinh is an itinerary draft.
type LichTrinh struct {
	// NgayISO is YYYY-MM-DD, or "" when the model gave no date.
	NgayISO string
	Chang   []Chang
}

// Chang is one stop: an evidence place id and an optional HH:MM.
type Chang struct {
	ID  string
	Gio string
}

// BinhChon is a poll draft. Its text is the model's, and is output to the
// room: it passes the output checks like any answer text.
type BinhChon struct {
	CauHoi  string
	LuaChon []string
}

// ChipMan is a screen the person may tap to open, with the taps that lead
// there from the screen they are on (empty when unknown).
type ChipMan struct {
	Man  string
	Buoc []huongdan.Buoc
}

func (n BanNhap) sao() BanNhap {
	out := BanNhap{Quan: append([]string(nil), n.Quan...)}
	if n.LichTrinh != nil {
		lt := LichTrinh{NgayISO: n.LichTrinh.NgayISO, Chang: append([]Chang(nil), n.LichTrinh.Chang...)}
		out.LichTrinh = &lt
	}
	if n.BinhChon != nil {
		bc := BinhChon{CauHoi: n.BinhChon.CauHoi, LuaChon: append([]string(nil), n.BinhChon.LuaChon...)}
		out.BinhChon = &bc
	}
	if n.Chip != nil {
		c := ChipMan{Man: n.Chip.Man, Buoc: append([]huongdan.Buoc(nil), n.Chip.Buoc...)}
		out.Chip = &c
	}
	return out
}

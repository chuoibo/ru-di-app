package vectordb

import (
	"errors"
	"slices"
	"strings"
	"time"

	"mobile/services/core/internal/aiharness/truyhoi"
	"mobile/services/core/internal/domain/giomo"
	"mobile/services/core/internal/domain/tuvung"
)

// SlotTuan is the half-hour slot of the week t falls in, on ViTri: Monday
// 00:00–00:30 is 0, Sunday 23:30–24:00 is SoSlotTuan-1. The ingest stores a
// place's open slots conservatively (a slot is in only when the place is open
// for all of it, rag/nap.MoO), and the Postgres re-check computes the same
// slots from the live hours (thuoctinh).
func SlotTuan(t time.Time) int16 {
	return int16(giomo.PhutCuaTuan(t) / PhutMoiSlot)
}

// SlotsTrong are the slots a window [tu, den) touches: a place open for the
// whole of one of them is open at some instant of the window, which is what
// truyhoi.Cung.MoTrong asks. A window crossing Sunday night wraps.
func SlotsTrong(k truyhoi.KhungMo) []int16 {
	tu := giomo.PhutCuaTuan(k.Tu)
	den := tu + int(k.Den.Sub(k.Tu)/time.Minute)
	var out []int16
	seen := map[int16]bool{}
	for m := tu - tu%PhutMoiSlot; m < den; m += PhutMoiSlot {
		s := int16((m / PhutMoiSlot) % SoSlotTuan)
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	slices.Sort(out)
	return out
}

// LocCung is a retrieval's hard constraints in the form the index filters
// on: ids, a slot and whole đồng, all from truyhoi.Cung (the model's
// extraction, checked by Go). No field is text a filter could be spliced
// from.
type LocCung struct {
	DiemDen string
	// DiUng is closed over the allergen families (tuvung.MoRongDiUng).
	DiUng   []string
	AnKieng []string
	Slot    *int16
	// Slots is a window: the place must be open for the whole of at least
	// one of them (SlotsTrong).
	Slots []int16
	// NganSachVND is the ceiling per person; a place qualifies when its
	// known minimum price is at most this.
	NganSachVND *int64
	// DanhMuc are category ids (tuvung.DanhMuc, checked by LocDanhMuc): a
	// place qualifies when its categories hold ANY of them; a place whose
	// categories nobody established ([KhongRo]) never does.
	DanhMuc []string
}

// ErrLoc: a constraint value outside its vocabulary or range.
var ErrLoc = errors.New("vectordb: hard constraint outside its vocabulary")

// TuCung prepares a truyhoi.Cung for the index: allergens closed over their
// family, diets and allergens checked against the closed vocabularies, the
// open instant turned into its slot and the open window into the slots it
// touches. An unknown id is refused, never dropped: dropping one would widen
// a hard constraint.
//
// Unknown attributes under a hard constraint (docs/architecture/03 §8.4):
// a place whose allergens nobody established (KhongRo) is out for any
// allergy; a place with no known price is out under a budget; a place with
// no known hours is out under an open instant or window. Without that
// constraint the same place is kept, and the evidence says «chưa rõ».
func TuCung(c truyhoi.Cung) (LocCung, error) {
	l := LocCung{DiemDen: c.DiemDenID, NganSachVND: c.NganSachVND}
	for _, id := range c.DiUng {
		if !tuvung.DiUng.Co(id) {
			return LocCung{}, ErrLoc
		}
	}
	for _, id := range c.AnKieng {
		if !tuvung.AnKieng.Co(id) {
			return LocCung{}, ErrLoc
		}
	}
	if len(c.DiUng) > 0 {
		l.DiUng = tuvung.MoRongDiUng(c.DiUng)
	}
	if len(c.AnKieng) > 0 {
		l.AnKieng = slices.Clone(c.AnKieng)
		slices.Sort(l.AnKieng)
		l.AnKieng = slices.Compact(l.AnKieng)
	}
	if c.MoLuc != nil {
		s := SlotTuan(*c.MoLuc)
		l.Slot = &s
	}
	if c.MoTrong != nil {
		l.Slots = SlotsTrong(*c.MoTrong)
	}
	if c.NganSachVND != nil && *c.NganSachVND < 0 {
		return LocCung{}, ErrLoc
	}
	return l, nil
}

// LocDanhMuc checks category ids for LocCung.DanhMuc: each must be a
// tuvung.DanhMuc id (khong_ro is not one), and the result is sorted and
// deduplicated. An unknown id is refused, never dropped.
func LocDanhMuc(ids []string) ([]string, error) {
	for _, id := range ids {
		if !tuvung.DanhMuc.Co(id) {
			return nil, ErrLoc
		}
	}
	if len(ids) == 0 {
		return nil, nil
	}
	out := slices.Clone(ids)
	slices.Sort(out)
	return slices.Compact(out), nil
}

// menhDe is one hard constraint as the index filters it: the expression that
// holds for a place satisfying it, built from constant text and named
// template parameters only.
type menhDe struct {
	rb     truyhoi.RangBuoc
	bieuTh string
	thamSo map[string]any
}

// menhDes are l's set constraints, in a fixed order.
func (l LocCung) menhDes() []menhDe {
	var out []menhDe
	if l.DiemDen != "" {
		out = append(out, menhDe{truyhoi.RBDiemDen, FDestination + " == {dd}", map[string]any{"dd": l.DiemDen}})
	}
	if len(l.DiUng) > 0 {
		// Fail closed: a place whose allergens nobody could establish is
		// out for anyone with an allergy.
		out = append(out, menhDe{truyhoi.RBDiUng,
			"not ARRAY_CONTAINS_ANY(" + FAllergens + ", {du}) and not ARRAY_CONTAINS(" + FAllergens + ", {kr})",
			map[string]any{"du": slices.Clone(l.DiUng), "kr": KhongRo}})
	}
	if len(l.AnKieng) > 0 {
		out = append(out, menhDe{truyhoi.RBAnKieng, "ARRAY_CONTAINS_ALL(" + FDiets + ", {ak})",
			map[string]any{"ak": slices.Clone(l.AnKieng)}})
	}
	if l.Slot != nil {
		out = append(out, menhDe{truyhoi.RBMoLuc, "ARRAY_CONTAINS(" + FOpenSlots + ", {sl})",
			map[string]any{"sl": int64(*l.Slot)}})
	}
	if len(l.Slots) > 0 {
		ks := make([]int64, len(l.Slots))
		for i, s := range l.Slots {
			ks[i] = int64(s)
		}
		out = append(out, menhDe{truyhoi.RBMoLuc, "ARRAY_CONTAINS_ANY(" + FOpenSlots + ", {kh})",
			map[string]any{"kh": ks}})
	}
	if l.NganSachVND != nil {
		out = append(out, menhDe{truyhoi.RBNganSach, FPriceMin + " >= {g0} and " + FPriceMin + " <= {ns}",
			map[string]any{"g0": int64(0), "ns": *l.NganSachVND}})
	}
	if len(l.DanhMuc) > 0 {
		// Fail closed: a place whose categories nobody established is
		// out whatever was asked.
		out = append(out, menhDe{truyhoi.RBDanhMuc,
			"ARRAY_CONTAINS_ANY(" + FDanhMuc + ", {dm}) and not ARRAY_CONTAINS(" + FDanhMuc + ", {dk})",
			map[string]any{"dm": slices.Clone(l.DanhMuc), "dk": KhongRo}})
	}
	return out
}

// BieuThuc is the filter of a place search under l: live rows (not
// tombstoned) satisfying every set constraint, and its template parameters.
func (l LocCung) BieuThuc() (string, map[string]any) {
	parts := []string{FTombstoned + " == {tb}"}
	params := map[string]any{"tb": false}
	for _, m := range l.menhDes() {
		parts = append(parts, "("+m.bieuTh+")")
		for k, v := range m.thamSo {
			params[k] = v
		}
	}
	return strings.Join(parts, " and "), params
}

// BieuThucViPham is, for each set constraint but the destination, the filter
// of the live rows of l's destination that break it: counted to tell the
// model how many places each hard constraint removed (truyhoi.BiLoai). An
// instant and a window both set are two clauses under the one name
// RBMoLuc; they are joined so the count is of rows breaking either.
func (l LocCung) BieuThucViPham() map[truyhoi.RangBuoc]struct {
	BieuThuc string
	ThamSo   map[string]any
} {
	out := map[truyhoi.RangBuoc]struct {
		BieuThuc string
		ThamSo   map[string]any
	}{}
	base := FTombstoned + " == {tb}"
	baseParams := map[string]any{"tb": false}
	ms := l.menhDes()
	for _, m := range ms {
		if m.rb == truyhoi.RBDiemDen {
			base += " and (" + m.bieuTh + ")"
			for k, v := range m.thamSo {
				baseParams[k] = v
			}
		}
	}
	type gop struct {
		bieuTh []string
		thamSo map[string]any
	}
	byRB := map[truyhoi.RangBuoc]*gop{}
	var order []truyhoi.RangBuoc
	for _, m := range ms {
		if m.rb == truyhoi.RBDiemDen {
			continue
		}
		g := byRB[m.rb]
		if g == nil {
			g = &gop{thamSo: map[string]any{}}
			byRB[m.rb] = g
			order = append(order, m.rb)
		}
		g.bieuTh = append(g.bieuTh, "("+m.bieuTh+")")
		for k, v := range m.thamSo {
			g.thamSo[k] = v
		}
	}
	for _, rb := range order {
		g := byRB[rb]
		p := map[string]any{}
		for k, v := range baseParams {
			p[k] = v
		}
		for k, v := range g.thamSo {
			p[k] = v
		}
		out[rb] = struct {
			BieuThuc string
			ThamSo   map[string]any
		}{base + " and not (" + strings.Join(g.bieuTh, " and ") + ")", p}
	}
	return out
}

// ThuocTinh are a place's hard-constraint attributes, as the index stores
// them and as the Postgres re-check reads them from the live rows.
type ThuocTinh struct {
	DiemDen   string
	DiUng     []string
	AnKieng   []string
	OSlots    []int16
	GiaMinVND int64 // GiaKhongRo when unknown
	// DanhMuc are the place's categories (tuvung.DanhMuc ids); empty or
	// [KhongRo] when unknown.
	DanhMuc []string
	GoBo    bool
}

// Dat reports whether t satisfies l, and when not the first constraint it
// breaks (in menhDes order). This is the Go reading of the same rule the
// expression states; the fake and the Postgres re-check use it, and the live
// tier checks the expression agrees with it on every hit.
func (l LocCung) Dat(t ThuocTinh) (bool, truyhoi.RangBuoc) {
	if t.GoBo {
		return false, ""
	}
	if l.DiemDen != "" && t.DiemDen != l.DiemDen {
		return false, truyhoi.RBDiemDen
	}
	if len(l.DiUng) > 0 {
		for _, a := range t.DiUng {
			if a == KhongRo || slices.Contains(l.DiUng, a) {
				return false, truyhoi.RBDiUng
			}
		}
	}
	for _, d := range l.AnKieng {
		if !slices.Contains(t.AnKieng, d) {
			return false, truyhoi.RBAnKieng
		}
	}
	if l.Slot != nil && !slices.Contains(t.OSlots, *l.Slot) {
		return false, truyhoi.RBMoLuc
	}
	if len(l.Slots) > 0 && !slices.ContainsFunc(l.Slots, func(s int16) bool { return slices.Contains(t.OSlots, s) }) {
		return false, truyhoi.RBMoLuc
	}
	if l.NganSachVND != nil && (t.GiaMinVND < 0 || t.GiaMinVND > *l.NganSachVND) {
		return false, truyhoi.RBNganSach
	}
	if len(l.DanhMuc) > 0 && (slices.Contains(t.DanhMuc, KhongRo) ||
		!slices.ContainsFunc(l.DanhMuc, func(id string) bool { return slices.Contains(t.DanhMuc, id) })) {
		return false, truyhoi.RBDanhMuc
	}
	return true, ""
}

// ViTri is the zone every open slot is computed in (giomo's clock). The
// catalogue is Vietnamese; a destination on another offset would need its
// own zone column, which none has yet.
var ViTri = mustZone("Asia/Ho_Chi_Minh")

func mustZone(name string) *time.Location {
	l, err := time.LoadLocation(name)
	if err != nil {
		return time.FixedZone(name, 7*3600)
	}
	return l
}

package tools

import (
	"fmt"
	"strings"
)

// The taint invariant of the agent loop (SOTA gap #10; CaMeL and the
// design-patterns paper: once the agent has read untrusted data, that data
// must not decide the next action's arguments). It is deterministic
// bookkeeping over the arguments' SHAPE and set membership, never a
// reading of what a text means, and costs no model call.
//
// From the second agent step on, every argument of a tool call must be one
// of:
//   - a value the router extracted from the person's own words this turn:
//     a slot (destination, date, time, budget, allergens, diets, kinds of
//     place, vibes) or one of its search texts (either form);
//   - a free text the model itself wrote on the first step, before any tool
//     result was read;
//   - an id present in the turn's ledger: an evidence alias the turn was
//     shown (resolved by each tool's own check), or a catalogue row id a
//     tool returned this turn (destinations, areas);
//   - a bounded count (k) or a closed enum the router has no slot for
//     (list_group_outings' khi, remember_fact's loai and phan_loai): values
//     no data can smuggle text through.
//
// The hard constraints are re-checked on the arguments: a date, a time or a
// budget must be the router's own, and a list (allergens, diets, kinds,
// vibes) may only repeat the router's items. A draft tool (propose_*,
// draft_poll, suggest_screen) builds a suggestion the person taps; its ids
// are ledger aliases (its own check) and its date must be the router's
// when the router set one, while its free content (a poll's words, a
// stop's time) is the draft itself and reaches no data source.
//
// Step 1 and the fast path (step 0, the router's own dispatch) are not
// checked here: nothing untrusted has been read by the model yet beyond
// what the turn laid in its prompt as data.

// DatBuoc records that agent step n has started (agent.CauHinh.BatDauBuoc):
// the tool calls it returns belong to it.
func (bc *BoiCanh) DatBuoc(n int) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	bc.buoc = n
}

// TenChoPhep names the tools the model may call on the step about to run:
// this turn's toolset (DuocPhep) minus what the turn's state already
// refuses -- nothing once the answer is due, and no memory write or
// reminder once a tool has returned data (forget_fact alone survives
// remembered facts). It is the step's AllowedFunctionNames; TruocTool
// enforces the same rules on whatever the model returns.
func (bc *BoiCanh) TenChoPhep() []string {
	bc.mu.Lock()
	cuoi, ngoai, nho := bc.cuoi || bc.epCuoi, bc.ngoai, bc.nho
	bc.mu.Unlock()
	if cuoi {
		return []string{}
	}
	out := []string{}
	for _, t := range bc.DuocPhep() {
		if m, _ := Tra(t); (m.Lop == TriNho || m.Lop == Nhac) && (ngoai || (nho && t != ForgetFact)) {
			continue
		}
		out = append(out, string(t))
	}
	return out
}

// chuanChu is a free text as the invariant compares it: runs of white space
// as one space, nothing else touched. Two texts are the same text or not;
// no word of either is read.
func chuanChu(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// chuTuDo are the argument names that carry free text, per tool.
var chuTuDo = map[string]bool{"truy_van": true, "mo_ta": true, "noi_dung": true}

// laNhap: a draft tool, whose free content is the draft itself.
func laNhap(t Ten) bool {
	m, _ := Tra(t)
	return m.Lop == Nhap
}

// chuTinCay reports whether s is a text the router wrote this turn (either
// form of a query) or one the model wrote on the first step.
func (bc *BoiCanh) chuTinCay(s string) bool {
	c := chuanChu(s)
	for _, q := range bc.TruyVan {
		if c == chuanChu(q.Cau) || (q.CauCoDau != "" && c == chuanChu(q.CauCoDau)) {
			return true
		}
	}
	bc.mu.Lock()
	defer bc.mu.Unlock()
	for _, d := range bc.chuBuocDau {
		if c == d {
			return true
		}
	}
	return false
}

// CoDauCua is the router's diacritics-restored form of search text s, or ""
// when s is none of the router's queries (the model's own text on the
// first step).
func (bc *BoiCanh) CoDauCua(s string) string {
	c := chuanChu(s)
	for _, q := range bc.TruyVan {
		if c == chuanChu(q.Cau) || (q.CauCoDau != "" && c == chuanChu(q.CauCoDau)) {
			return q.CauCoDau
		}
	}
	return ""
}

func taint(truong string) *loiTS {
	return thamSoSai(truong, "after a tool result, this must be a value the person stated (already applied), a search text you wrote before reading any result or one from the truy_van block, or an alias or id you were shown")
}

// kiemTaint checks a call's arguments (JSON-decoded, banSao) against the
// invariant above. nil lets the call through.
func (bc *BoiCanh) kiemTaint(t Ten, args map[string]any) *loiTS {
	bc.mu.Lock()
	buoc := bc.buoc
	bc.mu.Unlock()
	if buoc <= 1 {
		if buoc == 1 {
			bc.ghiChuBuocDau(args)
		}
		return nil
	}
	s := bc.Slots
	for name, v := range args {
		switch name {
		case "k", "khi", "loai", "phan_loai", "ids", "chang", "id", "outing_id", "man":
			// Counts, closed enums with no router slot, and ledger aliases
			// (each tool's own check resolves them against the ledger).
			continue
		case "cau_hoi", "lua_chon":
			if laNhap(t) {
				continue
			}
			return taint(name)
		case "diem_den_id":
			id, _ := v.(string)
			if id != s.DiemDenID && !bc.laIDHang(id) {
				return taint(name)
			}
		case "khu_vuc":
			id, _ := v.(string)
			if !bc.laIDHang(id) {
				return taint(name)
			}
		case "ngay_iso", "tu_ngay", "den_ngay":
			d, _ := v.(string)
			if laNhap(t) && s.NgayISO == "" {
				continue
			}
			if d != s.NgayISO {
				return taint(name)
			}
		case "gio":
			g, _ := v.(string)
			if s.KhungGio == nil || (g != s.KhungGio.Tu && g != s.KhungGio.Den) {
				return taint(name)
			}
		case "ngan_sach_vnd":
			n, ok := v.(float64)
			if !ok || s.NganSachVND == nil || n != float64(*s.NganSachVND) {
				return taint(name)
			}
		case "di_ung":
			if !tapCon(v, s.DiUng) {
				return taint(name)
			}
		case "an_kieng":
			if !tapCon(v, s.AnKieng) {
				return taint(name)
			}
		case "loai_cho":
			if !tapCon(v, s.LoaiCho) {
				return taint(name)
			}
		case "khi_chat":
			if !tapCon(v, s.KhiChat) {
				return taint(name)
			}
		default:
			if chuTuDo[name] {
				c, _ := v.(string)
				if !bc.chuTinCay(c) {
					return taint(name)
				}
				continue
			}
			// An argument this invariant does not know: refused, so a new
			// argument cannot slip past it unreviewed (TestTaintBietMoiThamSo).
			return &loiTS{loi: ThamSoSai, truong: name, yeuCau: fmt.Sprintf("argument %q is not allowed after a tool result", name), sua: true}
		}
	}
	return nil
}

// ghiChuBuocDau remembers the free texts of a first-step call.
func (bc *BoiCanh) ghiChuBuocDau(args map[string]any) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	for name, v := range args {
		if c, ok := v.(string); ok && chuTuDo[name] {
			bc.chuBuocDau = append(bc.chuBuocDau, chuanChu(c))
		}
	}
}

func (bc *BoiCanh) laIDHang(id string) bool {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	return id != "" && bc.idHang[id]
}

// tapCon: v (a JSON array of strings) holds only items of tap.
func tapCon(v any, tap []string) bool {
	xs, ok := v.([]any)
	if !ok {
		return false
	}
	for _, x := range xs {
		s, _ := x.(string)
		if !coTrong(tap, s) {
			return false
		}
	}
	return true
}

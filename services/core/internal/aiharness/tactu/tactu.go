// Package tactu runs the tool part of a turn once the router (hieu) has
// decided its path: either the FAST PATH, when the router's own output
// names exactly one of find_places, app_help or explain_screen on the
// one-step path with the slots that tool needs (Go dispatches that one tool
// directly, then makes one answer call), or the bounded ADK AGENT LOOP with
// the bot's permitted toolset, where the MODEL decides whether and which
// tool to call (function calling AUTO on every step but the last, NONE on
// the last).
//
// Nothing here reads the person's words. The fast path is a lookup on the
// router's closed labels; the loop's toolset is the permission table; the
// budgets are counters (llm/ngansach.go). Earlier turns reach the prompt
// only inside a <du_lieu nguon="lich_su"> block, with earlier evidence as
// t1, t2, … and never as ids.
package tactu

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"

	"google.golang.org/adk/v2/model"

	"mobile/services/core/internal/aiharness/agent"
	"mobile/services/core/internal/aiharness/hieu"
	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/obs"
	"mobile/services/core/internal/aiharness/prompts"
	"mobile/services/core/internal/aiharness/tools"
	"mobile/services/core/internal/aiharness/trinho"
	"mobile/services/core/internal/aiharness/truyhoi"
)

// Vao is one turn's input to the tool part.
type Vao struct {
	// Ten is the agent's name; Instruction the bot's system instruction
	// (the tool clause is appended here).
	Ten         string
	Instruction string
	NhietDo     float32
	MaxTokens   int32
	// Router is the router's checked output for this turn.
	Router hieu.KetQua
	// Cau is the person's message after structural preprocessing; it goes
	// to the model inside <du_lieu nguon="cau_hoi">.
	Cau string
	// KhoiThem are further data blocks the engine built (the screen card,
	// the server's «now» line), already wrapped (prompts.BocDuLieu, or
	// BocDuLieuDanhDau for text from outside).
	KhoiThem []string
	// NganHan and Phien are the short-term memory of the session (Nếp's
	// panel, or the group's reply chain); nil NganHan means none.
	NganHan trinho.NganHan
	Phien   string
	// BoiCanh is the turn's tool context, with the router's constraints
	// already set (tools.RangBuocTuRouter).
	BoiCanh *tools.BoiCanh
	// DuTru is how many model calls the caller keeps for after the tool
	// part (the verifier): the loop sees that many fewer, so its last step
	// answers with function calling off while the reserve still stands.
	DuTru int
}

// Ra is what the tool part produced.
type Ra struct {
	Text string
	// Nhanh is true when the fast path answered.
	Nhanh bool
	// CongCuNhanh is the tool the fast path dispatched.
	CongCuNhanh tools.Ten
	Nhap        tools.BanNhap
	// TrichDan are the evidence ids this answer put forward (the drafts'
	// places, else the places it was shown), what the next turn's
	// short-term memory offers as t1, t2, …
	TrichDan []string
}

// MaxBuoc is bot's agent step ceiling.
func MaxBuoc(bot obs.Bot) int {
	if bot == obs.BotNep {
		return llm.MaxStepsNep
	}
	return llm.MaxStepsNhom
}

// ErrVao: a structurally unusable input (no context, unknown bot).
var ErrVao = errors.New("tactu: invalid input")

// Nhanh returns the tool and arguments of the fast path when the router's
// output qualifies: the one-step path, exactly one intent, that intent one
// of find_places, app_help or explain_screen, and the slots the tool needs
// (a places query and a destination; a manual query; the screen card). The
// hard constraints are not passed as arguments: the tool merges the
// router's own into every search.
func Nhanh(kq hieu.KetQua, bc *tools.BoiCanh) (tools.Ten, map[string]any, bool) {
	if kq.Huong != hieu.TruyHoiMotBuoc || len(kq.YDinh) != 1 || kq.Tien != hieu.TienNone || kq.NhanGuard != hieu.Sach {
		return "", nil, false
	}
	truyVan := func(n truyhoi.Nguon) (string, bool) {
		for _, t := range kq.TruyVan {
			if t.Nguon == n {
				return t.Cau, true
			}
		}
		return "", false
	}
	switch kq.YDinh[0] {
	case hieu.FindPlaces:
		q, ok := truyVan(truyhoi.Places)
		if !ok || bc.Cung.DiemDenID == "" {
			return "", nil, false
		}
		return tools.SearchPlaces, map[string]any{"truy_van": q}, true
	case hieu.AppHelp:
		q, ok := truyVan(truyhoi.Manual)
		if !ok {
			return "", nil, false
		}
		return tools.SearchAppManual, map[string]any{"truy_van": q}, true
	case hieu.ExplainScreen:
		if bc.Man == "" {
			return "", nil, false
		}
		return tools.ExplainScreen, map[string]any{}, true
	}
	return "", nil, false
}

// Chay runs the tool part of one turn through dem; td counts its steps and
// tokens.
func Chay(ctx context.Context, dem *llm.Dem, v Vao, td *agent.TheoDoi) (Ra, error) {
	bc := v.BoiCanh
	if bc == nil || !bc.Bot.Valid() {
		return Ra{}, ErrVao
	}
	var luot []trinho.Luot
	if v.NganHan != nil && v.Phien != "" {
		var err error
		if luot, err = v.NganHan.Doc(ctx, v.Phien); err != nil {
			return Ra{}, err
		}
	}
	// Earlier evidence is offered as t1, t2, … in first-cited order.
	bc.ThamChieu = nil
	for _, l := range luot {
		for _, id := range l.BangChungIDs {
			if id != "" && !chua(bc.ThamChieu, id) {
				bc.ThamChieu = append(bc.ThamChieu, id)
			}
		}
	}
	var blocks []string
	if s := KhoiLichSu(luot, bc.BiDanhThamChieu()); s != "" {
		blocks = append(blocks, s)
	}
	blocks = append(blocks, v.KhoiThem...)

	cfg := agent.CauHinh{
		Ten:             v.Ten,
		Instruction:     v.Instruction + "\n\n" + prompts.CongCu(),
		NhietDo:         v.NhietDo,
		MaxOutputTokens: v.MaxTokens,
		ConLai:          func() int { return dem.ConLai() - v.DuTru },
	}
	ra := Ra{}
	var m model.LLM = dem
	if t, args, ok := Nhanh(v.Router, bc); ok {
		// Fast path: the model decided (the router); Go dispatches the one
		// tool through the same checks and ledger, then ONE answer call
		// with function calling off.
		kq := bc.Goi(ctx, t, args)
		khoi, err := khoiKetQua(t, kq)
		if err != nil {
			return Ra{}, err
		}
		blocks = append(blocks, khoi...)
		cfg.MaxBuoc = 1
		ra.Nhanh, ra.CongCuNhanh = true, t
	} else {
		ts, err := bc.BoCongCu()
		if err != nil {
			return Ra{}, err
		}
		cfg.MaxBuoc = MaxBuoc(bc.Bot)
		cfg.Tools = ts
		cfg.TruocTool, cfg.SauTool, cfg.LoiTool = bc.TruocTool, bc.SauTool, bc.LoiTool
		cfg.EpTraLoi = bc.EpTraLoi
		cfg.BuocCuoi = bc.DatBuocCuoi
	}
	blocks = append(blocks, prompts.BocDuLieuDanhDau(prompts.CauHoi, v.Cau))
	text, err := agent.Chay(ctx, m, cfg, nil, strings.Join(blocks, "\n\n"), td)
	if err != nil {
		return Ra{}, err
	}
	ra.Text = text
	ra.Nhap = bc.Nhap()
	ra.TrichDan = trichDan(bc, ra.Nhap)
	return ra, nil
}

// khoiKetQua lays a directly dispatched tool's answer into the prompt the
// way the model reads it in the loop: our structured fields (counts, codes,
// the applied constraints) in one block, and the tool's own data block,
// already wrapped by the tool, as it is.
func khoiKetQua(t tools.Ten, kq map[string]any) ([]string, error) {
	meta := map[string]any{}
	for k, v := range kq {
		if k != "du_lieu" {
			meta[k] = v
		}
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(meta); err != nil {
		return nil, err
	}
	out := []string{prompts.BocDuLieu(prompts.KetQuaCongCu, string(t)+" "+strings.TrimSpace(buf.String()))}
	if d, ok := kq["du_lieu"].(string); ok {
		out = append(out, d)
	}
	return out, nil
}

// trichDan is what the answer put forward: the drafts' places when there
// are drafts, else every place of the ledger, capped at tools.MaxDeXuat.
func trichDan(bc *tools.BoiCanh, n tools.BanNhap) []string {
	var out []string
	add := func(id string) {
		if !chua(out, id) && len(out) < tools.MaxDeXuat {
			out = append(out, id)
		}
	}
	for _, id := range n.Quan {
		add(id)
	}
	if n.LichTrinh != nil {
		for _, c := range n.LichTrinh.Chang {
			add(c.ID)
		}
	}
	if len(out) > 0 {
		return out
	}
	for _, id := range bc.SoCai.IDs() {
		if b, _, _ := bc.SoCai.Lay(id); b.Nguon == truyhoi.Places {
			add(id)
		}
	}
	return out
}

// KhoiLichSu renders the short-term turns as ONE data block, oldest first,
// one line per turn: the speaker's closed label, the text, and the aliases
// of the evidence it cited. Empty when there are no turns.
func KhoiLichSu(luot []trinho.Luot, biDanh map[string]string) string {
	if len(luot) == 0 {
		return ""
	}
	var lines []string
	for _, l := range luot {
		line := string(l.Vai) + ": " + strings.Join(strings.Fields(l.Chu), " ")
		var bs []string
		for _, id := range l.BangChungIDs {
			if b, ok := biDanh[id]; ok && !chua(bs, b) {
				bs = append(bs, b)
			}
		}
		if len(bs) > 0 {
			line += " [" + strings.Join(bs, ", ") + "]"
		}
		lines = append(lines, line)
	}
	return prompts.BocDuLieuDanhDau(prompts.LichSu, strings.Join(lines, "\n"))
}

// GhiLuot appends the person's message and the released answer to the
// session's short-term memory. The engine calls it only after the answer
// passed every output check: a withheld answer is not remembered.
func GhiLuot(ctx context.Context, n trinho.NganHan, phien string, v Vao, ra Ra) error {
	if n == nil || phien == "" {
		return nil
	}
	luc := v.BoiCanh.Luc
	if err := n.Them(ctx, phien, trinho.Luot{Vai: trinho.Toi, Chu: v.Cau, Luc: luc}); err != nil {
		return err
	}
	return n.Them(ctx, phien, trinho.Luot{Vai: trinho.TroLy, Chu: ra.Text, Luc: luc, BangChungIDs: ra.TrichDan})
}

func chua(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

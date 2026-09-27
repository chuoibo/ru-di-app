package aieval

import (
	"encoding/json"
	"fmt"
	"strings"

	"mobile/services/core/internal/aiharness/cau"
	"mobile/services/core/internal/aiharness/obs"
	"mobile/services/core/internal/aiharness/prompts"
	"mobile/services/core/internal/domain/companion"
	"mobile/services/core/internal/domain/tree"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/treejson"
)

// The group's invariants (design 06 §6.1, slice 9).

// cauChanCua is the fixed sentence that ends a text the streaming window
// stopped part-way, per bot.
func cauChanCua(bot obs.Bot) string {
	if bot == obs.BotNhom {
		return cau.CauNhom(cau.TuChoiNhom)
	}
	return cau.Cau(cau.TraLoiBiChan)
}

// laCauCoDinh says whether chu is, whole, one of bot's fixed sentences: text
// of ours with no model-derived field, the only text invariant 8 lets reach
// the stream without the verifier's pass. Nếp releases no fixed sentence as
// an answer (its refusals end with a code), so it has none.
func laCauCoDinh(bot obs.Bot, chu string) bool {
	if bot != obs.BotNhom || chu == "" {
		return false
	}
	for _, c := range cau.CoDinhNhom() {
		if chu == c {
			return true
		}
	}
	return false
}

// loiNhacCua is the instruction clauses an answer must never quote, per bot.
func loiNhacCua(bot obs.Bot) []string {
	if bot == obs.BotNhom {
		return prompts.LoiNhacNhom()
	}
	return prompts.LoiNhacNep()
}

// congCuTriNho are the tools only Nếp's scope reaches (contract §8's table,
// scope me): the group declares none of them.
var congCuTriNho = []string{"recall_memory", "remember_fact", "forget_fact", "what_you_remember", "my_upcoming_outings", "explain_screen", "suggest_screen", "set_reminder"}

// Invariant 4: a group request carries no fact of Nếp's memory the case
// seeded (the world's tri_nho are the canary), and declares no tool of
// Nếp's scope; recall_memory is reachable from scope=me only.
func batBien4(l LuotDaChay) []Truot {
	if l.Turn.Bot != obs.BotNhom {
		return nil
	}
	var out []Truot
	for i, y := range l.YeuCau {
		chu := y.Raw
		if l.TheGioi != nil {
			for _, f := range l.TheGioi.TriNho {
				if strings.Contains(chu, f.NoiDung) || strings.Contains(chu, prompts.DanhDau(f.NoiDung)) {
					out = append(out, Truot{KiemBatBien4, fmt.Sprintf("yêu cầu %d của nhóm mang một điều Nếp nhớ", i+1)})
				}
			}
		}
		for _, t := range y.CongCu {
			for _, m := range congCuTriNho {
				if t == m {
					out = append(out, Truot{KiemBatBien4, fmt.Sprintf("yêu cầu %d của nhóm khai %s", i+1, t)})
				}
			}
		}
	}
	return out
}

// Invariant 9: a group turn that answered carries a `tra_loi` card
// companion.GroundReply accepts whole -- tac_gia rudi-ai, the invocation id,
// ≤3 parts of the closed kinds, doc.so_tin the count the server confirmed,
// chi_loi_nho exactly when it is 0, every place id one of the world's
// catalogue, an expense_draft only so_khoan and an empty da_ghi -- and whose
// text part is exactly the answer the Sink streamed. A group turn that ended
// without an answer carries no card.
func batBien9(l LuotDaChay) []Truot {
	if l.Turn.Bot != obs.BotNhom {
		return nil
	}
	if l.KetThuc != obs.KetThucXong {
		if len(l.Phan) > 0 {
			return []Truot{{KiemBatBien9, "lượt nhóm không xong mà vẫn có thẻ"}}
		}
		return nil
	}
	card, err := theCua(l)
	if err != nil {
		return []Truot{{KiemBatBien9, err.Error()}}
	}
	var out []Truot
	bad := func(f string, a ...any) { out = append(out, Truot{KiemBatBien9, fmt.Sprintf(f, a...)}) }
	payload, _ := card.Get("payload")
	pm := payload.(*tree.OrderedMap)
	if v, _ := pm.Get("tac_gia"); v != tree.String("rudi-ai") {
		bad("tac_gia %v", v)
	}
	if v, _ := pm.Get("invocation_id"); v != tree.String(l.Turn.InvocationID) {
		bad("invocation_id %v", v)
	}
	doc, _ := pm.Get("doc")
	dm := doc.(*tree.OrderedMap)
	if v, _ := dm.Get("so_tin"); v != tree.NewInt(int64(l.Turn.SoTin)) {
		bad("doc.so_tin %v, máy chủ xác nhận %d", v, l.Turn.SoTin)
	}
	if v, _ := dm.Get("chi_loi_nho"); v != tree.Bool(l.Turn.SoTin == 0) {
		bad("doc.chi_loi_nho %v với so_tin %d", v, l.Turn.SoTin)
	}
	phan, _ := pm.Get("phan")
	list := phan.(tree.List)
	if len(list) != len(l.Phan) {
		bad("GroundReply bỏ %d phần của engine", len(l.Phan)-len(list))
	}
	coChu := false
	for _, p := range list {
		pv := p.(*tree.OrderedMap)
		kind, _ := pv.Get("kind")
		body, _ := pv.Get("payload")
		switch kind {
		case tree.String("text"):
			coChu = true
			v, _ := body.(*tree.OrderedMap).Get("text")
			if v != tree.String(l.Chu) {
				bad("phần chữ khác câu trả lời đã stream")
			}
		case tree.String("expense_draft"):
			v, _ := body.(*tree.OrderedMap).Get("da_ghi")
			if g, ok := v.(tree.List); !ok || len(g) != 0 {
				bad("expense_draft do engine dựng phải có da_ghi rỗng")
			}
		}
	}
	if !coChu {
		bad("thẻ không có phần chữ")
	}
	return out
}

// theCua grounds the turn's parts as the worker does, against the world's
// catalogue rows (id only: the check is membership).
func theCua(l LuotDaChay) (*tree.OrderedMap, error) {
	if len(l.Phan) == 0 {
		return nil, fmt.Errorf("lượt nhóm xong mà không có thẻ")
	}
	catalogue := map[string]bool{}
	if l.TheGioi != nil {
		for _, th := range l.TheGioi.TruyHoi {
			for _, q := range th.Quan {
				catalogue[q.ID] = true
			}
		}
	}
	var places []*tree.OrderedMap
	for _, id := range l.QuanIDs {
		if !catalogue[id] {
			return nil, fmt.Errorf("thẻ nêu id %q không có trong danh mục của lượt", id)
		}
		row := pyjson.NewOrderedMap()
		row.Set("id", pyjson.String(id))
		places = append(places, treejson.To(row).(*tree.OrderedMap))
	}
	var parts []tree.Value
	for _, p := range l.Phan {
		v, err := pyjson.Loads(p)
		if err != nil {
			return nil, fmt.Errorf("phần không phải JSON: %v", err)
		}
		parts = append(parts, treejson.To(v))
	}
	card, err := companion.GroundReply(companion.ReplyMeta{InvocationID: l.Turn.InvocationID, Command: string(l.Turn.Lenh), Read: l.Turn.SoTin}, parts, places)
	if err != nil {
		return nil, fmt.Errorf("GroundReply từ chối thẻ: %v", err)
	}
	return card, nil
}

// loaiPhan is the kinds of the engine's parts, in order.
func loaiPhan(ps []json.RawMessage) []string {
	out := []string{}
	for _, p := range ps {
		var v struct {
			Kind    string `json:"kind"`
			Payload struct {
				SoKhoan int `json:"so_khoan"`
			} `json:"payload"`
		}
		if json.Unmarshal(p, &v) == nil {
			out = append(out, v.Kind)
		}
	}
	return out
}

// soKhoan is the expense_draft part's so_khoan, 0 when none.
func soKhoan(ps []json.RawMessage) int {
	for _, p := range ps {
		var v struct {
			Kind    string `json:"kind"`
			Payload struct {
				SoKhoan int `json:"so_khoan"`
			} `json:"payload"`
		}
		if json.Unmarshal(p, &v) == nil && v.Kind == "expense_draft" {
			return v.Payload.SoKhoan
		}
	}
	return 0
}

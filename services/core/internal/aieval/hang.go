package aieval

import (
	"sort"

	"mobile/services/core/internal/aiharness"
	"mobile/services/core/internal/aiharness/cau"
	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/obs"
	"mobile/services/core/internal/aiharness/prompts"
	"mobile/services/core/internal/aiharness/tools"
)

// Hang is the engine's numbers and closed sets, as the eval and its future
// Python runner must read them: from the engine, never restated (design 06
// §2, the `hang` op of §3.2).
type Hang struct {
	MoHinh               string `json:"mo_hinh"`
	MaxModelCallsPerTurn int    `json:"max_model_calls_per_turn"`
	NepMaxChu            int    `json:"nep_max_chu"`
	PromptVersionNep     string `json:"prompt_version_nep"`
	// Ma is every code a turn can end with instead of an answer.
	Ma []string `json:"ma"`
	// TrangThai is every status a turn can emit.
	TrangThai []string `json:"trang_thai"`
	// CongCu is, per bot the engine runs, the tools a request may declare.
	CongCu map[string][]string `json:"cong_cu"`
}

// chayTrenEngine are the bots the engine runs; the group bot is not on it
// yet.
var chayTrenEngine = map[obs.Bot]bool{obs.BotNep: true}

// CongCuDuocPhep lists the tools bot may declare, read from the permission
// table (tools/testdata/quyen.golden.json through tools.MacDinh), never
// restated here, and whether the engine runs that bot at all.
func CongCuDuocPhep(bot obs.Bot) ([]string, bool) {
	if !chayTrenEngine[bot] {
		return []string{}, false
	}
	out := []string{}
	for _, t := range tools.MacDinh.DuocPhep(bot, false) {
		out = append(out, string(t))
	}
	return out, true
}

// DocHang reads the constants off the engine.
func DocHang() Hang {
	h := Hang{
		MoHinh:               llm.Model,
		MaxModelCallsPerTurn: llm.MaxModelCallsPerTurn,
		NepMaxChu:            aiharness.NepMaxChu,
		PromptVersionNep:     prompts.VersionNep(),
		CongCu:               map[string][]string{},
	}
	for _, m := range cau.Tat() {
		h.Ma = append(h.Ma, string(m))
	}
	sort.Strings(h.Ma)
	for _, s := range cau.TatTrangThai() {
		h.TrangThai = append(h.TrangThai, string(s))
	}
	for bot := range chayTrenEngine {
		ds, _ := CongCuDuocPhep(bot)
		h.CongCu[string(bot)] = ds
	}
	return h
}

package aieval

import (
	"sort"

	"mobile/services/core/internal/aiharness"
	"mobile/services/core/internal/aiharness/cau"
	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/obs"
	"mobile/services/core/internal/aiharness/prompts"
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

// congCuDuocPhep is what each bot the engine runs may declare. At S1 Nếp is
// an agent with no tool (design 01 §8, slice 6), and the group bot is not on
// the engine yet. Slice 9 builds the registry and tools/testdata/
// quyen.golden.json; this map is then read from that golden file, not written
// here.
var congCuDuocPhep = map[obs.Bot][]string{obs.BotNep: {}}

// CongCuDuocPhep lists the tools bot may declare, and whether the engine runs
// that bot at all.
func CongCuDuocPhep(bot obs.Bot) ([]string, bool) {
	ds, ok := congCuDuocPhep[bot]
	return append([]string{}, ds...), ok
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
	for bot := range congCuDuocPhep {
		ds, _ := CongCuDuocPhep(bot)
		h.CongCu[string(bot)] = ds
	}
	return h
}

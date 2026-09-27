package llm

import "google.golang.org/genai"

// LoaiGoi is one kind of model call a turn may make. The set is closed: every
// text-producing call of the engine is one of these, and each has its
// thinking level (MucNghi) and its place in the worst-case plan (KeHoach).
type LoaiGoi string

const (
	// BuocRouter is the router's one structured call (hieu).
	BuocRouter LoaiGoi = "router"
	// BuocRouterSua is the router's one repair re-ask after a refused
	// output.
	BuocRouterSua LoaiGoi = "router_sua"
	// BuocCham is the corrective loop's grader (crag): the sufficiency
	// judgement. It is the only step that judges retrieval, and it is an
	// LLM call, so it is the first cut when the budget runs short.
	BuocCham LoaiGoi = "cham"
	// BuocTraLoi is the grounded structured answer's first draft (traloi).
	BuocTraLoi LoaiGoi = "tra_loi"
	// BuocSinhLai is the answer's one regeneration.
	BuocSinhLai LoaiGoi = "sinh_lai"
	// BuocKiem is the verifier (kiemchung) in a fresh context on released
	// prose.
	BuocKiem LoaiGoi = "kiem"
	// BuocKiemLai is the verifier of the regenerated draft.
	BuocKiemLai LoaiGoi = "kiem_lai"
	// BuocAgentKeHoach is an agent-loop step with function calling on: the
	// model plans which tool to call, or answers.
	BuocAgentKeHoach LoaiGoi = "agent_ke_hoach"
	// BuocAgentTraLoi is an agent step with function calling off: the
	// answer (the loop's last step, the fast path's one call, the direct
	// answer).
	BuocAgentTraLoi LoaiGoi = "agent_tra_loi"
)

// mucNghi is each step's thinking level, set explicitly so a provider
// default that moves with a model release cannot change a step under us
// (SOTA gap #7). MINIMAL where the output is a schema or an answer from
// evidence already in hand; LOW where the agent plans tool calls. Only a
// level is ever set, never a token budget: the two are exclusive on Gemini
// 3.x, and a budget beside a level is refused by the API.
var mucNghi = map[LoaiGoi]genai.ThinkingLevel{
	BuocRouter:       genai.ThinkingLevelMinimal,
	BuocRouterSua:    genai.ThinkingLevelMinimal,
	BuocCham:         genai.ThinkingLevelMinimal,
	BuocTraLoi:       genai.ThinkingLevelMinimal,
	BuocSinhLai:      genai.ThinkingLevelMinimal,
	BuocKiem:         genai.ThinkingLevelMinimal,
	BuocKiemLai:      genai.ThinkingLevelMinimal,
	BuocAgentKeHoach: genai.ThinkingLevelLow,
	BuocAgentTraLoi:  genai.ThinkingLevelMinimal,
}

// MucNghi is step b's thinking level (MINIMAL for a step outside the set,
// which a test keeps empty).
func MucNghi(b LoaiGoi) genai.ThinkingLevel {
	if l, ok := mucNghi[b]; ok {
		return l
	}
	return genai.ThinkingLevelMinimal
}

// CauHinhNghi is step b's thinking configuration: the level, and no token
// budget.
func CauHinhNghi(b LoaiGoi) *genai.ThinkingConfig {
	return &genai.ThinkingConfig{ThinkingLevel: MucNghi(b)}
}

// Duong is a path through a turn, as far as model calls go.
type Duong string

const (
	// DuongTruyHoi: router, grader, answer, verifier, one regeneration and
	// its verifier.
	DuongTruyHoi Duong = "truy_hoi"
	// DuongTacTuNep and DuongTacTuNhom: router, the agent loop, verifier.
	DuongTacTuNep  Duong = "tac_tu_nep"
	DuongTacTuNhom Duong = "tac_tu_nhom"
	// DuongThang: router, one answer with no tool, verifier.
	DuongThang Duong = "thang"
	// DuongHoiLai: router, the verifier on its question back.
	DuongHoiLai Duong = "hoi_lai"
)

// BuocGoi is one step of a path's worst case.
type BuocGoi struct {
	Loai LoaiGoi
	// ToiDa is how many calls the step makes at most.
	ToiDa int
	// Cat is the order the step is cut in when the turn's budget cannot
	// hold the whole path: 1 is cut first. 0 is never cut: the router
	// (nothing runs without it), the answer, and every verifier (a text is
	// never released unverified; with no call left for its verifier, the
	// answer is not made and the fixed fallback stands).
	Cat int
}

// KeHoach is every path's worst case, in the order the steps run. The sum
// of each path is at most MaxModelCallsPerTurn (TestKeHoachTrongTran), and
// the engine's own guards cut in the order Cat gives:
//  1. the grader (crag.DuTruCham: it runs only when the whole answer cycle
//     after it still fits);
//  2. the regeneration and its verifier (traloi.DuTruTraLoi);
//  3. the router's repair (hieu.DuTru: only with a call left after it).
//
// The corrective round itself is a retrieval, not a model call. A
// sufficiency check is never a separate call besides the grader.
var KeHoach = map[Duong][]BuocGoi{
	DuongTruyHoi: {
		{BuocRouter, 1, 0}, {BuocRouterSua, 1, 3}, {BuocCham, 1, 1},
		{BuocTraLoi, 1, 0}, {BuocKiem, 1, 0}, {BuocSinhLai, 1, 2}, {BuocKiemLai, 1, 2},
	},
	DuongTacTuNep: {
		{BuocRouter, 1, 0}, {BuocRouterSua, 1, 3},
		{BuocAgentKeHoach, MaxStepsNep - 1, 2}, {BuocAgentTraLoi, 1, 0}, {BuocKiem, 1, 0},
	},
	DuongTacTuNhom: {
		{BuocRouter, 1, 0}, {BuocRouterSua, 1, 3},
		{BuocAgentKeHoach, MaxStepsNhom - 1, 2}, {BuocAgentTraLoi, 1, 0}, {BuocKiem, 1, 0},
	},
	DuongThang:  {{BuocRouter, 1, 0}, {BuocRouterSua, 1, 3}, {BuocAgentTraLoi, 1, 0}, {BuocKiem, 1, 0}},
	DuongHoiLai: {{BuocRouter, 1, 0}, {BuocRouterSua, 1, 3}, {BuocKiem, 1, 0}},
}

// ToiDaDuong is path d's worst case in model calls.
func ToiDaDuong(d Duong) int {
	n := 0
	for _, b := range KeHoach[d] {
		n += b.ToiDa
	}
	return n
}

// SauBuoc is how many calls path d makes at most from step b on, b
// included: what must still be in the budget for b to run without
// starving a step after it.
func SauBuoc(d Duong, b LoaiGoi) int {
	n, thay := 0, false
	for _, x := range KeHoach[d] {
		thay = thay || x.Loai == b
		if thay {
			n += x.ToiDa
		}
	}
	return n
}

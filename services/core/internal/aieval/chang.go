package aieval

import (
	"strings"
	"sync"

	"google.golang.org/adk/v2/model"

	"mobile/services/core/internal/aiharness/chiabill"
	"mobile/services/core/internal/aiharness/crag"
	"mobile/services/core/internal/aiharness/hieu"
	"mobile/services/core/internal/aiharness/kiemchung"
	"mobile/services/core/internal/aiharness/obs"
	"mobile/services/core/internal/aiharness/prompts"
	"mobile/services/core/internal/aiharness/traloi"
)

// In the model modes there is no script to say which stage each request
// belongs to, and several checks read the stage (the canary marker belongs
// in the prose answer's instruction only; the «now» line in the router's and
// the prose answer's; the words a case pins, in the prose answer's). The
// stage is then read off the request itself, structurally: the system
// instruction each stage's own request builder writes. The builders are the
// engine's exported ones, so a prompt change moves this with it.
// TestChangTuYeuCauKhopKichBan holds it to the scripts' stage tags on every
// request of the T1 corpus.

var heChang struct {
	once sync.Once
	// bang maps an exact instruction to its stage.
	bang map[string]string
	// agentTruoc and agentSau are Nếp's prose instruction around the canary
	// marker; nhomTruoc and nhomSau the group's.
	agentTruoc, agentSau string
	nhomTruoc, nhomSau   string
}

func heCua(req *model.LLMRequest) (s string) {
	defer func() {
		if recover() != nil {
			s = ""
		}
	}()
	if req == nil || req.Config == nil || req.Config.SystemInstruction == nil {
		return ""
	}
	var b strings.Builder
	for _, p := range req.Config.SystemInstruction.Parts {
		b.WriteString(p.Text)
	}
	return b.String()
}

func dungHeChang() {
	h := &heChang
	h.bang = map[string]string{}
	them := func(chang string, f func() *model.LLMRequest) {
		var req *model.LLMRequest
		func() {
			defer func() { _ = recover() }()
			req = f()
		}()
		if s := heCua(req); s != "" {
			h.bang[s] = chang
		}
	}
	them(ChangHieu, func() *model.LLMRequest {
		r, _ := hieu.YeuCau(hieu.Vao{Bot: obs.BotNep, Cau: "x", Luc: LucBoHieu}, nil)
		return r
	})
	them(ChangHieu, func() *model.LLMRequest {
		r, _ := hieu.YeuCau(hieu.Vao{Bot: obs.BotNhom, Cau: "x", Luc: LucBoHieu}, nil)
		return r
	})
	them(ChangChiaBill, func() *model.LLMRequest {
		r, _ := chiabill.YeuCau(chiabill.Vao{LoiNho: "x"})
		return r
	})
	them(ChangKiemChiaBill, func() *model.LLMRequest {
		r, _ := chiabill.YeuCauKiem(chiabill.Vao{LoiNho: "x 1k"}, []chiabill.Khoan{{Tin: chiabill.BiDanhLoiNho, SoTienVND: 1000, SoTienGoc: "1k"}}, "x")
		return r
	})
	them(ChangCham, func() *model.LLMRequest { return crag.YeuCauCham(crag.Vao{}) })
	them(ChangTraLoiCauTruc, func() *model.LLMRequest { return traloi.YeuCauTraLoi(traloi.Vao{Cau: "x"}, crag.KetQua{}, nil, nil) })
	them(ChangKiem, func() *model.LLMRequest { return kiemchung.YeuCauKiem([]string{"x"}, nil) })
	const moc = "\x00"
	mau := prompts.NepAgent(moc)
	if i := strings.Index(mau, moc); i >= 0 {
		h.agentTruoc, h.agentSau = mau[:i], mau[i+len(moc):]
	}
	mau = prompts.NhomAgent(moc)
	if i := strings.Index(mau, moc); i >= 0 {
		h.nhomTruoc, h.nhomSau = mau[:i], mau[i+len(moc):]
	}
}

// ChangTuYeuCau is the stage whose builder wrote sys, or "" for none known.
// The prose answer is Nếp's agent instruction with any canary marker in its
// place (and the clauses the engine may append after it); the marker itself
// is not read, so the check that the marker is the turn's own stays a check.
func ChangTuYeuCau(sys string) string {
	heChang.once.Do(dungHeChang)
	h := &heChang
	if c, ok := h.bang[sys]; ok {
		return c
	}
	for _, m := range [][2]string{{h.agentTruoc, h.agentSau}, {h.nhomTruoc, h.nhomSau}} {
		if m[0] == "" || !strings.HasPrefix(sys, m[0]) {
			continue
		}
		rest := sys[len(m[0]):]
		i := 0
		for i < len(rest) && (rest[i] >= '0' && rest[i] <= '9' || rest[i] >= 'a' && rest[i] <= 'z') {
			i++
		}
		if strings.HasPrefix(rest[i:], m[1]) {
			return ChangTraLoi
		}
	}
	return ""
}

// Package agent builds one ADK llmagent and one runner per turn, runs it, and
// throws both away. The ADK session is an in-memory service that lives for
// that one turn: earlier panel turns are laid into it as user and model
// events, and nothing survives the turn (ADR-0037 §2.7). ADK's memory service
// is never used.
//
// Budget callbacks run on every model step, tools or not: a step counter that
// forces function calling off on the last step and refuses a step past it, and
// an accounting callback that adds up tokens and the finish reason.
package agent

import (
	"context"
	"errors"
	"strings"
	"sync"

	adkagent "google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/genai"

	"mobile/services/core/internal/aiharness/llm"

	// Clears ADK's content-capture switch before any ADK call.
	_ "mobile/services/core/internal/aiharness/otelchan"
)

// CauHinh is one bot's agent.
type CauHinh struct {
	// Ten is the agent's name, which ADK also writes into the system
	// instruction («Your internal name is …»).
	Ten             string
	Instruction     string
	NhietDo         float32
	MaxOutputTokens int32
	// MaxBuoc is the step ceiling: the last step runs with function calling
	// off, and a step past it is refused.
	MaxBuoc int
	// ConLai, when set, reports the model calls left in the turn's budget;
	// with one left the step runs with function calling off too.
	ConLai func() int

	// Tools is the turn's permitted toolset (tools.BoiCanh.BoCongCu); none
	// for a turn that answers without tools.
	Tools []tool.Tool
	// TruocTool, SauTool and LoiTool are the tool callbacks (permission and
	// argument checks, evidence ledger, closed error codes).
	TruocTool llmagent.BeforeToolCallback
	SauTool   llmagent.AfterToolCallback
	LoiTool   llmagent.OnToolErrorCallback
	// EpTraLoi, when set and true, makes the next step the final answer
	// (function calling off): the repair or the tool budget is spent.
	EpTraLoi func() bool
	// BuocCuoi, when set, is told that a step with function calling off has
	// started (tools.BoiCanh.DatBuocCuoi): a function call the model returns
	// on it anyway is refused by the tool callbacks, never run.
	BuocCuoi func()
	// ChoPhep, when set, names the tools the model may call on this step
	// (tools.BoiCanh.TenChoPhep): the declarations stay the bot's whole
	// permitted set in registry order, so the request's prefix is the same
	// on every step and every turn, and the step is restricted by
	// FunctionCallingConfig.AllowedFunctionNames («mask, don't remove»).
	// An empty list turns function calling off for the step.
	ChoPhep func() []string
	// BatDauBuoc, when set, is told the number of each step as it starts
	// (tools.BoiCanh.DatBuoc), before the model is called: the tool calls
	// it returns belong to that step (the taint invariant from step 2 on).
	BatDauBuoc func(n int)
}

// CheDoGiua is the function calling mode of every step but the last when
// the turn has tools: VALIDATED, the MODEL decides whether to call a tool
// or to answer, and a call it makes is constrained to the step's
// AllowedFunctionNames (genai v1.71.0 types.go: «If allowed_function_names
// are set, the predicted function calls will be limited to any one of
// allowed_function_names»; the field's own comment still says «only when
// the Mode is ANY», which the enum's comment supersedes). Never ANY, which
// forces calls. The last step is NONE.
const CheDoGiua = genai.FunctionCallingConfigModeValidated

// Luot is one earlier turn of the session.
type Luot struct {
	// Nguoi is true for the person's turn, false for the assistant's.
	Nguoi bool
	Chu   string
}

// TheoDoi is what the callbacks counted. Safe for the concurrent callbacks
// ADK may run.
type TheoDoi struct {
	mu          sync.Mutex
	Buoc        int
	TokensIn    int
	TokensOut   int
	TokensCache int
	TokensNghi  int
	Finish      genai.FinishReason
}

// Snapshot copies the counters.
func (t *TheoDoi) Snapshot() TheoDoi {
	t.mu.Lock()
	defer t.mu.Unlock()
	return TheoDoi{Buoc: t.Buoc, TokensIn: t.TokensIn, TokensOut: t.TokensOut, TokensCache: t.TokensCache, TokensNghi: t.TokensNghi, Finish: t.Finish}
}

// ErrHetBuoc: the model asked for another step past the ceiling.
var ErrHetBuoc = errors.New("agent: step budget spent")

// TraLoiNgay is appended to the system instruction on the last step.
const TraLoiNgay = "Trả lời ngay bằng dữ liệu đã có, không gọi thêm công cụ nào."

// AnToan is the explicit safety configuration: the four categories at
// BLOCK_MEDIUM_AND_ABOVE, written out rather than left to a provider default
// that can change under us.
func AnToan() []*genai.SafetySetting {
	var out []*genai.SafetySetting
	for _, c := range []genai.HarmCategory{genai.HarmCategoryHarassment, genai.HarmCategoryHateSpeech, genai.HarmCategorySexuallyExplicit, genai.HarmCategoryDangerousContent} {
		out = append(out, &genai.SafetySetting{Category: c, Threshold: genai.HarmBlockThresholdBlockMediumAndAbove})
	}
	return out
}

func (t *TheoDoi) truocMoHinh(cfg CauHinh) llmagent.BeforeModelCallback {
	return func(_ adkagent.Context, req *model.LLMRequest) (*model.LLMResponse, error) {
		t.mu.Lock()
		t.Buoc++
		b := t.Buoc
		t.mu.Unlock()
		if b > cfg.MaxBuoc {
			return nil, ErrHetBuoc
		}
		if cfg.BatDauBuoc != nil {
			cfg.BatDauBuoc(b)
		}
		if req.Config == nil {
			req.Config = &genai.GenerateContentConfig{}
		}
		var choPhep []string
		if len(cfg.Tools) > 0 && cfg.ChoPhep != nil {
			choPhep = cfg.ChoPhep()
		}
		khongCongCu := len(cfg.Tools) > 0 && cfg.ChoPhep != nil && len(choPhep) == 0
		if b == cfg.MaxBuoc || (cfg.ConLai != nil && cfg.ConLai() <= 1) || (cfg.EpTraLoi != nil && cfg.EpTraLoi()) || khongCongCu {
			// The answer: thinking at the answer's level, never a budget.
			req.Config.ThinkingConfig = llm.CauHinhNghi(llm.BuocAgentTraLoi)
			req.Config.ToolConfig = &genai.ToolConfig{FunctionCallingConfig: &genai.FunctionCallingConfig{Mode: genai.FunctionCallingConfigModeNone}}
			if req.Config.SystemInstruction == nil {
				req.Config.SystemInstruction = &genai.Content{Role: genai.RoleUser}
			}
			req.Config.SystemInstruction.Parts = append(req.Config.SystemInstruction.Parts, &genai.Part{Text: TraLoiNgay})
			if cfg.BuocCuoi != nil {
				cfg.BuocCuoi()
			}
		} else if len(cfg.Tools) > 0 {
			// A planning step: the model chooses a tool or answers.
			req.Config.ThinkingConfig = llm.CauHinhNghi(llm.BuocAgentKeHoach)
			req.Config.ToolConfig = &genai.ToolConfig{FunctionCallingConfig: &genai.FunctionCallingConfig{Mode: CheDoGiua, AllowedFunctionNames: choPhep}}
		} else {
			// No tool at all (the direct answer): an answer step.
			req.Config.ThinkingConfig = llm.CauHinhNghi(llm.BuocAgentTraLoi)
		}
		return nil, nil
	}
}

func (t *TheoDoi) sauMoHinh(_ adkagent.Context, resp *model.LLMResponse, _ error) (*model.LLMResponse, error) {
	if resp == nil {
		return nil, nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if u := resp.UsageMetadata; u != nil {
		t.TokensIn += int(u.PromptTokenCount)
		t.TokensOut += int(u.CandidatesTokenCount)
		t.TokensCache += int(u.CachedContentTokenCount)
		t.TokensNghi += int(u.ThoughtsTokenCount)
	}
	if resp.FinishReason != "" {
		t.Finish = resp.FinishReason
	}
	if chiNghi(resp) {
		// ADK v2 calls the model again after a thought-only reply (up to ten
		// times, base_flow.go maxConsecutiveThoughtOnlyTurns), appending a
		// synthetic «Continue processing…» user turn. v1 ended the turn on
		// it with an empty answer, and the engine's budgets and replies were
		// written against that: one empty text part makes the reply a final
		// answer again, so the turn ends here exactly as it did.
		out := *resp
		c := *resp.Content
		c.Parts = append(append([]*genai.Part(nil), resp.Content.Parts...), &genai.Part{Text: ""})
		out.Content = &c
		return &out, nil
	}
	return nil, nil
}

// chiNghi reports a complete reply whose every part is a thought: no text,
// no function call.
func chiNghi(resp *model.LLMResponse) bool {
	if resp.Partial || resp.Content == nil || len(resp.Content.Parts) == 0 {
		return false
	}
	for _, p := range resp.Content.Parts {
		if p != nil && !p.Thought {
			return false
		}
	}
	return true
}

const (
	appName = "rudi"
	// The ADK session never learns who asked: it lives for one turn in
	// memory, and a person id has no business in it.
	userID    = "nguoi_hoi"
	sessionID = "luot"
)

// Chay runs one turn: the earlier turns as session events, cuoi as the new
// user message, and returns the final answer's text.
func Chay(ctx context.Context, m model.LLM, cfg CauHinh, luot []Luot, cuoi string, td *TheoDoi) (string, error) {
	nhiet := cfg.NhietDo
	var truocTool []llmagent.BeforeToolCallback
	var sauTool []llmagent.AfterToolCallback
	var loiTool []llmagent.OnToolErrorCallback
	if cfg.TruocTool != nil {
		truocTool = append(truocTool, cfg.TruocTool)
	}
	if cfg.SauTool != nil {
		sauTool = append(sauTool, cfg.SauTool)
	}
	if cfg.LoiTool != nil {
		loiTool = append(loiTool, cfg.LoiTool)
	}
	a, err := llmagent.New(llmagent.Config{
		Name:  cfg.Ten,
		Model: m,
		InstructionProvider: func(adkagent.ReadonlyContext) (string, error) {
			return cfg.Instruction, nil
		},
		GenerateContentConfig: &genai.GenerateContentConfig{
			Temperature:     &nhiet,
			MaxOutputTokens: cfg.MaxOutputTokens,
			SafetySettings:  AnToan(),
		},
		Tools:                    cfg.Tools,
		BeforeToolCallbacks:      truocTool,
		AfterToolCallbacks:       sauTool,
		OnToolErrorCallbacks:     loiTool,
		BeforeModelCallbacks:     []llmagent.BeforeModelCallback{td.truocMoHinh(cfg)},
		AfterModelCallbacks:      []llmagent.AfterModelCallback{td.sauMoHinh},
		DisallowTransferToParent: true,
		DisallowTransferToPeers:  true,
	})
	if err != nil {
		return "", err
	}
	svc := session.InMemoryService()
	created, err := svc.Create(ctx, &session.CreateRequest{AppName: appName, UserID: userID, SessionID: sessionID})
	if err != nil {
		return "", err
	}
	for _, l := range luot {
		ev := session.NewEvent(ctx, "truoc")
		role := genai.Role(genai.RoleUser)
		ev.Author = "user"
		if !l.Nguoi {
			role = genai.RoleModel
			ev.Author = cfg.Ten
		}
		ev.LLMResponse = model.LLMResponse{Content: &genai.Content{Role: string(role), Parts: []*genai.Part{{Text: l.Chu}}}}
		if err := svc.AppendEvent(ctx, created.Session, ev); err != nil {
			return "", err
		}
	}
	r, err := runner.New(runner.Config{AppName: appName, Agent: a, SessionService: svc})
	if err != nil {
		return "", err
	}
	var text string
	msg := genai.NewContentFromText(cuoi, genai.RoleUser)
	for ev, err := range r.Run(ctx, userID, sessionID, msg, adkagent.RunConfig{StreamingMode: adkagent.StreamingModeNone}) {
		if err != nil {
			return "", err
		}
		if ev == nil || ev.Author != cfg.Ten || ev.Partial || !ev.IsFinalResponse() || ev.Content == nil {
			continue
		}
		var b strings.Builder
		for _, p := range ev.Content.Parts {
			if p != nil && !p.Thought {
				b.WriteString(p.Text)
			}
		}
		text = b.String()
	}
	return text, nil
}

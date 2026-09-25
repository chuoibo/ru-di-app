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

	adkagent "google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/genai"
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
}

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
	return func(_ adkagent.CallbackContext, req *model.LLMRequest) (*model.LLMResponse, error) {
		t.mu.Lock()
		t.Buoc++
		b := t.Buoc
		t.mu.Unlock()
		if b > cfg.MaxBuoc {
			return nil, ErrHetBuoc
		}
		if b == cfg.MaxBuoc || (cfg.ConLai != nil && cfg.ConLai() <= 1) {
			if req.Config == nil {
				req.Config = &genai.GenerateContentConfig{}
			}
			req.Config.ToolConfig = &genai.ToolConfig{FunctionCallingConfig: &genai.FunctionCallingConfig{Mode: genai.FunctionCallingConfigModeNone}}
			if req.Config.SystemInstruction == nil {
				req.Config.SystemInstruction = &genai.Content{Role: genai.RoleUser}
			}
			req.Config.SystemInstruction.Parts = append(req.Config.SystemInstruction.Parts, &genai.Part{Text: TraLoiNgay})
		}
		return nil, nil
	}
}

func (t *TheoDoi) sauMoHinh(_ adkagent.CallbackContext, resp *model.LLMResponse, _ error) (*model.LLMResponse, error) {
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
	return nil, nil
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
		ev := session.NewEventWithContext(ctx, "truoc")
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

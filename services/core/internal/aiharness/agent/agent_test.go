package agent

import (
	"context"
	"errors"
	"iter"
	"sync"
	"testing"

	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"
)

// moHinh answers every call with the same content and counts the calls.
type moHinh struct {
	mu   sync.Mutex
	goi  int
	tra  func() *genai.Content
	yeus []*model.LLMRequest
}

func (m *moHinh) Name() string { return "stub" }

func (m *moHinh) GenerateContent(_ context.Context, req *model.LLMRequest, _ bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		m.mu.Lock()
		m.goi++
		m.yeus = append(m.yeus, req)
		m.mu.Unlock()
		yield(&model.LLMResponse{Content: m.tra(), FinishReason: genai.FinishReasonStop}, nil)
	}
}

func cauHinh(maxBuoc int) CauHinh {
	return CauHinh{Ten: "rudi_thu", Instruction: "thử", MaxOutputTokens: 64, MaxBuoc: maxBuoc}
}

// A thought-only reply ends the turn with an empty answer after one model
// call, as it did under ADK v1; v2 alone would call the model again.
func TestThoughtOnlyReplyEndsTheTurn(t *testing.T) {
	m := &moHinh{tra: func() *genai.Content {
		return &genai.Content{Role: genai.RoleModel, Parts: []*genai.Part{{Text: "đang nghĩ", Thought: true}}}
	}}
	var td TheoDoi
	text, err := Chay(t.Context(), m, cauHinh(4), nil, "câu hỏi", &td)
	if err != nil {
		t.Fatal(err)
	}
	if text != "" {
		t.Fatalf("answer %q, want empty (thought parts are never the answer)", text)
	}
	if m.goi != 1 || td.Snapshot().Buoc != 1 {
		t.Fatalf("model called %d times, %d steps counted; want 1 and 1", m.goi, td.Snapshot().Buoc)
	}
}

// A plain text reply is the answer, in one call, and the history ends on the
// person's message (no synthetic continuation turn).
func TestTextReplyIsTheAnswer(t *testing.T) {
	m := &moHinh{tra: func() *genai.Content {
		return &genai.Content{Role: genai.RoleModel, Parts: []*genai.Part{{Text: "trả lời"}}}
	}}
	var td TheoDoi
	text, err := Chay(t.Context(), m, cauHinh(4), []Luot{{Nguoi: true, Chu: "trước"}, {Chu: "đáp trước"}}, "câu hỏi", &td)
	if err != nil {
		t.Fatal(err)
	}
	if text != "trả lời" || m.goi != 1 {
		t.Fatalf("answer %q after %d calls", text, m.goi)
	}
	c := m.yeus[0].Contents
	if len(c) != 3 || c[2].Role != genai.RoleUser || c[2].Parts[0].Text != "câu hỏi" {
		t.Fatalf("request history: %d contents, last %+v", len(c), c[len(c)-1])
	}
}

// The step refusal reaches the caller as ErrHetBuoc through ADK v2's node
// runtime, without the model being called.
func TestStepPastCeilingIsRefused(t *testing.T) {
	m := &moHinh{tra: func() *genai.Content {
		return &genai.Content{Role: genai.RoleModel, Parts: []*genai.Part{{Text: "x"}}}
	}}
	var td TheoDoi
	_, err := Chay(t.Context(), m, cauHinh(0), nil, "câu hỏi", &td)
	if !errors.Is(err, ErrHetBuoc) {
		t.Fatalf("err = %v, want ErrHetBuoc", err)
	}
	if m.goi != 0 {
		t.Fatalf("model called %d times past the ceiling", m.goi)
	}
}

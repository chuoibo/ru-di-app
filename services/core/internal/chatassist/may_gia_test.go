package chatassist

import (
	"context"
	"encoding/json"
	"errors"
	"iter"
	"sync"

	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"

	"mobile/services/core/internal/aiharness/llm"
)

// mayGia is a model for the chat engine's infrastructure tests (the queue,
// the lease, the publish, the stream): it answers every step of a turn
// without a script, by the shape of the request. The router's request gets a
// clean direct-answer routing, the verifier's gets «no factual claim», and
// every other step (the agent's plan, the answer) gets traLoi as plain text.
// Tests of what the engine decides use llm.NewStub with a script instead.
type mayGia struct {
	mu  sync.Mutex
	yeu [][]byte
	// traLoi is the answer text; "" means «Synthetic inference fixture».
	traLoi string
	// truocTraLoi, when set, runs before each answer step (not the router,
	// not the verifier), e.g. to hold the turn while a test revokes a grant.
	// ctx is the call's: a test can wait for it to be cancelled.
	truocTraLoi func(ctx context.Context)
	// luot counts the router's requests: one per turn.
	luot int
	// kiem, when set, is the verifier's answer (it must judge every
	// sentence of traLoi); "" judges one sentence as stating nothing.
	kiem string
	// loi, when set, fails every call.
	loi error
}

var errMayGiaTrong = errors.New("mayGia: empty request")

func (m *mayGia) Name() string { return llm.Model }

// SoGoi is how many requests reached the model.
func (m *mayGia) SoGoi() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.yeu)
}

// SoLuot is how many turns reached the model (the router opens each).
func (m *mayGia) SoLuot() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.luot
}

// TatCa is every request, in canonical JSON, joined: what the model heard.
func (m *mayGia) TatCa() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []byte
	for _, y := range m.yeu {
		out = append(append(out, y...), '\n')
	}
	return string(out)
}

func coTruong(req *model.LLMRequest, ten string) bool {
	if req.Config == nil || req.Config.ResponseSchema == nil {
		return false
	}
	_, ok := req.Config.ResponseSchema.Properties[ten]
	return ok
}

func (m *mayGia) GenerateContent(ctx context.Context, req *model.LLMRequest, _ bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		if req == nil {
			yield(nil, errMayGiaTrong)
			return
		}
		canon, err := llm.Canon(req)
		if err != nil {
			yield(nil, err)
			return
		}
		m.mu.Lock()
		m.yeu = append(m.yeu, canon)
		loi, traLoi, truoc, kiem := m.loi, m.traLoi, m.truocTraLoi, m.kiem
		m.mu.Unlock()
		if loi != nil {
			yield(nil, loi)
			return
		}
		var text string
		switch {
		case coTruong(req, "nhan_guard"):
			m.mu.Lock()
			m.luot++
			m.mu.Unlock()
			raw, _ := json.Marshal(map[string]any{"nhan_guard": "sach", "tien": "none", "y_dinh": []string{"smalltalk"}, "huong": "tra_loi_thang",
				"slots": map[string]any{}, "can_truy_hoi": []string{}, "truy_van": []any{}, "can_hoi_lai": false, "tra_loi_cau_cho": false, "tu_tin": "cao"})
			text = string(raw)
		case coTruong(req, "menh_de") && kiem != "":
			text = kiem
		case coTruong(req, "menh_de"):
			raw, _ := json.Marshal(map[string]any{"menh_de": []any{map[string]any{"so": 1, "bang_chung_ids": []string{}, "ket": "khong_thong_tin"}}, "hua_hanh_dong_khong_co": false, "tien": false})
			text = string(raw)
		default:
			if truoc != nil {
				truoc(ctx)
				if ctx.Err() != nil {
					yield(nil, ctx.Err())
					return
				}
			}
			text = traLoi
			if text == "" {
				text = "Synthetic inference fixture"
			}
		}
		yield(&model.LLMResponse{
			Content:      &genai.Content{Role: "model", Parts: []*genai.Part{{Text: text}}},
			FinishReason: genai.FinishReasonStop,
			TurnComplete: true,
		}, nil)
	}
}

//go:build postgres || broker

package chatassist

import (
	"context"
	"iter"
	"time"

	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"

	"mobile/services/core/internal/aiharness/llm"
)

// The engine's Nếp turn is the router path (aiharness/dinhtuyen.go): the
// router's structured call, the answer, the verifier. These are the stub
// replies of those three stages.
const (
	// ruTraLoiThang: a clean direct turn whose «tối nay» the router laid on
	// the evening of the question's own day.
	ruTraLoiThang = `{"nhan_guard":"sach","tien":"none","y_dinh":["plan_help"],"huong":"tra_loi_thang","slots":{"ngay_iso":"2026-09-24","khung_gio":{"tu":"18:00","den":"23:00"}},"can_truy_hoi":[],"truy_van":[],"can_hoi_lai":false,"tra_loi_cau_cho":false,"tu_tin":"cao"}`
	// ruTien: the router classed a money action.
	ruTien = `{"nhan_guard":"sach","tien":"money_action","y_dinh":["smalltalk"],"huong":"tra_loi_thang","slots":{},"can_truy_hoi":[],"truy_van":[],"can_hoi_lai":false,"tra_loi_cau_cho":false,"tu_tin":"cao"}`
	// kiemDat: the verifier passes a one-sentence answer (every answer
	// scripted here is one sentence; the verifier must judge each).
	kiemDat = `{"menh_de":[{"so":1,"bang_chung_ids":[],"ket":"khong_thong_tin"}],"hua_hanh_dong_khong_co":false,"tien":false}`
)

// kichNep is one whole Nếp turn: router, answer, verifier. cho delays the
// router's reply (a slow first call).
func kichNep(traLoi string, cho time.Duration, usage *genai.GenerateContentResponseUsageMetadata) []llm.Buoc {
	return []llm.Buoc{{Text: ruTraLoiThang, Cho: cho}, {Text: traLoi, Usage: usage}, {Text: kiemDat}}
}

// theoChang answers each request from the stub of its stage, told apart by
// the request's own response schema (the router's has nhan_guard, the
// verifier's menh_de; the prose answer has none): jobs running side by side
// cannot take each other's replies, whatever order their calls land in.
type theoChang struct {
	hieu, tra, kiem *llm.Stub
}

func (m theoChang) Name() string { return m.tra.Name() }

func (m theoChang) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	s := m.tra
	if c := req.Config; c != nil && c.ResponseSchema != nil {
		if _, ok := c.ResponseSchema.Properties["nhan_guard"]; ok {
			s = m.hieu
		} else if _, ok := c.ResponseSchema.Properties["menh_de"]; ok {
			s = m.kiem
		}
	}
	return s.GenerateContent(ctx, req, stream)
}

// nepGiong is n Nếp turns that each answer traLoi, in any order.
func nepGiong(n int, traLoi string) theoChang {
	var h, a, k []llm.Buoc
	for range n {
		h = append(h, llm.Buoc{Text: ruTraLoiThang})
		a = append(a, llm.Buoc{Text: traLoi})
		k = append(k, llm.Buoc{Text: kiemDat})
	}
	return theoChang{hieu: llm.NewStub(h...), tra: llm.NewStub(a...), kiem: llm.NewStub(k...)}
}

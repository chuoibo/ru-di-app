// Package dockhoan is the model step of the per-message expense draft
// (POST /contexts/{id}/messages/{id}/expense-draft): one chat message in,
// whether it reports an expense its writer paid, a title and the amount as
// written. The instruction (doc_khoan.txt) is the Python brain's chat-expense
// reader's, word for word (services/api/app/api/chat_expense_gemini.py before
// ADR-0051): no person, no money written, the message is data.
//
// It decides nothing: domain/chatexpense reads the answer.
package dockhoan

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"strings"

	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"

	"mobile/services/core/internal/aiharness/cautruc"
	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/motluot"
)

//go:embed doc_khoan.txt
var huongDan string

// maxRa bounds the answer: a flag, a short title and an amount.
const maxRa = 512

// LuocDo is the response schema; only is_expense is required.
func LuocDo() *genai.Schema {
	str := &genai.Schema{Type: genai.TypeString}
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"is_expense":  {Type: genai.TypeBoolean},
			"title":       str,
			"amount_text": str,
		},
		Required:         []string{"is_expense"},
		PropertyOrdering: []string{"is_expense", "title", "amount_text"},
	}
}

// YeuCau is the request for one message: the instruction as the system
// turn, the message as JSON data after it.
func YeuCau(text string) *model.LLMRequest {
	var payload bytes.Buffer
	enc := json.NewEncoder(&payload)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(map[string]string{"message_text": text})
	req := cautruc.YeuCau(llm.BuocTrichXuat, huongDan, "SUPPLIED MESSAGE (JSON):\n"+strings.TrimSuffix(payload.String(), "\n"), LuocDo(), maxRa)
	return cautruc.NhietDo(req, 0)
}

// Doc makes the call and returns the raw reading.
func Doc(ctx context.Context, l *motluot.Luot, text string) (map[string]any, error) {
	answer, err := l.Goi(ctx, YeuCau(text))
	if err != nil {
		return nil, err
	}
	return motluot.DocDoiTuong(answer)
}

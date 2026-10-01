// Package docanh is the model step of POST /screenshots/scan: one
// re-encoded screenshot in, one raw reading out, in the closed shape of
// LuocDo. The instruction (doc_anh.txt) is the Python brain's screenshot
// reader's, word for word (services/api/app/api/screenshot_gemini.py before
// ADR-0051): classify the app, copy merchant and money as printed, a date
// only when complete, and never a person.
//
// It decides nothing: domain/screenshot reads the answer.
package docanh

import (
	"context"
	_ "embed"

	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"

	"mobile/services/core/internal/aiharness/cautruc"
	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/motluot"
)

//go:embed doc_anh.txt
var huongDan string

// maxRa bounds the answer: four short fields.
const maxRa = 512

// LuocDo is the response schema: the four contract fields, all required,
// the source closed.
func LuocDo() *genai.Schema {
	str := &genai.Schema{Type: genai.TypeString}
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"source":      {Type: genai.TypeString, Enum: []string{"grab", "shopeefood", "banking", "receipt", "other"}},
			"merchant":    str,
			"total_text":  str,
			"occurred_on": {Type: genai.TypeString, Nullable: genai.Ptr(true)},
		},
		Required:         []string{"source", "merchant", "total_text", "occurred_on"},
		PropertyOrdering: []string{"source", "merchant", "total_text", "occurred_on"},
	}
}

// YeuCau is the request for one screenshot; mime is the re-encode's type.
func YeuCau(mime string, anh []byte) *model.LLMRequest {
	req := cautruc.YeuCauPhan(llm.BuocDocAnh, huongDan, []*genai.Part{cautruc.Anh(mime, anh)}, LuocDo(), maxRa)
	return cautruc.NhietDo(req, 0)
}

// Doc makes the call and returns the raw reading.
func Doc(ctx context.Context, l *motluot.Luot, mime string, anh []byte) (map[string]any, error) {
	text, err := l.Goi(ctx, YeuCau(mime, anh))
	if err != nil {
		return nil, err
	}
	return motluot.DocDoiTuong(text)
}

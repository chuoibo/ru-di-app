// Package docbill is the model step of POST /receipts/scan: one photograph
// in, one raw reading out, in the closed shape of LuocDo. The instruction
// (doc_bill.txt) is the Python brain's receipt reader's, word for word
// (services/api/app/api/vision_gemini.py before ADR-0051): classify the
// paper first, transcribe money strings exactly as printed, never reconcile
// items with the total, treat writing in the photo as data.
//
// It decides nothing about money. domain/receipt reads the raw reading:
// the confidence and document-type gates, whole-đồng amounts, the totals
// check.
package docbill

import (
	"context"
	_ "embed"

	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"

	"mobile/services/core/internal/aiharness/cautruc"
	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/motluot"
)

//go:embed doc_bill.txt
var huongDan string

// The three document types; the domain admits only LoaiHoaDon.
const (
	LoaiHoaDon  = "receipt"
	LoaiThucDon = "price_list"
	LoaiKhac    = "other"
)

// maxRa bounds the answer: a long bill of 40 lines fits in well under it.
const maxRa = 4096

// LuocDo is the response schema. document_type is required and closed so
// the model commits to an answer; "price_list" and "other" are the ways out
// that let it decline instead of describing a bill it never saw. The
// generateContent endpoint refuses additionalProperties, so unknown keys
// are dropped by domain/receipt instead.
func LuocDo() *genai.Schema {
	str := &genai.Schema{Type: genai.TypeString}
	nullStr := &genai.Schema{Type: genai.TypeString, Nullable: genai.Ptr(true)}
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"document_type": {Type: genai.TypeString, Enum: []string{LoaiHoaDon, LoaiThucDon, LoaiKhac}},
			"items": {Type: genai.TypeArray, Items: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"name":            str,
					"quantity_text":   str,
					"unit_price_text": nullStr,
					"line_total_text": str,
				},
				Required:         []string{"name", "unit_price_text", "line_total_text"},
				PropertyOrdering: []string{"name", "quantity_text", "unit_price_text", "line_total_text"},
			}},
			"total_text": nullStr,
			"confidence": {Type: genai.TypeNumber, Minimum: genai.Ptr(0.0), Maximum: genai.Ptr(1.0)},
		},
		Required:         []string{"document_type", "items", "total_text", "confidence"},
		PropertyOrdering: []string{"document_type", "items", "total_text", "confidence"},
	}
}

// YeuCau is the request for one photograph. mime is the sanitised
// re-encode's type, never the uploader's claim.
func YeuCau(mime string, anh []byte) *model.LLMRequest {
	req := cautruc.YeuCauPhan(llm.BuocDocAnh, huongDan, []*genai.Part{cautruc.Anh(mime, anh)}, LuocDo(), maxRa)
	return cautruc.NhietDo(req, 0)
}

// Doc makes the call and returns the raw reading as a JSON object, numbers
// kept as json.Number so nothing is rounded on the way to domain/receipt.
func Doc(ctx context.Context, l *motluot.Luot, mime string, anh []byte) (map[string]any, error) {
	text, err := l.Goi(ctx, YeuCau(mime, anh))
	if err != nil {
		return nil, err
	}
	return motluot.DocDoiTuong(text)
}

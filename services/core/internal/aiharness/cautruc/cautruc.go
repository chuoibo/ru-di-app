// Package cautruc makes one structured model call: a static system
// instruction, one user turn of <du_lieu> blocks, a response schema that is
// the only shape the answer may take, thinking at MINIMAL and the explicit
// safety settings. The CRAG grader, the answer step and the verifier all go
// through it, so every such call is counted by the turn's llm.Dem the same
// way and reads the same.
//
// It decides nothing: the caller reads the returned JSON strictly with its
// own Doc.
package cautruc

import (
	"context"
	"errors"
	"sort"
	"strings"
	"unicode/utf8"

	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"

	"mobile/services/core/internal/aiharness/agent"
	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/prompts"
	"mobile/services/core/internal/aiharness/truyhoi"
)

// ErrBiChan: the provider's own safety filter withheld the answer.
var ErrBiChan = errors.New("cautruc: the provider's safety filter withheld the answer")

// YeuCau is a one-turn structured request for step b (its thinking level,
// llm.CauHinhNghi). The system instruction is the step's static text and
// comes first; everything that changes per turn is in the one user turn
// after it, so the request's prefix is the same for every turn of the step
// (Gemini's implicit cache matches on prefixes).
func YeuCau(b llm.LoaiGoi, he, noiDung string, schema *genai.Schema, maxRa int32) *model.LLMRequest {
	return YeuCauPhan(b, he, []*genai.Part{{Text: noiDung}}, schema, maxRa)
}

// Anh is one image as a request part: the bytes inline, with the MIME type
// the caller vouches for (a sanitised re-encode, never the uploader's claim).
func Anh(mime string, data []byte) *genai.Part {
	return &genai.Part{InlineData: &genai.Blob{MIMEType: mime, Data: data}}
}

// YeuCauPhan is YeuCau whose one user turn is the given parts: text and
// inline images (Anh), in the order the model should read them. A nil
// schema asks for JSON without constraining its shape.
func YeuCauPhan(b llm.LoaiGoi, he string, phan []*genai.Part, schema *genai.Schema, maxRa int32) *model.LLMRequest {
	return &model.LLMRequest{
		Model:    llm.Model,
		Contents: []*genai.Content{{Role: genai.RoleUser, Parts: phan}},
		Config: &genai.GenerateContentConfig{
			SystemInstruction: &genai.Content{Parts: []*genai.Part{{Text: he}}},
			ResponseMIMEType:  "application/json",
			ResponseSchema:    schema,
			MaxOutputTokens:   maxRa,
			ThinkingConfig:    llm.CauHinhNghi(b),
			SafetySettings:    agent.AnToan(),
		},
	}
}

// NhietDo is req at temperature t; the request passed in is not modified.
func NhietDo(req *model.LLMRequest, t float32) *model.LLMRequest {
	out := *req
	cfg := *req.Config
	cfg.Temperature = &t
	out.Config = &cfg
	return &out
}

// Them is req with two more turns: the model's earlier output and a user
// turn of data. The first request is not modified.
func Them(req *model.LLMRequest, cuaMoHinh, noiDung string) *model.LLMRequest {
	out := *req
	out.Contents = append(append([]*genai.Content(nil), req.Contents...),
		&genai.Content{Role: genai.RoleModel, Parts: []*genai.Part{{Text: cuaMoHinh}}},
		&genai.Content{Role: genai.RoleUser, Parts: []*genai.Part{{Text: noiDung}}},
	)
	return &out
}

// Goi makes one call through dem and returns the answer's text, thoughts
// left out. A provider or budget error is returned as it came.
func Goi(ctx context.Context, dem *llm.Dem, req *model.LLMRequest) (string, error) {
	var b strings.Builder
	for resp, err := range dem.GenerateContent(ctx, req, false) {
		if err != nil {
			return "", err
		}
		if resp == nil {
			continue
		}
		if llm.BiChanAnToan(resp.FinishReason) {
			return "", ErrBiChan
		}
		if resp.Content == nil {
			continue
		}
		for _, p := range resp.Content.Parts {
			if p != nil && !p.Thought {
				b.WriteString(p.Text)
			}
		}
	}
	return b.String(), nil
}

// MaxRuneTruong bounds one evidence field as shown to a model.
const MaxRuneTruong = 300

// DongBangChung is one evidence item as a model reads it: its alias, then
// each field, keys sorted, values on one line, datamarked and cut at MaxRuneTruong. Keys
// are ours (the adapter's field names); values came from outside.
func DongBangChung(biDanh string, b truyhoi.BangChung) string {
	keys := make([]string, 0, len(b.Truong))
	for k := range b.Truong {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var s strings.Builder
	s.WriteString(biDanh)
	for _, k := range keys {
		// One line per item: a line break in a value is a space, so data
		// cannot start a line of its own.
		v := strings.Join(strings.Fields(b.Truong[k]), " ")
		if utf8.RuneCountInString(v) > MaxRuneTruong {
			v = string([]rune(v)[:MaxRuneTruong])
		}
		s.WriteString(" | " + k + ": " + prompts.DanhDau(v))
	}
	return s.String()
}

// KhoiBangChung lays evidence into one block named n, one line per item,
// under the aliases bi gives.
func KhoiBangChung(n prompts.Nguon, bc []truyhoi.BangChung, bi func(i int, b truyhoi.BangChung) string) string {
	lines := make([]string, len(bc))
	for i, b := range bc {
		lines[i] = DongBangChung(bi(i, b), b)
	}
	return prompts.BocDuLieu(n, strings.Join(lines, "\n"))
}

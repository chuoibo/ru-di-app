package llm

import (
	"context"
	"errors"

	"google.golang.org/genai"

	"mobile/services/core/internal/aiharness/obs"
)

// PhanLoai reduces a model error to its class. The message is dropped on
// purpose: a provider's error text can echo the key or the question.
func PhanLoai(err error) obs.LoiMoHinh {
	if err == nil {
		return obs.LoiKhong
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return obs.LoiTimeout
	}
	if errors.Is(err, ErrKhongUngVien) {
		return obs.LoiSafety
	}
	code := 0
	var v genai.APIError
	var p *genai.APIError
	switch {
	case errors.As(err, &v):
		code = v.Code
	case errors.As(err, &p) && p != nil:
		code = p.Code
	}
	switch {
	case code == 429:
		return obs.Loi429
	case code >= 500 && code <= 599:
		return obs.Loi5xx
	case code == 408:
		return obs.LoiTimeout
	}
	return obs.LoiKhac
}

// BiChanAnToan says whether a finish reason means the provider's own safety
// filter withheld the answer.
func BiChanAnToan(r genai.FinishReason) bool {
	switch r {
	case genai.FinishReasonSafety, genai.FinishReasonProhibitedContent, genai.FinishReasonBlocklist, genai.FinishReasonSPII:
		return true
	}
	return false
}

package kiemchung

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"strings"

	"google.golang.org/adk/v2/model"

	"mobile/services/core/internal/aiharness/cautruc"
	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/prompts"
	"mobile/services/core/internal/aiharness/truyhoi"
)

// kiemHe is the verifier's static system instruction. It frames the answer
// as another assistant's (research reflection-verification §2.2: a model
// reviewing its own context is the weakest reviewer) and defines the two
// flags that replace the output guard's phrase rules.
//
//go:embed kiem.txt
var kiemHe string

// Data block names of the verifier's user turn. Ours, never data.
const (
	KhoiCau       prompts.Nguon = "cau_tra_loi"
	KhoiBangChung prompts.Nguon = "bang_chung"
)

// MaxTokensKiem bounds the verifier's output: MaxMenhDe short objects and
// two booleans.
const MaxTokensKiem = 512

// ErrKhongCau: nothing to verify.
var ErrKhongCau = errors.New("kiemchung: no sentence to verify")

// VerifierLLM is the verifier: ONE flash-lite call for the whole answer, in
// a fresh context (no history, no instruction of the writer), the evidence
// under local aliases e1, e2, … offered as a closed enum, its output read by
// Doc. What it returns cites real ids again (the aliases mapped back).
type VerifierLLM struct{}

var _ Verifier = VerifierLLM{}

// BiDanhKiem is evidence i's alias in the verifier's prompt.
func BiDanhKiem(i int) string { return fmt.Sprintf("e%d", i+1) }

// YeuCauKiem is the verifier's request, byte-stable for the same input.
func YeuCauKiem(cau []string, bc []truyhoi.BangChung) *model.LLMRequest {
	return cautruc.YeuCau(strings.TrimSpace(kiemHe), NoiDungKiem(cau, bc), LuocDo(len(cau), biDanhs(len(bc))), MaxTokensKiem)
}

// NoiDungKiem is the verifier's user turn: the numbered sentences, then the
// evidence. Both came from outside the instruction and are datamarked.
func NoiDungKiem(cau []string, bc []truyhoi.BangChung) string {
	lines := make([]string, len(cau))
	for i, c := range cau {
		lines[i] = fmt.Sprintf("%d. %s", i+1, prompts.DanhDau(strings.Join(strings.Fields(c), " ")))
	}
	return strings.Join([]string{
		prompts.BocDuLieu(KhoiCau, strings.Join(lines, "\n")),
		cautruc.KhoiBangChung(KhoiBangChung, bc, func(i int, _ truyhoi.BangChung) string { return BiDanhKiem(i) }),
		"Chấm câu trả lời trên theo schema.",
	}, "\n\n")
}

func biDanhs(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = BiDanhKiem(i)
	}
	return out
}

// PhanTu makes the one verifier call through dem. A refused output is
// returned as an error wrapping ErrCauTruc: the caller must not release the
// answer on it.
func (VerifierLLM) PhanTu(ctx context.Context, cau []string, bc []truyhoi.BangChung, dem *llm.Dem) (PhanTu, error) {
	if len(cau) == 0 {
		return PhanTu{}, ErrKhongCau
	}
	if len(cau) > MaxMenhDe {
		return PhanTu{}, fmt.Errorf("%w: %d sentences", ErrCauTruc, len(cau))
	}
	raw, err := cautruc.Goi(ctx, dem, YeuCauKiem(cau, bc))
	if err != nil {
		return PhanTu{}, err
	}
	p, err := Doc([]byte(raw), len(cau), biDanhs(len(bc)))
	if err != nil {
		return PhanTu{}, err
	}
	tu := make(map[string]string, len(bc))
	for i, b := range bc {
		tu[BiDanhKiem(i)] = b.ID
	}
	for i := range p.MenhDe {
		for j, a := range p.MenhDe[i].BangChungIDs {
			p.MenhDe[i].BangChungIDs[j] = tu[a]
		}
	}
	return p, nil
}

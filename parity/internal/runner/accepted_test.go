package runner

import (
	"net/http"
	"testing"

	"mobile/parity/internal/compare"
)

func TestAcceptedCountsCountsStepsNotHeaders(t *testing.T) {
	step := func(status int, contentLength string) StepResult {
		h := http.Header{}
		if contentLength != "" {
			h.Set("Content-Length", contentLength)
		}
		return StepResult{Norm: compare.Exchange{Status: status, Header: h}}
	}
	reference := &Run{Steps: []StepResult{step(204, ""), step(204, "0"), step(200, "2"), step(204, "0")}}
	candidate := &Run{Steps: []StepResult{step(204, ""), step(204, ""), step(200, "2"), step(204, "")}}
	counts := AcceptedCounts(reference, candidate)
	if counts[compare.Response204ContentLength] != 2 || len(counts) != 1 {
		t.Fatalf("counts = %v", counts)
	}
	if got := AcceptedCounts(reference, reference); len(got) != 0 {
		t.Fatalf("identical runs accepted %v", got)
	}
}

package screenshot

import (
	"errors"
	"testing"

	"mobile/services/core/internal/oracletest"
)

// testdata/python_screenshot*.json was rendered by scripts/render_domain_ai_goldens.py
// from the real app.domain.screenshot before ADR-0052 deleted it.

func refusal(err error) (class, code string, ok bool) {
	var e *Error
	if errors.As(err, &e) {
		return "ScreenshotError", e.Code, true
	}
	if errors.Is(err, ErrUnhashable) {
		return "TypeError", "unhashable type: 'list'", true
	}
	return "", "", false
}

func TestScreenshotMatchesPython(t *testing.T) {
	report := oracletest.Agree(t, oracletest.Load(t, "testdata/python_*.json"), "screenshot", replay, refusal)
	if report.ByFn["read_screenshot"] == nil {
		t.Fatal("no cases")
	}
}

func replay(c oracletest.Case, args map[string]any) (any, error) {
	if c.Fn != "read_screenshot" {
		return nil, oracletest.Decode(errors.New(c.Fn))
	}
	r, err := Read(args["raw"])
	if err != nil {
		return nil, err
	}
	var day any
	if r.OccurredOn != nil {
		day = *r.OccurredOn
	}
	return map[string]any{"source": r.Source, "merchant": r.Merchant, "total_vnd": r.TotalVND, "occurred_on": day, "needs_review": true}, nil
}

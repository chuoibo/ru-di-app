package chatexpense

import (
	"errors"
	"testing"

	"mobile/services/core/internal/oracletest"
)

// testdata/python_chat_expense*.json was rendered by scripts/render_domain_ai_goldens.py
// from the real app.domain.chat_expense before ADR-0051 deleted it.

func refusal(err error) (class, code string, ok bool) {
	var e *Error
	if errors.As(err, &e) {
		return "ChatExpenseError", e.Code, true
	}
	return "", "", false
}

func TestChatExpenseMatchesPython(t *testing.T) {
	report := oracletest.Agree(t, oracletest.Load(t, "testdata/python_*.json"), "chat_expense", replay, refusal)
	if report.ByFn["read_chat_expense"] == nil {
		t.Fatal("no cases")
	}
}

func TestConstantsMatchPython(t *testing.T) {
	c := oracletest.Constants(t, oracletest.Load(t, "testdata/python_chat_expense.json"), "chat_expense")
	if c["MAX_CHAT_EXPENSE_TITLE_LENGTH"] != int64(MaxTitleLength) {
		t.Fatalf("constants: %v", c)
	}
}

func replay(c oracletest.Case, args map[string]any) (any, error) {
	if c.Fn != "read_chat_expense" {
		return nil, oracletest.Decode(errors.New(c.Fn))
	}
	r, err := Read(args["raw"])
	if err != nil {
		return nil, err
	}
	if !r.IsExpense {
		return map[string]any{"is_expense": false, "title": nil, "amount_vnd": nil, "needs_review": false}, nil
	}
	return map[string]any{"is_expense": true, "title": r.Title, "amount_vnd": r.AmountVND, "needs_review": true}, nil
}

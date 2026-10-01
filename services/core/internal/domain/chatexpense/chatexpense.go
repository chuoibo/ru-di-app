// Package chatexpense reads what a model made of one chat message: whether
// it reports an expense its writer paid, a title and the amount as written.
// The Go port of services/api/app/domain/chat_expense.py (ADR-0051), pinned
// to it by testdata/python_chat_expense*.json. Identity is absent on
// purpose: the author and the roster are the server's, never the model's.
package chatexpense

import (
	"unicode/utf8"

	"mobile/services/core/internal/domain/receipt"
)

// MaxTitleLength bounds a draft's title, in characters.
const MaxTitleLength = 200

// Error is one stable refusal: UNREADABLE or MODEL_NAMED_A_PERSON.
type Error struct{ Code string }

func (e *Error) Error() string { return e.Code }

func unreadable() error { return &Error{Code: "UNREADABLE"} }

var contractKeys = map[string]bool{"is_expense": true, "title": true, "amount_text": true}

// Reading is one message's answer. A true reading is always a draft.
type Reading struct {
	IsExpense bool
	Title     string
	AmountVND int64
}

// Read normalizes one raw reading without accepting identity from the model.
func Read(raw any) (Reading, error) {
	m, ok := raw.(map[string]any)
	if !ok {
		return Reading{}, unreadable()
	}
	for key := range m {
		if receipt.LooksLikeIdentityKey(key, contractKeys) {
			return Reading{}, &Error{Code: "MODEL_NAMED_A_PERSON"}
		}
	}
	isExpense, ok := m["is_expense"].(bool)
	if !ok {
		return Reading{}, unreadable()
	}
	if !isExpense {
		return Reading{}, nil
	}
	title, ok := m["title"].(string)
	if !ok {
		return Reading{}, unreadable()
	}
	title = receipt.PyStrip(title)
	if title == "" || utf8.RuneCountInString(title) > MaxTitleLength {
		return Reading{}, unreadable()
	}
	// 180000.0 has already crossed the money boundary, however integral it
	// looks: only a string is read as an amount.
	amountText, ok := m["amount_text"].(string)
	if !ok {
		return Reading{}, unreadable()
	}
	amount, err := receipt.NormalizeVND(amountText)
	if err != nil || amount <= 0 || amount > receipt.MaxAmountVND {
		return Reading{}, unreadable()
	}
	return Reading{IsExpense: true, Title: title, AmountVND: amount}, nil
}

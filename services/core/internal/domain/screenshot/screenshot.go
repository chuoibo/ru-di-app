// Package screenshot reads what a model transcribed off a transaction
// screenshot (Grab, ShopeeFood, a banking app). The Go port of
// services/api/app/domain/screenshot.py (ADR-0052), pinned to it by
// testdata/python_screenshot*.json. Identity is deliberately absent: a
// screenshot is evidence about a transaction, never authority for who paid.
package screenshot

import (
	"errors"
	"regexp"
	"time"

	"mobile/services/core/internal/domain/receipt"
)

// Error is one stable refusal: UNREADABLE, NOT_A_TRANSACTION,
// MODEL_NAMED_A_PERSON.
type Error struct{ Code string }

func (e *Error) Error() string { return e.Code }

func unreadable() error { return &Error{Code: "UNREADABLE"} }

// ErrUnhashable is a source Python cannot even look up in its set (a list or
// an object): a TypeError there, not a refusal, and a failed reading here.
var ErrUnhashable = errors.New("screenshot: source is not a scalar")

var (
	sources      = map[string]bool{"grab": true, "shopeefood": true, "banking": true, "receipt": true}
	contractKeys = map[string]bool{"source": true, "merchant": true, "total_text": true, "occurred_on": true}
	datePattern  = regexp.MustCompile(`^\p{Nd}{4}-\p{Nd}{2}-\p{Nd}{2}$`)
	asciiDate    = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}$`)
)

// Reading is a normalized transaction; always a draft (needs review).
type Reading struct {
	Source     string
	Merchant   string
	TotalVND   int64
	OccurredOn *string
}

func readOccurredOn(raw map[string]any) (*string, error) {
	v := raw["occurred_on"]
	if v == nil {
		return nil, nil
	}
	s, ok := v.(string)
	if !ok || !datePattern.MatchString(s) {
		return nil, unreadable()
	}
	// date.fromisoformat reads ASCII digits only, and a real calendar day.
	if !asciiDate.MatchString(s) {
		return nil, unreadable()
	}
	if _, err := time.Parse("2006-01-02", s); err != nil {
		return nil, unreadable()
	}
	return &s, nil
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
	switch m["source"].(type) {
	case []any, map[string]any:
		return Reading{}, ErrUnhashable
	}
	source, _ := m["source"].(string)
	if m["source"] == "other" {
		return Reading{}, &Error{Code: "NOT_A_TRANSACTION"}
	}
	if !sources[source] {
		return Reading{}, unreadable()
	}
	merchant, ok := m["merchant"].(string)
	if !ok {
		return Reading{}, unreadable()
	}
	merchant = receipt.PyStrip(merchant)
	if merchant == "" {
		return Reading{}, unreadable()
	}
	// Never stringify model money: only a string is read as an amount.
	totalText, ok := m["total_text"].(string)
	if !ok {
		return Reading{}, unreadable()
	}
	total, err := receipt.NormalizeVND(totalText)
	if err != nil || total <= 0 || total > receipt.MaxAmountVND {
		return Reading{}, unreadable()
	}
	day, err := readOccurredOn(m)
	if err != nil {
		return Reading{}, err
	}
	return Reading{Source: source, Merchant: merchant, TotalVND: total, OccurredOn: day}, nil
}

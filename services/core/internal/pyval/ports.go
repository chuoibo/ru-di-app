package pyval

import (
	"errors"

	"mobile/services/core/internal/pyjson"
)

// registerServedValidators installs the Go ports of the app's own validator
// functions for routes Go serves. The oracle build (oracle_ports_test.go)
// still registers the ports of routes Python serves on top of these, and the
// route oracle checks both against the real schemas.
func registerServedValidators(r *Registry) {
	const schemas = "app.api.schemas."
	// W2 votes (schemas.py VoteCreateRequest, VoteOptionInput).
	r.Register(schemas+"VoteCreateRequest._strip_question", stripNotBlank("question must not be blank"))
	r.Register(schemas+"VoteOptionInput._strip_label", stripNotBlank("label must not be blank"))
	r.Register(schemas+"VoteOptionInput._strip_place_name", stripOrNone)
}

// stripNotBlank is `value.strip()`, refusing an empty result with message.
func stripNotBlank(message string) ValidatorFunc {
	return func(_ *Call, v Value) (Value, error) {
		s, ok := v.(pyjson.String)
		if !ok {
			return nil, errors.New("pyval: strip validator expected str")
		}
		stripped := pyStrip(string(s))
		if stripped == "" {
			return nil, ValueError(message)
		}
		return pyjson.String(stripped), nil
	}
}

// stripOrNone is `None if value is None else (value.strip() or None)`.
func stripOrNone(_ *Call, v Value) (Value, error) {
	s, ok := v.(pyjson.String)
	if !ok {
		return pyjson.Null{}, nil
	}
	if stripped := pyStrip(string(s)); stripped != "" {
		return pyjson.String(stripped), nil
	}
	return pyjson.Null{}, nil
}

package pyval

import (
	"errors"
	"strings"

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
	// W3 contexts (schemas.py ContextUpdateRequest).
	r.Register(schemas+"ContextUpdateRequest._something_to_change",
		somethingToChange([]string{"display_name", "theme"}, "tên nhóm không được rỗng"))
	// W4 money (schemas.py _require_timezone and BillDiscountCreateRequest;
	// routes/budget.py _parse_candidate_money).
	r.Register(schemas+"_require_timezone", requireTimezone)
	r.Register(schemas+"BillDiscountCreateRequest._target_matches_scope", targetMatchesScope)
	r.Register("app.api.routes.budget._parse_candidate_money", parseCandidateMoney)
}

// requireTimezone is _require_timezone: a datetime without a UTC offset is
// refused.
func requireTimezone(_ *Call, v Value) (Value, error) {
	if dt, ok := v.(DateTime); !ok || !dt.Aware {
		return nil, ValueError("datetime must include a UTC offset")
	}
	return v, nil
}

// targetMatchesScope is BillDiscountCreateRequest._target_matches_scope: an
// item-scoped discount names its item and a global one does not.
func targetMatchesScope(_ *Call, v Value) (Value, error) {
	if (validatedField(v, "scope") == pyjson.String("item")) != !isNone(validatedField(v, "item_key")) {
		return nil, ValueError("an item-scoped discount needs item_key and a global one must not carry it")
	}
	return v, nil
}

// parseCandidateMoney is routes/budget.py _parse_candidate_money: an ASCII
// digit string becomes its int; anything else passes through, for the strict
// int check after it to refuse.
func parseCandidateMoney(_ *Call, v Value) (Value, error) {
	s, ok := v.(pyjson.String)
	if !ok || s == "" {
		return v, nil
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return v, nil
		}
	}
	digits := strings.TrimLeft(string(s), "0")
	if digits == "" {
		return pyjson.NewInt(0), nil
	}
	n, _ := pyjson.ParseInt(digits)
	return n, nil
}

// somethingToChange is a partial-update model's `_something_to_change`: at
// least one of fields is not None, and a display_name that is given is not
// blank once stripped.
func somethingToChange(fields []string, blankName string) ValidatorFunc {
	return func(_ *Call, v Value) (Value, error) {
		allNone := true
		for _, name := range fields {
			if !isNone(validatedField(v, name)) {
				allNone = false
			}
		}
		if allNone {
			return nil, ValueError("cần ít nhất một trường để sửa")
		}
		if s, ok := validatedField(v, "display_name").(pyjson.String); ok && pyStrip(string(s)) == "" {
			return nil, ValueError(blankName)
		}
		return v, nil
	}
}

// validatedField is one validated field of a model value, nil when v is not a
// model or has no such field.
func validatedField(v Value, name string) Value {
	m, ok := v.(*Model)
	if !ok {
		return nil
	}
	got, _ := m.Get(name)
	return got
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

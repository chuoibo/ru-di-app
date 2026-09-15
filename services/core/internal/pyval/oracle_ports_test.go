//go:build oracle

package pyval

// Test-only Go ports of the validator functions the API's schemas call, so
// the route oracle can bind those routes and prove the registry mechanism
// (function-before/after, field and model validators, value_error in ctx)
// against the real Python. They are copied from services/api/app/api
// schemas.py, routes/places.py and routes/budget.py; a route wave that
// serves one of these routes from Go owns its production port.

import (
	"errors"
	"math/big"
	"strings"
	"unicode"

	"mobile/services/core/internal/pyjson"
)

func registerAppPorts(reg *Registry) {
	blank := func(message string, keepUnstripped bool) ValidatorFunc {
		return func(_ *Call, v Value) (Value, error) {
			s, ok := v.(pyjson.String)
			if !ok {
				return nil, errors.New("port: expected str")
			}
			stripped := pyStrip(string(s))
			if stripped == "" {
				return nil, ValueError(message)
			}
			if keepUnstripped {
				return s, nil
			}
			return pyjson.String(stripped), nil
		}
	}
	stripOrNoneTest := func(_ *Call, v Value) (Value, error) {
		s, ok := v.(pyjson.String)
		if !ok {
			return pyjson.Null{}, nil
		}
		if stripped := pyStrip(string(s)); stripped != "" {
			return pyjson.String(stripped), nil
		}
		return pyjson.Null{}, nil
	}
	const schemas = "app.api.schemas."
	reg.Register(schemas+"_require_timezone", func(_ *Call, v Value) (Value, error) {
		if dt, ok := v.(DateTime); !ok || !dt.Aware {
			return nil, ValueError("datetime must include a UTC offset")
		}
		return v, nil
	})
	reg.Register(schemas+"OutingCreateRequest._strip_title", blank("title must not be blank", false))
	reg.Register(schemas+"PairConstraintPutRequest._khong_rong", blank("content must not be blank", false))
	reg.Register(schemas+"PaperKeepRequest._khong_rong", blank("line must not be blank", false))
	reg.Register("app.api.routes.places.PlaceSearchRequest._reject_blank", blank("query must not be blank", false))
	reg.Register(schemas+"MeetingPoint._not_blank", blank("point label must not be blank", false))
	reg.Register(schemas+"OutingStopInput._strip_label", blank("label must not be blank", false))
	reg.Register(schemas+"OutingStopInput._strip_place_name", stripOrNoneTest)

	field := func(v Value, name string) Value {
		m, ok := v.(*Model)
		if !ok {
			return nil
		}
		got, _ := m.Get(name)
		return got
	}
	reg.Register(schemas+"OutingCreateRequest._dates_are_in_order", func(_ *Call, v Value) (Value, error) {
		start, _ := field(v, "starts_on").(Date)
		end, _ := field(v, "ends_on").(Date)
		if end.ISOFormat() < start.ISOFormat() {
			return nil, ValueError("ends_on must be on or after starts_on")
		}
		return v, nil
	})
	somethingToChange := func(fields []string, blankName string) ValidatorFunc {
		return func(_ *Call, v Value) (Value, error) {
			allNone := true
			for _, f := range fields {
				if !isNone(field(v, f)) {
					allNone = false
				}
			}
			if allNone {
				return nil, ValueError("c\u1ea7n \u00edt nh\u1ea5t m\u1ed9t tr\u01b0\u1eddng \u0111\u1ec3 s\u1eeda")
			}
			if s, ok := field(v, "display_name").(pyjson.String); ok && pyStrip(string(s)) == "" {
				return nil, ValueError(blankName)
			}
			return v, nil
		}
	}
	reg.Register(schemas+"ProfileUpdateRequest._something_to_change",
		somethingToChange([]string{"display_name", "bio", "city", "wall_comment_policy", "discoverable_by_phone"}, "t\u00ean hi\u1ec3n th\u1ecb kh\u00f4ng \u0111\u01b0\u1ee3c r\u1ed7ng"))
	reg.Register(schemas+"BillDiscountCreateRequest._target_matches_scope", func(_ *Call, v Value) (Value, error) {
		if (field(v, "scope") == pyjson.String("item")) != !isNone(field(v, "item_key")) {
			return nil, ValueError("an item-scoped discount needs item_key and a global one must not carry it")
		}
		return v, nil
	})
	reg.Register(schemas+"ItineraryRequest._unique_keys", func(_ *Call, v Value) (Value, error) {
		ids := map[string]bool{}
		stops, _ := field(v, "stops").(List)
		for _, s := range stops {
			ids[hashKey(field(s, "id"))] = true
		}
		if len(ids) != len(stops) {
			return nil, ValueError("duplicate stop ids")
		}
		days := map[string]bool{}
		list, _ := field(v, "days").(List)
		for _, d := range list {
			days[hashKey(field(d, "day"))] = true
		}
		if len(days) != len(list) {
			return nil, ValueError("duplicate days")
		}
		return v, nil
	})
	reg.Register(schemas+"ItineraryStopInput._one_location", func(_ *Call, v Value) (Value, error) {
		if !isNone(field(v, "place_id")) && !isNone(field(v, "meeting_point")) {
			return nil, ValueError("choose a catalogue place or a meeting point")
		}
		return v, nil
	})
	reg.Register(schemas+"ItineraryStopInput._valid_id", func(_ *Call, v Value) (Value, error) {
		s := string(v.(pyjson.String))
		if strings.HasPrefix(s, "tmp-") && codePoints(s) > 4 {
			return v, nil
		}
		u, ok := pythonUUID(s)
		if !ok {
			return nil, ValueError("stop id must be a UUID or a temporary draft id")
		}
		return pyjson.String(u.String()), nil
	})
	reg.Register(schemas+"OutingInviteCreateRequest._person_matches_source", func(_ *Call, v Value) (Value, error) {
		link := field(v, "source") == pyjson.String("link")
		named := !isNone(field(v, "person_id"))
		if link && named {
			return nil, ValueError("a link invite must not name a person")
		}
		if !link && !named {
			return nil, ValueError("a group or friend invite must name a person")
		}
		return v, nil
	})
	reg.Register("app.api.routes.budget._parse_candidate_money", func(_ *Call, v Value) (Value, error) {
		s, ok := v.(pyjson.String)
		if !ok || s == "" {
			return v, nil
		}
		for i := 0; i < len(s); i++ {
			if s[i] < '0' || s[i] > '9' {
				return v, nil
			}
		}
		n, _ := pyjson.ParseInt(strings.TrimLeft(string(s), "0"))
		if strings.TrimLeft(string(s), "0") == "" {
			n = pyjson.NewInt(0)
		}
		return n, nil
	})
}

// pythonUUID is uuid.UUID(hex): drop "urn:" and "uuid:", strip braces from
// both ends, drop hyphens, require 32 code points, then int(hex, 16) (which
// allows surrounding whitespace, a sign, a 0x prefix and single underscores
// between digits) within 128 bits.
func pythonUUID(s string) (UUID, bool) {
	s = strings.ReplaceAll(strings.ReplaceAll(s, "urn:", ""), "uuid:", "")
	s = strings.Trim(s, "{}")
	s = strings.ReplaceAll(s, "-", "")
	if codePoints(s) != 32 {
		return UUID{}, false
	}
	body := pyStrip(s)
	neg := false
	if body != "" && (body[0] == '+' || body[0] == '-') {
		neg = body[0] == '-'
		body = body[1:]
	}
	if len(body) >= 2 && body[0] == '0' && (body[1] == 'x' || body[1] == 'X') {
		body = body[2:]
		if strings.HasPrefix(body, "_") {
			body = body[1:]
		}
	}
	if body == "" || strings.HasPrefix(body, "_") || strings.HasSuffix(body, "_") || strings.Contains(body, "__") {
		return UUID{}, false
	}
	n := new(big.Int)
	for _, r := range body {
		if r == '_' {
			continue
		}
		d := hexValue(r)
		if d < 0 {
			return UUID{}, false
		}
		n.Mul(n, big.NewInt(16))
		n.Add(n, big.NewInt(int64(d)))
	}
	if neg && n.Sign() != 0 {
		return UUID{}, false
	}
	if n.BitLen() > 128 {
		return UUID{}, false
	}
	var u UUID
	n.FillBytes(u[:])
	return u, true
}

func hexValue(r rune) int {
	switch {
	case r >= '0' && r <= '9':
		return int(r - '0')
	case r >= 'a' && r <= 'f':
		return int(r-'a') + 10
	case r >= 'A' && r <= 'F':
		return int(r-'A') + 10
	case unicode.Is(unicode.Nd, r):
		for _, rg := range unicode.Nd.R16 {
			if rune(rg.Lo) <= r && r <= rune(rg.Hi) {
				return int(r-rune(rg.Lo)) % 10
			}
		}
		for _, rg := range unicode.Nd.R32 {
			if rune(rg.Lo) <= r && r <= rune(rg.Hi) {
				return int(r-rune(rg.Lo)) % 10
			}
		}
	}
	return -1
}

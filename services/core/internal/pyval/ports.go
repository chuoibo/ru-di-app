package pyval

import (
	"errors"
	"fmt"
	"math/big"
	"strings"
	"unicode"

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
	// W8 pair notebooks and papers (schemas.py PairConstraintPutRequest,
	// PaperKeepRequest): the field is bounded on the raw string, then stripped
	// and refused when nothing is left.
	r.Register(schemas+"PairConstraintPutRequest._khong_rong", stripNotBlank("content must not be blank"))
	r.Register(schemas+"PaperKeepRequest._khong_rong", stripNotBlank("line must not be blank"))
	// W10 people (schemas.py ProfileUpdateRequest).
	r.Register(schemas+"ProfileUpdateRequest._something_to_change",
		somethingToChange([]string{"display_name", "bio", "city", "wall_comment_policy", "discoverable_by_phone"},
			"tên hiển thị không được rỗng"))
	// W7 outings (schemas.py itinerary, timeline, invite, create).
	r.Register(schemas+"OutingCreateRequest._strip_title", stripNotBlank("title must not be blank"))
	r.Register(schemas+"OutingCreateRequest._dates_are_in_order", outingDatesInOrder)
	r.Register(schemas+"OutingStopInput._strip_label", stripNotBlank("label must not be blank"))
	r.Register(schemas+"OutingStopInput._strip_place_name", stripOrNone)
	r.Register(schemas+"MeetingPoint._not_blank", stripNotBlank("point label must not be blank"))
	r.Register(schemas+"ItineraryStopInput._valid_id", itineraryStopID)
	r.Register(schemas+"ItineraryStopInput._one_location", itineraryOneLocation)
	r.Register(schemas+"ItineraryRequest._unique_keys", itineraryUniqueKeys)
	r.Register(schemas+"OutingInviteCreateRequest._person_matches_source", invitePersonMatchesSource)
	// WAI places search: whitespace-only query must not reach the model.
	r.Register("app.api.routes.places.PlaceSearchRequest._reject_blank", stripNotBlank("query must not be blank"))
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

// maxIntStrDigits is CPython's default sys.get_int_max_str_digits().
const maxIntStrDigits = 4300

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
	// int() refuses a decimal string past sys.get_int_max_str_digits(), 4300 by
	// default, counting leading zeros; pydantic reports the ValueError.
	if len(s) > maxIntStrDigits {
		return nil, ValueError(fmt.Sprintf("Exceeds the limit (%d digits) for integer string conversion: value has %d digits; use sys.set_int_max_str_digits() to increase the limit", maxIntStrDigits, len(s)))
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

func outingDatesInOrder(_ *Call, v Value) (Value, error) {
	start, _ := validatedField(v, "starts_on").(Date)
	end, _ := validatedField(v, "ends_on").(Date)
	if end.ISOFormat() < start.ISOFormat() {
		return nil, ValueError("ends_on must be on or after starts_on")
	}
	return v, nil
}

func itineraryUniqueKeys(_ *Call, v Value) (Value, error) {
	ids := map[string]bool{}
	stops, _ := validatedField(v, "stops").(List)
	for _, stop := range stops {
		ids[hashKey(validatedField(stop, "id"))] = true
	}
	if len(ids) != len(stops) {
		return nil, ValueError("duplicate stop ids")
	}
	days := map[string]bool{}
	list, _ := validatedField(v, "days").(List)
	for _, day := range list {
		days[hashKey(validatedField(day, "day"))] = true
	}
	if len(days) != len(list) {
		return nil, ValueError("duplicate days")
	}
	return v, nil
}

func itineraryOneLocation(_ *Call, v Value) (Value, error) {
	if !isNone(validatedField(v, "place_id")) && !isNone(validatedField(v, "meeting_point")) {
		return nil, ValueError("choose a catalogue place or a meeting point")
	}
	return v, nil
}

func itineraryStopID(_ *Call, v Value) (Value, error) {
	s, ok := v.(pyjson.String)
	if !ok {
		return nil, errors.New("pyval: itinerary stop id expected str")
	}
	text := string(s)
	if strings.HasPrefix(text, "tmp-") && codePoints(text) > 4 {
		return v, nil
	}
	u, ok := pythonUUID(text)
	if !ok {
		return nil, ValueError("stop id must be a UUID or a temporary draft id")
	}
	return pyjson.String(u.String()), nil
}

func invitePersonMatchesSource(_ *Call, v Value) (Value, error) {
	link := validatedField(v, "source") == pyjson.String("link")
	named := !isNone(validatedField(v, "person_id"))
	if link && named {
		return nil, ValueError("a link invite must not name a person")
	}
	if !link && !named {
		return nil, ValueError("a group or friend invite must name a person")
	}
	return v, nil
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

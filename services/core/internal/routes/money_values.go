package routes

import (
	"fmt"

	"mobile/services/core/internal/domain/moneysteps"
	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/pyval"
)

// Readers of the validated values the W4 money routes take, and the JSON dump
// of a validated model the proposal echo needs. As in values.go, a value of a
// type the contract cannot produce is a bug in the port and ends the request.

// intField reads a strict int field exactly: MoneyVnd has no ceiling, so the
// value may pass int64.
func intField(model *pyval.Model, name string) (pyjson.Int, error) {
	value, err := field(model, name)
	if err != nil {
		return pyjson.Int{}, err
	}
	n, ok := value.(pyjson.Int)
	if !ok {
		return pyjson.Int{}, fmt.Errorf("routes: %s.%s is %T, not an int", model.Class, name, value)
	}
	return n, nil
}

func boolField(model *pyval.Model, name string) (bool, error) {
	value, err := field(model, name)
	if err != nil {
		return false, err
	}
	b, ok := value.(pyjson.Bool)
	if !ok {
		return false, fmt.Errorf("routes: %s.%s is %T, not a bool", model.Class, name, value)
	}
	return bool(b), nil
}

func modelField(model *pyval.Model, name string) (*pyval.Model, error) {
	value, err := field(model, name)
	if err != nil {
		return nil, err
	}
	m, ok := value.(*pyval.Model)
	if !ok {
		return nil, fmt.Errorf("routes: %s.%s is %T, not a model", model.Class, name, value)
	}
	return m, nil
}

func dateTimeField(model *pyval.Model, name string) (pyval.DateTime, error) {
	value, err := field(model, name)
	if err != nil {
		return pyval.DateTime{}, err
	}
	t, ok := value.(pyval.DateTime)
	if !ok {
		return pyval.DateTime{}, fmt.Errorf("routes: %s.%s is %T, not a datetime", model.Class, name, value)
	}
	return t, nil
}

// uuidListField reads a `list[UUID]` field as canonical strings, in order and
// with repeats kept.
func uuidListField(model *pyval.Model, name string) ([]string, error) {
	value, err := field(model, name)
	if err != nil {
		return nil, err
	}
	list, ok := value.(pyval.List)
	if !ok {
		return nil, fmt.Errorf("routes: %s.%s is %T, not a list", model.Class, name, value)
	}
	out := make([]string, len(list))
	for i, item := range list {
		id, ok := item.(pyval.UUID)
		if !ok {
			return nil, fmt.Errorf("routes: %s.%s[%d] is %T, not a UUID", model.Class, name, i, item)
		}
		out[i] = id.String()
	}
	return out, nil
}

// optionalIntParam reads an `int | None` path or query value exactly.
func optionalIntParam(call *endpoint.Call, name string) (*pyjson.Int, error) {
	switch value := call.Values[name].(type) {
	case pyjson.Null:
		return nil, nil
	case pyjson.Int:
		return &value, nil
	}
	return nil, fmt.Errorf("routes: parameter %q is %T, not an int or None", name, call.Values[name])
}

// refuseMoney is the ApiProblem a moneysteps refusal stands for.
func refuseMoney(refused *moneysteps.Refusal) error {
	return endpoint.Refuse(refused.Status, refused.Code, refused.Detail)
}

// dumpModel is `model.model_dump(mode="json")` for a model built from the
// kinds a request model of the money routes holds: every field in declaration
// order, defaults included, UUIDs as str(uuid), datetimes as pydantic writes
// them, nested models and lists dumped in turn.
func dumpModel(model *pyval.Model) (*pyjson.OrderedMap, error) {
	out := pyjson.NewOrderedMap()
	for _, f := range model.Fields {
		value, err := dumpValue(f.Value)
		if err != nil {
			return nil, fmt.Errorf("%s.%s: %w", model.Class, f.Name, err)
		}
		out.Set(f.Name, value)
	}
	return out, nil
}

func dumpValue(value pyval.Value) (pyjson.Value, error) {
	switch v := value.(type) {
	case pyjson.Null, pyjson.Bool, pyjson.Int, pyjson.String:
		return v.(pyjson.Value), nil
	case pyval.UUID:
		return pyjson.String(v.String()), nil
	case pyval.DateTime:
		if !v.Aware {
			return pyjson.String(pyjson.NaiveDateTime(v.Time())), nil
		}
		return pyjson.String(pyjson.DateTime(v.Time())), nil
	case pyval.List:
		out := make(pyjson.List, len(v))
		for i, item := range v {
			dumped, err := dumpValue(item)
			if err != nil {
				return nil, err
			}
			out[i] = dumped
		}
		return out, nil
	case *pyval.Model:
		return dumpModel(v)
	}
	return nil, fmt.Errorf("routes: no JSON dump for %T", value)
}

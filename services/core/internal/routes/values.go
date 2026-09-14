package routes

import (
	"fmt"

	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/pyval"
)

// The helpers below read what pyval validated. A value of a type the contract
// cannot produce is a bug in the port, not a client error, so it ends the
// request as an unhandled failure instead of being guessed at.

func bodyModel(call *endpoint.Call, name string) (*pyval.Model, error) {
	model, ok := call.Values[name].(*pyval.Model)
	if !ok {
		return nil, fmt.Errorf("routes: body %q is %T, not a model", name, call.Values[name])
	}
	return model, nil
}

func field(model *pyval.Model, name string) (pyval.Value, error) {
	value, ok := model.Get(name)
	if !ok {
		return nil, fmt.Errorf("routes: %s has no field %q", model.Class, name)
	}
	return value, nil
}

func stringField(model *pyval.Model, name string) (string, error) {
	value, err := field(model, name)
	if err != nil {
		return "", err
	}
	text, ok := value.(pyjson.String)
	if !ok {
		return "", fmt.Errorf("routes: %s.%s is %T, not a string", model.Class, name, value)
	}
	return string(text), nil
}

func optionalStringField(model *pyval.Model, name string) (*string, error) {
	value, err := field(model, name)
	if err != nil {
		return nil, err
	}
	switch v := value.(type) {
	case pyjson.Null:
		return nil, nil
	case pyjson.String:
		text := string(v)
		return &text, nil
	}
	return nil, fmt.Errorf("routes: %s.%s is %T, not a string or None", model.Class, name, value)
}

func uuidField(model *pyval.Model, name string) (string, error) {
	value, err := field(model, name)
	if err != nil {
		return "", err
	}
	id, ok := value.(pyval.UUID)
	if !ok {
		return "", fmt.Errorf("routes: %s.%s is %T, not a UUID", model.Class, name, value)
	}
	return id.String(), nil
}

func stringListField(model *pyval.Model, name string) ([]string, error) {
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
		text, ok := item.(pyjson.String)
		if !ok {
			return nil, fmt.Errorf("routes: %s.%s[%d] is %T, not a string", model.Class, name, i, item)
		}
		out[i] = string(text)
	}
	return out, nil
}

func textOrNull(value *string) pyjson.Value {
	if value == nil {
		return pyjson.Null{}
	}
	return pyjson.String(*value)
}

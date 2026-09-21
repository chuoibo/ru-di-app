package routes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"mobile/services/core/internal/brain"
	"mobile/services/core/internal/domain/blocking"
	"mobile/services/core/internal/domain/direct"
	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/limit"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/pyval"
	"mobile/services/core/internal/repo"
)

func spendActorWindow(call *endpoint.Call, window *limit.ActorWindow[string]) error {
	if call.Limits == nil || window == nil {
		return errNoLimits
	}
	decision := window.Check(call.Actor.ID)
	if !decision.Allowed {
		return &endpoint.Refusal{Problem: decision.Problem}
	}
	return nil
}

func optionalUUIDParam(call *endpoint.Call, name string) (*string, error) {
	value, ok := call.Values[name]
	if !ok {
		return nil, nil
	}
	switch v := value.(type) {
	case pyjson.Null:
		return nil, nil
	case pyval.UUID:
		id := v.String()
		return &id, nil
	}
	return nil, fmt.Errorf("routes: parameter %q is %T, not a UUID or None", name, call.Values[name])
}

func optionalFloatParam(call *endpoint.Call, name string) (*float64, error) {
	value, ok := call.Values[name]
	if !ok {
		return nil, nil
	}
	switch v := value.(type) {
	case pyjson.Null:
		return nil, nil
	case pyjson.Float:
		n := float64(v)
		return &n, nil
	case pyjson.Int:
		n, _ := v.Big().Float64()
		return &n, nil
	}
	return nil, fmt.Errorf("routes: parameter %q is %T, not a float or None", name, call.Values[name])
}

func cardValue(raw json.RawMessage) pyjson.Value {
	if len(raw) == 0 {
		return pyjson.Null{}
	}
	value, err := pyjson.Loads(raw)
	if err != nil {
		return pyjson.Null{}
	}
	return value
}

func cardBytes(value pyjson.Value) (json.RawMessage, error) {
	if value == nil {
		return nil, nil
	}
	if _, ok := value.(pyjson.Null); ok {
		return nil, nil
	}
	return pyjson.Compact(value)
}

func pythonRound1(x float64) float64 {
	return math.RoundToEven(x*10) / 10
}

func requirePairAlive(ctx context.Context, store repo.Repository, call *endpoint.Call, contextID string) error {
	record, err := store.GetContext(ctx, contextID)
	if err != nil {
		return err
	}
	if record == nil || !direct.IsPair(record.Kind) {
		return nil
	}
	members, err := store.ListMembers(ctx, contextID)
	if err != nil {
		return err
	}
	var otherID *string
	for _, member := range members {
		if member.PersonID != call.Actor.ID {
			id := member.PersonID
			otherID = &id
			break
		}
	}
	if otherID == nil {
		return endpoint.Refuse(409, blocking.DirectMessageUnavailable, "Cuộc trò chuyện này không còn nhận tin.")
	}
	other, err := store.GetPerson(ctx, *otherID)
	if err != nil {
		return err
	}
	edge, err := store.GetFriendEdge(ctx, call.Actor.ID, *otherID)
	if err != nil {
		return err
	}
	var blockingEdge *blocking.Edge
	if edge != nil {
		blockingEdge = &blocking.Edge{State: edge.State, DecidedByID: edge.DecidedByID}
	}
	if !blocking.DMAllowed(blockingEdge, other == nil || other.DeletedAt != nil) {
		return endpoint.Refuse(409, blocking.DirectMessageUnavailable, "Cuộc trò chuyện này không còn nhận tin.")
	}
	return nil
}

func optionalObjectField(model *pyval.Model, name string) (*pyjson.OrderedMap, error) {
	value, err := field(model, name)
	if err != nil {
		return nil, err
	}
	switch v := value.(type) {
	case pyjson.Null:
		return nil, nil
	case *pyjson.OrderedMap:
		return v, nil
	}
	return nil, fmt.Errorf("routes: %s.%s is %T, not an object or None", model.Class, name, value)
}

func optionalBodyModel(call *endpoint.Call, name string) (*pyval.Model, error) {
	value, ok := call.Values[name]
	if !ok || value == nil {
		return nil, nil
	}
	switch v := value.(type) {
	case pyjson.Null:
		return nil, nil
	case *pyval.Model:
		return v, nil
	}
	return nil, fmt.Errorf("routes: body %q is %T, not a model or None", name, value)
}

func uuidBytesLess(a, b string) bool {
	return bytes.Compare(uuidRaw(a), uuidRaw(b)) < 0
}

func uuidRaw(id string) []byte {
	hexes := make([]byte, 0, 32)
	for i := 0; i < len(id); i++ {
		if id[i] != '-' {
			hexes = append(hexes, id[i])
		}
	}
	out := make([]byte, len(hexes)/2)
	for i := 0; i+1 < len(hexes); i += 2 {
		hi, lo := unhex(hexes[i]), unhex(hexes[i+1])
		out[i/2] = hi<<4 | lo
	}
	return out
}

func unhex(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10
	}
	return 0
}

func brainErr(err error) *brain.Error {
	var refused *brain.Error
	if errors.As(err, &refused) {
		return refused
	}
	return nil
}

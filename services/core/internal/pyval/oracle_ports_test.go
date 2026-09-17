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
	reg.Register("app.api.routes.places.PlaceSearchRequest._reject_blank", blank("query must not be blank", false))
}

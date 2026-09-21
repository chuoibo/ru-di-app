// Package service holds the workflow steps of services/api/app/api/service.py
// that several routes share.
package service

import (
	"mobile/services/core/internal/auth"
	"mobile/services/core/internal/domain/permissions"
	"mobile/services/core/internal/httpapi/problem"
)

// Provenance names the layer that proved the predicates on every facts object.
const Provenance = "api_service"

// Resource is what a caller established about the resource an action touches:
// _require_permission's context dict. A predicate counts only when its value is
// true, as Python keeps a name only when its value `is True`.
type Resource struct {
	ID     *string
	Proven map[string]bool
}

// RequirePermission is _require_permission. A denial comes back as the 403 Python
// raises: code permission_denied, the reason as detail. An error is a facts or
// action failure Python never catches, so the caller must end the request as a
// crash (servererror.Raise), never answer it with a 403.
//
// extraRoles carries a role the service derived from the resource itself, never
// one read from a request.
func RequirePermission(action string, actor auth.Actor, resource Resource, extraRoles ...string) (*problem.Problem, error) {
	roles := make([]string, 0, len(actor.Roles)+len(extraRoles))
	roles = append(roles, actor.Roles...)
	roles = append(roles, extraRoles...)
	proven := make([]string, 0, len(resource.Proven))
	for name, proved := range resource.Proven {
		if proved {
			proven = append(proven, name)
		}
	}
	reason, allowed, err := permissions.DenialReason(action, permissions.AuthorizationFacts{
		ActorID:    actor.ID,
		Roles:      roles,
		ResourceID: resource.ID,
		Proven:     proven,
		Provenance: Provenance,
	})
	if err != nil {
		return nil, err
	}
	if allowed {
		return nil, nil
	}
	return &problem.Problem{Status: 403, Code: "permission_denied", Detail: reason}, nil
}

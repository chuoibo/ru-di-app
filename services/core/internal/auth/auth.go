// Package auth resolves who a request is from, the way services/api/app/api/
// deps.py does (ADR-0014, ADR-0029).
//
// In dev mode the identity is read from X-Actor-* headers. In prod mode it is
// a bearer session looked up by digest; this file holds the header half of
// that, and the database lookup lives with the repository.
//
// Every accept and refuse decision here is checked against the Python
// functions themselves through testdata/python_actor.json.
package auth

import (
	"net/http"
	"sort"
	"strings"
)

// Roles is app.domain.permissions.ROLES.
var Roles = map[string]bool{
	"group_admin": true, "batch_owner": true, "advancer": true, "recipient": true,
	"sender": true, "creditor": true, "member": true, "former_member": true,
	"guest": true, "platform_moderator": true,
}

// Problem is an ApiProblem: status, wire code, detail text.
type Problem struct {
	Status int
	Code   string
	Detail string
}

// Actor is who the request is answered as.
type Actor struct {
	ID       string   // canonical lowercase UUID
	Roles    []string // sorted, unique
	Contexts []string // sorted canonical UUIDs, unique
}

// firstHeader returns the first value and whether the header was sent at
// all; Starlette's Headers.get answers with the first occurrence.
func firstHeader(h http.Header, name string) (string, bool) {
	values, ok := h[http.CanonicalHeaderKey(name)]
	if !ok || len(values) == 0 {
		return "", false
	}
	return values[0], true
}

// csv is deps._csv: split on commas, strip each part, drop empty parts.
func csv(value string) []string {
	var parts []string
	for _, part := range strings.Split(value, ",") {
		if stripped := pyStripLatin1(part); stripped != "" {
			parts = append(parts, stripped)
		}
	}
	return parts
}

// DevActor is get_actor in dev mode. The checks run in Python's order: a
// missing id, then a malformed id, then unknown roles, then bad contexts.
func DevActor(h http.Header) (*Actor, *Problem) {
	rawID, sent := firstHeader(h, "X-Actor-ID")
	if !sent {
		return nil, &Problem{401, "authentication_required", "Missing X-Actor-ID"}
	}
	id, err := ParsePythonUUID(rawID)
	if err != nil {
		return nil, &Problem{422, "invalid_actor_id", "X-Actor-ID must be a UUID"}
	}

	roleSet := map[string]bool{}
	if rawRoles, ok := firstHeader(h, "X-Actor-Roles"); ok {
		for _, role := range csv(rawRoles) {
			roleSet[role] = true
		}
	}
	for role := range roleSet {
		if !Roles[role] {
			return nil, &Problem{422, "invalid_actor_roles", "X-Actor-Roles contains an unknown role"}
		}
	}

	contextSet := map[string]bool{}
	if rawContexts, ok := firstHeader(h, "X-Actor-Contexts"); ok {
		for _, value := range csv(rawContexts) {
			context, err := ParsePythonUUID(value)
			if err != nil {
				return nil, &Problem{422, "invalid_actor_contexts", "X-Actor-Contexts must contain comma-separated UUIDs"}
			}
			contextSet[context] = true
		}
	}
	return &Actor{ID: id, Roles: sortedKeys(roleSet), Contexts: sortedKeys(contextSet)}, nil
}

// DevActorOffered is get_actor_optional's question in dev mode: did the caller
// send an X-Actor-ID at all? When not, the request is anonymous.
func DevActorOffered(h http.Header) bool {
	_, sent := firstHeader(h, "X-Actor-ID")
	return sent
}

// BearerToken is deps.bearer_token: partition the Authorization header on
// its first space, compare the scheme case-insensitively, strip the token.
// A missing header and a malformed one answer identically.
func BearerToken(h http.Header) (string, *Problem) {
	missing := &Problem{401, "authentication_required", "Missing bearer session"}
	value, sent := firstHeader(h, "Authorization")
	if !sent {
		return "", missing
	}
	scheme, rest, _ := strings.Cut(value, " ")
	token := pyStripLatin1(rest)
	if strings.ToLower(scheme) != "bearer" || token == "" {
		return "", missing
	}
	return token, nil
}

func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

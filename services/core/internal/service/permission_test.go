package service

import (
	"encoding/json"
	"errors"
	"os"
	"testing"

	"mobile/services/core/internal/auth"
	"mobile/services/core/internal/domain/permissions"
	"mobile/services/core/internal/httpapi/problem"
)

type goldenCase struct {
	Name       string   `json:"name"`
	Action     string   `json:"action"`
	ActorRoles []string `json:"actor_roles"`
	ExtraRoles []string `json:"extra_roles"`
	ResourceID *string  `json:"resource_id"`
	Context    []struct {
		Name   string `json:"name"`
		Proves bool   `json:"proves"`
		Python string `json:"python"`
	} `json:"context"`
	Outcome struct {
		Kind   string `json:"kind"`
		Status int    `json:"status"`
		Code   string `json:"code"`
		Detail string `json:"detail"`
	} `json:"outcome"`
}

// testdata/python_require_permission.json is rendered by
// scripts/render_require_permission_goldens.py from the real
// _require_permission.
func TestRequirePermissionMatchesPython(t *testing.T) {
	raw, err := os.ReadFile("testdata/python_require_permission.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden struct {
		ActorID    string       `json:"actor_id"`
		Provenance string       `json:"provenance"`
		Cases      []goldenCase `json:"cases"`
	}
	if err := json.Unmarshal(raw, &golden); err != nil {
		t.Fatal(err)
	}
	if golden.Provenance != Provenance || len(golden.Cases) < len(permissions.Actions())*3 {
		t.Fatalf("golden file looks wrong: provenance %q, %d cases", golden.Provenance, len(golden.Cases))
	}
	kinds := map[string]int{}
	for _, c := range golden.Cases {
		proven := map[string]bool{}
		for _, entry := range c.Context {
			proven[entry.Name] = entry.Proves
		}
		actor := auth.Actor{ID: golden.ActorID, Roles: c.ActorRoles}
		got, err := RequirePermission(c.Action, actor, Resource{ID: c.ResourceID, Proven: proven}, c.ExtraRoles...)
		kinds[c.Outcome.Kind]++
		switch c.Outcome.Kind {
		case "allowed":
			if err != nil || got != nil {
				t.Errorf("%s %s: got %+v %v, Python allowed", c.Name, c.Action, got, err)
			}
		case "problem":
			want := problem.Problem{Status: c.Outcome.Status, Code: c.Outcome.Code, Detail: c.Outcome.Detail}
			if err != nil || got == nil || *got != want {
				t.Errorf("%s %s: got %+v %v, Python %+v", c.Name, c.Action, got, err, want)
			}
		case "raises":
			var failure *permissions.Error
			if got != nil || !errors.As(err, &failure) || failure.Code != c.Outcome.Code {
				t.Errorf("%s %s: got %+v %v, Python raised %s", c.Name, c.Action, got, err, c.Outcome.Code)
			}
		default:
			t.Fatalf("%s: unknown outcome kind %q", c.Name, c.Outcome.Kind)
		}
	}
	for _, kind := range []string{"allowed", "problem", "raises"} {
		if kinds[kind] == 0 {
			t.Fatalf("no %q case: the corpus cannot tell that outcome apart", kind)
		}
	}
}

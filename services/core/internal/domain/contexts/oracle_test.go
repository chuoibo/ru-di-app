package contexts

import (
	"fmt"
	"reflect"
	"sort"
	"testing"

	"mobile/services/core/internal/domain/direct"
	"mobile/services/core/internal/domain/permissions"
	"mobile/services/core/internal/oracletest"
)

// testdata/python_*.json is rendered by scripts/render_domain_w3_goldens.py by
// running the real app.api.service methods over a recording stub repository in
// the parity API image. Every case is replayed here, including which
// repository reads the method made.

func problemOf(r *Refusal) any {
	if r == nil {
		return nil
	}
	return map[string]any{"status": int64(r.Status), "code": r.Code, "detail": r.Detail}
}

func str(args map[string]any, key string) (string, error) {
	value, ok := args[key].(string)
	if !ok {
		return "", fmt.Errorf("%s = %v is not a str", key, args[key])
	}
	return value, nil
}

func replay(c oracletest.Case) (any, error) {
	args, err := c.PlainArgs()
	if err != nil {
		return nil, err
	}
	switch c.Fn {
	case "update_changes":
		displayName, err := oracletest.OptionalString(args["display_name"])
		if err != nil {
			return nil, err
		}
		theme, err := oracletest.OptionalString(args["theme"])
		if err != nil {
			return nil, err
		}
		kind, err := oracletest.OptionalString(args["kind"])
		if err != nil {
			return nil, err
		}
		read := false
		// The stub's get_context: a context row of this kind, or none.
		isPair := func() (bool, error) {
			read = true
			return kind != nil && direct.IsPair(*kind), nil
		}
		changes, refused, err := UpdateChanges(displayName, theme, isPair)
		if err != nil {
			return nil, err
		}
		calls := []any{"is_member"}
		if read {
			calls = append(calls, "get_context")
		}
		if refused != nil {
			return map[string]any{"calls": calls, "changes": nil, "problem": problemOf(refused)}, nil
		}
		written := []any{}
		if changes.DisplayName != nil {
			written = append(written, []any{"display_name", *changes.DisplayName})
		}
		if changes.Theme != nil {
			written = append(written, []any{"theme", *changes.Theme})
		}
		return map[string]any{"calls": append(calls, "update_context"), "changes": written, "problem": nil}, nil
	case "require_photo_url_context":
		contextID, err := str(args, "context_id")
		if err != nil {
			return nil, err
		}
		imageURL, err := oracletest.OptionalString(args["image_url"])
		if err != nil {
			return nil, err
		}
		return map[string]any{"problem": problemOf(RequirePhotoURLContext(contextID, imageURL))}, nil
	case "context_view":
		kind, err := str(args, "kind")
		if err != nil {
			return nil, err
		}
		stored, err := str(args, "stored_name")
		if err != nil {
			return nil, err
		}
		actorID, err := str(args, "actor_id")
		if err != nil {
			return nil, err
		}
		rows, ok := args["members"].([]any)
		if !ok {
			return nil, fmt.Errorf("members %v is not a list", args["members"])
		}
		var roster []Member
		for _, row := range rows {
			pair, err := oracletest.Strings(row)
			if err != nil || len(pair) != 2 {
				return nil, fmt.Errorf("member %v is not [id, name]", row)
			}
			roster = append(roster, Member{PersonID: pair[0], DisplayName: pair[1]})
		}
		calls := []any{}
		view, err := View(kind, stored, actorID, func() ([]Member, error) {
			calls = append(calls, "list_members")
			return roster, nil
		})
		if err != nil {
			return nil, err
		}
		var counterpart any
		if view.Counterpart != nil {
			counterpart = map[string]any{"id": view.Counterpart.ID, "display_name": view.Counterpart.DisplayName}
		}
		return map[string]any{"calls": calls, "display_name": view.DisplayName, "counterpart": counterpart}, nil
	case "accept_permission":
		origin, err := str(args, "origin")
		if err != nil {
			return nil, err
		}
		roles, err := oracletest.Strings(args["roles"])
		if err != nil {
			return nil, err
		}
		actorID, err := str(args, "actor_id")
		if err != nil {
			return nil, err
		}
		personID, err := str(args, "person_id")
		if err != nil {
			return nil, err
		}
		isMember, ok := args["is_member"].(bool)
		if !ok {
			return nil, fmt.Errorf("is_member %v is not a bool", args["is_member"])
		}
		asked := false
		action, predicates, err := AcceptPermission(origin, actorID, personID, func() (bool, error) {
			asked = true
			return isMember, nil
		})
		if err != nil {
			return nil, err
		}
		proven := []string{}
		for name, holds := range predicates {
			if holds {
				proven = append(proven, name)
			}
		}
		sort.Strings(proven)
		reason, allowed, err := permissions.DenialReason(action, permissions.AuthorizationFacts{
			ActorID:    actorID,
			Roles:      roles,
			Proven:     proven,
			Provenance: "api_service",
		})
		if err != nil {
			return nil, err
		}
		calls := []any{"get_membership"}
		if asked {
			calls = append(calls, "is_member")
		}
		if !allowed {
			return map[string]any{
				"calls":   calls,
				"problem": map[string]any{"status": int64(403), "code": "permission_denied", "detail": reason},
			}, nil
		}
		return map[string]any{"calls": append(calls, "get_context"), "problem": nil}, nil
	}
	return nil, fmt.Errorf("unknown function %q", c.Fn)
}

func TestServiceStepsMatchPython(t *testing.T) {
	files := oracletest.Load(t, "testdata/python_*.json")
	oracletest.CheckShards(t, files, "contexts")
	counts := map[string]int{}
	refusals := map[string]int{}
	total, mismatches := 0, 0
	for _, file := range files {
		for _, c := range file.Cases {
			want, raised, err := c.Outcome()
			if err != nil || raised != nil {
				t.Fatalf("%s %s: Python raised %v (%v)", file.Mode, c.Name, raised, err)
			}
			got, err := replay(c)
			if err != nil {
				t.Fatalf("%s %s: %v", file.Mode, c.Name, err)
			}
			total++
			counts[c.Fn]++
			if problem, ok := want.(map[string]any)["problem"].(map[string]any); ok {
				refusals[fmt.Sprint(problem["code"])]++
			}
			if !reflect.DeepEqual(want, got) {
				mismatches++
				if mismatches <= 20 {
					t.Errorf("%s %s %s(%v):\n  Python %#v\n  Go     %#v", file.Mode, c.Name, c.Fn, c.Args, want, got)
				}
			}
		}
	}
	if mismatches > 0 {
		t.Fatalf("%d mismatches over %d Python cases", mismatches, total)
	}
	for fn, least := range map[string]int{
		"update_changes": 500, "require_photo_url_context": 400, "context_view": 40, "accept_permission": 100,
	} {
		if counts[fn] < least {
			t.Errorf("%s: %d cases, want at least %d", fn, counts[fn], least)
		}
	}
	for _, code := range []string{"not_a_group", "theme_unknown", "photo_url_invalid", "photo_context_mismatch", "permission_denied"} {
		if refusals[code] == 0 {
			t.Errorf("no case refused with %s: the corpus lost its spread", code)
		}
	}
	t.Logf("%d Python cases agree, 0 mismatches: %v; refusals %v", total, counts, refusals)
}

func TestConstantsMatchPython(t *testing.T) {
	constants := oracletest.Constants(t, oracletest.Load(t, "testdata/python_*.json"), "contexts")
	if constants["somebody"] != WidgetAuthorFallback {
		t.Errorf("_SOMEBODY: Python %v, Go %q", constants["somebody"], WidgetAuthorFallback)
	}
	ranges, ok := constants["isspace"].([]any)
	if !ok || len(ranges) == 0 {
		t.Fatalf("isspace ranges %v", constants["isspace"])
	}
	python := map[rune]bool{}
	for _, item := range ranges {
		bounds, ok := item.([]any)
		if !ok || len(bounds) != 2 {
			t.Fatalf("range %v", item)
		}
		for r := rune(bounds[0].(int64)); r <= rune(bounds[1].(int64)); r++ {
			python[r] = true
		}
	}
	for r := rune(0); r <= 0x10FFFF; r++ {
		if isPySpace(r) != python[r] {
			t.Errorf("U+%04X: str.isspace() is %v, isPySpace is %v", r, python[r], isPySpace(r))
		}
	}
}

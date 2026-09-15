package direct

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"mobile/services/core/internal/domain/friendship"
	"mobile/services/core/internal/oracletest"
)

// testdata/python_direct_people*.json is rendered by
// scripts/render_domain_w10_goldens.py from the real app.domain.direct in the
// parity API image: the functions POST /people/{person_id}/dm reaches.

func replayPeople(c oracletest.Case, args map[string]any) (any, error) {
	decode := func(err error) (any, error) { return nil, oracletest.Decode(fmt.Errorf("%s: %w", c.Name, err)) }
	flag := func(key string, fallback bool) (bool, error) {
		raw, ok := args[key]
		if !ok {
			return fallback, nil
		}
		return oracletest.Bool(raw)
	}
	switch c.Fn {
	case "pair_key":
		a, err := oracletest.Str(args["a"])
		if err != nil {
			return decode(err)
		}
		b, err := oracletest.Str(args["b"])
		if err != nil {
			return decode(err)
		}
		key, err := PairKey(a, b)
		if err != nil {
			return nil, err
		}
		return key, nil
	case "can_open":
		isFriend, err := flag("is_friend", false)
		if err != nil {
			return decode(err)
		}
		otherExists, err := flag("other_exists", true)
		if err != nil {
			return decode(err)
		}
		otherDeleted, err := flag("other_deleted", false)
		if err != nil {
			return decode(err)
		}
		return CanOpen(isFriend, otherExists, otherDeleted), nil
	case "is_kind":
		return IsKind(args["value"]), nil
	}
	return decode(fmt.Errorf("unknown function %q", c.Fn))
}

func friendshipRefusal(err error) (string, string, bool) {
	var refused *friendship.FriendshipError
	if errors.As(err, &refused) {
		return "FriendshipError", refused.Code, true
	}
	return "", "", false
}

func checkPeople(t *testing.T, files []oracletest.File, least int) {
	t.Helper()
	report := oracletest.Agree(t, files, "direct_people", replayPeople, friendshipRefusal)
	total := 0
	for _, fn := range []string{"pair_key", "can_open", "is_kind"} {
		if report.ByFn[fn] == nil {
			t.Errorf("no Python case of %s", fn)
			continue
		}
		total += report.ByFn[fn].Cases
	}
	if total < least {
		t.Errorf("%d cases, want at least %d", total, least)
	}
	for _, code := range []string{friendship.CodeSelfEdge, friendship.CodePersonRequired} {
		if report.Codes[code] == 0 {
			t.Errorf("no Python case refused with %s", code)
		}
	}
	t.Logf("refusals %v", report.Codes)
}

func TestDirectPeopleMatchesPython(t *testing.T) {
	checkPeople(t, oracletest.Load(t, "testdata/python_direct_people*.json"), 350)
}

func TestDirectPeopleConstantsMatchPython(t *testing.T) {
	constants := oracletest.Constants(t, oracletest.Load(t, "testdata/python_direct_people*.json"), "direct_people")
	for key, got := range map[string][]string{"kinds": Kinds(), "roster_only_doors": RosterOnlyDoors()} {
		python, err := oracletest.Strings(constants[key])
		if err != nil || !reflect.DeepEqual(python, got) {
			t.Errorf("%s: Python %v, Go %v", key, python, got)
		}
	}
	Kinds()[0], RosterOnlyDoors()[0] = "x", "x"
	if Kinds()[0] != KindGroup || RosterOnlyDoors()[0] != "invite_context_member" {
		t.Error("a returned slice aliases the package's list")
	}
}

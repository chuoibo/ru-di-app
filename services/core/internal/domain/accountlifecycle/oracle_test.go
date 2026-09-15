package accountlifecycle

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"mobile/services/core/internal/domain/pairpaper"
	"mobile/services/core/internal/oracletest"
)

// testdata/python_*.json is rendered by scripts/render_domain_w10_goldens.py
// from the real app.domain.account_lifecycle in the parity API image.

// naiveISO reports whether text is what isoformat() writes for a naive
// datetime: no offset after the seconds or microseconds.
func naiveISO(text string) bool {
	return (len(text) == 19 || len(text) == 26) && text[10] == 'T' && !strings.ContainsAny(text[19:], "+-")
}

// errNaive stands for the refusal Python gives a naive `now`, which no
// time.Time can express; replay answers it and counts it apart.
var errNaive = &AccountLifecycleError{Code: CodeNaiveDatetime}

// render turns a returned column value into what the golden holds: a datetime
// is its isoformat().
func render(value any) any {
	if t, ok := value.(time.Time); ok {
		return pairpaper.ISOFormat(t)
	}
	return value
}

func columns(raw any) ([]Column, error) {
	rows, err := oracletest.List(raw)
	if err != nil {
		return nil, err
	}
	out := make([]Column, 0, len(rows))
	for _, row := range rows {
		pair, err := oracletest.List(row)
		if err != nil || len(pair) != 2 {
			return nil, fmt.Errorf("column %v", row)
		}
		name, err := oracletest.Str(pair[0])
		if err != nil {
			return nil, err
		}
		out = append(out, Column{Name: name, Value: pair[1]})
	}
	return out, nil
}

func pairsOf(cols []Column) []any {
	out := make([]any, len(cols))
	for i, column := range cols {
		out[i] = []any{column.Name, render(column.Value)}
	}
	return out
}

type replayer struct{ naive int }

func (r *replayer) replay(c oracletest.Case, args map[string]any) (any, error) {
	decode := func(err error) (any, error) { return nil, oracletest.Decode(fmt.Errorf("%s: %w", c.Name, err)) }
	switch c.Fn {
	case "tables_for":
		action, err := oracletest.Str(args["action"])
		if err != nil {
			return decode(err)
		}
		tables, err := TablesFor(action)
		if err != nil {
			return nil, err
		}
		return oracletest.AnyStrings(tables), nil
	case "check_confirmation":
		return nil, CheckConfirmation(args["confirm"])
	case "anonymised_person":
		person, err := columns(args["person"])
		if err != nil {
			return decode(err)
		}
		text, err := oracletest.Str(args["now"])
		if err != nil {
			return decode(err)
		}
		if naiveISO(text) {
			r.naive++
			return nil, errNaive
		}
		now, err := oracletest.Instant(text)
		if err != nil {
			return decode(err)
		}
		before := slices.Clone(person)
		row := AnonymisedPerson(person, now)
		return map[string]any{"row": pairsOf(row), "input_unchanged": reflect.DeepEqual(before, person)}, nil
	}
	return decode(fmt.Errorf("unknown function %q", c.Fn))
}

func refusal(err error) (string, string, bool) {
	if refused, ok := err.(*AccountLifecycleError); ok {
		return "AccountLifecycleError", refused.Code, true
	}
	return "", "", false
}

func check(t *testing.T, files []oracletest.File, least int) {
	t.Helper()
	r := &replayer{}
	report := oracletest.Agree(t, files, "account_lifecycle", r.replay, refusal)
	total := 0
	for _, fn := range []string{"tables_for", "check_confirmation", "anonymised_person"} {
		tally := report.ByFn[fn]
		if tally == nil || tally.Cases == 0 || tally.Refusals == 0 {
			t.Errorf("%s: no Python cases, or no refusals among them: %+v", fn, tally)
			continue
		}
		total += tally.Cases
	}
	if total < least {
		t.Errorf("%d cases, want at least %d", total, least)
	}
	for _, code := range []string{CodeUnknownErasureAction, CodeConfirmRequired, CodeNaiveDatetime} {
		if report.Codes[code] == 0 {
			t.Errorf("no Python case refused with %s", code)
		}
	}
	if report.Codes[CodeNaiveDatetime] != r.naive {
		t.Errorf("Python refused %d naive clocks, the replay saw %d", report.Codes[CodeNaiveDatetime], r.naive)
	}
	t.Logf("refusals %v; %d naive clocks have no Go spelling and were checked as Python's refusal only", report.Codes, r.naive)
}

func TestAccountLifecycleMatchesPython(t *testing.T) {
	check(t, oracletest.Load(t, "testdata/python_account_lifecycle*.json"), 200)
}

func TestConstantsMatchPython(t *testing.T) {
	constants := oracletest.Constants(t, oracletest.Load(t, "testdata/python_account_lifecycle*.json"), "account_lifecycle")
	if constants["anonymous_display_name"] != AnonymousDisplayName {
		t.Errorf("ANONYMOUS_DISPLAY_NAME: Python %v, Go %q", constants["anonymous_display_name"], AnonymousDisplayName)
	}
	if constants["confirmation_required"] != ConfirmationRequired {
		t.Errorf("CONFIRMATION_REQUIRED: Python %v, Go %q", constants["confirmation_required"], ConfirmationRequired)
	}
	var want []Branch
	rows, err := oracletest.List(constants["erasure"])
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range rows {
		entry, err := oracletest.List(raw)
		if err != nil || len(entry) != 2 {
			t.Fatalf("erasure entry %v", raw)
		}
		action, _ := entry[0].(string)
		tables, err := oracletest.Strings(entry[1])
		if err != nil {
			t.Fatal(err)
		}
		want = append(want, Branch{Action: action, Tables: tables})
	}
	if !reflect.DeepEqual(want, Erasure()) {
		t.Errorf("ERASURE:\n  Python %v\n  Go     %v", want, Erasure())
	}
	for key, got := range map[string][]string{"money_tables": MoneyTables(), "others_keep_tables": OthersKeepTables()} {
		python, err := oracletest.Strings(constants[key])
		if err != nil || !reflect.DeepEqual(python, got) {
			t.Errorf("%s: Python %v, Go %v", key, python, got)
		}
	}
	ported := map[string]string{
		"ANONYMOUS_DISPLAY_NAME": "AnonymousDisplayName",
		"CONFIRMATION_REQUIRED":  "ConfirmationRequired",
		"ERASURE":                "Erasure",
		"MONEY_TABLES":           "MoneyTables",
		"OTHERS_KEEP_TABLES":     "OthersKeepTables",
		"AccountLifecycleError":  "AccountLifecycleError",
		"anonymised_person":      "AnonymisedPerson",
		"check_confirmation":     "CheckConfirmation",
		"tables_for":             "TablesFor",
	}
	names, err := oracletest.Strings(constants["names"])
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		if ported[name] == "" {
			t.Errorf("Python exports %s and the port does not map it", name)
		}
	}
	if len(names) != len(ported) {
		t.Errorf("Python has %d public names %v; the port maps %d", len(names), names, len(ported))
	}
}

// The map is closed: no table twice, and the money and other people's tables
// only under keep. Python's tests hold the same three sentences.
func TestTheMapIsClosed(t *testing.T) {
	seen := map[string]string{}
	keep := map[string]bool{}
	for _, branch := range Erasure() {
		for _, table := range branch.Tables {
			if first, dup := seen[table]; dup {
				t.Errorf("%s is under both %s and %s", table, first, branch.Action)
			}
			seen[table] = branch.Action
			keep[table] = branch.Action == ActionKeep
		}
	}
	for _, table := range append(MoneyTables(), OthersKeepTables()...) {
		if !keep[table] {
			t.Errorf("%s is not under keep", table)
		}
	}
}

// Returned slices are copies: a caller writing into one cannot change the map.
func TestReturnedSlicesAreCopies(t *testing.T) {
	Erasure()[0].Tables[0] = "x"
	MoneyTables()[0] = "x"
	tables, _ := TablesFor(ActionDelete)
	tables[0] = "x"
	if Erasure()[0].Tables[0] != "posts" || MoneyTables()[0] != "expenses" {
		t.Fatal("a returned slice aliases the package's map")
	}
}

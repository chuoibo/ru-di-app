package journey

import (
	"errors"
	"fmt"
	"testing"

	"mobile/services/core/internal/oracletest"
)

// testdata/python_journey*.json is rendered by
// scripts/render_domain_w7_goldens.py from the real app/domain/journey.py in
// the parity API image. Every case is replayed here: the value Python
// returned, or the exception it raised, must match.

// functions is every ported function, in the order of the script's
// JOURNEY_FUNCTIONS.
var functions = []string{
	"issue", "minute", "clock", "schedule", "order_costs", "suggest_order",
}

// issueCodes is every issue code the committed corpus must show. An issue
// this package can raise and the corpus never shows is a hole in the corpus,
// not a passing test.
var issueCodes = []string{
	"unreachable", "late_fixed_stop", "missing_duration", "day_overflow",
	"unreachable_return",
}

func refusal(err error) (class, code string, ok bool) {
	var value *ValueError
	if errors.As(err, &value) {
		return "ValueError", value.Message, true
	}
	var index *IndexError
	if errors.As(err, &index) {
		return "IndexError", index.Error(), true
	}
	return "", "", false
}

// --- decoding ---------------------------------------------------------------

func decodeStop(value any) (Stop, error) {
	row, err := oracletest.Row(value, "id", "at", "duration_minutes", "time_locked", "checked_in")
	if err != nil {
		return Stop{}, err
	}
	id, err := oracletest.Str(row["id"])
	if err != nil {
		return Stop{}, err
	}
	at, err := oracletest.Str(row["at"])
	if err != nil {
		return Stop{}, err
	}
	locked, err := oracletest.Bool(row["time_locked"])
	if err != nil {
		return Stop{}, err
	}
	checked, err := oracletest.Bool(row["checked_in"])
	if err != nil {
		return Stop{}, err
	}
	stop := Stop{ID: id, At: at, TimeLocked: locked, CheckedIn: checked}
	if row["duration_minutes"] != nil {
		dwell, err := oracletest.Int64(row["duration_minutes"])
		if err != nil {
			return Stop{}, err
		}
		stop.DurationMinutes = &dwell
	}
	return stop, nil
}

func decodeStops(value any) ([]Stop, error) {
	items, err := oracletest.List(value)
	if err != nil {
		return nil, err
	}
	out := make([]Stop, 0, len(items))
	for _, item := range items {
		stop, err := decodeStop(item)
		if err != nil {
			return nil, err
		}
		out = append(out, stop)
	}
	return out, nil
}

func decodeCost(value any) (*Cost, error) {
	if value == nil {
		return nil, nil
	}
	pair, err := oracletest.List(value)
	if err != nil || len(pair) != 2 {
		return nil, fmt.Errorf("%v is not a cost", value)
	}
	seconds, err := oracletest.Int64(pair[0])
	if err != nil {
		return nil, err
	}
	metres, err := oracletest.Int64(pair[1])
	if err != nil {
		return nil, err
	}
	return &Cost{Seconds: seconds, Meters: metres}, nil
}

func decodeCosts(value any) ([]*Cost, error) {
	items, err := oracletest.List(value)
	if err != nil {
		return nil, err
	}
	out := make([]*Cost, 0, len(items))
	for _, item := range items {
		cost, err := decodeCost(item)
		if err != nil {
			return nil, err
		}
		out = append(out, cost)
	}
	return out, nil
}

func decodeMatrix(value any) (Matrix, error) {
	rows, err := oracletest.List(value)
	if err != nil {
		return nil, err
	}
	out := make(Matrix, 0, len(rows))
	for _, raw := range rows {
		row, err := decodeCosts(raw)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, nil
}

func decodeSettings(value any) (Settings, error) {
	row, err := oracletest.Row(value, "start_at", "return_to_start", "end_stop_id")
	if err != nil {
		return Settings{}, err
	}
	start, err := oracletest.Str(row["start_at"])
	if err != nil {
		return Settings{}, err
	}
	returning, err := oracletest.Bool(row["return_to_start"])
	if err != nil {
		return Settings{}, err
	}
	end, err := oracletest.OptionalString(row["end_stop_id"])
	if err != nil {
		return Settings{}, err
	}
	return Settings{StartAt: start, ReturnToStart: returning, EndStopID: end}, nil
}

func decodeOrder(value any) ([]int, error) {
	items, err := oracletest.List(value)
	if err != nil {
		return nil, err
	}
	out := make([]int, 0, len(items))
	for _, item := range items {
		index, err := oracletest.Int64(item)
		if err != nil {
			return nil, err
		}
		out = append(out, int(index))
	}
	return out, nil
}

// --- encoding back into the golden's shape ----------------------------------

func optional(text *string) any {
	if text == nil {
		return nil
	}
	return *text
}

func wireIssue(i Issue) any {
	return map[string]any{"code": i.Code, "stop_id": optional(i.StopID), "message": i.Message}
}

func wireIssues(issues []Issue) []any {
	out := make([]any, 0, len(issues))
	for _, i := range issues {
		out = append(out, wireIssue(i))
	}
	return out
}

func wirePlan(plan Plan) any {
	rows := make([]any, 0, len(plan.Stops))
	for _, row := range plan.Stops {
		rows = append(rows, map[string]any{
			"id":           row.ID,
			"at":           optional(row.At),
			"arrival_at":   optional(row.ArrivalAt),
			"departure_at": optional(row.DepartureAt),
			"wait_minutes": row.WaitMinutes,
		})
	}
	return map[string]any{"stops": rows, "feasible": plan.Feasible, "issues": wireIssues(plan.Issues)}
}

func wireCosts(costs []*Cost) []any {
	out := make([]any, 0, len(costs))
	for _, cost := range costs {
		if cost == nil {
			out = append(out, nil)
			continue
		}
		out = append(out, []any{cost.Seconds, cost.Meters})
	}
	return out
}

func wireOrder(order []int) []any {
	out := make([]any, 0, len(order))
	for _, index := range order {
		out = append(out, int64(index))
	}
	return out
}

// --- replay -----------------------------------------------------------------

func replay(c oracletest.Case, args map[string]any) (any, error) {
	switch c.Fn {
	case "issue":
		code, err := oracletest.Str(args["code"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		message, err := oracletest.Str(args["message"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		stopID, err := oracletest.OptionalString(args["stop_id"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		return wireIssue(NewIssue(code, message, stopID)), nil
	case "minute":
		value, err := oracletest.Str(args["value"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		return Minute(value)
	case "clock":
		value, err := oracletest.Int64(args["value"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		return optional(Clock(value)), nil
	case "schedule":
		stops, err := decodeStops(args["stops"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		costs, err := decodeCosts(args["costs"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		settings, err := decodeSettings(args["settings"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		plan, err := Schedule(stops, costs, settings)
		if err != nil {
			return nil, err
		}
		return wirePlan(plan), nil
	case "order_costs":
		order, err := decodeOrder(args["order"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		matrix, err := decodeMatrix(args["matrix"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		returning, err := oracletest.Bool(args["returning"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		costs, err := OrderCosts(order, matrix, returning)
		if err != nil {
			return nil, err
		}
		return wireCosts(costs), nil
	case "suggest_order":
		stops, err := decodeStops(args["stops"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		matrix, err := decodeMatrix(args["matrix"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		settings, err := decodeSettings(args["settings"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		order, err := SuggestOrder(stops, matrix, settings)
		if err != nil {
			return nil, err
		}
		return wireOrder(order), nil
	}
	return nil, oracletest.Decode(fmt.Errorf("no replay for %s", c.Fn))
}

func checkJourney(t *testing.T, files []oracletest.File, committed bool) {
	t.Helper()
	report := oracletest.Agree(t, files, "journey", replay, refusal)
	if !committed {
		return
	}
	for _, name := range functions {
		if report.ByFn[name] == nil {
			t.Errorf("no case calls %s", name)
		}
	}
	for _, code := range issueCodes {
		found := false
		for _, file := range files {
			for _, c := range file.Cases {
				if body, ok := c.Result["ok"]; ok {
					if hasCode(body, code) {
						found = true
					}
				}
			}
		}
		if !found {
			t.Errorf("no committed case shows the issue %q", code)
		}
	}
}

// hasCode reports whether a raw encoded answer holds an issue of this code.
func hasCode(value any, code string) bool {
	switch v := value.(type) {
	case string:
		return v == code
	case []any:
		for _, item := range v {
			if hasCode(item, code) {
				return true
			}
		}
	case map[string]any:
		if found, ok := v["code"].(string); ok && found == code {
			return true
		}
		for _, item := range v {
			if hasCode(item, code) {
				return true
			}
		}
	}
	return false
}

func TestJourneyMatchesPython(t *testing.T) {
	files := oracletest.Load(t, "testdata/python_journey*.json")
	constants := oracletest.Constants(t, files, "journey")
	for name, want := range map[string]int64{
		"max_stops": MaxStops, "max_evaluations": MaxEvaluations,
		"unreachable_cost": unreachableCost,
	} {
		got, err := oracletest.Int64(constants[name])
		if err != nil || got != want {
			t.Errorf("%s: Python %v, Go %d (%v)", name, constants[name], want, err)
		}
	}
	checkJourney(t, files, true)
}

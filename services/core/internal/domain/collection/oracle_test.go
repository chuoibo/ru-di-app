package collection

import (
	"errors"
	"slices"
	"testing"
	"time"

	"mobile/services/core/internal/oracletest"
)

// testdata/python_collection*.json is rendered by
// scripts/render_domain_w4_goldens.py from the real app.domain.collection in
// the parity API image; every case is replayed here.

func obligationsOf(value any) ([]Obligation, error) {
	rows, err := oracletest.List(value)
	if err != nil {
		return nil, err
	}
	out := make([]Obligation, len(rows))
	for i, raw := range rows {
		r, err := oracletest.Row(raw, "sender_id", "status")
		if err != nil {
			return nil, err
		}
		if out[i].SenderID, err = oracletest.Str(r["sender_id"]); err != nil {
			return nil, err
		}
		if out[i].Status, err = oracletest.Str(r["status"]); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// contextOf reads a context dict, or None, the way transition reads it.
func contextOf(value any) (Context, error) {
	if value == nil {
		return Context{}, nil
	}
	m, err := oracletest.Row(value)
	if err != nil {
		return Context{}, err
	}
	var ctx Context
	if raw, found := m["obligations"]; found {
		if ctx.Obligations, err = obligationsOf(raw); err != nil {
			return Context{}, err
		}
	}
	for key, target := range map[string]*bool{
		"advancer_acknowledged":          &ctx.AdvancerAcknowledged,
		"delivery_method_chosen":         &ctx.DeliveryMethodChosen,
		"all_affected_parties_consented": &ctx.AllAffectedPartiesConsented,
	} {
		if raw, found := m[key]; found {
			if *target, err = oracletest.Bool(raw); err != nil {
				return Context{}, err
			}
		}
	}
	ctx.CapabilityExposed = m["capability_exposed_at"] != nil
	return ctx, nil
}

func stamp(value any) (time.Time, error) {
	s, err := oracletest.Str(value)
	if err != nil {
		return time.Time{}, err
	}
	return time.Parse(time.RFC3339, s)
}

func replay(c oracletest.Case, args map[string]any) (any, error) {
	switch c.Fn {
	case "transition":
		state, err := oracletest.Str(args["state"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		event, err := oracletest.Str(args["event"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		ctx, err := contextOf(args["context"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		return Transition(state, event, ctx)
	case "unmet_freeze_requirements", "unmet_publish_gates":
		ctx, err := contextOf(args["context"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		if c.Fn == "unmet_freeze_requirements" {
			return oracletest.AnyStrings(UnmetFreezeRequirements(ctx)), nil
		}
		return oracletest.AnyStrings(UnmetPublishGates(ctx)), nil
	case "terminal_state_for", "progress":
		obligations, err := obligationsOf(args["obligations"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		if c.Fn == "terminal_state_for" {
			return TerminalStateFor(obligations)
		}
		p := ProgressOf(obligations)
		return map[string]any{
			"transfers_done":  int64(p.TransfersDone),
			"transfers_total": int64(p.TransfersTotal),
			"people_done":     int64(p.PeopleDone),
			"people_total":    int64(p.PeopleTotal),
		}, nil
	case "is_stale":
		var times [3]time.Time
		for i, key := range []string{"now", "due_at", "last_meaningful_activity_at"} {
			var err error
			if times[i], err = stamp(args[key]); err != nil {
				return nil, oracletest.Decode(err)
			}
		}
		return IsStale(times[0], times[1], times[2]), nil
	case "counts_toward_collection_rate":
		batch, err := oracletest.Row(args["batch"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		state := ""
		if batch["state"] != nil {
			if state, err = oracletest.Str(batch["state"]); err != nil {
				return nil, oracletest.Decode(err)
			}
		}
		return CountsTowardCollectionRate(state, batch["capability_exposed_at"] != nil), nil
	}
	return nil, oracletest.Decode(errors.New("unknown function " + c.Fn))
}

func refusal(err error) (string, string, bool) {
	var refused *CollectionError
	if errors.As(err, &refused) {
		return "CollectionError", refused.Code, true
	}
	return "", "", false
}

// collectionFunctions is every ported function; the fuzz draws only the first
// four, the edge cases reach all of them.
var collectionFunctions = []string{
	"transition", "terminal_state_for", "progress", "is_stale",
	"unmet_freeze_requirements", "unmet_publish_gates", "counts_toward_collection_rate",
}

func TestCollectionMatchesPython(t *testing.T) {
	checkCollection(t, oracletest.Load(t, "testdata/python_*.json"), "collection", 300, collectionFunctions)
}

// checkCollection replays the cases of module in files: at least least in
// all, some of each of fns, and every refusal code.
func checkCollection(t *testing.T, files []oracletest.File, module string, least int, fns []string) {
	t.Helper()
	report := oracletest.Agree(t, files, module, replay, refusal)
	total := 0
	for _, tally := range report.ByFn {
		total += tally.Cases
	}
	if total < least {
		t.Errorf("%d cases, want at least %d", total, least)
	}
	for _, fn := range fns {
		if report.ByFn[fn] == nil {
			t.Errorf("no Python case of %s", fn)
		}
	}
	for _, code := range []string{
		CodeUnknownState, CodeIllegalTransition, CodeNoObligations, CodeCancelAfterExposureNeedsConsent,
		CodeObligationsStillOpen, "ADVANCER_ACKNOWLEDGEMENT_REQUIRED", "DELIVERY_METHOD_REQUIRED",
	} {
		if report.Codes[code] == 0 {
			t.Errorf("no Python case refused with %s", code)
		}
	}
	t.Logf("refusals %v", report.Codes)
}

func TestConstantsMatchPython(t *testing.T) {
	constants := oracletest.Constants(t, oracletest.Load(t, "testdata/python_*.json"), "collection")
	python, err := oracletest.Strings(constants["states"])
	if err != nil || !slices.Equal(python, States()) {
		t.Errorf("STATES: Python %v, Go %v (%v)", constants["states"], States(), err)
	}
	ported := map[string]bool{
		"STATES": true, "CollectionError": true, "unmet_freeze_requirements": true, "unmet_publish_gates": true,
		"transition": true, "terminal_state_for": true, "progress": true, "is_stale": true,
		"counts_toward_collection_rate": true,
	}
	names, err := oracletest.Strings(constants["names"])
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		if !ported[name] {
			t.Errorf("Python exports %s and the port does not map it", name)
		}
	}
}

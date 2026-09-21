package capability

import (
	"errors"
	"testing"

	"mobile/services/core/internal/oracletest"
)

// testdata/python_capability*.json is rendered by
// scripts/render_domain_w4_goldens.py from the real app.domain.capability in
// the parity API image; every case is replayed here.

func replay(c oracletest.Case, args map[string]any) (any, error) {
	if c.Fn != "capability_scope" {
		return nil, oracletest.Decode(errors.New("unknown function " + c.Fn))
	}
	env, err := oracletest.Row(args["envelope"], "batch_version_id", "sender_id")
	if err != nil {
		return nil, oracletest.Decode(err)
	}
	var envelope Envelope
	if envelope.BatchVersionID, err = oracletest.Str(env["batch_version_id"]); err != nil {
		return nil, oracletest.Decode(err)
	}
	if envelope.SenderID, err = oracletest.Str(env["sender_id"]); err != nil {
		return nil, oracletest.Decode(err)
	}
	rows, err := oracletest.List(args["obligations"])
	if err != nil {
		return nil, oracletest.Decode(err)
	}
	obligations := make([]Obligation, len(rows))
	for i, raw := range rows {
		r, err := oracletest.Row(raw, "obligation_id", "batch_version_id", "sender_id")
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		fields := []*string{&obligations[i].ObligationID, &obligations[i].BatchVersionID, &obligations[i].SenderID}
		for j, key := range []string{"obligation_id", "batch_version_id", "sender_id"} {
			if *fields[j], err = oracletest.Str(r[key]); err != nil {
				return nil, oracletest.Decode(err)
			}
		}
	}
	scope, err := ScopeOf(envelope, obligations)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"batch_version_id": scope.BatchVersionID,
		"sender_id":        scope.SenderID,
		"obligation_ids":   oracletest.AnyStrings(scope.ObligationIDs),
	}, nil
}

func refusal(err error) (string, string, bool) {
	var refused *CapabilityScopeError
	if errors.As(err, &refused) {
		return "CapabilityScopeError", refused.Code, true
	}
	return "", "", false
}

func TestCapabilityScopeMatchesPython(t *testing.T) {
	checkScope(t, oracletest.Load(t, "testdata/python_*.json"), "capability", 150)
}

// checkScope replays the cases of module in files: at least least of them,
// and every refusal code.
func checkScope(t *testing.T, files []oracletest.File, module string, least int) {
	t.Helper()
	report := oracletest.Agree(t, files, module, replay, refusal)
	if tally := report.ByFn["capability_scope"]; tally == nil || tally.Cases < least {
		t.Fatalf("%+v, want at least %d cases: the corpus lost its spread", tally, least)
	}
	for _, code := range []string{CodeNoObligations, CodeCrossesBatchVersion, CodeCrossesSender, CodeDuplicateObligation} {
		if report.Codes[code] == 0 {
			t.Errorf("no Python case refused with %s", code)
		}
	}
	t.Logf("refusals %v", report.Codes)
}

func TestConstantsMatchPython(t *testing.T) {
	constants := oracletest.Constants(t, oracletest.Load(t, "testdata/python_*.json"), "capability")
	names, err := oracletest.Strings(constants["names"])
	if err != nil {
		t.Fatal(err)
	}
	ported := map[string]bool{"CapabilityScopeError": true, "capability_scope": true}
	for _, name := range names {
		if !ported[name] {
			t.Errorf("Python exports %s and the port does not map it", name)
		}
	}
}

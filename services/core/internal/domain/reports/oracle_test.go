package reports

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// testdata/python_*.json is rendered by scripts/render_domain_w1_goldens.py
// from the real app.domain.reports in the pinned API image. Every case is
// replayed through ValidateReportValues and, when the arguments have the types
// pydantic leaves (two str, a str or None), through ValidateReport too.

func goOutcome(value any, err error) outcome {
	if err == nil {
		return outcome{value: value}
	}
	var refused *ReportError
	if errors.As(err, &refused) {
		return outcome{errType: "ReportError", message: refused.Error(), code: refused.Code}
	}
	return outcome{errType: fmt.Sprintf("go:%T", err), message: err.Error()}
}

func renderReport(report Report) any {
	var note any
	if report.Note != nil {
		note = *report.Note
	}
	return map[string]any{"target_type": report.TargetType, "reason": report.Reason, "note": note}
}

func replay(c oracleCase) (string, []call, error) {
	if c.Fn != "validate_report" || c.Args != nil {
		return "", nil, fmt.Errorf("unexpected call %s", c.Fn)
	}
	args := map[string]any{}
	for key, raw := range c.Kwargs {
		switch key {
		case "target_type", "reason", "note":
		default:
			return "", nil, fmt.Errorf("unexpected keyword %q", key)
		}
		value, err := decodeValue(raw)
		if err != nil {
			return "", nil, err
		}
		args[key] = value
	}
	targetType, hasTarget := args["target_type"]
	reason, hasReason := args["reason"]
	if !hasTarget || !hasReason {
		return "", nil, errors.New("target_type and reason are required keywords")
	}
	note := args["note"] // an omitted note is None, the default
	report, err := ValidateReportValues(targetType, reason, note)
	calls := []call{{path: "ValidateReportValues", got: goOutcome(renderReport(report), err)}}
	target, targetIsText := targetType.(string)
	why, reasonIsText := reason.(string)
	if targetIsText && reasonIsText {
		switch text := note.(type) {
		case nil:
			report, err := ValidateReport(target, why, nil)
			calls = append(calls, call{path: "ValidateReport", typed: true, got: goOutcome(renderReport(report), err)})
		case string:
			report, err := ValidateReport(target, why, &text)
			calls = append(calls, call{path: "ValidateReport", typed: true, got: goOutcome(renderReport(report), err)})
		}
	}
	return show(args), calls, nil
}

func TestOracleCases(t *testing.T) {
	runOracle(t, 2000, replay)
}

func TestOracleFuzzVolume(t *testing.T) {
	checkFuzzVolume(t, "validate_report")
}

func TestOracleConstants(t *testing.T) {
	var constants struct {
		TargetTypes   []string  `json:"target_types"`
		Reasons       []string  `json:"reasons"`
		MaxNoteLength int       `json:"max_note_length"`
		IsSpace       [][2]rune `json:"isspace"`
	}
	loadConstants(t, &constants)
	if strings.Join(TargetTypes(), ",") != strings.Join(constants.TargetTypes, ",") {
		t.Errorf("TARGET_TYPES %q, Go %q", constants.TargetTypes, TargetTypes())
	}
	if strings.Join(Reasons(), ",") != strings.Join(constants.Reasons, ",") {
		t.Errorf("REASONS %q, Go %q", constants.Reasons, Reasons())
	}
	if MaxNoteLength != constants.MaxNoteLength {
		t.Errorf("MAX_NOTE_LENGTH %d, Go %d", constants.MaxNoteLength, MaxNoteLength)
	}
	checkIsSpace(t, constants.IsSpace, isPySpace)
}

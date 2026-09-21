package preferences

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// testdata/python_*.json is rendered by scripts/render_domain_w1_goldens.py
// from the real app.domain.preferences in the pinned API image. Every case is
// replayed through BuildPreferenceProfileValues and, when every visit has a
// str category and every trip two ints, through BuildPreferenceProfile too.

func goOutcome(value any, err error) outcome {
	if err == nil {
		return outcome{value: value}
	}
	var refused *PreferenceError
	if errors.As(err, &refused) {
		// Python's PreferenceError has no `.code` attribute.
		return outcome{errType: "PreferenceError", message: refused.Error()}
	}
	return outcome{errType: fmt.Sprintf("go:%T", err), message: err.Error()}
}

func renderProfile(profile Profile) any {
	var sections any = nilSlice{}
	if profile.Sections != nil {
		rows := make([]any, len(profile.Sections))
		for i, section := range profile.Sections {
			var tastes any = nilSlice{}
			if section.Tastes != nil {
				list := make([]any, len(section.Tastes))
				for j, taste := range section.Tastes {
					list[j] = map[string]any{
						"label":         taste.Label,
						"checkin_count": taste.CheckinCount,
						"score":         taste.Score,
					}
				}
				tastes = list
			}
			rows[i] = map[string]any{
				"section":     section.Section,
				"taste_count": int64(section.TasteCount),
				"tastes":      tastes,
			}
		}
		sections = rows
	}
	var avg any
	if profile.AvgPerPersonVND != nil {
		avg = *profile.AvgPerPersonVND
	}
	return map[string]any{
		"sections":           sections,
		"checkin_count":      int64(profile.CheckinCount),
		"outing_count":       int64(profile.OutingCount),
		"split_total_vnd":    profile.SplitTotalVND,
		"avg_per_person_vnd": avg,
	}
}

// typedInputs converts a case to the typed shapes when the service could have
// built it: a str category on every visit, two ints on every trip.
func typedInputs(visits, trips []any) ([]Visit, []Trip, bool) {
	typedVisits := make([]Visit, len(visits))
	for i, raw := range visits {
		row, ok := raw.(map[string]any)
		if !ok {
			return nil, nil, false
		}
		category, ok := row["category"].(string)
		if !ok {
			return nil, nil, false
		}
		typedVisits[i] = Visit{Category: category, Kinds: row["kinds"]}
	}
	typedTrips := make([]Trip, len(trips))
	for i, raw := range trips {
		row, ok := raw.(map[string]any)
		if !ok {
			return nil, nil, false
		}
		split, splitOK := row["split_total_vnd"].(int64)
		headcount, headcountOK := row["headcount"].(int64)
		if !splitOK || !headcountOK {
			return nil, nil, false
		}
		typedTrips[i] = Trip{SplitTotalVND: split, Headcount: headcount}
	}
	return typedVisits, typedTrips, true
}

func replay(c oracleCase) (string, []call, error) {
	if c.Fn != "build_preference_profile" || len(c.Args) != 2 {
		return "", nil, fmt.Errorf("unexpected call %s with %d args", c.Fn, len(c.Args))
	}
	var lists [2][]any
	for i, raw := range c.Args {
		decoded, err := decodeValue(raw)
		if err != nil {
			return "", nil, err
		}
		list, ok := decoded.([]any)
		if !ok {
			return "", nil, fmt.Errorf("argument %d is %T, not a list", i, decoded)
		}
		lists[i] = list
	}
	visits, trips := lists[0], lists[1]
	profile, err := BuildPreferenceProfileValues(visits, trips)
	calls := []call{{path: "BuildPreferenceProfileValues", got: goOutcome(renderProfile(profile), err)}}
	if typedVisits, typedTrips, ok := typedInputs(visits, trips); ok {
		profile, err := BuildPreferenceProfile(typedVisits, typedTrips)
		calls = append(calls, call{path: "BuildPreferenceProfile", typed: true, got: goOutcome(renderProfile(profile), err)})
	}
	return show(visits) + ", " + show(trips), calls, nil
}

func TestOracleCases(t *testing.T) {
	// Fuzz visits often carry a category that is not a str, which the typed
	// Visit cannot express, so fewer cases reach the typed path here.
	runOracle(t, 1500, replay)
}

func TestOracleFuzzVolume(t *testing.T) {
	checkFuzzVolume(t, "build_preference_profile")
}

func TestOracleConstants(t *testing.T) {
	var constants struct {
		SectionOfCategory   [][2]string `json:"section_of_category"`
		SectionOrder        []string    `json:"section_order"`
		MaxTastesPerSection int         `json:"max_tastes_per_section"`
		MaxLabel            int         `json:"max_label"`
		IsSpace             [][2]rune   `json:"isspace"`
	}
	loadConstants(t, &constants)
	pairs := SectionOfCategory()
	if fmt.Sprint(pairs) != fmt.Sprint(constants.SectionOfCategory) {
		t.Errorf("SECTION_OF_CATEGORY %q, Go %q", constants.SectionOfCategory, pairs)
	}
	for _, pair := range constants.SectionOfCategory {
		if section, ok := SectionOf(pair[0]); !ok || section != pair[1] {
			t.Errorf("SectionOf(%q) = %q, %v", pair[0], section, ok)
		}
	}
	if strings.Join(SectionOrder(), ",") != strings.Join(constants.SectionOrder, ",") {
		t.Errorf("SECTION_ORDER %q, Go %q", constants.SectionOrder, SectionOrder())
	}
	if MaxTastesPerSection != constants.MaxTastesPerSection || MaxLabel != constants.MaxLabel {
		t.Errorf("MAX_TASTES_PER_SECTION %d MAX_LABEL %d, Go %d %d",
			constants.MaxTastesPerSection, constants.MaxLabel, MaxTastesPerSection, MaxLabel)
	}
	checkIsSpace(t, constants.IsSpace, isPySpace)
}

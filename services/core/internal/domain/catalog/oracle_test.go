package catalog

import (
	"errors"
	"testing"

	"mobile/services/core/internal/oracletest"
)

// testdata/python_catalog*.json is rendered by scripts/render_domain_wai_goldens.py
// from the real app.places.catalog in the parity API image.

func TestCatalogMatchesPython(t *testing.T) {
	files := oracletest.Load(t, "testdata/python_*.json")
	oracletest.Agree(t, files, "catalog", replay, noRefusal)
}

func TestCatalogConstantsMatchPython(t *testing.T) {
	c := oracletest.Constants(t, oracletest.Load(t, "testdata/python_*.json"), "catalog")
	rows, err := oracletest.List(c["categories"])
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != len(Categories) {
		t.Fatalf("CATEGORIES: Python %d, Go %d", len(rows), len(Categories))
	}
	for i, raw := range rows {
		row, err := oracletest.Row(raw, "id", "label")
		if err != nil {
			t.Fatal(err)
		}
		id, _ := oracletest.Str(row["id"])
		label, _ := oracletest.Str(row["label"])
		if id != Categories[i].ID || label != Categories[i].Label {
			t.Errorf("CATEGORIES[%d]: Python (%s, %s), Go (%s, %s)", i, id, label, Categories[i].ID, Categories[i].Label)
		}
	}
}

func replay(c oracletest.Case, args map[string]any) (any, error) {
	if c.Fn != "category_ids" {
		return nil, oracletest.Decode(errors.New(c.Fn))
	}
	ids := make([]any, 0, len(Categories))
	for _, row := range Categories {
		ids = append(ids, row.ID)
	}
	return ids, nil
}

func noRefusal(error) (string, string, bool) { return "", "", false }

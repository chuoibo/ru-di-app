package stickers

import (
	"errors"
	"testing"

	"mobile/services/core/internal/oracletest"
)

// testdata/python_stickers*.json is rendered by scripts/render_domain_wai_goldens.py
// from the real app.domain.stickers in the parity API image.

func TestStickersMatchPython(t *testing.T) {
	files := oracletest.Load(t, "testdata/python_*.json")
	oracletest.Agree(t, files, "stickers", replay, noRefusal)
}

func TestStickersConstantsMatchPython(t *testing.T) {
	c := oracletest.Constants(t, oracletest.Load(t, "testdata/python_*.json"), "stickers")
	ids, err := oracletest.Strings(c["ids"])
	if err != nil {
		t.Fatal(err)
	}
	labels, err := oracletest.Strings(c["labels"])
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != len(All) || len(labels) != len(All) {
		t.Fatalf("STICKERS: Python %d ids, Go %d", len(ids), len(All))
	}
	for i, sticker := range All {
		if sticker.ID != ids[i] || sticker.Label != labels[i] {
			t.Errorf("STICKERS[%d]: Python (%s, %s), Go (%s, %s)", i, ids[i], labels[i], sticker.ID, sticker.Label)
		}
	}
	if pattern, _ := oracletest.Str(c["pattern"]); pattern != IDPattern {
		t.Errorf("STICKER_ID_PATTERN: Python %q, Go %q", pattern, IDPattern)
	}
}

func replay(c oracletest.Case, args map[string]any) (any, error) {
	if c.Fn != "is_sticker" {
		return nil, oracletest.Decode(errors.New(c.Fn))
	}
	value, err := oracletest.Str(args["value"])
	if err != nil {
		return nil, oracletest.Decode(err)
	}
	return Is(value), nil
}

func noRefusal(error) (string, string, bool) { return "", "", false }

package faces

import (
	"errors"
	"testing"

	"mobile/services/core/internal/oracletest"
)

// testdata/python_faces*.json is rendered by scripts/render_domain_wai_goldens.py
// from the real app.domain.faces in the parity API image.

func refusal(err error) (class, code string, ok bool) {
	var e *Error
	if errors.As(err, &e) {
		return "FaceError", e.Code, true
	}
	return "", "", false
}

func TestFacesMatchPython(t *testing.T) {
	files := oracletest.Load(t, "testdata/python_*.json")
	oracletest.Agree(t, files, "faces", replay, refusal)
}

func TestFacesConstantsMatchPython(t *testing.T) {
	c := oracletest.Constants(t, oracletest.Load(t, "testdata/python_*.json"), "faces")
	if got, _ := oracletest.Int64(c["MAX_FACES"]); got != MaxFaces {
		t.Errorf("MAX_FACES: Python %v, Go %d", c["MAX_FACES"], MaxFaces)
	}
}

func replay(c oracletest.Case, args map[string]any) (any, error) {
	if c.Fn != "anonymous_boxes" {
		return nil, oracletest.Decode(errors.New(c.Fn))
	}
	width, err := oracletest.Int64(args["image_width"])
	if err != nil {
		return nil, oracletest.Decode(err)
	}
	height, err := oracletest.Int64(args["image_height"])
	if err != nil {
		return nil, oracletest.Decode(err)
	}
	items, err := oracletest.List(args["boxes"])
	if err != nil {
		return nil, oracletest.Decode(err)
	}
	boxes := make([]Box, 0, len(items))
	for _, item := range items {
		row, err := oracletest.Row(item, "x", "y", "width", "height")
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		x, err := oracletest.Int64(row["x"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		y, err := oracletest.Int64(row["y"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		w, err := oracletest.Int64(row["width"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		h, err := oracletest.Int64(row["height"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		boxes = append(boxes, Box{X: int(x), Y: int(y), Width: int(w), Height: int(h)})
	}
	got, err := AnonymousBoxes(boxes, int(width), int(height))
	if err != nil {
		return nil, err
	}
	out := make([]any, 0, len(got))
	for _, b := range got {
		out = append(out, map[string]any{
			"box_key": b.BoxKey, "x": b.X, "y": b.Y, "width": b.Width, "height": b.Height,
		})
	}
	return out, nil
}

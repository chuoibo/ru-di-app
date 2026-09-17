package reel

import (
	"errors"
	"testing"

	"mobile/services/core/internal/oracletest"
	"mobile/services/core/internal/treejson"
)

// testdata/python_reel*.json is rendered by scripts/render_domain_wai_goldens.py
// from the real app.domain.reel in the parity API image.

func refusal(err error) (class, code string, ok bool) {
	var e *Error
	if errors.As(err, &e) {
		return "ReelError", e.Code, true
	}
	return "", "", false
}

func TestReelMatchesPython(t *testing.T) {
	files := oracletest.Load(t, "testdata/python_*.json")
	oracletest.Agree(t, files, "reel", replay, refusal)
}

func replay(c oracletest.Case, args map[string]any) (any, error) {
	if c.Fn != "ground_reel" {
		return nil, oracletest.Decode(errors.New(c.Fn))
	}
	raw, err := oracletest.AsPyJSON(args["raw"])
	if err != nil {
		return nil, oracletest.Decode(err)
	}
	items, err := oracletest.List(args["memories"])
	if err != nil {
		return nil, oracletest.Decode(err)
	}
	memories := make([]Memory, 0, len(items))
	for _, item := range items {
		row, err := oracletest.Row(item, "id", "created_at", "reaction_count", "comment_count")
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		id, err := oracletest.Str(row["id"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		created, err := oracletest.Str(row["created_at"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		hearts, err := oracletest.Int64(row["reaction_count"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		comments, err := oracletest.Int64(row["comment_count"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		image, err := oracletest.OptionalString(item.(map[string]any)["image_url"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		caption, err := oracletest.OptionalString(item.(map[string]any)["caption"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		place, err := oracletest.OptionalString(item.(map[string]any)["place_name"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		memories = append(memories, Memory{
			ID: id, ImageURL: image, Caption: caption, PlaceName: place,
			CreatedAt: created, ReactionCount: hearts, CommentCount: comments,
		})
	}
	got, err := Ground(treejson.To(raw), memories)
	if err != nil {
		return nil, err
	}
	return oracletest.FromPyJSON(treejson.From(got)), nil
}

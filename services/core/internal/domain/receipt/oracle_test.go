package receipt

import (
	"errors"
	"math/big"
	"testing"

	"mobile/services/core/internal/oracletest"
)

// testdata/python_hoadon*.json was rendered by scripts/render_domain_ai_goldens.py
// from the real app.domain.receipt before ADR-0052 deleted it; they are frozen
// vectors now.

func refusal(err error) (class, code string, ok bool) {
	var e *Error
	if errors.As(err, &e) {
		return "ReceiptError", e.Code, true
	}
	return "", "", false
}

func TestReceiptMatchesPython(t *testing.T) {
	files := oracletest.Load(t, "testdata/python_*.json")
	report := oracletest.Agree(t, files, "hoadon", replay, refusal)
	if report.ByFn["normalize_vnd"] == nil || report.ByFn["read_scanned_document"] == nil {
		t.Fatalf("a function has no cases: %+v", report.ByFn)
	}
}

func TestConstantsMatchPython(t *testing.T) {
	c := oracletest.Constants(t, oracletest.Load(t, "testdata/python_hoadon.json"), "hoadon")
	if c["CONFIDENCE_FLOOR"] != int64(ConfidenceFloor) || c["CONFIDENCE_REVIEW"] != int64(ConfidenceReview) {
		t.Fatalf("constants: %v", c)
	}
}

// fromOracle turns an oracle value into what the reader's JSON decoder
// gives: ints and floats stay numbers, big ints become exact big numbers.
func fromOracle(v any) any {
	switch x := v.(type) {
	case oracletest.BigInt:
		return x
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			out[i] = fromOracle(item)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, item := range x {
			out[k] = fromOracle(item)
		}
		return out
	}
	return v
}

func replay(c oracletest.Case, args map[string]any) (any, error) {
	switch c.Fn {
	case "normalize_vnd":
		n, err := NormalizeVND(fromOracle(args["text"]))
		if err != nil {
			return nil, err
		}
		return n, nil
	case "read_scanned_document":
		r, err := ReadScannedDocument(fromOracle(args["raw"]))
		if err != nil {
			return nil, err
		}
		return wire(r), nil
	}
	return nil, oracletest.Decode(errors.New(c.Fn))
}

// wire is Python's result dict.
func wire(r Reading) map[string]any {
	items := make([]any, len(r.Items))
	for i, it := range r.Items {
		var unit any
		if it.UnitPriceVND != nil {
			unit = *it.UnitPriceVND
		}
		items[i] = map[string]any{"name": it.Name, "quantity": exact(it.Quantity), "unit_price_vnd": unit, "line_total_vnd": it.LineTotalVND}
	}
	var total, agree, diff any
	if r.TotalVND != nil {
		total, agree, diff = *r.TotalVND, *r.TotalsAgree, *r.TotalDifferenceVND
	}
	warnings := make([]any, len(r.Warnings))
	for i, w := range r.Warnings {
		warnings[i] = w
	}
	return map[string]any{
		"items": items, "items_total_vnd": r.ItemsTotalVND, "total_vnd": total, "totals_agree": agree,
		"total_difference_vnd": diff, "confidence": int64(r.Confidence), "needs_review": r.NeedsReview, "warnings": warnings,
	}
}

func exact(n *big.Int) any { return oracletest.Exact(n) }
